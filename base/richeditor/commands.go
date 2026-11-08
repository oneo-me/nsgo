package richeditor

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Command is an edit over a state: it says whether it applies, and with a
// dispatch makes the change. With a nil dispatch it only checks.
type Command func(s *State, dispatch func(*Transaction)) bool

// ChainCommands is ProseMirror's chainCommands: the first command that
// applies runs.
func ChainCommands(cmds ...Command) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		for _, c := range cmds {
			if c(s, dispatch) {
				return true
			}
		}
		return false
	}
}

// Chain runs commands one after another, each on the state the one before
// left, and dispatches their changes as one transaction, as Tiptap's chain
// does: it says whether every one applied. Without a dispatch each checks
// on the state the ones before would leave.
func Chain(cmds ...Command) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		tr, cur, all := s.Tr(), s, true
		for _, c := range cmds {
			var got *Transaction
			if !c(cur, func(t *Transaction) { got = t }) {
				all = false
				continue
			}
			if got == nil || got.Err != nil {
				continue
			}
			next, err := cur.applyInner(got)
			if err != nil {
				all = false
				continue
			}
			for _, step := range got.Steps {
				tr.Step(step)
			}
			if got.SelectionSet() {
				tr.SetSelection(got.Selection().Bookmark().Resolve(tr.Doc))
			}
			if got.marksAt == len(got.Steps) && got.marks != nil {
				tr.SetStoredMarks(got.marks)
			}
			for k, v := range got.meta {
				tr.SetMeta(k, v)
			}
			if got.ScrolledIntoView() {
				tr.ScrollIntoView()
			}
			tr.Origin = got.Origin
			cur = next
		}
		if dispatch != nil && (tr.DocChanged() || tr.SelectionSet() || tr.StoredMarks() != nil) && tr.Err == nil {
			dispatch(tr)
		}
		return all
	}
}

// DeleteSelection deletes a selection that holds something.
func DeleteSelection(s *State, dispatch func(*Transaction)) bool {
	if s.Selection.Empty() {
		return false
	}
	if dispatch != nil {
		dispatch(s.Tr().DeleteSelection().ScrollIntoView())
	}
	return true
}

func cursorAtStart(s *State) *ResolvedPos {
	t, ok := s.Selection.(*TextSelection)
	if !ok || t.Cursor() == nil || t.Cursor().ParentOffset > 0 {
		return nil
	}
	return t.Cursor()
}

func cursorAtEnd(s *State) *ResolvedPos {
	t, ok := s.Selection.(*TextSelection)
	if !ok || t.Cursor() == nil || t.Cursor().ParentOffset < t.Cursor().Parent().Content.Size {
		return nil
	}
	return t.Cursor()
}

func findCutBefore(r *ResolvedPos) *ResolvedPos {
	if !r.Parent().Type.Spec.Isolating {
		for i := r.Depth - 1; i >= 0; i-- {
			if r.Index(i) > 0 {
				return r.Doc().Resolve(r.Before(i + 1))
			}
			if r.Node(i).Type.Spec.Isolating {
				break
			}
		}
	}
	return nil
}

func findCutAfter(r *ResolvedPos) *ResolvedPos {
	if !r.Parent().Type.Spec.Isolating {
		for i := r.Depth - 1; i >= 0; i-- {
			parent := r.Node(i)
			if r.Index(i)+1 < parent.ChildCount() {
				return r.Doc().Resolve(r.After(i + 1))
			}
			if parent.Type.Spec.Isolating {
				break
			}
		}
	}
	return nil
}

func textblockAt(node *Node, last, only bool) bool {
	for node != nil {
		if node.IsTextblock() {
			return true
		}
		if only && node.ChildCount() != 1 {
			return false
		}
		if last {
			node = node.LastChild()
		} else {
			node = node.FirstChild()
		}
	}
	return false
}

func joinMaybeClear(s *State, r *ResolvedPos, dispatch func(*Transaction)) bool {
	before, after := r.NodeBefore(), r.NodeAfter()
	index := r.Index(r.Depth)
	if before == nil || after == nil || !before.Type.compatibleContent(after.Type) {
		return false
	}
	if before.Content.Size == 0 && r.Parent().CanReplace(index-1, index, nil, 0, -1) {
		if dispatch != nil {
			dispatch(s.Tr().Delete(r.Pos-before.NodeSize(), r.Pos).ScrollIntoView())
		}
		return true
	}
	if !r.Parent().CanReplace(index, index+1, nil, 0, -1) || !(after.IsTextblock() || CanJoin(s.Doc, r.Pos)) {
		return false
	}
	if dispatch != nil {
		dispatch(s.Tr().Join(r.Pos, 1).ScrollIntoView())
	}
	return true
}

