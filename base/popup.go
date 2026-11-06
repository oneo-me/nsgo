package base

import (
	"time"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// Popups (Popover, Tooltip, Menu, ContextMenu, Dialog, Sheet) share one
// contract, NSH's: Open makes them controlled and DefaultOpen starts them
// open otherwise; OnOpenChange reports each request that would change
// what is shown, with its reason, and a controlled popup shows what Open
// returns. OnOpen and OnClose follow it being shown and gone. OnCancel
// hears every attempt to close it (Escape or back, a press outside or on
// the backdrop, a sheet's swipe), and NoCloseOn... flags keep it open on that attempt.

// Reason is why a popup asks to open or close.
type Reason uint8

const (
	ReasonTriggerPress Reason = iota + 1
	ReasonPressOutside
	ReasonBackdropPress
	ReasonCancel
	ReasonSelect
	ReasonHover
	ReasonLongPress
	ReasonContextMenu
	// ReasonSwipe is a sheet swiped down: base.Sheet's, or an iOS
	// dialog's.
	ReasonSwipe
)

func (r Reason) String() string {
	return [...]string{"", "triggerPress", "pressOutside", "backdropPress", "cancel", "select",
		"hover", "longPress", "contextMenu", "swipe"}[r]
}

// popup is the open state of one popup.
type popup struct {
	open     func() bool
	own      *core.Signal[bool]
	onChange func(bool, Reason)
	onOpen   func()
	onClose  func()
	onCancel func()
	// userClosed, for a controlled dialog, is set while the user closed
	// it and the app has not set Open false since.
	userClosed *core.Signal[bool]
}

func newPopup(cx *core.Scope, open func() bool, def bool, onChange func(bool, Reason), onOpen, onClose func()) *popup {
	p := &popup{open: open, onChange: onChange, onOpen: onOpen, onClose: onClose}
	if open == nil {
		p.own = core.NewSignal(cx, def)
		p.open = p.own.Get
	}
	return p
}

// lifecycle reports OnOpen and OnClose as there, tracked, turns: once a
// popup is all there and once it is all gone, a sheet after its motion.
func (p *popup) lifecycle(cx *core.Scope, there func() bool) {
	if p.onOpen == nil && p.onClose == nil {
		return
	}
	var was bool
	cx.Effect(func() {
		is := there()
		switch {
		case is && !was && p.onOpen != nil:
			cx.Untrack(p.onOpen)
		case !is && was && p.onClose != nil:
			cx.Untrack(p.onClose)
		}
		was = is
	})
}

// set asks to open or close the popup for reason; a request matching
// what is shown reports nothing.
func (p *popup) set(open bool, r Reason) {
	if p.open() == open {
		return
	}
	if p.own != nil {
		p.own.Set(open)
	}
	if p.userClosed != nil && !open {
		p.userClosed.Set(true)
	}
	if p.onChange != nil {
		p.onChange(open, r)
	}
}

// cancel reports an attempt to close the popup, which closes it unless
// keep holds.
func (p *popup) cancel(r Reason, keep bool) {
	if p.onCancel != nil {
		p.onCancel()
	}
	if !keep {
		p.set(false, r)
	}
}

// popupMotion keeps an overlay popup (Popover, Tooltip, a menu's on
// desktops) in the tree while it comes in and goes, as Base UI's data-starting-style and
// data-ending-style do and as a Toast's phases: the popup takes its
// starting part in its first frame, so a transition in its style runs
// its entry from there, and its ending part as it closes, leaving the
// tree once that part's transition is over. Without an ending part, or
// while the user asks for less motion, it comes and goes at once.
type popupMotion struct {
	starting, ending part
	// shown: in the tree, while open and while it goes.
	shown *core.Signal[bool]
	// entered: past its first frame.
	entered *core.Signal[bool]
	// closing: it goes, and leaves once its transition ends.
	closing                    *core.Signal[bool]
	startingClass, endingClass string
}

// newPopupMotion returns the motion of a popup following open.
func newPopupMotion(cx *core.Scope, open func() bool, starting, ending part) *popupMotion {
	m := &popupMotion{starting: starting, ending: ending,
		shown: core.NewSignal(cx, false), entered: core.NewSignal(cx, false), closing: core.NewSignal(cx, false),
		startingClass: parts.Important(starting.Class), endingClass: parts.Important(ending.Class)}
	cx.Effect(func() {
		o := open()
		cx.Untrack(func() { m.follow(cx, o) })
	})
	return m
}

func (m *popupMotion) follow(cx *core.Scope, open bool) {
	still := core.Untracked(cx, cx.Env().ReduceMotion)
	if open {
		// Opened again on its way out: back from where it is.
		m.closing.Set(false)
		if m.shown.GetUntracked() {
			return
		}
		m.entered.Set(still || !m.starting.Set())
		m.shown.Set(true)
		if !m.entered.GetUntracked() {
			// Past its first frame: the next display frame, or a frame's
			// time where none comes, whichever is first.
			enter := func() { m.entered.Set(true) }
			cx.RequestAnimationFrame(func(time.Time) { enter() })
			cx.AfterFunc(parts.FirstFrame, enter)
		}
		return
	}
	if !m.shown.GetUntracked() || m.closing.GetUntracked() {
		return
	}
	if still || !m.ending.Set() {
		m.shown.Set(false)
		return
	}
	m.closing.Set(true)
}

// transitionEnd is the popup's OnTransitionEnd: a closing popup leaves.
func (m *popupMotion) transitionEnd() {
	if m.closing.GetUntracked() {
		m.closing.Set(false)
		m.shown.Set(false)
	}
}

// class is the phase's class, after and over the popup's own.
func (m *popupMotion) class() string { return m.classed(m.startingClass, m.endingClass) }

// classed is starting or ending, by phase.
func (m *popupMotion) classed(starting, ending string) string {
	switch {
	case m.closing.Get():
		return ending
	case !m.entered.Get():
		return starting
	}
	return ""
}

// style merges the phase's style over fn's.
func (m *popupMotion) style(fn func() core.Style) func() core.Style {
	return m.styled(fn, m.starting.Style, m.ending.Style)
}

// styled merges starting or ending, by phase, over fn's style.
func (m *popupMotion) styled(fn func() core.Style, starting, ending core.Style) func() core.Style {
	return func() core.Style {
		var s core.Style
		if fn != nil {
			s = fn()
		}
		switch {
		case m.closing.Get():
			s = s.Merge(ending)
		case !m.entered.Get():
			s = s.Merge(starting)
		}
		return s
	}
}

// overlay gives o the motion's phases and the popup's part.
func (m *popupMotion) overlay(o core.Overlay, popup part) core.Overlay {
	o.Class, o.ClassFunc, o.Style, o.StyleFunc = popup.Class, m.class, popup.Style, m.style(popup.Fn)
	o.OnTransitionEnd = m.transitionEnd
	return o
}

// part is the class and styles the app gives one of a component's parts.
type part = parts.Part

// modalLayer shows m, holding the backdrop, the popup and what d lays out
// around it, while the motion keeps the popup in the tree.
func modalLayer(st *popup, motion *popupMotion, m core.Modal, d Dialog) core.Node {
	backdrop, popup := parts.Of(d.BackdropClass, d.BackdropStyle, d.BackdropStyleFunc), parts.Of(d.PopupClass, d.PopupStyle, d.PopupStyleFunc)
	return core.Show{When: motion.shown.Get, Then: func(cx *core.Scope) core.Node {
		var nodes []core.Node
		if backdrop.Set() {
			startingClass, endingClass := parts.Important(d.BackdropStartingClass), parts.Important(d.BackdropEndingClass)
			nodes = append(nodes, core.View{Class: backdrop.Class, Style: parts.Fill.Merge(backdrop.Style),
				ClassFunc: func() string { return motion.classed(startingClass, endingClass) },
				StyleFunc: motion.styled(backdrop.Fn, d.BackdropStartingStyle, d.BackdropEndingStyle),
				OnPress:   func(core.PressEvent) { st.cancel(ReasonBackdropPress, d.NoCloseOnBackdropPress) }, NoFocus: true,
				TestID: d.BackdropTestID})
		}
		nodes = append(nodes, d.Before...)
		// Its blank space takes presses, which would reach the backdrop.
		nodes = append(nodes, core.View{Class: popup.Class, ClassFunc: motion.class, Style: popup.Style, StyleFunc: motion.style(popup.Fn), Role: core.RoleDialog,
			OnPress: func(core.PressEvent) {}, NoFocus: true,
			OnTransitionEnd: motion.transitionEnd, Label: d.Label, TestID: d.TestID, Children: d.Children})
		nodes = append(nodes, d.After...)
		// What is going takes no more presses.
		nodes = append(nodes, core.Show{When: motion.closing.Get, Then: func(*core.Scope) core.Node {
			return core.View{Style: parts.Fill, OnPress: func(core.PressEvent) {}, NoFocus: true}
		}})
		m.Children = nodes
		m.OnRequestClose = func(reason string) {
			if reason == "swipe" {
				// An iOS sheet's swipe; NoCloseOnSwipe kept it on screen.
				st.cancel(ReasonSwipe, m.NoCloseOnSwipe)
				return
			}
			st.cancel(ReasonCancel, d.NoCloseOnEscape)
		}
		return m
	}}
}
