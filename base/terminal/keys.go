package terminal

import (
	"strings"
	"unicode"

	"github.com/nsgo-dev/nsgo/base/terminal/internal/vt"
)

// keyInfo is Ghostty's key for a key core names, and the character it
// types without modifiers, 0 for keys that type none.
type keyInfo struct {
	key vt.Key
	r   rune
}

// keys maps the keys core.Scope.OnKey names to Ghostty's.
var keys = map[string]keyInfo{
	"enter": {vt.KeyEnter, '\r'}, "escape": {vt.KeyEscape, 0}, "backspace": {vt.KeyBackspace, 0},
	"tab": {vt.KeyTab, '\t'}, "space": {vt.KeySpace, ' '}, "delete": {vt.KeyDelete, 0},
	"home": {vt.KeyHome, 0}, "end": {vt.KeyEnd, 0}, "pageup": {vt.KeyPageUp, 0}, "pagedown": {vt.KeyPageDown, 0},
	"left": {vt.KeyArrowLeft, 0}, "right": {vt.KeyArrowRight, 0}, "up": {vt.KeyArrowUp, 0}, "down": {vt.KeyArrowDown, 0},
	"-": {vt.KeyMinus, '-'}, "=": {vt.KeyEqual, '='}, ",": {vt.KeyComma, ','}, ".": {vt.KeyPeriod, '.'},
	"/": {vt.KeySlash, '/'}, ";": {vt.KeySemicolon, ';'}, "'": {vt.KeyQuote, '\''}, "[": {vt.KeyBracketLeft, '['},
	"]": {vt.KeyBracketRight, ']'}, "\\": {vt.KeyBackslash, '\\'}, "`": {vt.KeyBackquote, '`'},
}

func init() {
	for i := range 12 {
		keys["f"+itoa(i+1)] = keyInfo{vt.KeyF1 + vt.Key(i), 0}
	}
	for i := range 10 {
		keys[string(rune('0'+i))] = keyInfo{vt.KeyDigit0 + vt.Key(i), '0' + rune(i)}
	}
	for i := range 26 {
		keys[string(rune('a'+i))] = keyInfo{vt.KeyA + vt.Key(i), 'a' + rune(i)}
	}
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// namedKeys are the keys that type no text, or text the sink must not
// take (Tab moves the focus, Enter submits), which the view binds.
var namedKeys = []string{
	"enter", "escape", "backspace", "tab", "delete", "home", "end", "pageup", "pagedown",
	"left", "right", "up", "down",
	"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12",
}

// textKeys are the keys that type a character, which the view binds with
// Control, or Alt where Alt is no character's modifier.
var textKeys = func() []string {
	k := []string{"space", "-", "=", ",", ".", "/", ";", "'", "[", "]", "\\", "`"}
	for i := range 10 {
		k = append(k, string(rune('0'+i)))
	}
	for i := range 26 {
		k = append(k, string(rune('a'+i)))
	}
	return k
}()

// parseKey reads a key core names ("ctrl+shift+left") as Ghostty's key,
// the character it types without modifiers, and the modifiers.
func parseKey(name string) (vt.Key, rune, vt.Mods) {
	parts := strings.Split(name, "+")
	var mods vt.Mods
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "shift":
			mods |= vt.ModShift
		case "ctrl":
			mods |= vt.ModCtrl
		case "alt":
			mods |= vt.ModAlt
		case "meta":
			mods |= vt.ModSuper
		}
	}
	k := keys[parts[len(parts)-1]]
	return k.key, k.r, mods
}

// keyOfRune returns the key that types r, and whether it takes Shift, on
// a US keyboard, for text that comes without its key, as from the sink.
func keyOfRune(r rune) (vt.Key, rune, bool) {
	if r == ' ' {
		return vt.KeySpace, ' ', false
	}
	if r < 0x80 {
		lower := unicode.ToLower(r)
		if k, ok := keys[string(lower)]; ok && k.r > ' ' {
			return k.key, k.r, lower != r
		}
		if name, ok := shifted[r]; ok {
			return keys[name].key, keys[name].r, true
		}
	}
	return vt.KeyUnidentified, 0, false
}

// shifted are the keys of the symbols Shift types on a US keyboard.
var shifted = map[rune]string{
	'!': "1", '@': "2", '#': "3", '$': "4", '%': "5", '^': "6", '&': "7", '*': "8",
	'(': "9", ')': "0", '_': "-", '+': "=", '<': ",", '>': ".",
	'?': "/", ':': ";", '"': "'", '{': "[", '}': "]", '|': "\\", '~': "`",
}
