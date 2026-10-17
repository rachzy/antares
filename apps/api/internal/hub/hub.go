// Package hub fans job events out to WebSocket subscribers. Each job gets a Room that numbers its
// events and keeps a bounded ring of recent ones for replay. Rooms live in memory only; once a room
// is gone, history is rebuilt from stored milestones instead.
package hub

import (
	"encoding/json"
	"sync"
	"time"
)

// Event types: TypeState marks a milestone, TypeProgress a progress update within a stage,
// TypeResult the job's result and TypeError a failure or drop.
const (
	TypeState    = "state"
	TypeProgress = "progress"
	TypeResult   = "result"
	TypeError    = "error"
)

// Event is one message in a job's stream. Room.Append assigns Seq, which starts at 1 and grows by
// one per event, so a client resumes with ?since=<last seq>. Fraction is the stage's completion
// when Shaula reports one; Payload is the jobs.Result JSON of a TypeResult event.
type Event struct {
	Seq      uint64          `json:"seq"`
	Type     string          `json:"type"`
	Stage    string          `json:"stage,omitempty"`
	Message  string          `json:"message,omitempty"`
	Fraction *float64        `json:"fraction,omitempty"`
	At       time.Time       `json:"at"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// State returns a milestone event.
func State(stage, message string) Event {
	return Event{Type: TypeState, Stage: stage, Message: message}
}

// Progress returns a progress event; fraction may be nil.
func Progress(stage, message string, fraction *float64) Event {
	return Event{Type: TypeProgress, Stage: stage, Message: message, Fraction: fraction}
}

// Result returns the event carrying a finished job's result.
func Result(payload json.RawMessage) Event { return Event{Type: TypeResult, Payload: payload} }

// Failure returns an error event with a message for the client.
func Failure(message string) Event { return Event{Type: TypeError, Message: message} }

// Hub owns one Room per job id. It is safe for concurrent use.
type Hub struct {
	mu         sync.Mutex
	rooms      map[string]*Room
	ringSize   int
	connBuffer int
	clock      func() time.Time
}

// New returns a Hub whose rooms keep their last ringSize events and give each subscriber a buffer
// of connBuffer events; clock stamps events and measures presence.
func New(ringSize, connBuffer int, clock func() time.Time) *Hub {
	return &Hub{rooms: map[string]*Room{}, ringSize: ringSize, connBuffer: connBuffer, clock: clock}
}

// Room returns the room for id, creating it if needed.
func (h *Hub) Room(id string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[id]; ok {
		return r
	}
	r := newRoom(h.ringSize, h.connBuffer, h.clock)
	h.rooms[id] = r
	return r
}

// Lookup returns the room for id without creating one. No room means the job finished and was
// reaped, or the API restarted since it ran.
func (h *Hub) Lookup(id string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[id]
	return r, ok
}

// Reap forgets rooms closed before closedBefore and returns how many it removed; open rooms are
// kept however old.
func (h *Hub) Reap(closedBefore time.Time) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	removed := 0
	for id, r := range h.rooms {
		if at, ok := r.ClosedAt(); ok && at.Before(closedBefore) {
			delete(h.rooms, id)
			removed++
		}
	}
	return removed
}
