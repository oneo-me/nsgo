package base

import (
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/core"
)

// The list Select and Combobox open, its state machine and how it presents
// itself, with no look, as NSH's listbox.rs:
//
//   - listShown is the rows the items show for a query: a case-insensitive
//     substring of the titles, in their order, adjacent items of one Group
//     under a header, a section with no match dropped. What every query
//     needs of the items is made once per change of them (listIndex), and
//     a query holding the last one searches only its matches.
//   - listbox holds that in signals with the highlight, whose moves wake
//     the two rows they leave and reach.
//   - listbox.present lays it out in a dropdown: rows of a virtualized list
//     in the popup's scroll view, so the popup is as tall as its rows up to
//     the room it has. The app's RenderItem gets what an item is, never its
//     state: the row's Selected and Focused states are its classes.

const (
	// typeaheadReset is how long typeahead keeps what was typed.
	typeaheadReset = time.Second
	// listPage is the rows a page moves by before the list says how many
	// show.
	listPage = 10
	// listLit is how many signals the highlight is spread over: more than
	// the rows a popup shows.
	listLit = 32
	// listInView is the percent of a row that shows when it is in view.
	listInView = 99.5
)

// SelectItem is an item of a Select or a Combobox.
type SelectItem struct {
	Value, Title string
	Disabled     bool
	// Group is the section it is in: adjacent items of one group show
	// under one header.
	Group string
}

// SelectEntry is what RenderItem draws: what the item is, and its Index
// among the items.
type SelectEntry struct {
	Value, Title string
	Disabled     bool
	Group        string
	Index        int
}

// SelectGroup is what RenderGroup draws.
type SelectGroup struct{ Title string }

// listRow is a row of a list: the header of the section of items titled
// Title, the first of which is first, or item, which is -1 for a header.
type listRow struct {
	header bool
	title  string
	first  int
	item   int
}

// listIndex is what the list keeps of its items for every query.
type listIndex struct {
	items []SelectItem
	// folded are the titles in lower case, and section each item's
	// section's first item.
	folded  []string
	section []int
}

func newListIndex(items []SelectItem) *listIndex {
	x := &listIndex{items: items, folded: make([]string, len(items)), section: make([]int, len(items))}
	start := 0
	for i, item := range items {
		if i > 0 && items[i-1].Group != item.Group {
			start = i
		}
		x.section[i] = start
		x.folded[i] = strings.ToLower(item.Title)
	}
	return x
}

// matching are the items whose folded title holds query, among among
// when given: the matches of a query this one holds.
func (x *listIndex) matching(query string, among []int) []int {
	out := make([]int, 0, len(x.items))
	hit := func(i int) {
		if query == "" || strings.Contains(x.folded[i], query) {
			out = append(out, i)
		}
	}
	if among != nil {
		for _, i := range among {
			hit(i)
		}
		return out
	}
	for i := range x.items {
		hit(i)
	}
	return out
}

// rowItem is an item shown and its row.
type rowItem struct{ row, item int }

// listHeaderKey keeps a header's key whatever the query hides.
type listHeaderKey struct {
	title string
	first int
}

// listShown is the rows the items show for a query.
type listShown struct {
	rows []listRow
	// items are the items shown, in order, and enabled the enabled ones.
	items, enabled []rowItem
	keys           []any
}

func newListShown(x *listIndex, matches []int) *listShown {
	s := &listShown{rows: make([]listRow, 0, len(matches)), keys: make([]any, 0, len(matches))}
	section := -1
	for _, i := range matches {
		item := &x.items[i]
		if start := x.section[i]; start != section {
			section = start
			if item.Group != "" {
				s.keys = append(s.keys, listHeaderKey{item.Group, start})
				s.rows = append(s.rows, listRow{header: true, title: item.Group, first: start, item: -1})
			}
		}
		at := rowItem{len(s.rows), i}
		s.rows = append(s.rows, listRow{item: i})
		s.keys = append(s.keys, item.Value)
		s.items = append(s.items, at)
		if !item.Disabled {
			s.enabled = append(s.enabled, at)
		}
	}
	return s
}

