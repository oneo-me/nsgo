package plot

import (
	"math"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// Curve is how a Line or a Area connects its points, as
// gpui-base's Curve and d3's curve factories.
type Curve uint8

const (
	// CurveDefault is a mark's own: smooth for a Line and an Area, straight
	// for a RadialLine.
	CurveDefault Curve = iota
	// CurveNatural is a smooth curve through every point: cubic Béziers
	// through the neighbours' tangents.
	CurveNatural
	// CurveLinear is straight segments between points.
	CurveLinear
	// CurveStepAfter holds each value until the next point.
	CurveStepAfter
)

// or is c, or other for CurveDefault.
func (c Curve) or(other Curve) Curve {
	if c == CurveDefault {
		return other
	}
	return c
}

// controls are the control points of the cubic from points[i] to the next.
func controls(points []core.Point, i int) (x1, y1, x2, y2 float32) {
	n := len(points)
	p0, p1, p2, p3 := points[max(i-1, 0)], points[i], points[i+1], points[min(i+2, n-1)]
	return float32(p1.X + (p2.X-p0.X)/6), float32(p1.Y + (p2.Y-p0.Y)/6),
		float32(p2.X - (p3.X-p1.X)/6), float32(p2.Y - (p3.Y-p1.Y)/6)
}

// trace records the path through points, the current point being the
// first, as curve connects them.
func trace(d *core.Drawing, points []core.Point, curve Curve) {
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
			x1, y1, x2, y2 := controls(points, i)
			x, y := f(points[i+1])
			d.CubicTo(x1, y1, x2, y2, x, y)
		}
	}
}

// traceBack records the path trace records through points, the other way:
// from the last point, the current one, back to the first, so the bottom
// edge of a band follows the same curve as its top edge.
func traceBack(d *core.Drawing, points []core.Point, curve Curve) {
	n := len(points)
	f := func(p core.Point) (float32, float32) { return float32(p.X), float32(p.Y) }
	switch curve {
	case CurveLinear:
		for i := n - 2; i >= 0; i-- {
			d.LineTo(f(points[i]))
		}
	case CurveStepAfter:
		for i := n - 1; i > 0; i-- {
			d.LineTo(float32(points[i].X), float32(points[i-1].Y))
			d.LineTo(f(points[i-1]))
		}
	default:
		for i := n - 2; i >= 0; i-- {
			x1, y1, x2, y2 := controls(points, i)
			x, y := f(points[i])
			d.CubicTo(x2, y2, x1, y1, x, y)
		}
	}
}

// reach is how far what a mark traces through points goes past the top
// and bottom edges, height apart: a smooth curve swings past its highest
// and lowest points, never past its control points.
func reach(points []core.Point, curve Curve, height float32) float32 {
	var over float32
	take := func(y core.Points) { over = max(over, -float32(y), float32(y)-height) }
	for _, q := range points {
		take(q.Y)
	}
	if curve == CurveNatural {
		for i := 0; i < len(points)-1; i++ {
			_, y1, _, y2 := controls(points, i)
			take(core.Points(y1))
			take(core.Points(y2))
		}
	}
	return over
}

// Line is a series of a Plot drawn as a line through its points, as
// gpui-base's Line: one point per row with a value, placed by the plot's
// scales, a smooth curve unless Curve says otherwise. It strokes with its
// paint style (Stroke, StrokeWidth, StrokeDash; inherited from the plot),
// the text color where none is set, or with Color its category's of the
// plot's ColorScale, and with Dot shows a view the app styles on every
// point. Its values make the y scale's domain when the plot has none, it
// has a hover dot when it or the plot styles one, and it draws in with the
// plot's appear.
type Line struct {
	// Y is what it draws, one value per row; the plot's Y without.
	Y func() []float64
	// X gives it rows of its own, placed along the plot's x axis by these
	// values on a linear scale and by index on a point scale; without it
	// Y is a column of the plot's rows.
	X func() []float64
	// Scale is the y scale its values go along.
	Scale ValueScale
	Curve Curve
	// Color is the category of each row, which the plot's ColorScale gives
	// a color: a line is one series, so its first row with one gives it.
	Color func() []string
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
func (l Line) Render(cx *core.Scope) core.Node {
	plot := plotOf(cx, "Line")
	l.Curve = l.Curve.or(CurveNatural)
	var series plotSeries
	if plot != nil {
		series = newSeries(cx, plot, l.X, l.Y, nil, l.Scale)
		series.join(cx, plot, func() (float64, bool) { return 0, false }, parts.Of(l.HoverDotClass, l.HoverDotStyle, l.HoverDotStyleFunc))
	}
	bleed := func() float32 {
		if plot == nil {
			return strokeRoom
		}
		return strokeRoom + float32(math.Ceil(float64(reach(series.points.Get(), l.Curve, float32(plot.size.Get().Height)))))
	}
	children := []core.Node{markCanvas(plot, bleed, part{}, func(d *core.Drawing, plot *plotState, b float32) {
		d.StrokeFromView(paintOf(seriesColor(plot, l.Color), core.PaintCurrentColor)).LineWidthFromView().LineDashFromView()
		if plot == nil {
			return
		}
		points := series.points.Get()
		if len(points) == 0 {
			return
		}
		plot.clip(d, b)
		d.BeginPath()
		plot.traced(d, true, l.Scale, func() {
			d.MoveTo(float32(points[0].X), float32(points[0].Y))
			trace(d, points, l.Curve)
		})
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
			yTween := plot.yTweenOf(l.Scale)
			// Its point, while the scales' domains move: the dot moves with
			// them from where the layout puts it.
			x, y := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
			cx.Effect(func() {
				moving := plot.xTween.moving.Get() || yTween.moving.Get()
				points := series.points.Get()
				if i := item(); moving && i < len(points) {
					x.Set(float64(points[i].X))
					y.Set(float64(points[i].Y))
				}
			})
			return core.View{
				Style: core.Style{Position: core.Absolute, Width: core.Pt(0), Height: core.Pt(0),
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
					Animated: &core.AnimatedStyle{TranslateX: plot.xTween.moved(x), TranslateY: yTween.moved(y)}},
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
	return markLayer(parts.Of(l.Class, l.Style, l.StyleFunc), l.TestID, children...)
}
