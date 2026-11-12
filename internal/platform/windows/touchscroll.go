//go:build windows

package windows

import (
	"math"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/scrollbar"
)

// A touchscreen's finger drags scroll views (scrollbar.TouchDrag), as on
// Linux: Windows turns a touch into mouse messages, so a press and a pan
// start as with the mouse, until the finger moves past the slop along a
// scroll view under it; the view then takes the touch, whose pointer
// messages are no longer turned into mouse messages, and the press and
// pan are cancelled. Let go, the view glides, snaps or springs back on
// the frame clock, and a pull past the top refreshes, keeping room above
// the content for the spinner, or the app's own indicator, while it does.
// Values bound to the pull follow it.

// touchScroll is the touch that may drag a scroll view: its pointer, the
// views under it that scroll, innermost first, and the drag of the one
// that took it.
type touchScroll struct {
	pointer uint32
	views   []*view
	drags   []*scrollbar.TouchDrag
	taken   *view
	drag    *scrollbar.TouchDrag
}

// touchPointer follows a touch's pointer messages for the scroll views;
// it reports whether a scroll view took the message, which then becomes
// no mouse message.
func (w *window) touchPointer(message uint32, wParam, lParam uintptr) bool {
	id := uint32(wParam & 0xffff)
	var kind uint32
	if r, _, _ := getPointerType.Call(uintptr(id), uintptr(unsafe.Pointer(&kind))); r == 0 || kind != ptTouch {
		return false
	}
	at := point{int32(int16(lParam & 0xffff)), int32(int16(lParam >> 16 & 0xffff))}
	screenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
	p := w.point(at.X, at.Y)
	switch message {
	case wmPointerDown:
		if w.touching == nil {
			w.touchDown(id, p)
		}
		return false
	case wmPointerUpdate:
		return w.touching != nil && w.touching.pointer == id && w.touchMove(p)
	case wmPointerUp, wmPointerLeave:
		if w.touching == nil || w.touching.pointer != id {
			return false
		}
		return w.touchUp()
	}
	return false
}

// touchDown starts following a finger down at at for the scroll views
// under it; it stops their glides, as a finger on a moving list does.
func (w *window) touchDown(pointer uint32, at driver.Point) {
	w.touching = nil
	root := w.view(w.root)
	if root == nil {
		return
	}
	under := scrollsAt(root, at, nil)
	t := &touchScroll{pointer: pointer}
	now := time.Now()
	for i := len(under) - 1; i >= 0; i-- {
		v := under[i]
		if v.scroll.Behavior.NoScroll {
			continue
		}
		if v.glide != nil {
			v.glide = nil
			w.scrollEvent(v, driver.EventMomentumEnd)
		}
		t.views = append(t.views, v)
		t.drags = append(t.drags, scrollbar.NewTouchDrag(at, scrollbar.Along(&v.scroll, v.offset), now))
	}
	if len(t.views) > 0 {
		w.touching = t
	}
}

// touchMove follows the finger to at; it reports whether a scroll view
// has it.
func (w *window) touchMove(at driver.Point) bool {
	t := w.touching
	if t.taken == nil {
		if w.pan != nil && w.pan.active {
			w.touching = nil
			return false
		}
		for i, v := range t.views {
			if !v.deleted && t.drags[i].Claims(&v.scroll, at) {
				t.taken, t.drag = v, t.drags[i]
				break
			}
		}
		if t.taken == nil {
			return false
		}
		// The touch is the view's: no press, long press or pan.
		w.pressCancel()
		w.panLost()
		w.touch(w.mouse.Cancel(w.pointer))
		releaseCapture.Call()
		w.scrollEvent(t.taken, driver.EventScrollDragStart)
	}
	v := t.taken
	if v.deleted {
		w.touching = nil
		return true
	}
	c := &v.scroll
	raw := t.drag.Move(c, at, time.Now())
	lo, hi := scrollbar.Limits(c, v.frame.Size, v.size, v.refreshing)
	viewport := scrollbar.Along(c, driver.Point{X: v.frame.Size.Width, Y: v.frame.Size.Height})
	offset := v.offset
	scrollbar.SetAlong(c, &offset, scrollbar.Dragged(raw, lo, hi, viewport, !c.Behavior.NoBounces))
	w.moveOffset(v, offset)
	return true
}

