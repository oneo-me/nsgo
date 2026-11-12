package textview

// HTML into a Document: fragments (a model's reply, HTML inside Markdown)
// and whole pages (their body, as a reader mode shows it).
//
// x/net/html builds the tree as a browser does, unclosed and misplaced
// tags included; the reader then walks it in one of two contexts. In
// block context each element is a block of the model, or a container
// whose children are read in block context (a <div>, a <section>, an
// unknown element holding blocks), and runs of inline content between
// blocks become paragraphs. In inline context elements are inline
// content, and blocks met there (<span><p>…</p></span>) are their content
// on lines of their own. Whitespace collapses as a browser collapses it,
// except in <pre>.

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/nsgo-dev/nsgo/core"
)

const (
	// maxDepth bounds the elements read: the tree is walked recursively,
	// and malformed or hostile input may nest without bound.
	maxDepth = 256
	// maxColSpan bounds the columns a cell spans.
	maxColSpan = 64
)

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

var (
	// blockElements are read in block context, and make an unknown
	// element holding them a container.
	blockElements = set("address", "article", "aside", "blockquote", "center", "dd", "details", "dialog", "dir",
		"div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6",
		"header", "hgroup", "hr", "legend", "li", "main", "menu", "nav", "ol", "p", "pre", "search", "section",
		"summary", "table", "ul")
	// droppedElements never show their content: metadata, scripts, form
	// controls and embedded objects with no text of their own.
	droppedElements = set("audio", "base", "canvas", "colgroup", "datalist", "embed", "head", "iframe", "input",
		"link", "meta", "noscript", "object", "option", "param", "script", "select", "source", "style", "svg",
		"template", "textarea", "title", "track", "video")
	// voidElements have no closing tag.
	voidElements = set("area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source",
		"track", "wbr")
	// formattingElements hold Markdown between their tags in a Markdown
	// paragraph, as gpui-kit pairs them: <strong>b *c*</strong>.
	formattingElements = set("a", "b", "cite", "del", "dfn", "em", "i", "ins", "mark", "s", "small", "strike",
		"strong", "sub", "sup", "u", "var")
)

// ParseHTML reads HTML into a document: a fragment, or a whole page's
// body. Elements a reader does not show are skipped (metadata, scripts,
// styles, form controls, embedded media, and elements hidden, aria-hidden
// or styled display: none, such as KaTeX's HTML beside its MathML), and
// GitHub's own markup reads back as what it writes: alerts, task list
// boxes, code languages and formulas with a TeX annotation. Extensions
// with a Tag make custom nodes of their elements; patterns apply to text.
func ParseHTML(source string, extensions ...Extension) Document {
	return Document{Blocks: htmlBlocks(source, extensions)}
}

// htmlBlocks reads an HTML fragment as blocks, patterns applied.
func htmlBlocks(source string, ext []Extension) []Block {
	return newHTMLReader(ext).blocksOf(htmlBody(source, ext), 0, nil)
}

// htmlInlines reads an HTML fragment as inline content, part of a longer
// text: a space at either end stays, and patterns are left to the caller,
// which applies them to the whole text.
func htmlInlines(source string, ext []Extension) []Inline {
	body := htmlBody(source, ext)
	out := newHTMLReader(ext).inlineChildren(body, nil, 0)
	// A block at the end leaves the break that would end its line.
	last := body.LastChild
	for last != nil && isBlank(last) {
		last = last.PrevSibling
	}
	if last != nil && last.Type == html.ElementNode && blockElements[last.Data] && len(out) > 0 {
		if _, ok := out[len(out)-1].(Break); ok {
			out = out[:len(out)-1]
		}
	}
	// The parser drops whitespace before the body's first content.
	ws := whitespace{pending: source != "" && isHTMLSpace(rune(source[0]))}
	out = collapse(out, &ws)
	if ws.pending {
		out = pushText(out, " ")
	}
	return out
}

