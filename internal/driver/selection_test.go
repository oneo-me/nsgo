package driver

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// fakeTexts are texts set in a monospace font: each character is 8 pt
// wide, lines break at '\n' and are 20 pt high. Keys are the indices
// unless keys is set.
type fakeTexts struct {
	contents []string
	frames   []Bounds
	keys     []uint64
}

type frame struct{ x, y, w, h Points }

func newFake(texts ...any) *fakeTexts {
	f := &fakeTexts{}
	for i := 0; i < len(texts); i += 2 {
		r := texts[i+1].(frame)
		f.contents = append(f.contents, texts[i].(string))
		f.frames = append(f.frames, Bounds{Point{r.x, r.y}, Size{r.w, r.h}})
	}
	return f
}

func (f *fakeTexts) Len() int { return len(f.contents) }

func (f *fakeTexts) Content(i int) string { return f.contents[i] }

func (f *fakeTexts) Frame(i int) Bounds { return f.frames[i] }

func (f *fakeTexts) Key(i int) uint64 {
	if f.keys != nil {
		return f.keys[i]
	}
	return uint64(i)
}

func (f *fakeTexts) OffsetAt(i int, p Point) int {
	lines := strings.Split(f.contents[i], "\n")
	fr := f.frames[i]
	line := min(int(max(math.Floor(float64(p.Y-fr.Origin.Y)/20), 0)), len(lines)-1)
	column := int(max(math.Round(float64(p.X-fr.Origin.X)/8), 0))
	n := 0
	for _, l := range lines[:line] {
		n += utf16Len(l) + 1
	}
	for _, r := range lines[line] {
		if column == 0 {
			break
		}
		column--
		n += utf16Len(string(r))
	}
	return n
}

// column is three texts in a column: one line, two lines, one line, with
// gaps.
func column() *fakeTexts {
	return newFake(
		"Hello world", frame{0, 0, 200, 20},
		"Second line\nmore text", frame{0, 40, 200, 40},
		"Last", frame{0, 100, 200, 20},
	)
}

func at(x, y Points) Point { return Point{x, y} }

func pos(text, offset int) TextPosition { return TextPosition{text, offset} }

func wantBounds(t *testing.T, m *SelectionModel, start, end TextPosition) {
	t.Helper()
	s, e, ok := m.Bounds()
	if !ok || s != start || e != end {
		t.Fatalf("bounds = %v %v %v, want %v %v", s, e, ok, start, end)
	}
}

