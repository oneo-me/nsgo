package ui

import "github.com/oneo-me/nsgo/core"

// Selection is the rows a List or a Tree has chosen, by key, and the row
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

// List is a long run of the app's rows over core.List, built only near
// what shows, side by side in Columns for a grid. With a Selection its
// rows are chosen by a press and by the keyboard: arrows, Home, End and
// the page keys move the cursor, Shift extends a multiple selection and
// Mod+A chooses every row, Space toggles the cursor's row in one, and
// Enter or a double click activate a row. The list takes these keys
// while it has the keyboard focus, or after a press on a row until
// another list takes them. Give it a height, or let it fill what the app
// sizes.
type List struct {
	Count func() int
	// Key returns row i's identity, a comparable value such as an ID,
	// which the selection holds; without it, the selection holds indexes.
	Key func(i int) any
	// Row builds a row's content: a cell is built once and shows other
	// rows as it scrolls, so read row's index as a signal.
	Row       func(cx *core.Scope, row core.ListRow) core.Node
	Selection *Selection
	// OnActivate receives the row Enter or a double click activates, or
	// on touch screens a tap in a list that is not Multiple.
	OnActivate func(i int)
	// RowLabel is what assistive technology reads for row i, its text
	// when nil.
	RowLabel func(i int) string
	// Columns lays rows side by side, 1 when 0; the arrows then move
	// along a line and between lines.
	Columns int
	// EstimatedSize is a row's expected height, a row's height when 0.
	EstimatedSize core.Points
	Gap           core.Points
	Header        core.Node
	Footer        core.Node
	// Empty shows while Count is 0.
	Empty core.Node
	// OnLoadMore runs as the end of the rows comes near, for the next page.
	OnLoadMore func()
	// Label names the list for assistive technology.
	Label  string
	Ref    *core.Ref
	TestID string
	Style  core.Style
	// RowStyle merges over each row's.
	RowStyle core.Style

	// A Tree sets these: the keys it adds, and whether row i expands.
	keys     func(cx *core.Scope, l *listKeys)
	expanded func(i int) bool
}

// listKeys is a List's cursor as its keys move it.
type listKeys struct {
	list    *List
	columns int
	// at is where the cursor's key was found last, to look there first.
	at int
	// first and last are the rows wholly shown, -1 before the list
	// reports them.
	first, last int
	ref         *core.Ref
	animate     bool
}

// key returns row i's key.
func (k *listKeys) key(i int) any {
	if k.list.Key == nil {
		return i
	}
	return k.list.Key(i)
}

// cursor returns the cursor's row, -1 for none.
func (k *listKeys) cursor() int {
	s := k.list.Selection
	c := s.Cursor()
	if c == nil {
		return -1
	}
	n := k.list.Count()
	if k.at >= 0 && k.at < n && k.key(k.at) == c {
		return k.at
	}
	for i := range n {
		if k.key(i) == c {
			k.at = i
			return i
		}
	}
	return -1
}

// reveal scrolls row i into view, when it is not.
func (k *listKeys) reveal(i int) {
	switch {
	case k.first < 0:
		k.ref.ScrollToIndex(i, k.animate)
	case i < k.first:
		k.ref.ScrollToIndexAt(i, 0, 0, k.animate)
	case i > k.last:
		k.ref.ScrollToIndexAt(i, 1, 0, k.animate)
	}
}

// moveTo puts the cursor on row i, choosing it alone, or with extend the
// rows from the anchor to it.
func (k *listKeys) moveTo(i int, extend bool) {
	n := k.list.Count()
	if n == 0 {
		return
	}
	i = min(max(i, 0), n-1)
	s := k.list.Selection
	k.at = i
	if extend && s.multiple && s.anchor != nil {
		k.extendTo(i)
	} else {
		s.Set(k.key(i))
	}
	k.reveal(i)
}

// extendTo chooses the rows from the anchor to row i, keeping the anchor.
func (k *listKeys) extendTo(i int) {
	s := k.list.Selection
	anchor := s.anchor
	from := i
	for j := range k.list.Count() {
		if k.key(j) == anchor {
			from = j
			break
		}
	}
	lo, hi := min(from, i), max(from, i)
	keys := make([]any, 0, hi-lo+1)
	for j := lo; j <= hi; j++ {
		keys = append(keys, k.key(j))
	}
	s.Set(keys...)
	s.cursor, s.anchor = k.key(i), anchor
	s.changed()
}

