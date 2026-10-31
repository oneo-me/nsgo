package editor

import (
	"slices"
	"sort"

	"github.com/oneo-me/nsgo/core"
)

// The ref's methods for language features and the app's own popups:
// edits applied as one step, folds by nesting, offsets, boxes, scrolling
// and presses.

// Edit applies edits, ranges of the text as it is, as one step undo takes
// back; one overlapping an earlier one is skipped. The selections move
// with the text around them. Code actions and formatting edits go here.
func (r *Ref) Edit(edits []TextEdit) {
	e := r.e
	if e == nil || e.p.ReadOnly || len(edits) == 0 {
		return
	}
	list := make([]edit, len(edits))
	for i, x := range edits {
		from, to := e.pos(x.Range.Start), e.pos(x.Range.End)
		if to.less(from) {
			from, to = to, from
		}
		list[i] = edit{from: from, to: to, text: x.Text}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].from.less(list[j].from) })
	kept := list[:0]
	for _, x := range list {
		if n := len(kept); n > 0 && x.from.less(kept[n-1].to) {
			continue
		}
		kept = append(kept, x)
	}
	e.edit(func() []change {
		before := slices.Clone(e.st.sels)
		changes := e.st.buf.apply(kept)
		// A caret goes after what is inserted at it; a selection grows over
		// what is inserted at its ends.
		next := make([]selection, len(e.st.sels))
		for i, sel := range e.st.sels {
			start, end := mapThrough(changes, sel.start(), !sel.empty()), mapThrough(changes, sel.end(), false)
			if sel.head.less(sel.anchor) {
				start, end = end, start
			}
			next[i] = selection{anchor: start, head: end, goal: -1}
		}
		e.st.afterEdit(changes)
		e.st.setSels(next)
		e.st.buf.record(changes, before, slices.Clone(e.st.sels), kindOther)
		return changes
	})
}

// FoldRecursive folds the fold on line, or the innermost one around it,
// and every fold inside it.
func (r *Ref) FoldRecursive(line int) {
	if e := r.e; e != nil {
		if f, ok := e.foldAt(line); ok {
			e.foldAll(func(c foldRange) bool { return c.start >= f.start && c.end <= f.end })
		}
	}
}

// UnfoldRecursive unfolds the fold on or around line and every fold
// inside it.
func (r *Ref) UnfoldRecursive(line int) {
	e := r.e
	if e == nil {
		return
	}
	f, ok := e.foldAt(line)
	for _, x := range e.st.folded {
		// A folded range around line counts too, as Unfold's does.
		if x.start <= line && line <= x.end && (!ok || x.start < f.start) {
			f, ok = x, true
		}
	}
	if !ok {
		return
	}
	before := len(e.st.folded)
	e.st.folded = slices.DeleteFunc(e.st.folded, func(x foldRange) bool { return x.start >= f.start && x.end <= f.end })
	if len(e.st.folded) != before {
		e.foldsChanged()
	}
}

// FoldLevel folds every fold at nesting level level, 1 the outermost.
func (r *Ref) FoldLevel(level int) {
	if e := r.e; e != nil && level > 0 {
		all := e.st.foldCandidates()
		e.foldAll(func(c foldRange) bool {
			depth := 1
			for _, o := range all {
				if o != c && o.start <= c.start && c.end <= o.end {
					depth++
				}
			}
			return depth == level
		})
	}
}

// FoldRange folds the lines of f, wherever the folds start.
func (r *Ref) FoldRange(f FoldRange) {
	e := r.e
	if e == nil || f.EndLine <= f.StartLine {
		return
	}
	last := e.st.r().lines() - 1
	x := foldRange{min(max(f.StartLine, 0), last), min(f.EndLine, last)}
	if x.end <= x.start || slices.Contains(e.st.folded, x) {
		return
	}
	e.st.folded = append(e.st.folded, x)
	e.caretsOutOf([]foldRange{x})
	e.foldsChanged()
}

// ScrollToLine scrolls line into view, unfolding what hides it.
func (r *Ref) ScrollToLine(line int) { r.RevealPosition(Position{Line: line}, false) }

// RangeBounds returns the boxes of rng, one per row it covers.
func (r *Ref) RangeBounds(rng Range) []Box {
	e := r.e
	if e == nil {
		return nil
	}
	a, b := e.pos(rng.Start), e.pos(rng.End)
	if b.less(a) {
		a, b = b, a
	}
	var out []Box
	for _, x := range e.rowRects(a, b, 0, e.st.disp.count(), nil, "", nil) {
		if box, ok := e.boxOf(x); ok {
			out = append(out, box)
		}
	}
	return out
}

// ScrollOffset returns how far the text is scrolled.
func (r *Ref) ScrollOffset() core.Point {
	if r.e == nil {
		return core.Point{}
	}
	return core.Point{X: r.e.scrollX.GetUntracked(), Y: r.e.scrollY.GetUntracked()}
}

