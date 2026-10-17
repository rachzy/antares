package jobs_test

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/rachzy/antares/apps/api/internal/jobs"
)

type harness struct {
	mu        sync.Mutex
	absent    map[string]bool
	graceSeen []time.Duration
	started   []string
	dropped   []string
	positions map[string][]int
	hold      map[string]chan struct{}
}

func newHarness() *harness {
	return &harness{absent: map[string]bool{}, positions: map[string][]int{}, hold: map[string]chan struct{}{}}
}

func (h *harness) holdJob(ids ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range ids {
		h.hold[id] = make(chan struct{})
	}
}

func (h *harness) finish(id string) {
	h.mu.Lock()
	ch := h.hold[id]
	h.mu.Unlock()
	close(ch)
}

func (h *harness) setAbsent(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.absent[id] = true
}

func (h *harness) startedIDs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.started)
}

func (h *harness) droppedIDs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.dropped)
}

func (h *harness) options(maxActive, maxQueue int) jobs.SchedulerOptions {
	return jobs.SchedulerOptions{
		MaxActive: maxActive,
		MaxQueue:  maxQueue,
		Grace:     30 * time.Second,
		Present: func(id string, grace time.Duration) bool {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.graceSeen = append(h.graceSeen, grace)
			return !h.absent[id]
		},
		Run: func(id string) {
			h.mu.Lock()
			h.started = append(h.started, id)
			ch := h.hold[id]
			h.mu.Unlock()
			if ch != nil {
				<-ch
			}
		},
		Dropped: func(id string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.dropped = append(h.dropped, id)
		},
		Position: func(id string, position int) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.positions[id] = append(h.positions[id], position)
		},
	}
}

func eventually(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func mustEnqueue(t *testing.T, s *jobs.Scheduler, id string, wantPosition int) {
	t.Helper()
	got, err := s.Enqueue(id)
	if err != nil || got != wantPosition {
		t.Fatalf("Enqueue(%s) = %d, %v; want position %d", id, got, err, wantPosition)
	}
}

func TestStartsUpToMaxActiveAndQueuesTheRestInOrder(t *testing.T) {
	h := newHarness()
	h.holdJob("a", "b", "c", "d")
	s := jobs.NewScheduler(h.options(2, 10))

	mustEnqueue(t, s, "a", 0)
	mustEnqueue(t, s, "b", 0)
	mustEnqueue(t, s, "c", 1)
	mustEnqueue(t, s, "d", 2)

	eventually(t, "a and b running", func() bool { return len(h.startedIDs()) == 2 })
	if active, waiting := s.Stats(); active != 2 || waiting != 2 {
		t.Fatalf("stats = %d active, %d waiting", active, waiting)
	}

	h.finish("a")
	eventually(t, "c promoted", func() bool { ids := h.startedIDs(); return len(ids) == 3 && ids[2] == "c" })
	h.finish("b")
	eventually(t, "d promoted after c", func() bool { ids := h.startedIDs(); return len(ids) == 4 && ids[3] == "d" })

	h.finish("c")
	h.finish("d")
	s.Wait()
}

func TestAbsentHeadIsDroppedAndTheNextRuns(t *testing.T) {
	h := newHarness()
	h.holdJob("a", "c")
	s := jobs.NewScheduler(h.options(1, 10))

	mustEnqueue(t, s, "a", 0)
	mustEnqueue(t, s, "b", 1)
	mustEnqueue(t, s, "c", 2)
	h.setAbsent("b")

	h.finish("a")
	eventually(t, "c started past the absent b", func() bool { return slices.Equal(h.startedIDs(), []string{"a", "c"}) })
	if got := h.droppedIDs(); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("dropped = %v, want [b] exactly once", got)
	}

	h.finish("c")
	s.Wait()
}

func TestPresenceIsAskedWithTheConfiguredGrace(t *testing.T) {
	h := newHarness()
	h.holdJob("a")
	s := jobs.NewScheduler(h.options(1, 10))
	mustEnqueue(t, s, "a", 0)
	h.finish("a")
	s.Wait()

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.graceSeen) == 0 {
		t.Fatal("presence was never consulted before starting a job")
	}
	for _, grace := range h.graceSeen {
		if grace != 30*time.Second {
			t.Fatalf("grace = %s, want the configured 30s", grace)
		}
	}
}

