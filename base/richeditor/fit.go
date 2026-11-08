package richeditor

// fittable is a place the slice's content can go.
type fittable struct {
	sliceDepth, frontierDepth int
	parent                    *Node
	inject                    *Fragment
	wrap                      []*NodeType
}

type frontierLevel struct {
	typ   *NodeType
	match *ContentMatch
}

type closeLevel struct {
	depth int
	fit   *Fragment
	move  *ResolvedPos
}

// fitter places a slice's content into a gap: prosemirror-transform's
// Fitter. The frontier is the replacement's open side, unplaced what is left
// to place and placed what was.
type fitter struct {
	from, to *ResolvedPos
	unplaced *Slice
	frontier []frontierLevel
	placed   *Fragment
}

// ReplaceStepFit fits slice into from to to of doc: the step that inserts
// it, closing and wrapping nodes as needed, nil when it has no place there or
// nothing changes.
func ReplaceStepFit(doc *Node, from, to int, slice *Slice) Step {
	if slice == nil {
		slice = EmptySlice
	}
	if from == to && slice.Size() == 0 {
		return nil
	}
	f, t := doc.Resolve(from), doc.Resolve(to)
	if fitsTrivially(f, t, slice) {
		return &ReplaceStep{From: from, To: to, Slice: slice}
	}
	return newFitter(f, t, slice).fit()
}

func fitsTrivially(from, to *ResolvedPos, slice *Slice) bool {
	return slice.OpenStart == 0 && slice.OpenEnd == 0 && from.Start(from.Depth) == to.Start(to.Depth) &&
		from.Parent().CanReplace(from.Index(from.Depth), to.Index(to.Depth), slice.Content, 0, -1)
}

func newFitter(from, to *ResolvedPos, unplaced *Slice) *fitter {
	f := &fitter{from: from, to: to, unplaced: unplaced, placed: emptyFragment}
	for i := 0; i <= from.Depth; i++ {
		node := from.Node(i)
		f.frontier = append(f.frontier, frontierLevel{node.Type, node.contentMatchAt(from.IndexAfter(i))})
	}
	for i := from.Depth; i > 0; i-- {
		f.placed = FragmentFrom(from.Node(i).Copy(f.placed))
	}
	return f
}

func (f *fitter) depth() int { return len(f.frontier) - 1 }

func (f *fitter) fit() Step {
	for f.unplaced.Size() > 0 {
		if fit := f.findFittable(); fit != nil {
			if !f.placeNodes(fit) {
				return nil
			}
		} else if !f.openMore() {
			f.dropNode()
		}
	}
	moveInline := f.mustMoveInline()
	placedSize := f.placed.Size - f.depth() - f.from.Depth
	from := f.from
	closeTo := f.to
	if moveInline >= 0 {
		closeTo = from.Doc().Resolve(moveInline)
	}
	to := f.close(closeTo)
	if to == nil {
		return nil
	}
	content := f.placed
	openStart, openEnd := from.Depth, to.Depth
	for openStart > 0 && openEnd > 0 && content.ChildCount() == 1 {
		content = content.FirstChild().Content
		openStart--
		openEnd--
	}
	slice := NewSlice(content, openStart, openEnd)
	if moveInline >= 0 {
		return &ReplaceAroundStep{From: from.Pos, To: moveInline, GapFrom: f.to.Pos, GapTo: f.to.End(f.to.Depth), Slice: slice, Insert: max(placedSize, 0)}
	}
	if slice.Size() > 0 || from.Pos != f.to.Pos {
		return &ReplaceStep{From: from.Pos, To: to.Pos, Slice: slice}
	}
	return nil
}

func (f *fitter) findFittable() *fittable {
	startDepth := f.unplaced.OpenStart
	cur := f.unplaced.Content
	openEnd := f.unplaced.OpenEnd
	for d := 0; d < startDepth; d++ {
		node := cur.FirstChild()
		if cur.ChildCount() > 1 {
			openEnd = 0
		}
		if node.Type.Spec.Isolating && openEnd <= d {
			startDepth = d
			break
		}
		cur = node.Content
	}
	for pass := 1; pass <= 2; pass++ {
		top := startDepth
		if pass == 2 {
			top = f.unplaced.OpenStart
		}
		for sliceDepth := top; sliceDepth >= 0; sliceDepth-- {
			fragment := f.unplaced.Content
			var parent *Node
			if sliceDepth > 0 {
				parent = contentAt(f.unplaced.Content, sliceDepth-1).FirstChild()
				fragment = parent.Content
			}
			first := fragment.FirstChild()
			for frontierDepth := f.depth(); frontierDepth >= 0; frontierDepth-- {
				level := f.frontier[frontierDepth]
				if pass == 1 {
					var inject *Fragment
					found := false
					if first != nil {
						if level.match.MatchType(first.Type) != nil {
							found = true
						} else if inject = level.match.FillBefore(FragmentFrom(first), false, 0); inject != nil {
							found = true
						}
					} else if parent != nil && level.typ.compatibleContent(parent.Type) {
						found = true
					}
					if found {
						return &fittable{sliceDepth: sliceDepth, frontierDepth: frontierDepth, parent: parent, inject: inject}
					}
				} else if first != nil {
					if wrap := level.match.FindWrapping(first.Type); wrap != nil {
						return &fittable{sliceDepth: sliceDepth, frontierDepth: frontierDepth, parent: parent, wrap: wrap}
					}
				}
				if parent != nil && level.match.MatchType(parent.Type) != nil {
					break
				}
			}
		}
	}
	return nil
}

