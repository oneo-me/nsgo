// Package field keeps what a backend reports about one text field the way
// NSH's platforms do, whatever control edits it: the user's edits and
// their count, a composition and the value the app sets during one,
// the selection, keys and maxLength.
package field

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// State is the reporting side of one text field. Backends call it from
// their main thread as the control changes, and it dispatches the events
// the field's mask asks for.
type State struct {
	ID driver.NodeID
	// Events is the field's mask (ItemEvents); Config its last ItemInput.
	Events   driver.EventMask
	Config   driver.InputConfig
	Dispatch func(driver.Event)

	// Count is the user's edits reported.
	Count       uint32
	known       string // the text last reported or set
	composing   bool
	composition string
	held        *held // a value the app set during a composition
	selection   driver.TextRange
}

type held struct {
	text  string
	count uint32
}

func (s *State) send(ev driver.Event) {
	if s.Events.Has(ev.Kind) && s.Dispatch != nil {
		ev.ID = s.ID
		s.Dispatch(ev)
	}
}

// Known is the text the engine knows the field holds.
func (s *State) Known() string { return s.known }

// Edited reports the user's edit leaving text in the field, unless the
// engine already knows it holds text. It returns whether it reported.
func (s *State) Edited(text string) bool {
	if text == s.known {
		return false
	}
	s.known = text
	s.Count++
	if s.Dispatch != nil {
		// Always reported: the engine keeps the text.
		s.Dispatch(driver.Event{Kind: driver.EventChangeText, ID: s.ID, Text: text, Count: s.Count})
	}
	return true
}

// Value is ItemInputValue: it reports whether the backend sets the text
// now. A value older than the user's last edit is dropped, and one that
// comes during a composition is held until it ends, since setting the
// text would end the composition and commit what it composes.
func (s *State) Value(text string, count uint32) bool {
	if count < s.Count {
		return false
	}
	if s.composing {
		s.held = &held{text, count}
		return false
	}
	s.known = text
	return true
}

// Compose records the input method's composition (composing false once
// it ended) and reports a change. When it ends, it returns a value the
// app set meanwhile for the backend to set, provided no edit came since
// and current, the field's text, is what the engine knows.
func (s *State) Compose(composing bool, text, current string) (value string, apply bool) {
	if !composing {
		text = ""
	}
	if composing == s.composing && text == s.composition {
		return "", false
	}
	s.composing, s.composition = composing, text
	if !composing && s.held != nil {
		h := s.held
		s.held = nil
		if h.count >= s.Count && current == s.known {
			s.known = h.text
			value, apply = h.text, true
		}
	}
	s.send(driver.Event{Kind: driver.EventCompositionChange, Text: text, Finished: !composing})
	return value, apply
}

// Composing reports whether an input method composes in the field.
func (s *State) Composing() bool { return s.composing }

// Selected reports the selection the user left, in UTF-16 units, if it
// moved.
func (s *State) Selected(r driver.TextRange) {
	if r == s.selection {
		return
	}
	s.selection = r
	s.send(driver.Event{Kind: driver.EventSelectionChange, Index: r.Start, Count: uint32(r.End)})
}

// Selection is the selection last reported or set.
func (s *State) Selection() driver.TextRange { return s.selection }

// SetSelection records a selection the app set, which is not reported.
func (s *State) SetSelection(r driver.TextRange) { s.selection = r }

// Key reports a key the user typed: its text, "Backspace" or "Enter".
func (s *State) Key(key string) {
	s.send(driver.Event{Kind: driver.EventKeyPress, Text: driver.KeyOf(key)})
}

// Insert answers whether the user's edit replacing replaced UTF-16 units
// of current with insert may happen, as the platforms' should-change
// callbacks ask: it reports the key and returns the part of insert
// MaxLength leaves room for, with line breaks made spaces in a single
// line. ok is false when nothing of a non-empty insert fits, and
// whole whether kept is insert as it came.
func (s *State) Insert(current string, replaced int, insert string) (kept string, ok, whole bool) {
	if insert == "" {
		if replaced > 0 {
			s.Key("Backspace")
		}
		return "", true, true
	}
	text := insert
	if !s.Config.Multiline {
		text = SingleLine(text)
	}
	kept, ok = driver.LimitEdit(current, replaced, text, s.Config.MaxLength)
	if kept != "" {
		s.Key(kept)
	}
	return kept, ok, kept == insert
}

// Return decides what Return does: true when the backend lets the
// control insert a line break, else it has reported the key and the
// submit and returns whether to blur. lineBreak is a modifier held
// (Shift, Option, Alt) that breaks the line of a multiline field that
// submits.
func (s *State) Return(text string, lineBreak bool) (insert, blur bool) {
	b := s.Config.Submit()
	if b == driver.SubmitBehaviorNewline || s.Config.Multiline && lineBreak {
		return true, false
	}
	s.Key("Enter")
	s.send(driver.Event{Kind: driver.EventSubmitEditing, Text: text, Count: s.Count})
	return false, b == driver.SubmitBehaviorBlurAndSubmit
}

// SingleLine replaces line breaks with spaces, as a single-line field
// holds them.
func SingleLine(s string) string {
	out := []byte(nil)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\n' && c != '\r' {
			if out != nil {
				out = append(out, c)
			}
			continue
		}
		if out == nil {
			out = append([]byte{}, s[:i]...)
		}
		if c == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			i++
		}
		out = append(out, ' ')
	}
	if out == nil {
		return s
	}
	return string(out)
}
