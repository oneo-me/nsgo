package main

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/widget"
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
