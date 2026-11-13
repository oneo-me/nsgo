package driver

import (
	"strings"
	"unicode"
	"unicode/utf16"
)

// A selection runs across the texts of a selection area (a View with
// SelectionArea, or one selectable Text): from a place in one text to a
// place in another, in document order, as a browser selects across the
// elements of a page.
//
// The model is platform-independent and runs where the gestures are, on
// the platform's main thread, which knows the texts' layouts. A backend
// lists the area's texts in document order (SelectionTexts), feeds the
// model its gestures and draws what Ranges says with its own highlight;
// copying takes Text.
//
// Offsets are UTF-16 code units, as platforms count them, and never fall
// inside a surrogate pair.
//
// A point is hit-tested across the texts in reading order: a point inside
// a text's frame is in that text; otherwise, among the texts whose rows
// (vertical extent) contain the point, the horizontally nearest one;
// above every text, the first text's start; below every text, the last
// text's end; between rows, the end of the last text above or the start
// of the first text below, whichever is vertically nearer (the text above
// on a tie). The point is clamped into the chosen text's frame before
// SelectionTexts.OffsetAt.
//
// Words are runs of letters, digits, _ and combining marks, runs of
// spaces, or single other characters: punctuation, an emoji, each CJK
// ideograph or kana.

// TextPosition is a place in the texts of an area: a text, by its index
// in document order, and a UTF-16 offset in it.
type TextPosition struct {
	Text, Offset int
}

func (p TextPosition) less(o TextPosition) bool {
	return p.Text < o.Text || p.Text == o.Text && p.Offset < o.Offset
}

func minPos(a, b TextPosition) TextPosition {
	if b.less(a) {
		return b
	}
	return a
}

func maxPos(a, b TextPosition) TextPosition {
	if a.less(b) {
		return b
	}
	return a
}

// SelectionTexts is what a backend knows of the texts of one area, in
// document order (the area's subtree, depth first). Coordinates are the
// area's, scroll offsets of the views between applied.
type SelectionTexts interface {
	// Len is how many texts take part.
	Len() int
	// Content is text i's content.
	Content(i int) string
	// Frame is text i's content box.
	Frame(i int) Bounds
	// OffsetAt is the UTF-16 offset of text i nearest p, a point inside
	// its frame: where a click there puts the caret.
	OffsetAt(i int, p Point) int
	// Key is a stable identity of text i, such as its NodeID, so a
	// selection follows its texts when others come or go before them.
	Key(i int) uint64
}

// Granularity is what a press, drag or extension selects by.
type Granularity uint8

const (
	// ByCharacter is a press and drag.
	ByCharacter Granularity = iota
	// ByWord is a double click or a long press, then drags by words.
	ByWord
	// ByParagraph is a triple click: a whole text.
	ByParagraph
)

// SelectionHandle is one of the two ends of a selection, for touch
// handles.
type SelectionHandle uint8

const (
	HandleStart SelectionHandle = iota
	HandleEnd
)

// SelectionRange is what one text has selected: UTF-16 offsets from
// Start to End.
type SelectionRange struct {
	Text, Start, End int
}

// SelectionModel is the selection of one area: an anchor where it started
// and a focus where it goes, extended by words or paragraphs after a
// double or triple click. Its zero value selects nothing. Methods that
// change it report whether the selection changed, so the backend redraws
// only then.
type SelectionModel struct {
	set           bool
	anchor, focus TextPosition
	granularity   Granularity
	// The unit (word, paragraph) around the anchor that a drag by words
	// or paragraphs keeps selected.
	hasUnit            bool
	unitStart, unitEnd TextPosition
	// The keys of the texts of the anchor, the focus and the unit's ends,
	// as of the last change.
	keys [4]uint64
}

type selectionBounds struct {
	ok         bool
	start, end TextPosition
}

func (m *SelectionModel) bounds() selectionBounds {
	if !m.set {
		return selectionBounds{}
	}
	return selectionBounds{true, minPos(m.anchor, m.focus), maxPos(m.anchor, m.focus)}
}