// SetScrollOffset scrolls the text to p, as far as it goes.
func (r *Ref) SetScrollOffset(p core.Point) {
	e := r.e
	if e == nil {
		return
	}
	vp := e.viewport.GetUntracked()
	y := max(min(float32(p.Y), e.contentHeight()-float32(vp.Height)), 0)
	e.vs.ScrollTo(core.Point{Y: core.Points(y)}, false)
	e.scrollY.Set(core.Points(y))
	if !e.p.SoftWrap {
		x := max(min(float32(p.X), e.boxWidth()-(float32(vp.Width)-e.gutterWidth())), 0)
		e.hs.ScrollTo(core.Point{X: core.Points(x)}, false)
		e.scrollX.Set(core.Points(x))
	}
}

// VisibleRange returns the text the viewport shows, from its first row's
// start to its last row's end.
func (r *Ref) VisibleRange() Range {
	e := r.e
	if e == nil {
		return Range{}
	}
	n := e.st.disp.count()
	y, h := float32(e.scrollY.GetUntracked()), float32(e.viewport.GetUntracked().Height)
	first := min(max(int(y/e.lh), 0), n-1)
	last := min(max(int((y+h-1)/e.lh), first), n-1)
	rd := e.st.r()
	a, b := e.st.disp.row(first, rd), e.st.disp.row(last, rd)
	return Range{e.position(pos{a.line, a.start}), e.position(pos{b.line, b.end})}
}

// OffsetToPosition returns the place at offset, in UTF-16 units of the
// text, clamped to it.
func (r *Ref) OffsetToPosition(offset int) Position {
	e := r.e
	if e == nil {
		return Position{}
	}
	rd := e.st.r()
	offset = max(offset, 0)
	for i := range rd.lines() {
		n := utf16Col(rd.line(i), len(rd.line(i)))
		if offset <= n || i == rd.lines()-1 {
			return Position{i, min(offset, n)}
		}
		offset -= n + 1
	}
	return Position{}
}

// PositionToOffset returns p as an offset in UTF-16 units of the text.
func (r *Ref) PositionToOffset(p Position) int {
	if r.e == nil {
		return 0
	}
	return r.e.offset(r.e.pos(p))
}

// foldAt is the fold on line, or the innermost one around it.
func (e *ed) foldAt(line int) (foldRange, bool) {
	var best foldRange
	ok := false
	for _, f := range e.st.foldCandidates() {
		if f.start == line {
			return f, true
		}
		if f.start < line && line <= f.end && (!ok || f.start > best.start) {
			best, ok = f, true
		}
	}
	return best, ok
}

// foldAll folds every fold candidate which takes.
func (e *ed) foldAll(which func(foldRange) bool) {
	var added []foldRange
	for _, f := range e.st.foldCandidates() {
		if which(f) && !slices.Contains(e.st.folded, f) {
			e.st.folded = append(e.st.folded, f)
			added = append(added, f)
		}
	}
	if len(added) > 0 {
		e.caretsOutOf(added)
		e.foldsChanged()
	}
}

// caretsOutOf moves carets the folds hide to the end of their first line.
func (e *ed) caretsOutOf(folds []foldRange) {
	sels := slices.Clone(e.st.sels)
	for i, sel := range sels {
		for _, f := range folds {
			if sel.head.line > f.start && sel.head.line <= f.end {
				sels[i] = caret(pos{f.start, len(e.st.line(f.start))})
			}
		}
	}
	e.st.sels = sels
}

// boxOf is rect x of the rows' box in the editor, its scrolling applied,
// and in the root view; ok is false before the editor is laid out.
func (e *ed) boxOf(x rect) (Box, bool) {
	page, ok := e.box.Bounds()
	if !ok {
		return Box{}, false
	}
	sx := float32(e.scrollX.GetUntracked())
	if e.p.SoftWrap {
		sx = 0
	}
	size := core.Size{Width: core.Points(x.w), Height: core.Points(x.h)}
	return Box{
		Bounds: core.Bounds{Origin: core.Point{X: core.Points(e.gutterWidth() + x.x - sx), Y: core.Points(x.y - float32(e.scrollY.GetUntracked()))}, Size: size},
		Page:   core.Bounds{Origin: core.Point{X: page.Origin.X + core.Points(x.x), Y: page.Origin.Y + core.Points(x.y)}, Size: size},
	}, true
}

// charAt is the character at pt in the rows' box; ok is false off the
// text, past a row's end or outside the rows.
func (e *ed) charAt(pt core.Point) (pos, bool) {
	if pt.X < 0 || pt.Y < 0 || int(float32(pt.Y)/e.lh) >= e.st.disp.count() {
		return pos{}, false
	}
	near := e.posAt(pt)
	d := e.st.disp.row(int(float32(pt.Y)/e.lh), e.st.r())
	if e.xOf(near) > float32(pt.X) {
		if near.col > d.start {
			return e.st.prevChar(near), true
		}
		return near, false
	}
	return near, near.col < d.end
}

// textPress reports a press at pt of the rows' box, page in the root
// view, over a character.
func (e *ed) textPress(pt, page core.Point, mods core.Modifiers, clicks int) {
	if e.p.OnTextPress == nil {
		return
	}
	p, ok := e.charAt(pt)
	if !ok {
		return
	}
	a, b := e.st.lang.wordAt(e.st.line(p.line), p.col)
	e.p.OnTextPress(TextPressEvent{Position: e.position(p), Offset: e.offset(p), Word: e.rangeOf(span{pos{p.line, a}, pos{p.line, b}}),
		At: page, Modifiers: mods, Clicks: max(clicks, 1)})
}
