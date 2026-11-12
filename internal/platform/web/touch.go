//go:build js && wasm

package web

import (
	"slices"
	"syscall/js"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Touches for the views that listen to them: each pointer down on the
// page is a finger of core.EventTouch, seen before the views below take
// it. The browser cancels a pointer it scrolls the page with.

// fingers are the pointers down, by pointerId, in order.
type fingers struct {
	down    []driver.Touch
	blocked bool // the responder stops the browser's scrolling
}

func (s *surface) listenTouches(ls *listeners) {
	for _, name := range []string{"pointerdown", "pointermove", "pointerup", "pointercancel"} {
		fn := js.FuncOf(func(_ js.Value, args []js.Value) any {
			s.finger(args[0])
			return nil
		})
		*ls = append(*ls, listener{s.host, name, fn})
		s.host.Call("addEventListener", name, fn, true)
	}
	// A touchmove the page ignores scrolls it, so one is cancelled while
	// the touch responder holds the touches.
	block := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if s.touch.blocked && args[0].Get("cancelable").Bool() {
			args[0].Call("preventDefault")
		}
		return nil
	})
	*ls = append(*ls, listener{s.host, "touchmove", block})
	s.host.Call("addEventListener", "touchmove", block, map[string]any{"passive": false})
}

// finger reports pointer event e as a finger, while a view listens.
func (s *surface) finger(e js.Value) {
	id := e.Get("pointerId").Int()
	f := &s.touch
	i := slices.IndexFunc(f.down, func(t driver.Touch) bool { return t.ID == id })
	var phase driver.GesturePhase
	switch e.Get("type").String() {
	case "pointerdown":
		if i >= 0 || e.Get("button").Int() != 0 || !s.listensTouches() {
			return
		}
		phase = driver.GestureBegan
		f.down = append(f.down, driver.Touch{ID: id})
		i = len(f.down) - 1
	case "pointermove":
		if i < 0 {
			return
		}
		phase = driver.GestureChanged
	case "pointerup":
		phase = driver.GestureEnded
	default:
		phase = driver.GestureCancelled
	}
	if i < 0 {
		return
	}
	r := s.host.Call("getBoundingClientRect")
	f.down[i].PagePosition = driver.Point{
		X: driver.Points(e.Get("clientX").Float() - r.Get("left").Float() - s.host.Get("clientLeft").Float()),
		Y: driver.Points(e.Get("clientY").Float() - r.Get("top").Float() - s.host.Get("clientTop").Float()),
	}
	touches := slices.Clone(f.down)
	touches[i].Changed = true
	if phase == driver.GestureEnded || phase == driver.GestureCancelled {
		f.down = slices.Delete(f.down, i, i+1)
	}
	s.engine.Dispatch(driver.Event{Kind: driver.EventTouch, Phase: phase, Touches: touches,
		Time: time.Duration(e.Get("timeStamp").Float() * float64(time.Millisecond))})
}

// listensTouches reports whether a view listens to touches.
func (s *surface) listensTouches() bool {
	for _, v := range s.views {
		if v != nil && v.events.Has(driver.EventTouch) {
			return true
		}
	}
	return false
}
