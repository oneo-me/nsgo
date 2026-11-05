package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// ButtonVariant is a button's look, one of Onevra's.
type ButtonVariant uint8

const (
	// ButtonPrimary is the inverse of the page: the text's color with the
	// page's on it. It is the default.
	ButtonPrimary ButtonVariant = iota
	// ButtonSecondary is a quiet fill of the text's color.
	ButtonSecondary
	// ButtonAccent is filled with the accent, Onevra's primary.
	ButtonAccent
	// ButtonGhost is only a rim, Onevra's outline.
	ButtonGhost
)

// Button is Onevra's button, 28 pt tall: Text with an optional leading
// Icon, or the Icon alone in a square. base.Button gives the behavior; the
// look is ui's. Small is the same button, kept for callers that
// ask for a dense one.
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
	Flex    float32
	Small   bool
	TestID  string
	OnPress func()
}

// ButtonHeight is a button's height.
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
		fg = colors.OnLight
	case b.Variant == ButtonAccent:
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
			c := colors.Light()
			return c, Mix(c, colors.Background(), 0.12), Mix(c, colors.Background(), 0.25), c
		case b.Variant == ButtonAccent:
			p := cx.Palette()
			return p.Accent, p.AccentHover, p.AccentPressed, p.Accent
		case b.Variant == ButtonSecondary:
			// Over the card it sits on, as Onevra's is over the page.
			bg := colors.Surface()
			return Mix(bg, text, 0.08), Mix(bg, text, 0.12), Mix(bg, text, 0.18), Mix(bg, text, 0.08)
		}
		return 0, colors.Hover(), colors.Pressed(), colors.Border()
	}
	var children []core.Node
	if b.Icon != nil {
		children = append(children, Icon(b.Icon, 14, fg))
	}
	if b.Text != "" || b.Icon == nil {
		children = append(children, Text(b.Text, 13, Regular, fg))
	}
	shape := core.Style{Height: core.Pt(ButtonHeight), Padding: core.Symmetric(0, 14)}
	if b.Text == "" && b.Icon != nil {
		shape = core.Style{Width: core.Pt(ButtonHeight), Height: core.Pt(ButtonHeight)}
	}
	face := func() core.Style {
		rest, hover, pressed, border := fill()
		return shape.Merge(core.Style{CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
			Background: rest, BorderWidth: 1, BorderColor: border,
			GroupStyle: &core.GroupStyle{Hover: &core.Style{Background: hover}, Active: &core.Style{Background: pressed}}})
	}
	style := core.Style{Group: "button", FlexGrow: b.Flex, FlexShrink: b.Flex, CornerRadius: RadiusControl, Cursor: core.CursorPointer,
		Disabled: &core.Style{Opacity: core.Alpha(0.4)}}.Merge(slop(cx, ButtonHeight))
	return style, func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} }, core.View{StyleFunc: face, Children: children}
}
