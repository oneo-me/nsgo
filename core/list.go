package core

import (
	"slices"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// List renders a long run of rows lazily, as NSH's <List>: only the rows
// within a viewport of the visible ones are mounted, each in a cell that
// is reused for another row when it scrolls away. A cell's content is
// built once; its ListRow changes, so the content's bindings update
// instead of it being rebuilt. Row sizes start as an estimate and are
// corrected as rows are laid out, so offsets, columns, sticky rows and
// viewability are all decided in the core.
//
// Every scroll view prop of Scroll a list takes (Behavior, the scroll
// events, refreshing, OffsetValues) applies to its scroll view. Keep a
// row's state in the app's data, keyed by Key, not in the row's scope,
// which moves to other rows.
type List struct {
	Count func() int
	// Key returns row i's identity, a comparable value such as an ID.
	// With it, rows that stay keep their cells and sizes as rows come and
	// go, and rows inserted or removed above the viewport keep the
	// visible rows in place.
	Key func(i int) any
	// Row builds a cell's content; row says which row it shows.
	Row func(cx *Scope, row ListRow) Node
	// EstimatedSize is a row's expected size along the list; 50 when 0.
	EstimatedSize Points
	// NumColumns lays that many rows side by side across the list, each
	// line of rows as long as its longest.
	NumColumns    int
	Horizontal    bool
	HideIndicator bool
	// Inverted puts row 0 at the end (a chat): the list starts scrolled
	// to the end and keeps its distance from it as content changes. With
	// OnLoadMoreAtStart it does so only while row 0 shows; scrolled away,
	// rows arriving at row 0's end keep the visible rows in place.
	Inverted bool
	// MaintainVisibleContentPosition, when set, follows the end of a list
	// that is not inverted while it is within AutoscrollToBottomThreshold
	// of it, as content grows or the viewport shrinks.
	MaintainVisibleContentPosition *VisibleContentPosition
	// Gap separates rows, ColumnGap columns; the style's padding is the
	// content's.
	Gap       Points
	ColumnGap Points
	Header    Node
	Footer    Node
	// Empty shows while Count is 0.
	Empty Node
	// Separator builds what goes after each row but the last (before it
	// in an inverted list), in the row's cell.
	Separator func(cx *Scope) Node
	// StickyHeaderIndices stick those rows to the viewport's start, at
	// StickyHeaderOffset from it, while scrolled past, until the next one
	// pushes them out; one column, not inverted. The platform pins them on
	// every scroll. StickyHeaderIndicesFunc, when set, replaces them as
	// they change.
	StickyHeaderIndices     []int
	StickyHeaderIndicesFunc func() []int
	StickyHeaderOffset      Points
	// StickyHeaderOffsetValue, when set, is the animated value the
	// headers' offset follows in place of StickyHeaderOffset, as a header
	// above the list hides and shows.
	StickyHeaderOffsetValue AnimatedNode
	// InitialScrollIndex is the row the list starts scrolled to.
	InitialScrollIndex int
	// OnViewableItemsChange receives the viewable rows when they change,
	// by ViewabilityConfig's rules.
	OnViewableItemsChange func(ViewableItems)
	ViewabilityConfig     ViewabilityConfig
	// OnLoadMore runs when the viewport comes within LoadMoreThreshold
	// viewports of the content's end (1 when 0), once per count: also as
	// the list opens, so a first page shorter than that asks for the next.
	OnLoadMore        func()
	LoadMoreThreshold float32
	// OnLoadMoreAtStart is its counterpart at item 0's end, for a list
	// opened at an old item that loads what comes before it, within
	// LoadMoreAtStartThreshold viewports (1 when 0). It runs once per
	// approach: again after the viewport left that zone, or when the count
	// changed while it stayed there, and never before the user or a
	// scroll command moved the list.
	OnLoadMoreAtStart        func()
	LoadMoreAtStartThreshold float32
	// Viewport is what the list scrolls in: a scroll view of its own, or
	// with ListViewportParent the nearest scroll view around it on its
	// axis (or its screen, modal or overlay), which scrolls it with what
	// else it holds. Fixed when the list mounts.
	Viewport ListViewport

	// Behavior, ContentOffset, the scroll events, OnRefresh,
	// Refreshing, PullValue and OffsetValues are Scroll's, for the list's
	// scroll view. In its parent's viewport they are ignored, as are Inverted
	// and MaintainVisibleContentPosition.
	Behavior            ScrollBehavior
	BehaviorFunc        func() ScrollBehavior
	ContentOffset       Point
	OnScroll            func(ScrollEvent)
	ScrollEventThrottle time.Duration
	OnScrollDragStart   func(ScrollEvent)
	OnScrollDragEnd     func(ScrollEvent)
	OnMomentumStart     func(ScrollEvent)
	OnMomentumEnd       func(ScrollEvent)
	OnRefresh           func()
	Refreshing          func() bool
	PullValue           *AnimatedValue
	OffsetValues        AnimatedXY
	OnLayout            func(Bounds)
	Style               Style
	StyleFunc           func() Style
	Ref                 *Ref
	TestID              string
}

// Render implements Node.
func (l List) Render(*Scope) Node { return l }

// ListViewport is what a list scrolls in.
type ListViewport uint8

const (
	// ListViewportOwn is a scroll view of the list's own.
	ListViewportOwn ListViewport = iota
	// ListViewportParent lays the list out at its full length in its
	// parent and renders the rows near the viewport of the scroll view
	// around it.
	ListViewportParent
)

// VisibleContentPosition is List.MaintainVisibleContentPosition's.
type VisibleContentPosition struct {
	// AutoscrollToBottomThreshold is how near the end the list must be
	// to follow it.
	AutoscrollToBottomThreshold Points
}

// ViewabilityConfig is when a row counts as viewable. Without a
// threshold any visible point counts.
type ViewabilityConfig struct {
	// ViewAreaCoveragePercentThreshold is the percent of the viewport a
	// row must cover; it wins over ItemVisiblePercentThreshold.
	ViewAreaCoveragePercentThreshold float32
	// ItemVisiblePercentThreshold is the percent of a row that must be
	// visible.
	ItemVisiblePercentThreshold float32
	// MinimumViewTime is how long rows must stay viewable before they
	// are reported.
	MinimumViewTime time.Duration
}

// ViewableItems is what OnViewableItemsChange reports: the viewable rows
// and the rows that became or stopped being viewable since the last
// report.
type ViewableItems struct {
	Viewable []ViewToken
	Changed  []ViewToken
}

// ViewToken is a row in ViewableItems: its index, its Key (nil without
// List.Key) and whether it is viewable.
type ViewToken struct {
	Index      int
	Key        any
	IsViewable bool
}

// ListRow is the row a list cell shows, as signals, so content reading
// it follows the cell to its next row.
type ListRow struct {
	index, count *Signal[int]
}

// Index returns the row's index.
func (p ListRow) Index() int { return p.index.Get() }

// Count returns the number of rows.
func (p ListRow) Count() int { return p.count.Get() }

// First reports whether the row is the first.
func (p ListRow) First() bool { return p.index.Get() == 0 }

// Last reports whether the row is the last.
func (p ListRow) Last() bool { return p.index.Get() == p.count.Get()-1 }

type listNode struct {
	spec  List
	owner *Scope
	// view is the scroll view, or in its parent's viewport the plain view
	// that clips the cells.
	view   *node
	spacer *node // in the flow, as long as the content
	cells  *node // region of the row cells
	stuck  *node // region of the sticky cells, above the others
	extras [3]*node
	st     listState
	count  *Signal[int]
	keys   []any
	// byIndex holds the cells bound to rows; free the others, hidden.
	byIndex map[int]*listCell
	free    []*listCell
	cmds    []listCommand
	// loadAt and loadAtStart are the counts OnLoadMore and
	// OnLoadMoreAtStart last ran for, -1 for none.
	loadAt, loadAtStart int
	sticky              []StickyCell
	// outer is the view a parent-viewport list scrolls in, depth the
	// views between them.
	outer *node
	depth int
	// viewable is the range reported last; latest the one found last.
	viewable, latest [2]int
	viewStop         func() bool
	// waiting are the rows of the window not bound yet, in the order they
	// bind; paced counts those bound in commit pacedIn.
	waiting        []int
	paced, pacedIn int
}

// Paced binding, as NSH's: a fast scroll or a jump asks for a screen of
// cells at once, and each bind builds or rebinds a cell's subtree. What
// shows binds in the commit that asks for it; a list binds paceCells more
// of its window in a commit, at least one, while the commit has run less
// than paceTime, and the rest on the next display frames.
const (
	paceCells = 4
	paceTime  = 2 * time.Millisecond
)

// pace reports whether the commit may bind another of ln's waiting rows,
// counting it.
func (t *Tree) pace(ln *listNode) bool {
	if ln.pacedIn != t.commits {
		ln.pacedIn, ln.paced = t.commits, 0
	}
	if ln.paced >= paceCells || ln.paced > 0 && t.rt.clock.Now().Sub(t.commitStart) > paceTime {
		return false
	}
	ln.paced++
	return true
}

// paceLater has the next display frame commit again while lists wait to
// bind rows.
func (t *Tree) paceLater() {
	if t.paceStop != nil {
		return
	}
	for _, l := range t.lists {
		if !l.deleted && len(l.list.waiting) > 0 {
			t.paceStop = t.root.RequestAnimationFrame(func(time.Time) {
				t.paceStop = nil
				t.changed = true
			})
			return
		}
	}
}

// Pacing reports whether lists wait for display frames to bind the rest
// of their windows.
func (t *Tree) Pacing() bool { return t.paceStop != nil }

type listCell struct {
	scope *Scope
	pos   ListRow
	node  *node // the cell view
	index int
}

// listCommand is a scroll a Ref asked a list for, resolved with the
// sizes known in the next update.
type listCommand struct {
	pin      *listPin // ScrollToIndex or ScrollToEnd; nil for to
	to       Point
	animated bool
}

func (t *Tree) mountList(cx *Scope, parent *node, l List) {
	ln := &listNode{spec: l, owner: cx, byIndex: map[int]*listCell{}, loadAt: -1, loadAtStart: -1, st: newListState(),
		viewable: [2]int{-1, -1}, latest: [2]int{-1, -1}}
	if ln.spec.EstimatedSize <= 0 {
		ln.spec.EstimatedSize = 50
	}
	if ln.spec.LoadMoreThreshold <= 0 {
		ln.spec.LoadMoreThreshold = 1
	}
	if ln.spec.LoadMoreAtStartThreshold <= 0 {
		ln.spec.LoadMoreAtStartThreshold = 1
	}
	st := &ln.st
	st.estimated, st.horizontal, st.gap, st.crossGap = ln.spec.EstimatedSize, l.Horizontal, l.Gap, l.ColumnGap
	st.columns = max(l.NumColumns, 1)
	st.stickyOffset = l.StickyHeaderOffset
	if l.StickyHeaderOffsetValue != nil {
		st.stickyNode = l.StickyHeaderOffsetValue.AnimatedNodeID()
	}
	st.setSticky(l.StickyHeaderIndices)
	st.viewability = l.ViewabilityConfig
	st.parent = l.Viewport == ListViewportParent
	st.loadsAtStart = l.OnLoadMoreAtStart != nil
	if !st.parent {
		st.inverted = l.Inverted
		if m := l.MaintainVisibleContentPosition; m != nil {
			st.followEnd = max(m.AutoscrollToBottomThreshold, 0)
		}
	}

	var v *node
	if st.parent {
		v = t.newNode(driver.KindView, parent, cx)
		t.bindStyle(cx, v, Style{Overflow: Hidden}.Merge(l.Style), l.StyleFunc)
	} else {
		v = t.newNode(driver.KindScroll, parent, cx)
		t.bindStyle(cx, v, scrollDefaults.Merge(l.Style), l.StyleFunc)
	}
	ln.view = v
	v.list = ln
	v.h = handlers{layout: l.OnLayout}
	v.props = driver.ViewProps{TestID: l.TestID}
	t.setRef(v, l.Ref)
	if !st.parent {
		v.h.scroll = func(Point) {}
		if on := l.OnScroll; on != nil {
			v.h.scroll = func(p Point) { on(v.scrollEvent(p)) }
		}
		v.scroll = &scrollNode{config: driver.ScrollConfig{Horizontal: l.Horizontal, HideIndicator: l.HideIndicator}}
		t.bindScrollSpec(cx, v, scrollSpec{behavior: l.Behavior, behaviorFunc: l.BehaviorFunc,
			refreshing: l.Refreshing, onRefresh: l.OnRefresh, onScroll: l.OnScroll, throttle: l.ScrollEventThrottle,
			onDragStart: l.OnScrollDragStart, onDragEnd: l.OnScrollDragEnd,
			onMomentum: l.OnMomentumStart, onMomentumEnd: l.OnMomentumEnd})
		t.bindScroll(cx, v, l.OffsetValues, l.PullValue)
	}
	if l.ContentOffset != (Point{}) {
		ln.cmds = append(ln.cmds, listCommand{to: l.ContentOffset})
	}
	if l.InitialScrollIndex > 0 {
		ln.cmds = append(ln.cmds, listCommand{pin: &listPin{index: l.InitialScrollIndex}})
	}

	ln.count = NewSignal(cx, 0)
	ln.spacer = t.newNode(driver.KindView, v, cx)
	ln.spacer.base = Style{FlexShrink: 0}
	t.restyle(ln.spacer)
	extra := func(kind int, content Node) {
		c := t.newNode(driver.KindView, v, cx)
		c.base = Style{Position: Absolute}
		t.restyle(c)
		t.mount(cx, c, content)
		ln.extras[kind] = c
		st.extras[kind].present = true
	}
	if l.Header != nil {
		extra(extraHeader, l.Header)
	}
	if l.Footer != nil {
		extra(extraFooter, l.Footer)
	}
	if l.Empty != nil {
		extra(extraEmpty, Show{When: func() bool { return ln.count.Get() == 0 }, Then: func(*Scope) Node { return l.Empty }})
	}
	ln.cells = t.newRegion(v, cx)
	ln.stuck = t.newRegion(v, cx)
	if f := l.StickyHeaderIndicesFunc; f != nil {
		cx.renderEffect(func() {
			st.setSticky(f())
			t.relayout(v)
		})
	}
	cx.renderEffect(func() {
		n := max(l.Count(), 0)
		if l.Key != nil {
			keys := make([]any, n)
			for i := range keys {
				keys[i] = l.Key(i)
			}
			if at, removed, inserted, ok := listSplice(ln.keys, keys); ok && len(ln.keys) == st.count {
				t.spliceList(ln, at, removed, inserted)
			}
			ln.keys = keys
		}
		if n != st.count {
			st.setCount(n)
			ln.freeFrom(n)
		}
		ln.count.Set(n)
		t.relayout(v)
	})
	t.lists = append(t.lists, v)
}

// spliceList moves the list's measurements and cells with the rows that
// stay as removed rows at at are replaced by inserted ones.
func (t *Tree) spliceList(ln *listNode, at, removed, inserted int) {
	ln.st.splice(at, removed, inserted)
	cells := ln.byIndex
	ln.byIndex = make(map[int]*listCell, len(cells))
	for i, c := range cells {
		switch {
		case i < at:
			ln.byIndex[i] = c
		case i >= at+removed:
			c.index = i - removed + inserted
			c.pos.index.Set(c.index)
			ln.byIndex[c.index] = c
		default:
			ln.free = append(ln.free, c)
		}
	}
	ln.loadAt = -1
}

// loadMore runs OnLoadMore and OnLoadMoreAtStart as the viewport nears
// either end, as NSH's ListState::update_window raises them.
func (t *Tree) loadMore(ln *listNode, v *node) {
	st := &ln.st
	if st.viewport <= 0 {
		return
	}
	at, count := st.naturalOffset(), st.count
	end := at+st.viewport >= st.blocks().total-st.viewport*Points(ln.spec.LoadMoreThreshold)
	if f := ln.spec.OnLoadMore; f != nil && end && ln.loadAt != count {
		ln.loadAt = count
		t.after(v, f)
	}
	if at > st.viewport*Points(ln.spec.LoadMoreAtStartThreshold) {
		ln.loadAtStart = -1
	} else if f := ln.spec.OnLoadMoreAtStart; f != nil && st.moved && ln.loadAtStart != count {
		ln.loadAtStart = count
		t.after(v, f)
	}
}

// freeFrom frees the cells of rows count and after.
func (ln *listNode) freeFrom(count int) {
	for i, c := range ln.byIndex {
		if i >= count {
			delete(ln.byIndex, i)
			ln.free = append(ln.free, c)
		}
	}
}

// main is the main-axis length of a size.
func (ln *listNode) main(s Size) Points {
	if ln.spec.Horizontal {
		return s.Width
	}
	return s.Height
}

func (ln *listNode) cross(s Size) Points {
	if ln.spec.Horizontal {
		return s.Height
	}
	return s.Width
}

// along is a point at d along the list's axis.
func (ln *listNode) along(d Points) Point {
	if ln.spec.Horizontal {
		return Point{X: d}
	}
	return Point{Y: d}
}

// outerInput is where a parent-viewport list is in the view it scrolls
// in.
type outerInput struct {
	view                   *node
	start, offset          Points
	viewport, maxOffset    Points
	settled, scrolls, seen bool
}

// findOuter returns the view list n scrolls in: the nearest scroll view
// on its axis around it, else the root, modal, overlay or screen it is
// in, and the views between them.
func (ln *listNode) findOuter() (*node, int) {
	depth := 0
	for p := ln.view.parent; p != nil; p = p.parent {
		if p.kind == 0 {
			continue
		}
		depth++
		if p.scroll != nil && p.scroll.config.Horizontal == ln.spec.Horizontal {
			return p, depth
		}
		if p.isRoot || p.hoist != nil || p.screen != nil || p.accessory != nil || p.attach != nil || p.parent == nil {
			return p, depth
		}
	}
	return nil, depth
}

// outer measures where a parent-viewport list is in its view.
func (t *Tree) outer(ln *listNode) (outerInput, bool) {
	o, _ := ln.findOuter()
	if o == nil || ln.view.hidden || o.hidden {
		return outerInput{}, false
	}
	var start Points
	for n := ln.view; n != o; n = n.parent {
		if n.kind != 0 {
			start += ln.main(Size{Width: n.frame.Origin.X, Height: n.frame.Origin.Y})
		}
	}
	b := ln.view.style.borders()
	if ln.spec.Horizontal {
		start += b.Left
	} else {
		start += b.Top
	}
	in := outerInput{view: o, start: start, viewport: ln.main(o.frame.Size), settled: true}
	if sc := o.scroll; sc != nil {
		in.scrolls = true
		at := sc.main(sc.offset)
		if sc.to != nil {
			at = sc.main(sc.to.Point)
		}
		in.offset = at + sc.main(sc.by)
		in.maxOffset = max(ln.main(sc.content)-in.viewport, 0)
		in.settled = sc.settled()
	} else {
		in.maxOffset = max(ln.main(o.frame.Size)-in.viewport, 0)
	}
	return in, true
}

// updateList measures the cells laid out, keeps the visible content in
// place, follows scroll targets, moves the window of rows to the
// viewport and places the cells. It reports whether anything needs
// another layout.
func (t *Tree) updateList(v *node) bool {
	ln := v.list
	st := &ln.st
	var outer outerInput
	if st.parent {
		o, ok := t.outer(ln)
		if !ok {
			return false
		}
		outer = o
		st.setOuter(o.start, o.offset, o.viewport, o.maxOffset, o.settled)
		if sc := o.view.scroll; sc != nil && !slices.Contains(sc.dependents, ln) {
			sc.dependents = append(sc.dependents, ln)
		}
		ln.outer = o.view
	}
	changed := false
	ps, b := v.style.Padding, v.style.borders()
	if ln.spec.Horizontal {
		st.padStart, st.padEnd, st.crossStart, st.crossEnd = ps.Left, ps.Right, ps.Top, ps.Bottom
	} else {
		st.padStart, st.padEnd, st.crossStart, st.crossEnd = ps.Top, ps.Bottom, ps.Left, ps.Right
	}
	if st.rows.tree == nil {
		st.rebuild()
	}

	// Rows laid out since the last update correct the estimates.
	var anchor Points
	for i, c := range ln.byIndex {
		if c.node.layoutDirty || c.node.frame.Size == (Size{}) {
			continue
		}
		anchor += st.measured(i, ln.main(c.node.frame.Size))
	}
	for kind, c := range ln.extras {
		if c != nil && !c.layoutDirty {
			anchor += st.measuredExtra(kind, ln.main(c.frame.Size))
		}
	}
	if !st.parent {
		st.viewport = ln.main(v.frame.Size)
	}
	cross := ln.cross(v.frame.Size) - ln.cross(Size{Width: b.Left + b.Right, Height: b.Top + b.Bottom})
	st.offset += anchor
	anchor += st.takeSpliceAnchor()
	anchor += st.anchorEnd()
	var retarget *Points
	by, to, re := st.followPin(anchor)
	anchor += by
	if re {
		retarget = &to
	}

	// Scroll commands, now that sizes are known: the window follows a
	// jump in this same update.
	var outerTo *driver.MountItem
	for _, cmd := range ln.cmds {
		var offset Points
		if cmd.pin != nil {
			offset = st.targetOffset(cmd.pin)
		} else {
			offset = ln.main(Size{Width: cmd.to.X, Height: cmd.to.Y})
			if st.parent {
				offset = st.clamped(offset)
			}
		}
		st.pin = nil
		// Where the app scrolled the list may be near its start.
		st.moved = true
		if cmd.pin != nil {
			p := *cmd.pin
			p.at, p.animating = offset, cmd.animated
			st.pin = &p
		}
		retarget = nil
		if !cmd.animated {
			st.jump(offset)
		}
		if st.parent {
			outerTo = &driver.MountItem{Kind: driver.ItemScrollTo, ID: outer.view.id, Point: ln.along(outer.start + offset), Animated: cmd.animated}
			continue
		}
		t.scrollTo(v, ln.along(offset), cmd.animated)
	}
	if len(ln.cmds) > 0 {
		ln.cmds = ln.cmds[:0]
		changed = true
	}
	if retarget != nil {
		if st.parent {
			outerTo = &driver.MountItem{Kind: driver.ItemScrollTo, ID: outer.view.id, Point: ln.along(outer.start + *retarget), Animated: true}
		} else {
			t.scrollTo(v, ln.along(*retarget), true)
		}
	}

	// An offset past the content, after rows went, is where the platform
	// is about to clamp it.
	if !st.parent {
		in := v.scroll.config.Behavior.ContentInset
		st.offset = min(st.offset, max(st.contentLen()-st.viewport, 0)+ln.main(Size{Width: in.Right, Height: in.Bottom}))
	}

	// The window, then cells freed outside it and bound inside it.
	count := st.count
	first, last, ok, stickies := st.window()
	wanted := func(i int) bool { return ok && i >= first && i <= last || slices.Contains(stickies, i) }
	for i, c := range ln.byIndex {
		if !wanted(i) {
			delete(ln.byIndex, i)
			ln.free = append(ln.free, c)
		}
	}
	bind := func(i int) {
		if _, has := ln.byIndex[i]; has || i >= count {
			return
		}
		changed = true
		var c *listCell
		if k := len(ln.free); k > 0 {
			c, ln.free = ln.free[k-1], ln.free[:k-1]
			c.pos.index.Set(i)
		} else {
			c = t.newCell(ln, i)
		}
		c.index = i
		ln.byIndex[i] = c
	}
	// What shows binds now: the viewport, just ahead of it and the sticky
	// rows. The rest of the window waits, and binds nearest the leading
	// edge first as the commit's pace allows, then on the next frames.
	ln.waiting = ln.waiting[:0]
	if ok {
		start, end := st.urgentSpan()
		for i := first; i <= last; i++ {
			if _, has := ln.byIndex[i]; has || i >= count {
				continue
			}
			if st.overlaps(i, start, end) {
				bind(i)
			} else {
				ln.waiting = append(ln.waiting, i)
			}
		}
	}
	for _, i := range stickies {
		bind(i)
	}
	if len(ln.waiting) > 0 {
		st.paceOrder(ln.waiting)
		bound := 0
		for _, i := range ln.waiting {
			if !t.pace(ln) {
				break
			}
			bind(i)
			bound++
		}
		ln.waiting = append(ln.waiting[:0], ln.waiting[bound:]...)
	}
	t.loadMore(ln, v)
	t.reportViewable(ln)

	// Spare cells hide until a row needs them.
	for _, c := range ln.free {
		if c.node.base.Display != DisplayNone {
			c.node.base.Display = DisplayNone
			if t.restyle(c.node) {
				t.markLayout(c.node)
			}
		}
		t.restick(ln, c, false)
	}
	sticky := st.stickyEnabled()
	var ranges []StickyCell
	for i, c := range ln.byIndex {
		c.pos.count.Set(count)
		size := max(st.rowExtent(i), ln.main(c.node.frame.Size))
		at := st.toContent(st.itemAt(i), size)
		x, w := st.column(cross, st.columnOf(i))
		if t.placeCell(ln, c.node, at, x, w) {
			changed = true
		}
		isSticky := sticky && slices.Contains(st.sticky, i)
		t.restick(ln, c, isSticky)
		if isSticky && !c.node.layoutDirty {
			ranges = append(ranges, st.stickyRange(i, c.node.id, ln.main(c.node.frame.Size)))
		}
	}
	blocks := st.blocks()
	x, w := st.fullWidth(cross)
	for kind, c := range ln.extras {
		if c == nil {
			continue
		}
		at := [...]Points{blocks.header, blocks.footer, blocks.body}[kind]
		if t.placeCell(ln, c, st.toContent(at, ln.main(c.frame.Size)), x, w) {
			changed = true
		}
	}
	slices.SortFunc(ranges, func(a, b StickyCell) int { return int(a.Cell) - int(b.Cell) })
	ln.sticky = ranges

	total := st.contentLen() - st.padStart - st.padEnd
	if ln.main(Size{Width: ln.spacerLen(), Height: ln.spacerLen()}) != total {
		t.setSpacer(ln, total)
		t.relayout(ln.spacer)
		changed = true
	}

	// The anchoring scroll, sent with the content size; in its parent's
	// viewport the view around it scrolls.
	if st.parent {
		if sc := outer.view.scroll; sc != nil {
			if outerTo != nil {
				sc.to = outerTo
				t.mark(outer.view)
			}
			if anchor != 0 {
				d := ln.along(anchor)
				sc.by.X += d.X
				sc.by.Y += d.Y
				t.mark(outer.view)
			}
		}
	} else if anchor != 0 {
		d := ln.along(anchor)
		v.scroll.by.X += d.X
		v.scroll.by.Y += d.Y
		t.mark(v)
	}
	return changed
}

// placeCell moves cell c to at along the list and x across it, w wide, and
// shows it; it reports whether that changed its layout.
func (t *Tree) placeCell(ln *listNode, c *node, at, x, w Points) bool {
	s := &c.base
	type placed struct {
		display          Display
		position         Position
		l, t, w, h, b, r Length
	}
	at0 := placed{s.Display, s.Position, s.Left, s.Top, s.Width, s.Height, s.Bottom, s.Right}
	s.Display = 0
	s.Position = Absolute
	if ln.spec.Horizontal {
		s.Left, s.Top, s.Height, s.Bottom, s.Right = Pt(at), Pt(x), Pt(w), Length{}, Length{}
		s.Width = Length{}
	} else {
		s.Top, s.Left, s.Width, s.Bottom, s.Right = Pt(at), Pt(x), Pt(w), Length{}, Length{}
		s.Height = Length{}
	}
	if (placed{s.Display, s.Position, s.Left, s.Top, s.Width, s.Height, s.Bottom, s.Right}) == at0 {
		return false
	}
	if t.restyle(c) {
		t.markLayout(c)
	}
	return true
}

// restick moves cell c among the sticky cells, which draw above the
// others, or back.
func (t *Tree) restick(ln *listNode, c *listCell, sticky bool) {
	from, to := ln.cells, ln.stuck
	if !sticky {
		from, to = to, from
	}
	if c.node.parent != from {
		return
	}
	from.children = slices.DeleteFunc(from.children, func(n *node) bool { return n == c.node })
	to.children = append(to.children, c.node)
	c.node.parent = to
	t.carry(c.node)
	t.childrenChanged(from)
}

// reportViewable reports the viewable rows when they changed, once they
// stayed so for MinimumViewTime.
func (t *Tree) reportViewable(ln *listNode) {
	on := ln.spec.OnViewableItemsChange
	if on == nil {
		return
	}
	now := [2]int{-1, -1}
	if first, last, ok := ln.st.viewableItems(); ok {
		now = [2]int{first, last}
	}
	if now == ln.latest {
		return
	}
	ln.latest = now
	if ln.viewStop != nil {
		ln.viewStop()
		ln.viewStop = nil
	}
	report := func() {
		if ln.view.deleted || ln.latest == ln.viewable {
			return
		}
		before := ln.viewable
		ln.viewable = ln.latest
		on(ln.viewableItems(ln.viewable, before))
	}
	if d := ln.spec.ViewabilityConfig.MinimumViewTime; d > 0 {
		ln.viewStop = ln.owner.AfterFunc(d, func() {
			ln.viewStop = nil
			if ln.latest == now {
				report()
				t.changed = true
			}
		})
		return
	}
	t.after(ln.view, report)
}

// viewableItems is what OnViewableItemsChange gets for the rows now,
// viewable since before.
func (ln *listNode) viewableItems(now, before [2]int) ViewableItems {
	in := func(r [2]int, i int) bool { return r[0] >= 0 && i >= r[0] && i <= r[1] }
	token := func(i int, viewable bool) ViewToken {
		tok := ViewToken{Index: i, IsViewable: viewable}
		if i < len(ln.keys) {
			tok.Key = ln.keys[i]
		}
		return tok
	}
	var v ViewableItems
	if now[0] >= 0 {
		for i := now[0]; i <= now[1]; i++ {
			tok := token(i, true)
			v.Viewable = append(v.Viewable, tok)
			if !in(before, i) {
				v.Changed = append(v.Changed, tok)
			}
		}
	}
	if before[0] >= 0 {
		for i := before[0]; i <= before[1]; i++ {
			if !in(now, i) {
				v.Changed = append(v.Changed, token(i, false))
			}
		}
	}
	return v
}

func (ln *listNode) spacerLen() Points {
	l := ln.spacer.base.Height
	if ln.spec.Horizontal {
		l = ln.spacer.base.Width
	}
	p, _ := l.Points()
	return p
}

func (t *Tree) setSpacer(ln *listNode, total Points) {
	total = max(total, 0)
	if ln.spec.Horizontal {
		ln.spacer.base.Width = Pt(total)
	} else {
		ln.spacer.base.Height = Pt(total)
	}
	t.restyle(ln.spacer)
}

func (t *Tree) newCell(ln *listNode, i int) *listCell {
	c := &listCell{scope: &Scope{}, index: i}
	ln.owner.adopt(c.scope)
	c.pos = ListRow{index: NewSignal(c.scope, i), count: NewSignal(c.scope, ln.count.GetUntracked())}
	t.within(c.scope, func() {
		c.node = t.newNode(driver.KindView, ln.cells, c.scope)
		c.node.base = Style{Position: Absolute}
		if ln.spec.Horizontal {
			c.node.base.Direction = Row
		}
		t.restyle(c.node)
		row := ln.spec.Row(c.scope, c.pos)
		var sep Node
		if f := ln.spec.Separator; f != nil {
			pos := c.pos
			sep = Show{When: func() bool { return !pos.Last() }, Then: f}
		}
		if ln.spec.Inverted && sep != nil {
			t.mount(c.scope, c.node, sep)
		}
		t.mount(c.scope, c.node, row)
		if !ln.spec.Inverted && sep != nil {
			t.mount(c.scope, c.node, sep)
		}
	})
	return c
}

// updateLists updates every list: those with views of their own first,
// then those in their parents' viewports, outer ones first. It reports
// whether any needs another layout.
func (t *Tree) updateLists() bool {
	changed := false
	live := t.lists[:0]
	var parents []*node
	for _, l := range t.lists {
		if l.scroll != nil {
			l.scroll.dependents = l.scroll.dependents[:0]
		}
		if o := l.list.outer; o != nil && o.scroll != nil {
			o.scroll.dependents = o.scroll.dependents[:0]
		}
	}
	for _, l := range t.lists {
		if l.deleted {
			continue
		}
		live = append(live, l)
		if l.list.st.parent {
			_, l.list.depth = l.list.findOuter()
			parents = append(parents, l)
			continue
		}
		changed = t.within2(l, func() bool { return t.updateList(l) }) || changed
	}
	t.lists = live
	slices.SortStableFunc(parents, func(a, b *node) int { return a.list.depth - b.list.depth })
	for _, l := range parents {
		changed = t.within2(l, func() bool { return t.updateList(l) }) || changed
	}
	return changed
}

// sendSticky sends each scroll view's sticky cells when they changed:
// its own list's and those of the lists in its viewport.
func (t *Tree) sendSticky() {
	views := map[*node]bool{}
	for _, l := range t.lists {
		if l.deleted {
			continue
		}
		if l.scroll != nil {
			views[l] = true
		}
		if o := l.list.outer; o != nil && o.scroll != nil && !o.deleted {
			views[o] = true
		}
	}
	// Sticky views pin in the nearest scroll view around them.
	pinned := map[*node][]StickyCell{}
	for n := range t.sticky {
		if v, c, ok := pinRange(n); ok {
			pinned[v] = append(pinned[v], c)
		}
	}
	for v := range pinned {
		views[v] = true
	}
	for _, v := range t.stuckIn {
		if !v.deleted {
			views[v] = true
		}
	}
	t.stuckIn = t.stuckIn[:0]
	for v := range views {
		sc := v.scroll
		var cells []StickyCell
		if v.list != nil {
			cells = append(cells, v.list.sticky...)
		}
		for _, d := range sc.dependents {
			if !d.view.deleted && d.outer == v {
				cells = append(cells, d.sticky...)
			}
		}
		if own := pinned[v]; len(own) > 0 {
			slices.SortFunc(own, func(a, b StickyCell) int { return int(a.Cell) - int(b.Cell) })
			cells = append(cells, own...)
			t.stuckIn = append(t.stuckIn, v)
		}
		if !slices.Equal(cells, sc.sticky) {
			sc.sticky = cells
			t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemStickyCells, ID: v.id, Sticky: slices.Clone(cells)})
			t.changed = true
		}
	}
}

