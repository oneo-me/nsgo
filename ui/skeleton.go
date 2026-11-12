package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// skeletonStagger is how long after the block above each block starts
// its pulse, so a placeholder pulses in a calm wave.
const skeletonStagger = 90 * time.Millisecond

// SkeletonBlock is one pulsing shape of a placeholder, for content of
// the app's own shape: a rounded bar, or with Round a circle. Index
// staggers its pulse after the blocks before it.
type SkeletonBlock struct {
	Width, Height core.Length
	Round         bool
	Index         int
	Style         core.Style
}

// Render implements core.Node.
func (b SkeletonBlock) Render(cx *core.Scope) core.Node {
	s := core.Style{Width: b.Width, Height: b.Height, CornerRadius: RadiusSmall}
	if b.Round {
		s.CornerRadius, s.FlexShrink = RadiusFull, 0
	}
	if !ReduceMotion(cx) {
		s.Animation = core.KeyframeAnimation{Keyframes: core.KeyframesPulse, Delay: time.Duration(b.Index) * skeletonStagger}
	}
	colors := ColorsOf(cx)
	return core.View{Style: s.Merge(b.Style), StyleFunc: Filled(colors.Border)}
}

// Skeleton is a placeholder for text still loading: Lines bars (3 when 0,
// at most 6) as wide as it is, after a round avatar's with Avatar. Screen
// readers read it as one element named Label, such as "Loading".
type Skeleton struct {
	Lines  int
	Avatar bool
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (s Skeleton) Render(*core.Scope) core.Node {
	lines := s.Lines
	if lines == 0 {
		lines = 3
	}
	lines = min(max(lines, 1), 6)
	var children []core.Node
	index := 0
	if s.Avatar {
		d := core.Pt(avatarSize(0))
		children = append(children, SkeletonBlock{Width: d, Height: d, Round: true})
		index++
	}
	const bar = 12
	var bars []core.Node
	for i := range lines {
		width, height := core.Pct(100), core.Pt(bar*3/4)
		switch {
		case i == 0:
			width, height = core.Pct(48), core.Pt(bar)
		case i == lines-1:
			width = core.Pct(72)
		}
		bars = append(bars, SkeletonBlock{Width: width, Height: height, Index: index + i})
	}
	children = append(children, core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 10,
		Padding: core.Edges{Top: 4}}, Children: bars})
	return core.View{TestID: s.TestID, Accessible: true, Label: s.Label,
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignStart, Gap: 16}.Merge(s.Style), Children: children}
}

// SkeletonContent shows Placeholder while Loading and then the app's
// Children, which fade in.
type SkeletonContent struct {
	Loading     func() bool
	Placeholder Skeleton
	Children    []core.Node
	TestID      string
	Style       core.Style
}

// Render implements core.Node.
func (s SkeletonContent) Render(cx *core.Scope) core.Node {
	still := ReduceMotion(cx)
	return core.View{TestID: s.TestID, Style: s.Style, Children: []core.Node{core.Show{When: func() bool { return is(s.Loading) },
		Then: func(cx *core.Scope) core.Node { return s.Placeholder },
		Else: func(cx *core.Scope) core.Node {
			v := core.View{Children: s.Children}
			if !still {
				spec := core.FadeIn.Spec()
				spec.Timing.Duration = 250 * time.Millisecond
				v.Style.Animated = core.NewMotion(cx, &spec).Style()
			}
			return v
		}}}}
}
