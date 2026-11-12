//go:build darwin && !ios

package macos

import (
	"math"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/platform/scrollbar"
)

// A scroll view whose MaxZoomScale is above its MinZoomScale zooms, as
// iOS's zoom view does. The container's bounds hold the zoom, so AppKit
// scales the content and hit tests it where it shows; a new zoom commits
// at once, while offsets alone move as other scroll views' do. The view
// moves along both axes: trackpads rubber-band past the edges unless
// NoBounces and the system's momentum follows, without the resampling of
// one axis. Trackpad pinches zoom around the pointer, past the limits by a
// rubber band unless NoBouncesZoom, and spring back.

// zoomEase is a zooming view's easing: the content point (unzoomed) at
// viewport point focus goes from from to to as the zoom goes from scale
// to target, the offset within the limits on the way when clamped.
type zoomEase struct {
	from, to, focus driver.Point
	scale, target   float64
	clamped         bool
}

// zooms reports whether the view zooms its content.
func (v *view) zooms() bool { return v.sc.config.Zooms() }

// zoomValue is the zoom events report: 0 for views that do not zoom.
func (v *view) zoomValue() float32 {
	if v.zooms() {
		return float32(v.sc.zoom)
	}
	return 0
}

// clampAt returns offset p within the view's limits at zoom z.
func (v *view) clampAt(p driver.Point, z float64) driver.Point {
	xlo, xhi := v.axisLimits(true, z)
	ylo, yhi := v.axisLimits(false, z)
	return driver.Point{X: min(max(p.X, xlo), xhi), Y: min(max(p.Y, ylo), yhi)}
}

// limitZoom keeps scale within the view's zoom scales, past them by a
// rubber band while pinching unless NoBouncesZoom.
func (v *view) limitZoom(scale float64, rubber bool) float64 {
	c := &v.sc.config
	lo, hi := float64(c.MinZoomScale), float64(c.MaxZoomScale)
	rubber = rubber && !c.NoBouncesZoom
	switch {
	case scale > hi && rubber:
		return hi * math.Pow(scale/hi, 0.3)
	case scale > hi:
		return hi
	case scale < lo && rubber:
		return lo * math.Pow(scale/lo, 0.3)
	case scale < lo:
		return lo
	}
	return scale
}

// center is the middle of the viewport.
func (v *view) center() driver.Point {
	return driver.Point{X: v.frame.Size.Width / 2, Y: v.frame.Size.Height / 2}
}

// contentAt returns the content point (unzoomed) at viewport point f.
func (v *view) contentAt(f driver.Point) driver.Point {
	s := v.sc
	z := driver.Points(s.zoom)
	return driver.Point{X: (s.at.X + f.X) / z, Y: (s.at.Y + f.Y) / z}
}

// offsetFor returns the offset that shows content point c at viewport
// point f at zoom z.
func offsetFor(c, f driver.Point, z float64) driver.Point {
	return driver.Point{X: c.X*driver.Points(z) - f.X, Y: c.Y*driver.Points(z) - f.Y}
}

// clampZoom keeps a resting view within its limits, as the content or
// the viewport changes.
func (v *view) clampZoom() {
	s := v.sc
	if s.phase.kind == atRest {
		v.moveZoom(v.clampAt(s.at, s.zoom), s.zoom)
	}
}

// zoomTo follows ItemZoomTo: the view zooms to r.Scale with content point
// r.Center, or the one in the middle now, in the middle of the viewport.
func (v *view) zoomTo(r *driver.ZoomRequest, animated bool) {
	if !v.zooms() {
		return
	}
	f := v.center()
	c := v.contentAt(f)
	if r.Center != nil {
		c = *r.Center
	}
	v.interrupt()
	v.easeZoomTo(c, f, v.limitZoom(float64(r.Scale), false), animated)
}

// panTo moves the view to offset at, within its limits, at its zoom.
func (v *view) panTo(at driver.Point, animated bool) {
	f := v.center()
	z := driver.Points(v.sc.zoom)
	v.easeZoomTo(driver.Point{X: (at.X + f.X) / z, Y: (at.Y + f.Y) / z}, f, v.sc.zoom, animated)
}

// settleZoom springs the view back within its zoom scales around
// viewport point f, and its offset within the limits.
func (v *view) settleZoom(f driver.Point) {
	v.easeZoomTo(v.contentAt(f), f, v.limitZoom(v.sc.zoom, false), true)
}

// easeZoomTo eases the view to zoom z with content point c at viewport
// point f, or as near as the limits let it.
func (v *view) easeZoomTo(c, f driver.Point, z float64, animated bool) {
	s := v.sc
	at := v.clampAt(offsetFor(c, f, z), z)
	if !animated || abs(at.X-s.at.X) < 0.5 && abs(at.Y-s.at.Y) < 0.5 && math.Abs(z-s.zoom) < 1e-3 {
		s.phase = scrollPhase{}
		v.moveZoom(at, z)
		v.scrollEnded()
		return
	}
	zf := driver.Points(z)
	s.phase = scrollPhase{kind: easing, start: mediaTime(), duration: scrollbar.Animation, settles: true,
		zoom: zoomEase{from: v.contentAt(f), to: driver.Point{X: (at.X + f.X) / zf, Y: (at.Y + f.Y) / zf}, focus: f,
			scale: s.zoom, target: z, clamped: v.clampAt(s.at, s.zoom) == s.at}}
	v.tickOn()
}

