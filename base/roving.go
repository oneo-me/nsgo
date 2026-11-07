package base

import (
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// Roving focus, as NSH's roving.rs: a group whose items are one stop of
// the keyboard focus, between which the arrow keys move it, as WAI-ARIA's
// tab list and radio group patterns have it. Tabs and RadioGroup provide
// a group; each Tab or Radio inside joins it, and one outside any stays a
// stop of its own.
//
// The stop is the item the focus is on, else the current one (the
// selected tab, the checked radio), else the first enabled one: only that
// item is Focusable, so Tab enters and leaves the group in one step. An
// arrow key makes the item it moves to the stop, focuses it and activates
// it: a tab reports a press and a radio asks to be checked, the patterns'
// automatic activation. Right to left, Left and Right trade places; Up
// and Down keep theirs. Disabled items are skipped.

// rovingItem is an item of a group.
type rovingItem struct {
	ref               *core.Ref
	current, disabled func() bool
	activate          func()
	// stop is what the item's view reads: only the items the stop leaves
	// and reaches run again, however many a group has.
	stop *core.Signal[bool]
}

// rovingGroup is a group of items with one stop of the keyboard focus.
type rovingGroup struct {
	cx      *core.Scope
	items   []*rovingItem
	changed *core.Signal[int]
	focus   *core.Signal[*rovingItem]
	// last is the item the focus was on last, where a toolbar's Tab comes
	// back to.
	last     *core.Signal[*rovingItem]
	homeKeys bool
	// toolbar remembers last over the current item, and its axis keeps
	// the keys across it to its items; a zero axis moves on both.
	toolbar bool
	axis    Axis
	hasAxis bool
}

var rovingKey = core.NewKey[*rovingGroup]("base.roving", nil)

// newRoving provides a group to what the scope renders. homeKeys binds
// Home and End to the first and last item.
func newRoving(cx *core.Scope, homeKeys bool) *rovingGroup {
	g := &rovingGroup{cx: cx, changed: core.NewSignal(cx, 0), focus: core.NewSignal[*rovingItem](cx, nil),
		last: core.NewSignal[*rovingItem](cx, nil), homeKeys: homeKeys}
	rovingKey.Provide(cx, g)
	var shown *rovingItem
	cx.Effect(func() {
		next := g.stop()
		if next == shown {
			return
		}
		if shown != nil {
			shown.stop.Set(false)
		}
		if next != nil {
			next.stop.Set(true)
		}
		shown = next
	})
	return g
}

// sorted puts the items in tree order: an Each may mount one between
// others, and order among those not mounted yet is the order they came.
func (g *rovingGroup) sorted() {
	slices.SortStableFunc(g.items, func(a, b *rovingItem) int {
		switch {
		case a.ref.Before(b.ref):
			return -1
		case b.ref.Before(a.ref):
			return 1
		}
		return 0
	})
}

// stop is the item that is the group's Tab stop.
func (g *rovingGroup) stop() *rovingItem {
	g.changed.Get()
	g.sorted()
	enabled := func(i *rovingItem) bool { return !i.disabled() }
	if i := g.focus.Get(); i != nil && slices.Contains(g.items, i) && enabled(i) {
		return i
	}
	var first *rovingItem
	if g.toolbar {
		if i := g.last.Get(); i != nil && slices.Contains(g.items, i) && enabled(i) {
			return i
		}
	}
	for _, i := range g.items {
		if !enabled(i) {
			continue
		}
		if i.current() && !g.toolbar {
			return i
		}
		if first == nil {
			first = i
		}
	}
	return first
}

// current is the first current item's view, read in a computation that
// runs again as items come and go or become current.
func (g *rovingGroup) current() *core.Ref {
	g.changed.Get()
	g.sorted()
	for _, i := range g.items {
		if i.current() {
			return i.ref
		}
	}
	return nil
}

// add adds an item, which the group forgets when the scope ends.
func (g *rovingGroup) add(cx *core.Scope, ref *core.Ref, current, disabled func() bool, activate func()) *rovingItem {
	if current == nil {
		current = func() bool { return false }
	}
	if disabled == nil {
		disabled = func() bool { return false }
	}
	item := &rovingItem{ref: ref, current: current, disabled: disabled, activate: activate, stop: core.NewSignal(cx, false)}
	g.items = append(g.items, item)
	cx.Untrack(func() { g.changed.Update(func(n int) int { return n + 1 }) })
	cx.Cleanup(func() {
		g.items = slices.DeleteFunc(g.items, func(i *rovingItem) bool { return i == item })
		cx.Untrack(func() {
			if g.focus.GetUntracked() == item {
				g.focus.Set(nil)
			}
			if g.last.GetUntracked() == item {
				g.last.Set(nil)
			}
			g.changed.Update(func(n int) int { return n + 1 })
		})
	})
	return item
}

// step is where a key moves the focus.
type step uint8

const (
	stepPrevious step = iota
	stepNext
	stepFirst
	stepLast
)

// move moves the focus by s, around the ends, past disabled items, and
// activates the item it reaches.
func (g *rovingGroup) move(s step) {
	g.sorted()
	enabled := slices.DeleteFunc(slices.Clone(g.items), func(i *rovingItem) bool {
		return core.Untracked(g.cx, i.disabled)
	})
	if len(enabled) == 0 {
		return
	}
	from := slices.Index(enabled, g.focus.GetUntracked())
	to := 0
	switch {
	case s == stepFirst || s == stepNext && from < 0:
	case s == stepLast || s == stepPrevious && from < 0:
		to = len(enabled) - 1
	case s == stepNext:
		to = (from + 1) % len(enabled)
	default:
		to = (from + len(enabled) - 1) % len(enabled)
	}
	if to == from {
		return
	}
	item := enabled[to]
	// The stop first, so the focus finds the item focusable.
	g.focus.Set(item)
	g.last.Set(item)
	item.ref.Focus()
	if item.activate != nil {
		item.activate()
	}
}

// keys binds the group's keys; a Show holds them while the focus is on an
// item, so they take no key from the rest of the app.
func (g *rovingGroup) keys() core.Node {
	return core.Show{When: func() bool { return g.focus.Get() != nil }, Then: func(cx *core.Scope) core.Node {
		opts := core.KeyOptions{When: core.KeyNotEditing, Repeat: true}
		env := cx.Env()
		sided := func(key string, s, mirrored step) {
			cx.OnKeyWith(key, opts, func() {
				if core.Untracked(cx, env.RTL) {
					s = mirrored
				}
				g.move(s)
			})
		}
		if !g.hasAxis || g.axis == Horizontal {
			sided("left", stepPrevious, stepNext)
			sided("right", stepNext, stepPrevious)
		}
		if !g.hasAxis || g.axis == Vertical {
			sided("up", stepPrevious, stepPrevious)
			sided("down", stepNext, stepNext)
		}
		if g.homeKeys {
			sided("home", stepFirst, stepFirst)
			sided("end", stepLast, stepLast)
		}
		return nil
	}}
}

// rovingView puts view in the group around cx, if any: a stop only while
// it is the item the focus is on or would enter at, reporting the focus
// to the group.
func rovingView(cx *core.Scope, v core.View, current func() bool, activate func()) core.View {
	return rovingJoin(cx, rovingKey.Use(cx), v, current, activate)
}

// rovingJoin is rovingView in group g, none when it is nil.
func rovingJoin(cx *core.Scope, g *rovingGroup, v core.View, current func() bool, activate func()) core.View {
	if g == nil {
		return v
	}
	if v.Ref == nil {
		v.Ref = new(core.Ref)
	}
	item := g.add(cx, v.Ref, current, v.Disabled, activate)
	v.Focusable = item.stop.Get
	onFocus, onBlur := v.OnFocus, v.OnBlur
	v.OnFocus = func() {
		g.focus.Set(item)
		g.last.Set(item)
		if onFocus != nil {
			onFocus()
		}
	}
	v.OnBlur = func() {
		if g.focus.GetUntracked() == item {
			g.focus.Set(nil)
		}
		if onBlur != nil {
			onBlur()
		}
	}
	return v
}

// A bar is a Toolbar's or a ToggleGroup's group of the focus: the Toggles,
// Buttons and Links inside it join it, in a ToolbarGroup or a ToggleGroup
// inside a toolbar too, and an arrow key only moves the focus among them.
type bar struct {
	roving   *rovingGroup
	disabled func() bool
	// axis is a toolbar's, which its groups lay out along.
	axis    Axis
	toolbar bool
	// menu is the bar's menu that is open, if any: with one open, the
	// pointer over another menu's trigger opens that one instead, as a
	// menu bar does.
	menu **popup
}

var barKey = core.NewKey[*bar]("base.bar", nil)

// nestedBar is the bar for a part of outer that holds items: the same
// group of the focus, disabled with outer too.
func nestedBar(outer *bar, disabled func() bool) *bar {
	nested := *outer
	nested.disabled = func() bool { return is(disabled) || is(outer.disabled) }
	return &nested
}

// barItem puts v, a Toggle's, Button's or Link's view, in the bar around
// cx, if any: one of its stops, disabled with the bar too. current makes
// it where Tab enters a toggle group on its own.
func barItem(cx *core.Scope, v core.View, current func() bool) core.View {
	b := barKey.Use(cx)
	if b == nil {
		return v
	}
	own := v.Disabled
	v.Disabled = func() bool { return is(own) || is(b.disabled) }
	rovingKey.Provide(cx, b.roving)
	return rovingView(cx, v, current, nil)
}

// popupBar is the bar around a popup being made, which its trigger joins
// (barTrigger), left for what the popup makes from now on: a component
// the app puts in the trigger (a Button) is that one item, not another of
// its own, and the popup's content is no part of the bar.
func popupBar(cx *core.Scope) *bar {
	around := barKey.Use(cx)
	leaveBar(cx)
	return around
}

// leaveBar leaves the bar around cx, if any, for what it makes from now
// on.
func leaveBar(cx *core.Scope) {
	if barKey.Use(cx) != nil {
		barKey.Provide(cx, nil)
		rovingKey.Provide(cx, nil)
	}
}

// barTrigger puts v, a popup's trigger, in b when there is one: one of its
// stops, disabled with the bar too.
func barTrigger(cx *core.Scope, b *bar, v core.View) core.View {
	if b == nil {
		return v
	}
	own := v.Disabled
	v.Disabled = func() bool { return is(own) || is(b.disabled) }
	return rovingJoin(cx, b.roving, v, nil, nil)
}

// barMenu makes st the bar's open menu while it is open, and returns what
// the pointer over its trigger does: with another of the bar's menus
// open, it moves there.
func barMenu(cx *core.Scope, b *bar, st *popup) func(core.MouseEvent) {
	if b == nil || b.menu == nil {
		return nil
	}
	cx.Effect(func() {
		switch {
		case st.open():
			*b.menu = st
		case *b.menu == st:
			*b.menu = nil
		}
	})
	return func(core.MouseEvent) {
		if shown := *b.menu; shown != nil && shown != st {
			shown.set(false, ReasonHover)
			st.set(true, ReasonHover)
		}
	}
}