// rowOf is item's row, -1 when it is not shown.
func (s *listShown) rowOf(item int) int {
	i := sort.Search(len(s.items), func(i int) bool { return s.items[i].item >= item })
	if i < len(s.items) && s.items[i].item == item {
		return s.items[i].row
	}
	return -1
}

func (s *listShown) isEnabled(item int) bool {
	i := sort.Search(len(s.enabled), func(i int) bool { return s.enabled[i].item >= item })
	return i < len(s.enabled) && s.enabled[i].item == item
}

// step is where the highlight goes from item from by a move among the
// enabled items shown, -1 when none is.
func (s *listShown) step(from int, by listMove) int {
	return stepAmong(s.enabled, s.rowOf(from), by)
}

// follow is where the highlight goes when the items or the rows change: to
// value's item while it is shown and enabled, else to the first enabled
// item.
func (s *listShown) follow(items []SelectItem, value string) int {
	for _, e := range s.enabled {
		if items[e.item].Value == value {
			return e.item
		}
	}
	if len(s.enabled) > 0 {
		return s.enabled[0].item
	}
	return -1
}

// listMove is a move of the highlight; PageDown and PageUp count n.
type listMove struct {
	kind moveKind
	n    int
}

type moveKind uint8

const (
	moveNext moveKind = iota
	movePrevious
	moveFirst
	moveLast
	movePageDown
	movePageUp
)

// stepAmong is where the highlight goes by among order, the enabled items
// shown, from the row at (-1: none, or a row of no enabled item: from its
// place). Select's and Command's lists share it.
func stepAmong(order []rowItem, at int, by listMove) int {
	last := len(order) - 1
	if last < 0 {
		return -1
	}
	before, on := 0, false
	if at >= 0 {
		before = sort.Search(len(order), func(i int) bool { return order[i].row >= at })
		on = before < len(order) && order[before].row == at
	}
	n := max(by.n, 1)
	var k int
	switch {
	case by.kind == moveFirst:
	case by.kind == moveLast:
		k = last
	case at < 0:
		if by.kind == movePrevious || by.kind == movePageUp {
			k = last
		}
	case by.kind == moveNext:
		k = before
		if on {
			k++
		}
		if k > last {
			k = 0
		}
	case by.kind == movePrevious:
		k = before - 1
		if k < 0 {
			k = last
		}
	case by.kind == movePageDown:
		from := before
		if !on {
			from = max(before-1, 0)
		}
		k = min(from+n, last)
	default:
		k = max(min(before, last)-n, 0)
	}
	return order[k].item
}

// findTyped is the enabled item of order typed jumps to from the row at:
// the first, going on from it and around, whose folded text starts with
// it. One character typed again moves on to the next starting with it, as
// native lists do; more keep the item while it still matches.
func findTyped(order []rowItem, at int, folded []string, typed string) int {
	first, size := utf8.DecodeRuneInString(typed)
	if typed == "" {
		return -1
	}
	repeated := strings.Trim(typed, string(first)) == ""
	prefix := typed
	if repeated {
		prefix = typed[:size]
	}
	start := 0
	if at >= 0 {
		start = sort.Search(len(order), func(i int) bool {
			if repeated {
				return order[i].row > at
			}
			return order[i].row >= at
		})
	}
	for k := range order {
		if i := order[(start+k)%len(order)].item; strings.HasPrefix(folded[i], prefix) {
			return i
		}
	}
	return -1
}

// typeahead is what was typed less than a second apart.
type typeahead struct {
	typed string
	at    time.Time
}

// push adds r typed at at, and is what was typed since the last pause.
func (t *typeahead) push(r rune, at time.Time) string {
	if t.at.IsZero() || at.Sub(t.at) > typeaheadReset {
		t.typed = ""
	}
	t.at = at
	t.typed += string(unicode.ToLower(r))
	return t.typed
}

