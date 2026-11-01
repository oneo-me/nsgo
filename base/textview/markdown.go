package textview

import (
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Extension is syntax of the app's that becomes Custom nodes named Name,
// which the TextView's RenderCustom (blocks) and CustomSpan (inline)
// show. Set one of Pattern, Language, Container or Tag.
type Extension struct {
	Name string
	// Pattern is inline syntax matched in text, outside code and links
	// (`@(\w+)` for mentions). Each match is an inline node: its Text the
	// match, its groups the attributes "1", "2", ... and by name.
	Pattern *regexp.Regexp
	// Language makes fenced code of this language (```chart) a block
	// node: its Text the code, the rest of the info string the attribute
	// "meta".
	Language string
	// Container makes containers of this name (:::note … :::, as
	// remark-directive writes them) block nodes of the Markdown inside:
	// the label after the name (:::note[Heads up]) is the attribute
	// "label", {key=value .class #id} the other attributes. A container
	// inside another closes with fewer colons than the outer one.
	Container string
	// Tag makes HTML elements of this name, in any case (<Callout>), custom
	// nodes: their attributes the node's, their text its Text, and what
	// they hold its blocks when they stand alone, else its inline content.
	// An element may close itself as JSX writes it (<Callout />).
	Tag string
}

// Parse reads Markdown into a document: CommonMark with GitHub's tables,
// task lists, strikethrough, autolinks, footnotes and alerts, formulas,
// and the syntax of extensions. HTML inside reads as ParseHTML reads it;
// the Markdown between an element's tags, on lines of their own or in a
// paragraph (<b>x *y*</b>), stays Markdown.
//
// Formulas are TeX between dollars: $x$ and $$x$$ in text, a block
// between $$ fences on lines of their own, or fenced code of language
// math, as GitHub shows it. Single dollars around a space or before a
// digit are prose ("costs $5 and $10"). TeX's own \(x\) and \[x\] are
// formulas too.
func Parse(markdown string, extensions ...Extension) Document {
	return ParseWith(markdown, Options{}, extensions...)
}

// Options say what a Markdown document reads beyond CommonMark and
// GitHub's syntax, as NSH's MarkdownOptions; the zero value is NSH's
// default.
type Options struct {
	// NoMath reads dollars and \( \[ as text and math fences as code.
	NoMath bool
	// NoHTML shows HTML in the Markdown as text.
	NoHTML bool
	// NoAlerts reads > [!NOTE] and its kin as plain quotations.
	NoAlerts bool
	// Frontmatter reads YAML front matter (--- … --- first) as a Custom
	// block "frontmatter" whose Text is the YAML, rather than a rule and
	// text.
	Frontmatter bool
	// MDX reads JSX elements as Custom nodes named by their tag, with
	// their attributes and content, and {expressions} as Custom nodes
	// "expression" whose Text is the expression; HTML is then read as
	// elements too.
	MDX bool
	// Streaming repairs the syntax the last block leaves open before
	// parsing (RepairStreaming), for a document still arriving.
	Streaming bool
}

// mdxExtensions are the extensions MDX adds: every element, and braces
// in text (mdxExpression).
var (
	mdxExpression = regexp.MustCompile(`\{([^{}\n]*)\}`)
	mdxExtensions = []Extension{{Tag: anyTag}, {Name: "expression", Pattern: mdxExpression}}
)

// anyTag is the Tag of an extension that takes every element, named by
// its tag.
const anyTag = "*"

// ParseWith is Parse with options.
func ParseWith(markdown string, o Options, extensions ...Extension) Document {
	if o.Streaming {
		markdown = RepairStreaming(markdown)
	}
	var front *Custom
	if o.Frontmatter {
		if yaml, rest, ok := frontmatter(markdown); ok {
			front, markdown = &Custom{Name: "frontmatter", Text: yaml}, rest
		}
	}
	if o.MDX {
		extensions = append(slices.Clip(extensions), mdxExtensions...)
	}
	if !o.NoMath {
		markdown = rewriteTeX(markdown)
	}
	src := []byte(markdown)
	root := markdownFor(extensions, o.NoMath).Parser().Parse(text.NewReader(src))
	r := reader{src: src, ext: extensions, labels: map[int]string{}, slugs: slugs{}, opts: o}
	var doc Document
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if list, ok := n.(*east.FootnoteList); ok {
			for f := list.FirstChild(); f != nil; f = f.NextSibling() {
				if f, ok := f.(*east.Footnote); ok {
					r.labels[f.Index] = string(f.Ref)
				}
			}
		}
	}
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if list, ok := n.(*east.FootnoteList); ok {
			for f := list.FirstChild(); f != nil; f = f.NextSibling() {
				if f, ok := f.(*east.Footnote); ok {
					doc.Footnotes = append(doc.Footnotes,
						Footnote{Label: string(f.Ref), Index: f.Index, Children: r.blocks(f)})
				}
			}
		}
	}
	doc.Blocks = r.blockRun(root.FirstChild(), nil)
	if front != nil {
		doc.Blocks = append([]Block{*front}, doc.Blocks...)
	}
	return doc
}

