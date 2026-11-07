package base

import (
	"cmp"
	"log"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/oneo-me/nsgo/core"
)

// TreeItem is an item of a Tree, with the items under it.
type TreeItem struct {
	// ID identifies it in Expanded, Selected and the events: unique in the
	// tree, never its label.
	ID string
	// Key is what a Selection holds for it, ID when nil.
	Key any
	// Label is what its row shows, what assistive technology says and
	// what typeahead matches.
	Label string
	// Children make it a folder that expands and collapses.
	Children []TreeItem
	// Folder makes it one with no Children given yet, for items whose
	// children are read only as they expand.
	Folder bool
	// Disabled can't be selected, expanded or collapsed by the user.
	Disabled bool
}

// TreeEntry is an item's row as RenderItem draws it: what the item is,
// never its state, which is the row's own.
type TreeEntry struct {
	ID, Label string
	// Depth is 0 at the top, 1 under a top item.
	Depth    int
	Folder   bool
	Disabled bool
}

// treeRoot means a node has no parent.
const treeRoot = -1

// treeNode is a node of the flattened items.
type treeNode struct {
	id, label string
	key       any
	foldered  bool
	parent    int
	depth     int
	// end is the index past its subtree.
	end      int
	disabled bool
}

func (n *treeNode) folder(at int) bool { return n.foldered || n.end > at+1 }

// treeFlat is the items depth first, made once per change of them.
type treeFlat struct {
	nodes []treeNode
	index map[string]int
}

func newTreeFlat(items []TreeItem) *treeFlat {
	f := &treeFlat{index: map[string]int{}}
	f.push(items, treeRoot, 0)
	if d := len(f.nodes) - len(f.index); d > 0 {
		log.Printf("Tree: %d item(s) repeat the id of an item before them: ids are unique in a tree, and only the first expands or is selected", d)
	}
	return f
}

func (f *treeFlat) push(items []TreeItem, parent, depth int) {
	for i := range items {
		item := &items[i]
		at := len(f.nodes)
		key := item.Key
		if key == nil {
			key = item.ID
		}
		f.nodes = append(f.nodes, treeNode{id: item.ID, key: key, label: item.Label, parent: parent, depth: depth, end: at + 1, disabled: item.Disabled, foldered: item.Folder})
		if _, ok := f.index[item.ID]; !ok {
			f.index[item.ID] = at
		}
		f.push(item.Children, at, depth+1)
		f.nodes[at].end = len(f.nodes)
	}
}

func (f *treeFlat) find(id string) int {
	if at, ok := f.index[id]; ok {
		return at
	}
	return -1
}

// findKey is the node a Selection's key names, -1 for none.
func (f *treeFlat) findKey(key any) int {
	if key == nil {
		return -1
	}
	for i := range f.nodes {
		if f.nodes[i].key == key {
			return i
		}
	}
	return -1
}

func (f *treeFlat) folder(n int) bool  { return n >= 0 && n < len(f.nodes) && f.nodes[n].folder(n) }
func (f *treeFlat) enabled(n int) bool { return n >= 0 && n < len(f.nodes) && !f.nodes[n].disabled }

// ancestors are n's, the nearest first.
func (f *treeFlat) ancestors(n int) []int {
	var out []int
	for p := f.nodes[n].parent; p != treeRoot; p = f.nodes[p].parent {
		out = append(out, p)
	}
	return out
}

// children are n's, in order.
func (f *treeFlat) children(n int) []int {
	var out []int
	for c := n + 1; c < f.nodes[n].end; c = f.nodes[c].end {
		out = append(out, c)
	}
	return out
}

// open is which nodes are expanded, the folders of ids, sorted.
func (f *treeFlat) open(ids []string) []int {
	var open []int
	for _, id := range ids {
		if n := f.find(id); f.folder(n) {
			open = append(open, n)
		}
	}
	slices.Sort(open)
	return slices.Compact(open)
}

