package core

import (
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Keyboard shortcuts: the surface's table of bindings goes to the
// platform, which matches each key press on the main thread, consumes
// the presses that match and reports them with EventKey.

type keyBinding struct {
	driver.KeyBinding
	owner *Scope
	fn    func()
}

// KeyWhen says when a binding fires.
type KeyWhen uint8

const (
	// KeyAuto fires it unless a text input or a web view has the
	// keyboard focus and uses the key itself (EditsText).
	KeyAuto KeyWhen = iota
	// KeyAlways fires it wherever the keyboard focus is.
	KeyAlways
	// KeyNotEditing fires it only while no text input or web view has
	// the keyboard focus.
	KeyNotEditing
)

// KeyOptions are how a binding fires.
type KeyOptions struct {
	When KeyWhen
	// Repeat fires it again on the auto-repeats of a held key.
	Repeat bool
}

// OnKey runs fn when key is pressed, while the current owner lives. key
// names modifiers and a key joined by "+": "mod+s" (Command on Apple
// platforms, Control elsewhere), "ctrl+shift+z", "alt+left", "escape",
// "enter", "f5", "a". The newest binding of a key wins. It fires once
// while the key is held, and not while a text input that uses the key
// has the keyboard focus; OnKeyWith says else.
func (cx *Scope) OnKey(key string, fn func()) { cx.OnKeyWith(key, KeyOptions{}, fn) }

// OnKeyWith is OnKey firing as opts say.
func (cx *Scope) OnKeyWith(key string, opts KeyOptions, fn func()) {
	t := cx.rt.tree
	if t == nil {
		return
	}
	k := driver.NormalizeKey(key)
	inText := opts.When == KeyAlways || opts.When == KeyAuto && !EditsText(k)
	b := &keyBinding{KeyBinding: driver.KeyBinding{Key: k, InText: inText, Repeat: opts.Repeat}, owner: cx.current(), fn: fn}
	t.keys = append(t.keys, b)
	t.keysChanged = true
	t.changed = true
	b.owner.addCleanup(func() {
		t.keys = slices.DeleteFunc(t.keys, func(k *keyBinding) bool { return k == b })
		t.keysChanged = true
		t.changed = true
	})
}

type backHandler struct {
	owner *Scope
	fn    func() bool
}

// OnBackPress runs fn when back is pressed (Escape, the back button or
// gesture, the browser's back) while the current owner lives: after the
// overlays and modals, which take it first, and before the navigation
// stacks, the newest handler first. fn returns true to take it; when none
// does, the top stack pops, else the platform goes back itself.
func (cx *Scope) OnBackPress(fn func() bool) {
	t := cx.rt.tree
	if t == nil {
		return
	}
	b := &backHandler{owner: cx.current(), fn: fn}
	t.backs = append(t.backs, b)
	b.owner.addCleanup(func() { t.backs = slices.DeleteFunc(t.backs, func(o *backHandler) bool { return o == b }) })
}

// ExitApp is the core's part of mobile.ExitApp, which apps use: it
// leaves the app as Android's back button does when nothing takes it.
func (cx *Scope) ExitApp() {
	if t := cx.rt.tree; t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemBackUnhandled})
		t.changed = true
	}
}

// keymap returns the bindings, oldest first: the newest of a key wins.
func (t *Tree) keymap() []driver.KeyBinding {
	out := make([]driver.KeyBinding, 0, len(t.keys))
	for _, b := range t.keys {
		out = append(out, b.KeyBinding)
	}
	return out
}

func (t *Tree) pressKey(key string) {
	key = driver.NormalizeKey(key)
	for i := len(t.keys) - 1; i >= 0; i-- {
		if b := t.keys[i]; b.Key == key && !b.owner.disposed {
			t.within(b.owner, b.fn)
			return
		}
	}
}

// EditsText reports whether a text input with the keyboard focus uses
// the normalized key itself: keys that type, move the caret or delete,
// and select all, cut, copy, paste, undo and redo. A binding of it fires
// in text inputs only with KeyAlways.
func EditsText(key string) bool {
	parts := strings.Split(key, "+")
	name, mods := parts[len(parts)-1], parts[:len(parts)-1]
	command := slices.Contains(mods, "mod") || slices.Contains(mods, "ctrl") || slices.Contains(mods, "meta")
	switch {
	case len(name) > 1 && name[0] == 'f' && name[1] >= '1' && name[1] <= '9':
		return false
	case len(name) == 1:
		return !command || strings.Contains("acvxyz", name)
	}
	return true
}

// KeyLabel is key as menus and hints show it: ⌃⌥⇧⌘K where "mod" is
// Command (apple), else Ctrl+Alt+Shift+K, with "meta" as Win on Windows,
// Meta on the web and Super elsewhere.
func KeyLabel(key string, os OS, apple bool) string {
	parts := strings.Split(driver.NormalizeKey(key), "+")
	name, mods := parts[len(parts)-1], parts[:len(parts)-1]
	has := func(m string) bool { return slices.Contains(mods, m) }
	ctrl, cmd := has("ctrl"), has("meta")
	if has("mod") {
		if apple {
			cmd = true
		} else {
			ctrl = true
		}
	}
	var b strings.Builder
	if apple {
		for _, m := range []struct {
			on    bool
			glyph string
		}{{ctrl, "⌃"}, {has("alt"), "⌥"}, {has("shift"), "⇧"}, {cmd, "⌘"}} {
			if m.on {
				b.WriteString(m.glyph)
			}
		}
	} else {
		meta := map[OS]string{OSWindows: "Win+", OSWeb: "Meta+"}[os]
		if meta == "" {
			meta = "Super+"
		}
		for _, m := range []struct {
			on   bool
			name string
		}{{ctrl, "Ctrl+"}, {has("alt"), "Alt+"}, {has("shift"), "Shift+"}, {cmd, meta}} {
			if m.on {
				b.WriteString(m.name)
			}
		}
	}
	named := map[string][2]string{
		"enter": {"↩", "Enter"}, "escape": {"⎋", "Esc"}, "tab": {"⇥", "Tab"}, "backspace": {"⌫", "Backspace"},
		"delete": {"⌦", "Del"}, "up": {"↑", "↑"}, "down": {"↓", "↓"}, "left": {"←", "←"}, "right": {"→", "→"},
		"home": {"↖", "Home"}, "end": {"↘", "End"}, "pageup": {"⇞", "PgUp"}, "pagedown": {"⇟", "PgDn"},
		"space": {"Space", "Space"},
	}
	switch n, ok := named[name]; {
	case ok && apple:
		b.WriteString(n[0])
	case ok:
		b.WriteString(n[1])
	default:
		b.WriteString(strings.ToUpper(name))
	}
	return b.String()
}

// KeyLabel is key as the platform the app runs on shows it, as
// core.KeyLabel says.
func (cx *Scope) KeyLabel(key string) string {
	env := cx.Env()
	var os OS
	var apple bool
	Untracked(cx, func() bool {
		os, apple = env.OS(), env.AppleKeys()
		return true
	})
	return KeyLabel(key, os, apple)
}

// SetTitle is the core's part of desktop.SetTitle, which apps use.
func (cx *Scope) SetTitle(title string) {
	if t := cx.rt.tree; t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemTitle, Value: title})
		t.changed = true
	}
}