// Bounds returns the selection's start and end in document order, and
// false without a selection.
func (m *SelectionModel) Bounds() (start, end TextPosition, ok bool) {
	b := m.bounds()
	return b.start, b.end, b.ok
}

// IsEmpty reports whether nothing is selected: no selection, or an empty
// one.
func (m *SelectionModel) IsEmpty() bool {
	b := m.bounds()
	return !b.ok || b.start == b.end
}

// Press is a pointer press at p: one click puts the anchor there, with
// nothing selected until a drag, two select the word, three the whole
// text. With extend (Shift) it extends the selection from its anchor to p
// instead, by the current granularity.
func (m *SelectionModel) Press(texts SelectionTexts, p Point, clicks int, extend bool) bool {
	before := m.bounds()
	pos, ok := hitText(texts, p)
	if !ok {
		return m.Clear()
	}
	if extend && m.set {
		m.extendTo(texts, pos)
		m.remember(texts)
		return m.bounds() != before
	}
	g := ByCharacter
	switch {
	case clicks == 2:
		g = ByWord
	case clicks >= 3:
		g = ByParagraph
	}
	m.selectUnit(texts, pos, g)
	m.remember(texts)
	return m.bounds() != before
}

// Drag moves the focus to p while the pointer is pressed, by the press's
// granularity.
func (m *SelectionModel) Drag(texts SelectionTexts, p Point) bool {
	if !m.set {
		return false
	}
	pos, ok := hitText(texts, p)
	if !ok {
		return false
	}
	before := m.bounds()
	m.extendTo(texts, pos)
	m.remember(texts)
	return m.bounds() != before
}

// SelectWordAt selects the word at p, as a long press on a touch screen
// does.
func (m *SelectionModel) SelectWordAt(texts SelectionTexts, p Point) bool {
	before := m.bounds()
	pos, ok := hitText(texts, p)
	if !ok {
		return m.Clear()
	}
	m.selectUnit(texts, pos, ByWord)
	m.remember(texts)
	return m.bounds() != before
}

// MoveHandle moves the end h to p and keeps the other; they swap when one
// passes the other. From then on the selection goes by characters.
func (m *SelectionModel) MoveHandle(texts SelectionTexts, h SelectionHandle, p Point) bool {
	before := m.bounds()
	if !before.ok {
		return false
	}
	pos, ok := hitText(texts, p)
	if !ok {
		return false
	}
	m.anchor = before.start
	if h == HandleStart {
		m.anchor = before.end
	}
	m.focus = pos
	m.granularity, m.hasUnit = ByCharacter, false
	m.remember(texts)
	return m.bounds() != before
}

// SelectAll selects every text of the area.
func (m *SelectionModel) SelectAll(texts SelectionTexts) bool {
	if texts.Len() == 0 {
		return m.Clear()
	}
	before := m.bounds()
	last := texts.Len() - 1
	m.set = true
	m.anchor = TextPosition{}
	m.focus = TextPosition{last, utf16Len(texts.Content(last))}
	m.granularity, m.hasUnit = ByCharacter, false
	m.remember(texts)
	return m.bounds() != before
}

// Clear clears the selection.
func (m *SelectionModel) Clear() bool {
	had := m.set
	*m = SelectionModel{}
	return had
}

// TextsChanged updates the selection to texts, the new list: each end
// follows its text by key, offsets are clamped to what is there, and a
// selection whose text is gone is cleared.
func (m *SelectionModel) TextsChanged(texts SelectionTexts) bool {
	if !m.set {
		return false
	}
	find := func(key uint64, at int) (int, bool) {
		if at < texts.Len() && texts.Key(at) == key {
			return at, true
		}
		for i := range texts.Len() {
			if texts.Key(i) == key {
				return i, true
			}
		}
		return 0, false
	}
	a, okA := find(m.keys[0], m.anchor.Text)
	f, okF := find(m.keys[1], m.focus.Text)
	if !okA || !okF {
		return m.Clear()
	}
	moved := a != m.anchor.Text || f != m.focus.Text
	m.anchor.Text, m.focus.Text = a, f
	if m.hasUnit {
		i, okI := find(m.keys[2], m.unitStart.Text)
		j, okJ := find(m.keys[3], m.unitEnd.Text)
		m.hasUnit = okI && okJ
		m.unitStart.Text, m.unitEnd.Text = i, j
	}
	before := m.bounds()
	m.anchor = clampPosition(texts, m.anchor)
	m.focus = clampPosition(texts, m.focus)
	if m.hasUnit {
		m.unitStart, m.unitEnd = clampPosition(texts, m.unitStart), clampPosition(texts, m.unitEnd)
	} else {
		m.granularity = ByCharacter
	}
	m.remember(texts)
	return moved || m.bounds() != before
}

