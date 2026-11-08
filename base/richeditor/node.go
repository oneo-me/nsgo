package richeditor

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func utf16Len(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c < 0x80, c >= 0xC0 && c < 0xF0:
			n++
		case c >= 0xF0:
			n += 2
		}
	}
	return n
}

// byteAt is the byte of s at UTF-16 offset at; one inside a surrogate pair
// is the character's start.
func byteAt(s string, at int) int {
	if at <= 0 {
		return 0
	}
	n := 0
	for i, r := range s {
		w := utf16.RuneLen(r)
		if w < 0 {
			w = 1
		}
		if n+w > at {
			return i
		}
		n += w
		if n == at {
			return i + utf8.RuneLen(r)
		}
	}
	return len(s)
}

// cut16 is s from UTF-16 offset from to to.
func cut16(s string, from, to int) string {
	return s[byteAt(s, from):byteAt(s, to)]
}

// Node is a node of a document, never changed once made; a text node holds
// Text and the others Content.
type Node struct {
	Type    *NodeType
	Attrs   Attrs
	Content *Fragment
	Marks   []*Mark
	Text    string
	textLen int
}

// NodeSize is the positions it takes.
func (n *Node) NodeSize() int {
	switch {
	case n.IsText():
		return n.textLen
	case n.IsLeaf():
		return 1
	}
	return 2 + n.Content.Size
}

func (n *Node) ChildCount() int        { return n.Content.ChildCount() }
func (n *Node) Child(i int) *Node      { return n.Content.Child(i) }
func (n *Node) MaybeChild(i int) *Node { return n.Content.MaybeChild(i) }
func (n *Node) FirstChild() *Node      { return n.Content.FirstChild() }
func (n *Node) LastChild() *Node       { return n.Content.LastChild() }
func (n *Node) IsText() bool           { return n.Type.IsText() }
func (n *Node) IsBlock() bool          { return n.Type.IsBlock() }
func (n *Node) IsInline() bool         { return n.Type.IsInline() }
func (n *Node) IsTextblock() bool      { return n.Type.IsTextblock() }
func (n *Node) IsLeaf() bool           { return n.Type.IsLeaf() }
func (n *Node) IsAtom() bool           { return n.Type.IsAtom() }
func (n *Node) InlineContent() bool    { return n.Type.InlineContent }

// Attr reads an attribute as a number, 0 when it is none.
func (n *Node) AttrNum(name string) float64 {
	f, _ := n.Attrs[name].(float64)
	return f
}

// AttrStr reads an attribute as a string.
func (n *Node) AttrStr(name string) string {
	s, _ := n.Attrs[name].(string)
	return s
}

// NodesBetween calls f on the nodes from from to to, depth first, skipping
// a node's children when it returns false.
func (n *Node) NodesBetween(from, to int, f func(node *Node, pos int, parent *Node, index int) bool, start int) {
	n.Content.NodesBetween(from, to, f, start, n)
}

// Descendants calls f on every node inside.
func (n *Node) Descendants(f func(node *Node, pos int, parent *Node, index int) bool) {
	n.NodesBetween(0, n.Content.Size, f, 0)
}

// TextContent is its text.
func (n *Node) TextContent() string {
	if n.IsLeaf() && n.Type.Spec.LeafText != nil {
		return n.Type.Spec.LeafText(n)
	}
	return n.TextBetween(0, n.Content.Size, "", "")
}

// TextBetween is the text from from to to, blocks joined by blockSep and
// leaves as leafText or their LeafText.
func (n *Node) TextBetween(from, to int, blockSep, leafText string) string {
	if n.IsText() {
		return cut16(n.Text, from, to)
	}
	return n.Content.TextBetween(from, to, blockSep, leafText)
}

// SameMarkup says two nodes differ at most in content.
func (n *Node) SameMarkup(o *Node) bool { return n.HasMarkup(o.Type, o.Attrs, o.Marks) }

// HasMarkup says the node has that markup.
func (n *Node) HasMarkup(t *NodeType, attrs Attrs, marks []*Mark) bool {
	if attrs == nil {
		attrs = t.defaults
	}
	return n.Type == t && attrsEqual(n.Attrs, attrs) && marksEqual(n.Marks, marks)
}