// JoinBackward joins the textblock at a cursor's start with what is before.
func JoinBackward(s *State, dispatch func(*Transaction)) bool {
	cursor := cursorAtStart(s)
	if cursor == nil {
		return false
	}
	cut := findCutBefore(cursor)
	if cut == nil {
		rng := cursor.BlockRange(nil, nil)
		if rng != nil {
			if target := LiftTarget(rng); target >= 0 {
				if dispatch != nil {
					dispatch(s.Tr().Lift(rng, target).ScrollIntoView())
				}
				return true
			}
		}
		return false
	}
	before := cut.NodeBefore()
	if barrier(s, cut, dispatch, -1) {
		return true
	}
	if cursor.Parent().Content.Size == 0 && (textblockAt(before, true, false) || isNodeSelectable(before)) {
		for depth := cursor.Depth; ; depth-- {
			delStep := ReplaceStepFit(s.Doc, cursor.Before(depth), cursor.After(depth), EmptySlice)
			if rs, ok := delStep.(*ReplaceStep); ok && rs.Slice.Size() < rs.To-rs.From {
				if dispatch != nil {
					tr := s.Tr().Step(delStep)
					if textblockAt(before, true, false) {
						tr.SetSelection(SelectionFindFromOrNear(tr.Doc.Resolve(tr.Mapping.Map(cut.Pos, -1)), -1))
					} else {
						tr.SetSelection(NewNodeSelection(tr.Doc.Resolve(cut.Pos - before.NodeSize())))
					}
					dispatch(tr.ScrollIntoView())
				}
				return true
			}
			if depth == 1 || cursor.Node(depth-1).ChildCount() > 1 {
				break
			}
		}
	}
	if before.IsAtom() && cut.Depth == cursor.Depth-1 {
		if dispatch != nil {
			dispatch(s.Tr().Delete(cut.Pos-before.NodeSize(), cut.Pos).ScrollIntoView())
		}
		return true
	}
	return false
}

// SelectionFindFromOrNear is the selection from pos going dir, else near.
func SelectionFindFromOrNear(pos *ResolvedPos, dir int) Selection {
	if s := SelectionFindFrom(pos, dir, false); s != nil {
		return s
	}
	return SelectionNear(pos, dir)
}

// barrier is prosemirror-commands' deleteBarrier.
func barrier(s *State, cut *ResolvedPos, dispatch func(*Transaction), dir int) bool {
	before, after := cut.NodeBefore(), cut.NodeAfter()
	isolated := before.Type.Spec.Isolating || after.Type.Spec.Isolating
	if !isolated && joinMaybeClear(s, cut, dispatch) {
		return true
	}
	index := cut.Index(cut.Depth)
	canDelAfter := !isolated && cut.Parent().CanReplace(index, index+1, nil, 0, -1)
	if canDelAfter {
		match := before.contentMatchAt(before.ChildCount())
		if conn := match.FindWrapping(after.Type); conn != nil {
			first := after.Type
			if len(conn) > 0 {
				first = conn[0]
			}
			if m := match.MatchType(first); m != nil && m.ValidEnd {
				if dispatch != nil {
					end := cut.Pos + after.NodeSize()
					wrap := emptyFragment
					for i := len(conn) - 1; i >= 0; i-- {
						wrap = FragmentFrom(conn[i].Create(nil, wrap, nil))
					}
					wrap = FragmentFrom(before.Copy(wrap))
					tr := s.Tr().Step(&ReplaceAroundStep{From: cut.Pos - 1, To: end, GapFrom: cut.Pos, GapTo: end, Slice: NewSlice(wrap, 1, 0), Insert: len(conn), Structure: true})
					joinAt := tr.Doc.Resolve(end + 2*len(conn))
					if a := joinAt.NodeAfter(); a != nil && a.Type == before.Type && CanJoin(tr.Doc, joinAt.Pos) {
						tr.Join(joinAt.Pos, 1)
					}
					dispatch(tr.ScrollIntoView())
				}
				return true
			}
		}
	}
	var selAfter Selection
	if !after.Type.Spec.Isolating && !(dir > 0 && isolated) {
		selAfter = SelectionFindFrom(cut, 1, false)
	}
	if selAfter != nil {
		if rng := selAfter.FromPos().BlockRange(selAfter.ToPos(), nil); rng != nil {
			if target := LiftTarget(rng); target >= 0 && target >= cut.Depth {
				if dispatch != nil {
					dispatch(s.Tr().Lift(rng, target).ScrollIntoView())
				}
				return true
			}
		}
	}
	if canDelAfter && textblockAt(after, false, true) && textblockAt(before, true, false) {
		at := before
		var wrap []*Node
		for {
			wrap = append(wrap, at)
			if at.IsTextblock() {
				break
			}
			at = at.LastChild()
		}
		afterText, afterDepth := after, 1
		for ; !afterText.IsTextblock(); afterText = afterText.FirstChild() {
			afterDepth++
		}
		if at.CanReplace(at.ChildCount(), at.ChildCount(), afterText.Content, 0, -1) {
			if dispatch != nil {
				end := emptyFragment
				for i := len(wrap) - 1; i >= 0; i-- {
					end = FragmentFrom(wrap[i].Copy(end))
				}
				dispatch(s.Tr().Step(&ReplaceAroundStep{From: cut.Pos - len(wrap), To: cut.Pos + after.NodeSize(), GapFrom: cut.Pos + afterDepth, GapTo: cut.Pos + after.NodeSize() - afterDepth,
					Slice: NewSlice(end, len(wrap), 0), Structure: true}).ScrollIntoView())
			}
			return true
		}
	}
	return false
}

