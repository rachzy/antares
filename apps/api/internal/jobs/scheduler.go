package jobs

import (
	"errors"
	"sync"
	"time"
)

// ErrQueueFull is returned by Enqueue when every slot is busy and MaxQueue jobs are waiting;
// ErrStopped is returned after Stop.
var (
	ErrQueueFull = errors.New("the queue is full")
	ErrStopped   = errors.New("the scheduler is stopped")
)

// SchedulerOptions configures a Scheduler. Present is called with the scheduler's lock held and
// must not call back into it; the other callbacks run without the lock, Run on its own goroutine.
type SchedulerOptions struct {
	// MaxActive is how many jobs run at once.
	MaxActive int
	// MaxQueue is how many jobs may wait for a slot.
	MaxQueue int
	// Grace is passed to Present.
	Grace time.Duration
	// Present reports whether anyone has watched jobID within grace.
	Present func(jobID string, grace time.Duration) bool
	// Run executes the job; its slot is freed when Run returns.
	Run func(jobID string)
	// Dropped is called for a queued job removed because nobody was watching it.
	Dropped func(jobID string)
	// Position is called when a waiting job's 1-based queue position changes.
	Position func(jobID string, position int)
}

// Scheduler runs at most MaxActive jobs at once and queues the rest in FIFO order. Its state is in
// memory only. It is safe for concurrent use.
type Scheduler struct {
	opt SchedulerOptions

	mu      sync.Mutex
	active  int
	queue   []string
	lastPos map[string]int
	running map[string]bool
	stopped bool
	wg      sync.WaitGroup
}

type dispatch struct {
	start     []string
	drop      []string
	positions map[string]int
}

// NewScheduler returns a Scheduler with no jobs.
func NewScheduler(opt SchedulerOptions) *Scheduler {
	return &Scheduler{opt: opt, lastPos: map[string]int{}, running: map[string]bool{}}
}

// Enqueue queues jobID and starts jobs while slots are free. It returns the job's 1-based queue
// position, or 0 if it started at once; Position is not called for jobID itself, since the caller
// gets the position here. The job must already count as present (see hub.Room.Touch) or promotion
// drops it.
func (s *Scheduler) Enqueue(jobID string) (int, error) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return 0, ErrStopped
	}
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

// Sweep drops queued jobs nobody has watched within Grace and then promotes as usual. The janitor
// calls it periodically, so an abandoned job does not hold its queue place until a slot frees.
func (s *Scheduler) Sweep() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
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

// Position returns jobID's 1-based queue position and whether it is running; (0, false) means the
// scheduler does not know the job.
func (s *Scheduler) Position(jobID string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[jobID] {
		return 0, true
	}
	return s.positionLocked(jobID), false
}

// Stats returns the number of running and waiting jobs.
func (s *Scheduler) Stats() (active, waiting int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active, len(s.queue)
}

// Stop stops starting jobs: Enqueue then fails with ErrStopped, Sweep does nothing and queued jobs
// stay queued, to be failed by Store.FailInterrupted on the next start. Running jobs are not
// interrupted; Wait waits for them.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
}

// Wait blocks until every started job's Run has returned. Call it after Stop, otherwise jobs may
// keep starting.
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
	if s.stopped {
		return d
	}
	for s.active < s.opt.MaxActive && len(s.queue) > 0 {
		id := s.queue[0]
		s.queue = s.queue[1:]
		delete(s.lastPos, id)
		// Promote only jobs someone is watching: an abandoned job would spend a scarce slot on a
		// result nobody reads. Grace keeps polling clients, which are only seen at each poll,
		// counted as present.
		if !s.opt.Present(id, s.opt.Grace) {
			d.drop = append(d.drop, id)
			continue
		}
		s.active++
		s.running[id] = true
		// Adding under the lock, while not stopped, means no Add can follow Stop, so Wait after
		// Stop never races with one.
		s.wg.Add(1)
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

// dispatch runs the callbacks outside the lock, since they write to rooms and the database. A
// finished job frees its slot and promotes the next one.
func (s *Scheduler) dispatch(d dispatch) {
	for _, id := range d.drop {
		s.opt.Dropped(id)
	}
	for id, position := range d.positions {
		s.opt.Position(id, position)
	}
	for _, id := range d.start {
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