func wantText(t *testing.T, m *SelectionModel, texts SelectionTexts, want string) {
	t.Helper()
	if got := m.Text(texts); got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestSelectionOneClickPutsTheAnchor(t *testing.T) {
	texts := column()
	var m SelectionModel
	if !m.IsEmpty() || !m.Press(texts, at(17, 5), 1, false) || !m.IsEmpty() {
		t.Fatal("a click selects nothing but changes the selection")
	}
	wantBounds(t, &m, pos(0, 2), pos(0, 2))
	if m.Ranges(texts) != nil {
		t.Fatal("no ranges")
	}
	wantText(t, &m, texts, "")
}

func TestSelectionDragsAcrossTexts(t *testing.T) {
	texts := column()
	ranges := []SelectionRange{{0, 2, 11}, {1, 0, 21}, {2, 0, 3}}
	var m SelectionModel
	m.Press(texts, at(16, 5), 1, false)
	if !m.Drag(texts, at(24, 105)) {
		t.Fatal("drag changes nothing")
	}
	wantBounds(t, &m, pos(0, 2), pos(2, 3))
	if got := m.Ranges(texts); !reflect.DeepEqual(got, ranges) {
		t.Fatalf("ranges = %v", got)
	}
	wantText(t, &m, texts, "llo world\nSecond line\nmore text\nLas")
	if m.Drag(texts, at(24, 105)) {
		t.Fatal("the same point changes nothing")
	}

	m = SelectionModel{}
	m.Press(texts, at(24, 105), 1, false)
	m.Drag(texts, at(16, 5))
	wantBounds(t, &m, pos(0, 2), pos(2, 3))
	if got := m.Ranges(texts); !reflect.DeepEqual(got, ranges) {
		t.Fatalf("backward ranges = %v", got)
	}

	m = SelectionModel{}
	if m.Drag(texts, at(10, 10)) {
		t.Fatal("a drag without a press selects")
	}
}

func TestSelectionHitsBetweenAndOutsideRows(t *testing.T) {
	texts := column()
	var m SelectionModel
	m.Press(texts, at(0, 5), 1, false)
	m.Drag(texts, at(50, 25))
	wantBounds(t, &m, pos(0, 0), pos(0, 11))
	m.Drag(texts, at(50, 35))
	wantBounds(t, &m, pos(0, 0), pos(1, 0))
	// A tie goes to the text above.
	m.Drag(texts, at(50, 30))
	wantBounds(t, &m, pos(0, 0), pos(0, 11))

	m.Press(texts, at(40, 105), 1, false)
	m.Drag(texts, at(50, -10))
	wantBounds(t, &m, pos(0, 0), pos(2, 4))
	m.Press(texts, at(0, 5), 1, false)
	m.Drag(texts, at(10, 500))
	wantBounds(t, &m, pos(0, 0), pos(2, 4))
	wantText(t, &m, texts, "Hello world\nSecond line\nmore text\nLast")

	row := newFake("left", frame{0, 0, 32, 20}, "right", frame{100, 0, 40, 20})
	m.Press(row, at(8, 5), 1, false)
	m.Drag(row, at(50, 5))
	wantBounds(t, &m, pos(0, 1), pos(0, 4))
	m.Drag(row, at(90, 5))
	wantBounds(t, &m, pos(0, 1), pos(1, 0))
	m.Drag(row, at(300, 5))
	wantBounds(t, &m, pos(0, 1), pos(1, 5))
	wantText(t, &m, row, "eft\tright")
}

func TestSelectionByWordsAndParagraphs(t *testing.T) {
	texts := column()
	var m SelectionModel
	if !m.Press(texts, at(56, 5), 2, false) {
		t.Fatal("a double click selects nothing")
	}
	wantBounds(t, &m, pos(0, 6), pos(0, 11))
	wantText(t, &m, texts, "world")
	// Forward into the next text: to the end of the word there.
	m.Drag(texts, at(9, 45))
	wantBounds(t, &m, pos(0, 6), pos(1, 6))
	// Back before the anchor's word: from that word to the anchor's end.
	m.Drag(texts, at(8, 5))
	wantBounds(t, &m, pos(0, 0), pos(0, 11))
	// Back inside the anchor's word: just it.
	m.Drag(texts, at(64, 5))
	wantBounds(t, &m, pos(0, 6), pos(0, 11))

	// At a line's end: the word before.
	m.Press(texts, at(150, 45), 2, false)
	wantText(t, &m, texts, "line")
	m.Press(texts, at(150, 5), 2, false)
	wantText(t, &m, texts, "world")

	m.Press(texts, at(20, 65), 3, false)
	wantBounds(t, &m, pos(1, 0), pos(1, 21))
	m.Drag(texts, at(8, 105))
	wantBounds(t, &m, pos(1, 0), pos(2, 4))
	wantText(t, &m, texts, "Second line\nmore text\nLast")
	m.Drag(texts, at(8, 5))
	wantBounds(t, &m, pos(0, 0), pos(1, 21))

	if !m.SelectWordAt(texts, at(10, 65)) {
		t.Fatal("a long press selects nothing")
	}
	wantText(t, &m, texts, "more")
}

func TestSelectionShiftClickExtends(t *testing.T) {
	texts := column()
	var m SelectionModel
	m.Press(texts, at(16, 5), 1, false)
	if !m.Press(texts, at(24, 105), 1, true) {
		t.Fatal("Shift-click changes nothing")
	}
	wantBounds(t, &m, pos(0, 2), pos(2, 3))
	m.Press(texts, at(8, 5), 1, true)
	wantBounds(t, &m, pos(0, 1), pos(0, 2))

	// By words after a double click.
	m.Press(texts, at(56, 5), 2, false)
	m.Press(texts, at(8, 105), 1, true)
	wantBounds(t, &m, pos(0, 6), pos(2, 4))

	// Without a selection, a Shift-click is a click.
	m = SelectionModel{}
	m.Press(texts, at(16, 5), 1, true)
	wantBounds(t, &m, pos(0, 2), pos(0, 2))
}

func TestSelectionHandles(t *testing.T) {
	texts := column()
	var m SelectionModel
	if m.MoveHandle(texts, HandleEnd, at(0, 0)) {
		t.Fatal("a handle moves without a selection")
	}
	m.SelectAll(texts)
	if !m.MoveHandle(texts, HandleEnd, at(16, 5)) {
		t.Fatal("the end handle changes nothing")
	}
	wantBounds(t, &m, pos(0, 0), pos(0, 2))
	m.MoveHandle(texts, HandleStart, at(8, 45))
	wantBounds(t, &m, pos(0, 2), pos(1, 1))
	wantText(t, &m, texts, "llo world\nS")
	// The start handle passed the end: it is the end now.
	m.MoveHandle(texts, HandleEnd, at(24, 5))
	wantBounds(t, &m, pos(0, 2), pos(0, 3))
}

func TestSelectionSelectAllAndClear(t *testing.T) {
	texts := column()
	var m SelectionModel
	if !m.SelectAll(texts) || m.SelectAll(texts) {
		t.Fatal("select all changes once")
	}
	want := []SelectionRange{{0, 0, 11}, {1, 0, 21}, {2, 0, 4}}
	if got := m.Ranges(texts); !reflect.DeepEqual(got, want) {
		t.Fatalf("ranges = %v", got)
	}
	if !m.Clear() || m.Clear() {
		t.Fatal("clear changes once")
	}
	none := newFake()
	if m.SelectAll(none) || m.Press(none, at(0, 0), 1, false) {
		t.Fatal("no texts select")
	}
}

func TestSelectionCopiesRowsWithTabs(t *testing.T) {
	texts := newFake(
		"ab￼c", frame{0, 0, 80, 20},
		"de", frame{100, 5, 80, 20},
		"fg", frame{0, 40, 80, 20},
		"", frame{0, 60, 80, 20},
		"h", frame{0, 80, 80, 20},
	)
	var m SelectionModel
	m.SelectAll(texts)
	wantText(t, &m, texts, "abc\tde\nfg\nh")
	if n := len(m.Ranges(texts)); n != 4 {
		t.Fatalf("%d ranges, want the empty text left out", n)
	}
}

func TestSelectionNeverSplitsSurrogatePairs(t *testing.T) {
	texts := newFake("a😀b c", frame{0, 0, 200, 20})
	var m SelectionModel
	m.Press(texts, at(9, 5), 2, false)
	wantBounds(t, &m, pos(0, 1), pos(0, 3))
	wantText(t, &m, texts, "😀")
	m.Press(texts, at(0, 5), 1, false)
	m.Drag(texts, at(16, 5))
	wantText(t, &m, texts, "a😀")

	for _, c := range []struct{ in, want int }{{2, 1}, {3, 3}, {9, 4}} {
		if got := snap("a😀b", c.in); got != c.want {
			t.Errorf("snap(%d) = %d, want %d", c.in, got, c.want)
		}
	}
	if got := byteOffset("a😀b", 3); got != 5 {
		t.Errorf("byteOffset = %d", got)
	}
	if s, e := wordRange("a😀b", 2); s != 1 || e != 3 {
		t.Errorf("wordRange = %d..%d", s, e)
	}
}

func TestSelectionWords(t *testing.T) {
	text := "中文abc 测试, café_1  x\n\nend"
	for _, c := range []struct {
		offset int
		want   string
	}{
		{0, "中"}, {1, "文"}, {2, "abc"}, {4, "abc"}, {5, " "}, {6, "测"}, {8, ","},
		{10, "café_1"}, {16, "  "},
		// At the end of a line: the word before; on an empty line: nothing.
		{19, "x"}, {20, ""}, {24, "end"},
	} {
		s, e := wordRange(text, c.offset)
		if got := text[byteOffset(text, s):byteOffset(text, e)]; got != c.want {
			t.Errorf("word at %d = %q, want %q", c.offset, got, c.want)
		}
	}
	if s, e := wordRange("", 0); s != 0 || e != 0 {
		t.Errorf("empty text: %d..%d", s, e)
	}
}

func TestSelectionTextsChanged(t *testing.T) {
	texts := column()
	var m SelectionModel
	m.Press(texts, at(16, 5), 1, false)
	m.Drag(texts, at(24, 105))
	shorter := newFake(
		"Hello world", frame{0, 0, 200, 20},
		"Second", frame{0, 40, 200, 20},
		"L", frame{0, 100, 200, 20},
	)
	if !m.TextsChanged(shorter) || m.TextsChanged(shorter) {
		t.Fatal("texts changed once")
	}
	wantBounds(t, &m, pos(0, 2), pos(2, 1))
	fewer := newFake("Hello world", frame{0, 0, 200, 20})
	if !m.TextsChanged(fewer) || m.TextsChanged(fewer) {
		t.Fatal("texts gone changed once")
	}
	if _, _, ok := m.Bounds(); ok {
		t.Fatal("a selection of texts gone stays")
	}
}

func TestSelectionFollowsTextsByKey(t *testing.T) {
	texts := column()
	texts.keys = []uint64{10, 20, 30}
	var m SelectionModel
	m.Press(texts, at(0, 45), 1, false)
	m.Drag(texts, at(16, 105))
	wantBounds(t, &m, pos(1, 0), pos(2, 2))
	more := newFake(
		"New", frame{0, 0, 200, 20},
		"Hello world", frame{0, 30, 200, 20},
		"Second line\nmore text", frame{0, 70, 200, 40},
		"Last", frame{0, 130, 200, 20},
	)
	more.keys = []uint64{5, 10, 20, 30}
	if !m.TextsChanged(more) {
		t.Fatal("a text before changes nothing")
	}
	wantBounds(t, &m, pos(2, 0), pos(3, 2))
	wantText(t, &m, more, "Second line\nmore text\nLa")
	gone := newFake("New", frame{0, 0, 200, 20}, "Second line\nmore text", frame{0, 70, 200, 40})
	gone.keys = []uint64{5, 20}
	if !m.TextsChanged(gone) {
		t.Fatal("an end's text gone changes nothing")
	}
	if _, _, ok := m.Bounds(); ok {
		t.Fatal("the selection stays")
	}
}

func TestTextPressesOnlyOnLinks(t *testing.T) {
	links := []TextSpan{{Start: 0, End: 4}, {Start: 4, End: 9, Link: true}}
	for _, c := range []struct {
		spans []TextSpan
		at    int
		want  bool
	}{
		{nil, -1, true},
		{links[:1], 2, true},
		{links, 5, true},
		{links, 2, false},
		{links, 9, false},
		{links, -1, false},
	} {
		if got := PressesAt(c.spans, c.at); got != c.want {
			t.Errorf("PressesAt(%v, %d) = %v, want %v", c.spans, c.at, got, c.want)
		}
	}
}

// spannedTexts are fakeTexts that tell their spans.
type spannedTexts struct {
	*fakeTexts
	spans [][]TextSpan
}

func (s spannedTexts) Spans(i int) []TextSpan { return s.spans[i] }

// A copy leaves out the hidden spans of every text it takes, inside the
// selection and across its ends.
func TestSelectionCopiesLeaveHiddenSpansOut(t *testing.T) {
	texts := spannedTexts{column(), [][]TextSpan{
		{{Start: 0, End: 6}, {Start: 6, End: 11, Hidden: true}}, // "Hello " shown, "world" hidden
		nil,
		{{Start: 0, End: 2, Hidden: true}, {Start: 2, End: 4}}, // "La" hidden, "st" shown
	}}
	var m SelectionModel
	m.Press(texts, at(0, 5), 1, false)
	m.Drag(texts, at(200, 110))
	wantText(t, &m, texts, "Hello \nSecond line\nmore text\nst")
	// Without spans, as before.
	wantText(t, &m, column(), "Hello world\nSecond line\nmore text\nLast")
}

func TestCopiedTextCutsHiddenSpansAtTheRangesEnds(t *testing.T) {
	spans := []TextSpan{{Start: 0, End: 3}, {Start: 3, End: 8, Hidden: true}, {Start: 8, End: 12}}
	text := "abcHIDDEdefg"
	for _, c := range []struct {
		from, to int
		want     string
	}{{0, 12, "abcdefg"}, {1, 5, "bc"}, {5, 10, "de"}, {4, 7, ""}} {
		if got := CopiedText(text, spans, c.from, c.to); got != c.want {
			t.Errorf("CopiedText(%d, %d) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
	if got := VisibleText("a"+string(AttachmentRune)+"b", []TextSpan{{Start: 0, End: 1, Hidden: true}}); got != string(AttachmentRune)+"b" {
		t.Errorf("VisibleText = %q: attachments keep their place", got)
	}
}
