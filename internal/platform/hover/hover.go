// Package hover follows the pointer over views for the backends whose
// views hear no hovers of their own, Windows' and Linux's. As a tracking
// area or pointerenter does, a view stays hovered while the pointer is
// over it or any view inside it.
package hover

import (
	"slices"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Tracker holds the views the pointer is over that follow hovers.
type Tracker struct {
	over []driver.NodeID // outermost first
}

// Move sets the views under the pointer that follow hovers, outermost
// first, and returns the events that report the change: leaving views
// innermost first, then entering ones outermost first.
func (t *Tracker) Move(under []driver.NodeID) []driver.Event {
	var events []driver.Event
	for i := len(t.over) - 1; i >= 0; i-- {
		if !slices.Contains(under, t.over[i]) {
			events = append(events, driver.Event{Kind: driver.EventMouseLeave, ID: t.over[i]})
		}
	}
	for _, id := range under {
		if !slices.Contains(t.over, id) {
			events = append(events, driver.Event{Kind: driver.EventMouseEnter, ID: id})
		}
	}
	t.over = append(t.over[:0], under...)
	return events
}

// Pressed is the button a platform's button bits hold down: the first
// that is, the bits naming left, right, middle, back and forward.
func Pressed(state uint32, bits [5]uint32) driver.MouseButton {
	for i, b := range [...]driver.MouseButton{driver.MouseButtonLeft, driver.MouseButtonRight, driver.MouseButtonMiddle,
		driver.MouseButtonBack, driver.MouseButtonForward} {
		if state&bits[i] != 0 {
			return b
		}
	}
	return driver.MouseButtonNone
}

// Forget drops id, whose view went away, without reporting it.
func (t *Tracker) Forget(id driver.NodeID) {
	t.over = slices.DeleteFunc(t.over, func(o driver.NodeID) bool { return o == id })
}

// Follows reports whether a view reporting events m follows hovers: any
// of the three mouse events makes it take part.
func Follows(m driver.EventMask) bool {
	return m.Has(driver.EventMouseEnter) || m.Has(driver.EventMouseLeave) || m.Has(driver.EventMouseMove)
}