// touchUp lets the finger go: the view it dragged glides, snaps or
// springs back, refreshing when pulled far enough. It reports whether a
// view had the touch.
func (w *window) touchUp() bool {
	t := w.touching
	w.touching = nil
	if t == nil || t.taken == nil || t.taken.deleted {
		return false
	}
	v, c := t.taken, &t.taken.scroll
	if scrollbar.Refreshes(c, v.offset, v.refreshing) {
		v.refreshing = true
		w.scrollEvent(v, driver.EventRefresh)
	}
	w.scrollEvent(v, driver.EventScrollDragEnd)
	lo, hi := scrollbar.Limits(c, v.frame.Size, v.size, v.refreshing)
	velocity := t.drag.Velocity(c)
	if w.env.ReduceMotion {
		velocity = 0
	}
	m := scrollbar.Release(c, v.frame.Size, v.size, v.offset, velocity, lo, hi, time.Now())
	if m.Moves() {
		w.scrollEvent(v, driver.EventMomentumStart)
		w.glide(v, &m, true)
	} else {
		w.restScroll(v)
		w.spin(v)
	}
	return true
}

// glide moves v along m on the frame clock, reporting its momentum's end
// when momentum.
func (w *window) glide(v *view, m *scrollbar.Motion, momentum bool) {
	started := v.glide == nil
	v.glide = m
	if !started {
		return
	}
	w.onFrame(func(now time.Time) bool {
		m := v.glide
		if v.deleted || m == nil {
			return false
		}
		at, done := m.At(now)
		offset := v.offset
		scrollbar.SetAlong(&v.scroll, &offset, at)
		w.moveOffset(v, offset)
		if done {
			v.glide = nil
			if momentum {
				w.scrollEvent(v, driver.EventMomentumEnd)
			}
			w.restScroll(v)
			w.spin(v)
		}
		return !done
	})
}

// spin turns the spinner of v on the frame clock while it refreshes with
// it in view.
func (w *window) spin(v *view) {
	c := &v.scroll
	if !v.refreshing || c.Behavior.RefreshIndicator > 0 || v.spinning {
		return
	}
	v.spinning = true
	w.onFrame(func(time.Time) bool {
		if v.deleted || !v.refreshing || scrollbar.Pulled(&v.scroll, v.offset) <= 0 {
			v.spinning = false
			w.redrawView(v)
			return false
		}
		w.redrawView(v)
		return true
	})
}

// refreshChanged follows the app's Refreshing: room shows above the top
// while it refreshes and goes as it ends.
func (w *window) refreshChanged(v *view) {
	c := &v.scroll
	on := c.Refreshing && !c.Horizontal
	if on == v.refreshing {
		return
	}
	v.refreshing = on
	top := -c.Behavior.ContentInset.Top
	held := w.touching != nil && w.touching.taken == v
	switch {
	case on && v.offset.Y <= top+0.5 && !held:
		w.glideTo(v, top-c.Behavior.RefreshInset())
	case !on && v.offset.Y < top && !held:
		w.glideTo(v, top)
	}
	w.spin(v)
	w.redrawView(v)
}

// glideTo eases v to along its axis.
func (w *window) glideTo(v *view, to driver.Points) {
	m := scrollbar.Release(&v.scroll, v.frame.Size, v.size, v.offset, 0, to, to, time.Now())
	if m.Moves() {
		w.glide(v, &m, false)
	}
}

// drawSpinner draws the platform's refresh spinner in the room pulled
// above v's content: spokes that show as the pull grows and turn while v
// refreshes. world places v.
func (p *painter) drawSpinner(v *view, world matrix) {
	c := &v.scroll
	if !c.Refreshable || c.Horizontal || c.Behavior.RefreshIndicator > 0 {
		return
	}
	pulled := scrollbar.Pulled(c, v.offset)
	if pulled < 4 {
		return
	}
	const spokes = 12
	cx, cy := float64(v.frame.Size.Width)/2, float64(c.Behavior.ContentInset.Top)+float64(pulled)/2
	shown := spokes
	phase := 0
	if !v.refreshing {
		shown = int(min(float64(pulled)/float64(c.Behavior.RefreshThreshold()), 1) * spokes)
	} else {
		phase = int(time.Now().UnixMilli()/80) % spokes
	}
	for i := range shown {
		angle := 2 * math.Pi * float64(i) / spokes
		sin, cos := math.Sincos(angle)
		at := matrix{a: cos, b: sin, c: -sin, d: cos, e: cx, f: cy}.then(world)
		p.setTransform(at)
		alpha := float32(1)
		if v.refreshing {
			alpha = 0.25 + 0.75*float32((i-phase+spokes)%spokes)/spokes
		}
		p.fillRounded(rectF{-1.25, -10, 1.25, -5}, 1.25, driver.RGBA(0x737373, alpha))
	}
	p.setTransform(world)
}
