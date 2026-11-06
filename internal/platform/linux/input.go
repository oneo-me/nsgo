//go:build linux && !android

package linux

import (
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Keyvals GTK uses for named keys.
const (
	gdkEscape    = 0xff1b
	gdkReturn    = 0xff0d
	gdkKPEnter   = 0xff8d
	gdkBackspace = 0xff08
	gdkTab       = 0xff09
	gdkDelete    = 0xffff
	gdkInsert    = 0xff63
	gdkHome      = 0xff50
	gdkEnd       = 0xff57
	gdkPageUp    = 0xff55
	gdkPageDown  = 0xff56
	gdkLeft      = 0xff51
	gdkUp        = 0xff52
	gdkRight     = 0xff53
	gdkDown      = 0xff54
	gdkSpace     = 0x020
	gdkF1        = 0xffbe
	gdkF35       = 0xffe0
	gdkKP0       = 0xffb0
	gdkKP9       = 0xffb9
)

// GDK modifier bits.
const (
	gdkShift   = 1 << 0
	gdkControl = 1 << 2
	gdkAlt     = 1 << 3
	gdkSuper   = 1 << 26
	gdkMeta    = 1 << 28
)

// modifiers returns the modifier keys among GDK's state bits.
func modifiers(state uint32) driver.Modifiers {
	return driver.Modifiers{Control: state&gdkControl != 0, Alt: state&gdkAlt != 0, Shift: state&gdkShift != 0,
		Platform: state&(gdkSuper|gdkMeta) != 0}
}

const (
	propagationPhaseCapture = 1
	propagationPhaseBubble  = 2
)

// pan is a drag in progress on a view that follows pans.
type pan struct {
	v          *view
	start      driver.Point
	active     bool
	failed     bool
	last       driver.Point
	lastTime   time.Time
	velocity   driver.Point
	activation driver.PanActivation
}

// twoFinger is a pinch or a rotation in progress.
type twoFinger struct {
	v        *view
	kind     driver.EventKind
	value    float32
	velocity float32
	lastTime time.Time
}

// Callbacks handed to GTK. Each finds its window or view through the
// pointer GTK was connected with.

func clickPressedCallback(gesture uintptr, n int32, x, y float64, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing {
		return
	}
	w.hideMenuBar()
	// Presses on a field, a page or a video's controls are theirs, even
	// where they bubble up here.
	if w.nativeAt(gtkWidgetPick(w.surface, x, y, gtkPickDefault)) {
		w.pressConsumed = true
		return
	}
	if w.embedded && w.focus == nil {
		// A host's widget takes the keys once pressed.
		gtkWidgetGrabFocus(w.surface)
	}
	w.pointerDown(driver.Point{X: driver.Points(x), Y: driver.Points(y)},
		gtkGestureSingleGetCurrentButton(gesture), gtkGestureGetDevice(gesture),
		gtkEventControllerGetCurrentEventState(gesture), int(n))
}

// pointerDown follows a press of button at surface point at, the clicks'
// press in a row, with modifiers state.
func (w *window) pointerDown(at driver.Point, button uint32, device uintptr, state uint32, clicks int) {
	w.lastPointer, w.pressConsumed = at, false
	w.down = driver.Event{Modifiers: modifiers(state), Count: uint32(max(clicks, 1)), Page: at}
	touch := button == 1 && isTouch(device)
	if touch {
		w.touchDown(at)
	} else {
		w.touching = nil
	}
	if button == 1 {
		w.touch(w.mouse.Down(w.listensTouches(), at))
	}
	// A press on the surface leaves the focused field or page.
	if w.focus != nil || w.focusedWeb() != nil {
		w.focusEntry(nil)
	}
	w.pressOutside(at)
	if button == 1 && w.pressBar(at) {
		w.pressConsumed = true
		w.panning = nil
		return
	}
	if button == 3 {
		w.contextMenu(at)
		w.pressConsumed = true
		return
	}
	if button == 1 && !touch && w.selectPress(at, w.down.Modifiers, clicks) {
		w.pressConsumed = true
		w.panning = nil
		return
	}
	if w.beginWindowDrag(device, at) {
		w.pressConsumed = true
	}
	w.panDown(at)
	if button == 1 && !w.pressConsumed {
		w.startLongPress(at)
	}
}

func clickReleasedCallback(gesture uintptr, n int32, x, y float64, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing {
		return
	}
	w.pointerUp(driver.Point{X: driver.Points(x), Y: driver.Points(y)})
}

// pointerUp follows the release of a press at surface point at.
func (w *window) pointerUp(at driver.Point) {
	w.selectUp()
	w.touch(w.mouse.Up(at))
	w.cancelLongPress()
	if w.pressConsumed || w.panning != nil && w.panning.active {
		return
	}
	w.press(at)
}

func motionCallback(controller uintptr, x, y float64, data uintptr) {
	w := windowOf(data)
	if w == nil {
		return
	}
	var state uint32
	if controller != 0 { // tests move the pointer without one
		state = gtkEventControllerGetCurrentEventState(controller)
	}
	w.movePointer(driver.Point{X: driver.Points(x), Y: driver.Points(y)}, state)
	if w.barDrag == nil {
		w.hoverBars(w.lastPointer)
	}
}

func leaveCallback(controller uintptr, data uintptr) {
	w := windowOf(data)
	if w == nil {
		return
	}
	w.pointerIn = false
	w.hoverViews()
	if w.barDrag == nil {
		w.hoverBars(driver.Point{X: -1, Y: -1})
	}
}

func dragBeginCallback(gesture uintptr, offsetX, offsetY float64, data uintptr) {
	w := windowOf(data)
	if w == nil || w.panning == nil || w.panning.failed {
		return
	}
	w.panning.lastTime = time.Now()
}

func dragUpdateCallback(gesture uintptr, offsetX, offsetY float64, data uintptr) {
	w := windowOf(data)
	if w != nil && math.Hypot(offsetX, offsetY) > longPressSlop {
		w.cancelLongPress()
	}
	if w != nil && w.selectDrag(driver.Point{X: w.down.Page.X + driver.Points(offsetX), Y: w.down.Page.Y + driver.Points(offsetY)}) {
		return
	}
	if w != nil && w.touchMove(driver.Point{X: w.down.Page.X + driver.Points(offsetX), Y: w.down.Page.Y + driver.Points(offsetY)}) {
		return
	}
	if w != nil && w.barDrag != nil {
		d := w.barDrag
		w.dragBar(driver.Point{X: d.from.X + driver.Points(offsetX), Y: d.from.Y + driver.Points(offsetY)})
		return
	}
	if w == nil || w.panning == nil || w.panning.failed {
		return
	}
	g := w.panning
	if g.v.deleted {
		w.panning = nil
		return
	}
	from := g.start
	now := driver.Point{X: from.X + driver.Points(offsetX), Y: from.Y + driver.Points(offsetY)}
	d := driver.Point{X: now.X - from.X, Y: now.Y - from.Y}
	if !g.active {
		if fails(g.activation, d) {
			g.failed = true
			w.panning = nil
			return
		}
		if !activates(g.activation, d) {
			return
		}
		g.active = true
		w.reportGesture(driver.Event{Kind: driver.EventPan, ID: g.v.id, Phase: driver.GestureBegan, Point: d})
	}
	dt := time.Since(g.lastTime).Seconds()
	var velocity driver.Point
	if dt > 0 {
		velocity = driver.Point{X: driver.Points(float64(now.X-g.last.X) / dt), Y: driver.Points(float64(now.Y-g.last.Y) / dt)}
	}
	g.last, g.lastTime, g.velocity = now, time.Now(), velocity
	w.reportGesture(driver.Event{Kind: driver.EventPan, ID: g.v.id, Phase: driver.GestureChanged, Point: d, Velocity: velocity})
}

func dragEndCallback(gesture uintptr, offsetX, offsetY float64, data uintptr) {
	w := windowOf(data)
	if w != nil && w.touchUp() {
		return
	}
	if w != nil && w.barDrag != nil {
		w.releaseBar()
		w.hoverBars(w.lastPointer)
		gtkWidgetQueueDraw(w.surface)
		return
	}
	if w == nil || w.panning == nil {
		return
	}
	g := w.panning
	w.panning = nil
	if g.failed || g.v.deleted {
		return
	}
	d := driver.Point{X: driver.Points(offsetX), Y: driver.Points(offsetY)}
	if !g.active && !activates(g.activation, d) {
		return
	}
	if !g.active {
		g.active = true
		w.reportGesture(driver.Event{Kind: driver.EventPan, ID: g.v.id, Phase: driver.GestureBegan, Point: d})
	}
	w.reportGesture(driver.Event{Kind: driver.EventPan, ID: g.v.id, Phase: driver.GestureEnded, Point: d, Velocity: g.velocity})
}

// longPressDelay is how long a press is held to be a long press, unless
// the view's DelayLongPress says else.
const longPressDelay = 500 * time.Millisecond

// longPressSlop is how far a press may move and still become a long
// press, in points, as GTK's drag threshold.
const longPressSlop = 8

// startLongPress waits for the press at surface point at to be held as
// long as the view there that takes long presses asks. A timer of NSGo's
// own rather than GTK's long press gesture, whose delay is not the view's.
func (w *window) startLongPress(at driver.Point) {
	w.longPress++
	v := w.hitEvent(at, driver.EventLongPress)
	if v == nil {
		return
	}
	delay := longPressDelay
	if d := v.props.DelayLongPress; d > 0 {
		delay = d
	}
	seq, ev := w.longPress, w.down
	ev.Kind, ev.ID, ev.Page = driver.EventLongPress, v.id, at
	if b, ok := w.surfaceBounds(v); ok {
		ev.Point = driver.Point{X: at.X - b.Origin.X, Y: at.Y - b.Origin.Y}
		// A long press on a text's span is the span's.
		if l, ok := v.text.(*textLayout); ok && v.kind == driver.KindText {
			ev.Index = l.indexAt(ev.Point.X-v.content.Origin.X, ev.Point.Y-v.content.Origin.Y)
		}
	}
	time.AfterFunc(delay, func() {
		w.post(func() {
			if w.longPress != seq || w.closing || v.deleted {
				return
			}
			w.longPress++
			w.pressConsumed = true // the release presses nothing
			w.engine.Dispatch(ev)
		})
	})
}

// cancelLongPress stops waiting for the press in progress to be long.
func (w *window) cancelLongPress() { w.longPress++ }

func zoomChangedCallback(gesture uintptr, scale float64, data uintptr) {
	w := windowOf(data)
	if w == nil {
		return
	}
	w.reportTwoFinger(driver.EventPinch, float32(scale))
}

func rotateChangedCallback(gesture uintptr, angle, delta float64, data uintptr) {
	w := windowOf(data)
	if w == nil {
		return
	}
	w.reportTwoFinger(driver.EventRotation, float32(delta*math.Pi/180))
}

func gestureEndCallback(gesture uintptr, sequence uintptr, data uintptr) {
	w := windowOf(data)
	if w == nil {
		return
	}
	for _, g := range []*twoFinger{w.pinching, w.rotating} {
		if g != nil && g.v != nil && !g.v.deleted {
			w.reportGesture(driver.Event{Kind: g.kind, ID: g.v.id, Phase: driver.GestureEnded, Value: g.value,
				Velocity: driver.Point{X: driver.Points(g.velocity)}})
		}
	}
	w.pinching, w.rotating = nil, nil
}

func scrollCallback(controller uintptr, dx, dy float64, data uintptr) int32 {
	w := windowOf(data)
	if w == nil {
		return 0
	}
	at := w.lastPointer
	if w.controlAt(gtkWidgetPick(w.surface, float64(at.X), float64(at.Y), gtkPickDefault)) != nil {
		return 0 // the page or the controls scroll themselves
	}
	w.wheelEvent(gtkEventControllerGetCurrentEventState(controller), at, dx, dy)
	return 1
}

// wheelEvent follows a wheel turn with modifiers state at surface point
// at: a pinch with Control held, else a scroll.
func (w *window) wheelEvent(state uint32, at driver.Point, dx, dy float64) {
	if state&gdkControl != 0 {
		w.wheelPinch(at, dy)
		return
	}
	if state&gdkShift != 0 {
		dx, dy = dy, 0
	}
	// GTK's wheel turns come as whole units; a touchpad's as points.
	notch := 48.0
	if math.Abs(dx) <= 3 && math.Abs(dy) <= 3 {
		dx, dy = dx*notch, dy*notch
	}
	w.wheel(at, dx, dy)
}

func keyPressedCallback(controller uintptr, keyval, keycode, state uint32, data uintptr) int32 {
	w := windowOf(data)
	if w == nil {
		return 0
	}
	// GTK reports no auto-repeats: a key pressed again before it was
	// released repeats.
	repeat := w.held[keycode]
	if w.held == nil {
		w.held = map[uint32]bool{}
	}
	w.held[keycode] = true
	w.menuBarKey(keyval, state, true)
	if w.selectKey(keyval, state) {
		return 1
	}
	if w.shortcut(keyval, state, repeat) {
		return 1
	}
	if state&(gdkControl|gdkAlt|gdkSuper|gdkMeta) == 0 && (w.focusedWeb() != nil || w.focus != nil && keyval != gdkEscape) {
		return 0 // the page's keys, Escape among them, and the field's
	}
	if keyval == gdkEscape {
		if w.focus != nil {
			w.focusEntry(nil)
		}
		w.back()
		return 1
	}
	return 0
}

// setupInputs adds GTK's event controllers to the surface and the
// window. Events on views bubble up to the surface, which hit tests them
// itself, as NSH's input does: GTK's picking knows nothing of rounded
// clips or hidden views.
func (w *window) setupInputs() {
	shield := gtkEventControllerLegacyNew()
	gtkEventControllerSetPropagationPhase(shield, propagationPhaseCapture)
	gSignalConnect(shield, "event", cbShield, w.handle)
	gtkWidgetAddController(w.surface, shield)

	click := gtkGestureClickNew()
	gtkGestureSingleSetButton(click, 0)
	gSignalConnect(click, "pressed", cbClickPressed, w.handle)
	gSignalConnect(click, "released", cbClickReleased, w.handle)
	gtkWidgetAddController(w.surface, click)

	motion := gtkEventControllerMotionNew()
	gSignalConnect(motion, "motion", cbMotion, w.handle)
	gSignalConnect(motion, "leave", cbLeave, w.handle)
	gtkWidgetAddController(w.surface, motion)

	drag := gtkGestureDragNew()
	gtkGestureSingleSetButton(drag, 1)
	gSignalConnect(drag, "drag-begin", cbDragBegin, w.handle)
	gSignalConnect(drag, "drag-update", cbDragUpdate, w.handle)
	gSignalConnect(drag, "drag-end", cbDragEnd, w.handle)
	gtkWidgetAddController(w.surface, drag)

	zoom := gtkGestureZoomNew()
	gSignalConnect(zoom, "scale-changed", cbZoomChanged, w.handle)
	gSignalConnect(zoom, "end", cbGestureEnd, w.handle)
	gtkWidgetAddController(w.surface, zoom)

	rotate := gtkGestureRotateNew()
	gSignalConnect(rotate, "angle-changed", cbRotateChanged, w.handle)
	gSignalConnect(rotate, "end", cbGestureEnd, w.handle)
	gtkWidgetAddController(w.surface, rotate)

	scroll := gtkEventControllerScrollNew(3) // both axes
	gSignalConnect(scroll, "scroll", cbScroll, w.handle)
	gtkWidgetAddController(w.surface, scroll)

	keys := gtkEventControllerKeyNew()
	gtkEventControllerSetPropagationPhase(keys, propagationPhaseCapture)
	gSignalConnect(keys, "key-pressed", cbKeyPressed, w.handle)
	gSignalConnect(keys, "key-released", cbKeyReleased, w.handle)
	gSignalConnect(keys, "modifiers", cbModifiers, w.handle)
	// A window of its own hears every key; a host's widget those pressed
	// while the focus is in it.
	if w.embedded {
		gtkWidgetAddController(w.surface, keys)
	} else {
		gtkWidgetAddController(w.win, keys)
	}
}

// hitEvent returns the innermost view under p that reports kind.
func (w *window) hitEvent(p driver.Point, kind driver.EventKind) *view {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	return hit(root, p, func(v *view) bool { return v.events.Has(kind) })
}

// panDown starts following a drag at p if a view there follows pans.
func (w *window) panDown(p driver.Point) {
	w.panning = nil
	v := w.hitEvent(p, driver.EventPan)
	if v == nil {
		return
	}
	w.panning = &pan{v: v, start: p, last: p, lastTime: time.Now(), activation: v.props.PanActivation}
}

// reportGesture moves the animated values bound to a pan, pinch or
// rotation, so they are drawn in the frame that shows the move, and
// reports it.
func (w *window) reportGesture(ev driver.Event) {
	if w.animator.Gesture(&ev) {
		w.showAnimated()
	}
	w.engine.Dispatch(ev)
}

// activates reports whether a drag of d has moved far enough to start a
// pan.
func activates(a driver.PanActivation, d driver.Point) bool {
	if a.ActiveOffsetX == 0 && a.ActiveOffsetY == 0 {
		return math.Hypot(float64(d.X), float64(d.Y)) >= float64(driver.DefaultPanSlop)
	}
	return a.ActiveOffsetX != 0 && math.Abs(float64(d.X)) >= float64(a.ActiveOffsetX) ||
		a.ActiveOffsetY != 0 && math.Abs(float64(d.Y)) >= float64(a.ActiveOffsetY)
}

// fails reports whether a drag of d crossed a fail offset.
func fails(a driver.PanActivation, d driver.Point) bool {
	return a.FailOffsetX != 0 && math.Abs(float64(d.X)) > float64(a.FailOffsetX) ||
		a.FailOffsetY != 0 && math.Abs(float64(d.Y)) > float64(a.FailOffsetY)
}

// reportTwoFinger reports a touchscreen pinch or rotation under the
// pointer, beginning it on the first change.
func (w *window) reportTwoFinger(kind driver.EventKind, value float32) {
	slot := &w.pinching
	if kind == driver.EventRotation {
		slot = &w.rotating
	}
	if *slot == nil {
		v := w.hitEvent(w.lastPointer, kind)
		if v == nil {
			return
		}
		*slot = &twoFinger{v: v, kind: kind, value: rest(kind), lastTime: time.Now()}
		w.reportGesture(driver.Event{Kind: kind, ID: v.id, Phase: driver.GestureBegan, Value: rest(kind)})
	}
	g := *slot
	if g.v.deleted {
		*slot = nil
		return
	}
	if kind == driver.EventRotation {
		value = g.value + value
	}
	now := time.Now()
	dt := now.Sub(g.lastTime).Seconds()
	var velocity float32
	if dt > 0 {
		velocity = (value - g.value) / float32(dt)
	}
	g.value, g.lastTime, g.velocity = value, now, velocity
	w.reportGesture(driver.Event{Kind: kind, ID: g.v.id, Phase: driver.GestureChanged, Value: value,
		Velocity: driver.Point{X: driver.Points(velocity)}})
}

// rest is a gesture's value when it begins.
func rest(kind driver.EventKind) float32 {
	if kind == driver.EventPinch {
		return 1
	}
	return 0
}

const (
	pinchTimer = 1
	// pinchIdleMillis ends a touchpad pinch once its wheel turns stop.
	pinchIdleMillis = 150
)

// wheelPinch follows a touchpad's pinch, which comes as wheel turns with
// Control held, at p.
func (w *window) wheelPinch(p driver.Point, delta float64) {
	g := w.pinching
	if g != nil && g.v.deleted {
		g = nil
	}
	if g == nil {
		v := w.hitEvent(p, driver.EventPinch)
		if v == nil {
			return
		}
		g = &twoFinger{v: v, kind: driver.EventPinch, value: 1, lastTime: time.Now()}
		w.pinching = g
		w.reportGesture(driver.Event{Kind: driver.EventPinch, ID: v.id, Phase: driver.GestureBegan, Value: 1})
	}
	g.value *= float32(math.Pow(2, -delta/20))
	w.reportGesture(driver.Event{Kind: driver.EventPinch, ID: g.v.id, Phase: driver.GestureChanged, Value: g.value})
	if w.pinchTimeout != 0 {
		gSourceRemove(w.pinchTimeout)
	}
	w.pinchTimeout = gTimeoutAdd(pinchIdleMillis, cbPinchTimeout, w.handle)
}

func pinchTimeoutCallback(data uintptr) int32 {
	w := windowOf(data)
	w.pinchTimeout = 0
	if g := w.pinching; g != nil {
		w.pinching = nil
		if !g.v.deleted {
			w.reportGesture(driver.Event{Kind: driver.EventPinch, ID: g.v.id, Phase: driver.GestureEnded, Value: g.value})
		}
	}
	return 0 // G_SOURCE_REMOVE
}

// beginWindowDrag starts moving the window when the press is in a window
// drag region and not on a nearer control.
func (w *window) beginWindowDrag(device uintptr, at driver.Point) bool {
	root := w.view(w.root)
	if root == nil {
		return false
	}
	drag := hit(root, at, func(v *view) bool { return v.props.WindowDrag != driver.WindowDragInherit })
	if drag == nil || drag.props.WindowDrag != driver.WindowDragOn {
		return false
	}
	if control := hit(root, at, func(v *view) bool {
		return v.events.Has(driver.EventPress) || v.events.Has(driver.EventPan) || v.kind == driver.KindInput || v.kind == driver.KindWeb
	}); control != nil && descendant(control, drag) {
		return false
	}
	if device == 0 {
		return false
	}
	surface := gtkNativeGetSurface(gtkWidgetGetNative(w.surface))
	if surface == 0 {
		return false
	}
	gdkToplevelBeginMove(surface, device, 1, float64(at.X), float64(at.Y), 0)
	return true
}

// descendant reports whether v is child or below.
func descendant(v, child *view) bool {
	for p := child; p != nil; p = p.parent {
		if p == v {
			return true
		}
	}
	return false
}

// shortcut reports a key press matching the app's keymap or the
// application menu's shortcuts, and whether it took it.
func (w *window) shortcut(keyval, state uint32, repeat bool) bool {
	name := keyName(keyval, state&gdkShift != 0)
	if name == "" {
		return false
	}
	editing := w.focus != nil || w.focusedWeb() != nil
	var names []string
	ctrl, alt := state&gdkControl != 0, state&gdkAlt != 0
	shift, meta := state&gdkShift != 0, state&(gdkSuper|gdkMeta) != 0
	for _, control := range []string{"mod", "ctrl"} {
		var parts []string
		if ctrl {
			parts = append(parts, control)
		}
		if alt {
			parts = append(parts, "alt")
		}
		if shift {
			parts = append(parts, "shift")
		}
		if meta {
			parts = append(parts, "meta")
		}
		names = append(names, strings.Join(append(parts, name), "+"))
	}
	if key, consumed := driver.FindKey(w.keys, names, editing, repeat); consumed {
		if key != "" {
			w.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: key})
		}
		return true
	}
	if editing && state&(gdkControl|gdkAlt|gdkSuper|gdkMeta) == 0 {
		return false // a field's or a page's own keys
	}
	for _, key := range names {
		if w.menuShortcut(key) {
			return true
		}
	}
	return false
}