// pinRange returns the scroll view sticky view n pins in and its range
// of travel there, as CSS's position: sticky: from where its start meets
// the viewport's start less its Top (Left across a horizontal scroll
// view), to where its end meets its parent's content end.
func pinRange(n *node) (*node, StickyCell, bool) {
	if n.deleted || n.hidden || n.style.Display == DisplayNone || n.hoist != nil {
		return nil, StickyCell{}, false
	}
	var pos Point
	var sv *node
	for p := n.nativeParent(); p != nil; p = p.nativeParent() {
		if p.scroll != nil {
			sv = p
			break
		}
		if p.hoist != nil {
			return nil, StickyCell{}, false
		}
		pos.X += p.frame.Origin.X
		pos.Y += p.frame.Origin.Y
	}
	if sv == nil {
		return nil, StickyCell{}, false
	}
	horizontal := sv.scroll.config.Horizontal
	main := func(p Point) Points {
		if horizontal {
			return p.X
		}
		return p.Y
	}
	inset, viewport := n.style.Top, sv.frame.Size.Height
	if horizontal {
		inset, viewport = n.style.Left, sv.frame.Size.Width
	}
	at := inset.Resolve(viewport)
	if at.IsAuto() {
		return nil, StickyCell{}, false
	}
	// The parent's content box, in the scroll view's content.
	parent := n.nativeParent()
	var end Points
	if parent == sv {
		end = main(Point{X: sv.scroll.content.Width, Y: sv.scroll.content.Height})
	} else {
		l := &parent.lstate.Rounded
		box := contentBox(l)
		end = main(pos) + main(box.Origin) + main(Point{X: box.Size.Width, Y: box.Size.Height})
	}
	start := main(pos) + main(n.frame.Origin)
	size := main(Point{X: n.frame.Size.Width, Y: n.frame.Size.Height})
	travel := max(end-start-size, 0)
	c := StickyCell{Cell: n.id, Start: start - at, End: start - at + travel}
	if a := n.style.Animated; a != nil && a.StickyInset != nil {
		c.Node, c.Inset = a.StickyInset.AnimatedNodeID(), at
	}
	return sv, c, true
}

