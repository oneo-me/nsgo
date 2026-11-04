package ui

import (
	"math"
	"strconv"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// Image is a picture in a rounded well that shows Placeholder while it
// loads and if it fails: a faint picture icon when nil. It keeps
// AspectRatio, else the one Source declares, else 3:2, unless its style
// sizes both sides. With a Label it is an image to assistive technology;
// without one, decoration.
type Image struct {
	Source core.ImageSource
	Label  string
	// ResizeMode fits the picture into the well: Cover when 0.
	ResizeMode  core.ResizeMode
	AspectRatio float32
	Placeholder core.Node
	OnLoad      func(pixels core.Size)
	OnError     func(err error)
	// TestID names the picture, which tests report loads on.
	TestID string
	// Style merges over the well's.
	Style core.Style
}

// Render implements core.Node.
func (i Image) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	ratio := i.AspectRatio
	if ratio == 0 && i.Source.Size.Width > 0 && i.Source.Size.Height > 0 {
		ratio = float32(i.Source.Size.Width / i.Source.Size.Height)
	}
	if ratio == 0 {
		ratio = 3.0 / 2
	}
	loaded, failed := core.NewSignal(cx, false), core.NewSignal(cx, false)
	fade := 200 * time.Millisecond
	if reduceMotion(cx) {
		fade = time.Nanosecond
	}
	placeholder := func() core.Node {
		if i.Placeholder != nil {
			return i.Placeholder
		}
		return IconOf(func() IconName {
			if failed.Get() {
				return IconImageOff
			}
			return IconImage
		}, ButtonHeight, colors.Faint)
	}
	fill := core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)}
	children := []core.Node{
		core.Show{When: func() bool { return !loaded.Get() }, Then: func(cx *core.Scope) core.Node {
			return core.View{Style: fill.Merge(core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}),
				Children: []core.Node{placeholder()}}
		}},
	}
	if hasImage(i.Source) {
		children = append(children, core.Show{When: func() bool { return !failed.Get() }, Then: func(cx *core.Scope) core.Node {
			return core.Image{Source: i.Source, ResizeMode: i.ResizeMode, FadeDuration: fade, Label: i.Label, TestID: i.TestID, Style: fill,
				OnLoad: func(px core.Size) {
					loaded.Set(true)
					if i.OnLoad != nil {
						i.OnLoad(px)
					}
				},
				OnError: func(err error) {
					failed.Set(true)
					if i.OnError != nil {
						i.OnError(err)
					}
				}}
		}})
	}
	return core.View{Style: core.Style{AspectRatio: ratio, Overflow: core.Hidden, CornerRadius: RadiusControl}.Merge(i.Style),
		StyleFunc: Filled(colors.Inset), Children: children}
}

// compareStep is how far a screen reader's step moves a comparison's
// divider.
const compareStep = 5

// ImageCompare lays Before over After in a rounded frame and shows it up
// to a divider: Position is the share of the frame, from 0 to 100, that
// Before covers, 50 when nil. A press moves the divider there and a drag
// moves it along; screen readers step it as a slider named Label.
// BeforeLabel and AfterLabel caption the sides. Both sides are laid out
// at the frame's size, so their content lines up.
type ImageCompare struct {
	Before, After core.Node
	Position      func() float32
	OnChange      func(position float32)
	// Vertical puts Before on top, the divider across.
	Vertical                bool
	BeforeLabel, AfterLabel string
	Label                   string
	// AspectRatio is the frame's width over its height: 3:2 when 0.
	AspectRatio float32
	TestID      string
	Style       core.Style
}

