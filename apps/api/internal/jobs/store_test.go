package jobs_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"antares/api/internal/jobs"
	"antares/api/internal/store"
	"antares/api/internal/testdb"
)

var t0 = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) (jobs.Store, *sql.DB, int64) {
	t.Helper()
	db := testdb.Open(t)
	target, err := store.Targets{DB: db}.Upsert(context.Background(), store.Target{
		Catalog: "KIC", CatalogID: "8120608", RA: 298.65, Dec: 43.95,
		DisplayName: "Kepler-186", Designation: "KIC 8120608",
	}, t0)
	if err != nil {
		t.Fatal(err)
	}
	return jobs.Store{DB: db}, db, target.ID
}

func newJob(target int64, key string) jobs.NewJob {
	return jobs.NewJob{Kind: jobs.KindExtract, DedupeKey: key, TargetID: target, Request: json.RawMessage(`{"q":1}`)}
}

func TestTokensAreLongRandomAndOnlyHashedAtRest(t *testing.T) {
	a, err := jobs.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := jobs.NewToken()
	if a == b || len(a) != 43 {
		t.Fatalf("tokens must be unique and 43 characters, got %q and %q", a, b)
	}
	if !jobs.ValidToken(a) || jobs.ValidToken("short") || jobs.ValidToken(strings.Repeat("!", 43)) {
		t.Fatal("ValidToken accepts or rejects the wrong shapes")
	}
	if jobs.HashToken(a) == a || jobs.HashToken(a) != jobs.HashToken(a) {
		t.Fatal("the hash must be deterministic and differ from the token")
	}

	s, db, target := newStore(t)
	joined, err := s.Join(context.Background(), newJob(target, "k"), t0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var plain int
	if err := db.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE token_hash = ?`, joined.Token).Scan(&plain); err != nil || plain != 0 {
		t.Fatalf("the plaintext token is stored: count=%d err=%v", plain, err)
	}
	var hashed int
	if err := db.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE token_hash = ?`, jobs.HashToken(joined.Token)).Scan(&hashed); err != nil || hashed != 1 {
		t.Fatalf("the hashed token is missing: count=%d err=%v", hashed, err)
	}
}

