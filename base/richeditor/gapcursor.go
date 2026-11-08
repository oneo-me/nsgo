package richeditor

// The gap cursor is prosemirror-gapcursor's (MIT, © Marijn Haverbeke and
// others): a cursor between blocks where no textblock is, such as before a
// table that starts the document or between two images, which typing fills
// with the default textblock.

// GapCursor is a cursor at a position no text selection can hold.
type GapCursor struct{ selBase }

// NewGapCursor is a gap cursor at pos, which GapCursorValid accepts.
func NewGapCursor(pos *ResolvedPos) *GapCursor { return &GapCursor{newBase(pos, pos, nil)} }

func (s *GapCursor) Eq(o Selection) bool {
	g, ok := o.(*GapCursor)
	return ok && g.Head() == s.Head()
}

func (s *GapCursor) Map(doc *Node, m Mappable) Selection {
	return gapOrNear(doc.Resolve(m.Map(s.Head(), 1)))
}

func (s *GapCursor) Content() *Slice { return EmptySlice }

func (s *GapCursor) Bookmark() Bookmark { return gapBookmark{s.Head()} }

type gapBookmark struct{ pos int }

func (b gapBookmark) Map(m Mappable) Bookmark     { return gapBookmark{m.Map(b.pos, 1)} }
func (b gapBookmark) Resolve(doc *Node) Selection { return gapOrNear(doc.Resolve(b.pos)) }

func gapOrNear(r *ResolvedPos) Selection {
	if GapCursorValid(r) {
		return NewGapCursor(r)
	}
	return SelectionNear(r, 1)
}

// GapCursorValid says a gap cursor may stand at pos: between closed blocks,
// in a parent whose default content there is a textblock, unless its spec's
// Extra["allowGapCursor"] says otherwise.
func GapCursorValid(r *ResolvedPos) bool {
	parent := r.Parent()
	if parent.IsTextblock() || !closedBefore(r) || !closedAfter(r) {
		return false
	}
	if allow, ok := parent.Type.Spec.Extra["allowGapCursor"].(bool); ok {
		return allow
	}
	t := parent.contentMatchAt(r.Index(r.Depth)).defaultType()
	return t != nil && t.IsTextblock()
}

func closedBefore(r *ResolvedPos) bool {
	for d := r.Depth; d >= 0; d-- {
		index, parent := r.Index(d), r.Node(d)
		if index == 0 {
			if parent.Type.Spec.Isolating {
				return true
			}
			continue
		}
		for before := parent.Child(index - 1); ; before = before.LastChild() {
			if before.ChildCount() == 0 && !before.InlineContent() || before.IsAtom() || before.Type.Spec.Isolating {
				return true
			}
			if before.InlineContent() {
				return false
			}
		}
	}
	return true
}

func closedAfter(r *ResolvedPos) bool {
	for d := r.Depth; d >= 0; d-- {
		index, parent := r.IndexAfter(d), r.Node(d)
		if index == parent.ChildCount() {
			if parent.Type.Spec.Isolating {
				return true
			}
			continue
		}
		for after := parent.Child(index); ; after = after.FirstChild() {
			if after.ChildCount() == 0 && !after.InlineContent() || after.IsAtom() || after.Type.Spec.Isolating {
				return true
			}
			if after.InlineContent() {
				return false
			}
		}
	}
	return true
}

// FindGapCursorFrom is the first gap from pos going dir, moving off pos
// when mustMove; nil for none.
func FindGapCursorFrom(r *ResolvedPos, dir int, mustMove bool) *ResolvedPos {
	doc := r.Doc()
search:
	for {
		if !mustMove && GapCursorValid(r) {
			return r
		}
		pos := r.Pos
		var next *Node
		for d := r.Depth; ; d-- {
			parent := r.Node(d)
			if dir > 0 && r.IndexAfter(d) < parent.ChildCount() {
				next = parent.Child(r.IndexAfter(d))
				break
			}
			if dir < 0 && r.Index(d) > 0 {
				next = parent.Child(r.Index(d) - 1)
				break
			}
			if d == 0 {
				return nil
			}
			pos += dir
			if cur := doc.Resolve(pos); GapCursorValid(cur) {
				return cur
			}
		}
		for {
			inside := next.FirstChild()
			if dir < 0 {
				inside = next.LastChild()
			}
			if inside == nil {
				if next.IsAtom() && !next.IsText() && !isNodeSelectable(next) {
					r = doc.Resolve(pos + next.NodeSize()*dir)
					mustMove = false
					continue search
				}
				break
			}
			next = inside
			pos += dir
			if cur := doc.Resolve(pos); GapCursorValid(cur) {
				return cur
			}
		}
		return nil
	}
}

