// Package textview shows a document of Markdown or the app's own blocks
// with the primitives: texts with spans, views, scroll views and images.
// It is NSH's TextView: made for documents and the replies of AI models,
// with code, tables, footnotes and the app's own syntax.
//
// TextView has no look, as the rest of base: it takes a style per Part.
package textview

import (
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
)

// Document is what a TextView shows: its blocks, then its footnotes.
type Document struct {
	Blocks []Block
	// Footnotes are the notes the blocks refer to, in the order of their
	// first reference.
	Footnotes []Footnote
}

// PlainText returns the document as text, what copying all of it gives:
// blocks apart by a blank line, list items and table rows by a line break,
// table cells by a tab.
func (d Document) PlainText() string {
	var b strings.Builder
	blocks := d.Blocks
	for _, f := range d.Footnotes {
		blocks = append(blocks[:len(blocks):len(blocks)], f.Children...)
	}
	writeBlocks(&b, blocks, "\n\n")
	return b.String()
}

// Block is a block of a document: Paragraph, Heading, Blockquote, List,
// CodeBlock, Table, Rule, Details, Math or Custom.
type Block interface {
	// Kind names the block's type ("paragraph", "codeBlock"), or a custom
	// node's name.
	Kind() string
	block()
}

// Inline is content of a block's text: Text, Strong, Emphasis,
// Strikethrough, Underline, Mark, Superscript, Subscript, Small,
// Code, Keyboard, Math, Link, Image, Break, FootnoteReference or Custom.
type Inline interface{ inline() }

// Paragraph is a block of text.
type Paragraph struct{ Content []Inline }

// Heading is a title of level 1 to 6. Anchor is its slug, as GitHub makes
// it ("getting-started"), unique in its document.
type Heading struct {
	Level   int
	Content []Inline
	Anchor  string
}

// Blockquote is a quotation, or one of GitHub's alerts (> [!NOTE]).
type Blockquote struct {
	Children []Block
	Alert    Alert
}

// Alert is the kind of a GitHub alert; AlertNone is a plain quotation.
type Alert uint8

const (
	AlertNone Alert = iota
	// AlertNote is useful information.
	AlertNote
	// AlertTip is advice.
	AlertTip
	// AlertImportant is what must not be missed.
	AlertImportant
	// AlertWarning is what needs attention at once.
	AlertWarning
	// AlertCaution is the risks of an action.
	AlertCaution
)

// String returns the alert's title in English: "Note", "Tip", ... The
// view says it in the language the app shows.
func (a Alert) String() string {
	switch a {
	case AlertNote:
		return "Note"
	case AlertTip:
		return "Tip"
	case AlertImportant:
		return "Important"
	case AlertWarning:
		return "Warning"
	case AlertCaution:
		return "Caution"
	}
	return ""
}

// List is a bulleted or numbered list. Start is an ordered list's first
// number; Loose lists have blank lines between their items.
type List struct {
	Ordered bool
	Start   int
	Loose   bool
	Items   []ListItem
}

// ListItem is an item of a list. Task items (- [ ], - [x]) have a box,
// checked when Done.
type ListItem struct {
	Task, Done bool
	Children   []Block
}

// CodeBlock is a block of code. Language is the info string's first word
// ("go"), Meta the rest; Code has no final line break.
type CodeBlock struct {
	Language, Meta string
	Code           string
}

// Table is a table: its column alignments, its header row and its body.
type Table struct {
	Align []ColumnAlign
	Head  []TableCell
	Rows  [][]TableCell
}

// Columns returns the number of columns: the widest row's.
func (t Table) Columns() int {
	width := func(row []TableCell) int {
		n := 0
		for _, c := range row {
			n += c.span()
		}
		return n
	}
	n := max(len(t.Align), width(t.Head))
	for _, r := range t.Rows {
		n = max(n, width(r))
	}
	return n
}

// TableCell is a cell of a table. ColSpan is the columns it spans; 0 is
// one.
type TableCell struct {
	Content []Inline
	ColSpan int
}

func (c TableCell) span() int { return max(c.ColSpan, 1) }

// ColumnAlign aligns a table's column.
type ColumnAlign uint8

const (
	AlignNone ColumnAlign = iota
	AlignLeft
	AlignCenter
	AlignRight
)

// Rule is a thematic break.
type Rule struct{}

// Details is a disclosure (<details>): its summary, always shown, and its
// blocks, shown while open. Open is whether it starts open.
type Details struct {
	Open     bool
	Summary  []Inline
	Children []Block
}

// Math is a formula written in TeX, a block of its own or inline in
// text. HTML gives one for MathML with a TeX annotation, as KaTeX and
// MathJax write it.
type Math struct {
	TeX string
	// Display is set on a formula that is a block of its own, as
	// RenderMath gets it.
	Display bool
}

// Footnote is a footnote of a document: its label ("1", "note"), its
// number from 1 and its blocks.
type Footnote struct {
	Label    string
	Index    int
	Children []Block
}

// Custom is a node an Extension or the app made, block or inline, shown
// by the TextView's RenderCustom or CustomSpan, else as its content.
type Custom struct {
	// Name is the extension's ("mention", "chart").
	Name string
	// Attributes are a pattern's groups ("1", or by name), a container's
	// label and attributes, a fence's "meta".
	Attributes map[string]string
	// Text is a pattern's match, or a fence's code.
	Text string
	// Data is the app's own.
	Data any
	// Children are a container's blocks; Content inline content.
	Children []Block
	Content  []Inline
}

// Text is plain text.
type Text struct{ Text string }

// Strong is strong text, as **this**.
type Strong struct{ Content []Inline }

// Emphasis is emphasized text, as *this*.
type Emphasis struct{ Content []Inline }

