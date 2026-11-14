//go:build windows

package windows

import (
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	registerHotKey   = user32.NewProc("RegisterHotKey")
	unregisterHotKey = user32.NewProc("UnregisterHotKey")
)

const (
	wmHotKey = 0x0312

	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000
)

// hotKeysApply replaces the app's global shortcuts. RegisterHotKey sends
// WM_HOTKEY to this window even when another app has the focus.
func (w *window) hotKeysApply(keys []driver.KeyBinding) {
	w.unregisterHotKeys()
	if len(keys) == 0 || w.hwnd == 0 {
		return
	}
	if w.hotKeys == nil {
		w.hotKeys = make(map[uint32]struct{}, len(keys))
	}
	for _, key := range keys {
		mods, vk, ok := globalHotKey(key.Key)
		if !ok {
			continue
		}
		if r, _, _ := registerHotKey.Call(w.hwnd, uintptr(key.ID), uintptr(mods|modNoRepeat), vk); r != 0 {
			w.hotKeys[key.ID] = struct{}{}
		}
	}
}

// unregisterHotKeys releases all global shortcuts owned by the window.
func (w *window) unregisterHotKeys() {
	for id := range w.hotKeys {
		unregisterHotKey.Call(w.hwnd, uintptr(id))
		delete(w.hotKeys, id)
	}
}

// globalHotKey converts a normalized key binding to RegisterHotKey's
// modifiers and virtual key code. "mod" is Control on Windows.
func globalHotKey(key string) (mods uint32, vk uintptr, ok bool) {
	parts := strings.Split(key, "+")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return 0, 0, false
	}
	for _, modifier := range parts[:len(parts)-1] {
		switch modifier {
		case "mod", "ctrl":
			mods |= modControl
		case "alt":
			mods |= modAlt
		case "shift":
			mods |= modShift
		case "meta":
			mods |= modWin
		default:
			return 0, 0, false
		}
	}
	vk, ok = globalVirtualKey(parts[len(parts)-1])
	return mods, vk, ok
}

func globalVirtualKey(name string) (uintptr, bool) {
	if len(name) == 1 {
		c := name[0]
		switch {
		case c >= 'a' && c <= 'z':
			return uintptr(c - 'a' + 'A'), true
		case c >= '0' && c <= '9':
			return uintptr(c), true
		}
	}
	if strings.HasPrefix(name, "f") {
		if n, err := strconv.Atoi(name[1:]); err == nil && n >= 1 && n <= 24 {
			return uintptr(0x6f + n), true
		}
	}
	if vk, ok := globalSpecialKeys[name]; ok {
		return vk, true
	}
	return 0, false
}

var globalSpecialKeys = map[string]uintptr{
	"backspace": 0x08, "tab": 0x09, "enter": 0x0D, "escape": 0x1B, "space": 0x20,
	"pageup": 0x21, "pagedown": 0x22, "end": 0x23, "home": 0x24,
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28, "delete": 0x2E,
	";": 0xBA, "=": 0xBB, ",": 0xBC, "-": 0xBD, ".": 0xBE, "/": 0xBF,
	"`": 0xC0, "[": 0xDB, "\\": 0xDC, "]": 0xDD, "'": 0xDE,
	"plus": 0xBB, "minus": 0xBD,
}
