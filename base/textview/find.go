package textview

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/core"
)

// TextRef identifies a text-bearing block in a Document. Block is the path
// from the document's top-level block; Footnote is -1 for the main document.
type TextRef struct {
	Block    []int
	Footnote int
}

// Match is a byte range in one text-bearing block.
type Match struct {
	Text     TextRef
	From, To int
}

// FindOptions tune Document.Find.
type FindOptions struct {
	// CaseSensitive preserves the query's case. Without it, ASCII letters
	// compare without case while non-ASCII text remains unchanged.
	CaseSensitive bool
	// WholeWord requires a word boundary on both sides of the match.
	WholeWord bool
}

// Search is a query shared by one or more TextViews. TextViews register their
// current matches under a stable key while rendering; the selected index then
// follows those matches in document order.
type Search struct {
	query    *core.Signal[string]
	open     *core.Signal[bool]
	selected *core.Signal[int]
	count    *core.Signal[int]
	entries  map[string][]Match
}

// NewSearch creates a search state in cx's scope.
func NewSearch(cx *core.Scope) *Search {
	return &Search{query: core.NewSignal(cx, ""), open: core.NewSignal(cx, false), selected: core.NewSignal(cx, 0), count: core.NewSignal(cx, 0), entries: make(map[string][]Match)}
}

// SetQuery changes the query and opens the search when q is nonempty.
func (s *Search) SetQuery(q string) {
	s.query.Set(q)
	s.open.Set(q != "")
	s.selected.Set(0)
}

// Query returns the current query.
func (s *Search) Query() string { return s.query.Get() }

// Open reports whether the search is shown and its matches highlighted.
func (s *Search) Open() bool { return s.open.Get() }

// Close hides the search and clears its selected match.
func (s *Search) Close() {
	s.open.Set(false)
	s.selected.Set(0)
	s.count.Set(0)
}

// Count returns the number of matches in registered views.
func (s *Search) Count() int { return s.count.Get() }

// Selected returns the one-based selected match, or zero when there is none.
func (s *Search) Selected() int { return s.selected.Get() }

// Next selects the next match, wrapping at the end.
func (s *Search) Next() { s.step(1) }

// Previous selects the previous match, wrapping at the beginning.
func (s *Search) Previous() { s.step(-1) }

func (s *Search) step(delta int) {
	n := s.count.GetUntracked()
	if n == 0 {
		s.selected.Set(0)
		return
	}
	i := s.selected.GetUntracked() + delta
	if i < 1 {
		i = n
	} else if i > n {
		i = 1
	}
	s.selected.Set(i)
}

func (s *Search) register(key string, matches []Match) []Match {
	if key == "" {
		key = "default"
	}
	if !s.Open() {
		matches = nil
	}
	s.entries[key] = matches
	total := 0
	for _, ms := range s.entries {
		total += len(ms)
	}
	s.count.Set(total)
	if total == 0 {
		s.selected.Set(0)
	} else if i := s.selected.GetUntracked(); i < 1 || i > total {
		s.selected.Set(1)
	}
	return matches
}

func (s *Search) isSelected(key string, m Match) bool {
	if key == "" {
		key = "default"
	}
	want := s.selected.Get()
	if want == 0 {
		return false
	}
	i := 0
	for k, ms := range s.entries {
		for _, x := range ms {
			i++
			if k == key && x.Text.Footnote == m.Text.Footnote && slices.Equal(x.Text.Block, m.Text.Block) && x.From == m.From && x.To == m.To {
				return i == want
			}
		}
	}
	return false
}

// Find returns matches in the order the document shows them. A match never
// crosses a block, table cell or nested block. Images are excluded; their alt
// text is not part of rendered prose.
func (d Document) Find(query string, opts FindOptions) []Match {
	if query == "" {
		return nil
	}
	var out []Match
	for i, b := range d.Blocks {
		out = append(out, findBlock(nil, b, []int{i}, -1, query, opts)...)
	}
	for i, f := range d.Footnotes {
		for j, b := range f.Children {
			out = append(out, findBlock(nil, b, []int{j}, i, query, opts)...)
		}
	}
	return out
}

