//go:build windows && nsgotest

package windows

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/testhost"
)

// The test host: nsgo test -target windows runs each test in a window of
// its own. The mouse is the window's own messages, posted as the system
// posts them; keys go in at the backend's key entry points, with the
// thread's keyboard state holding the modifiers, as GetKeyState reads
// them.

func init() { testhost.Register(host{}) }

type host struct{}

// Main runs the tests here: each window runs its message loop on a
// thread of its own.
func (host) Main(run func() int) { os.Exit(run()) }

func (host) Open(app func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	test := &testOpen{options: o, ready: make(chan *window, 1)}
	go func() {
		runtime.LockOSThread()
		Run(app, Options{Title: "NSGo test", Width: o.Size.Width, Height: o.Size.Height, test: test})
	}()
	select {
	case w := <-test.ready:
		return &testSurface{w: w, test: test}, nil
	case <-time.After(20 * time.Second):
		return nil, errors.New("the window did not open")
	}
}

type testSurface struct {
	w    *window
	test *testOpen
}

var (
	hostGetWindow         = user32.NewProc("GetWindow")
	hostSendMessage       = user32.NewProc("SendMessageW")
	hostGetKeyboardState  = user32.NewProc("GetKeyboardState")
	hostSetKeyboardState  = user32.NewProc("SetKeyboardState")
	hostGetDC             = user32.NewProc("GetDC")
	hostReleaseDC         = user32.NewProc("ReleaseDC")
	hostPrintWindow       = user32.NewProc("PrintWindow")
	hostCreateDC          = gdi32.NewProc("CreateCompatibleDC")
	hostCreateBitmap      = gdi32.NewProc("CreateCompatibleBitmap")
	hostDeleteDC          = gdi32.NewProc("DeleteDC")
	hostSelectObject      = gdi32.NewProc("SelectObject")
	hostGetDIBits         = gdi32.NewProc("GetDIBits")
	hostDeleteObject      = gdi32.NewProc("DeleteObject")
	hostClientToScreen    = user32.NewProc("ClientToScreen")
	hostGetClientRectProc = user32.NewProc("GetClientRect")
)

const (
	hostMouseLeave  = 0x02A3
	hostLButtonUp   = 0x0202
	hostRButtonUp   = 0x0205
	hostKeyUp       = 0x0101
	pwRenderContent = 1 | 2
)

// onMain runs fn on the window's thread and waits for it.
func (t *testSurface) onMain(fn func()) {
	done := make(chan struct{})
	t.w.post(func() {
		defer close(done)
		fn()
	})
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
}

// lParam is at, in client pixels, as mouse messages carry it.
func (t *testSurface) lParam(at driver.Point) uintptr {
	x, y := t.w.px(at.X), t.w.px(at.Y)
	return uintptr(uint32(uint16(y))<<16 | uint32(uint16(x)))
}

func (t *testSurface) post(message, wParam uintptr, at driver.Point) {
	postMessageW.Call(t.w.hwnd, message, wParam, t.lParam(at))
}

func (t *testSurface) Press(at driver.Point, hold time.Duration) error {
	t.post(wmMouseMove, 0, at)
	t.post(wmLButtonDown, mkLButton, at)
	time.Sleep(max(hold+50*time.Millisecond, 50*time.Millisecond))
	t.post(hostLButtonUp, 0, at)
	return nil
}

func (t *testSurface) RightClick(at driver.Point) error {
	t.post(wmMouseMove, 0, at)
	t.post(wmRButtonDown, mkRButton, at)
	time.Sleep(30 * time.Millisecond)
	t.post(hostRButtonUp, 0, at)
	return nil
}

