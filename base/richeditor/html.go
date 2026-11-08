package richeditor

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
)

// WriteHTML writes a node as Tiptap's HTML.
func WriteHTML(n *Node) string {
	var b strings.Builder
	writeHTMLNode(&b, n)
	return b.String()
}

var htmlTags = map[string]string{"paragraph": "p", "blockquote": "blockquote", "bulletList": "ul", "orderedList": "ol", "listItem": "li",
	"tableRow": "tr", "tableCell": "td", "tableHeader": "th", "taskList": "ul"}

var htmlMarks = map[string]string{"bold": "strong", "italic": "em", "strike": "s", "code": "code", "underline": "u", "highlight": "mark", "subscript": "sub", "superscript": "sup"}

func writeHTMLNode(b *strings.Builder, n *Node) {
	if n.IsText() || n.IsInline() {
		for _, m := range n.Marks {
			if m.Type.Name == "link" {
				fmt.Fprintf(b, `<a href="%s">`, html.EscapeString(m.Attrs["href"].(string)))
			} else if tag := htmlMarks[m.Type.Name]; tag != "" {
				b.WriteString("<" + tag + ">")
			}
		}
		switch {
		case n.IsText():
			b.WriteString(html.EscapeString(n.Text))
		case n.Type.Name == "hardBreak":
			b.WriteString("<br>")
		case n.Type.Spec.LeafText != nil:
			fmt.Fprintf(b, `<span data-type="%s" data-id="%s">%s</span>`, n.Type.Name, html.EscapeString(n.AttrStr("id")), html.EscapeString(n.Type.Spec.LeafText(n)))
		}
		for i := len(n.Marks) - 1; i >= 0; i-- {
			m := n.Marks[i]
			if m.Type.Name == "link" {
				b.WriteString("</a>")
			} else if tag := htmlMarks[m.Type.Name]; tag != "" {
				b.WriteString("</" + tag + ">")
			}
		}
		return
	}
	kids := func() {
		for _, c := range n.Content.Content {
			writeHTMLNode(b, c)
		}
	}
	switch n.Type.Name {
	case "heading":
		tag := "h" + strconv.Itoa(max(int(n.AttrNum("level")), 1))
		b.WriteString("<" + tag + ">")
		kids()
		b.WriteString("</" + tag + ">")
	case "codeBlock":
		b.WriteString("<pre><code>" + html.EscapeString(n.TextContent()) + "</code></pre>")
	case "horizontalRule":
		b.WriteString("<hr>")
	case "image":
		fmt.Fprintf(b, `<img src="%s" alt="%s">`, html.EscapeString(n.AttrStr("src")), html.EscapeString(n.AttrStr("alt")))
	case "taskItem":
		fmt.Fprintf(b, `<li data-type="taskItem" data-checked="%v">`, n.Attrs["checked"])
		kids()
		b.WriteString("</li>")
	case "table":
		b.WriteString("<table><tbody>")
		kids()
		b.WriteString("</tbody></table>")
	default:
		if n.Type == n.Type.Schema.TopNode {
			kids()
			return
		}
		tag := htmlTags[n.Type.Name]
		if tag == "" {
			tag = "div"
		}
		b.WriteString("<" + tag)
		if isCellType(n.Type) {
			if c := colspan(n); c > 1 {
				fmt.Fprintf(b, ` colspan="%d"`, c)
			}
			if r := rowspan(n); r > 1 {
				fmt.Fprintf(b, ` rowspan="%d"`, r)
			}
			if w := colwidth(n); w != nil {
				parts := make([]string, len(w))
				for i, v := range w {
					parts[i] = strconv.FormatFloat(v, 'f', -1, 64)
				}
				fmt.Fprintf(b, ` data-colwidth="%s"`, strings.Join(parts, ","))
			}
		}
		if n.Type.Name == "taskList" {
			b.WriteString(` data-type="taskList"`)
		}
		b.WriteString(">")
		kids()
		b.WriteString("</" + tag + ">")
	}
}

// ParseHTML reads HTML into a document of the starter's names, what the
// schema does not have dropped to its text.
func ParseHTML(s *Schema, source string) (*Node, error) {
	root, err := nethtml.Parse(strings.NewReader(source))
	if err != nil {
		return nil, err
	}
	h := &htmlReader{s: s}
	blocks := h.blocks(root, nil)
	if len(blocks) == 0 {
		return s.TopNode.CreateAndFill(nil, nil, nil), nil
	}
	doc := s.TopNode.CreateAndFill(nil, FragmentFrom(blocks...), nil)
	if doc == nil {
		return nil, fmt.Errorf("richeditor: the HTML does not fit the schema")
	}
	return doc, nil
}

type htmlReader struct{ s *Schema }

