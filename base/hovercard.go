package base

import (
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// How long a HoverCard waits by default.
const (
	hoverOpenDelay  = 600 * time.Millisecond
	hoverCloseDelay = 300 * time.Millisecond
)

// HoverNever is a HoverCard's delay that never ends: it opens or closes
// only as the app or a press says.
const HoverNever time.Duration = math.MaxInt64

// HoverCard previews what its trigger points at. On desktops it opens once
// the pointer rested on the trigger for OpenDelay or the keyboard focus
// came to it, stays while the pointer or the focus is on the trigger or
// the card, and closes CloseDelay after both left; Escape and a press
// outside close it at once. On touch screens a press on the trigger opens
// and closes it and a press outside closes it. The card takes presses and
// looks like nothing.
type HoverCard struct {
	Open         func() bool
	DefaultOpen  bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	// OnPosition reports where the popup went, as Popover's.
	OnPosition func(core.OverlayPosition)
	// OpenDelay is how long the pointer rests or the focus stays before
	// the card opens, 600 ms when 0; CloseDelay how long both are away
	// before it closes, 300 ms when 0. HoverNever waits for good.
	OpenDelay, CloseDelay time.Duration

	Placement      core.Placement
	Align          core.Align
	Offset, Margin core.Points

	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	// StartingStyle and EndingStyle are the popup's, as Popover's.
	StartingClass string
	StartingStyle core.Style
	EndingClass   string
	EndingStyle   core.Style
	// Trigger keeps its own role, presses and look.
	Trigger  core.Node
	Children []core.Node
	TestID   string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (h HoverCard) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, h.Open, h.DefaultOpen, h.OnOpenChange, h.OnOpen, h.OnClose)
	m := newPopupMotion(cx, st.open, parts.Part{Class: h.StartingClass, Style: h.StartingStyle},
		parts.Part{Class: h.EndingClass, Style: h.EndingStyle})
	st.lifecycle(cx, m.shown.Get)
	// Where the pointer and the focus are, read only by the timer.
	var onTrigger, onCard, focused bool
	holds := func() bool { return onTrigger || onCard || focused }
	// The one pending open or close: a new one replaces it.
	var pending func() bool
	stop := func() {
		if pending != nil {
			pending()
			pending = nil
		}
	}
	after := func(open bool, r Reason) {
		stop()
		if core.Untracked(cx, st.open) == open {
			return
		}
		delay := h.CloseDelay
		if open {
			delay = h.OpenDelay
		}
		if delay == 0 {
			delay = hoverCloseDelay
			if open {
				delay = hoverOpenDelay
			}
		}
		if delay == HoverNever {
			return
		}
		fire := func() {
			pending = nil
			if open || !holds() {
				st.set(open, r)
			}
		}
		if delay < 0 {
			fire()
			return
		}
		pending = cx.AfterFunc(delay, fire)
	}
	atOnce := func(open bool, r Reason) {
		stop()
		st.set(open, r)
	}
	cx.Cleanup(stop)
	leave := func(r Reason) {
		if !holds() {
			after(false, r)
		}
	}
	anchor := new(core.Ref)
	desktop := core.Untracked(cx, cx.Env().Desktop)
	leaveBar(cx)
	trigger := core.View{Ref: anchor, Children: []core.Node{h.Trigger}, NoFocus: true}
	root := core.View{TestID: h.TestID, OnLayout: h.OnLayout, Tooltip: h.Tooltip}
	if desktop {
		root.OnFocusIn = func() {
			focused = true
			after(true, ReasonFocus)
		}
		root.OnFocusOut = func() {
			focused = false
			leave(ReasonFocus)
		}
		trigger.OnMouseEnter = func(core.MouseEvent) {
			onTrigger = true
			after(true, ReasonHover)
		}
		trigger.OnMouseLeave = func(core.MouseEvent) {
			onTrigger = false
			leave(ReasonHover)
		}
	} else {
		// No pointer to rest: a press opens and closes it. A disclosure
		// there, it says whether its card is open.
		trigger.Expanded = st.open
		trigger.OnPress = func(core.PressEvent) { atOnce(!core.Untracked(cx, st.open), ReasonTriggerPress) }
	}
	card := core.Show{When: m.shown.Get, Then: func(cx *core.Scope) core.Node {
		// The card leaves with the pointer on it: it hears no leave.
		cx.Cleanup(func() { onCard = false })
		o := m.overlay(core.Overlay{Anchor: anchor, AnchorContent: true, Placement: h.Placement, Align: h.Align, Offset: h.Offset, Margin: h.Margin,
			OnPosition: h.OnPosition,
			OnPressOutside: func(core.Point) {
				if core.Untracked(cx, st.open) {
					atOnce(false, ReasonPressOutside)
				}
			},
			OnCancel: func() {
				if core.Untracked(cx, st.open) {
					atOnce(false, ReasonCancel)
				}
			},
		}, parts.Of(h.PopupClass, h.PopupStyle, h.PopupStyleFunc))
		o.Children = h.Children
		if desktop {
			o.OnMouseEnter = func(core.MouseEvent) {
				onCard = true
				// Reached in time: it stays open.
				if core.Untracked(cx, st.open) {
					stop()
				}
			}
			o.OnMouseLeave = func(core.MouseEvent) {
				onCard = false
				leave(ReasonHover)
			}
		}
		return o
	}}
	root.Children = []core.Node{trigger, card}
	return root
}
