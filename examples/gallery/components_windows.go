//go:build windows

package main

import (
	"math"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/core"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")

	registerClassEx    = user32.NewProc("RegisterClassExW")
	createWindowEx     = user32.NewProc("CreateWindowExW")
	destroyWindow      = user32.NewProc("DestroyWindow")
	defWindowProc      = user32.NewProc("DefWindowProcW")
	sendMessage        = user32.NewProc("SendMessageW")
	moveWindow         = user32.NewProc("MoveWindow")
	getSysColorBrush   = user32.NewProc("GetSysColorBrush")
	getModuleHandle    = kernel32.NewProc("GetModuleHandleW")
	initCommonControls = comctl32.NewProc("InitCommonControlsEx")
)

const (
	wsChild          = 0x40000000
	wsVisible        = 0x10000000
	wsPopup          = 0x80000000
	wsClipChildren   = 0x02000000
	tbsNoTicks       = 0x0010
	iccBarClasses    = 0x0004
	colorWindow      = 5
	wmSize           = 0x0005
	wmHScroll        = 0x0114
	wmCtlColorStatic = 0x0138
	wmUser           = 0x0400
	tbmGetPos        = wmUser
	tbmSetPos        = wmUser + 5
	tbmSetRange      = wmUser + 6
	sliderMax        = 1000
)

// trackbar is a common controls trackbar, as a native component. It sits
// in a window of its own, the component's view, which paints its
// background and hears its moves (WM_HSCROLL), reporting them as "change"
// events.
type trackbar struct {
	host, bar uintptr
	cx        *core.NativeContext
	pos       uintptr // the position last set or reported
}

// wndClassEx is WNDCLASSEXW.
type wndClassEx struct {
	size, style            uint32
	proc                   uintptr
	clsExtra, wndExtra     int32
	instance, icon, cursor uintptr
	background             uintptr
	menuName, className    *uint16
	iconSm                 uintptr
}

var (
	trackbars = map[uintptr]*trackbar{} // by host
	hostClass *uint16                   // registered with the first slider
)

func init() {
	components[sliderComponent] = func(cx *core.NativeContext) core.NativeComponent {
		t := &trackbar{cx: cx}
		instance, _, _ := getModuleHandle.Call(0)
		if hostClass == nil {
			icc := struct{ size, classes uint32 }{8, iccBarClasses}
			initCommonControls.Call(uintptr(unsafe.Pointer(&icc)))
			hostClass, _ = syscall.UTF16PtrFromString("GallerySliderHost")
			brush, _, _ := getSysColorBrush.Call(colorWindow)
			wc := wndClassEx{proc: syscall.NewCallback(hostProc), instance: instance, background: brush,
				className: hostClass}
			wc.size = uint32(unsafe.Sizeof(wc))
			registerClassEx.Call(uintptr(unsafe.Pointer(&wc)))
		}
		// A hidden popup until the window makes it its child.
		t.host, _, _ = createWindowEx.Call(0, uintptr(unsafe.Pointer(hostClass)), 0, wsPopup|wsClipChildren,
			0, 0, 220, 28, 0, 0, instance, 0)
		if t.host != 0 {
			trackbars[t.host] = t
			class, _ := syscall.UTF16PtrFromString("msctls_trackbar32")
			t.bar, _, _ = createWindowEx.Call(0, uintptr(unsafe.Pointer(class)), 0, wsChild|wsVisible|tbsNoTicks,
				0, 0, 220, 28, t.host, 0, instance, 0)
			sendMessage.Call(t.bar, tbmSetRange, 1, sliderMax<<16)
		}
		cx.SetSize(core.Size{Width: 220, Height: 28})
		return t
	}
}

// hostProc fills the host with its trackbar, paints the trackbar's
// background and reports the positions the user moves it to.
func hostProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	t := trackbars[hwnd]
	switch {
	case t == nil:
	case msg == wmSize:
		moveWindow.Call(t.bar, 0, 0, lParam&0xffff, lParam>>16&0xffff, 1)
		return 0
	case msg == wmCtlColorStatic:
		brush, _, _ := getSysColorBrush.Call(colorWindow)
		return brush
	case msg == wmHScroll:
		if pos, _, _ := sendMessage.Call(t.bar, tbmGetPos, 0, 0); pos != t.pos {
			t.pos = pos
			t.cx.Emit("change", float64(pos)/sliderMax)
		}
		return 0
	}
	r, _, _ := defWindowProc.Call(hwnd, msg, wParam, lParam)
	return r
}

func (t *trackbar) View() any { return t.host }

func (t *trackbar) SetProp(name string, value any) {
	if v, ok := value.(float64); ok && name == "value" {
		t.setValue(v)
	}
}

func (t *trackbar) setValue(v float64) {
	t.pos = uintptr(math.Round(min(max(v, 0), 1) * sliderMax))
	sendMessage.Call(t.bar, tbmSetPos, 1, t.pos)
}

func (t *trackbar) Command(name string, args any) {
	if name == "reset" {
		t.setValue(0)
		t.cx.Emit("change", 0.0)
	}
}

// Close destroys the host and the trackbar in it.
func (t *trackbar) Close() {
	if t.host != 0 {
		delete(trackbars, t.host)
		destroyWindow.Call(t.host)
		t.host, t.bar = 0, 0
	}
}