func (t *typeahead) clear() { *t = typeahead{} }

// listQuery says where the query comes from: the app's Controlled when it
// sets one, else the search field's, which each opening starts at Default.
type listQuery struct {
	controlled func() *string
	def        func() string
}

// listbox is the list's state.
type listbox struct {
	cx         *core.Scope
	items      func() []SelectItem
	searchable func() bool
	query      listQuery
	own        *core.Signal[string]
	index      *core.Memo[*listIndex]
	shown      *core.Memo[*listShown]
	// highlight is the highlighted item, -1 none, and lit the same as each
	// row reads it.
	highlight *core.Signal[int]
	lit       [listLit]*core.Signal[int]
	// highlighted is its value, which it follows when the items change.
	highlighted *string
	open        bool
	// first and last are the rows in view, known once the list said.
	first, last int
	known       bool
	reveal      int
	list        *core.Ref
	pointer     *core.Point
	ahead       typeahead
	// a11y is the item whose row takes the screen reader's focus.
	a11y *core.Signal[int]
}

func newListbox(cx *core.Scope, items func() []SelectItem, searchable func() bool, query listQuery) *listbox {
	lb := &listbox{cx: cx, items: items, searchable: searchable, query: query, own: core.NewSignal(cx, ""),
		highlight: core.NewSignal(cx, -1), reveal: -1, list: new(core.Ref), a11y: core.NewSignal(cx, -1)}
	for i := range lb.lit {
		lb.lit[i] = core.NewSignal(cx, -1)
	}
	lb.index = core.NewMemoFunc(cx, func() *listIndex { return newListIndex(items()) }, func(a, b *listIndex) bool { return a == b })
	var lastIndex *listIndex
	var lastQuery string
	var lastMatches []int
	lb.shown = core.NewMemoFunc(cx, func() *listShown {
		x, q := lb.index.Get(), strings.ToLower(lb.queryNow())
		var among []int
		if x == lastIndex && lastQuery != "" && strings.Contains(q, lastQuery) {
			among = lastMatches
		}
		matches := x.matching(q, among)
		lastIndex, lastQuery, lastMatches = x, q, matches
		return newListShown(x, matches)
	}, func(a, b *listShown) bool { return a == b })
	// Other items or rows: the highlight follows its value.
	cx.Effect(func() {
		s, x := lb.shown.Get(), lb.index.Get()
		cx.Untrack(func() {
			if lb.highlighted == nil {
				return
			}
			next := lb.highlight.GetUntracked()
			if !s.isEnabled(next) || x.items[next].Value != *lb.highlighted {
				next = s.follow(x.items, *lb.highlighted)
			}
			if next != lb.highlight.GetUntracked() {
				lb.setHighlight(next, false)
			}
		})
	})
	// The app's query changed while open: as a typed one, the first match.
	if query.controlled != nil {
		var last *string
		cx.Effect(func() {
			now := query.controlled()
			same := now == last || now != nil && last != nil && *now == *last
			last = now
			if !same && now != nil && lb.open {
				cx.Untrack(lb.highlightFirst)
			}
		})
	}
	return lb
}

// queryNow is the query, tracked: what the field shows and the rows match.
func (lb *listbox) queryNow() string {
	if lb.query.controlled != nil {
		if q := lb.query.controlled(); q != nil {
			return *q
		}
	}
	return lb.own.Get()
}

func (lb *listbox) defaultQuery() string {
	if lb.query.def != nil {
		return core.Untracked(lb.cx, lb.query.def)
	}
	return ""
}

