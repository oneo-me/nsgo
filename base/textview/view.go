package textview

import (
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/core"
)

// TextView shows a document: Markdown or HTML, parsed once per change,
// else the app's own Document. Its blocks are rows keyed by position and kind, so
// a document that grows at its end (a reply streaming in) builds only its
// last block again.
type TextView struct {
	// Markdown is the document, read by Parse with Extensions;
	// MarkdownFunc replaces it, again when what it reads changes.
	Markdown     string
	MarkdownFunc func() string
	// HTML is the document as HTML, read by ParseHTML with Extensions,
	// when no Markdown is given; HTMLFunc replaces it.
	HTML     string
	HTMLFunc func() string
	// Document is the document as blocks the app made, when neither
	// Markdown nor HTML is given; DocumentFunc replaces it.
	Document     Document
	DocumentFunc func() Document
	// Extensions go after those Provide gives the views below it.
	Extensions []Extension
	// Options say what the Markdown reads, Streaming among them.
	Options Options
	// StreamingCaret, as a document streams in, follows its end: inline
	// after the last text of the last block (into quotes, lists and
	// custom nodes' content, after a code block's code, in a table's last
	// cell), else on a line of its own after it.
	StreamingCaret core.Node

	// Styles style the parts; StyleFunc's result for a part goes under
	// them, again when what it reads changes, and Classes under both.
	Classes   Classes
	Styles    Styles
	StyleFunc func(Part) core.Style
	// Selectable makes the view a selectable view: one selection runs
	// across its blocks, and on into the texts around inside an outer
	// selectable view.
	Selectable bool
	// Highlighter colors code blocks; without one their code is plain.
	Highlighter Highlighter

	// OnLinkPress receives a pressed link's URL: a heading's anchor is
	// "#<anchor>", a footnote reference "#footnote-<label>". Nothing opens
	// a URL unless OpenLinks; the app decides.
	OnLinkPress func(url string)
	// OpenLinks also opens a pressed link to another site or app (a URL
	// with a scheme: https:, mailto:) outside the app with Scope.OpenURL,
	// after OnLinkPress; anchors are only reported.
	OpenLinks bool
	// OnImagePress receives an image pressed that is in no link, for the
	// app to show it larger.
	OnImagePress func(image Image)
	// RenderCustom shows a custom node: a block, or an inline node as an
	// attachment in its text unless CustomSpan shows it. Without either, a
	// node shows its content.
	RenderCustom func(cx *core.Scope, node Custom) core.Node
	// CustomSpan shows a custom node in text as a span.
	CustomSpan func(node Custom) core.Span
	// RenderMath shows a formula, display when it is a block of its own
	// ($$…$$ on lines of its own), else as an attachment in its text.
	// Without it a formula shows its TeX in PartMath or PartMathBlock.
	RenderMath func(cx *core.Scope, math Math, display bool) core.Node
	// RenderCodeBlock shows a code block in place of the default: the code
	// in a horizontal scroll view.
	RenderCodeBlock func(cx *core.Scope, code CodeBlock) core.Node
	// CodeBlockActions are buttons over each code block's top right
	// corner, in PartCodeBlockHeader, each a PartCodeBlockAction showing
	// RenderCodeBlockAction (its name as text without it). CopyAction
	// copies the code, and reports itself done for 1.6 seconds; a press on
	// any reports OnCodeBlockAction.
	CodeBlockActions      []string
	RenderCodeBlockAction func(cx *core.Scope, action CodeBlockAction) core.Node
	OnCodeBlockAction     func(action string, code CodeBlock)
	// RenderTableActions shows content above each table, in
	// PartTableActions.
	RenderTableActions func(cx *core.Scope, table Table) core.Node
	// NoTableScroll lets a table shrink past its columns' floors rather
	// than scroll; NoTableScrollbar and NoCodeBlockScrollbar hide the
	// scroll bars of tables and code blocks.
	NoTableScroll, NoTableScrollbar, NoCodeBlockScrollbar bool
	// RenderImage shows an image in place of the default: alone in a
	// paragraph, a block; in a line of text, an attachment.
	RenderImage func(cx *core.Scope, image Image) core.Node
	TestID      string
}

// Highlighter colors code: byte ranges of the code with their styles, in
// order and apart. Ranges out of order, overlapping or off a character
// boundary are skipped. It runs while the block is built, so the signals
// it reads (the appearance) build the block again.
type Highlighter func(code CodeBlock) []Highlight

