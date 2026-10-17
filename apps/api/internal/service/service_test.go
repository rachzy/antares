package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"antares/api/internal/cache"
	"antares/api/internal/config"
	"antares/api/internal/hub"
	"antares/api/internal/jobs"
	"antares/api/internal/service"
	"antares/api/internal/store"
	"antares/api/internal/testdb"
	"antares/api/internal/upstream"
	"antares/api/internal/upstream/fake"
)

var defaultParams = cache.ExtractParams{Mission: "Kepler", SigmaClip: 5}

type env struct {
	svc     *service.Service
	db      *sql.DB
	shaula  *fake.Shaula
	fang    *fake.Fang
	cfg     config.Config
	started chan string
	hold    chan struct{}
}

func newEnv(t *testing.T, setup func(e *env)) *env {
	t.Helper()
	cfg, err := config.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	e := &env{
		db:      testdb.Open(t),
		shaula:  &fake.Shaula{},
		fang:    &fake.Fang{},
		cfg:     cfg,
		started: make(chan string, 64),
		hold:    make(chan struct{}),
	}
	if setup != nil {
		setup(e)
	}

	h := hub.New(e.cfg.RingSize, e.cfg.ConnBuffer, time.Now)
	sched := jobs.NewScheduler(jobs.SchedulerOptions{
		MaxActive: e.cfg.MaxActive, MaxQueue: e.cfg.MaxQueue, Grace: e.cfg.PromotionGrace,
		Present: func(id string, grace time.Duration) bool {
			room, ok := h.Lookup(id)
			return ok && room.Present(grace)
		},
		Run:      func(id string) { e.started <- id; <-e.hold },
		Dropped:  func(string) {},
		Position: func(string, int) {},
	})
	e.svc = &service.Service{
		Targets:     store.Targets{DB: e.db},
		Features:    cache.FeatureCache{DB: e.db},
		Predictions: cache.PredictionCache{DB: e.db},
		Jobs:        jobs.Store{DB: e.db},
		Hub:         h,
		Sched:       sched,
		Shaula:      e.shaula,
		Fang:        e.fang,
		Clock:       time.Now,
		Cfg:         e.cfg,
	}
	_ = e.svc.RefreshVersions(context.Background())
	t.Cleanup(func() {
		close(e.hold)
		sched.Wait()
	})
	return e
}

func kindOf(t *testing.T, err error) service.Kind {
	t.Helper()
	var se *service.Error
	if !errors.As(err, &se) {
		t.Fatalf("expected a *service.Error, got %T: %v", err, err)
	}
	return se.Kind
}

