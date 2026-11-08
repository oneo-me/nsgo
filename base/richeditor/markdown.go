package richeditor

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

var markdownParser = goldmark.New(goldmark.WithExtensions(extension.GFM))

// ParseMarkdown reads CommonMark and GFM into a document of the starter's
// names, keeping what the schema has: task items as taskItem, pipe tables as
// tables, the first row header cells.
func ParseMarkdown(s *Schema, source string) (*Node, error) {
	src := []byte(source)
	root := markdownParser.Parser().Parse(text.NewReader(src))
	m := &mdReader{s: s, src: src}
	nodes := m.blocks(root)
	if len(nodes) == 0 {
		return s.TopNode.CreateAndFill(nil, nil, nil), nil
	}
	doc := s.TopNode.CreateAndFill(nil, FragmentFrom(nodes...), nil)
	if doc == nil {
		return nil, fmt.Errorf("richeditor: the Markdown does not fit the schema")
	}
	return doc, nil
}

type mdReader struct {
	s   *Schema
	src []byte
}

func (m *mdReader) block(name string, attrs Attrs, kids ...*Node) *Node {
	t := m.s.Nodes[name]
	if t == nil {
		return nil
	}
	return t.Create(attrs, FragmentFrom(kids...), nil)
}

func (m *mdReader) paragraph(kids []*Node) *Node { return m.block("paragraph", nil, kids...) }

func (m *mdReader) blocks(parent ast.Node) []*Node {
	var out []*Node
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.Paragraph, *ast.TextBlock:
			out = append(out, m.splitBlocks(m.inline(c, nil))...)
		case *ast.Heading:
			out = append(out, m.block("heading", Attrs{"level": n.Level}, m.inline(c, nil)...))
		case *ast.Blockquote:
			out = append(out, m.block("blockquote", nil, m.blocks(c)...))
		case *ast.FencedCodeBlock:
			var attrs Attrs
			if lang := n.Language(m.src); len(lang) > 0 {
				attrs = Attrs{"language": string(lang)}
			}
			out = append(out, m.code(c, attrs))
		case *ast.CodeBlock:
			out = append(out, m.code(c, nil))
		case *ast.ThematicBreak:
			out = append(out, m.block("horizontalRule", nil))
		case *ast.HTMLBlock:
			var b bytes.Buffer
			for i := 0; i < n.Lines().Len(); i++ {
				l := n.Lines().At(i)
				b.Write(l.Value(m.src))
			}
			if n.HasClosure() {
				b.Write(n.ClosureLine.Value(m.src))
			}
			if strings.HasPrefix(strings.TrimSpace(b.String()), "<table") {
				if doc, err := ParseHTML(m.s, b.String()); err == nil {
					out = append(out, doc.Content.Content...)
				}
			}
		case *ast.List:
			out = append(out, m.list(n)...)
		case *east.Table:
			out = append(out, m.table(n))
		}
	}
	var kept []*Node
	for _, n := range out {
		if n != nil {
			kept = append(kept, n)
		}
	}
	return kept
}

// splitBlocks makes a paragraph of inline content, a block among it, such
// as the starter's images, breaking it.
func (m *mdReader) splitBlocks(inline []*Node) []*Node {
	var out, run []*Node
	flush := func() {
		if len(run) > 0 {
			out = append(out, m.paragraph(run))
			run = nil
		}
	}
	for _, n := range inline {
		if n.IsBlock() {
			flush()
			out = append(out, n)
		} else {
			run = append(run, n)
		}
	}
	flush()
	if len(out) == 0 {
		out = []*Node{m.paragraph(nil)}
	}
	return out
}

func (m *mdReader) code(n ast.Node, attrs Attrs) *Node {
	var b bytes.Buffer
	for i := 0; i < n.Lines().Len(); i++ {
		l := n.Lines().At(i)
		b.Write(l.Value(m.src))
	}
	body := strings.TrimSuffix(b.String(), "\n")
	var kids []*Node
	if body != "" {
		kids = []*Node{m.s.Text(body)}
	}
	return m.block("codeBlock", attrs, kids...)
}

// isTaskItem says a list item starts with a check box.
func isTaskItem(item ast.Node) (checked, ok bool) {
	first := item.FirstChild()
	if first == nil {
		return false, false
	}
	if box, is := first.FirstChild().(*east.TaskCheckBox); is {
		return box.IsChecked, true
	}
	return false, false
}

