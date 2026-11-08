package richeditor

import (
	"errors"
	"slices"
)

// Mappable maps positions through changes.
type Mappable interface {
	Map(pos, assoc int) int
	MapResult(pos, assoc int) MapResult
}

// MapResult is a position mapped: where it went and what became of the
// content around it.
type MapResult struct {
	Pos     int
	deleted int
	// recover names the range it was in, for undoing through a mirror.
	recover int
}

const (
	delBefore = 1
	delAfter  = 2
	delAcross = 4
	delSide   = 8
)

// Deleted says the token on the side it was mapped with is gone.
func (m MapResult) Deleted() bool { return m.deleted&delSide != 0 }

// DeletedBefore and DeletedAfter say the token before or after it is gone.
func (m MapResult) DeletedBefore() bool { return m.deleted&(delBefore|delAcross) != 0 }
func (m MapResult) DeletedAfter() bool  { return m.deleted&(delAfter|delAcross) != 0 }

// DeletedAcross says tokens on both sides are gone.
func (m MapResult) DeletedAcross() bool { return m.deleted&delAcross != 0 }

const (
	recoverMask   = (1 << 16) - 1
	recoverFactor = 1 << 16
)

func makeRecover(index, offset int) int { return index + offset*recoverFactor }
func recoverIndex(v int) int            { return v & recoverMask }
func recoverOffset(v int) int           { return (v - v&recoverMask) / recoverFactor }

// StepMap is how a step moves positions: triples of a start, an old size
// and a new size.
type StepMap struct {
	ranges   []int
	inverted bool
}

// EmptyStepMap moves nothing.
var EmptyStepMap = &StepMap{}

func (m *StepMap) recover(value int) int {
	diff, index := 0, recoverIndex(value)
	if !m.inverted {
		for i := 0; i < index; i++ {
			diff += m.ranges[i*3+2] - m.ranges[i*3+1]
		}
	}
	return m.ranges[index*3] + diff + recoverOffset(value)
}

// MapResult maps pos, assoc saying which side it sticks to.
func (m *StepMap) MapResult(pos, assoc int) MapResult { return m.mapping(pos, assoc, false) }

// Map maps pos.
func (m *StepMap) Map(pos, assoc int) int { return m.mapping(pos, assoc, true).Pos }

func (m *StepMap) mapping(pos, assoc int, simple bool) MapResult {
	diff := 0
	oldIndex, newIndex := 1, 2
	if m.inverted {
		oldIndex, newIndex = 2, 1
	}
	for i := 0; i < len(m.ranges); i += 3 {
		start := m.ranges[i]
		if m.inverted {
			start -= diff
		}
		if start > pos {
			break
		}
		oldSize, newSize := m.ranges[i+oldIndex], m.ranges[i+newIndex]
		end := start + oldSize
		if pos <= end {
			side := assoc
			if oldSize == 0 {
				// Keep assoc.
			} else if pos == start {
				side = -1
			} else if pos == end {
				side = 1
			}
			result := start + diff
			if side >= 0 {
				result += newSize
			}
			if simple {
				return MapResult{Pos: result}
			}
			side2 := end
			if assoc < 0 {
				side2 = start
			}
			recoverValue := -1
			if pos != side2 {
				recoverValue = makeRecover(i/3, pos-start)
			}
			del := 0
			if pos == start {
				del = delAfter
			} else if pos == end {
				del = delBefore
			} else {
				del = delAcross
			}
			if (assoc < 0 && pos != start) || (assoc >= 0 && pos != end) {
				del |= delSide
			}
			return MapResult{Pos: result, deleted: del, recover: recoverValue}
		}
		diff += newSize - oldSize
	}
	return MapResult{Pos: pos + diff, recover: -1}
}

// ForEach calls f with each changed range: old start and end, new start
// and end.
func (m *StepMap) ForEach(f func(oldStart, oldEnd, newStart, newEnd int)) {
	oldIndex, newIndex := 1, 2
	if m.inverted {
		oldIndex, newIndex = 2, 1
	}
	diff := 0
	for i := 0; i < len(m.ranges); i += 3 {
		start := m.ranges[i]
		oldStart := start
		newStart := start + diff
		if m.inverted {
			oldStart = start - diff
			newStart = start
		}
		oldSize, newSize := m.ranges[i+oldIndex], m.ranges[i+newIndex]
		f(oldStart, oldStart+oldSize, newStart, newStart+newSize)
		diff += newSize - oldSize
	}
}

// Invert is the map back.
func (m *StepMap) Invert() *StepMap { return &StepMap{ranges: m.ranges, inverted: !m.inverted} }

// Mapping is a run of maps, some mirroring others, so what one deleted the
// other's undoing brings back.
type Mapping struct {
	Maps     []*StepMap
	From, To int
	mirror   []int
	own      bool
}

