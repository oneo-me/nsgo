package main

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestFlowDiagram lays the plan out, selects a step, adds one after it and
// turns the layout.
func TestFlowDiagram(t *testing.T) {
	app := startDesktop(t, nsgotest.Size(1200, 900))
	open(app, "FlowDiagram")
	app.Find("flow >> label:Review the launch")
	app.Screenshot("flow-diagram")
	// Two fingers on a touchpad pan the camera, the plane's translation.
	camera := func() driver.Point {
		x, y, _ := flowShown.Camera()
		return driver.Point{X: driver.Points(x), Y: driver.Points(y)}
	}
	before := camera()
	app.TouchpadScroll("id:flow", driver.Point{}, driver.Point{X: 40, Y: 60})
	if c := camera(); c.X-before.X < 39 || c.Y-before.Y < 59 {
		t.Fatalf("two fingers moved the camera from %v to %v", before, c)
	}
	for range 5 {
		app.Press("label:Zoom in")
	}
	app.Advance(time.Second)
	app.Screenshot("flow-diagram-zoomed")
	app.Press("label:Fit view")
	app.Press("label:Add a step")
	app.Find("flow >> label:Follow-up step 1")
	if got := app.Find("id:flow-selected").Text; got != "added-1" {
		t.Fatalf("selected %q after adding", got)
	}
	app.PressAt("id:flow", driver.Point{X: 4, Y: 4})
	if got := app.Find("id:flow-selected").Text; got != "—" {
		t.Fatalf("selected %q after pressing empty space", got)
	}
	app.Press("role:switch Lay out left to right")
	app.Screenshot("flow-diagram-left-to-right")
	app.Press("role:switch Show the empty state")
	app.Find("flow >> text:No steps planned yet")
}
