package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
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
	Trigger Button
	// Face, when set, draws the trigger in place of Trigger's look and
	// content, for one that follows the app's state.
	Face      core.Node
	Placement core.Placement
	Align     core.Align
	Children  []core.Node
	// MaxHeight, when set, scrolls the items past that height.
	MaxHeight core.Points
	Label     string
	TestID    string
}

// Render implements core.Node.
func (m Menu) Render(cx *core.Scope) core.Node {
	style, focus, face := m.Trigger.parts(cx)
	if m.Face != nil {
		face = m.Face
	}
	popup, backdrop := menuSurface(cx)
	children := m.Children
	if m.MaxHeight > 0 {
		popup, children = menuScroll(cx, popup, m.MaxHeight, children)
	}
	return base.Menu{Open: m.Open, OnOpenChange: m.OnOpenChange, Placement: m.Placement, Align: m.Align, Offset: 4, Margin: 8,
		Style: style, StyleFunc: focus, Trigger: face, PopupStyleFunc: popup, BackdropStyle: backdrop,
		Children: children, Label: triggerLabel(m.Label, m.Trigger), TestID: m.TestID}
}

// menuPadding is a desktop popup's padding around its items.
const menuPadding core.Points = 4

// menuScroll scrolls children past height, with a popup's padding moved
// onto them to keep the scroll bar against its edge.
func menuScroll(cx *core.Scope, popup func() core.Style, height core.Points, children []core.Node) (func() core.Style, []core.Node) {
	var pad core.Edges
	surface := popup
	if !Touch(cx) {
		pad = core.All(menuPadding)
		surface = func() core.Style {
			s := popup()
			s.Padding = core.Edges{}
			return s
		}
	}
	return surface, []core.Node{core.Scroll{Style: core.Style{MaxHeight: core.Pt(height)},
		Children: []core.Node{core.View{Style: core.Style{Padding: pad}, Children: children}}}}
}

// menuSurface is the popup on desktops, or the sheet and its backdrop on
// Touch screens.
func menuSurface(cx *core.Scope) (popup func() core.Style, backdrop core.Style) {
	if !Touch(cx) {
		return func() core.Style {
			return FinOf(cx).Menu().Over(core.Style{Padding: core.All(menuPadding), MinWidth: core.Pt(180), CornerRadius: RadiusControl + 2,
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
	// Shortcut is the keys that choose it, such as "mod+c", shown at its
	// end on desktops as the platform writes them.
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
		children = append(children, core.Text{Content: cx.KeyLabel(i.Shortcut), MaxLines: 1,
			Style: TypeCaption.Merge(core.Style{Margin: core.Edges{Left: 16}}), StyleFunc: Colored(colors.Muted)})
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
	return base.MenuSeparator{Style: core.Style{Height: core.Pt(1), Margin: core.Symmetric(4, 8)}, StyleFunc: Filled(ColorsOf(cx).Line)}
}

// menuRowHeight and sheetRowHeight are the height of a row in a popup and
// across a sheet.
const (
	menuRowHeight  core.Points = 26
	sheetRowHeight core.Points = 44
)

// menuIconSize is an item's icon on the device: smaller in a popup, whose
// rows are compact.
func menuIconSize(cx *core.Scope) core.Points {
	if Touch(cx) {
		return iconSize
	}
	return 14
}

// menuRow is an item's row and its look: compact in a popup, a sheet's
// row across a sheet.
func menuRow(cx *core.Scope) (core.Style, func() core.Style) {
	colors := ColorsOf(cx)
	s := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Disabled: Dimmed()}
	if !Touch(cx) {
		s.MinHeight, s.Padding, s.Gap, s.CornerRadius = core.Pt(menuRowHeight), core.Symmetric(0, 8), 6, RadiusControl-2
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
	size := menuIconSize(cx)
	var out []core.Node
	if checked != nil {
		out = append(out, core.View{Style: core.Style{FlexShrink: 0}, StyleFunc: func() core.Style {
			if !checked() {
				return core.Style{Opacity: core.Alpha(0)}
			}
			return core.Style{}
		}, Children: []core.Node{Icon(lucide.Check, size, colors.Text)}})
	}
	if icon != nil {
		out = append(out, Icon(icon, size, colors.Muted))
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
