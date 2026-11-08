package richeditor

import (
	"math"

	"github.com/oneo-me/nsgo/core"
)

// Block drags are Tiptap's drag handle over ProseMirror's view.dragging:
// the app's handle carries the block at a position, or the siblings a
// selection covers, to a drop cursor between blocks, and the drop is one
// undo step.

// DragSource is what a handle at pos carries, from to to: the siblings the
// selection covers when pos is in a selection over several, else the block
// at pos, a defining parent standing for its first child, as a list item for
// its paragraph. ok is false where there is no block.
func DragSource(s *State, pos int) (from, to int, ok bool) {
	r := s.Doc.Resolve(pos)
	depth := r.Depth
	if after := r.NodeAfter(); after != nil && after.IsBlock() {
		depth = r.Depth + 1
	}
	if depth == 0 {
		return 0, 0, false
	}
	for depth > 1 && r.Index(depth-1) == 0 {
		parent := r.Node(depth - 1)
		if !parent.Type.Spec.Defining || parent.Type.Spec.Isolating || tableRole(parent.Type) != "" {
			break
		}
		depth--
	}
	before := func(d int) int {
		if d == r.Depth+1 {
			return r.Pos
		}
		return r.Before(d)
	}
	after := func(d int) int {
		if d == r.Depth+1 {
			return r.Pos + r.NodeAfter().NodeSize()
		}
		return r.After(d)
	}
	from, to = before(depth), after(depth)
	sel := s.Selection
	if _, cells := sel.(*CellSelection); !cells && !sel.Empty() && sel.From() <= pos && pos <= sel.To() {
		if rng := sel.FromPos().BlockRange(sel.ToPos(), nil); rng != nil && rng.Depth == depth-1 && rng.EndIndex()-rng.StartIndex() > 1 {
			return rng.Start(), rng.End(), true
		}
	}
	return from, to, true
}

// MoveBlocks moves the blocks from from to to to target, a position
// between blocks outside them whose parent takes them, as one transaction
// that selects what moved.
func MoveBlocks(from, to, target int) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if from >= to || target >= from && target <= to || target < 0 || target > s.Doc.Content.Size {
			return false
		}
		content := s.Doc.Slice(from, to, false).Content
		if !canDrop(s.Doc, target, content) {
			return false
		}
		if dispatch == nil {
			return true
		}
		tr := s.Tr().DeleteRange(from, to)
		at := tr.Mapping.Map(target, 1)
		if target < from {
			at = tr.Mapping.Map(target, -1)
		}
		tr.Insert(at, content.Content...)
		if tr.Err != nil {
			return false
		}
		if content.ChildCount() == 1 && isNodeSelectable(content.Child(0)) {
			tr.SetSelection(NodeSelectionAt(tr.Doc, at))
		} else {
			tr.SetSelection(TextSelectionBetween(tr.Doc.Resolve(at), tr.Doc.Resolve(at+content.Size), 0))
		}
		tr.Origin = OriginDrop
		tr.SetMeta(uiEvent, "drop")
		dispatch(tr.ScrollIntoView())
		return true
	}
}

// canDrop says the parent at pos, a position between blocks, takes content
// there.
func canDrop(doc *Node, pos int, content *Fragment) bool {
	r := doc.Resolve(pos)
	if r.Parent().InlineContent() || r.TextOffset() != 0 {
		return false
	}
	i := r.Index(r.Depth)
	return r.Parent().CanReplace(i, i, content, 0, -1)
}

// blockDragState is a handle's drag: what it carries and where it drops,
// target negative for nowhere, and the drop cursor's line.
type blockDragState struct {
	on       bool
	from, to int
	target   int
	line     Rect2
}

// pageOrigin is the content box's top left in the surface.
func (v *rv) pageOrigin() (float64, float64, bool) {
	b, ok := v.content.Bounds()
	return float64(b.Origin.X), float64(b.Origin.Y), ok
}