// Highlight styles the bytes from Start to End of a code block's code,
// Class under Style.
type Highlight struct {
	Start, End int
	Class      string
	Style      core.Style
}

// row is a block with its position, the key of its row.
type row struct {
	index int
	block Block
}

type rowKey struct {
	index int
	kind  string
}

// Plugin is an extension a view gives every TextView below it, with
// what shows its custom nodes: Render comes before a view's
// RenderCustom, which shows the nodes Render leaves (returning nil).
type Plugin struct {
	Extension Extension
	Render    func(cx *core.Scope, node Custom) core.Node
}

var pluginsKey = core.NewKey[[]Plugin]("textview plugins", nil)

// Provide gives every TextView below cx's owner plugins, after those of
// the views around it.
func Provide(cx *core.Scope, plugins ...Plugin) {
	pluginsKey.Provide(cx, append(slices.Clip(pluginsKey.Use(cx)), plugins...))
}

// Render implements core.Node.
func (tv TextView) Render(cx *core.Scope) core.Node {
	plugins := pluginsKey.Use(cx)
	if len(plugins) > 0 {
		var ext []Extension
		for _, p := range plugins {
			ext = append(ext, p.Extension)
		}
		tv.Extensions = append(ext, tv.Extensions...)
	}
	r := &renderer{tv: tv, plugins: plugins, cx: cx}
	doc := core.NewMemoFunc(cx, tv.document, nil)
	rows := func() []row {
		blocks := doc.Get().Blocks
		out := make([]row, len(blocks))
		for i, b := range blocks {
			out[i] = row{i, b}
		}
		return out
	}
	count := core.NewMemo(cx, func() int { return len(doc.Get().Blocks) })
	notes := core.NewMemoFunc(cx, func() []Footnote { return doc.Get().Footnotes }, equal[[]Footnote])
	return core.View{
		StyleFunc:  func() core.Style { return r.style(PartRoot) },
		Selectable: tv.Selectable,
		TestID:     tv.TestID,
		Children: []core.Node{
			core.Each(rows, func(r row) rowKey { return rowKey{r.index, r.block.Kind()} },
				func(cx *core.Scope, item func() row, index func() int) core.Node {
					block := core.NewMemoFunc(cx, func() Block { return item().block }, equal[Block])
					// The last row holds the caret, so a row made last
					// once is made again when another follows it.
					last := core.NewMemo(cx, func() bool { return tv.StreamingCaret != nil && index() == count.Get()-1 })
					return core.Dynamic(func(cx *core.Scope) core.Node {
						return r.blockAt(cx, block.Get(), index() == 0, 0, last.Get())
					})
				}),
			core.Show{When: func() bool { return tv.StreamingCaret != nil && count.Get() == 0 },
				Then: func(*core.Scope) core.Node { return tv.StreamingCaret }},
			core.Dynamic(func(cx *core.Scope) core.Node { return r.footnotes(cx, notes.Get()) }),
		},
	}
}

func (tv TextView) document() Document {
	switch {
	case tv.MarkdownFunc != nil:
		return ParseWith(tv.MarkdownFunc(), tv.Options, tv.Extensions...)
	case tv.Markdown != "":
		return ParseWith(tv.Markdown, tv.Options, tv.Extensions...)
	case tv.HTMLFunc != nil:
		return ParseHTML(tv.HTMLFunc(), tv.Extensions...)
	case tv.HTML != "":
		return ParseHTML(tv.HTML, tv.Extensions...)
	case tv.DocumentFunc != nil:
		return tv.DocumentFunc()
	}
	return tv.Document
}

// CopyAction is the code block action that copies the code.
const CopyAction = "copy"

// copiedFor is how long the copy action reports itself done.
const copiedFor = 1600 * time.Millisecond

// CodeBlockAction is what RenderCodeBlockAction gets about an action.
type CodeBlockAction struct {
	Name string
	Code CodeBlock
	// Done reports, tracked, that the action just ran: the copy action's
	// check, for 1.6 seconds.
	Done func() bool
}

func equal[T any](a, b T) bool { return reflect.DeepEqual(a, b) }

// noStyle is the style of text outside any inline part.
func noStyle() core.Style { return core.Style{} }

// renderer builds a TextView's blocks with the primitives.
type renderer struct {
	tv      TextView
	plugins []Plugin
	// cx opens the links that leave the document.
	cx *core.Scope
}

