package richeditor

import (
	"errors"
	"slices"
)

// Transform collects steps, the documents between them and their mapping.
// A step that fails leaves it unchanged and sets Err, after which it takes
// no more.
type Transform struct {
	Doc     *Node
	Steps   []Step
	Docs    []*Node
	Mapping *Mapping
	Err     error
}

// NewTransform starts a transform of doc.
func NewTransform(doc *Node) *Transform { return &Transform{Doc: doc, Mapping: NewMapping()} }

// Before is the document before the steps.
func (t *Transform) Before() *Node {
	if len(t.Docs) > 0 {
		return t.Docs[0]
	}
	return t.Doc
}

// DocChanged says a step ran.
func (t *Transform) DocChanged() bool { return len(t.Steps) > 0 }

// Step applies s.
func (t *Transform) Step(s Step) *Transform {
	if t.Err != nil {
		return t
	}
	doc, err := s.Apply(t.Doc)
	if err != nil {
		t.Err = err
		return t
	}
	t.addStep(s, doc)
	return t
}

func (t *Transform) addStep(s Step, doc *Node) {
	t.Docs = append(t.Docs, t.Doc)
	t.Steps = append(t.Steps, s)
	t.Mapping.AppendMap(s.StepMap(), -1)
	t.Doc = doc
}

// Replace fits slice into from to to.
func (t *Transform) Replace(from, to int, slice *Slice) *Transform {
	if t.Err != nil {
		return t
	}
	if step := ReplaceStepFit(t.Doc, from, to, slice); step != nil {
		t.Step(step)
	}
	return t
}

// ReplaceWith replaces from to to with content.
func (t *Transform) ReplaceWith(from, to int, content ...*Node) *Transform {
	return t.Replace(from, to, NewSlice(FragmentFrom(content...), 0, 0))
}

// Delete deletes from to to.
func (t *Transform) Delete(from, to int) *Transform { return t.Replace(from, to, EmptySlice) }

// Insert inserts content at pos.
func (t *Transform) Insert(pos int, content ...*Node) *Transform {
	return t.ReplaceWith(pos, pos, content...)
}

func coveredDepths(from, to *ResolvedPos) []int {
	var result []int
	for d := min(from.Depth, to.Depth); d >= 0; d-- {
		start := from.Start(d)
		if start < from.Pos-(from.Depth-d) || to.End(d) > to.Pos+(to.Depth-d) ||
			from.Node(d).Type.Spec.Isolating || to.Node(d).Type.Spec.Isolating {
			break
		}
		if start == to.Start(d) || d == from.Depth && d == to.Depth && from.Parent().InlineContent() && to.Parent().InlineContent() && d > 0 && to.Start(d-1) == start-1 {
			result = append(result, d)
		}
	}
	return result
}

// DeleteRange deletes from to to, growing it over nodes it covers whole.
func (t *Transform) DeleteRange(from, to int) *Transform {
	if t.Err != nil {
		return t
	}
	f, r := t.Doc.Resolve(from), t.Doc.Resolve(to)
	if f.Parent().IsTextblock() && r.Parent().IsTextblock() && f.Start(f.Depth) != r.Start(r.Depth) && f.ParentOffset == 0 && r.ParentOffset == 0 {
		shared := f.SharedDepth(to)
		isolated := false
		for d := shared + 1; d <= f.Depth; d++ {
			isolated = isolated || f.Node(d).Type.Spec.Isolating
		}
		for d := shared + 1; d <= r.Depth; d++ {
			isolated = isolated || r.Node(d).Type.Spec.Isolating
		}
		if !isolated {
			for d := f.Depth; d > 0 && from == f.Start(d); d-- {
				from = f.Before(d)
			}
			for d := r.Depth; d > 0 && to == r.Start(d); d-- {
				to = r.Before(d)
			}
			f, r = t.Doc.Resolve(from), t.Doc.Resolve(to)
		}
	}
	covered := coveredDepths(f, r)
	for i, depth := range covered {
		last := i == len(covered)-1
		if last && depth == 0 || f.Node(depth).Type.ContentMatch.ValidEnd {
			return t.Delete(f.Start(depth), r.End(depth))
		}
		if depth > 0 && (last || f.Node(depth-1).CanReplace(f.Index(depth-1), r.IndexAfter(depth-1), emptyFragment, 0, -1)) {
			return t.Delete(f.Before(depth), r.After(depth))
		}
	}
	for d := 1; d <= f.Depth && d <= r.Depth; d++ {
		if from-f.Start(d) == f.Depth-d && to > f.End(d) && r.End(d)-to != r.Depth-d && f.Start(d-1) == r.Start(d-1) &&
			f.Node(d-1).CanReplace(f.Index(d-1), r.Index(d-1), emptyFragment, 0, -1) {
			return t.Delete(f.Before(d), to)
		}
	}
	return t.Delete(from, to)
}

