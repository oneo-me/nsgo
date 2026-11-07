package base

import "github.com/oneo-me/nsgo/core"

// Selection is the rows a Tree or ui.List has chosen, by key, and the row
// its keyboard cursor is on. The app makes it with NewSelection and keeps
// it with the rest of its state: it reads it as it reads a signal, and
// sets it as the list does.
type Selection struct {
	multiple bool
	version  *core.Signal[int]
	chosen   map[any]struct{}
	order    []any
	// cursor is the row the keyboard is on, anchor where a range from
	// Shift starts; nil for none.
	cursor, anchor any
}

// NewSelection returns an empty selection of one row, or with multiple
// of any number.
func NewSelection(cx *core.Scope, multiple bool) *Selection {
	return &Selection{multiple: multiple, version: core.NewSignal(cx, 0), chosen: map[any]struct{}{}}
}

// Multiple reports whether the selection holds any number of rows.
func (s *Selection) Multiple() bool { return s.multiple }

// Has reports whether the row of key is chosen.
func (s *Selection) Has(key any) bool {
	s.version.Get()
	_, ok := s.chosen[key]
	return ok
}

// Keys returns the chosen rows' keys in the order they were chosen.
func (s *Selection) Keys() []any {
	s.version.Get()
	return append([]any(nil), s.order...)
}

// Len returns how many rows are chosen.
func (s *Selection) Len() int {
	s.version.Get()
	return len(s.order)
}

// Cursor returns the key of the row the keyboard is on, nil for none.
func (s *Selection) Cursor() any {
	s.version.Get()
	return s.cursor
}

// Anchor returns the key of the row a range from Shift starts at, nil for
// none.
func (s *Selection) Anchor() any {
	s.version.Get()
	return s.anchor
}

// Set chooses the rows of keys alone, only the last of them in a
// selection of one, and moves the cursor to the last.
func (s *Selection) Set(keys ...any) {
	if !s.multiple && len(keys) > 1 {
		keys = keys[len(keys)-1:]
	}
	s.chosen, s.order = map[any]struct{}{}, nil
	for _, k := range keys {
		if _, ok := s.chosen[k]; !ok {
			s.chosen[k] = struct{}{}
			s.order = append(s.order, k)
		}
	}
	if len(keys) > 0 {
		s.cursor, s.anchor = keys[len(keys)-1], keys[len(keys)-1]
	}
	s.changed()
}

// SetRange chooses the rows of keys alone, as a range from anchor that
// reaches cursor.
func (s *Selection) SetRange(anchor, cursor any, keys ...any) {
	s.Set(keys...)
	s.cursor, s.anchor = cursor, anchor
	s.changed()
}

// Toggle chooses the row of key, or unchooses it when chosen, and moves
// the cursor to it. A selection of one keeps no other row.
func (s *Selection) Toggle(key any) {
	if _, ok := s.chosen[key]; ok {
		delete(s.chosen, key)
		for i, k := range s.order {
			if k == key {
				s.order = append(s.order[:i:i], s.order[i+1:]...)
				break
			}
		}
		s.cursor, s.anchor = key, key
		s.changed()
		return
	}
	if !s.multiple {
		s.Set(key)
		return
	}
	s.chosen[key] = struct{}{}
	s.order = append(s.order, key)
	s.cursor, s.anchor = key, key
	s.changed()
}

// Clear chooses no row and leaves the cursor where it is.
func (s *Selection) Clear() {
	s.chosen, s.order = map[any]struct{}{}, nil
	s.changed()
}

// SetCursor moves the keyboard to the row of key without choosing it.
func (s *Selection) SetCursor(key any) {
	s.cursor = key
	s.changed()
}

func (s *Selection) changed() { s.version.Update(func(n int) int { return n + 1 }) }
