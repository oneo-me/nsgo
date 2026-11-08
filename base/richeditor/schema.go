// Package richeditor is a rich text editor after ProseMirror's architecture
// (NSH's RichEditor, nsh/docs/dev/rich-editor.md): a document whose structure
// a Schema defines, changes as invertible, mappable steps grouped into
// transactions, immutable states, commands and plugins. The kernel depends on
// no view and is tested on its own; View renders its states.
//
// Positions count as ProseMirror's do: a text its UTF-16 code units, a leaf
// node one, a node with content its two boundaries.
//
// The kernel is a port of ProseMirror's (MIT, © Marijn Haverbeke and others)
// and the extension layer follows Tiptap's (MIT, © Tiptap GmbH), whose
// structures, algorithms and names it keeps; see NOTICE.
package richeditor

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// Attrs are a node's or mark's attributes: JSON's values, numbers as
// float64, never changed once made.
type Attrs = map[string]any

// AttrSpec is an attribute: required unless it has a default.
type AttrSpec struct {
	Name       string
	Default    any
	HasDefault bool
	// Validate refuses a value, for documents that come from outside.
	Validate func(any) error
}

// NodeSpec is a type of node.
type NodeSpec struct {
	Name string
	// Content is its content expression ("paragraph+", "inline*",
	// "(heading | paragraph) block*"); a leaf without one.
	Content string
	// Marks are the marks allowed in it, space separated, "_" for every
	// one and "none" for none; unset, an inline container's take every one.
	Marks string
	// Group is the groups it belongs to, space separated.
	Group  string
	Inline bool
	// Atom has a node with content count as one unit from outside.
	Atom bool
	// NoSelect keeps node selections off it.
	NoSelect bool
	// Code has its content kept as code: marks and commands treat it so.
	Code bool
	// Defining keeps a node when its content is replaced, and its content
	// keeps it when pasted; Isolating stops edits at its boundaries.
	Defining, Isolating bool
	// Whitespace "pre" keeps its white space as it is.
	Whitespace string
	Attrs      []AttrSpec
	// LeafText is what a leaf is as text.
	LeafText func(n *Node) string
	// RenderText is what the view shows for an inline leaf, one position
	// however long, a mention's @name.
	RenderText func(n *Node) string
	// Extra is what other parts read: a list's "listMarker", a table's
	// "tableRole".
	Extra map[string]any
}

// MarkSpec is a type of mark.
type MarkSpec struct {
	Name  string
	Attrs []AttrSpec
	// NotInclusive keeps text typed at its end out of it.
	NotInclusive bool
	// Excludes are the marks it cannot coexist with, space separated: its
	// own type when "", every one with "_", none with "none".
	Excludes string
	Group    string
	// RenderText is unused by marks' kernel; Extra is as NodeSpec's.
	Extra map[string]any
}

// SchemaSpec is a schema's types in order: the first block type is the
// default one, and TopNode, "doc" when "", the document's.
type SchemaSpec struct {
	Nodes   []NodeSpec
	Marks   []MarkSpec
	TopNode string
}

// Schema is a document's node and mark types.
type Schema struct {
	Spec    SchemaSpec
	Nodes   map[string]*NodeType
	Marks   map[string]*MarkType
	TopNode *NodeType
	// nodeOrder and markOrder are the types as the spec gives them.
	nodeOrder []*NodeType
	markOrder []*MarkType
}

// NodeType is a type of a schema's nodes.
type NodeType struct {
	Name          string
	Schema        *Schema
	Spec          NodeSpec
	Groups        []string
	ContentMatch  *ContentMatch
	InlineContent bool
	// MarkSet is the marks allowed, nil for every one.
	MarkSet []*MarkType
	attrs   []AttrSpec
	// defaults are the attributes when every one has a default.
	defaults Attrs
}

// MarkType is a type of a schema's marks.
type MarkType struct {
	Name     string
	Schema   *Schema
	Spec     MarkSpec
	Rank     int
	excluded []*MarkType
	instance *Mark
}

func hasDefaults(specs []AttrSpec) bool {
	return !slices.ContainsFunc(specs, func(a AttrSpec) bool { return !a.HasDefault })
}

