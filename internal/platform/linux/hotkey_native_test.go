//go:build linux && !android

package linux

import (
	"fmt"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestGlobalShortcutsReachTheBackend runs an app that registers a global shortcut in a real GTK window: the shortcut reaches X11's grabs, a grabbed press runs it, and so does the portal's activation of its session.
func TestGlobalShortcutsReachTheBackend(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") == "" {
		t.Skip("grabs keys on an X11 display")
	}
	title := "nsgo-hotkeys"
	app := func(cx *core.Scope) core.Node {
		fired := core.NewSignal(cx, 0)
		cx.RegisterHotKey("ctrl+alt+k", func() { fired.Update(func(n int) int { return n + 1 }) })
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d", title, fired.Get())) })
		return core.View{Style: core.Style{Background: driver.RGB(0xffffff)}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 200, Height: 100})
	waitForTitle(t, w, title+"|0")

	var combo x11HotKey
	var id uint32
	deadline := time.Now().Add(5 * time.Second)
	for id == 0 {
		w.onLoop(t, func() {
			if w.hotKeys != nil && w.hotKeys.x11 != nil {
				for c, i := range w.hotKeys.x11.bindings {
					combo, id = c, i
				}
			}
		})
		if id == 0 && time.Now().After(deadline) {
			t.Fatal("the app's shortcut did not reach X11's grabs")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if combo.modifiers != x11ControlMask|x11Mod1Mask {
		t.Errorf("the grab's modifiers are %#x, want Control and Mod1", combo.modifiers)
	}

	// A press as the server reports it, with NumLock on and the second layout group.
	var ev [24]uint64 // sizeof(XEvent) on LP64
	p := unsafe.Pointer(&ev[0])
	*(*int32)(p) = x11KeyPress
	*(*uint32)(unsafe.Add(p, 80)) = combo.modifiers | x11Mod2Mask | 1<<13
	*(*uint32)(unsafe.Add(p, 84)) = uint32(combo.keycode)
	w.onLoop(t, func() {
		if x11HotKeyEvent(0, uintptr(p), w.handle) == 0 {
			t.Error("GTK kept the grabbed press")
		}
	})
	waitForTitle(t, w, title+"|1")

	// The portal names the session in Activated's first argument, on its own object.
	session := globalShortcutPath + "/session/1_42/nsgo_create_1"
	activate := func(of string) {
		params := portalVariant("(osta{sv})", fmt.Sprintf("(objectpath %s, %s, uint64 0, @a{sv} {})", portalQuote(of), portalQuote("1")))
		if params == 0 {
			t.Error("the signal's arguments did not parse")
			return
		}
		defer gVariantUnref(params)
		globalHotKeyActivated(0, 0, uintptr(unsafe.Pointer(cstr(globalShortcutPath))), 0, 0, params, w.handle)
	}
	w.onLoop(t, func() {
		bindGlobalHotKeys()
		w.closeHotKeys()
		w.hotKeys = &globalHotKeys{session: session, ids: map[string]uint32{"1": id}}
		activate(globalShortcutPath + "/session/1_42/other")
		activate(session)
	})
	waitForTitle(t, w, title+"|2")
	time.Sleep(200 * time.Millisecond)
	if got := w.titleOf(t); got != title+"|2" {
		t.Errorf("another session's activation ran the shortcut: %q", got)
	}
}