func (t *testSurface) Drag(path []driver.Point) error {
	if len(path) == 0 {
		return errors.New("an empty drag")
	}
	t.post(wmMouseMove, 0, path[0])
	t.post(wmLButtonDown, mkLButton, path[0])
	time.Sleep(100 * time.Millisecond)
	at := path[0]
	for _, to := range path[1:] {
		dx, dy := to.X-at.X, to.Y-at.Y
		n := int(max(absPoints(dx), absPoints(dy))/10) + 1
		for i := 1; i <= n; i++ {
			f := driver.Points(i) / driver.Points(n)
			t.post(wmMouseMove, mkLButton, driver.Point{X: at.X + dx*f, Y: at.Y + dy*f})
			time.Sleep(16 * time.Millisecond)
		}
		at = to
	}
	t.post(hostLButtonUp, 0, at)
	return nil
}

func absPoints(v driver.Points) driver.Points {
	if v < 0 {
		return -v
	}
	return v
}

// Scroll turns the wheel, whose messages carry screen coordinates: a
// notch is 120.
func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	p := point{t.w.px(at.X), t.w.px(at.Y)}
	hostClientToScreen.Call(t.w.hwnd, uintptr(unsafe.Pointer(&p)))
	lParam := uintptr(uint32(uint16(p.Y))<<16 | uint32(uint16(p.X)))
	turn := func(message uintptr, d driver.Points) {
		n := int(absPoints(d)/20) + 1
		for range n {
			// The backend scrolls wheelStep a notch.
			units := int16(-d / driver.Points(n) * 120 / wheelStep)
			postMessageW.Call(t.w.hwnd, message, uintptr(uint32(uint16(units))<<16), lParam)
			time.Sleep(16 * time.Millisecond)
		}
	}
	if delta.Y != 0 {
		turn(wmMouseWheel, delta.Y)
	}
	if delta.X != 0 {
		turn(wmMouseHWheel, -delta.X)
	}
	return nil
}

func (t *testSurface) Hover(at driver.Point) error {
	if at.X < 0 || at.Y < 0 {
		postMessageW.Call(t.w.hwnd, hostMouseLeave, 0, 0)
		return nil
	}
	t.post(wmMouseMove, 0, at)
	return nil
}

