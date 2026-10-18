package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"antares/api/internal/cache"
	"antares/api/internal/hub"
	"antares/api/internal/jobs"
	"antares/api/internal/store"
	"antares/api/internal/upstream"
)

// ExtractInput is a request to extract, and for Analyze also score, a star's features. A nil
// SigmaClip means 5. ForceRefresh skips the caches and always queues a job. RunID and StarID only
// affect Analyze.
type ExtractInput struct {
	Query        string
	RA           *float64
	Dec          *float64
	Mission      string
	SigmaClip    *float64
	DownloadAll  bool
	Author       string
	Exptime      string
	UseTLS       bool
	MaskEclipses bool
	ForceRefresh bool
	RunID        string
	StarID       string
}

// Outcome is either a finished Result, from the cache, or a queued job: the caller's Token and its
// queue Position, 0 meaning it has a slot already.
type Outcome struct {
	Result   *jobs.Result
	Token    string
	Position int
}

// Extract returns cached features for the star and parameters, or queues an extract job.
func (s *Service) Extract(ctx context.Context, in ExtractInput) (Outcome, error) {
	return s.submit(ctx, jobs.KindExtract, in)
}

// Analyze is Extract plus scoring with a model run, the newest unless RunID is set. On a feature
// cache hit it scores synchronously.
func (s *Service) Analyze(ctx context.Context, in ExtractInput) (Outcome, error) {
	return s.submit(ctx, jobs.KindAnalyze, in)
}

func normaliseParams(in ExtractInput) (cache.ExtractParams, error) {
	mission, _, err := canonicalMission(in.Mission)
	if err != nil {
		return cache.ExtractParams{}, err
	}
	sigma := 5.0
	if in.SigmaClip != nil {
		sigma = *in.SigmaClip
	}
	if math.IsNaN(sigma) || sigma <= 0 || sigma > 50 {
		return cache.ExtractParams{}, errf(KindInvalid, "sigma_clip must be greater than 0 and at most 50")
	}
	author, exptime := strings.TrimSpace(in.Author), strings.TrimSpace(in.Exptime)
	if len(author) > 64 || len(exptime) > 32 {
		return cache.ExtractParams{}, errf(KindInvalid, "author or exptime is too long")
	}
	if len(in.StarID) > maxStarIDLength {
		return cache.ExtractParams{}, errf(KindInvalid, "star_id is longer than %d characters", maxStarIDLength)
	}
	return cache.ExtractParams{
		Mission: mission, SigmaClip: sigma, DownloadAll: in.DownloadAll, Author: author,
		Exptime: exptime, UseTLS: in.UseTLS, MaskEclipses: in.MaskEclipses,
	}, nil
}

func (s *Service) submit(ctx context.Context, kind string, in ExtractInput) (Outcome, error) {
	params, err := normaliseParams(in)
	if err != nil {
		return Outcome{}, err
	}
	version, ok := s.knownShaulaVersion()
	if !ok {
		return Outcome{}, errf(KindUnavailable, "the pipeline service has not been reached yet; try again shortly")
	}
	target, err := s.Resolve(ctx, ResolveInput{Query: in.Query, Mission: params.Mission, RA: in.RA, Dec: in.Dec})
	if err != nil {
		return Outcome{}, err
	}

	var run upstream.Run
	if kind == jobs.KindAnalyze {
		if run, err = s.pickRun(ctx, in.RunID); err != nil {
			return Outcome{}, err
		}
	}

	if !in.ForceRefresh {
		entry, hit, err := s.Features.Get(ctx, target.ID, params.Hash(), version, s.Clock())
		if err != nil {
			return Outcome{}, errf(KindInternal, "feature cache: %v", err)
		}
		if hit {
			result := jobs.Result{Target: InfoOf(target), Cached: true, ShaulaVersion: version, Features: entry.Rows}
			if kind == jobs.KindAnalyze {
				scored, err := s.score(ctx, entry.Rows, entry.FeaturesHash, run, in.StarID, &target.ID)
				if err != nil {
					return Outcome{}, err
				}
				result.ModelRunID, result.SchemaVersion, result.Predictions = scored.runID, scored.schema, scored.rows
			}
			return Outcome{Result: &result}, nil
		}
	}
	return s.enqueue(ctx, kind, target, params, version, run.RunID, in.StarID)
}