// listCommand queues a scroll for list ln's next update.
func (t *Tree) listCommand(ln *listNode, c listCommand) {
	ln.cmds = append(ln.cmds, c)
	t.relayout(ln.view)
}

// scrollTo sends scroll view n to p, counting the scroll.
func (t *Tree) scrollTo(n *node, p Point, animated bool) {
	n.scroll.issue(true, p)
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemScrollTo, ID: n.id, Point: p, Animated: animated})
	t.changed = true
}

// scrolled records where the platform reports scroll view n is, for the
// lists that scroll with it.
func (t *Tree) scrolled(n *node, ev driver.Event) {
	sc := n.scroll
	sc.report(ev.Point, ev.Count)
	if ln := n.list; ln != nil && !ln.st.parent {
		ln.st.movedTo(sc.main(sc.offset), sc.settled())
		t.changed = true
	}
	if len(sc.dependents) > 0 {
		t.changed = true
	}
}

// emitScrollBy sends the lists' anchoring scroll of scroll view n: before
// its content shrinks, after it grows, since platforms clamp offsets to
// the content.
func (t *Tree) emitScrollBy(n *node, before bool) {
	sc := n.scroll
	if sc.by == (Point{}) {
		return
	}
	shrinks := sc.main(Point{X: sc.content.Width, Y: sc.content.Height}) < sc.main(Point{X: n.sent.content.Width, Y: n.sent.content.Height})
	if before != shrinks {
		return
	}
	d := sc.by
	sc.by = Point{}
	// A scroll to an offset not sent yet moves with it instead: it goes
	// out after this batch's items.
	for i := len(t.commands) - 1; i >= 0; i-- {
		if c := &t.commands[i]; c.Kind == driver.ItemScrollTo && c.ID == n.id {
			c.Point.X += d.X
			c.Point.Y += d.Y
			for k := len(sc.pending) - 1; k >= 0; k-- {
				if sc.pending[k].to {
					sc.pending[k].p = c.Point
					break
				}
			}
			sc.offset.X += d.X
			sc.offset.Y += d.Y
			return
		}
	}
	sc.issue(false, d)
	t.items = append(t.items, driver.MountItem{Kind: driver.ItemScrollBy, ID: n.id, Point: d})
}

