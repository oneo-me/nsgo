// Package sheet is the sheet base's dialogs, menus and docks come up as:
// a panel from an edge of the window over a fading backdrop.
package sheet

import (
	"time"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// How long a sheet takes to come in and to go, as NSH's.
const (
	sheetIn  = 280 * time.Millisecond
	sheetOut = 200 * time.Millisecond
)

// A bottom sheet that follows downward drags closes on a release past
// this share of its height or this speed in points a second.
const (
	swipeShare    = 0.3
	swipeVelocity = 800
)

// Motion moves a panel in from an edge over a fading backdrop and
// back out, as NSH's SheetMotion: the platform runs both on animated
// values, and the parts stay in the tree until the motion out is over.
type Motion struct {
	cx *core.Scope
	// shown: the parts are in the tree, while open and while they go.
	shown *core.Signal[bool]
	// settled: it came in and has not started to go.
	settled *core.Signal[bool]
	// closing: it goes, and leaves the tree once its motion is over.
	closing *core.Signal[bool]
	// fade is the backdrop's opacity and how far in the panel is; drag
	// how far a finger pulled the panel down, in points.
	fade, drag   *core.AnimatedValue
	awayX, awayY *core.AnimatedValue
	// x and y are the panel's translation.
	x, y core.AnimatedNode
	away [2]float64
	side core.Placement
	// entering: shown, waiting for the panel's size to come in from.
	entering bool
	// generation tells a motion's end from an older one's.
	generation int
}

// NewMotion returns a motion following open, its panel on side.
func NewMotion(cx *core.Scope, open func() bool, side core.Placement) *Motion {
	m := &Motion{cx: cx, side: side,
		shown: core.NewSignal(cx, false), settled: core.NewSignal(cx, false), closing: core.NewSignal(cx, false),
		fade: core.NewAnimatedValue(cx, 0), drag: core.NewAnimatedValue(cx, 0),
		awayX: core.NewAnimatedValue(cx, 0), awayY: core.NewAnimatedValue(cx, 0)}
	remaining := core.Interpolate(m.fade, core.Interpolation{Input: []float64{0, 1}, Output: []float64{1, 0}})
	// The finger pulls the panel down, never up.
	down := core.Interpolate(m.drag, core.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 1},
		ExtrapolateRight: core.ExtrapolateExtend})
	m.x = core.Mul(m.awayX, remaining)
	m.y = core.Add(core.Mul(m.awayY, remaining), down)
	cx.Effect(func() {
		o := open()
		cx.Untrack(func() { m.follow(o) })
	})
	return m
}

// Shown reports, tracked, whether the sheet is in the tree.
func (m *Motion) Shown() bool { return m.shown.Get() }

// There reports, tracked, whether the sheet is there for its lifecycle:
// from when it came in until it is gone.
func (m *Motion) There() bool {
	return m.settled.Get() || m.shown.Get() && m.closing.Get()
}

func (m *Motion) reduced() bool { return core.Untracked(m.cx, m.cx.Env().ReduceMotion) }

func (m *Motion) follow(open bool) {
	if open {
		if m.closing.GetUntracked() {
			// Opened again on its way out: back in from where it is.
			m.closing.Set(false)
			m.comeIn()
			return
		}
		if m.shown.GetUntracked() {
			return
		}
		m.fade.Set(0)
		m.drag.Set(0)
		m.shown.Set(true)
		// Out of sight until the panel is laid out: as far as the window.
		m.setAway(core.Untracked(m.cx, m.cx.Env().Window))
		m.entering = true
		return
	}
	if !m.shown.GetUntracked() || m.closing.GetUntracked() {
		return
	}
	gen := m.next()
	m.entering = false
	m.settled.Set(false)
	if m.reduced() {
		m.shown.Set(false)
		return
	}
	m.closing.Set(true)
	out := core.TimingConfig{Duration: sheetOut, Easing: core.BezierEasing(core.EaseIn)}
	gone := func() {
		if m.generation == gen && m.closing.GetUntracked() {
			m.closing.Set(false)
			m.shown.Set(false)
		}
	}
	// Gone when the platform reports the motion's end, or after its time
	// on one that reports none.
	m.cx.StartAnimation(core.Parallel(core.Timing(m.drag, 0, out), core.Timing(m.fade, 0, out)),
		func(bool) { gone() })
	m.cx.AfterFunc(sheetOut, gone)
}

// laidOut gives the panel's size, which it comes in from.
func (m *Motion) laidOut(size core.Size) {
	m.setAway(size)
	if m.entering {
		m.entering = false
		m.comeIn()
	}
}

// comeIn moves the panel from where it is to its place and the backdrop
// to opaque; it is settled once there.
func (m *Motion) comeIn() {
	gen := m.next()
	if m.reduced() {
		m.drag.Set(0)
		m.fade.Set(1)
		m.settled.Set(true)
		return
	}
	in := core.TimingConfig{Duration: sheetIn, Easing: core.BezierEasing(core.EaseOut)}
	there := func() {
		if m.generation == gen && !m.closing.GetUntracked() && m.shown.GetUntracked() {
			m.settled.Set(true)
		}
	}
	m.cx.StartAnimation(core.Parallel(core.Timing(m.drag, 0, in), core.Timing(m.fade, 1, in)),
		func(bool) { there() })
	m.cx.AfterFunc(sheetIn, there)
}

