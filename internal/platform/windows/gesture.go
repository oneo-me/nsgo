//go:build windows

package windows

import (
	"math"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Gestures. A drag with the mouse (or a finger, which Windows turns into
// mouse messages once its own pan gesture is off) is a pan of the
// innermost view under it that follows pans, once the view's
// PanActivation decides so; an active pan takes the press. Pinches and
// rotations come from a touch screen's WM_GESTURE, and pinches also from
// a precision touchpad, which sends them as wheel turns with Control
// held.

var (
	setCapture             = user32.NewProc("SetCapture")
	releaseCapture         = user32.NewProc("ReleaseCapture")
	setGestureConfig       = user32.NewProc("SetGestureConfig")
	getGestureInfo         = user32.NewProc("GetGestureInfo")
	closeGestureInfoHandle = user32.NewProc("CloseGestureInfoHandle")
)

const (
	wmMouseMove      = 0x0200
	wmGesture        = 0x0119
	wmCaptureChanged = 0x0215
	mkLButton        = 0x0001
	mkRButton        = 0x0002
	mkMButton        = 0x0010
	mkXButton1       = 0x0020
	mkXButton2       = 0x0040
	mkControl        = 0x0008

	gidZoom   = 3
	gidPan    = 4
	gidRotate = 5
	gfBegin   = 0x1
	gfEnd     = 0x4

	// pinchTimer ends a touchpad pinch once its wheel turns stop.
	pinchTimer      = 2
	pinchIdleMillis = 150
)

type gestureConfig struct{ ID, Want, Block uint32 }

// gestureInfo is GESTUREINFO on 64-bit Windows, the only Windows NSGo
// builds for.
type gestureInfo struct {
	Size      uint32
	Flags     uint32
	ID        uint32
	Target    uintptr
	Location  [2]int16
	Instance  uint32
	Sequence  uint32
	Arguments uint64
	Extra     uint32
}

// pan is a drag in progress on a view that follows pans.
type pan struct {
	v          *view
	start      driver.Point // where the pointer went down, in client points
	active     bool
	last       driver.Point // the last translation reported
	lastTime   time.Time
	velocity   driver.Point
	activation driver.PanActivation
}

// twoFinger is a pinch or a rotation in progress.
type twoFinger struct {
	v         *view
	kind      driver.EventKind
	value     float32
	velocity  float32
	lastTime  time.Time
	startDist float64 // a touch pinch's distance between the fingers when it began
	// at is where a pinch is about, in client points.
	at driver.Point
}

// configureGestures asks Windows for pinches and rotations, and turns
// its pan off so a finger's drag comes as the mouse's.
func (w *window) configureGestures() {
	configs := []gestureConfig{
		{ID: gidZoom, Want: 1},
		{ID: gidRotate, Want: 1},
		{ID: gidPan, Block: 1},
	}
	setGestureConfig.Call(w.hwnd, 0, uintptr(len(configs)), uintptr(unsafe.Pointer(&configs[0])),
		unsafe.Sizeof(configs[0]))
}

// gestureView returns the innermost view at p (client points) that
// follows gestures of kind.
func (w *window) gestureView(p driver.Point, kind driver.EventKind) *view {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	return hit(root, p, func(v *view) bool { return v.events.Has(kind) })
}

// panDown starts following a drag at p, if a view there follows pans.
func (w *window) panDown(p driver.Point) {
	w.pan = nil
	v := w.gestureView(p, driver.EventPan)
	if v == nil {
		return
	}
	w.pan = &pan{v: v, start: p, lastTime: time.Now(), activation: v.props.PanActivation}
	setCapture.Call(w.hwnd)
}

// endPan stops following the drag; the window lets the mouse go.
func (w *window) endPan() {
	if w.pan == nil {
		return
	}
	w.pan = nil
	releaseCapture.Call()
}

// panMove follows the drag to p: it decides whether the pan activates,
// then reports its moves.
func (w *window) panMove(p driver.Point) {
	g := w.pan
	if g == nil {
		return
	}
	if g.v.deleted {
		w.endPan()
		return
	}
	d := driver.Point{X: p.X - g.start.X, Y: p.Y - g.start.Y}
	now := time.Now()
	if !g.active {
		switch g.activation.Decide(d) {
		case 1:
			g.active = true
			g.last, g.lastTime = d, now
			w.pressCancel() // the pan took the press
			w.gestured(driver.Event{Kind: driver.EventPan, ID: g.v.id, Phase: driver.GestureBegan, Point: d})
		case -1:
			w.endPan()
		}
		return
	}
	if dt := now.Sub(g.lastTime).Seconds(); dt > 0 {
		vx, vy := driver.Points(float64(d.X-g.last.X)/dt), driver.Points(float64(d.Y-g.last.Y)/dt)
		g.velocity = driver.Point{X: g.velocity.X*0.3 + vx*0.7, Y: g.velocity.Y*0.3 + vy*0.7}
	}
	g.last, g.lastTime = d, now
	w.gestured(driver.Event{Kind: driver.EventPan, ID: g.v.id, Phase: driver.GestureChanged, Point: d, Velocity: g.velocity})
}

// panUp ends the drag at p and reports whether an active pan took it,
// so it is no press.
func (w *window) panUp(p driver.Point) bool {
	g := w.pan
	if g == nil {
		return false
	}
	w.endPan()
	if !g.active || g.v.deleted {
		return false
	}
	d := driver.Point{X: p.X - g.start.X, Y: p.Y - g.start.Y}
	velocity := g.velocity
	if time.Since(g.lastTime) > 100*time.Millisecond {
		velocity = driver.Point{} // the pointer stopped before it let go
	}
	w.gestured(driver.Event{Kind: driver.EventPan, ID: g.v.id, Phase: driver.GestureEnded, Point: d, Velocity: velocity})
	return true
}

// panLost cancels the pan when something else took the mouse.
func (w *window) panLost() {
	g := w.pan
	if g == nil {
		return
	}
	w.pan = nil
	if g.active && !g.v.deleted {
		w.gestured(driver.Event{Kind: driver.EventPan, ID: g.v.id, Phase: driver.GestureCancelled, Point: g.last})
	}
}

// reportGesture sends the gesture's state in phase, with its velocity.
func (w *window) reportGesture(g *twoFinger, phase driver.GesturePhase, value float32) {
	now := time.Now()
	if phase == driver.GestureChanged {
		if dt := now.Sub(g.lastTime).Seconds(); dt > 0 {
			g.velocity = g.velocity*0.3 + float32(float64(value-g.value)/dt)*0.7
		}
	}
	g.value, g.lastTime = value, now
	velocity := g.velocity
	if phase == driver.GestureBegan {
		velocity = 0
	}
	ev := driver.Event{Kind: g.kind, ID: g.v.id, Phase: phase, Value: value, Velocity: driver.Point{X: driver.Points(velocity)}}
	if g.kind == driver.EventPinch {
		if b, ok := w.surfaceBounds(g.v); ok {
			ev.Point = driver.Point{X: g.at.X - b.Origin.X, Y: g.at.Y - b.Origin.Y}
		}
	}
	w.gestured(ev)
}

// gesture handles WM_GESTURE and reports whether it did.
func (w *window) gesture(lParam uintptr) bool {
	gi := gestureInfo{}
	gi.Size = uint32(unsafe.Sizeof(gi))
	if r, _, _ := getGestureInfo.Call(lParam, uintptr(unsafe.Pointer(&gi))); r == 0 {
		return false
	}
	var kind driver.EventKind
	switch gi.ID {
	case gidZoom:
		kind = driver.EventPinch
	case gidRotate:
		kind = driver.EventRotation
	default:
		return false
	}
	slot := &w.pinch
	if kind == driver.EventRotation {
		slot = &w.rotation
	}
	at := point{int32(gi.Location[0]), int32(gi.Location[1])}
	screenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
	if g := *slot; g != nil {
		g.at = w.point(at.X, at.Y)
	}
	if gi.Flags&gfBegin != 0 {
		*slot = nil
		if v := w.gestureView(w.point(at.X, at.Y), kind); v != nil {
			*slot = &twoFinger{v: v, kind: kind, startDist: float64(uint32(gi.Arguments)), at: w.point(at.X, at.Y)}
			w.reportGesture(*slot, driver.GestureBegan, rest(kind))
		}
	} else if g := *slot; g != nil && !g.v.deleted {
		value := g.value
		switch kind {
		case driver.EventPinch:
			if g.startDist > 0 {
				value = float32(float64(uint32(gi.Arguments)) / g.startDist)
			}
		case driver.EventRotation:
			// The angle since the rotation began, counterclockwise.
			angle := float64(uint32(gi.Arguments))/65535*4*math.Pi - 2*math.Pi
			value = float32(-angle)
		}
		if gi.Flags&gfEnd != 0 {
			w.reportGesture(g, driver.GestureEnded, value)
			*slot = nil
		} else {
			w.reportGesture(g, driver.GestureChanged, value)
		}
	}
	closeGestureInfoHandle.Call(lParam)
	return true
}

// rest is a gesture's value when it begins.
func rest(kind driver.EventKind) float32 {
	if kind == driver.EventPinch {
		return 1
	}
	return 0
}

// wheelPinch follows a precision touchpad's pinch, which comes as wheel
// turns with Control held, at p; it reports whether a view took it.
func (w *window) wheelPinch(p driver.Point, delta int16) bool {
	g := w.wheelPinching
	if g != nil && g.v.deleted {
		g = nil
	}
	if g == nil {
		v := w.gestureView(p, driver.EventPinch)
		if v == nil {
			return false
		}
		g = &twoFinger{v: v, kind: driver.EventPinch}
		w.wheelPinching = g
		g.at = p
		w.reportGesture(g, driver.GestureBegan, 1)
	}
	g.at = p
	// A notch of a mouse wheel scales by about a fifth.
	w.reportGesture(g, driver.GestureChanged, g.value*float32(math.Pow(2, float64(delta)/120/4)))
	setTimer.Call(w.hwnd, pinchTimer, pinchIdleMillis, 0)
	return true
}

// wheelPinchEnded ends the touchpad pinch once its turns stopped.
func (w *window) wheelPinchEnded() {
	killTimer.Call(w.hwnd, pinchTimer)
	if g := w.wheelPinching; g != nil {
		w.wheelPinching = nil
		if !g.v.deleted {
			w.reportGesture(g, driver.GestureEnded, g.value)
		}
	}
}
