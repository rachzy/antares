package hub_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"antares/api/internal/hub"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newRoom(ring, buf int) (*hub.Room, *fakeClock) {
	clock := newClock()
	return hub.New(ring, buf, clock.Now).Room("job"), clock
}

func seqs(events []hub.Event) []uint64 {
	out := make([]uint64, len(events))
	for i, e := range events {
		out[i] = e.Seq
	}
	return out
}

func TestAppendAssignsIncreasingSequenceNumbers(t *testing.T) {
	room, clock := newRoom(10, 10)
	first := room.Append(hub.State("queued", "waiting"))
	second := room.Append(hub.Progress("downloading", "", nil))
	if first.Seq != 1 || second.Seq != 2 || room.LastSeq() != 2 {
		t.Fatalf("seqs = %d, %d, last %d", first.Seq, second.Seq, room.LastSeq())
	}
	if !first.At.Equal(clock.Now()) {
		t.Fatalf("At = %v, want the clock's time", first.At)
	}
}

func TestEventConstructors(t *testing.T) {
	frac := 0.5
	cases := map[string]hub.Event{
		hub.TypeState:    hub.State("queued", "m"),
		hub.TypeProgress: hub.Progress("period_search", "m", &frac),
		hub.TypeResult:   hub.Result(json.RawMessage(`{"a":1}`)),
		hub.TypeError:    hub.Failure("boom"),
	}
	for want, ev := range cases {
		if ev.Type != want {
			t.Errorf("type = %q, want %q", ev.Type, want)
		}
	}
	if cases[hub.TypeProgress].Fraction == nil || *cases[hub.TypeProgress].Fraction != 0.5 {
		t.Error("fraction lost")
	}
	if string(cases[hub.TypeResult].Payload) != `{"a":1}` || cases[hub.TypeError].Message != "boom" {
		t.Error("payload or message lost")
	}
}

func TestRingEvictsTheOldestAndReportsTruncation(t *testing.T) {
	room, _ := newRoom(3, 10)
	for i := 0; i < 5; i++ {
		room.Append(hub.State("s", "m"))
	}

	cases := []struct {
		since     uint64
		want      []uint64
		truncated bool
	}{
		{0, []uint64{3, 4, 5}, true},
		{1, []uint64{3, 4, 5}, true},
		{2, []uint64{3, 4, 5}, false},
		{4, []uint64{5}, false},
		{5, nil, false},
	}
	for _, c := range cases {
		sub := room.Subscribe(c.since)
		got := seqs(sub.Replay)
		if len(got) != len(c.want) {
			t.Errorf("since %d: replay %v, want %v", c.since, got, c.want)
		} else {
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("since %d: replay %v, want %v", c.since, got, c.want)
				}
			}
		}
		if sub.Truncated != c.truncated {
			t.Errorf("since %d: truncated = %v, want %v", c.since, sub.Truncated, c.truncated)
		}
		sub.Cancel()
	}
}

func TestReplayAndLiveHaveNoGapOrDuplicate(t *testing.T) {
	room, _ := newRoom(1000, 1000)
	const total = 500

	done := make(chan struct{})
	go func() {
		for i := 0; i < total; i++ {
			room.Append(hub.State("s", "m"))
			if i%7 == 0 {
				time.Sleep(10 * time.Microsecond)
			}
		}
		close(done)
	}()

	time.Sleep(time.Millisecond)
	sub := room.Subscribe(0)
	<-done
	room.Close()

	got := seqs(sub.Replay)
	for ev := range sub.C {
		got = append(got, ev.Seq)
	}
	if len(got) != total {
		t.Fatalf("received %d events, want %d", len(got), total)
	}
	for i, seq := range got {
		if seq != uint64(i+1) {
			t.Fatalf("event %d has seq %d: a gap or duplicate between replay and live", i, seq)
		}
	}
}

func TestSlowConsumerIsDroppedWithoutBlockingTheProducer(t *testing.T) {
	room, _ := newRoom(300, 4)
	slow := room.Subscribe(0)
	healthy := room.Subscribe(0)

	var received []uint64
	collected := make(chan struct{})
	go func() {
		for ev := range healthy.C {
			received = append(received, ev.Seq)
		}
		close(collected)
	}()

	const total = 200
	produced := make(chan struct{})
	go func() {
		for i := 0; i < total; i++ {
			room.Append(hub.State("s", "m"))
			time.Sleep(100 * time.Microsecond)
		}
		close(produced)
	}()

	select {
	case <-produced:
	case <-time.After(5 * time.Second):
		t.Fatal("the producer blocked on a consumer that never reads")
	}
	room.Close()
	<-collected

	if !slow.Dropped() {
		t.Error("the stuck consumer should have been dropped")
	}
	drained := 0
	for range slow.C {
		drained++
	}
	if drained > 4 {
		t.Errorf("dropped consumer saw %d events, want at most its buffer of 4", drained)
	}
	if len(received) != total {
		t.Fatalf("the healthy consumer received %d events, want %d", len(received), total)
	}
	if healthy.Dropped() {
		t.Error("the healthy consumer must not be dropped")
	}
}