// style merges the parts' styles: StyleFunc's for each, then Styles'.
func (r *renderer) style(parts ...Part) core.Style {
	var s core.Style
	if r.tv.StyleFunc != nil {
		for _, p := range parts {
			s = s.Merge(r.tv.StyleFunc(p))
		}
	}
	for _, p := range parts {
		s = s.Merge(r.tv.Styles[p])
	}
	return s
}

// class joins the parts' classes, later parts winning.
func (r *renderer) class(parts ...Part) string {
	var out string
	for _, p := range parts {
		if c := r.tv.Classes[p]; c != "" {
			out = joinClasses(out, c)
		}
	}
	return out
}

func joinClasses(a, b string) string {
	if a == "" {
		return b
	}
	return a + " " + b
}

func (r *renderer) blocks(cx *core.Scope, blocks []Block, depth int) []core.Node {
	return r.blocksAt(cx, blocks, depth, false)
}

// blocksAt builds blocks, the caret in the last when caret.
func (r *renderer) blocksAt(cx *core.Scope, blocks []Block, depth int, caret bool) []core.Node {
	out := make([]core.Node, len(blocks))
	for i, b := range blocks {
		out[i] = r.blockAt(cx, b, i == 0, depth, caret && i == len(blocks)-1)
	}
	if caret && len(blocks) == 0 {
		out = append(out, r.tv.StreamingCaret)
	}
	return out
}

func (r *renderer) block(cx *core.Scope, b Block, first bool, depth int) core.Node {
	return r.blockAt(cx, b, first, depth, false)
}

// caretSpans appends the streaming caret to spans, inline, when caret.
func (r *renderer) caretSpans(spans []core.Span, caret bool) []core.Span {
	if !caret {
		return spans
	}
	return append(spans, core.Span{Attachment: r.tv.StreamingCaret})
}

// blockAt builds b. first drops a heading's top margin; depth is how many
// lists b is in; caret puts the streaming caret at its end.
func (r *renderer) blockAt(cx *core.Scope, b Block, first bool, depth int, caret bool) core.Node {
	// Blocks with no text of their own to end take the caret on a line
	// after them.
	after := func(n core.Node) core.Node {
		if !caret {
			return n
		}
		return core.Group{n, r.tv.StreamingCaret}
	}
	switch b := b.(type) {
	case Paragraph:
		if images, ok := onlyImages(b.Content); ok {
			nodes := make([]core.Node, len(images))
			for i, im := range images {
				nodes[i] = r.image(cx, im.image, im.url)
			}
			return after(core.Group(nodes))
		}
		return core.Text{Spans: r.caretSpans(r.spans(nil, b.Content, noStyle, "", nil), caret), Class: r.class(PartParagraph), StyleFunc: func() core.Style { return r.style(PartParagraph) }}
	case Heading:
		level := min(max(b.Level, 1), 6)
		heading := func() core.Style {
			s := r.style(PartHeading, PartHeading1+Part(level-1))
			if first {
				s.Margin.Top = 0
			}
			return s
		}
		return core.Text{Spans: r.caretSpans(r.spans(nil, b.Content, noStyle, "", nil), caret), Class: r.class(PartHeading, PartHeading1+Part(level-1)),
			StyleFunc: heading, Role: core.RoleHeader}
	case Blockquote:
		children := r.blocksAt(cx, b.Children, depth, caret)
		if b.Alert == AlertNone {
			return core.View{Class: r.class(PartBlockquote), StyleFunc: func() core.Style { return r.style(PartBlockquote) }, Children: children}
		}
		alert := []Part{PartBlockquote, PartAlert, PartAlertNote + Part(b.Alert-AlertNote)}
		return core.View{Class: r.class(alert...), StyleFunc: func() core.Style { return r.style(alert...) },
			Children: []core.Node{
				core.Text{Content: b.Alert.String(), Class: r.class(PartAlertTitle), StyleFunc: func() core.Style { return r.style(PartAlertTitle) }},
				core.View{Class: r.class(PartAlertContent), StyleFunc: func() core.Style { return r.style(PartAlertContent) }, Children: children},
			}}
	case List:
		return r.list(cx, b, depth, caret)
	case CodeBlock:
		if r.tv.RenderCodeBlock != nil {
			return after(r.tv.RenderCodeBlock(cx, b))
		}
		return r.code(cx, b, caret)
	case Table:
		return r.table(cx, b, caret)
	case Rule:
		return after(core.View{Class: r.class(PartRule), StyleFunc: func() core.Style { return r.style(PartRule) }})
	case Details:
		return after(r.details(cx, b, depth))
	case Math:
		if r.tv.RenderMath != nil {
			return after(r.tv.RenderMath(cx, b, true))
		}
		if caret {
			return core.Text{Spans: r.caretSpans([]core.Span{{Content: b.TeX}}, true), Class: r.class(PartMathBlock), StyleFunc: func() core.Style { return r.style(PartMathBlock) }}
		}
		return core.Text{Content: b.TeX, Class: r.class(PartMathBlock), StyleFunc: func() core.Style { return r.style(PartMathBlock) }}
	case Custom:
		if n := r.custom(cx, b); n != nil {
			return after(n)
		}
		switch {
		case len(b.Children) > 0:
			return core.View{Class: r.class(PartCustom), StyleFunc: func() core.Style { return r.style(PartCustom) }, Children: r.blocksAt(cx, b.Children, depth, caret)}
		case len(b.Content) > 0:
			return core.Text{Spans: r.caretSpans(r.spans(nil, b.Content, noStyle, "", nil), caret), Class: r.class(PartCustom), StyleFunc: func() core.Style { return r.style(PartCustom) }}
		}
		if caret {
			return core.Text{Spans: r.caretSpans([]core.Span{{Content: b.Text}}, true), Class: r.class(PartCustom), StyleFunc: func() core.Style { return r.style(PartCustom) }}
		}
		return core.Text{Content: b.Text, Class: r.class(PartCustom), StyleFunc: func() core.Style { return r.style(PartCustom) }}
	}
	return nil
}

