package ui

import "github.com/oneo-me/nsgo/core"

// RoundButton is a round icon button, 32 pt unless Size says otherwise:
// the inverse of the page as ButtonPrimary, the default, and Onevra's
// raised toolbar face otherwise. IconFunc and LabelFunc replace Icon and
// Label.
type RoundButton struct {
	Icon      func() *core.SVG
	IconFunc  func() *core.SVG
	Label     string
	LabelFunc func() string
	Variant   ButtonVariant
	Size      core.Points
	OnPress   func()
}

// Render implements core.Node.
func (b RoundButton) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	size := b.Size
	if size == 0 {
		size = 32
	}
	name := b.IconFunc
	if name == nil {
		name = b.Icon
	}
	label := b.LabelFunc
	if label == nil {
		label = constant(b.Label)
	}
	primary := b.Variant == ButtonPrimary
	fg := colors.Text
	if primary {
		fg = colors.OnLight
	}
	face := func() core.Style {
		s := core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
		if primary {
			s.Background = colors.Light()
		} else {
			s = FinOf(cx).Raised().Over(s)
			s.Background, s.BorderWidth, s.BorderColor = colors.Raised(), 1, colors.Border()
			s.GroupStyle = &core.GroupStyle{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}
		}
		return s
	}
	return Press{OnPress: b.OnPress, Scale: 0.96, LabelFunc: label, HitSlop: max(0, (44-size)/2), Style: core.Style{Group: "button"},
		Children: []core.Node{core.View{StyleFunc: face, Children: []core.Node{IconOf(name, size*0.45, fg)}}}}
}
