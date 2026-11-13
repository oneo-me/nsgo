package core

import (
	"math"
	"slices"
)

// listState is a list's bookkeeping, as NSH's ListState: item sizes, row
// offsets, the window, anchoring, sticky rows, viewability and pinned
// scroll targets. The tree feeds it measurements and scroll offsets and
// applies what it decides to the cells.
//
// Positions run along the main axis in natural coordinates: from the
// start of the content (padding, header, rows, footer). An inverted list
// lays the same content out from the end (item 0 at the bottom) and keeps
// its distance from the end as content changes.
type listState struct {
	count     int
	estimated Points
	columns   int
	// sizes are the items' measured main-axis sizes; NaN until measured.
	sizes []Points
	// rows holds each row's size (its largest item) plus the gap after it.
	rows   fenwick
	extras [3]listExtra
	// The content's padding along the main axis, across it, and the gaps
	// between rows and between columns.
	padStart, padEnd     Points
	crossStart, crossEnd Points
	gap, crossGap        Points
	horizontal, inverted bool
	// endAligned rests the list at its end (ListAlignEnd): content
	// shorter than the viewport sits at its end, after lead.
	endAligned bool
	// followEnd is AutoscrollToBottomThreshold, or negative when the
	// list does not follow its end.
	followEnd    Points
	sticky       []int // sorted
	stickyOffset Points
	stickyNode   uint32 // the animated node stickyOffset follows, if any
	viewability  ViewabilityConfig
	// offset is the main-axis scroll offset in content coordinates;
	// viewport the viewport's length.
	offset, viewport Points
	// anchoredEnd is contentLen - viewport when the offset was last
	// anchored to the end (inverted lists and followEnd); NaN before.
	anchoredEnd  Points
	spliceAnchor Points
	pin          *listPin
	// moved is set once the user or a scroll command moved the list.
	moved bool
	// lead is the last scroll step reported, in natural coordinates:
	// positive towards the end. Paced binding binds ahead of it first.
	lead Points
	// loadsAtStart: the list has OnLoadMoreAtStart, so an inverted one
	// keeps its visible rows in place while scrolled away from its start,
	// where the pages it loads arrive (keepsVisibleRows).
	loadsAtStart bool
	// parent: virtualized against the view around the list; outerStart
	// is where the list's content starts in that view's content (NaN
	// until placed), limits the list offsets the view scrolls to.
	parent     bool
	outerStart Points
	limits     [2]Points
}

// listExtra is a header, footer or empty cell's presence and size.
type listExtra struct {
	present bool
	size    Points
}

const (
	extraHeader = iota
	extraFooter
	extraEmpty
)

// listBlocks are where the blocks of a list's content start, in natural
// coordinates, and its total length.
type listBlocks struct{ header, body, footer, total Points }

// listPin is a scroll command's target, kept after the command so the
// list follows it as estimated sizes become real ones.
type listPin struct {
	end          bool // ScrollToEnd; else ScrollToIndex
	index        int
	viewPosition float32
	viewOffset   Points
	at           Points // the offset last sent for it
	// animating: an animated scroll the platform has not landed yet; the
	// pin follows by retargeting it.
	animating bool
}

// pinSlack is how far a reported offset may be from a pinned one and
// still count as the same: platforms round offsets to pixels.
const pinSlack Points = 1

// stickyRange is a sticky cell's travel in scroll offsets: from start it
// moves with the offset, as far as end.
type stickyRange = StickyCell

var nan = Points(math.NaN())

func isNaN(p Points) bool { return p != p }

func newListState() listState {
	return listState{estimated: 50, columns: 1, followEnd: -1, anchoredEnd: nan, outerStart: nan}
}

// --- Geometry ---

func (s *listState) effective(i int) Points {
	if v := s.sizes[i]; !isNaN(v) {
		return v
	}
	return s.estimated
}

func (s *listState) rowCount() int   { return (s.count + s.columns - 1) / s.columns }
func (s *listState) rowOf(i int) int { return i / s.columns }
func (s *listState) columnOf(i int) int {
	return i % s.columns
}

