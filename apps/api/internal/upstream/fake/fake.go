// Package fake provides in-memory upstream.Shaula and upstream.Fang for tests, with canned defaults
// and call recording.
package fake

import (
	"context"
	"sync"

	"antares/api/internal/upstream"
)

var (
	_ upstream.Shaula = (*Shaula)(nil)
	_ upstream.Fang   = (*Fang)(nil)
)

// DefaultResolved is what the fake Shaula resolves every query to: Kepler-186.
func DefaultResolved() upstream.Resolved {
	return upstream.Resolved{
		Catalog: "KIC", CatalogID: "8120608", RA: 298.65272, Dec: 43.95502,
		DisplayName: "Kepler-186", Designation: "KIC 8120608",
	}
}

// DefaultFeatureSet is DefaultExtract's result: one row, with one null feature.
func DefaultFeatureSet(req upstream.ExtractRequest) upstream.FeatureSet {
	return upstream.FeatureSet{
		Target:         req.Target,
		Mission:        req.Mission,
		LibraryVersion: "0.0.1a0",
		Rows:           []map[string]any{{"period_days": 129.9, "MES": 12.5, "vshape_metric": nil}},
	}
}

// DefaultExtract reports four stages, ending with Shaula's own "done", then returns
// DefaultFeatureSet.
func DefaultExtract(_ context.Context, req upstream.ExtractRequest, onProgress func(upstream.Progress)) (upstream.FeatureSet, error) {
	for _, stage := range []string{"downloading", "detrending", "period_search", "done"} {
		onProgress(upstream.Progress{Stage: stage, Message: "stage " + stage})
	}
	return DefaultFeatureSet(req), nil
}

// DefaultRun is the run the fake Fang lists by default.
func DefaultRun() upstream.Run {
	return upstream.Run{
		RunID: "20261001T120000Z-abcd1234", CreatedAt: "2026-10-01T12:00:00Z",
		SchemaVersion: 1, SelectedModel: "lightgbm", Threshold: 0.42,
	}
}

// Shaula is a fake upstream.Shaula; it is safe for concurrent use. An empty LibraryVersion reports
// "0.0.1a0", a set VersionErr fails Version, and nil ResolveFn or ExtractFn select DefaultResolved
// and DefaultExtract.
type Shaula struct {
	LibraryVersion string
	VersionErr     error
	ResolveFn      func(ctx context.Context, req upstream.ResolveRequest) (upstream.Resolved, error)
	ExtractFn      func(ctx context.Context, req upstream.ExtractRequest, onProgress func(upstream.Progress)) (upstream.FeatureSet, error)

	mu              sync.Mutex
	resolveCalls    int
	extractRequests []upstream.ExtractRequest
}

// Version returns VersionErr if set, otherwise the library version.
func (f *Shaula) Version(context.Context) (string, error) {
	if f.VersionErr != nil {
		return "", f.VersionErr
	}
	if f.LibraryVersion == "" {
		return "0.0.1a0", nil
	}
	return f.LibraryVersion, nil
}

// Resolve counts the call and delegates to ResolveFn or DefaultResolved.
func (f *Shaula) Resolve(ctx context.Context, req upstream.ResolveRequest) (upstream.Resolved, error) {
	f.mu.Lock()
	f.resolveCalls++
	f.mu.Unlock()
	if f.ResolveFn != nil {
		return f.ResolveFn(ctx, req)
	}
	return DefaultResolved(), nil
}

// Extract records req and delegates to ExtractFn or DefaultExtract.
func (f *Shaula) Extract(ctx context.Context, req upstream.ExtractRequest, onProgress func(upstream.Progress)) (upstream.FeatureSet, error) {
	f.mu.Lock()
	f.extractRequests = append(f.extractRequests, req)
	f.mu.Unlock()
	if f.ExtractFn != nil {
		return f.ExtractFn(ctx, req, onProgress)
	}
	return DefaultExtract(ctx, req, onProgress)
}

// ResolveCount returns how many times Resolve was called.
func (f *Shaula) ResolveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.resolveCalls
}

// ExtractCount returns how many times Extract was called.
func (f *Shaula) ExtractCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.extractRequests)
}

// ExtractRequests returns a copy of the requests Extract received, in order.
func (f *Shaula) ExtractRequests() []upstream.ExtractRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]upstream.ExtractRequest(nil), f.extractRequests...)
}

// Fang is a fake upstream.Fang; it is safe for concurrent use. An empty LibraryVersion reports
// "0.0.1a0" and a set VersionErr fails Version. ListRuns fails with ListRunsErr, lists nothing with
// NoRuns, and otherwise returns Runs, or DefaultRun if Runs is nil. A nil PredictFn selects the
// default Predict.
type Fang struct {
	LibraryVersion string
	VersionErr     error
	Runs           []upstream.Run
	NoRuns         bool
	ListRunsErr    error
	PredictFn      func(ctx context.Context, req upstream.PredictRequest) (upstream.Prediction, error)

	mu              sync.Mutex
	predictRequests []upstream.PredictRequest
}

// Version returns VersionErr if set, otherwise the library version.
func (f *Fang) Version(context.Context) (string, error) {
	if f.VersionErr != nil {
		return "", f.VersionErr
	}
	if f.LibraryVersion == "" {
		return "0.0.1a0", nil
	}
	return f.LibraryVersion, nil
}

// ListRuns returns the configured runs; see Fang.
func (f *Fang) ListRuns(context.Context) ([]upstream.Run, error) {
	if f.ListRunsErr != nil {
		return nil, f.ListRunsErr
	}
	if f.NoRuns {
		return nil, nil
	}
	if f.Runs != nil {
		return f.Runs, nil
	}
	return []upstream.Run{DefaultRun()}, nil
}

// Predict records req and delegates to PredictFn; by default it copies each row and adds prob_stack
// 0.71, the requested model_run_id and star_id (default "unknown").
func (f *Fang) Predict(ctx context.Context, req upstream.PredictRequest) (upstream.Prediction, error) {
	f.mu.Lock()
	f.predictRequests = append(f.predictRequests, req)
	f.mu.Unlock()
	if f.PredictFn != nil {
		return f.PredictFn(ctx, req)
	}
	starID := req.StarID
	if starID == "" {
		starID = "unknown"
	}
	rows := make([]map[string]any, len(req.Rows))
	for i, row := range req.Rows {
		out := make(map[string]any, len(row)+3)
		for key, value := range row {
			out[key] = value
		}
		out["prob_stack"] = 0.71
		out["model_run_id"] = req.RunID
		out["star_id"] = starID
		rows[i] = out
	}
	return upstream.Prediction{Rows: rows, ModelRunID: req.RunID, SchemaVersion: 1}, nil
}

// PredictCount returns how many times Predict was called.
func (f *Fang) PredictCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.predictRequests)
}

// PredictRequests returns a copy of the requests Predict received, in order.
func (f *Fang) PredictRequests() []upstream.PredictRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]upstream.PredictRequest(nil), f.predictRequests...)
}