// NewMapping makes a mapping of maps.
func NewMapping(maps ...*StepMap) *Mapping { return &Mapping{Maps: maps, To: len(maps)} }

// Slice is the maps from from to to.
func (m *Mapping) Slice(from, to int) *Mapping {
	return &Mapping{Maps: m.Maps, mirror: m.mirror, From: from, To: to}
}

// AppendMap adds a map, mirroring map mirrors when it is not negative.
func (m *Mapping) AppendMap(sm *StepMap, mirrors int) {
	m.Maps = append(slices.Clip(m.Maps), sm)
	m.To = len(m.Maps)
	if mirrors >= 0 {
		m.setMirror(len(m.Maps)-1, mirrors)
	}
}

// AppendMapping adds another mapping's maps.
func (m *Mapping) AppendMapping(o *Mapping) {
	start := len(m.Maps)
	for i := range o.Maps {
		mirr := o.GetMirror(i)
		arg := -1
		if mirr >= 0 && mirr < i {
			arg = start + mirr
		}
		m.AppendMap(o.Maps[i], arg)
	}
}

// GetMirror is the map mirroring n, -1 for none.
func (m *Mapping) GetMirror(n int) int {
	for i := 0; i < len(m.mirror); i++ {
		if m.mirror[i] == n {
			if i%2 == 1 {
				return m.mirror[i-1]
			}
			return m.mirror[i+1]
		}
	}
	return -1
}

func (m *Mapping) setMirror(n, mirror int) { m.mirror = append(slices.Clip(m.mirror), n, mirror) }

// AppendMappingInverted adds another mapping's maps inverted, last first.
func (m *Mapping) AppendMappingInverted(o *Mapping) {
	total := len(m.Maps) + len(o.Maps)
	for i := len(o.Maps) - 1; i >= 0; i-- {
		mirr := o.GetMirror(i)
		arg := -1
		if mirr >= 0 && mirr > i {
			arg = total - mirr - 1
		}
		m.AppendMap(o.Maps[i].Invert(), arg)
	}
}

// Invert is the mapping back.
func (m *Mapping) Invert() *Mapping {
	inverse := NewMapping()
	inverse.AppendMappingInverted(m)
	return inverse
}

// Map maps pos.
func (m *Mapping) Map(pos, assoc int) int {
	if len(m.mirror) > 0 {
		return m.mapping(pos, assoc).Pos
	}
	for i := m.From; i < m.To; i++ {
		pos = m.Maps[i].Map(pos, assoc)
	}
	return pos
}

// MapResult maps pos and reports what happened.
func (m *Mapping) MapResult(pos, assoc int) MapResult { return m.mapping(pos, assoc) }

func (m *Mapping) mapping(pos, assoc int) MapResult {
	deleted := 0
	for i := m.From; i < m.To; i++ {
		result := m.Maps[i].MapResult(pos, assoc)
		if result.recover >= 0 {
			if corr := m.GetMirror(i); corr >= 0 && corr > i && corr < m.To {
				i = corr
				pos = m.Maps[corr].recover(result.recover)
				continue
			}
		}
		deleted |= result.deleted
		pos = result.Pos
	}
	return MapResult{Pos: pos, deleted: deleted, recover: -1}
}

// Step is one change of a document: it applies or fails, maps positions,
// inverts against the document it applied to and maps over other changes.
type Step interface {
	Apply(doc *Node) (*Node, error)
	StepMap() *StepMap
	Invert(doc *Node) Step
	// Map is the step over another change, nil when that deleted it.
	Map(m Mappable) Step
	// Merge joins the step with the next, nil when it cannot.
	Merge(other Step) Step
}

func fromReplace(doc *Node, from, to int, slice *Slice) (*Node, error) {
	return doc.Replace(from, to, slice)
}

// ReplaceStep replaces From to To with Slice.
type ReplaceStep struct {
	From, To  int
	Slice     *Slice
	Structure bool
}

func (s *ReplaceStep) Apply(doc *Node) (*Node, error) {
	if s.Structure && contentBetween(doc, s.From, s.To) {
		return nil, errors.New("structure replace would overwrite content")
	}
	return fromReplace(doc, s.From, s.To, s.Slice)
}

func (s *ReplaceStep) StepMap() *StepMap {
	return &StepMap{ranges: []int{s.From, s.To - s.From, s.Slice.Size()}}
}

func (s *ReplaceStep) Invert(doc *Node) Step {
	return &ReplaceStep{From: s.From, To: s.From + s.Slice.Size(), Slice: doc.Slice(s.From, s.To, false)}
}

func (s *ReplaceStep) Map(m Mappable) Step {
	from, to := m.MapResult(s.From, 1), m.MapResult(s.To, -1)
	if from.DeletedAcross() && to.DeletedAcross() {
		return nil
	}
	return &ReplaceStep{From: from.Pos, To: max(from.Pos, to.Pos), Slice: s.Slice, Structure: s.Structure}
}