// renders reports whether a plugin renders custom nodes named name.
func (r *renderer) renders(name string) bool {
	for _, p := range r.plugins {
		if p.Render != nil && p.Extension.Name == name {
			return true
		}
	}
	return false
}

// custom shows a custom node through the plugins' renderers, then
// RenderCustom; nil when neither shows it.
func (r *renderer) custom(cx *core.Scope, b Custom) core.Node {
	for _, p := range r.plugins {
		if p.Render != nil && p.Extension.Name == b.Name {
			if n := p.Render(cx, b); n != nil {
				return n
			}
		}
	}
	if r.tv.RenderCustom != nil {
		return r.tv.RenderCustom(cx, b)
	}
	return nil
}

// details shows a disclosure: its summary, which a press opens and
// closes, and its blocks, made each time it opens.
func (r *renderer) details(cx *core.Scope, d Details, depth int) core.Node {
	open := core.NewSignal(cx, d.Open)
	return core.View{Class: r.class(PartDetails), StyleFunc: func() core.Style { return r.style(PartDetails) }, Children: []core.Node{
		core.View{Class: r.class(PartSummary), StyleFunc: func() core.Style { return r.style(PartSummary) }, Role: core.RoleButton, OnPress: func(core.PressEvent) { open.Set(!open.Get()) },
			Children: []core.Node{
				core.Text{ContentFunc: func() string {
					if open.Get() {
						return "▾"
					}
					return "▸"
				}, Class: r.class(PartSummaryMarker), StyleFunc: func() core.Style { return r.style(PartSummaryMarker) }},
				core.Text{Spans: r.spans(nil, d.Summary, noStyle, "", nil), Style: core.Style{FlexGrow: 1, FlexShrink: 1}},
			}},
		core.Show{When: open.Get, Then: func(cx *core.Scope) core.Node {
			return core.Group(r.blocks(cx, d.Children, depth))
		}},
	}}
}

var bullets = [...]string{"•", "◦", "▪"}

func (r *renderer) list(cx *core.Scope, l List, depth int, caret bool) core.Node {
	kind := PartUnorderedList
	if l.Ordered {
		kind = PartOrderedList
	}
	items := make([]core.Node, len(l.Items))
	for i, it := range l.Items {
		var marker core.Node
		switch {
		case it.Task:
			parts := []Part{PartTaskMarker}
			check := ""
			if it.Done {
				parts = append(parts, PartTaskMarkerChecked)
				check = "✓"
			}
			done := it.Done
			marker = core.View{Class: r.class(parts...), StyleFunc: func() core.Style { return r.style(parts...) }, Role: core.RoleCheckbox, Checked: func() bool { return done },
				Children: []core.Node{core.Text{Content: check}}}
		case l.Ordered:
			marker = core.Text{Content: strconv.Itoa(l.Start+i) + ".", Class: r.class(PartListMarker), StyleFunc: func() core.Style { return r.style(PartListMarker) }}
		default:
			marker = core.Text{Content: bullets[depth%len(bullets)], Class: r.class(PartListMarker), StyleFunc: func() core.Style { return r.style(PartListMarker) }}
		}
		content := []Part{PartListItemContent}
		if it.Task && it.Done {
			content = append(content, PartTaskDone)
		}
		items[i] = core.View{Class: r.class(PartListItem), StyleFunc: func() core.Style { return r.style(PartListItem) }, Children: []core.Node{
			marker,
			core.View{Class: r.class(content...), StyleFunc: func() core.Style { return r.style(content...) }, Children: r.blocksAt(cx, it.Children, depth+1, caret && i == len(l.Items)-1)},
		}}
	}
	return core.View{Class: r.class(PartList, kind), StyleFunc: func() core.Style { return r.style(PartList, kind) }, Children: items}
}