// rowSize is row r's size: its largest item.
func (s *listState) rowSize(r int) Points {
	var m Points
	for i := r * s.columns; i < min((r+1)*s.columns, len(s.sizes)); i++ {
		m = max(m, s.effective(i))
	}
	return m
}

func (s *listState) rebuild() {
	rows := make([]Points, s.rowCount())
	for r := range rows {
		rows[r] = s.rowSize(r) + s.gap
	}
	s.rows = newFenwick(rows)
}

// itemsLen is the rows' length without the gap after the last.
func (s *listState) itemsLen() Points {
	if s.count == 0 {
		return 0
	}
	return s.rows.sum(s.rowCount()) - s.gap
}

func (s *listState) blocks() listBlocks {
	at, any := s.padStart, false
	place := func(present bool, size Points) Points {
		if !present {
			return at
		}
		if any {
			at += s.gap
		}
		any = true
		here := at
		at += size
		return here
	}
	var b listBlocks
	b.header = place(s.extras[extraHeader].present, s.extras[extraHeader].size)
	if s.count > 0 {
		b.body = place(true, s.itemsLen())
	} else {
		b.body = place(s.extras[extraEmpty].present, s.extras[extraEmpty].size)
	}
	b.footer = place(s.extras[extraFooter].present, s.extras[extraFooter].size)
	b.total = at + s.padEnd
	return b
}

// contentLen is the content's length along the main axis; an inverted
// list fills its viewport so its rows sit at the bottom.
func (s *listState) contentLen() Points {
	total := s.blocks().total
	if s.inverted || s.endAligned {
		return max(total, s.viewport)
	}
	return total
}

// lead is the room before the content of an end-aligned list shorter
// than its viewport.
func (s *listState) leadRoom() Points {
	if !s.endAligned {
		return 0
	}
	return max(s.viewport-s.blocks().total, 0)
}

// toContent is the content coordinate of a span at natural position at.
func (s *listState) toContent(at, size Points) Points {
	if s.inverted {
		return s.contentLen() - at - size
	}
	return at + s.leadRoom()
}

// naturalOffset is the viewport's start in natural coordinates.
func (s *listState) naturalOffset() Points {
	if s.inverted {
		return s.contentLen() - s.offset - s.viewport
	}
	return s.offset - s.leadRoom()
}

// position is the item the viewport starts in, from the side item 0 is
// on, and how far past its start the viewport starts: index 0 and a
// negative offset in a header before it; false without items.
func (s *listState) position() (index int, offset Points, ok bool) {
	if s.count == 0 {
		return 0, 0, false
	}
	at := s.naturalOffset()
	if body := s.blocks().body; at < body {
		return 0, at - body, true
	}
	first, _, ok := s.itemsIn(at, at)
	if !ok {
		first = s.count - 1
	}
	return first, at - s.itemAt(first), true
}

// itemAt is item i's natural position: its row's start.
func (s *listState) itemAt(i int) Points {
	return s.blocks().body + s.rows.sum(s.rowOf(i))
}

func (s *listState) rowExtent(i int) Points { return s.rowSize(s.rowOf(i)) }

// column is where column col starts across the list and its width, for
// a list cross long.
func (s *listState) column(cross Points, col int) (Points, Points) {
	n := Points(s.columns)
	w := max((cross-s.crossStart-s.crossEnd-s.crossGap*(n-1))/n, 0)
	return s.crossStart + Points(col)*(w+s.crossGap), w
}

// fullWidth is the span of the header, footer and empty cell across.
func (s *listState) fullWidth(cross Points) (Points, Points) {
	return s.crossStart, max(cross-s.crossStart-s.crossEnd, 0)
}

// --- Updates ---

func (s *listState) setCount(count int) {
	if count == s.count {
		return
	}
	if count < len(s.sizes) {
		s.sizes = s.sizes[:count]
	}
	for len(s.sizes) < count {
		s.sizes = append(s.sizes, nan)
	}
	s.count = count
	s.rebuild()
}

func (s *listState) setColumns(n int) {
	n = max(n, 1)
	if n != s.columns {
		s.columns = n
		for i := range s.sizes {
			s.sizes[i] = nan
		}
		s.rebuild()
	}
}

