package richeditor

import (
	"errors"
	"fmt"
	"slices"
)

// ResolvedPos is a position with what it is in.
type ResolvedPos struct {
	Pos          int
	Depth        int
	ParentOffset int
	path         []resolvedStep
}

type resolvedStep struct {
	node   *Node
	index  int
	offset int
}

// Resolve resolves pos, which must be in the document.
func (n *Node) Resolve(pos int) *ResolvedPos {
	if pos < 0 || pos > n.Content.Size {
		panic(fmt.Sprintf("richeditor: position %d out of range", pos))
	}
	var path []resolvedStep
	start, parentOffset := 0, pos
	for node := n; ; {
		index, offset := node.Content.findIndex(parentOffset, false)
		rem := parentOffset - offset
		path = append(path, resolvedStep{node, index, start + offset})
		if rem == 0 {
			break
		}
		node = node.Child(index)
		if node.IsText() {
			break
		}
		parentOffset = rem - 1
		start += offset + 1
	}
	return &ResolvedPos{Pos: pos, Depth: len(path) - 1, ParentOffset: parentOffset, path: path}
}

func (r *ResolvedPos) resolveDepth(d int) int {
	if d < 0 {
		return r.Depth + d
	}
	return d
}

// Parent is the node it is directly in.
func (r *ResolvedPos) Parent() *Node { return r.Node(r.Depth) }

// Doc is the document.
func (r *ResolvedPos) Doc() *Node { return r.Node(0) }

// Node is its ancestor at depth d; a negative d counts from its own.
func (r *ResolvedPos) Node(d int) *Node { return r.path[r.resolveDepth(d)].node }

// Index is the child of its ancestor at depth d it points into.
func (r *ResolvedPos) Index(d int) int { return r.path[r.resolveDepth(d)].index }

// IndexAfter is the index after it at depth d.
func (r *ResolvedPos) IndexAfter(d int) int {
	d = r.resolveDepth(d)
	if d == r.Depth && r.TextOffset() == 0 {
		return r.Index(d)
	}
	return r.Index(d) + 1
}

// Start is where the content of its ancestor at depth d starts.
func (r *ResolvedPos) Start(d int) int {
	d = r.resolveDepth(d)
	if d == 0 {
		return 0
	}
	return r.path[d-1].offset + 1
}

// End is where it ends.
func (r *ResolvedPos) End(d int) int {
	d = r.resolveDepth(d)
	return r.Start(d) + r.Node(d).Content.Size
}

// Before is the position before its ancestor at depth d.
func (r *ResolvedPos) Before(d int) int {
	d = r.resolveDepth(d)
	if d == 0 {
		panic("richeditor: there is no position before the top-level node")
	}
	if d == r.Depth+1 {
		return r.Pos
	}
	return r.path[d-1].offset
}

// After is the position after it.
func (r *ResolvedPos) After(d int) int {
	d = r.resolveDepth(d)
	if d == 0 {
		panic("richeditor: there is no position after the top-level node")
	}
	if d == r.Depth+1 {
		return r.Pos
	}
	return r.path[d-1].offset + r.path[d].node.NodeSize()
}

// TextOffset is how far into a text node it is.
func (r *ResolvedPos) TextOffset() int { return r.Pos - r.path[len(r.path)-1].offset }

// NodeAfter is the node after it, a text's rest, nil for none.
func (r *ResolvedPos) NodeAfter() *Node {
	parent := r.Parent()
	index := r.Index(r.Depth)
	if index == parent.ChildCount() {
		return nil
	}
	child := parent.Child(index)
	if d := r.TextOffset(); d != 0 {
		return child.Cut(d, child.NodeSize())
	}
	return child
}

// NodeBefore is the node before it, a text's start, nil for none.
func (r *ResolvedPos) NodeBefore() *Node {
	index := r.Index(r.Depth)
	if d := r.TextOffset(); d != 0 {
		return r.Parent().Child(index).Cut(0, d)
	}
	if index == 0 {
		return nil
	}
	return r.Parent().Child(index - 1)
}

// PosAtIndex is the position of child index of its ancestor at depth d.
func (r *ResolvedPos) PosAtIndex(index, d int) int {
	d = r.resolveDepth(d)
	node := r.path[d].node
	pos := 0
	if d > 0 {
		pos = r.path[d-1].offset + 1
	}
	for i := range index {
		pos += node.Child(i).NodeSize()
	}
	return pos
}

