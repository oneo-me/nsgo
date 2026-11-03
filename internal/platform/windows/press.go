//go:build windows

package windows

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Presses as they happen: the left button going down on a view that
// follows presses, held there as a long press, and going up. The press
// itself is the engine's hit test at the release (press).

const (
	longPressTimer = 4
	// longPressDelay is how long a press is held to be a long press,
	// unless the view's DelayLongPress says else.
	longPressDelay = 500 * time.Millisecond
	// pressSlop is how far a press may move, in points, and still become
	// a long press.
	pressSlop = 8
)

// pressing is the press of the left button in progress.
type pressing struct {
	v    *view
	at   driver.Point // where it went down, in the surface
	long bool         // it was held as a long press
}

// pressDown starts following a press at at, in surface points, on the
// innermost view there that follows presses.
func (w *window) pressDown(at driver.Point) {
	w.pressCancel()
	root := w.view(w.root)
	if root == nil {
		return
	}
	v := hit(root, at, func(v *view) bool {
		m := v.events
		return m.Has(driver.EventPress) || m.Has(driver.EventLongPress) || m.Has(driver.EventPressDown) || m.Has(driver.EventPressUp)
	})
	if v == nil {
		return
	}
	w.pressing = &pressing{v: v, at: at}
	w.pressEvent(driver.EventPressDown)
	if v.events.Has(driver.EventLongPress) {
		delay := longPressDelay
		if d := v.props.DelayLongPress; d > 0 {
			delay = d
		}
		setTimer.Call(w.hwnd, longPressTimer, uintptr(delay.Milliseconds()), 0)
	}
}

// pressEvent reports kind about the press in progress, with where it went
// down and the keys held then.
func (w *window) pressEvent(kind driver.EventKind) {
	p := w.pressing
	if p == nil || p.v.deleted || !p.v.events.Has(kind) {
		return
	}
	ev := w.down
	ev.Kind, ev.ID, ev.Page = kind, p.v.id, p.at
	if b, ok := w.surfaceBounds(p.v); ok {
		ev.Point = driver.Point{X: p.at.X - b.Origin.X, Y: p.at.Y - b.Origin.Y}
		// A long press on a text's span is the span's.
		if l, ok := textOf(p.v); ok {
			ev.Index = l.indexAt(float32(ev.Point.X-p.v.content.Origin.X), float32(ev.Point.Y-p.v.content.Origin.Y))
		}
	}
	w.engine.Dispatch(ev)
}

// pressMoved stops waiting for a long press once the press moved away.
func (w *window) pressMoved(at driver.Point) {
	if p := w.pressing; p != nil && math.Hypot(float64(at.X-p.at.X), float64(at.Y-p.at.Y)) > pressSlop {
		killTimer.Call(w.hwnd, longPressTimer)
	}
}

// longPressed reports the press held as a long press.
func (w *window) longPressed() {
	killTimer.Call(w.hwnd, longPressTimer)
	if p := w.pressing; p != nil && !p.long {
		p.long = true
		w.pressEvent(driver.EventLongPress)
	}
}

// pressUp ends the press in progress, reporting whether it was a long
// press, which makes the release no press.
func (w *window) pressUp() bool {
	p := w.pressing
	if p == nil {
		return false
	}
	w.pressEvent(driver.EventPressUp)
	w.pressing = nil
	killTimer.Call(w.hwnd, longPressTimer)
	return p.long
}

// pressCancel ends the press in progress with no press: a pan took it,
// or another window the mouse.
func (w *window) pressCancel() {
	if w.pressing != nil {
		w.pressUp()
	}
}