// htmlBody parses source and returns its body, or the document if it has
// none.
func htmlBody(source string, ext []Extension) *html.Node {
	doc, err := html.Parse(strings.NewReader(closeExtensionTags(source, ext)))
	if err != nil {
		return &html.Node{Type: html.DocumentNode}
	}
	for n := doc.FirstChild; n != nil; n = n.NextSibling {
		if isElement(n, "html") {
			for b := n.FirstChild; b != nil; b = b.NextSibling {
				if isElement(b, "body") {
					return b
				}
			}
		}
	}
	return doc
}

// closeExtensionTags writes <tag … /> of the extensions' tags as
// <tag …></tag>: HTML ignores the slash of a non-void element, which
// would then hold everything after it, while extension tags are written
// as JSX writes them.
func closeExtensionTags(source string, ext []Extension) string {
	if !strings.Contains(source, "/>") {
		return source
	}
	for _, e := range ext {
		if e.Tag == anyTag {
			// Every element, as MDX writes them, but HTML's void ones.
			source = anySelfClosing.ReplaceAllStringFunc(source, func(m string) string {
				sub := anySelfClosing.FindStringSubmatch(m)
				if voidElements[strings.ToLower(sub[1])] {
					return m
				}
				return "<" + sub[1] + sub[2] + "></" + sub[1] + ">"
			})
			continue
		}
		if e.Tag == "" || voidElements[strings.ToLower(e.Tag)] {
			continue
		}
		re := regexp.MustCompile(`(?i)<(` + regexp.QuoteMeta(e.Tag) + `)((?:\s[^<>]*?)?)\s*/>`)
		source = re.ReplaceAllString(source, "<$1$2></$1>")
	}
	return source
}

var anySelfClosing = regexp.MustCompile(`<([A-Za-z][\w.:-]*)((?:\s[^<>]*?)?)\s*/>`)

// htmlReader is one walk of a tree.
type htmlReader struct {
	ext   []Extension
	slugs slugs
	// checkbox is a task item's box, read as its state, not as content.
	checkbox *html.Node
}

func newHTMLReader(ext []Extension) *htmlReader {
	return &htmlReader{ext: ext, slugs: slugs{}}
}

// flow is the blocks being read, and the run of inline content after the
// last one.
type flow struct {
	blocks []Block
	run    []Inline
}

// blocksOf reads n's children in block context, all but skip.
func (r *htmlReader) blocksOf(n *html.Node, depth int, skip *html.Node) []Block {
	var f flow
	if depth < maxDepth {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c != skip {
				r.blockNode(c, &f, depth+1)
			}
		}
	}
	r.flush(&f)
	return f.blocks
}

// flush ends the run of inline content as a paragraph, unless it shows
// nothing.
func (r *htmlReader) flush(f *flow) {
	content := r.finish(f.run)
	f.run = nil
	if len(content) > 0 {
		f.blocks = append(f.blocks, Paragraph{Content: content})
	}
}

func (r *htmlReader) push(f *flow, b Block) {
	r.flush(f)
	f.blocks = append(f.blocks, b)
}

