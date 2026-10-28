package app_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"antares/api/internal/app"
	"antares/api/internal/config"
	"antares/api/internal/hub"
	"antares/api/internal/jobs"
	"antares/api/internal/store"
	"antares/api/internal/testdb"
	"antares/api/internal/upstream"
	"antares/api/internal/upstream/fake"
)

type rig struct {
	srv    *httptest.Server
	shaula *fake.Shaula
	fang   *fake.Fang
	app    *app.App
}

func newRig(t *testing.T, env map[string]string, setup func(*fake.Shaula, *fake.Fang)) *rig {
	t.Helper()
	return newRigWithDB(t, testdb.Open(t), env, setup)
}

func newRigWithDB(t *testing.T, db *sql.DB, env map[string]string, setup func(*fake.Shaula, *fake.Fang)) *rig {
	t.Helper()
	values := map[string]string{
		"ANTARES_PROMOTION_GRACE":   "80ms",
		"ANTARES_COALESCE_INTERVAL": "5ms",
	}
	for k, v := range env {
		values[k] = v
	}
	cfg, err := config.Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	shaula, fang := &fake.Shaula{}, &fake.Fang{}
	if setup != nil {
		setup(shaula, fang)
	}
	application, err := app.New(context.Background(), cfg, db, shaula, fang, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(application.Handler)
	t.Cleanup(func() {
		srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = application.Shutdown(ctx)
	})
	return &rig{srv: srv, shaula: shaula, fang: fang, app: application}
}

func (g *rig) post(t *testing.T, path, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(g.srv.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (g *rig) get(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(g.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (g *rig) dial(t *testing.T, path string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(g.srv.URL, "http")+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func drain(conn *websocket.Conn) ([]hub.Event, websocket.StatusCode) {
	var events []hub.Event
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var frame struct {
			Events []hub.Event `json:"events"`
		}
		err := wsjson.Read(ctx, conn, &frame)
		cancel()
		if err != nil {
			return events, websocket.CloseStatus(err)
		}
		events = append(events, frame.Events...)
	}
}

func stages(events []hub.Event) []string {
	var out []string
	for _, ev := range events {
		if ev.Type != hub.TypeState && ev.Type != hub.TypeProgress {
			continue
		}
		if len(out) == 0 || out[len(out)-1] != ev.Stage {
			out = append(out, ev.Stage)
		}
	}
	return out
}

func text(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	s, ok := m[key].(string)
	if !ok {
		t.Fatalf("%q is missing or not a string in %v", key, m)
	}
	return s
}

func eventually(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func stateOf(g *rig, t *testing.T, token string) string {
	_, view := g.get(t, "/subscription/"+token)
	s, _ := view["state"].(string)
	return s
}

func TestAnalyzeFlowEndToEnd(t *testing.T) {
	g := newRig(t, nil, nil)

	code, body := g.post(t, "/analyze", `{"query":"Kepler-186f","mission":"Kepler"}`)
	if code != http.StatusAccepted {
		t.Fatalf("status %d body %v", code, body)
	}
	token := text(t, body, "token")
	if body["subscription_path"] != "/subscription/"+token {
		t.Fatalf("body = %v", body)
	}

	events, closeStatus := drain(g.dial(t, "/subscription/"+token))
	if closeStatus != websocket.StatusNormalClosure {
		t.Fatalf("close status = %d, want 1000", closeStatus)
	}
	want := []string{"queued", "running", "downloading", "detrending", "period_search", "predicting"}
	if got := stages(events); !slices.Equal(got, want) {
		t.Fatalf("stages = %v, want %v", got, want)
	}
	last := events[len(events)-1]
	if last.Type != hub.TypeResult {
		t.Fatalf("the stream must end with the result, got %+v", last)
	}
	var result jobs.Result
	if err := json.Unmarshal(last.Payload, &result); err != nil {
		t.Fatal(err)
	}
	if result.Cached || len(result.Features) != 1 || len(result.Predictions) != 1 || result.ModelRunID != fake.DefaultRun().RunID {
		t.Fatalf("result = %+v", result)
	}
	if result.Features[0]["vshape_metric"] != nil {
		t.Fatalf("a missing value must stay null: %v", result.Features[0])
	}
	for i := 1; i < len(events); i++ {
		if events[i].Seq != events[i-1].Seq+1 {
			t.Fatalf("event sequence has a gap at %d: %d then %d", i, events[i-1].Seq, events[i].Seq)
		}
	}

	code, again := g.post(t, "/analyze", `{"query":"kepler 186","mission":"Kepler"}`)
	if code != http.StatusOK || again["status"] != "done" {
		t.Fatalf("the repeat must be answered from the caches: %d %v", code, again)
	}
	cached := again["result"].(map[string]any)
	if cached["cached"] != true || len(cached["predictions"].([]any)) != 1 {
		t.Fatalf("cached result = %v", cached)
	}
	if g.shaula.ExtractCount() != 1 || g.fang.PredictCount() != 1 {
		t.Fatalf("upstream calls: extract=%d predict=%d, want 1 and 1", g.shaula.ExtractCount(), g.fang.PredictCount())
	}

	_, view := g.get(t, "/subscription/"+token)
	if view["state"] != "done" || view["result"] == nil {
		t.Fatalf("status snapshot = %v", view)
	}
	var persisted []string
	for _, m := range view["milestones"].([]any) {
		persisted = append(persisted, m.(map[string]any)["stage"].(string))
	}
	if !slices.Equal(persisted, want) {
		t.Fatalf("persisted milestones = %v, want %v", persisted, want)
	}

	if code, runs := g.get(t, "/ml/runs"); code != 200 || len(runs["runs"].([]any)) != 1 {
		t.Fatalf("runs = %d %v", code, runs)
	}
	if _, health := g.get(t, "/healthz"); health["ready"] != true {
		t.Fatalf("health = %v", health)
	}
}

func TestAnExtractionFailureIsReportedAndNeverCached(t *testing.T) {
	g := newRig(t, nil, func(s *fake.Shaula, _ *fake.Fang) {
		s.ExtractFn = func(_ context.Context, _ upstream.ExtractRequest, onProgress func(upstream.Progress)) (upstream.FeatureSet, error) {
			onProgress(upstream.Progress{Stage: "downloading", Message: "Downloading"})
			return upstream.FeatureSet{}, &upstream.Error{Kind: upstream.KindPrecondition, Message: "No Kepler light curves found for 'Kepler-186'"}
		}
	})

	_, body := g.post(t, "/pipeline/extract", `{"query":"Kepler-186"}`)
	token := text(t, body, "token")
	events, closeStatus := drain(g.dial(t, "/subscription/"+token))

	if closeStatus != websocket.StatusNormalClosure {
		t.Fatalf("close status = %d", closeStatus)
	}
	last := events[len(events)-1]
	if last.Type != hub.TypeError || !strings.Contains(last.Message, "No Kepler light curves found") {
		t.Fatalf("last event = %+v", last)
	}
	for _, ev := range events {
		if ev.Type == hub.TypeResult {
			t.Fatal("a failed extraction must never emit a result")
		}
	}
	eventually(t, "the job to be recorded as failed", func() bool { return stateOf(g, t, token) == "failed" })

	code, _ := g.post(t, "/pipeline/extract", `{"query":"Kepler-186"}`)
	if code != http.StatusAccepted {
		t.Fatalf("a failed extraction must not populate the cache: got %d", code)
	}
	eventually(t, "the star to be extracted again", func() bool { return g.shaula.ExtractCount() == 2 })
}

func storedState(t *testing.T, db *sql.DB, token string) string {
	t.Helper()
	job, ok, err := jobs.Store{DB: db}.ByToken(context.Background(), token, time.Now())
	if err != nil || !ok {
		t.Fatalf("ByToken = %v, %v", ok, err)
	}
	return job.State
}

func TestAbsentQueuedSubscriberIsDroppedAndAConnectedOneRuns(t *testing.T) {
	release := make(chan struct{})
	db := testdb.Open(t)
	g := newRigWithDB(t, db, map[string]string{"ANTARES_MAX_ACTIVE": "1"}, func(s *fake.Shaula, _ *fake.Fang) {
		s.ResolveFn = func(_ context.Context, req upstream.ResolveRequest) (upstream.Resolved, error) {
			query := req.Query
			id := map[string]string{"A": "1", "B": "2", "C": "3"}[query]
			return upstream.Resolved{Catalog: "KIC", CatalogID: id, RA: 10, Dec: 10, DisplayName: query, Designation: "KIC " + id}, nil
		}
		s.ExtractFn = func(ctx context.Context, req upstream.ExtractRequest, _ func(upstream.Progress)) (upstream.FeatureSet, error) {
			select {
			case <-release:
			case <-ctx.Done():
				return upstream.FeatureSet{}, ctx.Err()
			}
			return fake.DefaultFeatureSet(req), nil
		}
	})

	_, a := g.post(t, "/pipeline/extract", `{"query":"A"}`)
	_, b := g.post(t, "/pipeline/extract", `{"query":"B"}`)
	_, c := g.post(t, "/pipeline/extract", `{"query":"C"}`)
	if a["status"] != "running" || b["position"] != float64(1) || c["position"] != float64(2) {
		t.Fatalf("a=%v b=%v c=%v", a, b, c)
	}
	tokenA, tokenB, tokenC := text(t, a, "token"), text(t, b, "token"), text(t, c, "token")

	g.dial(t, "/subscription/"+tokenC)
	time.Sleep(250 * time.Millisecond)
	close(release)

	eventually(t, "B to be dropped", func() bool { return storedState(t, db, tokenB) == "dropped" })
	_, view := g.get(t, "/subscription/"+tokenB)
	if !strings.Contains(view["error"].(string), "connected") {
		t.Fatalf("the user must be told why: %v", view["error"])
	}
	eventually(t, "A to finish", func() bool { return stateOf(g, t, tokenA) == "done" })
	eventually(t, "the connected C to run and finish", func() bool { return stateOf(g, t, tokenC) == "done" })
	if n := g.shaula.ExtractCount(); n != 2 {
		t.Fatalf("Shaula ran %d extractions, want 2: the dropped job must never start", n)
	}
}

func TestRestartFailsInterruptedJobs(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	target, err := store.Targets{DB: db}.Upsert(ctx, store.Target{
		Catalog: "KIC", CatalogID: "8120608", RA: 298.65, Dec: 43.95,
		DisplayName: "Kepler-186", Designation: "KIC 8120608",
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	js := jobs.Store{DB: db}
	join := func(key string) jobs.Joined {
		j, err := js.Join(ctx, jobs.NewJob{Kind: jobs.KindExtract, DedupeKey: key, TargetID: target.ID, Request: json.RawMessage(`{}`)}, time.Now(), time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	queued, running, finished := join("queued"), join("running"), join("finished")
	if _, err := js.MarkRunning(ctx, running.Job.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := js.Finish(ctx, finished.Job.ID, jobs.StateDone, json.RawMessage(`{"features":[]}`), "", time.Now()); err != nil {
		t.Fatal(err)
	}

	g := newRigWithDB(t, db, nil, nil)

	for name, token := range map[string]string{"queued": queued.Token, "running": running.Token} {
		code, view := g.get(t, "/subscription/"+token)
		if code != http.StatusOK || view["state"] != "failed" || !strings.Contains(view["error"].(string), "restart") {
			t.Errorf("%s job after restart: %d %v: an old token must report failed, never 404", name, code, view)
		}
	}
	if _, view := g.get(t, "/subscription/"+finished.Token); view["state"] != "done" {
		t.Errorf("a finished job must be left alone: %v", view)
	}

	events, closeStatus := drain(g.dial(t, "/subscription/"+queued.Token))
	if len(events) == 0 || events[len(events)-1].Type != hub.TypeError || closeStatus != websocket.StatusNormalClosure {
		t.Fatalf("the WebSocket must replay the failure and close: %+v status %d", events, closeStatus)
	}
}

func TestReadinessReflectsTheUpstreams(t *testing.T) {
	g := newRig(t, nil, func(_ *fake.Shaula, f *fake.Fang) { f.VersionErr = io.ErrUnexpectedEOF })
	code, health := g.get(t, "/healthz")
	if code != 200 || health["ready"] != false || health["fang_version"] != "unknown" || health["shaula_version"] != "0.0.1a0" {
		t.Fatalf("health = %d %v", code, health)
	}
}

func TestTickRemovesExpiredSubscriptionsAndOrphanedJobs(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	cfg, _ := config.Load(func(key string) string {
		if key == "ANTARES_TOKEN_RETENTION" {
			return "1ms"
		}
		return ""
	})
	application, err := app.New(ctx, cfg, db, &fake.Shaula{}, &fake.Fang{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(application.Handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/pipeline/extract", "application/json", strings.NewReader(`{"query":"Kepler-186"}`))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	token := text(t, body, "token")

	eventually(t, "the job to finish", func() bool {
		var state string
		_ = db.QueryRow(`SELECT state FROM jobs LIMIT 1`).Scan(&state)
		return state == jobs.StateDone
	})
	time.Sleep(20 * time.Millisecond)
	application.Tick(ctx)

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("jobs left after Tick = %d (err %v)", remaining, err)
	}
	r, err := http.Get(srv.URL + "/subscription/" + token)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("an expired token must be 404, got %d", r.StatusCode)
	}

	shutdown, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = application.Shutdown(shutdown)
}

func TestShutdownLeavesQueuedJobsQueuedForRestartRecovery(t *testing.T) {
	release := make(chan struct{})
	db := testdb.Open(t)
	g := newRigWithDB(t, db, map[string]string{"ANTARES_MAX_ACTIVE": "1"}, func(s *fake.Shaula, _ *fake.Fang) {
		s.ResolveFn = func(_ context.Context, req upstream.ResolveRequest) (upstream.Resolved, error) {
			id := map[string]string{"A": "1", "B": "2"}[req.Query]
			return upstream.Resolved{Catalog: "KIC", CatalogID: id, RA: 10, Dec: 10, DisplayName: req.Query, Designation: "KIC " + id}, nil
		}
		s.ExtractFn = func(ctx context.Context, req upstream.ExtractRequest, _ func(upstream.Progress)) (upstream.FeatureSet, error) {
			select {
			case <-release:
			case <-ctx.Done():
				return upstream.FeatureSet{}, ctx.Err()
			}
			return fake.DefaultFeatureSet(req), nil
		}
	})
	_, a := g.post(t, "/pipeline/extract", `{"query":"A"}`)
	_, b := g.post(t, "/pipeline/extract", `{"query":"B"}`)
	tokenA, tokenB := text(t, a, "token"), text(t, b, "token")
	g.dial(t, "/subscription/"+tokenB)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- g.app.Shutdown(ctx)
	}()
	time.Sleep(50 * time.Millisecond)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if s := stateOf(g, t, tokenA); s != "done" {
		t.Fatalf("running job state = %q, want done", s)
	}
	if s := stateOf(g, t, tokenB); s != "queued" {
		t.Fatalf("queued job state = %q, want queued (neither started nor dropped)", s)
	}
	if n := g.shaula.ExtractCount(); n != 1 {
		t.Fatalf("extractions = %d, want 1", n)
	}
}

type hangingShaula struct{ fake.Shaula }

func (*hangingShaula) Version(ctx context.Context) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func TestVersionRefreshIsBounded(t *testing.T) {
	defer app.SetVersionTimeout(50 * time.Millisecond)()
	cfg, _ := config.Load(func(string) string { return "" })
	start := time.Now()
	application, err := app.New(context.Background(), cfg, testdb.Open(t), &hangingShaula{}, &fake.Fang{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	application.Tick(context.Background())
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("New and Tick took %v with a hanging upstream", elapsed)
	}
}

func TestPollingKeepsAQueuedJobPresent(t *testing.T) {
	release := make(chan struct{})
	g := newRig(t, map[string]string{"ANTARES_MAX_ACTIVE": "1"}, func(s *fake.Shaula, _ *fake.Fang) {
		s.ResolveFn = func(_ context.Context, req upstream.ResolveRequest) (upstream.Resolved, error) {
			id := map[string]string{"A": "1", "B": "2"}[req.Query]
			return upstream.Resolved{Catalog: "KIC", CatalogID: id, RA: 10, Dec: 10, DisplayName: req.Query, Designation: "KIC " + id}, nil
		}
		s.ExtractFn = func(ctx context.Context, req upstream.ExtractRequest, _ func(upstream.Progress)) (upstream.FeatureSet, error) {
			if req.Target == "KIC 1" {
				select {
				case <-release:
				case <-ctx.Done():
					return upstream.FeatureSet{}, ctx.Err()
				}
			}
			return fake.DefaultFeatureSet(req), nil
		}
	})

	_, a := g.post(t, "/pipeline/extract", `{"query":"A"}`)
	_, b := g.post(t, "/pipeline/extract", `{"query":"B"}`)
	tokenA, tokenB := text(t, a, "token"), text(t, b, "token")

	for range 12 {
		time.Sleep(25 * time.Millisecond)
		if s := stateOf(g, t, tokenB); s != "queued" {
			t.Fatalf("polled job state = %q, want queued", s)
		}
	}
	g.app.Tick(context.Background())
	if s := stateOf(g, t, tokenB); s != "queued" {
		t.Fatalf("a job whose client polls must survive the sweep, got %q", s)
	}
	close(release)
	eventually(t, "A to finish", func() bool { return stateOf(g, t, tokenA) == "done" })
	eventually(t, "the polled B to run and finish", func() bool { return stateOf(g, t, tokenB) == "done" })
}

type contextFang struct{ fake.Fang }

func (f *contextFang) Version(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return f.Fang.Version(ctx)
}

func TestAHangingShaulaStillLetsFangsVersionBeRead(t *testing.T) {
	defer app.SetVersionTimeout(50 * time.Millisecond)()
	cfg, _ := config.Load(func(string) string { return "" })
	application, err := app.New(context.Background(), cfg, testdb.Open(t), &hangingShaula{}, &contextFang{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(application.Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var health map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&health)
	if health["fang_version"] != "0.0.1a0" || health["shaula_version"] != "unknown" {
		t.Fatalf("health = %v: each upstream needs its own version timeout", health)
	}
}

func TestQueuedAndRunningArriveWhileTheDownloadIsStillGoing(t *testing.T) {
	release := make(chan struct{})
	g := newRig(t, nil, func(s *fake.Shaula, _ *fake.Fang) {
		s.ExtractFn = func(ctx context.Context, _ upstream.ExtractRequest, _ func(upstream.Progress)) (upstream.FeatureSet, error) {
			select {
			case <-release:
				return fake.DefaultFeatureSet(upstream.ExtractRequest{}), nil
			case <-ctx.Done():
				return upstream.FeatureSet{}, ctx.Err()
			}
		}
	})
	defer close(release)

	code, body := g.post(t, "/pipeline/extract", `{"ra_deg":290.1,"dec_deg":44.5,"mission":"Kepler"}`)
	if code != http.StatusAccepted {
		t.Fatalf("status %d body %v", code, body)
	}
	conn := g.dial(t, "/subscription/"+text(t, body, "token"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var frame struct {
		Events []hub.Event `json:"events"`
	}
	if err := wsjson.Read(ctx, conn, &frame); err != nil {
		t.Fatalf("no frame while the extraction runs: %v", err)
	}
	if got := stages(frame.Events); !slices.Equal(got, []string{"queued", "running"}) {
		t.Fatalf("first frame stages = %v", got)
	}
}
