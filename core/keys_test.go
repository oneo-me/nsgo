package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestKeyBindingsFireAsTheirOptionsSay(t *testing.T) {
	keymap := []driver.KeyBinding{
		{Key: "down", InText: !EditsText("down")},
		{Key: "mod+k", InText: !EditsText("mod+k")},
		{Key: "mod+z", InText: true, Repeat: true},
	}
	for _, c := range []struct {
		names           []string
		editing, repeat bool
		key             string
		consumed        bool
	}{
		{[]string{"down"}, false, false, "down", true},
		{[]string{"down"}, true, false, "", false}, // the field moves its caret
		{[]string{"mod+k", "meta+k"}, true, false, "mod+k", true},
		{[]string{"mod+k", "meta+k"}, false, true, "", true}, // held: consumed, fires once
		{[]string{"mod+z"}, true, true, "mod+z", true},
		{[]string{"x"}, false, false, "", false},
	} {
		key, consumed := driver.FindKey(keymap, c.names, c.editing, c.repeat)
		if key != c.key || consumed != c.consumed {
			t.Errorf("FindKey(%v, editing %v, repeat %v) = %q, %v; want %q, %v", c.names, c.editing, c.repeat,
				key, consumed, c.key, c.consumed)
		}
	}
}

func TestKeyLabelsFollowThePlatform(t *testing.T) {
	for _, c := range []struct {
		key   string
		os    OS
		apple bool
		want  string
	}{
		{"mod+shift+l", OSMacOS, true, "⇧⌘L"},
		{"mod+shift+l", OSWindows, false, "Ctrl+Shift+L"},
		{"ctrl+alt+escape", OSMacOS, true, "⌃⌥⎋"},
		{"meta+k", OSWindows, false, "Win+K"},
		{"meta+k", OSLinux, false, "Super+K"},
		{"f5", OSWeb, false, "F5"},
		{"mod+enter", OSWeb, true, "⌘↩"},
	} {
		if got := KeyLabel(c.key, c.os, c.apple); got != c.want {
			t.Errorf("KeyLabel(%q, %s) = %q, want %q", c.key, c.os, got, c.want)
		}
	}
}

func TestBackGoesToTheNewestHandlerThatTakesIt(t *testing.T) {
	var log []string
	var inner *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		inner = NewSignal(cx, true)
		cx.OnBackPress(func() bool { log = append(log, "outer"); return true })
		return Show{When: inner.Get, Then: func(cx *Scope) Node {
			cx.OnBackPress(func() bool { log = append(log, "inner passes"); return false })
			return View{}
		}}
	})
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	inner.Set(false)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventBack})
	if want := []string{"inner passes", "outer", "outer"}; !slices.Equal(log, want) {
		t.Fatalf("log = %q, want %q", log, want)
	}
}