// setHighlight highlights item; by a key it is revealed and screen readers
// hear of it.
func (lb *listbox) setHighlight(item int, keyboard bool) {
	lb.highlighted = nil
	x := lb.index.GetUntracked()
	if item >= 0 && item < len(x.items) {
		lb.highlighted = &x.items[item].Value
	}
	if was := lb.highlight.GetUntracked(); was != item {
		lb.cx.Batch(func() {
			if was >= 0 {
				lb.lit[was%listLit].Set(-1)
			}
			if item >= 0 {
				lb.lit[item%listLit].Set(item)
			}
			lb.highlight.Set(item)
		})
	}
	if keyboard && item >= 0 {
		lb.revealItem(item)
		lb.tellScreenReader(item)
	}
}

// isHighlighted reports it, tracked: a move wakes only its two rows.
func (lb *listbox) isHighlighted(item int) bool { return lb.lit[item%listLit].Get() == item }

// moveBy moves the highlight by a key.
func (lb *listbox) moveBy(by listMove) {
	if next := lb.shown.GetUntracked().step(lb.highlight.GetUntracked(), by); next >= 0 {
		lb.setHighlight(next, true)
	}
}

// typed is the enabled item a key r jumps to from item from.
func (lb *listbox) typed(r rune, from int) int {
	s := lb.shown.GetUntracked()
	return findTyped(s.enabled, s.rowOf(from), lb.index.GetUntracked().folded, lb.ahead.push(r, lb.cx.Now()))
}

// page is how many rows a page is: those in view.
func (lb *listbox) page() int {
	if !lb.known {
		return listPage
	}
	return max(lb.last+1-lb.first, 1)
}

// opened starts the query, the pointer and typeahead again and highlights
// the item initial picks.
func (lb *listbox) opened(initial func(s *listShown, items []SelectItem) int) {
	lb.own.Set(lb.defaultQuery())
	lb.open, lb.pointer, lb.known, lb.reveal = true, nil, false, -1
	lb.a11y.Set(-1)
	lb.ahead.clear()
	lb.setHighlight(initial(lb.shown.GetUntracked(), lb.index.GetUntracked().items), true)
}

// closed leaves the query at its default.
func (lb *listbox) closed() {
	lb.open = false
	lb.own.Set(lb.defaultQuery())
	lb.a11y.Set(-1)
	lb.ahead.clear()
}

// edited is the user typing query in the search field: while the app sets
// no query it is the field's, and the first match is highlighted.
func (lb *listbox) edited(query string) {
	if lb.query.controlled != nil && lb.query.controlled() != nil {
		return
	}
	lb.own.Set(query)
	lb.highlightFirst()
}

func (lb *listbox) highlightFirst() {
	lb.setHighlight(lb.shown.GetUntracked().step(-1, listMove{kind: moveFirst}), true)
}

// revealItem brings item into view, once, when it is not wholly there.
func (lb *listbox) revealItem(item int) {
	row := lb.shown.GetUntracked().rowOf(item)
	switch {
	case row < 0:
	case lb.known:
		lb.scrollTo(row)
	default:
		lb.reveal = row
	}
}

func (lb *listbox) scrollTo(row int) {
	switch {
	case row >= lb.first && row <= lb.last:
	case row < lb.first:
		// Up to the item, with its section's header when it is the first
		// of its section.
		if rows := lb.shown.GetUntracked().rows; row > 0 && rows[row-1].header {
			row--
		}
		lb.list.ScrollToIndexAt(row, 0, 0, false)
	default:
		lb.list.ScrollToIndexAt(row, 1, 0, false)
	}
}

// viewable hears which rows are in view.
func (lb *listbox) viewable(v core.ViewableItems) {
	lb.known = len(v.Viewable) > 0
	if !lb.known {
		return
	}
	lb.first, lb.last = v.Viewable[0].Index, v.Viewable[len(v.Viewable)-1].Index
	if r := lb.reveal; r >= 0 {
		lb.reveal = -1
		lb.scrollTo(r)
	}
}