func keyReleasedCallback(controller uintptr, keyval, keycode, state uint32, data uintptr) {
	if w := windowOf(data); w != nil {
		delete(w.held, keycode)
		w.menuBarKey(keyval, state, false)
	}
}

// modifiersCallback reports a modifier key pressed or released while the
// pointer rests over the window as a mouse move to where it is with the
// new modifiers, so a view hears Control come and go under a still mouse.
func modifiersCallback(controller uintptr, state uint32, data uintptr) int32 {
	w := windowOf(data)
	if w == nil || !w.pointerIn || state&gdkButtonsMask != 0 {
		return 0
	}
	w.mouseMoved(state)
	return 0
}

// keyName names a keyval as core.NormalizeKey does, or "" for a key it
// has no name for.
func keyName(keyval uint32, shifted bool) string {
	switch {
	case keyval == gdkBackspace:
		return "backspace"
	case keyval == gdkTab:
		return "tab"
	case keyval == gdkReturn || keyval == gdkKPEnter:
		return "enter"
	case keyval == gdkEscape:
		return "escape"
	case keyval == gdkSpace:
		return "space"
	case keyval == gdkDelete:
		return "delete"
	case keyval == gdkInsert:
		return "insert"
	case keyval == gdkHome:
		return "home"
	case keyval == gdkEnd:
		return "end"
	case keyval == gdkPageUp:
		return "pageup"
	case keyval == gdkPageDown:
		return "pagedown"
	case keyval == gdkLeft:
		return "left"
	case keyval == gdkUp:
		return "up"
	case keyval == gdkRight:
		return "right"
	case keyval == gdkDown:
		return "down"
	case keyval >= gdkF1 && keyval <= gdkF35:
		return "f" + itoa(int(keyval-gdkF1)+1)
	case keyval >= gdkKP0 && keyval <= gdkKP9:
		return itoa(int(keyval - gdkKP0))
	}
	r := rune(keyval)
	if unicode.IsPrint(r) && r != ' ' {
		if shifted {
			r = unshifted(r)
		}
		return string(unicode.ToLower(r))
	}
	return ""
}

