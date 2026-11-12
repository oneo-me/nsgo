package main

import (
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

const (
	// axisLock is how far, in points, a touchpad's fingers move before
	// the scroll is taken as across or along.
	axisLock = 6
	// minSample is the shortest time between moves the fingers' speed is
	// measured over, and maxSpeed the fastest it is taken as, in points a
	// second: three cards' fling.
	minSample = 4 * time.Millisecond
	maxSpeed  = 3000
)

// swipe is a touchpad's two-finger scroll turning the ring, as a drag on
// the floor does: across, it turns the ring point for point; along, it is
// left alone. As the fingers lift the ring flings on with their speed,
// and the system's momentum after them goes unheard, the ring's spring
// carrying on instead.
type swipe struct {
	r *ring
	// live is while the fingers are down; across and along say which way
	// they were taken, neither until they have moved axisLock.
	live, across, along bool
	moved, lock         core.Point
	// speed is the fingers' smoothed speed in points a second, and last
	// when they last moved.
	speed float64
	last  time.Time
}

func (s *swipe) wheel(e core.WheelEvent) {
	if !e.Touchpad {
		return
	}
	switch e.Phase {
	case core.GestureBegan:
		*s = swipe{r: s.r, live: true, last: time.Now()}
		s.r.grab()
	case core.GestureChanged:
		if !s.live || s.along {
			return
		}
		if !s.across {
			s.lock.X += e.Delta.X
			s.lock.Y += e.Delta.Y
			if math.Hypot(float64(s.lock.X), float64(s.lock.Y)) < axisLock {
				return
			}
			if math.Abs(float64(s.lock.Y)) > math.Abs(float64(s.lock.X)) {
				s.along = true
				return
			}
			s.across = true
			e.Delta.X = s.lock.X
		}
		if now, dt := time.Now(), time.Since(s.last); dt >= minSample {
			s.speed = 0.6*s.speed + 0.4*float64(e.Delta.X)/dt.Seconds()
			s.last = now
		}
		s.moved.X += e.Delta.X
		at := s.r.base + float64(s.moved.X)*dragRate
		s.r.turn.Set(at)
		s.r.front.Set(facing(at))
	case core.GestureEnded, core.GestureCancelled:
		if !s.live {
			return
		}
		s.live = false
		if !s.across {
			return
		}
		speed := float64(e.Velocity.X)
		if speed == 0 && time.Since(s.last) < 80*time.Millisecond {
			speed = s.speed
		}
		speed = max(min(speed, maxSpeed), -maxSpeed)
		s.r.fling(s.r.base+float64(s.moved.X)*dragRate, speed*dragRate)
	}
}
