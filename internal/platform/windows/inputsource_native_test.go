//go:build windows

package windows

import (
	"testing"
	"time"
	"unsafe"

	"github.com/oneo-me/nsgo/core"
)

// TestInputSources selects another enabled keyboard layout and restores
// the one it replaced; an unknown or disabled one changes nothing.
func TestInputSources(t *testing.T) {
	scopes := make(chan *core.Scope, 1)
	hwnd := startWith(t, func(c *core.Scope) core.Node {
		scopes <- c
		return core.View{}
	}, Options{Title: "nsgo-input-sources|", Width: 320, Height: 200})
	cx := <-scopes
	var start string
	var others []string
	onMain(t, hwnd, func(w *window) {
		start = currentInputSource()
		n, _, _ := getKeyboardLayoutList.Call(0, 0)
		hkls := make([]uintptr, n)
		if n > 0 {
			getKeyboardLayoutList.Call(n, uintptr(unsafe.Pointer(&hkls[0])))
		}
		// Each enabled layout's ID, as it reads once active.
		for _, hkl := range hkls {
			if was, _, _ := activateKeyboardLayout.Call(hkl, 0); was != 0 {
				if id := currentInputSource(); id != start {
					others = append(others, id)
				}
				activateKeyboardLayout.Call(was, 0)
			}
		}
	})
	current := func() string {
		var id string
		onMain(t, hwnd, func(*window) { id = currentInputSource() })
		return id
	}
	waitFor := func(want string) {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); current() != want; {
			if time.Now().After(deadline) {
				t.Fatalf("layout %q, want %q", current(), want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	cx.Post(func() { cx.SelectInputSource("0000ffff") })
	time.Sleep(200 * time.Millisecond)
	waitFor(start)
	cx.Post(cx.RestoreInputSource)
	if len(others) == 0 {
		t.Logf("only %s is enabled: nothing to switch to", start)
		return
	}
	cx.Post(func() { cx.SelectInputSource(others[0]) })
	waitFor(others[0])
	cx.Post(cx.RestoreInputSource)
	waitFor(start)
}

// TestLifecycle reports the app active or inactive while its window shows,
// and in the background while it is minimized.
func TestLifecycle(t *testing.T) {
	scopes := make(chan *core.Scope, 1)
	hwnd := startWith(t, func(c *core.Scope) core.Node {
		scopes <- c
		return core.View{}
	}, Options{Title: "nsgo-lifecycle|", Width: 320, Height: 200})
	cx := <-scopes
	state := func() core.Lifecycle {
		got := make(chan core.Lifecycle, 1)
		cx.Post(func() { got <- core.Untracked(cx, cx.Env().Lifecycle) })
		return <-got
	}
	waitFor := func(ok func(core.Lifecycle) bool, what string) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !ok(state()); {
			if time.Now().After(deadline) {
				t.Fatalf("lifecycle %v, want %s", state(), what)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	waitFor(func(l core.Lifecycle) bool { return l != core.LifecycleBackground }, "shown")
	onMain(t, hwnd, func(w *window) { showWindow.Call(w.hwnd, swMinimize) })
	waitFor(func(l core.Lifecycle) bool { return l == core.LifecycleBackground }, "background")
	onMain(t, hwnd, func(w *window) { showWindow.Call(w.hwnd, swRestore) })
	waitFor(func(l core.Lifecycle) bool { return l != core.LifecycleBackground }, "shown again")
}