// ReplaceRange replaces from to to with slice, moving the range up and the
// slice open as what is selected and pasted suggest.
func (t *Transform) ReplaceRange(from, to int, slice *Slice) *Transform {
	if t.Err != nil {
		return t
	}
	if slice.Size() == 0 {
		return t.DeleteRange(from, to)
	}
	f, r := t.Doc.Resolve(from), t.Doc.Resolve(to)
	if fitsTrivially(f, r, slice) {
		return t.Step(&ReplaceStep{From: from, To: to, Slice: slice})
	}
	targetDepths := coveredDepths(f, r)
	if len(targetDepths) > 0 && targetDepths[len(targetDepths)-1] == 0 {
		targetDepths = targetDepths[:len(targetDepths)-1]
	}
	preferredTarget := -(f.Depth + 1)
	targetDepths = append([]int{preferredTarget}, targetDepths...)
	pos := f.Pos - 1
	for d := f.Depth; d > 0; d-- {
		spec := f.Node(d).Type.Spec
		if spec.Defining || spec.Isolating {
			break
		}
		if slices.Contains(targetDepths, d) {
			preferredTarget = d
		} else if f.Before(d) == pos {
			targetDepths = slices.Insert(targetDepths, 1, -d)
		}
		pos--
	}
	preferredIndex := max(slices.Index(targetDepths, preferredTarget), 0)
	var leftNodes []*Node
	preferredDepth := slice.OpenStart
	for content, i := slice.Content, 0; ; i++ {
		node := content.FirstChild()
		if node == nil {
			break
		}
		leftNodes = append(leftNodes, node)
		if i == slice.OpenStart {
			break
		}
		content = node.Content
	}
	for d := preferredDepth - 1; d >= 0 && d < len(leftNodes); d-- {
		left := leftNodes[d]
		def := left.Type.Spec.Defining
		abs := preferredTarget
		if abs < 0 {
			abs = -abs
		}
		if def && !left.SameMarkup(f.Node(abs-1)) {
			preferredDepth = d
		} else if def || !left.Type.IsTextblock() {
			break
		}
	}
	for j := slice.OpenStart; j >= 0; j-- {
		openDepth := (j + preferredDepth + 1) % (slice.OpenStart + 1)
		if openDepth >= len(leftNodes) {
			continue
		}
		insert := leftNodes[openDepth]
		for i := range targetDepths {
			target := targetDepths[(i+preferredIndex)%len(targetDepths)]
			expand := true
			if target < 0 {
				expand = false
				target = -target
			}
			parent := f.Node(target - 1)
			index := f.Index(target - 1)
			if parent.CanReplaceWith(index, index, insert.Type, insert.Marks) {
				end := to
				if expand {
					end = r.After(target)
				}
				return t.Replace(f.Before(target), end, NewSlice(closeFragment(slice.Content, 0, slice.OpenStart, openDepth, nil), openDepth, slice.OpenEnd))
			}
		}
	}
	startSteps := len(t.Steps)
	for i := len(targetDepths) - 1; i >= 0; i-- {
		t.Replace(from, to, slice)
		if len(t.Steps) > startSteps {
			break
		}
		depth := targetDepths[i]
		if depth < 0 {
			continue
		}
		from, to = f.Before(depth), r.After(depth)
	}
	return t
}

func closeFragment(fragment *Fragment, depth, oldOpen, newOpen int, parent *Node) *Fragment {
	if depth < oldOpen && fragment.ChildCount() > 0 {
		first := fragment.FirstChild()
		fragment = fragment.ReplaceChild(0, first.Copy(closeFragment(first.Content, depth+1, oldOpen, newOpen, first)))
	}
	if depth > newOpen && parent != nil {
		match := parent.contentMatchAt(0)
		start := fragment
		if before := match.FillBefore(fragment, false, 0); before != nil {
			start = before.Append(fragment)
		}
		fragment = start
		if m := match.MatchFragment(start, 0, -1); m != nil {
			if after := m.FillBefore(emptyFragment, true, 0); after != nil {
				fragment = start.Append(after)
			}
		}
	}
	return fragment
}

