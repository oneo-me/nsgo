package main

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// The performance scenarios run only when NSGO_GALLERY_PERF is set, under
// nsgo test -target with -fps, whose readings each second are what they
// measure: they check nothing, and mark what the readings measure (Mark)
// ([performance](../../docs/architecture/performance.md#keeping-frames-cheap)).
func perfScenarios(t *testing.T) {
	if os.Getenv("NSGO_GALLERY_PERF") == "" {
		t.Skip("set NSGO_GALLERY_PERF to measure")
	}
}

// perfOptions measure a scenario in a window of
// NSGO_GALLERY_PERF_SIZE's WxH, 1200×800 by default.
func perfOptions() []nsgotest.Option {
	var w, h float64
	if _, err := fmt.Sscanf(os.Getenv("NSGO_GALLERY_PERF_SIZE"), "%gx%g", &w, &h); err != nil {
		w, h = 1200, 800
	}
	return []nsgotest.Option{nsgotest.Size(driver.Points(w), driver.Points(h)), nsgotest.Measure()}
}

// perfSeconds is NSGO_GALLERY_PERF_SECONDS, how long a scenario that
// waits measures, else d seconds: longer for a profiler to sample.
func perfSeconds(d int) time.Duration {
	if n, err := strconv.Atoi(os.Getenv("NSGO_GALLERY_PERF_SECONDS")); err == nil && n > 0 {
		d = n
	}
	return time.Duration(d) * time.Second
}

// perfOpen starts the gallery as the target shows it, a phone's layout
// on a phone, and opens demo.
func perfOpen(t *testing.T, demo string) *nsgotest.App {
	perfScenarios(t)
	if target := nsgotest.Target(); target == "ios_simulator" || target == "android" {
		// A phone opens where it was last, as a phone's tab bar is the
		// system's, which steps cannot press.
		return nsgotest.Start(t, Root, nsgotest.Environment(core.Environment{Locale: "en-US"}),
			nsgotest.Storage(map[string]string{lastDemo: demo}), nsgotest.Measure())
	}
	app := startDesktop(t, perfOptions()...)
	open(app, demo)
	return app
}

// TestPerfDataTable lets the DataTable tick, then scrolls it 6000 points
// down and back three times while it ticks.
func TestPerfDataTable(t *testing.T) {
	app := perfOpen(t, "DataTable")
	app.Find("text:rows 1–")
	app.Advance(2 * time.Second)
	app.Mark("DataTable ticking")
	app.Advance(perfSeconds(5))
	app.Mark("DataTable scrolling")
	for range 3 {
		app.Scroll("id:data-table-scroll", 6000)
		app.Scroll("id:data-table-scroll", -6000)
	}
	app.Mark("-")
}

// TestPerfStack pushes four pages on the NavStack demo's deep stack and
// pops back to its index, three times: a phone's back button at a time,
// as the demo is a page of a phone's own stack, which To the index would
// leave.
func TestPerfStack(t *testing.T) {
	app := perfOpen(t, "NavStack")
	target := nsgotest.Target()
	phone := target == "ios_simulator" || target == "android"
	app.Advance(time.Second)
	app.Mark("Stack slides")
	for range 3 {
		app.Press("role:button A deep stack")
		for range 4 {
			app.Press("role:button Deeper")
		}
		if phone {
			for range 5 {
				app.Back()
			}
		} else {
			app.Press("role:button To the index")
		}
	}
	app.Mark("-")
}

// TestPerfAnimations runs the Animations page as it opens.
func TestPerfAnimations(t *testing.T) {
	app := perfOpen(t, "Animations")
	app.Advance(2 * time.Second)
	app.Mark("Animations")
	app.Advance(perfSeconds(6))
	app.Mark("-")
}
