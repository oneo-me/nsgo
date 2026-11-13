package main

import (
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/widget"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestCounts(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(420, 360))
	app.Press("id:increment")
	app.Press("id:increment")
	app.Press("id:decrement")
	if got := app.Find("id:count").Text; got != "1" {
		t.Errorf("count = %q, want 1", got)
	}
	if v, _ := app.Storage().GetItem(countKey); v != "1" {
		t.Errorf("stored count = %q, want 1", v)
	}
	// A test is no app built with its widgets, so the update says why.
	if got := app.Find("id:status").Text; widget.Available() || got == "" {
		t.Errorf("status = %q", got)
	}
	app.Press("id:reset")
	if got := app.Find("id:count").Text; got != "0" {
		t.Errorf("count after reset = %q, want 0", got)
	}
}

// TestLiveActivity starts, updates and ends the counter's Live Activity:
// on the iOS simulator and Android through the system, and headless with
// the error the platform reports.
func TestLiveActivity(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(420, 360))
	app.Press("id:activity")
	if !widget.ActivitiesEnabled() {
		if got := app.Find("id:status").Text; !strings.Contains(got, "Live Activities") {
			t.Errorf("status = %q, want why there are no Live Activities", got)
		}
		return
	}
	running := func() int {
		n := 0
		for _, a := range widget.Activities() {
			if a.Kind() == activityKind {
				n++
			}
		}
		return n
	}
	if n := running(); n != 1 {
		t.Fatalf("%d activities run after Start", n)
	}
	app.Press("id:increment")
	app.Press("id:activity")
	app.Settle()
	deadline := time.Now().Add(5 * time.Second)
	for running() > 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if n := running(); n != 0 {
		t.Errorf("%d activities run after End", n)
	}
}

func TestActivityContent(t *testing.T) {
	c := activityContent(-3)
	if c.Status != "-3" || c.Progress != 0.7 || c.Actions[0].ID != "set:-4" || c.Actions[1].ID != "set:-2" {
		t.Errorf("content = %+v", c)
	}
	if _, err := widget.ActivityState(c); err != nil {
		t.Error(err)
	}
}

func TestEntry(t *testing.T) {
	e := entry(3, time.Date(2026, 1, 1, 10, 15, 0, 0, time.UTC))
	for _, f := range []widget.Family{widget.Medium, widget.Circular, widget.Rectangular, widget.Inline} {
		if e.Families[f] == nil {
			t.Errorf("no view for %s", f)
		}
	}
}