// tellScreenReader moves its focus to item's row or, while a search field
// edits, announces the title, so the field keeps the focus.
func (lb *listbox) tellScreenReader(item int) {
	if !core.Untracked(lb.cx, lb.cx.Env().ScreenReader) {
		return
	}
	if core.Untracked(lb.cx, lb.searchable) {
		lb.cx.Announce(lb.index.GetUntracked().items[item].Title)
		return
	}
	lb.a11y.Set(item)
}

// hovered is the pointer moving over item's row, to at: it is highlighted
// unless the pointer is where it was, a row scrolled under a still pointer.
func (lb *listbox) hovered(item int, at core.Point) {
	if lb.pointer != nil && *lb.pointer == at {
		return
	}
	lb.pointer = &at
	items := lb.index.GetUntracked().items
	if item < len(items) && !items[item].Disabled && lb.highlight.GetUntracked() != item {
		lb.setHighlight(item, false)
	}
}

// typeAhead jumps the highlight to the item r, with what was typed before
// it, leads to.
func (lb *listbox) typeAhead(r rune) {
	if item := lb.typed(r, lb.highlight.GetUntracked()); item >= 0 {
		lb.setHighlight(item, true)
	}
}

// keys binds the keys of the open list while open holds: Up and Down move
// the highlight around the ends, Home and End to the first and last, Page
// Up and Down by a page, Return chooses and, without a search field, Space.
// With a search field they fire while it edits, Home and End staying its.
func (lb *listbox) keys(open func() bool, choose func(int)) core.Node {
	return core.Show{When: open, Then: func(*core.Scope) core.Node {
		return core.Dynamic(func(cx *core.Scope) core.Node {
			searchable := lb.searchable()
			when := core.KeyNotEditing
			if searchable {
				when = core.KeyAlways
			}
			bind := func(key string, when core.KeyWhen, repeat bool, f func()) {
				cx.OnKeyWith(key, core.KeyOptions{When: when, Repeat: repeat}, f)
			}
			move := func(key string, when core.KeyWhen, by func() listMove) {
				bind(key, when, true, func() { lb.moveBy(by()) })
			}
			fixed := func(kind moveKind) func() listMove { return func() listMove { return listMove{kind: kind} } }
			move("up", when, fixed(movePrevious))
			move("down", when, fixed(moveNext))
			move("home", core.KeyNotEditing, fixed(moveFirst))
			move("end", core.KeyNotEditing, fixed(moveLast))
			move("pageup", when, func() listMove { return listMove{movePageUp, lb.page()} })
			move("pagedown", when, func() listMove { return listMove{movePageDown, lb.page()} })
			accept := func() {
				if item := lb.highlight.GetUntracked(); item >= 0 {
					choose(item)
				}
			}
			bind("enter", when, false, accept)
			if !searchable {
				bind("space", core.KeyNotEditing, false, accept)
			}
			return nil
		})
	}}
}

// typeaheadKeys binds the keys typeahead takes, letters and digits, while
// active holds and no text input edits, to typed with the character.
func typeaheadKeys(active func() bool, typed func(rune)) core.Node {
	return core.Show{When: active, Then: func(cx *core.Scope) core.Node {
		for _, r := range "abcdefghijklmnopqrstuvwxyz0123456789" {
			cx.OnKeyWith(string(r), core.KeyOptions{When: core.KeyNotEditing}, func() { typed(r) })
		}
		return nil
	}}
}

// listSearch are the search field's parts.
type listSearch struct {
	class, placeholder, label string
	style                     core.Style
	// field draws the input, the input itself when nil.
	field func(core.Input) core.Node
}

// listFooter is the part below the rows.
type listFooter struct {
	content core.Node
	class   string
	style   core.Style
}

// listParts are what the list takes of its component.
type listParts struct {
	frame       dropdownFrame
	list, item  part
	group       part
	search      *listSearch
	footer      *listFooter
	cover       *dropdownCover
	renderItem  func(SelectEntry) core.Node
	renderGroup func(SelectGroup) core.Node
	empty       core.Node
}

