package richeditor

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// normalizeKey orders a binding's modifiers Alt, Ctrl, Meta, Mod, Shift, as
// the view names keys: Mod stays Mod, the platform's being the view's to
// know.
func normalizeKey(name string) string {
	parts := strings.Split(name, "-")
	result := parts[len(parts)-1]
	if result == "" {
		result = "-"
	}
	if result == "Space" {
		result = " "
	}
	var alt, ctrl, shift, meta, mod bool
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "cmd", "meta", "m":
			meta = true
		case "a", "alt":
			alt = true
		case "c", "ctrl", "control":
			ctrl = true
		case "s", "shift":
			shift = true
		case "mod":
			mod = true
		}
	}
	if shift {
		result = "Shift-" + result
	}
	if mod {
		result = "Mod-" + result
	}
	if meta {
		result = "Meta-" + result
	}
	if ctrl {
		result = "Ctrl-" + result
	}
	if alt {
		result = "Alt-" + result
	}
	return result
}

// Keymap binds keys ("Mod-b", "Shift-Enter", "Backspace") to commands; a key's
// commands run until one applies. A binding with Shift spelled out goes
// before the shifted character's.
func Keymap(key string, bindings map[string]Command) *Plugin {
	normal := make(map[string]Command, len(bindings))
	for name, c := range bindings {
		normal[normalizeKey(name)] = c
	}
	return &Plugin{Key: key, HandleKeyDown: func(v *Session, name string) bool {
		run := func(c Command) bool {
			return c != nil && c(v.State(), v.Dispatch)
		}
		if run(normal[name]) {
			return true
		}
		// "Shift-B" for a character falls to the binding of "B".
		mods, char, ok := splitChar(name)
		if ok && strings.Contains(mods, "Shift-") {
			return run(normal[strings.Replace(mods, "Shift-", "", 1)+char])
		}
		return false
	}}
}

func splitChar(name string) (mods, char string, ok bool) {
	at := strings.LastIndex(name, "-")
	if at == len(name)-1 {
		at = strings.LastIndex(name[:at], "-")
	}
	char = name[at+1:]
	return name[:at+1], char, utf8.RuneCountInString(char) == 1
}

// MergeBindings joins keymaps, a key's commands running in the given order.
func MergeBindings(maps ...map[string]Command) map[string]Command {
	out := map[string]Command{}
	order := map[string][]Command{}
	for _, m := range maps {
		names := make([]string, 0, len(m))
		for n := range m {
			names = append(names, n)
		}
		slices.Sort(names)
		for _, n := range names {
			k := normalizeKey(n)
			order[k] = append(order[k], m[n])
		}
	}
	for k, cmds := range order {
		out[k] = ChainCommands(cmds...)
	}
	return out
}

// BaseKeymap is what a press of Enter and the deleting keys do anywhere.
func BaseKeymap() map[string]Command {
	backspace := ChainCommands(UndoInputRule, DeleteSelection, JoinBackward, SelectNodeBackward)
	del := ChainCommands(DeleteSelection, JoinForward, SelectNodeForward)
	return map[string]Command{
		"Enter":           ChainCommands(NewlineInCode, CreateParagraphNear, LiftEmptyBlock, SplitBlock),
		"Mod-Enter":       ExitCode,
		"Backspace":       backspace,
		"Mod-Backspace":   backspace,
		"Shift-Backspace": backspace,
		"Delete":          del,
		"Mod-Delete":      del,
		"Mod-a":           SelectAll,
	}
}
