package base

import (
	"time"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// Tooltip shows a hint beside its trigger while the pointer rests on it
// (on desktops), at once while the focus the keyboard moved is on it or
// inside it (Tab, an arrow, a shortcut, the app's Focus after a key), in
// every idiom, until it leaves or Escape is pressed, or while the app
// opens it, the only other way on touch screens. A press that focuses
// the trigger leaves it to the pointer, as focus-visible: does. It never
// takes the pointer. A View's Tooltip is the shorter form, with the
// defaults.
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
	hint := t
	hint.Class, hint.Style, hint.StyleFunc, hint.TestID, hint.Trigger = "", core.Style{}, nil, "", nil
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, TestID: t.TestID,
		Tooltip: hint, Children: []core.Node{t.Trigger}}
}

// Hint implements core.HintTooltip: given as a view's Tooltip, the
// tooltip shows against that view, Trigger left out.
func (t Tooltip) Hint(cx *core.Scope, anchor *core.Ref) (core.Node, func(core.HintChange)) {
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
	hovered, focused := false, core.NewSignal(cx, false)
	// The pointer resting on the trigger asks to open, on desktops, and
	// leaving it to close; the keyboard's focus asks at once, in every
	// idiom, and its leaving to close, unless the pointer is still on it.
	change := func(c core.HintChange) {
		if stop != nil {
			stop()
			stop = nil
		}
		switch c {
		case core.HintHover:
			hovered = true
			if !t.Disabled && !focused.GetUntracked() && cx.Env().Desktop() {
				stop = cx.AfterFunc(delay, func() { stop = nil; st.set(true, ReasonHover) })
			}
		case core.HintLeave:
			hovered = false
			if !focused.GetUntracked() {
				st.set(false, ReasonHover)
			}
		case core.HintFocus:
			focused.Set(true)
			if !t.Disabled {
				st.set(true, ReasonFocus)
			}
		case core.HintBlur:
			focused.Set(false)
			if !hovered {
				st.set(false, ReasonFocus)
			}
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
	return core.Group{
		core.Show{When: m.shown.Get, Then: func(cx *core.Scope) core.Node {
			return m.overlay(core.Overlay{Anchor: anchor, Placement: place, Align: t.Align, Offset: offset, Margin: margin,
				Children: t.Children, Role: core.RoleAlert, PointerEvents: core.PointerEventsNone}, parts.Of(class, style, fn))
		}},
		// Escape hides what the keyboard's focus shows (WAI-ARIA's
		// tooltip pattern), while it shows.
		core.Show{When: func() bool { return st.open() && focused.Get() }, Then: func(cx *core.Scope) core.Node {
			cx.OnKey("escape", func() {
				focused.Set(false)
				st.set(false, ReasonCancel)
			})
			return nil
		}},
	}, change
}
