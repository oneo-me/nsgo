//go:build darwin && !ios

package macos

import (
	"slices"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// Keyboard focus for pressable views, as NSH's macOS backend gives it: a
// view that takes presses or is Focusable is in AppKit's key view loop
// with the text fields, unless it is disabled, hidden, NoFocus or under a
// newer modal;
// a click presses it without focusing it, and Return or Space press it
// once while it has the focus. A modal remembers what had the focus as it
// came and gives it back when it goes, if the focus fell to the window.

// focusMethods are the views' first responder methods.
func focusMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{Cmd: sel("acceptsFirstResponder"), Fn: func(self objc.ID, _ objc.SEL) bool {
			v := views[self]
			return v != nil && v.focusable() && !v.surface.clicking()
		}},
		{Cmd: sel("canBecomeKeyView"), Fn: func(self objc.ID, _ objc.SEL) bool {
			v := views[self]
			return v != nil && v.focusable()
		}},
		{Cmd: sel("becomeFirstResponder"), Fn: func(self objc.ID, cmd objc.SEL) bool {
			if v := views[self]; v != nil {
				v.dispatch(driver.EventFocus)
			}
			return true
		}},
		{Cmd: sel("resignFirstResponder"), Fn: func(self objc.ID, cmd objc.SEL) bool {
			if v := views[self]; v != nil {
				v.dispatch(driver.EventBlur)
			}
			return true
		}},
		{Cmd: sel("keyDown:"), Fn: func(self objc.ID, cmd objc.SEL, event objc.ID) {
			v := views[self]
			const tab, shift = 48, 1 << 17
			if v != nil && objc.Send[uint16](event, sel("keyCode")) == tab {
				// The surface around takes the keys that reach it; Tab moves
				// the focus here.
				if objc.Send[uint](event, sel("modifierFlags"))&shift != 0 {
					v.surface.window.Send(sel("selectPreviousKeyView:"), self)
				} else {
					v.surface.window.Send(sel("selectNextKeyView:"), self)
				}
				return
			}
			if v == nil || !v.pressable() || !activates(event) {
				apple.Super(self, cmd, event)
				return
			}
			if !objc.Send[bool](event, sel("isARepeat")) {
				for _, k := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
					v.report(driver.Event{Kind: k, Index: -1})
				}
			}
		}},
		{Cmd: sel("keyUp:"), Fn: func(self objc.ID, cmd objc.SEL, event objc.ID) {
			if v := views[self]; v == nil || !v.pressable() || !activates(event) {
				apple.Super(self, cmd, event)
			}
		}},
	}
}

// activates reports whether a key event is Return, the keypad's Enter or
// Space with no modifier.
func activates(event objc.ID) bool {
	const shift, control, option, command = 1 << 17, 1 << 18, 1 << 19, 1 << 20
	code := objc.Send[uint16](event, sel("keyCode"))
	return (code == 36 || code == 76 || code == 49) &&
		objc.Send[uint](event, sel("modifierFlags"))&(shift|control|option|command) == 0
}

// focusable reports whether v may take the keyboard focus.
func (v *view) focusable() bool {
	if v.native == 0 || !v.pressable() && !v.props.Focusable || v.props.Disabled || v.props.NoFocus || v.kind == driver.KindInput ||
		objc.Send[bool](v.native, sel("isHiddenOrHasHiddenAncestor")) || v.native.Send(selWindow) == 0 {
		return false
	}
	if n := len(v.surface.modals); n > 0 {
		top := v.surface.modals[n-1]
		return objc.Send[bool](v.native, sel("isDescendantOf:"), top.native)
	}
	return true
}

// clicking reports whether the window is handling a mouse press, which
// presses a view without focusing it.
func (s *surface) clicking() bool {
	const leftDown, rightDown, otherDown = 1, 3, 25
	e := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication")).Send(sel("currentEvent"))
	if e == 0 {
		return false
	}
	t := objc.Send[uint](e, sel("type"))
	return t == leftDown || t == rightDown || t == otherDown
}

// focusedPressable reports whether a view that takes presses has the
// keyboard focus: Return and Space are its, not the keymap's.
func (s *surface) focusedPressable() bool {
	v := views[s.window.Send(sel("firstResponder"))]
	return v != nil && v.surface == s && v.focusable()
}

// modalCame remembers what had the focus as modal v came: a field's
// NSTextField rather than the shared field editor it edits in.
func (s *surface) modalCame(v *view) {
	first := s.window.Send(sel("firstResponder"))
	if objc.Send[bool](first, sel("isKindOfClass:"), objc.GetClass("NSTextView")) &&
		objc.Send[bool](first, sel("isFieldEditor")) {
		first = first.Send(sel("delegate"))
	}
	if first != 0 && first != s.window {
		v.focusBefore = first.Send(sel("retain"))
	}
	s.modals = append(s.modals, v)
}

// modalGone gives the focus back to what had it as modal v came, if it
// fell to the window or the surface meanwhile and that can still take it.
func (s *surface) modalGone(v *view) {
	i := slices.Index(s.modals, v)
	if i < 0 {
		return
	}
	s.modals = slices.Delete(s.modals, i, i+1)
	back := v.focusBefore
	v.focusBefore = 0
	if back == 0 {
		return
	}
	defer back.Send(selRelease)
	// A newer modal that remembered a view of v remembers v's instead.
	for _, m := range s.modals[i:] {
		if m.focusBefore != 0 && objc.Send[bool](m.focusBefore, sel("isKindOfClass:"), objc.GetClass("NSView")) &&
			objc.Send[bool](m.focusBefore, sel("isDescendantOf:"), v.native) {
			m.focusBefore.Send(selRelease)
			m.focusBefore = back.Send(sel("retain"))
			return
		}
	}
	first := s.window.Send(sel("firstResponder"))
	if first != 0 && first != s.window && first != s.native {
		return
	}
	if back.Send(selWindow) != s.window || !objc.Send[bool](back, sel("acceptsFirstResponder")) {
		return
	}
	s.window.Send(sel("makeFirstResponder:"), back)
	// A field comes back with the caret at the end, its text unselected.
	if !objc.Send[bool](back, sel("respondsToSelector:"), sel("currentEditor")) {
		return
	}
	if editor := back.Send(sel("currentEditor")); editor != 0 {
		n := objc.Send[uint](editor.Send(sel("string")), sel("length"))
		editor.Send(sel("setSelectedRange:"), apple.NSRange{Location: n})
	}
}
