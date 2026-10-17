package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"antares/api/internal/db"
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

func fromRow(row db.Job) Job {
	job := Job{
		ID: row.ID, DedupeKey: row.DedupeKey, Kind: row.Kind, State: row.State, TargetID: row.TargetID,
		Request: json.RawMessage(row.RequestJson), Error: row.Error.String,
	}
	if row.ResultJson.Valid {
		job.Result = json.RawMessage(row.ResultJson.String)
	}
	job.CreatedAt, _ = store.ParseTime(row.CreatedAt)
	if row.StartedAt.Valid {
		t, _ := store.ParseTime(row.StartedAt.String)
		job.StartedAt = &t
	}
	if row.FinishedAt.Valid {
		t, _ := store.ParseTime(row.FinishedAt.String)
		job.FinishedAt = &t
	}
	return job
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
	q := db.New(tx)

	row, err := q.ActiveJobByKey(ctx, in.DedupeKey)
	job, created := fromRow(row), false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id, err := newID()
		if err != nil {
			return Joined{}, err
		}
		if err := q.InsertJob(ctx, db.InsertJobParams{
			ID: id, DedupeKey: in.DedupeKey, Kind: in.Kind, State: StateQueued,
			TargetID: in.TargetID, RequestJson: string(in.Request), CreatedAt: store.FormatTime(now),
		}); err != nil {
			return Joined{}, err
		}
		job = Job{ID: id, DedupeKey: in.DedupeKey, Kind: in.Kind, State: StateQueued,
			TargetID: in.TargetID, Request: in.Request, CreatedAt: now}
		created = true
	case err != nil:
		return Joined{}, err
	}

	if err := q.InsertSubscription(ctx, db.InsertSubscriptionParams{
		TokenHash: HashToken(token), JobID: job.ID,
		CreatedAt: store.FormatTime(now), ExpiresAt: store.FormatTime(now.Add(retention)),
	}); err != nil {
		return Joined{}, err
	}
	if err := tx.Commit(); err != nil {
		return Joined{}, err
	}
	return Joined{Job: job, Token: token, Created: created}, nil
}

func (s Store) Get(ctx context.Context, id string) (Job, error) {
	row, err := db.New(s.DB).GetJob(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	return fromRow(row), nil
}

func (s Store) ByToken(ctx context.Context, token string, now time.Time) (Job, bool, error) {
	row, err := db.New(s.DB).JobByToken(ctx, db.JobByTokenParams{TokenHash: HashToken(token), Now: store.FormatTime(now)})
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return fromRow(row), true, nil
}

func (s Store) MarkRunning(ctx context.Context, id string, now time.Time) (bool, error) {
	n, err := db.New(s.DB).MarkJobRunning(ctx, db.MarkJobRunningParams{
		Running: StateRunning, Now: store.NullTime(now), ID: id, Queued: StateQueued,
	})
	return n > 0, err
}

func (s Store) Finish(ctx context.Context, id, state string, result json.RawMessage, errMsg string, now time.Time) error {
	return db.New(s.DB).FinishJob(ctx, db.FinishJobParams{
		State: state, ResultJson: store.NullString(string(result)), Error: store.NullString(errMsg),
		FinishedAt: store.NullTime(now), ID: id,
	})
}

func (s Store) AddMilestone(ctx context.Context, jobID string, seq int, stage, message string, at time.Time) error {
	return db.New(s.DB).AddMilestone(ctx, db.AddMilestoneParams{
		JobID: jobID, Seq: int64(seq), Stage: stage, Message: message, At: store.FormatTime(at),
	})
}

func (s Store) Milestones(ctx context.Context, jobID string) ([]Milestone, error) {
	rows, err := db.New(s.DB).Milestones(ctx, jobID)
	if err != nil {
		return nil, err
	}
	var out []Milestone
	for _, row := range rows {
		m := Milestone{Seq: int(row.Seq), Stage: row.Stage, Message: row.Message}
		m.At, _ = store.ParseTime(row.At)
		out = append(out, m)
	}
	return out, nil
}

func (s Store) FailInterrupted(ctx context.Context, now time.Time) (int64, error) {
	return db.New(s.DB).FailActiveJobs(ctx, db.FailActiveJobsParams{
		State: StateFailed, Error: store.NullString(InterruptedMessage), FinishedAt: store.NullTime(now),
	})
}

func (s Store) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := db.New(tx)

	if err := q.DeleteExpiredSubscriptions(ctx, store.FormatTime(now)); err != nil {
		return 0, err
	}
	n, err := q.DeleteUnsubscribedFinishedJobs(ctx)
	if err != nil {
		return 0, err
	}
	return n, tx.Commit()
}
