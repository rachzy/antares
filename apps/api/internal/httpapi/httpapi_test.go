package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"antares/api/internal/config"
	"antares/api/internal/httpapi"
	"antares/api/internal/service"
	"antares/api/internal/store"
	"antares/api/internal/upstream"
)

var errUnexpected = errors.New("stub: unexpected call")

type stub struct {
	resolve func(context.Context, service.ResolveInput) (store.Target, error)
	extract func(context.Context, service.ExtractInput) (service.Outcome, error)
	analyze func(context.Context, service.ExtractInput) (service.Outcome, error)
	predict func(context.Context, service.PredictInput) (service.PredictResult, error)
	runs    func(context.Context) ([]upstream.Run, error)
	watch   func(context.Context, string, uint64) (*service.Watch, error)
	status  func(context.Context, string) (service.SubscriptionView, error)

	latestFeatures        func(context.Context, service.ResolveInput) (service.FeaturesView, error)
	latestFeaturesByToken func(context.Context, string) (service.FeaturesView, error)
	starInfo              func(context.Context, service.ResolveInput) (service.InfoView, error)

	shaulaVersion, fangVersion string
}

func (s *stub) StarInfo(ctx context.Context, in service.ResolveInput) (service.InfoView, error) {
	if s.starInfo == nil {
		return service.InfoView{}, errUnexpected
	}
	return s.starInfo(ctx, in)
}

func (s *stub) Resolve(ctx context.Context, in service.ResolveInput) (store.Target, error) {
	if s.resolve == nil {
		return store.Target{}, errUnexpected
	}
	return s.resolve(ctx, in)
}

func (s *stub) Extract(ctx context.Context, in service.ExtractInput) (service.Outcome, error) {
	if s.extract == nil {
		return service.Outcome{}, errUnexpected
	}
	return s.extract(ctx, in)
}

func (s *stub) Analyze(ctx context.Context, in service.ExtractInput) (service.Outcome, error) {
	if s.analyze == nil {
		return service.Outcome{}, errUnexpected
	}
	return s.analyze(ctx, in)
}

func (s *stub) Predict(ctx context.Context, in service.PredictInput) (service.PredictResult, error) {
	if s.predict == nil {
		return service.PredictResult{}, errUnexpected
	}
	return s.predict(ctx, in)
}

func (s *stub) Runs(ctx context.Context) ([]upstream.Run, error) {
	if s.runs == nil {
		return nil, errUnexpected
	}
	return s.runs(ctx)
}

func (s *stub) Watch(ctx context.Context, token string, since uint64) (*service.Watch, error) {
	if s.watch == nil {
		return nil, errUnexpected
	}
	return s.watch(ctx, token, since)
}

func (s *stub) Status(ctx context.Context, token string) (service.SubscriptionView, error) {
	if s.status == nil {
		return service.SubscriptionView{}, errUnexpected
	}
	return s.status(ctx, token)
}

func (s *stub) LatestFeatures(ctx context.Context, in service.ResolveInput) (service.FeaturesView, error) {
	if s.latestFeatures == nil {
		return service.FeaturesView{}, errUnexpected
	}
	return s.latestFeatures(ctx, in)
}

func (s *stub) LatestFeaturesByToken(ctx context.Context, token string) (service.FeaturesView, error) {
	if s.latestFeaturesByToken == nil {
		return service.FeaturesView{}, errUnexpected
	}
	return s.latestFeaturesByToken(ctx, token)
}

func (s *stub) Versions() (string, string) { return s.shaulaVersion, s.fangVersion }

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func handler(t *testing.T, s *stub, mutate func(*config.Config)) http.Handler {
	t.Helper()
	cfg, err := config.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return httpapi.New(s, cfg, quietLog())
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not an error document: %v", rec.Body.String(), err)
	}
	return body
}

func TestHealthzReportsVersionsAndReadiness(t *testing.T) {
	ready := do(handler(t, &stub{shaulaVersion: "0.0.1a0", fangVersion: "0.0.1a0"}, nil), "GET", "/healthz", "")
	var body map[string]any
	_ = json.Unmarshal(ready.Body.Bytes(), &body)
	if ready.Code != 200 || body["ready"] != true || body["shaula_version"] != "0.0.1a0" || body["status"] != "ok" {
		t.Fatalf("ready = %d %v", ready.Code, body)
	}

	notReady := do(handler(t, &stub{shaulaVersion: "0.0.1a0"}, nil), "GET", "/healthz", "")
	body = nil
	_ = json.Unmarshal(notReady.Body.Bytes(), &body)
	if notReady.Code != 200 || body["ready"] != false || body["fang_version"] != "unknown" {
		t.Fatalf("not ready = %d %v: liveness stays 200 but readiness must say false", notReady.Code, body)
	}
}

