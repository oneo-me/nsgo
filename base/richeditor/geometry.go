package richeditor

import (
	"math"
	"slices"
	"sort"

	"github.com/oneo-me/nsgo/core"
)

// boxOf is a block's box in the content box.
func (v *rv) boxOf(b *block) (Rect2, bool) {
	return v.refBox(b.ref)
}

func (v *rv) refBox(r *core.Ref) (Rect2, bool) {
	box, ok := r.Bounds()
	origin, have := v.content.Bounds()
	if !ok || !have {
		return Rect2{}, false
	}
	return Rect2{float64(box.Origin.X - origin.Origin.X), float64(box.Origin.Y - origin.Origin.Y), float64(box.Size.Width), float64(box.Size.Height)}, true
}

// textblockAt is the textblock holding pos, nil for none.
func (v *rv) textblockAt(pos int) *block {
	at := sort.Search(len(v.textblocks), func(i int) bool {
		b := v.textblocks[i]
		return b.pos+1+b.nowNode.Content.Size >= pos
	})
	if at < len(v.textblocks) && v.textblocks[at].pos < pos {
		return v.textblocks[at]
	}
	return nil
}

// CoordsAtPos implements Geometry.
func (v *rv) CoordsAtPos(pos int) (Rect2, bool) {
	b := v.textblockAt(pos)
	if b == nil {
		for _, l := range v.leaves {
			if l.pos == pos {
				if r, ok := v.boxOf(l); ok {
					r.Width = 0
					return r, true
				}
			}
		}
		return Rect2{}, false
	}
	frame, ok := v.boxOf(b)
	if !ok {
		return Rect2{}, false
	}
	off := pos - b.pos - 1
	text := b.textOf(off)
	// A composition's caret is after what it composed.
	if st := v.s.State(); v.composing.GetUntracked() != "" && st.Selection.Head() == pos {
		for _, r := range b.runs {
			if r.docLen == 0 && r.docAt == off {
				text = r.textAt + r.textLen
			}
		}
	}
	box, ok := b.ref.TextCaretBox(text, false)
	if !ok {
		return Rect2{frame.X, frame.Y, 0, frame.Height}, true
	}
	return Rect2{frame.X + float64(box.Origin.X), frame.Y + float64(box.Origin.Y), 0, float64(box.Size.Height)}, true
}

func rectDistance(r Rect2, x, y float64) float64 {
	dx := max(r.X-x, 0, x-(r.X+r.Width))
	dy := max(r.Y-y, 0, y-(r.Y+r.Height))
	// Height first: a click beside a line goes to that line.
	return dy*4096 + dx
}

// cellOf is the table cell around b, nil for none.
func cellOf(b *block) *block {
	for p := b.parent; p != nil; p = p.parent {
		if isCellType(p.typ) {
			return p
		}
	}
	return nil
}

// hit is the nearest textblock to a point, a press in a cell staying in that
// cell, since a neighbor beside it may be nearer.
func (v *rv) hit(x, y float64) *block {
	var inCell *block
	for _, t := range v.tables {
		for _, c := range t.kids.GetUntracked() {
			if r, ok := v.boxOf(c); ok && x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
				inCell = c
			}
		}
	}
	best, bestAt := math.Inf(1), (*block)(nil)
	for _, b := range v.textblocks {
		if inCell != nil && cellOf(b) != inCell {
			continue
		}
		if r, ok := v.boxOf(b); ok {
			if d := rectDistance(r, x, y); d < best {
				best, bestAt = d, b
			}
		}
	}
	return bestAt
}

// PosAtCoords implements Geometry.
func (v *rv) PosAtCoords(x, y float64) (int, bool) {
	b := v.hit(x, y)
	if b == nil {
		return 0, false
	}
	frame, _ := v.boxOf(b)
	t, _, ok := b.ref.TextCaretAt(core.Point{X: core.Points(x - frame.X), Y: core.Points(y - frame.Y)})
	if !ok {
		return b.pos + 1, true
	}
	return b.pos + 1 + b.docOf(t), true
}