// JoinForward joins the textblock at a cursor's end with what is after.
func JoinForward(s *State, dispatch func(*Transaction)) bool {
	cursor := cursorAtEnd(s)
	if cursor == nil {
		return false
	}
	cut := findCutAfter(cursor)
	if cut == nil {
		return false
	}
	after := cut.NodeAfter()
	if barrier(s, cut, dispatch, 1) {
		return true
	}
	if cursor.Parent().Content.Size == 0 && (textblockAt(after, false, false) || isNodeSelectable(after)) {
		for depth := cursor.Depth; ; depth-- {
			delStep := ReplaceStepFit(s.Doc, cursor.Before(depth), cursor.After(depth), EmptySlice)
			if rs, ok := delStep.(*ReplaceStep); ok && rs.Slice.Size() < rs.To-rs.From {
				if dispatch != nil {
					tr := s.Tr().Step(delStep)
					if textblockAt(after, false, false) {
						tr.SetSelection(SelectionFindFromOrNear(tr.Doc.Resolve(tr.Mapping.Map(cut.Pos, 1)), 1))
					} else {
						tr.SetSelection(NewNodeSelection(tr.Doc.Resolve(tr.Mapping.Map(cut.Pos, 1))))
					}
					dispatch(tr.ScrollIntoView())
				}
				return true
			}
			if depth == 1 || cursor.Node(depth-1).ChildCount() > 1 {
				break
			}
		}
	}
	if after.IsAtom() && cut.Depth == cursor.Depth-1 {
		if dispatch != nil {
			dispatch(s.Tr().Delete(cut.Pos, cut.Pos+after.NodeSize()).ScrollIntoView())
		}
		return true
	}
	return false
}

// SelectNodeBackward selects the node before a cursor at a textblock's
// start.
func SelectNodeBackward(s *State, dispatch func(*Transaction)) bool {
	cursor := cursorAtStart(s)
	if cursor == nil {
		return false
	}
	cut := findCutBefore(cursor)
	if cut == nil {
		return false
	}
	node := cut.NodeBefore()
	if !isNodeSelectable(node) {
		return false
	}
	if dispatch != nil {
		dispatch(s.Tr().SetSelection(NewNodeSelection(s.Doc.Resolve(cut.Pos - node.NodeSize()))).ScrollIntoView())
	}
	return true
}

// SelectNodeForward selects the node after a cursor at a textblock's end.
func SelectNodeForward(s *State, dispatch func(*Transaction)) bool {
	cursor := cursorAtEnd(s)
	if cursor == nil {
		return false
	}
	cut := findCutAfter(cursor)
	if cut == nil {
		return false
	}
	node := cut.NodeAfter()
	if !isNodeSelectable(node) {
		return false
	}
	if dispatch != nil {
		dispatch(s.Tr().SetSelection(NewNodeSelection(cut)).ScrollIntoView())
	}
	return true
}