// Render implements core.Node.
func (p ImageCompare) Render(cx *core.Scope) core.Node {
	ratio := p.AspectRatio
	if ratio == 0 {
		ratio = 3.0 / 2
	}
	size := core.NewSignal(cx, core.Size{})
	at := func() float32 {
		if p.Position == nil {
			return 50
		}
		return clampShare(p.Position())
	}
	set := func(v float32) {
		if v = clampShare(v); p.OnChange != nil && v != core.Untracked(cx, at) {
			p.OnChange(v)
		}
	}
	extent := func() core.Points {
		s := size.GetUntracked()
		if p.Vertical {
			return s.Height
		}
		return s.Width
	}
	var start float32
	pan := func(e core.PanEvent) {
		d, e0 := e.Translation.X, extent()
		if p.Vertical {
			d = e.Translation.Y
		}
		if e.Phase == core.GestureBegan {
			start = core.Untracked(cx, at)
		}
		if e0 > 0 {
			set(start + float32(d/e0*100))
		}
	}
	fill := core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0)}
	full := fill.Merge(core.Style{Right: core.Pt(0), Bottom: core.Pt(0)})
	reveal := core.View{Style: fill.Merge(core.Style{Overflow: core.Hidden}), StyleFunc: func() core.Style {
		if p.Vertical {
			return core.Style{Right: core.Pt(0), Height: core.Pct(at())}
		}
		return core.Style{Bottom: core.Pt(0), Width: core.Pct(at())}
	}, Children: []core.Node{core.View{Style: fill, StyleFunc: func() core.Style {
		s := size.Get()
		return core.Style{Width: core.Pt(s.Width), Height: core.Pt(s.Height)}
	}, Children: []core.Node{p.Before}}}}
	// The captions and the divider are white on black, which read on any
	// picture.
	white := core.RGB(0xffffff)
	caption := func(label string, s core.Style) core.Node {
		if label == "" {
			return nil
		}
		return core.View{PointerEvents: core.PointerEventsNone, Style: typeCaption.Merge(core.Style{Position: core.Absolute,
			Padding: core.Symmetric(2, 8), CornerRadius: radiusFull, Background: Black(0.5), Color: white, FontWeight: Medium}).Merge(s),
			Children: []core.Node{line(label, core.Style{}, 1)}}
	}
	inset := core.Pt(12)
	after := core.Style{Top: inset, Right: inset}
	if p.Vertical {
		after = core.Style{Bottom: inset, Left: inset}
	}
	const knob = 32
	chevrons := IconSplitX
	edge := core.Style{Position: core.Absolute, Background: white, Shadows: []core.BoxShadow{{Blur: 6, Color: Black(0.3)}},
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	if p.Vertical {
		chevrons = IconSplitY
		edge.Left, edge.Right, edge.Height, edge.Margin = core.Pt(0), core.Pt(0), core.Pt(2), core.Edges{Top: -1}
	} else {
		edge.Top, edge.Bottom, edge.Width, edge.Margin = core.Pt(0), core.Pt(0), core.Pt(2), core.Edges{Left: -1}
	}
	divider := core.View{PointerEvents: core.PointerEventsNone, Style: edge, StyleFunc: func() core.Style {
		if p.Vertical {
			return core.Style{Top: core.Pct(at())}
		}
		return core.Style{Left: core.Pct(at())}
	}, Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(knob), Height: core.Pt(knob), FlexShrink: 0, CornerRadius: knob / 2,
		Background: white, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		Shadows: []core.BoxShadow{{Offset: core.Point{Y: 1}, Blur: 4, Color: Black(0.3)}}},
		Children: []core.Node{Icon(chevrons, knob/2, Fixed(core.RGB(0x262626)))}}}}
	activation := core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12}
	cursor := core.CursorColResize
	if p.Vertical {
		activation, cursor = core.PanActivation{ActiveOffsetY: 2, FailOffsetX: 12}, core.CursorRowResize
	}
	children := []core.Node{core.View{Style: full, Children: []core.Node{p.After}}, reveal}
	for _, c := range []core.Node{caption(p.BeforeLabel, core.Style{Top: inset, Left: inset}), caption(p.AfterLabel, after)} {
		if c != nil {
			children = append(children, c)
		}
	}
	children = append(children, divider)
	return core.View{TestID: p.TestID, Role: core.RoleAdjustable, Label: p.Label, Accessible: true,
		AccessibilityValue: func() core.AccessibilityValue {
			return core.AccessibilityValue{Min: 0, Max: 100, Now: at(), Text: strconv.Itoa(int(math.Round(float64(at())))) + "%"}
		},
		AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
		OnAccessibilityAction: func(name string) {
			switch name {
			case core.ActionIncrement:
				set(core.Untracked(cx, at) + compareStep)
			case core.ActionDecrement:
				set(core.Untracked(cx, at) - compareStep)
			}
		},
		OnLayout: func(b core.Bounds) { size.Set(b.Size) },
		OnPress: func(e core.PressEvent) {
			if e0 := extent(); e0 > 0 {
				d := e.Position.X
				if p.Vertical {
					d = e.Position.Y
				}
				set(float32(d / e0 * 100))
			}
		},
		OnPan: pan, PanActivation: activation,
		Style: core.Style{AspectRatio: ratio, Overflow: core.Hidden, CornerRadius: RadiusCard, Cursor: cursor,
			MinWidth: core.Pt(80), MinHeight: core.Pt(80)}.Merge(p.Style),
		StyleFunc: Filled(ColorsOf(cx).Inset),
		Children:  children}
}

// clampShare keeps v from 0 to 100.
func clampShare(v float32) float32 { return min(max(v, 0), 100) }

// Carousel is a row of Count slides that snaps one to the center, its
// neighbors in view, smaller and dimmer: a swipe or a trackpad moves it,
// and the arrows and the indicators below choose a slide. The app keeps
// the centered slide's Index and sets it from OnChange, also as the user
// scrolls; setting it glides the slides there. Every slide stays built,
// so keep the set short.
type Carousel struct {
	Count int
	Slide func(cx *core.Scope, i int) core.Node
	Index func() int
	// OnChange receives the slide the user brings to the center.
	OnChange func(i int)
	// SlideWidth is the widest a slide grows: 340 points when 0. A slide
	// takes at most 80% of the carousel.
	SlideWidth core.Points
	// Label names the carousel, and SlideLabel slide i of count and its
	// indicator, "i+1 / count" when nil.
	Label                    string
	SlideLabel               func(i, count int) string
	PreviousLabel, NextLabel string
	TestID                   string
	Style                    core.Style
}