// remember records the keys of the texts the selection is in.
func (m *SelectionModel) remember(texts SelectionTexts) {
	key := func(p TextPosition) uint64 {
		if p.Text < texts.Len() {
			return texts.Key(p.Text)
		}
		return 0
	}
	m.keys = [4]uint64{key(m.anchor), key(m.focus), 0, 0}
	if m.hasUnit {
		m.keys[2], m.keys[3] = key(m.unitStart), key(m.unitEnd)
	}
}

// Extent is the box of the texts the selection touches, for
// EventSelectionChange; no Size without a selection.
func (m *SelectionModel) Extent(texts SelectionTexts) Bounds {
	var box Bounds
	for _, r := range m.Ranges(texts) {
		if r.End <= r.Start {
			continue
		}
		f := texts.Frame(r.Text)
		if box.Size.Height == 0 {
			box = f
			continue
		}
		x0, y0 := min(box.Origin.X, f.Origin.X), min(box.Origin.Y, f.Origin.Y)
		x1 := max(box.Origin.X+box.Size.Width, f.Origin.X+f.Size.Width)
		y1 := max(box.Origin.Y+box.Size.Height, f.Origin.Y+f.Size.Height)
		box = Bounds{Origin: Point{X: x0, Y: y0}, Size: Size{Width: x1 - x0, Height: y1 - y0}}
	}
	return box
}

// Ranges returns what each text has selected, in order, leaving out
// empty ranges: what the backend highlights.
func (m *SelectionModel) Ranges(texts SelectionTexts) []SelectionRange {
	b := m.bounds()
	if !b.ok || texts.Len() == 0 || b.start == b.end {
		return nil
	}
	var out []SelectionRange
	for i := b.start.Text; i <= min(b.end.Text, texts.Len()-1); i++ {
		n := utf16Len(texts.Content(i))
		from, to := 0, n
		if i == b.start.Text {
			from = min(b.start.Offset, n)
		}
		if i == b.end.Text {
			to = min(b.end.Offset, n)
		}
		if from < to {
			out = append(out, SelectionRange{i, from, to})
		}
	}
	return out
}

// SelectionSpans is what SelectionTexts whose texts have spans also tell:
// text i's spans, whose hidden ones a copy leaves out.
type SelectionSpans interface {
	Spans(i int) []TextSpan
}

// Text returns the selected text as a copy gives it: each text's selected
// part, attachments (U+FFFC) and hidden spans left out, texts on one row
// (their frames overlap vertically) joined by a tab and the others by a
// line break.
func (m *SelectionModel) Text(texts SelectionTexts) string {
	spans, _ := texts.(SelectionSpans)
	var out strings.Builder
	var previous Bounds
	for k, r := range m.Ranges(texts) {
		frame := texts.Frame(r.Text)
		if k > 0 {
			sameRow := previous.Origin.Y < frame.Origin.Y+frame.Size.Height &&
				frame.Origin.Y < previous.Origin.Y+previous.Size.Height
			if sameRow {
				out.WriteByte('\t')
			} else {
				out.WriteByte('\n')
			}
		}
		previous = frame
		content := texts.Content(r.Text)
		from, to := byteOffset(content, r.Start), byteOffset(content, r.End)
		var hidden []TextSpan
		if spans != nil {
			hidden = spans.Spans(r.Text)
		}
		out.WriteString(CopiedText(content, hidden, from, to))
	}
	return out.String()
}

