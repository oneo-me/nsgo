package plot

import (
	"math"
	"slices"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// radialEpsilon: an angle this close to a whole turn is one.
const radialEpsilon = 1e-4

// spokeAngle is where row index of count goes round the turn from start to
// end: evenly, a step of a whole turn's 1/count apart round a whole turn, so
// the last row is a step before the first, as d3's scaleBand places a
// radar's axes, and from start to end over a part of one, as its scalePoint.
func spokeAngle(index, count int, start, end float32) float32 {
	span := end - start
	steps := max(count-1, 0)
	if math.Abs(float64(span)) >= tau-radialEpsilon {
		steps = count
	}
	if steps == 0 {
		return start
	}
	return start + span*float32(index)/float32(steps)
}

// polar is the point at angle radius out from the center.
func polar(cx, cy, angle, radius float32) [2]float32 {
	return [2]float32{cx + radius*sin32(angle), cy - radius*cos32(angle)}
}

// radialDomain is the domain of the plot's linear y scale, which places
// radial values; ok is false on a point or band y scale.
func radialDomain(plot *plotState) ([2]float64, bool) {
	m := plot.yMap.Get()
	return m.linear.ends(), m.kind == axisLinear
}

// radialSpoke is a row of a radial line: its angle and its value.
type radialSpoke struct {
	row   int
	angle float32
	value float64
}

// radialPose is what the app gives a radial line, which its motion moves
// between: its rows' angles and values, the domain that places them and its
// radii. The plot's size is not part of it, so a resize moves nothing.
type radialPose struct {
	spokes       []radialSpoke
	domain       [2]float64
	hasDomain    bool
	inner, outer radiusOf
}

func (p radialPose) same(o radialPose) bool {
	return p.domain == o.domain && p.hasDomain == o.hasDomain && p.inner == o.inner && p.outer == o.outer &&
		slices.Equal(p.spokes, o.spokes)
}

func lerpDomain(a, b [2]float64, t float32) [2]float64 {
	f := float64(t)
	return [2]float64{a[0] + (b[0]-a[0])*f, a[1] + (b[1]-a[1])*f}
}

// lerp is where the line is drawn when a change stops a move part way: each
// row's angle and value t of the way to to's, a row new in to where to has
// it, the domain and the radii too.
func (p radialPose) lerp(to radialPose, t float32) radialPose {
	spokes := make([]radialSpoke, len(to.spokes))
	for i, s := range to.spokes {
		spokes[i] = s
		for _, f := range p.spokes {
			if f.row == s.row {
				spokes[i] = radialSpoke{s.row, f.angle + (s.angle-f.angle)*t, f.value + (s.value-f.value)*float64(t)}
				break
			}
		}
	}
	domain := to.domain
	if p.hasDomain && to.hasDomain {
		domain = lerpDomain(p.domain, to.domain, t)
	}
	return radialPose{spokes, domain, to.hasDomain, p.inner.lerp(to.inner, t), p.outer.lerp(to.outer, t)}
}

// frame is where it draws on a plot of size.
func (p radialPose) frame(size core.Size) radialFrame {
	ring := ringOf(size, p.inner, p.outer)
	f := radialFrame{cx: ring.cx, cy: ring.cy, inner: ring.inner, outer: ring.outer}
	if p.hasDomain {
		m := newLinear(p.domain, [2]float32{ring.inner, ring.outer})
		for _, s := range p.spokes {
			if r, ok := m.tick(s.value); ok {
				f.vertices = append(f.vertices, radialVertex{s.row, s.angle, r})
			}
		}
	}
	return f
}

// radialVertex is a row's vertex: its angle and how far out it is.
type radialVertex struct {
	row           int
	angle, radius float32
}

// radialFrame is a radial line where it draws: its center, its radii and its
// vertices.
type radialFrame struct {
	cx, cy, inner, outer float32
	vertices             []radialVertex
}

func (f radialFrame) pointOf(row int) (core.Point, bool) {
	for _, v := range f.vertices {
		if v.row == row {
			p := polar(f.cx, f.cy, v.angle, v.radius)
			return core.Point{X: core.Points(p[0]), Y: core.Points(p[1])}, true
		}
	}
	return core.Point{}, false
}

// rowAt is the row of the vertex nearest to at by angle, within the outer
// radius; none outside it or without vertices.
func (f radialFrame) rowAt(at core.Point) (int, bool) {
	dx, dy := float64(float32(at.X)-f.cx), float64(float32(at.Y)-f.cy)
	if float32(math.Hypot(dx, dy)) > f.outer {
		return 0, false
	}
	// The screen's angle as a pie angle.
	angle := math.Atan2(dy, dx) + halfPi
	apart := func(a float32) float64 {
		d := math.Mod(angle-float64(a), tau)
		if d < 0 {
			d += tau
		}
		return min(d, tau-d)
	}
	found, best := -1, math.Inf(1)
	for _, v := range f.vertices {
		if d := apart(v.angle); d < best {
			found, best = v.row, d
		}
	}
	return found, found >= 0
}

// grown is the frame it grows out of as it appears: each vertex at the inner
// radius.
func (f radialFrame) grown() radialFrame {
	f.vertices = slices.Clone(f.vertices)
	for i := range f.vertices {
		f.vertices[i].radius = f.inner
	}
	return f
}

// toward is the frame f, an earlier shape, moves to to from, vertex for
// vertex: a row in both from its own place, a new row from the inner radius
// at its angle, so it grows out as its neighbours move.
func (f radialFrame) toward(to radialFrame) radialFrame {
	out := radialFrame{cx: to.cx, cy: to.cy, inner: f.inner, outer: f.outer, vertices: make([]radialVertex, len(to.vertices))}
	for i, v := range to.vertices {
		out.vertices[i] = radialVertex{v.row, v.angle, f.inner}
		for _, w := range f.vertices {
			if w.row == v.row {
				out.vertices[i] = w
				break
			}
		}
	}
	return out
}

// points is its vertices' points, each its own distance out, or radius when
// not negative.
func (f radialFrame) points(radius float32) [][2]float32 {
	out := make([][2]float32, len(f.vertices))
	for i, v := range f.vertices {
		r := v.radius
		if radius >= 0 {
			r = radius
		}
		out[i] = polar(f.cx, f.cy, v.angle, r)
	}
	return out
}

// segKind is a step of a path.
type segKind uint8

const (
	segMove segKind = iota
	segLine
	segCubic
	segClose
)

// pathSeg is a step of a path, its numbers in the plot's coordinates: x and
// y in turn, the last pair the point it goes to.
type pathSeg struct {
	kind segKind
	n    [6]float32
}

func moveSeg(p [2]float32) pathSeg { return pathSeg{segMove, [6]float32{p[0], p[1]}} }
func lineSeg(p [2]float32) pathSeg { return pathSeg{segLine, [6]float32{p[0], p[1]}} }

// tracePoints adds the path through points as curve connects them, closed
// back to the first, starting with a move to the first point, or with join a
// line to it from where the path is.
func tracePoints(out []pathSeg, points [][2]float32, curve Curve, closed, join bool) []pathSeg {
	n := len(points)
	if n == 0 {
		return out
	}
	if join {
		out = append(out, lineSeg(points[0]))
	} else {
		out = append(out, moveSeg(points[0]))
	}
	segments := n - 1
	if closed && n > 2 {
		segments = n
	}
	at := func(i int) [2]float32 { return points[i%n] }
	switch curve {
	case CurveLinear:
		for i := 1; i <= segments; i++ {
			out = append(out, lineSeg(at(i)))
		}
	case CurveStepAfter:
		for i := range segments {
			a, b := at(i), at(i+1)
			out = append(out, lineSeg([2]float32{b[0], a[1]}), lineSeg(b))
		}
	default:
		// Each segment a cubic through its neighbours' tangents; closed, the
		// neighbours wrap round, open, the ends are their own neighbours.
		for i := range segments {
			p0, p3 := points[max(i-1, 0)], points[min(i+2, n-1)]
			if closed {
				p0, p3 = at(i+n-1), at(i+2)
			}
			p1, p2 := at(i), at(i+1)
			out = append(out, pathSeg{segCubic, [6]float32{p1[0] + (p2[0]-p0[0])/6, p1[1] + (p2[1]-p0[1])/6,
				p2[0] - (p3[0]-p1[0])/6, p2[1] - (p3[1]-p1[1])/6, p2[0], p2[1]}})
		}
	}
	if closed && n > 2 {
		out = append(out, pathSeg{kind: segClose})
	}
	return out
}

// radialOutline is what a radial line draws of a frame: its line, and its
// area down to the inner radius.
type radialOutline struct{ line, area []pathSeg }

// outlineOf is the outline of f; hole keeps its area clear of the inner
// radius, else it closes to the center.
func outlineOf(f radialFrame, curve Curve, closed, hole bool) radialOutline {
	outer := f.points(-1)
	line := tracePoints(nil, outer, curve, closed, false)
	area := slices.Clone(line)
	if len(outer) > 1 {
		inner := f.points(f.inner)
		slices.Reverse(inner)
		ring := closed && len(outer) > 2
		switch {
		case ring && hole:
			// A ring: the inner loop the other way round.
			area = tracePoints(area, inner, curve, true, false)
		case ring:
		case hole:
			area = append(tracePoints(area, inner, curve, false, true), pathSeg{kind: segClose})
		default:
			area = append(area, lineSeg([2]float32{f.cx, f.cy}), pathSeg{kind: segClose})
		}
	}
	return radialOutline{line, area}
}

// reachPast is how far it reaches past the edges of a plot of size.
func (o radialOutline) reachPast(size core.Size) float32 {
	w, h := float32(size.Width), float32(size.Height)
	var over float32
	for _, s := range o.line {
		count := 0
		switch s.kind {
		case segMove, segLine:
			count = 2
		case segCubic:
			count = 6
		}
		for i := 0; i < count; i += 2 {
			x, y := s.n[i], s.n[i+1]
			over = max(over, -x, x-w, -y, y-h)
		}
	}
	return over
}

// recordPath records the path of to, moving from from, step for step, as t
// goes from 0 to 1.
func recordPath(d *core.Drawing, to, from []pathSeg, t core.AnimatedNode) {
	for i, seg := range to {
		at := func(k int) num {
			if t != nil && i < len(from) {
				return lerpNum(t, from[i].n[k], seg.n[k])
			}
			return constNum(seg.n[k])
		}
		switch seg.kind {
		case segMove:
			moveTo(d, at(0), at(1))
		case segLine:
			lineTo(d, at(0), at(1))
		case segCubic:
			cubicTo(d, at(0), at(1), at(2), at(3), at(4), at(5))
		case segClose:
			d.ClosePath()
		}
	}
}

// RadialLine is a series of a Plot around its center, as gpui-base's
// RadialLine and d3's lineRadial and areaRadial: a point per row with a
// value, at the row's angle and as far out as the plot's y scale puts its
// value between InnerRadius and OuterRadius, the rows without one skipped;
// Closed joins the last point to the first, a radar's polygon. It strokes
// its line with its Stroke, the text color where none is set, and fills its
// area down to InnerRadius with its Fill, none where none is set, over its
// background; its paint inherits from the plot as CSS's does. Its values
// count in the y scale's domain when the plot has none, with 0, the center.
// Over it, the plot reports the row whose angle is nearest the pointer's
// within the outer radius, and its hover dot sits on that row's point. It
// grows out of its inner radius with the plot's appear, and moves to new
// values, angles and radii with Motion.Update.
type RadialLine struct {
	// Radius places each row out from the center, values of the plot's y
	// scale, from InnerRadius at the domain's smallest to OuterRadius at its
	// largest; the plot's Y without. NaN skips a row.
	Radius func() []float64
	// Angle is where each row goes round the center, radians from 12
	// o'clock, clockwise. Without it, the rows go evenly from StartAngle to
	// EndAngle, in order.
	Angle func() []float64
	// StartAngle and EndAngle are 0 and a whole turn without them. Over a
	// part of a turn the last row is on EndAngle; round a whole one, a step
	// before the first.
	StartAngle, EndAngle func() float32
	// InnerRadius and OuterRadius are a Pie's.
	InnerRadius, OuterRadius func() core.Length
	// Curve is straight segments by default.
	Curve  Curve
	Closed bool
	// Dot shows a view styled by DotStyle centered on every point.
	Dot          bool
	DotClass     string
	DotStyle     core.Style
	DotStyleFunc func() core.Style
	// BackgroundStyle's Background and Gradients fill a box around its outer
	// circle, shown only inside its area, under its Fill, as an Area's.
	BackgroundClass     string
	BackgroundStyle     core.Style
	BackgroundStyleFunc func() core.Style
	// HoverDotStyle styles its hover dot, on its point at the row under the
	// pointer; the plot's DotStyle where none is set.
	HoverDotClass     string
	HoverDotStyle     core.Style
	HoverDotStyleFunc func() core.Style
	Class             string
	Style             core.Style
	StyleFunc         func() core.Style
	TestID            string
}

// Render implements core.Node.
func (r RadialLine) Render(cx *core.Scope) core.Node {
	own := parts.Of(r.Class, r.Style, r.StyleFunc)
	plot := plotOf(cx, "RadialLine")
	if plot == nil {
		return markLayer(own, r.TestID)
	}
	curve := r.Curve.or(CurveLinear)
	radiiValues := func() []float64 {
		if r.Radius != nil {
			return r.Radius()
		}
		return plot.y()
	}
	// Its values, and the center's 0, in the y domain; its hover dot is its
	// own, not one the plot places along the x axis.
	plot.valueMark(cx, core.NewMemo(cx, func() plotExtent { return extentOf(radiiValues()).with(0) }),
		func() axis { return axisY })
	moves := newMoves(cx, plot, false, func() (radialPose, bool) {
		count := plot.rows()
		values := radiiValues()
		var angles []float64
		if r.Angle != nil {
			angles = r.Angle()
		}
		start, end := angleOr(r.StartAngle, 0), angleOr(r.EndAngle, tau)
		var spokes []radialSpoke
		for i := range count {
			if i >= len(values) || !isFinite(values[i]) {
				continue
			}
			angle := spokeAngle(i, count, start, end)
			if r.Angle != nil {
				if i >= len(angles) || !isFinite(angles[i]) {
					continue
				}
				angle = float32(angles[i])
			}
			spokes = append(spokes, radialSpoke{i, angle, values[i]})
		}
		domain, ok := radialDomain(plot)
		in, out := radii(r.InnerRadius, r.OuterRadius)
		return radialPose{spokes, domain, ok, in, out}, true
	})
	// Where it draws at rest: the pose its motion moves to.
	rest := core.NewMemoFunc(cx, func() radialFrame { return moves.now().to.frame(plot.size.Get()) }, nil)
	plot.addHit(cx, &plotHit{
		at:     func(at core.Point) (int, bool) { return rest.GetUntracked().rowAt(at) },
		center: func(row int) (core.Point, bool) { return rest.GetUntracked().pointOf(row) },
	})
	// Its outlines now: at rest, or the two a motion moves between, with the
	// same steps, a hole in both if either has one.
	outlines := func() (radialOutline, radialOutline, core.AnimatedNode) {
		f := rest.Get()
		var from radialFrame
		var t core.AnimatedNode
		if plot.appearing.Get() {
			from, t = f.grown(), plot.appear.progress()
		} else if m := moves.now(); m.moving {
			from, t = m.from.frame(plot.size.Get()).toward(f), moves.progress
		}
		hole := f.inner > 0 || t != nil && from.inner > 0
		to := outlineOf(f, curve, r.Closed, hole)
		if t == nil {
			return to, radialOutline{}, nil
		}
		return to, outlineOf(from, curve, r.Closed, hole), t
	}
	bleed := func() float32 {
		f := rest.Get()
		o := outlineOf(f, curve, r.Closed, f.inner > 0)
		return strokeRoom + float32(math.Ceil(float64(o.reachPast(plot.size.Get()))))
	}
	var children []core.Node
	background := parts.Of(r.BackgroundClass, r.BackgroundStyle, r.BackgroundStyleFunc)
	if background.Set() {
		children = append(children, markCanvas(plot, bleed, background, func(d *core.Drawing, plot *plotState, _ float32) {
			if plot == nil {
				nothing(d)
				return
			}
			to, from, t := outlines()
			if len(to.line) == 0 {
				nothing(d)
				return
			}
			// A box round its outer circle: a radial gradient centers on the
			// plot's center.
			f := rest.Get()
			area := core.Bounds{Origin: core.Point{X: core.Points(f.cx - f.outer), Y: core.Points(f.cy - f.outer)},
				Size: core.Size{Width: core.Points(2 * f.outer), Height: core.Points(2 * f.outer)}}
			d.BeginPath()
			recordPath(d, to.area, from.area, t)
			d.FillBackgroundFromView(area)
		}))
	}
	children = append(children, markCanvas(plot, bleed, part{}, func(d *core.Drawing, plot *plotState, _ float32) {
		d.FillFromView(core.PaintNone)
		if plot == nil {
			return
		}
		to, from, t := outlines()
		if len(to.line) == 0 {
			return
		}
		d.BeginPath()
		recordPath(d, to.area, from.area, t)
		d.Fill()
		d.StrokeFromView(core.PaintCurrentColor).LineWidthFromView().LineDashFromView().BeginPath()
		recordPath(d, to.line, from.line, t)
		d.Stroke()
	}))
	if r.Dot {
		children = append(children, core.Each(func() []int {
			f := rest.Get()
			out := make([]int, len(f.vertices))
			for i, v := range f.vertices {
				out[i] = v.row
			}
			return out
		}, func(row int) int { return row }, func(cx *core.Scope, item func() int, _ func() int) core.Node {
			return r.dotView(cx, plot, rest, moves, item())
		}))
	}
	if hover := parts.Of(r.HoverDotClass, r.HoverDotStyle, r.HoverDotStyleFunc); !plot.noHover && (hover.Set() || plot.dot.Set()) {
		h := plot.hoverPlace(cx, func(row int) (core.Bounds, bool) {
			p, ok := rest.Get().pointOf(row)
			return core.Bounds{Origin: p}, ok
		})
		class := hover.Class
		if class == "" {
			class = plot.dot.Class
		}
		children = append(children, dotAnchor(h.x, h.y, core.Mul(plot.fade, h.mine), core.View{Class: class,
			StyleFunc: func() core.Style {
				s := plot.dot.Style
				if plot.dot.Fn != nil {
					s = s.Merge(plot.dot.Fn())
				}
				s = s.Merge(hover.Style)
				if hover.Fn != nil {
					s = s.Merge(hover.Fn())
				}
				return s
			}}))
	}
	return markLayer(own, r.TestID, children...)
}

// dotView is the dot on row's point. While the line appears or moves it
// shows from where the motion starts, the distance closing on the frame
// clock as the line's points move.
func (r RadialLine) dotView(cx *core.Scope, plot *plotState, rest *core.Memo[radialFrame], moves *moves[radialPose], row int) core.Node {
	v := func() *core.AnimatedValue { return core.NewAnimatedValue(cx, 0) }
	ax, ay, ux, uy := v(), v(), v(), v()
	cx.Effect(func() {
		f := rest.Get()
		at, ok := f.pointOf(row)
		if !ok {
			return
		}
		off := func(from core.Point, has bool, x, y *core.AnimatedValue) {
			if !has {
				from = at
			}
			x.Set(float64(from.X - at.X))
			y.Set(float64(from.Y - at.Y))
		}
		appearing := plot.appearing.Get()
		grown, hasGrown := f.grown().pointOf(row)
		off(grown, appearing && hasGrown, ax, ay)
		var before core.Point
		hasBefore := false
		if m := moves.now(); m.moving && !appearing {
			before, hasBefore = m.from.frame(plot.size.Get()).toward(f).pointOf(row)
		}
		off(before, hasBefore, ux, uy)
	})
	leftA := core.Sub(core.Constant(1), plot.appear.progress())
	leftU := moves.left()
	return core.View{
		Style: core.Style{Position: core.Absolute, Width: pts(0), Height: pts(0), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Animated: &core.AnimatedStyle{TranslateX: core.Add(core.Mul(ax, leftA), core.Mul(ux, leftU)),
				TranslateY: core.Add(core.Mul(ay, leftA), core.Mul(uy, leftU))}},
		StyleFunc: func() core.Style {
			if p, ok := rest.Get().pointOf(row); ok {
				return core.Style{Left: core.Pt(p.X), Top: core.Pt(p.Y)}
			}
			return core.Style{}
		},
		PointerEvents: core.PointerEventsNone,
		Children:      []core.Node{core.View{Class: r.DotClass, Style: r.DotStyle, StyleFunc: r.DotStyleFunc}},
	}
}

// radialGridShape is what a radial grid's motion moves between: the domain
// placing its rings and its radii.
type radialGridShape struct {
	domain       [2]float64
	hasDomain    bool
	inner, outer radiusOf
}

func (s radialGridShape) same(o radialGridShape) bool { return s == o }

func (s radialGridShape) lerp(to radialGridShape, t float32) radialGridShape {
	domain := to.domain
	if s.hasDomain && to.hasDomain {
		domain = lerpDomain(s.domain, to.domain, t)
	}
	return radialGridShape{domain, to.hasDomain, s.inner.lerp(to.inner, t), s.outer.lerp(to.outer, t)}
}

// RadialGrid is a radar's grid in a Plot, Grid's polar counterpart: rings
// around the plot's center at the values Rings of its y scale, polygons
// through the rows' angles or Circular, and with Spokes a line out from the
// inner radius at each row's angle. It strokes with its paint style,
// inherited from the plot, the text color where none is set. Its values do
// not count in the scale's domain; its rings move with the domain and the
// radii over Motion.Update, with the radial lines.
type RadialGrid struct {
	// Rings is where its rings go, as values of the plot's y scale.
	Rings func() []float64
	// Spokes draws a spoke at each row's angle, the rows evenly from
	// StartAngle to EndAngle as a RadialLine without an Angle places them.
	Spokes bool
	// Circular makes its rings circles, arcs over a part of a turn; by
	// default they are polygons through the rows' angles, circles with
	// fewer than three rows.
	Circular bool
	// StartAngle, EndAngle, InnerRadius and OuterRadius are a RadialLine's.
	StartAngle, EndAngle     func() float32
	InnerRadius, OuterRadius func() core.Length
	Class                    string
	Style                    core.Style
	StyleFunc                func() core.Style
	TestID                   string
}

// Render implements core.Node.
func (g RadialGrid) Render(cx *core.Scope) core.Node {
	own := parts.Of(g.Class, g.Style, g.StyleFunc)
	plot := plotOf(cx, "RadialGrid")
	if plot == nil {
		return markLayer(own, g.TestID)
	}
	moves := newMoves(cx, plot, false, func() (radialGridShape, bool) {
		domain, ok := radialDomain(plot)
		in, out := radii(g.InnerRadius, g.OuterRadius)
		return radialGridShape{domain, ok, in, out}, true
	})
	bleed := func() float32 {
		in, out := radii(g.InnerRadius, g.OuterRadius)
		ring := ringOf(plot.size.Get(), in, out)
		size := plot.size.Get()
		half := float32(min(size.Width, size.Height)) / 2
		return strokeRoom + float32(math.Ceil(float64(max(max(ring.outer, ring.inner)-half, 0))))
	}
	return markLayer(own, g.TestID, markCanvas(plot, bleed, part{}, func(d *core.Drawing, plot *plotState, _ float32) {
		d.StrokeFromView(core.PaintCurrentColor).LineWidthFromView().LineDashFromView().BeginPath()
		if plot == nil {
			return
		}
		size := plot.size.Get()
		m := moves.now()
		ring := ringOf(size, m.to.inner, m.to.outer)
		fromRing := ring
		var t core.AnimatedNode
		if m.moving {
			fromRing, t = ringOf(size, m.from.inner, m.from.outer), moves.progress
		}
		mapOf := func(s radialGridShape, r pieRing) (linearMap, bool) {
			return newLinear(s.domain, [2]float32{r.inner, r.outer}), s.hasDomain
		}
		toMap, hasTo := mapOf(m.to, ring)
		fromMap, hasFrom := mapOf(m.from, fromRing)
		hasFrom = hasFrom && m.moving
		// A point r out at angle, moving out from where r0 put it.
		at := func(angle, r0, r float32) (num, num) {
			a, b := polar(ring.cx, ring.cy, angle, r0), polar(ring.cx, ring.cy, angle, r)
			return lerpNum(t, a[0], b[0]), lerpNum(t, a[1], b[1])
		}
		count := plot.rows()
		a0, a1 := angleOr(g.StartAngle, 0), angleOr(g.EndAngle, tau)
		whole := math.Abs(float64(a1-a0)) >= tau-radialEpsilon
		angles := make([]float32, count)
		for i := range angles {
			angles[i] = spokeAngle(i, count, a0, a1)
		}
		fewest := 2
		if whole {
			fewest = 3
		}
		polygon := !g.Circular && len(angles) >= fewest
		if hasTo && g.Rings != nil {
			for _, v := range g.Rings() {
				r, ok := toMap.tick(v)
				if !ok {
					continue
				}
				r0 := r
				if hasFrom {
					if q, ok := fromMap.tick(v); ok {
						r0 = q
					}
				}
				if polygon {
					for i, a := range angles {
						x, y := at(a, r0, r)
						if i == 0 {
							moveTo(d, x, y)
						} else {
							lineTo(d, x, y)
						}
					}
					if whole {
						d.ClosePath()
					}
					continue
				}
				x, y := at(a0, r0, r)
				s, e := a0+screenAngle, a1+screenAngle
				if whole {
					e = s + tau
				}
				moveTo(d, x, y)
				arcTo(d, constNum(ring.cx), constNum(ring.cy), lerpNum(t, r0, r), constNum(s), constNum(e), e < s)
			}
		}
		if g.Spokes {
			for _, a := range angles {
				x, y := at(a, fromRing.inner, ring.inner)
				moveTo(d, x, y)
				x, y = at(a, fromRing.outer, ring.outer)
				lineTo(d, x, y)
			}
		}
		d.Stroke()
	}))
}
