package flow

import (
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

const (
	minZoom      = .1
	maxZoom      = 2.5
	zoomStep     = 1.25
	fitPadding   = 32
	revealMargin = 16
	// wheelZoomDistance and pinchZoomDistance are the scroll distances in
	// points that zoom by ×1.5: a wheel's notches, and a touchpad's scroll
	// with Control or Command held.
	wheelZoomDistance = 120
	pinchZoomDistance = 48
)

// cameraSpring eases the camera toward where it goes in about a tenth of a
// second, keeping its speed when it is sent somewhere else.
var cameraSpring = core.SpringConfig{Stiffness: 666, Damping: 71, Mass: 1}

// camera shows the diagram's point p at x + p·zoom in the view.
type camera struct{ x, y, zoom float64 }

// moveCamera sends the camera to c, at once or easing there on the
// platform.
func (v *view) moveCamera(c camera, animated bool) {
	v.s.cam = c
	if v.cameraRun != nil {
		v.cameraRun.Stop()
	}
	if !animated || v.reduceMotion() {
		v.camX.Set(c.x)
		v.camY.Set(c.y)
		v.zoom.Set(c.zoom)
		return
	}
	v.cameraRun = v.cx.StartAnimation(core.Parallel(core.Spring(v.camX, c.x, cameraSpring),
		core.Spring(v.camY, c.y, cameraSpring), core.Spring(v.zoom, c.zoom, cameraSpring)), nil)
}

// resize keeps a diagram the user has not moved fitted to the view, and
// otherwise keeps the same point at the view's center.
func (v *view) resize(width, height float64) {
	if width <= 0 || height <= 0 || (width == v.width && height == v.height) {
		return
	}
	if v.width == 0 || !v.ready.GetUntracked() {
		v.width, v.height = width, height
		return
	}
	dx, dy := (width-v.width)/2, (height-v.height)/2
	v.width, v.height = width, height
	if !v.s.userMoved {
		v.fit(true)
		return
	}
	c := v.s.cam
	v.moveCamera(camera{c.x + dx, c.y + dy, c.zoom}, false)
}

func (v *view) fit(immediate bool) {
	if len(v.nodes) == 0 || v.width == 0 {
		return
	}
	w, h := max(1, v.result.width), max(1, v.result.height)
	zoom := clamp(min(max(1, v.width-2*fitPadding)/w, max(1, v.height-2*fitPadding)/h), minZoom, 1)
	v.s.userMoved = false
	v.moveCamera(camera{v.width/2 - w/2*zoom, v.height/2 - h/2*zoom, zoom}, !immediate)
}

// zoomAt scales the view around a point of the view, at once or easing
// there.
func (v *view) zoomAt(x, y, factor float64, animated bool) {
	from := v.s.cam
	zoom := clamp(from.zoom*factor, minZoom, maxZoom)
	gx, gy := (x-from.x)/from.zoom, (y-from.y)/from.zoom
	v.s.userMoved = true
	v.moveCamera(camera{x - gx*zoom, y - gy*zoom, zoom}, animated)
}

func (v *view) reveal(id string) {
	i, ok := v.index[id]
	if !ok || i >= len(v.result.x) || v.width == 0 {
		return
	}
	w, h := v.nodes[i].size()
	cx, cy := v.result.x[i]+w/2, v.result.y[i]+h/2
	c := v.s.cam
	v.s.userMoved = true
	v.moveCamera(camera{v.width/2 - cx*c.zoom, v.height/2 - cy*c.zoom, c.zoom}, true)
}

// revealIfHidden reveals a node that is not wholly in view.
func (v *view) revealIfHidden(id string) {
	i, ok := v.index[id]
	if !ok || i >= len(v.result.x) {
		return
	}
	w, h := v.nodes[i].size()
	c := v.s.cam
	x, y := v.result.x[i]*c.zoom+c.x, v.result.y[i]*c.zoom+c.y
	if x < revealMargin || y < revealMargin || x+w*c.zoom > v.width-revealMargin || y+h*c.zoom > v.height-revealMargin {
		v.reveal(id)
	}
}

// hit returns the node under a point of the view, or -1.
func (v *view) hit(p core.Point) int {
	c := v.s.cam
	if c.zoom <= 0 || !v.ready.GetUntracked() {
		return -1
	}
	gx, gy := (float64(p.X)-c.x)/c.zoom, (float64(p.Y)-c.y)/c.zoom
	for i := len(v.nodes) - 1; i >= 0; i-- {
		if i >= len(v.result.x) || !v.nodes[i].measured {
			continue
		}
		w, h := v.nodes[i].size()
		if x, y := v.result.x[i], v.result.y[i]; gx >= x && gx <= x+w && gy >= y && gy <= y+h {
			return i
		}
	}
	return -1
}

// pan moves the camera by a translation the view already shows, as a drag
// leaves it or a scroll moves it.
func (v *view) pan(dx, dy float64) {
	c := v.s.cam
	v.s.userMoved = true
	v.moveCamera(camera{c.x + dx, c.y + dy, c.zoom}, false)
}

// wheel pans and zooms: a touchpad's two fingers pan unless Control or
// Command is held, which zooms as a pinch does; a wheel zooms, except a
// sideways wheel, which pans across, or one with Shift held on macOS.
func (v *view) wheel(e core.WheelEvent) {
	if !v.ready.GetUntracked() {
		return
	}
	modified := e.Modifiers.Control || e.Modifiers.Platform
	switch {
	case e.Phase == core.GestureEnded || e.Phase == core.GestureCancelled:
		v.glide(float64(e.Velocity.X), float64(e.Velocity.Y))
	case e.Touchpad && !modified:
		v.pan(float64(e.Delta.X), float64(e.Delta.Y))
	case !e.Touchpad && !modified && e.Delta.Y == 0:
		v.pan(float64(e.Delta.X), 0)
	default:
		distance := float64(wheelZoomDistance)
		if e.Touchpad {
			distance = pinchZoomDistance
		}
		v.zoomAt(float64(e.Position.X), float64(e.Position.Y), math.Exp(float64(e.Delta.Y)*math.Log(1.5)/distance), false)
	}
}

// glide carries a pan let go at a velocity on, slowing as a scroll view's
// glide does.
func (v *view) glide(vx, vy float64) {
	if v.reduceMotion() || math.Hypot(vx, vy) < 1 {
		return
	}
	c := v.s.cam
	if v.cameraRun != nil {
		v.cameraRun.Stop()
	}
	x, y := core.DecayConfig{Velocity: vx}, core.DecayConfig{Velocity: vy}
	v.s.cam = camera{core.ProjectDecay(c.x, x), core.ProjectDecay(c.y, y), c.zoom}
	v.cameraRun = v.cx.StartAnimation(core.Parallel(core.Decay(v.camX, x), core.Decay(v.camY, y)), nil)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(v, hi)) }