// LiftEmptyBlock lifts an empty block out of its parent.
func LiftEmptyBlock(s *State, dispatch func(*Transaction)) bool {
	t, ok := s.Selection.(*TextSelection)
	if !ok || t.Cursor() == nil || t.Cursor().Parent().Content.Size > 0 {
		return false
	}
	cursor := t.Cursor()
	if cursor.Depth > 1 && cursor.After(cursor.Depth) != cursor.End(-1) {
		before := cursor.Before(cursor.Depth)
		if CanSplit(s.Doc, before, 1, nil) {
			if dispatch != nil {
				dispatch(s.Tr().Split(before, 1, nil).ScrollIntoView())
			}
			return true
		}
	}
	rng := cursor.BlockRange(nil, nil)
	if rng == nil {
		return false
	}
	target := LiftTarget(rng)
	if target < 0 {
		return false
	}
	if dispatch != nil {
		dispatch(s.Tr().Lift(rng, target).ScrollIntoView())
	}
	return true
}

// NewlineInCode inserts a line break in a code block.
func NewlineInCode(s *State, dispatch func(*Transaction)) bool {
	from, to := s.Selection.FromPos(), s.Selection.ToPos()
	if !from.Parent().Type.Spec.Code || !from.SameParent(to) {
		return false
	}
	if dispatch != nil {
		dispatch(s.Tr().InsertText("\n", -1, -1).ScrollIntoView())
	}
	return true
}

func defaultBlockAt(m *ContentMatch) *NodeType {
	for i := 0; i < m.EdgeCount(); i++ {
		if t, _ := m.Edge(i); t.IsTextblock() && !t.hasRequiredAttrs() {
			return t
		}
	}
	return nil
}

// CreateParagraphNear makes a paragraph beside a selected block.
func CreateParagraphNear(s *State, dispatch func(*Transaction)) bool {
	sel := s.Selection
	from, to := sel.FromPos(), sel.ToPos()
	_, isAll := sel.(*AllSelection)
	if isAll || from.Parent().InlineContent() || to.Parent().InlineContent() {
		return false
	}
	t := defaultBlockAt(to.Parent().contentMatchAt(to.IndexAfter(to.Depth)))
	if t == nil || !to.Parent().CanReplaceWith(to.IndexAfter(to.Depth), to.IndexAfter(to.Depth), t, nil) {
		return false
	}
	if dispatch != nil {
		side := to.Pos
		if from.ParentOffset == 0 && to.Index(to.Depth) < to.Parent().ChildCount() {
			side = from.Pos
		} else if from.ParentOffset != 0 || to.Index(to.Depth) >= to.Parent().ChildCount() {
			side = to.Pos
		}
		tr := s.Tr().Insert(side, t.CreateAndFill(nil, nil, nil))
		tr.SetSelection(NewTextSelection(tr.Doc.Resolve(side+1), nil))
		dispatch(tr.ScrollIntoView())
	}
	return true
}

// SplitBlock splits the textblock at the selection.
func SplitBlock(s *State, dispatch func(*Transaction)) bool {
	sel := s.Selection
	from := sel.FromPos()
	if ns, ok := sel.(*NodeSelection); ok && ns.Node.IsBlock() {
		if from.ParentOffset == 0 || !CanSplit(s.Doc, from.Pos, 1, nil) {
			return false
		}
		if dispatch != nil {
			dispatch(s.Tr().Split(from.Pos, 1, nil).ScrollIntoView())
		}
		return true
	}
	if from.Depth == 0 {
		return false
	}
	var types []NodeTypeAttrs
	var splitDepth int
	var deflt *NodeType
	atEnd, atStart := false, false
	for d := from.Depth; ; d-- {
		node := from.Node(d)
		if node.IsBlock() {
			atEnd = from.End(d) == from.Pos+(from.Depth-d)
			atStart = from.Start(d) == from.Pos-(from.Depth-d)
			deflt = defaultBlockAt(from.Node(d - 1).contentMatchAt(from.IndexAfter(d - 1)))
			var t NodeTypeAttrs
			if atEnd && deflt != nil {
				t = NodeTypeAttrs{Type: deflt}
			}
			types = append([]NodeTypeAttrs{t}, types...)
			splitDepth = d
			break
		}
		if d == 1 {
			return false
		}
		types = append([]NodeTypeAttrs{{}}, types...)
	}
	tr := s.Tr()
	if _, ok := sel.(*TextSelection); ok || isAllSel(sel) {
		tr.DeleteSelection()
	}
	splitPos := tr.Mapping.Map(from.Pos, 1)
	can := CanSplit(tr.Doc, splitPos, len(types), types)
	if !can {
		types[0] = NodeTypeAttrs{}
		if deflt != nil {
			types[0] = NodeTypeAttrs{Type: deflt}
		}
		can = CanSplit(tr.Doc, splitPos, len(types), types)
	}
	if !can {
		return false
	}
	tr.Split(splitPos, len(types), types)
	if !atEnd && atStart && from.Node(splitDepth).Type != deflt && deflt != nil {
		first := tr.Mapping.Map(from.Before(splitDepth), 1)
		r := tr.Doc.Resolve(first)
		if from.Node(splitDepth-1).CanReplaceWith(r.Index(r.Depth), r.Index(r.Depth)+1, deflt, nil) {
			tr.SetNodeMarkup(tr.Mapping.Map(from.Before(splitDepth), 1), deflt, nil, nil)
		}
	}
	if dispatch != nil {
		dispatch(tr.ScrollIntoView())
	}
	return true
}

