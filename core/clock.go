package core

import (
	"slices"
	"sync"
	"time"
)

// Clock is the time a graph's timers run on. Go cannot hook time
// globally, so tests inject a ManualClock where an app would use the
// system's.
type Clock interface {
	Now() time.Time
	// AfterFunc calls f on its own goroutine after d, unless stop is
	// called first; stop reports whether it prevented the call.
	AfterFunc(d time.Duration, f func()) (stop func() bool)
}

// SystemClock is the system's time.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

func (SystemClock) AfterFunc(d time.Duration, f func()) func() bool {
	return time.AfterFunc(d, f).Stop
}

// ManualClock moves only when Advance is called. It is safe for
// concurrent use.
type ManualClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*manualTimer
	// wall is how far the wall time is set from now (SetWallTime).
	wall time.Duration
}

type manualTimer struct {
	at      time.Time
	f       func()
	stopped bool
}

// NewManualClock returns a clock reading start.
func NewManualClock(start time.Time) *ManualClock { return &ManualClock{now: start} }

func (c *ManualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// SetWallTime sets the wall time the app reads (Scope.WallTime, a TSX
// app's Date) to t, moving no timer: the clock's own time goes on.
func (c *ManualClock) SetWallTime(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.wall = t.Sub(c.now)
}

// WallTime is the wall time the app reads: Now, unless SetWallTime set
// it apart.
func (c *ManualClock) WallTime() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now.Add(c.wall)
}

func (c *ManualClock) AfterFunc(d time.Duration, f func()) func() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &manualTimer{at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		was := t.stopped
		t.stopped = true
		return !was
	}
}

// Jump moves the clock ahead by d at once, as a machine waking from
// sleep: each timer due by then runs once, in order, at the end; timers
// those set run on later moves.
func (c *ManualClock) Jump(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*manualTimer
	for _, t := range c.timers {
		if !t.stopped && !t.at.After(c.now) {
			due = append(due, t)
		}
	}
	slices.SortStableFunc(due, func(a, b *manualTimer) int { return a.at.Compare(b.at) })
	for _, t := range due {
		t.stopped = true
	}
	c.timers = slicesDeleteStopped(c.timers)
	c.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

// Advance moves the clock by d, calling the timers that come due on the
// way in order, each at its own time.
func (c *ManualClock) Advance(d time.Duration) {
	c.mu.Lock()
	end := c.now.Add(d)
	c.mu.Unlock()
	for {
		c.mu.Lock()
		var next *manualTimer
		for _, t := range c.timers {
			if !t.stopped && !t.at.After(end) && (next == nil || t.at.Before(next.at)) {
				next = t
			}
		}
		if next == nil {
			c.now = end
			c.timers = slicesDeleteStopped(c.timers)
			c.mu.Unlock()
			return
		}
		next.stopped = true
		c.now = next.at
		c.mu.Unlock()
		next.f()
	}
}

func slicesDeleteStopped(ts []*manualTimer) []*manualTimer {
	out := ts[:0]
	for _, t := range ts {
		if !t.stopped {
			out = append(out, t)
		}
	}
	return out
}

// Now returns the time on the graph's clock: the system's, or a test's
// manual clock.
func (cx *Scope) Now() time.Time { return cx.rt.clock.Now() }

// WallTime returns the date and time the app shows: Now, unless a test's
// clock set its wall time apart (ManualClock.SetWallTime).
func (cx *Scope) WallTime() time.Time {
	if w, ok := cx.rt.clock.(interface{ WallTime() time.Time }); ok {
		return w.WallTime()
	}
	return cx.rt.clock.Now()
}