// frontmatter splits YAML front matter off the start of markdown: a ---
// line, the YAML, and a --- or ... line.
func frontmatter(markdown string) (yaml, rest string, ok bool) {
	first, body, found := strings.Cut(markdown, "\n")
	if !found || strings.TrimRight(first, " \t\r") != "---" {
		return "", markdown, false
	}
	at := 0
	for at <= len(body) {
		line, next, more := strings.Cut(body[at:], "\n")
		if l := strings.TrimRight(line, " \t\r"); l == "---" || l == "..." {
			yaml = strings.TrimSuffix(body[:at], "\n")
			if !more {
				return yaml, "", true
			}
			return yaml, next, true
		}
		if !more {
			break
		}
		at += len(line) + 1
	}
	return "", markdown, false
}

var (
	plainMarkdown     [2]goldmark.Markdown
	plainMarkdownOnce [2]sync.Once
)

// markdownFor returns the parser for extensions' containers, with or
// without formulas, shared when there are none.
func markdownFor(extensions []Extension, noMath bool) goldmark.Markdown {
	names := map[string]string{}
	for _, e := range extensions {
		if e.Container != "" {
			names[e.Container] = e.Name
		}
	}
	if len(names) == 0 {
		i := 0
		if noMath {
			i = 1
		}
		plainMarkdownOnce[i].Do(func() { plainMarkdown[i] = newMarkdown(nil, noMath) })
		return plainMarkdown[i]
	}
	return newMarkdown(names, noMath)
}

func newMarkdown(containers map[string]string, noMath bool) goldmark.Markdown {
	var blocks, inlines []util.PrioritizedValue
	if !noMath {
		blocks = append(blocks, util.Prioritized(&mathBlockParser{}, 650))
		inlines = append(inlines, util.Prioritized(&mathInlineParser{}, 150))
	}
	if len(containers) > 0 {
		blocks = append(blocks, util.Prioritized(&containerParser{names: containers}, 50))
	}
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(
			parser.WithBlockParsers(blocks...),
			parser.WithInlineParsers(inlines...),
		),
	)
}

// reader turns goldmark's syntax tree into a Document.
type reader struct {
	src    []byte
	ext    []Extension
	labels map[int]string // footnote labels by number
	slugs  slugs
	opts   Options
}

func (r *reader) blocks(parent ast.Node) []Block {
	return r.blockRun(parent.FirstChild(), nil)
}

// blockRun reads the blocks from n up to stop, or to the last sibling.
// An HTML block that opens an element another closes holds the Markdown
// between them.
func (r *reader) blockRun(n, stop ast.Node) []Block {
	var out []Block
	for ; n != nil && n != stop; n = n.NextSibling() {
		h, ok := n.(*ast.HTMLBlock)
		if !ok {
			if b := r.block(n); b != nil {
				out = append(out, b)
			}
			continue
		}
		if r.opts.NoHTML {
			// Shown as written.
			out = append(out, Paragraph{Content: []Inline{Text{strings.TrimRight(r.html(h), "\n")}}})
			continue
		}
		if close := r.htmlElement(h, &out); close != nil {
			n = close
			continue
		}
		out = append(out, htmlBlocks(r.html(h), r.ext)...)
	}
	return out
}

