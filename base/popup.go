package base

import (
	"time"

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
		startingClass: important(starting.class), endingClass: important(ending.class)}
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
		m.entered.Set(still || !m.starting.set())
		m.shown.Set(true)
		if !m.entered.GetUntracked() {
			// Past its first frame: the next display frame, or a frame's
			// time where none comes, whichever is first.
			enter := func() { m.entered.Set(true) }
			cx.RequestAnimationFrame(func(time.Time) { enter() })
			cx.AfterFunc(toastFirstFrame, enter)
		}
		return
	}
	if !m.shown.GetUntracked() || m.closing.GetUntracked() {
		return
	}
	if still || !m.ending.set() {
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
func (m *popupMotion) class() string {
	switch {
	case m.closing.Get():
		return m.endingClass
	case !m.entered.Get():
		return m.startingClass
	}
	return ""
}

// style merges the phase's style over fn's.
func (m *popupMotion) style(fn func() core.Style) func() core.Style {
	return func() core.Style {
		var s core.Style
		if fn != nil {
			s = fn()
		}
		switch {
		case m.closing.Get():
			s = s.Merge(m.ending.style)
		case !m.entered.Get():
			s = s.Merge(m.starting.style)
		}
		return s
	}
}

// overlay gives o the motion's phases and the popup's part.
func (m *popupMotion) overlay(o core.Overlay, popup part) core.Overlay {
	o.Class, o.ClassFunc, o.Style, o.StyleFunc = popup.class, m.class, popup.style, m.style(popup.fn)
	o.OnTransitionEnd = m.transitionEnd
	return o
}

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
	m := newPopupMotion(cx, st.open, part{class: p.StartingClass, style: p.StartingStyle},
		part{class: p.EndingClass, style: p.EndingStyle})
	st.lifecycle(cx, m.shown.Get)
	anchor := new(core.Ref)
	outside := func() { st.cancel(ReasonPressOutside, p.NoCloseOnPressOutside) }
	return core.Group{
		core.View{Ref: anchor, Class: p.Class, Style: p.Style, StyleFunc: p.StyleFunc, Label: p.Label, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip, Role: core.RoleButton,
			Expanded: st.open,
			OnPress:  func(core.PressEvent) { st.set(!st.open(), ReasonTriggerPress) },
			Children: []core.Node{p.Trigger}},
		core.Show{When: m.shown.Get, Then: func(cx *core.Scope) core.Node {
			popup := m.overlay(core.Overlay{
				Anchor: anchor, Placement: p.Placement, Align: p.Align, Offset: p.Offset, Margin: p.Margin,
				Children: p.Children, Role: core.RoleDialog,
				OnCancel: func() { st.cancel(ReasonCancel, p.NoCloseOnEscape) },
			}, part{p.PopupClass, p.PopupStyle, p.PopupStyleFunc})
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
	return core.Overlay{Style: cover, Children: []core.Node{
		core.View{Style: fill, NoFocus: true, OnPress: func(core.PressEvent) { press() }}}}
}

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
			OnHoverIn: func() { hover(true) }, OnHoverOut: func() { hover(false) }, Children: []core.Node{t.Trigger}},
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
	m := newPopupMotion(cx, st.open, part{class: t.StartingClass, style: t.StartingStyle},
		part{class: t.EndingClass, style: t.EndingStyle})
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
	class, style, fn := joinClasses(t.Class, t.PopupClass), t.Style.Merge(t.PopupStyle), t.PopupStyleFunc
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
			Children: t.Children, Role: core.RoleAlert, PointerEvents: core.PointerEventsNone}, part{class, style, fn})
	}}, hover
}

// Dialog is a modal layer over the app: an optional backdrop and a
// popup centered on it.
type Dialog struct {
	Open         func() bool
	DefaultOpen  bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	OnCancel     func()

	Animation              core.ModalAnimation
	NoCloseOnEscape        bool
	NoCloseOnBackdropPress bool
	// Presentation and NoCloseOnSwipe are iOS's, IOS and Android hold the
	// props only one of them has, and StatusBar is set over the app's
	// while the dialog is open, as core.Modal's are.
	Presentation   core.ModalPresentation
	NoCloseOnSwipe bool
	IOS            core.IOSModalProps
	Android        core.AndroidModalProps
	StatusBar      core.StatusBarSetting
	StatusBarFunc  func() core.StatusBarSetting
	// BackdropClass, BackdropStyle or BackdropStyleFunc, when set, draws a
	// backdrop whose press asks to close; BackdropTestID names it.
	BackdropClass     string
	BackdropStyle     core.Style
	BackdropStyleFunc func() core.Style
	BackdropTestID    string
	PopupClass        string
	PopupStyle        core.Style
	PopupStyleFunc    func() core.Style
	// LayerClass and LayerStyle style the layer that holds the backdrop
	// and the popup, over its default, which centers the popup.
	LayerClass string
	LayerStyle core.Style
	Children   []core.Node
	Label      string
	TestID     string
}

