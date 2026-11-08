//go:build darwin && !ios

package macos

import (
	"strings"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// Event types and modifier flags, from NSEvent.h.
const (
	evLeftDown, evLeftUp, evRightDown, evRightUp, evMoved, evLeftDragged = 1, 2, 3, 4, 5, 6
	evKeyDown, evKeyUp                                                   = 10, 11
	flagShift, flagControl, flagOption, flagCommand                      = 1 << 17, 1 << 18, 1 << 19, 1 << 20
)

func uptime() float64 {
	return objc.Send[float64](objc.ID(objc.GetClass("NSProcessInfo")).Send(sel("processInfo")), sel("systemUptime"))
}

// keyCodes are the virtual key codes of the US layout's keys by what
// they type unshifted, and of the keys tests and web input name, from
// Carbon's Events.h.
var keyCodes = map[string]uint16{
	"enter": 36, "return": 36, "tab": 48, "space": 49, "backspace": 51, "escape": 53, "delete": 117,
	"left": 123, "right": 124, "down": 125, "up": 126, "home": 115, "end": 119, "pageup": 116, "pagedown": 121,
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9, "b": 11, "q": 12, "w": 13,
	"e": 14, "r": 15, "y": 16, "t": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22, "5": 23, "9": 25, "7": 26,
	"8": 28, "0": 29, "o": 31, "u": 32, "i": 34, "p": 35, "l": 37, "j": 38, "k": 40, "n": 45, "m": 46,
	"=": 24, "-": 27, "]": 30, "[": 33, "'": 39, ";": 41, "\\": 42, ",": 43, "/": 44, ".": 47, "`": 50, " ": 49,
}

// usShifted are the US layout's characters Shift types, by the key's
// unshifted one.
var usShifted = map[rune]string{
	'!': "1", '@': "2", '#': "3", '$': "4", '%': "5", '^': "6", '&': "7", '*': "8", '(': "9", ')': "0",
	'_': "-", '+': "=", '{': "[", '}': "]", '|': "\\", ':': ";", '"': "'", '<': ",", '>': ".", '?': "/", '~': "`",
}

// keyChars are what the special keys type, as AppKit reports them.
var keyChars = map[string]string{
	"enter": "\r", "return": "\r", "tab": "\t", "space": " ", "backspace": "\x7f", "escape": "\x1b",
	"delete": "", "left": "", "right": "", "down": "", "up": "",
	"home": "", "end": "", "pageup": "", "pagedown": "",
}

// noKey is the code of a key no US key types, which pages read as
// "Unidentified".
const noKey = 0xffff

// sendWebInput gives web, a WKWebView, input as the user's own: AppKit
// events handed to it, whose page events are trusted. It takes the
// keyboard focus first, as a click would.
func sendWebInput(web objc.ID, in driver.WebInput) {
	window := web.Send(sel("window"))
	if window == 0 {
		return
	}
	if window.Send(sel("firstResponder")) != web {
		window.Send(sel("makeFirstResponder:"), web)
	}
	number := objc.Send[int](window, sel("windowNumber"))
	switch in.Kind {
	case driver.WebInputClick:
		// CSS pixels are points at the page's zoom and magnification.
		scale := objc.Send[float64](web, sel("pageZoom")) * objc.Send[float64](web, sel("magnification"))
		at := objc.Send[apple.CGPoint](web, sel("convertPoint:toView:"),
			apple.CGPoint{X: float64(in.Point.X) * scale, Y: float64(in.Point.Y) * scale}, objc.ID(0))
		for _, kind := range []uint{evLeftDown, evLeftUp} {
			pressure := float32(1)
			if kind == evLeftUp {
				pressure = 0
			}
			ev := objc.ID(objc.GetClass("NSEvent")).Send(
				sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
				kind, at, uint(0), uptime(), number, objc.ID(0), 0, 1, pressure)
			if kind == evLeftDown {
				web.Send(sel("mouseDown:"), ev)
			} else {
				web.Send(sel("mouseUp:"), ev)
			}
		}
	case driver.WebInputText:
		// The US layout's key for each character, so the page's
		// KeyboardEvent.code names the key a user would press; a line
		// break is Return, and what no US key types no key at all.
		for _, r := range in.Text {
			chars := string(r)
			if r == '\n' {
				chars = "\r"
			}
			plain, shift := strings.ToLower(chars), r >= 'A' && r <= 'Z'
			if base, ok := usShifted[r]; ok {
				plain, shift = base, true
			}
			code, ok := keyCodes[plain]
			if chars == "\r" {
				code, ok = 36, true
			}
			if !ok {
				code, plain = noKey, chars
			}
			webKey(web, number, chars, plain, code, shift)
		}
	case driver.WebInputKey:
		name := strings.ToLower(in.Key)
		code, ok := keyCodes[name]
		if !ok || keyChars[name] == "" {
			return
		}
		webKey(web, number, keyChars[name], keyChars[name], code, false)
	}
}

// webKey hands web a key's down and up typing chars, plain without Shift.
func webKey(web objc.ID, window int, chars, plain string, code uint16, shift bool) {
	var flags uint
	if shift {
		flags = flagShift
	}
	for _, kind := range []uint{evKeyDown, evKeyUp} {
		// keyCode and isARepeat follow the eight register arguments, so
		// they go on the stack packed as Apple's arm64 ABI packs them,
		// which purego, giving each a slot, would not: one word carries
		// both.
		stacked := uint64(code) << 16
		ev := objc.ID(objc.GetClass("NSEvent")).Send(
			sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
			kind, apple.CGPoint{}, flags, uptime(), window, objc.ID(0), apple.NSString(chars), apple.NSString(plain), stacked)
		if kind == evKeyDown {
			web.Send(sel("keyDown:"), ev)
		} else {
			web.Send(sel("keyUp:"), ev)
		}
	}
}