// gapArrow moves to the next gap going dir along axis ("horiz", "vert"),
// from a text selection only at its textblock's end that way.
func gapArrow(v *Session, axis string, dir int) bool {
	st := v.state
	sel := st.Selection
	start := sel.FromPos()
	if dir > 0 {
		start = sel.ToPos()
	}
	mustMove := sel.Empty()
	if _, ok := sel.(*TextSelection); ok {
		if !v.endOfTextblock(axis, dir) || start.Depth == 0 {
			return false
		}
		mustMove = false
		if dir > 0 {
			start = st.Doc.Resolve(start.After(start.Depth))
		} else {
			start = st.Doc.Resolve(start.Before(start.Depth))
		}
	}
	found := FindGapCursorFrom(start, dir, mustMove)
	if found == nil {
		return false
	}
	v.Dispatch(st.Tr().SetSelection(NewGapCursor(found)).ScrollIntoView())
	return true
}

// endOfTextblock says the head is at its textblock's edge going dir: its
// first or last position across, its first or last line by the view's
// geometry, else its first or last position, up and down.
func (v *Session) endOfTextblock(axis string, dir int) bool {
	head := v.state.Selection.HeadPos()
	if !head.Parent().InlineContent() {
		return true
	}
	edge := head.Start(head.Depth)
	if dir > 0 {
		edge = head.End(head.Depth)
	}
	if axis == "horiz" || head.Pos == edge {
		return head.Pos == edge
	}
	g := v.geometry()
	if g == nil {
		return false
	}
	at, ok1 := g.CoordsAtPos(head.Pos)
	end, ok2 := g.CoordsAtPos(edge)
	if !ok1 || !ok2 {
		return false
	}
	return at.Y+at.Height/2 > end.Y && at.Y+at.Height/2 < end.Y+end.Height
}

// OpenGap fills a gap cursor with the textblock its place takes, and puts
// a text cursor in it, for an input method to compose there; it says
// whether it did.
func (v *Session) OpenGap() bool {
	sel, ok := v.state.Selection.(*GapCursor)
	if !ok || !v.editable {
		return false
	}
	from := sel.FromPos()
	wrap := from.Parent().contentMatchAt(from.Index(from.Depth)).FindWrapping(v.state.Schema.Nodes["text"])
	if wrap == nil {
		return false
	}
	frag := EmptyFragment()
	for i := len(wrap) - 1; i >= 0; i-- {
		frag = FragmentFrom(wrap[i].CreateAndFill(nil, frag, nil))
	}
	tr := v.state.Tr().Replace(from.Pos, from.Pos, NewSlice(frag, 0, 0))
	tr.SetSelection(SelectionNear(tr.Doc.Resolve(from.Pos+1), 1))
	tr.Origin = OriginInput
	v.Dispatch(tr)
	return true
}

// GapCursorPlugin is prosemirror-gapcursor's plugin: arrows move into gaps,
// a press in one puts the cursor there, and the view draws it.
func GapCursorPlugin() *Plugin {
	return &Plugin{Key: "gapCursor",
		HandleKeyDown: func(v *Session, key string) bool {
			switch key {
			case "ArrowLeft":
				return gapArrow(v, "horiz", -1)
			case "ArrowRight":
				return gapArrow(v, "horiz", 1)
			case "ArrowUp":
				return gapArrow(v, "vert", -1)
			case "ArrowDown":
				return gapArrow(v, "vert", 1)
			}
			return false
		},
		HandleClick: func(v *Session, pos, clicks int, _ bool) bool {
			if !v.editable || clicks != 1 {
				return false
			}
			r := v.state.Doc.Resolve(pos)
			if !GapCursorValid(r) {
				return false
			}
			v.Dispatch(v.state.Tr().SetSelection(NewGapCursor(r)))
			return true
		}}
}