func TestCloseEndsSubscriptionsAndLateSubscribersGetTheReplay(t *testing.T) {
	room, _ := newRoom(10, 10)
	room.Append(hub.State("a", ""))
	room.Append(hub.State("b", ""))
	live := room.Subscribe(0)
	room.Append(hub.Result(json.RawMessage(`{}`)))
	room.Close()
	room.Close()

	if !room.Closed() {
		t.Fatal("Closed() = false")
	}
	if _, ok := room.ClosedAt(); !ok {
		t.Fatal("ClosedAt not recorded")
	}
	events := 0
	for range live.C {
		events++
	}
	if events != 1 {
		t.Errorf("live subscriber saw %d events after subscribing, want 1", events)
	}
	if ev := room.Append(hub.State("late", "")); ev.Seq != 0 {
		t.Error("appending to a closed room must be a no-op")
	}

	late := room.Subscribe(0)
	if len(late.Replay) != 3 {
		t.Errorf("late subscriber replay = %d events, want 3", len(late.Replay))
	}
	if _, open := <-late.C; open {
		t.Error("a late subscriber's channel must already be closed")
	}
}

func TestPresenceFollowsConnectionsAndGrace(t *testing.T) {
	room, clock := newRoom(10, 10)
	grace := 30 * time.Second

	if !room.Present(grace) {
		t.Fatal("a new room must count as present: the browser has not connected yet")
	}
	clock.Advance(31 * time.Second)
	if room.Present(grace) {
		t.Fatal("nobody ever connected and the grace has passed")
	}

	sub := room.Subscribe(0)
	clock.Advance(time.Hour)
	if !room.Present(grace) {
		t.Fatal("an attached subscriber means present, however long ago it attached")
	}
	sub.Cancel()
	if !room.Present(grace) {
		t.Fatal("a blip just after detaching must still count as present")
	}
	clock.Advance(31 * time.Second)
	if room.Present(grace) {
		t.Fatal("detached beyond the grace")
	}
	room.Touch()
	if !room.Present(grace) {
		t.Fatal("Touch must restart the grace period")
	}
}

func TestCancelIsIdempotentAndKeepsTheConnectionCount(t *testing.T) {
	room, clock := newRoom(10, 10)
	a := room.Subscribe(0)
	b := room.Subscribe(0)
	a.Cancel()
	a.Cancel()
	clock.Advance(time.Hour)
	if !room.Present(time.Second) {
		t.Fatal("cancelling twice must not detach the other subscriber")
	}
	b.Cancel()
	clock.Advance(time.Minute)
	if room.Present(time.Second) {
		t.Fatal("everyone detached")
	}
}

func TestBatchesDeliverReplayFirstThenCoalesceLiveEvents(t *testing.T) {
	room, _ := newRoom(100, 100)
	room.Append(hub.State("a", ""))
	room.Append(hub.State("b", ""))
	room.Append(hub.State("c", ""))
	sub := room.Subscribe(0)
	defer sub.Cancel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := hub.Batches(ctx, sub, 30*time.Millisecond)

	next := func() []hub.Event {
		t.Helper()
		select {
		case batch, ok := <-out:
			if !ok {
				t.Fatal("batches closed early")
			}
			return batch
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for a batch")
			return nil
		}
	}

	if got := next(); len(got) != 3 {
		t.Fatalf("replay batch = %d events, want 3", len(got))
	}
	for i := 0; i < 5; i++ {
		room.Append(hub.Progress("period_search", "", nil))
	}
	if got := next(); len(got) != 5 {
		t.Fatalf("live batch = %d events, want the 5 appended together coalesced", len(got))
	}
	room.Close()
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("expected the channel to close with the room")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("batches did not end when the room closed")
	}
}

func TestBatchesStopWhenTheContextIsCancelled(t *testing.T) {
	room, _ := newRoom(10, 10)
	sub := room.Subscribe(0)
	defer sub.Cancel()
	ctx, cancel := context.WithCancel(context.Background())
	out := hub.Batches(ctx, sub, 10*time.Millisecond)
	cancel()
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("unexpected batch")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not stop after cancellation")
	}
}

func TestReapRemovesOnlyOldClosedRooms(t *testing.T) {
	clock := newClock()
	h := hub.New(10, 10, clock.Now)
	old, recent, open := h.Room("old"), h.Room("recent"), h.Room("open")
	old.Close()
	clock.Advance(2 * time.Hour)
	recent.Close()
	_ = open

	if n := h.Reap(clock.Now().Add(-time.Hour)); n != 1 {
		t.Fatalf("Reap removed %d rooms, want 1", n)
	}
	if _, ok := h.Lookup("old"); ok {
		t.Error("the old closed room should be gone")
	}
	for _, id := range []string{"recent", "open"} {
		if _, ok := h.Lookup(id); !ok {
			t.Errorf("room %s must be kept", id)
		}
	}
}
