// Package finger reports a mouse as finger 0 of core.EventTouch, for the
// desktop backends: its primary button going down, moving while down and
// going up are a finger's touches.
package finger

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Mouse follows one mouse's primary button.
type Mouse struct {
	down   bool
	origin time.Time
}

// Listens reports whether a view reporting events m listens to touches.
func Listens(m driver.EventMask) bool { return m.Has(driver.EventTouch) }

// Down reports the button going down at p, in surface points, when a view
// listens; the touches it starts are reported until the button goes up.
func (m *Mouse) Down(listening bool, p driver.Point) (driver.Event, bool) {
	m.down = listening
	if m.origin.IsZero() {
		m.origin = time.Now()
	}
	return m.event(driver.GestureBegan, p), listening
}

// Move reports the mouse moving to p while the button is down.
func (m *Mouse) Move(p driver.Point) (driver.Event, bool) {
	return m.event(driver.GestureChanged, p), m.down
}

// Up reports the button going up at p.
func (m *Mouse) Up(p driver.Point) (driver.Event, bool) {
	was := m.down
	m.down = false
	return m.event(driver.GestureEnded, p), was
}

// Cancel reports the touches taken away, such as by another window
// capturing the mouse, at p.
func (m *Mouse) Cancel(p driver.Point) (driver.Event, bool) {
	was := m.down
	m.down = false
	return m.event(driver.GestureCancelled, p), was
}

func (m *Mouse) event(phase driver.GesturePhase, p driver.Point) driver.Event {
	return driver.Event{Kind: driver.EventTouch, Phase: phase, Time: time.Since(m.origin),
		Touches: []driver.Touch{{PagePosition: p, Changed: true}}}
}