// Render implements core.Node.
func (c Carousel) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	n := max(c.Count, 0)
	animate := !reduceMotion(cx)
	width := core.NewSignal(cx, core.Points(0))
	offset := core.NewAnimatedValue(cx, 0)
	ref := &core.Ref{}
	clampIndex := func(i int) int { return min(max(i, 0), max(n-1, 0)) }
	index := func() int {
		if c.Index == nil {
			return 0
		}
		return clampIndex(c.Index())
	}
	shown := core.NewSignal(cx, core.Untracked(cx, index))
	const gap = 12
	slideWidth := func(w core.Points) core.Points {
		most := c.SlideWidth
		if most == 0 {
			most = 340
		}
		return max(min(most, w*0.8), 1)
	}
	step := func(w core.Points) core.Points { return slideWidth(w) + gap }
	label := c.SlideLabel
	if label == nil {
		label = func(i, count int) string { return strconv.Itoa(i+1) + " / " + strconv.Itoa(count) }
	}
	choose := func(i int) {
		if i = clampIndex(i); c.OnChange != nil && i != core.Untracked(cx, index) {
			c.OnChange(i)
		}
	}
	// The app's index scrolls the slides there unless the user's scroll
	// is what set it; a new width centers it again at once.
	var laidOut core.Points
	cx.Effect(func() {
		w, i := width.Get(), index()
		if w == 0 {
			return
		}
		resized := w != laidOut
		laidOut = w
		if resized || i != shown.GetUntracked() {
			ref.ScrollTo(core.Point{X: core.Points(i) * step(w)}, animate && !resized)
		}
	})
	onScroll := func(e core.ScrollEvent) {
		w := width.GetUntracked()
		if w == 0 {
			return
		}
		i := clampIndex(int(math.Round(float64(e.Offset.X / step(w)))))
		if i != shown.GetUntracked() {
			shown.Set(i)
			choose(i)
		}
	}
	var slides []core.Node
	for i := range n {
		slides = append(slides, core.View{Label: label(i, n), StyleFunc: func() core.Style {
			w := width.Get()
			s := core.Style{Width: core.Pt(slideWidth(w)), FlexShrink: 0}
			if w == 0 {
				return s
			}
			at := float64(core.Points(i) * step(w))
			d := float64(step(w))
			input := []float64{at - d, at, at + d}
			s.Animated = &core.AnimatedStyle{
				Scale: core.Interpolate(offset, core.Interpolation{Input: input, Output: []float64{0.9, 1, 0.9}, Extrapolate: core.ExtrapolateClamp}),
				Opacity: core.Interpolate(offset, core.Interpolation{Input: input, Output: []float64{0.5, 1, 0.5},
					Extrapolate: core.ExtrapolateClamp}),
			}
			return s
		}, Children: []core.Node{c.Slide(cx, i)}})
	}
	track := core.Scroll{Horizontal: true, HideIndicator: true, Ref: ref, TestID: c.TestID, OffsetValues: core.AnimatedXY{X: offset},
		BehaviorFunc: func() core.ScrollBehavior {
			return core.ScrollBehavior{SnapToInterval: step(width.Get()), DecelerationRate: core.DecelerationFast,
				DisableIntervalMomentum: true}
		},
		OnScroll: onScroll, OnMomentumEnd: onScroll, OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) },
		Children: []core.Node{core.View{StyleFunc: func() core.Style {
			w := width.Get()
			side := (w - slideWidth(w)) / 2
			return core.Style{Direction: core.Row, AlignItems: core.AlignStretch, Gap: gap, Padding: core.Symmetric(0, max(side, 0))}
		}, Children: slides}}}
	var dots []core.Node
	for i := range n {
		dot := core.Style{Height: core.Pt(6), Width: core.Pt(6), CornerRadius: radiusFull}
		if animate {
			dot.Transition = quick
		}
		dots = append(dots, core.View{Role: core.RoleTab, Label: label(i, n), Selected: func() bool { return shown.Get() == i },
			HitSlop: core.All(4), OnPress: func(core.PressEvent) { choose(i) }, Style: dot,
			StyleFunc: func() core.Style {
				return core.Style{Background: colors.Border(), Selected: &core.Style{Width: core.Pt(22), Background: colors.Accent()},
					Focus: FocusRing(cx, 2)}
			}})
	}
	arrow := func(icon IconName, name string, by int) core.Node {
		return IconButton{Icon: icon, Label: name, OnPress: func() { choose(core.Untracked(cx, index) + by) },
			Disabled: func() bool { i := shown.Get() + by; return i < 0 || i >= n }}
	}
	controls := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 12},
		Children: []core.Node{
			arrow(IconBack, c.PreviousLabel, -1),
			core.View{Role: core.RoleTabList, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: dots},
			arrow(IconChevron, c.NextLabel, 1),
		}}
	return core.View{Label: c.Label, Style: core.Style{Gap: 12}.Merge(c.Style), Children: []core.Node{track, controls}}
}
