package hub

import (
	"encoding/json"
	"sync"
	"time"
)

const (
	TypeState    = "state"
	TypeProgress = "progress"
	TypeResult   = "result"
	TypeError    = "error"
)

type Event struct {
	Seq      uint64          `json:"seq"`
	Type     string          `json:"type"`
	Stage    string          `json:"stage,omitempty"`
	Message  string          `json:"message,omitempty"`
	Fraction *float64        `json:"fraction,omitempty"`
	At       time.Time       `json:"at"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

func State(stage, message string) Event {
	return Event{Type: TypeState, Stage: stage, Message: message}
}

func Progress(stage, message string, fraction *float64) Event {
	return Event{Type: TypeProgress, Stage: stage, Message: message, Fraction: fraction}
}

func Result(payload json.RawMessage) Event { return Event{Type: TypeResult, Payload: payload} }

func Failure(message string) Event { return Event{Type: TypeError, Message: message} }

type Hub struct {
	mu         sync.Mutex
	rooms      map[string]*Room
	ringSize   int
	connBuffer int
	clock      func() time.Time
}

func New(ringSize, connBuffer int, clock func() time.Time) *Hub {
	return &Hub{rooms: map[string]*Room{}, ringSize: ringSize, connBuffer: connBuffer, clock: clock}
}

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

func (h *Hub) Lookup(id string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[id]
	return r, ok
}

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
