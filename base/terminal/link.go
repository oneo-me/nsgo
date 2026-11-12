package terminal

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/base/terminal/internal/vt"
	"github.com/nsgo-dev/nsgo/core"
)

// linkRows bounds how many rows a link is looked for above and below the
// pointer's, through a line wrapping over them.
const linkRows = 32

// urlPattern matches URLs printed as text, of the schemes Ghostty's does
// that Scope.OpenURL opens. A URL stops at spaces, quotes and the
// punctuation and box drawing of tables and CJK text around it.
var urlPattern = regexp.MustCompile("(?i)\\b(?:(?:https?|ftp|ssh|git|gemini|gopher|ipfs|ipns)://|mailto:|tel:|magnet:\\?|news:)" +
	"[^\\s<>\"'`\\\\^|\\x{2500}-\\x{259f}，。；：！？、（）《》「」『』“”‘’【】…]+")

// link is a link the viewport shows.
type link struct {
	url   string
	spans []linkSpan
}

// linkSpan is a link's cells in a row of the viewport, x0 to x1 inclusive.
type linkSpan struct{ y, x0, x1 int }

// same reports whether l is o, in the same cells.
func (l link) same(o link) bool {
	return l.url == o.url && slices.Equal(l.spans, o.spans)
}

// cells returns the columns the link takes in row y of the viewport.
func (l link) cells(y int) (x0, x1 int, ok bool) {
	for _, s := range l.spans {
		if s.y == y {
			return s.x0, s.x1, true
		}
	}
	return 0, 0, false
}

// gridCell is a cell of the screen and its scrollback.
type gridCell struct{ x, y int }

// linkAt returns the link in a cell of the viewport: a program's hyperlink
// (OSC 8), or a URL printed as text, which may wrap over rows, some of them
// out of the viewport. Links OpenURL refuses are none.
func linkAt(term *vt.Terminal, col, row int) link {
	ref, ok := term.CellAt(col, row)
	if !ok {
		return link{}
	}
	y, ok := term.ScreenRow(&ref)
	if !ok {
		return link{}
	}
	text, cells := wrappedText(term, y)
	i := slices.Index(cells, gridCell{col, y})
	if i < 0 {
		return link{}
	}
	var uri string
	start, end := i, i+1
	if uri = term.Hyperlink(ref); uri != "" {
		same := func(c gridCell) bool {
			ref, ok := term.ScreenCellAt(c.x, c.y)
			return ok && term.Hyperlink(ref) == uri
		}
		for start > 0 && same(cells[start-1]) {
			start--
		}
		for end < len(cells) && same(cells[end]) {
			end++
		}
	} else if start, end, ok = findURL(text, i); ok {
		uri = text[start:end]
	}
	if uri == "" || core.CheckURL(uri) != nil {
		return link{}
	}
	_, rows := term.Size()
	return link{uri, linkSpans(cells[start:end], y-row, rows)}
}

// wrappedText returns the text of the line that screen row y is a row of,
// joining the rows it soft-wraps over, and the cell of each of its bytes.
func wrappedText(term *vt.Terminal, y int) (string, []gridCell) {
	first, last := y, y
	for first > 0 && y-first < linkRows && term.ScreenRowWraps(first-1) {
		first--
	}
	for last-y < linkRows && term.ScreenRowWraps(last) {
		last++
	}
	var b strings.Builder
	var cells []gridCell
	for row := first; row <= last; row++ {
		text, cols := term.ScreenRowText(row)
		for i, r := range text {
			n, _ := b.WriteRune(r)
			for range n {
				cells = append(cells, gridCell{cols[i], row})
			}
		}
	}
	return b.String(), cells
}

// findURL returns the bytes of the URL in text that byte i is in.
func findURL(text string, i int) (start, end int, ok bool) {
	for _, m := range urlPattern.FindAllStringIndex(text, -1) {
		if i < m[0] {
			break
		}
		if end := m[0] + len(trimURL(text[m[0]:m[1]])); i < end {
			return m[0], end, true
		}
	}
	return 0, 0, false
}

// trimURL drops what ends a sentence after a URL, and the closing brackets
// of what the URL is in: "(see http://a.b/c_(d))." keeps the one it opens.
func trimURL(s string) string {
	for s != "" {
		r, size := utf8.DecodeLastRuneInString(s)
		i := strings.IndexRune(")]}", r)
		switch {
		case strings.ContainsRune(".,;:!?*_~", r):
		case i >= 0 && strings.Count(s, string(r)) > strings.Count(s, string("([{"[i])):
		default:
			return s
		}
		s = s[:len(s)-size]
	}
	return s
}

// linkSpans returns the spans of cells in the viewport whose first row is
// screen row top.
func linkSpans(cells []gridCell, top, rows int) []linkSpan {
	var spans []linkSpan
	for _, c := range cells {
		y := c.y - top
		if y < 0 || y >= rows {
			continue
		}
		if n := len(spans); n > 0 && spans[n-1].y == y {
			spans[n-1].x0, spans[n-1].x1 = min(spans[n-1].x0, c.x), max(spans[n-1].x1, c.x)
			continue
		}
		spans = append(spans, linkSpan{y, c.x, c.x})
	}
	return spans
}