// emitScrollTo sends the scroll the lists in n's viewport asked for.
func (t *Tree) emitScrollTo(n *node) {
	sc := n.scroll
	if sc.to == nil {
		return
	}
	it := *sc.to
	sc.to = nil
	lo, hi := Points(0), max(sc.main(Point{X: sc.content.Width, Y: sc.content.Height})-sc.main(Point{X: n.frame.Size.Width, Y: n.frame.Size.Height}), 0)
	if sc.config.Horizontal {
		it.Point.X = min(max(it.Point.X, lo), hi)
	} else {
		it.Point.Y = min(max(it.Point.Y, lo), hi)
	}
	sc.issue(true, it.Point)
	t.commands = append(t.commands, it)
}

// fenwick is a Fenwick tree of row sizes: prefix sums and the row at an
// offset in O(log n).
type fenwick struct {
	tree []Points // 1-based
	vals []Points
}

func newFenwick(vals []Points) fenwick {
	f := fenwick{tree: make([]Points, len(vals)+1), vals: vals}
	for i, v := range vals {
		f.tree[i+1] += v
		if j := i + 1 + (i+1)&-(i+1); j <= len(vals) {
			f.tree[j] += f.tree[i+1]
		}
	}
	return f
}

func (f *fenwick) at(i int) Points { return f.vals[i] }

func (f *fenwick) set(i int, v Points) {
	d := v - f.vals[i]
	f.vals[i] = v
	for j := i + 1; j < len(f.tree); j += j & -j {
		f.tree[j] += d
	}
}

// sum returns the total of the first n sizes.
func (f *fenwick) sum(n int) Points {
	var s Points
	for j := min(n, len(f.vals)); j > 0; j -= j & -j {
		s += f.tree[j]
	}
	return s
}

// find returns the row at offset: the first i whose sum(i+1) > offset.
func (f *fenwick) find(offset Points) int {
	pos := 0
	step := 1
	for step*2 <= len(f.vals) {
		step *= 2
	}
	for ; step > 0 && len(f.vals) > 0; step /= 2 {
		if next := pos + step; next <= len(f.vals) && f.tree[next] <= offset {
			pos = next
			offset -= f.tree[next]
		}
	}
	return pos
}