func (r *htmlReader) blockNode(n *html.Node, f *flow, depth int) {
	if n.Type == html.TextNode {
		f.run = pushText(f.run, n.Data)
		return
	}
	if n.Type != html.ElementNode || isHidden(n) || droppedElements[n.Data] {
		return
	}
	if e, ok := r.extension(n.Data); ok {
		if isStandalone(n) {
			c := r.custom(n, e)
			c.Children = r.blocksOf(n, depth, nil)
			r.push(f, c)
		} else {
			f.run = r.inlineNode(n, f.run, depth)
		}
		return
	}
	switch name := n.Data; {
	case len(name) == 2 && name[0] == 'h' && name[1] >= '1' && name[1] <= '6':
		content := r.inlineContent(n, depth)
		anchor, ok := attr(n, "id")
		if !ok || anchor == "" {
			anchor = r.slugs.slug(PlainText(content))
		}
		r.push(f, Heading{Level: int(name[1] - '0'), Content: content, Anchor: anchor})
	case name == "blockquote":
		r.push(f, Blockquote{Children: r.blocksOf(n, depth, nil)})
	case name == "ul" || name == "ol" || name == "menu" || name == "dir":
		r.push(f, r.list(n, name == "ol", depth))
	case name == "pre":
		r.push(f, codeBlock(n))
	case name == "hr":
		r.push(f, Rule{})
	case name == "table":
		r.table(n, f, depth)
	case name == "details":
		var summary *html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if isElement(c, "summary") {
				summary = c
				break
			}
		}
		d := Details{}
		_, d.Open = attr(n, "open")
		if summary != nil {
			d.Summary = r.inlineContent(summary, depth)
		}
		d.Children = r.blocksOf(n, depth, summary)
		r.push(f, d)
	case name == "dt":
		if content := r.inlineContent(n, depth); len(content) > 0 {
			r.push(f, Paragraph{Content: []Inline{Strong{content}}})
		}
	case name == "math" && attrIs(n, "display", "block"):
		if tex, ok := texAnnotation(n); ok {
			r.push(f, Math{TeX: tex})
		} else {
			f.run = r.inlineNode(n, f.run, depth)
		}
	case name == "div" && alertOf(n) != AlertNone:
		// GitHub's alerts as its HTML writes them: the title is the
		// alert's kind, which the view shows its own way.
		var title *html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if hasClass(c, "markdown-alert-title") {
				title = c
				break
			}
		}
		r.push(f, Blockquote{Children: r.blocksOf(n, depth, title), Alert: alertOf(n)})
	case blockElements[name] || (depth < maxDepth && containsBlock(n, depth)):
		// A container: its children are blocks among the others.
		r.flush(f)
		if depth < maxDepth {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				r.blockNode(c, f, depth+1)
			}
		}
		r.flush(f)
	default:
		f.run = r.inlineNode(n, f.run, depth)
	}
}

func (r *htmlReader) list(n *html.Node, ordered bool, depth int) List {
	l := List{Ordered: ordered}
	// Content outside <li> (invalid, but written) is an item of its own.
	var stray flow
	endStray := func() {
		r.flush(&stray)
		if len(stray.blocks) > 0 {
			l.Items = append(l.Items, ListItem{Children: stray.blocks})
			stray.blocks = nil
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !isElement(c, "li") {
			if depth < maxDepth {
				r.blockNode(c, &stray, depth+1)
			}
			continue
		}
		endStray()
		for p := c.FirstChild; p != nil; p = p.NextSibling {
			l.Loose = l.Loose || isElement(p, "p")
		}
		item := ListItem{}
		box := taskCheckbox(c)
		if box != nil {
			item.Task = true
			_, item.Done = attr(box, "checked")
		}
		saved := r.checkbox
		r.checkbox = box
		item.Children = r.blocksOf(c, depth+1, nil)
		r.checkbox = saved
		l.Items = append(l.Items, item)
	}
	endStray()
	if ordered {
		l.Start = 1
		if s, ok := attr(n, "start"); ok {
			if v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				l.Start = int(min(max(v, 0), 1<<31-1))
			}
		}
	}
	return l
}