// MarksHere are the marks typed text would take here.
func (r *ResolvedPos) MarksHere() []*Mark {
	parent := r.Parent()
	index := r.Index(r.Depth)
	if parent.Content.Size == 0 {
		return nil
	}
	if r.TextOffset() != 0 {
		return parent.Child(index).Marks
	}
	main, other := parent.MaybeChild(index-1), parent.MaybeChild(index)
	if main == nil {
		main, other = other, main
	}
	marks := main.Marks
	for _, m := range slices.Clone(marks) {
		if m.Type.Spec.NotInclusive && (other == nil || !m.IsInSet(other.Marks)) {
			marks = m.RemoveFromSet(marks)
		}
	}
	return marks
}

// MarksAcross are the marks after it that also go on to end.
func (r *ResolvedPos) MarksAcross(end *ResolvedPos) []*Mark {
	after := r.Parent().MaybeChild(r.Index(r.Depth))
	if after == nil || !after.IsInline() {
		return nil
	}
	marks := after.Marks
	next := end.Parent().MaybeChild(end.Index(end.Depth))
	for _, m := range slices.Clone(marks) {
		if m.Type.Spec.NotInclusive && (next == nil || !m.IsInSet(next.Marks)) {
			marks = m.RemoveFromSet(marks)
		}
	}
	return marks
}

// SharedDepth is the deepest ancestor holding pos too.
func (r *ResolvedPos) SharedDepth(pos int) int {
	for depth := r.Depth; depth > 0; depth-- {
		if r.Start(depth) <= pos && r.End(depth) >= pos {
			return depth
		}
	}
	return 0
}