// Eq says two nodes are equal, whole.
func (n *Node) Eq(o *Node) bool {
	if n == o {
		return true
	}
	if !n.SameMarkup(o) {
		return false
	}
	if n.IsText() {
		return n.Text == o.Text
	}
	return n.Content.Eq(o.Content)
}

// Copy is the node with other content.
func (n *Node) Copy(content *Fragment) *Node {
	if n.IsText() {
		return n
	}
	if content == nil {
		content = emptyFragment
	}
	return &Node{Type: n.Type, Attrs: n.Attrs, Content: content, Marks: n.Marks}
}

// Mark is the node with other marks.
func (n *Node) Mark(marks []*Mark) *Node {
	if marksEqual(marks, n.Marks) {
		return n
	}
	c := *n
	c.Marks = marks
	return &c
}

// WithText is a text node with other text.
func (n *Node) WithText(text string) *Node {
	if text == n.Text {
		return n
	}
	c := *n
	c.Text, c.textLen = text, utf16Len(text)
	return &c
}

// Cut is the node from from to to.
func (n *Node) Cut(from, to int) *Node {
	if n.IsText() {
		if from == 0 && to == n.textLen {
			return n
		}
		return n.WithText(cut16(n.Text, from, to))
	}
	if from == 0 && to == n.Content.Size {
		return n
	}
	return n.Copy(n.Content.Cut(from, to))
}

// Slice is the document from from to to, open as far as the two share no
// parent, or to their deepest shared depth with includeParents.
func (n *Node) Slice(from, to int, includeParents bool) *Slice {
	if from == to {
		return EmptySlice
	}
	f, t := n.Resolve(from), n.Resolve(to)
	depth := 0
	if !includeParents {
		depth = f.SharedDepth(to)
	}
	start := f.Start(depth)
	node := f.Node(depth)
	content := node.Content.Cut(f.Pos-start, t.Pos-start)
	return NewSlice(content, f.Depth-depth, t.Depth-depth)
}

// Replace is the node with from to to replaced by slice.
func (n *Node) Replace(from, to int, slice *Slice) (*Node, error) {
	return replaceDoc(n.Resolve(from), n.Resolve(to), slice)
}

// NodeAt is the node at pos, nil for none.
func (n *Node) NodeAt(pos int) *Node {
	for node := n; ; {
		index, offset := node.Content.findIndex(pos, false)
		node = node.Content.MaybeChild(index)
		if node == nil {
			return nil
		}
		if offset == pos || node.IsText() {
			return node
		}
		pos -= offset + 1
	}
}

// RangeHasMark says a node from from to to has a mark of m.
func (n *Node) RangeHasMark(from, to int, m *MarkType) bool {
	found := false
	if to > from {
		n.NodesBetween(from, to, func(node *Node, _ int, _ *Node, _ int) bool {
			if m.IsInSet(node.Marks) != nil {
				found = true
			}
			return !found
		}, 0)
	}
	return found
}

// CanReplace says replacing children from to to with replacement, from
// start to end of it, keeps the node valid.
func (n *Node) CanReplace(from, to int, replacement *Fragment, start, end int) bool {
	if replacement == nil {
		replacement = emptyFragment
	}
	if end < 0 {
		end = replacement.ChildCount()
	}
	one := n.contentMatchAt(from).MatchFragment(replacement, start, end)
	var two *ContentMatch
	if one != nil {
		two = one.MatchFragment(n.Content, to, -1)
	}
	if two == nil || !two.ValidEnd {
		return false
	}
	for i := start; i < end; i++ {
		if !n.Type.AllowsMarks(replacement.Child(i).Marks) {
			return false
		}
	}
	return true
}

// CanReplaceWith says a node of t in place of children from to to keeps
// the node valid.
func (n *Node) CanReplaceWith(from, to int, t *NodeType, marks []*Mark) bool {
	if marks != nil && !n.Type.AllowsMarks(marks) {
		return false
	}
	start := n.contentMatchAt(from).MatchType(t)
	var end *ContentMatch
	if start != nil {
		end = start.MatchFragment(n.Content, to, -1)
	}
	return end != nil && end.ValidEnd
}

// CanAppend says other's content can follow the node's.
func (n *Node) CanAppend(o *Node) bool {
	if o.Content.Size > 0 {
		return n.CanReplace(n.ChildCount(), n.ChildCount(), o.Content, 0, -1)
	}
	return n.Type.compatibleContent(o.Type)
}

