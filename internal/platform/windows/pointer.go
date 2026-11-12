//go:build windows

package windows

// The pointer on hosted content, as NSH's input.rs gives it to hosted
// items: the window finds the innermost view under the pointer, the one
// drawn on top there, and when it is a composed web view, the page takes
// the pointer as it comes (every button, the moves and the release while
// one is held, the wheel unless the page does not scroll); when it is a
// video's control bar, the bar takes presses and drags. Everything else
// goes on as before: presses and gestures for the engine.

import (
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	getMessageTime = user32.NewProc("GetMessageTime")
	getCursorPos   = user32.NewProc("GetCursorPos")
)

const (
	wmSetCursor    = 0x0020
	wmRButtonUp    = 0x0205
	wmMButtonDown  = 0x0207
	wmMButtonUp    = 0x0208
	wmXButtonDown  = 0x020B
	wmXButtonUp    = 0x020C
	wmMouseLeave   = 0x02A3
	htClientArea   = 1
	idcArrow       = 32512
	idcWait        = 32514
	idcCross       = 32515
	idcSizeNWSE    = 32642
	idcSizeNESW    = 32643
	idcSizeWE      = 32644
	idcSizeNS      = 32645
	idcSizeAll     = 32646
	idcNo          = 32648
	idcHand        = 32649
	idcAppStarting = 32650
	idcHelp        = 32651
	xButtonForward = 2
)

// pick returns the innermost view at p, in the coordinates of v's
// parent's content, drawn on top there; nil when p is outside v.
func pick(v *view, p driver.Point) *view {
	if v.style.Hidden || v.style.Opacity <= 0 {
		return nil
	}
	self, children := v.reach(p)
	if children {
		local := driver.Point{X: p.X - v.frame.Origin.X + v.offset.X, Y: p.Y - v.frame.Origin.Y + v.offset.Y}
		for i := len(v.children) - 1; i >= 0; i-- {
			if h := pick(v.children[i], local); h != nil {
				return h
			}
		}
	}
	if self {
		return v
	}
	return nil
}

// pickAt returns the view under client pixel x, y.
func (w *window) pickAt(x, y int32) *view {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	return pick(root, w.point(x, y))
}

// webAt returns the composed web view under client pixel x, y, and the
// point in its pixels.
func (w *window) webAt(x, y int32) (*webView, int32, int32) {
	v := w.pickAt(x, y)
	if v == nil || v.web == nil || !v.web.composed() || v.web.at == nil {
		return nil, 0, 0
	}
	px, py := v.web.local(x, y)
	return v.web, px, py
}

// local maps client pixel x, y to the page's pixels, where the last frame
// put it.
func (wv *webView) local(x, y int32) (int32, int32) {
	if wv.at == nil {
		return x, y
	}
	lx, ly := wv.at.m.invert().apply(float64(x), float64(y))
	return int32(lx + 0.5), int32(ly + 0.5)
}

// barAt returns the video whose control bar is under client pixel x, y,
// and the point in its view's points.
func (w *window) barAt(x, y int32) (*video, float32, float32) {
	v := w.pickAt(x, y)
	if v == nil || v.video == nil {
		return nil, 0, 0
	}
	b, _ := w.surfaceBounds(v)
	at := w.point(x, y)
	lx, ly := float32(at.X-b.Origin.X), float32(at.Y-b.Origin.Y)
	if !v.video.barAt(lx, ly, float32(v.frame.Size.Width), float32(v.frame.Size.Height)) {
		return nil, 0, 0
	}
	return v.video, lx, ly
}

// mouseButton returns the button of a mouse button message, 0 left, 1
// right, 2 middle, 3 back and 4 forward, and whether it went down.
func mouseButton(message uint32, wParam uintptr) (button int, down, ok bool) {
	switch message {
	case wmLButtonDown, wmLButtonUp:
		return 0, message == wmLButtonDown, true
	case wmRButtonDown, wmRButtonUp:
		return 1, message == wmRButtonDown, true
	case wmMButtonDown, wmMButtonUp:
		return 2, message == wmMButtonDown, true
	case wmXButtonDown, wmXButtonUp:
		button = 3
		if wParam>>16&0xffff == xButtonForward {
			button = 4
		}
		return button, message == wmXButtonDown, true
	}
	return 0, false, false
}

