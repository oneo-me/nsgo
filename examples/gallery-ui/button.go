package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Press is a pressable that sinks a little under the finger and springs
// back; the spring runs on the platform, only press in and out reach the
// engine.
type Press struct {
	OnPress, OnLongPress func()
	Disabled             func() bool
	// Scale is how far it shrinks (default 0.97).
	Scale float64
	// StyleFunc merges over Style, as a View's does.
	Style     core.Style
	StyleFunc func() core.Style
	// Role is a button's by default.
	Role core.Role
	// LabelFunc replaces Label, as a View's does.
	Label     string
	LabelFunc func() string
	Selected  func() bool
	HitSlop   core.Points
	TestID    string
	Children  []core.Node
}

// Render implements core.Node.
func (p Press) Render(cx *core.Scope) core.Node {
	s := core.NewAnimatedValue(cx, 1)
	scale := p.Scale
	if scale == 0 {
		scale = 0.97
	}
	role := p.Role
	if role == 0 {
		role = core.RoleButton
	}
	var onPress, onLongPress func(core.PressEvent)
	if p.OnPress != nil {
		onPress = func(core.PressEvent) { p.OnPress() }
	}
	if p.OnLongPress != nil {
		onLongPress = func(core.PressEvent) { p.OnLongPress() }
	}
	style := p.Style
	style.Animated = &core.AnimatedStyle{Scale: s}
	style.Disabled = &core.Style{Opacity: core.Alpha(0.4)}
	return core.View{OnPress: onPress, OnLongPress: onLongPress, Disabled: p.Disabled, Selected: p.Selected,
		OnPressDown: func(core.PressEvent) { Spring(cx, s, scale, pressSpring) },
		OnPressUp:   func(core.PressEvent) { Spring(cx, s, 1, pressSpring) },
		Role:        role, Label: p.Label, LabelFunc: p.LabelFunc, HitSlop: core.All(p.HitSlop), TestID: p.TestID,
		Style: style, StyleFunc: p.StyleFunc, Children: p.Children}
}

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
// look is the gallery's. Small is the same button, kept for callers that
// ask for a dense one.
type Button struct {
	Text    string
	Variant ButtonVariant
	// Color fills it with that color instead.
	Color func() core.Color
	Icon  IconName
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
	if b.Icon != "" {
		children = append(children, Icon(b.Icon, 14, fg))
	}
	if b.Text != "" || b.Icon == "" {
		children = append(children, Text(b.Text, 13, Regular, fg))
	}
	shape := core.Style{Height: core.Pt(ButtonHeight), Padding: core.Symmetric(0, 14)}
	if b.Text == "" && b.Icon != "" {
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

// RoundButton is a round icon button, 32 pt unless Size says otherwise:
// the inverse of the page as ButtonPrimary, the default, and Onevra's
// raised toolbar face otherwise. IconFunc and LabelFunc replace Icon and
// Label.
type RoundButton struct {
	Icon      IconName
	IconFunc  func() IconName
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
		name = func() IconName { return b.Icon }
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
	return Press{OnPress: b.OnPress, Scale: 0.96, LabelFunc: label, HitSlop: max(0, (44-size)/2), Style: core.Style{Group: "button", Cursor: core.CursorPointer},
		Children: []core.Node{core.View{StyleFunc: face, Children: []core.Node{IconOf(name, size*0.45, fg)}}}}
}

// IconButton is a square icon button with no fill at rest, Size points
// across (28 when 0), as a toolbar's: a fill under the pointer, held
// down and while Selected, faded while Disabled. IconFunc and LabelFunc
// replace Icon and Label, what assistive technology reads.
type IconButton struct {
	Icon      IconName
	IconFunc  func() IconName
	Label     string
	LabelFunc func() string
	Size      core.Points
	Selected  func() bool
	Disabled  func() bool
	OnPress   func()
	TestID    string
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
		name = func() IconName { return b.Icon }
	}
	return core.View{OnPress: func(core.PressEvent) { b.OnPress() }, Role: core.RoleButton, Label: b.Label, LabelFunc: b.LabelFunc,
		Selected: b.Selected, Disabled: b.Disabled, TestID: b.TestID, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff},
		Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: 8, Cursor: core.CursorPointer, Disabled: dimmed()},
		StyleFunc: func() core.Style {
			return core.Style{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}, Focus: FocusRing(cx, -2)}
		},
		Children: []core.Node{IconOf(name, size*0.6, colors.Muted)}}
}

// ToggleButton is Onevra's toggle button, 28 pt tall: a quiet bordered
// face while off, filled with the accent while On, Text with an optional
// leading Icon or the Icon alone. A press asks OnChange for the other
// state; assistive technology hears it as a selected button. Label is
// what it reads, Text when empty.
type ToggleButton struct {
	Text     string
	Icon     IconName
	Label    string
	On       func() bool
	OnChange func(on bool)
	Disabled func() bool
	TestID   string
}

// Render implements core.Node.
func (b ToggleButton) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := b.Label
	if label == "" {
		label = b.Text
	}
	ink := func() core.Color {
		if is(b.On) {
			return colors.OnAccent()
		}
		return colors.Text()
	}
	var children []core.Node
	if b.Icon != "" {
		children = append(children, Icon(b.Icon, 14, ink))
	}
	if b.Text != "" || b.Icon == "" {
		t := Text(b.Text, 13, Regular, ink)
		t.MaxLines = 1
		children = append(children, t)
	}
	shape := core.Style{Height: core.Pt(ButtonHeight), Padding: core.Symmetric(0, 14)}
	if b.Text == "" && b.Icon != "" {
		shape = core.Style{Width: core.Pt(ButtonHeight), Height: core.Pt(ButtonHeight)}
	}
	face := func() core.Style {
		s := shape.Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
			CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, Transition: quick})
		if is(b.On) {
			p := cx.Palette()
			s.Background, s.BorderColor = p.Accent, p.Accent
			s.GroupStyle = &core.GroupStyle{Hover: &core.Style{Background: p.AccentHover, BorderColor: p.AccentHover},
				Active: &core.Style{Background: p.AccentPressed, BorderColor: p.AccentPressed}}
			return s
		}
		s.Background, s.BorderColor = transparent, colors.Border()
		s.GroupStyle = &core.GroupStyle{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}
		return s
	}
	return base.Button{Label: label, TestID: b.TestID, Selected: b.On, Disabled: b.Disabled,
		OnPress: func(core.PressEvent) {
			if b.OnChange != nil {
				b.OnChange(!is(b.On))
			}
		},
		Style:     core.Style{Group: "button", CornerRadius: RadiusControl, Cursor: core.CursorPointer, Disabled: dimmed()}.Merge(slop(cx, ButtonHeight)),
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Children:  []core.Node{core.View{StyleFunc: face, Children: children}}}
}
