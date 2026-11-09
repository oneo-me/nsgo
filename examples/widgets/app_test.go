package main

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/widget"
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

func TestEntry(t *testing.T) {
	e := entry(3, time.Date(2026, 1, 1, 10, 15, 0, 0, time.UTC))
	for _, f := range []widget.Family{widget.Medium, widget.Circular, widget.Rectangular, widget.Inline} {
		if e.Families[f] == nil {
			t.Errorf("no view for %s", f)
		}
	}
}
