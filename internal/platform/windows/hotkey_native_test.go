//go:build windows

package windows

import (
	"runtime"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// heldBySomeone reports whether another hot key already holds key's combination: RegisterHotKey refuses a combination any window or thread on the desktop has registered.
func heldBySomeone(t *testing.T, key string) bool {
	t.Helper()
	mods, vk, ok := globalHotKey(key)
	if !ok {
		t.Fatalf("globalHotKey(%q) failed", key)
	}
	const probe = 0xBFFF // the last ID an application may use
	if r, _, _ := registerHotKey.Call(0, probe, uintptr(mods|modNoRepeat), vk); r != 0 {
		unregisterHotKey.Call(0, probe)
		return false
	}
	return true
}

// waitForHeld waits until key's combination is held, or free.
func waitForHeld(t *testing.T, key string, held bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for heldBySomeone(t, key) != held {
		if time.Now().After(deadline) {
			t.Fatalf("%s held = %v, want %v", key, !held, held)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestGlobalHotKeysReachTheApp runs an app the way nsgo.Run runs an app function, on a hidden root window, and registers global shortcuts through the core: the backend holds their combinations, WM_HOTKEY runs the shortcut, dropping one releases its combination while the other stays, and closing the app releases the rest.
func TestGlobalHotKeysReachTheApp(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	// The probes register hot keys for this thread, so they must stay on it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const kept, dropped = "ctrl+alt+shift+f23", "ctrl+alt+shift+f24"
	if heldBySomeone(t, kept) || heldBySomeone(t, dropped) {
		t.Skip("another app holds the test's shortcuts")
	}
	title := "nsgo-native-hotkeys"
	fired := make(chan string, 8)
	app := core.AppRoot(func(cx *core.Scope) {
		// The core numbers shortcuts from 1 in the order they register.
		temp := cx.Child()
		cx.RegisterHotKey(kept, func() {
			fired <- kept
			temp.Dispose()
		})
		temp.RegisterHotKey(dropped, func() { fired <- dropped })
	})
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 360, Height: 240, Hidden: true})
	}()
	hwnd := waitForWindow(t, title, done)
	closed := false
	t.Cleanup(func() {
		if closed {
			return
		}
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	waitForHeld(t, kept, true)
	waitForHeld(t, dropped, true)

	next := func() string {
		t.Helper()
		select {
		case key := <-fired:
			return key
		case <-time.After(5 * time.Second):
			t.Fatal("no shortcut ran")
			return ""
		}
	}
	postMessageW.Call(hwnd, wmHotKey, 2, 0)
	if got := next(); got != dropped {
		t.Fatalf("WM_HOTKEY 2 ran %q, want %q", got, dropped)
	}
	// The kept shortcut drops the other, which the backend unregisters.
	postMessageW.Call(hwnd, wmHotKey, 1, 0)
	if got := next(); got != kept {
		t.Fatalf("WM_HOTKEY 1 ran %q, want %q", got, kept)
	}
	waitForHeld(t, dropped, false)
	waitForHeld(t, kept, true)
	// An ID no longer registered runs nothing; messages arrive in order, so the next shortcut to run is the kept one.
	postMessageW.Call(hwnd, wmHotKey, 2, 0)
	postMessageW.Call(hwnd, wmHotKey, 1, 0)
	if got := next(); got != kept {
		t.Fatalf("after the drop the next shortcut was %q, want %q", got, kept)
	}

	postMessageW.Call(hwnd, wmClose, 0, 0)
	select {
	case <-done:
		closed = true
	case <-time.After(5 * time.Second):
		t.Fatal("the window did not close")
	}
	waitForHeld(t, kept, false)
}
