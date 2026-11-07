package base

import (
	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// Popover shows content beside its trigger: pressing the trigger opens
// it, pressing outside or Escape closes it. On desktops a press outside
// goes on to what is under the pointer; on touch screens a transparent
// backdrop takes it, so it only closes the popover.
type Popover struct {
	Open         func() bool
	DefaultOpen  bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	// OnCancel hears every attempt to close it, allowed or not.
	OnCancel func()
	// OnPosition reports where the popup went: the other side of Placement
	// when only that one had room, its frame and the room on that side.
	OnPosition func(core.OverlayPosition)

	Placement             core.Placement
	Align                 core.Align
	Offset, Margin        core.Points
	NoCloseOnEscape       bool
	NoCloseOnPressOutside bool

	Class          string // the trigger's
	Style          core.Style
	StyleFunc      func() core.Style
	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	// StartingStyle styles the popup in its first frame and EndingStyle
	// as it goes (popupMotion); they merge over PopupStyle, and their
	// classes are important.
	StartingClass string
	StartingStyle core.Style
	EndingClass   string
	EndingStyle   core.Style
	Trigger       core.Node
	Children      []core.Node
	Label         string
	TestID        string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (p Popover) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, p.Open, p.DefaultOpen, p.OnOpenChange, p.OnOpen, p.OnClose)
	st.onCancel = p.OnCancel
	m := newPopupMotion(cx, st.open, parts.Part{Class: p.StartingClass, Style: p.StartingStyle},
		parts.Part{Class: p.EndingClass, Style: p.EndingStyle})
	st.lifecycle(cx, m.shown.Get)
	anchor := new(core.Ref)
	outside := func() { st.cancel(ReasonPressOutside, p.NoCloseOnPressOutside) }
	around := popupBar(cx)
	return core.Group{
		barTrigger(cx, around, core.View{Ref: anchor, Class: p.Class, Style: p.Style, StyleFunc: p.StyleFunc, Label: p.Label, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip, Role: core.RoleButton,
			Expanded: st.open, ShowsExpanded: true,
			OnPress:  func(core.PressEvent) { st.set(!st.open(), ReasonTriggerPress) },
			Children: []core.Node{p.Trigger}}),
		core.Show{When: m.shown.Get, Then: func(cx *core.Scope) core.Node {
			popup := m.overlay(core.Overlay{
				Anchor: anchor, AnchorContent: true, Placement: p.Placement, Align: p.Align, Offset: p.Offset, Margin: p.Margin,
				Children: p.Children, Role: core.RoleDialog, OnPosition: p.OnPosition,
				OnCancel: func() { st.cancel(ReasonCancel, p.NoCloseOnEscape) },
			}, parts.Of(p.PopupClass, p.PopupStyle, p.PopupStyleFunc))
			if core.Untracked(cx, cx.Env().Desktop) {
				popup.OnPressOutside = func(core.Point) { outside() }
				return popup
			}
			// The backdrop leaves as the popover closes, so a press goes
			// on while it goes.
			return core.Group{core.Show{When: st.open, Then: func(*core.Scope) core.Node { return touchBackdrop(outside) }}, popup}
		}},
	}
}

// touchBackdrop is a transparent layer over the root view that takes a
// press outside a popup on a touch screen, so it goes no further.
func touchBackdrop(press func()) core.Node {
	return core.Overlay{Style: parts.Cover, Children: []core.Node{
		core.View{Style: parts.Fill, NoFocus: true, OnPress: func(core.PressEvent) { press() }}}}
}
