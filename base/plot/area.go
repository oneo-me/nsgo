package plot

import (
	"math"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// Area is a series of a Plot drawn as the region between its line and
// a baseline, as gpui-base's Area: its top edge goes through its points as
// a Line's does and it closes down to Y0, or with Lows along a bottom edge
// through those values with the same curve, a band (d3's area().y0). It
// fills with its Fill (inherited from the plot), the text color where none
// is set or with Color its category's, over its background
// (BackgroundStyle's Background and Gradients, shown inside its shape only;
// Fill is then none where none is set), and strokes its top edge with its
// Stroke, none where none is set. Its values and baseline make the y
// scale's domain when the plot has none; it has a hover dot when it or the
// plot styles one, and draws in with the plot's appear.
type Area struct {
	// Y1 makes its top edge, one value per row; the plot's Y without.
	Y1 func() []float64
	// X gives it rows of its own, as a Line's.
	X func() []float64
	// Y0 is the value of its baseline on the y scale; without it or Lows,
	// the end of the y scale where its smallest values are: the bottom edge,
	// the top one when the scale is reversed.
	Y0 func() float64
	// Lows is a bottom value per row, in place of Y0: a day's low to its
	// high, a forecast's fan. A row without one is skipped.
	Lows func() []float64
	// Scale is the y scale its edges go along.
	Scale ValueScale
	Curve Curve
	// Color is the category of its rows, as a Line's; its Fill where
	// nothing sets one.
	Color func() []string
	// BackgroundStyle's Background and Gradients fill a box around its
	// shape, from its first point to its last and from its highest point
	// to its baseline, shown only inside the shape, under its Fill, as
	// CSS's clip-path shows a box's background: a gradient that fades out
	// toward the baseline.
	BackgroundClass     string
	BackgroundStyle     core.Style
	BackgroundStyleFunc func() core.Style
	// HoverDotStyle merges over the plot's DotStyle for its hover dot, and
	// HoverDotClass follows its DotClass; a DisplayNone shows none, as for
	// an area under a line with its own.
	HoverDotClass     string
	HoverDotStyle     core.Style
	HoverDotStyleFunc func() core.Style
	// Style is its paint: Fill, and Stroke, StrokeWidth and StrokeDash for
	// its top edge.
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
}

// Render implements core.Node.
func (a Area) Render(cx *core.Scope) core.Node {
	plot := plotOf(cx, "Area")
	a.Curve = a.Curve.or(CurveNatural)
	var series plotSeries
	if plot != nil {
		series = newSeries(cx, plot, a.X, a.Y1, a.Lows, a.Scale)
		series.join(cx, plot, func() (float64, bool) {
			if a.Y0 == nil {
				return 0, false
			}
			return a.Y0(), true
		}, parts.Of(a.HoverDotClass, a.HoverDotStyle, a.HoverDotStyleFunc))
	}
	// Its constant baseline: Y0's value, or the edge of the plot where the
	// y scale's smallest values are.
	base := func(plot *plotState) float32 {
		m := plot.yMapOf(a.Scale)
		if a.Y0 != nil {
			v, _ := m.tick(a.Y0())
			return v
		}
		if m.kind == axisLinear {
			return m.linear.rng[0]
		}
		return float32(plot.size.Get().Height)
	}
	// How far past the plot it draws: its edges' swing, or its baseline.
	bleed := func() float32 {
		if plot == nil {
			return strokeRoom
		}
		height := float32(plot.size.Get().Height)
		e := series.drawn.Get()
		swing := max(reach(e.top, a.Curve, height), reach(e.wasTop, a.Curve, height))
		var past float32
		if len(e.bottom) > 0 {
			past = max(reach(e.bottom, a.Curve, height), reach(e.wasBottom, a.Curve, height))
		} else {
			floor := base(plot)
			past = max(-floor, floor-height)
		}
		return strokeRoom + float32(math.Ceil(float64(max(swing, past))))
	}
	// Its shape: the top edge, to the baseline or the bottom edge, and
	// back. Both edges of a band and a baseline at a value move with the y
	// scale's domain; the plot's edge stays.
	// While Motion.Update moves it, each point moves from where its row's
	// was.
	shape := func(d *core.Drawing, plot *plotState, e seriesEdges) {
		t := series.progress(e)
		points, was := e.top, e.wasTop
		n := len(points)
		d.BeginPath()
		plot.traced(d, true, a.Scale, func() {
			startMoving(d, was, points, t, false)
			traceMoving(d, was, points, t, a.Curve, false)
		})
		if lows := e.bottom; len(lows) > 0 {
			end := []core.Point{lows[len(lows)-1]}
			plot.traced(d, true, a.Scale, func() {
				startMoving(d, e.wasBottom[len(lows)-1:], end, t, true)
				traceMoving(d, e.wasBottom, lows, t, a.Curve, true)
			})
		} else {
			scale, floor := noScale, constNum(base(plot))
			if a.Y0 != nil {
				scale = a.Scale
			}
			x := func(i int) num {
				if t == nil {
					return constNum(float32(points[i].X))
				}
				return lerpNum(t, float32(was[i].X), float32(points[i].X))
			}
			plot.traced(d, true, scale, func() {
				lineTo(d, x(n-1), floor)
				lineTo(d, x(0), floor)
			})
		}
		d.ClosePath()
	}
	background := parts.Of(a.BackgroundClass, a.BackgroundStyle, a.BackgroundStyleFunc)
	hasBackground := background.Set()
	var children []core.Node
	if hasBackground {
		// A canvas whose view takes the app's background and shows it
		// inside the shape only.
		children = append(children, markCanvas(plot, bleed, background, func(d *core.Drawing, plot *plotState, b float32) {
			var e seriesEdges
			if plot != nil {
				e = series.drawn.Get()
			}
			points, lows := e.top, e.bottom
			if len(points) == 0 {
				nothing(d)
				return
			}
			// From its highest point to its lowest, the baseline's or the
			// bottom edge's included.
			start := float32(points[0].Y)
			if len(lows) == 0 {
				start = base(plot)
			}
			top, bottom := start, start
			for _, q := range points {
				top, bottom = min(top, float32(q.Y)), max(bottom, float32(q.Y))
			}
			for _, q := range lows {
				top, bottom = min(top, float32(q.Y)), max(bottom, float32(q.Y))
			}
			// Where it moves from too, so its background spans the move.
			for _, q := range append(append([]core.Point(nil), e.wasTop...), e.wasBottom...) {
				top, bottom = min(top, float32(q.Y)), max(bottom, float32(q.Y))
			}
			first, last := points[0], points[len(points)-1]
			area := core.Bounds{Origin: core.Point{X: first.X, Y: core.Points(top)},
				Size: core.Size{Width: last.X - first.X, Height: core.Points(bottom - top)}}
			plot.clip(d, b)
			shape(d, plot, e)
			d.FillBackgroundFromView(area)
		}))
	}
	children = append(children, markCanvas(plot, bleed, part{}, func(d *core.Drawing, plot *plotState, b float32) {
		initial := core.PaintCurrentColor
		if hasBackground {
			initial = core.PaintNone
		}
		d.FillFromView(paintOf(seriesColor(plot, a.Color), initial))
		if plot == nil {
			return
		}
		e := series.drawn.Get()
		if len(e.top) == 0 {
			return
		}
		plot.clip(d, b)
		shape(d, plot, e)
		d.Fill()
		d.StrokeFromView(core.PaintNone).LineWidthFromView().LineDashFromView()
		d.BeginPath()
		t := series.progress(e)
		plot.traced(d, true, a.Scale, func() {
			startMoving(d, e.wasTop, e.top, t, false)
			traceMoving(d, e.wasTop, e.top, t, a.Curve, false)
		})
		d.Stroke()
	}))
	return markLayer(parts.Of(a.Class, a.Style, a.StyleFunc), a.TestID, children...)
}
