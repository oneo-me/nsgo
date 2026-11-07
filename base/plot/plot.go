package plot

import (
	"math"
	"slices"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// part is the class and styles the app gives one of a component's parts.
type part = parts.Part

// Motion is the timing of a plot's motion. Each is the app's; a nil
// one moves at once, as does every one while the user asks for reduced
// motion.
type Motion struct {
	// Pointer is the spring the hover's parts glide to the next row with
	// when the keys or a screen reader step there, and a pie's slice under
	// the pointer lifts with. The pointer's own moves never take it: the
	// parts follow the pointer in the same frame, so the hover never trails
	// the hand.
	Pointer *core.SpringConfig
	// Enter fades the hover's parts in as the pointer comes over the plot,
	// and Exit fades them out after it leaves.
	Enter, Exit *core.TimingConfig
	// Appear draws the marks in, when the plot is first shown with an
	// AppearGeneration and whenever that changes: lines and areas from the
	// leading edge, bars growing out of their base.
	Appear *core.TimingConfig
	// Update moves marks to a new shape when the app changes it, as d3's
	// update transitions: a pie's and an arc's radii and angles, and a
	// linear scale's domain, interpolated as d3 transitions a scale's
	// domain, so the marks keep their shapes while the grid lines, the
	// axes' ticks and the hover's parts move with them.
	Update *core.TimingConfig
}

// TooltipPlacement is where a Plot's tooltip goes; either way it stays
// inside the plot's edges.
type TooltipPlacement uint8

const (
	// TooltipTop puts it at the top of the plot, centered on the cross
	// line, TooltipOffset below the top edge.
	TooltipTop TooltipPlacement = iota
	// TooltipPointer puts it beside the pointer: on the side of the cross
	// line toward the plot's middle, below the pointer in the upper half
	// and above it in the lower one, TooltipOffset from both.
	TooltipPointer
)

// alignedPitch is the height an aligned step of the y scales takes at
// least, in points: from 2 to 10 steps, as the plot's height allows.
const alignedPitch = 32

// Plot is the frame of a chart, as gpui-base's: the rows of data, the
// scales of its axes, its size as their range, the pointer over it and its
// motion. It draws nothing itself: its marks (Line, Area, Bar, Rule, Dot,
// Cell, Grid, Pie, Arc, RadialLine, RadialGrid, Sankey, Treemap), its
// children, each fill it and draw in order over each other, natively from
// the rows, each in a canvas of its own or more, and its Axis and Label
// children write its edges and texts at values of its scales.
//
// It reports the row nearest to the pointer along the axis of the rows, or
// what a mark finds under it (a slice, a dot, a cell), with OnHoverChange
// and shows the hover's parts the app styles (the cross line, a dot per
// mark, the tooltip), moved and faded on the platform's frame clock with
// Motion; the pointer moves them and never draws a mark again.
//
// The keyboard drives the same hover: the plot is a stop of the focus, and
// while it has it Left and Right move to the row on that side, Home and End
// to the first and last, Escape clears it; over a grid of cells the arrows
// step across it on the screen. Screen readers hear one image, Label and
// AccessibilityValue, and increment and decrement step through the rows.
// Style sizes it, and the marks inherit its paint properties (Stroke,
// Fill, StrokeWidth, StrokeDash) as CSS inherits them.
type Plot struct {
	// Count is how many rows it has; the length of X, else of Y, without.
	Count func() int
	// Y is the values the marks draw unless they have their own, one per
	// row; NaN skips a row.
	Y func() []float64
	// X places the rows along a linear x scale; a row's index without it.
	X func() []float64
	// XScale is a point scale unless it says otherwise (linear with a band
	// YScale, for bars across); YScale is linear, its domain the extent of
	// what the marks draw unless it gives one.
	XScale, YScale func() Scale
	// Y2Scale is a second linear scale along the y axis, for the values of
	// the marks that name it (ScaleY2), as dual-axis charts put a series of
	// other units on a scale of its own. Without it, marks on ScaleY2 go on
	// a linear scale of their values' extent.
	Y2Scale func() Scale
	// NoAlignTicks keeps each y domain the extent of its values. Otherwise,
	// while both are automatic and marks are on both, each is widened to
	// nice steps, the same number on both, zero on the same place when both
	// hold it, as ECharts' alignTicks; the axes' default ticks and a Grid
	// without values take those steps.
	NoAlignTicks bool
	// ColorScale gives the marks the colors of their rows' categories and
	// numbers; none where it is nil.
	ColorScale func() ColorScale
	// NoHover draws the marks only, following no pointer: a sparkline in
	// a list.
	NoHover bool
	Motion  Motion
	// AppearGeneration opts in to Motion.Appear: the marks draw in when
	// the plot is first shown, and again whenever it changes. It returns a
	// comparable value: a row's key, a period. Inside an AppearScope, a
	// generation that finished there shows whole at once.
	AppearGeneration func() any
	// Label says what the chart shows and what stands out;
	// AccessibilityValue the app's words for the row under the pointer.
	Label              string
	LabelFunc          func() string
	AccessibilityValue func() core.AccessibilityValue
	// CrossLineStyle styles the cross line, a view as tall as the plot
	// centered on the row under the pointer; there is none without it or
	// CrossLineClass. On a band scale it is as wide as the band sits in the
	// row's band, so a width of 100% fills it, and on a band y scale it lies
	// across the plot.
	CrossLineClass     string
	CrossLineStyle     core.Style
	CrossLineStyleFunc func() core.Style
	// DotStyle styles the hover's dots, a view per Line and Area
	// centered on its point at the row under the pointer; a mark's
	// HoverDotStyle merges over it for its own dot, its color over the
	// plot's size and ring, and its HoverDotClass follows DotClass.
	DotClass     string
	DotStyle     core.Style
	DotStyleFunc func() core.Style
	// Tooltip is the app's tooltip, placed by TooltipPlacement
	// TooltipOffset away and moved and faded with the cross line.
	Tooltip          core.Node
	TooltipPlacement TooltipPlacement
	TooltipOffset    core.Points
	// OnHoverChange receives the row under the pointer when another one
	// is, and -1 once the pointer leaves the plot.
	OnHoverChange func(index int)
	Class         string
	Style         core.Style
	StyleFunc     func() core.Style
	Children      []core.Node
	TestID        string
	// OnLayout is its view's, as a core.View's.
	OnLayout func(core.Bounds)
}

// plotKey is the plot a mark is in.
var plotKey = core.NewKey[*plotState]("plot", nil)

// noAxis is no axis of bands.
const noAxis axis = 255

// appear is the appear's time on the platform's frame clock, linear from 0
// to 1 over Motion.Appear's duration, which each mark eases on its own
// slice of it.
type appear struct {
	time   *core.AnimatedValue
	easing *core.Signal[core.Easing]
}

// progress is how far the whole plot has appeared, 0 to 1, eased.
func (a appear) progress() core.AnimatedNode { return a.staggered(0, 1, 0) }

// staggered is how far mark index of count has appeared: the marks start
// one after another over the first spread of the time and each runs for
// the rest of it, so the last one finishes with the appear.
func (a appear) staggered(index, count int, spread float32) core.AnimatedNode {
	s := float64(min(max(spread, 0), 0.95))
	if count <= 1 {
		s = 0
	}
	start := 0.0
	if count > 1 {
		start = s * float64(min(index, count-1)) / float64(count-1)
	}
	return core.Interpolate(a.time, core.Interpolation{Input: []float64{start, start + 1 - s},
		Output: []float64{0, 1}, Easing: a.easing.GetUntracked()})
}

// plotHit is a mark's test of the row under a point of the plot, and where
// a row is for the keys; grid, when set, is where a key steps from a row
// across a grid of them.
type plotHit struct {
	key    int
	at     func(p core.Point) (int, bool)
	center func(row int) (core.Point, bool)
	grid   func(from int, step keyStep) (int, bool)
}

// plotInset is what a y axis's labels need kept inside the plot's top and
// bottom edges.
type plotInset struct {
	key  int
	need *core.Memo[[2]float32]
}

// plotState is a plot's state, for its marks.
type plotState struct {
	cx   *core.Scope
	rows func() int
	y    func() []float64
	// x is the plot's X, nil without one.
	x func() []float64
	// xMap places values along the x axis and xs is where each of the
	// plot's rows goes along it (NaN: nowhere).
	xMap        *core.Memo[axisMap]
	xs          *core.Memo[[]float32]
	yMap, y2Map *core.Memo[axisMap]
	// The moves of the linear axes to new domains.
	xTween, yTween, y2Tween *tween
	size                    *core.Signal[core.Size]
	marks                   *core.Signal[[]*plotMark]
	next                    int
	appear                  appear
	// appearing says the appear runs: the lines and areas clip to it, the
	// bars grow, a pie's slices sweep in.
	appearing *core.Signal[bool]
	motion    Motion
	ratio     func() float32
	bars      *core.Signal[[]*barEntry]
	barLayout *core.Memo[*barLayout]
	// crossX and crossY are the hover's place along each axis, and fade
	// how far it has faded in.
	crossX, crossY, fade *core.AnimatedValue
	// hovered is the row under the pointer, -1 for none.
	hovered *core.Signal[int]
	// keyed says the keys or a screen reader moved the hover last: its
	// parts may glide to the row on Motion.Pointer.
	keyed    bool
	insets   *core.Signal[[]*plotInset]
	hits     *core.Signal[[]*plotHit]
	noHover  bool
	dot      part
	bandAxis *core.Memo[axis]
	colors   func() ColorScale
	hasColor bool
	// met is the categories outside the color scale's domain the marks
	// have met, and metOf the scale they were met for.
	met         []string
	metOf       ColorScale
	cells       *core.Signal[[]*cellEntry]
	xCategories *core.Memo[[]string]
	yCategories *core.Memo[[]string]
	cellExtent  *core.Memo[plotExtent]
}

// sized reports whether the plot has been laid out with an area to draw
// on; before that a mark records nothing.
func (s *plotState) sized() bool {
	size := s.size.Get()
	return size.Width > 0 && size.Height > 0
}

// still reports whether the user asks for reduced motion.
func (s *plotState) still() bool { return core.Untracked(s.cx, s.cx.Env().ReduceMotion) }

// yMapOf places values along the y scale scale names.
func (s *plotState) yMapOf(scale ValueScale) axisMap {
	if scale == ScaleY2 {
		return s.y2Map.Get()
	}
	return s.yMap.Get()
}

func (s *plotState) yTweenOf(scale ValueScale) *tween {
	if scale == ScaleY2 {
		return s.y2Tween
	}
	return s.yTween
}

func (s *plotState) tweenOf(a axis) *tween {
	switch a {
	case axisX:
		return s.xTween
	case axisY2:
		return s.y2Tween
	}
	return s.yTween
}

// nextKey is the next mark's key.
func (s *plotState) nextKey() int {
	s.next++
	return s.next
}

// clip records the appear's clip while it runs: the marks show from the
// leading edge, where the x scale starts, to the appear's progress of the
// plot's width, and bleed points past the others.
func (s *plotState) clip(d *core.Drawing, bleed float32) {
	if !s.appearing.Get() {
		return
	}
	size := s.size.Get()
	w, h := float32(size.Width), float32(size.Height)
	shown := core.Mul(s.appear.progress(), core.Constant(w))
	lead, edge := -bleed, shown
	if s.xMap.Get().backward() {
		lead, edge = w+bleed, core.Sub(core.Constant(w), shown)
	}
	d.BeginPath().MoveTo(lead, -bleed).LineTo(w, -bleed).Bind(core.ArgX, edge).
		LineTo(w, h+bleed).Bind(core.ArgX, edge).LineTo(lead, h+bleed).ClosePath().Clip()
}

// traced records what fn adds to the path where the moving scales show it,
// along x if x and along the y scale y names: through the moves' transform
// while one runs. The path keeps its points after the transform is undone,
// so its stroke is not scaled.
func (s *plotState) traced(d *core.Drawing, x bool, y ValueScale, fn func()) {
	yt := s.yTween
	if y != noScale {
		yt = s.yTweenOf(y)
	}
	alongX := x && s.xTween.moving.Get()
	alongY := y != noScale && yt.moving.Get()
	if !alongX && !alongY {
		fn()
		return
	}
	d.Save().Transform(1, 0, 0, 1, 0, 0)
	if alongX {
		d.Bind(core.ArgX1, s.xTween.scale).Bind(core.ArgX, s.xTween.offset)
	}
	if alongY {
		d.Bind(core.ArgY2, yt.scale).Bind(core.ArgY, yt.offset)
	}
	fn()
	d.Restore()
}

// categoryColors is the colors the color scale gives categories, 0 for
// none, a category met first taking the next color.
func (s *plotState) categoryColors(categories []string) ([]core.Color, bool) {
	if !s.hasColor {
		return nil, false
	}
	scale := s.colors()
	if scale.kind != colorOrdinal {
		return nil, false
	}
	if !scale.equal(s.metOf) {
		s.metOf, s.met = scale, nil
	}
	out := make([]core.Color, len(categories))
	for i, c := range categories {
		out[i] = scale.colorOf(&s.met, c)
	}
	return out, true
}

// shades is the colors a ramp of the color scale gives values, over its
// domain or extent.
func (s *plotState) shades(values []float64, extent plotExtent) ([]core.Color, bool) {
	if !s.hasColor {
		return nil, false
	}
	scale := s.colors()
	domain, ok := scale.spread(extent)
	if !ok {
		return nil, false
	}
	out := make([]core.Color, len(values))
	for i, v := range values {
		out[i] = scale.shade(v, domain)
	}
	return out, true
}

// isRamp reports whether the color scale is a ramp of numbers.
func (s *plotState) isRamp() bool { return s.hasColor && s.colors().kind != colorOrdinal }

// add adds m while the current owner lives.
func (s *plotState) add(cx *core.Scope, m *plotMark) {
	m.key = s.nextKey()
	s.marks.Update(func(all []*plotMark) []*plotMark { return append(slices.Clip(all), m) })
	cx.Cleanup(func() {
		s.marks.Update(func(all []*plotMark) []*plotMark {
			return slices.DeleteFunc(slices.Clone(all), func(o *plotMark) bool { return o == m })
		})
	})
}

// addInset keeps need (top, bottom) inside the plot's top and bottom edges
// while the current owner lives: the linear y scales' range starts and
// ends that far in.
func (s *plotState) addInset(cx *core.Scope, need *core.Memo[[2]float32]) {
	in := &plotInset{key: s.nextKey(), need: need}
	s.insets.Update(func(all []*plotInset) []*plotInset { return append(slices.Clip(all), in) })
	cx.Cleanup(func() {
		s.insets.Update(func(all []*plotInset) []*plotInset {
			return slices.DeleteFunc(slices.Clone(all), func(o *plotInset) bool { return o == in })
		})
	})
}

// addHit tests what is under the pointer with h while the current owner
// lives.
func (s *plotState) addHit(cx *core.Scope, h *plotHit) {
	h.key = s.nextKey()
	s.hits.Update(func(all []*plotHit) []*plotHit { return append(slices.Clip(all), h) })
	cx.Cleanup(func() {
		s.hits.Update(func(all []*plotHit) []*plotHit {
			return slices.DeleteFunc(slices.Clone(all), func(o *plotHit) bool { return o == h })
		})
	})
}

// extentOn is the extent of what the marks draw along a.
func extentOn(marks *core.Signal[[]*plotMark], a axis) plotExtent {
	var e plotExtent
	for _, m := range marks.Get() {
		if m.axis() == a {
			e = e.join(m.extent.Get())
		}
	}
	return e
}

// bandCategories are the categories of the bands of an axis whose scale is
// s: its domain, else the categories cells read along the axis, in the
// order met; nil for a band per row.
func bandCategories(s Scale, cells []*cellEntry, a axis) []string {
	if s.kind != scaleBand {
		return nil
	}
	if s.band.Domain != nil {
		return unique(s.band.Domain)
	}
	var met []string
	some := false
	for _, cell := range cells {
		texts, ok := cell.categories(a)
		if !ok {
			continue
		}
		some = true
		for _, t := range texts {
			if t != "" && !slices.Contains(met, t) {
				met = append(met, t)
			}
		}
	}
	if some && met == nil {
		met = []string{}
	}
	return met
}

// Render implements core.Node.
func (p Plot) Render(cx *core.Scope) core.Node {
	size := core.NewSignal(cx, core.Size{})
	marks := core.NewSignalFunc(cx, []*plotMark(nil), nil)
	insets := core.NewSignalFunc(cx, []*plotInset(nil), nil)
	inset := core.NewMemo(cx, func() [2]float32 {
		var out [2]float32
		for _, in := range insets.Get() {
			need := in.need.Get()
			out = [2]float32{max(out[0], need[0]), max(out[1], need[1])}
		}
		return out
	})
	scaleOf := func(f func() Scale) (Scale, bool) {
		if f == nil {
			return Scale{}, false
		}
		return f(), true
	}
	y := func() []float64 {
		if p.Y == nil {
			return nil
		}
		return p.Y()
	}
	rows := func() int {
		switch {
		case p.Count != nil:
			return p.Count()
		case p.X != nil:
			return len(p.X())
		}
		return len(y())
	}
	x := func() []float64 {
		if p.X == nil {
			return nil
		}
		return p.X()
	}
	// The axis with bands, from the scales the app names, not their maps,
	// which read the marks' extents, which read this.
	bandAxis := core.NewMemo(cx, func() axis {
		if s, ok := scaleOf(p.XScale); ok && s.kind == scaleBand {
			return axisX
		}
		if s, ok := scaleOf(p.YScale); ok && s.kind == scaleBand {
			return axisY
		}
		return noAxis
	})
	cells := core.NewSignalFunc(cx, []*cellEntry(nil), nil)
	sameStrings := func(a, b []string) bool { return slices.Equal(a, b) && (a == nil) == (b == nil) }
	xCategories := core.NewMemoFunc(cx, func() []string {
		s, _ := scaleOf(p.XScale)
		return bandCategories(s, cells.Get(), axisX)
	}, sameStrings)
	yCategories := core.NewMemoFunc(cx, func() []string {
		s, _ := scaleOf(p.YScale)
		return bandCategories(s, cells.Get(), axisY)
	}, sameStrings)
	bands := func(categories []string, n int) int {
		if categories != nil {
			return len(categories)
		}
		return n
	}
	xMap := core.NewMemo(cx, func() axisMap {
		width := float32(size.Get().Width)
		n := rows()
		s, ok := scaleOf(p.XScale)
		if !ok && bandAxis.Get() == axisY {
			// Bars across a band y axis put their values along x.
			s = LinearScale()
		}
		rng := [2]float32{0, width}
		if s.reverse {
			rng = [2]float32{width, 0}
		}
		switch s.kind {
		case scaleBand:
			return axisMap{kind: axisBand, bands: newBands(bands(xCategories.Get(), n), rng, s.band, s.reverse)}
		case scaleLinear:
			domain := s.domain
			if !s.hasDomain {
				e := extentOf(x())
				if !e.ok {
					e = extentOn(marks, axisX)
				}
				if !e.ok && n > 0 {
					e = plotExtent{0, float64(n - 1), true}
				}
				domain = e.ends()
			}
			return axisMap{kind: axisLinear, linear: newLinear(domain, rng)}
		}
		return axisMap{kind: axisPoint, len: n, rng: rng}
	})
	xs := core.NewMemoFunc(cx, func() []float32 { return xMap.Get().ticks(rows(), x()) }, nil)
	// The y scales laid out in the same steps, while both domains are the
	// marks' and marks are on both.
	type alignment struct {
		y, y2 aligned
		ok    bool
	}
	alignedScales := core.NewMemo(cx, func() alignment {
		if p.NoAlignTicks {
			return alignment{}
		}
		s, ok := scaleOf(p.YScale)
		s2, ok2 := scaleOf(p.Y2Scale)
		if ok && (s.kind != scaleLinear || s.hasDomain) || ok2 && s2.hasDomain {
			return alignment{}
		}
		e, e2 := extentOn(marks, axisY), extentOn(marks, axisY2)
		if !e.ok || !e2.ok {
			return alignment{}
		}
		in := inset.Get()
		room := float32(size.Get().Height) - in[0] - in[1]
		steps := min(max(int(math.Floor(float64(room/alignedPitch))), 2), 10)
		a, b, found := align(e.ends(), e2.ends(), steps)
		return alignment{a, b, found}
	})
	yMap := core.NewMemo(cx, func() axisMap {
		height := float32(size.Get().Height)
		n := rows()
		s, _ := scaleOf(p.YScale)
		if p.YScale == nil {
			s = LinearScale()
		}
		in := inset.Get()
		// Rows from the top edge down, values from the bottom edge up;
		// reversed, the other way.
		down, up := [2]float32{0, height}, [2]float32{height - in[1], in[0]}
		if s.reverse {
			down, up = [2]float32{height, 0}, [2]float32{in[0], height - in[1]}
		}
		switch {
		case s.kind == scalePoint:
			return axisMap{kind: axisPoint, len: n, rng: down}
		case s.kind == scaleBand:
			return axisMap{kind: axisBand, bands: newBands(bands(yCategories.Get(), n), down, s.band, s.reverse)}
		case s.hasDomain:
			return axisMap{kind: axisLinear, linear: newLinear(s.domain, up)}
		}
		if a := alignedScales.Get(); a.ok {
			return axisMap{kind: axisLinear, linear: alignedLinear(a.y, up)}
		}
		return axisMap{kind: axisLinear, linear: newLinear(extentOn(marks, axisY).ends(), up)}
	})
	y2Map := core.NewMemo(cx, func() axisMap {
		height := float32(size.Get().Height)
		s, _ := scaleOf(p.Y2Scale)
		in := inset.Get()
		rng := [2]float32{height - in[1], in[0]}
		if s.reverse {
			rng = [2]float32{in[0], height - in[1]}
		}
		if a := alignedScales.Get(); !s.hasDomain && a.ok {
			return axisMap{kind: axisLinear, linear: alignedLinear(a.y2, rng)}
		}
		domain := s.domain
		if !s.hasDomain {
			domain = extentOn(marks, axisY2).ends()
		}
		return axisMap{kind: axisLinear, linear: newLinear(domain, rng)}
	})
	hoverAxis := core.NewMemo(cx, func() axis {
		if bandAxis.Get() == axisY {
			return axisY
		}
		return axisX
	})
	// Where each row is along the axis the hover finds rows along.
	rowsAt := core.NewMemoFunc(cx, func() []float32 {
		if hoverAxis.Get() == axisX {
			return xs.Get()
		}
		return yMap.Get().ticks(rows(), nil)
	}, nil)

	place := newTooltipPlace(cx, p.TooltipPlacement, p.TooltipOffset)
	bars := core.NewSignalFunc(cx, []*barEntry(nil), nil)
	hits := core.NewSignalFunc(cx, []*plotHit(nil), nil)
	state := &plotState{cx: cx, rows: rows, y: y, x: p.X, xMap: xMap, xs: xs, yMap: yMap, y2Map: y2Map, size: size, marks: marks,
		xTween: newTween(cx), yTween: newTween(cx), y2Tween: newTween(cx),
		appear:    appear{time: core.NewAnimatedValue(cx, 1), easing: core.NewSignalFunc(cx, core.Easing{}, nil)},
		appearing: core.NewSignal(cx, false), motion: p.Motion, ratio: cx.Env().PixelRatio,
		bars: bars, barLayout: newBarLayout(cx, bars), crossX: place.x, crossY: core.NewAnimatedValue(cx, 0),
		fade: core.NewAnimatedValue(cx, 0), hovered: core.NewSignal(cx, -1), insets: insets, hits: hits, noHover: p.NoHover,
		dot: parts.Of(p.DotClass, p.DotStyle, p.DotStyleFunc), bandAxis: bandAxis, colors: p.ColorScale, hasColor: p.ColorScale != nil,
		cells: cells, xCategories: xCategories, yCategories: yCategories}
	state.cellExtent = core.NewMemo(cx, func() plotExtent {
		var e plotExtent
		for _, c := range cells.Get() {
			e = e.join(extentOf(c.values()))
		}
		return e
	})
	plotKey.Provide(cx, state)

	// The appear: from the first frame with a size, and again for each new
	// generation; at once without a timing or with reduced motion, whole
	// at once for a generation the plot's AppearScope saw finish. Its time
	// runs linearly; the marks ease it.
	if p.AppearGeneration != nil {
		memory := newAppearMemory(cx)
		shown := core.NewMemo(cx, func() bool { return size.Get().Width > 0 })
		clock := state.appear.time
		cx.Effect(func() {
			key := p.AppearGeneration()
			if !shown.Get() {
				return
			}
			finished := memory.isFinished(key)
			if t := p.Motion.Appear; t != nil && t.Duration > 0 && !state.still() && !finished {
				linear := *t
				linear.Easing = core.Easing{}
				state.appear.easing.Set(t.Easing)
				clock.Set(0)
				state.appearing.Set(true)
				// A newer generation stops this one and ends it itself.
				cx.StartAnimation(core.Timing(clock, 1, linear), func(done bool) {
					if done {
						memory.finish(key)
						state.appearing.Set(false)
					}
				})
				return
			}
			clock.Set(1)
			state.appearing.Set(false)
			if !finished {
				memory.finish(key)
			}
		})
	}

	// The linear axes' moves to new domains, over Motion.Update.
	cx.Effect(func() { state.xTween.follow(xMap.Get(), p.Motion.Update) })
	cx.Effect(func() { state.yTween.follow(yMap.Get(), p.Motion.Update) })
	cx.Effect(func() { state.y2Tween.follow(y2Map.Get(), p.Motion.Update) })

	// The hover: where its parts are and how far they have faded in.
	crossX, crossY, fade, hovered := place.x, state.crossY, state.fade, state.hovered
	fadeTo := func(to float64, t *core.TimingConfig, still bool) {
		if t != nil && !still {
			cx.StartAnimation(core.Timing(fade, to, *t), nil)
		} else {
			fade.Set(to)
		}
	}
	// pointerX, when pointed, says a mark found the row and the parts follow
	// the pointer rather than the row. keys says the keys or a screen reader
	// moved the hover, which may glide.
	follow := func(index int, pointerX float32, pointed, keys bool) {
		state.keyed = keys
		was := hovered.GetUntracked()
		if was == index {
			if index >= 0 && pointed {
				place.rowAt(pointerX)
				crossX.Set(float64(pointerX))
			}
			return
		}
		hovered.Set(index)
		if p.OnHoverChange != nil {
			p.OnHoverChange(index)
		}
		still := state.still()
		if index < 0 {
			fadeTo(0, p.Motion.Exit, still)
			return
		}
		at, across := pointerX, false
		if !pointed {
			all := rowsAt.GetUntracked()
			if index >= len(all) {
				return
			}
			at, across = all[index], hoverAxis.GetUntracked() == axisY
		}
		if !across {
			place.rowAt(at)
		}
		// Entering, the parts take the row where they are, then fade in. A
		// step of the keys glides to the next row on the pointer spring;
		// the pointer's moves land in the same frame.
		entering := was < 0
		to := func(v *core.AnimatedValue, target float32) {
			if s := p.Motion.Pointer; s != nil && keys && !entering && !still {
				cx.StartAnimation(core.Spring(v, float64(target), *s), nil)
			} else {
				v.Set(float64(target))
			}
		}
		if across {
			to(crossY, at)
		} else {
			to(crossX, at)
		}
		// A mark that finds its rows itself on a band y scale: the cross
		// line lies across the plot at the row's band.
		if pointed && hoverAxis.GetUntracked() == axisY {
			if all := rowsAt.GetUntracked(); index < len(all) && !math.IsNaN(float64(all[index])) {
				to(crossY, all[index])
			}
		}
		// The dots of the series through the row (bars have none).
		if !across {
			for _, m := range marks.GetUntracked() {
				if q, ok := m.near(at); ok {
					to(m.dotX, float32(q.X))
					to(m.dotY, float32(q.Y))
				}
			}
		}
		if entering {
			fadeTo(1, p.Motion.Enter, still)
		}
	}
	// The hovered row's parts stay on it as the rows or the scales change,
	// at once: a domain's move carries them with the marks. Over a radial
	// mark or across, they follow the pointer instead.
	cx.Effect(func() {
		rowsAt.Get()
		for _, m := range marks.Get() {
			if m.points != nil {
				m.points.Get()
			}
		}
		index := hovered.GetUntracked()
		if index < 0 || hoverAxis.GetUntracked() != axisX || len(hits.GetUntracked()) > 0 {
			return
		}
		all := rowsAt.GetUntracked()
		if index >= len(all) || math.IsNaN(float64(all[index])) {
			return
		}
		at := all[index]
		place.rowAt(at)
		crossX.Set(float64(at))
		for _, m := range marks.GetUntracked() {
			if q, ok := m.near(at); ok {
				m.dotX.Set(float64(q.X))
				m.dotY.Set(float64(q.Y))
			}
		}
	})

	// The keyboard: the same hover, a row at a time, while the plot has the
	// focus.
	focused := core.NewSignal(cx, false)
	pointerOver := false
	radial := func() bool { return len(hits.GetUntracked()) > 0 }
	sliceAt := func(i int) (core.Point, bool) {
		all := hits.GetUntracked()
		for j := len(all) - 1; j >= 0; j-- {
			if q, ok := all[j].center(i); ok {
				return q, true
			}
		}
		return core.Point{}, false
	}
	placed := func(i int) bool {
		if radial() {
			_, ok := sliceAt(i)
			return ok
		}
		all := rowsAt.GetUntracked()
		return i < len(all) && !math.IsNaN(float64(all[i]))
	}
	stepTo := func(step keyStep) (int, bool) {
		return stepRow(core.Untracked(cx, rows), placed, hovered.GetUntracked(), step)
	}
	keyTo := func(index int, ok bool) {
		if !ok {
			return
		}
		if radial() {
			if q, found := sliceAt(index); found {
				place.pointerAt(float32(q.Y))
				follow(index, float32(q.X), true, true)
			}
			return
		}
		all := rowsAt.GetUntracked()
		if index >= len(all) {
			return
		}
		at := all[index]
		// No pointer to follow: a tooltip beside it goes beside the first
		// series' point at the row, else the plot's middle (bars).
		s := size.GetUntracked()
		if hoverAxis.GetUntracked() == axisX {
			y := float32(s.Height) / 2
			for _, m := range marks.GetUntracked() {
				if q, found := m.near(at); found {
					y = float32(q.Y)
					break
				}
			}
			place.pointerAt(y)
		} else {
			mid := float32(s.Width) / 2
			crossX.Set(float64(mid))
			place.rowAt(mid)
			place.pointerAt(at)
		}
		follow(index, 0, false, true)
	}
	clear := func() { follow(-1, 0, false, true) }
	// Left and Right follow the screen: the rows along a reversed x scale
	// run right to left (a pie's slices keep the rows' order).
	mirrored := func() bool {
		return !radial() && hoverAxis.GetUntracked() == axisX && xMap.GetUntracked().backward()
	}
	gridStep := func() func(int, keyStep) (int, bool) {
		all := hits.GetUntracked()
		for j := len(all) - 1; j >= 0; j-- {
			if all[j].grid != nil {
				return all[j].grid
			}
		}
		return nil
	}
	keyStepTo := func(step keyStep) (int, bool) {
		if grid := gridStep(); grid != nil {
			return grid(hovered.GetUntracked(), step)
		}
		return stepTo(step)
	}
	onGrid := core.NewMemo(cx, func() bool {
		return slices.ContainsFunc(hits.Get(), func(h *plotHit) bool { return h.grid != nil })
	})

	hasCross := parts.Of(p.CrossLineClass, p.CrossLineStyle, p.CrossLineStyleFunc).Set()
	hasDots := state.dot.Set()
	dotStyle := func() core.Style {
		if p.DotStyleFunc == nil {
			return p.DotStyle
		}
		return p.DotStyle.Merge(p.DotStyleFunc())
	}
	children := slices.Clone(p.Children)
	if !p.NoHover {
		children = append(children, bindKeys(func() bool { return focused.Get() }, mirrored, onGrid.Get, hovered,
			keyStepTo, keyTo, clear))
		if hasCross {
			line := func(s core.Style) core.Node {
				return core.View{Class: p.CrossLineClass, Style: s.Merge(p.CrossLineStyle), StyleFunc: p.CrossLineStyleFunc}
			}
			band := func() core.Points {
				m := xMap.Get()
				if hoverAxis.Get() != axisX {
					m = yMap.Get()
				}
				if b, ok := m.band(); ok {
					return core.Points(b.width())
				}
				return 0
			}
			children = append(children, core.Show{When: func() bool { return hoverAxis.Get() == axisX },
				Then: func(cx *core.Scope) core.Node {
					return core.View{
						Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0),
							Width: core.Pt(0), AlignItems: core.AlignCenter,
							Animated: &core.AnimatedStyle{TranslateX: state.xTween.shown(crossX), Opacity: fade}},
						PointerEvents: core.PointerEventsNone,
						Children: []core.Node{core.View{Style: core.Style{Height: core.Pct(100), AlignItems: core.AlignCenter},
							StyleFunc: func() core.Style { return core.Style{Width: core.Pt(band())} },
							Children:  []core.Node{line(core.Style{Height: core.Pct(100)})}}},
					}
				},
				Else: func(cx *core.Scope) core.Node {
					// Across the plot, at the row along the y axis.
					return core.View{
						Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0),
							Height: core.Pt(0), JustifyContent: core.JustifyCenter,
							Animated: &core.AnimatedStyle{TranslateY: crossY, Opacity: fade}},
						PointerEvents: core.PointerEventsNone,
						Children: []core.Node{core.View{Style: core.Style{JustifyContent: core.JustifyCenter},
							StyleFunc: func() core.Style { return core.Style{Height: core.Pt(band())} },
							Children:  []core.Node{line(core.Style{Width: core.Pct(100)})}}},
					}
				}})
		}
		// A dot per series, styled by the mark or else by the plot; none
		// where neither styles one, and none for bars.
		type dotKey struct {
			key  int
			axis axis
		}
		children = append(children, core.Each(marks.Get, func(m *plotMark) dotKey { return dotKey{m.key, m.axis()} },
			func(cx *core.Scope, item func() *plotMark, _ func() int) core.Node {
				m := item()
				if m.dot == nil || !m.dot.Set() && !hasDots {
					return core.Group{}
				}
				yTween := state.tweenOf(m.axis())
				return dotAnchor(state.xTween.shown(m.dotX), yTween.shown(m.dotY), fade, core.View{Class: parts.JoinClasses(p.DotClass, m.dot.Class),
					StyleFunc: func() core.Style {
						s := dotStyle().Merge(m.dot.Style)
						if m.dot.Fn != nil {
							s = s.Merge(m.dot.Fn())
						}
						return s
					}})
			}))
		if p.Tooltip != nil {
			children = append(children, core.Show{When: func() bool { return hoverAxis.Get() == axisX },
				Then: func(*core.Scope) core.Node { return place.view(state.xTween.shown(crossX), fade, p.Tooltip) },
				Else: func(*core.Scope) core.Node { return place.view(crossX, fade, p.Tooltip) }})
		}
	}

	view := core.View{
		Class:              p.Class,
		Style:              p.Style,
		StyleFunc:          p.StyleFunc,
		Accessible:         true,
		Role:               core.RoleImage,
		Label:              p.Label,
		LabelFunc:          p.LabelFunc,
		AccessibilityValue: p.AccessibilityValue,
		TestID:             p.TestID,
		OnLayout: func(b core.Bounds) {
			size.Set(b.Size)
			place.plotIs(b.Size)
			if p.OnLayout != nil {
				p.OnLayout(b)
			}
		},
		Children: children,
	}
	if p.NoHover {
		return view
	}
	view.Focusable = func() bool { return true }
	view.OnFocus = func() { focused.Set(true) }
	view.OnBlur = func() {
		focused.Set(false)
		// The keys' hover goes with the focus; the pointer's stays.
		if !pointerOver {
			clear()
		}
	}
	view.AccessibilityActions = []core.AccessibilityAction{{Name: "increment"}, {Name: "decrement"}}
	view.OnAccessibilityAction = func(name string) {
		switch name {
		case "increment":
			keyTo(stepTo(stepNext))
		case "decrement":
			keyTo(stepTo(stepPrevious))
		}
	}
	// Hovering, not dragging: a drag leaves the hover where it was.
	view.OnMouseMove = func(e core.MouseEvent) {
		if e.PressedButton != core.MouseButtonNone {
			return
		}
		pointerOver = true
		px, py := float32(e.Position.X), float32(e.Position.Y)
		place.pointerAt(py)
		// Over what a mark finds: its row, if any.
		if all := hits.GetUntracked(); len(all) > 0 {
			for j := len(all) - 1; j >= 0; j-- {
				if i, ok := all[j].at(e.Position); ok {
					follow(i, px, true, false)
					return
				}
			}
			follow(-1, px, true, false)
			return
		}
		// Across (bands down the y axis): the rows are found along y, and
		// the tooltip follows the pointer along x.
		at := px
		if hoverAxis.GetUntracked() == axisY {
			crossX.Set(float64(px))
			place.rowAt(px)
			at = py
		}
		nearest, best := -1, float32(math.Inf(1))
		for i, x := range rowsAt.GetUntracked() {
			if d := float32(math.Abs(float64(x - at))); d < best {
				nearest, best = i, d
			}
		}
		follow(nearest, 0, false, false)
	}
	view.OnMouseLeave = func(core.MouseEvent) {
		pointerOver = false
		follow(-1, 0, false, false)
	}
	return view
}

