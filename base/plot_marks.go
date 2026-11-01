package base

import (
	"log"
	"math"
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// PlotCurve is how a PlotLine or a PlotArea connects its points, as
// gpui-base's Curve and d3's curve factories.
type PlotCurve uint8

const (
	// CurveNatural is a smooth curve through every point: cubic Béziers
	// through the neighbours' tangents.
	CurveNatural PlotCurve = iota
	// CurveLinear is straight segments between points.
	CurveLinear
	// CurveStepAfter holds each value until the next point.
	CurveStepAfter
)

// strokeRoom is how far a mark's canvas reaches past the plot's edges for
// its strokes: a stroke centered on an edge is half outside the plot, and
// a canvas draws nothing outside itself. Strokes up to twice this wide draw
// whole.
const strokeRoom = 8

// plotMark is a mark the plot knows: its extent along the y axis, its
// points, where its hover dot is and how the app styles it.
type plotMark struct {
	key        int
	extent     *core.Memo[plotExtent]
	points     *core.Memo[[]core.Point]
	dotX, dotY *core.AnimatedValue
	dot        part // its own hover dot's
}

// near is the mark's point nearest to x along the x axis.
func (m *plotMark) near(x float32) (core.Point, bool) {
	var best core.Point
	ok, d := false, math.Inf(1)
	for _, q := range m.points.GetUntracked() {
		if e := math.Abs(float64(float32(q.X) - x)); e < d {
			best, ok, d = q, true, e
		}
	}
	return best, ok
}

// plotSeries is what a mark draws: its points in the plot's space, the
// rows without a value skipped, as gpui-base's shapes skip them.
type plotSeries struct {
	points *core.Memo[[]core.Point]
	ys     func() []float64
}

// newSeries is the series of the values y (the plot's without), placed
// along the x axis where the plot's rows go, or by x on rows of its own.
func newSeries(cx *core.Scope, plot *plotState, x, y func() []float64) plotSeries {
	ys := func() []float64 {
		if y != nil {
			return y()
		}
		return plot.y()
	}
	xs := func() []float32 {
		if x == nil {
			return plot.xs.Get()
		}
		own := x()
		return plot.xMap.Get().ticks(len(own), own)
	}
	points := core.NewMemoFunc(cx, func() []core.Point {
		at, values, yMap := xs(), ys(), plot.yMap.Get()
		out := make([]core.Point, 0, len(values))
		for i, v := range values {
			if i >= len(at) {
				break
			}
			py, ok := yMap.tick(v)
			if ok && !math.IsNaN(float64(at[i])) {
				out = append(out, core.Point{X: core.Points(at[i]), Y: core.Points(py)})
			}
		}
		return out
	}, slices.Equal)
	return plotSeries{points: points, ys: ys}
}

// join adds the series to plot while the mark lives: its values, and
// also's, in the y domain, and a hover dot.
func (s plotSeries) join(cx *core.Scope, plot *plotState, also func() (float64, bool), dot part) {
	extent := core.NewMemo(cx, func() plotExtent {
		e := extentOf(s.ys())
		if v, ok := also(); ok {
			e = e.with(v)
		}
		return e
	})
	plot.add(cx, &plotMark{extent: extent, points: s.points, dotX: core.NewAnimatedValue(cx, 0),
		dotY: core.NewAnimatedValue(cx, 0), dot: dot})
}

// plotOf is the plot a mark is in, nil outside any.
func plotOf(cx *core.Scope, mark string) *plotState {
	p := plotKey.Use(cx)
	if p == nil {
		log.Printf("base: %s draws only inside a Plot", mark)
	}
	return p
}

// trace records the path through points, the current point being the
// first, as curve connects them.
func trace(d *core.Drawing, points []core.Point, curve PlotCurve) {
	n := len(points)
	f := func(p core.Point) (float32, float32) { return float32(p.X), float32(p.Y) }
	switch curve {
	case CurveLinear:
		for _, q := range points[1:] {
			d.LineTo(f(q))
		}
	case CurveStepAfter:
		for i := 1; i < n; i++ {
			d.LineTo(float32(points[i].X), float32(points[i-1].Y))
			d.LineTo(f(points[i]))
		}
	default:
		// Each segment a cubic through its neighbours' tangents, the ends
		// their own neighbours.
		for i := 0; i < n-1; i++ {
			p0, p1, p2, p3 := points[max(i-1, 0)], points[i], points[i+1], points[min(i+2, n-1)]
			d.CubicTo(float32(p1.X+(p2.X-p0.X)/6), float32(p1.Y+(p2.Y-p0.Y)/6),
				float32(p2.X-(p3.X-p1.X)/6), float32(p2.Y-(p3.Y-p1.Y)/6), float32(p2.X), float32(p2.Y))
		}
	}
}

// reach is how far what a mark traces through points goes past the top
// and bottom edges, height apart: a smooth curve swings past its highest
// and lowest points, never past its control points.
func reach(points []core.Point, curve PlotCurve, height float32) float32 {
	var over float32
	take := func(y core.Points) { over = max(over, -float32(y), float32(y)-height) }
	for _, q := range points {
		take(q.Y)
	}
	if curve == CurveNatural {
		n := len(points)
		for i := 0; i < n-1; i++ {
			p0, p1, p2, p3 := points[max(i-1, 0)], points[i], points[i+1], points[min(i+2, n-1)]
			take(p1.Y + (p2.Y-p0.Y)/6)
			take(p2.Y - (p3.Y-p1.Y)/6)
		}
	}
	return over
}

// markLayer is a mark's view: it fills the plot, takes no pointer (the
// plot follows it) and holds the mark's paint style.
func markLayer(p part, testID string, children ...core.Node) core.Node {
	return core.View{
		TestID:        testID,
		Class:         p.class,
		Style:         core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)}.Merge(p.style),
		StyleFunc:     p.fn,
		PointerEvents: core.PointerEventsNone,
		Children:      children,
	}
}