func TestResolveReturnsTheTargetAndForwardsTheInput(t *testing.T) {
	var got service.ResolveInput
	h := handler(t, &stub{resolve: func(_ context.Context, in service.ResolveInput) (store.Target, error) {
		got = in
		return store.Target{Catalog: "KIC", CatalogID: "8120608", RA: 298.65, Dec: 43.95, DisplayName: "Kepler-186", Designation: "KIC 8120608"}, nil
	}}, nil)

	rec := do(h, "POST", "/targets/resolve", `{"query":"Kepler-186f","mission":"Kepler"}`)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["designation"] != "KIC 8120608" || body["catalog_id"] != "8120608" || body["display_name"] != "Kepler-186" || body["ra_deg"] != 298.65 {
		t.Fatalf("body = %v", body)
	}
	if got.Query != "Kepler-186f" || got.Mission != "Kepler" || got.RA != nil || got.Dec != nil {
		t.Fatalf("service input = %+v", got)
	}
}

func TestResolveByPositionForwardsCoordinates(t *testing.T) {
	var got service.ResolveInput
	h := handler(t, &stub{resolve: func(_ context.Context, in service.ResolveInput) (store.Target, error) {
		got = in
		return store.Target{Catalog: "KIC", CatalogID: "8120608", RA: 298.65272, Dec: 43.95502, DisplayName: "Kepler-186", Designation: "KIC 8120608"}, nil
	}}, nil)

	rec := do(h, "POST", "/targets/resolve", `{"ra_deg":298.65,"dec_deg":43.95}`)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if got.Query != "" || got.RA == nil || *got.RA != 298.65 || got.Dec == nil || *got.Dec != 43.95 {
		t.Fatalf("service input = %+v", got)
	}
}

