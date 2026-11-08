package richeditor

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
)

// NodeJSON is a node in ProseMirror's JSON shape, which Tiptap's documents
// have too.
func NodeJSON(n *Node) map[string]any {
	out := map[string]any{"type": n.Type.Name}
	if len(n.Attrs) > 0 {
		out["attrs"] = maps.Clone(n.Attrs)
	}
	if n.IsText() {
		out["text"] = n.Text
	}
	if n.Content.Size > 0 {
		kids := make([]any, n.ChildCount())
		for i, c := range n.Content.Content {
			kids[i] = NodeJSON(c)
		}
		out["content"] = kids
	}
	if len(n.Marks) > 0 {
		marks := make([]any, len(n.Marks))
		for i, m := range n.Marks {
			mj := map[string]any{"type": m.Type.Name}
			if len(m.Attrs) > 0 {
				mj["attrs"] = maps.Clone(m.Attrs)
			}
			marks[i] = mj
		}
		out["marks"] = marks
	}
	return out
}

// NodeFromJSON reads a node of s, checking only what it needs to.
func NodeFromJSON(s *Schema, v map[string]any) (*Node, error) {
	name, _ := v["type"].(string)
	var marks []*Mark
	if raw, ok := v["marks"].([]any); ok {
		for _, r := range raw {
			mj, _ := r.(map[string]any)
			mn, _ := mj["type"].(string)
			t := s.Marks[mn]
			if t == nil {
				return nil, fmt.Errorf("richeditor: unknown mark type %q", mn)
			}
			attrs, _ := mj["attrs"].(map[string]any)
			if attrs == nil && t.instance == nil {
				return nil, fmt.Errorf("richeditor: mark %q needs attributes", mn)
			}
			m, err := computeAttrs(t.Spec.Attrs, attrs)
			if err != nil {
				return nil, fmt.Errorf("richeditor: %w", err)
			}
			marks = append(marks, &Mark{Type: t, Attrs: m})
		}
	}
	if name == "text" {
		text, _ := v["text"].(string)
		if text == "" {
			return nil, errors.New("richeditor: empty text nodes are not allowed")
		}
		return s.Text(text, marks...), nil
	}
	t := s.Nodes[name]
	if t == nil {
		return nil, fmt.Errorf("richeditor: unknown node type %q", name)
	}
	var kids []*Node
	if raw, ok := v["content"].([]any); ok {
		for _, r := range raw {
			c, _ := r.(map[string]any)
			n, err := NodeFromJSON(s, c)
			if err != nil {
				return nil, err
			}
			kids = append(kids, n)
		}
	}
	attrs, _ := v["attrs"].(map[string]any)
	computed, err := computeAttrs(t.attrs, attrs)
	if err != nil {
		return nil, fmt.Errorf("richeditor: %w", err)
	}
	return &Node{Type: t, Attrs: computed, Content: FragmentFrom(kids...), Marks: markSetFrom(marks)}, nil
}

// DocFromJSONString reads a document from JSON and checks it.
func DocFromJSONString(s *Schema, text string) (*Node, error) {
	var v map[string]any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil, err
	}
	doc, err := NodeFromJSON(s, v)
	if err != nil {
		return nil, err
	}
	return doc, doc.Check()
}

// DocJSONString writes a document as JSON.
func DocJSONString(n *Node) string {
	b, err := json.Marshal(NodeJSON(n))
	if err != nil {
		return ""
	}
	return string(b)
}

// PlainText is a document's text, a blank line between its blocks, a table's
// rows as lines of tab-separated cells.
func PlainText(doc *Node) string {
	var b strings.Builder
	writePlain(&b, doc.Content, "\n\n")
	return b.String()
}

func writePlain(b *strings.Builder, f *Fragment, sep string) {
	for i, n := range f.Content {
		if i > 0 {
			b.WriteString(sep)
		}
		switch {
		case n.IsTextblock():
			b.WriteString(n.Content.TextBetween(0, n.Content.Size, "", ""))
		case tableRole(n.Type) == "table":
			writePlain(b, n.Content, "\n")
		case tableRole(n.Type) == "row":
			writePlain(b, n.Content, "\t")
		case n.IsLeaf():
			if n.Type.Spec.LeafText != nil {
				b.WriteString(n.Type.Spec.LeafText(n))
			}
		case isListType(n.Type):
			writePlain(b, n.Content, "\n")
		default:
			writePlain(b, n.Content, "\n")
		}
	}
}

// SliceToText is what a slice copies as.
func SliceToText(s *Slice) string {
	return PlainText(&Node{Content: s.Content, Type: &NodeType{}})
}

// SliceFromText is pasted text as a slice: lines of the default block, a
// paragraph per run, in code as the text it is.
func SliceFromText(s *Schema, text string, r *ResolvedPos) *Slice {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if r != nil && r.Parent().Type.Spec.Code {
		if text == "" {
			return EmptySlice
		}
		return NewSlice(FragmentFrom(s.Text(text)), 0, 0)
	}
	marks := []*Mark(nil)
	if r != nil {
		marks = r.MarksHere()
	}
	block := s.DefaultBlock()
	var nodes []*Node
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		var content *Fragment
		if line != "" {
			content = FragmentFrom(s.Text(line, marks...))
		}
		nodes = append(nodes, block.Create(nil, content, nil))
	}
	if len(nodes) == 0 {
		return EmptySlice
	}
	return MaxOpen(FragmentFrom(nodes...), false)
}
