package plot

import (
	"math"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
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
		swing := reach(series.points.Get(), a.Curve, height)
		var past float32
		if lows := series.lows.Get(); len(lows) > 0 {
			past = reach(lows, a.Curve, height)
		} else {
			floor := base(plot)
			past = max(-floor, floor-height)
		}
		return strokeRoom + float32(math.Ceil(float64(max(swing, past))))
	}
	// Its shape: the top edge, to the baseline or the bottom edge, and
	// back. Both edges of a band and a baseline at a value move with the y
	// scale's domain; the plot's edge stays.
	shape := func(d *core.Drawing, plot *plotState, points, lows []core.Point) {
		first, last := points[0], points[len(points)-1]
		d.BeginPath()
		plot.traced(d, true, a.Scale, func() {
			d.MoveTo(float32(first.X), float32(first.Y))
			trace(d, points, a.Curve)
		})
		if len(lows) > 0 {
			end := lows[len(lows)-1]
			plot.traced(d, true, a.Scale, func() {
				d.LineTo(float32(end.X), float32(end.Y))
				traceBack(d, lows, a.Curve)
			})
		} else {
			scale, floor := noScale, base(plot)
			if a.Y0 != nil {
				scale = a.Scale
			}
			plot.traced(d, true, scale, func() {
				d.LineTo(float32(last.X), floor).LineTo(float32(first.X), floor)
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
			var points, lows []core.Point
			if plot != nil {
				points, lows = series.points.Get(), series.lows.Get()
			}
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
			first, last := points[0], points[len(points)-1]
			area := core.Bounds{Origin: core.Point{X: first.X, Y: core.Points(top)},
				Size: core.Size{Width: last.X - first.X, Height: core.Points(bottom - top)}}
			plot.clip(d, b)
			shape(d, plot, points, lows)
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
		points, lows := series.points.Get(), series.lows.Get()
		if len(points) == 0 {
			return
		}
		plot.clip(d, b)
		shape(d, plot, points, lows)
		d.Fill()
		d.StrokeFromView(core.PaintNone).LineWidthFromView().LineDashFromView()
		d.BeginPath()
		plot.traced(d, true, a.Scale, func() {
			d.MoveTo(float32(points[0].X), float32(points[0].Y))
			trace(d, points, a.Curve)
		})
		d.Stroke()
	}))
	return markLayer(parts.Of(a.Class, a.Style, a.StyleFunc), a.TestID, children...)
}