func (s *ReplaceStep) Merge(other Step) Step {
	o, ok := other.(*ReplaceStep)
	if !ok || o.Structure || s.Structure {
		return nil
	}
	if s.From+s.Slice.Size() == o.From && s.Slice.OpenEnd == 0 && o.Slice.OpenStart == 0 {
		slice := EmptySlice
		if s.Slice.Size()+o.Slice.Size() != 0 {
			slice = NewSlice(s.Slice.Content.Append(o.Slice.Content), s.Slice.OpenStart, o.Slice.OpenEnd)
		}
		return &ReplaceStep{From: s.From, To: s.To + (o.To - o.From), Slice: slice}
	}
	if o.To == s.From && s.Slice.OpenStart == 0 && o.Slice.OpenEnd == 0 {
		slice := EmptySlice
		if s.Slice.Size()+o.Slice.Size() != 0 {
			slice = NewSlice(o.Slice.Content.Append(s.Slice.Content), o.Slice.OpenStart, s.Slice.OpenEnd)
		}
		return &ReplaceStep{From: o.From, To: s.To, Slice: slice}
	}
	return nil
}

// ReplaceAroundStep replaces From to To, keeping GapFrom to GapTo as the
// content at Insert of Slice.
type ReplaceAroundStep struct {
	From, To, GapFrom, GapTo int
	Slice                    *Slice
	Insert                   int
	Structure                bool
}

func (s *ReplaceAroundStep) Apply(doc *Node) (*Node, error) {
	if s.Structure && (contentBetween(doc, s.From, s.GapFrom) || contentBetween(doc, s.GapTo, s.To)) {
		return nil, errors.New("structure gap-replace would overwrite content")
	}
	gap := doc.Slice(s.GapFrom, s.GapTo, false)
	if gap.OpenStart != 0 || gap.OpenEnd != 0 {
		return nil, errors.New("gap is not a flat range")
	}
	inserted := s.Slice.InsertAt(s.Insert, gap.Content)
	if inserted == nil {
		return nil, errors.New("content does not fit in gap")
	}
	return fromReplace(doc, s.From, s.To, inserted)
}

func (s *ReplaceAroundStep) StepMap() *StepMap {
	return &StepMap{ranges: []int{s.From, s.GapFrom - s.From, s.Insert, s.GapTo, s.To - s.GapTo, s.Slice.Size() - s.Insert}}
}

func (s *ReplaceAroundStep) Invert(doc *Node) Step {
	gap := s.GapTo - s.GapFrom
	removed, err := doc.Slice(s.From, s.To, false).RemoveBetween(s.GapFrom-s.From, s.GapTo-s.From)
	if err != nil {
		panic("richeditor: " + err.Error())
	}
	return &ReplaceAroundStep{From: s.From, To: s.From + s.Slice.Size() + gap, GapFrom: s.From + s.Insert, GapTo: s.From + s.Insert + gap,
		Slice: removed, Insert: s.GapFrom - s.From, Structure: s.Structure}
}

func (s *ReplaceAroundStep) Map(m Mappable) Step {
	from, to := m.MapResult(s.From, 1), m.MapResult(s.To, -1)
	gapFrom, gapTo := from.Pos, to.Pos
	if s.From != s.GapFrom {
		gapFrom = m.Map(s.GapFrom, -1)
	}
	if s.To != s.GapTo {
		gapTo = m.Map(s.GapTo, 1)
	}
	if from.DeletedAcross() && to.DeletedAcross() || gapFrom < from.Pos || gapTo > to.Pos {
		return nil
	}
	return &ReplaceAroundStep{From: from.Pos, To: to.Pos, GapFrom: gapFrom, GapTo: gapTo, Slice: s.Slice, Insert: s.Insert, Structure: s.Structure}
}

func (s *ReplaceAroundStep) Merge(Step) Step { return nil }

func contentBetween(doc *Node, from, to int) bool {
	r := doc.Resolve(from)
	dist := to - from
	depth := r.Depth
	for dist > 0 && depth > 0 && r.IndexAfter(depth) == r.Node(depth).ChildCount() {
		depth--
		dist--
	}
	if dist > 0 {
		next := r.Node(depth).MaybeChild(r.IndexAfter(depth))
		for dist > 0 {
			if next == nil || next.IsLeaf() {
				return true
			}
			next = next.FirstChild()
			dist--
		}
	}
	return false
}

func mapFragment(f *Fragment, fn func(child, parent *Node, index int) *Node, parent *Node) *Fragment {
	mapped := make([]*Node, 0, f.ChildCount())
	for i, child := range f.Content {
		if child.Content.Size > 0 {
			child = child.Copy(mapFragment(child.Content, fn, child))
		}
		if child.IsInline() {
			child = fn(child, parent, i)
		}
		mapped = append(mapped, child)
	}
	return FragmentFrom(mapped...)
}