func (r *renderer) code(cx *core.Scope, c CodeBlock, caret bool) core.Node {
	spans := func() []core.Span {
		var highlights []Highlight
		if r.tv.Highlighter != nil {
			highlights = r.tv.Highlighter(c)
		}
		return codeSpans(c.Code, highlights)
	}
	// The code recolors in place as the highlighter's colors change (the
	// appearance, a lexer done); the caret is an attachment, which only
	// Spans carries, so a block streaming in is built again instead.
	code := core.Text{SpansFunc: spans, Class: r.class(PartCodeBlockCode), StyleFunc: func() core.Style { return r.style(PartCodeBlockCode) }}
	if caret {
		code.SpansFunc, code.Spans = nil, r.caretSpans(spans(), true)
	}
	children := []core.Node{core.Scroll{Horizontal: true, HideIndicator: r.tv.NoCodeBlockScrollbar,
		Class: r.class(PartCodeBlockScroll), StyleFunc: func() core.Style { return r.style(PartCodeBlockScroll) }, Children: []core.Node{code}}}
	if len(r.tv.CodeBlockActions) > 0 {
		actions := make([]core.Node, len(r.tv.CodeBlockActions))
		for i, name := range r.tv.CodeBlockActions {
			actions[i] = r.codeAction(cx, name, c)
		}
		children = append(children, core.View{
			Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Direction: core.Row},
			Class: r.class(PartCodeBlockHeader), StyleFunc: func() core.Style { return r.style(PartCodeBlockHeader) },
			Children: actions,
		})
	}
	return core.View{Class: r.class(PartCodeBlock), StyleFunc: func() core.Style { return r.style(PartCodeBlock) }, Children: children}
}

// codeAction is a code block's action button.
func (r *renderer) codeAction(cx *core.Scope, name string, c CodeBlock) core.Node {
	done := core.NewSignal(cx, false)
	var stop func() bool
	a := CodeBlockAction{Name: name, Code: c, Done: done.Get}
	var content core.Node = core.Text{Content: name}
	if r.tv.RenderCodeBlockAction != nil {
		content = r.tv.RenderCodeBlockAction(cx, a)
	}
	return core.View{Class: r.class(PartCodeBlockAction), StyleFunc: func() core.Style { return r.style(PartCodeBlockAction) }, Role: core.RoleButton, Label: name,
		OnPress: func(core.PressEvent) {
			if name == CopyAction {
				cx.SetClipboard(c.Code)
				if stop != nil {
					stop()
				}
				done.Set(true)
				stop = cx.AfterFunc(copiedFor, func() { done.Set(false) })
			}
			if r.tv.OnCodeBlockAction != nil {
				r.tv.OnCodeBlockAction(name, c)
			}
		},
		Children: []core.Node{content}}
}

// codeSpans splits code into spans at the highlights that are in order,
// apart and on character boundaries.
func codeSpans(code string, highlights []Highlight) []core.Span {
	var out []core.Span
	at := 0
	boundary := func(i int) bool { return i == len(code) || utf8.RuneStart(code[i]) }
	for _, h := range highlights {
		if h.Start < at || h.End <= h.Start || h.End > len(code) || !boundary(h.Start) || !boundary(h.End) {
			continue
		}
		if h.Start > at {
			out = append(out, core.Span{Content: code[at:h.Start]})
		}
		out = append(out, core.Span{Content: code[h.Start:h.End], Class: h.Class, Style: h.Style})
		at = h.End
	}
	if at < len(code) || len(out) == 0 {
		out = append(out, core.Span{Content: code[at:]})
	}
	return out
}

