package richeditor

// SelectionRange is a range a selection covers.
type SelectionRange struct{ From, To *ResolvedPos }

// Selection is what is selected: text, a node, everything or a table's
// cells. Each kind maps through changes, gives its content as a Slice,
// replaces it, and has a bookmark that maps without a document.
type Selection interface {
	Anchor() int
	Head() int
	From() int
	To() int
	FromPos() *ResolvedPos
	ToPos() *ResolvedPos
	HeadPos() *ResolvedPos
	AnchorPos() *ResolvedPos
	Ranges() []SelectionRange
	Empty() bool
	Eq(Selection) bool
	Map(doc *Node, m Mappable) Selection
	Content() *Slice
	Replace(tr *Transaction, content *Slice)
	ReplaceWith(tr *Transaction, node *Node)
	Bookmark() Bookmark
}

// Bookmark is a selection without a document.
type Bookmark interface {
	Map(m Mappable) Bookmark
	Resolve(doc *Node) Selection
}

// selBase holds what the kinds share.
type selBase struct {
	anchor, head *ResolvedPos
	ranges       []SelectionRange
}

func newBase(anchor, head *ResolvedPos, ranges []SelectionRange) selBase {
	if ranges == nil {
		from, to := anchor, head
		if head.Pos < anchor.Pos {
			from, to = head, anchor
		}
		ranges = []SelectionRange{{from, to}}
	}
	return selBase{anchor, head, ranges}
}

func (s *selBase) Anchor() int              { return s.anchor.Pos }
func (s *selBase) Head() int                { return s.head.Pos }
func (s *selBase) HeadPos() *ResolvedPos    { return s.head }
func (s *selBase) AnchorPos() *ResolvedPos  { return s.anchor }
func (s *selBase) FromPos() *ResolvedPos    { return s.ranges[0].From }
func (s *selBase) ToPos() *ResolvedPos      { return s.ranges[0].To }
func (s *selBase) From() int                { return s.ranges[0].From.Pos }
func (s *selBase) To() int                  { return s.ranges[0].To.Pos }
func (s *selBase) Ranges() []SelectionRange { return s.ranges }

func (s *selBase) Empty() bool {
	for _, r := range s.ranges {
		if r.From.Pos != r.To.Pos {
			return false
		}
	}
	return true
}

func (s *selBase) Content() *Slice {
	return s.FromPos().Doc().Slice(s.From(), s.To(), true)
}

func (s *selBase) Replace(tr *Transaction, content *Slice) {
	if content == nil {
		content = EmptySlice
	}
	lastNode := content.Content.LastChild()
	var lastParent *Node
	for i := 0; i < content.OpenEnd; i++ {
		lastParent = lastNode
		lastNode = lastNode.LastChild()
	}
	mapFrom := len(tr.Steps)
	for i, r := range s.ranges {
		mapping := tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps))
		c := EmptySlice
		if i == 0 {
			c = content
		}
		tr.ReplaceRange(mapping.Map(r.From.Pos, 1), mapping.Map(r.To.Pos, 1), c)
		if i == 0 {
			bias := -1
			if lastNode != nil && !lastNode.IsInline() || lastNode == nil && lastParent != nil && lastParent.IsTextblock() {
				bias = 1
			}
			selectionToInsertionEnd(tr, mapFrom, bias)
		}
	}
}

func (s *selBase) ReplaceWith(tr *Transaction, node *Node) {
	mapFrom := len(tr.Steps)
	for i, r := range s.ranges {
		mapping := tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps))
		from, to := mapping.Map(r.From.Pos, 1), mapping.Map(r.To.Pos, 1)
		if i > 0 {
			tr.DeleteRange(from, to)
			continue
		}
		tr.ReplaceRangeWith(from, to, node)
		bias := -1
		if !node.IsInline() {
			bias = 1
		}
		selectionToInsertionEnd(tr, mapFrom, bias)
	}
}

// selectionToInsertionEnd puts the selection after what the steps from
// start replaced.
func selectionToInsertionEnd(tr *Transaction, start, bias int) {
	last := len(tr.Steps) - 1
	if last < start {
		return
	}
	switch step := tr.Steps[last].(type) {
	case *ReplaceStep, *ReplaceAroundStep:
		_ = step
	default:
		return
	}
	end := -1
	tr.Mapping.Maps[last].ForEach(func(_, _, _, newTo int) {
		if end < 0 {
			end = newTo
		}
	})
	if end >= 0 {
		tr.SetSelection(SelectionNear(tr.Doc.Resolve(end), bias))
	}
}

// TextSelection is a caret or range in inline content.
type TextSelection struct{ selBase }

