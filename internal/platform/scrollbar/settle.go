package scrollbar

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// WheelIdle is how long after its last turn a mouse wheel has settled.
const WheelIdle = 150 * time.Millisecond

// WheelSnap is the speed (points per millisecond) a settled wheel snaps
// with: fast enough to move on to the next snap point that way, as NSH's
// desktops do.
const WheelSnap = 0.6

// Clamp returns offset within the offsets a scroll view of size frame
// showing content takes: from minus its leading content inset to the end
// of the content and its trailing inset.
func Clamp(c *driver.ScrollConfig, frame, content driver.Size, offset driver.Point) driver.Point {
	in := c.Behavior.ContentInset
	offset.X = min(max(offset.X, -in.Left), max(content.Width+in.Right-frame.Width, -in.Left))
	offset.Y = min(max(offset.Y, -in.Top), max(content.Height+in.Bottom-frame.Height, -in.Top))
	return offset
}

// KeepStart returns where a view at offset goes when its content inset
// changes from old to inset: one resting at its start stays at the start,
// showing the inset, as on NSH's other backends.
func KeepStart(old, inset driver.Edges, offset driver.Point) driver.Point {
	if offset.X <= -old.Left+0.5 {
		offset.X = -inset.Left
	}
	if offset.Y <= -old.Top+0.5 {
		offset.Y = -inset.Top
	}
	return offset
}

// Wheel follows the wheel turns on a snapping scroll view until they
// settle.
type Wheel struct {
	last  time.Time
	total driver.Points
}

// Turn records a turn moving the offset by d along the view's axis.
func (w *Wheel) Turn(now time.Time, d driver.Points) {
	if w.last.IsZero() {
		w.total = 0
	}
	w.last, w.total = now, w.total+d
}

// Turning reports whether turns came within WheelIdle of now.
func (w *Wheel) Turning() bool { return !w.last.IsZero() }

// Settled reports whether the turns stopped by now; it then forgets
// them, and returns the speed to snap with in the wheel's direction.
func (w *Wheel) Settled(now time.Time) (velocity float32, ok bool) {
	if w.last.IsZero() || now.Sub(w.last) < WheelIdle {
		return 0, false
	}
	w.last = time.Time{}
	switch {
	case w.total >= 1:
		return WheelSnap, true
	case w.total <= -1:
		return -WheelSnap, true
	}
	return 0, true
}

// Settle returns where a snapping or paging scroll view at offset comes
// to rest when the user lets it go at velocity (points per millisecond
// along its axis), and false when it does not snap.
func Settle(c *driver.ScrollConfig, frame, content driver.Size, offset driver.Point, velocity float32) (driver.Point, bool) {
	b := &c.Behavior
	if !b.Snaps() {
		return offset, false
	}
	at, viewport, length := offset.Y, frame.Height, content.Height+b.ContentInset.Bottom
	if c.Horizontal {
		at, viewport, length = offset.X, frame.Width, content.Width+b.ContentInset.Right
	}
	to, ok := b.SnapTarget(at, at, velocity, viewport, length)
	if !ok {
		return offset, false
	}
	if c.Horizontal {
		offset.X = to
	} else {
		offset.Y = to
	}
	return Clamp(c, frame, content, offset), true
}
