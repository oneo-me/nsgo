package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// scrollbarLook places a bar over a side of the view and draws its thumb,
// a capsule that shows while the view scrolls.
func scrollbarLook(cx *core.Scope, horizontal bool) *base.ScrollbarLook {
	colors := ColorsOf(cx)
	s := core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Right: core.Pt(0), Width: core.Pt(10), Padding: core.Symmetric(2, 3)}
	if horizontal {
		s = core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Height: core.Pt(10), Padding: core.Symmetric(3, 2)}
	}
	return &base.ScrollbarLook{Style: s, ThumbStyle: core.Style{CornerRadius: RadiusFull},
		ThumbStyleFunc: func() core.Style {
			return core.Style{Background: Alpha(colors.Text(), 0.3), Hover: &core.Style{Background: Alpha(colors.Text(), 0.45)},
				Active: &core.Style{Background: Alpha(colors.Text(), 0.55)}}
		}}
}

// ScrollbarTheme is ui's bars for every scroll view and list, which
// base.SetScrollbarTheme installs over the platform's indicators: they
// show while a view scrolls, as the platforms' overlay scrollers do, and
// fade out.
func ScrollbarTheme(cx *core.Scope) *base.ScrollbarTheme {
	return &base.ScrollbarTheme{Motion: base.ScrollbarMotion{Idle: 800, Enter: 120, Exit: 240},
		Vertical: scrollbarLook(cx, false), Horizontal: scrollbarLook(cx, true)}
}

// Scrollbar is a bar of ui's look for a scroll view or list it names,
// which the app places over the target's edge. The app turns the target's
// own indicator off with HideIndicator.
type Scrollbar struct {
	Scroll     func() *core.Ref
	Horizontal bool
	Mode       func() base.ScrollbarMode
	Label      string
	TestID     string
	Style      core.Style
}

// Render implements core.Node.
func (s Scrollbar) Render(cx *core.Scope) core.Node {
	look := scrollbarLook(cx, s.Horizontal)
	return base.Scrollbar{Scroll: s.Scroll, Horizontal: s.Horizontal, Mode: s.Mode, Label: s.Label, TestID: s.TestID,
		Style: look.Style.Merge(s.Style), ThumbStyle: look.ThumbStyle, ThumbStyleFunc: look.ThumbStyleFunc}
}