// step moves the cursor by delta rows, from the first row when it is on
// none.
func (k *listKeys) step(delta int, extend bool) {
	c := k.cursor()
	if c < 0 {
		k.moveTo(0, false)
		return
	}
	k.moveTo(c+delta, extend)
}

// page is how many rows a page key moves: a viewport's worth.
func (k *listKeys) page() int {
	if k.first < 0 {
		return 10 * k.columns
	}
	return max(k.last-k.first, 1)
}

// activate activates row i.
func (k *listKeys) activate(i int) {
	if i >= 0 && k.list.OnActivate != nil {
		k.list.OnActivate(i)
	}
}

// bind binds the keys while the list holds them.
func (k *listKeys) bind(cx *core.Scope) {
	repeat := core.KeyOptions{Repeat: true}
	s := k.list.Selection
	cx.OnKeyWith("up", repeat, func() { k.step(-k.columns, false) })
	cx.OnKeyWith("down", repeat, func() { k.step(k.columns, false) })
	if k.columns > 1 {
		cx.OnKeyWith("left", repeat, func() { k.step(-1, false) })
		cx.OnKeyWith("right", repeat, func() { k.step(1, false) })
	}
	cx.OnKey("home", func() { k.moveTo(0, false) })
	cx.OnKey("end", func() { k.moveTo(k.list.Count()-1, false) })
	cx.OnKeyWith("pageup", repeat, func() { k.step(-k.page(), false) })
	cx.OnKeyWith("pagedown", repeat, func() { k.step(k.page(), false) })
	cx.OnKey("enter", func() { k.activate(k.cursor()) })
	if s.multiple {
		cx.OnKeyWith("shift+up", repeat, func() { k.step(-k.columns, true) })
		cx.OnKeyWith("shift+down", repeat, func() { k.step(k.columns, true) })
		if k.columns > 1 {
			cx.OnKeyWith("shift+left", repeat, func() { k.step(-1, true) })
			cx.OnKeyWith("shift+right", repeat, func() { k.step(1, true) })
		}
		cx.OnKey("shift+home", func() { k.moveTo(0, true) })
		cx.OnKey("shift+end", func() { k.moveTo(k.list.Count()-1, true) })
		cx.OnKey("mod+a", func() {
			keys := make([]any, k.list.Count())
			for i := range keys {
				keys[i] = k.key(i)
			}
			cursor := s.cursor
			s.Set(keys...)
			if cursor != nil {
				s.cursor = cursor
			}
			s.changed()
		})
		cx.OnKey("space", func() {
			if c := k.cursor(); c >= 0 {
				s.Toggle(k.key(c))
			}
		})
	} else {
		cx.OnKey("space", func() {
			if c := k.cursor(); c >= 0 {
				s.Set(k.key(c))
			}
		})
	}
	if k.list.keys != nil {
		k.list.keys(cx, k)
	}
}

// press chooses row i as a press with mods does: alone, toggled with
// Mod (or a tap in a multiple selection on a touch screen), or the range
// from the anchor with Shift.
func (k *listKeys) press(i int, e core.PressEvent, touch bool) {
	s := k.list.Selection
	m := e.Modifiers
	k.at = i
	switch {
	case s.multiple && m.Shift && s.anchor != nil:
		k.extendTo(i)
	case s.multiple && (m.Platform || m.Control || touch):
		s.Toggle(k.key(i))
	default:
		s.Set(k.key(i))
	}
	if e.Clicks == 2 || touch && !s.multiple {
		k.activate(i)
	}
}