// dotAnchor holds a dot centered on the animated point (x, y), faded by
// fade: a box with no size the dot overflows evenly. It takes no pointer.
func dotAnchor(x, y, fade core.AnimatedNode, dot core.Node) core.Node {
	return core.View{
		Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Width: core.Pt(0), Height: core.Pt(0),
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Animated: &core.AnimatedStyle{TranslateX: x, TranslateY: y, Opacity: fade}},
		PointerEvents: core.PointerEventsNone,
		Children:      []core.Node{dot},
	}
}

// updateTiming is Motion.Update, nil where the plot moves at once.
func (s *plotState) updateTiming() *core.TimingConfig {
	if t := s.motion.Update; t != nil && t.Duration > 0 && !s.still() {
		return t
	}
	return nil
}

// springTo moves v to target on the pointer spring when glide, else at once.
func (s *plotState) springTo(v *core.AnimatedValue, target float32, glide bool) {
	if sp := s.motion.Pointer; sp != nil && glide {
		s.cx.StartAnimation(core.Spring(v, float64(target), *sp), nil)
	} else {
		v.Set(float64(target))
	}
}

// tooltipPlace is where a plot's tooltip goes, kept inside the plot:
// between its left and right edges at the top, inside all its edges beside
// the pointer. The layout puts the tooltip on a side of a point with no
// size (its margins the offset), and the platform moves that point to the
// cross line, and the pointer, and clamps it from the plot's size and the
// tooltip's, which layout sets as animated values: nothing runs on the
// engine as the pointer moves, and the clamp follows the cross line's
// glide.
type tooltipPlace struct {
	placement TooltipPlacement
	offset    core.Points
	// x and y are where the point goes before the clamp: the cross line,
	// and the pointer beside it.
	x, y         *core.AnimatedValue
	plotW, plotH *core.AnimatedValue
	tipW, tipH   *core.AnimatedValue
	// flipX and flipY put the tooltip left of the cross line and above
	// the pointer, in the plot's right and lower halves.
	flipX, flipY *core.Signal[bool]
	size         core.Size
}

