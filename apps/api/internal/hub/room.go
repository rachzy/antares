package hub

import (
	"sync"
	"time"
)

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
	for sub := range r.subs {
		select {
		case sub.ch <- ev:
		default:
			r.dropLocked(sub)
		}
	}
	return ev
}

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

func (r *Room) Closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

func (r *Room) ClosedAt() (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closedAt, r.closed
}

func (r *Room) LastSeq() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nextSeq - 1
}

func (r *Room) Touch() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastSeen = r.clock()
}

func (r *Room) Present(grace time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conns > 0 || r.clock().Sub(r.lastSeen) <= grace
}

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

func (s *Subscription) Dropped() bool {
	s.room.mu.Lock()
	defer s.room.mu.Unlock()
	return s.dropped
}

func (s *Subscription) Cancel() {
	s.room.mu.Lock()
	defer s.room.mu.Unlock()
	s.room.detachLocked(s)
	if !s.chClosed {
		close(s.ch)
		s.chClosed = true
	}
}