// Render implements core.Node.
func (l List) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	onDesktop := !touch(cx)
	columns := max(l.Columns, 1)
	ref := l.Ref
	if ref == nil {
		ref = &core.Ref{}
	}
	k := &listKeys{list: &l, columns: columns, first: -1, last: -1, ref: ref, animate: !reduceMotion(cx)}
	count := l.Count
	if count == nil {
		count = func() int { return 0 }
	}
	l.Count = count
	selectable := l.Selection != nil
	// focused is the keyboard focus on the list; held counts the times
	// the list took the keys, 0 while it holds none, so taking them again
	// binds them anew, above another list's.
	focused, held := core.NewSignal(cx, false), core.NewSignal(cx, 0)
	take := func() { held.Update(func(n int) int { return n + 1 }) }

	row := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(rowHeight(cx) - 8),
		Padding: core.Symmetric(4, 10), CornerRadius: RadiusControl, MinWidth: core.Pt(0)}
	interactive := selectable || l.OnActivate != nil
	if interactive {
		row.Cursor = core.CursorDefault
	}
	// look is a row's colors, and the ring of the keyboard's cursor while
	// ringed says so.
	look := func(ringed bool) core.Style {
		c := colors.Now()
		s := core.Style{Color: c.Text}
		if interactive {
			s.Selected = &core.Style{Background: c.Selection}
			if onDesktop {
				s.Hover = &core.Style{Background: c.Hover}
			} else {
				s.Active = &core.Style{Background: c.Pressed}
			}
		}
		if ringed {
			s.OutlineWidth, s.OutlineColor, s.OutlineOffset = 2, c.Info, -2
		}
		return s.Merge(l.RowStyle)
	}
	cell := func(cx *core.Scope, r core.ListRow) core.Node {
		content := []core.Node{}
		if l.Row != nil {
			content = append(content, l.Row(cx, r))
		}
		v := core.View{Style: row, StyleFunc: func() core.Style { return look(false) }, Children: content}
		if l.RowLabel != nil {
			v.LabelFunc = func() string { return l.RowLabel(r.Index()) }
		}
		if l.expanded != nil {
			v.Expanded = func() bool { return l.expanded(r.Index()) }
		}
		if !selectable && l.OnActivate == nil {
			return v
		}
		v.Role, v.NoFocus = core.RoleButton, true
		v.OnPress = func(e core.PressEvent) {
			take()
			i := r.Index()
			if selectable {
				k.press(i, e, !onDesktop)
			} else if e.Clicks == 2 || !onDesktop {
				k.activate(i)
			}
		}
		if selectable {
			s := l.Selection
			v.Selected = func() bool { return s.Has(k.key(r.Index())) }
			// The cursor's ring shows while the keyboard is on the list.
			v.StyleFunc = func() core.Style { return look(focused.Get() && s.Cursor() == k.key(r.Index())) }
		}
		return v
	}
	size := l.EstimatedSize
	if size == 0 {
		size = rowHeight(cx)
	}
	list := core.List{Count: count, Key: l.Key, Row: cell, EstimatedSize: size, NumColumns: columns, Gap: l.Gap, ColumnGap: l.Gap,
		Header: l.Header, Footer: l.Footer, Empty: l.Empty, OnLoadMore: l.OnLoadMore, Ref: ref,
		ViewabilityConfig: core.ViewabilityConfig{ItemVisiblePercentThreshold: 100},
		OnViewableItemsChange: func(v core.ViewableItems) {
			k.first, k.last = -1, -1
			for _, tok := range v.Viewable {
				if k.first < 0 || tok.Index < k.first {
					k.first = tok.Index
				}
				k.last = max(k.last, tok.Index)
			}
		},
		Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Padding: core.All(4)}}
	outer := core.View{TestID: l.TestID, Label: l.Label, Style: core.Style{MinHeight: core.Pt(0), CornerRadius: RadiusControl}.Merge(l.Style),
		Children: []core.Node{list}}
	if !selectable {
		return outer
	}
	outer.Children = append(outer.Children, core.Dynamic(func(cx *core.Scope) core.Node {
		if held.Get() > 0 {
			k.bind(cx)
		}
		return nil
	}))
	if onDesktop {
		// On desktops the list is one Tab stop, which takes the keys while
		// it has the keyboard focus.
		outer.OnPress = func(core.PressEvent) { take() }
		outer.OnFocus = func() { focused.Set(true); take() }
		outer.OnBlur = func() { focused.Set(false); held.Set(0) }
		outer.StyleFunc = func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} }
	}
	return outer
}
