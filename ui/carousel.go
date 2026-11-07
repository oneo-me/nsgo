package ui

import (
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Carousel is base.Carousel in Onevra's look, a row of Count slides that
// snaps one to the center, its neighbors in view, smaller and dimmer: a
// swipe or a trackpad moves it, and the arrows and the indicators below
// choose a slide. The app keeps the centered slide's Index and sets it from
// OnChange as the user's scroll rests; setting it glides the slides there.
// Every slide stays built, so keep the set short.
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
	index := func() int {
		if c.Index == nil {
			return 0
		}
		return c.Index()
	}
	var slides []core.Node
	for i := range n {
		slides = append(slides, base.CarouselItem{Label: label(i, n), Children: []core.Node{c.Slide(cx, i)}, StyleFunc: func() core.Style {
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
		}})
	}
	track := base.CarouselContent{TestID: c.TestID, OffsetValues: core.AnimatedXY{X: offset}, OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) },
		Style: core.Style{AlignItems: core.AlignStretch, Gap: gap}, Children: slides,
		StyleFunc: func() core.Style {
			w := width.Get()
			return core.Style{Padding: core.Symmetric(0, max((w-slideWidth(w))/2, 0))}
		}}
	var dots []core.Node
	for i := range n {
		dot := core.Style{Height: core.Pt(6), Width: core.Pt(6), CornerRadius: RadiusFull}
		if animate {
			dot.Transition = quick
		}
		dots = append(dots, base.CarouselPaginationItem{Index: i, Label: label(i, n), HitSlop: core.All(4), Style: dot,
			StyleFunc: func() core.Style {
				return core.Style{Background: colors.Border(), Selected: &core.Style{Width: core.Pt(22), Background: colors.Accent()},
					Focus: FocusRing(cx, 2)}
			}})
	}
	arrow := func(icon func() *core.SVG) core.Node { return Icon(icon, iconSize, Inherited) }
	face := core.Style{Width: core.Pt(ButtonHeight), Height: core.Pt(ButtonHeight), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		CornerRadius: RadiusControl, Transition: quick, Disabled: Dimmed()}
	look := func() core.Style {
		return core.Style{Color: colors.Text(), Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}, Focus: FocusRing(cx, 2)}
	}
	controls := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 12},
		Children: []core.Node{
			base.CarouselPrevious{Label: c.PreviousLabel, Style: face, StyleFunc: look, Children: []core.Node{arrow(lucide.ChevronLeft)}},
			base.CarouselPagination{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: dots},
			base.CarouselNext{Label: c.NextLabel, Style: face, StyleFunc: look, Children: []core.Node{arrow(lucide.ChevronRight)}},
		}}
	return base.Carousel{Index: func() int { return index() }, OnChange: c.OnChange, Label: c.Label, Style: core.Style{Gap: 12}.Merge(c.Style),
		Children: []core.Node{track, controls}}
}