// hostedMouse gives a mouse message to the page or the control bar under
// it, or holding it, and reports whether one took it.
func (w *window) hostedMouse(message uint32, wParam, lParam uintptr) bool {
	x, y := int32(int16(lParam&0xffff)), int32(int16(lParam>>16&0xffff))
	time, _, _ := getMessageTime.Call()
	if message == wmMouseMove {
		return w.hostedMove(x, y, uint32(time))
	}
	button, down, ok := mouseButton(message, wParam)
	if !ok {
		return false
	}
	if wv := w.rawWeb; wv != nil {
		// Every button goes to the page while one is held on it.
		px, py := wv.local(x, y)
		wv.mouse(button, down, px, py, uint32(time))
		if wv.buttons == 0 {
			w.rawWeb = nil
			releaseCapture.Call()
		}
		return true
	}
	if vd := w.barVideo; vd != nil {
		if !down && button == 0 {
			w.barVideo = nil
			releaseCapture.Call()
			vd.released()
		}
		return true
	}
	if !down {
		return false
	}
	if wv, px, py := w.webAt(x, y); wv != nil {
		w.pressOutside(x, y)
		w.rawWeb = wv
		setCapture.Call(w.hwnd)
		wv.mouse(button, true, px, py, uint32(time))
		if wv.buttons == 0 {
			w.rawWeb = nil
			releaseCapture.Call()
		}
		return true
	}
	// A press on the window's own content takes the keyboard from a page.
	for wv := range w.webs {
		if wv.focused {
			wv.focusWindow()
		}
	}
	if button == 0 {
		if vd, lx, _ := w.barAt(x, y); vd != nil {
			w.barVideo = vd
			setCapture.Call(w.hwnd)
			vd.press(lx, float32(vd.v.frame.Size.Width))
			return true
		}
	}
	return false
}

// hostedMove follows the mouse to x, y: to the page holding it or under
// it, or along the scrubber being dragged. It reports whether a held page
// or bar took the move; hover moves go on to the window too.
func (w *window) hostedMove(x, y int32, time uint32) bool {
	if wv := w.rawWeb; wv != nil {
		px, py := wv.local(x, y)
		wv.mouse(-1, false, px, py, time)
		return true
	}
	if vd := w.barVideo; vd != nil {
		b, _ := w.surfaceBounds(vd.v)
		vd.drag(float32(w.point(x, y).X-b.Origin.X), float32(vd.v.frame.Size.Width))
		return true
	}
	wv, px, py := w.webAt(x, y)
	if w.hoverWeb != nil && w.hoverWeb != wv {
		w.hoverWeb.leave()
		w.hoverWeb = nil
	}
	if wv != nil {
		// pointerMoved asked to hear when the mouse leaves.
		w.hoverWeb = wv
		wv.mouse(-1, false, px, py, time)
	}
	return false
}

// mouseLeft follows the mouse out of the client area.
func (w *window) mouseLeft() {
	w.tracking = false
	w.pointerIn = false
	w.hoverViews()
	if wv := w.hoverWeb; wv != nil && w.rawWeb == nil {
		w.hoverWeb = nil
		wv.leave()
	}
	if w.barDrag == nil {
		w.hoverBars(driver.Point{X: -1, Y: -1})
	}
}

// captureLost lets the page or bar holding the mouse go when another
// window took it.
func (w *window) captureLost(to uintptr) {
	if to == w.hwnd {
		return
	}
	if wv := w.rawWeb; wv != nil {
		w.rawWeb = nil
		wv.leave()
	}
	if vd := w.barVideo; vd != nil {
		w.barVideo = nil
		vd.released()
	}
}

// hostedWheel gives a wheel turn at client pixel x, y to
// the page under it, reporting whether the page took it.
func (w *window) hostedWheel(x, y int32, delta int16, horizontal bool) bool {
	if w.rawWeb != nil {
		return false
	}
	wv, px, py := w.webAt(x, y)
	return wv != nil && wv.wheel(delta, horizontal, px, py)
}

