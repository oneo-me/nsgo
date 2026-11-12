package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestExitsLeaveWithTheirMotion hides the Exits card, which stays while
// its exit plays and goes once it ends.
func TestExitsLeaveWithTheirMotion(t *testing.T) {
	headless(t)
	app := startDesktop(t, nsgotest.ManualFrames())
	open(app, "Animations")
	app.Advance(time.Second)
	app.Press("exit-toggle")
	app.Advance(50 * time.Millisecond)
	if !app.Exists("exit-card") {
		t.Fatal("the card went before its exit played")
	}
	if o := app.Animated("exit-card").Opacity; o <= 0 || o >= 1 {
		t.Fatalf("leaving, the card shows at opacity %v", o)
	}
	app.Advance(time.Second)
	if app.Exists("exit-card") {
		t.Fatal("the card stayed after its exit")
	}
	app.Press("exit-toggle")
	app.Advance(time.Second)
	if !app.Exists("exit-card") || app.Animated("exit-card").Opacity != 1 {
		t.Fatal("the card did not come back")
	}
}

// TestSharedElementsOpenATile opens a tile into its panel and back, on
// any target, the panel growing from the tile.
func TestSharedElementsOpenATile(t *testing.T) {
	var app *nsgotest.App
	if target := nsgotest.Target(); target == "ios_simulator" || target == "android" {
		app = startPhone(t)
		openPhone(app, "Animations")
	} else {
		app = startDesktop(t)
		open(app, "Animations")
	}
	app.Press("tile-2")
	if h := app.Find("tile-2").Frame.Size.Height; h != 120 {
		t.Fatalf("the opened tile is %v tall, want the panel's 120", h)
	}
	for _, id := range []string{"tile-0", "tile-1", "tile-3"} {
		if h := app.Find(id).Frame.Size.Height; h != 48 {
			t.Fatalf("%s is %v tall beside the panel, want 48", id, h)
		}
	}
	app.Screenshot("shared-element-open")
	app.Press("tile-2")
	if h := app.Find("tile-2").Frame.Size.Height; h != 48 {
		t.Fatalf("the closed tile is %v tall, want 48", h)
	}
}

// TestResistingDragsSlowPastTheEnds drags the knob past its track's end:
// it shows resisted short of the drag, then springs back to the end.
func TestResistingDragsSlowPastTheEnds(t *testing.T) {
	headless(t)
	app := startDesktop(t, nsgotest.ManualFrames())
	open(app, "Animations")
	app.Pan("resist-knob", core.Point{X: 40}, core.Point{X: 300})
	if x := app.Animated("resist-knob").Transform.TranslateX; x <= 80 || x >= 300 {
		t.Fatalf("dragged 300 points, the knob shows at %v, want resisted past 80", x)
	}
	app.Advance(2 * time.Second)
	if x := app.Animated("resist-knob").Transform.TranslateX; x < 79.5 || x > 80.5 {
		t.Fatalf("let go, the knob rests at %v, want the track's end at 80", x)
	}
}