// ReplaceRangeWith replaces from to to with node.
func (t *Transform) ReplaceRangeWith(from, to int, node *Node) *Transform {
	if t.Err != nil {
		return t
	}
	if !node.IsInline() && from == to && t.Doc.Resolve(from).Parent().Content.Size > 0 {
		if point := InsertPoint(t.Doc, from, node.Type); point >= 0 {
			from, to = point, point
		}
	}
	return t.ReplaceRange(from, to, NewSlice(FragmentFrom(node), 0, 0))
}

// AddMark adds mark from from to to where it is allowed.
func (t *Transform) AddMark(from, to int, mark *Mark) *Transform {
	var removed []Step
	var added []*AddMarkStep
	var removing *RemoveMarkStep
	var adding *AddMarkStep
	t.Doc.NodesBetween(from, to, func(node *Node, pos int, parent *Node, _ int) bool {
		if !node.IsInline() {
			return true
		}
		marks := node.Marks
		if mark.IsInSet(marks) || !parent.Type.AllowsMarkType(mark.Type) {
			return true
		}
		start, end := max(pos, from), min(pos+node.NodeSize(), to)
		newSet := mark.AddToSet(marks)
		for _, m := range marks {
			if !m.IsInSet(newSet) {
				if removing != nil && removing.To == start && removing.Mark.Eq(m) {
					removing.To = end
				} else {
					removing = &RemoveMarkStep{start, end, m}
					removed = append(removed, removing)
				}
			}
		}
		if adding != nil && adding.To == start {
			adding.To = end
		} else {
			adding = &AddMarkStep{start, end, mark}
			added = append(added, adding)
		}
		return true
	}, 0)
	for _, s := range removed {
		t.Step(s)
	}
	for _, s := range added {
		t.Step(s)
	}
	return t
}

// RemoveMark removes mark from from to to; with a nil mark every mark, and
// with a type its marks by it.
func (t *Transform) RemoveMark(from, to int, mark *Mark, typ *MarkType) *Transform {
	type matched struct {
		style *Mark
		from  int
		to    int
		step  int
	}
	var found []*matched
	step := 0
	t.Doc.NodesBetween(from, to, func(node *Node, pos int, _ *Node, _ int) bool {
		if !node.IsInline() {
			return true
		}
		step++
		var toRemove []*Mark
		switch {
		case typ != nil:
			for _, m := range node.Marks {
				if m.Type == typ {
					toRemove = append(toRemove, m)
				}
			}
		case mark != nil:
			if mark.IsInSet(node.Marks) {
				toRemove = []*Mark{mark}
			}
		default:
			toRemove = node.Marks
		}
		if len(toRemove) > 0 {
			end := min(pos+node.NodeSize(), to)
			for _, style := range toRemove {
				var hit *matched
				for _, m := range found {
					if m.step == step-1 && style.Eq(m.style) {
						hit = m
					}
				}
				if hit != nil {
					hit.to, hit.step = end, step
				} else {
					found = append(found, &matched{style, max(pos, from), end, step})
				}
			}
		}
		return true
	}, 0)
	for _, m := range found {
		t.Step(&RemoveMarkStep{m.from, m.to, m.style})
	}
	return t
}

// ClearIncompatible drops what a node of t at pos could not hold.
func (t *Transform) ClearIncompatible(pos int, typ *NodeType, match *ContentMatch) *Transform {
	if match == nil {
		match = typ.ContentMatch
	}
	node := t.Doc.NodeAt(pos)
	var repl []Step
	cur := pos + 1
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		end := cur + child.NodeSize()
		allowed := match.MatchType(child.Type)
		if allowed == nil {
			repl = append(repl, &ReplaceStep{From: cur, To: end, Slice: EmptySlice})
		} else {
			match = allowed
			for _, m := range child.Marks {
				if !typ.AllowsMarkType(m.Type) {
					t.Step(&RemoveMarkStep{cur, end, m})
				}
			}
		}
		cur = end
	}
	if !match.ValidEnd {
		fill := match.FillBefore(emptyFragment, true, 0)
		t.Replace(cur, cur, NewSlice(fill, 0, 0))
	}
	for i := len(repl) - 1; i >= 0; i-- {
		t.Step(repl[i])
	}
	return t
}

func canCut(node *Node, start, end int) bool {
	return (start == 0 || node.CanReplace(start, node.ChildCount(), nil, 0, -1)) &&
		(end == node.ChildCount() || node.CanReplace(0, end, nil, 0, -1))
}