func isAllSel(s Selection) bool {
	_, ok := s.(*AllSelection)
	return ok
}

// SelectAll selects the document.
func SelectAll(s *State, dispatch func(*Transaction)) bool {
	if dispatch != nil {
		dispatch(s.Tr().SetSelection(NewAllSelection(s.Doc)))
	}
	return true
}

// SelectParentNode selects the block around the selection.
func SelectParentNode(s *State, dispatch func(*Transaction)) bool {
	from, to := s.Selection.FromPos(), s.Selection.ToPos()
	same := from.SharedDepth(to.Pos)
	if same == 0 {
		return false
	}
	pos := from.Before(same)
	if dispatch != nil {
		dispatch(s.Tr().SetSelection(NodeSelectionAt(s.Doc, pos)))
	}
	return true
}

// ToggleMark toggles a mark over the selection, or in the stored marks at a
// cursor.
func ToggleMark(t *MarkType, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		sel := s.Selection
		var cursor *ResolvedPos
		if ts, ok := sel.(*TextSelection); ok {
			cursor = ts.Cursor()
		}
		if sel.Empty() && cursor == nil || !markApplies(s.Doc, sel.Ranges(), t) {
			return false
		}
		if dispatch == nil {
			return true
		}
		if cursor != nil {
			tr := s.Tr()
			if t.IsInSet(orMarksHere(s, cursor)) != nil {
				tr.RemoveStoredMark(t)
			} else {
				tr.AddStoredMark(t.Create(attrs))
			}
			dispatch(tr)
			return true
		}
		has := false
		tr := s.Tr()
		for _, r := range sel.Ranges() {
			if has {
				break
			}
			has = s.Doc.RangeHasMark(r.From.Pos, r.To.Pos, t)
		}
		for _, r := range sel.Ranges() {
			if has {
				tr.RemoveMark(r.From.Pos, r.To.Pos, nil, t)
				continue
			}
			from, to := r.From.Pos, r.To.Pos
			// White space around a selection stays out of the mark.
			if text := s.Doc.TextBetween(from, to, "", ""); text != "" {
				spaceStart := utf16Len(text) - utf16Len(strings.TrimLeftFunc(text, unicode.IsSpace))
				spaceEnd := utf16Len(text) - utf16Len(strings.TrimRightFunc(text, unicode.IsSpace))
				if from+spaceStart < to-spaceEnd {
					from += spaceStart
					to -= spaceEnd
				}
			}
			tr.AddMark(from, to, t.Create(attrs))
		}
		dispatch(tr.ScrollIntoView())
		return true
	}
}

func orMarksHere(s *State, cursor *ResolvedPos) []*Mark {
	if s.StoredMarks != nil {
		return s.StoredMarks
	}
	return cursor.MarksHere()
}

func markApplies(doc *Node, ranges []SelectionRange, t *MarkType) bool {
	for _, r := range ranges {
		can := r.From.Depth == 0 && doc.Type.AllowsMarkType(t)
		doc.NodesBetween(r.From.Pos, r.To.Pos, func(node *Node, _ int, _ *Node, _ int) bool {
			if can {
				return false
			}
			can = node.InlineContent() && node.Type.AllowsMarkType(t)
			return true
		}, 0)
		if can {
			return true
		}
	}
	return false
}

// SetBlockType sets the selected textblocks to t.
func SetBlockType(t *NodeType, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		applicable := false
		for _, r := range s.Selection.Ranges() {
			if applicable {
				break
			}
			s.Doc.NodesBetween(r.From.Pos, r.To.Pos, func(node *Node, pos int, _ *Node, _ int) bool {
				if applicable {
					return false
				}
				if !node.IsTextblock() || node.HasMarkup(t, t.computeAttrs(attrs), nil) {
					return true
				}
				if node.Type == t {
					applicable = true
				} else {
					at := s.Doc.Resolve(pos)
					index := at.Index(at.Depth)
					applicable = at.Parent().CanReplaceWith(index, index+1, t, nil)
				}
				return true
			}, 0)
		}
		if !applicable {
			return false
		}
		if dispatch != nil {
			tr := s.Tr()
			for _, r := range s.Selection.Ranges() {
				tr.SetBlockType(r.From.Pos, r.To.Pos, t, func(*Node) Attrs { return attrs })
			}
			dispatch(tr.ScrollIntoView())
		}
		return true
	}
}