func (r *htmlReader) table(n *html.Node, f *flow, depth int) {
	if depth >= maxDepth {
		return
	}
	var rows, foot []*html.Node
	trs := func(section *html.Node) []*html.Node {
		var out []*html.Node
		for c := section.FirstChild; c != nil; c = c.NextSibling {
			if isElement(c, "tr") {
				out = append(out, c)
			}
		}
		return out
	}
	head := false
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.Data {
		case "caption":
			if content := r.inlineContent(c, depth); len(content) > 0 {
				r.push(f, Paragraph{Content: content})
			}
		case "thead":
			if !head {
				// The head goes first wherever it is written.
				head = true
				rows = append(trs(c), rows...)
			} else {
				rows = append(rows, trs(c)...)
			}
		case "tbody":
			rows = append(rows, trs(c)...)
		case "tfoot":
			foot = append(foot, trs(c)...)
		case "tr":
			rows = append(rows, c)
		}
	}
	rows = append(rows, foot...)
	if len(rows) == 0 {
		return
	}
	cells := make([][]TableCell, len(rows))
	aligns := make([][]ColumnAlign, len(rows))
	for i, row := range rows {
		cells[i], aligns[i] = r.row(row, depth+1)
	}
	// Columns by the cells' spans, not their count.
	width := func(row []TableCell) int {
		w := 0
		for _, c := range row {
			w += c.span()
		}
		return w
	}
	columns := 0
	for _, row := range cells {
		columns = max(columns, width(row))
	}
	if columns == 0 {
		return
	}
	// A column aligns as its header cell says, else as its first row's.
	t := Table{Align: make([]ColumnAlign, columns)}
	for i := range t.Align {
		if i < len(aligns[0]) {
			t.Align[i] = aligns[0][i]
		}
		if t.Align[i] == AlignNone && len(aligns) > 1 && i < len(aligns[1]) {
			t.Align[i] = aligns[1][i]
		}
	}
	pad := func(row []TableCell) []TableCell {
		for range columns - width(row) {
			row = append(row, TableCell{})
		}
		return row
	}
	t.Head = pad(cells[0])
	for _, row := range cells[1:] {
		t.Rows = append(t.Rows, pad(row))
	}
	r.push(f, t)
}

// row returns the cells of a row and the alignment of each column they
// span.
func (r *htmlReader) row(n *html.Node, depth int) ([]TableCell, []ColumnAlign) {
	var cells []TableCell
	var align []ColumnAlign
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !isElement(c, "th") && !isElement(c, "td") {
			continue
		}
		span := 1
		if s, ok := attr(c, "colspan"); ok {
			if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				span = min(max(v, 1), maxColSpan)
			}
		}
		cell := TableCell{Content: r.inlineContent(c, depth)}
		if span > 1 {
			cell.ColSpan = span
		}
		cells = append(cells, cell)
		a := cellAlign(c)
		for range span {
			align = append(align, a)
		}
	}
	return cells, align
}

// inlineContent reads n's content in inline context, finished.
func (r *htmlReader) inlineContent(n *html.Node, depth int) []Inline {
	return r.finish(r.inlineChildren(n, nil, depth))
}

// inlineChildren appends n's children read in inline context to out.
func (r *htmlReader) inlineChildren(n *html.Node, out []Inline, depth int) []Inline {
	if depth >= maxDepth {
		return out
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = r.inlineNode(c, out, depth+1)
	}
	return out
}

