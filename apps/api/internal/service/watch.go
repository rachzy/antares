package service

import (
	"context"
	"encoding/json"
	"time"

	"antares/api/internal/hub"
	"antares/api/internal/jobs"
)

type Watch struct {
	Job     jobs.Job
	Sub     *hub.Subscription
	History []hub.Event
}

type MilestoneView struct {
	Seq     int       `json:"seq"`
	Stage   string    `json:"stage"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

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
		view.LastSeq = lastMilestone + 1
	} else {
		view.LastSeq = lastMilestone
	}
	if job.State == jobs.StateQueued {
		view.Position, _ = s.Sched.Position(job.ID)
	}
	return view, nil
}
