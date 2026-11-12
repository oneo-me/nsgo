//go:build windows

package main

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
)

// The Windows host is Win32 through syscall: a top-level window holding a
// static label, a button and the two NSGo views as child windows, run by
// its own message loop.

func init() {
	// The message loop and the windows stay on the thread that made them.
	runtime.LockOSThread()
}

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	ole32   = syscall.NewLazyDLL("ole32.dll")
	kernel  = syscall.NewLazyDLL("kernel32.dll")
	procs   = map[string]*syscall.LazyProc{}
	wndProc = syscall.NewCallback(hostProc)
)

func call(name string, args ...uintptr) uintptr {
	p := procs[name]
	if p == nil {
		p = user32.NewProc(name)
		procs[name] = p
	}
	r, _, _ := p.Call(args...)
	return r
}

func wide(s string) uintptr {
	p, _ := syscall.UTF16PtrFromString(s)
	return uintptr(unsafe.Pointer(p))
}

// wndClassEx is WNDCLASSEXW.
type wndClassEx struct {
	size, style            uint32
	proc                   uintptr
	clsExtra, wndExtra     int32
	instance, icon, cursor uintptr
	background             uintptr
	menuName, className    uintptr
	iconSm                 uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
	private uint32
}

const (
	wmDestroy  = 0x0002
	wmSize     = 0x0005
	wmCommand  = 0x0111
	wmRunQueue = 0x8000 + 64 // WM_APP + 64: onMain's functions are queued

	wsOverlappedWindow = 0x00CF0000
	wsChild            = 0x40000000
	wsVisible          = 0x10000000
	wsClipChildren     = 0x02000000
	bsPushButton       = 0
	cwUseDefault       = 0x80000000
	colorWindow        = 5
)

var (
	mainWindow uintptr
	label      uintptr
	button     uintptr
	views      []*nsgo.View
	onClick    = map[uintptr]func(){}
	queuedMu   sync.Mutex
	queued     []func()
)

func runHost() {
	ole32.NewProc("OleInitialize").Call(0)
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	cursor := call("LoadCursorW", 0, 32512)
	bg := call("GetSysColorBrush", colorWindow)
	for _, name := range []string{"EmbedHost", "EmbedBadge"} {
		wc := wndClassEx{proc: wndProc, instance: instance, cursor: cursor, background: bg, className: wide(name)}
		wc.size = uint32(unsafe.Sizeof(wc))
		call("RegisterClassExW", uintptr(unsafe.Pointer(&wc)))
	}
	mainWindow = call("CreateWindowExW", 0, wide("EmbedHost"), wide("An app that embeds NSGo"),
		wsOverlappedWindow|wsClipChildren, cwUseDefault, cwUseDefault, 960, 680, 0, 0, instance, 0)
	label = call("CreateWindowExW", 0, wide("STATIC"), wide("The host's own controls are above; NSGo's below."),
		wsChild|wsVisible, 0, 0, 0, 0, mainWindow, 0, instance, 0)
	button = call("CreateWindowExW", 0, wide("BUTTON"), wide("Send a message"), wsChild|wsVisible|bsPushButton,
		0, 0, 0, 0, mainWindow, 0, instance, 0)
	h := &host{
		status: func(s string) { call("SetWindowTextW", label, wide(s)) },
		onMain: func(fn func()) {
			queuedMu.Lock()
			queued = append(queued, fn)
			queuedMu.Unlock()
			call("PostMessageW", mainWindow, wmRunQueue, 0, 0)
		},
	}
	onClick[button] = h.send
	views = h.newViews(nsgo.Parent(mainWindow))
	layout()
	call("ShowWindow", mainWindow, 5)

	var m msg
	for int32(call("GetMessageW", uintptr(unsafe.Pointer(&m)), 0, 0, 0)) > 0 {
		call("TranslateMessage", uintptr(unsafe.Pointer(&m)))
		call("DispatchMessageW", uintptr(unsafe.Pointer(&m)))
	}
	h.close()
}

// layout puts the label and the button on top and the views side by
// side below, in the window's pixels.
func layout() {
	var r struct{ left, top, right, bottom int32 }
	call("GetClientRect", mainWindow, uintptr(unsafe.Pointer(&r)))
	dpi := int32(call("GetDpiForWindow", mainWindow))
	if dpi == 0 {
		dpi = 96
	}
	px := func(pt int32) int32 { return pt * dpi / 96 }
	bar := px(44)
	move := func(hwnd uintptr, x, y, w, h int32) {
		call("MoveWindow", hwnd, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
	}
	move(label, px(12), px(14), r.right-px(180), px(20))
	move(button, r.right-px(156), px(8), px(144), px(28))
	half := r.right / 2
	for i, v := range views {
		move(v.Handle(), int32(i)*half, bar, half, r.bottom-bar)
	}
}

func hostProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmSize:
		if hwnd == mainWindow {
			layout()
		} else if b := badges[hwnd]; b != nil {
			var r struct{ left, top, right, bottom int32 }
			call("GetClientRect", hwnd, uintptr(unsafe.Pointer(&r)))
			call("MoveWindow", b.button, 0, 0, uintptr(r.right), uintptr(r.bottom), 1)
		}
		return 0
	case wmCommand:
		const bnClicked = 0
		if wParam>>16 == bnClicked {
			if fn := onClick[lParam]; fn != nil {
				fn()
			}
		}
		return 0
	case wmRunQueue:
		queuedMu.Lock()
		fns := queued
		queued = nil
		queuedMu.Unlock()
		for _, fn := range fns {
			fn()
		}
		return 0
	case wmDestroy:
		if hwnd == mainWindow {
			call("PostQuitMessage", 0)
		}
		return 0
	}
	return call("DefWindowProcW", hwnd, uintptr(message), wParam, lParam)
}

// badge is the host's badge: a push button in a window of its own, which
// hears its clicks, showing the app's count and emitting "tap".
type badge struct{ window, button uintptr }

var badges = map[uintptr]*badge{} // by window

func newBadge(cx *core.NativeContext) core.NativeComponent {
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	b := &badge{}
	// NSGo makes the window a child of its view's.
	b.window = call("CreateWindowExW", 0, wide("EmbedBadge"), 0, wsChild|wsClipChildren, 0, 0, 120, 28, mainWindow, 0, instance, 0)
	b.button = call("CreateWindowExW", 0, wide("BUTTON"), wide("Badge 0"), wsChild|wsVisible|bsPushButton,
		0, 0, 120, 28, b.window, 0, instance, 0)
	badges[b.window] = b
	onClick[b.button] = func() { cx.Emit("tap", nil) }
	return b
}

func (b *badge) View() any { return b.window }

func (b *badge) SetProp(name string, value any) {
	if name == "count" {
		call("SetWindowTextW", b.button, wide(fmt.Sprintf("Badge %v", value)))
	}
}

func (b *badge) Command(string, any) {}

func (b *badge) Close() {
	delete(onClick, b.button)
	delete(badges, b.window)
	call("DestroyWindow", b.window)
}
