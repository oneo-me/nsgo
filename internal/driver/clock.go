package driver

import "time"

// Monitor reads what a surface's frames cost: engine.PerfMonitor implements
// it, and a backend installs one only when the app runs with --nsgo-fps.
//
// A Clock reports to it, so a backend never times a frame itself.
type Monitor interface {
	// RecordWork records the main thread's work on a frame, and whether the
	// frame changed what the screen shows.
	RecordWork(cost time.Duration, dirty bool)
	// Vsync records a display refresh at at, interval after the previous.
	Vsync(at time.Time, interval time.Duration)
}

// Clock is a surface's frame clock: the animator's time base, the frames the
// display shows, and the monitor that reads them.
//
// A backend gives it the time its display reports and a way to wake the
// surface, and keeps no clock of its own: Step moves the animation graph,
// BeginFrame and EndFrame time the work between two refreshes, and Watched
// says whether the display has to keep running while nothing animates.
type Clock struct {
	anim  *Animator
	epoch time.Time
	// monitor, if any, reads the frames. While one watches, the clock keeps
	// running even when nothing animates, so the readings cover a still app.
	monitor Monitor
	// frame is when the frame in progress began, last when the display last
	// refreshed, and period the interval between refreshes.
	frame, last time.Time
	period      time.Duration
}

// NewClock returns the clock of animator a's surface.
func NewClock(a *Animator) *Clock {
	return NewClockAt(a, time.Now())
}

// NewClockAt returns the clock of animator a's surface counting from epoch,
// as a backend that already keeps one does.
func NewClockAt(a *Animator, epoch time.Time) *Clock {
	return &Clock{anim: a, epoch: epoch, period: time.Second / 60}
}

// Watch installs the monitor that reads the clock's frames, as a surface
// with an option to measure does at its start.
func (c *Clock) Watch(m Monitor) { c.monitor = m }

// Watched reports whether a monitor reads the clock, which keeps the display
// running when nothing animates.
func (c *Clock) Watched() bool { return c.monitor != nil }

// Elapsed is the animator's time: since the clock started.
func (c *Clock) Elapsed() time.Duration { return time.Since(c.epoch) }

// Step moves the animation graph to now, an elapsed time.
func (c *Clock) Step(now time.Duration) { c.anim.Step(now) }

// BeginFrame marks the start of the work a frame costs.
func (c *Clock) BeginFrame(now time.Time) { c.frame = now }

// EndFrame closes the frame the work since BeginFrame belongs to, at the
// refresh now, dirty saying whether the work changed what the screen shows.
//
// period is the display's own period where the platform knows it, and 0
// where the clock has to measure the interval between refreshes.
func (c *Clock) EndFrame(now time.Time, period time.Duration, dirty bool) {
	if c.monitor == nil {
		return
	}
	if period <= 0 {
		period = c.period
		if !c.last.IsZero() {
			// A pause (the display stopped, the app slept) is not a slow
			// display, so it does not move the period.
			if gap := now.Sub(c.last); gap < 100*time.Millisecond {
				period = gap
			}
		}
	}
	c.last, c.period = now, period
	c.monitor.RecordWork(now.Sub(c.frame), dirty)
	c.monitor.Vsync(now, period)
}