func (e *env) seedFeatures(t *testing.T, query string, params cache.ExtractParams) store.Target {
	t.Helper()
	target, err := e.svc.Resolve(context.Background(), service.ResolveInput{Query: query, Mission: params.Mission})
	if err != nil {
		t.Fatal(err)
	}
	err = e.svc.Features.Put(context.Background(), cache.FeatureEntry{
		TargetID: target.ID, ParamsHash: params.Hash(), ShaulaVersion: "0.0.1a0",
		Rows: []map[string]any{{"period_days": 129.9, "vshape_metric": nil}}, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func floatPtr(v float64) *float64 { return &v }

func twoStars(e *env) {
	e.shaula.ResolveFn = func(_ context.Context, req upstream.ResolveRequest) (upstream.Resolved, error) {
		query := req.Query
		id := map[string]string{"A": "1", "B": "2"}[query]
		return upstream.Resolved{Catalog: "KIC", CatalogID: id, RA: 10, Dec: 10, DisplayName: query, Designation: "KIC " + id}, nil
	}
}

func TestResolveUsesTheAliasCacheAfterTheFirstCall(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()

	first, err := e.svc.Resolve(ctx, service.ResolveInput{Query: "Kepler-186f"})
	if err != nil || first.Designation != "KIC 8120608" {
		t.Fatalf("Resolve = %+v, %v", first, err)
	}
	for _, query := range []string{"Kepler-186f", "kepler 186f", "KIC 8120608", "Kepler-186"} {
		got, err := e.svc.Resolve(ctx, service.ResolveInput{Query: query})
		if err != nil || got.ID != first.ID {
			t.Fatalf("%q: %+v, %v", query, got, err)
		}
	}
	if n := e.shaula.ResolveCount(); n != 1 {
		t.Fatalf("Shaula was asked %d times, want 1: every later name must hit the alias cache", n)
	}
}

func TestResolveIsPerMission(t *testing.T) {
	e := newEnv(t, func(e *env) {
		e.shaula.ResolveFn = func(_ context.Context, req upstream.ResolveRequest) (upstream.Resolved, error) {
			if req.Mission == "TESS" {
				return upstream.Resolved{Catalog: "TIC", CatalogID: "251848941", RA: 7.3, Dec: -30.4, DisplayName: "Kepler-186", Designation: "TIC 251848941"}, nil
			}
			return fake.DefaultResolved(), nil
		}
	})
	kepler, err := e.svc.Resolve(context.Background(), service.ResolveInput{Query: "Kepler-186", Mission: "kepler"})
	if err != nil {
		t.Fatal(err)
	}
	tess, err := e.svc.Resolve(context.Background(), service.ResolveInput{Query: "Kepler-186", Mission: " TESS "})
	if err != nil {
		t.Fatal(err)
	}
	if kepler.Catalog != "KIC" || tess.Catalog != "TIC" || e.shaula.ResolveCount() != 2 {
		t.Fatalf("kepler=%+v tess=%+v calls=%d: one name must resolve separately per mission", kepler, tess, e.shaula.ResolveCount())
	}
}

func TestResolveRejectsBadInput(t *testing.T) {
	e := newEnv(t, nil)
	cases := map[string]service.ResolveInput{
		"no query and no coordinates":  {},
		"unsupported mission":          {Query: "Kepler-186", Mission: "Hubble"},
		"query too long":               {Query: strings.Repeat("x", service.MaxQueryLength+1)},
		"right ascension out of range": {RA: floatPtr(400), Dec: floatPtr(0)},
		"declination out of range":     {RA: floatPtr(10), Dec: floatPtr(91)},
		"nan coordinate":               {RA: floatPtr(math.NaN()), Dec: floatPtr(0)},
		"query and coordinates":        {Query: "Kepler-186", RA: floatPtr(10), Dec: floatPtr(10)},
		"only one coordinate":          {RA: floatPtr(10)},
	}
	for name, in := range cases {
		_, err := e.svc.Resolve(context.Background(), in)
		if err == nil || kindOf(t, err) != service.KindInvalid {
			t.Errorf("%s: err = %v, want KindInvalid", name, err)
		}
	}
	if e.shaula.ResolveCount() != 0 {
		t.Fatal("invalid input must never reach Shaula")
	}
	if _, err := e.svc.Resolve(context.Background(), service.ResolveInput{Query: strings.Repeat("x", service.MaxQueryLength)}); err != nil {
		t.Fatalf("a query of exactly the limit must be accepted: %v", err)
	}
}

func TestResolveByCoordinatesAsksShaula(t *testing.T) {
	var seen []upstream.ResolveRequest
	e := newEnv(t, func(e *env) {
		e.shaula.ResolveFn = func(_ context.Context, req upstream.ResolveRequest) (upstream.Resolved, error) {
			seen = append(seen, req)
			return fake.DefaultResolved(), nil
		}
	})
	target, err := e.svc.Resolve(context.Background(), service.ResolveInput{RA: floatPtr(298.65), Dec: floatPtr(43.95)})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || e.shaula.ResolveCount() != 1 {
		t.Fatalf("Shaula was asked %d times, want 1", len(seen))
	}
	got := seen[0]
	if got.Query != "" || got.RA == nil || got.Dec == nil || *got.RA != 298.65 || *got.Dec != 43.95 || got.Mission != "Kepler" {
		t.Fatalf("Shaula received %+v", got)
	}
	if target.Designation != "KIC 8120608" {
		t.Fatalf("target = %+v", target)
	}

	for _, alias := range []string{"298.65", "43.95", "298.65 43.95", "298.65,43.95"} {
		if _, ok, err := e.svc.Targets.ByAlias(context.Background(), alias, "KIC"); err != nil || ok {
			t.Errorf("%q must not have become an alias: ok=%v err=%v", alias, ok, err)
		}
	}
	var aliases int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM target_aliases`).Scan(&aliases); err != nil {
		t.Fatal(err)
	}
	if aliases != 2 {
		t.Fatalf("a position resolve stores only the display name and designation as aliases, got %d rows", aliases)
	}
}

func TestZeroCoordinatesAreAValidPosition(t *testing.T) {
	var seen upstream.ResolveRequest
	e := newEnv(t, func(e *env) {
		e.shaula.ResolveFn = func(_ context.Context, req upstream.ResolveRequest) (upstream.Resolved, error) {
			seen = req
			return fake.DefaultResolved(), nil
		}
	})
	if _, err := e.svc.Resolve(context.Background(), service.ResolveInput{RA: floatPtr(0), Dec: floatPtr(0)}); err != nil {
		t.Fatal(err)
	}
	if seen.RA == nil || seen.Dec == nil || *seen.RA != 0 || *seen.Dec != 0 {
		t.Fatalf("Shaula must receive non-nil pointers to 0, got %+v", seen)
	}
}

func TestResolvePositionNotFoundIsNotFound(t *testing.T) {
	e := newEnv(t, func(e *env) {
		e.shaula.ResolveFn = func(context.Context, upstream.ResolveRequest) (upstream.Resolved, error) {
			return upstream.Resolved{}, &upstream.Error{Kind: upstream.KindNotFound, Message: "no star"}
		}
	})
	_, err := e.svc.Resolve(context.Background(), service.ResolveInput{RA: floatPtr(10), Dec: floatPtr(10)})
	if err == nil || kindOf(t, err) != service.KindNotFound {
		t.Fatalf("err = %v, want KindNotFound", err)
	}
}

func TestPositionAndNameResolveToTheSameTarget(t *testing.T) {
	e := newEnv(t, nil)
	byName, err := e.svc.Resolve(context.Background(), service.ResolveInput{Query: "Kepler-186"})
	if err != nil {
		t.Fatal(err)
	}
	byPosition, err := e.svc.Resolve(context.Background(), service.ResolveInput{RA: floatPtr(298.65272), Dec: floatPtr(43.95502)})
	if err != nil {
		t.Fatal(err)
	}
	if byPosition.ID != byName.ID {
		t.Fatalf("by name %+v, by position %+v: the same star must be one target", byName, byPosition)
	}
}

func TestResolveMapsUpstreamErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want service.Kind
	}{
		"not found":   {&upstream.Error{Kind: upstream.KindNotFound, Message: "no such star"}, service.KindNotFound},
		"invalid":     {&upstream.Error{Kind: upstream.KindInvalid, Message: "bad"}, service.KindInvalid},
		"failed":      {&upstream.Error{Kind: upstream.KindPrecondition, Message: "no light curves"}, service.KindUnprocessable},
		"unavailable": {&upstream.Error{Kind: upstream.KindUnavailable, Message: "down"}, service.KindUnavailable},
		"internal":    {&upstream.Error{Kind: upstream.KindInternal, Message: "boom"}, service.KindInternal},
		"unexpected":  {errors.New("connection reset"), service.KindUnavailable},
	} {
		e := newEnv(t, func(e *env) {
			e.shaula.ResolveFn = func(context.Context, upstream.ResolveRequest) (upstream.Resolved, error) {
				return upstream.Resolved{}, tc.err
			}
		})
		_, err := e.svc.Resolve(context.Background(), service.ResolveInput{Query: "x"})
		if err == nil || kindOf(t, err) != tc.want {
			t.Errorf("%s: err = %v, want kind %v", name, err, tc.want)
		}
	}
}

func TestExtractQueuesAJobOnACacheMiss(t *testing.T) {
	e := newEnv(t, nil)
	out, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != nil || !jobs.ValidToken(out.Token) || out.Position != 0 {
		t.Fatalf("outcome = %+v", out)
	}
	job, ok, err := e.svc.Jobs.ByToken(context.Background(), out.Token, time.Now())
	if err != nil || !ok || job.Kind != jobs.KindExtract {
		t.Fatalf("job = %+v ok=%v err=%v", job, ok, err)
	}
	var payload jobs.Payload
	if err := json.Unmarshal(job.Request, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Target.Designation != "KIC 8120608" || payload.Params != defaultParams {
		t.Fatalf("payload = %+v", payload)
	}
	milestones, _ := e.svc.Jobs.Milestones(context.Background(), job.ID)
	if len(milestones) != 1 || milestones[0].Stage != "queued" {
		t.Fatalf("milestones = %+v", milestones)
	}
	select {
	case id := <-e.started:
		if id != job.ID {
			t.Fatalf("scheduler started %s, want %s", id, job.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the job was never handed to the scheduler")
	}
}

func TestExtractAnswersFromTheFeatureCacheWithoutAJob(t *testing.T) {
	e := newEnv(t, nil)
	e.seedFeatures(t, "Kepler-186", defaultParams)

	out, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "kepler 186"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || !out.Result.Cached || out.Token != "" {
		t.Fatalf("outcome = %+v", out)
	}
	if len(out.Result.Features) != 1 || out.Result.ShaulaVersion != "0.0.1a0" || out.Result.Target.Designation != "KIC 8120608" {
		t.Fatalf("result = %+v", out.Result)
	}
	var jobsCreated int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&jobsCreated); err != nil || jobsCreated != 0 {
		t.Fatalf("a cache hit must not create a job: %d (err %v)", jobsCreated, err)
	}
	if e.shaula.ExtractCount() != 0 {
		t.Fatal("a cache hit must not reach Shaula")
	}
}

func TestDifferentParametersAreDifferentCacheEntries(t *testing.T) {
	e := newEnv(t, nil)
	e.seedFeatures(t, "Kepler-186", defaultParams)

	out, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186", SigmaClip: floatPtr(3)})
	if err != nil || out.Result != nil || out.Token == "" {
		t.Fatalf("a different sigma_clip must miss the cache and queue: %+v, %v", out, err)
	}
}

func TestForceRefreshBypassesTheCache(t *testing.T) {
	e := newEnv(t, nil)
	e.seedFeatures(t, "Kepler-186", defaultParams)

	out, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186", ForceRefresh: true})
	if err != nil || out.Result != nil || out.Token == "" {
		t.Fatalf("outcome = %+v, %v", out, err)
	}
}

func TestIdenticalSubmissionsShareOneJob(t *testing.T) {
	e := newEnv(t, nil)
	const callers = 6

	var wg sync.WaitGroup
	outcomes := make([]service.Outcome, callers)
	errs := make([]error, callers)
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			outcomes[i], errs[i] = e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186"})
		}()
	}
	close(start)
	wg.Wait()

	jobIDs := map[string]bool{}
	tokens := map[string]bool{}
	for i, out := range outcomes {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		job, ok, err := e.svc.Jobs.ByToken(context.Background(), out.Token, time.Now())
		if err != nil || !ok {
			t.Fatalf("caller %d token: ok=%v err=%v", i, ok, err)
		}
		jobIDs[job.ID] = true
		tokens[out.Token] = true
	}
	if len(jobIDs) != 1 {
		t.Fatalf("%d jobs were created, want 1: simultaneous requests for one star must share a job", len(jobIDs))
	}
	if len(tokens) != callers {
		t.Fatalf("%d distinct tokens, want %d: each caller needs its own", len(tokens), callers)
	}

	<-e.started
	select {
	case id := <-e.started:
		t.Fatalf("a second job %s was started", id)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestQueueFullIsBusyAndLeavesNoQueuedJob(t *testing.T) {
	e := newEnv(t, func(e *env) {
		twoStars(e)
		e.cfg.MaxActive, e.cfg.MaxQueue = 1, 0
	})
	if _, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "A"}); err != nil {
		t.Fatal(err)
	}
	<-e.started

	_, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "B"})
	if err == nil || kindOf(t, err) != service.KindBusy {
		t.Fatalf("err = %v, want KindBusy", err)
	}
	var active, dropped int
	_ = e.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE state IN ('queued', 'running')`).Scan(&active)
	_ = e.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE state = 'dropped'`).Scan(&dropped)
	if active != 1 || dropped != 1 {
		t.Fatalf("active=%d dropped=%d: the rejected job must not stay queued", active, dropped)
	}
}

func TestExtractWithAnUnknownShaulaVersionIsUnavailable(t *testing.T) {
	e := newEnv(t, func(e *env) { e.shaula.VersionErr = errors.New("connection refused") })
	_, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186"})
	if err == nil || kindOf(t, err) != service.KindUnavailable || !strings.Contains(err.Error(), "pipeline") {
		t.Fatalf("err = %v, want KindUnavailable naming the pipeline", err)
	}
	if e.shaula.ResolveCount() != 0 {
		t.Fatal("nothing may be attempted before the cache key can be built")
	}
}

func TestRejectedParametersNeverReachShaula(t *testing.T) {
	e := newEnv(t, nil)
	for name, in := range map[string]service.ExtractInput{
		"sigma zero":     {Query: "x", SigmaClip: floatPtr(0)},
		"sigma negative": {Query: "x", SigmaClip: floatPtr(-1)},
		"sigma nan":      {Query: "x", SigmaClip: floatPtr(math.NaN())},
		"sigma huge":     {Query: "x", SigmaClip: floatPtr(51)},
		"author long":    {Query: "x", Author: strings.Repeat("a", 65)},
		"exptime long":   {Query: "x", Exptime: strings.Repeat("e", 33)},
		"bad mission":    {Query: "x", Mission: "Hubble"},
	} {
		if _, err := e.svc.Extract(context.Background(), in); err == nil || kindOf(t, err) != service.KindInvalid {
			t.Errorf("%s: err = %v, want KindInvalid", name, err)
		}
	}
	if e.shaula.ResolveCount() != 0 {
		t.Fatal("validation must happen before any upstream call")
	}
}

func TestAnalyzeWithNoModelIsUnavailable(t *testing.T) {
	e := newEnv(t, func(e *env) { e.fang.NoRuns = true })
	_, err := e.svc.Analyze(context.Background(), service.ExtractInput{Query: "Kepler-186"})
	if err == nil || kindOf(t, err) != service.KindUnavailable || !strings.Contains(err.Error(), "no trained model") {
		t.Fatalf("err = %v, want KindUnavailable saying no trained model", err)
	}
	if _, err := e.svc.Predict(context.Background(), service.PredictInput{Rows: []map[string]any{{"a": 1.0}}}); kindOf(t, err) != service.KindUnavailable {
		t.Fatalf("Predict with no model: %v", err)
	}
}

func TestAnalyzeWithAnUnknownRunIsNotFound(t *testing.T) {
	e := newEnv(t, nil)
	_, err := e.svc.Analyze(context.Background(), service.ExtractInput{Query: "Kepler-186", RunID: "no-such-run"})
	if err == nil || kindOf(t, err) != service.KindNotFound {
		t.Fatalf("err = %v, want KindNotFound", err)
	}
}

func TestAnalyzeScoresCachedFeaturesAndCachesThePrediction(t *testing.T) {
	e := newEnv(t, nil)
	e.seedFeatures(t, "Kepler-186", defaultParams)
	runID := fake.DefaultRun().RunID

	first, err := e.svc.Analyze(context.Background(), service.ExtractInput{Query: "Kepler-186", StarID: "KIC-8120608"})
	if err != nil || first.Result == nil {
		t.Fatalf("Analyze = %+v, %v", first, err)
	}
	if !first.Result.Cached || first.Result.ModelRunID != runID || len(first.Result.Predictions) != 1 {
		t.Fatalf("result = %+v", first.Result)
	}
	reqs := e.fang.PredictRequests()
	if len(reqs) != 1 || reqs[0].RunID != runID {
		t.Fatalf("the service must pass a concrete run id to Fang, got %+v", reqs)
	}

	second, err := e.svc.Analyze(context.Background(), service.ExtractInput{Query: "Kepler-186", StarID: "KIC-8120608"})
	if err != nil || second.Result == nil || e.fang.PredictCount() != 1 {
		t.Fatalf("the second call must be served from the prediction cache: calls=%d err=%v", e.fang.PredictCount(), err)
	}
}

func TestAnalyzeQueuesWhenFeaturesAreMissingAndRecordsTheRun(t *testing.T) {
	e := newEnv(t, nil)
	out, err := e.svc.Analyze(context.Background(), service.ExtractInput{Query: "Kepler-186", StarID: "mine"})
	if err != nil || out.Result != nil || out.Token == "" {
		t.Fatalf("outcome = %+v, %v", out, err)
	}
	job, _, _ := e.svc.Jobs.ByToken(context.Background(), out.Token, time.Now())
	var payload jobs.Payload
	_ = json.Unmarshal(job.Request, &payload)
	if job.Kind != jobs.KindAnalyze || payload.RunID != fake.DefaultRun().RunID || payload.StarID != "mine" {
		t.Fatalf("job = %+v payload = %+v", job, payload)
	}
}

func TestPredictCachesAndRestoresTheCallersStarID(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()

	first, err := e.svc.Predict(ctx, service.PredictInput{Rows: []map[string]any{{"a": math.NaN(), "b": 2.0}}, StarID: "alpha"})
	if err != nil || first.Cached || e.fang.PredictCount() != 1 {
		t.Fatalf("first = %+v, %v, calls=%d", first, err, e.fang.PredictCount())
	}
	if first.Predictions[0]["star_id"] != "alpha" {
		t.Fatalf("predictions = %v", first.Predictions)
	}

	second, err := e.svc.Predict(ctx, service.PredictInput{Rows: []map[string]any{{"b": 2.0, "a": nil}}, StarID: "beta"})
	if err != nil || !second.Cached || e.fang.PredictCount() != 1 {
		t.Fatalf("NaN and null must share a cache entry: second = %+v, %v, calls=%d", second, err, e.fang.PredictCount())
	}
	if second.Predictions[0]["star_id"] != "beta" {
		t.Fatalf("a cached row must carry the caller's own star id, got %v", second.Predictions[0]["star_id"])
	}

	third, _ := e.svc.Predict(ctx, service.PredictInput{Rows: []map[string]any{{"a": nil, "b": 2.0}}})
	if third.Predictions[0]["star_id"] != "unknown" {
		t.Fatalf("with no star id the default is %q, got %v", "unknown", third.Predictions[0]["star_id"])
	}
}

func TestPredictValidatesBeforeCallingFang(t *testing.T) {
	e := newEnv(t, nil)
	tooMany := make([]map[string]any, service.MaxPredictRows+1)
	for i := range tooMany {
		tooMany[i] = map[string]any{"a": 1.0}
	}
	cases := map[string]struct {
		in   service.PredictInput
		want service.Kind
	}{
		"no rows":      {service.PredictInput{}, service.KindInvalid},
		"too many":     {service.PredictInput{Rows: tooMany}, service.KindInvalid},
		"empty row":    {service.PredictInput{Rows: []map[string]any{{}}}, service.KindInvalid},
		"long star id": {service.PredictInput{Rows: []map[string]any{{"a": 1.0}}, StarID: strings.Repeat("s", 201)}, service.KindInvalid},
		"unknown run":  {service.PredictInput{Rows: []map[string]any{{"a": 1.0}}, RunID: "nope"}, service.KindNotFound},
	}
	for name, tc := range cases {
		if _, err := e.svc.Predict(context.Background(), tc.in); err == nil || kindOf(t, err) != tc.want {
			t.Errorf("%s: err = %v, want kind %v", name, err, tc.want)
		}
	}
	if e.fang.PredictCount() != 0 {
		t.Fatal("invalid requests must never reach Fang")
	}
}

func TestRunsAreReturnedNewestFirst(t *testing.T) {
	e := newEnv(t, func(e *env) {
		e.fang.Runs = []upstream.Run{
			{RunID: "20260101T000000Z-aaaaaaaa"}, {RunID: "20261231T000000Z-bbbbbbbb"}, {RunID: "20260615T000000Z-cccccccc"},
		}
	})
	runs, err := e.svc.Runs(context.Background())
	if err != nil || len(runs) != 3 || runs[0].RunID != "20261231T000000Z-bbbbbbbb" || runs[2].RunID != "20260101T000000Z-aaaaaaaa" {
		t.Fatalf("runs = %+v, %v", runs, err)
	}
	out, err := e.svc.Predict(context.Background(), service.PredictInput{Rows: []map[string]any{{"a": 1.0}}})
	if err != nil || out.ModelRunID != "20261231T000000Z-bbbbbbbb" {
		t.Fatalf("no run id must select the newest: %+v, %v", out, err)
	}
}

func TestRefreshVersionsSupersedesFeaturesFromOtherVersions(t *testing.T) {
	e := newEnv(t, nil)
	target := e.seedFeatures(t, "Kepler-186", defaultParams)
	if shaula, fang := e.svc.Versions(); shaula != "0.0.1a0" || fang != "0.0.1a0" {
		t.Fatalf("versions = %q, %q", shaula, fang)
	}

	e.shaula.LibraryVersion = "0.0.2"
	if err := e.svc.RefreshVersions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := e.svc.Features.Get(context.Background(), target.ID, defaultParams.Hash(), "0.0.1a0", time.Now()); ok {
		t.Fatal("features from the previous version must be superseded")
	}
	out, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186"})
	if err != nil || out.Result != nil {
		t.Fatalf("after a version change the star must be extracted again: %+v, %v", out, err)
	}
}

func TestRefreshVersionsReportsAFailureButKeepsTheOtherService(t *testing.T) {
	e := newEnv(t, func(e *env) { e.fang.VersionErr = errors.New("model service down") })
	shaula, fang := e.svc.Versions()
	if shaula != "0.0.1a0" || fang != "" {
		t.Fatalf("versions = %q, %q", shaula, fang)
	}
	if err := e.svc.RefreshVersions(context.Background()); err == nil {
		t.Fatal("a failing service must be reported")
	}
}

func TestWatchAndStatusForALiveJob(t *testing.T) {
	e := newEnv(t, nil)
	out, err := e.svc.Extract(context.Background(), service.ExtractInput{Query: "Kepler-186"})
	if err != nil {
		t.Fatal(err)
	}

	watch, err := e.svc.Watch(context.Background(), out.Token, 0)
	if err != nil || watch.Sub == nil {
		t.Fatalf("Watch = %+v, %v", watch, err)
	}
	defer watch.Sub.Cancel()
	if len(watch.Sub.Replay) != 1 || watch.Sub.Replay[0].Stage != "queued" {
		t.Fatalf("replay = %+v", watch.Sub.Replay)
	}

	view, err := e.svc.Status(context.Background(), out.Token)
	if err != nil {
		t.Fatal(err)
	}
	if view.Kind != jobs.KindExtract || view.State != jobs.StateQueued || len(view.Milestones) != 1 || view.LastSeq != 1 {
		t.Fatalf("view = %+v", view)
	}
}

func TestWatchRejectsUnknownAndMalformedTokens(t *testing.T) {
	e := newEnv(t, nil)
	valid, _ := jobs.NewToken()
	for name, token := range map[string]string{"unknown": valid, "malformed": "abc", "empty": ""} {
		if _, err := e.svc.Watch(context.Background(), token, 0); err == nil || kindOf(t, err) != service.KindNotFound {
			t.Errorf("Watch %s: %v", name, err)
		}
		if _, err := e.svc.Status(context.Background(), token); err == nil || kindOf(t, err) != service.KindNotFound {
			t.Errorf("Status %s: %v", name, err)
		}
	}
}

func TestFinishedJobsAreServedFromPersistedHistory(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	target, _ := e.svc.Resolve(ctx, service.ResolveInput{Query: "Kepler-186"})

	finish := func(key, state, errMsg string, result json.RawMessage) string {
		joined, err := e.svc.Jobs.Join(ctx, jobs.NewJob{Kind: jobs.KindExtract, DedupeKey: key, TargetID: target.ID, Request: json.RawMessage(`{}`)}, time.Now(), time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		_ = e.svc.Jobs.AddMilestone(ctx, joined.Job.ID, 1, "queued", "Waiting", time.Now())
		_ = e.svc.Jobs.AddMilestone(ctx, joined.Job.ID, 2, "running", "Started", time.Now())
		if err := e.svc.Jobs.Finish(ctx, joined.Job.ID, state, result, errMsg, time.Now()); err != nil {
			t.Fatal(err)
		}
		return joined.Token
	}

	done := finish("done", jobs.StateDone, "", json.RawMessage(`{"features":[]}`))
	watch, err := e.svc.Watch(ctx, done, 0)
	if err != nil || watch.Sub != nil || len(watch.History) != 3 {
		t.Fatalf("history = %+v, %v", watch, err)
	}
	last := watch.History[2]
	if last.Type != hub.TypeResult || last.Seq != 3 || string(last.Payload) != `{"features":[]}` {
		t.Fatalf("final event = %+v", last)
	}

	later, _ := e.svc.Watch(ctx, done, 2)
	if len(later.History) != 1 || later.History[0].Type != hub.TypeResult {
		t.Fatalf("since=2 must return only the final event, got %+v", later.History)
	}

	past, _ := e.svc.Watch(ctx, done, 50)
	if len(past.History) != 1 || past.History[0].Type != hub.TypeResult || past.History[0].Seq <= 50 {
		t.Fatalf("a client ahead of the persisted milestones must still get the final event, got %+v", past.History)
	}

	failed := finish("failed", jobs.StateFailed, "No Kepler light curves found", nil)
	failedWatch, _ := e.svc.Watch(ctx, failed, 0)
	if end := failedWatch.History[len(failedWatch.History)-1]; end.Type != hub.TypeError || !strings.Contains(end.Message, "No Kepler") {
		t.Fatalf("a failed job must end with an error event, got %+v", end)
	}

	view, err := e.svc.Status(ctx, done)
	if err != nil || view.State != jobs.StateDone || string(view.Result) != `{"features":[]}` || view.LastSeq != 3 {
		t.Fatalf("view = %+v, %v", view, err)
	}
}

func TestUnavailableMessagesHideUpstreamDetail(t *testing.T) {
	detail := &upstream.Error{Kind: upstream.KindUnavailable, Message: "transport: Error while dialing: dial tcp 10.1.2.3:50051: connect: connection refused"}
	e := newEnv(t, func(e *env) {
		e.shaula.ResolveFn = func(context.Context, upstream.ResolveRequest) (upstream.Resolved, error) {
			return upstream.Resolved{}, detail
		}
		e.fang.ListRunsErr = detail
	})
	ctx := context.Background()
	_, resolveErr := e.svc.Resolve(ctx, service.ResolveInput{Query: "x"})
	_, runsErr := e.svc.Runs(ctx)
	for want, err := range map[string]error{
		"the pipeline service is unavailable; try again later": resolveErr,
		"the model service is unavailable; try again later":    runsErr,
	} {
		if err == nil || kindOf(t, err) != service.KindUnavailable || err.Error() != want {
			t.Errorf("err = %v, want %q", err, want)
		}
	}
}

func TestAModelThatDoesNotMatchTheSchemaIsUnavailable(t *testing.T) {
	e := newEnv(t, func(e *env) {
		e.fang.PredictFn = func(context.Context, upstream.PredictRequest) (upstream.Prediction, error) {
			return upstream.Prediction{}, &upstream.Error{Kind: upstream.KindPrecondition, Message: "model does not match the installed schema"}
		}
	})
	_, err := e.svc.Predict(context.Background(), service.PredictInput{Rows: []map[string]any{{"a": 1.0}}})
	if err == nil || kindOf(t, err) != service.KindUnavailable {
		t.Fatalf("err = %v, want KindUnavailable", err)
	}
}