// leafAt is the atom block at a point.
func (v *rv) leafAt(x, y float64) *block {
	for _, l := range v.leaves {
		if r, ok := v.boxOf(l); ok && x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
			return l
		}
	}
	return nil
}

func (v *rv) selectionRects() []Rect2 {
	st := v.s.State()
	sel := st.Selection
	var out []Rect2
	switch sel.(type) {
	case *CellSelection, *NodeSelection:
		return nil
	}
	if sel.Empty() {
		return nil
	}
	from, to := sel.From(), sel.To()
	for _, b := range v.textblocks {
		start, end := b.pos+1, b.pos+1+b.nowNode.Content.Size
		if end < from {
			continue
		}
		if start > to {
			break
		}
		frame, ok := v.boxOf(b)
		if !ok {
			continue
		}
		a, z := b.textOf(max(from, start)-start), b.textOf(min(to, end)-start)
		boxes := b.ref.TextRangeBoxes(a, z)
		for _, box := range boxes {
			out = append(out, Rect2{frame.X + float64(box.Origin.X), frame.Y + float64(box.Origin.Y), float64(box.Size.Width), float64(box.Size.Height)})
		}
		// A selected line break shows as a sliver.
		if len(boxes) == 0 || to > end {
			if c, ok := b.ref.TextCaretBox(z, false); ok {
				out = append(out, Rect2{frame.X + float64(c.Origin.X), frame.Y + float64(c.Origin.Y), 4, float64(c.Size.Height)})
			}
		}
	}
	return out
}

// SelectionRect implements Geometry: the union of the selection's lines, or
// of its cells, or the node it holds.
func (v *rv) SelectionRect() (Rect2, bool) {
	var boxes []Rect2
	switch s := v.s.State().Selection.(type) {
	case *CellSelection:
		table := s.AnchorCell.Node(-1)
		start := s.AnchorCell.Start(-1)
		m := TableMapOf(table)
		cells := m.CellsInRect(m.RectBetween(s.AnchorCell.Pos-start, s.HeadCell.Pos-start))
		for _, t := range v.tables {
			if t.nowNode != table {
				continue
			}
			for _, k := range t.kids.GetUntracked() {
				if slices.Contains(cells, k.pos-start) {
					if r, ok := v.boxOf(k); ok {
						boxes = append(boxes, r)
					}
				}
			}
		}
	case *NodeSelection:
		for _, l := range v.leaves {
			if l.pos == s.From() {
				if r, ok := v.boxOf(l); ok {
					boxes = append(boxes, r)
				}
			}
		}
	default:
		boxes = v.selectionRects()
		if len(boxes) == 0 {
			if r, ok := v.CoordsAtPos(s.Head()); ok {
				boxes = []Rect2{r}
			}
		}
	}
	if len(boxes) == 0 {
		return Rect2{}, false
	}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, b := range boxes {
		x0, y0, x1, y1 = min(x0, b.X), min(y0, b.Y), max(x1, b.X+b.Width), max(y1, b.Y+b.Height)
	}
	return Rect2{x0, y0, x1 - x0, y1 - y0}, true
}

// revealCaret scrolls the head into view after a change that asks to.
func (v *rv) revealCaret() {
	if !v.reveal {
		return
	}
	r, ok := v.CoordsAtPos(v.s.State().Selection.Head())
	if !ok {
		return
	}
	v.reveal = false
	height := float64(v.viewport.GetUntracked().Height)
	y := float64(v.scrollY.GetUntracked())
	const margin = 4
	switch {
	case height <= 0:
	case r.Y-margin < y:
		v.vs.ScrollTo(core.Point{Y: core.Points(max(r.Y-margin, 0))}, false)
	case r.Y+r.Height+margin > y+height:
		v.vs.ScrollTo(core.Point{Y: core.Points(r.Y + r.Height + margin - height)}, false)
	}
}