// htmlElement reads an element whose opening and closing tags are blocks
// of their own with Markdown between them (<details>, a blank line,
// Markdown, </details>) as one element holding the Markdown's blocks. It
// returns the closing tag's block, or nil when open is no such element.
func (r *reader) htmlElement(open *ast.HTMLBlock, out *[]Block) ast.Node {
	source := r.html(open)
	name, ok := unclosedElement(source)
	if !ok {
		return nil
	}
	close := matchingClose(open, name, func(n ast.Node) (string, bool) {
		h, ok := n.(*ast.HTMLBlock)
		if !ok {
			return "", false
		}
		return r.html(h), true
	})
	if close == nil {
		return nil
	}
	element := htmlBlocks(source+"\n"+strings.TrimSpace(r.html(close.(*ast.HTMLBlock))), r.ext)
	if len(element) == 0 {
		return nil
	}
	inner := r.blockRun(open.NextSibling(), close)
	switch b := element[len(element)-1].(type) {
	case Details:
		b.Children = append(b.Children, inner...)
		element[len(element)-1] = b
	case Blockquote:
		b.Children = append(b.Children, inner...)
		element[len(element)-1] = b
	case Custom:
		b.Children = append(b.Children, inner...)
		if len(b.Content) == 0 {
			b.Text = Document{Blocks: b.Children}.PlainText()
		}
		element[len(element)-1] = b
	default:
		return nil
	}
	*out = append(*out, element...)
	return close
}

// html returns an HTML block's source.
func (r *reader) html(h *ast.HTMLBlock) string {
	var b strings.Builder
	for i := 0; i < h.Lines().Len(); i++ {
		seg := h.Lines().At(i)
		b.Write(seg.Value(r.src))
	}
	if h.HasClosure() {
		b.Write(h.ClosureLine.Value(r.src))
	}
	return b.String()
}

func (r *reader) block(n ast.Node) Block {
	switch n := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return Paragraph{Content: r.content(n)}
	case *ast.Heading:
		content := r.content(n)
		return Heading{Level: n.Level, Content: content, Anchor: r.anchor(PlainText(content))}
	case *ast.ThematicBreak:
		return Rule{}
	case *ast.CodeBlock:
		return CodeBlock{Code: r.lines(n)}
	case *ast.FencedCodeBlock:
		var lang, meta string
		if n.Info != nil {
			info := strings.TrimSpace(string(n.Info.Segment.Value(r.src)))
			lang, meta, _ = strings.Cut(info, " ")
			meta = strings.TrimSpace(meta)
		}
		code := r.lines(n)
		for _, e := range r.ext {
			if e.Language != "" && strings.EqualFold(e.Language, lang) {
				c := Custom{Name: e.Name, Text: code}
				if meta != "" {
					c.Attributes = map[string]string{"meta": meta}
				}
				return c
			}
		}
		if strings.EqualFold(lang, "math") && !r.opts.NoMath {
			return Math{TeX: code}
		}
		return CodeBlock{Language: lang, Meta: meta, Code: code}
	case *ast.Blockquote:
		return r.quote(n)
	case *ast.List:
		l := List{Ordered: n.IsOrdered(), Loose: !n.IsTight}
		if l.Ordered {
			l.Start = n.Start
		}
		for it := n.FirstChild(); it != nil; it = it.NextSibling() {
			item := ListItem{Children: r.blocks(it)}
			if first := it.FirstChild(); first != nil {
				if box, ok := first.FirstChild().(*east.TaskCheckBox); ok {
					item.Task, item.Done = true, box.IsChecked
				}
			}
			l.Items = append(l.Items, item)
		}
		return l
	case *east.Table:
		t := Table{}
		for _, a := range n.Alignments {
			t.Align = append(t.Align, [...]ColumnAlign{east.AlignLeft: AlignLeft, east.AlignRight: AlignRight,
				east.AlignCenter: AlignCenter, east.AlignNone: AlignNone}[a])
		}
		for row := n.FirstChild(); row != nil; row = row.NextSibling() {
			var cells []TableCell
			for c := row.FirstChild(); c != nil; c = c.NextSibling() {
				cells = append(cells, TableCell{Content: r.content(c)})
			}
			if _, head := row.(*east.TableHeader); head {
				t.Head = cells
			} else {
				t.Rows = append(t.Rows, cells)
			}
		}
		return t
	case *mathBlock:
		return Math{TeX: r.lines(n)}
	case *container:
		return Custom{Name: n.name, Attributes: n.attrs, Children: r.blocks(n)}
	}
	return nil
}