func newTooltipPlace(cx *core.Scope, placement TooltipPlacement, offset core.Points) *tooltipPlace {
	v := func() *core.AnimatedValue { return core.NewAnimatedValue(cx, 0) }
	return &tooltipPlace{placement: placement, offset: offset, x: v(), y: v(), plotW: v(), plotH: v(),
		tipW: v(), tipH: v(), flipX: core.NewSignal(cx, false), flipY: core.NewSignal(cx, false)}
}

func (t *tooltipPlace) plotIs(size core.Size) {
	t.size = size
	t.plotW.Set(float64(size.Width))
	t.plotH.Set(float64(size.Height))
}

// rowAt is the hover at the row at x.
func (t *tooltipPlace) rowAt(x float32) { t.flipX.Set(x > float32(t.size.Width)/2) }

// pointerAt is the pointer at y.
func (t *tooltipPlace) pointerAt(y float32) {
	if t.placement != TooltipPointer {
		return
	}
	t.y.Set(float64(y))
	t.flipY.Set(y > float32(t.size.Height)/2)
}

// clamped is at clamped so that a box of extent starting -(share*extent +
// offset) from it stays within 0 to room: lo + clamp01((at-lo)/(room-extent))
// * (room-extent), lo being the least at.
func clamped(at, room, extent core.AnimatedNode, share, offset core.AnimatedNode) core.AnimatedNode {
	rng := core.Sub(room, extent)
	lo := core.Add(core.Mul(extent, share), offset)
	t := core.Interpolate(core.Div(core.Sub(at, lo), rng), core.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 1}})
	return core.Add(lo, core.Mul(t, rng))
}