func (s *listState) setSticky(indices []int) {
	sticky := slices.Clone(indices)
	slices.Sort(sticky)
	s.sticky = slices.Compact(sticky)
}

// splice replaces removed items at at by inserted new ones. Measurements
// follow the items that stay, and when the change is above the first
// visible item the visible content keeps its place (spliceAnchor).
func (s *listState) splice(at, removed, inserted int) {
	at = min(at, s.count)
	removed = min(removed, s.count-at)
	if removed == 0 && inserted == 0 {
		return
	}
	s.pin = nil
	// The first visible item and where it is in the viewport. An inverted
	// list at its start (item 0 in view at the bottom) keeps its distance
	// from the end instead, so what arrives there shows. In its parent's
	// viewport, only while the list's start is scrolled past.
	anchor, fromTop, anchored := 0, Points(0), false
	if s.keepsVisibleRows() && s.count > 0 && s.viewport > 0 && (!s.parent || s.offset > 0) {
		top := s.naturalOffset()
		row := s.rows.find(max(top-s.blocks().body, 0))
		anchor = min(row*s.columns, s.count-1)
		fromTop = s.itemAt(anchor) - top
		anchored = true
	}
	naturalBefore := s.naturalOffset()
	fresh := make([]Points, inserted)
	for i := range fresh {
		fresh[i] = nan
	}
	s.sizes = slices.Replace(s.sizes, at, at+removed, fresh...)
	s.count = s.count - removed + inserted
	s.rebuild()
	if anchored && anchor >= at+removed {
		moved := anchor - removed + inserted
		if s.inverted {
			// anchorEnd brings the natural offset back to what it was;
			// the visible rows need it moved as far as they did.
			d := max(s.itemAt(moved)-fromTop, 0) - naturalBefore
			s.offset -= d
			s.spliceAnchor -= d
		} else {
			d := s.itemAt(moved) - fromTop - s.naturalOffset()
			s.offset += d
			s.spliceAnchor += d
		}
	}
}

// keepsVisibleRows reports whether changes before the viewport keep the
// visible rows in place: always for a list that is not inverted. An
// inverted one keeps its distance from the end (the bottom) unless it
// loads at its start and is laid out and scrolled away from it: at its
// start it keeps showing item 0, a chat's newest message.
func (s *listState) keepsVisibleRows() bool {
	return !s.inverted || s.loadsAtStart && !isNaN(s.anchoredEnd) && s.naturalOffset() > pinSlack
}

// beforeViewportChanged is the scroll that keeps the visible rows in
// place, when keep, after something before the viewport changed by d at
// natural offset natural. An inverted list cancels what anchorEnd will
// scroll to keep its distance from the end, without passing its start.
func (s *listState) beforeViewportChanged(keep bool, d, natural Points) Points {
	switch {
	case !keep:
		return 0
	case s.inverted:
		return -max(d, -natural)
	}
	return d
}

// takeSpliceAnchor returns the scroll a splice asked for, once.
func (s *listState) takeSpliceAnchor() Points {
	d := s.spliceAnchor
	s.spliceAnchor = 0
	return d
}

// measured records item i's size and returns the change when its row
// lies wholly before the viewport, which the list scrolls by so the
// visible content stays put.
func (s *listState) measured(i int, size Points) Points {
	if i >= len(s.sizes) || s.sizes[i] == size {
		return 0
	}
	row := s.rowOf(i)
	before := s.rowSize(row)
	start := s.itemAt(i)
	natural := s.naturalOffset()
	keep := start < natural && start+before <= natural && s.keepsVisibleRows()
	s.sizes[i] = size
	d := s.rowSize(row) - before
	if d != 0 {
		s.rows.set(row, s.rows.at(row)+d)
	}
	return s.beforeViewportChanged(keep, d, natural)
}