// normalize makes a value JSON's: numbers float64, so values compare.
func normalize(v any) any {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case []any:
		out := make([]any, len(n))
		for i := range n {
			out[i] = normalize(n[i])
		}
		return out
	case []float64:
		out := make([]any, len(n))
		for i := range n {
			out[i] = n[i]
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(n))
		for k, e := range n {
			out[k] = normalize(e)
		}
		return out
	}
	return v
}

func computeAttrs(specs []AttrSpec, given Attrs) (Attrs, error) {
	out := make(Attrs, len(specs))
	for _, a := range specs {
		v, ok := given[a.Name]
		switch {
		case ok:
			v = normalize(v)
		case a.HasDefault:
			v = normalize(a.Default)
		default:
			return nil, fmt.Errorf("no value supplied for attribute %s", a.Name)
		}
		out[a.Name] = v
	}
	return out, nil
}

func checkAttrs(specs []AttrSpec, values Attrs, kind, name string) error {
	for k := range values {
		if !slices.ContainsFunc(specs, func(a AttrSpec) bool { return a.Name == k }) {
			return fmt.Errorf("unsupported attribute %s for %s of type %s", k, kind, name)
		}
	}
	for _, a := range specs {
		if a.Validate != nil {
			if err := a.Validate(values[a.Name]); err != nil {
				return fmt.Errorf("invalid value for attribute %s of %s %s: %w", a.Name, kind, name, err)
			}
		}
	}
	return nil
}

