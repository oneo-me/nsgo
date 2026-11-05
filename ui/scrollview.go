package ui

import "github.com/oneo-me/nsgo/core"

// edgeFade is how far content fades at an edge it scrolls past.
const edgeFade core.Points = 24

// ScrollView scrolls its Children (a column, or a row when Horizontal)
// and fades them into EdgeColor at each edge more of them lies past, so
// the reader sees there is more. TopBar and BottomBar float the app's
// controls over a vertical view's edges on a FloatingBar, and the
// content starts and ends clear of them. The app sizes it, as a
// core.Scroll.
type ScrollView struct {
	Horizontal bool
	Children   []core.Node
	TopBar     []core.Node
	BottomBar  []core.Node
	// TopBarLabel and BottomBarLabel name the bars for assistive
	// technology.
	TopBarLabel, BottomBarLabel string
	// EdgeColor is what the content scrolls over: the page's background
	// when nil.
	EdgeColor func() core.Color
	OnScroll  func(core.ScrollEvent)
	Ref       *core.Ref
	TestID    string
	// Style merges over the frame's; ContentStyle over the content's,
	// such as its padding and gap.
	Style        core.Style
	ContentStyle core.Style
}

// Render implements core.Node.
func (v ScrollView) Render(cx *core.Scope) core.Node {
	color := v.EdgeColor
	if color == nil {
		color = ColorsOf(cx).Background
	}
	offset := core.NewAnimatedValue(cx, 0)
	viewport, content := core.NewSignal(cx, core.Points(0)), core.NewSignal(cx, core.Points(0))
	along := func(s core.Size) core.Points {
		if v.Horizontal {
			return s.Width
		}
		return s.Height
	}
	values := core.AnimatedXY{Y: offset}
	if v.Horizontal {
		values = core.AnimatedXY{X: offset}
	}
	bars := !v.Horizontal
	const bar = ButtonHeight + 8
	var pad core.Edges
	var top, bottom core.Points
	if bars && v.TopBar != nil {
		top = 8 + bar
		pad.Top = top + 8
	}
	if bars && v.BottomBar != nil {
		bottom = 8 + bar
		pad.Bottom = bottom + 8
	}
	inner := core.Style{Padding: pad}
	if v.Horizontal {
		inner.Direction = core.Row
	}
	scroll := core.Scroll{Horizontal: v.Horizontal, OffsetValues: values, Ref: v.Ref, TestID: v.TestID, OnScroll: v.OnScroll,
		Style:    core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0)},
		OnLayout: func(b core.Bounds) { viewport.Set(along(b.Size)) },
		Children: []core.Node{core.View{Style: inner.Merge(v.ContentStyle), Children: v.Children,
			OnLayout: func(b core.Bounds) { content.Set(along(b.Size)) }}}}
	// fade is the band at one edge, shown as far as the content goes on
	// past it.
	fade := func(end bool, extent core.Points) core.Node {
		s := core.Style{Position: core.Absolute}
		size := core.Pt(extent + edgeFade)
		angle := float32(180)
		switch {
		case v.Horizontal && !end:
			s.Top, s.Bottom, s.Left, s.Width, angle = core.Pt(0), core.Pt(0), core.Pt(0), size, 90
		case v.Horizontal:
			s.Top, s.Bottom, s.Right, s.Width, angle = core.Pt(0), core.Pt(0), core.Pt(0), size, 270
		case !end:
			s.Left, s.Right, s.Top, s.Height = core.Pt(0), core.Pt(0), core.Pt(0), size
		default:
			s.Left, s.Right, s.Bottom, s.Height, angle = core.Pt(0), core.Pt(0), core.Pt(0), size, 0
		}
		return core.View{PointerEvents: core.PointerEventsNone, Style: s, StyleFunc: func() core.Style {
			room := float64(content.Get() - viewport.Get())
			if room <= 0 {
				return core.Style{Opacity: core.Alpha(0)}
			}
			input, output := []float64{0, float64(edgeFade)}, []float64{0, 1}
			if end {
				input, output = []float64{room - float64(edgeFade), room}, []float64{1, 0}
			}
			c := color()
			return core.Style{Gradients: []core.Gradient{{Kind: core.GradientLinear, Angle: angle, Stops: []core.ColorStop{
				{Color: c, At: core.Pt(0)}, {Color: c.Faded(0.9), At: core.Pt(extent)}, {Color: c.Faded(0), At: core.Pct(100)}}}},
				Animated: &core.AnimatedStyle{Opacity: core.Interpolate(offset,
					core.Interpolation{Input: input, Output: output, Extrapolate: core.ExtrapolateClamp})}}
		}}
	}
	children := []core.Node{scroll, fade(false, top), fade(true, bottom)}
	float := func(items []core.Node, label string, end bool) core.Node {
		s := core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 8)}
		if end {
			s.Bottom = core.Pt(8)
		} else {
			s.Top = core.Pt(8)
		}
		return core.View{PointerEvents: core.PointerEventsBoxNone, Style: s, Children: []core.Node{FloatingBar{Children: items, Label: label}}}
	}
	if bars && v.TopBar != nil {
		children = append(children, float(v.TopBar, v.TopBarLabel, false))
	}
	if bars && v.BottomBar != nil {
		children = append(children, float(v.BottomBar, v.BottomBarLabel, true))
	}
	return core.View{Style: core.Style{FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0), Overflow: core.Hidden}.Merge(v.Style),
		Children: children}
}