// measuredExtra records an extra cell's size, anchoring as measured does.
func (s *listState) measuredExtra(kind int, size Points) Points {
	b := s.blocks()
	natural, keeps := s.naturalOffset(), s.keepsVisibleRows()
	e := &s.extras[kind]
	if e.size == size {
		return 0
	}
	d := size - e.size
	e.size = size
	at := b.header
	switch kind {
	case extraFooter:
		at = b.footer
	case extraEmpty:
		at = b.body
	}
	// Wholly before the viewport's start, as it was before the change.
	return s.beforeViewportChanged(keeps && at < natural && at+size-d <= natural, d, natural)
}

// anchorEnd keeps an inverted list's distance from the end of its
// content, starting at the end, and keeps a list that follows its end at
// it while within the threshold. It returns how far to scroll.
func (s *listState) anchorEnd() Points {
	if s.viewport <= 0 {
		return 0
	}
	end := max(s.contentLen()-s.viewport, 0)
	if !s.inverted {
		if s.followEnd < 0 {
			return 0
		}
		was := s.anchoredEnd
		s.anchoredEnd = end
		// At the end, it stays there as the content grows or shrinks:
		// scroll views keep an offset past content that shrank.
		if isNaN(was) || end == was || was-s.offset > s.followEnd {
			return 0
		}
		d := end - s.offset
		s.offset = end
		return d
	}
	var d Points
	if isNaN(s.anchoredEnd) {
		d = end - s.offset
	} else {
		d = end - s.anchoredEnd
	}
	s.anchoredEnd = end
	s.offset += d
	return d
}

// jump moves the list to offset ahead of the platform: the window
// follows it at once, and inverted lists anchor to it.
func (s *listState) jump(offset Points) {
	s.offset = offset
	if s.inverted {
		s.anchoredEnd = max(s.contentLen()-s.viewport, 0)
	}
}

// offsetLimits are the offsets the list scrolls to: its content's, or in
// its parent's viewport its view's, in the list's coordinates.
func (s *listState) offsetLimits() (Points, Points) {
	if s.parent {
		return s.limits[0], s.limits[1]
	}
	return 0, max(s.contentLen()-s.viewport, 0)
}

func (s *listState) clamped(offset Points) Points {
	lo, hi := s.offsetLimits()
	return min(max(offset, lo), hi)
}

// offsetForIndex is the offset that shows item index at viewPosition (0
// the viewport's start, 1 its end), shifted by viewOffset.
func (s *listState) offsetForIndex(index int, viewPosition float32, viewOffset Points) Points {
	index = min(index, max(s.count-1, 0))
	size := s.rowSize(s.rowOf(index))
	at := s.toContent(s.itemAt(index), size)
	free := s.viewport - size
	var offset Points
	if s.inverted {
		offset = at - Points(1-viewPosition)*free + viewOffset
	} else {
		offset = at - Points(viewPosition)*free - viewOffset
	}
	return s.clamped(offset)
}

func (s *listState) endOffset() Points {
	switch {
	case s.inverted:
		return 0
	case s.parent:
		return min(max(s.contentLen()-s.viewport, s.limits[0]), s.limits[1])
	}
	return max(s.contentLen()-s.viewport, 0)
}

func (s *listState) targetOffset(p *listPin) Points {
	if p.end {
		return s.endOffset()
	}
	return s.offsetForIndex(p.index, p.viewPosition, p.viewOffset)
}

// movedTo records the offset the platform reports, with every scroll the
// core issued applied when settled: a pinned target the list is away
// from then was left by the user.
func (s *listState) movedTo(at Points, settled bool) {
	if step := at - s.offset; step != 0 {
		if s.inverted {
			step = -step
		}
		s.lead = step
	}
	if abs32(at-s.offset) > pinSlack {
		// Not a platform's rounding: the user scrolled.
		s.moved = true
	}
	s.offset = at
	p := s.pin
	if p == nil || !settled {
		return
	}
	if abs32(at-p.at) <= pinSlack {
		p.animating = false
	} else if !p.animating {
		s.pin = nil
	}
}

