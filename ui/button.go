package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// ButtonVariant is a button's look, one of Onevra's.
type ButtonVariant uint8

const (
	// ButtonPrimary is filled with the accent, for the main action. It is
	// the default.
	ButtonPrimary ButtonVariant = iota
	// ButtonSecondary is a quiet fill of the text's color.
	ButtonSecondary
	// ButtonOutline is only a rim.
	ButtonOutline
	// ButtonGhost has no rim or fill at rest, only a fill under the pointer
	// and held down, as an IconButton's, such as a menu's trigger in a
	// toolbar.
	ButtonGhost
)

// Button is Onevra's button, as tall as its Size: Text with an optional
// leading Icon, or the Icon alone in a square. base.Button gives the
// behavior; the look is ui's.
type Button struct {
	Text    string
	Variant ButtonVariant
	// Color fills it with that color instead.
	Color func() core.Color
	Icon  func() *core.SVG
	// Label is what assistive technology reads, Text when empty: give it
	// to a button of an icon alone.
	Label    string
	Disabled func() bool
	// Flex grows it to share a row with its siblings.
	Flex float32
	Size Size
	// Side is the square of a button of an icon alone, its height when 0,
	// to line it up with what is not a control's height. IconSize is its
	// glyph's, the Size's when 0, and Tint its color, the look's when nil.
	Side, IconSize core.Points
	Tint           func() core.Color
	TestID         string
	OnPress        func()
}

// ButtonHeight is a button's height at SizeDefault.
const ButtonHeight core.Points = 28

// Render implements core.Node.
func (b Button) Render(cx *core.Scope) core.Node {
	style, focus, face := b.parts(cx)
	var press func(core.PressEvent)
	if b.OnPress != nil {
		press = func(core.PressEvent) { b.OnPress() }
	}
	return base.Button{OnPress: press, Disabled: b.Disabled, Label: b.label(), TestID: b.TestID,
		Style: style, StyleFunc: focus, Children: []core.Node{face}}
}

// label is what assistive technology reads.
func (b Button) label() string {
	if b.Label != "" {
		return b.Label
	}
	return b.Text
}

// parts are the button's pressable style, its focus ring and its face,
// which follows the pressable's hover and press: what a popup's trigger
// takes from a Button.
func (b Button) parts(cx *core.Scope) (core.Style, func() core.Style, core.Node) {
	colors := ColorsOf(cx)
	fg := colors.Text
	switch {
	case b.Color != nil:
		fg = Fixed(core.RGB(0xffffff))
	case b.Variant == ButtonPrimary:
		fg = colors.OnAccent
	}
	// fill is the face at rest, under the pointer and held down.
	fill := func() (rest, hover, pressed, border core.Color) {
		text := colors.Text()
		switch {
		case b.Color != nil:
			c := b.Color()
			return c, Mix(c, Black(1), 0.12), Mix(c, Black(1), 0.25), c
		case b.Variant == ButtonPrimary:
			p := cx.Palette()
			return p.Accent, p.AccentHover, p.AccentPressed, p.Accent
		case b.Variant == ButtonSecondary:
			// Over the card it sits on, as Onevra's is over the page.
			bg := colors.Surface()
			return Mix(bg, text, 0.08), Mix(bg, text, 0.12), Mix(bg, text, 0.18), Mix(bg, text, 0.08)
		}
		if b.Variant == ButtonGhost {
			return 0, colors.Hover(), colors.Pressed(), core.Transparent
		}
		return 0, colors.Hover(), colors.Pressed(), colors.Border()
	}
	var children []core.Node
	if b.Icon != nil {
		glyph, tint := b.IconSize, b.Tint
		if glyph == 0 {
			glyph = b.Size.glyph()
		}
		if tint == nil {
			tint = fg
		}
		children = append(children, Icon(b.Icon, glyph, tint))
	}
	if b.Text != "" || b.Icon == nil {
		children = append(children, Text(b.Text, b.Size.text().FontSize, Regular, fg))
	}
	shape := core.Style{Height: core.Pt(b.Size.Height()), Padding: core.Symmetric(0, b.Size.inset())}
	if b.Text == "" && b.Icon != nil {
		side := b.Side
		if side == 0 {
			side = b.Size.Height()
		}
		shape = core.Style{Width: core.Pt(side), Height: core.Pt(side)}
	}
	face := func() core.Style {
		rest, hover, pressed, border := fill()
		return shape.Merge(core.Style{CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
			Background: rest, BorderWidth: 1, BorderColor: border,
			GroupStyle: &core.GroupStyle{Hover: &core.Style{Background: hover}, Active: &core.Style{Background: pressed}, Expanded: &core.Style{Background: hover}}})
	}
	style := core.Style{Group: "button", FlexGrow: b.Flex, FlexShrink: b.Flex, CornerRadius: RadiusControl,
		Disabled: &core.Style{Opacity: core.Alpha(0.4)}}
	return style, func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} }, core.View{StyleFunc: face, Children: children}
}