// selectUnit puts the anchor at pos and selects the unit of g around it
// (nothing for characters).
func (m *SelectionModel) selectUnit(texts SelectionTexts, pos TextPosition, g Granularity) {
	m.set, m.granularity = true, g
	if g == ByCharacter {
		m.anchor, m.focus, m.hasUnit = pos, pos, false
		return
	}
	m.unitStart, m.unitEnd = unitAt(texts, pos, g)
	m.anchor, m.focus, m.hasUnit = m.unitStart, m.unitEnd, true
}

// extendTo moves the focus to pos by the current granularity: by words or
// paragraphs, the anchor's unit stays selected and the focus goes to the
// edge of the unit at pos farther from it.
func (m *SelectionModel) extendTo(texts SelectionTexts, pos TextPosition) {
	if !m.hasUnit {
		m.focus = pos
		return
	}
	start, end := unitAt(texts, pos, m.granularity)
	if pos.less(m.unitStart) {
		m.anchor, m.focus = m.unitEnd, minPos(start, m.unitStart)
	} else {
		m.anchor, m.focus = m.unitStart, maxPos(end, m.unitEnd)
	}
}

// hitText returns the position under p by the hit-testing rule above, and
// false without texts.
func hitText(texts SelectionTexts, p Point) (TextPosition, bool) {
	count := texts.Len()
	if count == 0 {
		return TextPosition{}, false
	}
	at := func(i int) TextPosition {
		f := texts.Frame(i)
		c := Point{
			X: min(max(p.X, f.Origin.X), max(f.Origin.X+f.Size.Width, f.Origin.X)),
			Y: min(max(p.Y, f.Origin.Y), max(f.Origin.Y+f.Size.Height, f.Origin.Y)),
		}
		return clampPosition(texts, TextPosition{i, texts.OffsetAt(i, c)})
	}
	for i := range count {
		if texts.Frame(i).Contains(p) {
			return at(i), true
		}
	}
	// The texts on the point's row: the horizontally nearest.
	nearest, best := -1, Points(0)
	for i := range count {
		f := texts.Frame(i)
		if p.Y < f.Origin.Y || p.Y >= f.Origin.Y+f.Size.Height {
			continue
		}
		d := f.Origin.X - p.X
		if p.X >= f.Origin.X {
			d = max(p.X-(f.Origin.X+f.Size.Width), 0)
		}
		if nearest < 0 || d < best {
			nearest, best = i, d
		}
	}
	if nearest >= 0 {
		return at(nearest), true
	}
	// Between rows: the end of the last text above or the start of the
	// first text below, whichever is nearer.
	above, below := -1, -1
	for i := count - 1; i >= 0; i-- {
		if f := texts.Frame(i); f.Origin.Y+f.Size.Height <= p.Y {
			above = i
			break
		}
	}
	for i := range count {
		if texts.Frame(i).Origin.Y > p.Y {
			below = i
			break
		}
	}
	endOf := func(i int) TextPosition { return TextPosition{i, utf16Len(texts.Content(i))} }
	switch {
	case above >= 0 && below >= 0:
		fa, fb := texts.Frame(above), texts.Frame(below)
		if p.Y-(fa.Origin.Y+fa.Size.Height) <= fb.Origin.Y-p.Y {
			return endOf(above), true
		}
		return TextPosition{below, 0}, true
	case above >= 0:
		return endOf(above), true
	case below >= 0:
		return TextPosition{below, 0}, true
	}
	// Only frames of no height, which no row contains.
	return endOf(count - 1), true
}

// unitAt returns the unit of g around pos: its word or its whole text.
func unitAt(texts SelectionTexts, pos TextPosition, g Granularity) (TextPosition, TextPosition) {
	content := texts.Content(pos.Text)
	start, end := pos.Offset, pos.Offset
	switch g {
	case ByWord:
		start, end = wordRange(content, pos.Offset)
	case ByParagraph:
		start, end = 0, utf16Len(content)
	}
	return TextPosition{pos.Text, start}, TextPosition{pos.Text, end}
}

// charKind is what a character is for word boundaries.
type charKind uint8