// followPin is how a pinned list follows its target after measurements:
// by is a scroll to add to the anchoring one, to an animated scroll's
// new target. The pin ends once the list is there and every item in the
// viewport is measured.
func (s *listState) followPin(anchored Points) (by Points, to Points, retarget bool) {
	p := s.pin
	if p == nil || s.viewport <= 0 {
		return 0, 0, false
	}
	target := s.targetOffset(p)
	if p.animating {
		p.at += anchored
		if abs32(target-p.at) > pinSlack {
			p.at = target
			return 0, target, true
		}
		return 0, 0, false
	}
	d := target - s.offset
	if d != 0 {
		s.jump(target)
	}
	p.at = target
	if s.viewportMeasured() {
		s.pin = nil
	}
	return d, 0, false
}

func (s *listState) viewportMeasured() bool {
	start := s.naturalOffset()
	first, last, ok := s.itemsIn(start, start+s.viewport)
	if !ok {
		return true
	}
	for _, v := range s.sizes[first : last+1] {
		if isNaN(v) {
			return false
		}
	}
	return true
}

// setOuter places a list in its parent's viewport: its content starts at
// start of the view's content, which is at offset with a viewport that
// long and scrolls as far as maxOffset.
func (s *listState) setOuter(start, offset, viewport, maxOffset Points, settled bool) {
	if shift := start - s.outerStart; shift != 0 && !isNaN(shift) && s.pin != nil {
		// The list moving in its view is no scroll of the user's.
		s.pin.at -= shift
	}
	s.outerStart = start
	s.limits = [2]Points{-start, max(maxOffset-start, -start)}
	s.viewport = viewport
	s.movedTo(offset-start, settled)
}

// --- Window ---

// itemsIn returns the items of the rows overlapping the natural span
// start to end.
func (s *listState) itemsIn(start, end Points) (first, last int, ok bool) {
	if s.count == 0 {
		return 0, 0, false
	}
	body := s.blocks().body
	if end < body || start > body+s.itemsLen() {
		return 0, 0, false
	}
	firstRow := min(s.rows.find(max(start-body, 0)), s.rowCount()-1)
	lastRow := min(s.rows.find(max(end-body, 0)), s.rowCount()-1)
	return firstRow * s.columns, min((lastRow+1)*s.columns-1, s.count-1), true
}

// window returns the items to render: the viewport and a viewport of
// overscan either side, the pinned sticky item and the one before it.
func (s *listState) window() (first, last int, ok bool, sticky []int) {
	if s.count == 0 || s.viewport <= 0 {
		return 0, 0, false, nil
	}
	at := s.naturalOffset()
	over := s.viewport * listOverscan
	first, last, ok = s.itemsIn(at-over, at+s.viewport+over)
	if a, found := s.activeSticky(); found {
		sticky = append(sticky, a)
		// Scrolled back, the one before shows at once, pushed by it.
		if k := slices.Index(s.sticky, a); k > 0 {
			sticky = append(sticky, s.sticky[k-1])
		}
	}
	return first, last, ok, sticky
}

// listOverscan is how far beyond the viewport cells stay rendered, in
// viewports.
const listOverscan = 1

// urgentSpan is the natural span whose rows bind in the commit that shows
// them: the viewport, and ahead of it in the scroll direction twice the
// last step, what a fling's next frames show. A step longer than the
// viewport is a jump: only the viewport. The rest of the window binds
// paced (Tree.pace).
func (s *listState) urgentSpan() (start, end Points) {
	at := s.naturalOffset()
	step := abs32(s.lead)
	ahead := Points(0)
	if step <= s.viewport {
		ahead = min(2*step, s.viewport*listOverscan)
	}
	if s.lead > 0 {
		return at, at + s.viewport + ahead
	}
	return at - ahead, at + s.viewport
}

// overlaps reports whether item i's row overlaps the natural span start
// to end.
func (s *listState) overlaps(i int, start, end Points) bool {
	top := s.itemAt(i)
	return top < end && top+s.rowExtent(i) > start
}

// paceOrder sorts waiting items to bind nearest the viewport first, on
// the side the list scrolls towards before the other.
func (s *listState) paceOrder(items []int) {
	at := s.naturalOffset()
	top, bottom := at, at+s.viewport
	forward := s.lead >= 0
	key := func(i int) (bool, Points) {
		start := s.itemAt(i)
		if start >= bottom {
			return !forward, start - bottom
		}
		return forward, max(top-(start+s.rowExtent(i)), 0)
	}
	slices.SortFunc(items, func(a, b int) int {
		behindA, da := key(a)
		behindB, db := key(b)
		switch {
		case behindA != behindB:
			if behindA {
				return 1
			}
			return -1
		case da < db:
			return -1
		case da > db:
			return 1
		}
		return a - b
	})
}

