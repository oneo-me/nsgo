package base

import (
	"slices"
	"time"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/base/internal/sheet"
	"github.com/oneo-me/nsgo/core"
)

// Menus present themselves by the device, as NSH's: on desktops a popup
// at the trigger (or the pointer) whose submenus cascade beside the item
// that opens them, on hover or press; on touch screens a sheet at the
// bottom over a backdrop, whose submenus show their items in place of
// their menu's, with the app's Back row first. The items reach their
// menu through context, so the app arranges them freely.

// A context menu opens after a finger rests this long on its trigger,
// sooner than the platforms' long press so it opens while the finger is
// down, unless the finger moves this far.
const (
	menuHold        = 400 * time.Millisecond
	menuSlop        = 10
	menuSlopSquared = menuSlop * menuSlop
)

// menuContext is what a menu's items reach through menuKey.
type menuContext struct {
	// close closes the whole menu for a reason.
	close func(Reason)
	// sheet: presented as a sheet on a touch screen.
	sheet bool
	// path holds the submenus a sheet shows, innermost last.
	path *core.Signal[[]int]
	// level is the submenu the items under this context are in, 0 the
	// menu.
	level int
	// openSub is the submenu of this level open on a desktop, 0 none.
	openSub *core.Signal[int]
	// next numbers the menu's submenus.
	next *int
	// popup is the popup part, for the submenus' popups.
	popup part
}

// shown reports, tracked, whether a sheet shows this level's items.
func (m *menuContext) shown() bool {
	p := m.path.Get()
	if len(p) == 0 {
		return m.level == 0
	}
	return p[len(p)-1] == m.level
}

// hiddenUnlessShown is the style of an item of a sheet's level that is
// not shown: out of the tree's layout and of what screen readers read.
func (m *menuContext) hiddenUnlessShown() func() core.Style {
	if m == nil || !m.sheet {
		return nil
	}
	return func() core.Style {
		if m.shown() {
			return core.Style{}
		}
		return core.Style{Display: core.DisplayNone}
	}
}

var menuKey = core.NewKey[*menuContext]("menu", nil)

// menuParts are the placement and the parts both kinds of menus take.
type menuParts struct {
	placement       core.Placement
	align           core.Align
	offset, margin  core.Points
	popup, backdrop part
	// starting and ending are the desktop popup's phases (popupMotion).
	starting, ending part
	label, testID    string
}

// menuSurface is a menu while it is open: its popup at anchor, or at
// point when anchor is nil, or its sheet, with the context its items
// read.
func menuSurface(cx *core.Scope, st *popup, anchor *core.Ref, point *core.Signal[core.Point], p menuParts,
	items []core.Node) core.Node {
	onSheet := !core.Untracked(cx, cx.Env().Desktop)
	path := core.NewSignalFunc(cx, []int(nil), slices.Equal)
	openSub := core.NewSignal(cx, 0)
	next := 0
	ctx := &menuContext{close: func(r Reason) { st.set(false, r) }, sheet: onSheet, path: path, openSub: openSub,
		next: &next, popup: p.popup}
	dismiss := func(r Reason) { st.cancel(r, false) }
	// Escape and back leave the innermost submenu of a sheet first.
	cancel := func() {
		if open := path.GetUntracked(); len(open) > 0 {
			path.Set(open[:len(open)-1])
			return
		}
		dismiss(ReasonCancel)
	}
	if onSheet {
		m := sheet.NewMotion(cx, st.open, core.PlaceBottom)
		st.lifecycle(cx, m.There)
		cx.Effect(func() {
			// Each opening starts at the top.
			if m.Shown() {
				cx.Untrack(func() { path.Set(nil) })
			}
		})
		return core.Func(func(cx *core.Scope) core.Node {
			menuKey.Provide(cx, ctx)
			return sheet.Layer{Motion: m, Side: core.PlaceBottom, Backdrop: p.backdrop,
				BackdropPress: func() { dismiss(ReasonBackdropPress) },
				Panel:         p.popup, Cancel: cancel,
				Role: core.RoleMenu, Label: p.label, TestID: p.testID, Content: items}.Node()
		})
	}
	m := newPopupMotion(cx, st.open, p.starting, p.ending)
	st.lifecycle(cx, m.shown.Get)
	return core.Show{When: m.shown.Get, Then: func(cx *core.Scope) core.Node {
		openSub.Set(0)
		menuKey.Provide(cx, ctx)
		o := m.overlay(core.Overlay{
			Anchor: anchor, Placement: p.placement, Align: p.align, Offset: p.offset, Margin: p.margin,
			Children: items, Role: core.RoleMenu, Label: p.label, TestID: p.testID,
			OnPressOutside: func(core.Point) { dismiss(ReasonPressOutside) },
			OnCancel:       cancel,
		}, p.popup)
		if anchor == nil {
			o.AnchorBounds = func() core.Bounds { return core.Bounds{Origin: point.Get()} }
		}
		return o
	}}
}