// LiftTarget is the depth a range can lift to, -1 for none.
func LiftTarget(r *NodeRange) int {
	parent := r.Parent()
	content := parent.Content.CutByIndex(r.StartIndex(), r.EndIndex())
	for depth := r.Depth; ; depth-- {
		node := r.From.Node(depth)
		index, endIndex := r.From.Index(depth), r.To.IndexAfter(depth)
		if depth < r.Depth && node.CanReplace(index, endIndex, content, 0, -1) {
			return depth
		}
		if depth == 0 || node.Type.Spec.Isolating || !canCut(node, index, endIndex) {
			break
		}
	}
	return -1
}

// Lift moves range up to target.
func (t *Transform) Lift(r *NodeRange, target int) *Transform {
	from, to, depth := r.From, r.To, r.Depth
	gapStart, gapEnd := from.Before(depth+1), to.After(depth+1)
	start, end := gapStart, gapEnd
	before := emptyFragment
	openStart := 0
	for d, splitting := depth, false; d > target; d-- {
		if splitting || from.Index(d) > 0 {
			splitting = true
			before = FragmentFrom(from.Node(d).Copy(before))
			openStart++
		} else {
			start--
		}
	}
	after := emptyFragment
	openEnd := 0
	for d, splitting := depth, false; d > target; d-- {
		if splitting || to.After(d+1) < to.End(d) {
			splitting = true
			after = FragmentFrom(to.Node(d).Copy(after))
			openEnd++
		} else {
			end++
		}
	}
	return t.Step(&ReplaceAroundStep{From: start, To: end, GapFrom: gapStart, GapTo: gapEnd,
		Slice: NewSlice(before.Append(after), openStart, openEnd), Insert: before.Size - openStart, Structure: true})
}

// NodeTypeAttrs is a wrapper and its attributes.
type NodeTypeAttrs struct {
	Type  *NodeType
	Attrs Attrs
}

// FindWrapping is the wrappers a range takes to sit in typ, nil for none.
func FindWrapping(r *NodeRange, typ *NodeType, attrs Attrs, innerRange *NodeRange) []NodeTypeAttrs {
	if innerRange == nil {
		innerRange = r
	}
	around := findWrappingOutside(r, typ)
	if around == nil {
		return nil
	}
	inner := findWrappingInside(innerRange, typ)
	if inner == nil {
		return nil
	}
	out := make([]NodeTypeAttrs, 0, len(around)+1+len(inner))
	for _, t := range around {
		out = append(out, NodeTypeAttrs{t, nil})
	}
	out = append(out, NodeTypeAttrs{typ, attrs})
	for _, t := range inner {
		out = append(out, NodeTypeAttrs{t, nil})
	}
	return out
}

func findWrappingOutside(r *NodeRange, typ *NodeType) []*NodeType {
	parent := r.Parent()
	around := parent.contentMatchAt(r.StartIndex()).FindWrapping(typ)
	if around == nil {
		return nil
	}
	outer := typ
	if len(around) > 0 {
		outer = around[0]
	}
	if !parent.CanReplaceWith(r.StartIndex(), r.EndIndex(), outer, nil) {
		return nil
	}
	return around
}

func findWrappingInside(r *NodeRange, typ *NodeType) []*NodeType {
	parent := r.Parent()
	inner := parent.Child(r.StartIndex())
	inside := typ.ContentMatch.FindWrapping(inner.Type)
	if inside == nil {
		return nil
	}
	last := typ
	if len(inside) > 0 {
		last = inside[len(inside)-1]
	}
	match := last.ContentMatch
	for i := r.StartIndex(); match != nil && i < r.EndIndex(); i++ {
		match = match.MatchType(parent.Child(i).Type)
	}
	if match == nil || !match.ValidEnd {
		return nil
	}
	return inside
}

// Wrap wraps range in wrappers.
func (t *Transform) Wrap(r *NodeRange, wrappers []NodeTypeAttrs) *Transform {
	content := emptyFragment
	for i := len(wrappers) - 1; i >= 0; i-- {
		if content.Size > 0 {
			match := wrappers[i].Type.ContentMatch.MatchFragment(content, 0, -1)
			if match == nil || !match.ValidEnd {
				t.Err = errors.New("wrapper type given to Transform.Wrap does not form valid content of its parent wrapper")
				return t
			}
		}
		content = FragmentFrom(wrappers[i].Type.Create(wrappers[i].Attrs, content, nil))
	}
	start, end := r.Start(), r.End()
	return t.Step(&ReplaceAroundStep{From: start, To: end, GapFrom: start, GapTo: end, Slice: NewSlice(content, 0, 0), Insert: len(wrappers), Structure: true})
}

