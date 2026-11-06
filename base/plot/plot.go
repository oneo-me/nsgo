package plot

import (
	"math"
	"slices"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// part is the class and styles the app gives one of a component's parts.
type part = parts.Part

// Scale is how an axis of a Plot places values, as gpui-base's
// scales do: a point scale places the rows in order, evenly from one edge
// to the other (one row in the middle), and a linear scale places a number
// in proportion to where it lies in its domain. The zero value is a point
// scale; a y axis is always linear, so there it is a linear scale with no
// domain.
type Scale struct {
	linear    bool
	domain    [2]float64
	hasDomain bool
}

// PointScale places the rows in order.
func PointScale() Scale { return Scale{} }

// LinearScale places values in proportion over the extent of domain's two
// values, in either order; without them, over the extent of what the plot
// draws.
func LinearScale(domain ...float64) Scale {
	s := Scale{linear: true}
	if len(domain) == 2 && isFinite(domain[0]) && isFinite(domain[1]) {
		s.domain, s.hasDomain = [2]float64{domain[0], domain[1]}, true
	}
	return s
}

// Motion is the timing of a plot's motion. Each is the app's; a nil
// one moves at once, as does every one while the user asks for reduced
// motion.
type Motion struct {
	// Pointer is the spring the hover's parts follow the row under the
	// pointer with.
	Pointer *core.SpringConfig
	// Enter fades the hover's parts in as the pointer comes over the plot,
	// and Exit fades them out after it leaves.
	Enter, Exit *core.TimingConfig
	// Appear draws the marks in from the leading edge, when the plot is
	// first shown with an AppearGeneration and whenever that changes.
	Appear *core.TimingConfig
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

// Plot is the frame of a chart, as gpui-base's: the rows of data, the
// scales of its x and y axes, its size as their range, the pointer over it
// and its motion. It draws nothing itself: its marks (Line, Area,
// Grid), its children, each fill it and draw in order over each other,
// natively from the rows, each in a canvas of its own.
//
// It reports the row nearest to the pointer along the x axis with
// OnHoverChange and shows the hover's parts the app styles (the cross
// line, a dot per mark, the tooltip), moved and faded on the platform's
// frame clock with Motion; the pointer moves them and never draws a mark
// again. Screen readers hear one image, Label and AccessibilityValue.
// Style sizes it, and the marks inherit its paint properties (Stroke,
// Fill, StrokeWidth, StrokeDash) as CSS inherits them.
type Plot struct {
	// Y is the values the marks draw unless they have their own, one per
	// row; NaN skips a row.
	Y func() []float64
	// X places the rows along a linear x scale; a row's index without it.
	X func() []float64
	// XScale is a point scale unless it returns LinearScale; YScale's
	// domain is the extent of what the marks draw unless it gives one.
	XScale, YScale func() Scale
	// NoHover draws the marks only, following no pointer: a sparkline in
	// a list.
	NoHover bool
	Motion  Motion
	// AppearGeneration opts in to Motion.Appear: the marks draw in when
	// the plot is first shown, and again whenever it changes.
	AppearGeneration func() int
	// Label says what the chart shows and what stands out;
	// AccessibilityValue the app's words for the row under the pointer.
	Label              string
	LabelFunc          func() string
	AccessibilityValue func() core.AccessibilityValue
	// CrossLineStyle styles the cross line, a view as tall as the plot
	// centered on the row under the pointer; there is none without it or
	// CrossLineClass.
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

// plotState is a plot's state, for its marks.
type plotState struct {
	y func() []float64
	// xMap places values along the x axis and xs is where each of the
	// plot's rows goes along it (NaN: nowhere).
	xMap  *core.Memo[plotXMap]
	xs    *core.Memo[[]float32]
	yMap  *core.Memo[plotLinear]
	size  *core.Signal[core.Size]
	marks *core.Signal[[]*plotMark]
	next  int
	// edge is how far the marks have drawn in, in points, while appearing
	// says the appear runs and the marks clip to it.
	edge      core.AnimatedNode
	appearing *core.Signal[bool]
	ratio     func() float32
}

// sized reports whether the plot has been laid out with an area to draw
// on; before that a mark records nothing.
func (s *plotState) sized() bool {
	size := s.size.Get()
	return size.Width > 0 && size.Height > 0
}

// clip records the appear's clip while it runs: the marks show from the
// leading edge to the animated edge, and bleed points past the others.
func (s *plotState) clip(d *core.Drawing, bleed float32) {
	if !s.appearing.Get() {
		return
	}
	size := s.size.Get()
	w, h := float32(size.Width), float32(size.Height)
	d.BeginPath().MoveTo(-bleed, -bleed).LineTo(w, -bleed).Bind(core.ArgX, s.edge).
		LineTo(w, h+bleed).Bind(core.ArgX, s.edge).LineTo(-bleed, h+bleed).ClosePath().Clip()
}

// add adds m while the current owner lives.
func (s *plotState) add(cx *core.Scope, m *plotMark) {
	s.next++
	m.key = s.next
	s.marks.Update(func(all []*plotMark) []*plotMark { return append(slices.Clip(all), m) })
	cx.Cleanup(func() {
		s.marks.Update(func(all []*plotMark) []*plotMark {
			return slices.DeleteFunc(slices.Clone(all), func(o *plotMark) bool { return o == m })
		})
	})
}

// plotXMap places values along a plot's x axis: row indices evenly over
// the range on a point scale, values in proportion on a linear one.
type plotXMap struct {
	point  bool
	len    int
	rng    [2]float32
	linear plotLinear
}

func (m plotXMap) tick(v float64) (float32, bool) {
	if !m.point {
		return m.linear.tick(v)
	}
	switch {
	case m.len == 0 || !isFinite(v):
		return 0, false
	case m.len == 1:
		return (m.rng[0] + m.rng[1]) / 2, true
	}
	return m.rng[0] + float32(v)*(m.rng[1]-m.rng[0])/float32(m.len-1), true
}

// ticks is where n rows go: by values on a linear scale, else by index.
func (m plotXMap) ticks(n int, values []float64) []float32 {
	out := make([]float32, n)
	for i := range out {
		v := float64(i)
		if !m.point && values != nil {
			v = values[i]
		}
		at, ok := m.tick(v)
		if !ok {
			at = float32(math.NaN())
		}
		out[i] = at
	}
	return out
}

// plotLinear maps a domain onto a range in proportion, as d3's
// scaleLinear: start to rng[0], start+diff to rng[1].
type plotLinear struct {
	start, diff float64
	rng         [2]float32
}

func newPlotLinear(domain [2]float64, rng [2]float32) plotLinear {
	lo, hi := min(domain[0], domain[1]), max(domain[0], domain[1])
	return plotLinear{start: lo, diff: hi - lo, rng: rng}
}

// tick is where v goes; a domain of one value puts it in the middle, as d3
// does.
func (l plotLinear) tick(v float64) (float32, bool) {
	if !isFinite(v) {
		return 0, false
	}
	ratio := 0.5
	if l.diff != 0 {
		ratio = (v - l.start) / l.diff
	}
	return l.rng[0] + float32(ratio)*(l.rng[1]-l.rng[0]), true
}

// plotExtent is the extent of some values, ok when any is finite.
type plotExtent struct {
	lo, hi float64
	ok     bool
}

func (e plotExtent) with(v float64) plotExtent {
	switch {
	case !isFinite(v):
		return e
	case !e.ok:
		return plotExtent{v, v, true}
	}
	return plotExtent{min(e.lo, v), max(e.hi, v), true}
}

func extentOf(values []float64) plotExtent {
	var e plotExtent
	for _, v := range values {
		e = e.with(v)
	}
	return e
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Render implements core.Node.
func (p Plot) Render(cx *core.Scope) core.Node {
	size := core.NewSignal(cx, core.Size{})
	marks := core.NewSignalFunc(cx, []*plotMark(nil), nil)
	y := func() []float64 {
		if p.Y == nil {
			return nil
		}
		return p.Y()
	}
	x := func() []float64 {
		if p.X == nil {
			return nil
		}
		return p.X()
	}
	rows := func() int {
		if p.X != nil {
			return len(p.X())
		}
		return len(y())
	}
	xMap := core.NewMemo(cx, func() plotXMap {
		rng := [2]float32{0, float32(size.Get().Width)}
		n := rows()
		if p.XScale == nil || !p.XScale().linear {
			return plotXMap{point: true, len: n, rng: rng}
		}
		s := p.XScale()
		domain := s.domain
		if !s.hasDomain {
			e := extentOf(x())
			if p.X == nil && n > 0 {
				e = plotExtent{0, float64(n - 1), true}
			}
			domain = [2]float64{e.lo, e.hi}
		}
		return plotXMap{linear: newPlotLinear(domain, rng)}
	})
	xs := core.NewMemoFunc(cx, func() []float32 { return xMap.Get().ticks(rows(), x()) }, nil)
	yMap := core.NewMemo(cx, func() plotLinear {
		rng := [2]float32{float32(size.Get().Height), 0}
		if p.YScale != nil {
			if s := p.YScale(); s.hasDomain {
				return newPlotLinear(s.domain, rng)
			}
		}
		var e plotExtent
		for _, m := range marks.Get() {
			if me := m.extent.Get(); me.ok {
				e = e.with(me.lo).with(me.hi)
			}
		}
		return newPlotLinear([2]float64{e.lo, e.hi}, rng)
	})

	// The hover's place: the plot's size and the tooltip's as animated
	// values, so the platform keeps the tooltip inside the plot.
	place := newTooltipPlace(cx, p.TooltipPlacement, p.TooltipOffset)
	state := &plotState{y: y, xMap: xMap, xs: xs, yMap: yMap, size: size, marks: marks,
		appearing: core.NewSignal(cx, false), ratio: cx.Env().PixelRatio}
	reveal := core.NewAnimatedValue(cx, 1)
	state.edge = core.Mul(reveal, place.plotW)
	plotKey.Provide(cx, state)

	// The appear: from the first frame with a size, and again for each
	// new generation; at once without a timing or with reduced motion.
	if p.AppearGeneration != nil {
		shown := core.NewMemo(cx, func() bool { return size.Get().Width > 0 })
		cx.Effect(func() {
			p.AppearGeneration()
			if !shown.Get() {
				return
			}
			still := core.Untracked(cx, cx.Env().ReduceMotion)
			if t := p.Motion.Appear; t != nil && t.Duration > 0 && !still {
				reveal.Set(0)
				state.appearing.Set(true)
				// A newer generation stops this one and ends it itself.
				cx.StartAnimation(core.Timing(reveal, 1, *t), func(finished bool) {
					if finished {
						state.appearing.Set(false)
					}
				})
				return
			}
			reveal.Set(1)
			state.appearing.Set(false)
		})
	}

	// The hover: where its parts are and how far they have faded in.
	crossX := place.x
	fade := core.NewAnimatedValue(cx, 0)
	hovered := -1
	fadeTo := func(to float64, t *core.TimingConfig, still bool) {
		if t != nil && !still {
			cx.StartAnimation(core.Timing(fade, to, *t), nil)
		} else {
			fade.Set(to)
		}
	}
	follow := func(index int) {
		was := hovered
		if was == index {
			return
		}
		hovered = index
		if p.OnHoverChange != nil {
			p.OnHoverChange(index)
		}
		still := core.Untracked(cx, cx.Env().ReduceMotion)
		if index < 0 {
			fadeTo(0, p.Motion.Exit, still)
			return
		}
		all := xs.GetUntracked()
		if index >= len(all) {
			return
		}
		at := all[index]
		place.rowAt(at)
		// Entering, the parts take the row where they are, then fade in;
		// over the plot they glide to the next one on the pointer spring.
		entering := was < 0
		to := func(v *core.AnimatedValue, target float32) {
			if s := p.Motion.Pointer; s != nil && !entering && !still {
				cx.StartAnimation(core.Spring(v, float64(target), *s), nil)
			} else {
				v.Set(float64(target))
			}
		}
		to(crossX, at)
		for _, m := range marks.GetUntracked() {
			if q, ok := m.near(at); ok {
				to(m.dotX, float32(q.X))
				to(m.dotY, float32(q.Y))
			}
		}
		if entering {
			fadeTo(1, p.Motion.Enter, still)
		}
	}

	hasCross := parts.Of(p.CrossLineClass, p.CrossLineStyle, p.CrossLineStyleFunc).Set()
	hasDots := parts.Of(p.DotClass, p.DotStyle, p.DotStyleFunc).Set()
	dotStyle := func() core.Style {
		if p.DotStyleFunc == nil {
			return p.DotStyle
		}
		return p.DotStyle.Merge(p.DotStyleFunc())
	}
	children := slices.Clone(p.Children)
	if !p.NoHover {
		if hasCross {
			children = append(children, core.View{
				Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0),
					Width: core.Pt(0), AlignItems: core.AlignCenter,
					Animated: &core.AnimatedStyle{TranslateX: crossX, Opacity: fade}},
				PointerEvents: core.PointerEventsNone,
				Children: []core.Node{core.View{Class: p.CrossLineClass,
					Style: core.Style{Height: core.Pct(100)}.Merge(p.CrossLineStyle), StyleFunc: p.CrossLineStyleFunc}},
			})
		}
		// A dot per mark, styled by the mark or else by the plot; none
		// where neither styles one.
		children = append(children, core.Each(marks.Get, func(m *plotMark) int { return m.key },
			func(cx *core.Scope, item func() *plotMark, _ func() int) core.Node {
				m := item()
				if !m.dot.Set() && !hasDots {
					return core.Group{}
				}
				return dotAnchor(m.dotX, m.dotY, fade, core.View{Class: parts.JoinClasses(p.DotClass, m.dot.Class),
					StyleFunc: func() core.Style {
						s := dotStyle().Merge(m.dot.Style)
						if m.dot.Fn != nil {
							s = s.Merge(m.dot.Fn())
						}
						return s
					}})
			}))
		if p.Tooltip != nil {
			children = append(children, place.view(fade, p.Tooltip))
		}
	}

	return core.View{
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
		OnMouseMove: func(e core.MouseEvent) {
			if p.NoHover {
				return
			}
			place.pointerAt(float32(e.Position.Y))
			at := float32(e.Position.X)
			nearest, best := -1, float32(math.Inf(1))
			for i, x := range xs.GetUntracked() {
				if d := float32(math.Abs(float64(x - at))); d < best {
					nearest, best = i, d
				}
			}
			follow(nearest)
		},
		OnMouseLeave: func(core.MouseEvent) { follow(-1) },
		Children:     children,
	}
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
func (t *tooltipPlace) view(fade core.AnimatedNode, content core.Node) core.Node {
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
		x := clamped(core.Add(t.x, core.Mul(t.tipW, shift)), t.plotW, t.tipW, shareX, offX)
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
