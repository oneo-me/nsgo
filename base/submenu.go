package base

import (
	"slices"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// Submenu is an item that opens more items: on desktops a popup beside
// it, opened as the pointer comes over it or presses it; on touch screens
// a press shows its items in the sheet in place of the ones around it,
// with Back first. Its popups take the menu's PopupStyle unless it has
// its own. Choosing one of its items closes the whole menu.
type Submenu struct {
	Disabled func() bool
	// Style styles the item that opens it.
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	// PopupClass or PopupStyle, when set, styles its popup instead of the
	// menu's.
	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	Trigger        core.Node
	// Back is the row above its items in a sheet that returns to the
	// items it is among, as Escape and back do: the app's, as every
	// label is.
	Back      core.Node
	BackClass string
	BackStyle core.Style
	Children  []core.Node
	Label     string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (s Submenu) Render(cx *core.Scope) core.Node {
	menu := menuKey.Use(cx)
	trigger := func(press func(), hover func(), expanded func() bool, ref *core.Ref, style func() core.Style) core.View {
		return core.View{Ref: ref, Class: s.Class, Style: s.Style, StyleFunc: merged(s.StyleFunc, style),
			Role: core.RoleMenuItem, Label: s.Label, TestID: s.TestID, OnLayout: s.OnLayout, Tooltip: s.Tooltip, Disabled: s.Disabled, Expanded: expanded,
			OnPress: func(core.PressEvent) {
				if press != nil {
					press()
				}
			},
			OnMouseEnter: func(core.MouseEvent) { hover() }, Children: []core.Node{s.Trigger}}
	}
	if menu == nil {
		// Outside a menu: nothing to open.
		return trigger(nil, nil, nil, nil, nil)
	}
	*menu.next++
	id := *menu.next
	inner := *menu
	inner.level, inner.openSub = id, core.NewSignal(cx, 0)
	enabled := func() bool { return s.Disabled == nil || !core.Untracked(cx, s.Disabled) }
	if menu.sheet {
		path := menu.path
		into := func() {
			if enabled() {
				path.Set(append(slices.Clone(path.GetUntracked()), id))
			}
		}
		back := func() {
			if p := path.GetUntracked(); len(p) > 0 {
				path.Set(p[:len(p)-1])
			}
		}
		onPath := func() bool { return slices.Contains(path.Get(), id) }
		innermost := func() bool { p := path.Get(); return len(p) > 0 && p[len(p)-1] == id }
		return core.Group{
			core.Show{When: menu.shown, Then: func(*core.Scope) core.Node {
				return trigger(into, nil, nil, nil, nil)
			}},
			// While it is on the path: a submenu inside it shows its own
			// items in place of these, which hide themselves.
			core.Show{When: onPath, Then: func(cx *core.Scope) core.Node {
				menuKey.Provide(cx, &inner)
				nodes := []core.Node{}
				if s.Back != nil {
					nodes = append(nodes, core.Show{When: innermost, Then: func(*core.Scope) core.Node {
						return core.View{Class: s.BackClass, Style: s.BackStyle, Role: core.RoleButton,
							OnPress: func(core.PressEvent) { back() }, Children: []core.Node{s.Back}}
					}})
				}
				return core.Group(append(nodes, s.Children...))
			}},
		}
	}
	anchor := new(core.Ref)
	open := func() {
		if enabled() {
			menu.openSub.Set(id)
		}
	}
	isOpen := func() bool { return menu.openSub.Get() == id }
	popup := menu.popup
	if own := (parts.Of(s.PopupClass, s.PopupStyle, s.PopupStyleFunc)); own.Set() {
		popup = own
	}
	return core.Group{
		trigger(open, open, isOpen, anchor, nil),
		core.Show{When: isOpen, Then: func(cx *core.Scope) core.Node {
			menuKey.Provide(cx, &inner)
			return core.Overlay{Anchor: anchor, Placement: core.PlaceRight, Align: core.AlignStart,
				Class: popup.Class, Style: popup.Style, StyleFunc: popup.Fn, Children: s.Children, Role: core.RoleMenu,
				OnCancel: func() { menu.openSub.Set(0) }}
		}},
	}
}
