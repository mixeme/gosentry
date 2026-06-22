package scheduler

import "time"

// Clock supplies the scheduler with the current time and a stream of ticks.
// Hiding both behind an interface lets tests drive the loop deterministically —
// firing ticks and controlling "now" — instead of waiting on the wall clock.
// Production uses RealClock.
type Clock interface {
	// Now returns the current time. It is the value passed to the tick callback
	// on each tick, so a fake can make due-evaluation deterministic.
	Now() time.Time
	// Ticks returns a channel that delivers a value on every scheduler tick. The
	// scheduler reads it for the lifetime of the loop.
	Ticks() <-chan time.Time
	// Stop releases the resources backing Ticks. The scheduler calls it once when
	// the loop exits.
	Stop()
}

// RealClock is the production Clock: wall-clock time and a one-second ticker.
//
// A one-second cadence is accurate enough for cron-style desktop automation —
// five-field cron expressions have minute precision, while @every values may be
// shorter for testing and lightweight local tasks — and it keeps a single timer
// instead of one per job.
type RealClock struct {
	ticker *time.Ticker
}

// NewRealClock returns a real clock. The underlying ticker is created lazily on
// the first Ticks call so a clock that is never started leaks nothing.
func NewRealClock() *RealClock {
	return &RealClock{}
}

// Now returns the wall-clock time.
func (c *RealClock) Now() time.Time {
	return time.Now()
}

// Ticks starts (once) and returns the one-second ticker channel.
func (c *RealClock) Ticks() <-chan time.Time {
	if c.ticker == nil {
		c.ticker = time.NewTicker(time.Second)
	}
	return c.ticker.C
}

// Stop halts the ticker if it was ever started.
func (c *RealClock) Stop() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
}
