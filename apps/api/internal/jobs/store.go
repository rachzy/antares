package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"antares/api/internal/store"
)

const (
	StateQueued  = "queued"
	StateRunning = "running"
	StateDone    = "done"
	StateFailed  = "failed"
	StateDropped = "dropped"

	KindExtract = "extract"
	KindAnalyze = "analyze"

	InterruptedMessage = "interrupted by an API restart; resubmit"
)

var ErrNotFound = errors.New("job not found")

type Job struct {
	ID         string
	DedupeKey  string
	Kind       string
	State      string
	TargetID   int64
	Request    json.RawMessage
	Result     json.RawMessage
	Error      string
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

func (j Job) Terminal() bool {
	switch j.State {
	case StateDone, StateFailed, StateDropped:
		return true
	}
	return false
}

type Milestone struct {
	Seq     int
	Stage   string
	Message string
	At      time.Time
}

type NewJob struct {
	Kind      string
	DedupeKey string
	TargetID  int64
	Request   json.RawMessage
}

type Joined struct {
	Job     Job
	Token   string
	Created bool
}

type Store struct{ DB *sql.DB }

const jobColumns = `id, dedupe_key, kind, state, target_id, request_json, result_json, error, created_at, started_at, finished_at`

const joinedJobColumns = `j.id, j.dedupe_key, j.kind, j.state, j.target_id, j.request_json, j.result_json, j.error, j.created_at, j.started_at, j.finished_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanJob(row rowScanner) (Job, error) {
	var (
		job                                Job
		request, created                   string
		result, errText, started, finished sql.NullString
	)
	if err := row.Scan(&job.ID, &job.DedupeKey, &job.Kind, &job.State, &job.TargetID,
		&request, &result, &errText, &created, &started, &finished); err != nil {
		return Job{}, err
	}
	job.Request = json.RawMessage(request)
	if result.Valid {
		job.Result = json.RawMessage(result.String)
	}
	job.Error = errText.String
	job.CreatedAt, _ = store.ParseTime(created)
	if started.Valid {
		t, _ := store.ParseTime(started.String)
		job.StartedAt = &t
	}
	if finished.Valid {
		t, _ := store.ParseTime(finished.String)
		job.FinishedAt = &t
	}
	return job, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func (s Store) Join(ctx context.Context, in NewJob, now time.Time, retention time.Duration) (Joined, error) {
	token, err := NewToken()
	if err != nil {
		return Joined{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		joined, err := s.joinOnce(ctx, in, token, now, retention)
		if err == nil {
			return joined, nil
		}
		if !isUniqueViolation(err) {
			return Joined{}, err
		}
	}
	return Joined{}, errors.New("could not join or create a job after several attempts")
}

func (s Store) joinOnce(ctx context.Context, in NewJob, token string, now time.Time, retention time.Duration) (Joined, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Joined{}, err
	}
	defer func() { _ = tx.Rollback() }()

	job, err := scanJob(tx.QueryRowContext(ctx,
		`SELECT `+jobColumns+` FROM jobs WHERE dedupe_key = ? AND state IN ('queued', 'running')`, in.DedupeKey))
	created := false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id, err := newID()
		if err != nil {
			return Joined{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO jobs (id, dedupe_key, kind, state, target_id, request_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, in.DedupeKey, in.Kind, StateQueued, in.TargetID, string(in.Request), store.FormatTime(now)); err != nil {
			return Joined{}, err
		}
		job = Job{ID: id, DedupeKey: in.DedupeKey, Kind: in.Kind, State: StateQueued,
			TargetID: in.TargetID, Request: in.Request, CreatedAt: now}
		created = true
	case err != nil:
		return Joined{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO subscriptions (token_hash, job_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		HashToken(token), job.ID, store.FormatTime(now), store.FormatTime(now.Add(retention))); err != nil {
		return Joined{}, err
	}
	if err := tx.Commit(); err != nil {
		return Joined{}, err
	}
	return Joined{Job: job, Token: token, Created: created}, nil
}

func (s Store) Get(ctx context.Context, id string) (Job, error) {
	job, err := scanJob(s.DB.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return job, err
}

func (s Store) ByToken(ctx context.Context, token string, now time.Time) (Job, bool, error) {
	job, err := scanJob(s.DB.QueryRowContext(ctx, `
		SELECT `+joinedJobColumns+` FROM subscriptions s JOIN jobs j ON j.id = s.job_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, HashToken(token), store.FormatTime(now)))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func (s Store) MarkRunning(ctx context.Context, id string, now time.Time) (bool, error) {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE jobs SET state = ?, started_at = ? WHERE id = ? AND state = ?`,
		StateRunning, store.FormatTime(now), id, StateQueued)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s Store) Finish(ctx context.Context, id, state string, result json.RawMessage, errMsg string, now time.Time) error {
	var resultValue, errorValue any
	if len(result) > 0 {
		resultValue = string(result)
	}
	if errMsg != "" {
		errorValue = errMsg
	}
	_, err := s.DB.ExecContext(ctx, `
		UPDATE jobs SET state = ?, result_json = ?, error = ?, finished_at = ?
		WHERE id = ? AND state IN ('queued', 'running')`,
		state, resultValue, errorValue, store.FormatTime(now), id)
	return err
}

func (s Store) AddMilestone(ctx context.Context, jobID string, seq int, stage, message string, at time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO job_milestones (job_id, seq, stage, message, at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (job_id, seq) DO NOTHING`, jobID, seq, stage, message, store.FormatTime(at))
	return err
}

func (s Store) Milestones(ctx context.Context, jobID string) ([]Milestone, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT seq, stage, message, at FROM job_milestones WHERE job_id = ? ORDER BY seq`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Milestone
	for rows.Next() {
		var m Milestone
		var at string
		if err := rows.Scan(&m.Seq, &m.Stage, &m.Message, &at); err != nil {
			return nil, err
		}
		m.At, _ = store.ParseTime(at)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s Store) FailInterrupted(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE jobs SET state = ?, error = ?, finished_at = ? WHERE state IN ('queued', 'running')`,
		StateFailed, InterruptedMessage, store.FormatTime(now))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s Store) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM subscriptions WHERE expires_at <= ?`, store.FormatTime(now)); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `
		DELETE FROM jobs WHERE state IN ('done', 'failed', 'dropped')
		AND NOT EXISTS (SELECT 1 FROM subscriptions s WHERE s.job_id = jobs.id)`)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, tx.Commit()
}
