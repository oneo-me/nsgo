package base

import (
	"sort"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

// CommandItem is a command of a Command.
type CommandItem struct {
	ID, Label string
	// Keywords are found as its Label is, without showing.
	Keywords []string
	// Group is the section it is in: adjacent items of one group show
	// under one header.
	Group string
	// Shortcut shows as what it is pressed with: "mod+n". The app binds it.
	Shortcut          string
	Disabled, Checked bool
}

// CommandEntry is what RenderItem draws: what the item is, with its
// shortcut as this platform shows it.
type CommandEntry struct {
	ID, Label         string
	Keywords          []string
	Group             string
	Shortcut          string
	ShortcutLabel     string
	Disabled, Checked bool
}

// CommandGroup is what RenderGroup draws.
type CommandGroup struct{ Heading string }

// CommandRef is a Command's ref.
type CommandRef struct {
	focus     func()
	highlight func(id string) bool
	count     func() int
}

// Focus puts the keyboard on the search field, or the command without one.
func (r *CommandRef) Focus() {
	if r.focus != nil {
		r.focus()
	}
}

// Highlight highlights the item of id, reporting whether it is shown and
// enabled.
func (r *CommandRef) Highlight(id string) bool { return r.highlight != nil && r.highlight(id) }

// MatchedCount is how many items show.
func (r *CommandRef) MatchedCount() int {
	if r.count == nil {
		return 0
	}
	return r.count()
}

// Command is a command palette: a search field over the app's commands
// filtered natively by label and keywords, in a virtualized list under
// their sections' headers. The first enabled match is highlighted while
// the field keeps the keyboard focus, so Up, Down, Page Up and Down move
// it, Return chooses and Escape clears the query, then reports OnCancel.
// Without a search field, Home, End, Space and letters by typeahead work
// too. The rows' Focused and Checked styles are the app's: it has no look.
type Command struct {
	Items func() []CommandItem
	// Query makes the query the app's, nil for the field's own, from
	// DefaultQuery.
	Query        func() *string
	DefaultQuery string
	// NoSearch shows no field, and every item.
	NoSearch bool
	// NoFilter leaves the items to the app, as a search it answers itself.
	NoFilter    bool
	Placeholder string
	AutoFocus   bool
	// Loading says a search is under way: the list is busy and Empty waits.
	Loading func() bool
	// NoSeparator leaves the separators between sections out.
	NoSeparator bool
	// SearchLabel names the field for assistive technology, over Label.
	SearchLabel string
	Ref         *CommandRef

	Class          string
	Style          core.Style
	StyleFunc      func() core.Style
	SearchBarClass string
	SearchBarStyle core.Style
	// SearchBarStyleFunc is the bar's style following what it reads.
	SearchBarStyleFunc func() core.Style
	SearchClass        string
	SearchStyle        core.Style
	// SearchField draws the search field, as a styled form does.
	SearchField func(core.Input) core.Node
	ListClass   string
	ListStyle   core.Style
	ItemClass   string
	ItemStyle   core.Style
	// ItemStyleFunc is each row's style following what it reads, and so
	// are GroupStyleFunc and SeparatorStyleFunc.
	ItemStyleFunc      func() core.Style
	GroupClass         string
	GroupStyle         core.Style
	GroupStyleFunc     func() core.Style
	SeparatorClass     string
	SeparatorStyle     core.Style
	SeparatorStyleFunc func() core.Style

	Header, Footer, Empty      core.Node
	SearchPrefix, SearchSuffix core.Node
	RenderItem                 func(CommandEntry) core.Node
	RenderGroup                func(CommandGroup) core.Node

	OnSelect          func(id string)
	OnHighlightChange func(highlighted *string)
	OnQueryChange     func(query string)
	OnCancel          func()
	Label             string
	TestID            string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// commandIndex is what the palette keeps of its items for every query.
type commandIndex struct {
	items []CommandItem
	// folded is what queries match, section each item's section's first
	// item, and ids each ID's first item.
	folded  []string
	section []int
	ids     map[string]int
}

func newCommandIndex(items []CommandItem) *commandIndex {
	x := &commandIndex{items: items, folded: make([]string, len(items)), section: make([]int, len(items)), ids: make(map[string]int, len(items))}
	start := 0
	for i, item := range items {
		if i > 0 && items[i-1].Group != item.Group {
			start = i
		}
		x.section[i] = start
		var text strings.Builder
		text.WriteString(strings.ToLower(item.Label))
		for _, k := range item.Keywords {
			text.WriteByte('\n')
			text.WriteString(strings.ToLower(k))
		}
		x.folded[i] = text.String()
		if _, ok := x.ids[item.ID]; !ok {
			x.ids[item.ID] = i
		}
	}
	return x
}

// matching are the items whose folded text holds query, among among when
// given: the matches of a query this one holds.
func (x *commandIndex) matching(query string, among []int) []int {
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

type commandRowKind uint8

const (
	commandItemRow commandRowKind = iota
	commandHeaderRow
	commandSeparatorRow
)

// commandRow is a row of the palette.
type commandRow struct {
	kind    commandRowKind
	heading string
	first   int
	item    int
}

type commandSeparatorKey struct{ first int }

// commandShown is the rows for a query.
type commandShown struct {
	rows           []commandRow
	items, enabled []rowItem
	keys           []any
}

func newCommandShown(x *commandIndex, matches []int, separators bool) *commandShown {
	s := &commandShown{rows: make([]commandRow, 0, len(matches))}
	section := -1
	for _, i := range matches {
		item := &x.items[i]
		if start := x.section[i]; start != section {
			// A separator only once a section was shown before this one:
			// never leading, trailing or doubled.
			if separators && section >= 0 {
				s.keys = append(s.keys, commandSeparatorKey{start})
				s.rows = append(s.rows, commandRow{kind: commandSeparatorRow, item: -1})
			}
			section = start
			if item.Group != "" {
				s.keys = append(s.keys, listHeaderKey{item.Group, start})
				s.rows = append(s.rows, commandRow{kind: commandHeaderRow, heading: item.Group, first: start, item: -1})
			}
		}
		at := rowItem{len(s.rows), i}
		s.rows = append(s.rows, commandRow{item: i})
		s.keys = append(s.keys, item.ID)
		s.items = append(s.items, at)
		if !item.Disabled {
			s.enabled = append(s.enabled, at)
		}
	}
	return s
}

func (s *commandShown) rowOf(item int) int {
	i := sort.Search(len(s.items), func(i int) bool { return s.items[i].item >= item })
	if i < len(s.items) && s.items[i].item == item {
		return s.items[i].row
	}
	return -1
}

func (s *commandShown) isEnabled(item int) bool {
	i := sort.Search(len(s.enabled), func(i int) bool { return s.enabled[i].item >= item })
	return i < len(s.enabled) && s.enabled[i].item == item
}

// palette is a Command's state.
type palette struct {
	cx        *core.Scope
	c         Command
	index     *core.Memo[*commandIndex]
	shown     *core.Memo[*commandShown]
	highlight *core.Signal[int]
	lit       [listLit]*core.Signal[int]
	// highlighted is the highlighted item's ID.
	highlighted *string
	first, last int
	known       bool
	reveal      int
	list        *core.Ref
	pointer     *core.Point
	ahead       typeahead
	a11y        *core.Signal[int]
}

func (p *palette) setHighlight(item int, keyboard bool) {
	var id *string
	x := p.index.GetUntracked()
	if item >= 0 && item < len(x.items) {
		id = &x.items[item].ID
	}
	changed := !sameValue(p.highlighted, id)
	p.highlighted = id
	if was := p.highlight.GetUntracked(); was != item {
		p.cx.Batch(func() {
			if was >= 0 {
				p.lit[was%listLit].Set(-1)
			}
			if item >= 0 {
				p.lit[item%listLit].Set(item)
			}
			p.highlight.Set(item)
		})
	}
	if changed && p.c.OnHighlightChange != nil {
		p.c.OnHighlightChange(id)
	}
	if keyboard && item >= 0 {
		p.revealItem(item)
		p.tellScreenReader(item)
	}
}

func (p *palette) moveBy(by listMove) {
	s := p.shown.GetUntracked()
	if next := stepAmong(s.enabled, s.rowOf(p.highlight.GetUntracked()), by); next >= 0 {
		p.setHighlight(next, true)
	}
}

// follow moves the highlight as the rows change: reset, for a new query,
// to the first enabled item, else it stays while it can.
func (p *palette) follow(reset, initial bool) {
	s, x := p.shown.GetUntracked(), p.index.GetUntracked()
	next := -1
	if !reset && p.highlighted != nil {
		if i, ok := x.ids[*p.highlighted]; ok && s.isEnabled(i) {
			next = i
		}
	}
	if next < 0 {
		next = stepAmong(s.enabled, -1, listMove{kind: moveFirst})
	}
	p.setHighlight(next, !initial && (reset || next != p.highlight.GetUntracked()))
}

func (p *palette) page() int {
	if !p.known {
		return listPage
	}
	return max(p.last+1-p.first, 1)
}

func (p *palette) revealItem(item int) {
	row := p.shown.GetUntracked().rowOf(item)
	switch {
	case row < 0:
	case p.known:
		p.scrollTo(row)
	default:
		p.reveal = row
	}
}

func (p *palette) scrollTo(row int) {
	switch {
	case row >= p.first && row <= p.last:
	case row < p.first:
		if rows := p.shown.GetUntracked().rows; row > 0 && rows[row-1].kind == commandHeaderRow {
			row--
		}
		p.list.ScrollToIndexAt(row, 0, 0, false)
	default:
		p.list.ScrollToIndexAt(row, 1, 0, false)
	}
}

func (p *palette) viewable(v core.ViewableItems) {
	p.known = len(v.Viewable) > 0
	if !p.known {
		return
	}
	p.first, p.last = v.Viewable[0].Index, v.Viewable[len(v.Viewable)-1].Index
	if r := p.reveal; r >= 0 {
		p.reveal = -1
		p.scrollTo(r)
	}
}

func (p *palette) tellScreenReader(item int) {
	if !core.Untracked(p.cx, p.cx.Env().ScreenReader) {
		return
	}
	if !p.c.NoSearch {
		p.cx.Announce(p.index.GetUntracked().items[item].Label)
		return
	}
	p.a11y.Set(item)
}

func (p *palette) hovered(item int, at core.Point) {
	if p.pointer != nil && *p.pointer == at {
		return
	}
	p.pointer = &at
	items := p.index.GetUntracked().items
	if item < len(items) && !items[item].Disabled && p.highlight.GetUntracked() != item {
		p.setHighlight(item, false)
	}
}

func (p *palette) typeAhead(r rune) {
	s := p.shown.GetUntracked()
	typed := p.ahead.push(r, p.cx.Now())
	if next := findTyped(s.enabled, s.rowOf(p.highlight.GetUntracked()), p.index.GetUntracked().folded, typed); next >= 0 {
		p.setHighlight(next, true)
	}
}

func (p *palette) choose(item int) {
	items := p.index.GetUntracked().items
	if item >= 0 && item < len(items) && !items[item].Disabled && p.c.OnSelect != nil {
		p.c.OnSelect(items[item].ID)
	}
}

func (p *palette) accept() {
	if item := p.highlight.GetUntracked(); item >= 0 {
		p.choose(item)
	}
}

// Render implements core.Node.
func (c Command) Render(cx *core.Scope) core.Node {
	own := core.NewSignal(cx, c.DefaultQuery)
	query := func() string {
		if c.Query != nil {
			if q := c.Query(); q != nil {
				return *q
			}
		}
		return own.Get()
	}
	p := &palette{cx: cx, c: c, highlight: core.NewSignal(cx, -1), reveal: -1, list: new(core.Ref), a11y: core.NewSignal(cx, -1)}
	for i := range p.lit {
		p.lit[i] = core.NewSignal(cx, -1)
	}
	p.index = core.NewMemoFunc(cx, func() *commandIndex { return newCommandIndex(c.Items()) }, func(a, b *commandIndex) bool { return a == b })
	var lastIndex *commandIndex
	var lastQuery string
	var lastMatches []int
	p.shown = core.NewMemoFunc(cx, func() *commandShown {
		x := p.index.Get()
		q := ""
		if !c.NoSearch && !c.NoFilter {
			q = strings.ToLower(strings.TrimSpace(query()))
		}
		var among []int
		if x == lastIndex && lastQuery != "" && strings.Contains(q, lastQuery) {
			among = lastMatches
		}
		matches := x.matching(q, among)
		lastIndex, lastQuery, lastMatches = x, q, matches
		return newCommandShown(x, matches, !c.NoSeparator)
	}, func(a, b *commandShown) bool { return a == b })
	// Each change of the rows: a new query highlights its first enabled
	// item, new items keep the highlighted one while they can.
	var seen *string
	cx.Effect(func() {
		q := query()
		p.shown.Get()
		cx.Untrack(func() {
			initial := seen == nil
			reset := initial || *seen != q
			seen = &q
			p.follow(reset, initial)
		})
	})
	root, field := new(core.Ref), new(core.Ref)
	fieldFocused, rootFocused := core.NewSignal(cx, false), core.NewSignal(cx, false)
	focused := func() bool {
		if c.NoSearch {
			return rootFocused.Get()
		}
		return fieldFocused.Get()
	}
	edited := func(text string) {
		if c.Query == nil || c.Query() == nil {
			own.Set(text)
		}
		if c.OnQueryChange != nil {
			c.OnQueryChange(text)
		}
	}
	keys := core.Show{When: focused, Then: func(cx *core.Scope) core.Node {
		when := core.KeyAlways
		if c.NoSearch {
			when = core.KeyNotEditing
		}
		bind := func(key string, when core.KeyWhen, repeat bool, f func()) {
			cx.OnKeyWith(key, core.KeyOptions{When: when, Repeat: repeat}, f)
		}
		bind("up", when, true, func() { p.moveBy(listMove{kind: movePrevious}) })
		bind("down", when, true, func() { p.moveBy(listMove{kind: moveNext}) })
		// A search field's own while it edits.
		bind("home", core.KeyNotEditing, true, func() { p.moveBy(listMove{kind: moveFirst}) })
		bind("end", core.KeyNotEditing, true, func() { p.moveBy(listMove{kind: moveLast}) })
		bind("pageup", when, true, func() { p.moveBy(listMove{movePageUp, p.page()}) })
		bind("pagedown", when, true, func() { p.moveBy(listMove{movePageDown, p.page()}) })
		bind("enter", when, false, p.accept)
		if c.NoSearch {
			bind("space", core.KeyNotEditing, false, p.accept)
		}
		// Escape clears a query, and with none it is OnCancel: a focused
		// text field would only leave the field otherwise.
		bind("escape", core.KeyAlways, false, func() {
			if !c.NoSearch && core.Untracked(cx, query) != "" {
				edited("")
			} else if c.OnCancel != nil {
				c.OnCancel()
			}
		})
		return nil
	}}
	typed := typeaheadKeys(func() bool { return c.NoSearch && focused() }, p.typeAhead)
	if c.Ref != nil {
		c.Ref.focus = func() {
			if c.NoSearch {
				root.Focus()
			} else {
				field.Focus()
			}
		}
		c.Ref.highlight = func(id string) bool {
			i, ok := p.index.GetUntracked().ids[id]
			if !ok || !p.shown.GetUntracked().isEnabled(i) {
				return false
			}
			p.setHighlight(i, true)
			return true
		}
		c.Ref.count = func() int { return len(p.shown.GetUntracked().items) }
	}
	var children []core.Node
	if c.Header != nil {
		children = append(children, c.Header)
	}
	if !c.NoSearch {
		var bar []core.Node
		if c.SearchPrefix != nil {
			bar = append(bar, c.SearchPrefix)
		}
		label := c.SearchLabel
		if label == "" {
			label = c.Label
		}
		bar = append(bar, drawnField(c.SearchField, core.Input{Ref: field, Value: query, Placeholder: c.Placeholder, AutoFocus: c.AutoFocus, Label: label,
			// A touch keyboard's return key chooses and keeps the field.
			SubmitBehavior: core.SubmitBehaviorSubmit, Class: c.SearchClass, Style: c.SearchStyle,
			OnFocus: func() { fieldFocused.Set(true) }, OnBlur: func() { fieldFocused.Set(false) },
			OnChange: edited, OnSubmit: func(string) { p.accept() }}))
		if c.SearchSuffix != nil {
			bar = append(bar, c.SearchSuffix)
		}
		children = append(children, core.View{Class: c.SearchBarClass, Style: c.SearchBarStyle, StyleFunc: c.SearchBarStyleFunc, Children: bar})
	}
	rows := core.List{Ref: p.list, Viewport: core.ListViewportParent, EstimatedSize: 32,
		Count: func() int { return len(p.shown.Get().rows) },
		Key: func(i int) any {
			if keys := p.shown.GetUntracked().keys; i < len(keys) {
				return keys[i]
			}
			return i
		},
		Row:                   p.row,
		ViewabilityConfig:     core.ViewabilityConfig{ItemVisiblePercentThreshold: listInView},
		OnViewableItemsChange: p.viewable}
	children = append(children, core.Scroll{Role: core.RoleListbox, Label: c.Label, Busy: c.Loading, Class: c.ListClass,
		Style: core.Style{FlexShrink: 1, MinHeight: core.Pt(0)}.Merge(c.ListStyle), Children: []core.Node{rows}})
	if c.Empty != nil {
		// While a search is under way no item is no answer yet.
		children = append(children, core.Show{When: func() bool { return len(p.shown.Get().rows) == 0 && !is(c.Loading) },
			Then: func(*core.Scope) core.Node { return c.Empty }})
	}
	if c.Footer != nil {
		children = append(children, c.Footer)
	}
	children = append(children, keys, typed)
	return core.View{Ref: root, Class: c.Class, Style: c.Style, StyleFunc: c.StyleFunc, TestID: c.TestID, OnLayout: c.OnLayout, Tooltip: c.Tooltip,
		Focusable: func() bool { return c.NoSearch }, OnFocus: func() { rootFocused.Set(true) }, OnBlur: func() { rootFocused.Set(false) },
		Children: children}
}

func (p *palette) row(cx *core.Scope, r core.ListRow) core.Node {
	c := p.c
	entry := core.NewMemo(cx, func() commandRow {
		if rows := p.shown.Get().rows; r.Index() < len(rows) {
			return rows[r.Index()]
		}
		return commandRow{item: -1}
	})
	kind := core.NewMemo(cx, func() commandRowKind { return entry.Get().kind })
	return core.Dynamic(func(cx *core.Scope) core.Node {
		switch kind.Get() {
		case commandHeaderRow:
			content := core.Node(core.Text{ContentFunc: func() string { return entry.Get().heading }})
			if c.RenderGroup != nil {
				content = core.Dynamic(func(*core.Scope) core.Node {
					e := entry.Get()
					if e.kind != commandHeaderRow {
						return nil
					}
					return c.RenderGroup(CommandGroup{Heading: e.heading})
				})
			}
			return core.View{Role: core.RoleHeader, NoFocus: true, Class: c.GroupClass, Style: c.GroupStyle, StyleFunc: c.GroupStyleFunc, Children: []core.Node{content}}
		case commandSeparatorRow:
			return core.View{NoFocus: true, AccessibilityElementsHidden: func() bool { return true }, Class: c.SeparatorClass, Style: c.SeparatorStyle, StyleFunc: c.SeparatorStyleFunc}
		}
		return p.itemRow(cx, entry)
	})
}

// commandEntry is item as RenderItem draws it.
func commandEntry(cx *core.Scope, item CommandItem) CommandEntry {
	e := CommandEntry{ID: item.ID, Label: item.Label, Keywords: item.Keywords, Group: item.Group, Shortcut: item.Shortcut,
		Disabled: item.Disabled, Checked: item.Checked}
	if item.Shortcut != "" {
		e.ShortcutLabel = core.Untracked(cx, func() string { return cx.KeyLabel(item.Shortcut) })
	}
	return e
}

func (p *palette) itemRow(cx *core.Scope, row *core.Memo[commandRow]) core.Node {
	c := p.c
	index := func() int { return row.Get().item }
	item := func() (CommandItem, bool) {
		i := index()
		if items := p.index.Get().items; i >= 0 && i < len(items) {
			return items[i], true
		}
		return CommandItem{}, false
	}
	disabled := core.NewMemo(cx, func() bool { it, _ := item(); return it.Disabled })
	checked := core.NewMemo(cx, func() bool { it, _ := item(); return it.Checked })
	label := core.NewMemo(cx, func() string { it, _ := item(); return it.Label })
	highlighted := core.NewMemo(cx, func() bool { i := index(); return i >= 0 && p.lit[i%listLit].Get() == i })
	me := new(core.Ref)
	cx.Effect(func() {
		if i := index(); i >= 0 && p.a11y.Get() == i {
			cx.Untrack(func() {
				me.SetAccessibilityFocus()
				p.a11y.Set(-1)
			})
		}
	})
	desktop := core.Untracked(cx, cx.Env().Desktop)
	downed := false
	choose := func() {
		if i := row.GetUntracked().item; i >= 0 {
			p.choose(i)
		}
	}
	content := core.Node(core.Text{ContentFunc: label.Get})
	if c.RenderItem != nil {
		content = core.Dynamic(func(*core.Scope) core.Node {
			it, ok := item()
			if !ok {
				return nil
			}
			return c.RenderItem(commandEntry(cx, it))
		})
	}
	return core.View{Ref: me, Role: core.RoleOption, NoFocus: true, Disabled: disabled.Get, Checked: checked.Get, LabelFunc: label.Get,
		PartStates: func() core.PartStates { return core.PartStates{Focused: highlighted.Get()} },
		Class:      c.ItemClass, Style: c.ItemStyle, StyleFunc: c.ItemStyleFunc,
		OnMouseMove: func(e core.MouseEvent) {
			if e.PressedButton == core.MouseButtonNone {
				if i := row.GetUntracked().item; i >= 0 {
					p.hovered(i, e.PagePosition)
				}
			}
		},
		// Desktops choose as the button goes down, so the search field
		// keeps the keyboard focus; the press that ends it chooses no more.
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
		Children: []core.Node{content}}
}