// quote reads a blockquote, or an alert when its first line is [!KIND].
func (r *reader) quote(n *ast.Blockquote) Block {
	first, ok := n.FirstChild().(*ast.Paragraph)
	if !ok || first.Lines().Len() == 0 {
		return Blockquote{Children: r.blocks(n)}
	}
	line := first.Lines().At(0)
	alert := alertKind(strings.TrimSpace(string(line.Value(r.src))))
	if alert == AlertNone || r.opts.NoAlerts {
		return Blockquote{Children: r.blocks(n)}
	}
	q := Blockquote{Alert: alert}
	c := first.FirstChild()
	for ; c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); !ok || t.Segment.Stop > line.Stop {
			break
		}
	}
	if rest := r.finish(r.inlineRun(nil, c, nil)); len(rest) > 0 {
		q.Children = append(q.Children, Paragraph{Content: rest})
	}
	q.Children = append(q.Children, r.blockRun(first.NextSibling(), nil)...)
	return q
}

func alertKind(line string) Alert {
	switch strings.ToUpper(line) {
	case "[!NOTE]":
		return AlertNote
	case "[!TIP]":
		return AlertTip
	case "[!IMPORTANT]":
		return AlertImportant
	case "[!WARNING]":
		return AlertWarning
	case "[!CAUTION]":
		return AlertCaution
	}
	return AlertNone
}

// lines returns a code block's code without its final line break.
func (r *reader) lines(n ast.Node) string {
	var b bytes.Buffer
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		b.Write(seg.Value(r.src))
	}
	return strings.TrimRight(b.String(), "\n")
}

// content reads a block's inline content.
func (r *reader) content(n ast.Node) []Inline {
	return r.finish(r.inlineRun(nil, n.FirstChild(), nil))
}

// inlineRun appends the inline content from n up to stop, or to the last
// sibling. An HTML tag another closes reads as one element from the
// source between them; a formatting element holds the Markdown between
// its tags.
func (r *reader) inlineRun(out []Inline, n, stop ast.Node) []Inline {
	for ; n != nil && n != stop; n = n.NextSibling() {
		open, ok := n.(*ast.RawHTML)
		if !ok || r.opts.NoHTML {
			out = r.inline(out, n)
			continue
		}
		tag := r.raw(open)
		name, ok := unclosedElement(tag)
		var close ast.Node
		if ok {
			close = matchingClose(open, name, func(n ast.Node) (string, bool) {
				h, ok := n.(*ast.RawHTML)
				if !ok {
					return "", false
				}
				return r.raw(h), true
			})
		}
		if close == nil {
			out = append(out, htmlInlines(tag, r.ext)...)
			continue
		}
		if in, ok := r.formatting(open, close.(*ast.RawHTML), strings.ToLower(name)); ok {
			out = append(out, in)
		} else {
			start := open.Segments.At(0).Start
			end := close.(*ast.RawHTML).Segments.At(close.(*ast.RawHTML).Segments.Len() - 1).Stop
			out = append(out, htmlInlines(string(r.src[start:end]), r.ext)...)
		}
		n = close
	}
	return out
}

// formatting reads a formatting element whose content is Markdown
// (<strong>b *c*</strong>: c is emphasized): the element as HTML reads its
// opening tag (a <mark>'s color, a link's URL), holding the Markdown
// between its tags.
func (r *reader) formatting(open, close *ast.RawHTML, name string) (Inline, bool) {
	// slot stands for the content while the opening tag is read.
	const slot = "\ue000"
	if !formattingElements[name] {
		return nil, false
	}
	read := htmlInlines(r.raw(open)+slot+"</"+name+">", r.ext)
	if len(read) != 1 {
		return nil, false
	}
	content := join(r.inlineRun(nil, open.NextSibling(), close))
	isSlot := func(c []Inline) bool { return len(c) == 1 && c[0] == Inline(Text{slot}) }
	switch in := read[0].(type) {
	case Strong:
		return Strong{content}, isSlot(in.Content)
	case Emphasis:
		return Emphasis{content}, isSlot(in.Content)
	case Strikethrough:
		return Strikethrough{content}, isSlot(in.Content)
	case Underline:
		return Underline{content}, isSlot(in.Content)
	case Mark:
		return Mark{content, in.Color}, isSlot(in.Content)
	case Superscript:
		return Superscript{content}, isSlot(in.Content)
	case Subscript:
		return Subscript{content}, isSlot(in.Content)
	case Small:
		return Small{content}, isSlot(in.Content)
	case Link:
		return Link{URL: in.URL, Title: in.Title, Content: content}, isSlot(in.Content)
	}
	return nil, false
}

// raw returns an inline HTML tag's source.
func (r *reader) raw(h *ast.RawHTML) string {
	var b strings.Builder
	for i := 0; i < h.Segments.Len(); i++ {
		s := h.Segments.At(i)
		b.Write(s.Value(r.src))
	}
	return b.String()
}

