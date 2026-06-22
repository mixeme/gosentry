package scheduler

import (
	"context"
	"time"
)

// Scheduler is a thin timing loop. It owns no job or runtime state: on every
// clock tick it calls the injected tick function with the current time, and that
// function — the application service's RunDue — decides what, if anything, to
// run. Keeping all state and mutation in the service makes the service the sole
// writer (resolving the old shared-*[]Job data race) and reduces the scheduler
// to a loop that is trivially testable with a fake Clock.
type Scheduler struct {
	clock Clock
	tick  func(now time.Time)

	ctx    context.Context
	cancel context.CancelFunc
}

// NewScheduler builds a scheduler that calls tick on every Clock tick. The clock
// is injected so tests can drive the loop without the wall clock.
func NewScheduler(clock Clock, tick func(now time.Time)) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		clock:  clock,
		tick:   tick,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start launches the loop on its own goroutine and returns immediately.
func (s *Scheduler) Start() {
	go func() {
		ticks := s.clock.Ticks()
		defer s.clock.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticks:
				// Pass the clock's notion of "now" rather than the tick value so a
				// fake clock can control due-evaluation precisely.
				s.tick(s.clock.Now())
			}
		}
	}()
}

// Stop ends the loop. A tick already in progress finishes; no further ticks are
// delivered.
func (s *Scheduler) Stop() {
	s.cancel()
}
