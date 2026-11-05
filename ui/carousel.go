package ui

import (
	"math"
	"strconv"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

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
	animate := !ReduceMotion(cx)
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
		dot := core.Style{Height: core.Pt(6), Width: core.Pt(6), CornerRadius: RadiusFull}
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
	arrow := func(icon func() *core.SVG, name string, by int) core.Node {
		return IconButton{Icon: icon, Label: name, OnPress: func() { choose(core.Untracked(cx, index) + by) },
			Disabled: func() bool { i := shown.Get() + by; return i < 0 || i >= n }}
	}
	controls := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 12},
		Children: []core.Node{
			arrow(lucide.ChevronLeft, c.PreviousLabel, -1),
			core.View{Role: core.RoleTabList, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: dots},
			arrow(lucide.ChevronRight, c.NextLabel, 1),
		}}
	return core.View{Label: c.Label, Style: core.Style{Gap: 12}.Merge(c.Style), Children: []core.Node{track, controls}}
}