// list is a run of a list's items, a run of task items splitting off as a
// task list.
func (m *mdReader) list(l *ast.List) []*Node {
	var out []*Node
	var run []*Node
	task := false
	flush := func() {
		if len(run) == 0 {
			return
		}
		switch {
		case task:
			out = append(out, m.block("taskList", nil, run...))
		case l.IsOrdered():
			var attrs Attrs
			if l.Start > 1 {
				attrs = Attrs{"start": l.Start}
			}
			out = append(out, m.block("orderedList", attrs, run...))
		default:
			out = append(out, m.block("bulletList", nil, run...))
		}
		run = nil
	}
	for c := l.FirstChild(); c != nil; c = c.NextSibling() {
		checked, isTask := isTaskItem(c)
		isTask = isTask && m.s.Nodes["taskItem"] != nil
		if isTask != task {
			flush()
			task = isTask
		}
		kids := m.blocks(c)
		if len(kids) == 0 || kids[0].Type.Name != "paragraph" {
			kids = append([]*Node{m.paragraph(nil)}, kids...)
		}
		if isTask {
			run = append(run, m.block("taskItem", Attrs{"checked": checked}, kids...))
		} else {
			run = append(run, m.block("listItem", nil, kids...))
		}
	}
	flush()
	return out
}

func (m *mdReader) table(t *east.Table) *Node {
	width := 0
	for r := t.FirstChild(); r != nil; r = r.NextSibling() {
		width = max(width, r.ChildCount())
	}
	var rows []*Node
	for r := t.FirstChild(); r != nil; r = r.NextSibling() {
		_, header := r.(*east.TableHeader)
		name := "tableCell"
		if header {
			name = "tableHeader"
		}
		var cells []*Node
		for c := r.FirstChild(); c != nil; c = c.NextSibling() {
			cells = append(cells, m.block(name, nil, m.paragraph(m.inline(c, nil))))
		}
		for len(cells) < width {
			cells = append(cells, m.block(name, nil, m.paragraph(nil)))
		}
		rows = append(rows, m.block("tableRow", nil, cells...))
	}
	return m.block("table", nil, rows...)
}

func (m *mdReader) mark(name string, attrs Attrs, marks []*Mark) []*Mark {
	if t := m.s.Marks[name]; t != nil {
		return t.Create(attrs).AddToSet(marks)
	}
	return marks
}

func (m *mdReader) inline(parent ast.Node, marks []*Mark) []*Node {
	var out []*Node
	add := func(text string) {
		if text != "" {
			out = append(out, m.s.Text(text, marks...))
		}
	}
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.Text:
			add(string(n.Value(m.src)))
			if n.HardLineBreak() {
				if t := m.s.Nodes["hardBreak"]; t != nil {
					out = append(out, t.Create(nil, nil, marks))
				}
			} else if n.SoftLineBreak() {
				add(" ")
			}
		case *ast.String:
			add(string(n.Value))
		case *ast.Emphasis:
			name := "italic"
			if n.Level == 2 {
				name = "bold"
			}
			out = append(out, m.inline(c, m.mark(name, nil, marks))...)
		case *east.Strikethrough:
			out = append(out, m.inline(c, m.mark("strike", nil, marks))...)
		case *ast.CodeSpan:
			var b bytes.Buffer
			for k := c.FirstChild(); k != nil; k = k.NextSibling() {
				if t, ok := k.(*ast.Text); ok {
					b.Write(t.Value(m.src))
				}
			}
			if b.Len() > 0 {
				out = append(out, m.s.Text(b.String(), m.mark("code", nil, marks)...))
			}
		case *ast.Link:
			out = append(out, m.inline(c, m.mark("link", Attrs{"href": string(n.Destination)}, marks))...)
		case *ast.AutoLink:
			label := string(n.Label(m.src))
			out = append(out, m.s.Text(label, m.mark("link", Attrs{"href": string(n.URL(m.src))}, marks)...))
		case *ast.Image:
			t := m.s.Nodes["image"]
			if t == nil {
				continue
			}
			var alt bytes.Buffer
			for k := c.FirstChild(); k != nil; k = k.NextSibling() {
				if tx, ok := k.(*ast.Text); ok {
					alt.Write(tx.Value(m.src))
				}
			}
			out = append(out, t.Create(Attrs{"src": string(n.Destination), "alt": alt.String(), "title": nilIfEmpty(string(n.Title))}, nil, nil))
		case *east.TaskCheckBox:
		}
	}
	return out
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// WriteMarkdown writes a document as Markdown: GFM's task items and pipe
// tables, and a table a pipe table cannot hold as its HTML on one line,
// which reads back as it was.
func WriteMarkdown(doc *Node) string {
	var b strings.Builder
	writeBlocks(&b, doc.Content, "")
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeBlocks(b *strings.Builder, f *Fragment, indent string) {
	for i, n := range f.Content {
		if i > 0 {
			b.WriteString(indent + "\n")
		}
		writeBlock(b, n, indent)
	}
}

func writeBlock(b *strings.Builder, n *Node, indent string) {
	switch n.Type.Name {
	case "heading":
		b.WriteString(indent + strings.Repeat("#", max(int(n.AttrNum("level")), 1)) + " " + inlineMarkdown(n) + "\n")
	case "blockquote":
		var inner strings.Builder
		writeBlocks(&inner, n.Content, "")
		for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
			b.WriteString(strings.TrimRight(indent+"> "+line, " ") + "\n")
		}
	case "codeBlock":
		b.WriteString(indent + "```" + n.AttrStr("language") + "\n")
		for _, line := range strings.Split(n.TextContent(), "\n") {
			b.WriteString(indent + line + "\n")
		}
		b.WriteString(indent + "```\n")
	case "horizontalRule":
		b.WriteString(indent + "---\n")
	case "image":
		b.WriteString(indent + "![" + n.AttrStr("alt") + "](" + n.AttrStr("src") + ")\n")
	case "bulletList", "orderedList", "taskList":
		for i, item := range n.Content.Content {
			prefix := "- "
			switch n.Type.Name {
			case "orderedList":
				prefix = strconv.Itoa(max(int(n.AttrNum("start")), 1)+i) + ". "
			case "taskList":
				prefix = "- [ ] "
				if c, _ := item.Attrs["checked"].(bool); c {
					prefix = "- [x] "
				}
			}
			var inner strings.Builder
			for j, c := range item.Content.Content {
				// A list right under the item's text keeps it tight.
				if j > 0 && !isListType(c.Type) {
					inner.WriteString("\n")
				}
				writeBlock(&inner, c, "")
			}
			pad := strings.Repeat(" ", len(prefix))
			for j, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
				if j == 0 {
					b.WriteString(indent + prefix + line + "\n")
				} else if line == "" {
					b.WriteString("\n")
				} else {
					b.WriteString(indent + pad + line + "\n")
				}
			}
		}
	case "table":
		writeTable(b, n, indent)
	default:
		if n.IsTextblock() {
			b.WriteString(indent + inlineMarkdown(n) + "\n")
		}
	}
}

