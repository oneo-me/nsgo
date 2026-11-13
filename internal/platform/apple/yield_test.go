//go:build darwin

package apple

import (
	"testing"
	"time"
)

func TestTheYieldTimerFollowsTheFrames(t *testing.T) {
	const fast, slow = time.Second / 120, time.Second / 60
	var c yieldClock
	now := time.Unix(1000, 0)
	// next gives c a frame gap after the last and reports whether the
	// timer runs after it.
	next := func(gap time.Duration) bool {
		now = now.Add(gap)
		return c.frame(now)
	}

	for i := range 120 {
		if next(fast) {
			t.Fatalf("the timer runs after frame %d at 120 Hz", i)
		}
	}
	for i := range 60 {
		if !next(slow) {
			t.Fatalf("no timer runs after frame %d at 60 Hz", i)
		}
	}
	if !next(fast) {
		t.Error("the timer stops with the first close frame")
	}
	for range 24 {
		next(fast)
	}
	if next(fast) {
		t.Error("the timer still runs 200 ms after the last slow frame")
	}

	if c.idle(now.Add(slow)) {
		t.Error("the frames count as stopped one frame after the last")
	}
	if !c.idle(now.Add(yieldLinger)) {
		t.Error("the frames do not count as stopped after yieldLinger")
	}
	if next(time.Second) {
		t.Error("the timer runs after the first frame that follows a pause")
	}
}

// Two windows tick in one refresh: a close frame and a far one, in turn.
func TestTheYieldTimerStaysForTwoWindows(t *testing.T) {
	const refresh, apart = time.Second / 60, 100 * time.Microsecond
	var c yieldClock
	now := time.Unix(1000, 0)
	next := func(gap time.Duration) bool {
		now = now.Add(gap)
		return c.frame(now)
	}
	if next(time.Hour) || next(apart) {
		t.Fatal("the timer runs after the first refresh")
	}
	for i := range 60 {
		if !next(refresh-apart) || !next(apart) {
			t.Fatalf("no timer runs after refresh %d", i+1)
		}
	}
}