// NewSchema makes a schema, refusing what cannot make one: a content
// expression that does not read or requires a type no document can fill.
func NewSchema(spec SchemaSpec) (*Schema, error) {
	s := &Schema{Spec: spec, Nodes: map[string]*NodeType{}, Marks: map[string]*MarkType{}}
	for _, n := range spec.Nodes {
		if _, ok := s.Nodes[n.Name]; ok {
			return nil, fmt.Errorf("node type %s defined twice", n.Name)
		}
		t := &NodeType{Name: n.Name, Schema: s, Spec: n, Groups: strings.Fields(n.Group), attrs: n.Attrs}
		if hasDefaults(n.Attrs) {
			t.defaults, _ = computeAttrs(n.Attrs, nil)
		}
		s.Nodes[n.Name] = t
		s.nodeOrder = append(s.nodeOrder, t)
	}
	top := cmpOr(spec.TopNode, "doc")
	if s.Nodes[top] == nil {
		return nil, fmt.Errorf("schema is missing its top node type %q", top)
	}
	if s.Nodes["text"] == nil {
		return nil, errors.New("every schema needs a 'text' type")
	}
	if len(s.Nodes["text"].attrs) > 0 {
		return nil, errors.New("the text node type should not have attributes")
	}
	for i, m := range spec.Marks {
		if _, ok := s.Marks[m.Name]; ok {
			return nil, fmt.Errorf("mark type %s defined twice", m.Name)
		}
		t := &MarkType{Name: m.Name, Schema: s, Spec: m, Rank: i}
		if hasDefaults(m.Attrs) {
			attrs, _ := computeAttrs(m.Attrs, nil)
			t.instance = &Mark{Type: t, Attrs: attrs}
		}
		s.Marks[m.Name] = t
		s.markOrder = append(s.markOrder, t)
	}
	cache := map[string]*ContentMatch{}
	for _, t := range s.nodeOrder {
		match, ok := cache[t.Spec.Content]
		if !ok {
			var err error
			if match, err = parseContentMatch(t.Spec.Content, s.Nodes); err != nil {
				return nil, fmt.Errorf("%s: %w", t.Name, err)
			}
			cache[t.Spec.Content] = match
		}
		t.ContentMatch = match
		t.InlineContent = match.inlineContent()
		switch {
		case t.Spec.Marks == "_", t.Spec.Marks == "" && t.InlineContent:
		case t.Spec.Marks == "none", t.Spec.Marks == "":
			t.MarkSet = []*MarkType{}
		default:
			set, err := s.gatherMarks(strings.Fields(t.Spec.Marks))
			if err != nil {
				return nil, err
			}
			t.MarkSet = set
		}
	}
	for _, m := range s.markOrder {
		switch m.Spec.Excludes {
		case "":
			m.excluded = []*MarkType{m}
		case "none":
		default:
			set, err := s.gatherMarks(strings.Fields(m.Spec.Excludes))
			if err != nil {
				return nil, err
			}
			m.excluded = set
		}
	}
	s.TopNode = s.Nodes[top]
	return s, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (s *Schema) gatherMarks(names []string) ([]*MarkType, error) {
	var out []*MarkType
	for _, name := range names {
		if m := s.Marks[name]; m != nil {
			out = append(out, m)
			continue
		}
		found := false
		for _, m := range s.markOrder {
			if name == "_" || slices.Contains(strings.Fields(m.Spec.Group), name) {
				out, found = append(out, m), true
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown mark type: %q", name)
		}
	}
	return out, nil
}

// Text makes a text node, never empty.
func (s *Schema) Text(text string, marks ...*Mark) *Node {
	t := s.Nodes["text"]
	return &Node{Type: t, Attrs: t.defaults, Content: emptyFragment, Marks: markSetFrom(marks), Text: text, textLen: utf16Len(text)}
}

// Node makes a node of type name, panicking on an unknown one; attributes
// fill with their defaults. The schema's content rules are Check's.
func (s *Schema) Node(name string, attrs Attrs, children ...*Node) *Node {
	t := s.Nodes[name]
	if t == nil {
		panic("richeditor: unknown node type " + name)
	}
	return t.Create(attrs, FragmentFrom(children...), nil)
}

// Mark makes a mark of type name.
func (s *Schema) Mark(name string, attrs Attrs) *Mark {
	t := s.Marks[name]
	if t == nil {
		panic("richeditor: unknown mark type " + name)
	}
	return t.Create(attrs)
}

// DefaultBlock is the first block type in the top node's content.
func (s *Schema) DefaultBlock() *NodeType {
	return s.TopNode.ContentMatch.defaultType()
}

// IsBlock, IsText, IsInline, IsLeaf, IsTextblock and IsAtom say what a
// type's nodes are.
func (t *NodeType) IsBlock() bool     { return !t.Spec.Inline && t.Name != "text" }
func (t *NodeType) IsInline() bool    { return !t.IsBlock() }
func (t *NodeType) IsText() bool      { return t.Name == "text" }
func (t *NodeType) IsTextblock() bool { return t.IsBlock() && t.InlineContent }
func (t *NodeType) IsLeaf() bool      { return t.ContentMatch == emptyMatch }
func (t *NodeType) IsAtom() bool      { return t.IsLeaf() || t.Spec.Atom }

func (t *NodeType) inGroup(name string) bool { return slices.Contains(t.Groups, name) }

func (t *NodeType) hasRequiredAttrs() bool {
	return slices.ContainsFunc(t.attrs, func(a AttrSpec) bool { return !a.HasDefault })
}

func (t *NodeType) compatibleContent(other *NodeType) bool {
	return t == other || t.ContentMatch.compatible(other.ContentMatch)
}

func (t *NodeType) computeAttrs(attrs Attrs) Attrs {
	if attrs == nil && t.defaults != nil {
		return t.defaults
	}
	out, err := computeAttrs(t.attrs, attrs)
	if err != nil {
		panic("richeditor: " + err.Error())
	}
	return out
}

// Create makes a node without checking its content.
func (t *NodeType) Create(attrs Attrs, content *Fragment, marks []*Mark) *Node {
	if t.IsText() {
		panic("richeditor: NodeType.Create can't construct text nodes")
	}
	if content == nil {
		content = emptyFragment
	}
	return &Node{Type: t, Attrs: t.computeAttrs(attrs), Content: content, Marks: markSetFrom(marks)}
}

// CreateAndFill makes a node filling what its content requires around
// content, nil when it cannot.
func (t *NodeType) CreateAndFill(attrs Attrs, content *Fragment, marks []*Mark) *Node {
	attrs = t.computeAttrs(attrs)
	if content == nil {
		content = emptyFragment
	}
	if content.Size > 0 {
		before := t.ContentMatch.FillBefore(content, false, 0)
		if before == nil {
			return nil
		}
		content = before.Append(content)
	}
	matched := t.ContentMatch.MatchFragment(content, 0, -1)
	if matched == nil {
		return nil
	}
	after := matched.FillBefore(emptyFragment, true, 0)
	if after == nil {
		return nil
	}
	return &Node{Type: t, Attrs: attrs, Content: content.Append(after), Marks: markSetFrom(marks)}
}

// ValidContent says content fits the type.
func (t *NodeType) ValidContent(content *Fragment) bool {
	result := t.ContentMatch.MatchFragment(content, 0, -1)
	if result == nil || !result.ValidEnd {
		return false
	}
	for _, c := range content.Content {
		if !t.AllowsMarks(c.Marks) {
			return false
		}
	}
	return true
}

// AllowsMarkType says the type's content may hold marks of m.
func (t *NodeType) AllowsMarkType(m *MarkType) bool {
	return t.MarkSet == nil || slices.Contains(t.MarkSet, m)
}

// AllowsMarks says it may hold all of marks.
func (t *NodeType) AllowsMarks(marks []*Mark) bool {
	if t.MarkSet == nil {
		return true
	}
	for _, m := range marks {
		if !t.AllowsMarkType(m.Type) {
			return false
		}
	}
	return true
}

// allowedMarks are marks without those the type refuses.
func (t *NodeType) allowedMarks(marks []*Mark) []*Mark {
	if t.MarkSet == nil {
		return marks
	}
	var out []*Mark
	for _, m := range marks {
		if t.AllowsMarkType(m.Type) {
			out = append(out, m)
		}
	}
	return out
}

// Create makes a mark of the type.
func (t *MarkType) Create(attrs Attrs) *Mark {
	if attrs == nil && t.instance != nil {
		return t.instance
	}
	out, err := computeAttrs(t.Spec.Attrs, attrs)
	if err != nil {
		panic("richeditor: " + err.Error())
	}
	return &Mark{Type: t, Attrs: out}
}

// Excludes says marks of the type exclude marks of other.
func (t *MarkType) Excludes(other *MarkType) bool { return slices.Contains(t.excluded, other) }

// RemoveFromSet is set without marks of the type.
func (t *MarkType) RemoveFromSet(set []*Mark) []*Mark {
	return slices.DeleteFunc(slices.Clone(set), func(m *Mark) bool { return m.Type == t })
}

// IsInSet is the mark of the type in set, nil for none.
func (t *MarkType) IsInSet(set []*Mark) *Mark {
	for _, m := range set {
		if m.Type == t {
			return m
		}
	}
	return nil
}

// Mark is a mark on inline content.
type Mark struct {
	Type  *MarkType
	Attrs Attrs
}

// AddToSet is set with the mark, in rank order, without what it excludes;
// set itself when a mark there excludes it.
func (m *Mark) AddToSet(set []*Mark) []*Mark {
	var out []*Mark
	placed := false
	for i, other := range set {
		if m.Eq(other) {
			return set
		}
		if m.Type.Excludes(other.Type) {
			if out == nil {
				out = slices.Clone(set[:i])
			}
			continue
		}
		if other.Type.Excludes(m.Type) {
			return set
		}
		if !placed && other.Type.Rank > m.Type.Rank {
			if out == nil {
				out = slices.Clone(set[:i])
			}
			out = append(out, m)
			placed = true
		}
		if out != nil {
			out = append(out, other)
		}
	}
	if out == nil {
		out = slices.Clone(set)
	}
	if !placed {
		out = append(out, m)
	}
	return out
}

// RemoveFromSet is set without the mark.
func (m *Mark) RemoveFromSet(set []*Mark) []*Mark {
	for i, other := range set {
		if m.Eq(other) {
			return slices.Delete(slices.Clone(set), i, i+1)
		}
	}
	return set
}

// IsInSet says set holds the mark.
func (m *Mark) IsInSet(set []*Mark) bool {
	return slices.ContainsFunc(set, m.Eq)
}

// Eq says two marks are of one type with equal attributes.
func (m *Mark) Eq(o *Mark) bool { return m == o || m.Type == o.Type && attrsEqual(m.Attrs, o.Attrs) }

func marksEqual(a, b []*Mark) bool {
	return slices.EqualFunc(a, b, (*Mark).Eq)
}

func markSetFrom(marks []*Mark) []*Mark {
	if len(marks) < 2 {
		return marks
	}
	var set []*Mark
	for _, m := range marks {
		set = m.AddToSet(set)
	}
	return set
}

func attrsEqual(a, b Attrs) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return maps.EqualFunc(a, b, func(x, y any) bool { return reflect.DeepEqual(x, y) })
}
