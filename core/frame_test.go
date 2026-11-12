package core

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A frame callback runs once, with the frame's time, and not again.
func TestRequestAnimationFrameRunsOnce(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	h.clock.Advance(5 * time.Millisecond)
	var runs int
	var at time.Time
	h.root.RequestAnimationFrame(func(when time.Time) { runs++; at = when })

	h.frame()
	if runs != 1 {
		t.Fatalf("the callback ran %d times, want once", runs)
	}
	if want := h.clock.Now(); !at.Equal(want) {
		t.Fatalf("frame time = %v, want the clock's %v", at, want)
	}
	h.clock.Advance(driver.FrameInterval)
	h.frame()
	if runs != 1 {
		t.Fatalf("an uncancelled callback ran again: %d times", runs)
	}
}

// stop cancels the wait, and so does disposing the owner that registered
// it: a screen that goes takes its frame callbacks with it.
func TestRequestAnimationFrameStops(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	runs := 0
	stop := h.root.RequestAnimationFrame(func(time.Time) { runs++ })
	stop()
	h.frame()
	if runs != 0 {
		t.Fatalf("a stopped callback ran %d times", runs)
	}

	child := h.root.Child()
	child.RequestAnimationFrame(func(time.Time) { runs++ })
	child.Dispose()
	h.frame()
	if runs != 0 {
		t.Fatalf("a disposed owner's callback ran %d times", runs)
	}
}

// The host is asked for one frame at a time, however many callbacks wait,
// and again only when one of them waits for the next.
func TestRequestAnimationFrameAsksOncePerFrame(t *testing.T) {
	requests := 0
	root := NewRoot(RootOptions{
		Post:         func(func()) {},
		Clock:        NewManualClock(time.Unix(0, 0)),
		RequestFrame: func() { requests++ },
	})
	root.RequestAnimationFrame(func(time.Time) {})
	root.RequestAnimationFrame(func(time.Time) {})
	if requests != 1 {
		t.Fatalf("the host was asked %d times for one frame, want once", requests)
	}
	root.RunFrame()
	if requests != 1 {
		t.Fatalf("the host was asked again with nothing waiting: %d", requests)
	}

	var loop func(time.Time)
	loop = func(time.Time) { root.RequestAnimationFrame(loop) }
	root.RequestAnimationFrame(loop)
	if requests != 2 {
		t.Fatalf("registering a callback did not ask for a frame: %d requests", requests)
	}
	root.RunFrame()
	if requests != 3 {
		t.Fatalf("a callback waiting again did not ask for the next frame: %d requests", requests)
	}
}

// A callback that registers another during a frame waits for the next
// one, and a callback stopped during a frame does not come back in it.
func TestRequestAnimationFrameChangesDuringAFrame(t *testing.T) {
	root := NewRoot(RootOptions{
		Post:         func(func()) {},
		Clock:        NewManualClock(time.Unix(0, 0)),
		RequestFrame: func() {},
	})
	var later, stopped int
	var stop func()
	root.RequestAnimationFrame(func(time.Time) {
		root.RequestAnimationFrame(func(time.Time) { later++ })
		stop()
	})
	stop = root.RequestAnimationFrame(func(time.Time) { stopped++ })

	root.RunFrame()
	if later != 0 {
		t.Fatalf("a callback registered during a frame ran in it: %d times", later)
	}
	root.RunFrame()
	if later != 1 {
		t.Fatalf("a callback registered during a frame ran %d times, want once", later)
	}
	if stopped > 1 {
		t.Fatalf("a callback stopped during a frame ran %d times: it came back", stopped)
	}
}
