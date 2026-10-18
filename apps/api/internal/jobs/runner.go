package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"antares/api/internal/cache"
	"antares/api/internal/hub"
	"antares/api/internal/upstream"
)

// Cancellation causes Execute sets on a job's context, so describe can tell a stall or the time
// limit from a shutdown.
var (
	errStalled  = errors.New("stalled")
	errTimedOut = errors.New("timed out")
)

// internalMessage is what clients see for unexpected failures; the details are only logged.
const internalMessage = "internal error"

// upstreamFailure records which service failed, for the client-facing message.
type upstreamFailure struct {
	service string
	err     error
}

func (f *upstreamFailure) Error() string { return f.service + ": " + f.err.Error() }
func (f *upstreamFailure) Unwrap() error { return f.err }

// TargetInfo is the public description of a target; RA and Dec are ICRS degrees.
type TargetInfo struct {
	Catalog     string  `json:"catalog"`
	CatalogID   string  `json:"catalog_id"`
	DisplayName string  `json:"display_name"`
	Designation string  `json:"designation"`
	RA          float64 `json:"ra_deg"`
	Dec         float64 `json:"dec_deg"`
}

// Payload is a job's request, stored as Job.Request. RunID is the model run picked at submission
// and StarID labels the prediction rows; both only matter to analyze jobs.
type Payload struct {
	Target   TargetInfo          `json:"target"`
	TargetID int64               `json:"target_id"`
	Params   cache.ExtractParams `json:"params"`
	RunID    string              `json:"run_id,omitempty"`
	StarID   string              `json:"star_id,omitempty"`
}

// Result is a job's output, also returned directly on a cache hit. Cached reports that the features
// came from the cache; the model fields are empty for extract jobs.
type Result struct {
	Target        TargetInfo       `json:"target"`
	Cached        bool             `json:"cached"`
	ShaulaVersion string           `json:"shaula_version"`
	Features      []map[string]any `json:"features"`
	ModelRunID    string           `json:"model_run_id,omitempty"`
	SchemaVersion int              `json:"schema_version,omitempty"`
	Predictions   []map[string]any `json:"predictions,omitempty"`
}

// Runner executes jobs: it extracts features with Shaula and caches them, then for analyze jobs
// scores them with Fang and caches the predictions, publishing progress to the job's room.
// ShaulaVersion supplies the version to cache under when an extraction does not report one.
type Runner struct {
	Jobs          Store
	Features      cache.FeatureCache
	Predictions   cache.PredictionCache
	Hub           *hub.Hub
	Shaula        upstream.Shaula
	Fang          upstream.Fang
	Clock         func() time.Time
	Log           *slog.Logger
	JobTimeout    time.Duration
	StallTimeout  time.Duration
	TESSTTL       time.Duration
	ShaulaVersion func() string
}

type execution struct {
	r     *Runner
	room  *hub.Room
	jobID string
	stage string
	last  atomic.Int64
}

// touch uses the wall clock, not Runner.Clock, because watchStall measures real time.
func (e *execution) touch() { e.last.Store(time.Now().UnixNano()) }

func (e *execution) record(ev hub.Event, stage, message string) {
	e.touch()
	// Background: a milestone is bookkeeping for history replay and must be written even after the
	// job's context is cancelled.
	if err := e.r.Jobs.AddMilestone(context.Background(), e.jobID, int(ev.Seq), stage, message, ev.At); err != nil {
		e.r.Log.Warn("persist milestone", "job", e.jobID, "error", err)
	}
}

func (e *execution) state(stage, message string) {
	ev := e.room.Append(hub.State(stage, message))
	e.stage = stage
	e.record(ev, stage, message)
}

func (e *execution) onProgress(p upstream.Progress) {
	// Shaula's own "done" is not relayed: the job is not done until the features are cached and,
	// for analyze, scored. The result event marks the end.
	if p.Stage == "done" {
		return
	}
	fraction := p.Fraction
	if fraction != nil && (math.IsNaN(*fraction) || math.IsInf(*fraction, 0)) {
		fraction = nil
	}
	ev := e.room.Append(hub.Progress(p.Stage, p.Message, fraction))
	// Only a stage change becomes a milestone; progress within a stage just resets the stall timer.
	if p.Stage != e.stage {
		e.stage = p.Stage
		e.record(ev, p.Stage, p.Message)
		return
	}
	e.touch()
}