func (m *Motion) next() int {
	m.generation++
	return m.generation
}

// setAway sets the translation that puts a panel of size out of sight
// past its side; Left and Right swap where the layout runs right to left.
func (m *Motion) setAway(size core.Size) {
	var x, y float64
	w, h := float64(size.Width), float64(size.Height)
	rtl := core.Untracked(m.cx, m.cx.Env().RTL)
	switch m.side {
	case core.PlaceTop:
		y = -h
	case core.PlaceLeft:
		x = -w
	case core.PlaceRight:
		x = w
	default:
		y = h
	}
	if rtl {
		x = -x
	}
	if x != m.away[0] {
		m.awayX.Set(x)
	}
	if y != m.away[1] {
		m.awayY.Set(y)
	}
	m.away = [2]float64{x, y}
}

// panelPlace is where a panel on side sits in its layer.
func panelPlace(side core.Placement) core.Style {
	zero := core.Pt(0)
	switch side {
	case core.PlaceTop:
		return core.Style{Position: core.Absolute, Left: zero, Right: zero, Top: zero}
	case core.PlaceLeft:
		return core.Style{Position: core.Absolute, Top: zero, Bottom: zero, Left: zero}
	case core.PlaceRight:
		return core.Style{Position: core.Absolute, Top: zero, Bottom: zero, Right: zero}
	}
	return core.Style{Position: core.Absolute, Left: zero, Right: zero, Bottom: zero}
}

// Layer is what a sheet shows while its motion is shown: a backdrop
// over the whole root view whose presses backdropPress hears, fading
// with the motion, and the panel on side.
type Layer struct {
	Motion        *Motion
	Side          core.Placement
	Backdrop      parts.Part
	BackdropPress func()
	Panel         parts.Part
	// Swipe lets a finger drag the panel down, and Swiped hears a release
	// that asks it to close.
	Swipe   bool
	Swiped  func()
	Cancel  func()
	Role    core.Role
	Label   string
	TestID  string
	Content []core.Node
	// Layer styles the overlay holding the backdrop and the panel.
	Layer parts.Part
}

// Node is the layer.
func (l Layer) Node() core.Node {
	m := l.Motion
	return core.Show{When: m.shown.Get, Then: func(cx *core.Scope) core.Node {
		// The backdrop fades with the panel and with a drag away.
		extent := core.NewAnimatedValue(cx, 1)
		down := core.Interpolate(m.drag, core.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 1},
			ExtrapolateRight: core.ExtrapolateExtend})
		opacity := core.Interpolate(core.Sub(m.fade, core.Div(down, extent)),
			core.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 1}})
		var height core.Points = 1
		backdrop := core.View{Class: l.Backdrop.Class,
			Style:     parts.Fill.Merge(l.Backdrop.Style).Merge(core.Style{Animated: &core.AnimatedStyle{Opacity: opacity}}),
			StyleFunc: l.Backdrop.Fn, NoFocus: true,
			OnPress: func(core.PressEvent) { l.BackdropPress() }}
		panel := core.View{
			Class: l.Panel.Class,
			Style: panelPlace(l.Side).Merge(l.Panel.Style).Merge(core.Style{Animated: &core.AnimatedStyle{
				TranslateX: m.x, TranslateY: m.y}}),
			StyleFunc: l.Panel.Fn, Role: l.Role, Label: l.Label, TestID: l.TestID, Children: l.Content,
			// Its blank space takes presses, which would reach the backdrop.
			OnPress: func(core.PressEvent) {}, NoFocus: true,
			OnLayout: func(b core.Bounds) {
				height = max(b.Size.Height, 1)
				extent.Set(float64(height))
				m.laidOut(b.Size)
			},
		}
		if l.Swipe {
			panel.PanValues = core.AnimatedXY{Y: m.drag}
			panel.PanActivation = core.PanActivation{ActiveOffsetY: 8, FailOffsetX: 12}
			// On iOS a scroll view inside at its top hands the drag on.
			panel.PanScrollHandoff = true
			panel.OnPan = func(e core.PanEvent) {
				if e.Phase != core.GestureEnded && e.Phase != core.GestureCancelled {
					return
				}
				distance := max(e.Values.Y, 0)
				if e.Phase == core.GestureEnded && distance > 0 &&
					(distance >= swipeShare*height || e.Velocity.Y >= swipeVelocity) {
					l.Swiped()
				}
				// Still open, as a controlled sheet may stay: it comes back.
				if !m.closing.GetUntracked() {
					m.comeIn()
				}
			}
		}
		return core.Overlay{Class: l.Layer.Class, Style: parts.Cover.Merge(l.Layer.Style), StyleFunc: l.Layer.Fn,
			PointerEvents: core.PointerEventsBoxNone, OnCancel: l.Cancel, Children: []core.Node{backdrop, panel}}
	}}
}
