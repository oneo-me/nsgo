//go:build darwin && !ios

package main

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
)

// The macOS host is AppKit through purego: an NSWindow whose content view
// holds a status label, a button and the two NSGo views, run by
// NSApplication's own loop.

func init() {
	// AppKit runs on the main thread, which main starts on.
	runtime.LockOSThread()
}

var sel = objc.RegisterName

type cgPoint struct{ X, Y float64 }
type cgSize struct{ Width, Height float64 }
type cgRect struct {
	Origin cgPoint
	Size   cgSize
}

func rect(x, y, w, h float64) cgRect { return cgRect{cgPoint{x, y}, cgSize{w, h}} }

func nsString(s string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), s+"\x00")
}

var (
	// actions are the host's target-action callbacks, by sender.
	actions = map[objc.ID]func(){}
	target  objc.ID
	// queued runs on the main thread through dispatch_async_f.
	dispatchAsyncF func(queue, ctx, fn uintptr)
	mainQueue      uintptr
)

func runHost() {
	if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		panic(err)
	}
	libdispatch, err := purego.Dlopen("/usr/lib/system/libdispatch.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&dispatchAsyncF, libdispatch, "dispatch_async_f")
	mainQueue, err = purego.Dlsym(libdispatch, "_dispatch_main_q")
	if err != nil {
		panic(err)
	}
	c, err := objc.RegisterClass("EmbedHostTarget", objc.GetClass("NSObject"), []*objc.Protocol{}, []objc.FieldDef{},
		[]objc.MethodDef{
			{Cmd: sel("act:"), Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) {
				if fn := actions[sender]; fn != nil {
					fn()
				}
			}},
			{Cmd: sel("windowWillClose:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
				objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication")).Send(sel("terminate:"), objc.ID(0))
			}},
		})
	if err != nil {
		panic(err)
	}
	target = objc.ID(c).Send(sel("new"))

	nsApp := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	nsApp.Send(sel("setActivationPolicy:"), 0)
	const titled, closable, miniaturizable, resizable = 1, 2, 4, 8
	window := objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(
		sel("initWithContentRect:styleMask:backing:defer:"), rect(0, 0, 900, 640),
		uint(titled|closable|miniaturizable|resizable), uint(2), false)
	window.Send(sel("setTitle:"), nsString("An app that embeds NSGo"))
	window.Send(sel("setDelegate:"), target)
	content := window.Send(sel("contentView"))

	const widthSizable, heightSizable, minYMargin = 2, 16, 8
	label := objc.ID(objc.GetClass("NSTextField")).Send(sel("labelWithString:"), nsString("The host's own views are above; NSGo's below."))
	label.Send(sel("setFrame:"), rect(16, 600, 600, 22))
	label.Send(sel("setAutoresizingMask:"), uint(widthSizable|minYMargin))
	content.Send(sel("addSubview:"), label)

	h := &host{
		status: func(s string) { label.Send(sel("setStringValue:"), nsString(s)) },
		onMain: func(fn func()) {
			queuedMu.Lock()
			queued = append(queued, fn)
			queuedMu.Unlock()
			dispatchAsyncF(mainQueue, 0, runQueuedCallback)
		},
	}
	button := objc.ID(objc.GetClass("NSButton")).Send(sel("buttonWithTitle:target:action:"),
		nsString("Send a message"), target, sel("act:"))
	button.Send(sel("setFrame:"), rect(740, 594, 144, 32))
	button.Send(sel("setAutoresizingMask:"), uint(1|minYMargin)) // minXMargin
	actions[button] = h.send
	content.Send(sel("addSubview:"), button)

	const viewMinX, viewMaxX = 1, 4
	for i, v := range h.newViews() {
		nv := objc.ID(v.Handle())
		nv.Send(sel("setFrame:"), rect(float64(i)*450, 0, 450, 580))
		mask := uint(heightSizable | viewMaxX)
		if i == 1 {
			mask = heightSizable | viewMinX
		}
		nv.Send(sel("setAutoresizingMask:"), mask)
		content.Send(sel("addSubview:"), nv)
	}
	window.Send(sel("center"))
	window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	nsApp.Send(sel("activateIgnoringOtherApps:"), true)
	nsApp.Send(sel("run"))
	h.close()
}

// queued are the functions onMain runs on the main thread.
var (
	queuedMu          sync.Mutex
	queued            []func()
	runQueuedCallback = purego.NewCallback(func(uintptr) {
		queuedMu.Lock()
		fns := queued
		queued = nil
		queuedMu.Unlock()
		for _, fn := range fns {
			fn()
		}
	})
)

// badge is the host's badge: an NSButton showing the app's count, which
// emits "tap" when clicked.
type badge struct {
	button objc.ID
	cx     *core.NativeContext
}

func newBadge(cx *core.NativeContext) core.NativeComponent {
	b := &badge{cx: cx}
	b.button = objc.ID(objc.GetClass("NSButton")).Send(sel("buttonWithTitle:target:action:"),
		nsString("Badge 0"), target, sel("act:")).Send(sel("retain"))
	actions[b.button] = func() { cx.Emit("tap", nil) }
	return b
}

func (b *badge) View() any { return uintptr(b.button) }

func (b *badge) SetProp(name string, value any) {
	if name == "count" {
		b.button.Send(sel("setTitle:"), nsString(fmt.Sprintf("Badge %v", value)))
	}
}

func (b *badge) Command(string, any) {}

func (b *badge) Close() {
	delete(actions, b.button)
	b.button.Send(sel("release"))
}