// easeZoom shows easing e at progress k.
func (v *view) easeZoom(e *zoomEase, k float64) {
	z := e.scale + (e.target-e.scale)*k
	kp := driver.Points(k)
	c := driver.Point{X: e.from.X + (e.to.X-e.from.X)*kp, Y: e.from.Y + (e.to.Y-e.from.Y)*kp}
	at := offsetFor(c, e.focus, z)
	if e.clamped {
		at = v.clampAt(at, z)
	}
	v.moveZoom(at, z)
}

// zoomCanScroll is canScroll's for a zooming view: it is not at the edge
// along either axis the deltas go.
func (v *view) zoomCanScroll(dx, dy driver.Points) bool {
	at, z := v.sc.at, v.sc.zoom
	xlo, xhi := v.axisLimits(true, z)
	ylo, yhi := v.axisLimits(false, z)
	return dx > 0 && at.X < xhi-0.5 || dx < 0 && at.X > xlo+0.5 ||
		dy > 0 && at.Y < yhi-0.5 || dy < 0 && at.Y > ylo+0.5
}

// zoomWheelBy is wheelBy's for a zooming view.
func (v *view) zoomWheelBy(dx, dy driver.Points) {
	s := v.sc
	p := &s.phase
	to := v.zoomWheelTo(dx, dy)
	if p.gesture {
		// Trackpads stretch past the edges, as macOS's do.
		p.rawAt.X, p.rawAt.Y = p.rawAt.X+dx, p.rawAt.Y+dy
		bounces := !s.config.Behavior.NoBounces
		xlo, xhi := v.axisLimits(true, s.zoom)
		ylo, yhi := v.axisLimits(false, s.zoom)
		to = driver.Point{X: scrollbar.Dragged(p.rawAt.X, xlo, xhi, v.frame.Size.Width, bounces),
			Y: scrollbar.Dragged(p.rawAt.Y, ylo, yhi, v.frame.Size.Height, bounces)}
	}
	p.last = mediaTime()
	v.moveZoom(to, s.zoom)
	v.tickOn()
}

// zoomWheelTo is where wheel deltas (dx, dy) move a zooming view: past an
// edge, only back.
func (v *view) zoomWheelTo(dx, dy driver.Points) driver.Point {
	at, z := v.sc.at, v.sc.zoom
	xlo, xhi := v.axisLimits(true, z)
	ylo, yhi := v.axisLimits(false, z)
	return driver.Point{X: min(max(at.X+dx, min(xlo, at.X)), max(xhi, at.X)),
		Y: min(max(at.Y+dy, min(ylo, at.Y)), max(yhi, at.Y))}
}

// zoomWheelEnd is wheelEnd's for a zooming view: it springs back, or
// waits for the system's momentum after a gesture.
func (v *view) zoomWheelEnd(gesture bool) {
	s := v.sc
	if gesture && v.clampAt(s.at, s.zoom) == s.at {
		s.phase = scrollPhase{kind: awaitingMomentum, last: mediaTime()}
		v.tickOn()
		return
	}
	v.settleZoom(v.center())
}

// pinchZoomView returns the zooming scroll view a trackpad pinch over
// native zooms: the nearest around it, unless a view nearer follows
// pinches itself.
func pinchZoomView(native objc.ID) *view {
	for n := native; n != 0; n = n.Send(selSuperview) {
		v := views[n]
		if v == nil || v.native != n {
			continue
		}
		if v.events.Has(driver.EventPinch) {
			return nil
		}
		if v.sc != nil && v.zooms() && !v.sc.config.NoPinchZoom {
			return v
		}
	}
	return nil
}

// pinchBy follows a trackpad pinch's event, magnified by m since the
// last: the view zooms around the pointer.
func (v *view) pinchBy(event objc.ID, m float64) {
	s := v.sc
	phase := objc.Send[uint](event, sel("phase"))
	if s.phase.kind != pinching {
		if phase&(phaseBegan|phaseChanged) == 0 {
			return
		}
		v.interrupt()
		s.phase = scrollPhase{kind: pinching, rawZoom: s.zoom}
	}
	p := objc.Send[apple.CGPoint](v.native, selConvertPoint, objc.Send[apple.CGPoint](event, selLocationInWindow), objc.ID(0))
	f := driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}
	c := v.contentAt(f)
	s.phase.rawZoom *= max(1+m, 0.01)
	z := v.limitZoom(s.phase.rawZoom, true)
	v.moveZoom(v.clampAt(offsetFor(c, f, z), z), z)
	if phase&(phaseEnded|phaseCancelled) != 0 {
		s.phase = scrollPhase{}
		v.settleZoom(f)
	}
}