// unclosedElement returns the name of the element tag opens and does not
// close (<details>, <b class="x">), but not of void elements.
func unclosedElement(tag string) (string, bool) {
	inner, ok := strings.CutPrefix(strings.TrimLeftFunc(tag, unicode.IsSpace), "<")
	if !ok {
		return "", false
	}
	n := 0
	for n < len(inner) && (inner[n] == '-' || inner[n] < utf8.RuneSelf && (unicode.IsLetter(rune(inner[n])) || unicode.IsDigit(rune(inner[n])))) {
		n++
	}
	if n == 0 || !unicode.IsLetter(rune(inner[0])) {
		return "", false
	}
	name := inner[:n]
	if voidElements[strings.ToLower(name)] {
		return "", false
	}
	end := strings.IndexByte(inner, '>')
	if end < 0 || strings.HasSuffix(inner[:end], "/") {
		return "", false
	}
	if strings.Contains(strings.ToLower(inner), "</"+strings.ToLower(name)) {
		return "", false
	}
	return name, true
}

// isClosingTag reports whether tag closes the element name (</details>).
func isClosingTag(tag, name string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(tag), "</")
	if !ok {
		return false
	}
	rest, ok = strings.CutSuffix(rest, ">")
	return ok && strings.EqualFold(strings.TrimSpace(rest), name)
}

// matchingClose returns the sibling after open that closes the element
// name, reading HTML siblings' source with source.
func matchingClose(open ast.Node, name string, source func(ast.Node) (string, bool)) ast.Node {
	depth := 0
	for n := open.NextSibling(); n != nil; n = n.NextSibling() {
		s, ok := source(n)
		if !ok {
			continue
		}
		if inner, ok := unclosedElement(s); ok && strings.EqualFold(inner, name) {
			depth++
		} else if isClosingTag(s, name) {
			if depth == 0 {
				return n
			}
			depth--
		}
	}
	return nil
}

// finish joins neighboring texts, trims the end and splits text where
// patterns match.
func (r *reader) finish(in []Inline) []Inline {
	out := join(in)
	if len(out) > 0 {
		if t, ok := out[len(out)-1].(Text); ok {
			if t.Text = strings.TrimRightFunc(t.Text, unicode.IsSpace); t.Text == "" {
				out = out[:len(out)-1]
			} else {
				out[len(out)-1] = t
			}
		}
	}
	return applyPatterns(out, r.ext)
}

// join joins neighboring texts.
func join(in []Inline) []Inline {
	var out []Inline
	for _, it := range in {
		if t, ok := it.(Text); ok && len(out) > 0 {
			if last, ok := out[len(out)-1].(Text); ok {
				out[len(out)-1] = Text{last.Text + t.Text}
				continue
			}
		}
		out = append(out, it)
	}
	return out
}

func (r *reader) inline(out []Inline, n ast.Node) []Inline {
	switch n := n.(type) {
	case *ast.Text:
		out = append(out, Text{r.value(n)})
		switch {
		case n.HardLineBreak():
			out = append(out, Break{})
		case n.SoftLineBreak():
			out = append(out, Text{" "})
		}
	case *ast.String:
		out = append(out, Text{string(n.Value)})
	case *ast.CodeSpan:
		out = append(out, Code{r.text(n)})
	case *mathInline:
		out = append(out, Math{TeX: n.tex})
	case *ast.Emphasis:
		if n.Level >= 2 {
			out = append(out, Strong{r.children(n)})
		} else {
			out = append(out, Emphasis{r.children(n)})
		}
	case *east.Strikethrough:
		out = append(out, Strikethrough{r.children(n)})
	case *ast.Link:
		out = append(out, Link{URL: string(n.Destination), Title: string(n.Title), Content: r.children(n)})
	case *ast.AutoLink:
		url := string(n.URL(r.src))
		if n.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(url), "mailto:") {
			url = "mailto:" + url
		}
		out = append(out, Link{URL: url, Content: []Inline{Text{string(n.Label(r.src))}}})
	case *ast.Image:
		out = append(out, Image{Src: string(n.Destination), Alt: r.text(n), Title: string(n.Title)})
	case *ast.RawHTML:
		if r.opts.NoHTML {
			out = append(out, Text{r.raw(n)})
			break
		}
		out = append(out, htmlInlines(r.raw(n), r.ext)...)
	case *east.FootnoteLink:
		out = append(out, FootnoteReference{Label: r.labels[n.Index], Index: n.Index})
	case *east.TaskCheckBox, *east.FootnoteBacklink:
	default:
		out = r.inlineRun(out, n.FirstChild(), nil)
	}
	return out
}

