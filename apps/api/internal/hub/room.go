package hub

import (
	"sync"
	"time"
)

// Room is one job's event stream: a ring of recent events plus live subscribers. It is safe for
// concurrent use. Presence, whether anyone is watching the job, is derived from open subscriptions
// and the time of the last Touch or disconnect.
type Room struct {
	mu       sync.Mutex
	clock    func() time.Time
	connBuf  int
	ring     []Event
	start    int
	count    int
	nextSeq  uint64
	subs     map[*Subscription]struct{}
	conns    int
	lastSeen time.Time
	closed   bool
	closedAt time.Time
}

func newRoom(ringSize, connBuf int, clock func() time.Time) *Room {
	return &Room{
		clock:    clock,
		connBuf:  connBuf,
		ring:     make([]Event, ringSize),
		nextSeq:  1,
		subs:     map[*Subscription]struct{}{},
		lastSeen: clock(),
	}
}

// Append stamps ev with the next Seq (and At, if unset), keeps it in the ring and offers it to
// every subscriber, returning the stamped event. After Close it does nothing and returns the zero
// Event.
func (r *Room) Append(ev Event) Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return Event{}
	}
	ev.Seq = r.nextSeq
	r.nextSeq++
	if ev.At.IsZero() {
		ev.At = r.clock()
	}
	if r.count < len(r.ring) {
		r.ring[(r.start+r.count)%len(r.ring)] = ev
		r.count++
	} else {
		r.ring[r.start] = ev
		r.start = (r.start + 1) % len(r.ring)
	}
	// Never block the job on a slow client: drop a subscriber whose buffer is full. It sees C close
	// with Dropped true and can reconnect with since to resume from the ring.
	for sub := range r.subs {
		select {
		case sub.ch <- ev:
		default:
			r.dropLocked(sub)
		}
	}
	return ev
}

// Close ends the stream: subscribers still read what is buffered, then C closes, and later Appends
// are ignored. It is idempotent.
func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.closedAt = r.clock()
	for sub := range r.subs {
		if !sub.chClosed {
			close(sub.ch)
			sub.chClosed = true
		}
	}
}

// Closed reports whether Close was called.
func (r *Room) Closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// ClosedAt returns when the room was closed and whether it is.
func (r *Room) ClosedAt() (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closedAt, r.closed
}

// LastSeq returns the Seq of the newest event, or 0 if there is none.
func (r *Room) LastSeq() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nextSeq - 1
}

// Touch records that someone is watching now, such as a status poll, starting a new grace window
// for Present.
func (r *Room) Touch() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastSeen = r.clock()
}

// Present reports whether anyone is watching: a subscriber is connected, or the last Touch,
// disconnect or the room's creation was at most grace ago. The grace covers clients that poll
// instead of holding a WebSocket, and brief reconnects.
func (r *Room) Present(grace time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conns > 0 || r.clock().Sub(r.lastSeen) <= grace
}

// Subscribe returns a subscription whose Replay holds the buffered events after since and whose C
// then delivers live ones. The snapshot and the registration happen under one lock, so no event
// falls between Replay and C or appears in both. On a closed room C is already closed. The caller
// must Cancel the subscription.
func (r *Room) Subscribe(since uint64) *Subscription {
	r.mu.Lock()
	defer r.mu.Unlock()
	sub := &Subscription{room: r, ch: make(chan Event, r.connBuf)}
	sub.C = sub.ch
	sub.Replay, sub.Truncated = r.snapshotLocked(since)
	r.conns++
	r.subs[sub] = struct{}{}
	if r.closed {
		close(sub.ch)
		sub.chClosed = true
	}
	return sub
}

func (r *Room) snapshotLocked(since uint64) ([]Event, bool) {
	out := make([]Event, 0, r.count)
	for i := 0; i < r.count; i++ {
		if ev := r.ring[(r.start+i)%len(r.ring)]; ev.Seq > since {
			out = append(out, ev)
		}
	}
	truncated := r.count > 0 && r.ring[r.start].Seq > since+1
	return out, truncated
}

func (r *Room) detachLocked(sub *Subscription) {
	if sub.detached {
		return
	}
	sub.detached = true
	delete(r.subs, sub)
	r.conns--
	r.lastSeen = r.clock()
}

func (r *Room) dropLocked(sub *Subscription) {
	sub.dropped = true
	r.detachLocked(sub)
	if !sub.chClosed {
		close(sub.ch)
		sub.chClosed = true
	}
}

// Subscription is one subscriber of a Room. Replay holds the buffered events newer than since,
// oldest first; Truncated reports that some of the events after since have already left the ring. C
// delivers the events after Replay and is closed when the room closes, when the subscriber is
// dropped for falling behind, or on Cancel.
type Subscription struct {
	Replay    []Event
	Truncated bool
	C         <-chan Event

	room     *Room
	ch       chan Event
	detached bool
	chClosed bool
	dropped  bool
}

// Dropped reports whether C was closed because the subscriber's buffer overflowed.
func (s *Subscription) Dropped() bool {
	s.room.mu.Lock()
	defer s.room.mu.Unlock()
	return s.dropped
}

// Cancel detaches the subscription and closes C. It is idempotent and counts as a disconnect for
// presence.
func (s *Subscription) Cancel() {
	s.room.mu.Lock()
	defer s.room.mu.Unlock()
	s.room.detachLocked(s)
	if !s.chClosed {
		close(s.ch)
		s.chClosed = true
	}
}