func TestJoinCreatesAJobAndASubscription(t *testing.T) {
	s, _, target := newStore(t)
	joined, err := s.Join(context.Background(), newJob(target, "k"), t0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !joined.Created || joined.Job.State != jobs.StateQueued || joined.Job.ID == "" || joined.Token == "" {
		t.Fatalf("joined = %+v", joined)
	}
	if string(joined.Job.Request) != `{"q":1}` || joined.Job.TargetID != target {
		t.Fatalf("job = %+v", joined.Job)
	}
	got, ok, err := s.ByToken(context.Background(), joined.Token, t0)
	if err != nil || !ok || got.ID != joined.Job.ID {
		t.Fatalf("ByToken: %+v ok=%v err=%v", got, ok, err)
	}
}

func TestJoinSharesTheActiveJobButIssuesAnotherToken(t *testing.T) {
	s, _, target := newStore(t)
	ctx := context.Background()
	first, err := s.Join(ctx, newJob(target, "same"), t0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Join(ctx, newJob(target, "same"), t0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.Job.ID != first.Job.ID {
		t.Fatalf("the second caller must join the first job: %+v", second)
	}
	if second.Token == first.Token {
		t.Fatal("each caller must get its own token")
	}
	if _, err := s.MarkRunning(ctx, first.Job.ID, t0); err != nil {
		t.Fatal(err)
	}
	third, _ := s.Join(ctx, newJob(target, "same"), t0, time.Hour)
	if third.Created || third.Job.ID != first.Job.ID {
		t.Fatal("a running job must also be joined")
	}
	other, _ := s.Join(ctx, newJob(target, "different"), t0, time.Hour)
	if !other.Created || other.Job.ID == first.Job.ID {
		t.Fatal("a different key must create a different job")
	}
}

func TestJoinAfterCompletionCreatesAFreshJob(t *testing.T) {
	s, _, target := newStore(t)
	ctx := context.Background()
	first, _ := s.Join(ctx, newJob(target, "k"), t0, time.Hour)
	if err := s.Finish(ctx, first.Job.ID, jobs.StateDone, json.RawMessage(`{}`), "", t0); err != nil {
		t.Fatal(err)
	}
	again, err := s.Join(ctx, newJob(target, "k"), t0, time.Hour)
	if err != nil || !again.Created || again.Job.ID == first.Job.ID {
		t.Fatalf("after a job finishes the same key starts a new one: %+v err=%v", again, err)
	}
}

func TestByTokenRejectsUnknownAndExpired(t *testing.T) {
	s, _, target := newStore(t)
	ctx := context.Background()
	joined, _ := s.Join(ctx, newJob(target, "k"), t0, time.Hour)

	if _, ok, _ := s.ByToken(ctx, joined.Token, t0.Add(30*time.Minute)); !ok {
		t.Error("a live token must resolve")
	}
	if _, ok, _ := s.ByToken(ctx, joined.Token, t0.Add(2*time.Hour)); ok {
		t.Error("an expired token must not resolve")
	}
	other, _ := jobs.NewToken()
	if _, ok, _ := s.ByToken(ctx, other, t0); ok {
		t.Error("an unknown token must not resolve")
	}
}

func TestMarkRunningAndFinishOnlyActOnActiveJobs(t *testing.T) {
	s, _, target := newStore(t)
	ctx := context.Background()
	joined, _ := s.Join(ctx, newJob(target, "k"), t0, time.Hour)
	id := joined.Job.ID

	if ok, err := s.MarkRunning(ctx, id, t0); err != nil || !ok {
		t.Fatalf("first MarkRunning: ok=%v err=%v", ok, err)
	}
	if ok, _ := s.MarkRunning(ctx, id, t0); ok {
		t.Error("a job that is already running must not be started twice")
	}
	if err := s.Finish(ctx, id, jobs.StateDone, json.RawMessage(`{"ok":true}`), "", t0); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(ctx, id, jobs.StateFailed, nil, "late failure", t0); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, id)
	if err != nil || got.State != jobs.StateDone || string(got.Result) != `{"ok":true}` || got.Error != "" {
		t.Fatalf("a finished job must not be overwritten: %+v err=%v", got, err)
	}
	if !got.Terminal() || got.StartedAt == nil || got.FinishedAt == nil {
		t.Fatalf("timestamps or terminal flag missing: %+v", got)
	}
	if _, err := s.Get(ctx, "nope"); err != jobs.ErrNotFound {
		t.Fatalf("Get unknown = %v, want ErrNotFound", err)
	}
}

func TestMilestonesAreOrderedAndIdempotent(t *testing.T) {
	s, _, target := newStore(t)
	ctx := context.Background()
	joined, _ := s.Join(ctx, newJob(target, "k"), t0, time.Hour)
	id := joined.Job.ID
	for _, m := range []struct {
		seq   int
		stage string
	}{{3, "detrending"}, {1, "queued"}, {2, "running"}} {
		if err := s.AddMilestone(ctx, id, m.seq, m.stage, "msg "+m.stage, t0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddMilestone(ctx, id, 2, "running", "duplicate", t0); err != nil {
		t.Fatalf("a repeated milestone must be ignored, not fail: %v", err)
	}
	got, err := s.Milestones(ctx, id)
	if err != nil || len(got) != 3 {
		t.Fatalf("milestones = %+v err=%v", got, err)
	}
	for i, want := range []string{"queued", "running", "detrending"} {
		if got[i].Stage != want || got[i].Seq != i+1 {
			t.Errorf("milestone %d = %+v, want stage %s", i, got[i], want)
		}
	}
}

func TestFailInterruptedOnlyAffectsActiveJobs(t *testing.T) {
	s, _, target := newStore(t)
	ctx := context.Background()
	queued, _ := s.Join(ctx, newJob(target, "queued"), t0, time.Hour)
	running, _ := s.Join(ctx, newJob(target, "running"), t0, time.Hour)
	done, _ := s.Join(ctx, newJob(target, "done"), t0, time.Hour)
	_, _ = s.MarkRunning(ctx, running.Job.ID, t0)
	_ = s.Finish(ctx, done.Job.ID, jobs.StateDone, json.RawMessage(`{}`), "", t0)

	n, err := s.FailInterrupted(ctx, t0)
	if err != nil || n != 2 {
		t.Fatalf("FailInterrupted = %d, %v; want 2", n, err)
	}
	for _, id := range []string{queued.Job.ID, running.Job.ID} {
		got, _ := s.Get(ctx, id)
		if got.State != jobs.StateFailed || got.Error != jobs.InterruptedMessage {
			t.Errorf("job %s = %+v", id, got)
		}
	}
	if got, _ := s.Get(ctx, done.Job.ID); got.State != jobs.StateDone {
		t.Error("a finished job must be left alone")
	}
	if _, ok, _ := s.ByToken(ctx, queued.Token, t0); !ok {
		t.Error("a token issued before the restart must keep working")
	}
}

func TestDeleteExpiredKeepsJobsWithLiveSubscriptions(t *testing.T) {
	s, db, target := newStore(t)
	ctx := context.Background()
	old, _ := s.Join(ctx, newJob(target, "old"), t0, time.Hour)
	_ = s.Finish(ctx, old.Job.ID, jobs.StateDone, json.RawMessage(`{}`), "", t0)
	_ = s.AddMilestone(ctx, old.Job.ID, 1, "queued", "m", t0)

	shared, _ := s.Join(ctx, newJob(target, "shared"), t0, time.Hour)
	late, _ := s.Join(ctx, newJob(target, "shared"), t0.Add(3*time.Hour), time.Hour)
	_ = s.Finish(ctx, shared.Job.ID, jobs.StateDone, json.RawMessage(`{}`), "", t0)

	running, _ := s.Join(ctx, newJob(target, "running"), t0, time.Hour)

	n, err := s.DeleteExpired(ctx, t0.Add(2*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("DeleteExpired removed %d jobs (err %v), want 1", n, err)
	}
	if _, err := s.Get(ctx, old.Job.ID); err != jobs.ErrNotFound {
		t.Error("an expired, finished job should be deleted")
	}
	var milestones int
	if err := db.QueryRow(`SELECT COUNT(*) FROM job_milestones WHERE job_id = ?`, old.Job.ID).Scan(&milestones); err != nil || milestones != 0 {
		t.Errorf("milestones must cascade: %d (err %v)", milestones, err)
	}
	if _, ok, _ := s.ByToken(ctx, late.Token, t0.Add(2*time.Hour)); !ok {
		t.Error("a job with a live subscription must be kept")
	}
	if _, err := s.Get(ctx, running.Job.ID); err != nil {
		t.Error("a job that has not finished must never be deleted, even with an expired subscription")
	}
}

func TestConcurrentJoinCreatesExactlyOneJob(t *testing.T) {
	s, _, target := newStore(t)
	const callers = 8

	var wg sync.WaitGroup
	results := make([]jobs.Joined, callers)
	errs := make([]error, callers)
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = s.Join(context.Background(), newJob(target, "same-star"), t0, time.Hour)
		}()
	}
	close(start)
	wg.Wait()

	created := 0
	tokens := map[string]bool{}
	for i, r := range results {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		if r.Created {
			created++
		}
		if r.Job.ID != results[0].Job.ID {
			t.Fatalf("caller %d is on job %s, not %s", i, r.Job.ID, results[0].Job.ID)
		}
		tokens[r.Token] = true
	}
	if created != 1 {
		t.Fatalf("%d callers created a job, want exactly 1", created)
	}
	if len(tokens) != callers {
		t.Fatalf("%d distinct tokens, want %d", len(tokens), callers)
	}
}