// watchStall cancels the job with errStalled once no event has arrived for stall. It checks four
// times per stall period, so the cut-off is between stall and 1.25 × stall.
func (e *execution) watchStall(ctx context.Context, cancel context.CancelCauseFunc, stall time.Duration) {
	interval := max(stall/4, time.Millisecond)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Since(time.Unix(0, e.last.Load())) > stall {
				cancel(errStalled)
				return
			}
		}
	}
}

// Execute runs one job and closes its room. It returns quietly if the job is no longer queued. The
// job is bounded by JobTimeout overall and StallTimeout between events, and is interrupted when
// parent is cancelled; any failure is recorded on the job and published as an error event with a
// client-safe message.
func (r *Runner) Execute(parent context.Context, jobID string) {
	room := r.Hub.Room(jobID)
	defer room.Close()

	// Three things can cancel ctx: the parent at shutdown, watchStall (errStalled) and the time
	// limit (errTimedOut). describe reads context.Cause to tell them apart.
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	ctx, stop := context.WithTimeoutCause(ctx, r.JobTimeout, errTimedOut)
	defer stop()

	job, err := r.Jobs.Get(ctx, jobID)
	if err != nil {
		r.fail(jobID, room, r.internal(jobID, fmt.Errorf("load job: %w", err)))
		return
	}
	started, err := r.Jobs.MarkRunning(ctx, jobID, r.Clock())
	if err != nil {
		r.fail(jobID, room, r.internal(jobID, fmt.Errorf("start job: %w", err)))
		return
	}
	if !started {
		return
	}

	e := &execution{r: r, room: room, jobID: jobID}
	e.touch()
	e.state("running", "Started")
	go e.watchStall(ctx, cancel, r.StallTimeout)

	result, err := r.guarded(ctx, e, job)
	if err != nil {
		r.fail(jobID, room, r.describe(ctx, jobID, err))
		return
	}
	payload, err := json.Marshal(result)
	if err != nil {
		r.fail(jobID, room, r.internal(jobID, fmt.Errorf("encode result: %w", err)))
		return
	}
	// The work succeeded, so record it even if the deadline passed meanwhile.
	if err := r.Jobs.Finish(context.WithoutCancel(ctx), jobID, StateDone, payload, "", r.Clock()); err != nil {
		r.fail(jobID, room, r.internal(jobID, fmt.Errorf("save result: %w", err)))
		return
	}
	room.Append(hub.Result(payload))
}

// Drop marks a queued job as dropped with message, publishes it and closes the room.
func (r *Runner) Drop(ctx context.Context, jobID, message string) {
	room := r.Hub.Room(jobID)
	if err := r.Jobs.Finish(ctx, jobID, StateDropped, nil, message, r.Clock()); err != nil {
		r.Log.Error("record drop", "job", jobID, "error", err)
	}
	room.Append(hub.Failure(message))
	room.Close()
}

// fail uses Background so the failure is recorded even when the job's own context is what failed.
func (r *Runner) fail(jobID string, room *hub.Room, message string) {
	if err := r.Jobs.Finish(context.Background(), jobID, StateFailed, nil, message, r.Clock()); err != nil {
		r.Log.Error("record failure", "job", jobID, "error", err)
	}
	room.Append(hub.Failure(message))
}

