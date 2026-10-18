package service

import (
	"context"
	"encoding/json"
	"time"

	"antares/api/internal/hub"
	"antares/api/internal/jobs"
)

// Watch is an opened event stream. Sub is set while the job's room is in memory, and the caller
// must Cancel it; otherwise History holds the events rebuilt from the database, ending with the
// terminal event if the job finished.
type Watch struct {
	Job     jobs.Job
	Sub     *hub.Subscription
	History []hub.Event
}

// MilestoneView is a milestone as returned by the status endpoint.
type MilestoneView struct {
	Seq     int       `json:"seq"`
	Stage   string    `json:"stage"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// SubscriptionView is a subscription's status, for clients polling instead of streaming. LastSeq is
// the since to pass when switching to the WebSocket.
type SubscriptionView struct {
	Kind       string          `json:"kind"`
	State      string          `json:"state"`
	Position   int             `json:"position,omitempty"`
	Error      string          `json:"error,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Milestones []MilestoneView `json:"milestones"`
	LastSeq    uint64          `json:"last_seq"`
}

func (s *Service) jobByToken(ctx context.Context, token string) (jobs.Job, error) {
	if !jobs.ValidToken(token) {
		return jobs.Job{}, errf(KindNotFound, "unknown or expired subscription")
	}
	job, ok, err := s.Jobs.ByToken(ctx, token, s.Clock())
	if err != nil {
		return jobs.Job{}, errf(KindInternal, "look up subscription: %v", err)
	}
	if !ok {
		return jobs.Job{}, errf(KindNotFound, "unknown or expired subscription")
	}
	return job, nil
}

// Watch opens the stream of a token's job, starting after event since.
func (s *Service) Watch(ctx context.Context, token string, since uint64) (*Watch, error) {
	job, err := s.jobByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if room, ok := s.Hub.Lookup(job.ID); ok {
		return &Watch{Job: job, Sub: room.Subscribe(since)}, nil
	}
	history, err := s.history(ctx, job, since)
	if err != nil {
		return nil, err
	}
	return &Watch{Job: job, History: history}, nil
}

// history rebuilds the stream once the room is gone. Only milestones were stored, not progress
// events, so the terminal event is numbered max(since, last milestone)+1: newer than anything the
// client has seen, so it is never filtered out as already delivered.
func (s *Service) history(ctx context.Context, job jobs.Job, since uint64) ([]hub.Event, error) {
	milestones, err := s.Jobs.Milestones(ctx, job.ID)
	if err != nil {
		return nil, errf(KindInternal, "load milestones: %v", err)
	}
	events := []hub.Event{}
	var last uint64
	for _, m := range milestones {
		last = uint64(m.Seq)
		if last > since {
			events = append(events, hub.Event{Seq: last, Type: hub.TypeState, Stage: m.Stage, Message: m.Message, At: m.At})
		}
	}
	if final, ok := terminalEvent(job, max(last, since)+1); ok {
		events = append(events, final)
	}
	return events, nil
}

func terminalEvent(job jobs.Job, seq uint64) (hub.Event, bool) {
	var at time.Time
	if job.FinishedAt != nil {
		at = *job.FinishedAt
	}
	switch job.State {
	case jobs.StateDone:
		return hub.Event{Seq: seq, Type: hub.TypeResult, Payload: job.Result, At: at}, true
	case jobs.StateFailed, jobs.StateDropped:
		return hub.Event{Seq: seq, Type: hub.TypeError, Message: job.Error, At: at}, true
	}
	return hub.Event{}, false
}

// Status returns a token's job state, milestones, queue position and LastSeq. Polling counts as
// presence: it touches the room, so a polling client keeps its queued job from being dropped.
func (s *Service) Status(ctx context.Context, token string) (SubscriptionView, error) {
	job, err := s.jobByToken(ctx, token)
	if err != nil {
		return SubscriptionView{}, err
	}
	if room, ok := s.Hub.Lookup(job.ID); ok {
		room.Touch()
	}
	milestones, err := s.Jobs.Milestones(ctx, job.ID)
	if err != nil {
		return SubscriptionView{}, errf(KindInternal, "load milestones: %v", err)
	}

	view := SubscriptionView{
		Kind: job.Kind, State: job.State, Error: job.Error, Result: job.Result,
		Milestones: make([]MilestoneView, 0, len(milestones)),
	}
	var lastMilestone uint64
	for _, m := range milestones {
		view.Milestones = append(view.Milestones, MilestoneView{Seq: m.Seq, Stage: m.Stage, Message: m.Message, At: m.At})
		lastMilestone = uint64(m.Seq)
	}

	if room, ok := s.Hub.Lookup(job.ID); ok {
		view.LastSeq = room.LastSeq()
	} else if _, terminal := terminalEvent(job, 0); terminal {
		// Match the number history gives the terminal event.
		view.LastSeq = lastMilestone + 1
	} else {
		view.LastSeq = lastMilestone
	}
	if job.State == jobs.StateQueued {
		view.Position, _ = s.Sched.Position(job.ID)
	}
	return view, nil
}
