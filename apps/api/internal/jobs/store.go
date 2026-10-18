// Package jobs stores jobs and their subscriptions, schedules them onto a bounded number of slots
// and runs them against Shaula and Fang, publishing progress through the hub.
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

// Job states, kinds and stored messages.
const (
	// Queued and running jobs are active; done, failed and dropped are terminal.
	StateQueued  = "queued"
	StateRunning = "running"
	StateDone    = "done"
	StateFailed  = "failed"
	StateDropped = "dropped"

	// An extract job stops after the features; an analyze job also scores them with Fang.
	KindExtract = "extract"
	KindAnalyze = "analyze"

	// InterruptedMessage is the error recorded on jobs left active by the API's previous run.
	InterruptedMessage = "interrupted by an API restart; resubmit"
)

// ErrNotFound is returned by Store.Get for an unknown job id.
var ErrNotFound = errors.New("job not found")

// Job is a row of the jobs table. At most one active job has a given DedupeKey. Request holds the
// job's Payload JSON; Result the Result JSON once done; Error the client-facing message once failed
// or dropped. StartedAt and FinishedAt are nil until then.
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

// Terminal reports whether the job is done, failed or dropped.
func (j Job) Terminal() bool {
	switch j.State {
	case StateDone, StateFailed, StateDropped:
		return true
	}
	return false
}

// Milestone is a persisted state change of a job, kept so its history can be replayed after its
// room is gone. Seq is the event's Seq in the room.
type Milestone struct {
	Seq     int
	Stage   string
	Message string
	At      time.Time
}

// NewJob describes the job Store.Join joins or creates.
type NewJob struct {
	Kind      string
	DedupeKey string
	TargetID  int64
	Request   json.RawMessage
}

// Joined is the result of Store.Join: the job, the caller's own subscription token, and whether
// this call created the job and so must enqueue it.
type Joined struct {
	Job     Job
	Token   string
	Created bool
}

// Store reads and writes the jobs, subscriptions and job_milestones tables.
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

// Join subscribes the caller to the active job with in.DedupeKey, creating a queued one if there is
// none, and issues a token valid for retention. Concurrent identical requests share one job, but
// every caller gets its own token, so each polls and expires on its own; only the token's hash is
// stored.
func (s Store) Join(ctx context.Context, in NewJob, now time.Time, retention time.Duration) (Joined, error) {
	token, err := NewToken()
	if err != nil {
		return Joined{}, err
	}
	// Immediate transactions (see store.Open) make the read-then-insert atomic, and the partial
	// unique index jobs_one_active_per_key enforces one active job per key regardless. If the index
	// fires, another writer created the job first, and the retry joins it.
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

// Get returns the job with id, or ErrNotFound.
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

// ByToken returns the job a token subscribes to; the bool is false for an unknown or expired token.
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

// MarkRunning moves a queued job to running and reports whether it did; false means the job was no
// longer queued.
func (s Store) MarkRunning(ctx context.Context, id string, now time.Time) (bool, error) {
	n, err := db.New(s.DB).MarkJobRunning(ctx, db.MarkJobRunningParams{
		Running: StateRunning, Now: store.NullTime(now), ID: id, Queued: StateQueued,
	})
	return n > 0, err
}

// Finish records a terminal state with its result or error message, either of which may be empty.
// Only active jobs change, so the first terminal state wins.
func (s Store) Finish(ctx context.Context, id, state string, result json.RawMessage, errMsg string, now time.Time) error {
	return db.New(s.DB).FinishJob(ctx, db.FinishJobParams{
		State: state, ResultJson: store.NullString(string(result)), Error: store.NullString(errMsg),
		FinishedAt: store.NullTime(now), ID: id,
	})
}

// AddMilestone persists a milestone; for a seq already recorded it keeps the first.
func (s Store) AddMilestone(ctx context.Context, jobID string, seq int, stage, message string, at time.Time) error {
	return db.New(s.DB).AddMilestone(ctx, db.AddMilestoneParams{
		JobID: jobID, Seq: int64(seq), Stage: stage, Message: message, At: store.FormatTime(at),
	})
}

// Milestones returns a job's milestones in Seq order.
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

// FailInterrupted marks every job still queued or running as failed with InterruptedMessage and
// returns how many it marked. It runs at startup: no job survives a restart.
func (s Store) FailInterrupted(ctx context.Context, now time.Time) (int64, error) {
	return db.New(s.DB).FailActiveJobs(ctx, db.FailActiveJobsParams{
		State: StateFailed, Error: store.NullString(InterruptedMessage), FinishedAt: store.NullTime(now),
	})
}

// DeleteExpired removes expired subscriptions, then every finished job no subscription refers to,
// with its milestones, and returns how many jobs it deleted.
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