func (r *htmlReader) inlineNode(n *html.Node, out []Inline, depth int) []Inline {
	if n.Type == html.TextNode {
		return pushText(out, n.Data)
	}
	if n.Type != html.ElementNode || isHidden(n) || droppedElements[n.Data] || n == r.checkbox {
		return out
	}
	if e, ok := r.extension(n.Data); ok {
		c := r.custom(n, e)
		c.Content = r.inlineContent(n, depth)
		return append(out, c)
	}
	inner := func() []Inline { return r.inlineChildren(n, nil, depth) }
	switch n.Data {
	case "br":
		return append(out, Break{})
	case "strong", "b":
		return append(out, Strong{inner()})
	case "em", "i", "cite", "dfn", "var":
		return append(out, Emphasis{inner()})
	case "del", "s", "strike":
		return append(out, Strikethrough{inner()})
	case "u", "ins":
		return append(out, Underline{inner()})
	case "mark":
		return append(out, Mark{Content: inner(), Color: markColor(n)})
	case "sup":
		return append(out, Superscript{inner()})
	case "sub":
		return append(out, Subscript{inner()})
	case "small":
		return append(out, Small{inner()})
	case "code", "tt", "samp":
		return append(out, Code{collapsed(textContent(n))})
	case "kbd":
		return append(out, Keyboard{strings.TrimSpace(collapsed(textContent(n)))})
	case "a":
		content := inner()
		if href, _ := attr(n, "href"); strings.TrimSpace(href) != "" {
			title, _ := attr(n, "title")
			return append(out, Link{URL: strings.TrimSpace(href), Title: title, Content: content})
		}
		return append(out, content...)
	case "img":
		if src, _ := attr(n, "src"); strings.TrimSpace(src) != "" {
			im := Image{Src: strings.TrimSpace(src)}
			im.Alt, _ = attr(n, "alt")
			im.Title, _ = attr(n, "title")
			im.Width, im.Height = imageLength(n, "width"), imageLength(n, "height")
			return append(out, im)
		}
		if alt, ok := attr(n, "alt"); ok {
			return pushText(out, alt)
		}
		return out
	case "q":
		out = pushText(out, "“")
		out = r.inlineChildren(n, out, depth)
		return pushText(out, "”")
	case "math":
		if tex, ok := texAnnotation(n); ok {
			return append(out, Math{TeX: tex})
		}
		return pushText(out, textContent(n))
	case "pre":
		// Code in a line: its lines apart by breaks.
		for i, line := range strings.Split(codeBlock(n).Code, "\n") {
			if i > 0 {
				out = append(out, Break{})
			}
			out = append(out, Code{line})
		}
		return out
	case "hr":
		return append(out, Break{})
	}
	if blockElements[n.Data] {
		// A block in inline context: its content on lines of its own.
		if len(out) > 0 && !endsWithBreak(out) {
			out = append(out, Break{})
		}
		out = r.inlineChildren(n, out, depth)
		if len(out) > 0 && !endsWithBreak(out) {
			out = append(out, Break{})
		}
		return out
	}
	return r.inlineChildren(n, out, depth)
}

func endsWithBreak(in []Inline) bool {
	_, ok := in[len(in)-1].(Break)
	return ok
}

// finish makes inline content what a block holds: whitespace collapsed,
// breaks at its ends dropped (a browser shows none), patterns applied.
func (r *htmlReader) finish(in []Inline) []Inline {
	ws := whitespace{lineStart: true}
	out := collapse(in, &ws)
	for len(out) > 0 && endsWithBreak(out) {
		out = out[:len(out)-1]
	}
	for len(out) > 0 {
		if _, ok := out[0].(Break); !ok {
			break
		}
		out = out[1:]
	}
	if len(out) == 0 {
		return nil
	}
	return applyPatterns(out, r.ext)
}

// extension returns the extension of elements named name.
func (r *htmlReader) extension(name string) (Extension, bool) {
	for _, e := range r.ext {
		if e.Tag != "" && strings.EqualFold(e.Tag, name) {
			return e, true
		}
	}
	for _, e := range r.ext {
		if e.Tag == anyTag {
			return Extension{Name: name, Tag: name}, true
		}
	}
	return Extension{}, false
}

// custom returns n's custom node without its content.
func (r *htmlReader) custom(n *html.Node, e Extension) Custom {
	c := Custom{Name: e.Name, Text: strings.TrimSpace(collapsed(textContent(n)))}
	for _, a := range n.Attr {
		if c.Attributes == nil {
			c.Attributes = map[string]string{}
		}
		c.Attributes[a.Key] = a.Val
	}
	return c
}

// whitespace is where collapsing whitespace is.
type whitespace struct {
	// pending is whitespace skipped: a space comes before the next
	// content.
	pending bool
	// lineStart is the start of a line, where whitespace is dropped.
	lineStart bool
}

