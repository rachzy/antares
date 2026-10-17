package jobs

import (
	"errors"
	"sync"
	"time"
)

var ErrQueueFull = errors.New("the queue is full")

type SchedulerOptions struct {
	MaxActive int
	MaxQueue  int
	Grace     time.Duration
	Present   func(jobID string, grace time.Duration) bool
	Run       func(jobID string)
	Dropped   func(jobID string)
	Position  func(jobID string, position int)
}

type Scheduler struct {
	opt SchedulerOptions

	mu      sync.Mutex
	active  int
	queue   []string
	lastPos map[string]int
	running map[string]bool
	wg      sync.WaitGroup
}

type dispatch struct {
	start     []string
	drop      []string
	positions map[string]int
}

func NewScheduler(opt SchedulerOptions) *Scheduler {
	return &Scheduler{opt: opt, lastPos: map[string]int{}, running: map[string]bool{}}
}

func (s *Scheduler) Enqueue(jobID string) (int, error) {
	s.mu.Lock()
	if s.active >= s.opt.MaxActive && len(s.queue) >= s.opt.MaxQueue {
		s.mu.Unlock()
		return 0, ErrQueueFull
	}
	s.queue = append(s.queue, jobID)
	d := s.pumpLocked()
	delete(d.positions, jobID)
	position := s.positionLocked(jobID)
	s.mu.Unlock()

	s.dispatch(d)
	return position, nil
}

func (s *Scheduler) Sweep() {
	s.mu.Lock()
	var absent []string
	kept := make([]string, 0, len(s.queue))
	for _, id := range s.queue {
		if s.opt.Present(id, s.opt.Grace) {
			kept = append(kept, id)
			continue
		}
		absent = append(absent, id)
		delete(s.lastPos, id)
	}
	s.queue = kept
	d := s.pumpLocked()
	d.drop = append(d.drop, absent...)
	s.mu.Unlock()

	s.dispatch(d)
}

func (s *Scheduler) Position(jobID string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[jobID] {
		return 0, true
	}
	return s.positionLocked(jobID), false
}

func (s *Scheduler) Stats() (active, waiting int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active, len(s.queue)
}

func (s *Scheduler) Wait() { s.wg.Wait() }

func (s *Scheduler) positionLocked(jobID string) int {
	for i, id := range s.queue {
		if id == jobID {
			return i + 1
		}
	}
	return 0
}

func (s *Scheduler) pumpLocked() dispatch {
	d := dispatch{positions: map[string]int{}}
	for s.active < s.opt.MaxActive && len(s.queue) > 0 {
		id := s.queue[0]
		s.queue = s.queue[1:]
		delete(s.lastPos, id)
		if !s.opt.Present(id, s.opt.Grace) {
			d.drop = append(d.drop, id)
			continue
		}
		s.active++
		s.running[id] = true
		d.start = append(d.start, id)
	}
	for i, id := range s.queue {
		if s.lastPos[id] != i+1 {
			s.lastPos[id] = i + 1
			d.positions[id] = i + 1
		}
	}
	return d
}

func (s *Scheduler) dispatch(d dispatch) {
	for _, id := range d.drop {
		s.opt.Dropped(id)
	}
	for id, position := range d.positions {
		s.opt.Position(id, position)
	}
	for _, id := range d.start {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.opt.Run(id)

			s.mu.Lock()
			s.active--
			delete(s.running, id)
			next := s.pumpLocked()
			s.mu.Unlock()
			s.dispatch(next)
		}()
	}
}
