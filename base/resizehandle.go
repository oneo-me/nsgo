package base

import (
	"math"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// handleBandSpec is a handle's band: its axis, parts and behavior.
type handleBandSpec struct {
	horizontal bool
	// edge places the band hugging its container's edge from inside;
	// zero centers it on a zero-length slot between two panels.
	edge        HandleEdge
	line, thumb part
	label       string
	testID      string
	disabled    func() bool
	onPan       func(core.PanEvent)
	onStep      func(string)
	// onDoublePress, when set, hears a second press in a row.
	onDoublePress func()
	value         func() core.AccessibilityValue
	touch         bool
	// state follows the pointer's progression with the band; content is
	// the app's, inside the line.
	state   *core.Signal[ResizeHandleState]
	content core.Node
}

// HandleEdge is where a ResizeHandle sits on its container.
type HandleEdge uint8

const (
	// HandleCentered straddles the boundary it sits on.
	HandleCentered HandleEdge = iota
	// HandleLeading hugs the container's leading edge from inside: its
	// left, or top across.
	HandleLeading
	// HandleTrailing hugs its trailing edge from inside.
	HandleTrailing
)

// handleBandNode is a handle: a band 3 points across with the resize
// cursor, its line in the middle, or on the edge of the container it
// hugs from inside. Between two panels it takes no room: negative
// margins give back its thickness, so the band lies over the boundary
// within its parent's bounds, where the platforms give it its cursor and
// track the pointer over it.
func handleBandNode(h handleBandSpec) core.Node {
	zero := core.Pt(0)
	band := core.Style{Group: handleGroup, ZIndex: 1, FlexShrink: 0,
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	line := core.Style{}
	// The thumb is centered by alignment on a point of the line in its
	// middle, which holds the quarter turn, so the app's thumb keeps its
	// own Transform. The point is the line's width square, so the thumb
	// lies on the line's pixels when its width is odd too.
	over := core.Style{Position: core.Absolute, Width: core.Pt(1), Height: core.Pt(1),
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	thumb := core.Style{Position: core.Absolute}
	switch h.edge {
	case HandleLeading:
		band.Position = core.Absolute
		band.AlignItems, band.JustifyContent = core.AlignStart, core.JustifyStart
	case HandleTrailing:
		band.Position = core.Absolute
		band.AlignItems, band.JustifyContent = core.AlignEnd, core.JustifyEnd
	}
	// The margins either side give the band's thickness back, in whole
	// points, so its line stays on a pixel of the boundary.
	before := core.Points(math.Floor(float64(parts.HandleBand) / 2))
	after := parts.HandleBand - before
	var activation core.PanActivation
	if h.horizontal {
		band.Width, band.Cursor = core.Pt(parts.HandleBand), core.CursorColResize
		switch h.edge {
		case HandleLeading:
			band.Top, band.Bottom, band.Left = zero, zero, zero
			over.Left = zero
		case HandleTrailing:
			band.Top, band.Bottom, band.Right = zero, zero, zero
			over.Right = zero
		default:
			band.Margin = core.Edges{Left: -before, Right: -after}
			over.Left = core.Pt(before)
		}
		line.Width, line.Height = core.Pt(1), core.Pct(100)
		over.Top = core.Pct(50)
		activation = core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12}
	} else {
		band.Height, band.Cursor = core.Pt(parts.HandleBand), core.CursorRowResize
		switch h.edge {
		case HandleLeading:
			band.Left, band.Right, band.Top = zero, zero, zero
			over.Top = zero
		case HandleTrailing:
			band.Left, band.Right, band.Bottom = zero, zero, zero
			over.Bottom = zero
		default:
			band.Margin = core.Edges{Top: -before, Bottom: -after}
			over.Top = core.Pt(before)
		}
		line.Height, line.Width = core.Pt(1), core.Pct(100)
		over.Left = core.Pct(50)
		over.Transform.Rotate = 90
		activation = core.PanActivation{ActiveOffsetY: 2, FailOffsetX: 12}
	}
	var slop core.Edges
	if h.touch {
		if h.horizontal {
			slop = core.Edges{Left: handleSlop, Right: handleSlop}
		} else {
			slop = core.Edges{Top: handleSlop, Bottom: handleSlop}
		}
	}
	parts := []core.Node{core.View{Class: h.line.Class, Style: line.Merge(h.line.Style), StyleFunc: h.line.Fn, Children: []core.Node{h.content}}}
	if h.thumb.Set() {
		parts = append(parts, core.View{Style: over, Children: []core.Node{
			core.View{Class: h.thumb.Class, Style: thumb.Merge(h.thumb.Style), StyleFunc: h.thumb.Fn}}})
	}
	set := func(next ResizeHandleState) {
		if h.state.GetUntracked() != next {
			h.state.Set(next)
		}
	}
	off := func() bool { return h.disabled != nil && h.disabled() }
	pan := func(e core.PanEvent) {
		switch e.Phase {
		case core.GestureBegan:
			if !off() {
				set(HandleDragging)
			}
		case core.GestureEnded, core.GestureCancelled:
			set(HandleIdle)
		}
		h.onPan(e)
	}
	return core.View{
		Style: band, Disabled: h.disabled, HitSlop: slop,
		OnPan: pan, PanActivation: activation,
		OnHoverIn: func() {
			if h.state.GetUntracked() == HandleIdle {
				set(HandleHovered)
			}
		},
		OnHoverOut: func() {
			if h.state.GetUntracked() == HandleHovered {
				set(HandleIdle)
			}
		},
		// A pointer or finger going down presses the handle until it lifts
		// or a drag takes over, which ends with the pan.
		OnTouch: func(e core.TouchEvent) {
			switch e.Phase {
			case core.TouchStart:
				if !off() && h.state.GetUntracked() != HandleDragging {
					set(HandlePressed)
				}
			case core.TouchEnd, core.TouchCancel:
				if h.state.GetUntracked() == HandlePressed {
					set(HandleIdle)
				}
			}
		},
		// A press, a pointer that lifts without a drag, is a drag that
		// did not move, as NSH's handle reports one from the pointer going
		// down: a DataTable fits a column on two of them.
		OnPress: func(e core.PressEvent) {
			if off() {
				return
			}
			h.onPan(core.PanEvent{Phase: core.GestureBegan})
			h.onPan(core.PanEvent{Phase: core.GestureEnded})
			if e.Clicks == 2 && h.onDoublePress != nil {
				h.onDoublePress()
			}
		},
		NoFocus: true,
		Role:    core.RoleAdjustable, Label: h.label, TestID: h.testID,
		AccessibilityActions: handleActions, OnAccessibilityAction: h.onStep, AccessibilityValue: h.value,
		Children: parts,
	}
}

// handleActions are what assistive technology moves a handle with.
var handleActions = []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}}

