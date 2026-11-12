//go:build darwin && !ios

package macos

import (
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Global shortcuts are Carbon's hot keys, which fire whichever app has
// the focus and need no accessibility permission. Carbon reports each to
// the application's event target, which hands its ID to the surface that
// registered it.

var carbon struct {
	loaded                    bool
	registerEventHotKey       func(code, modifiers uint32, id uint64, target uintptr, options uint32, ref *uintptr) int32
	unregisterEventHotKey     func(ref uintptr) int32
	getApplicationEventTarget func() uintptr
	installEventHandler       func(target, handler uintptr, count uint32, types unsafe.Pointer, data uintptr, ref *uintptr) int32
	getEventParameter         func(event uintptr, name, typ uint32, actual uintptr, size uintptr, actualSize uintptr, data unsafe.Pointer) int32
}

// hotKey is a registered global shortcut and the surface it reports to.
type hotKey struct {
	ref   uintptr
	owner *surface
}

// registered are the hot keys by ID. Only the main thread uses them.
var registered = map[uint32]hotKey{}

// fourCC is a Carbon four-character code.
func fourCC(s string) uint32 {
	return uint32(s[0])<<24 | uint32(s[1])<<16 | uint32(s[2])<<8 | uint32(s[3])
}

// loadCarbon loads Carbon's hot key functions and installs the handler
// of their presses, once; it reports whether they are there.
func loadCarbon() bool {
	if carbon.loaded {
		return carbon.registerEventHotKey != nil
	}
	carbon.loaded = true
	lib, err := purego.Dlopen("/System/Library/Frameworks/Carbon.framework/Carbon", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return false
	}
	purego.RegisterLibFunc(&carbon.registerEventHotKey, lib, "RegisterEventHotKey")
	purego.RegisterLibFunc(&carbon.unregisterEventHotKey, lib, "UnregisterEventHotKey")
	purego.RegisterLibFunc(&carbon.getApplicationEventTarget, lib, "GetApplicationEventTarget")
	purego.RegisterLibFunc(&carbon.installEventHandler, lib, "InstallEventHandler")
	purego.RegisterLibFunc(&carbon.getEventParameter, lib, "GetEventParameter")
	handler := purego.NewCallback(func(next, event, data uintptr) uintptr {
		var id [2]uint32 // EventHotKeyID: its signature and ID
		carbon.getEventParameter(event, fourCC("----"), fourCC("hkid"), 0, unsafe.Sizeof(id), 0, unsafe.Pointer(&id))
		if k, ok := registered[id[1]]; ok && k.owner.engine != nil && !k.owner.closed {
			k.owner.engine.Dispatch(driver.Event{Kind: driver.EventHotKey, Count: id[1]})
		}
		return 0
	})
	const hotKeyPressed = 5
	spec := [2]uint32{fourCC("keyb"), hotKeyPressed}
	var ref uintptr
	carbon.installEventHandler(carbon.getApplicationEventTarget(), handler, 1, unsafe.Pointer(&spec), 0, &ref)
	return true
}

// hotKeys registers keys as the app's global shortcuts in place of the
// ones before.
func (s *surface) hotKeys(keys []driver.KeyBinding) {
	if len(keys) == 0 && len(registered) == 0 || !loadCarbon() {
		return
	}
	unregisterHotKeys()
	signature := uint64(fourCC("nsgo"))
	for _, k := range keys {
		code, mods, ok := carbonKey(k.Key)
		if !ok {
			continue
		}
		var ref uintptr
		// EventHotKeyID is its signature and the ID, passed in one register.
		if carbon.registerEventHotKey(code, mods, signature|uint64(k.ID)<<32, carbon.getApplicationEventTarget(), 0, &ref) == 0 {
			registered[k.ID] = hotKey{ref, s}
		}
	}
}

// unregisterHotKeys unregisters every hot key.
func unregisterHotKeys() {
	for id, k := range registered {
		carbon.unregisterEventHotKey(k.ref)
		delete(registered, id)
	}
}

// Carbon's modifiers.
const carbonCmd, carbonShift, carbonOption, carbonControl = 1 << 8, 1 << 9, 1 << 11, 1 << 12

// carbonKey returns a normalized key's virtual key code and Carbon
// modifiers; mod is Command.
func carbonKey(key string) (code, mods uint32, ok bool) {
	parts := strings.Split(key, "+")
	name := parts[len(parts)-1]
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "mod", "meta":
			mods |= carbonCmd
		case "ctrl":
			mods |= carbonControl
		case "alt":
			mods |= carbonOption
		case "shift":
			mods |= carbonShift
		}
	}
	c, ok := virtualKeys[name]
	return uint32(c), mods, ok
}

// virtualKeys are the ANSI keyboard's virtual key codes (kVK_*) of the
// keys driver.NormalizeKey names.
var virtualKeys = map[string]uint16{
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9, "b": 11, "q": 12,
	"w": 13, "e": 14, "r": 15, "y": 16, "t": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22, "5": 23,
	"=": 24, "plus": 24, "9": 25, "7": 26, "-": 27, "minus": 27, "8": 28, "0": 29, "]": 30, "o": 31,
	"u": 32, "[": 33, "i": 34, "p": 35, "enter": 36, "l": 37, "j": 38, "'": 39, "k": 40, ";": 41,
	"\\": 42, ",": 43, "/": 44, "n": 45, "m": 46, ".": 47, "tab": 48, "space": 49, "`": 50,
	"backspace": 51, "escape": 53, "f17": 64, "f18": 79, "f19": 80, "f20": 90, "f5": 96, "f6": 97,
	"f7": 98, "f3": 99, "f8": 100, "f9": 101, "f11": 103, "f13": 105, "f16": 106, "f14": 107,
	"f10": 109, "f12": 111, "f15": 113, "home": 115, "pageup": 116, "delete": 117, "f4": 118,
	"end": 119, "f2": 120, "pagedown": 121, "f1": 122, "left": 123, "right": 124, "down": 125, "up": 126,
}
