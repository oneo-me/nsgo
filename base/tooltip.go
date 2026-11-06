package base

import (
	"time"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// Tooltip shows a hint beside its trigger while the pointer rests on it
// (on desktops), or while the app opens it, the only way on touch
// screens. It never takes the pointer. A View's Tooltip is the shorter
// form, with the defaults.
type Tooltip struct {
	Open         func() bool
	DefaultOpen  bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()

	// Placement defaults to above the trigger.
	Placement core.Placement
	Align     core.Align
	// Offset and Margin are 4 when 0.
	Offset, Margin core.Points
	// Delay before it opens on hover; half a second when 0.
	Delay    time.Duration
	Disabled bool

	Class          string // the trigger's
	Style          core.Style
	StyleFunc      func() core.Style
	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	// StartingStyle and EndingStyle are the popup's, as Popover's.
	StartingClass string
	StartingStyle core.Style
	EndingClass   string
	EndingStyle   core.Style
	Trigger       core.Node
	Children      []core.Node
	TestID        string
}

// Render implements core.Node.
func (t Tooltip) Render(cx *core.Scope) core.Node {
	anchor := new(core.Ref)
	tip, hover := t.Hint(cx, anchor)
	return core.Group{
		core.View{Ref: anchor, Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, TestID: t.TestID,
			OnMouseEnter: func(core.MouseEvent) { hover(true) }, OnMouseLeave: func(core.MouseEvent) { hover(false) }, Children: []core.Node{t.Trigger}},
		tip,
	}
}

// Hint implements core.HintTooltip: given as a view's Tooltip, the
// tooltip shows against that view, Trigger left out.
func (t Tooltip) Hint(cx *core.Scope, anchor *core.Ref) (core.Node, func(on bool)) {
	st := newPopup(cx, t.Open, t.DefaultOpen && !t.Disabled, t.OnOpenChange, t.OnOpen, t.OnClose)
	if t.Disabled {
		// A disabled tooltip never shows.
		st.open = func() bool { return false }
	}
	m := newPopupMotion(cx, st.open, parts.Part{Class: t.StartingClass, Style: t.StartingStyle},
		parts.Part{Class: t.EndingClass, Style: t.EndingStyle})
	st.lifecycle(cx, m.shown.Get)
	delay := t.Delay
	if delay == 0 {
		delay = 500 * time.Millisecond
	}
	place := t.Placement
	if place == core.PlaceDefault {
		place = core.PlaceTop
	}
	offset, margin := t.Offset, t.Margin
	if offset == 0 {
		offset = 4
	}
	if margin == 0 {
		margin = 4
	}
	var stop func() bool
	// The pointer resting on the trigger asks to open, on desktops, and
	// leaving it to close.
	hover := func(on bool) {
		if stop != nil {
			stop()
			stop = nil
		}
		switch {
		case !on:
			st.set(false, ReasonHover)
		case !t.Disabled && cx.Env().Desktop():
			stop = cx.AfterFunc(delay, func() { stop = nil; st.set(true, ReasonHover) })
		}
	}
	// An element's tooltip has no trigger of its own: Class and Style
	// style the popup, under PopupClass and PopupStyle.
	class, style, fn := parts.JoinClasses(t.Class, t.PopupClass), t.Style.Merge(t.PopupStyle), t.PopupStyleFunc
	if t.StyleFunc != nil {
		fn = func() core.Style {
			s := t.StyleFunc()
			if t.PopupStyleFunc != nil {
				s = s.Merge(t.PopupStyleFunc())
			}
			return s
		}
	}
	return core.Show{When: m.shown.Get, Then: func(cx *core.Scope) core.Node {
		return m.overlay(core.Overlay{Anchor: anchor, Placement: place, Align: t.Align, Offset: offset, Margin: margin,
			Children: t.Children, Role: core.RoleAlert, PointerEvents: core.PointerEventsNone}, parts.Of(class, style, fn))
	}}, hover
}
