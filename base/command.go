package base

import (
	"cmp"
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

// CommandItem is a command of a Command.
type CommandItem struct {
	ID, Label string
	// Description says more of it, for the app's row to draw. Queries do
	// not match it.
	Description string
	// Keywords are found as its Label is, without showing.
	Keywords []string
	// Group is the section it is in: the items of one group show together
	// under one header, wherever they are among the items.
	Group string
	// Shortcut shows as what it is pressed with: "mod+n". The app binds it.
	Shortcut string
	// Icon is the glyph for the app's row to draw, and Leading content to
	// draw in its place. Trailing is content for the row's end. Queries
	// match none of them.
	Icon              *core.SVG
	Leading, Trailing core.Node
	Disabled, Checked bool
}

// CommandEntry is what RenderItem draws: what the item is, with its
// shortcut as this platform shows it.
type CommandEntry struct {
	ID, Label         string
	Description       string
	Keywords          []string
	Group             string
	Shortcut          string
	ShortcutLabel     string
	Icon              *core.SVG
	Leading, Trailing core.Node
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
// matched natively by FilterCommands, or the app's Filter, in a virtualized
// list under their sections' headers, a section showing where its best
// match ranks. The first enabled match is highlighted while
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
	NoFilter bool
	// Filter replaces FilterCommands for a search of the items at hand: it
	// gets them and the query, and must not change them.
	Filter      func(items []CommandItem, query string) []CommandItem
	Placeholder string
	AutoFocus   bool
	// ExpandOnSearch shows no rows, nor Empty, Pending or the error, until
	// the query holds more than spaces, and ExpandedClass and ExpandedStyle
	// join the command's from then on, for a height the rows fill.
	ExpandOnSearch bool
	ExpandedClass  string
	ExpandedStyle  core.Style
	// Loading says a search is under way: the list is busy and Empty waits.
	Loading func() bool
	// Pending takes the rows' place while Loading, and nothing can be
	// chosen. Without it the rows stay as they were.
	Pending core.Node
	// Error says why there are no rows, empty while there is nothing to
	// say: they go and nothing can be chosen, and RenderError draws it.
	Error       func() string
	RenderError func(message string) core.Node
	// InputSource is the input source to select once the search field
	// has the keyboard, as core.Scope.SelectInputSource names it
	// ("com.apple.keylayout.ABC" for Latin commands on macOS), restored as the command goes unless
	// the user chose another meanwhile or KeepInputSource. A source that
	// is not enabled changes nothing.
	InputSource     string
	KeepInputSource bool
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

// FilterCommands is the items the query's words all match, ignoring case,
// without changing items: exact labels first, then labels they start,
// labels holding them and last keywords holding them, ties in the items'
// order. No words match every item.
func FilterCommands(items []CommandItem, query string) []CommandItem {
	x := newCommandIndex(items)
	matches := x.matching(strings.ToLower(query), nil)
	out := make([]CommandItem, len(matches))
	for i, m := range matches {
		out[i] = items[m]
	}
	return out
}

// commandIndex is what the palette keeps of its items for every query.
type commandIndex struct {
	items []CommandItem
	// folded is what queries match, its first labels[i] bytes the label,
	// section each item's group's first item, and ids each ID's first item.
	folded  []string
	labels  []int
	section []int
	ids     map[string]int
}

func newCommandIndex(items []CommandItem) *commandIndex {
	x := &commandIndex{items: items, folded: make([]string, len(items)), labels: make([]int, len(items)), section: make([]int, len(items)), ids: make(map[string]int, len(items))}
	groups := make(map[string]int)
	for i, item := range items {
		start, ok := groups[item.Group]
		if !ok {
			start = i
			groups[item.Group] = i
		}
		x.section[i] = start
		var text strings.Builder
		text.WriteString(strings.ToLower(item.Label))
		x.labels[i] = text.Len()
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

// matching are the items query's words all match, best first, among among
// when given: the matches of a query this one holds.
func (x *commandIndex) matching(query string, among []int) []int {
	terms := strings.Fields(query)
	if among == nil {
		among = make([]int, len(x.items))
		for i := range among {
			among[i] = i
		}
	}
	if len(terms) == 0 {
		return among
	}
	type match struct{ item, score int }
	found := make([]match, 0, len(among))
	for _, i := range among {
		if score, ok := x.score(i, terms); ok {
			found = append(found, match{i, score})
		}
	}
	slices.SortFunc(found, func(a, b match) int { return cmp.Or(cmp.Compare(a.score, b.score), cmp.Compare(a.item, b.item)) })
	out := make([]int, len(found))
	for i, m := range found {
		out[i] = m.item
	}
	return out
}

// score is how far item i is from what terms ask, lower being better, and
// whether they all match it.
func (x *commandIndex) score(i int, terms []string) (int, bool) {
	label, keywords := x.folded[i][:x.labels[i]], x.folded[i][x.labels[i]:]
	score := 0
	for _, term := range terms {
		switch {
		case label == term:
		case strings.HasPrefix(label, term):
			score++
		case strings.Contains(label, term):
			score += 2
		case strings.Contains(keywords, term):
			score += 3
		default:
			return 0, false
		}
	}
	return score, true
}

// picked is the indices of what a Filter gave back, in its order.
func (x *commandIndex) picked(items []CommandItem) []int {
	out := make([]int, 0, len(items))
	for _, item := range items {
		if i, ok := x.ids[item.ID]; ok {
			out = append(out, i)
		}
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
	// at is each shown item's row.
	at map[int]int
}

func newCommandShown(x *commandIndex, matches []int, separators bool) *commandShown {
	s := &commandShown{rows: make([]commandRow, 0, len(matches)), at: make(map[int]int, len(matches))}
	// The sections in the order of their best matches, each whole.
	var sections []int
	buckets := make(map[int][]int)
	for _, i := range matches {
		start := x.section[i]
		if _, ok := buckets[start]; !ok {
			sections = append(sections, start)
		}
		buckets[start] = append(buckets[start], i)
	}
	for n, start := range sections {
		// A separator only once a section was shown before this one:
		// never leading, trailing or doubled.
		if separators && n > 0 {
			s.keys = append(s.keys, commandSeparatorKey{start})
			s.rows = append(s.rows, commandRow{kind: commandSeparatorRow, item: -1})
		}
		if group := x.items[start].Group; group != "" {
			s.keys = append(s.keys, listHeaderKey{group, start})
			s.rows = append(s.rows, commandRow{kind: commandHeaderRow, heading: group, first: start, item: -1})
		}
		for _, i := range buckets[start] {
			item := &x.items[i]
			at := rowItem{len(s.rows), i}
			s.at[i] = at.row
			s.rows = append(s.rows, commandRow{item: i})
			s.keys = append(s.keys, item.ID)
			s.items = append(s.items, at)
			if !item.Disabled {
				s.enabled = append(s.enabled, at)
			}
		}
	}
	return s
}

func (s *commandShown) rowOf(item int) int {
	if row, ok := s.at[item]; ok {
		return row
	}
	return -1
}

func (s *commandShown) isEnabled(x *commandIndex, item int) bool {
	_, ok := s.at[item]
	return ok && !x.items[item].Disabled
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
		if i, ok := x.ids[*p.highlighted]; ok && s.isEnabled(x, i) {
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
	expanded := func() bool { return !c.ExpandOnSearch || strings.TrimSpace(query()) != "" }
	failure := func() string {
		if c.Error != nil {
			return c.Error()
		}
		return ""
	}
	pending := func() bool { return c.Pending != nil && is(c.Loading) }
	p.index = core.NewMemoFunc(cx, func() *commandIndex { return newCommandIndex(c.Items()) }, func(a, b *commandIndex) bool { return a == b })
	var lastIndex *commandIndex
	var lastQuery string
	var lastMatches []int
	p.shown = core.NewMemoFunc(cx, func() *commandShown {
		x := p.index.Get()
		if !expanded() || failure() != "" || pending() {
			return newCommandShown(x, nil, false)
		}
		if c.Filter != nil && !c.NoSearch && !c.NoFilter {
			return newCommandShown(x, x.picked(c.Filter(x.items, query())), !c.NoSeparator)
		}
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
	fieldFocused, rootFocused, composing := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
	focused := func() bool {
		if c.NoSearch {
			return rootFocused.Get()
		}
		// An input method keeps its keys while it composes.
		return fieldFocused.Get() && !composing.Get()
	}
	if c.InputSource != "" && !c.NoSearch {
		selected := false
		cx.Effect(func() {
			if !selected && fieldFocused.Get() && !composing.Get() {
				selected = true
				cx.SelectInputSource(c.InputSource)
			}
		})
		cx.Cleanup(func() {
			if selected && !c.KeepInputSource {
				cx.RestoreInputSource()
			}
		})
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
			if !ok || !p.shown.GetUntracked().isEnabled(p.index.GetUntracked(), i) {
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
			OnCompositionChange: func(e core.Composition) { composing.Set(e.Composing) },
			OnChange:            edited, OnSubmit: func(string) { p.accept() }}))
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
		Style: core.Style{FlexShrink: 1, MinHeight: core.Pt(0)}.Merge(c.ListStyle),
		StyleFunc: func() core.Style {
			if len(p.shown.Get().rows) == 0 {
				return core.Style{Display: core.DisplayNone}
			}
			return core.Style{}
		}, Children: []core.Node{rows}})
	if c.Empty != nil {
		// While a search is under way no item is no answer yet.
		children = append(children, core.Show{When: func() bool {
			return expanded() && len(p.shown.Get().rows) == 0 && !is(c.Loading) && failure() == ""
		}, Then: func(*core.Scope) core.Node { return c.Empty }})
	}
	if c.Pending != nil {
		children = append(children, core.Show{When: func() bool { return expanded() && pending() && failure() == "" },
			Then: func(*core.Scope) core.Node { return c.Pending }})
	}
	if c.RenderError != nil {
		children = append(children, core.Show{When: func() bool { return expanded() && failure() != "" },
			Then: func(*core.Scope) core.Node {
				return core.Dynamic(func(*core.Scope) core.Node { return c.RenderError(failure()) })
			}})
	}
	if c.Footer != nil {
		children = append(children, c.Footer)
	}
	children = append(children, keys, typed)
	style, class := c.StyleFunc, func() string { return c.Class }
	if c.ExpandOnSearch {
		class = func() string {
			if expanded() {
				return c.Class + " " + c.ExpandedClass
			}
			return c.Class
		}
		style = func() core.Style {
			var s core.Style
			if c.StyleFunc != nil {
				s = c.StyleFunc()
			}
			if expanded() {
				s = s.Merge(c.ExpandedStyle)
			}
			return s
		}
	}
	return core.View{Ref: root, ClassFunc: class, Style: c.Style, StyleFunc: style, TestID: c.TestID, OnLayout: c.OnLayout, Tooltip: c.Tooltip,
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
	e := CommandEntry{ID: item.ID, Label: item.Label, Description: item.Description, Keywords: item.Keywords, Group: item.Group, Shortcut: item.Shortcut,
		Icon: item.Icon, Leading: item.Leading, Trailing: item.Trailing, Disabled: item.Disabled, Checked: item.Checked}
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