// describe turns a job error into the message clients see. Client-caused upstream errors keep the
// service's own message; an unavailable service gets a generic one; anything else is logged and
// replaced by internalMessage, so internals never leak.
func (r *Runner) describe(ctx context.Context, jobID string, err error) string {
	cause := context.Cause(ctx)
	switch {
	case errors.Is(cause, errStalled):
		return fmt.Sprintf("no progress for %s; giving up", r.StallTimeout)
	case errors.Is(cause, errTimedOut):
		return fmt.Sprintf("exceeded the %s time limit", r.JobTimeout)
	case cause != nil && errors.Is(err, context.Canceled):
		return "interrupted: the API is shutting down"
	}
	var failure *upstreamFailure
	var ue *upstream.Error
	if errors.As(err, &failure) && errors.As(failure.err, &ue) {
		switch ue.Kind {
		case upstream.KindNotFound, upstream.KindInvalid, upstream.KindPrecondition:
			if failure.service == "model" {
				return "prediction failed: " + ue.Message
			}
			return ue.Message
		case upstream.KindUnavailable:
			r.Log.Warn("job failed", "job", jobID, "error", err)
			return fmt.Sprintf("the %s service is unavailable; try again later", failure.service)
		}
	}
	return r.internal(jobID, err)
}

func (r *Runner) internal(jobID string, err error) string {
	r.Log.Error("job failed", "job", jobID, "error", err)
	return internalMessage
}

func (r *Runner) guarded(ctx context.Context, e *execution, job Job) (result Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.Log.Error("job panicked", "job", job.ID, "panic", recovered)
			err = errors.New(internalMessage)
		}
	}()
	return r.work(ctx, e, job)
}

func (r *Runner) work(ctx context.Context, e *execution, job Job) (Result, error) {
	var p Payload
	if err := json.Unmarshal(job.Request, &p); err != nil {
		return Result{}, fmt.Errorf("bad job payload: %w", err)
	}

	set, err := r.Shaula.Extract(ctx, upstream.ExtractRequest{
		Target:       p.Target.Designation,
		Mission:      p.Params.Mission,
		SigmaClip:    p.Params.SigmaClip,
		DownloadAll:  p.Params.DownloadAll,
		Author:       p.Params.Author,
		Exptime:      p.Params.Exptime,
		UseTLS:       p.Params.UseTLS,
		MaskEclipses: p.Params.MaskEclipses,
	}, e.onProgress)
	if err != nil {
		return Result{}, &upstreamFailure{service: "pipeline", err: err}
	}

	version := set.LibraryVersion
	if version == "" {
		version = r.ShaulaVersion()
	}
	rows := cache.SanitizeRows(set.Rows)
	now := r.Clock()
	// Cache the features before scoring, so a later prediction failure still leaves them for the
	// next request.
	if err := r.Features.Put(ctx, cache.FeatureEntry{
		TargetID:      p.TargetID,
		ParamsHash:    p.Params.Hash(),
		ShaulaVersion: version,
		Rows:          rows,
		CreatedAt:     now,
		ExpiresAt:     cache.ExpiresAt(p.Params.Mission, now, r.TESSTTL),
	}); err != nil {
		return Result{}, fmt.Errorf("cache features: %w", err)
	}

	result := Result{Target: p.Target, ShaulaVersion: version, Features: rows}
	if job.Kind != KindAnalyze {
		return result, nil
	}

	e.state("predicting", "Scoring candidates")
	prediction, err := r.Fang.Predict(ctx, upstream.PredictRequest{Rows: rows, RunID: p.RunID, StarID: p.StarID})
	if err != nil {
		return Result{}, &upstreamFailure{service: "model", err: err}
	}
	scored := cache.SanitizeRows(prediction.Rows)
	hash, err := cache.FeaturesHash(rows)
	if err != nil {
		return Result{}, err
	}
	if err := r.Predictions.Put(ctx, cache.PredictionEntry{
		FeaturesHash:  hash,
		ModelRunID:    prediction.ModelRunID,
		SchemaVersion: prediction.SchemaVersion,
		Rows:          scored,
		TargetID:      &p.TargetID,
		CreatedAt:     now,
	}); err != nil {
		return Result{}, fmt.Errorf("cache predictions: %w", err)
	}
	result.ModelRunID = prediction.ModelRunID
	result.SchemaVersion = prediction.SchemaVersion
	result.Predictions = scored
	return result, nil
}