func canChangeType(doc *Node, pos int, typ *NodeType) bool {
	r := doc.Resolve(pos)
	index := r.Index(r.Depth)
	return r.Parent().CanReplaceWith(index, index+1, typ, nil)
}

// SetBlockType sets the textblocks from from to to to typ.
func (t *Transform) SetBlockType(from, to int, typ *NodeType, attrs func(*Node) Attrs) *Transform {
	if !typ.IsTextblock() {
		t.Err = errors.New("type given to SetBlockType should be a textblock")
		return t
	}
	mapFrom := len(t.Steps)
	t.Doc.NodesBetween(from, to, func(node *Node, pos int, _ *Node, _ int) bool {
		var a Attrs
		if attrs != nil {
			a = attrs(node)
		}
		if node.IsTextblock() && !node.HasMarkup(typ, typ.computeAttrs(a), nil) && canChangeType(t.Doc, t.Mapping.Slice(mapFrom, len(t.Mapping.Maps)).Map(pos, 1), typ) {
			t.ClearIncompatible(t.Mapping.Slice(mapFrom, len(t.Mapping.Maps)).Map(pos, 1), typ, nil)
			mapping := t.Mapping.Slice(mapFrom, len(t.Mapping.Maps))
			startM, endM := mapping.Map(pos, 1), mapping.Map(pos+node.NodeSize(), 1)
			t.Step(&ReplaceAroundStep{From: startM, To: endM, GapFrom: startM + 1, GapTo: endM - 1,
				Slice: NewSlice(FragmentFrom(typ.Create(a, nil, node.Marks)), 0, 0), Insert: 1, Structure: true})
			return false
		}
		return true
	}, 0)
	return t
}

// SetNodeMarkup changes the node at pos, keeping its content.
func (t *Transform) SetNodeMarkup(pos int, typ *NodeType, attrs Attrs, marks []*Mark) *Transform {
	node := t.Doc.NodeAt(pos)
	if node == nil {
		t.Err = errors.New("no node at given position")
		return t
	}
	if typ == nil {
		typ = node.Type
	}
	if marks == nil {
		marks = node.Marks
	}
	newNode := typ.Create(attrs, nil, marks)
	if node.IsLeaf() {
		return t.ReplaceWith(pos, pos+node.NodeSize(), newNode)
	}
	if !typ.ValidContent(node.Content) {
		t.Err = errors.New("invalid content for node type " + typ.Name)
		return t
	}
	return t.Step(&ReplaceAroundStep{From: pos, To: pos + node.NodeSize(), GapFrom: pos + 1, GapTo: pos + node.NodeSize() - 1,
		Slice: NewSlice(FragmentFrom(newNode), 0, 0), Insert: 1, Structure: true})
}

// SetNodeAttribute sets one attribute of the node at pos.
func (t *Transform) SetNodeAttribute(pos int, attr string, value any) *Transform {
	return t.Step(&AttrStep{pos, attr, value})
}

// CanSplit says pos can split depth levels, each new level's types or the
// same when nil.
func CanSplit(doc *Node, pos, depth int, typesAfter []NodeTypeAttrs) bool {
	r := doc.Resolve(pos)
	base := r.Depth - depth
	var innerType NodeTypeAttrs
	if n := len(typesAfter); n > 0 && typesAfter[n-1].Type != nil {
		innerType = typesAfter[n-1]
	} else {
		innerType = NodeTypeAttrs{Type: r.Parent().Type}
	}
	if base < 0 || r.Parent().Type.Spec.Isolating ||
		!r.Parent().CanReplace(r.Index(r.Depth), r.Parent().ChildCount(), nil, 0, -1) ||
		!innerType.Type.ValidContent(r.Parent().Content.CutByIndex(r.Index(r.Depth), r.Parent().ChildCount())) {
		return false
	}
	for d, i := r.Depth-1, depth-2; d > base; d, i = d-1, i-1 {
		node := r.Node(d)
		index := r.Index(d)
		if node.Type.Spec.Isolating {
			return false
		}
		rest := node.Content.CutByIndex(index, node.ChildCount())
		var overrideChild NodeTypeAttrs
		if i+1 >= 0 && i+1 < len(typesAfter) {
			overrideChild = typesAfter[i+1]
		}
		if overrideChild.Type != nil {
			rest = rest.ReplaceChild(0, overrideChild.Type.Create(overrideChild.Attrs, nil, nil))
		}
		after := node.Type
		if i >= 0 && i < len(typesAfter) && typesAfter[i].Type != nil {
			after = typesAfter[i].Type
		}
		if !node.CanReplace(index+1, node.ChildCount(), nil, 0, -1) || !after.ValidContent(rest) {
			return false
		}
	}
	index := r.IndexAfter(base)
	baseType := r.Node(base + 1).Type
	if len(typesAfter) > 0 && typesAfter[0].Type != nil {
		baseType = typesAfter[0].Type
	}
	return r.Node(base).CanReplaceWith(index, index, baseType, nil)
}

