package plot

import (
	"math"
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// Angles are radians, 0 at 12 o'clock and growing clockwise, as gpui-base's
// and d3's are; radii are points or a share of the largest circle the plot
// holds.

const (
	tau        = 2 * math.Pi
	halfPi     = math.Pi / 2
	pieEpsilon = 1e-6
	// screenAngle is the canvas's angle of a pie angle's 0: 3 o'clock is the
	// canvas's 0.
	screenAngle = -halfPi
	// motionSamples is the samples of a moving slice's outline.
	motionSamples = 64
	// labelAppearStart is how far into the appear a pie's labels start
	// fading in: once their slices are mostly drawn.
	labelAppearStart = 0.7
	// labelMinSpan: a slice narrower than this has no label.
	labelMinSpan = math.Pi / 360
)

func sin32(a float32) float32 { return float32(math.Sin(float64(a))) }
func cos32(a float32) float32 { return float32(math.Cos(float64(a))) }

// radiusOf is a radius as a plot's size resolves it: points, and a share of
// the largest circle the plot holds (half its smaller side), so a radius on
// its way from a percentage to points resolves at any size.
type radiusOf struct{ points, share float32 }

func radiusFrom(l core.Length) radiusOf {
	if pct, ok := l.Percent(); ok {
		return radiusOf{share: pct / 100}
	}
	p, _ := l.Points()
	return radiusOf{points: float32(p)}
}

func (r radiusOf) at(half float32) float32 { return max(r.points+r.share*half, 0) }

func (r radiusOf) lerp(to radiusOf, t float32) radiusOf {
	return radiusOf{r.points + (to.points-r.points)*t, r.share + (to.share-r.share)*t}
}

// radii reads a mark's radii, the outer one 100% without one.
func radii(inner, outer func() core.Length) (radiusOf, radiusOf) {
	in, out := radiusOf{}, radiusOf{share: 1}
	if inner != nil {
		in = radiusFrom(inner())
	}
	if outer != nil {
		out = radiusFrom(outer())
	}
	return in, out
}

// angleOr is what f reads, or other without one.
func angleOr(f func() float32, other float32) float32 {
	if f == nil {
		return other
	}
	return f()
}

// pieRing is where a radial mark draws: the plot's center and its radii.
type pieRing struct {
	cx, cy, inner, outer float32
}

func ringOf(size core.Size, inner, outer radiusOf) pieRing {
	w, h := float32(size.Width), float32(size.Height)
	half := min(w, h) / 2
	return pieRing{w / 2, h / 2, inner.at(half), outer.at(half)}
}

// reachPast is how far a mark of this ring, its outer edge lift further
// out, reaches past the edges of a plot of size.
func (r pieRing) reachPast(size core.Size, lift float32) float32 {
	return max(r.outer+lift-float32(min(size.Width, size.Height))/2, 0)
}

// pieSlice is a slice of a pie: its row and the angles it spans.
type pieSlice struct {
	row        int
	start, end float32
}

// pieArcs is the slices of values from start to end: one per row with a
// value above zero, in the rows' order, each spanning its share of the
// angle.
func pieArcs(values []float64, start, end float32) []pieSlice {
	sum := 0.0
	for _, v := range values {
		if v > 0 && isFinite(v) {
			sum += v
		}
	}
	var out []pieSlice
	k := start
	for row, v := range values {
		if v > 0 && isFinite(v) {
			from := k
			if sum > 0 {
				k += float32(v/sum) * (end - start)
			}
			out = append(out, pieSlice{row, from, k})
		}
	}
	return out
}

// sliceAt is the row of the slice under at: within a slice's angles and
// between the ring's radii. The angles are read from the pie's own first
// angle, from, so a pie that starts before 12 o'clock or turns the other way
// finds its slices too.
func (r pieRing) sliceAt(slices []pieSlice, from float32, at core.Point) (int, bool) {
	dx, dy := float64(float32(at.X)-r.cx), float64(float32(at.Y)-r.cy)
	radius := float32(math.Hypot(dx, dy))
	if radius < r.inner || radius > r.outer {
		return 0, false
	}
	turn := math.Mod(math.Atan2(dy, dx)+halfPi-float64(from), tau)
	if turn < 0 {
		turn += tau
	}
	angle := from + float32(turn)
	for _, s := range slices {
		if angle >= min(s.start, s.end) && angle < max(s.start, s.end) {
			return s.row, true
		}
	}
	return 0, false
}

// middle is the middle of slice: halfway between the ring's radii, at the
// middle of its angles.
func (r pieRing) middle(s pieSlice) core.Point {
	radius, angle := (r.inner+r.outer)/2, (s.start+s.end)/2
	return core.Point{X: core.Points(r.cx + radius*sin32(angle)), Y: core.Points(r.cy - radius*cos32(angle))}
}

// sectorShape is a radial mark's shape at a moment.
type sectorShape struct{ inner, outer, lo, hi float32 }

func (s sectorShape) lerp(to sectorShape, t float32) sectorShape {
	at := func(a, b float32) float32 { return a + (b-a)*t }
	return sectorShape{at(s.inner, to.inner), at(s.outer, to.outer), at(s.lo, to.lo), at(s.hi, to.hi)}
}

// arcPose is an arc's pose: its radii and the angles it spans, lo before hi.
type arcPose struct {
	inner, outer radiusOf
	lo, hi       float32
}

func newArcPose(inner, outer radiusOf, start, end float32) arcPose {
	return arcPose{inner, outer, min(start, end), max(start, end)}
}

func (a arcPose) shape(size core.Size) sectorShape {
	r := ringOf(size, a.inner, a.outer)
	return sectorShape{r.inner, r.outer, a.lo, a.hi}
}

func (a arcPose) same(o arcPose) bool { return a == o }

func (a arcPose) lerp(to arcPose, t float32) arcPose {
	at := func(x, y float32) float32 { return x + (y-x)*t }
	return arcPose{a.inner.lerp(to.inner, t), a.outer.lerp(to.outer, t), at(a.lo, to.lo), at(a.hi, to.hi)}
}

// piePose is a pie's pose: its radii, its first angle and its slices.
type piePose struct {
	inner, outer radiusOf
	start        float32
	slices       []pieSlice
}

func (p piePose) same(o piePose) bool {
	return p.inner == o.inner && p.outer == o.outer && p.start == o.start && slices.Equal(p.slices, o.slices)
}

func (p piePose) slice(row int) (arcPose, bool) {
	for _, s := range p.slices {
		if s.row == row {
			return newArcPose(p.inner, p.outer, s.start, s.end), true
		}
	}
	return arcPose{}, false
}

// sliceFrom is where row's slice moves from among the slices before a
// change: its own angles, or for a slice new to the pie, none at all where
// the slice before it ended, so it opens as its neighbours make room.
func sliceFrom(from []pieSlice, row int, start float32) (float32, float32) {
	at := start
	for _, s := range from {
		if s.row == row {
			return s.start, s.end
		}
		if s.row < row {
			at = s.end
		}
	}
	return at, at
}

func (p piePose) lerp(to piePose, t float32) piePose {
	at := func(a, b float32) float32 { return a + (b-a)*t }
	out := make([]pieSlice, len(to.slices))
	for i, s := range to.slices {
		a, b := sliceFrom(p.slices, s.row, to.start)
		out[i] = pieSlice{s.row, at(a, s.start), at(b, s.end)}
	}
	return piePose{p.inner.lerp(to.inner, t), p.outer.lerp(to.outer, t), to.start, out}
}

// shapeMove is a radial mark's move to the shape it draws: from from, as
// far as progress has gone.
type shapeMove struct {
	progress core.AnimatedNode
	from     sectorShape
}

// shapeMoveOf is how a mark of to moves now: it sweeps in from start while
// the plot appears, else it moves from from while its update runs.
func shapeMoveOf(plot *plotState, to sectorShape, start float32, from *shapeMove) *shapeMove {
	if plot.appearing.Get() {
		return &shapeMove{plot.appear.progress(), sectorShape{to.inner, to.outer, start, start}}
	}
	return from
}

// pieCorner is a pair of corners of a slice, one at each of its edges, as
// d3's cornerTangents: circles of radius, 0 for a sharp corner, whose
// centers lie distance from the ring's center, angle in from the slice's
// edges.
type pieCorner struct{ radius, distance, angle float32 }

// pieOutline is what a slice draws: its angles, and its corners at the rim
// and at the hole.
type pieOutline struct {
	lo, hi       float32
	outer, inner pieCorner
}

func asin32(v float32) float32 { return float32(math.Asin(float64(min(v, 1)))) }

// newOutline is the outline of shape, pad between it and its neighbours,
// its corners rounded by up to corner; ok is false when it is narrower than
// its pad. As d3 does, a corner is at most half the ring's thickness, and
// smaller where the slice has no angle for it. A whole turn has no corners.
func newOutline(shape sectorShape, pad, corner float32) (pieOutline, bool) {
	lo, hi := shape.lo, shape.hi
	span := hi - lo
	r1 := shape.outer
	if r1 < pieEpsilon || span < pieEpsilon {
		return pieOutline{}, false
	}
	e := newEdges(shape.inner, r1, pad)
	r0, h := e.r0, e.h
	// The sine of half the slice: its edges meet h/half out.
	half := sin32(span / 2)
	if span < math.Pi && r1*half <= h {
		return pieOutline{}, false
	}
	rc := float32(0)
	if span < tau-pieEpsilon {
		rc = max(min(corner, (r1-r0)/2, (r1-h)/2), 0)
	}
	// The largest corner whose circle fits between the edges and the rim.
	rc1 := rc
	if span < math.Pi {
		rc1 = max(min(rc, (r1*half-h)/(1+half)), 0)
	}
	d1 := r1 - rc1
	outer := pieCorner{rc1, d1, min(asin32((h+rc1)/d1), span/2)}
	radius, off := e.inner(span)
	inner := pieCorner{0, radius, off}
	// On the hole: round its corners too, the circle outside it.
	if rc > 0 && r0 > h && radius == r0 {
		rc0 := rc
		if span < math.Pi {
			rc0 = max(min(rc, (r0*half-h)/(1-half)), 0)
		}
		d0 := r0 + rc0
		inner = pieCorner{rc0, d0, min(asin32((h+rc0)/d0), span/2)}
	}
	return pieOutline{lo, hi, outer, inner}, true
}

// pointOutline is what a moving slice narrower than its pad draws: a point
// on the middle of its rim.
func pointOutline(shape sectorShape) pieOutline {
	middle := pieCorner{0, shape.outer, max(shape.hi-shape.lo, 0) / 2}
	return pieOutline{shape.lo, shape.hi, middle, middle}
}

func (o pieOutline) rounded() bool { return o.outer.radius > 0 || o.inner.radius > 0 }

// pieEdges is the edges of the slices of a ring whose gaps are pad wide at
// the rim and as wide at every radius: h is half that width.
type pieEdges struct{ r0, r1, h float32 }

func newEdges(r0, r1, pad float32) pieEdges { return pieEdges{max(r0, 0), r1, r1 * sin32(pad/2)} }

// inner is the inner edge of a slice spanning span: the radius it lies on
// and how far its ends are turned in from the slice's angles. On the hole
// where the slice reaches it; a slice narrower than a half turn that does
// not ends where its edges meet, a wider one rounds the center at half the
// gap's width.
func (e pieEdges) inner(span float32) (float32, float32) {
	if e.h <= 0 {
		return e.r0, 0
	}
	if span < math.Pi {
		// Where the two edges meet, on the slice's middle.
		meet := float32(math.Inf(1))
		if s := sin32(span / 2); s > pieEpsilon {
			meet = e.h / s
		}
		if meet >= e.r1 {
			return e.r1, span / 2
		}
		if meet > e.r0 {
			return meet, span / 2
		}
	}
	r := max(e.r0, e.h)
	return r, asin32(e.h / r)
}

// samples is what a radial mark draws over its motion, T of each sampled
// shape: each number of its drawing is a constant, or the frame clock's
// piecewise-linear map of the motion's progress through the numbers of the
// samples, so angles, radii and corners move together.
type samples[T any] struct {
	progress core.AnimatedNode
	ts       []float64
	all      []T
}

// sampled is f of to, or of its moves under m, sampled densely near the
// start, where a sweep opens a slice from nothing.
func sampled[T any](to sectorShape, m *shapeMove, f func(sectorShape) T) samples[T] {
	if m == nil {
		return samples[T]{ts: []float64{1}, all: []T{f(to)}}
	}
	s := samples[T]{progress: m.progress}
	for i := range motionSamples + 1 {
		t := float64(i) / motionSamples
		t *= t
		s.ts = append(s.ts, t)
		s.all = append(s.all, f(m.from.lerp(to, float32(t))))
	}
	return s
}

// numOf is f of the samples: a constant where it does not move.
func numOf[T any](s samples[T], f func(T) float32) num {
	values := make([]float64, len(s.all))
	moves := false
	first := f(s.all[0])
	for i, v := range s.all {
		x := f(v)
		values[i] = float64(x)
		moves = moves || x != first
	}
	if s.progress == nil || !moves {
		return constNum(first)
	}
	return nodeNum(core.Interpolate(s.progress, core.Interpolation{Input: s.ts, Output: values}))
}

// outlineAngles is an arc's start and its end length past it, the length
// never below 0, so the canvas never takes a sliver for a whole turn the
// other way.
func outlineAngles(s samples[pieOutline], start, length func(pieOutline) float32, ccw bool) (num, num) {
	a0 := numOf(s, func(o pieOutline) float32 { return start(o) + screenAngle })
	l := numOf(s, func(o pieOutline) float32 { return max(length(o), 0) })
	if ccw {
		return a0, a0.sub(l)
	}
	return a0, a0.add(l)
}

// cornerCenter is the center of a corner distance out at angle, pushed out
// by lift along the way there.
func cornerCenter(s samples[pieOutline], cx, cy float32, angle, distance func(pieOutline) float32, lift *num) (num, num) {
	x := numOf(s, func(o pieOutline) float32 { return cx + distance(o)*cos32(angle(o)+screenAngle) })
	y := numOf(s, func(o pieOutline) float32 { return cy + distance(o)*sin32(angle(o)+screenAngle) })
	if lift == nil {
		return x, y
	}
	cos := numOf(s, func(o pieOutline) float32 { return cos32(angle(o) + screenAngle) })
	sin := numOf(s, func(o pieOutline) float32 { return sin32(angle(o) + screenAngle) })
	return x.add(lift.mul(cos)), y.add(lift.mul(sin))
}

// sector records an annular sector of to around (cx, cy), its outer edge
// lift past the ring's, pad between it and its neighbours, its corners
// rounded by up to corner. The gaps keep the width the pad makes at the rim
// at every radius: each edge runs parallel to its radial, offset by half
// that width. Where a slice is too narrow to reach the hole its edges meet
// in a point short of the center; a slice narrower than its pad draws
// nothing. A whole turn takes no pad and no corners, as d3's. While m runs
// the shape moves from its from. ok is false with nothing to draw.
func sector(d *core.Drawing, cx, cy float32, to sectorShape, lift *num, pad, corner float32, m *shapeMove) bool {
	if to.hi-to.lo >= tau-pieEpsilon {
		pad = 0
	} else {
		pad = min(max(pad, 0), math.Pi)
	}
	var o samples[pieOutline]
	if m == nil {
		at, ok := newOutline(to, pad, corner)
		if !ok {
			return false
		}
		o = sampled(to, nil, func(sectorShape) pieOutline { return at })
	} else {
		o = sampled(to, m, func(shape sectorShape) pieOutline {
			if at, ok := newOutline(shape, pad, corner); ok {
				return at
			}
			return pointOutline(shape)
		})
	}
	lifted := func(v num) num {
		if lift != nil {
			return v.add(*lift)
		}
		return v
	}
	span := func(o pieOutline) float32 { return o.hi - o.lo }
	center := func(v float32) num { return constNum(v) }
	d.BeginPath()
	if !slices.ContainsFunc(o.all, pieOutline.rounded) {
		rim := lifted(numOf(o, func(o pieOutline) float32 { return o.outer.distance }))
		a0, a1 := outlineAngles(o, func(o pieOutline) float32 { return o.lo + o.outer.angle },
			func(o pieOutline) float32 { return span(o) - 2*o.outer.angle }, false)
		arcTo(d, center(cx), center(cy), rim, a0, a1, false)
		a0, a1 = outlineAngles(o, func(o pieOutline) float32 { return o.hi - o.inner.angle },
			func(o pieOutline) float32 { return span(o) - 2*o.inner.angle }, true)
		arcTo(d, center(cx), center(cy), numOf(o, func(o pieOutline) float32 { return o.inner.distance }), a0, a1, true)
		d.ClosePath()
		return true
	}
	// Clockwise around the slice: the corner at its start's rim, the rim,
	// the corner at its end's; the corner at its end's hole, the hole's edge
	// back, the corner at its start's. Each corner turns from its edge, a
	// quarter turn off the edge's radial, to the circle it meets.
	outerRadius := numOf(o, func(o pieOutline) float32 { return o.outer.radius })
	innerRadius := numOf(o, func(o pieOutline) float32 { return o.inner.radius })
	outerDist := func(o pieOutline) float32 { return o.outer.distance }
	innerDist := func(o pieOutline) float32 { return o.inner.distance }
	x, y := cornerCenter(o, cx, cy, func(o pieOutline) float32 { return o.lo + o.outer.angle }, outerDist, lift)
	a0, a1 := outlineAngles(o, func(o pieOutline) float32 { return o.lo - halfPi },
		func(o pieOutline) float32 { return halfPi + o.outer.angle }, false)
	arcTo(d, x, y, outerRadius, a0, a1, false)
	rim := lifted(numOf(o, func(o pieOutline) float32 { return o.outer.distance + o.outer.radius }))
	a0, a1 = outlineAngles(o, func(o pieOutline) float32 { return o.lo + o.outer.angle },
		func(o pieOutline) float32 { return span(o) - 2*o.outer.angle }, false)
	arcTo(d, center(cx), center(cy), rim, a0, a1, false)
	x, y = cornerCenter(o, cx, cy, func(o pieOutline) float32 { return o.hi - o.outer.angle }, outerDist, lift)
	a0, a1 = outlineAngles(o, func(o pieOutline) float32 { return o.hi - o.outer.angle },
		func(o pieOutline) float32 { return halfPi + o.outer.angle }, false)
	arcTo(d, x, y, outerRadius, a0, a1, false)
	x, y = cornerCenter(o, cx, cy, func(o pieOutline) float32 { return o.hi - o.inner.angle }, innerDist, nil)
	a0, a1 = outlineAngles(o, func(o pieOutline) float32 { return o.hi + halfPi },
		func(o pieOutline) float32 { return halfPi - o.inner.angle }, false)
	arcTo(d, x, y, innerRadius, a0, a1, false)
	hole := numOf(o, func(o pieOutline) float32 { return o.inner.distance - o.inner.radius })
	a0, a1 = outlineAngles(o, func(o pieOutline) float32 { return o.hi - o.inner.angle },
		func(o pieOutline) float32 { return span(o) - 2*o.inner.angle }, true)
	arcTo(d, center(cx), center(cy), hole, a0, a1, true)
	x, y = cornerCenter(o, cx, cy, func(o pieOutline) float32 { return o.lo + o.inner.angle }, innerDist, nil)
	a0, a1 = outlineAngles(o, func(o pieOutline) float32 { return o.lo + o.inner.angle + math.Pi },
		func(o pieOutline) float32 { return halfPi - o.inner.angle }, false)
	arcTo(d, x, y, innerRadius, a0, a1, false)
	d.ClosePath()
	return true
}

// paintSector fills what sector records with the view's Fill and strokes it
// with its Stroke, none where none is set: a slice's edge in the surface's
// color draws the gaps between slices.
func paintSector(d *core.Drawing, fill core.Paint, sector func() bool) {
	d.FillFromView(fill).StrokeFromView(core.PaintNone).LineWidthFromView().LineDashFromView()
	if sector() {
		d.Fill()
		d.Stroke()
	}
}

// Pie is a pie of a Plot's rows, as gpui-base's Pie: a slice per row worth
// more than nothing, each its share of the turn from StartAngle to EndAngle,
// in the rows' order, around the plot's center between InnerRadius, a
// donut's hole, and OuterRadius, PadAngle apart. Each slice is a canvas of
// its own, so each paints with its own classes and styles over the pie's, as
// CSS inherits Fill and Stroke: Fill, the text color where none is set or
// with Color its category's, and Stroke, none. Over a pie, the plot reports
// the row of the slice under the pointer, none in the hole or outside, and
// its tooltip follows the pointer; that slice grows by HoverLift and the
// others fade by HoverDim. The ring sweeps in from its first slice with the
// plot's appear, and when the values or the radii change each slice moves
// from its old angles to its new ones with Motion.Update, d3's arcTween.
// With a Label, each slice has a text outside the ring and, styled, a leader
// line to it. Its values take no part in the plot's scales.
type Pie struct {
	// Value is what each row is worth; the plot's Y without. A row worth
	// nothing, less, or no number has no slice.
	Value func() []float64
	// StartAngle is where the first slice starts, 0 without it, and EndAngle
	// where the last ends, a whole turn without it; less than StartAngle
	// lays the slices out the other way. Radians from 12 o'clock, clockwise.
	StartAngle, EndAngle func() float32
	// PadAngle is the angle between neighbouring slices at the rim: each gap
	// keeps the width it has there at every radius.
	PadAngle func() float32
	// InnerRadius is the radius of the hole, 0 without it: a donut. And
	// OuterRadius of the slices, 100% without it. Points, or a percentage of
	// the largest circle the plot holds.
	InnerRadius, OuterRadius func() core.Length
	// CornerRadius rounds each slice's corners, as d3's arc.cornerRadius: at
	// most half the ring's thickness and smaller on a slice too narrow for
	// two whole corners.
	CornerRadius core.Points
	// SliceStyle gives every slice its Fill and Stroke, and SliceStyleOf and
	// SliceClassOf a slice of a row its own, over them.
	SliceClass   string
	SliceClassOf func(row int) string
	SliceStyle   core.Style
	SliceStyleOf func(row int) core.Style
	// HoverLift is how far the slice under the pointer grows past the
	// others, on the plot's pointer spring; HoverDim how much the others
	// fade, as a share of their opacity, with the hover's fade.
	HoverLift core.Points
	HoverDim  float32
	// Color is the category of each slice, which the plot's ColorScale
	// gives a color: its Fill where nothing sets one.
	Color func() []string
	// Label is the text beside each slice, outside the ring toward its
	// middle, LabelGap past the rim, then across to that radius on the
	// slice's side of the pie: it starts there on the right half and ends
	// there on the left. An empty text, or a slice narrower than half a
	// degree, has none. LabelGap is 15 without it.
	Label          func() []string
	LabelGap       core.Points
	LabelClass     string
	LabelStyle     core.Style
	LabelStyleFunc func() core.Style
	// LabelLineStyle styles a label's leader line, and LabelLineStyleOf and
	// LabelLineClassOf a row's over it: from the slice's rim out to the
	// label's radius, then across to the text, its Stroke the text color
	// where none is set. Drawn only with a class or a style.
	LabelLineClass   string
	LabelLineClassOf func(row int) string
	LabelLineStyle   core.Style
	LabelLineStyleOf func(row int) core.Style
	Class            string
	Style            core.Style
	StyleFunc        func() core.Style
	TestID           string
}

// liftOf is how far row's slice has lifted, 0 to 1: on the pointer spring
// while the plot moves, at once without one or with reduced motion.
func liftOf(cx *core.Scope, plot *plotState, row int) *core.AnimatedValue {
	lifted := core.NewAnimatedValue(cx, 0)
	target := float32(0)
	cx.Effect(func() {
		to := float32(0)
		if plot.hovered.Get() == row {
			to = 1
		}
		if target != to {
			target = to
			plot.springTo(lifted, to, !plot.still())
		}
	})
	return lifted
}

// Render implements core.Node.
func (p Pie) Render(cx *core.Scope) core.Node {
	own := part{Class: p.Class, Style: p.Style, Fn: p.StyleFunc}
	plot := plotOf(cx, "Pie")
	if plot == nil {
		return markLayer(own, p.TestID)
	}
	values := func() []float64 {
		if p.Value != nil {
			return p.Value()
		}
		return plot.y()
	}
	start := func() float32 { return angleOr(p.StartAngle, 0) }
	end := func() float32 { return angleOr(p.EndAngle, tau) }
	sliceList := core.NewMemoFunc(cx, func() []pieSlice { return pieArcs(values(), start(), end()) }, slices.Equal)
	ring := core.NewMemo(cx, func() pieRing {
		in, out := radii(p.InnerRadius, p.OuterRadius)
		return ringOf(plot.size.Get(), in, out)
	})
	moves := newMoves(cx, plot, false, func() (piePose, bool) {
		in, out := radii(p.InnerRadius, p.OuterRadius)
		return piePose{in, out, start(), sliceList.Get()}, true
	})
	plot.addHit(cx, &plotHit{
		at: func(at core.Point) (int, bool) {
			from := min(core.Untracked(cx, start), core.Untracked(cx, end))
			return ring.GetUntracked().sliceAt(sliceList.GetUntracked(), from, at)
		},
		center: func(row int) (core.Point, bool) {
			for _, s := range sliceList.GetUntracked() {
				if s.row == row {
					return ring.GetUntracked().middle(s), true
				}
			}
			return core.Point{}, false
		},
	})
	lifts, dim := p.HoverLift > 0, min(max(p.HoverDim, 0), 1)
	lift := float32(max(p.HoverLift, 0))
	bleed := func() float32 {
		return strokeRoom + float32(math.Ceil(float64(ring.Get().reachPast(plot.size.Get(), lift))))
	}
	shapes := func(row int) (sectorShape, *shapeMove, float32, bool) {
		m := moves.now()
		slice, ok := m.to.slice(row)
		if !ok {
			return sectorShape{}, nil, 0, false
		}
		size := plot.size.Get()
		to := slice.shape(size)
		var from *shapeMove
		if m.moving {
			a, b := sliceFrom(m.from.slices, row, m.to.start)
			from = &shapeMove{moves.progress, newArcPose(m.from.inner, m.from.outer, a, b).shape(size)}
		}
		return to, shapeMoveOf(plot, to, m.to.start, from), m.to.start, true
	}
	rows := func() []int {
		all := sliceList.Get()
		out := make([]int, len(all))
		for i, s := range all {
			out[i] = s.row
		}
		return out
	}
	children := []core.Node{core.Each(rows, func(row int) int { return row }, func(cx *core.Scope, item func() int, _ func() int) core.Node {
		row := item()
		var lifted *core.AnimatedValue
		if (lifts || dim > 0) && !plot.noHover {
			lifted = liftOf(cx, plot, row)
		}
		style := p.SliceStyle
		if dim > 0 && lifted != nil {
			// 1 - dim*fade*(1-lifted).
			others := core.Mul(core.Sub(core.Constant(1), lifted), plot.fade)
			style.Animated = &core.AnimatedStyle{Opacity: core.Sub(core.Constant(1), core.Mul(others, core.Constant(dim)))}
		}
		var fn func() core.Style
		if p.SliceStyleOf != nil {
			fn = func() core.Style { return p.SliceStyleOf(row) }
		}
		canvas := markCanvas(plot, bleed, part{Class: p.SliceClass, Style: style, Fn: fn}, func(d *core.Drawing, plot *plotState, _ float32) {
			if plot == nil {
				return
			}
			to, m, _, ok := shapes(row)
			if !ok {
				return
			}
			r := ring.Get()
			var by *num
			if lifts && lifted != nil {
				n := nodeNum(core.Mul(core.Mul(lifted, plot.fade), core.Constant(lift)))
				by = &n
			}
			var c core.Color
			if p.Color != nil {
				if colors, ok := plot.categoryColors(p.Color()); ok && row < len(colors) {
					c = colors[row]
				}
			}
			paintSector(d, paintOf(c, core.PaintCurrentColor), func() bool {
				return sector(d, r.cx, r.cy, to, by, angleOr(p.PadAngle, 0), float32(p.CornerRadius), m)
			})
		})
		if p.SliceClassOf != nil {
			canvas.ClassFunc = func() string { return p.SliceClassOf(row) }
		}
		return canvas
	})}
	if p.Label != nil {
		gap := p.LabelGap
		if gap == 0 {
			gap = 15
		}
		type labelKey struct{ row int }
		children = append(children, core.Each(func() []labelKey {
			var out []labelKey
			for _, s := range sliceList.Get() {
				if textAt(p.Label, s.row) != "" {
					out = append(out, labelKey{s.row})
				}
			}
			return out
		}, func(k labelKey) labelKey { return k }, func(cx *core.Scope, item func() labelKey, _ func() int) core.Node {
			return p.sliceLabel(cx, plot, ring, shapes, item().row, float32(gap), lift)
		}))
	}
	return markLayer(own, p.TestID, children...)
}

// labelPlace is where a slice's label goes: its leader line runs from the
// rim at the slice's middle out to the label's radius, gap past the rim,
// then across to that radius on the label's side, where the text hangs. dir
// is the direction of the slice's middle: a lift moves the label along it.
type labelPlace struct {
	dirX, dirY, rimX, rimY, elbowX, elbowY, endX, endY, side float32
}

func newLabelPlace(cx, cy float32, shape sectorShape, gap, side float32) labelPlace {
	middle := (shape.lo + shape.hi) / 2
	dx, dy := sin32(middle), -cos32(middle)
	r := shape.outer + gap
	return labelPlace{dx, dy, cx + shape.outer*dx, cy + shape.outer*dy, cx + r*dx, cy + r*dy, cx + side*r, cy + r*dy, side}
}

func labelSide(shape sectorShape) float32 {
	if sin32((shape.lo+shape.hi)/2) > 0 {
		return 1
	}
	return -1
}

// labelPoint is a place of the samples, moved out by lift along the slice's
// middle.
func labelPoint(s samples[labelPlace], x, y func(labelPlace) float32, lift *num) (num, num) {
	px, py := numOf(s, x), numOf(s, y)
	if lift == nil {
		return px, py
	}
	dx := numOf(s, func(l labelPlace) float32 { return l.dirX })
	dy := numOf(s, func(l labelPlace) float32 { return l.dirY })
	return px.add(lift.mul(dx)), py.add(lift.mul(dy))
}

// sliceLabel is the label of row's slice, in a layer of its own: its leader
// line, drawn only when styled, and a point with no size where the line
// ends, the text beside it on the label's side. It follows its slice's
// motion and lift, fades in over the end of the appear and takes no pointer.
func (p Pie) sliceLabel(cx *core.Scope, plot *plotState, ring *core.Memo[pieRing],
	shapes func(int) (sectorShape, *shapeMove, float32, bool), row int, gap, lift float32) core.Node {
	var lifted *core.AnimatedValue
	if lift > 0 && !plot.noHover {
		lifted = liftOf(cx, plot, row)
	}
	places := func() (labelPlace, samples[labelPlace], bool) {
		to, m, _, ok := shapes(row)
		if !ok || to.hi-to.lo < labelMinSpan {
			return labelPlace{}, samples[labelPlace]{}, false
		}
		r, side := ring.Get(), labelSide(to)
		place := func(s sectorShape) labelPlace { return newLabelPlace(r.cx, r.cy, s, gap, side) }
		return place(to), sampled(to, m, place), true
	}
	liftNum := func() *num {
		if lifted == nil {
			return nil
		}
		n := nodeNum(core.Mul(core.Mul(lifted, plot.fade), core.Constant(lift)))
		return &n
	}
	bleed := func() float32 {
		return strokeRoom + float32(math.Ceil(float64(ring.Get().reachPast(plot.size.Get(), lift+gap))))
	}
	var children []core.Node
	lineSet := p.LabelLineClass != "" || p.LabelLineClassOf != nil || !p.LabelLineStyle.IsZero() || p.LabelLineStyleOf != nil
	if lineSet {
		var fn func() core.Style
		if p.LabelLineStyleOf != nil {
			fn = func() core.Style { return p.LabelLineStyleOf(row) }
		}
		line := markCanvas(plot, bleed, part{Class: p.LabelLineClass, Style: p.LabelLineStyle, Fn: fn}, func(d *core.Drawing, plot *plotState, _ float32) {
			d.StrokeFromView(core.PaintCurrentColor).LineWidthFromView().LineDashFromView()
			if plot == nil {
				return
			}
			_, s, ok := places()
			if !ok {
				return
			}
			by := liftNum()
			x0, y0 := labelPoint(s, func(l labelPlace) float32 { return l.rimX }, func(l labelPlace) float32 { return l.rimY }, by)
			x1, y1 := labelPoint(s, func(l labelPlace) float32 { return l.elbowX }, func(l labelPlace) float32 { return l.elbowY }, by)
			x2, y2 := labelPoint(s, func(l labelPlace) float32 { return l.endX }, func(l labelPlace) float32 { return l.endY }, by)
			d.BeginPath()
			moveTo(d, x0, y0)
			lineTo(d, x1, y1)
			lineTo(d, x2, y2)
			d.Stroke()
		})
		if p.LabelLineClassOf != nil {
			line.ClassFunc = func() string { return p.LabelLineClassOf(row) }
		}
		children = append(children, line)
	}
	// Made again for each move: laid out where it rests and moved from there
	// by the frame clock.
	children = append(children, core.Dynamic(func(cx *core.Scope) core.Node {
		rest, s, ok := places()
		if !ok {
			return core.Group{}
		}
		x, y := labelPoint(s, func(l labelPlace) float32 { return l.endX }, func(l labelPlace) float32 { return l.endY }, liftNum())
		animated := &core.AnimatedStyle{}
		if x.n != nil {
			animated.TranslateX = core.Sub(x.n, core.Constant(rest.endX))
		}
		if y.n != nil {
			animated.TranslateY = core.Sub(y.n, core.Constant(rest.endY))
		}
		justify := core.JustifyEnd
		if rest.side > 0 {
			justify = core.JustifyStart
		}
		return core.View{
			Style: core.Style{Position: core.Absolute, Left: pts(rest.endX), Top: pts(rest.endY), Width: pts(0), Height: pts(0),
				Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: justify, Animated: animated},
			PointerEvents: core.PointerEventsNone,
			Children: []core.Node{core.Text{ContentFunc: func() string { return textAt(p.Label, row) }, Class: p.LabelClass,
				Style: p.LabelStyle, StyleFunc: p.LabelStyleFunc}},
		}
	}))
	fadeIn := core.Interpolate(plot.appear.progress(), core.Interpolation{Input: []float64{labelAppearStart, 1}, Output: []float64{0, 1}})
	return core.View{
		Style: core.Style{Position: core.Absolute, Left: pts(0), Top: pts(0), Right: pts(0), Bottom: pts(0),
			Animated: &core.AnimatedStyle{Opacity: fadeIn}},
		PointerEvents: core.PointerEventsNone,
		Children:      children,
	}
}

// Arc is an arc of a Plot, as gpui-base's Arc: the sector from StartAngle to
// EndAngle around the plot's center, between InnerRadius and OuterRadius; a
// gauge's track and its value, or a ring behind a pie. It fills with its
// Fill, the text color where none is set, and strokes with its Stroke, none
// where none is set, as CSS inherits them from the plot, and sweeps in from
// StartAngle with the plot's appear; when its angles or radii change it
// moves to the new ones with Motion.Update, so a gauge sweeps round to its
// new value. It is a shape, not a row: the pointer over it finds nothing.
type Arc struct {
	// StartAngle and EndAngle are a Pie's, 0 and a whole turn without them.
	StartAngle, EndAngle func() float32
	// InnerRadius and OuterRadius are a Pie's.
	InnerRadius, OuterRadius func() core.Length
	// CornerRadius rounds its corners; half the thickness gives round caps.
	// A whole ring has none.
	CornerRadius core.Points
	Class        string
	Style        core.Style
	StyleFunc    func() core.Style
	TestID       string
}

// Render implements core.Node.
func (a Arc) Render(cx *core.Scope) core.Node {
	plot := plotOf(cx, "Arc")
	start := func() float32 { return angleOr(a.StartAngle, 0) }
	end := func() float32 { return angleOr(a.EndAngle, tau) }
	var moves *moves[arcPose]
	if plot != nil {
		moves = newMoves(cx, plot, false, func() (arcPose, bool) {
			in, out := radii(a.InnerRadius, a.OuterRadius)
			return newArcPose(in, out, start(), end()), true
		})
	}
	ring := func() pieRing {
		in, out := radii(a.InnerRadius, a.OuterRadius)
		return ringOf(plot.size.Get(), in, out)
	}
	bleed := func() float32 {
		if plot == nil {
			return strokeRoom
		}
		return strokeRoom + float32(math.Ceil(float64(ring().reachPast(plot.size.Get(), 0))))
	}
	return markLayer(part{Class: a.Class, Style: a.Style, Fn: a.StyleFunc}, a.TestID, markCanvas(plot, bleed, part{}, func(d *core.Drawing, plot *plotState, _ float32) {
		if plot == nil {
			return
		}
		m := moves.now()
		size := plot.size.Get()
		to := m.to.shape(size)
		var from *shapeMove
		if m.moving {
			from = &shapeMove{moves.progress, m.from.shape(size)}
		}
		motion := shapeMoveOf(plot, to, start(), from)
		r := ring()
		paintSector(d, core.PaintCurrentColor, func() bool {
			return sector(d, r.cx, r.cy, to, nil, 0, float32(a.CornerRadius), motion)
		})
	}))
}
