package ui

import "github.com/oneo-me/nsgo/core"

// IconButton is a square icon button with no fill at rest, Size points
// across (28 when 0), as a toolbar's: a fill under the pointer, held
// down and while Selected, faded while Disabled. IconFunc and LabelFunc
// replace Icon and Label, what assistive technology reads.
type IconButton struct {
	Icon      func() *core.SVG
	IconFunc  func() *core.SVG
	Label     string
	LabelFunc func() string
	Size      core.Points
	// IconSize is the glyph's, 60% of Size when 0, and Tint its color, the
	// muted text's when nil.
	IconSize core.Points
	Tint     func() core.Color
	Selected func() bool
	Disabled func() bool
	OnPress  func()
	TestID   string
}

// Render implements core.Node.
func (b IconButton) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	size := b.Size
	if size == 0 {
		size = 28
	}
	name := b.IconFunc
	if name == nil {
		name = b.Icon
	}
	glyph, tint := b.IconSize, b.Tint
	if glyph == 0 {
		glyph = size * 0.6
	}
	if tint == nil {
		tint = colors.Muted
	}
	return core.View{OnPress: func(core.PressEvent) { b.OnPress() }, Role: core.RoleButton, Label: b.Label, LabelFunc: b.LabelFunc,
		Selected: b.Selected, Disabled: b.Disabled, TestID: b.TestID, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff},
		Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: 8, Disabled: Dimmed()},
		StyleFunc: func() core.Style {
			return core.Style{Hover: &core.Style{Background: Alpha(colors.Text(), 0.08)}, Active: &core.Style{Background: Alpha(colors.Text(), 0.14)}, Focus: FocusRing(cx, -2)}
		},
		Children: []core.Node{IconOf(name, glyph, tint)}}
}