// listHooks are what the list asks of its component.
type listHooks struct {
	open     func() bool
	selected func(value string) bool
	choose   func(item int)
	// queryChanged hears the user typing in the search field.
	queryChanged func(string)
	dismiss      func(Reason)
	shown        func(bool)
}

// present is the list while it is open.
func (lb *listbox) present(cx *core.Scope, anchor *core.Ref, width *core.Signal[core.Points], onContent bool, p listParts, h listHooks) core.Node {
	return dropdown{anchor: anchor, onContent: onContent, width: width, cover: p.cover, frame: p.frame, open: h.open, dismiss: h.dismiss, shown: h.shown,
		content: func(onSheet bool) []core.Node { return lb.content(onSheet, p, h) }}.node(cx)
}

func (lb *listbox) content(onSheet bool, p listParts, h listHooks) []core.Node {
	var nodes []core.Node
	if f := p.search; f != nil {
		nodes = append(nodes, core.Show{When: lb.searchable, Then: func(*core.Scope) core.Node {
			return drawnField(f.field, core.Input{Value: lb.queryNow, Placeholder: f.placeholder, Label: f.label, Class: f.class, Style: f.style,
				// Desktops type into it at once; a touch screen's keyboard
				// comes when the user asks.
				AutoFocus: !onSheet,
				OnChange: func(text string) {
					lb.edited(text)
					if h.queryChanged != nil {
						h.queryChanged(text)
					}
				}})
		}})
	}
	rows := core.List{Ref: lb.list, Viewport: core.ListViewportParent, EstimatedSize: 32,
		Count: func() int { return len(lb.shown.Get().rows) },
		Key: func(i int) any {
			if keys := lb.shown.GetUntracked().keys; i < len(keys) {
				return keys[i]
			}
			return i
		},
		Row:                   func(cx *core.Scope, r core.ListRow) core.Node { return lb.row(cx, r, p, h) },
		ViewabilityConfig:     core.ViewabilityConfig{ItemVisiblePercentThreshold: listInView},
		OnViewableItemsChange: lb.viewable}
	listStyle := core.Style{FlexShrink: 1, MinHeight: core.Pt(0)}.Merge(p.list.Style)
	nodes = append(nodes, core.Scroll{Role: core.RoleListbox, Class: p.list.Class, Style: listStyle, StyleFunc: p.list.Fn, Children: []core.Node{rows}})
	if p.empty != nil {
		nodes = append(nodes, core.Show{When: func() bool { return len(lb.shown.Get().rows) == 0 },
			Then: func(*core.Scope) core.Node { return p.empty }})
	}
	if f := p.footer; f != nil && f.content != nil {
		nodes = append(nodes, core.View{Class: f.class, Style: f.style, Children: []core.Node{f.content}})
	}
	return nodes
}

// row is the row at r: a section's header or an item.
func (lb *listbox) row(cx *core.Scope, r core.ListRow, p listParts, h listHooks) core.Node {
	entry := core.NewMemo(cx, func() listRow {
		if rows := lb.shown.Get().rows; r.Index() < len(rows) {
			return rows[r.Index()]
		}
		return listRow{item: -1}
	})
	return core.Show{When: func() bool { return entry.Get().header },
		Then: func(cx *core.Scope) core.Node { return lb.headerRow(entry, p) },
		Else: func(cx *core.Scope) core.Node { return lb.itemRow(cx, entry, p, h) }}
}

func (lb *listbox) headerRow(entry *core.Memo[listRow], p listParts) core.Node {
	content := core.Node(core.Text{ContentFunc: func() string { return entry.Get().title }})
	if p.renderGroup != nil {
		content = core.Dynamic(func(*core.Scope) core.Node {
			e := entry.Get()
			if !e.header {
				return nil
			}
			return p.renderGroup(SelectGroup{Title: e.title})
		})
	}
	return core.View{Role: core.RoleHeader, NoFocus: true, Class: p.group.Class, Style: p.group.Style, StyleFunc: p.group.Fn,
		Children: []core.Node{content}}
}

