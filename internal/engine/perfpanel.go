package engine

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The performance panel every backend draws over its surface, as macOS's
// HUD: a dark translucent card at the top right with the readings
// (PerfSnapshot.Readings) in light monospaced text above the trace of
// the last frames' main-thread cost. Backends lay it out with these
// sizes and draw PerfTrace's bars.

// PerfInterval is how often a panel shows new readings.
const PerfInterval = 500 * time.Millisecond

// The panel's width, its distance from the surface's edges, its padding,
// the trace's height and the text's size, in points.
const (
	PerfWidth       = 262.0
	PerfMargin      = 10.0
	PerfPadding     = 8.0
	PerfGraphHeight = 36.0
	PerfFontSize    = 10.5
	PerfRadius      = 8.0
)

// The panel's colors: its fill, its text, and the trace's fill, budget
// line and bars within half the budget, within it and over it.
var (
	PerfPanelColor  = driver.RGBA(0x121214, 0.86)
	PerfTextColor   = driver.RGB(0xebedf2)
	PerfTraceFill   = driver.RGBA(0xffffff, 0.08)
	PerfTraceBudget = driver.RGBA(0xffffff, 0.45)
	PerfTraceGood   = driver.RGBA(0x4dd973, 0.9)
	PerfTraceWarn   = driver.RGBA(0xfabf40, 0.9)
	PerfTraceBad    = driver.RGBA(0xfa5954, 0.9)
)

// PerfBar is a rectangle of the trace, in its box's points with y down.
type PerfBar struct {
	X, Y, W, H float64
	Color      driver.Color
}

// PerfTrace returns the trace of s's frames in a w × h box: its fill,
// a bar per frame, newest at the right, scaled so that twice the budget
// (one refresh) fills the box, and the budget's line at half height.
func PerfTrace(s PerfSnapshot, w, h float64) []PerfBar {
	budget := 1000.0 / 60
	if s.RefreshRate != nil && *s.RefreshRate > 0 {
		budget = 1000 / float64(*s.RefreshRate)
	}
	bars := []PerfBar{{0, 0, w, h, PerfTraceFill}}
	scale := h / (2 * budget)
	bar := w / perfCapacity
	frames := s.Frames[max(0, len(s.Frames)-perfCapacity):]
	start := perfCapacity - len(frames)
	for i, d := range frames {
		ms := float64(d) / float64(time.Millisecond)
		height := min(max(ms*scale, 1), h)
		c := PerfTraceBad
		switch {
		case ms <= budget/2:
			c = PerfTraceGood
		case ms <= budget:
			c = PerfTraceWarn
		}
		bars = append(bars, PerfBar{float64(start+i) * bar, h - height, max(bar-0.5, 0.5), height, c})
	}
	return append(bars, PerfBar{0, float64(int(h / 2)), w, 1, PerfTraceBudget})
}