// ResizeHandle is a handle's band on its own, on a container's edge
// (Edge) or straddling a boundary: it reports a drag, and the app
// resizes what it edges. Screen readers' steps report as a drag of 10
// points. OnDoublePress hears a double click or tap, which apps take to
// restore what it edges to its default size.
type ResizeHandle struct {
	// Axis is the axis it moves along.
	Axis Axis
	Edge HandleEdge
	// OnDragStart, OnDragMove and OnDragEnd follow a drag, with its
	// offset along Axis since it started.
	OnDragStart   func()
	OnDragMove    func(offset core.Points)
	OnDragEnd     func(offset core.Points)
	OnDoublePress func()
	Disabled      func() bool
	// OnStateChange follows the pointer's progression with the handle.
	OnStateChange func(ResizeHandleState)
	// Value is what assistive technology says the handle is at.
	Value          func() core.AccessibilityValue
	Class          string // the line's
	Style          core.Style
	StyleFunc      func() core.Style
	ThumbClass     string
	ThumbStyle     core.Style
	ThumbStyleFunc func() core.Style
	Label          string
	// Children are the app's, inside the line.
	Children []core.Node
	TestID   string
}

// Render implements core.Node.
func (r ResizeHandle) Render(cx *core.Scope) core.Node {
	horizontal := r.Axis != Vertical
	state := core.NewSignal(cx, HandleIdle)
	if f := r.OnStateChange; f != nil {
		reported := HandleIdle
		cx.Effect(func() {
			if now := state.Get(); now != reported {
				reported = now
				cx.Untrack(func() { f(now) })
			}
		})
	}
	offset := func(e core.PanEvent) core.Points {
		if !horizontal {
			return e.Translation.Y
		}
		if cx.Env().RTL() {
			return -e.Translation.X
		}
		return e.Translation.X
	}
	off := func() bool { return r.Disabled != nil && r.Disabled() }
	pan := func(e core.PanEvent) {
		if off() {
			return
		}
		switch e.Phase {
		case core.GestureBegan:
			if r.OnDragStart != nil {
				r.OnDragStart()
			}
		case core.GestureEnded, core.GestureCancelled:
			if r.OnDragEnd != nil {
				r.OnDragEnd(offset(e))
			}
			return
		}
		if r.OnDragMove != nil {
			r.OnDragMove(offset(e))
		}
	}
	step := func(name string) {
		if off() {
			return
		}
		d := handleStep
		switch name {
		case core.ActionIncrement:
		case core.ActionDecrement:
			d = -d
		default:
			return
		}
		if r.OnDragStart != nil {
			r.OnDragStart()
		}
		if r.OnDragMove != nil {
			r.OnDragMove(d)
		}
		if r.OnDragEnd != nil {
			r.OnDragEnd(d)
		}
	}
	return handleBandNode(handleBandSpec{horizontal: horizontal, edge: r.Edge, line: parts.Of(r.Class, r.Style, r.StyleFunc),
		thumb: parts.Of(r.ThumbClass, r.ThumbStyle, r.ThumbStyleFunc), label: r.Label, testID: r.TestID, disabled: r.Disabled, onPan: pan, onStep: step,
		onDoublePress: r.OnDoublePress, value: r.Value, touch: !core.Untracked(cx, cx.Env().Desktop), state: state, content: r.content()})
}

func (r ResizeHandle) content() core.Node {
	if len(r.Children) == 0 {
		return nil
	}
	return core.Group(r.Children)
}
