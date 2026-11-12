package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// IconButtonVariant is an IconButton's look.
type IconButtonVariant uint8

const (
	// IconButtonGhost is a square with no fill at rest, as a toolbar's: a
	// fill under the pointer, held down and while Selected. It is the
	// default.
	IconButtonGhost IconButtonVariant = iota
	// IconButtonPrimary is a circle filled with the accent, for the main
	// action.
	IconButtonPrimary
	// IconButtonRaised is a circle on a raised toolbar face.
	IconButtonRaised
)

// IconButton is a button of an icon alone, Size points across, faded while
// Disabled. IconFunc and LabelFunc replace Icon and Label, what assistive
// technology reads.
type IconButton struct {
	Icon      func() *core.SVG
	IconFunc  func() *core.SVG
	Label     string
	LabelFunc func() string
	Variant   IconButtonVariant
	// Size is 28 for IconButtonGhost and 32 for a circle when 0.
	Size core.Points
	// IconSize is the glyph's, 60% of Size for IconButtonGhost and 45% for
	// a circle when 0, and Tint its color, the look's when nil.
	IconSize core.Points
	Tint     func() core.Color
	Selected func() bool
	Disabled func() bool
	OnPress  func()
	TestID   string
}

// Render implements core.Node.
func (b IconButton) Render(cx *core.Scope) core.Node {
	if b.Variant != IconButtonGhost {
		return b.circle(cx)
	}
	colors := ColorsOf(cx)
	size := b.Size
	if size == 0 {
		size = 28
	}
	tint := b.Tint
	if tint == nil {
		tint = colors.Text
	}
	return base.Button{OnPress: func(core.PressEvent) { b.OnPress() }, Label: b.Label, LabelFunc: b.LabelFunc,
		Selected: b.Selected, Disabled: b.Disabled, TestID: b.TestID, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff},
		Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: 8, Disabled: Dimmed()},
		StyleFunc: func() core.Style {
			return core.Style{Hover: &core.Style{Background: Alpha(colors.Text(), 0.08)}, Active: &core.Style{Background: Alpha(colors.Text(), 0.14)}, FocusVisible: FocusRing(cx, -2)}
		},
		Children: []core.Node{IconOf(b.name(), b.glyph(size, 0.6), tint)}}
}

// circle is the round looks: a face that sinks under the finger.
func (b IconButton) circle(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	size := b.Size
	if size == 0 {
		size = 32
	}
	primary := b.Variant == IconButtonPrimary
	tint := b.Tint
	if tint == nil {
		tint = colors.Text
		if primary {
			tint = colors.OnAccent
		}
	}
	face := func() core.Style {
		s := core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
		if primary {
			p := cx.Palette()
			s.Background = p.Accent
			s.GroupStyle = &core.GroupStyle{Hover: &core.Style{Background: p.AccentHover}, Active: &core.Style{Background: p.AccentPressed}}
		} else {
			s = FinOf(cx).Raised().Over(s)
			s.Background, s.BorderWidth, s.BorderColor = colors.Raised(), 1, colors.Border()
			s.GroupStyle = &core.GroupStyle{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}
		}
		return s
	}
	return Press{OnPress: b.OnPress, Scale: 0.96, Label: b.Label, LabelFunc: b.LabelFunc, Selected: b.Selected, Disabled: b.Disabled, TestID: b.TestID,
		HitSlop: max(0, (44-size)/2), Style: core.Style{Group: "button"},
		Children: []core.Node{core.View{StyleFunc: face, Children: []core.Node{IconOf(b.name(), b.glyph(size, 0.45), tint)}}}}
}

// name is what gives the icon.
func (b IconButton) name() func() *core.SVG {
	if b.IconFunc != nil {
		return b.IconFunc
	}
	return b.Icon
}

// glyph is the icon's size, ratio of size unless IconSize sets it.
func (b IconButton) glyph(size core.Points, ratio float64) core.Points {
	if b.IconSize != 0 {
		return b.IconSize
	}
	return size * core.Points(ratio)
}