// NewTextSelection selects from anchor to head, which sit in inline
// content; head is anchor when nil.
func NewTextSelection(anchor, head *ResolvedPos) *TextSelection {
	if head == nil {
		head = anchor
	}
	return &TextSelection{newBase(anchor, head, nil)}
}

// TextSelectionBetween is the text selection nearest the two positions.
func TextSelectionBetween(anchor, head *ResolvedPos, bias int) Selection {
	dPos := anchor.Pos - head.Pos
	if bias == 0 || dPos != 0 {
		if dPos >= 0 {
			bias = 1
		} else {
			bias = -1
		}
	}
	if !head.Parent().InlineContent() {
		found := SelectionFindFrom(head, bias, true)
		if found == nil {
			found = SelectionFindFrom(head, -bias, true)
		}
		if found == nil {
			return SelectionNear(anchor, bias)
		}
		head = found.HeadPos()
	}
	if !anchor.Parent().InlineContent() {
		if dPos == 0 {
			anchor = head
		} else {
			found := SelectionFindFrom(anchor, -bias, true)
			if found == nil {
				found = SelectionFindFrom(anchor, bias, true)
			}
			if found == nil {
				return NewTextSelection(anchor, head)
			}
			anchor = found.AnchorPos()
			if (anchor.Pos < head.Pos) != (dPos < 0) {
				anchor = head
			}
		}
	}
	return NewTextSelection(anchor, head)
}

// TextSelectionAt selects from anchor to head.
func TextSelectionAt(doc *Node, anchor, head int) Selection {
	a := doc.Resolve(anchor)
	return TextSelectionBetween(a, doc.Resolve(head), 0)
}

// Cursor is the empty selection at the head, nil unless it is one.
func (s *TextSelection) Cursor() *ResolvedPos {
	if s.anchor.Pos == s.head.Pos {
		return s.head
	}
	return nil
}

func (s *TextSelection) Eq(o Selection) bool {
	t, ok := o.(*TextSelection)
	return ok && t.Anchor() == s.Anchor() && t.Head() == s.Head()
}

func (s *TextSelection) Map(doc *Node, m Mappable) Selection {
	head := doc.Resolve(m.Map(s.Head(), 1))
	if !head.Parent().InlineContent() {
		return SelectionNear(head, 1)
	}
	return NewTextSelection(doc.Resolve(m.Map(s.Anchor(), 1)), head)
}

func (s *TextSelection) Replace(tr *Transaction, content *Slice) {
	s.selBase.Replace(tr, content)
	if content == nil || content == EmptySlice {
		if marks := s.FromPos().MarksAcross(s.ToPos()); marks != nil {
			tr.EnsureMarks(marks)
		}
	}
}

func (s *TextSelection) Bookmark() Bookmark { return textBookmark{s.Anchor(), s.Head()} }

type textBookmark struct{ anchor, head int }

func (b textBookmark) Map(m Mappable) Bookmark {
	return textBookmark{m.Map(b.anchor, 1), m.Map(b.head, 1)}
}
func (b textBookmark) Resolve(doc *Node) Selection { return TextSelectionAt(doc, b.anchor, b.head) }

// NodeSelection selects one node.
type NodeSelection struct {
	selBase
	Node *Node
}

// NewNodeSelection selects the node after pos.
func NewNodeSelection(pos *ResolvedPos) *NodeSelection {
	node := pos.NodeAfter()
	end := pos.Node(0).Resolve(pos.Pos + node.NodeSize())
	return &NodeSelection{newBase(pos, end, nil), node}
}

// NodeSelectionAt selects the node at from.
func NodeSelectionAt(doc *Node, from int) *NodeSelection { return NewNodeSelection(doc.Resolve(from)) }

func isNodeSelectable(n *Node) bool { return !n.IsText() && !n.Type.Spec.NoSelect }

func (s *NodeSelection) Eq(o Selection) bool {
	n, ok := o.(*NodeSelection)
	return ok && n.Anchor() == s.Anchor()
}

func (s *NodeSelection) Map(doc *Node, m Mappable) Selection {
	r := m.MapResult(s.Anchor(), 1)
	pos := doc.Resolve(r.Pos)
	if r.Deleted() {
		return SelectionNear(pos, 1)
	}
	return NewNodeSelection(pos)
}

func (s *NodeSelection) Content() *Slice { return NewSlice(FragmentFrom(s.Node), 0, 0) }

func (s *NodeSelection) Bookmark() Bookmark { return nodeBookmark{s.Anchor()} }

type nodeBookmark struct{ anchor int }