// dedupeKey identifies equivalent work: kind, target, extraction parameters, Shaula version and
// model run, so a job started before an upgrade is not joined after it. star_id is left out because
// it only labels the output rows; a caller joining a running job gets the first submitter's star_id
// in its predictions.
func dedupeKey(kind string, targetID int64, paramsHash, version, runID string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{kind, strconv.FormatInt(targetID, 10), paramsHash, version, runID}, "|")))
	return hex.EncodeToString(sum[:])
}

func (s *Service) enqueue(ctx context.Context, kind string, target store.Target, params cache.ExtractParams, version, runID, starID string) (Outcome, error) {
	payload, err := json.Marshal(jobs.Payload{
		Target: InfoOf(target), TargetID: target.ID, Params: params, RunID: runID, StarID: starID,
	})
	if err != nil {
		return Outcome{}, errf(KindInternal, "encode job: %v", err)
	}
	joined, err := s.Jobs.Join(ctx, jobs.NewJob{
		Kind: kind, DedupeKey: dedupeKey(kind, target.ID, params.Hash(), version, runID),
		TargetID: target.ID, Request: payload,
	}, s.Clock(), s.Cfg.TokenRetention)
	if err != nil {
		return Outcome{}, errf(KindInternal, "create job: %v", err)
	}

	room := s.Hub.Room(joined.Job.ID)
	// Count the submitter as present before enqueueing: promotion drops jobs nobody is watching,
	// and the client has not polled or connected yet.
	room.Touch()
	if !joined.Created {
		position, _ := s.Sched.Position(joined.Job.ID)
		return Outcome{Token: joined.Token, Position: position}, nil
	}

	ev := room.Append(hub.State("queued", "Waiting for a free slot"))
	if err := s.Jobs.AddMilestone(ctx, joined.Job.ID, int(ev.Seq), "queued", "Waiting for a free slot", ev.At); err != nil {
		s.log().Warn("persist queued milestone", "job", joined.Job.ID, "error", err)
	}
	position, err := s.Sched.Enqueue(joined.Job.ID)
	if errors.Is(err, jobs.ErrQueueFull) {
		message := "the queue was full"
		// Record the rejection even if the client has gone.
		if ferr := s.Jobs.Finish(context.WithoutCancel(ctx), joined.Job.ID, jobs.StateDropped, nil, message, s.Clock()); ferr != nil {
			s.log().Error("record rejected job", "job", joined.Job.ID, "error", ferr)
		}
		room.Append(hub.Failure(message))
		room.Close()
		return Outcome{}, errf(KindBusy, "the queue is full; try again in a few minutes")
	}
	if err != nil {
		return Outcome{}, errf(KindInternal, "enqueue job: %v", err)
	}
	return Outcome{Token: joined.Token, Position: position}, nil
}

type scored struct {
	rows   []map[string]any
	runID  string
	schema int
	cached bool
}

// score serves predictions from the cache or asks Fang and caches them. One cache entry serves
// every star id, so cached rows get the caller's own star_id.
func (s *Service) score(ctx context.Context, rows []map[string]any, featuresHash string, run upstream.Run, starID string, targetID *int64) (scored, error) {
	entry, hit, err := s.Predictions.Get(ctx, featuresHash, run.RunID)
	if err != nil {
		return scored{}, errf(KindInternal, "prediction cache: %v", err)
	}
	if hit {
		return scored{rows: withStarID(entry.Rows, starID), runID: entry.ModelRunID, schema: entry.SchemaVersion, cached: true}, nil
	}

	pctx, cancel := context.WithTimeout(ctx, predictTimeout)
	defer cancel()
	prediction, err := s.Fang.Predict(pctx, upstream.PredictRequest{Rows: rows, RunID: run.RunID, StarID: starID})
	if err != nil {
		return scored{}, s.fromUpstream("model", err)
	}
	out := cache.SanitizeRows(prediction.Rows)
	if err := s.Predictions.Put(ctx, cache.PredictionEntry{
		FeaturesHash: featuresHash, ModelRunID: prediction.ModelRunID, SchemaVersion: prediction.SchemaVersion,
		Rows: out, TargetID: targetID, CreatedAt: s.Clock(),
	}); err != nil {
		return scored{}, errf(KindInternal, "cache predictions: %v", err)
	}
	return scored{rows: out, runID: prediction.ModelRunID, schema: prediction.SchemaVersion}, nil
}