func findBlock(out []Match, b Block, path []int, footnote int, query string, opts FindOptions) []Match {
	appendText := func(text string) {
		for _, m := range findText(text, query, opts) {
			out = append(out, Match{Text: TextRef{Block: append([]int(nil), path...), Footnote: footnote}, From: m[0], To: m[1]})
		}
	}
	switch b := b.(type) {
	case Paragraph:
		appendText(plainTextWithoutImages(b.Content))
	case Heading:
		appendText(plainTextWithoutImages(b.Content))
	case CodeBlock:
		appendText(b.Code)
	case Math:
		appendText(b.TeX)
	case Blockquote:
		for i, child := range b.Children {
			out = append(out, findBlock(nil, child, append(path, i), footnote, query, opts)...)
		}
	case List:
		for i, item := range b.Items {
			for j, child := range item.Children {
				out = append(out, findBlock(nil, child, append(append(path, i), j), footnote, query, opts)...)
			}
		}
	case Table:
		rows := append([][]TableCell{b.Head}, b.Rows...)
		for i, row := range rows {
			for j, cell := range row {
				appendTextAt(&out, plainTextWithoutImages(cell.Content), append(append(path, i), j), footnote, query, opts)
			}
		}
	case Details:
		appendText(plainTextWithoutImages(b.Summary))
		for i, child := range b.Children {
			out = append(out, findBlock(nil, child, append(path, i), footnote, query, opts)...)
		}
	case Custom:
		if len(b.Content) > 0 {
			appendText(plainTextWithoutImages(b.Content))
		} else if len(b.Children) > 0 {
			for i, child := range b.Children {
				out = append(out, findBlock(nil, child, append(path, i), footnote, query, opts)...)
			}
		} else {
			appendText(b.Text)
		}
	}
	return out
}

func appendTextAt(out *[]Match, text string, path []int, footnote int, query string, opts FindOptions) {
	for _, m := range findText(text, query, opts) {
		*out = append(*out, Match{Text: TextRef{Block: append([]int(nil), path...), Footnote: footnote}, From: m[0], To: m[1]})
	}
}

// plainTextWithoutImages follows the document's visible text while leaving
// image descriptions out of search results.
func plainTextWithoutImages(content []Inline) string {
	var b strings.Builder
	writeSearchInlines(&b, content)
	return b.String()
}

func writeSearchInlines(b *strings.Builder, content []Inline) {
	for _, in := range content {
		switch in := in.(type) {
		case Text:
			b.WriteString(in.Text)
		case Code:
			b.WriteString(in.Code)
		case Strong:
			writeSearchInlines(b, in.Content)
		case Emphasis:
			writeSearchInlines(b, in.Content)
		case Strikethrough:
			writeSearchInlines(b, in.Content)
		case Underline:
			writeSearchInlines(b, in.Content)
		case Mark:
			writeSearchInlines(b, in.Content)
		case Superscript:
			writeSearchInlines(b, in.Content)
		case Subscript:
			writeSearchInlines(b, in.Content)
		case Small:
			writeSearchInlines(b, in.Content)
		case Keyboard:
			b.WriteString(in.Text)
		case Math:
			b.WriteString(in.TeX)
		case Link:
			writeSearchInlines(b, in.Content)
		case Break:
			b.WriteByte('\n')
		case FootnoteReference:
			// Footnote references are navigation markers, not searchable prose.
		case Custom:
			if len(in.Content) > 0 {
				writeSearchInlines(b, in.Content)
			} else {
				b.WriteString(in.Text)
			}
		}
	}
}

func findText(text, query string, opts FindOptions) [][2]int {
	if query == "" || len(query) > len(text) {
		return nil
	}
	needle := query
	input := text
	if !opts.CaseSensitive {
		needle = foldASCII(needle)
		input = foldASCII(input)
	}
	var out [][2]int
	for at := 0; ; {
		i := strings.Index(input[at:], needle)
		if i < 0 {
			break
		}
		i += at
		end := i + len(needle)
		if !opts.WholeWord || (wordBoundary(text, i) && wordBoundary(text, end)) {
			out = append(out, [2]int{i, end})
		}
		at = max(i+1, end)
		if at > len(input) {
			break
		}
	}
	return out
}

func foldASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func wordBoundary(s string, at int) bool {
	if at <= 0 || at >= len(s) {
		return true
	}
	left, _ := decodeLastRune(s[:at])
	right, _ := decodeFirstRune(s[at:])
	return !isWord(left) || !isWord(right)
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

func decodeLastRune(s string) (rune, int) {
	return utf8.DecodeLastRuneInString(s)
}

func decodeFirstRune(s string) (rune, int) {
	return utf8.DecodeRuneInString(s)
}