// table lays the cells out as one grid, so columns line up across rows:
// each column fits its content when there is room and shrinks, its cells
// wrapping, down to a floor estimated from its text; below the floors the
// grid scrolls.
func (r *renderer) table(cx *core.Scope, t Table, caret bool) core.Node {
	cols := t.Columns()
	// The floors are estimates, made with the font size at hand.
	em := core.Untracked(cx, func() core.Points { return r.style(PartTableCell).FontSize })
	if em == 0 {
		em = core.DefaultFontSize
	}
	floors := make([]float64, cols)
	rows := append([][]TableCell{t.Head}, t.Rows...)
	for _, row := range rows {
		col := 0
		for _, c := range row {
			if c.span() == 1 && col < cols {
				floors[col] = max(floors[col], textWidth(PlainText(c.Content)))
			}
			col += c.span()
		}
	}
	var tracks strings.Builder
	var total float64
	for i, f := range floors {
		f = min(f+2, min(max(f/2, 10), 30)) * float64(em)
		total += f
		if i > 0 {
			tracks.WriteByte(' ')
		}
		fmt.Fprintf(&tracks, "minmax(%gpx, auto)", f)
	}
	var children []core.Node
	for i, row := range rows {
		line := core.GridLines{Start: core.GridLine(i + 1)}
		rowParts, cellParts := []Part{PartTableRow}, []Part{PartTableCell}
		if i == 0 {
			rowParts, cellParts = append(rowParts, PartTableHead), append(cellParts, PartTableHeaderCell)
		}
		children = append(children, core.View{Class: r.class(rowParts...), StyleFunc: func() core.Style {
			return r.style(rowParts...).Merge(core.Style{GridRow: line, GridColumn: core.GridLines{Start: core.GridLine(1), End: core.GridLine(-1)}})
		}})
		col := 0
		for _, c := range row {
			at, span := col, c.span()
			s := func() core.Style {
				s := r.style(cellParts...)
				if at < len(t.Align) {
					s.TextAlign = [...]core.TextAlign{AlignNone: s.TextAlign, AlignLeft: core.TextAlignStart,
						AlignCenter: core.TextAlignCenter, AlignRight: core.TextAlignEnd}[t.Align[at]]
				}
				s.GridRow = line
				s.GridColumn = core.GridLines{Start: core.GridLine(at + 1), End: core.GridSpan(span)}
				return s
			}
			lastCell := caret && i == len(rows)-1 && col+c.span() >= cols
			children = append(children, core.Text{Spans: r.caretSpans(r.spans(nil, c.Content, noStyle, "", nil), lastCell), Class: r.class(cellParts...), StyleFunc: s})
			col += c.span()
		}
	}
	grid := core.Style{Display: core.DisplayGrid, GridTemplateColumns: core.GridTracks(tracks.String()), Width: core.Pct(100)}
	var body core.Node
	if r.tv.NoTableScroll {
		body = core.View{Style: grid, Children: children}
	} else {
		grid.MinWidth = core.Pt(core.Points(total))
		body = core.Scroll{Horizontal: true, HideIndicator: r.tv.NoTableScrollbar, Class: r.class(PartTableScroll), StyleFunc: func() core.Style { return r.style(PartTableScroll) },
			Children: []core.Node{core.View{Style: grid, Children: children}}}
	}
	nodes := []core.Node{body}
	if r.tv.RenderTableActions != nil {
		nodes = []core.Node{core.View{Class: r.class(PartTableActions), StyleFunc: func() core.Style { return r.style(PartTableActions) }, Children: []core.Node{r.tv.RenderTableActions(cx, t)}}, body}
	}
	return core.View{Class: r.class(PartTable), StyleFunc: func() core.Style { return r.style(PartTable) }, Children: nodes}
}

// textWidth estimates the width of text's widest line in ems: wide (East
// Asian) characters an em, spaces a third, the rest a little over half.
func textWidth(text string) float64 {
	var widest float64
	for line := range strings.SplitSeq(text, "\n") {
		var w float64
		for _, c := range line {
			switch {
			case c == ' ':
				w += 0.3
			case c >= 0x1100 && c <= 0x115f, c >= 0x2e80 && c <= 0xa4cf, c >= 0xac00 && c <= 0xd7a3,
				c >= 0xf900 && c <= 0xfaff, c >= 0xfe30 && c <= 0xfe4f, c >= 0xff00 && c <= 0xff60,
				c >= 0xffe0 && c <= 0xffe6, c >= 0x1f300 && c <= 0x1faff, c >= 0x20000 && c <= 0x3fffd:
				w++
			default:
				w += 0.6
			}
		}
		widest = max(widest, w)
	}
	return widest
}

