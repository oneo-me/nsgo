package main

import (
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/widget"
)

// TestWidgets writes the widgets from the demo, which a test, no app built
// with them, reports it cannot show.
func TestWidgets(t *testing.T) {
	headless(t)
	app := startDesktop(t)
	open(app, "Widgets")
	status := ""
	for deadline := time.Now().Add(5 * time.Second); status == "" && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		app.Settle()
		status = app.Find("id:widgets-status").Text
	}
	if widget.Available() || status == "" {
		t.Fatalf("status = %q", status)
	}
	app.Press("id:widgets-placeholder")
	app.Press("id:widgets-update")
}

// TestWidgetFamilies gives Activity a view of its own in every family but
// small, which takes View, and a timeline that walks on.
func TestWidgetFamilies(t *testing.T) {
	tr := locales.In("en")
	now := time.Date(2026, 11, 9, 10, 30, 0, 0, time.UTC)
	entries := activityTimeline(tr, defaultWidgets, now)
	if len(entries) != 6 || !entries[5].Date.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("timeline = %d entries, last at %v", len(entries), entries[len(entries)-1].Date)
	}
	for _, f := range []widget.Family{widget.Medium, widget.Large, widget.ExtraLarge, widget.Circular, widget.Rectangular, widget.Inline} {
		if entries[0].Families[f] == nil {
			t.Errorf("no view for %s", f)
		}
	}
	if entries[0].View == nil {
		t.Error("no view for small")
	}
	off := defaultWidgets
	off.timeline = false
	if n := len(activityTimeline(tr, off, now)); n != 1 {
		t.Errorf("timeline off = %d entries, want 1", n)
	}
	photo := photoEntry(tr, defaultWidgets, now)
	if len(photo.Families) != 3 || photo.View == nil {
		t.Errorf("photo = %+v", photo)
	}
	if p := widgetPicture(2); len(p) < 1000 || string(p[1:4]) != "PNG" {
		t.Errorf("picture of %d bytes", len(p))
	}
}

// TestLiveActivity starts the walk from the Widgets demo: on the iOS
// simulator and Android through the system, which updates it as the steps
// change and ends it, and headless with the reason it cannot.
func TestLiveActivity(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		steps := core.NewSignal(cx, float64(defaultWidgets.steps))
		goal := core.NewSignal(cx, float64(defaultWidgets.goal))
		accent := core.NewSignal(cx, defaultWidgets.accent)
		state := func() widgetState {
			return widgetState{steps: int(steps.Get()), goal: int(goal.Get()), accent: accent.Get()}
		}
		return liveActivityCard(cx, steps, goal, accent, state)
	}, nsgotest.Size(390, 600))
	app.Press("id:activity-toggle")
	if !widget.ActivitiesEnabled() {
		if got := app.Find("id:activity-status").Text; !strings.Contains(got, "Live Activities") {
			t.Errorf("status = %q, want why there are no Live Activities", got)
		}
		return
	}
	walking := func() int {
		n := 0
		for _, a := range widget.Activities() {
			if a.Kind() == walkKind {
				n++
			}
		}
		return n
	}
	if n := walking(); n != 1 {
		t.Fatalf("%d walks after Start", n)
	}
	app.Press("id:activity-toggle")
	for deadline := time.Now().Add(5 * time.Second); walking() > 0 && time.Now().Before(deadline); {
		time.Sleep(100 * time.Millisecond)
	}
	if n := walking(); n != 0 {
		t.Errorf("%d walks after End", n)
	}
}

// TestWalk keeps the whole state in a button's action, and its content
// within what a push carries.
func TestWalk(t *testing.T) {
	s := widgetState{steps: 6420, goal: 10000, accent: 2}
	c := walkContent(locales.In("zh-Hans"), "zh-Hans", s)
	lang, next, ok := parseWalk(c.Actions[0].ID)
	if !ok || lang != "zh-Hans" || next != (widgetState{steps: 6920, goal: 10000, accent: 2}) {
		t.Errorf("More = %q, %+v, %v", lang, next, ok)
	}
	if c.Actions[1].ID != "end" || c.Status != "64%" || c.Progress != 0.642 {
		t.Errorf("content = %+v", c)
	}
	if _, _, ok := parseWalk("end"); ok {
		t.Error("end parsed as a walk")
	}
	if _, err := widget.ActivityState(c); err != nil {
		t.Error(err)
	}
}