// BlockRange is the flat range of blocks around from here to other, nil
// for none.
func (r *ResolvedPos) BlockRange(other *ResolvedPos, pred func(*Node) bool) *NodeRange {
	if other == nil {
		other = r
	}
	for d := r.Depth - boolInt(r.Parent().InlineContent() || r.Pos == other.Pos); d >= 0; d-- {
		if other.Pos <= r.End(d) && (pred == nil || pred(r.Node(d))) {
			return &NodeRange{From: r, To: other, Depth: d}
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SameParent says two positions are in one node.
func (r *ResolvedPos) SameParent(o *ResolvedPos) bool {
	return r.Pos-r.ParentOffset == o.Pos-o.ParentOffset
}

// NodeRange is a flat range of a node's children.
type NodeRange struct {
	From, To *ResolvedPos
	Depth    int
}

func (n *NodeRange) Start() int      { return n.From.Before(n.Depth + 1) }
func (n *NodeRange) End() int        { return n.To.After(n.Depth + 1) }
func (n *NodeRange) Parent() *Node   { return n.From.Node(n.Depth) }
func (n *NodeRange) StartIndex() int { return n.From.Index(n.Depth) }
func (n *NodeRange) EndIndex() int   { return n.To.IndexAfter(n.Depth) }

// replaceDoc is prosemirror-model's replace.
func replaceDoc(from, to *ResolvedPos, slice *Slice) (*Node, error) {
	if slice.OpenStart > from.Depth {
		return nil, errors.New("inserted content deeper than insertion position")
	}
	if from.Depth-slice.OpenStart != to.Depth-slice.OpenEnd {
		return nil, errors.New("inconsistent open depths")
	}
	return replaceOuter(from, to, slice, 0)
}

func replaceOuter(from, to *ResolvedPos, slice *Slice, depth int) (*Node, error) {
	index := from.Index(depth)
	node := from.Node(depth)
	if index == to.Index(depth) && depth < from.Depth-slice.OpenStart {
		inner, err := replaceOuter(from, to, slice, depth+1)
		if err != nil {
			return nil, err
		}
		return node.Copy(node.Content.ReplaceChild(index, inner)), nil
	}
	if slice.Content.Size == 0 {
		content, err := replaceTwoWay(from, to, depth)
		if err != nil {
			return nil, err
		}
		return closeNode(node, content)
	}
	if slice.OpenStart == 0 && slice.OpenEnd == 0 && from.Depth == depth && to.Depth == depth {
		parent := from.Parent()
		content := parent.Content
		return closeNode(parent, content.Cut(0, from.ParentOffset).Append(slice.Content).Append(content.Cut(to.ParentOffset, -1)))
	}
	start, end := prepareSliceForReplace(slice, from)
	content, err := replaceThreeWay(from, start, end, to, depth)
	if err != nil {
		return nil, err
	}
	return closeNode(node, content)
}

func checkJoin(main, sub *Node) error {
	if !sub.Type.compatibleContent(main.Type) {
		return fmt.Errorf("cannot join %s onto %s", sub.Type.Name, main.Type.Name)
	}
	return nil
}

func joinable(before, after *ResolvedPos, depth int) (*Node, error) {
	node := before.Node(depth)
	if err := checkJoin(node, after.Node(depth)); err != nil {
		return nil, err
	}
	return node, nil
}

func addNode(child *Node, target []*Node) []*Node {
	if last := len(target) - 1; last >= 0 && child.IsText() && child.SameMarkup(target[last]) {
		target[last] = child.WithText(target[last].Text + child.Text)
		return target
	}
	return append(target, child)
}

func addRange(start, end *ResolvedPos, depth int, target []*Node) []*Node {
	node := end
	if node == nil {
		node = start
	}
	n := node.Node(depth)
	startIndex, endIndex := 0, 0
	if end != nil {
		endIndex = end.Index(depth)
	} else {
		endIndex = n.ChildCount()
	}
	if start != nil {
		startIndex = start.Index(depth)
		if start.Depth > depth {
			startIndex++
		} else if start.TextOffset() != 0 {
			target = addNode(start.NodeAfter(), target)
			startIndex++
		}
	}
	for i := startIndex; i < endIndex; i++ {
		target = addNode(n.Child(i), target)
	}
	if end != nil && end.Depth == depth && end.TextOffset() != 0 {
		target = addNode(end.NodeBefore(), target)
	}
	return target
}

func closeNode(node *Node, content *Fragment) (*Node, error) {
	if !node.Type.ValidContent(content) {
		return nil, fmt.Errorf("invalid content for node %s: %s", node.Type.Name, content)
	}
	return node.Copy(content), nil
}

func replaceThreeWay(from, start, end, to *ResolvedPos, depth int) (*Fragment, error) {
	var openStart, openEnd *Node
	var err error
	if from.Depth > depth {
		if openStart, err = joinable(from, start, depth+1); err != nil {
			return nil, err
		}
	}
	if to.Depth > depth {
		if openEnd, err = joinable(end, to, depth+1); err != nil {
			return nil, err
		}
	}
	var content []*Node
	content = addRange(nil, from, depth, content)
	if openStart != nil && openEnd != nil && start.Index(depth) == end.Index(depth) {
		if err := checkJoin(openStart, openEnd); err != nil {
			return nil, err
		}
		inner, err := replaceThreeWay(from, start, end, to, depth+1)
		if err != nil {
			return nil, err
		}
		closed, err := closeNode(openStart, inner)
		if err != nil {
			return nil, err
		}
		content = addNode(closed, content)
	} else {
		if openStart != nil {
			inner, err := replaceTwoWay(from, start, depth+1)
			if err != nil {
				return nil, err
			}
			closed, err := closeNode(openStart, inner)
			if err != nil {
				return nil, err
			}
			content = addNode(closed, content)
		}
		content = addRange(start, end, depth, content)
		if openEnd != nil {
			inner, err := replaceTwoWay(end, to, depth+1)
			if err != nil {
				return nil, err
			}
			closed, err := closeNode(openEnd, inner)
			if err != nil {
				return nil, err
			}
			content = addNode(closed, content)
		}
	}
	content = addRange(to, nil, depth, content)
	return FragmentFrom(content...), nil
}

func replaceTwoWay(from, to *ResolvedPos, depth int) (*Fragment, error) {
	var content []*Node
	content = addRange(nil, from, depth, content)
	if from.Depth > depth {
		t, err := joinable(from, to, depth+1)
		if err != nil {
			return nil, err
		}
		inner, err := replaceTwoWay(from, to, depth+1)
		if err != nil {
			return nil, err
		}
		closed, err := closeNode(t, inner)
		if err != nil {
			return nil, err
		}
		content = addNode(closed, content)
	}
	content = addRange(to, nil, depth, content)
	return FragmentFrom(content...), nil
}

func prepareSliceForReplace(slice *Slice, along *ResolvedPos) (*ResolvedPos, *ResolvedPos) {
	extra := along.Depth - slice.OpenStart
	parent := along.Node(extra)
	node := parent.Copy(slice.Content)
	for i := extra - 1; i >= 0; i-- {
		node = along.Node(i).Copy(FragmentFrom(node))
	}
	return node.Resolve(slice.OpenStart + extra), node.Resolve(node.Content.Size - slice.OpenEnd - extra)
}