// Menu opens a list of MenuItems when its trigger is pressed: beside the
// trigger on desktops, as a sheet at the bottom on touch screens.
// Choosing an item closes it, as do a press outside or on the backdrop,
// Escape and back, which OnCancel hears.
type Menu struct {
	Open         func() bool
	DefaultOpen  bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	OnCancel     func()

	// Placement, Align, Offset and Margin place the popup on desktops;
	// Align lines it up with its trigger, at its start when 0.
	Placement      core.Placement
	Align          core.Align
	Offset, Margin core.Points

	Class     string // the trigger's
	Style     core.Style
	StyleFunc func() core.Style
	// PopupStyle styles the popup on desktops and the sheet on touch
	// screens, and its submenus' popups.
	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	// StartingStyle and EndingStyle are the popup's on desktops, as
	// Popover's; a sheet has its own motion.
	StartingClass string
	StartingStyle core.Style
	EndingClass   string
	EndingStyle   core.Style
	// BackdropStyle styles the backdrop behind the sheet on touch screens.
	BackdropClass     string
	BackdropStyle     core.Style
	BackdropStyleFunc func() core.Style
	Trigger           core.Node
	// Children are the items, made on each opening.
	Children []core.Node
	Label    string
	TestID   string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (m Menu) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, m.Open, m.DefaultOpen, m.OnOpenChange, m.OnOpen, m.OnClose)
	st.onCancel = m.OnCancel
	anchor := new(core.Ref)
	align := m.Align
	if align == 0 {
		align = core.AlignStart
	}
	return core.Group{
		core.View{Ref: anchor, Class: m.Class, Style: m.Style, StyleFunc: m.StyleFunc, Label: m.Label, TestID: m.TestID, OnLayout: m.OnLayout, Tooltip: m.Tooltip, Role: core.RoleButton,
			Expanded: st.open,
			OnPress:  func(core.PressEvent) { st.set(!st.open(), ReasonTriggerPress) },
			Children: []core.Node{m.Trigger}},
		menuSurface(cx, st, anchor, nil, menuParts{placement: m.Placement, align: align, offset: m.Offset, margin: m.Margin,
			popup:    parts.Of(m.PopupClass, m.PopupStyle, m.PopupStyleFunc),
			backdrop: parts.Of(m.BackdropClass, m.BackdropStyle, m.BackdropStyleFunc),
			starting: parts.Part{Class: m.StartingClass, Style: m.StartingStyle},
			ending:   parts.Part{Class: m.EndingClass, Style: m.EndingStyle}}, m.Children),
	}
}

// MenuItem is one choice of a menu: pressing it reports OnSelect and
// closes the menu. Its look is the app's, with its state styles.
type MenuItem struct {
	OnSelect        func()
	Disabled        func() bool
	NoCloseOnSelect bool
	Class           string
	Style           core.Style
	StyleFunc       func() core.Style
	Children        []core.Node
	Label           string
	TestID          string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (i MenuItem) Render(cx *core.Scope) core.Node {
	menu := menuKey.Use(cx)
	children := i.Children
	if menu != nil && menu.sheet {
		// Only the items of the level shown are there, for screen readers
		// and tests too.
		children = []core.Node{core.Show{When: menu.shown, Then: func(*core.Scope) core.Node { return core.Group(i.Children) }}}
	}
	return core.View{Class: i.Class, Style: i.Style, StyleFunc: merged(i.StyleFunc, menu.hiddenUnlessShown()),
		Role: core.RoleMenuItem, Label: i.Label, TestID: i.TestID, OnLayout: i.OnLayout, Tooltip: i.Tooltip, Disabled: i.Disabled, Children: children,
		// Over another item, a submenu of its level closes.
		OnMouseEnter: func(core.MouseEvent) {
			if menu != nil && !menu.sheet {
				menu.openSub.Set(0)
			}
		},
		OnPress: func(core.PressEvent) {
			if i.OnSelect != nil {
				i.OnSelect()
			}
			if menu != nil && !i.NoCloseOnSelect {
				menu.close(ReasonSelect)
			}
		}}
}

// merged returns a style function merging b over a; either may be nil.
func merged(a, b func() core.Style) func() core.Style {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	return func() core.Style { return a().Merge(b()) }
}

// MenuSeparator separates groups of items, shown with its level's.
type MenuSeparator struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (s MenuSeparator) Render(cx *core.Scope) core.Node {
	menu := menuKey.Use(cx)
	return core.View{Class: s.Class, Style: s.Style, StyleFunc: merged(s.StyleFunc, menu.hiddenUnlessShown()), NoFocus: true,
		TestID: s.TestID, OnLayout: s.OnLayout, Tooltip: s.Tooltip}
}