func (n *Node) contentMatchAt(index int) *ContentMatch {
	m := n.Type.ContentMatch.MatchFragment(n.Content, 0, index)
	if m == nil {
		panic("richeditor: called contentMatchAt on a node with invalid content")
	}
	return m
}

// Check says the node and all inside fit the schema.
func (n *Node) Check() error {
	if err := checkAttrs(n.Type.attrs, n.Attrs, "node", n.Type.Name); err != nil {
		return err
	}
	var copy []*Mark
	for _, m := range n.Marks {
		if err := checkAttrs(m.Type.Spec.Attrs, m.Attrs, "mark", m.Type.Name); err != nil {
			return err
		}
		copy = m.AddToSet(copy)
	}
	if !marksEqual(copy, n.Marks) {
		return fmt.Errorf("invalid collection of marks for node %s", n.Type.Name)
	}
	if !n.Type.ValidContent(n.Content) {
		return fmt.Errorf("invalid content for node %s: %s", n.Type.Name, n.Content)
	}
	for _, c := range n.Content.Content {
		if err := c.Check(); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) String() string {
	if n.IsText() {
		s := fmt.Sprintf("%q", n.Text)
		for i := len(n.Marks) - 1; i >= 0; i-- {
			s = n.Marks[i].Type.Name + "(" + s + ")"
		}
		return s
	}
	if n.Content.Size == 0 {
		return n.Type.Name
	}
	return n.Type.Name + "(" + strings.TrimSuffix(strings.TrimPrefix(n.Content.String(), "<"), ">") + ")"
}

// Fragment is a node's children.
type Fragment struct {
	Content []*Node
	Size    int
}

var emptyFragment = &Fragment{}

// EmptyFragment holds nothing.
func EmptyFragment() *Fragment { return emptyFragment }

// FragmentFrom makes a fragment of nodes, joining text with equal marks.
func FragmentFrom(nodes ...*Node) *Fragment {
	if len(nodes) == 0 {
		return emptyFragment
	}
	var joined []*Node
	size := 0
	for i, node := range nodes {
		size += node.NodeSize()
		if i > 0 && node.IsText() && nodes[i-1].SameMarkup(node) {
			if joined == nil {
				joined = slices.Clone(nodes[:i])
			}
			last := joined[len(joined)-1]
			joined[len(joined)-1] = last.WithText(last.Text + node.Text)
		} else if joined != nil {
			joined = append(joined, node)
		}
	}
	if joined == nil {
		joined = nodes
	}
	return &Fragment{Content: joined, Size: size}
}

func (f *Fragment) ChildCount() int { return len(f.Content) }

// Child is its child i.
func (f *Fragment) Child(i int) *Node { return f.Content[i] }

// MaybeChild is its child i, nil past the end.
func (f *Fragment) MaybeChild(i int) *Node {
	if i >= 0 && i < len(f.Content) {
		return f.Content[i]
	}
	return nil
}

func (f *Fragment) FirstChild() *Node { return f.MaybeChild(0) }
func (f *Fragment) LastChild() *Node  { return f.MaybeChild(len(f.Content) - 1) }

// NodesBetween calls fn on the nodes between from and to.
func (f *Fragment) NodesBetween(from, to int, fn func(*Node, int, *Node, int) bool, start int, parent *Node) {
	pos := 0
	for i, child := range f.Content {
		if pos >= to {
			break
		}
		end := pos + child.NodeSize()
		if end > from && fn(child, start+pos, parent, i) && child.Content.Size > 0 {
			s := pos + 1
			child.NodesBetween(max(0, from-s), min(child.Content.Size, to-s), fn, start+s)
		}
		pos = end
	}
}

// Descendants calls fn on every node inside.
func (f *Fragment) Descendants(fn func(*Node, int, *Node, int) bool) {
	f.NodesBetween(0, f.Size, fn, 0, nil)
}

// TextBetween is its text from from to to.
func (f *Fragment) TextBetween(from, to int, blockSep, leafText string) string {
	var text strings.Builder
	first := true
	f.NodesBetween(from, to, func(node *Node, pos int, _ *Node, _ int) bool {
		var part string
		hasPart := true
		switch {
		case node.IsText():
			part = cut16(node.Text, max(from, pos)-pos, to-pos)
		case !node.IsLeaf():
			hasPart = false
		case leafText != "":
			part = leafText
		case node.Type.Spec.LeafText != nil:
			part = node.Type.Spec.LeafText(node)
		}
		if node.IsBlock() && (node.IsLeaf() && part != "" || node.IsTextblock()) && blockSep != "" {
			if first {
				first = false
			} else {
				text.WriteString(blockSep)
			}
		}
		if hasPart {
			text.WriteString(part)
		}
		return true
	}, 0, nil)
	return text.String()
}

// Append is the fragment followed by other.
func (f *Fragment) Append(o *Fragment) *Fragment {
	if o.Size == 0 {
		return f
	}
	if f.Size == 0 {
		return o
	}
	last, first := f.LastChild(), o.FirstChild()
	content := slices.Clone(f.Content)
	i := 0
	if last.IsText() && last.SameMarkup(first) {
		content[len(content)-1] = last.WithText(last.Text + first.Text)
		i = 1
	}
	content = append(content, o.Content[i:]...)
	return &Fragment{Content: content, Size: f.Size + o.Size}
}

// Cut is the fragment from from to to; to past its end when negative.
func (f *Fragment) Cut(from, to int) *Fragment {
	if to < 0 {
		to = f.Size
	}
	if from == 0 && to == f.Size {
		return f
	}
	var result []*Node
	size := 0
	if to > from {
		pos := 0
		for _, child := range f.Content {
			if pos >= to {
				break
			}
			end := pos + child.NodeSize()
			if end > from {
				if pos < from || end > to {
					if child.IsText() {
						child = child.Cut(max(0, from-pos), min(child.textLen, to-pos))
					} else {
						child = child.Cut(max(0, from-pos-1), min(child.Content.Size, to-pos-1))
					}
				}
				result = append(result, child)
				size += child.NodeSize()
			}
			pos = end
		}
	}
	return &Fragment{Content: result, Size: size}
}

// CutByIndex is its children from from to to.
func (f *Fragment) CutByIndex(from, to int) *Fragment {
	if from == to {
		return emptyFragment
	}
	if from == 0 && to == len(f.Content) {
		return f
	}
	content := f.Content[from:to:to]
	size := 0
	for _, c := range content {
		size += c.NodeSize()
	}
	return &Fragment{Content: content, Size: size}
}

// ReplaceChild is the fragment with child i replaced.
func (f *Fragment) ReplaceChild(i int, node *Node) *Fragment {
	current := f.Content[i]
	if current == node {
		return f
	}
	content := slices.Clone(f.Content)
	content[i] = node
	return &Fragment{Content: content, Size: f.Size + node.NodeSize() - current.NodeSize()}
}

// AddToStart is the fragment after node.
func (f *Fragment) AddToStart(node *Node) *Fragment {
	return &Fragment{Content: append([]*Node{node}, f.Content...), Size: f.Size + node.NodeSize()}
}

// AddToEnd is the fragment before node.
func (f *Fragment) AddToEnd(node *Node) *Fragment {
	return &Fragment{Content: append(slices.Clone(f.Content), node), Size: f.Size + node.NodeSize()}
}

// Eq says two fragments are equal.
func (f *Fragment) Eq(o *Fragment) bool {
	return slices.EqualFunc(f.Content, o.Content, (*Node).Eq)
}

// findIndex is the child at pos and its offset; with round, the side
// nearer when pos is inside one.
func (f *Fragment) findIndex(pos int, round bool) (int, int) {
	if pos == 0 {
		return 0, 0
	}
	if pos == f.Size {
		return len(f.Content), pos
	}
	if pos > f.Size || pos < 0 {
		panic(fmt.Sprintf("richeditor: position %d outside of fragment (%s)", pos, f))
	}
	cur := 0
	for i := 0; ; i++ {
		end := cur + f.Child(i).NodeSize()
		if end >= pos {
			if end == pos || round {
				return i + 1, end
			}
			return i, cur
		}
		cur = end
	}
}

func (f *Fragment) String() string {
	parts := make([]string, len(f.Content))
	for i, c := range f.Content {
		parts[i] = c.String()
	}
	return "<" + strings.Join(parts, ", ") + ">"
}

// findDiffStart is the first position where two fragments differ, -1 for
// none.
func findDiffStart(a, b *Fragment, pos int) int {
	for i := 0; ; i++ {
		if i == a.ChildCount() || i == b.ChildCount() {
			if a.ChildCount() == b.ChildCount() {
				return -1
			}
			return pos
		}
		ca, cb := a.Child(i), b.Child(i)
		if ca == cb {
			pos += ca.NodeSize()
			continue
		}
		if !ca.SameMarkup(cb) {
			return pos
		}
		if ca.IsText() && ca.Text != cb.Text {
			ra, rb := utf16.Encode([]rune(ca.Text)), utf16.Encode([]rune(cb.Text))
			j := 0
			for j < len(ra) && j < len(rb) && ra[j] == rb[j] {
				j++
				pos++
			}
			return pos
		}
		if ca.Content.Size > 0 || cb.Content.Size > 0 {
			if inner := findDiffStart(ca.Content, cb.Content, pos+1); inner >= 0 {
				return inner
			}
		}
		pos += ca.NodeSize()
	}
}

// Slice is a piece of a document: content with OpenStart and OpenEnd
// levels open on each side.
type Slice struct {
	Content            *Fragment
	OpenStart, OpenEnd int
}

// EmptySlice holds nothing.
var EmptySlice = &Slice{Content: emptyFragment}

// NewSlice makes a slice.
func NewSlice(content *Fragment, openStart, openEnd int) *Slice {
	return &Slice{Content: content, OpenStart: openStart, OpenEnd: openEnd}
}

// Size is the positions it adds.
func (s *Slice) Size() int { return s.Content.Size - s.OpenStart - s.OpenEnd }

// Eq says two slices are equal.
func (s *Slice) Eq(o *Slice) bool {
	return s.Content.Eq(o.Content) && s.OpenStart == o.OpenStart && s.OpenEnd == o.OpenEnd
}

// MaxOpen is the most-open slice of fragment.
func MaxOpen(f *Fragment, openIsolating bool) *Slice {
	openStart, openEnd := 0, 0
	for n := f.FirstChild(); n != nil && !n.IsLeaf() && (openIsolating || !n.Type.Spec.Isolating); n = n.FirstChild() {
		openStart++
	}
	for n := f.LastChild(); n != nil && !n.IsLeaf() && (openIsolating || !n.Type.Spec.Isolating); n = n.LastChild() {
		openEnd++
	}
	return NewSlice(f, openStart, openEnd)
}

func removeRange(content *Fragment, from, to int) (*Fragment, error) {
	index, offset := content.findIndex(from, false)
	child := content.MaybeChild(index)
	indexTo, offsetTo := content.findIndex(to, false)
	if offset == from || child.IsText() {
		if offsetTo != to && !content.Child(indexTo).IsText() {
			return nil, errors.New("removing non-flat range")
		}
		return content.Cut(0, from).Append(content.Cut(to, -1)), nil
	}
	if index != indexTo {
		return nil, errors.New("removing non-flat range")
	}
	inner, err := removeRange(child.Content, from-offset-1, to-offset-1)
	if err != nil {
		return nil, err
	}
	return content.ReplaceChild(index, child.Copy(inner)), nil
}

func insertInto(content *Fragment, dist int, insert *Fragment, parent *Node) *Fragment {
	index, offset := content.findIndex(dist, false)
	child := content.MaybeChild(index)
	if offset == dist || child.IsText() {
		if parent != nil && !parent.CanReplace(index, index, insert, 0, -1) {
			return nil
		}
		return content.Cut(0, dist).Append(insert).Append(content.Cut(dist, -1))
	}
	inner := insertInto(child.Content, dist-offset-1, insert, nil)
	if inner == nil {
		return nil
	}
	return content.ReplaceChild(index, child.Copy(inner))
}

// InsertAt is the slice with fragment inserted at pos, nil when it fails.
func (s *Slice) InsertAt(pos int, f *Fragment) *Slice {
	content := insertInto(s.Content, pos+s.OpenStart, f, nil)
	if content == nil {
		return nil
	}
	return NewSlice(content, s.OpenStart, s.OpenEnd)
}

// RemoveBetween is the slice without from to to.
func (s *Slice) RemoveBetween(from, to int) (*Slice, error) {
	content, err := removeRange(s.Content, from+s.OpenStart, to+s.OpenStart)
	if err != nil {
		return nil, err
	}
	return NewSlice(content, s.OpenStart, s.OpenEnd), nil
}