// collapse collapses whitespace as a browser does (one space between
// words, none at the start of a line or before a break), merges texts
// and drops empty formatting. A skipped space is written before the
// content after it, so it lands outside an element that ends with it
// (<a>link </a>next).
func collapse(in []Inline, ws *whitespace) []Inline {
	out := make([]Inline, 0, len(in))
	start := func() {
		if ws.pending {
			ws.pending = false
			out = pushText(out, " ")
		}
		ws.lineStart = false
	}
	for _, it := range in {
		switch it := it.(type) {
		case Text:
			var b strings.Builder
			for _, c := range it.Text {
				if isHTMLSpace(c) {
					ws.pending = ws.pending || !ws.lineStart
					continue
				}
				if ws.pending {
					ws.pending = false
					b.WriteByte(' ')
				}
				ws.lineStart = false
				b.WriteRune(c)
			}
			out = pushText(out, b.String())
		case Break:
			ws.pending, ws.lineStart = false, true
			out = append(out, it)
		case Strong:
			out = wrap(out, it.Content, ws, func(c []Inline) Inline { return Strong{c} })
		case Emphasis:
			out = wrap(out, it.Content, ws, func(c []Inline) Inline { return Emphasis{c} })
		case Strikethrough:
			out = wrap(out, it.Content, ws, func(c []Inline) Inline { return Strikethrough{c} })
		case Underline:
			out = wrap(out, it.Content, ws, func(c []Inline) Inline { return Underline{c} })
		case Mark:
			out = wrap(out, it.Content, ws, func(c []Inline) Inline { return Mark{c, it.Color} })
		case Superscript:
			out = wrap(out, it.Content, ws, func(c []Inline) Inline { return Superscript{c} })
		case Subscript:
			out = wrap(out, it.Content, ws, func(c []Inline) Inline { return Subscript{c} })
		case Small:
			out = wrap(out, it.Content, ws, func(c []Inline) Inline { return Small{c} })
		case Link:
			// A link is content even when empty: an anchor to jump to.
			start()
			it.Content = nilIfEmpty(collapse(it.Content, ws))
			out = append(out, it)
		case Custom:
			start()
			// Its content is its own: no space carries in or out.
			inner := whitespace{lineStart: true}
			it.Content = nilIfEmpty(collapse(it.Content, &inner))
			out = append(out, it)
		case Code:
			if it.Code != "" {
				start()
				out = append(out, it)
			}
		default:
			start()
			out = append(out, it)
		}
	}
	return out
}

func nilIfEmpty(in []Inline) []Inline {
	if len(in) == 0 {
		return nil
	}
	return in
}

// wrap appends content, collapsed and wrapped by make, unless it is
// empty. A space skipped before it goes before the element, not inside
// (a <b>bold</b>: the space is not bold).
func wrap(out, content []Inline, ws *whitespace, make func([]Inline) Inline) []Inline {
	pending := ws.pending
	content = collapse(content, ws)
	if pending && len(content) > 0 {
		if t, ok := content[0].(Text); ok && strings.HasPrefix(t.Text, " ") {
			if t.Text = t.Text[1:]; t.Text == "" {
				content = content[1:]
			} else {
				content[0] = t
			}
			out = pushText(out, " ")
		}
	}
	if len(content) > 0 {
		out = append(out, make(content))
	}
	return out
}

// pushText appends text, merged into a text out ends with.
func pushText(out []Inline, text string) []Inline {
	if text == "" {
		return out
	}
	if len(out) > 0 {
		if last, ok := out[len(out)-1].(Text); ok {
			out[len(out)-1] = Text{last.Text + text}
			return out
		}
	}
	return append(out, Text{text})
}

