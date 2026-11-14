package driver

import (
	"testing"
	"time"
)

// recorder is a Monitor that keeps what a clock reports.
type recorder struct {
	costs, periods []time.Duration
	dirty          []bool
	at             []time.Time
}

func (r *recorder) RecordWork(cost time.Duration, dirty bool) {
	r.costs = append(r.costs, cost)
	r.dirty = append(r.dirty, dirty)
}

func (r *recorder) Vsync(at time.Time, interval time.Duration) {
	r.at = append(r.at, at)
	r.periods = append(r.periods, interval)
}

func TestClockTimesFramesForItsMonitor(t *testing.T) {
	c := NewClock(NewAnimator(func(Event) {}))
	if c.Watched() {
		t.Fatal("a new clock is watched")
	}
	// Without a monitor a frame is not recorded.
	start := time.Now()
	c.BeginFrame(start)
	c.EndFrame(start.Add(2*time.Millisecond), time.Second/60, true)

	r := &recorder{}
	c.Watch(r)
	if !c.Watched() {
		t.Fatal("a clock with a monitor is not watched")
	}
	// The display's own period, where the platform knows it.
	c.BeginFrame(start)
	c.EndFrame(start.Add(3*time.Millisecond), time.Second/60, true)
	// The interval between refreshes, where it does not.
	c.BeginFrame(start.Add(16 * time.Millisecond))
	c.EndFrame(start.Add(19*time.Millisecond), 0, false)
	// A pause is not a slow display, so the period stays.
	c.BeginFrame(start.Add(200 * time.Millisecond))
	c.EndFrame(start.Add(201*time.Millisecond), 0, false)

	if len(r.costs) != 3 {
		t.Fatalf("recorded %d frames, want 3", len(r.costs))
	}
	if r.costs[0] != 3*time.Millisecond || !r.dirty[0] {
		t.Fatalf("first frame cost %v dirty %v, want 3ms and dirty", r.costs[0], r.dirty[0])
	}
	if r.periods[0] != time.Second/60 {
		t.Fatalf("first period %v, want the display's", r.periods[0])
	}
	if r.costs[1] != 3*time.Millisecond || r.dirty[1] {
		t.Fatalf("second frame cost %v dirty %v, want 3ms and clean", r.costs[1], r.dirty[1])
	}
	if r.periods[1] != 16*time.Millisecond {
		t.Fatalf("second period %v, want the measured 16ms", r.periods[1])
	}
	if r.periods[2] != 16*time.Millisecond {
		t.Fatalf("period after a pause %v, want the last 16ms", r.periods[2])
	}
}

func TestClockCountsFromItsEpoch(t *testing.T) {
	epoch := time.Now().Add(-time.Second)
	c := NewClockAt(NewAnimator(func(Event) {}), epoch)
	if got := c.Elapsed(); got < time.Second {
		t.Fatalf("elapsed %v from an epoch a second back", got)
	}
}

func TestClockStepsTheAnimatorAndElapses(t *testing.T) {
	c := NewClock(NewAnimator(func(Event) {}))
	start := c.Elapsed()
	time.Sleep(2 * time.Millisecond)
	if now := c.Elapsed(); now <= start {
		t.Fatalf("elapsed %v did not move past %v", now, start)
	}
	var moved bool
	c.anim.report = func(Event) { moved = true }
	c.anim.Apply([]AnimationOp{{Kind: AnimCreateValue, Node: 1},
		{Kind: AnimStart, ID: 1, Animation: &Animation{Kind: AnimationTiming, Value: 1, To: 1, Timing: TimingConfig{Duration: 10 * time.Millisecond}}}})
	c.Step(0)
	c.Step(20 * time.Millisecond)
	if !moved {
		t.Fatal("stepping the clock did not step the animator")
	}
}