// Split splits the node around pos, depth levels up, with typesAfter for
// the new nodes.
func (t *Transform) Split(pos, depth int, typesAfter []NodeTypeAttrs) *Transform {
	r := t.Doc.Resolve(pos)
	before, after := emptyFragment, emptyFragment
	for d, e, i := r.Depth, r.Depth-depth, depth-1; d > e; d, i = d-1, i-1 {
		before = FragmentFrom(r.Node(d).Copy(before))
		var typeAfter NodeTypeAttrs
		if i >= 0 && i < len(typesAfter) {
			typeAfter = typesAfter[i]
		}
		if typeAfter.Type != nil {
			after = FragmentFrom(typeAfter.Type.Create(typeAfter.Attrs, after, nil))
		} else {
			after = FragmentFrom(r.Node(d).Copy(after))
		}
	}
	return t.Step(&ReplaceStep{From: pos, To: pos, Slice: NewSlice(before.Append(after), depth, depth), Structure: true})
}

// CanJoin says the nodes around pos can join.
func CanJoin(doc *Node, pos int) bool {
	r := doc.Resolve(pos)
	index := r.Index(r.Depth)
	return joinableNodes(r.NodeBefore(), r.NodeAfter()) && r.Parent().CanReplace(index, index+1, nil, 0, -1)
}

func canAppendWithSubstitutedLinebreaks(a, b *Node) bool { return a.CanAppend(b) }

func joinableNodes(a, b *Node) bool {
	return a != nil && b != nil && !a.IsLeaf() && canAppendWithSubstitutedLinebreaks(a, b)
}

// JoinPoint is the nearest place around pos, going dir, that can join, -1
// for none.
func JoinPoint(doc *Node, pos, dir int) int {
	r := doc.Resolve(pos)
	for d := r.Depth; ; d-- {
		var before, after *Node
		index := r.Index(d)
		if d == r.Depth {
			before = r.NodeBefore()
			after = r.NodeAfter()
		} else if dir > 0 {
			before = r.Node(d + 1)
			index++
			after = r.Node(d).MaybeChild(index)
		} else {
			before = r.Node(d).MaybeChild(index - 1)
			after = r.Node(d + 1)
		}
		if before != nil && !before.IsTextblock() && joinableNodes(before, after) && r.Node(d).CanReplace(index, index+1, nil, 0, -1) {
			return pos
		}
		if d == 0 {
			break
		}
		if dir < 0 {
			pos = r.Before(d)
		} else {
			pos = r.After(d)
		}
	}
	return -1
}

// Join joins the nodes depth levels around pos.
func (t *Transform) Join(pos, depth int) *Transform {
	return t.Step(&ReplaceStep{From: pos - depth, To: pos + depth, Slice: EmptySlice, Structure: true})
}

// InsertPoint is where a node of typ fits at or near pos, -1 for nowhere.
func InsertPoint(doc *Node, pos int, typ *NodeType) int {
	r := doc.Resolve(pos)
	if r.Parent().CanReplaceWith(r.Index(r.Depth), r.Index(r.Depth), typ, nil) {
		return pos
	}
	if r.ParentOffset == 0 {
		for d := r.Depth - 1; d >= 0; d-- {
			index := r.Index(d)
			if r.Node(d).CanReplaceWith(index, index, typ, nil) {
				return r.Before(d + 1)
			}
			if index > 0 {
				return -1
			}
		}
	}
	if r.ParentOffset == r.Parent().Content.Size {
		for d := r.Depth - 1; d >= 0; d-- {
			index := r.IndexAfter(d)
			if r.Node(d).CanReplaceWith(index, index, typ, nil) {
				return r.After(d + 1)
			}
			if index < r.Node(d).ChildCount() {
				return -1
			}
		}
	}
	return -1
}