// isHTMLSpace reports ASCII whitespace, what HTML collapses; a no-break
// space is a letter.
func isHTMLSpace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// collapsed returns text with each run of whitespace one space.
func collapsed(text string) string {
	var b strings.Builder
	space := false
	for _, c := range text {
		if isHTMLSpace(c) {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(c)
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

func isElement(n *html.Node, name string) bool {
	return n.Type == html.ElementNode && n.Data == name
}

func attr(n *html.Node, name string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == name {
			return a.Val, true
		}
	}
	return "", false
}

func attrIs(n *html.Node, name, value string) bool {
	v, ok := attr(n, name)
	return ok && v == value
}

func hasClass(n *html.Node, class string) bool {
	v, _ := attr(n, "class")
	for _, c := range strings.Fields(v) {
		if c == class {
			return true
		}
	}
	return false
}

// isHidden reports an element hidden from readers (hidden, aria-hidden,
// an inline display: none): KaTeX's HTML beside its MathML, icons.
func isHidden(n *html.Node) bool {
	if _, ok := attr(n, "hidden"); ok {
		return true
	}
	if v, _ := attr(n, "aria-hidden"); strings.TrimSpace(v) == "true" {
		return true
	}
	v, _ := styleProperty(n, "display")
	return strings.EqualFold(v, "none")
}

// styleProperty returns a property of n's style attribute.
func styleProperty(n *html.Node, property string) (string, bool) {
	style, _ := attr(n, "style")
	for decl := range strings.SplitSeq(style, ";") {
		key, value, ok := strings.Cut(decl, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), property) {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

func isBlank(n *html.Node) bool {
	switch n.Type {
	case html.TextNode:
		return strings.TrimFunc(n.Data, isHTMLSpace) == ""
	case html.CommentNode:
		return true
	}
	return false
}

// isStandalone reports an element with only blocks, or nothing, beside it
// in its parent: a block itself rather than inline content.
func isStandalone(n *html.Node) bool {
	isBlock := func(s *html.Node) bool {
		return s == nil || s.Type == html.ElementNode && (blockElements[s.Data] || s.Data == "br")
	}
	before, after := n.PrevSibling, n.NextSibling
	for before != nil && isBlank(before) {
		before = before.PrevSibling
	}
	for after != nil && isBlank(after) {
		after = after.NextSibling
	}
	return isBlock(before) && isBlock(after)
}

// containsBlock reports whether a descendant of n is a block.
func containsBlock(n *html.Node, depth int) bool {
	if depth >= maxDepth {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && blockElements[c.Data] || containsBlock(c, depth+1) {
			return true
		}
	}
	return false
}

// textContent returns the text of n's descendants, <br> as a line break.
func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node, depth int)
	walk = func(n *html.Node, depth int) {
		if depth >= maxDepth {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type == html.TextNode:
				b.WriteString(c.Data)
			case isElement(c, "br"):
				b.WriteByte('\n')
			case c.Type == html.ElementNode && !droppedElements[c.Data]:
				walk(c, depth+1)
			}
		}
	}
	walk(n, 0)
	return b.String()
}

// codeBlock reads a <pre>: its text as written, its language from its
// code's class (language-go, lang-go), its own, or its lang attribute.
func codeBlock(n *html.Node) CodeBlock {
	c := CodeBlock{Code: strings.TrimSuffix(textContent(n), "\n")}
	for k := n.FirstChild; k != nil && c.Language == ""; k = k.NextSibling {
		if isElement(k, "code") {
			c.Language = language(k)
		}
	}
	if c.Language == "" {
		c.Language = language(n)
	}
	if c.Language == "" {
		c.Language, _ = attr(n, "lang")
	}
	if c.Language == "" {
		c.Language, _ = attr(n, "data-lang")
	}
	return c
}

func language(n *html.Node) string {
	v, _ := attr(n, "class")
	for _, class := range strings.Fields(v) {
		for _, prefix := range []string{"language-", "lang-"} {
			if l, ok := strings.CutPrefix(class, prefix); ok {
				return l
			}
		}
	}
	return ""
}

// texAnnotation returns the TeX of MathML from its annotation, as KaTeX
// and MathJax write it.
func texAnnotation(n *html.Node) (string, bool) {
	var find func(n *html.Node, depth int) *html.Node
	find = func(n *html.Node, depth int) *html.Node {
		if depth >= maxDepth {
			return nil
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if enc, _ := attr(c, "encoding"); isElement(c, "annotation") && strings.EqualFold(enc, "application/x-tex") {
				return c
			}
			if a := find(c, depth+1); a != nil {
				return a
			}
		}
		return nil
	}
	a := find(n, 0)
	if a == nil {
		return "", false
	}
	return strings.TrimSpace(textContent(a)), true
}

// imageLength returns the size an <img> asks for on one side: its
// attribute, else its style's property; auto when neither is a length.
func imageLength(n *html.Node, side string) core.Length {
	if v, ok := attr(n, side); ok {
		if l, ok := parseImageLength(v); ok {
			return l
		}
	}
	if v, ok := styleProperty(n, side); ok {
		if l, ok := parseImageLength(v); ok {
			return l
		}
	}
	return core.Length{}
}

// parseImageLength reads a length as gpui-kit reads an image's: points
// (32, 32px) or a percentage (50%), not negative.
func parseImageLength(s string) (core.Length, bool) {
	s = strings.TrimSpace(s)
	pct := false
	if v, ok := strings.CutSuffix(s, "%"); ok {
		s, pct = v, true
	} else if v, ok := strings.CutSuffix(s, "px"); ok {
		s = v
	}
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "eEnNiI") {
		return core.Length{}, false
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil || v < 0 {
		return core.Length{}, false
	}
	if pct {
		return core.Pct(float32(v)), true
	}
	return core.Pt(core.Points(v)), true
}

// markColor returns the background a <mark> asks for: its color
// attribute, else its style's background-color. The attribute names four
// colors as gpui-kit's renderer does (blue and yellow are Tailwind's 500
// and 400, not CSS's); the rest are CSS colors.
func markColor(n *html.Node) core.Color {
	if v, ok := attr(n, "color"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "black":
			return core.RGB(0x000000)
		case "white":
			return core.RGB(0xffffff)
		case "blue":
			return core.RGB(0x3b82f6)
		case "yellow":
			return core.RGB(0xfacc15)
		}
		if c, ok := parseCSSColor(v); ok {
			return c
		}
	}
	if v, ok := styleProperty(n, "background-color"); ok {
		if c, ok := parseCSSColor(v); ok {
			return c
		}
	}
	return 0
}