func withStarID(rows []map[string]any, starID string) []map[string]any {
	if starID == "" {
		starID = "unknown"
	}
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		copied := make(map[string]any, len(row)+1)
		for key, value := range row {
			copied[key] = value
		}
		copied["star_id"] = starID
		out[i] = copied
	}
	return out
}

// Runs lists Fang's servable model runs, newest first: run ids start with their creation time.
func (s *Service) Runs(ctx context.Context) ([]upstream.Run, error) {
	rctx, cancel := context.WithTimeout(ctx, listRunsTimeout)
	defer cancel()
	runs, err := s.Fang.ListRuns(rctx)
	if err != nil {
		return nil, s.fromUpstream("model", err)
	}
	sorted := append([]upstream.Run(nil), runs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].RunID > sorted[j].RunID })
	return sorted, nil
}

func (s *Service) pickRun(ctx context.Context, wanted string) (upstream.Run, error) {
	runs, err := s.Runs(ctx)
	if err != nil {
		return upstream.Run{}, err
	}
	if len(runs) == 0 {
		return upstream.Run{}, errf(KindUnavailable, "no trained model is available yet")
	}
	if wanted == "" {
		return runs[0], nil
	}
	for _, run := range runs {
		if run.RunID == wanted {
			return run, nil
		}
	}
	return upstream.Run{}, errf(KindNotFound, "no servable model run %q", wanted)
}

// PredictInput is a request to score feature rows posted by the client.
type PredictInput struct {
	Rows   []map[string]any
	RunID  string
	StarID string
}

// PredictResult is Predict's response; Cached reports a prediction cache hit.
type PredictResult struct {
	ModelRunID    string           `json:"model_run_id"`
	SchemaVersion int              `json:"schema_version"`
	Predictions   []map[string]any `json:"predictions"`
	Cached        bool             `json:"cached"`
}

// Predict validates and sanitizes the rows, then scores them with the chosen run, reusing cached
// predictions for identical rows.
func (s *Service) Predict(ctx context.Context, in PredictInput) (PredictResult, error) {
	if len(in.Rows) == 0 {
		return PredictResult{}, errf(KindInvalid, "features must contain at least one row")
	}
	if len(in.Rows) > MaxPredictRows {
		return PredictResult{}, errf(KindInvalid, "features may contain at most %d rows", MaxPredictRows)
	}
	for i, row := range in.Rows {
		if len(row) == 0 {
			return PredictResult{}, errf(KindInvalid, "features[%d] is empty", i)
		}
	}
	if len(in.StarID) > maxStarIDLength {
		return PredictResult{}, errf(KindInvalid, "star_id is longer than %d characters", maxStarIDLength)
	}

	rows := cache.SanitizeRows(in.Rows)
	hash, err := cache.FeaturesHash(rows)
	if err != nil {
		return PredictResult{}, errf(KindInvalid, "features cannot be encoded: %v", err)
	}
	run, err := s.pickRun(ctx, in.RunID)
	if err != nil {
		return PredictResult{}, err
	}
	out, err := s.score(ctx, rows, hash, run, in.StarID, nil)
	if err != nil {
		return PredictResult{}, err
	}
	return PredictResult{ModelRunID: out.runID, SchemaVersion: out.schema, Predictions: out.rows, Cached: out.cached}, nil
}