func (r *renderer) footnotes(cx *core.Scope, notes []Footnote) core.Node {
	if len(notes) == 0 {
		return nil
	}
	rows := make([]core.Node, len(notes))
	for i, f := range notes {
		rows[i] = core.View{Class: r.class(PartFootnote), StyleFunc: func() core.Style { return r.style(PartFootnote) }, Children: []core.Node{
			core.Text{Content: strconv.Itoa(f.Index) + ".", Class: r.class(PartFootnoteLabel), StyleFunc: func() core.Style { return r.style(PartFootnoteLabel) }},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1}, Children: r.blocks(cx, f.Children, 0)},
		}}
	}
	return core.View{Class: r.class(PartFootnotes), StyleFunc: func() core.Style { return r.style(PartFootnotes) }, Children: rows}
}

// spans appends content's spans to out, each styled over style and
// class, the inline parts around it, and pressing link, the link around
// it, if any.
func (r *renderer) spans(out []core.Span, content []Inline, style func() core.Style, class string, link func(core.PressEvent)) []core.Span {
	// under is style with part p's over it, and inner class with p's.
	under := func(p Part) func() core.Style {
		return func() core.Style { return style().Merge(r.style(p)) }
	}
	inner := func(p Part) string { return joinClasses(class, r.class(p)) }
	for _, in := range content {
		switch in := in.(type) {
		case Text:
			out = append(out, core.Span{Content: in.Text, Class: class, StyleFunc: style, OnPress: link})
		case Strong:
			out = r.spans(out, in.Content, under(PartStrong), inner(PartStrong), link)
		case Emphasis:
			out = r.spans(out, in.Content, under(PartEmphasis), inner(PartEmphasis), link)
		case Strikethrough:
			out = r.spans(out, in.Content, under(PartStrikethrough), inner(PartStrikethrough), link)
		case Underline:
			out = r.spans(out, in.Content, under(PartUnderline), inner(PartUnderline), link)
		case Mark:
			mark := func() core.Style {
				s := style().Merge(r.style(PartMark))
				if in.Color != 0 {
					s.Background = in.Color
				}
				return s
			}
			out = r.spans(out, in.Content, mark, inner(PartMark), link)
		case Superscript:
			out = r.spans(out, in.Content, under(PartSuperscript), inner(PartSuperscript), link)
		case Subscript:
			out = r.spans(out, in.Content, under(PartSubscript), inner(PartSubscript), link)
		case Small:
			out = r.spans(out, in.Content, under(PartSmall), inner(PartSmall), link)
		case Code:
			out = append(out, core.Span{Content: in.Code, Class: inner(PartCode), StyleFunc: under(PartCode), OnPress: link})
		case Keyboard:
			out = append(out, core.Span{Content: in.Text, Class: inner(PartKeyboard), StyleFunc: under(PartKeyboard), OnPress: link})
		case Math:
			if render := r.tv.RenderMath; render != nil {
				out = append(out, core.Span{Attachment: core.Func(func(cx *core.Scope) core.Node { return render(cx, in, false) })})
			} else {
				out = append(out, core.Span{Content: in.TeX, Class: inner(PartMath), StyleFunc: under(PartMath), OnPress: link})
			}
		case Link:
			out = r.spans(out, in.Content, under(PartLink), inner(PartLink), r.press(in.URL))
		case Image:
			out = append(out, core.Span{Attachment: r.inlineImage(in, style, link)})
		case Break:
			out = append(out, core.Span{Content: "\n", Class: class, StyleFunc: style})
		case FootnoteReference:
			out = append(out, core.Span{Content: "[" + strconv.Itoa(in.Index) + "]", Class: inner(PartFootnoteReference),
				StyleFunc: under(PartFootnoteReference), OnPress: r.press("#footnote-" + in.Label)})
		case Custom:
			if r.tv.CustomSpan != nil {
				s := r.tv.CustomSpan(in)
				own, ownFunc := s.Style, s.StyleFunc
				s.Class = joinClasses(class, s.Class)
				s.Style, s.StyleFunc = core.Style{}, func() core.Style {
					st := style().Merge(own)
					if ownFunc != nil {
						st = st.Merge(ownFunc())
					}
					return st
				}
				out = append(out, s)
			} else if r.tv.RenderCustom != nil || r.renders(in.Name) {
				out = append(out, core.Span{Attachment: core.Func(func(cx *core.Scope) core.Node { return r.custom(cx, in) })})
			} else {
				var b strings.Builder
				in.write(&b)
				out = append(out, core.Span{Content: b.String(), Class: inner(PartCustom), StyleFunc: under(PartCustom), OnPress: link})
			}
		}
	}
	return out
}