// cellAlign returns the alignment a cell asks for: align, or its style's
// text-align.
func cellAlign(n *html.Node) ColumnAlign {
	v, ok := attr(n, "align")
	if !ok {
		v, _ = styleProperty(n, "text-align")
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "left", "start":
		return AlignLeft
	case "center":
		return AlignCenter
	case "right", "end":
		return AlignRight
	}
	return AlignNone
}

// taskCheckbox returns the box of a task item: an <input type=checkbox>
// before its content, directly or in its first paragraph, as Markdown
// renderers write it.
func taskCheckbox(li *html.Node) *html.Node {
	first := func(n *html.Node) *html.Node {
		c := n.FirstChild
		for c != nil && isBlank(c) {
			c = c.NextSibling
		}
		return c
	}
	n := first(li)
	if n != nil && isElement(n, "p") {
		n = first(n)
	}
	if n == nil || !isElement(n, "input") {
		return nil
	}
	if t, _ := attr(n, "type"); !strings.EqualFold(t, "checkbox") {
		return nil
	}
	return n
}

// alertOf returns the alert of GitHub's HTML for one: <div
// class="markdown-alert markdown-alert-note">.
func alertOf(n *html.Node) Alert {
	v, _ := attr(n, "class")
	for _, class := range strings.Fields(v) {
		if kind, ok := strings.CutPrefix(class, "markdown-alert-"); ok {
			if a := alertKind("[!" + kind + "]"); a != AlertNone {
				return a
			}
		}
	}
	return AlertNone
}