// Strikethrough is struck text, as ~~this~~.
type Strikethrough struct{ Content []Inline }

// Underline is underlined text, as <u>this</u>.
type Underline struct{ Content []Inline }

// Mark is marked text, as <mark>this</mark>. Color is the background it
// asks for; zero is the view's.
type Mark struct {
	Content []Inline
	Color   core.Color
}

// Superscript is text above the line, as <sup>2</sup>.
type Superscript struct{ Content []Inline }

// Subscript is text below the line, as <sub>2</sub>.
type Subscript struct{ Content []Inline }

// Small is smaller text, as <small>this</small>.
type Small struct{ Content []Inline }

// Code is code in text, as `this`.
type Code struct{ Code string }

// Keyboard is keys to press, as <kbd>Ctrl</kbd>+<kbd>C</kbd>.
type Keyboard struct{ Text string }

// Link is a link to URL.
type Link struct {
	URL, Title string
	Content    []Inline
}

// Image is an image: its URL, the text that describes it and its title.
// Width and Height are the size it asks for; unset is its natural size.
type Image struct {
	Src, Alt, Title string
	Width, Height   core.Length
}

// Break is a hard line break.
type Break struct{}

// FootnoteReference refers to a footnote by its label and number.
type FootnoteReference struct {
	Label string
	Index int
}

func (Paragraph) Kind() string  { return "paragraph" }
func (Heading) Kind() string    { return "heading" }
func (Blockquote) Kind() string { return "blockquote" }
func (List) Kind() string       { return "list" }
func (CodeBlock) Kind() string  { return "codeBlock" }
func (Table) Kind() string      { return "table" }
func (Rule) Kind() string       { return "rule" }
func (Details) Kind() string    { return "details" }
func (Math) Kind() string       { return "math" }
func (c Custom) Kind() string   { return c.Name }

func (Paragraph) block()  {}
func (Heading) block()    {}
func (Blockquote) block() {}
func (List) block()       {}
func (CodeBlock) block()  {}
func (Table) block()      {}
func (Rule) block()       {}
func (Details) block()    {}
func (Math) block()       {}
func (Custom) block()     {}

func (Text) inline()              {}
func (Strong) inline()            {}
func (Emphasis) inline()          {}
func (Strikethrough) inline()     {}
func (Underline) inline()         {}
func (Mark) inline()              {}
func (Superscript) inline()       {}
func (Subscript) inline()         {}
func (Small) inline()             {}
func (Code) inline()              {}
func (Keyboard) inline()          {}
func (Math) inline()              {}
func (Link) inline()              {}
func (Image) inline()             {}
func (Break) inline()             {}
func (FootnoteReference) inline() {}
func (Custom) inline()            {}

// PlainText returns content as text: images their description, a footnote
// reference its number in brackets.
func PlainText(content []Inline) string {
	var b strings.Builder
	writeInlines(&b, content)
	return b.String()
}

func writeBlocks(b *strings.Builder, blocks []Block, sep string) {
	for i, bl := range blocks {
		if i > 0 {
			b.WriteString(sep)
		}
		writeBlock(b, bl)
	}
}

func writeBlock(b *strings.Builder, bl Block) {
	switch bl := bl.(type) {
	case Paragraph:
		writeInlines(b, bl.Content)
	case Heading:
		writeInlines(b, bl.Content)
	case Blockquote:
		writeBlocks(b, bl.Children, "\n\n")
	case List:
		for i, it := range bl.Items {
			if i > 0 {
				b.WriteByte('\n')
			}
			writeBlocks(b, it.Children, "\n")
		}
	case CodeBlock:
		b.WriteString(bl.Code)
	case Table:
		for i, row := range append([][]TableCell{bl.Head}, bl.Rows...) {
			if i > 0 {
				b.WriteByte('\n')
			}
			for j, c := range row {
				if j > 0 {
					b.WriteByte('\t')
				}
				writeInlines(b, c.Content)
			}
		}
	case Details:
		writeInlines(b, bl.Summary)
		if len(bl.Summary) > 0 && len(bl.Children) > 0 {
			b.WriteString("\n\n")
		}
		writeBlocks(b, bl.Children, "\n\n")
	case Math:
		b.WriteString(bl.TeX)
	case Custom:
		bl.write(b)
	}
}

func (c Custom) write(b *strings.Builder) {
	switch {
	case len(c.Content) > 0:
		writeInlines(b, c.Content)
	case len(c.Children) > 0:
		writeBlocks(b, c.Children, "\n\n")
	default:
		b.WriteString(c.Text)
	}
}

func writeInlines(b *strings.Builder, content []Inline) {
	for _, in := range content {
		switch in := in.(type) {
		case Text:
			b.WriteString(in.Text)
		case Code:
			b.WriteString(in.Code)
		case Strong:
			writeInlines(b, in.Content)
		case Emphasis:
			writeInlines(b, in.Content)
		case Strikethrough:
			writeInlines(b, in.Content)
		case Underline:
			writeInlines(b, in.Content)
		case Mark:
			writeInlines(b, in.Content)
		case Superscript:
			writeInlines(b, in.Content)
		case Subscript:
			writeInlines(b, in.Content)
		case Small:
			writeInlines(b, in.Content)
		case Keyboard:
			b.WriteString(in.Text)
		case Math:
			b.WriteString(in.TeX)
		case Link:
			writeInlines(b, in.Content)
		case Image:
			b.WriteString(in.Alt)
		case Break:
			b.WriteByte('\n')
		case FootnoteReference:
			b.WriteString("[" + strconv.Itoa(in.Index) + "]")
		case Custom:
			in.write(b)
		}
	}
}