// markCanvas is a mark's canvas: the plot's area and bleed() points around
// it, for half its strokes and its curves' swing past the plot's edges,
// its drawing in the plot's coordinates. Before the plot's first layout
// draw gets a nil plot. own is the canvas's own part, over its place.
func markCanvas(plot *plotState, bleed func() float32, own part,
	draw func(d *core.Drawing, plot *plotState, bleed float32)) core.Node {
	return core.Canvas{
		Class: own.class,
		Style: core.Style{Position: core.Absolute},
		StyleFunc: func() core.Style {
			b := core.Pt(core.Points(-bleed()))
			s := core.Style{Left: b, Top: b, Right: b, Bottom: b}.Merge(own.style)
			if own.fn != nil {
				s = s.Merge(own.fn())
			}
			return s
		},
		Draw: func(d *core.Drawing, _ core.Size) {
			if plot == nil || !plot.sized() {
				draw(d, nil, 0)
				return
			}
			b := bleed()
			d.Translate(b, b)
			draw(d, plot, b)
		},
	}
}

// PlotLine is a series of a Plot drawn as a line through its points, as
// gpui-base's Line: one point per row with a value, placed by the plot's
// scales, a smooth curve unless Curve says otherwise. It strokes with its
// paint style (Stroke, StrokeWidth, StrokeDash; inherited from the plot),
// the text color where none is set, and with Dot shows a view the app
// styles on every point. Its values make the y scale's domain when the
// plot has none, it has a hover dot when it or the plot styles one, and it
// draws in with the plot's appear.
type PlotLine struct {
	// Y is what it draws, one value per row; the plot's Y without.
	Y func() []float64
	// X gives it rows of its own, placed along the plot's x axis by these
	// values on a linear scale and by index on a point scale; without it
	// Y is a column of the plot's rows.
	X     func() []float64
	Curve PlotCurve
	// Dot shows a view styled by DotStyle centered on every point.
	Dot          bool
	DotClass     string
	DotStyle     core.Style
	DotStyleFunc func() core.Style
	// HoverDotStyle merges over the plot's DotStyle for its hover dot, and
	// HoverDotClass follows its DotClass; a DisplayNone shows none.
	HoverDotClass     string
	HoverDotStyle     core.Style
	HoverDotStyleFunc func() core.Style
	// Style is its paint: Stroke, StrokeWidth and StrokeDash.
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
}