const (
	// kindWord is letters, digits, _ and combining marks: runs of them
	// are words.
	kindWord charKind = iota
	// kindSpace is spaces and tabs: a run is one unit.
	kindSpace
	// kindSingle is a line break, punctuation, an emoji, a CJK ideograph
	// or kana: each is a unit of its own.
	kindSingle
)

func kindOf(r rune) charKind {
	switch {
	case r == '\n' || r == '\r' || r == ' ' || r == ' ':
		return kindSingle
	case unicode.IsSpace(r):
		return kindSpace
	case spaceless(r):
		return kindSingle
	case unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || combining(r):
		return kindWord
	}
	return kindSingle
}

// spaceless reports characters of scripts written without spaces between
// words (CJK ideographs, kana): each one is a word.
func spaceless(r rune) bool {
	switch {
	case r >= 0x3040 && r <= 0x30FF, // Hiragana, Katakana
		r >= 0x31F0 && r <= 0x31FF,   // Katakana phonetic extensions
		r >= 0x3400 && r <= 0x4DBF,   // CJK extension A
		r >= 0x4E00 && r <= 0x9FFF,   // CJK unified ideographs
		r >= 0xF900 && r <= 0xFAFF,   // CJK compatibility ideographs
		r >= 0xFF66 && r <= 0xFF9F,   // Half-width Katakana
		r >= 0x20000 && r <= 0x3FFFF: // CJK extensions B and on
		return true
	}
	return false
}

// combining reports marks that belong to the word before them.
func combining(r rune) bool {
	switch {
	case r >= 0x0300 && r <= 0x036F,
		r >= 0x1AB0 && r <= 0x1AFF,
		r >= 0x1DC0 && r <= 0x1DFF,
		r >= 0x20D0 && r <= 0x20FF,
		r >= 0xFE20 && r <= 0xFE2F,
		r == 0x200D, // zero-width joiner
		r == 0xFE0F: // emoji presentation
		return true
	}
	return false
}

// wordRange returns the UTF-16 range of the word at offset of text: the
// word of the character after the offset, or of the one before at a
// line's or the text's end. It is empty on an empty line or text.
func wordRange(text string, offset int) (int, int) {
	type char struct {
		start int
		r     rune
	}
	var chars []char
	at := 0
	for _, r := range text {
		chars = append(chars, char{at, r})
		at += utf16.RuneLen(r)
	}
	offset = snap(text, min(offset, at))
	after := len(chars)
	for i, c := range chars {
		if c.start >= offset {
			after = i
			break
		}
	}
	isBreak := func(i int) bool {
		return i >= len(chars) || chars[i].r == '\n' || chars[i].r == '\r'
	}
	index := after
	if isBreak(after) {
		// At a line's or the text's end: the character before, unless the
		// line is empty.
		if after == 0 || isBreak(after-1) {
			return offset, offset
		}
		index = after - 1
	}
	kind := kindOf(chars[index].r)
	first, last := index, index
	if kind != kindSingle {
		for first > 0 && kindOf(chars[first-1].r) == kind {
			first--
		}
		for last+1 < len(chars) && kindOf(chars[last+1].r) == kind {
			last++
		}
	}
	return chars[first].start, chars[last].start + utf16.RuneLen(chars[last].r)
}

// utf16Len returns text's length in UTF-16 code units.
func utf16Len(text string) int {
	n := 0
	for _, r := range text {
		n += utf16.RuneLen(r)
	}
	return n
}

// byteOffset returns the UTF-8 index of UTF-16 offset of text, rounded
// down to a character and clamped to the end.
func byteOffset(text string, offset int) int {
	at := 0
	for i, r := range text {
		next := at + utf16.RuneLen(r)
		if next > offset {
			return i
		}
		at = next
	}
	return len(text)
}

// snap clamps offset to text and moves it out of a surrogate pair, to the
// pair's start.
func snap(text string, offset int) int {
	at := 0
	for _, r := range text {
		next := at + utf16.RuneLen(r)
		if next > offset {
			return at
		}
		at = next
	}
	return at
}

func clampPosition(texts SelectionTexts, pos TextPosition) TextPosition {
	return TextPosition{pos.Text, snap(texts.Content(pos.Text), pos.Offset)}
}
