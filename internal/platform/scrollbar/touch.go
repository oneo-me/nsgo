package scrollbar

import (
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Touch scrolling on the desktops that keep offsets of their own, as
// NSH's nsh_core::scroll drags them: a finger that moves past TouchSlop
// along a scroll view's axis drags its content, rubber-banding past the
// edges where the view bounces; let go, the view glides by its
// deceleration, bouncing at the edge it reaches, snaps, or springs back
// to its edge. Pulled far enough past its top, a refreshable view
// refreshes and keeps room above its content while it does. Mouse drags
// never scroll.

// TouchSlop is how far a finger moves before it drags a scroll view.
const TouchSlop driver.Points = 10

// glideStop is the speed, in points per millisecond, a glide stops at.
const glideStop = 0.01

// Along returns p's coordinate along the view's axis.
func Along(c *driver.ScrollConfig, p driver.Point) driver.Points {
	if c.Horizontal {
		return p.X
	}
	return p.Y
}

// SetAlong sets p's coordinate along the view's axis to v.
func SetAlong(c *driver.ScrollConfig, p *driver.Point, v driver.Points) {
	if c.Horizontal {
		p.X = v
	} else {
		p.Y = v
	}
}

// Limits returns the offsets along its axis a view of size frame showing
// content rests within: from its leading content inset to the end of the
// content and its trailing inset, with the refresh room above the content
// while it refreshes.
func Limits(c *driver.ScrollConfig, frame, content driver.Size, refreshing bool) (lo, hi driver.Points) {
	in := c.Behavior.ContentInset
	lo, hi = -in.Top, content.Height+in.Bottom-frame.Height
	if c.Horizontal {
		lo, hi = -in.Left, content.Width+in.Right-frame.Width
	}
	hi = max(hi, lo)
	if refreshing && !c.Horizontal {
		lo -= c.Behavior.RefreshInset()
	}
	return lo, hi
}

// Pulled returns how far offset is pulled past the top of a vertical
// view: what values bound to its pull follow.
func Pulled(c *driver.ScrollConfig, offset driver.Point) driver.Points {
	if c.Horizontal {
		return 0
	}
	return max(-c.Behavior.ContentInset.Top-offset.Y, 0)
}

// Refreshes reports whether a view let go at offset refreshes: it is
// refreshable, not refreshing yet, and pulled past its threshold.
func Refreshes(c *driver.ScrollConfig, offset driver.Point, refreshing bool) bool {
	return c.Refreshable && !c.Horizontal && !refreshing && Pulled(c, offset) >= c.Behavior.RefreshThreshold()
}

// TouchDrag follows a finger on a scroll view from where it went down.
type TouchDrag struct {
	down    driver.Point
	from    driver.Points // the offset along the axis at the down
	start   time.Time
	claimed bool
	speed   Velocity
}

// NewTouchDrag starts following a finger down at at on a view whose offset
// along its axis is from.
func NewTouchDrag(at driver.Point, from driver.Points, now time.Time) *TouchDrag {
	d := &TouchDrag{down: at, from: from, start: now}
	d.speed.Add(0, at)
	return d
}

// Claimed reports whether the drag took the finger.
func (d *TouchDrag) Claimed() bool { return d.claimed }

// Claims reports whether a finger now at at drags the view: moved past
// TouchSlop along its axis, and more along it than across. Once it does,
// the drag starts from where the finger is, so the content does not jump.
func (d *TouchDrag) Claims(c *driver.ScrollConfig, at driver.Point) bool {
	if d.claimed {
		return true
	}
	dx, dy := at.X-d.down.X, at.Y-d.down.Y
	along, across := dy, dx
	if c.Horizontal {
		along, across = dx, dy
	}
	if abs(along) < TouchSlop || abs(along) < abs(across) {
		return false
	}
	d.claimed = true
	d.down = at
	d.speed.Reset()
	return true
}

// Move records the finger at at and returns the offset it asks along the
// axis, before any rubber band.
func (d *TouchDrag) Move(c *driver.ScrollConfig, at driver.Point, now time.Time) driver.Points {
	d.speed.Add(now.Sub(d.start), at)
	return d.from - (Along(c, at) - Along(c, d.down))
}

// Velocity returns the offset's speed along the axis as the finger lifts,
// in points per millisecond: the finger's, reversed.
func (d *TouchDrag) Velocity(c *driver.ScrollConfig) float32 {
	x, y := d.speed.Speed()
	if c.Horizontal {
		return -x
	}
	return -y
}

// Motion is where a released view goes along its axis: an eased move to
// a point, or a glide that slows by the view's deceleration.
type Motion struct {
	from, to driver.Points
	start    time.Time
	duration time.Duration
	// A glide's speed at the start and what it keeps per millisecond,
	// within lo and hi. One that bounces hits edge after hits at hitSpeed.
	glide       bool
	speed, rate float32
	lo, hi      driver.Points
	stopsAfter  time.Duration
	bounces     bool
	edge        driver.Points
	hits        time.Duration
	hitSpeed    float32
}

// Release returns how a view of size frame showing content, let go at
// offset with velocity, comes to rest: back to its edge when past one,
// to its snap point, or gliding. lo and hi are Limits'.
func Release(c *driver.ScrollConfig, frame, content driver.Size, offset driver.Point, velocity float32, lo, hi driver.Points, now time.Time) Motion {
	at := Along(c, offset)
	m := Motion{from: at, start: now}
	b := &c.Behavior
	viewport, length := frame.Height, content.Height+b.ContentInset.Bottom
	if c.Horizontal {
		viewport, length = frame.Width, content.Width+b.ContentInset.Right
	}
	switch {
	case at < lo || at > hi:
		m.to = min(max(at, lo), hi)
		m.duration = Animation
	case b.Snaps():
		rate := float64(b.Deceleration())
		projected := at + driver.Points(float64(velocity)*rate/(1-rate))
		to, ok := b.SnapTarget(at, projected, velocity, viewport, length)
		if !ok {
			to = at
		}
		m.to = min(max(to, lo), hi)
		m.duration = SnapDuration(m.to-at, velocity)
	case abs(driver.Points(velocity)) > glideStop:
		m.glide, m.speed, m.rate, m.lo, m.hi = true, velocity, b.Deceleration(), lo, hi
		// The speed falls as rate^t: it is glideStop after that long.
		m.stopsAfter = time.Duration(math.Log(glideStop/math.Abs(float64(velocity)))/math.Log(float64(m.rate))) * time.Millisecond
		m.to = m.glideAt(m.stopsAfter)
		m.duration = m.stopsAfter
		if !b.NoBounces && (m.to < lo || m.to > hi) {
			m.bounceAt(min(max(m.to, lo), hi))
		}
	default:
		m.to = at
	}
	return m
}

// glideAt is where a glide is t after it started, unclamped.
func (m *Motion) glideAt(t time.Duration) driver.Points {
	ms := float64(t) / float64(time.Millisecond)
	ln := math.Log(float64(m.rate))
	return m.from + driver.Points(float64(m.speed)*(math.Pow(float64(m.rate), ms)-1)/ln)
}

// bounceAt makes the glide bounce at edge, which it reaches: where it
// gets there, rate^ms is 1 + (edge-from)·ln(rate)/speed, and so is the
// share of its speed left.
func (m *Motion) bounceAt(edge driver.Points) {
	ln := math.Log(float64(m.rate))
	left := 1 + float64(edge-m.from)*ln/float64(m.speed)
	m.bounces, m.edge = true, edge
	m.hits = time.Duration(math.Log(max(left, 1e-9)) / ln * float64(time.Millisecond))
	m.hitSpeed = m.speed * float32(left)
	m.duration = m.hits + 20*bounceTime
}

// Moves reports whether the motion goes anywhere.
func (m *Motion) Moves() bool { return m.duration > 0 && (m.to != m.from || m.glide) }

// At returns where the view is at now, and whether it came to rest. A
// glide bounces at the edge it reaches, or stops there when the view does
// not bounce.
func (m *Motion) At(now time.Time) (driver.Points, bool) {
	t := now.Sub(m.start)
	if m.bounces && t >= m.hits {
		over, back := Bounce(m.hitSpeed, t-m.hits)
		if back || t >= m.duration {
			return m.edge, true
		}
		return m.edge + over, false
	}
	if t >= m.duration {
		if m.glide {
			return min(max(m.to, m.lo), m.hi), true
		}
		return m.to, true
	}
	if m.glide {
		p := m.glideAt(t)
		if p <= m.lo || p >= m.hi {
			return min(max(p, m.lo), m.hi), !m.bounces
		}
		return p, false
	}
	return m.from + (m.to-m.from)*driver.Points(EaseOut(float64(t)/float64(m.duration))), false
}

func abs(p driver.Points) driver.Points {
	if p < 0 {
		return -p
	}
	return p
}