func (t *testSurface) Type(text string) error {
	for _, r := range text {
		if r == '\n' {
			if err := t.Key("enter"); err != nil {
				return err
			}
			continue
		}
		for _, u := range utf16.Encode([]rune{r}) {
			postMessageW.Call(t.w.hwnd, wmChar, uintptr(u), 1)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

var (
	immSetCompositionStringW = imm32.NewProc("ImmSetCompositionStringW")
	immSetOpenStatus         = imm32.NewProc("ImmSetOpenStatus")
	immGetOpenStatus         = imm32.NewProc("ImmGetOpenStatus")
)

const scsSetStr = 0x0009 // SCS_SETSTR: GCS_COMPREAD | GCS_COMPSTR

// Compose has the window's IME compose text, through IMM as the IME's
// own composition goes: the IME answers with WM_IME_COMPOSITION. A
// machine whose input language has no IME composes nothing.
func (t *testSurface) Compose(text string) error {
	var err error
	t.onMain(func() {
		if t.w.focusedField() == nil {
			err = errors.New("no field has the keyboard")
			return
		}
		t.w.withContext(func(himc uintptr) {
			if text == "" {
				immNotifyIME.Call(himc, niCompositionStr, cpsCancel, 0)
				return
			}
			if open, _, _ := immGetOpenStatus.Call(himc); open == 0 {
				immSetOpenStatus.Call(himc, 1)
			}
			u := utf16.Encode([]rune(text))
			if ok, _, _ := immSetCompositionStringW.Call(himc, scsSetStr, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)*2), 0, 0); ok == 0 {
				err = testhost.ErrNoInputMethod
			}
		})
	})
	time.Sleep(30 * time.Millisecond)
	return err
}

// Commit composes text and has the IME complete it, which commits it.
func (t *testSurface) Commit(text string) error {
	if err := t.Compose(text); err != nil {
		return err
	}
	t.onMain(func() {
		t.w.withContext(func(himc uintptr) { immNotifyIME.Call(himc, niCompositionStr, cpsComplete, 0) })
	})
	time.Sleep(30 * time.Millisecond)
	return nil
}

// virtualKeys are the virtual-key codes of the keys tests name.
var virtualKeys = map[string]uintptr{
	"enter": vkReturn, "return": vkReturn, "escape": 0x1B, "backspace": 0x08, "tab": 0x09, "space": 0x20,
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28, "home": 0x24, "end": 0x23, "pageup": 0x21,
	"pagedown": 0x22, "delete": 0x2E,
}

// Key presses key with the modifiers held in the thread's keyboard
// state: the focused field edits as its keys would, a keymap binding
// fires, and else the window's own key handling takes it.
func (t *testSurface) Key(key string) error {
	parts := strings.Split(strings.ToLower(key), "+")
	name := parts[len(parts)-1]
	var ctrl, shift, alt, meta bool
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "mod", "ctrl", "control":
			ctrl = true
		case "shift":
			shift = true
		case "alt", "option":
			alt = true
		case "meta", "super", "win":
			meta = true
		default:
			return fmt.Errorf("unknown modifier %q in %q", m, key)
		}
	}
	vk, ok := virtualKeys[name]
	if !ok && len(name) == 1 {
		c := strings.ToUpper(name)[0]
		if c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			vk, ok = uintptr(c), true
		}
	}
	if !ok {
		return fmt.Errorf("unknown key %q", key)
	}
	t.onMain(func() {
		var saved, state [256]byte
		hostGetKeyboardState.Call(uintptr(unsafe.Pointer(&saved)))
		state = saved
		hold := func(v uintptr, on bool) {
			if on {
				state[v] |= 0x80
			}
		}
		hold(vkControl, ctrl)
		hold(vkShift, shift)
		hold(vkMenu, alt)
		hold(vkLWin, meta)
		hostSetKeyboardState.Call(uintptr(unsafe.Pointer(&state)))
		defer hostSetKeyboardState.Call(uintptr(unsafe.Pointer(&saved)))
		if w := t.w; w.focusedField() != nil && w.editKey(vk, ctrl, shift, alt) {
			return
		}
		if t.w.shortcut(vk, 1) {
			return
		}
		hostSendMessage.Call(t.w.hwnd, wmKeyDown, vk, 1)
		hostSendMessage.Call(t.w.hwnd, hostKeyUp, vk, 1<<31|1<<30|1)
	})
	return nil
}

// Back is Escape, twice in a field, whose first only leaves it.
func (t *testSurface) Back() error {
	var editing bool
	t.onMain(func() { editing = t.w.focusedField() != nil })
	if editing {
		t.Key("escape")
	}
	return t.Key("escape")
}