func (s *listState) stickyEnabled() bool {
	return len(s.sticky) > 0 && s.columns == 1 && !s.inverted
}

// activeSticky is the sticky item pinned now: the last one starting
// before the viewport's start plus the sticky offset.
func (s *listState) activeSticky() (int, bool) {
	if !s.stickyEnabled() {
		return 0, false
	}
	at := s.naturalOffset() + s.stickyOffset
	active, found := 0, false
	for _, i := range s.sticky {
		if i >= s.count || s.itemAt(i) > at {
			break
		}
		active, found = i, true
	}
	return active, found
}

// stickyRange is sticky item index's travel: from where it reaches the
// sticky offset from the viewport's start to where the next sticky item
// pushes it out. In its parent's viewport it is in the view's offsets,
// and the last one stops where its end meets the end of the list's
// content.
func (s *listState) stickyRange(index int, cell NodeID, size Points) StickyCell {
	start := s.itemAt(index) - s.stickyOffset
	end := Points(math.Inf(1))
	if k := slices.IndexFunc(s.sticky, func(i int) bool { return i > index && i < s.count }); k >= 0 {
		end = max(s.itemAt(s.sticky[k])-size-s.stickyOffset, start)
	} else if s.parent {
		end = max(s.blocks().total-s.padEnd-size-s.stickyOffset, start)
	}
	if s.parent {
		start += s.outerStart
		end += s.outerStart
	}
	return StickyCell{Cell: cell, Start: start, End: end, Node: s.stickyNode, Inset: s.stickyOffset}
}

// viewableItems are the items viewable under the list's viewability
// config, as React Native's ViewabilityHelper decides.
func (s *listState) viewableItems() (first, last int, ok bool) {
	if s.viewport <= 0 {
		return 0, 0, false
	}
	top := s.naturalOffset()
	bottom := top + s.viewport
	first, last, ok = s.itemsIn(top, bottom)
	if !ok {
		return 0, 0, false
	}
	viewable := func(row int) bool {
		start := s.itemAt(row * s.columns)
		size := s.rowSize(row)
		end := start + size
		if start >= top && end <= bottom {
			return true
		}
		pixels := max(min(end, bottom)-max(start, top), 0)
		var percent, threshold float32
		switch c := &s.viewability; {
		case c.ViewAreaCoveragePercentThreshold > 0:
			percent, threshold = float32(100*pixels/s.viewport), c.ViewAreaCoveragePercentThreshold
		case c.ItemVisiblePercentThreshold > 0:
			percent, threshold = float32(100*pixels/max(size, 1e-6)), c.ItemVisiblePercentThreshold
		default:
			return pixels > 0
		}
		return percent >= threshold
	}
	firstRow, lastRow := s.rowOf(first), s.rowOf(last)
	for firstRow <= lastRow && !viewable(firstRow) {
		firstRow++
	}
	for lastRow >= firstRow && !viewable(lastRow) {
		lastRow--
	}
	if firstRow > lastRow {
		return 0, 0, false
	}
	return firstRow * s.columns, min((lastRow+1)*s.columns-1, s.count-1), true
}

// listSplice returns how the keys changed from old to new: removed items
// at at replaced by inserted ones, by the common start and end, and false
// when they are the same.
func listSplice(old, new []any) (at, removed, inserted int, changed bool) {
	n := min(len(old), len(new))
	for at < n && old[at] == new[at] {
		at++
	}
	end := 0
	for end < n-at && old[len(old)-1-end] == new[len(new)-1-end] {
		end++
	}
	removed, inserted = len(old)-at-end, len(new)-at-end
	return at, removed, inserted, removed > 0 || inserted > 0
}

func abs32(p Points) Points {
	if p < 0 {
		return -p
	}
	return p
}