// unshifted maps a shifted punctuation character back to the key that
// types it. GTK no longer exposes the keyboard layout's unshifted keys.
func unshifted(r rune) rune {
	if u := strings.IndexRune("~!@#$%^&*()_+{}|:\"<>?", r); u >= 0 {
		return []rune("`1234567890-=[]\\;',./")[u]
	}
	return r
}

// itoa is strconv.Itoa without the import at every call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// menuShortcut runs the menu item bound to key, if any.
func (w *window) menuShortcut(key string) bool {
	ref, ok := w.chrome.keys[key]
	if !ok || ref.notInText && (w.focus != nil || w.focusedWeb() != nil) {
		return false
	}
	return w.menuChosen(ref)
}

// menuChosen runs an application menu item's role, or reports its ID.
func (w *window) menuChosen(ref menuRef) bool {
	if ref.id != 0 {
		w.engine.Dispatch(driver.Event{Kind: driver.EventMenu, Count: ref.id})
		return true
	}
	switch ref.role {
	case driver.MenuQuit, driver.MenuClose:
		w.close()
	case driver.MenuMinimize:
		w.minimize()
	case driver.MenuZoom:
		w.toggleMaximize()
	case driver.MenuToggleFullScreen:
		if w.chrome.fullScreen {
			gtkWindowUnfullscreen(w.win)
		} else {
			gtkWindowFullscreen(w.win)
		}
		w.chrome.fullScreen = !w.chrome.fullScreen
		w.updateEnvironment()
	case driver.MenuUndo, driver.MenuRedo, driver.MenuCut, driver.MenuCopy, driver.MenuPaste, driver.MenuSelectAll:
		if w.focus != nil {
			action := map[driver.MenuRole]string{
				driver.MenuUndo: "text.undo", driver.MenuRedo: "text.redo",
				driver.MenuCut: "clipboard.cut", driver.MenuCopy: "clipboard.copy",
				driver.MenuPaste: "clipboard.paste", driver.MenuSelectAll: "selection.select-all",
			}[ref.role]
			gtkWidgetActivateAction(w.focus.entry, cstr(action), 0)
			return true
		}
		w.webEdit(ref.role)
	default:
		return false
	}
	return true
}