// view is the tooltip holding content, faded by fade.
func (t *tooltipPlace) view(at, fade core.AnimatedNode, content core.Node) core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		// The clamp's terms from the placement: how far the tooltip's width
		// moves it from the point (shift, -1/2 to center it), where its box
		// starts from the point along each axis, as a share of its size and
		// an offset, and whether it follows the pointer (1 or 0).
		shift, shareX, offX, shareY, offY, pointer := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0),
			core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
		off := float64(t.offset)
		side := func(flipped bool) (float64, float64) {
			if flipped {
				return 1, off
			}
			return 0, -off
		}
		cx.Effect(func() {
			if t.placement == TooltipTop {
				shift.Set(-0.5)
				shareX.Set(0)
				offX.Set(0)
				shareY.Set(0)
				offY.Set(-off)
				pointer.Set(0)
				return
			}
			sx, ox := side(t.flipX.Get())
			sy, oy := side(t.flipY.Get())
			shift.Set(0)
			shareX.Set(sx)
			offX.Set(ox)
			shareY.Set(sy)
			offY.Set(oy)
			pointer.Set(1)
		})
		x := clamped(core.Add(at, core.Mul(t.tipW, shift)), t.plotW, t.tipW, shareX, offX)
		// At the top edge for TooltipTop: its offset there is the app's.
		y := core.Mul(clamped(t.y, t.plotH, t.tipH, shareY, offY), pointer)
		beside := t.placement == TooltipPointer
		// A row with no size whose item does not shrink: the content keeps
		// its own size and overflows the point on the side the alignment
		// gives it, its margins away from it.
		return core.View{
			Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Width: core.Pt(0), Height: core.Pt(0),
				Direction: core.Row, Animated: &core.AnimatedStyle{TranslateX: x, TranslateY: y, Opacity: fade}},
			StyleFunc: func() core.Style {
				s := core.Style{JustifyContent: core.JustifyStart, AlignItems: core.AlignStart}
				if beside && t.flipX.Get() {
					s.JustifyContent = core.JustifyEnd
				}
				if beside && t.flipY.Get() {
					s.AlignItems = core.AlignEnd
				}
				return s
			},
			PointerEvents: core.PointerEventsNone,
			Children: []core.Node{core.View{
				OnLayout: func(b core.Bounds) {
					t.tipW.Set(float64(b.Size.Width))
					t.tipH.Set(float64(b.Size.Height))
				},
				StyleFunc: func() core.Style {
					var m core.Edges
					if !beside {
						m.Top = t.offset
						return core.Style{Margin: m}
					}
					if t.flipX.Get() {
						m.Right = t.offset
					} else {
						m.Left = t.offset
					}
					if t.flipY.Get() {
						m.Bottom = t.offset
					} else {
						m.Top = t.offset
					}
					return core.Style{Margin: m}
				},
				Children: []core.Node{content},
			}},
		}
	})
}