// Render implements core.Node.
func (d Dialog) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, d.Open, d.DefaultOpen, d.OnOpenChange, d.OnOpen, d.OnClose)
	st.onCancel = d.OnCancel
	if d.Open != nil {
		// Closed by the user, it stays closed while Open stays true, until
		// the app sets it false and true again.
		st.userClosed = core.NewSignal(cx, false)
		cx.Effect(func() {
			if !d.Open() {
				st.userClosed.Set(false)
			}
		})
		st.open = func() bool { return d.Open() && !st.userClosed.Get() }
	}
	// The platform presents the modal: OnOpen and OnClose follow once its
	// animation would have ended, or as soon as a platform reports it
	// shown or gone.
	var stop func() bool
	report := func(open bool) func() {
		return func() {
			if stop != nil {
				stop()
				stop = nil
			}
			if f := map[bool]func(){true: d.OnOpen, false: d.OnClose}[open]; f != nil {
				cx.Untrack(f)
			}
		}
	}
	was := false
	cx.Effect(func() {
		open := st.open()
		if open == was {
			return
		}
		was = open
		if stop != nil {
			stop()
			stop = nil
		}
		if wait := d.Animation.Duration(); wait > 0 {
			stop = cx.AfterFunc(wait, report(open))
			return
		}
		report(open)()
	})
	m := core.Modal{Animation: d.Animation, Presentation: d.Presentation, NoCloseOnSwipe: d.NoCloseOnSwipe,
		NoCloseOnEscape: d.NoCloseOnEscape, NoCloseOnBackdropPress: d.NoCloseOnBackdropPress,
		IOS: d.IOS, Android: d.Android, StatusBar: d.StatusBar, StatusBarFunc: d.StatusBarFunc, Class: d.LayerClass,
		OnShown: report(true), OnDismissed: report(false),
		Style: core.Style{JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter}.Merge(d.LayerStyle)}
	return modalLayer(st, m, part{d.BackdropClass, d.BackdropStyle, d.BackdropStyleFunc}, d.BackdropTestID, d.NoCloseOnBackdropPress,
		d.NoCloseOnEscape, part{d.PopupClass, d.PopupStyle, d.PopupStyleFunc}, d.Children, d.Label, d.TestID)
}

// Sheet is a panel at an edge of the app (Side) over a backdrop that
// covers the rest and takes its presses, on top of everything in its root
// view, the same on every device. It slides in from its edge as the
// backdrop fades in, and back out as it closes, at once when the user
// asked for less motion; OnOpen and OnClose follow the motion's end. A
// press on the backdrop, Escape and back close it, and with
// CloseOnSwipe a bottom sheet follows a downward drag and closes on a
// release past 30% of its height or 800 points a second.
type Sheet struct {
	Open         func() bool
	DefaultOpen  bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	OnCancel     func()

	// Side is the edge the panel sits on; the bottom by default.
	Side                   core.Placement
	NoCloseOnEscape        bool
	NoCloseOnBackdropPress bool
	CloseOnSwipe           bool
	// BackdropStyle styles the backdrop, which fades with the panel: give
	// it a translucent background.
	BackdropClass     string
	BackdropStyle     core.Style
	BackdropStyleFunc func() core.Style
	PopupClass        string
	PopupStyle        core.Style
	PopupStyleFunc    func() core.Style
	// Class and Style style the layer that holds the backdrop and the
	// panel, which covers the root view.
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	// Children are the content, made on each opening.
	Children []core.Node
	Label    string
	TestID   string
}

// Render implements core.Node.
func (s Sheet) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, s.Open, s.DefaultOpen, s.OnOpenChange, s.OnOpen, s.OnClose)
	st.onCancel = s.OnCancel
	side := s.Side
	if side == core.PlaceDefault {
		side = core.PlaceBottom
	}
	m := newSheetMotion(cx, st.open, side)
	st.lifecycle(cx, m.there)
	return sheetLayer{m: m, side: side, backdrop: part{s.BackdropClass, s.BackdropStyle, s.BackdropStyleFunc},
		backdropPress: func() { st.cancel(ReasonBackdropPress, s.NoCloseOnBackdropPress) },
		panel:         part{s.PopupClass, s.PopupStyle, s.PopupStyleFunc},
		swipe:         s.CloseOnSwipe && side == core.PlaceBottom,
		swiped:        func() { st.cancel(ReasonSwipe, false) },
		cancel:        func() { st.cancel(ReasonCancel, s.NoCloseOnEscape) },
		role:          core.RoleDialog, label: s.Label, testID: s.TestID, content: s.Children,
		layer: part{s.Class, s.Style, s.StyleFunc}}.node()
}

// part is the class and styles the app gives one of a component's parts.
type part struct {
	class string
	style core.Style
	fn    func() core.Style
}

// set reports whether the app styles the part.
func (p part) set() bool { return p.class != "" || !p.style.IsZero() || p.fn != nil }

// joinClasses returns a's classes then b's, which win over a's.
func joinClasses(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}

var fill = core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)}

// cover is an overlay over its whole root view; overlays are as large as
// their content otherwise.
var cover = core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Width: core.Pct(100), Height: core.Pct(100)}

// modalLayer shows m, holding the backdrop and the popup, while st is
// open.
func modalLayer(st *popup, m core.Modal, backdrop part, backdropID string, keepOnBackdrop, keepOnEscape bool, popup part,
	children []core.Node, label, testID string) core.Node {
	return core.Show{When: st.open, Then: func(cx *core.Scope) core.Node {
		var nodes []core.Node
		if backdrop.set() {
			nodes = append(nodes, core.View{Class: backdrop.class, Style: fill.Merge(backdrop.style), StyleFunc: backdrop.fn,
				OnPress: func(core.PressEvent) { st.cancel(ReasonBackdropPress, keepOnBackdrop) }, NoFocus: true,
				TestID: backdropID})
		}
		nodes = append(nodes, core.View{Class: popup.class, Style: popup.style, StyleFunc: popup.fn, Role: core.RoleDialog,
			Label: label, TestID: testID, Children: children})
		m.Children = nodes
		m.OnRequestClose = func(reason string) {
			if reason == "swipe" {
				// An iOS sheet's swipe; NoCloseOnSwipe kept it on screen.
				st.cancel(ReasonSwipe, m.NoCloseOnSwipe)
				return
			}
			st.cancel(ReasonCancel, keepOnEscape)
		}
		return m
	}}
}