// WrapIn wraps the selected blocks in t.
func WrapIn(t *NodeType, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		rng := s.Selection.FromPos().BlockRange(s.Selection.ToPos(), nil)
		if rng == nil {
			return false
		}
		wrapping := FindWrapping(rng, t, attrs, nil)
		if wrapping == nil {
			return false
		}
		if dispatch != nil {
			dispatch(s.Tr().Wrap(rng, wrapping).ScrollIntoView())
		}
		return true
	}
}

// Lift lifts the selected block out of its parent.
func Lift(s *State, dispatch func(*Transaction)) bool {
	rng := s.Selection.FromPos().BlockRange(s.Selection.ToPos(), nil)
	if rng == nil {
		return false
	}
	target := LiftTarget(rng)
	if target < 0 {
		return false
	}
	if dispatch != nil {
		dispatch(s.Tr().Lift(rng, target).ScrollIntoView())
	}
	return true
}

// ExitCode leaves a code block to the block after it.
func ExitCode(s *State, dispatch func(*Transaction)) bool {
	head, anchor := s.Selection.HeadPos(), s.Selection.AnchorPos()
	if !head.Parent().Type.Spec.Code || !head.SameParent(anchor) {
		return false
	}
	above := head.Node(-1)
	after := head.IndexAfter(-1)
	t := defaultBlockAt(above.contentMatchAt(after))
	if t == nil || !above.CanReplaceWith(after, after, t, nil) {
		return false
	}
	if dispatch != nil {
		pos := head.After(head.Depth)
		tr := s.Tr().ReplaceWith(pos, pos, t.CreateAndFill(nil, nil, nil))
		tr.SetSelection(SelectionNear(tr.Doc.Resolve(pos), 1))
		dispatch(tr.ScrollIntoView())
	}
	return true
}

// InsertHardBreak replaces the selection with a node of t.
func InsertHardBreak(t *NodeType) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		from := s.Selection.FromPos()
		if !from.Parent().InlineContent() || !from.Parent().Type.AllowsMarks(nil) {
			return false
		}
		index := from.Index(from.Depth)
		if !from.Parent().CanReplaceWith(index, index, t, nil) {
			return false
		}
		if dispatch != nil {
			dispatch(s.Tr().ReplaceSelectionWith(t.Create(nil, nil, nil), true).ScrollIntoView())
		}
		return true
	}
}

// WrapInList wraps the selected blocks in a list of listType.
func WrapInList(listType *NodeType, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		from, to := s.Selection.FromPos(), s.Selection.ToPos()
		rng := from.BlockRange(to, nil)
		if rng == nil {
			return false
		}
		doJoin := false
		outer := rng
		if rng.Depth >= 2 && from.Node(rng.Depth-1).Type.compatibleContent(listType) && rng.StartIndex() == 0 {
			if from.Index(rng.Depth-1) == 0 {
				return false
			}
			r := s.Doc.Resolve(rng.Start() - 2)
			outer = &NodeRange{From: r, To: r, Depth: rng.Depth}
			if rng.EndIndex() < rng.Parent().ChildCount() {
				rng = &NodeRange{From: from, To: s.Doc.Resolve(to.End(rng.Depth)), Depth: rng.Depth}
			}
			doJoin = true
		}
		wrap := FindWrapping(outer, listType, attrs, rng)
		if wrap == nil {
			return false
		}
		if dispatch != nil {
			dispatch(doWrapInList(s.Tr(), rng, wrap, doJoin, listType).ScrollIntoView())
		}
		return true
	}
}

func doWrapInList(tr *Transaction, rng *NodeRange, wrappers []NodeTypeAttrs, joinBefore bool, listType *NodeType) *Transaction {
	content := emptyFragment
	for i := len(wrappers) - 1; i >= 0; i-- {
		content = FragmentFrom(wrappers[i].Type.Create(wrappers[i].Attrs, content, nil))
	}
	start, end := rng.Start(), rng.End()
	jb := 2 * boolInt(joinBefore)
	tr.Step(&ReplaceAroundStep{From: start - jb, To: end, GapFrom: start, GapTo: end, Slice: NewSlice(content, 0, 0), Insert: len(wrappers), Structure: true})
	found := 0
	for i, w := range wrappers {
		if w.Type == listType {
			found = i + 1
		}
	}
	splitDepth := len(wrappers) - found
	splitPos := start + len(wrappers) - jb
	parent := rng.Parent()
	for i, e, first := rng.StartIndex(), rng.EndIndex(), true; i < e; i, first = i+1, false {
		if !first && CanSplit(tr.Doc, splitPos, splitDepth, nil) {
			tr.Split(splitPos, splitDepth, nil)
			splitPos += 2 * splitDepth
		}
		splitPos += parent.Child(i).NodeSize()
	}
	return tr
}