func (f *fitter) openMore() bool {
	u := f.unplaced
	if maxOpen(u.Content, false) <= u.OpenStart {
		return false
	}
	openEnd := u.OpenEnd
	if u.Size() > 1 && maxOpen(u.Content, true) > openEnd {
		openEnd++
	}
	f.unplaced = NewSlice(u.Content, u.OpenStart+1, openEnd)
	return true
}

func (f *fitter) dropNode() {
	u := f.unplaced
	inner := contentAt(u.Content, u.OpenStart)
	if inner.ChildCount() <= 1 && u.OpenStart > 0 {
		openAtEnd := u.Content.Size-u.OpenStart <= u.OpenStart+inner.Size
		openEnd := u.OpenEnd
		if openAtEnd {
			openEnd = u.OpenStart - 1
		}
		f.unplaced = NewSlice(dropFromFragment(u.Content, u.OpenStart-1, 1), u.OpenStart-1, openEnd)
	} else {
		f.unplaced = NewSlice(dropFromFragment(u.Content, u.OpenStart, 1), u.OpenStart, u.OpenEnd)
	}
}

func (f *fitter) placeNodes(fit *fittable) bool {
	for f.depth() > fit.frontierDepth {
		f.closeFrontierNode()
	}
	for _, t := range fit.wrap {
		if !f.openFrontierNode(t, nil, nil) {
			return false
		}
	}
	slice := f.unplaced
	fragment := slice.Content
	if fit.parent != nil {
		fragment = fit.parent.Content
	}
	openStart := slice.OpenStart - fit.sliceDepth
	taken := 0
	var add []*Node
	level := f.frontier[fit.frontierDepth]
	match := level.match
	if fit.inject != nil {
		add = append(add, fit.inject.Content...)
		if match = match.MatchFragment(fit.inject, 0, -1); match == nil {
			return false
		}
	}
	openEndCount := fragment.Size + fit.sliceDepth - (slice.Content.Size - slice.OpenEnd)
	for taken < fragment.ChildCount() {
		next := fragment.Child(taken)
		matches := match.MatchType(next.Type)
		if matches == nil {
			break
		}
		taken++
		if taken > 1 || openStart == 0 || next.Content.Size > 0 {
			match = matches
			oStart, oEnd := 0, -1
			if taken == 1 {
				oStart = openStart
			}
			if taken == fragment.ChildCount() {
				oEnd = openEndCount
			}
			add = append(add, closeNodeStart(next.Mark(level.typ.allowedMarks(next.Marks)), oStart, oEnd))
		}
	}
	toEnd := taken == fragment.ChildCount()
	if !toEnd {
		openEndCount = -1
	}
	f.placed = addToFragment(f.placed, fit.frontierDepth, FragmentFrom(add...))
	f.frontier[fit.frontierDepth].match = match
	if toEnd && openEndCount < 0 && fit.parent != nil && fit.parent.Type == f.frontier[f.depth()].typ && len(f.frontier) > 1 {
		f.closeFrontierNode()
	}
	cur := fragment
	for i := 0; i < openEndCount; i++ {
		node := cur.LastChild()
		f.frontier = append(f.frontier, frontierLevel{node.Type, node.contentMatchAt(node.ChildCount())})
		cur = node.Content
	}
	switch {
	case !toEnd:
		f.unplaced = NewSlice(dropFromFragment(slice.Content, fit.sliceDepth, taken), slice.OpenStart, slice.OpenEnd)
	case fit.sliceDepth == 0:
		f.unplaced = EmptySlice
	default:
		openEnd := slice.OpenEnd
		if openEndCount >= 0 {
			openEnd = fit.sliceDepth - 1
		}
		f.unplaced = NewSlice(dropFromFragment(slice.Content, fit.sliceDepth-1, 1), fit.sliceDepth-1, openEnd)
	}
	return true
}

func (f *fitter) mustMoveInline() int {
	if !f.to.Parent().IsTextblock() {
		return -1
	}
	top := f.frontier[f.depth()]
	if !top.typ.IsTextblock() || contentAfterFits(f.to, f.to.Depth, top.typ, top.match, false) == nil {
		return -1
	}
	if f.to.Depth == f.depth() {
		if l := f.findCloseLevel(f.to); l != nil && l.depth == f.depth() {
			return -1
		}
	}
	depth := f.to.Depth
	after := f.to.After(depth)
	for depth > 1 {
		depth--
		if after != f.to.End(depth) {
			break
		}
		after++
	}
	return after
}

