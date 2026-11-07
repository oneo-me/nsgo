package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Menus are base's: a popup at the trigger or the pointer on desktops,
// whose submenus cascade; a sheet from the bottom on touch screens, whose
// submenus replace its items. Their items are MenuItem, MenuSeparator and
// Submenu, in any arrangement.

// Menu opens its items from a trigger, which Trigger's look and content
// draw.
type Menu struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	// Trigger is the trigger's look and content; its OnPress and Disabled
	// go unused.
	Trigger   Button
	Placement core.Placement
	Align     core.Align
	Children  []core.Node
	Label     string
	TestID    string
}

// Render implements core.Node.
func (m Menu) Render(cx *core.Scope) core.Node {
	style, focus, face := m.Trigger.parts(cx)
	popup, backdrop := menuSurface(cx)
	return base.Menu{Open: m.Open, OnOpenChange: m.OnOpenChange, Placement: m.Placement, Align: m.Align, Offset: 4, Margin: 8,
		Style: style, StyleFunc: focus, Trigger: face, PopupStyleFunc: popup, BackdropStyle: backdrop,
		Children: m.Children, Label: triggerLabel(m.Label, m.Trigger), TestID: m.TestID}
}

// menuSurface is the popup on desktops, or the sheet and its backdrop on
// Touch screens.
func menuSurface(cx *core.Scope) (popup func() core.Style, backdrop core.Style) {
	if !Touch(cx) {
		return func() core.Style {
			return FinOf(cx).Menu().Over(core.Style{Padding: core.All(4), MinWidth: core.Pt(200), CornerRadius: RadiusControl + 2,
				BorderWidth: 1, Color: ColorsOf(cx).Text()})
		}, core.Style{}
	}
	safe := core.Untracked(cx, cx.Env().SafeArea)
	return func() core.Style {
		return sheetPanel(cx, core.PlaceBottom).Merge(core.Style{Padding: core.Edges{Top: 8, Bottom: 8 + safe.Bottom,
			Left: safe.Left, Right: safe.Right}})
	}, scrim
}

// MenuItemVariant is a MenuItem's look.
type MenuItemVariant uint8

const (
	// MenuItemDefault is in the text's color.
	MenuItemDefault MenuItemVariant = iota
	// MenuItemDanger is in the danger color, for a destructive choice.
	MenuItemDanger
)

// MenuItem is a choice of a menu. Checked, when set, keeps a check before
// its text while it returns true, for an option of the menu.
type MenuItem struct {
	Text string
	Icon func() *core.SVG
	// Shortcut is the keys that choose it, shown at its end on desktops.
	Shortcut string
	// Leading goes before its text, after its check and icon, and Trailing
	// at its end, where a button's presses stay its own.
	Leading         core.Node
	Trailing        core.Node
	Variant         MenuItemVariant
	Checked         func() bool
	Disabled        func() bool
	OnSelect        func()
	NoCloseOnSelect bool
	TestID          string
}

// Render implements core.Node.
func (i MenuItem) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	style, look := menuRow(cx)
	fg := colors.Text
	if i.Variant == MenuItemDanger {
		fg = colors.Danger
	}
	children := menuLeading(cx, i.Icon, i.Checked)
	if i.Leading != nil {
		children = append(children, i.Leading)
	}
	children = append(children, core.Text{Content: i.Text, MaxLines: 1, Style: TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}),
		StyleFunc: Colored(fg)})
	if i.Shortcut != "" && !Touch(cx) {
		children = append(children, core.Text{Content: i.Shortcut, MaxLines: 1, Style: TypeCaption, StyleFunc: Colored(colors.Muted)})
	}
	if i.Trailing != nil {
		children = append(children, i.Trailing)
	}
	return base.MenuItem{OnSelect: i.OnSelect, Disabled: i.Disabled, NoCloseOnSelect: i.NoCloseOnSelect, Style: style, StyleFunc: look,
		Children: children, Label: i.Text, TestID: i.TestID}
}

// MenuSeparator divides groups of items.
type MenuSeparator struct{}

// Render implements core.Node.
func (MenuSeparator) Render(cx *core.Scope) core.Node {
	return base.MenuSeparator{Style: core.Style{Height: core.Pt(1), Margin: core.Symmetric(4, 0)}, StyleFunc: Filled(ColorsOf(cx).Line)}
}

// sheetRowHeight is the height of a row across a sheet.
const sheetRowHeight core.Points = 44

// menuRow is an item's row and its look: compact in a popup, a sheet's
// row across a sheet.
func menuRow(cx *core.Scope) (core.Style, func() core.Style) {
	colors := ColorsOf(cx)
	s := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Disabled: Dimmed()}
	if !Touch(cx) {
		s.MinHeight, s.Padding, s.CornerRadius = core.Pt(ButtonHeight), core.Symmetric(0, 8), RadiusControl-2
		return s, func() core.Style {
			return core.Style{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}
		}
	}
	s.MinHeight, s.Padding, s.Gap = core.Pt(sheetRowHeight), core.Symmetric(0, 20), 12
	return s, func() core.Style { return core.Style{Active: &core.Style{Background: colors.Pressed()}} }
}

// menuLeading is what goes before an item's text: its check, its icon,
// or nothing.
func menuLeading(cx *core.Scope, icon func() *core.SVG, checked func() bool) []core.Node {
	colors := ColorsOf(cx)
	var out []core.Node
	if checked != nil {
		out = append(out, core.View{Style: core.Style{FlexShrink: 0}, StyleFunc: func() core.Style {
			if !checked() {
				return core.Style{Opacity: core.Alpha(0)}
			}
			return core.Style{}
		}, Children: []core.Node{Icon(lucide.Check, iconSize, colors.Text)}})
	}
	if icon != nil {
		out = append(out, Icon(icon, iconSize, colors.Muted))
	}
	return out
}

// MenuLabel heads a group of a menu's items.
type MenuLabel struct{ Text string }

// Render implements core.Node.
func (l MenuLabel) Render(cx *core.Scope) core.Node {
	t := Text(l.Text, 11.5, Medium, ColorsOf(cx).Muted)
	t.Style.Padding = core.Edges{Top: 6, Right: 8, Bottom: 3, Left: 8}
	return t
}