func (lb *listbox) itemRow(cx *core.Scope, row *core.Memo[listRow], p listParts, h listHooks) core.Node {
	index := func() int { return row.Get().item }
	item := core.NewMemo(cx, func() SelectEntry {
		i := index()
		if items := lb.index.Get().items; i >= 0 && i < len(items) {
			it := items[i]
			return SelectEntry{Value: it.Value, Title: it.Title, Disabled: it.Disabled, Group: it.Group, Index: i}
		}
		return SelectEntry{Index: -1}
	})
	disabled := func() bool { return item.Get().Disabled }
	highlighted := core.NewMemo(cx, func() bool { i := index(); return i >= 0 && lb.isHighlighted(i) })
	selected := core.NewMemo(cx, func() bool { e := item.Get(); return e.Index >= 0 && h.selected(e.Value) })
	me := new(core.Ref)
	cx.Effect(func() {
		if i := index(); i >= 0 && lb.a11y.Get() == i {
			cx.Untrack(func() {
				me.SetAccessibilityFocus()
				lb.a11y.Set(-1)
			})
		}
	})
	desktop := core.Untracked(cx, cx.Env().Desktop)
	downed := false
	choose := func() {
		if !core.Untracked(cx, h.open) {
			return
		}
		if e := item.GetUntracked(); e.Index >= 0 && !e.Disabled {
			h.choose(e.Index)
		}
	}
	content := core.Node(core.Text{ContentFunc: func() string { return item.Get().Title }})
	if p.renderItem != nil {
		content = core.Dynamic(func(*core.Scope) core.Node {
			e := item.Get()
			if e.Index < 0 {
				return nil
			}
			return p.renderItem(e)
		})
	}
	children := []core.Node{content}
	if c := p.cover; c != nil {
		children = append(children, core.Show{When: selected.Get, Then: func(*core.Scope) core.Node {
			return core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
				NoFocus: true, PointerEvents: core.PointerEventsNone, Ref: c.view,
				OnLayout: func(core.Bounds) { c.shown.Set(true) }}
		}})
	}
	return core.View{Ref: me, Role: core.RoleOption, NoFocus: true, Disabled: disabled, Selected: selected.Get,
		LabelFunc:  func() string { return item.Get().Title },
		PartStates: func() core.PartStates { return core.PartStates{Focused: highlighted.Get()} },
		Class:      p.item.Class, Style: p.item.Style, StyleFunc: p.item.Fn,
		// The pointer highlights a row, as long as no button is down.
		OnMouseMove: func(e core.MouseEvent) {
			if e.PressedButton == core.MouseButtonNone {
				if i := row.GetUntracked().item; i >= 0 {
					lb.hovered(i, e.PagePosition)
				}
			}
		},
		// Desktops choose as the button goes down, so a search field keeps
		// the keyboard focus; the press that ends it chooses nothing more.
		OnPressDown: func(core.PressEvent) {
			if desktop {
				downed = true
				choose()
			}
		},
		OnPress: func(core.PressEvent) {
			if !downed {
				choose()
			}
			downed = false
		},
		Children: children}
}

// drawnField is in as field draws it, itself when field is nil.
func drawnField(field func(core.Input) core.Node, in core.Input) core.Node {
	if field != nil {
		return field(in)
	}
	return in
}

// selectedTitles is a trigger's value: the titles of values' items in
// values' order, joined by ", "; none while no item is.
func selectedTitles(items []SelectItem, values []string) core.AccessibilityValue {
	var titles []string
	for _, v := range values {
		if i := slices.IndexFunc(items, func(it SelectItem) bool { return it.Value == v }); i >= 0 {
			titles = append(titles, items[i].Title)
		}
	}
	return core.AccessibilityValue{Text: strings.Join(titles, ", ")}
}