// GDK's event types and modifier bits the shield reads.
const (
	gdkMotionNotify  = 1
	gdkButtonPress   = 2
	gdkButtonRelease = 3
	gdkScrollEvent   = 15
	gdkScrollSmooth  = 4
	gdkButtonsMask   = 1<<8 | 1<<9 | 1<<10
	gdkButton1       = 1 << 8
	gdkButton2       = 1 << 9
	gdkButton3       = 1 << 10
)

// shieldCallback keeps from a hosted control (a page, a video's
// controls) the events GTK would give it where the app does not show it,
// as NSH's shields_page does: GTK picks widgets by their allocations and
// knows nothing of rounded clips, so a control a scroll view or a
// rounded parent cuts off would take clicks, hovers and wheels outside
// what is drawn of it. The view actually under the pointer gets them.
func shieldCallback(controller, event, data uintptr) int32 {
	w := windowOf(data)
	if w == nil || w.closing {
		return 0
	}
	return boolInt(w.shield(event))
}

func (w *window) shield(event uintptr) bool {
	typ := gdkEventGetEventType(event)
	if typ != gdkMotionNotify && typ != gdkButtonPress && typ != gdkButtonRelease && typ != gdkScrollEvent {
		return false
	}
	at, ok := w.eventPoint(event)
	if !ok {
		return false
	}
	if w.shielding {
		// The press went to the app; its moves and release follow.
		switch typ {
		case gdkButtonRelease:
			w.shielding = false
			w.pointerUp(at)
			return true
		case gdkMotionNotify:
			w.movePointer(at, gdkEventGetModifierState(event))
			return true
		}
	}
	if typ == gdkButtonRelease || typ == gdkMotionNotify && gdkEventGetModifierState(event)&gdkButtonsMask != 0 {
		return false // drags that began in the control stay with it
	}
	if !w.hostsControls() {
		return false
	}
	v := w.controlAt(gtkWidgetPick(w.surface, float64(at.X), float64(at.Y), gtkPickDefault))
	if v == nil {
		return false
	}
	if root := w.view(w.root); root != nil && hit(root, at, func(*view) bool { return true }) == v {
		return false
	}
	switch typ {
	case gdkMotionNotify:
		w.movePointer(at, gdkEventGetModifierState(event))
	case gdkButtonPress:
		w.shielding = true
		w.pointerDown(at, gdkButtonEventGetButton(event), gdkEventGetDevice(event), gdkEventGetModifierState(event), 1)
	case gdkScrollEvent:
		var dx, dy float64
		switch gdkScrollEventGetDirection(event) {
		case gdkScrollSmooth:
			gdkScrollEventGetDeltas(event, &dx, &dy)
		case 0:
			dy = -1
		case 1:
			dy = 1
		case 2:
			dx = -1
		case 3:
			dx = 1
		}
		w.lastPointer = at
		w.wheelEvent(gdkEventGetModifierState(event), at, dx, dy)
	}
	return true
}

// eventPoint returns where event happened in the surface's points.
func (w *window) eventPoint(event uintptr) (driver.Point, bool) {
	var x, y float64
	if gdkEventGetPosition(event, &x, &y) == 0 {
		return driver.Point{}, false
	}
	native := gtkWidgetGetNative(w.surface)
	if native == 0 {
		return driver.Point{}, false
	}
	var sx, sy float64
	gtkNativeGetSurfaceTransform(native, &sx, &sy)
	var out graphenePoint
	if gtkWidgetComputePoint(native, w.surface, &graphenePoint{float32(x - sx), float32(y - sy)}, &out) == 0 {
		return driver.Point{}, false
	}
	return driver.Point{X: driver.Points(out.x), Y: driver.Points(out.y)}, true
}