// Answer clicks the button at index of the alert the window draws, or
// presses Escape for -1, as a user would.
func (t *testSurface) Answer(id uint32, index int) error {
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		drawn := false
		t.onMain(func() {
			w := t.w
			if !w.alerting || w.alertShown.ID != id {
				err = fmt.Errorf("no alert %d shows", id)
				return
			}
			s := w.alert
			if s == nil || len(s.buttons) == 0 {
				return // not drawn yet
			}
			drawn = true
			if index < 0 {
				postMessageW.Call(w.hwnd, wmKeyDown, vkEscape, 1)
				return
			}
			if index >= len(s.buttons) {
				err = fmt.Errorf("alert %d has no button %d", id, index)
				return
			}
			r := s.buttons[index]
			x := int32(float64((r.left+r.right)/2) * w.scale)
			y := int32(float64((r.top+r.bottom)/2) * w.scale)
			at := uintptr(uint32(y)<<16 | uint32(x)&0xffff)
			postMessageW.Call(w.hwnd, wmLButtonDown, mkLButton, at)
			postMessageW.Call(w.hwnd, wmLButtonUp, 0, at)
		})
		if err != nil || drawn {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the alert %d is not drawn", id)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Screenshot copies the window's client area as it composed it.
func (t *testSurface) Screenshot() ([]byte, error) {
	var img *image.RGBA
	t.onMain(func() {
		hwnd := t.w.hwnd
		var client rect
		hostGetClientRectProc.Call(hwnd, uintptr(unsafe.Pointer(&client)))
		width, height := client.Right, client.Bottom
		if width <= 0 || height <= 0 {
			return
		}
		dc, _, _ := hostGetDC.Call(hwnd)
		defer hostReleaseDC.Call(hwnd, dc)
		mem, _, _ := hostCreateDC.Call(dc)
		defer hostDeleteDC.Call(mem)
		bmp, _, _ := hostCreateBitmap.Call(dc, uintptr(width), uintptr(height))
		defer hostDeleteObject.Call(bmp)
		old, _, _ := hostSelectObject.Call(mem, bmp)
		hostPrintWindow.Call(hwnd, mem, pwRenderContent)
		hostSelectObject.Call(mem, old)
		pix := make([]byte, int(width)*int(height)*4)
		type bitmapHeader struct {
			Size                         uint32
			Width, Height                int32
			Planes, BitCount             uint16
			Compression, SizeImage       uint32
			XPelsPerMeter, YPelsPerMeter int32
			ClrUsed, ClrImportant        uint32
			Colors                       [1]uint32
		}
		info := bitmapHeader{Width: width, Height: -height, Planes: 1, BitCount: 32}
		info.Size = uint32(unsafe.Sizeof(info)) - 4
		if n, _, _ := hostGetDIBits.Call(mem, bmp, 0, uintptr(height), uintptr(unsafe.Pointer(&pix[0])),
			uintptr(unsafe.Pointer(&info)), 0); n == 0 {
			return
		}
		img = image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
		for i := 0; i < len(pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = pix[i+2], pix[i+1], pix[i], 0xff
		}
	})
	if img == nil {
		return nil, errors.New("Windows could not copy the window's pixels")
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Animating counts transitions, layout animations and animated values
// that end.
func (t *testSurface) Sync() { t.w.engine.Sync() }

func (t *testSurface) Animating() bool {
	var busy bool
	t.onMain(func() {
		busy = t.w.animator.Ending()
		for _, v := range t.w.views {
			busy = busy || v != nil && (v.transition != nil || v.moving != nil || v.fading != nil)
		}
	})
	return busy
}

func (t *testSurface) Resize(size driver.Size) error {
	t.onMain(func() {
		const noMove, noZOrder = 0x0002, 0x0004
		frame := rect{Right: t.w.px(size.Width), Bottom: t.w.px(size.Height)}
		adjustWindowRect.Call(uintptr(unsafe.Pointer(&frame)), wsOverlappedWindow|wsClipChildren, 0)
		setWindowPos.Call(t.w.hwnd, 0, 0, 0, uintptr(frame.Right-frame.Left), uintptr(frame.Bottom-frame.Top), noMove|noZOrder)
	})
	return nil
}

func (t *testSurface) SetEnvironment(env driver.Environment) error {
	t.test.pin.Set(&env)
	t.w.engine.Repin()
	return nil
}

// ChooseMenu implements testhost.Surface: the menu bar's command, as
// WM_COMMAND brings it.
func (t *testSurface) ChooseMenu(id uint32) error {
	var err error
	t.onMain(func() {
		for i, r := range t.w.chrome.entries {
			if r.id == id {
				postMessageW.Call(t.w.hwnd, wmCommand, uintptr(i+1), 0)
				return
			}
		}
		err = fmt.Errorf("the menu bar has no item %d", id)
	})
	return err
}

// ChoosePopup implements testhost.Surface: the context menu tracking
// ends, and its item id is carried out after, as a click on it would.
func (t *testSurface) ChoosePopup(id uint32) error {
	deadline := time.Now().Add(3 * time.Second)
	for {
		tracking := false
		t.onMain(func() {
			if tracking = t.w.popupTracking; tracking {
				t.w.popupChoice = &id
				endMenu.Call()
			}
		})
		if tracking {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no context menu shows")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Several implements testhost.Host: a desktop shows several windows.
func (host) Several() bool { return true }