// AddMarkStep adds Mark from From to To.
type AddMarkStep struct {
	From, To int
	Mark     *Mark
}

func (s *AddMarkStep) Apply(doc *Node) (*Node, error) {
	old := doc.Slice(s.From, s.To, false)
	from := doc.Resolve(s.From)
	parent := from.Node(from.SharedDepth(s.To))
	content := mapFragment(old.Content, func(node, parent *Node, _ int) *Node {
		if !node.IsAtom() || !parent.Type.AllowsMarkType(s.Mark.Type) {
			return node
		}
		return node.Mark(s.Mark.AddToSet(node.Marks))
	}, parent)
	return fromReplace(doc, s.From, s.To, NewSlice(content, old.OpenStart, old.OpenEnd))
}

func (s *AddMarkStep) StepMap() *StepMap { return EmptyStepMap }
func (s *AddMarkStep) Invert(*Node) Step { return &RemoveMarkStep{s.From, s.To, s.Mark} }

func (s *AddMarkStep) Map(m Mappable) Step {
	from, to := m.MapResult(s.From, 1), m.MapResult(s.To, -1)
	if from.Deleted() && to.Deleted() || from.Pos >= to.Pos {
		return nil
	}
	return &AddMarkStep{from.Pos, to.Pos, s.Mark}
}

func (s *AddMarkStep) Merge(other Step) Step {
	if o, ok := other.(*AddMarkStep); ok && o.Mark.Eq(s.Mark) && s.From <= o.To && s.To >= o.From {
		return &AddMarkStep{min(s.From, o.From), max(s.To, o.To), s.Mark}
	}
	return nil
}

// RemoveMarkStep removes Mark from From to To.
type RemoveMarkStep struct {
	From, To int
	Mark     *Mark
}

func (s *RemoveMarkStep) Apply(doc *Node) (*Node, error) {
	old := doc.Slice(s.From, s.To, false)
	content := mapFragment(old.Content, func(node, _ *Node, _ int) *Node { return node.Mark(s.Mark.RemoveFromSet(node.Marks)) }, doc)
	return fromReplace(doc, s.From, s.To, NewSlice(content, old.OpenStart, old.OpenEnd))
}

func (s *RemoveMarkStep) StepMap() *StepMap { return EmptyStepMap }
func (s *RemoveMarkStep) Invert(*Node) Step { return &AddMarkStep{s.From, s.To, s.Mark} }

func (s *RemoveMarkStep) Map(m Mappable) Step {
	from, to := m.MapResult(s.From, 1), m.MapResult(s.To, -1)
	if from.Deleted() && to.Deleted() || from.Pos >= to.Pos {
		return nil
	}
	return &RemoveMarkStep{from.Pos, to.Pos, s.Mark}
}

func (s *RemoveMarkStep) Merge(other Step) Step {
	if o, ok := other.(*RemoveMarkStep); ok && o.Mark.Eq(s.Mark) && s.From <= o.To && s.To >= o.From {
		return &RemoveMarkStep{min(s.From, o.From), max(s.To, o.To), s.Mark}
	}
	return nil
}

// AttrStep sets an attribute of the node at Pos.
type AttrStep struct {
	Pos   int
	Attr  string
	Value any
}

func (s *AttrStep) Apply(doc *Node) (*Node, error) {
	node := doc.NodeAt(s.Pos)
	if node == nil {
		return nil, errors.New("no node at attribute step's position")
	}
	attrs := make(Attrs, len(node.Attrs))
	for k, v := range node.Attrs {
		attrs[k] = v
	}
	attrs[s.Attr] = normalize(s.Value)
	updated := node.Type.Create(attrs, node.Content, node.Marks)
	if node.IsLeaf() {
		return fromReplace(doc, s.Pos, s.Pos+1, NewSlice(FragmentFrom(updated), 0, 0))
	}
	return fromReplace(doc, s.Pos, s.Pos+1, NewSlice(FragmentFrom(updated.Copy(emptyFragment)), 0, 1))
}

func (s *AttrStep) StepMap() *StepMap { return EmptyStepMap }

func (s *AttrStep) Invert(doc *Node) Step {
	return &AttrStep{s.Pos, s.Attr, doc.NodeAt(s.Pos).Attrs[s.Attr]}
}

func (s *AttrStep) Map(m Mappable) Step {
	pos := m.MapResult(s.Pos, 1)
	if pos.DeletedAfter() {
		return nil
	}
	return &AttrStep{pos.Pos, s.Attr, s.Value}
}

func (s *AttrStep) Merge(Step) Step { return nil }
