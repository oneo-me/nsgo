package scrollbar

import (
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The motion of scroll views that keep offsets of their own, as NSH's
// nsh_core::scroll moves them: the rubber band, eased scrolls and snaps,
// the indicator's geometry and fade, and the trackpad's events shown in
// step with the display. Lengths are points along the view's axis;
// velocities are points per millisecond, positive as the offset grows.

const (
	// Animation is how long animated scrolls and springs back take.
	Animation = 300 * time.Millisecond
	// IndicatorHold is how long the indicator stays after the last
	// scroll; it then fades out in IndicatorFade.
	IndicatorHold = 700 * time.Millisecond
	IndicatorFade = 300 * time.Millisecond
	// IndicatorMargin keeps the indicator off the ends of the viewport.
	IndicatorMargin driver.Points = 2
	// indicatorMin is the shortest indicator: short enough to see where it
	// is in a long list, long enough to see.
	indicatorMin driver.Points = 32
)

// Rubber returns how far content dragged over points past an edge of a
// viewport dim long moves: iOS's rubber band.
func Rubber(over, dim driver.Points) driver.Points {
	dim = max(dim, 1)
	return (1 - 1/(over*0.55/dim+1)) * dim
}

// Dragged returns where a gesture that took the offset to raw puts it,
// with lo and hi the offsets it rests within, in a viewport dim long:
// past them it rubber-bands when bounces, and stops otherwise.
func Dragged(raw, lo, hi, dim driver.Points, bounces bool) driver.Points {
	switch {
	case raw < lo && bounces:
		return lo - Rubber(lo-raw, dim)
	case raw < lo:
		return lo
	case raw > hi && bounces:
		return hi + Rubber(raw-hi, dim)
	case raw > hi:
		return hi
	}
	return raw
}

const (
	// bounceTime is the time constant of the bounce a glide makes as it
	// hits an edge: it overshoots most that long after, and is back within
	// about ten.
	bounceTime = 80 * time.Millisecond
	// bounceSpeed is the fastest a bounce starts, in points per
	// millisecond, so it overshoots at most about 90 points.
	bounceSpeed = 3
)

// Bounce returns how far past an edge content that hit it at velocity is
// t later, as macOS's and iOS's glides bounce: a critically damped spring
// that starts at that velocity and comes back to the edge. It reports
// whether the content is back.
func Bounce(velocity float32, t time.Duration) (driver.Points, bool) {
	v := min(max(float64(velocity), -bounceSpeed), bounceSpeed)
	k := max(float64(t), 0) / float64(bounceTime)
	over := v * float64(bounceTime/time.Millisecond) * k * math.Exp(-k)
	return driver.Points(over), k > 1 && math.Abs(over) < 0.5
}

// EaseOut returns the cubic ease out at t, from 0 to 1.
func EaseOut(t float64) float64 {
	t = min(max(t, 0), 1)
	return 1 - (1-t)*(1-t)*(1-t)
}

// SnapDuration returns how long an eased move by distance takes when it
// starts at velocity: ease out starts at three times its mean speed,
// which then matches the release, within 180 to 600 ms; a move against
// the release takes Animation. Nothing to move takes no time.
func SnapDuration(distance driver.Points, velocity float32) time.Duration {
	if math.Abs(float64(distance)) < 0.5 {
		return 0
	}
	if velocity*float32(distance) <= 0 {
		return Animation
	}
	ms := 3 * math.Abs(float64(distance)) / math.Abs(float64(velocity))
	return time.Duration(min(max(ms, 180), 600) * float64(time.Millisecond))
}

// IndicatorAlpha returns the indicator's opacity since it last showed.
func IndicatorAlpha(since time.Duration) float32 {
	if since < IndicatorHold {
		return 1
	}
	return float32(max(1-float64(since-IndicatorHold)/float64(IndicatorFade), 0))
}

// Indicator returns where the indicator starts along a viewport of that
// length showing total points from offset (0 at the start of total) and
// how long it is, or false when everything fits. Pulled past an edge, it
// shrinks, as on iOS.
func Indicator(viewport, total, offset driver.Points) (start, length driver.Points, ok bool) {
	if total <= viewport+0.5 || viewport <= 4*IndicatorMargin {
		return 0, 0, false
	}
	track := viewport - 2*IndicatorMargin
	end := total - viewport
	var over driver.Points
	if offset < 0 {
		over = -offset
	} else {
		over = max(offset-end, 0)
	}
	length = min(max(track*viewport/total-over, indicatorMin), track)
	t := min(max(offset/end, 0), 1)
	return IndicatorMargin + (track-length)*t, length, true
}

// revealMargin is the room kept around a view scrolled into view.
const revealMargin driver.Points = 16

// RevealDelta returns how far a scroll view moves along its axis so that
// the span from start to end, in its viewport, shows within the viewport
// with a margin; the start wins when the span does not fit.
func RevealDelta(start, end, viewport driver.Points) driver.Points {
	margin := min(revealMargin, viewport/4)
	switch {
	case start < margin:
		return start - margin
	case end > viewport-margin:
		return min(end-(viewport-margin), start-margin)
	}
	return 0
}

// Resampler shows where trackpad events put an offset in step with the
// display. Events come at about the display's rate but not in step with
// it: applied as they come, some frames move twice and some not at all,
// and a steady scroll judders. Each event records the offset at its
// time; each frame shows the path one event interval back, interpolated,
// so every frame moves by what that interval did.
type Resampler struct {
	points []sample
	// Interval is the time between events, smoothed.
	Interval time.Duration
}

type sample struct {
	at  time.Duration
	pos driver.Points
}

// Active reports whether events wait to be shown.
func (r *Resampler) Active() bool { return len(r.points) > 0 }

// Reset forgets the events.
func (r *Resampler) Reset() { r.points = r.points[:0] }

// Goal returns where the newest event put the offset.
func (r *Resampler) Goal() (driver.Points, bool) {
	if len(r.points) == 0 {
		return 0, false
	}
	return r.points[len(r.points)-1].pos, true
}

// Push records that an event at at put the offset at pos; shown is the
// offset showing, where the motion starts an interval before the first
// event.
func (r *Resampler) Push(at time.Duration, pos, shown driver.Points) {
	if r.Interval == 0 {
		r.Interval = time.Second / 120
	}
	n := len(r.points)
	switch {
	case n == 0:
		r.points = append(r.points, sample{at - r.Interval, shown}, sample{at, pos})
	case at <= r.points[n-1].at:
		// The same instant, or out of order: the newest wins.
		r.points[n-1].pos = pos
	default:
		// Gaps beyond a 60 Hz frame are pauses, not the rate.
		gap := min(max(at-r.points[n-1].at, 4*time.Millisecond), 17*time.Millisecond)
		r.Interval += (gap - r.Interval) / 4
		r.points = append(r.points, sample{at, pos})
	}
}

// Shift moves the recorded offsets by d, as the core moves the content.
func (r *Resampler) Shift(d driver.Points) {
	for i := range r.points {
		r.points[i].pos += d
	}
}

// Sample returns the offset at t, dropping the events before it, and
// whether t is past the newest event.
func (r *Resampler) Sample(t time.Duration) (driver.Points, bool) {
	drop := 0
	for len(r.points)-drop > 2 && r.points[drop+1].at <= t {
		drop++
	}
	if drop > 0 {
		r.points = r.points[:copy(r.points, r.points[drop:])]
	}
	switch len(r.points) {
	case 0:
		return 0, true
	case 1:
		return r.points[0].pos, true
	}
	p0, p1 := r.points[0], r.points[1]
	if t >= p1.at {
		return p1.pos, true
	}
	k := 1.0
	if p1.at > p0.at {
		k = min(max(float64(t-p0.at)/float64(p1.at-p0.at), 0), 1)
	}
	return p0.pos + (p1.pos-p0.pos)*driver.Points(k), false
}

// Velocity follows a pointer's positions for its speed over the last
// 100 ms.
type Velocity struct {
	samples [16]struct {
		at time.Duration
		p  driver.Point
	}
	count, head int
}

// velocityWindow is how far back Velocity looks.
const velocityWindow = 100 * time.Millisecond

// Reset forgets the positions.
func (v *Velocity) Reset() { v.count, v.head = 0, 0 }

// Add records that the pointer is at p at time at.
func (v *Velocity) Add(at time.Duration, p driver.Point) {
	v.samples[v.head].at, v.samples[v.head].p = at, p
	v.head = (v.head + 1) % len(v.samples)
	v.count = min(v.count+1, len(v.samples))
}

// Speed returns the pointer's velocity in points per millisecond, zero
// after a pause.
func (v *Velocity) Speed() (x, y float32) {
	if v.count < 2 {
		return 0, 0
	}
	n := len(v.samples)
	newest := v.samples[(v.head+n-1)%n]
	oldest := newest
	for i := 1; i < v.count; i++ {
		s := v.samples[(v.head+n-1-i)%n]
		if newest.at-s.at > velocityWindow {
			break
		}
		oldest = s
	}
	ms := float32(newest.at-oldest.at) / float32(time.Millisecond)
	if ms < 1 {
		return 0, 0
	}
	return float32(newest.p.X-oldest.p.X) / ms, float32(newest.p.Y-oldest.p.Y) / ms
}
