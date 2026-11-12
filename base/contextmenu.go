package base

import (
	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// ContextMenu opens a menu of MenuItems about its trigger: a secondary
// click opens it at the pointer on desktops, and a finger resting 400 ms
// on the trigger opens it as a sheet on touch screens, while the
// trigger's own presses and the scroll views around it go on.
type ContextMenu struct {
	Open         func() bool
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	OnCancel     func()
	// OnPosition reports where the popup went on desktops, as Popover's.
	OnPosition func(core.OverlayPosition)
	Disabled   bool

	// Offset and Margin place the popup beside the pointer on desktops.
	Offset, Margin core.Points
	Class          string // the content's
	Style          core.Style
	StyleFunc      func() core.Style
	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	// StartingStyle and EndingStyle are Menu's.
	StartingClass     string
	StartingStyle     core.Style
	EndingClass       string
	EndingStyle       core.Style
	BackdropClass     string
	BackdropStyle     core.Style
	BackdropStyleFunc func() core.Style
	Trigger           core.Node
	Children          []core.Node
	TestID            string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (m ContextMenu) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, m.Open, false, m.OnOpenChange, m.OnOpen, m.OnClose)
	st.onCancel = m.OnCancel
	point := core.NewSignal(cx, core.Point{})
	open := func(at core.Point, r Reason) {
		if m.Disabled {
			return
		}
		point.Set(at)
		st.set(true, r)
	}
	var stopHold func() bool
	var holdAt core.Point
	endHold := func() {
		if stopHold != nil {
			stopHold()
			stopHold = nil
		}
	}
	return core.Group{
		core.View{Class: m.Class, Style: m.Style, StyleFunc: m.StyleFunc, TestID: m.TestID, OnLayout: m.OnLayout, Tooltip: m.Tooltip,
			OnContextMenu: func(e core.ContextMenuEvent) { open(e.PagePosition, ReasonContextMenu) },
			// Observed, never claimed: the trigger's presses go on.
			OnTouch: func(e core.TouchEvent) {
				if cx.Env().Desktop() || len(e.Touches) == 0 {
					return
				}
				at := e.Touches[0].PagePosition
				switch e.Phase {
				case core.TouchStart:
					endHold()
					if len(e.Touches) != 1 {
						return
					}
					holdAt = at
					stopHold = cx.AfterFunc(menuHold, func() {
						stopHold = nil
						open(holdAt, ReasonLongPress)
					})
				case core.TouchMove:
					if dx, dy := at.X-holdAt.X, at.Y-holdAt.Y; dx*dx+dy*dy > menuSlopSquared {
						endHold()
					}
				default:
					endHold()
				}
			},
			Children: []core.Node{m.Trigger}},
		menuSurface(cx, st, nil, point, menuParts{placement: core.PlaceBottom, align: core.AlignStart, onPosition: m.OnPosition,
			offset: m.Offset, margin: m.Margin, popup: parts.Of(m.PopupClass, m.PopupStyle, m.PopupStyleFunc),
			backdrop: parts.Of(m.BackdropClass, m.BackdropStyle, m.BackdropStyleFunc),
			starting: parts.Part{Class: m.StartingClass, Style: m.StartingStyle},
			ending:   parts.Part{Class: m.EndingClass, Style: m.EndingStyle}}, m.Children),
	}
}