// PageCoordsAtPos implements pageGeometry: CoordsAtPos in surface points.
func (v *rv) PageCoordsAtPos(pos int) (Rect2, bool) {
	r, ok := v.CoordsAtPos(pos)
	x, y, have := v.pageOrigin()
	if !ok || !have {
		return Rect2{}, false
	}
	r.X, r.Y = r.X+x, r.Y+y
	return r, true
}

// PosAtPageCoords implements pageGeometry.
func (v *rv) PosAtPageCoords(x, y float64) (int, bool) {
	ox, oy, ok := v.pageOrigin()
	if !ok {
		return 0, false
	}
	return v.PosAtCoords(x-ox, y-oy)
}

func (v *rv) dragBlock(pos int, x, y float64) bool {
	st := v.s.State()
	if !v.s.Editable() {
		return false
	}
	from, to, ok := DragSource(st, pos)
	if !ok {
		return false
	}
	if node := st.Doc.NodeAt(from); node != nil && node.NodeSize() == to-from && isNodeSelectable(node) {
		v.s.Dispatch(st.Tr().SetSelection(NodeSelectionAt(st.Doc, from)))
	}
	v.blockDrag.Set(blockDragState{on: true, from: from, to: to, target: -1})
	v.dragTo(x, y)
	return true
}

func (v *rv) dragTo(x, y float64) {
	d := v.blockDrag.GetUntracked()
	if !d.on {
		return
	}
	_, oy, ok := v.pageOrigin()
	if !ok {
		return
	}
	content := v.s.State().Doc.Slice(d.from, d.to, false).Content
	d.target, d.line = v.dropTarget(y-oy, d.from, d.to, content)
	if d.target >= d.from && d.target <= d.to {
		d.target = -1
	}
	v.blockDrag.Set(d)
}

func (v *rv) endDrag() bool {
	d := v.blockDrag.GetUntracked()
	if !d.on {
		return false
	}
	v.blockDrag.Set(blockDragState{target: -1})
	return d.target >= 0 && v.s.Run(MoveBlocks(d.from, d.to, d.target))
}

// dropTarget is where blocks dropped at a point of the content box land:
// before or after the innermost block at its height whose parent takes
// them, by the half the point is over, wherever it is across; off every
// block, beside the nearest that takes them. It is -1 for nowhere.
func (v *rv) dropTarget(y float64, from, to int, content *Fragment) (int, Rect2) {
	doc := v.s.State().Doc
	best, bestAt, bestLine := math.Inf(1), -1, Rect2{}
	inner := -1
	var innerLine Rect2
	eachBlock(v.root, func(b *block) bool {
		if b.pos >= from && b.pos < to {
			return true
		}
		if b.parent != nil && tableRole(b.parent.typ) == "table" {
			return true
		}
		r, ok := v.boxOf(b)
		if !ok {
			return true
		}
		pos, line := b.pos, Rect2{r.X, r.Y, r.Width, 0}
		if y >= r.Y+r.Height/2 {
			pos, line.Y = b.pos+b.nowNode.NodeSize(), r.Y+r.Height
		}
		if !canDrop(doc, pos, content) {
			return true
		}
		if y >= r.Y && y < r.Y+r.Height {
			inner, innerLine = pos, line
			return true
		}
		if d := max(r.Y-y, y-(r.Y+r.Height)); d <= best {
			best, bestAt, bestLine = d, pos, line
		}
		return true
	})
	if inner >= 0 {
		return inner, innerLine
	}
	return bestAt, bestLine
}

// dropCursor is the line where dragged blocks will land.
func (v *rv) dropCursor() core.Node {
	shown := func() bool { d := v.blockDrag.Get(); return d.on && d.target >= 0 }
	return core.Show{When: shown, Then: func(cx *core.Scope) core.Node {
		return core.View{Class: v.p.Classes.DropCursor, TestID: "rich-editor-drop-cursor", PointerEvents: core.PointerEventsNone,
			StyleFunc: func() core.Style {
				l := v.blockDrag.Get().line
				return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(l.X)), Top: core.Pt(core.Points(l.Y) - 1),
					Width: core.Pt(core.Points(l.Width)), Height: core.Pt(2)}.Merge(v.looks.Get().DropCursor)
			}}
	}}
}