func attrOf(n *nethtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func (h *htmlReader) make(name string, attrs Attrs, kids ...*Node) *Node {
	t := h.s.Nodes[name]
	if t == nil {
		return nil
	}
	return t.Create(attrs, FragmentFrom(kids...), nil)
}

func isInlineTag(tag string) bool {
	switch tag {
	case "a", "b", "strong", "i", "em", "u", "s", "del", "strike", "code", "mark", "sub", "sup", "span", "br", "font", "abbr", "small", "label":
		return true
	}
	return false
}

func (h *htmlReader) blocks(n *nethtml.Node, marks []*Mark) []*Node {
	var out []*Node
	var run []*Node
	flush := func() {
		if len(run) > 0 {
			blank := true
			for _, r := range run {
				if !r.IsText() || strings.TrimSpace(r.Text) != "" {
					blank = false
				}
			}
			if !blank {
				out = append(out, h.make("paragraph", nil, run...))
			}
			run = nil
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == nethtml.TextNode || c.Type == nethtml.ElementNode && isInlineTag(c.Data) {
			run = append(run, h.inlineOf(c, marks)...)
			continue
		}
		if c.Type != nethtml.ElementNode {
			continue
		}
		flush()
		out = append(out, h.block(c, marks)...)
	}
	flush()
	var kept []*Node
	for _, n := range out {
		if n != nil {
			kept = append(kept, n)
		}
	}
	return kept
}

func (h *htmlReader) block(n *nethtml.Node, marks []*Mark) []*Node {
	switch n.Data {
	case "p":
		return []*Node{h.make("paragraph", nil, h.inline(n, marks)...)}
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return []*Node{h.make("heading", Attrs{"level": int(n.Data[1] - '0')}, h.inline(n, marks)...)}
	case "blockquote":
		return []*Node{h.make("blockquote", nil, h.blocks(n, marks)...)}
	case "pre":
		text := textOf(n)
		var kids []*Node
		if text != "" {
			kids = []*Node{h.s.Text(strings.TrimSuffix(text, "\n"))}
		}
		return []*Node{h.make("codeBlock", nil, kids...)}
	case "hr":
		return []*Node{h.make("horizontalRule", nil)}
	case "img":
		return []*Node{h.make("image", Attrs{"src": attrOf(n, "src"), "alt": attrOf(n, "alt")})}
	case "ul", "ol":
		name := "bulletList"
		item := "listItem"
		if n.Data == "ol" {
			name = "orderedList"
		}
		if attrOf(n, "data-type") == "taskList" {
			name, item = "taskList", "taskItem"
		}
		var items []*Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != nethtml.ElementNode || c.Data != "li" {
				continue
			}
			kids := h.blocks(c, marks)
			if len(kids) == 0 || kids[0].Type.Name != "paragraph" {
				kids = append([]*Node{h.make("paragraph", nil)}, kids...)
			}
			var attrs Attrs
			if item == "taskItem" {
				attrs = Attrs{"checked": attrOf(c, "data-checked") == "true"}
			}
			items = append(items, h.make(item, attrs, kids...))
		}
		return []*Node{h.make(name, nil, items...)}
	case "table":
		var rows []*Node
		var walk func(p *nethtml.Node)
		walk = func(p *nethtml.Node) {
			for c := p.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != nethtml.ElementNode {
					continue
				}
				if c.Data != "tr" {
					walk(c)
					continue
				}
				var cells []*Node
				for d := c.FirstChild; d != nil; d = d.NextSibling {
					if d.Type != nethtml.ElementNode || d.Data != "td" && d.Data != "th" {
						continue
					}
					name := "tableCell"
					if d.Data == "th" {
						name = "tableHeader"
					}
					attrs := Attrs{}
					if v, err := strconv.Atoi(attrOf(d, "colspan")); err == nil && v > 1 {
						attrs["colspan"] = v
					}
					if v, err := strconv.Atoi(attrOf(d, "rowspan")); err == nil && v > 1 {
						attrs["rowspan"] = v
					}
					if w := attrOf(d, "data-colwidth"); w != "" {
						var widths []any
						for _, p := range strings.Split(w, ",") {
							f, _ := strconv.ParseFloat(p, 64)
							widths = append(widths, f)
						}
						attrs["colwidth"] = widths
					}
					kids := h.blocks(d, marks)
					if len(kids) == 0 {
						kids = []*Node{h.make("paragraph", nil)}
					}
					cells = append(cells, h.make(name, attrs, kids...))
				}
				rows = append(rows, h.make("tableRow", nil, cells...))
			}
		}
		walk(n)
		return []*Node{h.make("table", nil, rows...)}
	}
	return h.blocks(n, marks)
}

func textOf(n *nethtml.Node) string {
	if n.Type == nethtml.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textOf(c))
	}
	return b.String()
}

func (h *htmlReader) inline(n *nethtml.Node, marks []*Mark) []*Node {
	var out []*Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, h.inlineOf(c, marks)...)
	}
	return out
}

func (h *htmlReader) inlineOf(n *nethtml.Node, marks []*Mark) []*Node {
	if n.Type == nethtml.TextNode {
		text := strings.Join(strings.Fields(n.Data), " ")
		if strings.TrimSpace(n.Data) != "" {
			if len(n.Data) > 0 && (n.Data[0] == ' ' || n.Data[0] == '\n') {
				text = " " + text
			}
			if last := n.Data[len(n.Data)-1]; last == ' ' || last == '\n' {
				text += " "
			}
		}
		if text == "" {
			return nil
		}
		return []*Node{h.s.Text(text, marks...)}
	}
	if n.Type != nethtml.ElementNode {
		return nil
	}
	name := ""
	var attrs Attrs
	switch n.Data {
	case "b", "strong":
		name = "bold"
	case "i", "em":
		name = "italic"
	case "s", "del", "strike":
		name = "strike"
	case "u":
		name = "underline"
	case "code":
		name = "code"
	case "mark":
		name = "highlight"
	case "sub":
		name = "subscript"
	case "sup":
		name = "superscript"
	case "a":
		name, attrs = "link", Attrs{"href": attrOf(n, "href")}
	case "br":
		if t := h.s.Nodes["hardBreak"]; t != nil {
			return []*Node{t.Create(nil, nil, marks)}
		}
		return nil
	}
	if t := h.s.Marks[name]; t != nil {
		marks = t.Create(attrs).AddToSet(marks)
	}
	return h.inline(n, marks)
}