func (f *treeFlat) entry(n int) TreeEntry {
	node := &f.nodes[n]
	return TreeEntry{ID: node.id, Label: node.label, Depth: node.depth, Folder: node.folder(n), Disabled: node.disabled}
}

// treeRows are the nodes the expanded items show, in order: a collapsed
// subtree skipped whole, so they cost the rows shown.
func treeRows(f *treeFlat, open []int) []int {
	var rows []int
	next := 0
	for at := 0; at < len(f.nodes); {
		rows = append(rows, at)
		for next < len(open) && open[next] < at {
			next++
		}
		if next < len(open) && open[next] == at {
			at++
		} else {
			at = f.nodes[at].end
		}
	}
	return rows
}

// treeRowOf is the row of node n, -1 when it does not show.
func treeRowOf(rows []int, n int) int {
	if at, ok := slices.BinarySearch(rows, n); ok {
		return at
	}
	return -1
}

// treeRowNear is the row of n, else of its nearest ancestor that shows.
func treeRowNear(f *treeFlat, rows []int, n int) int {
	if r := treeRowOf(rows, n); r >= 0 {
		return r
	}
	for _, a := range f.ancestors(n) {
		if r := treeRowOf(rows, a); r >= 0 {
			return r
		}
	}
	return -1
}

// treeStep is where the selection goes from node from, -1 for none, by s,
// among the enabled rows; -1 when none is.
func treeStep(f *treeFlat, rows []int, from int, s step) int {
	count := len(rows)
	at := -1
	if from >= 0 {
		at = treeRowNear(f, rows, from)
	}
	find := func(start, by int) int {
		for k := range count {
			if n := rows[((start+by*k)%count+count)%count]; f.enabled(n) {
				return n
			}
		}
		return -1
	}
	if count == 0 {
		return -1
	}
	switch {
	case s == stepFirst || s == stepNext && at < 0:
		return find(0, 1)
	case s == stepLast || s == stepPrevious && at < 0:
		return find(count-1, -1)
	case s == stepNext:
		return find(at+1, 1)
	}
	return find(at-1, -1)
}

// treeTyped is the enabled node typed jumps to from from, going on from
// it and around: the same character typed again moves on to the next label
// starting with it, more characters keep the node while it still matches.
func treeTyped(f *treeFlat, rows []int, from int, typed string) int {
	chars := []rune(typed)
	count := len(rows)
	if len(chars) == 0 || count == 0 {
		return -1
	}
	repeated := !slices.ContainsFunc(chars, func(c rune) bool { return c != chars[0] })
	prefix := typed
	if repeated {
		prefix = string(chars[0])
	}
	start := 0
	if from >= 0 {
		if r := treeRowNear(f, rows, from); r >= 0 {
			start = r
			if repeated {
				start++
			}
		}
	}
	for k := range count {
		n := rows[(start+k)%count]
		if f.enabled(n) && strings.HasPrefix(strings.ToLower(f.nodes[n].label), prefix) {
			return n
		}
	}
	return -1
}

// treeCursor is the node the keys act on: selected's row, else its
// nearest ancestor that shows, else the first enabled row.
func treeCursor(f *treeFlat, rows []int, selected int) int {
	if selected >= 0 {
		if r := treeRowNear(f, rows, selected); r >= 0 {
			return rows[r]
		}
	}
	return treeStep(f, rows, -1, stepFirst)
}

// treeLit spreads the node the rows follow over signals, as Solid's
// createSelector does, so a move wakes few rows.
const treeLit = 32

type treeFollow struct {
	lit [treeLit]*core.Signal[int]
}

func newTreeFollow(cx *core.Scope, node func() int) *treeFollow {
	l := &treeFollow{}
	for i := range l.lit {
		l.lit[i] = core.NewSignal(cx, -1)
	}
	was := -1
	cx.Effect(func() {
		now := node()
		cx.Untrack(func() {
			if was == now {
				return
			}
			cx.Batch(func() {
				if was >= 0 {
					l.of(was).Set(-1)
				}
				if now >= 0 {
					l.of(now).Set(now)
				}
			})
			was = now
		})
	})
	return l
}

