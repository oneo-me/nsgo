//go:build darwin

package macos

import (
	"testing"
	"time"
)

// A frame that still has time runs. Skipping is for frames that cannot
// land, not a throttle: skipping these would stutter a screen that is
// keeping up.
func TestFrameRunsWhenItHasTime(t *testing.T) {
	if shouldSkip(minFrameWork+0.001, 0) {
		t.Fatal("a frame with time left was skipped")
	}
}

// A frame with less than minFrameWork to go cannot finish before the
// composite, so it is dropped rather than drawn late.
func TestFrameSkipsWhenOutOfTime(t *testing.T) {
	if !shouldSkip(minFrameWork/2, 0) {
		t.Fatal("a frame past its budget ran anyway: it lands after the composite")
	}
}

// Skipping must not become a stall. At the bound the frame runs even with
// no time left, or a busy screen would stop drawing altogether.
func TestSkipsAreBounded(t *testing.T) {
	if !shouldSkip(0, maxFrameSkips-1) {
		t.Fatal("a frame should still skip below the bound")
	}
	if shouldSkip(0, maxFrameSkips) {
		t.Fatalf("skipping never stopped at %d: the app would stop drawing", maxFrameSkips)
	}
}

// No cap asks for every frame; a cap holds the frame back until its
// interval has passed, and lets the one at the boundary through.
func TestFrameCapHoldsFramesBack(t *testing.T) {
	if !frameDue(0, 0) {
		t.Fatal("without a cap every tick must be a frame")
	}
	sixteen := time.Second / 60
	if frameDue(sixteen, sixteen/2) {
		t.Fatal("a frame ran half an interval early: the cap does nothing")
	}
	if !frameDue(sixteen, sixteen) {
		t.Fatal("the frame at the interval boundary must run")
	}
}