// Render implements core.Node.
func (l PlotLine) Render(cx *core.Scope) core.Node {
	plot := plotOf(cx, "PlotLine")
	var series plotSeries
	if plot != nil {
		series = newSeries(cx, plot, l.X, l.Y)
		series.join(cx, plot, func() (float64, bool) { return 0, false }, part{l.HoverDotClass, l.HoverDotStyle, l.HoverDotStyleFunc})
	}
	bleed := func() float32 {
		if plot == nil {
			return strokeRoom
		}
		return strokeRoom + float32(math.Ceil(float64(reach(series.points.Get(), l.Curve, float32(plot.size.Get().Height)))))
	}
	children := []core.Node{markCanvas(plot, bleed, part{}, func(d *core.Drawing, plot *plotState, b float32) {
		d.StrokeFromView(core.PaintCurrentColor).LineWidthFromView().LineDashFromView()
		if plot == nil {
			return
		}
		points := series.points.Get()
		if len(points) == 0 {
			return
		}
		plot.clip(d, b)
		d.BeginPath().MoveTo(float32(points[0].X), float32(points[0].Y))
		trace(d, points, l.Curve)
		d.Stroke()
	})}
	if l.Dot && plot != nil {
		children = append(children, core.Each(func() []int {
			n := len(series.points.Get())
			out := make([]int, n)
			for i := range out {
				out[i] = i
			}
			return out
		}, func(i int) int { return i }, func(cx *core.Scope, item func() int, _ func() int) core.Node {
			return core.View{
				Style: core.Style{Position: core.Absolute, Width: core.Pt(0), Height: core.Pt(0),
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				StyleFunc: func() core.Style {
					points := series.points.Get()
					if i := item(); i < len(points) {
						return core.Style{Left: core.Pt(points[i].X), Top: core.Pt(points[i].Y)}
					}
					return core.Style{}
				},
				PointerEvents: core.PointerEventsNone,
				Children:      []core.Node{core.View{Class: l.DotClass, Style: l.DotStyle, StyleFunc: l.DotStyleFunc}},
			}
		}))
	}
	return markLayer(part{l.Class, l.Style, l.StyleFunc}, l.TestID, children...)
}

// PlotArea is a series of a Plot drawn as the region between its line and
// a baseline, as gpui-base's Area: its top edge goes through its points as
// a PlotLine's does and it closes down to Y0. It fills with its Fill
// (inherited from the plot), the text color where none is set, over its
// background (BackgroundStyle's Background and Gradients, shown inside its
// shape only; Fill is then none where none is set), and strokes its top
// edge with its Stroke, none where none is set. Its values and baseline
// make the y scale's domain when the plot has none; it has a hover dot
// when it or the plot styles one, and draws in with the plot's appear.
type PlotArea struct {
	// Y1 makes its top edge, one value per row; the plot's Y without.
	Y1 func() []float64
	// X gives it rows of its own, as a PlotLine's.
	X func() []float64
	// Y0 is the value of its baseline on the y scale; the plot's bottom
	// edge without it.
	Y0    func() float64
	Curve PlotCurve
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
func (a PlotArea) Render(cx *core.Scope) core.Node {
	plot := plotOf(cx, "PlotArea")
	var series plotSeries
	if plot != nil {
		series = newSeries(cx, plot, a.X, a.Y1)
		series.join(cx, plot, func() (float64, bool) {
			if a.Y0 == nil {
				return 0, false
			}
			return a.Y0(), true
		}, part{a.HoverDotClass, a.HoverDotStyle, a.HoverDotStyleFunc})
	}
	base := func(plot *plotState) float32 {
		if a.Y0 != nil {
			v, _ := plot.yMap.Get().tick(a.Y0())
			return v
		}
		return float32(plot.size.Get().Height)
	}
	// How far past the plot it draws: its curve's swing and its baseline.
	bleed := func() float32 {
		if plot == nil {
			return strokeRoom
		}
		height := float32(plot.size.Get().Height)
		floor := base(plot)
		swing := reach(series.points.Get(), a.Curve, height)
		return strokeRoom + float32(math.Ceil(float64(max(swing, -floor, floor-height))))
	}
	// Its shape: the top edge, down to the baseline and back.
	shape := func(d *core.Drawing, plot *plotState, points []core.Point) {
		first, last, floor := points[0], points[len(points)-1], base(plot)
		d.BeginPath().MoveTo(float32(first.X), float32(first.Y))
		trace(d, points, a.Curve)
		d.LineTo(float32(last.X), floor).LineTo(float32(first.X), floor).ClosePath()
	}
	background := part{a.BackgroundClass, a.BackgroundStyle, a.BackgroundStyleFunc}
	hasBackground := background.set()
	var children []core.Node
	if hasBackground {
		// A canvas whose view takes the app's background and shows it
		// inside the shape only, from its first drawing on, so its view
		// never draws the background as a box.
		children = append(children, markCanvas(plot, bleed, background, func(d *core.Drawing, plot *plotState, b float32) {
			var points []core.Point
			if plot != nil {
				points = series.points.Get()
			}
			if len(points) == 0 {
				d.BeginPath().FillBackgroundFromView(core.Bounds{})
				return
			}
			floor := base(plot)
			top, bottom := floor, floor
			for _, q := range points {
				top, bottom = min(top, float32(q.Y)), max(bottom, float32(q.Y))
			}
			first, last := points[0], points[len(points)-1]
			area := core.Bounds{Origin: core.Point{X: first.X, Y: core.Points(top)},
				Size: core.Size{Width: last.X - first.X, Height: core.Points(bottom - top)}}
			plot.clip(d, b)
			shape(d, plot, points)
			d.FillBackgroundFromView(area)
		}))
	}
	children = append(children, markCanvas(plot, bleed, part{}, func(d *core.Drawing, plot *plotState, b float32) {
		initial := core.PaintCurrentColor
		if hasBackground {
			initial = core.PaintNone
		}
		d.FillFromView(initial)
		if plot == nil {
			return
		}
		points := series.points.Get()
		if len(points) == 0 {
			return
		}
		plot.clip(d, b)
		shape(d, plot, points)
		d.Fill()
		d.StrokeFromView(core.PaintNone).LineWidthFromView().LineDashFromView()
		d.BeginPath().MoveTo(float32(points[0].X), float32(points[0].Y))
		trace(d, points, a.Curve)
		d.Stroke()
	}))
	return markLayer(part{a.Class, a.Style, a.StyleFunc}, a.TestID, children...)
}

// PlotGrid is lines across a Plot, as gpui-base's Grid: vertical at the
// values X of its x scale (row indices on a point scale) and horizontal at
// the values Y of its y scale, from edge to edge, each on whole device
// pixels so a thin line stays sharp. It strokes with its paint style
// (Stroke, StrokeWidth, StrokeDash; inherited from the plot), the text
// color where none is set. Its values do not count in the domains.
type PlotGrid struct {
	X, Y      func() []float64
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
}

// Render implements core.Node.
func (g PlotGrid) Render(cx *core.Scope) core.Node {
	plot := plotOf(cx, "PlotGrid")
	values := func(f func() []float64) []float64 {
		if f == nil {
			return nil
		}
		return f()
	}
	return markLayer(part{g.Class, g.Style, g.StyleFunc}, g.TestID, markCanvas(plot, func() float32 { return strokeRoom }, part{},
		func(d *core.Drawing, plot *plotState, _ float32) {
			d.StrokeFromView(core.PaintCurrentColor).LineWidthFromView().LineDashFromView().BeginPath()
			if plot == nil {
				return
			}
			size := plot.size.Get()
			w, h := float32(size.Width), float32(size.Height)
			// A line on the middle of a device pixel row or column: a
			// hairline 1 point wide on a 1x screen is one pixel, not two
			// halves.
			ratio := plot.ratio()
			sharp := func(v float32) float32 { return (float32(math.Floor(float64(v*ratio))) + 0.5) / ratio }
			xMap := plot.xMap.Get()
			for _, v := range values(g.X) {
				if at, ok := xMap.tick(v); ok {
					d.MoveTo(sharp(at), 0).LineTo(sharp(at), h)
				}
			}
			yMap := plot.yMap.Get()
			for _, v := range values(g.Y) {
				if at, ok := yMap.tick(v); ok {
					d.MoveTo(0, sharp(at)).LineTo(w, sharp(at))
				}
			}
			d.Stroke()
		}))
}