// SplitListItem splits the list item at the selection; an empty one leaves
// the list or lifts.
func SplitListItem(itemType *NodeType, itemAttrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		from, to := s.Selection.FromPos(), s.Selection.ToPos()
		if ns, ok := s.Selection.(*NodeSelection); ok && ns.Node.IsBlock() || from.Depth < 2 || !from.SameParent(to) {
			return false
		}
		grandParent := from.Node(-1)
		if grandParent.Type != itemType {
			return false
		}
		if from.Parent().Content.Size == 0 && from.Node(-1).ChildCount() == from.IndexAfter(-1) {
			if from.Depth == 3 || from.Node(-3).Type != itemType || from.Index(-2) != from.Node(-2).ChildCount()-1 {
				return false
			}
			if dispatch != nil {
				wrap := emptyFragment
				depthBefore := 3
				if from.Index(-1) > 0 {
					depthBefore = 1
				} else if from.Index(-2) > 0 {
					depthBefore = 2
				}
				for d := from.Depth - depthBefore; d >= from.Depth-3; d-- {
					wrap = FragmentFrom(from.Node(d).Copy(wrap))
				}
				depthAfter := 3
				if from.IndexAfter(-1) < from.Node(-2).ChildCount() {
					depthAfter = 1
				} else if from.IndexAfter(-2) < from.Node(-3).ChildCount() {
					depthAfter = 2
				}
				wrap = wrap.Append(FragmentFrom(itemType.CreateAndFill(nil, nil, nil)))
				start := from.Before(from.Depth - (depthBefore - 1))
				tr := s.Tr().Replace(start, from.After(-depthAfter), NewSlice(wrap, 4-depthBefore, 0))
				sel := -1
				tr.Doc.NodesBetween(start, tr.Doc.Content.Size, func(node *Node, pos int, _ *Node, _ int) bool {
					if sel > -1 {
						return false
					}
					if node.IsTextblock() && node.Content.Size == 0 {
						sel = pos + 1
					}
					return true
				}, 0)
				if sel > -1 {
					tr.SetSelection(SelectionNear(tr.Doc.Resolve(sel), 1))
				}
				dispatch(tr.ScrollIntoView())
			}
			return true
		}
		var nextType *NodeType
		if to.Pos == from.End(from.Depth) {
			nextType = grandParent.contentMatchAt(0).defaultType()
		}
		tr := s.Tr().Delete(from.Pos, to.Pos)
		var types []NodeTypeAttrs
		if nextType != nil {
			types = []NodeTypeAttrs{{}, {Type: nextType}}
			if itemAttrs != nil {
				types[0] = NodeTypeAttrs{itemType, itemAttrs}
			}
		}
		if !CanSplit(tr.Doc, from.Pos, 2, types) {
			return false
		}
		if dispatch != nil {
			dispatch(tr.Split(from.Pos, 2, types).ScrollIntoView())
		}
		return true
	}
}

// LiftListItem lifts the selected list items out of their list.
func LiftListItem(itemType *NodeType) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		from, to := s.Selection.FromPos(), s.Selection.ToPos()
		rng := from.BlockRange(to, func(n *Node) bool { return n.ChildCount() > 0 && n.FirstChild().Type == itemType })
		if rng == nil {
			return false
		}
		if dispatch == nil {
			return true
		}
		if from.Node(rng.Depth-1).Type == itemType {
			return liftToOuterList(s, dispatch, itemType, rng)
		}
		return liftOutOfList(s, dispatch, rng)
	}
}