func TestQueueFullIsReportedAndRecovers(t *testing.T) {
	h := newHarness()
	h.holdJob("a", "b")
	s := jobs.NewScheduler(h.options(1, 1))

	mustEnqueue(t, s, "a", 0)
	mustEnqueue(t, s, "b", 1)
	if _, err := s.Enqueue("c"); !errors.Is(err, jobs.ErrQueueFull) {
		t.Fatalf("err = %v, want ErrQueueFull", err)
	}

	h.finish("a")
	eventually(t, "b running", func() bool { return len(h.startedIDs()) == 2 })
	mustEnqueue(t, s, "c", 1)

	h.finish("b")
	eventually(t, "c running", func() bool { return len(h.startedIDs()) == 3 })
	s.Wait()
}

func TestZeroLengthQueueRejectsWhenBusy(t *testing.T) {
	h := newHarness()
	h.holdJob("a")
	s := jobs.NewScheduler(h.options(1, 0))
	mustEnqueue(t, s, "a", 0)
	if _, err := s.Enqueue("b"); !errors.Is(err, jobs.ErrQueueFull) {
		t.Fatalf("err = %v, want ErrQueueFull", err)
	}
	h.finish("a")
	s.Wait()
}

func TestPositionsAreReportedWhenTheyChange(t *testing.T) {
	h := newHarness()
	h.holdJob("a", "b", "c")
	s := jobs.NewScheduler(h.options(1, 10))

	mustEnqueue(t, s, "a", 0)
	mustEnqueue(t, s, "b", 1)
	mustEnqueue(t, s, "c", 2)

	h.mu.Lock()
	if len(h.positions["b"]) != 0 || len(h.positions["c"]) != 0 {
		h.mu.Unlock()
		t.Fatal("the position returned by Enqueue must not also be announced")
	}
	h.mu.Unlock()

	h.finish("a")
	eventually(t, "c told it moved up", func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return slices.Equal(h.positions["c"], []int{1})
	})
	h.mu.Lock()
	if len(h.positions["b"]) != 0 {
		h.mu.Unlock()
		t.Fatal("b started, so it must not be sent a position")
	}
	h.mu.Unlock()

	h.finish("b")
	h.finish("c")
	s.Wait()
}

func TestSweepDropsAbsentWaitingJobsWithoutTouchingRunningOnes(t *testing.T) {
	h := newHarness()
	h.holdJob("a", "c")
	s := jobs.NewScheduler(h.options(1, 10))

	mustEnqueue(t, s, "a", 0)
	mustEnqueue(t, s, "b", 1)
	mustEnqueue(t, s, "c", 2)
	h.setAbsent("b")
	h.setAbsent("a")

	s.Sweep()
	if got := h.droppedIDs(); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("dropped = %v, want only the waiting b", got)
	}
	if active, waiting := s.Stats(); active != 1 || waiting != 1 {
		t.Fatalf("stats = %d active, %d waiting", active, waiting)
	}
	eventually(t, "c moved to position 1", func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return slices.Contains(h.positions["c"], 1)
	})

	h.finish("a")
	eventually(t, "c running", func() bool { return slices.Contains(h.startedIDs(), "c") })
	h.finish("c")
	s.Wait()
}

func TestPositionReportsRunningWaitingAndUnknown(t *testing.T) {
	h := newHarness()
	h.holdJob("a", "b")
	s := jobs.NewScheduler(h.options(1, 10))
	mustEnqueue(t, s, "a", 0)
	mustEnqueue(t, s, "b", 1)

	if pos, running := s.Position("a"); pos != 0 || !running {
		t.Errorf("Position(a) = %d, %v", pos, running)
	}
	if pos, running := s.Position("b"); pos != 1 || running {
		t.Errorf("Position(b) = %d, %v", pos, running)
	}
	if pos, running := s.Position("zzz"); pos != 0 || running {
		t.Errorf("Position(zzz) = %d, %v", pos, running)
	}
	h.finish("a")
	h.finish("b")
	s.Wait()
}

func TestWaitBlocksUntilPromotedJobReturns(t *testing.T) {
	h := newHarness()
	h.holdJob("a", "b")
	s := jobs.NewScheduler(h.options(1, 10))
	mustEnqueue(t, s, "a", 0)
	mustEnqueue(t, s, "b", 1)
	h.finish("a")
	eventually(t, "b running", func() bool { return slices.Contains(h.startedIDs(), "b") })

	done := make(chan struct{})
	go func() { s.Wait(); close(done) }()
	select {
	case <-done:
		t.Fatal("Wait returned while b was still running")
	case <-time.After(50 * time.Millisecond):
	}
	h.finish("b")
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return after b finished")
	}
}