func (f *fitter) findCloseLevel(to *ResolvedPos) *closeLevel {
scan:
	for i := min(f.depth(), to.Depth); i >= 0; i-- {
		level := f.frontier[i]
		dropInner := i < to.Depth && to.End(i+1) == to.Pos+(to.Depth-(i+1))
		fit := contentAfterFits(to, i, level.typ, level.match, dropInner)
		if fit == nil {
			continue
		}
		for d := i - 1; d >= 0; d-- {
			l := f.frontier[d]
			m := contentAfterFits(to, d, l.typ, l.match, true)
			if m == nil || m.ChildCount() != 0 {
				continue scan
			}
		}
		move := to
		if dropInner {
			move = to.Doc().Resolve(to.After(i + 1))
		}
		return &closeLevel{i, fit, move}
	}
	return nil
}

func (f *fitter) close(to *ResolvedPos) *ResolvedPos {
	c := f.findCloseLevel(to)
	if c == nil {
		return nil
	}
	for f.depth() > c.depth {
		f.closeFrontierNode()
	}
	if c.fit.ChildCount() > 0 {
		f.placed = addToFragment(f.placed, c.depth, c.fit)
	}
	to = c.move
	for d := c.depth + 1; d <= to.Depth; d++ {
		node := to.Node(d)
		add := node.Type.ContentMatch.FillBefore(node.Content, true, to.Index(d))
		if add == nil || !f.openFrontierNode(node.Type, node.Attrs, add) {
			return nil
		}
	}
	return to
}

func (f *fitter) openFrontierNode(t *NodeType, attrs Attrs, content *Fragment) bool {
	top := &f.frontier[f.depth()]
	top.match = top.match.MatchType(t)
	if top.match == nil {
		return false
	}
	f.placed = addToFragment(f.placed, f.depth(), FragmentFrom(t.Create(attrs, content, nil)))
	f.frontier = append(f.frontier, frontierLevel{t, t.ContentMatch})
	return true
}

func (f *fitter) closeFrontierNode() {
	open := f.frontier[len(f.frontier)-1]
	f.frontier = f.frontier[:len(f.frontier)-1]
	if add := open.match.FillBefore(emptyFragment, true, 0); add != nil && add.ChildCount() > 0 {
		f.placed = addToFragment(f.placed, len(f.frontier), add)
	}
}

func dropFromFragment(fragment *Fragment, depth, count int) *Fragment {
	if depth == 0 {
		return fragment.CutByIndex(min(count, fragment.ChildCount()), fragment.ChildCount())
	}
	first := fragment.FirstChild()
	return fragment.ReplaceChild(0, first.Copy(dropFromFragment(first.Content, depth-1, count)))
}

func addToFragment(fragment *Fragment, depth int, content *Fragment) *Fragment {
	if depth == 0 {
		return fragment.Append(content)
	}
	last := fragment.ChildCount() - 1
	return fragment.ReplaceChild(last, fragment.Child(last).Copy(addToFragment(fragment.Child(last).Content, depth-1, content)))
}

func contentAt(fragment *Fragment, depth int) *Fragment {
	for range depth {
		fragment = fragment.FirstChild().Content
	}
	return fragment
}

func closeNodeStart(node *Node, openStart, openEnd int) *Node {
	if openStart <= 0 {
		return node
	}
	frag := node.Content
	if openStart > 1 {
		innerEnd := 0
		if frag.ChildCount() == 1 {
			innerEnd = openEnd - 1
		}
		frag = frag.ReplaceChild(0, closeNodeStart(frag.FirstChild(), openStart-1, innerEnd))
	}
	if before := node.Type.ContentMatch.FillBefore(frag, false, 0); before != nil {
		frag = before.Append(frag)
	}
	if openEnd <= 0 {
		if m := node.Type.ContentMatch.MatchFragment(frag, 0, -1); m != nil {
			if after := m.FillBefore(emptyFragment, true, 0); after != nil {
				frag = frag.Append(after)
			}
		}
	}
	return node.Copy(frag)
}

func contentAfterFits(to *ResolvedPos, depth int, t *NodeType, match *ContentMatch, open bool) *Fragment {
	node := to.Node(depth)
	index := to.Index(depth)
	if open {
		index = to.IndexAfter(depth)
	}
	if index == node.ChildCount() && !t.compatibleContent(node.Type) {
		return nil
	}
	fit := match.FillBefore(node.Content, true, index)
	if fit == nil {
		return nil
	}
	for i := index; i < node.ChildCount(); i++ {
		if !t.AllowsMarks(node.Child(i).Marks) {
			return nil
		}
	}
	return fit
}

func maxOpen(fragment *Fragment, end bool) int {
	count := 0
	for fragment.ChildCount() > 0 {
		ch := fragment.FirstChild()
		if end {
			ch = fragment.LastChild()
		}
		if ch.IsAtom() {
			break
		}
		fragment = ch.Content
		count++
	}
	return count
}