func writeTable(b *strings.Builder, n *Node, indent string) {
	pipe := n.ChildCount() > 0
	for r, row := range n.Content.Content {
		for _, cell := range row.Content.Content {
			simple := colspan(cell) == 1 && rowspan(cell) == 1 && cell.ChildCount() == 1 && cell.FirstChild().Type.Name == "paragraph"
			if !simple || (r == 0) != (tableRole(cell.Type) == "header_cell") {
				pipe = false
			}
		}
	}
	if !pipe {
		b.WriteString(indent + strings.ReplaceAll(WriteHTML(n), "\n", "") + "\n")
		return
	}
	line := func(cells []string) { b.WriteString(indent + "| " + strings.Join(cells, " | ") + " |\n") }
	for r, row := range n.Content.Content {
		cells := make([]string, row.ChildCount())
		for i, cell := range row.Content.Content {
			cells[i] = strings.ReplaceAll(inlineMarkdown(cell.FirstChild()), "|", `\|`)
		}
		line(cells)
		if r == 0 {
			rule := make([]string, len(cells))
			for i := range rule {
				rule[i] = "---"
			}
			line(rule)
		}
	}
}

var markFences = map[string][2]string{"bold": {"**", "**"}, "italic": {"*", "*"}, "strike": {"~~", "~~"}, "code": {"`", "`"}}

func escapeMarkdown(s string) string {
	return strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `[`, `\[`, `~`, `\~`).Replace(s)
}

// inlineMarkdown is a textblock's marks as fences that stay open across
// nodes sharing them.
func inlineMarkdown(n *Node) string {
	var b strings.Builder
	var open []*Mark
	closeTo := func(keep int) {
		for i := len(open) - 1; i >= keep; i-- {
			m := open[i]
			if m.Type.Name == "link" {
				b.WriteString("](" + m.Attrs["href"].(string) + ")")
			} else {
				b.WriteString(markFences[m.Type.Name][1])
			}
		}
		open = open[:keep]
	}
	for _, c := range n.Content.Content {
		var want []*Mark
		for _, m := range c.Marks {
			if _, ok := markFences[m.Type.Name]; ok || m.Type.Name == "link" {
				want = append(want, m)
			}
		}
		keep := 0
		for keep < len(open) && keep < len(want) && open[keep].Eq(want[keep]) {
			keep++
		}
		closeTo(keep)
		for _, m := range want[keep:] {
			if m.Type.Name == "link" {
				b.WriteString("[")
			} else {
				b.WriteString(markFences[m.Type.Name][0])
			}
			open = append(open, m)
		}
		switch {
		case c.IsText():
			if c.Type.Schema.Marks["code"] != nil && c.Type.Schema.Marks["code"].IsInSet(c.Marks) != nil {
				b.WriteString(c.Text)
			} else {
				b.WriteString(escapeMarkdown(c.Text))
			}
		case c.Type.Name == "hardBreak":
			b.WriteString("  \n")
		case c.Type.Spec.LeafText != nil:
			b.WriteString(c.Type.Spec.LeafText(c))
		}
	}
	closeTo(0)
	return b.String()
}
