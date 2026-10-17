package jobs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/rachzy/antares/apps/api/internal/cache"
	"github.com/rachzy/antares/apps/api/internal/hub"
	"github.com/rachzy/antares/apps/api/internal/jobs"
	"github.com/rachzy/antares/apps/api/internal/store"
	"github.com/rachzy/antares/apps/api/internal/testdb"
	"github.com/rachzy/antares/apps/api/internal/upstream"
	"github.com/rachzy/antares/apps/api/internal/upstream/fake"
)

type rig struct {
	runner *jobs.Runner
	store  jobs.Store
	feats  cache.FeatureCache
	preds  cache.PredictionCache
	hub    *hub.Hub
	shaula *fake.Shaula
	fang   *fake.Fang
	target int64
	seq    int
}

func newRig(t *testing.T) *rig {
	t.Helper()
	db := testdb.Open(t)
	target, err := store.Targets{DB: db}.Upsert(context.Background(), store.Target{
		Catalog: "KIC", CatalogID: "8120608", RA: 298.65, Dec: 43.95,
		DisplayName: "Kepler-186", Designation: "KIC 8120608",
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	g := &rig{
		store:  jobs.Store{DB: db},
		feats:  cache.FeatureCache{DB: db},
		preds:  cache.PredictionCache{DB: db},
		hub:    hub.New(500, 64, time.Now),
		shaula: &fake.Shaula{},
		fang:   &fake.Fang{},
		target: target.ID,
	}
	g.runner = &jobs.Runner{
		Jobs: g.store, Features: g.feats, Predictions: g.preds, Hub: g.hub,
		Shaula: g.shaula, Fang: g.fang, Clock: time.Now,
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		JobTimeout: 10 * time.Second, StallTimeout: 10 * time.Second, TESSTTL: 720 * time.Hour,
		ShaulaVersion: func() string { return "0.0.1a0" },
	}
	return g
}

func (g *rig) payload(mutate func(*jobs.Payload)) jobs.Payload {
	p := jobs.Payload{
		Target: jobs.TargetInfo{
			Catalog: "KIC", CatalogID: "8120608", DisplayName: "Kepler-186",
			Designation: "KIC 8120608", RA: 298.65, Dec: 43.95,
		},
		TargetID: g.target,
		Params:   cache.ExtractParams{Mission: "Kepler", SigmaClip: 5},
		RunID:    "run-1",
		StarID:   "KIC-8120608",
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}

func (g *rig) submit(t *testing.T, kind string, mutate func(*jobs.Payload)) (jobs.Job, jobs.Payload) {
	t.Helper()
	p := g.payload(mutate)
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	g.seq++
	joined, err := g.store.Join(context.Background(), jobs.NewJob{
		Kind: kind, DedupeKey: fmt.Sprintf("k%d", g.seq), TargetID: g.target, Request: body,
	}, time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return joined.Job, p
}

func (g *rig) events(jobID string) []hub.Event {
	room, ok := g.hub.Lookup(jobID)
	if !ok {
		return nil
	}
	sub := room.Subscribe(0)
	defer sub.Cancel()
	return sub.Replay
}

func (g *rig) job(t *testing.T, id string) jobs.Job {
	t.Helper()
	job, err := g.store.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func TestExtractJobRelaysProgressCachesAndFinishes(t *testing.T) {
	g := newRig(t)
	job, p := g.submit(t, jobs.KindExtract, nil)

	g.runner.Execute(context.Background(), job.ID)

	done := g.job(t, job.ID)
	if done.State != jobs.StateDone || done.Error != "" {
		t.Fatalf("job = %+v", done)
	}
	var result jobs.Result
	if err := json.Unmarshal(done.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Cached || result.ShaulaVersion != "0.0.1a0" || len(result.Features) != 1 || result.Target.Designation != "KIC 8120608" {
		t.Fatalf("result = %+v", result)
	}
	if result.Features[0]["vshape_metric"] != nil || result.Features[0]["period_days"] != 129.9 {
		t.Fatalf("features = %v", result.Features)
	}

	events := g.events(job.ID)
	if len(events) < 5 {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Type != hub.TypeState || events[0].Stage != "running" {
		t.Fatalf("first event = %+v", events[0])
	}
	if last := events[len(events)-1]; last.Type != hub.TypeResult || len(last.Payload) == 0 {
		t.Fatalf("last event = %+v", last)
	}
	for _, ev := range events {
		if ev.Stage == "done" {
			t.Fatalf("Shaula's own done stage must not be relayed: %+v", ev)
		}
	}
	room, _ := g.hub.Lookup(job.ID)
	if !room.Closed() {
		t.Fatal("the room must be closed once the job ends")
	}

	var stages []string
	milestones, err := g.store.Milestones(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range milestones {
		stages = append(stages, m.Stage)
	}
	if strings.Join(stages, ",") != "running,downloading,detrending,period_search" {
		t.Fatalf("milestones = %v, want one per stage change", stages)
	}

	entry, ok, err := g.feats.Get(context.Background(), g.target, p.Params.Hash(), "0.0.1a0", time.Now())
	if err != nil || !ok || len(entry.Rows) != 1 || entry.ExpiresAt != nil {
		t.Fatalf("feature cache: %+v ok=%v err=%v", entry, ok, err)
	}
	req := g.shaula.ExtractRequests()[0]
	if req.Target != "KIC 8120608" || req.Mission != "Kepler" || req.SigmaClip != 5 {
		t.Fatalf("Shaula must be asked for the designation, got %+v", req)
	}
}

func TestAnalyzeJobScoresAndCachesPredictions(t *testing.T) {
	g := newRig(t)
	job, _ := g.submit(t, jobs.KindAnalyze, nil)

	g.runner.Execute(context.Background(), job.ID)

	done := g.job(t, job.ID)
	if done.State != jobs.StateDone {
		t.Fatalf("job = %+v", done)
	}
	var result jobs.Result
	_ = json.Unmarshal(done.Result, &result)
	if result.ModelRunID != "run-1" || result.SchemaVersion != 1 || len(result.Predictions) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.Predictions[0]["prob_stack"] != 0.71 {
		t.Fatalf("predictions = %v", result.Predictions)
	}

	reqs := g.fang.PredictRequests()
	if len(reqs) != 1 || reqs[0].RunID != "run-1" || reqs[0].StarID != "KIC-8120608" || len(reqs[0].Rows) != 1 {
		t.Fatalf("predict requests = %+v", reqs)
	}
	hash, _ := cache.FeaturesHash(result.Features)
	if _, ok, _ := g.preds.Get(context.Background(), hash, "run-1"); !ok {
		t.Fatal("the prediction must be cached under the features hash and the run id")
	}
	stages := ""
	milestones, _ := g.store.Milestones(context.Background(), job.ID)
	for _, m := range milestones {
		stages += m.Stage + ","
	}
	if !strings.HasSuffix(stages, "period_search,predicting,") {
		t.Fatalf("milestones = %s, want predicting after the extraction stages", stages)
	}
}

func TestUpstreamFailureFailsTheJobAndCachesNothing(t *testing.T) {
	g := newRig(t)
	g.shaula.ExtractFn = func(_ context.Context, _ upstream.ExtractRequest, onProgress func(upstream.Progress)) (upstream.FeatureSet, error) {
		onProgress(upstream.Progress{Stage: "downloading", Message: "Downloading"})
		return upstream.FeatureSet{}, &upstream.Error{Kind: upstream.KindPrecondition, Message: "No Kepler light curves found for 'x'"}
	}
	job, p := g.submit(t, jobs.KindExtract, nil)

	g.runner.Execute(context.Background(), job.ID)

	failed := g.job(t, job.ID)
	if failed.State != jobs.StateFailed || !strings.Contains(failed.Error, "No Kepler light curves found") {
		t.Fatalf("job = %+v", failed)
	}
	if len(failed.Result) != 0 {
		t.Fatal("a failed job must carry no result")
	}
	events := g.events(job.ID)
	if last := events[len(events)-1]; last.Type != hub.TypeError || !strings.Contains(last.Message, "No Kepler") {
		t.Fatalf("last event = %+v", last)
	}
	for _, ev := range events {
		if ev.Type == hub.TypeResult {
			t.Fatal("a partial stream must never look like a result")
		}
	}
	if _, ok, _ := g.feats.Get(context.Background(), g.target, p.Params.Hash(), "0.0.1a0", time.Now()); ok {
		t.Fatal("a failed extraction must not populate the cache")
	}
	if room, _ := g.hub.Lookup(job.ID); !room.Closed() {
		t.Fatal("the room must be closed so the slot and the subscribers are released")
	}
}

func TestPredictionFailureKeepsTheFeaturesButFailsTheJob(t *testing.T) {
	g := newRig(t)
	g.fang.PredictFn = func(context.Context, upstream.PredictRequest) (upstream.Prediction, error) {
		return upstream.Prediction{}, &upstream.Error{Kind: upstream.KindUnavailable, Message: "transport: Error while dialing: dial tcp 10.1.2.3:50052: connect: connection refused"}
	}
	job, p := g.submit(t, jobs.KindAnalyze, nil)

	g.runner.Execute(context.Background(), job.ID)

	failed := g.job(t, job.ID)
	if failed.State != jobs.StateFailed || failed.Error != "the model service is unavailable; try again later" {
		t.Fatalf("job = %+v", failed)
	}
	if _, ok, _ := g.feats.Get(context.Background(), g.target, p.Params.Hash(), "0.0.1a0", time.Now()); !ok {
		t.Fatal("valid features must stay cached so a retry skips the ten-minute extraction")
	}
}

func TestStalledExtractionIsCancelled(t *testing.T) {
	g := newRig(t)
	g.runner.StallTimeout = 80 * time.Millisecond
	g.shaula.ExtractFn = func(ctx context.Context, _ upstream.ExtractRequest, _ func(upstream.Progress)) (upstream.FeatureSet, error) {
		<-ctx.Done()
		return upstream.FeatureSet{}, ctx.Err()
	}
	job, p := g.submit(t, jobs.KindExtract, nil)

	start := time.Now()
	g.runner.Execute(context.Background(), job.ID)

	if time.Since(start) > 3*time.Second {
		t.Fatal("the stall detector did not free the job")
	}
	failed := g.job(t, job.ID)
	if failed.State != jobs.StateFailed || !strings.Contains(failed.Error, "no progress") {
		t.Fatalf("job = %+v", failed)
	}
	if _, ok, _ := g.feats.Get(context.Background(), g.target, p.Params.Hash(), "0.0.1a0", time.Now()); ok {
		t.Fatal("a stalled extraction must not populate the cache")
	}
}

func TestJobDeadlineIsEnforced(t *testing.T) {
	g := newRig(t)
	g.runner.JobTimeout = 80 * time.Millisecond
	g.runner.StallTimeout = 5 * time.Second
	g.shaula.ExtractFn = func(ctx context.Context, _ upstream.ExtractRequest, onProgress func(upstream.Progress)) (upstream.FeatureSet, error) {
		for {
			select {
			case <-ctx.Done():
				return upstream.FeatureSet{}, ctx.Err()
			case <-time.After(10 * time.Millisecond):
				onProgress(upstream.Progress{Stage: "period_search", Message: "still going"})
			}
		}
	}
	job, _ := g.submit(t, jobs.KindExtract, nil)

	g.runner.Execute(context.Background(), job.ID)

	failed := g.job(t, job.ID)
	if failed.State != jobs.StateFailed || !strings.Contains(failed.Error, "time limit") {
		t.Fatalf("job = %+v", failed)
	}
}

func TestAPanicInAnUpstreamFailsTheJobInsteadOfCrashing(t *testing.T) {
	g := newRig(t)
	g.shaula.ExtractFn = func(context.Context, upstream.ExtractRequest, func(upstream.Progress)) (upstream.FeatureSet, error) {
		panic("kaboom")
	}
	job, _ := g.submit(t, jobs.KindExtract, nil)

	g.runner.Execute(context.Background(), job.ID)

	failed := g.job(t, job.ID)
	if failed.State != jobs.StateFailed || failed.Error != "internal error" {
		t.Fatalf("job = %+v", failed)
	}
}

func TestOnlyTESSFeaturesExpire(t *testing.T) {
	g := newRig(t)
	for mission, wantExpiry := range map[string]bool{"Kepler": false, "K2": false, "TESS": true} {
		job, p := g.submit(t, jobs.KindExtract, func(p *jobs.Payload) { p.Params.Mission = mission })
		g.runner.Execute(context.Background(), job.ID)
		entry, ok, err := g.feats.Get(context.Background(), g.target, p.Params.Hash(), "0.0.1a0", time.Now())
		if err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", mission, ok, err)
		}
		if (entry.ExpiresAt != nil) != wantExpiry {
			t.Errorf("%s: expiry = %v, want expiring=%v", mission, entry.ExpiresAt, wantExpiry)
		}
	}
}

func TestFeaturesAreCachedUnderTheVersionThatProducedThem(t *testing.T) {
	g := newRig(t)
	g.shaula.ExtractFn = func(_ context.Context, req upstream.ExtractRequest, _ func(upstream.Progress)) (upstream.FeatureSet, error) {
		set := fake.DefaultFeatureSet(req)
		set.LibraryVersion = "9.9.9"
		return set, nil
	}
	job, p := g.submit(t, jobs.KindExtract, nil)
	g.runner.Execute(context.Background(), job.ID)

	if _, ok, _ := g.feats.Get(context.Background(), g.target, p.Params.Hash(), "9.9.9", time.Now()); !ok {
		t.Fatal("the entry belongs to the version that produced it")
	}
	if _, ok, _ := g.feats.Get(context.Background(), g.target, p.Params.Hash(), "0.0.1a0", time.Now()); ok {
		t.Fatal("it must not be filed under the version the API happened to know")
	}
	var result jobs.Result
	_ = json.Unmarshal(g.job(t, job.ID).Result, &result)
	if result.ShaulaVersion != "9.9.9" {
		t.Fatalf("result version = %q", result.ShaulaVersion)
	}
}

func TestAJobThatAlreadyFinishedIsNotRunAgain(t *testing.T) {
	g := newRig(t)
	job, _ := g.submit(t, jobs.KindExtract, nil)
	if err := g.store.Finish(context.Background(), job.ID, jobs.StateDropped, nil, "no client", time.Now()); err != nil {
		t.Fatal(err)
	}

	g.runner.Execute(context.Background(), job.ID)

	if g.shaula.ExtractCount() != 0 {
		t.Fatal("a dropped job must never reach Shaula")
	}
	if got := g.job(t, job.ID); got.State != jobs.StateDropped {
		t.Fatalf("state = %s", got.State)
	}
}

func TestDropRecordsTheStateAndNotifiesSubscribers(t *testing.T) {
	g := newRig(t)
	job, _ := g.submit(t, jobs.KindExtract, nil)
	room := g.hub.Room(job.ID)
	sub := room.Subscribe(0)
	defer sub.Cancel()

	g.runner.Drop(context.Background(), job.ID, "nobody was connected when your turn came; resubmit")

	got := g.job(t, job.ID)
	if got.State != jobs.StateDropped || !strings.Contains(got.Error, "resubmit") {
		t.Fatalf("job = %+v", got)
	}
	ev := <-sub.C
	if ev.Type != hub.TypeError || !strings.Contains(ev.Message, "resubmit") {
		t.Fatalf("event = %+v", ev)
	}
	if _, open := <-sub.C; open {
		t.Fatal("the room must be closed after a drop")
	}
}

func TestJobErrorsNeverExposeInternalDetail(t *testing.T) {
	const private = "dial tcp 10.1.2.3:50051"
	for name, tc := range map[string]struct {
		extract func() error
		breakDB string
		request string
		want    string
	}{
		"unreachable pipeline": {
			extract: func() error {
				return &upstream.Error{Kind: upstream.KindUnavailable, Message: "transport: Error while dialing: " + private}
			},
			want: "the pipeline service is unavailable; try again later",
		},
		"upstream internal": {
			extract: func() error {
				return &upstream.Error{Kind: upstream.KindInternal, Message: "stream reset by " + private}
			},
			want: "internal error",
		},
		"unexpected error": {
			extract: func() error { return fmt.Errorf("read %s: connection reset", private) },
			want:    "internal error",
		},
		"cache write":  {breakDB: "DROP TABLE feature_cache", want: "internal error"},
		"bad payload":  {request: `{"target":5}`, want: "internal error"},
		"precondition": {extract: func() error { return &upstream.Error{Kind: upstream.KindPrecondition, Message: "No light curves"} }, want: "No light curves"},
	} {
		t.Run(name, func(t *testing.T) {
			g := newRig(t)
			var logs strings.Builder
			g.runner.Log = slog.New(slog.NewTextHandler(&logs, nil))
			if tc.extract != nil {
				g.shaula.ExtractFn = func(context.Context, upstream.ExtractRequest, func(upstream.Progress)) (upstream.FeatureSet, error) {
					return upstream.FeatureSet{}, tc.extract()
				}
			}
			if tc.breakDB != "" {
				if _, err := g.feats.DB.Exec(tc.breakDB); err != nil {
					t.Fatal(err)
				}
			}
			var job jobs.Job
			if tc.request != "" {
				joined, err := g.store.Join(context.Background(), jobs.NewJob{
					Kind: jobs.KindExtract, DedupeKey: "bad", TargetID: g.target, Request: json.RawMessage(tc.request),
				}, time.Now(), time.Hour)
				if err != nil {
					t.Fatal(err)
				}
				job = joined.Job
			} else {
				job, _ = g.submit(t, jobs.KindExtract, nil)
			}

			g.runner.Execute(context.Background(), job.ID)

			failed := g.job(t, job.ID)
			if failed.State != jobs.StateFailed || failed.Error != tc.want {
				t.Fatalf("error = %q, want %q", failed.Error, tc.want)
			}
			for _, leak := range []string{"10.1.2.3", "no such table", "json", "cache features"} {
				if strings.Contains(failed.Error, leak) {
					t.Fatalf("job error %q leaks %q", failed.Error, leak)
				}
			}
			if tc.want == "internal error" && !strings.Contains(logs.String(), job.ID) {
				t.Fatalf("the detail must be logged with the job id: %s", logs.String())
			}
		})
	}
}
