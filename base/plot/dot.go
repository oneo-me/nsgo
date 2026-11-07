package plot

import (
	"cmp"
	"math"
	"slices"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// RadiusScale is how a Dot's values of Sizes become radii, as d3's
// scaleSqrt: the square roots of Domain onto the radii Min to Max, so with
// a Min of 0 a dot's area is in proportion to its value, as a bubble's
// should be. Values outside the domain take the range's ends.
type RadiusScale struct {
	Min, Max core.Points
	// Domain is the values the range's ends stand for; 0 to the largest
	// value without HasDomain.
	Domain    [2]float64
	HasDomain bool
}

// radius is the radius of value over domain, NaN for no value: d3's signed
// square root, so a domain through 0 keeps its order. A domain of one value
// puts it in the middle of the range.
func (s RadiusScale) radius(value float64, domain [2]float64) float32 {
	if !isFinite(value) {
		return float32(math.NaN())
	}
	root := func(v float64) float64 { return math.Copysign(math.Sqrt(math.Abs(v)), v) }
	lo, hi := root(domain[0]), root(domain[1])
	t := 0.5
	if d := hi - lo; d != 0 && isFinite(d) {
		t = min(max((root(value)-lo)/d, 0), 1)
	}
	return max(float32(s.Min)+float32(s.Max-s.Min)*float32(t), 0)
}

// Dot is a series of a Plot drawn as a dot per row, as Observable Plot's dot
// and Vega-Lite's point: at the row's place along the x axis and its Value
// along the y axis, or across, down a band y scale, its value along x. Its
// radius is R, one for every dot, or Sizes, one per row (a bubble chart, the
// area in proportion to a value with RScale), the bigger dots drawn first.
// It fills with its Fill (inherited from the plot), the text color where
// none is set or with Color each dot's category's, and strokes with its
// Stroke where one is set. Its values count in the value scale's domain when
// the plot has none. The pointer finds the dot whose center is nearest to
// it, among those within HitRadius of their edge, which the plot reports and
// marks with the hover dot, as big as that dot; the keys and screen readers
// step through the dots in the rows' order. With the plot's appear, its dots
// grow from nothing, one after another with Stagger; with Motion.Update they
// move to new values and sizes, new rows' dots grow in and the dots of rows
// that are gone shrink away; while one is hovered, the others fade by
// HoverDim.
type Dot struct {
	// Value is where each dot goes along the value axis; the plot's Y
	// without. A row without a value has no dot.
	Value func() []float64
	// R is the radius of every dot, 3 without it.
	R core.Points
	// Sizes is a value per row in place of R, which RScale makes a radius
	// (the value is the radius without one). A row whose value is missing
	// has no dot.
	Sizes  func() []float64
	RScale *RadiusScale
	// Stagger and HoverDim are a Bar's.
	Stagger, HoverDim float32
	// Color is the category of each dot, which the plot's ColorScale gives
	// a color: its Fill where nothing sets one.
	Color func() []string
	// HitRadius is how far past a dot's edge the pointer still finds it, 8
	// without it; of the dots within it, the one whose center is nearest
	// (Voronoi's cells), so a small bubble over a big one is found around
	// its own center.
	HitRadius core.Points
	// Scale is the y scale its values go along; dots across a band y scale
	// put their values along x.
	Scale ValueScale
	// HoverDotStyle styles its hover dot, a view as big as the dot under the
	// pointer, centered on it; the plot's DotStyle where none is set. There
	// is none where neither styles one.
	HoverDotClass     string
	HoverDotStyle     core.Style
	HoverDotStyleFunc func() core.Style
	Class             string
	Style             core.Style
	StyleFunc         func() core.Style
	TestID            string
}

// dotSpot is a row's dot as the app sets it: where it goes in the plot's
// values, its x on a linear x scale, else NaN, and its radius in points,
// NaN for none.
type dotSpot struct {
	x, value float64
	r        float32
}

func (s dotSpot) shown() bool { return isFinite(s.value) && !math.IsNaN(float64(s.r)) }

// dotPose is the dots as the app sets them, one per row: what Motion.Update
// moves between, in the plot's values, so that a new size moves nothing and
// a new domain moves the dots with the scales.
type dotPose []dotSpot

func (p dotPose) same(o dotPose) bool { return slices.Equal(p, o) }

func (p dotPose) lerp(to dotPose, t float32) dotPose {
	nan := math.NaN()
	none := dotSpot{nan, nan, float32(nan)}
	out := make(dotPose, max(len(p), len(to)))
	f := float64(t)
	for i := range out {
		a, b := none, none
		if i < len(p) {
			a = p[i]
		}
		if i < len(to) {
			b = to[i]
		}
		switch {
		case a.shown() && b.shown():
			out[i] = dotSpot{a.x + (b.x-a.x)*f, a.value + (b.value-a.value)*f, a.r + (b.r-a.r)*t}
		case b.shown():
			b.r *= t
			out[i] = b
		case a.shown():
			a.r *= 1 - t
			out[i] = a
		default:
			out[i] = none
		}
	}
	return out
}

// dotPlaced is a dot as drawn: its row, its center and radius on the plot.
type dotPlaced struct {
	row     int
	x, y, r float32
}

// placedDots is where the rows of p go on the plot's scales, in drawing
// order: the bigger dots first.
func placedDots(plot *plotState, p dotPose, scale ValueScale) []dotPlaced {
	xMap, yMap := plot.xMap.Get(), plot.yMap.Get()
	valuesY := plot.yMapOf(scale)
	across := yMap.kind == axisBand && xMap.kind != axisBand
	out := make([]dotPlaced, 0, len(p))
	for row, s := range p {
		if !s.shown() {
			continue
		}
		along := func(m axisMap) (float32, bool) {
			if m.kind == axisLinear && isFinite(s.x) {
				return m.tick(s.x)
			}
			return m.tick(float64(row))
		}
		var x, y float32
		var okX, okY bool
		if across {
			x, okX = xMap.tick(s.value)
			y, okY = along(yMap)
		} else {
			x, okX = along(xMap)
			y, okY = valuesY.tick(s.value)
		}
		if okX && okY {
			out = append(out, dotPlaced{row, x, y, s.r})
		}
	}
	slices.SortStableFunc(out, func(a, b dotPlaced) int { return cmp.Compare(b.r, a.r) })
	return out
}

// dotAt is the dot nearest to at among dots, by the distance to its center:
// only dots whose edge is reach from at at most count, and on a tie the one
// drawn last.
func dotAt(dots []dotPlaced, at core.Point, reach float32) (int, bool) {
	found, best := -1, float32(math.Inf(1))
	for i := len(dots) - 1; i >= 0; i-- {
		d := dots[i]
		center := float32(math.Hypot(float64(d.x-float32(at.X)), float64(d.y-float32(at.Y))))
		if center-d.r <= reach && center < best {
			found, best = d.row, center
		}
	}
	return found, found >= 0
}

// Render implements core.Node.
func (t Dot) Render(cx *core.Scope) core.Node {
	own := parts.Of(t.Class, t.Style, t.StyleFunc)
	plot := plotOf(cx, "Dot")
	if plot == nil {
		return markLayer(own, t.TestID)
	}
	values := func() []float64 {
		if t.Value != nil {
			return t.Value()
		}
		return plot.y()
	}
	along := func() axis {
		if plot.bandAxis.Get() == axisY {
			return axisX
		}
		return axisOf(t.Scale)
	}
	plot.valueMark(cx, core.NewMemo(cx, func() plotExtent { return extentOf(values()) }), along)
	radius := t.R
	if radius == 0 {
		radius = 3
	}
	hit := t.HitRadius
	if hit == 0 {
		hit = 8
	}
	moves := newMoves(cx, plot, false, func() (dotPose, bool) {
		v := values()
		var xs []float64
		if plot.x != nil {
			xs = plot.x()
		}
		var sizes []float64
		var domain [2]float64
		if t.Sizes != nil {
			sizes = t.Sizes()
			if t.RScale != nil {
				domain = t.RScale.Domain
				if !t.RScale.HasDomain {
					domain = [2]float64{0, extentOf(sizes).hi}
				}
			}
		}
		nan := math.NaN()
		pose := make(dotPose, len(v))
		for row, value := range v {
			x := nan
			if row < len(xs) {
				x = xs[row]
			}
			r := float32(radius)
			if t.Sizes != nil {
				size := nan
				if row < len(sizes) {
					size = sizes[row]
				}
				switch {
				case t.RScale != nil:
					r = t.RScale.radius(size, domain)
				case isFinite(size) && size >= 0:
					r = float32(size)
				default:
					r = float32(nan)
				}
			}
			pose[row] = dotSpot{x, value, r}
		}
		return pose, true
	})
	// Where the dots are at rest: what the pointer finds, where the hover
	// dot goes.
	rest := core.NewMemoFunc(cx, func() []dotPlaced {
		if !plot.sized() {
			return nil
		}
		return placedDots(plot, moves.now().to, t.Scale)
	}, nil)
	plot.addHit(cx, &plotHit{
		at: func(at core.Point) (int, bool) { return dotAt(rest.GetUntracked(), at, float32(hit)) },
		center: func(row int) (core.Point, bool) {
			for _, d := range rest.GetUntracked() {
				if d.row == row {
					return core.Point{X: core.Points(d.x), Y: core.Points(d.y)}, true
				}
			}
			return core.Point{}, false
		},
	})
	hoveredAt := plot.hoveredIndex(cx)
	bleed := func() float32 {
		m := moves.now()
		widest := func(p dotPose) float32 {
			var r float32
			for _, s := range p {
				if !math.IsNaN(float64(s.r)) {
					r = max(r, s.r)
				}
			}
			return r
		}
		r := widest(m.to)
		if m.moving {
			r = max(r, widest(m.from))
		}
		return strokeRoom + float32(math.Ceil(float64(r)))
	}
	canvas := markCanvas(plot, bleed, part{}, func(d *core.Drawing, plot *plotState, _ float32) {
		var colors []core.Color
		hasColors := false
		if plot != nil && t.Color != nil {
			colors, hasColors = plot.categoryColors(t.Color())
		}
		d.StrokeFromView(core.PaintNone).LineWidthFromView().LineDashFromView()
		if !hasColors {
			d.FillFromView(core.PaintCurrentColor)
		}
		if plot == nil {
			return
		}
		m := moves.now()
		to := placedDots(plot, m.to, t.Scale)
		var from []dotPlaced
		count := len(m.to)
		if m.moving {
			from = placedDots(plot, m.from, t.Scale)
			count = max(count, len(m.from))
		}
		appearing := plot.appearing.Get()
		yTween := plot.yTweenOf(t.Scale)
		xMoving, yMoving := plot.xTween.moving.Get(), yTween.moving.Get()
		dim := min(max(t.HoverDim, 0), 1)
		if plot.noHover {
			dim = 0
		}
		find := func(dots []dotPlaced, row int) (dotPlaced, bool) {
			for _, o := range dots {
				if o.row == row {
					return o, true
				}
			}
			return dotPlaced{}, false
		}
		// The dots in the order they draw: those at rest, bigger first, then
		// those of rows that are gone, shrinking away.
		order := make([]int, 0, len(to))
		for _, o := range to {
			order = append(order, o.row)
		}
		for _, o := range from {
			if !slices.Contains(order, o.row) {
				order = append(order, o.row)
			}
		}
		var q core.AnimatedNode
		if m.moving {
			q = moves.progress
		}
		for _, row := range order {
			a, okA := find(from, row)
			b, okB := find(to, row)
			var x, y, r num
			switch {
			case okA && okB:
				x, y, r = lerpNum(q, a.x, b.x), lerpNum(q, a.y, b.y), lerpNum(q, a.r, b.r)
			case okB && m.moving:
				x, y, r = constNum(b.x), constNum(b.y), lerpNum(q, 0, b.r)
			case okB:
				x, y, r = constNum(b.x), constNum(b.y), constNum(b.r)
			default:
				x, y, r = constNum(a.x), constNum(a.y), lerpNum(q, a.r, 0)
			}
			if appearing {
				r = r.mul(nodeNum(plot.grown(row, count, t.Stagger)))
			}
			if xMoving {
				x = plot.xTween.shownNum(x)
			}
			if yMoving {
				y = yTween.shownNum(y)
			}
			if dim > 0 {
				setAlpha(d, plot.indexOpacity(hoveredAt, dim, row))
			}
			if hasColors {
				var c core.Color
				if row < len(colors) {
					c = colors[row]
				}
				d.FillFromView(paintOf(c, core.PaintCurrentColor))
			}
			d.BeginPath()
			arcTo(d, x, y, r, constNum(0), constNum(2*math.Pi), false)
			d.Fill()
			d.Stroke()
		}
	})
	children := []core.Node{canvas}
	hover := parts.Of(t.HoverDotClass, t.HoverDotStyle, t.HoverDotStyleFunc)
	if !plot.noHover && (hover.Set() || plot.dot.Set()) {
		children = append(children, t.hoverDot(cx, plot, rest, hover))
	}
	return markLayer(own, t.TestID, children...)
}

// hoverDot is a Dot's hover dot: as big as the hovered row's dot and
// centered on it, faded with the hover; styled by the dot, or else the plot.
func (t Dot) hoverDot(cx *core.Scope, plot *plotState, rest *core.Memo[[]dotPlaced], own part) core.Node {
	h := plot.hoverPlace(cx, func(row int) (core.Bounds, bool) {
		for _, d := range rest.Get() {
			if d.row == row {
				return core.Bounds{Origin: core.Point{X: core.Points(d.x), Y: core.Points(d.y)},
					Size: core.Size{Width: core.Points(d.r * 2), Height: core.Points(d.r * 2)}}, true
			}
		}
		return core.Bounds{}, false
	})
	yTween := plot.yTweenOf(t.Scale)
	class := own.Class
	style := func() core.Style {
		s := core.Style{}
		if plot.dot.Fn != nil && !own.Set() {
			s = plot.dot.Fn()
		}
		if !own.Set() {
			s = plot.dot.Style.Merge(s)
		}
		s = s.Merge(own.Style)
		if own.Fn != nil {
			s = s.Merge(own.Fn())
		}
		b := h.bounds.Get()
		return s.Merge(core.Style{Width: core.Pt(b.Size.Width), Height: core.Pt(b.Size.Height)})
	}
	if class == "" {
		class = plot.dot.Class
	}
	return dotAnchor(plot.xTween.shown(h.x), yTween.shown(h.y), core.Mul(plot.fade, h.mine), core.View{Class: class, StyleFunc: style})
}
