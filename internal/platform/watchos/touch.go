//go:build ios

package watchos

import (
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The host reports the one touch a watch has, on the whole surface;
// this file finds the views it reaches, as the Linux backend does for its
// pointer: presses and long presses of the innermost view that takes
// them, and pans of the innermost that pans, which hold the scroll views
// around it still while they move.

// longPressDelay is how long a touch is held to be a long press, unless
// the view's DelayLongPress says else; longPressSlop how far it may move.
const (
	longPressDelay = 500 * time.Millisecond
	longPressSlop  = 8
)

// touchState follows the touch on the surface.
type touchState struct {
	down     bool
	start    driver.Point // on the surface
	pressed  *view        // the view the touch presses, nil once it is no press
	pressAt  driver.Event
	long     int  // numbers long press waits; a later touch drops earlier ones
	consumed bool // a long press fired: the release presses nothing

	pan      *view
	panning  bool
	last     driver.Point
	lastTime float64
	velocity driver.Point
	locked   []*view // the scroll views a pan holds still
}

// Host.swift's touch phases.
const (
	touchBegan uint8 = iota
	touchMoved
	touchEnded
	touchCancelled
)

func hostTouch(phase uint8, x, y, t float64) {
	at := driver.Point{X: driver.Points(x), Y: driver.Points(y)}
	switch phase {
	case touchBegan:
		s.touchDown(at, t)
	case touchMoved:
		s.touchMove(at, t)
	case touchEnded:
		s.touchUp(at, t, false)
	case touchCancelled:
		s.touchUp(at, t, true)
	}
	s.flush()
}

func pressable(v *view) bool {
	m := v.events
	return m.Has(driver.EventPress) || m.Has(driver.EventLongPress) || m.Has(driver.EventPressDown) || m.Has(driver.EventPressUp)
}

func (s *surface) touchDown(at driver.Point, t float64) {
	g := &s.touch
	*g = touchState{down: true, start: at, last: at, lastTime: t, long: g.long + 1}
	root := s.view(s.root)
	if root == nil {
		return
	}
	s.pressOutside(root, at)
	if v := hit(root, at, pressable); v != nil {
		g.pressed = v
		g.pressAt = driver.Event{Count: 1, Page: at, Point: s.local(v, at), Index: -1}
		g.pressAt.Index = v.indexAt(g.pressAt.Point)
		v.pressEvent(driver.EventPressDown, g.pressAt)
		s.waitLongPress(v)
	}
	g.pan = hit(root, at, func(v *view) bool { return v.events.Has(driver.EventPan) })
}

func (s *surface) touchMove(at driver.Point, t float64) {
	g := &s.touch
	if !g.down {
		return
	}
	d := driver.Point{X: at.X - g.start.X, Y: at.Y - g.start.Y}
	if math.Hypot(float64(d.X), float64(d.Y)) > longPressSlop {
		g.long++
	}
	if p := g.pan; p != nil && !p.deleted {
		if !g.panning {
			switch p.props.PanActivation.Decide(d) {
			case 1:
				g.panning = true
				s.cancelPress()
				s.lockScrolls(p)
				p.report(driver.Event{Kind: driver.EventPan, Phase: driver.GestureBegan, Point: d})
			case -1:
				g.pan = nil
			}
		}
		if g.panning {
			if dt := t - g.lastTime; dt > 0 {
				inst := driver.Point{X: (at.X - g.last.X) / driver.Points(dt), Y: (at.Y - g.last.Y) / driver.Points(dt)}
				// Smoothed, so one uneven sample does not throw a flick.
				g.velocity = driver.Point{X: inst.X*0.7 + g.velocity.X*0.3, Y: inst.Y*0.7 + g.velocity.Y*0.3}
			}
			p.report(driver.Event{Kind: driver.EventPan, Phase: driver.GestureChanged, Point: d, Velocity: g.velocity})
		}
	}
	g.last, g.lastTime = at, t
}

func (s *surface) touchUp(at driver.Point, t float64, cancelled bool) {
	g := &s.touch
	if !g.down {
		return
	}
	g.down = false
	g.long++
	if p := g.pan; p != nil && g.panning && !p.deleted {
		phase, velocity := driver.GestureEnded, g.velocity
		if cancelled {
			phase = driver.GestureCancelled
		}
		// A drag that stopped before the finger lifted is no flick.
		if t-g.lastTime > 0.05 {
			velocity = driver.Point{}
		}
		d := driver.Point{X: at.X - g.start.X, Y: at.Y - g.start.Y}
		p.report(driver.Event{Kind: driver.EventPan, Phase: phase, Point: d, Velocity: velocity})
	}
	s.unlockScrolls()
	v := g.pressed
	g.pressed, g.pan, g.panning = nil, nil, false
	if v == nil || v.deleted {
		return
	}
	v.pressEvent(driver.EventPressUp, g.pressAt)
	if cancelled || g.consumed {
		return
	}
	if p := s.local(v, at); v.within(p) {
		e := g.pressAt
		e.Point, e.Index = p, v.indexAt(p)
		v.pressEvent(driver.EventPress, e)
	}
}

// cancelPress ends the press the touch began, as a scroll or a pan takes
// the touch: the view hears it go up, and no press.
func (s *surface) cancelPress() {
	g := &s.touch
	g.long++
	if v := g.pressed; v != nil {
		g.pressed = nil
		if !v.deleted {
			v.pressEvent(driver.EventPressUp, g.pressAt)
		}
	}
	if !g.panning {
		g.pan = nil
	}
}

func (v *view) pressEvent(kind driver.EventKind, e driver.Event) {
	e.Kind = kind
	v.report(e)
}

// waitLongPress fires v's long press once the touch is held still long
// enough.
func (s *surface) waitLongPress(v *view) {
	if !v.events.Has(driver.EventLongPress) {
		return
	}
	delay := longPressDelay
	if d := v.props.DelayLongPress; d > 0 {
		delay = d
	}
	seq := s.touch.long
	time.AfterFunc(delay, func() {
		onMain(func() {
			g := &s.touch
			if g.long != seq || g.pressed != v || v.deleted {
				return
			}
			g.consumed = true
			v.pressEvent(driver.EventLongPress, g.pressAt)
			s.flush()
		})
	})
}

// pressOutside tells the views that listen for presses outside them and
// do not hold at.
func (s *surface) pressOutside(root *view, at driver.Point) {
	var walk func(v *view)
	walk = func(v *view) {
		if v.events.Has(driver.EventPressOutside) {
			if b, ok := s.surfaceBounds(v); ok && !b.Contains(at) {
				v.report(driver.Event{Kind: driver.EventPressOutside, Page: at})
			}
		}
		for _, c := range v.children {
			walk(c)
		}
	}
	walk(root)
}

// lockScrolls holds the scroll views around v still while its pan runs.
func (s *surface) lockScrolls(v *view) {
	for p := v.parent; p != nil; p = p.parent {
		if p.kind == driver.KindScroll {
			s.touch.locked = append(s.touch.locked, p)
			s.w.op(opLock, p.id)
			s.w.flag(true)
		}
	}
}

func (s *surface) unlockScrolls() {
	for _, p := range s.touch.locked {
		if !p.deleted {
			s.w.op(opLock, p.id)
			s.w.flag(false)
		}
	}
	s.touch.locked = nil
}

// hit returns the innermost view under p that match takes, p in the
// coordinates of v's parent's content, the child drawn on top first.
func hit(v *view, p driver.Point, match func(*view) bool) *view {
	if v.style.Hidden || v.deleted {
		return nil
	}
	f := v.shownFrame()
	self, children := driver.Reach(f, &v.props, v.overflow, p)
	if children {
		local := driver.Point{X: p.X - f.Origin.X + v.offset.X, Y: p.Y - f.Origin.Y + v.offset.Y}
		for i := len(v.children) - 1; i >= 0; i-- {
			if h := hit(v.children[i], local, match); h != nil {
				return h
			}
		}
	}
	if self && match(v) {
		return v
	}
	return nil
}

// surfaceBounds is where v is on the surface, and whether it is in the
// root's tree.
func (s *surface) surfaceBounds(v *view) (driver.Bounds, bool) {
	b, top := v.shownFrame(), v
	for p := v.parent; p != nil; p = p.parent {
		f := p.shownFrame()
		b.Origin.X += f.Origin.X - p.offset.X
		b.Origin.Y += f.Origin.Y - p.offset.Y
		top = p
	}
	return b, top.id == s.root
}

// local is surface point at in v's coordinates.
func (s *surface) local(v *view, at driver.Point) driver.Point {
	b, _ := s.surfaceBounds(v)
	return driver.Point{X: at.X - b.Origin.X, Y: at.Y - b.Origin.Y}
}

// within reports whether p, in v's coordinates, is on v or in its hit
// slop, where a press ends as a press.
func (v *view) within(p driver.Point) bool {
	sl, f := v.props.HitSlop, v.shownFrame()
	return p.X >= -sl.Left && p.Y >= -sl.Top && p.X < f.Size.Width+sl.Right && p.Y < f.Size.Height+sl.Bottom
}

// indexAt is the byte offset of the character at p in a text view, for
// presses on its links, or -1.
func (v *view) indexAt(p driver.Point) int {
	if v.layout == nil || v.kind != driver.KindText {
		return -1
	}
	return v.layout.IndexAt(cgPoint(p), cgRect(v.box))
}
