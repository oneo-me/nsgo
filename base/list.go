package base

import (
	"cmp"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

// List is a long run of the app's rows over core.List, built only near
// what shows, side by side in Columns for a grid. It looks like nothing:
// the app sizes it and styles its rows (ItemClass, ItemStyle), whose
// Selected and Focused states say what is chosen and where the keyboard's
// cursor is.
//
// With a Selection its rows are chosen by a press and by the keyboard:
// arrows, Home, End and the page keys move the cursor, Shift extends a
// multiple selection and Mod+A chooses every row, Space toggles the
// cursor's row in one, letters move it by typeahead over RowLabel, and
// Enter or a double click activate a row. The list takes these keys while
// it has the keyboard focus, or after a press on a row until another list
// takes them.
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
	// TapSelects has a tap only select on touch screens, and a second
	// activate, for rows a tap should not leave.
	TapSelects bool
	// RowLabel is what assistive technology reads for row i, its text
	// when nil, and what typeahead matches.
	RowLabel func(i int) string
	// Columns lays rows side by side, 1 when 0; the arrows then move
	// along a line and between lines.
	Columns int
	// EstimatedSize is a row's expected height, 20 when 0.
	EstimatedSize core.Points
	Gap           core.Points
	Header        core.Node
	Footer        core.Node
	// Empty shows while Count is 0.
	Empty core.Node
	// OnLoadMore runs as the end of the rows comes near, for the next page.
	OnLoadMore func()
	// Label names the list for assistive technology.
	Label string
	// Ref is the list of rows', for the app's scrolls.
	Ref       *core.Ref
	TestID    string
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	// ListClass and ListStyle are the list of rows', whose padding is the
	// content's.
	ListClass string
	ListStyle core.Style
	ItemClass string
	ItemStyle core.Style
	// ItemStyleFunc is each row's style following what it reads.
	ItemStyleFunc func() core.Style
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
	// announce has the screen reader's focus follow the cursor to row i.
	announce func(i int)
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
	if extend && s.Multiple() && s.Anchor() != nil {
		k.extendTo(i)
	} else {
		s.Set(k.key(i))
	}
	k.reveal(i)
	k.announce(i)
}

// extendTo chooses the rows from the anchor to row i, keeping the anchor.
func (k *listKeys) extendTo(i int) {
	s := k.list.Selection
	anchor := s.Anchor()
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
	s.SetRange(anchor, k.key(i), keys...)
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

// typed moves the cursor to the next row whose label starts with prefix,
// from the row after the cursor when one character repeats, as a tree's.
func (k *listKeys) typed(prefix string) {
	n := k.list.Count()
	chars := []rune(prefix)
	repeated := true
	for _, c := range chars {
		repeated = repeated && c == chars[0]
	}
	if repeated {
		prefix = string(chars[0])
	}
	start := max(k.cursor(), 0)
	if repeated && k.cursor() >= 0 {
		start++
	}
	for i := range n {
		row := (start + i) % n
		if strings.HasPrefix(strings.ToLower(k.list.RowLabel(row)), prefix) {
			k.moveTo(row, false)
			return
		}
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
	if s.Multiple() {
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
			cursor := s.Cursor()
			s.Set(keys...)
			if cursor != nil {
				s.SetCursor(cursor)
			}
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
}

// press chooses row i as a press with mods does: alone, toggled with
// Mod (or a tap in a multiple selection on a touch screen), or the range
// from the anchor with Shift.
func (k *listKeys) press(i int, e core.PressEvent, touch bool) {
	s := k.list.Selection
	m := e.Modifiers
	k.at = i
	switch {
	case s.Multiple() && m.Shift && s.Anchor() != nil:
		k.extendTo(i)
	case s.Multiple() && (m.Platform || m.Control || touch):
		s.Toggle(k.key(i))
	default:
		s.Set(k.key(i))
	}
	if e.Clicks >= 2 || touch && !s.Multiple() && !k.list.TapSelects {
		k.activate(i)
	}
}

// Render implements core.Node.
func (l List) Render(cx *core.Scope) core.Node {
	env := cx.Env()
	onDesktop := core.Untracked(cx, env.Desktop)
	columns := max(l.Columns, 1)
	ref := l.Ref
	if ref == nil {
		ref = &core.Ref{}
	}
	// spoken is the row the screen reader's focus goes to, -1 for none.
	spoken := core.NewSignal(cx, -1)
	k := &listKeys{list: &l, columns: columns, first: -1, last: -1, ref: ref, animate: !core.Untracked(cx, env.ReduceMotion),
		announce: func(i int) {
			if core.Untracked(cx, env.ScreenReader) {
				spoken.Set(i)
			}
		}}
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
	cell := func(cx *core.Scope, r core.ListRow) core.Node {
		content := []core.Node{}
		if l.Row != nil {
			content = append(content, l.Row(cx, r))
		}
		v := core.View{Class: l.ItemClass, Style: l.ItemStyle, StyleFunc: l.ItemStyleFunc, Children: content}
		if l.RowLabel != nil {
			v.LabelFunc = func() string { return l.RowLabel(r.Index()) }
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
			me := new(core.Ref)
			v.Ref, v.Role = me, core.RoleOption
			v.Selected = func() bool { return s.Has(k.key(r.Index())) }
			v.PartStates = func() core.PartStates {
				return core.PartStates{Focused: focused.Get() && s.Cursor() == k.key(r.Index())}
			}
			cx.Effect(func() {
				if i := spoken.Get(); i >= 0 && i == r.Index() {
					cx.Untrack(func() {
						me.SetAccessibilityFocus()
						spoken.Set(-1)
					})
				}
			})
		}
		return v
	}
	list := core.List{Count: count, Key: l.Key, Row: cell, EstimatedSize: cmp.Or(l.EstimatedSize, 20), NumColumns: columns, Gap: l.Gap, ColumnGap: l.Gap,
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
		Class: l.ListClass, Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}.Merge(l.ListStyle)}
	outer := core.View{Class: l.Class, TestID: l.TestID, Label: l.Label, Style: core.Style{MinHeight: core.Pt(0)}.Merge(l.Style),
		StyleFunc: l.StyleFunc, Children: []core.Node{list}}
	if !selectable {
		return outer
	}
	outer.Role = core.RoleListbox
	outer.Children = append(outer.Children, core.Dynamic(func(cx *core.Scope) core.Node {
		if held.Get() == 0 {
			return nil
		}
		k.bind(cx)
		if l.RowLabel == nil {
			return nil
		}
		return typedPrefix(cx, func() bool { return true }, k.typed)
	}))
	if onDesktop {
		// On desktops the list is one Tab stop, which takes the keys while
		// it has the keyboard focus.
		outer.OnPress = func(core.PressEvent) { take() }
		outer.OnFocus = func() { focused.Set(true); take() }
		outer.OnBlur = func() { focused.Set(false); held.Set(0) }
	}
	return outer
}