// hostedPointer gives a touch or pen contact to the page it went down
// on, reporting whether the page took it; others become mouse messages.
func (w *window) hostedPointer(message uint32, wParam, lParam uintptr) bool {
	id := uint32(wParam & 0xffff)
	var kind uint32
	if r, _, _ := getPointerType.Call(uintptr(id), uintptr(unsafe.Pointer(&kind))); r == 0 ||
		kind != ptTouch && kind != ptPen {
		return false
	}
	at := point{int32(int16(lParam & 0xffff)), int32(int16(lParam >> 16 & 0xffff))}
	screenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
	wv := w.touchWebs[id]
	if wv == nil {
		if message != wmPointerDown {
			return false
		}
		if wv, _, _ = w.webAt(at.X, at.Y); wv == nil {
			return false
		}
	}
	px, py := wv.local(at.X, at.Y)
	if !wv.pointer(message, id, kind == ptTouch, px, py) {
		delete(w.touchWebs, id)
		return false
	}
	if message == wmPointerUp || message == wmPointerLeave {
		delete(w.touchWebs, id)
	} else {
		w.touchWebs[id] = wv
	}
	return true
}

// viewCursor shows the cursor of the nearest view under the pointer, or
// its ancestor's, and reports whether it set one.
func (w *window) viewCursor() bool {
	var at point
	if r, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&at))); r == 0 {
		return false
	}
	screenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
	v := w.pickAt(at.X, at.Y)
	for ; v != nil; v = v.parent {
		if c := v.style.Cursor; c != driver.CursorAuto {
			setCursor.Call(systemCursor(c))
			return true
		}
	}
	return false
}

// systemCursor loads and keeps the Windows cursor of c; 0 hides the
// pointer.
var cursors [driver.CursorNone + 1]uintptr

func systemCursor(c driver.Cursor) uintptr {
	if c == driver.CursorNone {
		return 0
	}
	id := uintptr(idcArrow)
	switch c {
	case driver.CursorPointer, driver.CursorGrab, driver.CursorGrabbing:
		id = idcHand
	case driver.CursorText:
		id = idcIBeam
	case driver.CursorWait:
		id = idcWait
	case driver.CursorProgress:
		id = idcAppStarting
	case driver.CursorHelp:
		id = idcHelp
	case driver.CursorCrosshair:
		id = idcCross
	case driver.CursorMove:
		id = idcSizeAll
	case driver.CursorNotAllowed:
		id = idcNo
	case driver.CursorColResize:
		id = idcSizeWE
	case driver.CursorRowResize:
		id = idcSizeNS
	case driver.CursorNwseResize:
		id = idcSizeNWSE
	case driver.CursorNeswResize:
		id = idcSizeNESW
	}
	if cursors[c] == 0 {
		h, _, _ := loadCursorW.Call(0, id)
		cursors[c] = h
	}
	return cursors[c]
}

// hostedCursor sets the cursor over the client area: the page's, or a
// hand over a control bar's button. It reports whether it set one.
func (w *window) hostedCursor() bool {
	var at point
	getCursorPos.Call(uintptr(unsafe.Pointer(&at)))
	screenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
	if wv, _, _ := w.webAt(at.X, at.Y); wv != nil && wv.cursor != 0 {
		setCursor.Call(wv.cursor)
		return true
	}
	if vd, lx, _ := w.barAt(at.X, at.Y); vd != nil && lx < videoButton {
		hand, _, _ := loadCursorW.Call(0, idcHand)
		setCursor.Call(hand)
		return true
	}
	return false
}

// forget drops what the window's pointer holds of wv, which closes.
func (w *window) forget(wv *webView) {
	if w.rawWeb == wv {
		w.rawWeb = nil
		releaseCapture.Call()
	}
	if w.hoverWeb == wv {
		w.hoverWeb = nil
	}
	for id, t := range w.touchWebs {
		if t == wv {
			delete(w.touchWebs, id)
		}
	}
}