// press returns what a press on a link to url does, if anything: report
// it, then with OpenLinks open a URL that leaves the document.
func (r *renderer) press(link string) func(core.PressEvent) {
	open := r.tv.OpenLinks && leaves(link)
	if r.tv.OnLinkPress == nil && !open {
		return nil
	}
	return func(core.PressEvent) {
		if r.tv.OnLinkPress != nil {
			r.tv.OnLinkPress(link)
		}
		if open {
			r.cx.OpenURL(link, nil)
		}
	}
}

// leaves reports whether link names another site or app: it has a
// scheme.
func leaves(link string) bool {
	u, err := url.Parse(link)
	return err == nil && u.Scheme != ""
}

// imagePress returns what a press on an image in no link does: report it
// with OnImagePress.
func (r *renderer) imagePress(im Image) func(core.PressEvent) {
	if r.tv.OnImagePress == nil {
		return nil
	}
	return func(core.PressEvent) { r.tv.OnImagePress(im) }
}

type linkedImage struct {
	image Image
	url   string
}

// onlyImages returns a paragraph's images when it holds nothing else
// (links around them aside): they show as blocks.
func onlyImages(content []Inline) ([]linkedImage, bool) {
	var out []linkedImage
	for _, in := range content {
		switch in := in.(type) {
		case Image:
			out = append(out, linkedImage{image: in})
		case Link:
			if len(in.Content) != 1 {
				return nil, false
			}
			im, ok := in.Content[0].(Image)
			if !ok {
				return nil, false
			}
			out = append(out, linkedImage{image: im, url: in.URL})
		default:
			return nil, false
		}
	}
	return out, len(out) > 0
}

// inlineImage shows an image in a line of text styled style: the size it
// asks for, else three quarters of a line tall at its natural aspect.
func (r *renderer) inlineImage(im Image, style func() core.Style, link func(core.PressEvent)) core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		if r.tv.RenderImage != nil {
			return r.tv.RenderImage(cx, im)
		}
		natural := core.NewSignal(cx, core.Size{})
		press := link
		if press == nil {
			press = r.imagePress(im)
		}
		return core.Image{
			Source:  core.Src(im.Src),
			Label:   im.Alt,
			OnLoad:  natural.Set,
			OnPress: press,
			StyleFunc: func() core.Style {
				text := r.style(PartRoot, PartParagraph).Merge(style())
				line := text.LineHeight
				if size := text.FontSize; line == 0 && size > 0 {
					line = size * 1.2
				} else if line == 0 {
					line = core.DefaultFontSize * 1.2
				}
				s := core.Style{Width: im.Width, Height: im.Height}
				if s.Width.IsAuto() && s.Height.IsAuto() {
					s.Height = core.Pt(line * 0.75)
				}
				if n := natural.Get(); n.Width > 0 && n.Height > 0 {
					s.AspectRatio = float32(n.Width / n.Height)
				} else if s.Width.IsAuto() {
					s.Width = s.Height
				}
				return s
			},
		}
	})
}

// image shows an image as a block: the size it asks for, else its
// natural size once loaded, at most the text's width.
func (r *renderer) image(cx *core.Scope, im Image, url string) core.Node {
	if r.tv.RenderImage != nil {
		return r.tv.RenderImage(cx, im)
	}
	natural := core.NewSignal(cx, core.Size{})
	press := r.imagePress(im)
	if url != "" {
		press = r.press(url)
	}
	return core.Image{
		Source: core.Src(im.Src),
		Label:  im.Alt,
		OnLoad: natural.Set,
		StyleFunc: func() core.Style {
			s := core.Style{MaxWidth: core.Pct(100), AlignSelf: core.AlignStart, Width: im.Width, Height: im.Height}
			if n := natural.Get(); n.Width > 0 && n.Height > 0 {
				s.AspectRatio = float32(n.Width / n.Height)
				if im.Width.IsAuto() && im.Height.IsAuto() {
					s.Width = core.Pt(n.Width)
				}
			}
			return s.Merge(r.style(PartImage))
		},
		Class:   r.class(PartImage),
		OnPress: press,
	}
}