func (b nodeBookmark) Map(m Mappable) Bookmark {
	r := m.MapResult(b.anchor, 1)
	if r.Deleted() {
		return textBookmark{r.Pos, r.Pos}
	}
	return nodeBookmark{r.Pos}
}

func (b nodeBookmark) Resolve(doc *Node) Selection {
	r := doc.Resolve(b.anchor)
	if node := r.NodeAfter(); node != nil && isNodeSelectable(node) {
		return NewNodeSelection(r)
	}
	return SelectionNear(r, 1)
}

// AllSelection selects the whole document.
type AllSelection struct{ selBase }

// NewAllSelection selects doc.
func NewAllSelection(doc *Node) *AllSelection {
	return &AllSelection{newBase(doc.Resolve(0), doc.Resolve(doc.Content.Size), nil)}
}

func (s *AllSelection) Eq(o Selection) bool {
	_, ok := o.(*AllSelection)
	return ok
}

func (s *AllSelection) Map(doc *Node, _ Mappable) Selection { return NewAllSelection(doc) }

func (s *AllSelection) Replace(tr *Transaction, content *Slice) {
	if content == nil {
		content = EmptySlice
	}
	if content == EmptySlice || content.Size() == 0 {
		tr.Delete(0, tr.Doc.Content.Size)
		if sel := SelectionAtStart(tr.Doc); sel != nil && !sel.Eq(tr.Selection()) {
			tr.SetSelection(sel)
		}
		return
	}
	s.selBase.Replace(tr, content)
}

func (s *AllSelection) Bookmark() Bookmark { return allBookmark{} }

type allBookmark struct{}

func (allBookmark) Map(Mappable) Bookmark       { return allBookmark{} }
func (allBookmark) Resolve(doc *Node) Selection { return NewAllSelection(doc) }

// SelectionFindFrom is the first valid selection from pos going dir, a text
// one with textOnly; nil for none.
func SelectionFindFrom(pos *ResolvedPos, dir int, textOnly bool) Selection {
	var inner Selection
	if pos.Parent().InlineContent() {
		inner = NewTextSelection(pos, nil)
	} else {
		inner = findSelectionIn(pos.Node(0), pos.Parent(), pos.Pos, pos.Index(pos.Depth), dir, textOnly)
	}
	if inner != nil {
		return inner
	}
	for depth := pos.Depth - 1; depth >= 0; depth-- {
		var found Selection
		if dir < 0 {
			found = findSelectionIn(pos.Node(0), pos.Node(depth), pos.Before(depth+1), pos.Index(depth), dir, textOnly)
		} else {
			found = findSelectionIn(pos.Node(0), pos.Node(depth), pos.After(depth+1), pos.Index(depth)+1, dir, textOnly)
		}
		if found != nil {
			return found
		}
	}
	return nil
}

func findSelectionIn(doc, node *Node, pos, index, dir int, text bool) Selection {
	if node.InlineContent() {
		return NewTextSelection(doc.Resolve(pos), nil)
	}
	i := index
	if dir < 0 {
		i = index - 1
	}
	for ; (dir > 0 && i < node.ChildCount()) || (dir < 0 && i >= 0); i += dir {
		child := node.Child(i)
		if !child.IsAtom() {
			offset := 0
			if dir > 0 {
				offset = 1
			} else {
				offset = -1
			}
			start := 0
			if dir < 0 {
				start = child.ChildCount()
			}
			if inner := findSelectionIn(doc, child, pos+offset, start, dir, text); inner != nil {
				return inner
			}
		} else if !text && isNodeSelectable(child) {
			at := pos
			if dir < 0 {
				at = pos - child.NodeSize()
			}
			return NewNodeSelection(doc.Resolve(at))
		}
		pos += child.NodeSize() * dir
	}
	return nil
}

// SelectionNear is a valid selection near pos, bias the side looked at
// first.
func SelectionNear(pos *ResolvedPos, bias int) Selection {
	if s := SelectionFindFrom(pos, bias, false); s != nil {
		return s
	}
	if s := SelectionFindFrom(pos, -bias, false); s != nil {
		return s
	}
	return NewAllSelection(pos.Node(0))
}

// SelectionAtStart is the first selection in doc.
func SelectionAtStart(doc *Node) Selection {
	if s := findSelectionIn(doc, doc, 0, 0, 1, false); s != nil {
		return s
	}
	return NewAllSelection(doc)
}

// SelectionAtEnd is the last selection in doc.
func SelectionAtEnd(doc *Node) Selection {
	if s := findSelectionIn(doc, doc, doc.Content.Size, doc.ChildCount(), -1, false); s != nil {
		return s
	}
	return NewAllSelection(doc)
}
