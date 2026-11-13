package base

import (
	"slices"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// The keys of a desktop menu, as native menus and WAI-ARIA's menu
// pattern take them: while a level is open, Down and Up move the focus
// through its enabled items around the ends, Home and End to the first
// and last, Page Down and Page Up to the last and first in view and a
// view on from there, stopping at the ends; Right opens a submenu with
// the focus on its first item and Left closes it, the focus back on its
// trigger. The keys go on from the item the focus is on or the pointer
// last came over, else from an end, so they reach a menu a press opened.
// A menu the keyboard opened focuses its first item. A level on its way
// out takes no keys.

// menuItem is an item of a level the keys move through: a MenuItem or a
// Submenu's trigger.
type menuItem struct {
	ref      *core.Ref
	disabled func() bool
	// open opens a submenu, the focus on its first item.
	open func()
}

// menuLevel is a level of a desktop menu: its items, the one the keys go
// on from and the scroll view the items are in.
type menuLevel struct {
	cx      *core.Scope
	items   []*menuItem
	current *menuItem
	// scroll is the scroll view the items are in, panel the popup.
	scroll, panel *core.Ref
	// close closes a submenu, the focus back on its trigger.
	close func()
	// focusFirst asks the next opening to focus the first item.
	focusFirst bool
}

func newMenuLevel(cx *core.Scope) *menuLevel {
	return &menuLevel{cx: cx, scroll: new(core.Ref), panel: new(core.Ref)}
}

// add adds an item, which the level forgets as the scope ends.
func (l *menuLevel) add(cx *core.Scope, ref *core.Ref, disabled func() bool, open func()) *menuItem {
	if disabled == nil {
		disabled = func() bool { return false }
	}
	item := &menuItem{ref: ref, disabled: disabled, open: open}
	l.items = append(l.items, item)
	cx.Cleanup(func() {
		l.items = slices.DeleteFunc(l.items, func(i *menuItem) bool { return i == item })
		if l.current == item {
			l.current = nil
		}
	})
	return item
}

// enabled is the enabled items in tree order.
func (l *menuLevel) enabled() []*menuItem {
	items := slices.DeleteFunc(slices.Clone(l.items), func(i *menuItem) bool { return core.Untracked(l.cx, i.disabled) })
	slices.SortStableFunc(items, func(a, b *menuItem) int {
		switch {
		case a.ref.Before(b.ref):
			return -1
		case b.ref.Before(a.ref):
			return 1
		}
		return 0
	})
	return items
}

func (l *menuLevel) focus(i *menuItem) {
	l.current = i
	i.ref.Focus()
}

// opened focuses the first item once the level's items are there, if
// the keyboard opened it.
func (l *menuLevel) opened(cx *core.Scope) {
	if !l.focusFirst {
		return
	}
	l.focusFirst = false
	cx.Post(func() {
		if items := l.enabled(); len(items) > 0 {
			l.focus(items[0])
		}
	})
}

// move moves the focus by s, around the ends.
func (l *menuLevel) move(s step) {
	items := l.enabled()
	if len(items) == 0 {
		return
	}
	from := slices.Index(items, l.current)
	to := 0
	switch {
	case s == stepFirst || s == stepNext && from < 0:
	case s == stepLast || s == stepPrevious && from < 0:
		to = len(items) - 1
	case s == stepNext:
		to = (from + 1) % len(items)
	default:
		to = (from + len(items) - 1) % len(items)
	}
	l.focus(items[to])
}

// page moves the focus to the last enabled item in view, or the first
// going up, and from there a view on, stopping at the ends.
func (l *menuLevel) page(down bool) {
	items := l.enabled()
	view, ok := l.scroll.Bounds()
	if len(items) == 0 || !ok {
		return
	}
	n := len(items)
	top, bottom := make([]core.Points, n), make([]core.Points, n)
	for i, it := range items {
		b, _ := it.ref.Bounds()
		top[i], bottom[i] = b.Origin.Y, b.Origin.Y+b.Size.Height
	}
	start, h := view.Origin.Y, view.Size.Height
	shows := func(i int) bool { return top[i] >= start && bottom[i] <= start+h }
	from := slices.Index(items, l.current)
	to := -1
	if down {
		for i := range n {
			if shows(i) {
				to = i
			}
		}
		if to < 0 || from >= to {
			at := max(from, 0)
			to = at
			for i := at; i < n && bottom[i] <= bottom[at]+h; i++ {
				to = i
			}
			if to == at && at < n-1 {
				to = at + 1
			}
		}
	} else {
		for i := n - 1; i >= 0; i-- {
			if shows(i) {
				to = i
			}
		}
		if to < 0 || from >= 0 && from <= to {
			at := from
			if at < 0 {
				at = n - 1
			}
			to = at
			for i := at; i >= 0 && top[i] >= top[at]-h; i-- {
				to = i
			}
			if to == at && at > 0 {
				to = at - 1
			}
		}
	}
	l.focus(items[to])
}

// keys binds the level's keys while open holds.
func (l *menuLevel) keys(open func() bool) core.Node {
	return core.Show{When: open, Then: func(cx *core.Scope) core.Node {
		opts := core.KeyOptions{When: core.KeyNotEditing, Repeat: true}
		env := cx.Env()
		cx.OnKeyWith("down", opts, func() { l.move(stepNext) })
		cx.OnKeyWith("up", opts, func() { l.move(stepPrevious) })
		cx.OnKeyWith("home", opts, func() { l.move(stepFirst) })
		cx.OnKeyWith("end", opts, func() { l.move(stepLast) })
		cx.OnKeyWith("pagedown", opts, func() { l.page(true) })
		cx.OnKeyWith("pageup", opts, func() { l.page(false) })
		// Into a submenu and out, mirrored right to left.
		into, out := "right", "left"
		if core.Untracked(cx, env.RTL) {
			into, out = out, into
		}
		cx.OnKeyWith(into, opts, func() {
			if c := l.current; c != nil && c.open != nil && !core.Untracked(l.cx, c.disabled) {
				c.open()
			}
		})
		if l.close != nil {
			cx.OnKeyWith(out, opts, l.close)
		}
		return nil
	}}
}

// heldToRoom puts a level's items in a scroll view inside its popup,
// which the core holds to the room on its side, with the padding and the
// gap the popup's style gives moved onto them, so its scroll bar stays
// against the popup's edge.
func heldToRoom(o core.Overlay, popup part, l *menuLevel) (core.Overlay, part) {
	inner := func() core.Style {
		s := popup.Style
		if popup.Fn != nil {
			s = s.Merge(popup.Fn())
		}
		return core.Style{Padding: s.Padding, Gap: s.Gap, RowGap: s.RowGap, ColumnGap: s.ColumnGap}
	}
	outer := popup
	outer.Style.Padding, outer.Style.Gap, outer.Style.RowGap, outer.Style.ColumnGap = core.Edges{}, 0, 0, 0
	if fn := popup.Fn; fn != nil {
		outer.Fn = func() core.Style {
			s := fn()
			s.Padding, s.Gap, s.RowGap, s.ColumnGap = core.Edges{}, 0, 0, 0
			return s
		}
	}
	o.HoldToRoom = true
	o.Children = []core.Node{core.Scroll{Ref: l.scroll, Style: core.Style{FlexShrink: 1, MinHeight: core.Pt(0)},
		Children: []core.Node{core.View{StyleFunc: inner, Children: o.Children}}}}
	return o, parts.Part(outer)
}
