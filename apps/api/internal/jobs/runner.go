package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/rachzy/antares/apps/api/internal/cache"
	"github.com/rachzy/antares/apps/api/internal/hub"
	"github.com/rachzy/antares/apps/api/internal/upstream"
)

var (
	errStalled  = errors.New("stalled")
	errTimedOut = errors.New("timed out")
)

type TargetInfo struct {
	Catalog     string  `json:"catalog"`
	CatalogID   string  `json:"catalog_id"`
	DisplayName string  `json:"display_name"`
	Designation string  `json:"designation"`
	RA          float64 `json:"ra_deg"`
	Dec         float64 `json:"dec_deg"`
}

type Payload struct {
	Target   TargetInfo          `json:"target"`
	TargetID int64               `json:"target_id"`
	Params   cache.ExtractParams `json:"params"`
	RunID    string              `json:"run_id,omitempty"`
	StarID   string              `json:"star_id,omitempty"`
}

type Result struct {
	Target        TargetInfo       `json:"target"`
	Cached        bool             `json:"cached"`
	ShaulaVersion string           `json:"shaula_version"`
	Features      []map[string]any `json:"features"`
	ModelRunID    string           `json:"model_run_id,omitempty"`
	SchemaVersion int              `json:"schema_version,omitempty"`
	Predictions   []map[string]any `json:"predictions,omitempty"`
}

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

func (e *execution) touch() { e.last.Store(time.Now().UnixNano()) }

func (e *execution) record(ev hub.Event, stage, message string) {
	e.touch()
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
	if p.Stage == "done" {
		return
	}
	ev := e.room.Append(hub.Progress(p.Stage, p.Message, p.Fraction))
	if p.Stage != e.stage {
		e.stage = p.Stage
		e.record(ev, p.Stage, p.Message)
		return
	}
	e.touch()
}

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

func (r *Runner) Execute(parent context.Context, jobID string) {
	room := r.Hub.Room(jobID)
	defer room.Close()

	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	ctx, stop := context.WithTimeoutCause(ctx, r.JobTimeout, errTimedOut)
	defer stop()

	job, err := r.Jobs.Get(ctx, jobID)
	if err != nil {
		r.fail(jobID, room, fmt.Sprintf("load job: %v", err))
		return
	}
	started, err := r.Jobs.MarkRunning(ctx, jobID, r.Clock())
	if err != nil {
		r.fail(jobID, room, fmt.Sprintf("start job: %v", err))
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
		r.fail(jobID, room, r.describe(ctx, err))
		return
	}
	payload, err := json.Marshal(result)
	if err != nil {
		r.fail(jobID, room, fmt.Sprintf("encode result: %v", err))
		return
	}
	if err := r.Jobs.Finish(context.WithoutCancel(ctx), jobID, StateDone, payload, "", r.Clock()); err != nil {
		r.fail(jobID, room, fmt.Sprintf("save result: %v", err))
		return
	}
	room.Append(hub.Result(payload))
}

func (r *Runner) Drop(ctx context.Context, jobID, message string) {
	room := r.Hub.Room(jobID)
	if err := r.Jobs.Finish(ctx, jobID, StateDropped, nil, message, r.Clock()); err != nil {
		r.Log.Error("record drop", "job", jobID, "error", err)
	}
	room.Append(hub.Failure(message))
	room.Close()
}

func (r *Runner) fail(jobID string, room *hub.Room, message string) {
	if err := r.Jobs.Finish(context.Background(), jobID, StateFailed, nil, message, r.Clock()); err != nil {
		r.Log.Error("record failure", "job", jobID, "error", err)
	}
	room.Append(hub.Failure(message))
}

func (r *Runner) describe(ctx context.Context, err error) string {
	cause := context.Cause(ctx)
	switch {
	case errors.Is(cause, errStalled):
		return fmt.Sprintf("no progress for %s; giving up", r.StallTimeout)
	case errors.Is(cause, errTimedOut):
		return fmt.Sprintf("exceeded the %s time limit", r.JobTimeout)
	case cause != nil && errors.Is(err, context.Canceled):
		return "interrupted: the API is shutting down"
	}
	return err.Error()
}

func (r *Runner) guarded(ctx context.Context, e *execution, job Job) (result Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.Log.Error("job panicked", "job", job.ID, "panic", recovered)
			err = errors.New("internal error")
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
		return Result{}, err
	}

	version := set.LibraryVersion
	if version == "" {
		version = r.ShaulaVersion()
	}
	rows := cache.SanitizeRows(set.Rows)
	now := r.Clock()
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
		return Result{}, fmt.Errorf("prediction failed: %w", err)
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