// children reads n's inline content; patterns run once over the whole
// block, in finish.
func (r *reader) children(n ast.Node) []Inline {
	return join(r.inlineRun(nil, n.FirstChild(), nil))
}

// text returns the text of n's descendants.
func (r *reader) text(n ast.Node) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch c := c.(type) {
			case *ast.Text:
				b.WriteString(r.value(c))
				if c.SoftLineBreak() {
					b.WriteByte(' ')
				}
			case *ast.String:
				b.Write(c.Value)
			case *mathInline:
				b.WriteString(c.tex)
			}
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// value returns a text's characters: escapes (\$) and character
// references (&amp;) resolved, except in code.
func (r *reader) value(t *ast.Text) string {
	v := t.Segment.Value(r.src)
	if !t.IsRaw() {
		v = util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(v)))
	}
	return string(v)
}

func (r *reader) anchor(text string) string { return r.slugs.slug(text) }

// slugs makes headings' anchors unique in a document.
type slugs map[string]int

// slug returns the next heading's slug as GitHub makes it: lower case,
// spaces as hyphens, punctuation left out, a count after repeats.
func (r slugs) slug(text string) string {
	var b strings.Builder
	for _, c := range strings.TrimSpace(text) {
		switch {
		case c == ' ':
			b.WriteByte('-')
		case unicode.IsLetter(c) || unicode.IsDigit(c) || c == '-' || c == '_':
			b.WriteRune(unicode.ToLower(c))
		}
	}
	base := b.String()
	slug := base
	for {
		if _, seen := r[slug]; !seen {
			break
		}
		r[base]++
		slug = base + "-" + strconv.Itoa(r[base])
	}
	r[slug] = 0
	return slug
}

// applyPatterns splits text outside code and links where an extension's
// pattern matches, the earliest match first.
func applyPatterns(in []Inline, ext []Extension) []Inline {
	var patterns []Extension
	for _, e := range ext {
		if e.Pattern != nil {
			patterns = append(patterns, e)
		}
	}
	if len(patterns) == 0 {
		return in
	}
	var walk func([]Inline) []Inline
	walk = func(in []Inline) []Inline {
		var out []Inline
		for _, it := range in {
			switch it := it.(type) {
			case Text:
				out = append(out, splitText(it.Text, patterns)...)
			case Strong:
				out = append(out, Strong{walk(it.Content)})
			case Emphasis:
				out = append(out, Emphasis{walk(it.Content)})
			case Strikethrough:
				out = append(out, Strikethrough{walk(it.Content)})
			case Underline:
				out = append(out, Underline{walk(it.Content)})
			case Mark:
				out = append(out, Mark{walk(it.Content), it.Color})
			case Superscript:
				out = append(out, Superscript{walk(it.Content)})
			case Subscript:
				out = append(out, Subscript{walk(it.Content)})
			case Small:
				out = append(out, Small{walk(it.Content)})
			default:
				out = append(out, it)
			}
		}
		return out
	}
	return walk(in)
}

func splitText(s string, patterns []Extension) []Inline {
	var out []Inline
	for s != "" {
		at, end, which := -1, 0, -1
		var groups []int
		for i, p := range patterns {
			m := p.Pattern.FindStringSubmatchIndex(s)
			if m == nil || m[1] == m[0] {
				continue
			}
			if at < 0 || m[0] < at {
				at, end, which, groups = m[0], m[1], i, m
			}
		}
		if which < 0 {
			break
		}
		if at > 0 {
			out = append(out, Text{s[:at]})
		}
		p := patterns[which]
		c := Custom{Name: p.Name, Text: s[at:end]}
		if p.Pattern == mdxExpression {
			c.Text = s[groups[2]:groups[3]]
		}
		names := p.Pattern.SubexpNames()
		for g := 1; g*2 < len(groups); g++ {
			if groups[g*2] < 0 {
				continue
			}
			if c.Attributes == nil {
				c.Attributes = map[string]string{}
			}
			v := s[groups[g*2]:groups[g*2+1]]
			c.Attributes[strconv.Itoa(g)] = v
			if names[g] != "" {
				c.Attributes[names[g]] = v
			}
		}
		out = append(out, c)
		s = s[end:]
	}
	if s != "" {
		out = append(out, Text{s})
	}
	return out
}
