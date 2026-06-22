package scheduler

import (
	"sync"
	"testing"
	"time"
)

// fakeClock is a Clock whose ticks and "now" are driven by the test instead of
// the wall clock, so the scheduler loop can be exercised deterministically.
type fakeClock struct {
	ticks chan time.Time

	mu      sync.Mutex
	now     time.Time
	stopped bool
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{ticks: make(chan time.Time, 1), now: now}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Ticks() <-chan time.Time { return c.ticks }

func (c *fakeClock) Stop() {
	c.mu.Lock()
	c.stopped = true
	c.mu.Unlock()
}

func (c *fakeClock) isStopped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopped
}

// fire advances the clock to t and delivers one tick.
func (c *fakeClock) fire(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
	c.ticks <- t
}

func TestSchedulerCallsTickWithClockNow(t *testing.T) {
	clock := newFakeClock(time.Unix(0, 0))
	got := make(chan time.Time, 1)
	s := NewScheduler(clock, func(now time.Time) { got <- now })
	s.Start()
	defer s.Stop()

	want := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	clock.fire(want)

	select {
	case now := <-got:
		if !now.Equal(want) {
			t.Errorf("tick now = %v, want %v", now, want)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler did not call tick after a clock tick")
	}
}

func TestSchedulerStopReleasesClock(t *testing.T) {
	clock := newFakeClock(time.Now())
	s := NewScheduler(clock, func(time.Time) {})
	s.Start()
	s.Stop()

	// After Stop the loop exits and releases the clock via the deferred Stop.
	deadline := time.Now().Add(time.Second)
	for !clock.isStopped() {
		if time.Now().After(deadline) {
			t.Fatal("clock was not stopped after scheduler Stop")
		}
		time.Sleep(time.Millisecond)
	}
}
