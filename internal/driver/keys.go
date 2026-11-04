package driver

import (
	"slices"
	"strings"
)

// KeyBinding is one binding of the keymap the platform matches key
// presses against, on the main thread.
type KeyBinding struct {
	// Key is normalized, as NormalizeKey writes it.
	Key string
	// InText fires it while a text input or a web view has the keyboard
	// focus.
	InText bool
	// Repeat fires it again on the auto-repeats of a held key; else they
	// are consumed and fire nothing.
	Repeat bool
	// ID is a global shortcut's (ItemHotKeys), which EventHotKey reports.
	ID uint32
}

// NormalizeKey returns key lowercased with its modifiers in the order
// mod, ctrl, alt, shift, meta; "cmd" and "command" are meta, "option" is
// alt and "control" is ctrl.
func NormalizeKey(key string) string {
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(key, " ", "")), "+")
	name := parts[len(parts)-1]
	alias := map[string]string{"cmd": "meta", "command": "meta", "option": "alt", "control": "ctrl"}
	var given []string
	for _, m := range parts[:len(parts)-1] {
		if a, ok := alias[m]; ok {
			m = a
		}
		given = append(given, m)
	}
	var mods []string
	for _, m := range []string{"mod", "ctrl", "alt", "shift", "meta"} {
		if slices.Contains(given, m) {
			mods = append(mods, m)
		}
	}
	return strings.Join(append(mods, name), "+")
}

// FindKey says what a key press does with keymap: names are the press's
// normalized names (with "mod" and with the literal modifier), editing
// whether a text input or web view has the keyboard focus, and repeat
// whether it is a held key's auto-repeat. It returns the binding that
// fires, or "" and whether the press is consumed anyway: the repeat of a
// binding that fires once.
func FindKey(keymap []KeyBinding, names []string, editing, repeat bool) (key string, consumed bool) {
	for i := len(keymap) - 1; i >= 0; i-- {
		b := keymap[i]
		if !slices.Contains(names, b.Key) || editing && !b.InText {
			continue
		}
		if repeat && !b.Repeat {
			return "", true
		}
		return b.Key, true
	}
	return "", false
}