func (l *treeFollow) of(n int) *core.Signal[int] { return l.lit[n%treeLit] }
func (l *treeFollow) is(n int) bool              { return l.of(n).Get() == n }

// Tree is items that hold items, virtualized: only the rows in view are
// made, and the children of a collapsed item never are. The expanded items
// are Expanded and OnExpandedChange, or its own from DefaultExpanded, the
// selected one Selected and OnSelectedChange, or its own. A press selects
// a row and expands or collapses a folder; a double press acts on it
// (OnAction). It is one Tab stop: Up and Down move the selection between
// the rows shown, past disabled items, around the ends, Right expands a
// folder and then moves to its first enabled child, Left collapses it and
// then moves to its parent, Home and End go to the first and last row,
// Return and Space expand or collapse a folder, Return acts on a leaf,
// and letters select by typeahead. It looks like nothing: the app sizes it
// and styles the rows (ItemClass) or draws them (RenderItem).
type Tree struct {
	Items func() []TreeItem
	// Expanded makes the expanded items controlled.
	Expanded        func() []string
	DefaultExpanded []string
	// Selected makes the selected item controlled, nil for none.
	Selected        func() *string
	DefaultSelected string
	Disabled        func() bool
	// Reveal springs the rows after an item to their places as it expands
	// and collapses, as Collapsible's Reveal does.
	Reveal bool
	// Selection holds the selection in place of Selected and the tree's
	// own, any number of rows when Multiple: Shift and Mod extend it as in
	// a List. OnSelectedChange then hears nothing.
	Selection *Selection
	// SelectOnly has a press select a row and leave expanding to Row's
	// TreeRow.Toggle, a chevron's.
	SelectOnly bool
	// Label is what assistive technology calls the tree.
	Label     string
	ListClass string
	ListStyle core.Style
	ItemClass string
	ItemStyle core.Style
	// ItemStyleFunc is each row's style following what it reads.
	ItemStyleFunc func() core.Style
	// RenderItem draws an item's row, for the rows in view; Row does with
	// what the row follows, so its views stay as the cell shows other
	// items.
	RenderItem func(entry TreeEntry) core.Node
	Row        func(cx *core.Scope, row TreeRow) core.Node
	// EstimatedSize is a row's expected height, 20 when 0; Empty shows
	// while there are no items.
	EstimatedSize core.Points
	Empty         core.Node
	// ListRef is the list of rows', for the app's scrolls.
	ListRef *core.Ref
	// OnExpandedChange receives the expanded ids as they would be, then
	// OnExpand or OnCollapse the item.
	OnExpandedChange func(expanded []string)
	OnSelectedChange func(selected string)
	OnExpand         func(id string)
	OnCollapse       func(id string)
	// OnAction hears Return on a selected leaf, or a double press on any
	// item, after the press's own events.
	OnAction func(id string)
	// Ref's Focus and RevealItem are the tree's.
	Ref       *TreeRef
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// TreeRow is the item a cell of a Tree shows, as signals.
type TreeRow struct {
	Row core.ListRow
	// Entry is what the item is, and Key what a Selection holds for it.
	Entry func() TreeEntry
	Key   func() any
	// Expanded says it is expanded, and Toggle expands or collapses it.
	Expanded func() bool
	Toggle   func()
}

// TreeRef is a Tree's ref.
type TreeRef struct {
	root   *core.Ref
	reveal func(id string) bool
}

// Focus takes the keyboard focus.
func (r *TreeRef) Focus() {
	if r.root != nil {
		r.root.Focus()
	}
}

// RevealItem brings the item of id into view: the items above it are
// expanded, as OnExpandedChange and OnExpand report, and its row is
// scrolled to once it shows.
func (r *TreeRef) RevealItem(id string) {
	if r.reveal != nil && !r.reveal(id) {
		log.Printf("Tree.RevealItem: no item %q", id)
	}
}

// Render implements core.Node.
func (t Tree) Render(cx *core.Scope) core.Node {
	own := core.NewSignalFunc(cx, slices.Clone(t.DefaultExpanded), slices.Equal[[]string])
	expanded := func() []string {
		if t.Expanded != nil {
			return t.Expanded()
		}
		return own.Get()
	}
	ownSelected := core.NewSignal(cx, t.DefaultSelected)
	selected := func() string {
		if t.Selected == nil {
			return ownSelected.Get()
		}
		if s := t.Selected(); s != nil {
			return *s
		}
		return ""
	}
	items := func() []TreeItem {
		if t.Items != nil {
			return t.Items()
		}
		return nil
	}
	flat := core.NewMemoFunc(cx, func() *treeFlat { return newTreeFlat(items()) }, func(a, b *treeFlat) bool { return false })
	open := core.NewMemoFunc(cx, func() []int { return flat.Get().open(expanded()) }, slices.Equal[[]int])
	shown := core.NewMemoFunc(cx, func() []int { return treeRows(flat.Get(), open.Get()) }, slices.Equal[[]int])
	current := core.NewMemo(cx, func() int {
		if t.Selection != nil {
			return flat.Get().findKey(t.Selection.Cursor())
		}
		if id := selected(); id != "" {
			return flat.Get().find(id)
		}
		return -1
	})
	cursor := core.NewMemo(cx, func() int { return treeCursor(flat.Get(), shown.Get(), current.Get()) })
	litSelected, litCursor := newTreeFollow(cx, current.Get), newTreeFollow(cx, cursor.Get)
	root, list := new(core.Ref), t.ListRef
	if list == nil {
		list = new(core.Ref)
	}
	// focused is the keyboard focus on the tree; held counts the times a
	// press took the keys, as a List's, so they go on from the row pressed
	// where a press moves no focus.
	focused, held := core.NewSignal(cx, false), core.NewSignal(cx, 0)
	a11y := core.NewSignal(cx, -1)
	env := cx.Env()
	first, last, known := 0, 0, false
	pendingRow := -1
	pendingID := ""
	scrollTo := func(row int) {
		if row >= first && row <= last {
			return
		}
		at := float32(1)
		if row < first {
			at = 0
		}
		list.ScrollToIndexAt(row, at, 0, false)
	}
	bring := func(row int) {
		if known {
			scrollTo(row)
		} else {
			pendingRow = row
		}
	}
	untrackedFlat := func() *treeFlat { return flat.GetUntracked() }
	isOpen := func(n int) bool {
		_, ok := slices.BinarySearch(open.GetUntracked(), n)
		return ok
	}
	cx.Effect(func() {
		shown.Get()
		cx.Untrack(func() {
			if pendingID == "" {
				return
			}
			id := pendingID
			pendingID = ""
			if n := untrackedFlat().find(id); n >= 0 {
				if row := treeRowOf(shown.GetUntracked(), n); row >= 0 {
					bring(row)
				}
			}
		})
	})
	// Each change of what is expanded, not of the items: the rows after
	// it spring to their places.
	var lastFlat *treeFlat
	cx.Effect(func() {
		now := flat.Get()
		open.Get()
		cx.Untrack(func() {
			same := lastFlat == now
			lastFlat = now
			if same && t.Reveal && !env.ReduceMotion() {
				cx.AnimateLayout(core.LayoutAnimation{Update: &revealTransition})
			}
		})
	})
	expandedNow := func() []string { return slices.Clone(core.Untracked(cx, expanded)) }
	report := func(ids []string) {
		if t.Expanded == nil {
			own.Set(ids)
		}
		if t.OnExpandedChange != nil {
			t.OnExpandedChange(ids)
		}
	}
	setOpen := func(n int, next bool) {
		f := untrackedFlat()
		if !f.folder(n) || isOpen(n) == next {
			return
		}
		id := f.nodes[n].id
		ids := expandedNow()
		before := len(ids)
		if next {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		} else {
			ids = slices.DeleteFunc(ids, func(i string) bool { return i == id })
		}
		if len(ids) == before {
			return
		}
		report(ids)
		if next && t.OnExpand != nil {
			t.OnExpand(id)
		}
		if !next && t.OnCollapse != nil {
			t.OnCollapse(id)
		}
	}
	selectNode := func(n int, byKey bool) {
		f := untrackedFlat()
		if n < 0 || n >= len(f.nodes) {
			return
		}
		id := f.nodes[n].id
		switch {
		case t.Selection != nil:
			t.Selection.Set(f.nodes[n].key)
		case core.Untracked(cx, selected) != id:
			if t.Selected == nil {
				ownSelected.Set(id)
			}
			if t.OnSelectedChange != nil {
				t.OnSelectedChange(id)
			}
		}
		if byKey {
			if row := treeRowOf(shown.GetUntracked(), n); row >= 0 {
				bring(row)
				if core.Untracked(cx, env.ScreenReader) {
					a11y.Set(n)
				}
			}
		}
	}
	move := func(s step) func() {
		return func() { selectNode(treeStep(untrackedFlat(), shown.GetUntracked(), current.GetUntracked(), s), true) }
	}
	// The node Right, Left, Return and Space act on: the cursor, while
	// something is selected.
	actedOn := func() int {
		if current.GetUntracked() < 0 {
			return -1
		}
		return cursor.GetUntracked()
	}
	right := func() {
		if current.GetUntracked() < 0 {
			move(stepFirst)()
			return
		}
		n := actedOn()
		f := untrackedFlat()
		if !f.folder(n) || !f.enabled(n) {
			return
		}
		if !isOpen(n) {
			setOpen(n, true)
			return
		}
		for _, c := range f.children(n) {
			if f.enabled(c) {
				selectNode(c, true)
				return
			}
		}
	}
	left := func() {
		if current.GetUntracked() < 0 {
			move(stepFirst)()
			return
		}
		n := actedOn()
		f := untrackedFlat()
		if n < 0 {
			return
		}
		if f.folder(n) && f.enabled(n) && isOpen(n) {
			setOpen(n, false)
		} else if p := f.nodes[n].parent; p != treeRoot && f.enabled(p) {
			selectNode(p, true)
		}
	}
	toggle := func() {
		if n := actedOn(); untrackedFlat().enabled(n) {
			setOpen(n, !isOpen(n))
		}
	}
	act := func(n int) {
		if f := untrackedFlat(); f.enabled(n) && t.OnAction != nil {
			t.OnAction(f.nodes[n].id)
		}
	}
	activate := func() {
		n := actedOn()
		if n < 0 {
			return
		}
		if untrackedFlat().folder(n) {
			toggle()
		} else {
			act(n)
		}
	}
	active := func() bool { return (focused.Get() || held.Get() > 0) && !is(t.Disabled) }
	keys := bindWhile(active, core.KeyNotEditing, true,
		controlKey{"down", true, move(stepNext)}, controlKey{"up", true, move(stepPrevious)},
		controlKey{"home", false, move(stepFirst)}, controlKey{"end", false, move(stepLast)},
		controlKey{"right", true, right}, controlKey{"left", true, left},
		controlKey{"enter", false, activate}, controlKey{"space", false, toggle})
	typed := treeTypeahead(cx, active, func(prefix string) {
		selectNode(treeTyped(untrackedFlat(), shown.GetUntracked(), current.GetUntracked(), prefix), true)
	})
	desktop := env.Desktop
	// selectPressed selects n as a press does: in a multiple Selection
	// Shift takes the rows from the anchor and Mod toggles it.
	selectPressed := func(n int, e core.PressEvent) {
		s := t.Selection
		if s == nil || !s.Multiple() {
			selectNode(n, false)
			return
		}
		f, rows := untrackedFlat(), shown.GetUntracked()
		key := f.nodes[n].key
		switch m := e.Modifiers; {
		case m.Shift && s.anchor != nil:
			from, to := treeRowOf(rows, f.findKey(s.anchor)), treeRowOf(rows, n)
			if from < 0 {
				from = to
			}
			anchor := s.anchor
			keys := []any{}
			for r := min(from, to); r <= max(from, to); r++ {
				keys = append(keys, f.nodes[rows[r]].key)
			}
			s.Set(keys...)
			s.cursor, s.anchor = key, anchor
			s.changed()
		case m.Platform || m.Control || !core.Untracked(cx, desktop):
			s.Toggle(key)
		default:
			s.Set(key)
		}
	}
	press := func(n int, e core.PressEvent) {
		if core.Untracked(cx, func() bool { return is(t.Disabled) }) || !untrackedFlat().enabled(n) {
			return
		}
		selectPressed(n, e)
		if !t.SelectOnly {
			setOpen(n, !isOpen(n))
		}
		// The keys go on from the row pressed.
		held.Update(func(n int) int { return n + 1 })
		if core.Untracked(cx, desktop) && !focused.GetUntracked() {
			root.Focus()
		}
		if e.Clicks == 2 {
			act(n)
		}
	}
	if t.Ref != nil {
		t.Ref.root = root
		t.Ref.reveal = func(id string) bool {
			f := untrackedFlat()
			n := f.find(id)
			if n < 0 {
				return false
			}
			ids := expandedNow()
			ancestors := f.ancestors(n)
			var added []string
			for i := len(ancestors) - 1; i >= 0; i-- {
				if a := ancestors[i]; !isOpen(a) && !slices.Contains(ids, f.nodes[a].id) {
					added = append(added, f.nodes[a].id)
				}
			}
			if len(added) > 0 {
				pendingID = id
				report(append(ids, added...))
				if t.OnExpand != nil {
					for _, a := range added {
						t.OnExpand(a)
					}
				}
			}
			if (pendingID != "" || len(added) == 0) && treeRowOf(shown.GetUntracked(), n) >= 0 {
				pendingID = ""
				bring(treeRowOf(shown.GetUntracked(), n))
			}
			return true
		}
	}
	row := func(cx *core.Scope, r core.ListRow) core.Node {
		node := core.NewMemo(cx, func() int {
			rows := shown.Get()
			if at := r.Index(); at < len(rows) {
				return rows[at]
			}
			return -1
		})
		entry := core.NewMemo(cx, func() TreeEntry {
			if n := node.Get(); n >= 0 {
				return flat.Get().entry(n)
			}
			return TreeEntry{}
		})
		disabled := core.NewMemo(cx, func() bool { return is(t.Disabled) || entry.Get().Disabled })
		isSelected := func() bool {
			n := node.Get()
			if t.Selection != nil {
				return n >= 0 && t.Selection.Has(flat.Get().nodes[n].key)
			}
			return n >= 0 && litSelected.is(n)
		}
		atCursor := func() bool { n := node.Get(); return n >= 0 && litCursor.is(n) }
		me := new(core.Ref)
		cx.Effect(func() {
			if n := node.Get(); n >= 0 && a11y.Get() == n {
				cx.Untrack(func() {
					me.SetAccessibilityFocus()
					a11y.Set(-1)
				})
			}
		})
		label := func() string { return entry.Get().Label }
		v := core.View{Ref: me, Role: core.RoleTreeItem, NoFocus: true, Disabled: disabled.Get, LabelFunc: label, Selected: isSelected,
			AccessibilityLevel: func() int { return entry.Get().Depth + 1 },
			PartStates:         func() core.PartStates { return core.PartStates{Focused: atCursor() && focused.Get()} },
			Class:              t.ItemClass, Style: t.ItemStyle, StyleFunc: t.ItemStyleFunc,
			OnPress: func(e core.PressEvent) {
				if n := node.GetUntracked(); n >= 0 {
					press(n, e)
				}
			}}
		v.Expanded = func() bool { n := node.Get(); return n >= 0 && flat.Get().folder(n) && slices.Contains(open.Get(), n) }
		v.Expandable = func() bool { return entry.Get().Folder }
		switch {
		case t.Row != nil:
			v.Children = []core.Node{t.Row(cx, TreeRow{Row: r, Entry: entry.Get, Expanded: v.Expanded,
				Key: func() any {
					if n := node.Get(); n >= 0 {
						return flat.Get().nodes[n].key
					}
					return nil
				},
				Toggle: func() {
					if n := node.GetUntracked(); n >= 0 && !core.Untracked(cx, disabled.Get) {
						setOpen(n, !isOpen(n))
					}
				}})}
		case t.RenderItem != nil:
			v.Children = []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				e := entry.Get()
				if e.ID == "" {
					return nil
				}
				return t.RenderItem(e)
			})}
		default:
			v.Children = []core.Node{core.Text{ContentFunc: label}}
		}
		return v
	}
	rows := core.List{Ref: list, Viewport: core.ListViewportParent, EstimatedSize: cmp.Or(t.EstimatedSize, 20), Empty: t.Empty,
		Count: func() int { return len(shown.Get()) },
		Key: func(i int) any {
			rows := shown.GetUntracked()
			if i < len(rows) {
				return untrackedFlat().nodes[rows[i]].id
			}
			return i
		},
		Row:               row,
		ViewabilityConfig: core.ViewabilityConfig{ItemVisiblePercentThreshold: 99.5},
		OnViewableItemsChange: func(v core.ViewableItems) {
			known = len(v.Viewable) > 0
			if !known {
				return
			}
			first, last = v.Viewable[0].Index, v.Viewable[len(v.Viewable)-1].Index
			if pendingRow >= 0 {
				r := pendingRow
				pendingRow = -1
				scrollTo(r)
			}
		}}
	listStyle := core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)}.Merge(t.ListStyle)
	return core.View{Ref: root, Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleTree, Label: t.Label, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Disabled:  t.Disabled,
		Focusable: func() bool { return !is(t.Disabled) },
		OnFocus: func() {
			focused.Set(true)
			// The screen reader's focus goes to the row the keys act on.
			cx.Untrack(func() {
				if n := cursor.Get(); env.ScreenReader() && n >= 0 {
					if r := treeRowOf(shown.Get(), n); r >= 0 {
						bring(r)
						a11y.Set(n)
					}
				}
			})
		},
		OnBlur: func() {
			focused.Set(false)
			held.Set(0)
		},
		Children: []core.Node{core.Scroll{Class: t.ListClass, Style: listStyle, Children: []core.Node{rows}}, keys, typed}}
}

// treeTypeaheadPause is how long typeahead waits before it starts over.
const treeTypeaheadPause = 500 * time.Millisecond

// treeTypeahead binds letters and digits while active, reporting what was
// typed lately, in lower case.
func treeTypeahead(cx *core.Scope, active func() bool, typed func(prefix string)) core.Node {
	text := ""
	var at time.Time
	push := func(c rune) {
		now := cx.Now()
		if now.Sub(at) > treeTypeaheadPause {
			text = ""
		}
		at = now
		text += string(unicode.ToLower(c))
		typed(text)
	}
	return core.Show{When: active, Then: func(cx *core.Scope) core.Node {
		for _, c := range "abcdefghijklmnopqrstuvwxyz0123456789" {
			cx.OnKeyWith(string(c), core.KeyOptions{When: core.KeyNotEditing}, func() { push(c) })
		}
		return nil
	}}
}