func TestRequestBodiesAreValidatedStrictly(t *testing.T) {
	h := handler(t, &stub{}, nil)
	cases := map[string]string{
		"not json":      `not json`,
		"empty body":    ``,
		"unknown field": `{"query":"x","surprise":1}`,
		"two documents": `{"query":"x"}{"query":"y"}`,
		"wrong type":    `{"query":5}`,
		"truncated":     `{"query":`,
	}
	for name, body := range cases {
		rec := do(h, "POST", "/targets/resolve", body)
		if rec.Code != 400 || decodeError(t, rec).Error.Code != "invalid_request" {
			t.Errorf("%s: status %d body %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestOversizedBodyIs413(t *testing.T) {
	huge := `{"query":"` + strings.Repeat("x", (1<<20)+10) + `"}`
	rec := do(handler(t, &stub{}, nil), "POST", "/targets/resolve", huge)
	if rec.Code != 413 || decodeError(t, rec).Error.Code != "too_large" {
		t.Fatalf("status %d body %.80s", rec.Code, rec.Body.String())
	}
}

func TestServiceErrorsMapToStatusCodes(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{"invalid", &service.Error{Kind: service.KindInvalid, Message: "bad input"}, 400, "invalid_request", "bad input"},
		{"not found", &service.Error{Kind: service.KindNotFound, Message: "no such star"}, 404, "not_found", "no such star"},
		{"unprocessable", &service.Error{Kind: service.KindUnprocessable, Message: "no light curves"}, 422, "unprocessable", "no light curves"},
		{"unavailable", &service.Error{Kind: service.KindUnavailable, Message: "no trained model is available yet"}, 503, "unavailable", "no trained model is available yet"},
		{"busy", &service.Error{Kind: service.KindBusy, Message: "the queue is full"}, 503, "busy", "the queue is full"},
		{"internal hides detail", &service.Error{Kind: service.KindInternal, Message: "sqlite: disk I/O error at /var/lib/x.db"}, 500, "internal", "internal error"},
		{"unexpected error hides detail", errors.New("secret connection string"), 500, "internal", "internal error"},
	}
	for _, tc := range cases {
		h := handler(t, &stub{resolve: func(context.Context, service.ResolveInput) (store.Target, error) { return store.Target{}, tc.err }}, nil)
		rec := do(h, "POST", "/targets/resolve", `{"query":"x"}`)
		body := decodeError(t, rec)
		if rec.Code != tc.status || body.Error.Code != tc.code || body.Error.Message != tc.message {
			t.Errorf("%s: %d %+v, want %d %s %q", tc.name, rec.Code, body.Error, tc.status, tc.code, tc.message)
		}
		if tc.name == "busy" && rec.Header().Get("Retry-After") != "30" {
			t.Errorf("busy must carry Retry-After: 30, got %q", rec.Header().Get("Retry-After"))
		}
	}
}

func TestRunsAlwaysReturnsAList(t *testing.T) {
	h := handler(t, &stub{runs: func(context.Context) ([]upstream.Run, error) { return nil, nil }}, nil)
	rec := do(h, "GET", "/ml/runs", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"runs":[]}` {
		t.Fatalf("status %d body %s: an empty registry must be [] not null", rec.Code, rec.Body.String())
	}

	h = handler(t, &stub{runs: func(context.Context) ([]upstream.Run, error) {
		return []upstream.Run{{RunID: "r1", CreatedAt: "2026-10-01T12:00:00Z", SchemaVersion: 1, SelectedModel: "lightgbm", Threshold: 0.42}}, nil
	}}, nil)
	rec = do(h, "GET", "/ml/runs", "")
	var body struct {
		Runs []map[string]any `json:"runs"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Runs) != 1 || body.Runs[0]["run_id"] != "r1" || body.Runs[0]["threshold"] != 0.42 || body.Runs[0]["schema_version"] != float64(1) {
		t.Fatalf("runs = %v", body.Runs)
	}
}

func TestPredictForwardsTheBodyAndReturnsTheResult(t *testing.T) {
	var got service.PredictInput
	h := handler(t, &stub{predict: func(_ context.Context, in service.PredictInput) (service.PredictResult, error) {
		got = in
		return service.PredictResult{ModelRunID: "r1", SchemaVersion: 1, Predictions: []map[string]any{{"prob_stack": 0.71}}}, nil
	}}, nil)

	rec := do(h, "POST", "/ml/predict", `{"features":[{"period_days":3.5,"vshape_metric":null}],"run_id":"r1","star_id":"KIC-1"}`)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if len(got.Rows) != 1 || got.Rows[0]["period_days"] != 3.5 || got.Rows[0]["vshape_metric"] != nil || got.RunID != "r1" || got.StarID != "KIC-1" {
		t.Fatalf("service input = %+v", got)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["model_run_id"] != "r1" || body["cached"] != false || body["predictions"] == nil {
		t.Fatalf("body = %v", body)
	}
}

func TestAPanicBecomesAJSON500AndTheServerSurvives(t *testing.T) {
	calls := 0
	h := handler(t, &stub{resolve: func(context.Context, service.ResolveInput) (store.Target, error) {
		calls++
		if calls == 1 {
			panic("kaboom")
		}
		return store.Target{Designation: "KIC 1"}, nil
	}}, nil)

	rec := do(h, "POST", "/targets/resolve", `{"query":"x"}`)
	if rec.Code != 500 || decodeError(t, rec).Error.Code != "internal" || strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "POST", "/targets/resolve", `{"query":"x"}`); rec.Code != 200 {
		t.Fatalf("the next request must still work, got %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	h := handler(t, &stub{runs: func(context.Context) ([]upstream.Run, error) { return nil, nil }}, func(c *config.Config) {
		c.AllowedOrigins = []string{"http://localhost:4200"}
	})

	allowed := httptest.NewRequest("GET", "/ml/runs", nil)
	allowed.Header.Set("Origin", "http://localhost:4200")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, allowed)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4200" || rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("allowed origin headers = %v", rec.Header())
	}

	foreign := httptest.NewRequest("GET", "/ml/runs", nil)
	foreign.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, foreign)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("a foreign origin must not be granted access")
	}

	preflight := httptest.NewRequest("OPTIONS", "/ml/predict", nil)
	preflight.Header.Set("Origin", "http://localhost:4200")
	preflight.Header.Set("Access-Control-Request-Method", "POST")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, preflight)
	if rec.Code != 204 || !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("preflight = %d %v", rec.Code, rec.Header())
	}

	if rec := do(h, "GET", "/ml/runs", ""); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("a request with no Origin must get no CORS headers")
	}
}
