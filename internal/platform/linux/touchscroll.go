//go:build linux && !android

package linux

import (
	"math"
	"time"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/scrollbar"
)

// A touchscreen's finger drags scroll views (scrollbar.TouchDrag): the
// innermost scroll view under it that moves along the drag takes it once
// it passes the slop, unless a pan took it first; the press it began
// presses nothing. Let go, the view glides, snaps or springs back on the
// frame clock, and a pull past the top refreshes, keeping room above the
// content for the spinner, or the app's own indicator, while it does.
// Values bound to the pull follow it.

const gdkSourceTouchscreen = 3

var (
	gdkDeviceGetSource func(uintptr) int32
	gActionActivate    func(action, parameter uintptr)
)

func bindTouchScroll() {
	purego.RegisterLibFunc(&gdkDeviceGetSource, gtkLib, "gdk_device_get_source")
	purego.RegisterLibFunc(&gActionActivate, gioLib, "g_action_activate")
}

// touchScroll is the touch that may drag a scroll view: the views under
// it that scroll, innermost first, and the drag of the one that took it.
type touchScroll struct {
	views []*view
	drags []*scrollbar.TouchDrag
	taken *view
	drag  *scrollbar.TouchDrag
}

// isTouch reports whether device is a touchscreen.
func isTouch(device uintptr) bool {
	return device != 0 && gdkDeviceGetSource(device) == gdkSourceTouchscreen
}

// touchDown starts following a finger down at at for the scroll views
// under it; it stops their glides, as a finger on a moving list does.
func (w *window) touchDown(at driver.Point) {
	w.touching = nil
	root := w.view(w.root)
	if root == nil {
		return
	}
	under := scrollsAt(root, at, nil)
	t := &touchScroll{}
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
	if t == nil {
		return false
	}
	if t.taken == nil {
		if w.panning != nil && w.panning.active {
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
		w.cancelLongPress()
		w.pressConsumed = true
		w.panning = nil
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
// springs back, refreshing when pulled far enough.
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
		v.glide = &m
		w.scrollEvent(v, driver.EventMomentumStart)
		w.animate(v)
	} else {
		w.restScroll(v)
	}
	return true
}

// stepGlide moves v along its glide; it reports whether it goes on.
func (w *window) stepGlide(v *view, now time.Time) bool {
	m := v.glide
	if m == nil {
		return v.refreshing && v.scroll.Behavior.RefreshIndicator == 0 && scrollbar.Pulled(&v.scroll, v.offset) > 0
	}
	at, done := m.At(now)
	offset := v.offset
	scrollbar.SetAlong(&v.scroll, &offset, at)
	w.moveOffset(v, offset)
	if done {
		v.glide = nil
		w.scrollEvent(v, driver.EventMomentumEnd)
		w.restScroll(v)
	}
	return !done || v.refreshing
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
	switch {
	case on && v.offset.Y <= top+0.5 && w.touching == nil:
		w.glideTo(v, top-c.Behavior.RefreshInset())
	case !on && v.offset.Y < top && (w.touching == nil || w.touching.taken != v):
		w.glideTo(v, top)
	}
	gtkWidgetQueueDraw(v.widget)
}

// glideTo eases v to along its axis.
func (w *window) glideTo(v *view, to driver.Points) {
	lo, hi := to, to
	m := scrollbar.Release(&v.scroll, v.frame.Size, v.size, v.offset, 0, lo, hi, time.Now())
	if !m.Moves() {
		return
	}
	v.glide = &m
	w.animate(v)
}

// snapshotSpinner draws the platform's refresh spinner in the room
// pulled above v's content: an arc that grows with the pull and turns
// while v refreshes.
func snapshotSpinner(v *view, snap uintptr, now time.Time) {
	c := &v.scroll
	if !c.Refreshable || c.Horizontal || c.Behavior.RefreshIndicator > 0 {
		return
	}
	pulled := scrollbar.Pulled(c, v.offset)
	if pulled < 4 {
		return
	}
	const radius, thick = 10, 2.5
	cx, cy := float64(v.frame.Size.Width)/2, float64(c.Behavior.ContentInset.Top)+float64(pulled)/2
	area := grapheneRect{float32(cx - radius - thick), float32(cy - radius - thick), 2 * (radius + thick), 2 * (radius + thick)}
	cr := gtkSnapshotAppendCairo(snap, &area)
	sweep := min(float64(pulled)/float64(c.Behavior.RefreshThreshold()), 1) * 1.6 * math.Pi
	turn := -math.Pi / 2
	if v.refreshing {
		sweep = 1.6 * math.Pi
		turn += math.Mod(float64(now.UnixMilli())/1000*2*math.Pi, 2*math.Pi)
	}
	cairoSetSourceRGBA(cr, 0.45, 0.45, 0.45, 1)
	cairoSetLineWidth(cr, thick)
	cairoNewPath(cr)
	cairoArc(cr, cx, cy, radius, turn, turn+sweep)
	cairoStroke(cr)
	cairoDestroy(cr)
}
