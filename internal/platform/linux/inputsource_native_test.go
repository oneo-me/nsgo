//go:build linux && !android

package linux

import (
	"os"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// TestInputSources selects an input source of the session's framework
// while a field has the focus and restores the one it replaced. A second
// enabled source to switch to is NSGO_TEST_INPUT_SOURCE's; without one, an
// unknown source changes nothing.
func TestInputSources(t *testing.T) {
	scopes := make(chan *core.Scope, 1)
	var field core.Ref
	app := func(c *core.Scope) core.Node {
		scopes <- c
		return core.Input{Ref: &field, TestID: "field"}
	}
	startTestWindow(t, app, Options{Title: "nsgo-input-sources", Width: 360, Height: 240})
	cx := <-scopes
	cx.Post(field.Focus)
	im := inputMethods()
	if im == nil {
		t.Skip("the session runs neither Fcitx 5 nor IBus")
	}
	var start string
	for deadline := time.Now().Add(5 * time.Second); start == "" && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
		start = im.current()
	}
	if start == "" {
		t.Skip("no input context has the focus")
	}
	waitFor := func(want string) {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); im.current() != want; {
			if time.Now().After(deadline) {
				t.Fatalf("input source %q, want %q", im.current(), want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	cx.Post(func() { cx.SelectInputSource("nsgo-no-such-source") })
	time.Sleep(300 * time.Millisecond)
	waitFor(start)
	cx.Post(cx.RestoreInputSource)
	other := os.Getenv("NSGO_TEST_INPUT_SOURCE")
	if other == "" || other == start {
		return
	}
	cx.Post(func() { cx.SelectInputSource(other) })
	waitFor(other)
	cx.Post(cx.RestoreInputSource)
	waitFor(start)
}

// TestLifecycle reports the app in use while its window shows, and in
// the background while it is hidden.
func TestLifecycle(t *testing.T) {
	scopes := make(chan *core.Scope, 1)
	app := func(c *core.Scope) core.Node {
		scopes <- c
		return core.View{}
	}
	w := startTestWindow(t, app, Options{Title: "nsgo-lifecycle", Width: 360, Height: 240})
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
	w.onLoop(t, func() { gtkWidgetSetVisible(w.win, 0) })
	waitFor(func(l core.Lifecycle) bool { return l == core.LifecycleBackground }, "background")
	w.onLoop(t, func() { gtkWindowPresent(w.win) })
	waitFor(func(l core.Lifecycle) bool { return l != core.LifecycleBackground }, "shown again")
}