func liftToOuterList(s *State, dispatch func(*Transaction), itemType *NodeType, rng *NodeRange) bool {
	tr := s.Tr()
	end := rng.End()
	endOfList := rng.To.End(rng.Depth)
	if end < endOfList {
		tr.Step(&ReplaceAroundStep{From: end - 1, To: endOfList, GapFrom: end, GapTo: endOfList,
			Slice: NewSlice(FragmentFrom(itemType.Create(nil, FragmentFrom(rng.Parent().Copy(nil)), nil)), 1, 0), Insert: 1, Structure: true})
		rng = &NodeRange{From: tr.Doc.Resolve(rng.From.Pos), To: tr.Doc.Resolve(endOfList), Depth: rng.Depth}
	}
	target := LiftTarget(rng)
	if target < 0 {
		return false
	}
	tr.Lift(rng, target)
	after := tr.Doc.Resolve(tr.Mapping.Map(end, -1) - 1)
	if CanJoin(tr.Doc, after.Pos) && after.NodeBefore() != nil && after.NodeBefore().Type == after.NodeAfter().Type {
		tr.Join(after.Pos, 1)
	}
	dispatch(tr.ScrollIntoView())
	return true
}

func liftOutOfList(s *State, dispatch func(*Transaction), rng *NodeRange) bool {
	tr := s.Tr()
	list := rng.Parent()
	for pos, i := rng.End(), rng.EndIndex()-1; i > rng.StartIndex(); i-- {
		pos -= list.Child(i).NodeSize()
		tr.Delete(pos-1, pos+1)
	}
	start := tr.Doc.Resolve(rng.Start())
	item := start.NodeAfter()
	if tr.Mapping.Map(rng.End(), 1) != rng.Start()+item.NodeSize() {
		return false
	}
	atStart := rng.StartIndex() == 0
	atEnd := rng.EndIndex() == list.ChildCount()
	parent := start.Node(-1)
	indexBefore := start.Index(-1)
	content := item.Content
	if !atEnd {
		content = content.Append(FragmentFrom(list))
	}
	if !parent.CanReplace(indexBefore+boolInt(!atStart), indexBefore+1, content, 0, -1) {
		return false
	}
	begin := start.Pos
	end := begin + item.NodeSize()
	before := emptyFragment
	if !atStart {
		before = FragmentFrom(list.Copy(emptyFragment))
	}
	after := emptyFragment
	if !atEnd {
		after = FragmentFrom(list.Copy(emptyFragment))
	}
	tr.Step(&ReplaceAroundStep{From: begin - boolInt(atStart), To: end + boolInt(atEnd), GapFrom: begin + 1, GapTo: end - 1,
		Slice: NewSlice(before.Append(after), boolInt(!atStart), boolInt(!atEnd)), Insert: boolInt(!atStart), Structure: true})
	dispatch(tr.ScrollIntoView())
	return true
}

// SinkListItem nests the selected list items in the item before them.
func SinkListItem(itemType *NodeType) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		from, to := s.Selection.FromPos(), s.Selection.ToPos()
		rng := from.BlockRange(to, func(n *Node) bool { return n.ChildCount() > 0 && n.FirstChild().Type == itemType })
		if rng == nil {
			return false
		}
		startIndex := rng.StartIndex()
		if startIndex == 0 {
			return false
		}
		parent := rng.Parent()
		nodeBefore := parent.Child(startIndex - 1)
		if nodeBefore.Type != itemType {
			return false
		}
		if dispatch != nil {
			nestedBefore := nodeBefore.LastChild() != nil && nodeBefore.LastChild().Type == parent.Type
			inner := emptyFragment
			open, back := 1, 1
			if nestedBefore {
				inner = FragmentFrom(itemType.Create(nil, nil, nil))
				open, back = 3, 3
			}
			slice := NewSlice(FragmentFrom(itemType.Create(nil, FragmentFrom(parent.Type.Create(nil, inner, nil)), nil)), open, 0)
			before, after := rng.Start(), rng.End()
			dispatch(s.Tr().Step(&ReplaceAroundStep{From: before - back, To: after, GapFrom: before, GapTo: after, Slice: slice, Insert: 1, Structure: true}).ScrollIntoView())
		}
		return true
	}
}

// DeleteCharBefore and the word commands of the view move by clusters; this
// is the first cluster's size before offset of text.
func clusterBefore(text string, offset int) int {
	if offset <= 0 {
		return 0
	}
	at := byteAt(text, offset)
	r, size := utf8.DecodeLastRuneInString(text[:at])
	if r == utf8.RuneError {
		return 1
	}
	n := utf16Len(text[at-size : at])
	// Joiners and marks stay with what they follow.
	for at-size > 0 {
		before, s := utf8.DecodeLastRuneInString(text[:at-size])
		if r != 0x200d && before != 0x200d && !unicode.Is(unicode.Mn, r) && !isVariation(r) {
			break
		}
		at -= size
		size, r = s, before
		n += utf16Len(text[at-size : at])
	}
	return n
}

func isVariation(r rune) bool {
	return r >= 0xfe00 && r <= 0xfe0f || r >= 0x1f3fb && r <= 0x1f3ff
}
