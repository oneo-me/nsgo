package core

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Text is laid out once: the engine prepares it with the platform's
// TextEngine, measures it at the widths layout asks for and lays it out
// at its final size. The main thread only draws the result.

// AttachmentLayout is a TextLayout of text with attachments. The engine
// asks it where each went after laying the text out.
type AttachmentLayout interface {
	// Attachment returns the box of attachment i in the content box, or
	// false when the line limit cut it off.
	Attachment(i int) (Bounds, bool)
}

// LineLayout is a TextLayout that reports its lines. The core asks it
// only when the text has an OnTextLayout handler, on the engine side,
// before the layout goes to the main thread; the core fills Text in.
type LineLayout interface {
	TextLines() []TextLine
}

// ApproxTextEngine measures text without a platform: every character is
// 0.6 of the font size wide, lines are 1.2 of it tall, and lines wrap at
// spaces. Its font's ascent is the line, its descent 0 and its x height
// half the font size. Headless tests use it.
type ApproxTextEngine struct{}

// Prepare implements TextEngine.
func (ApproxTextEngine) Prepare(t *driver.AttributedText) driver.PreparedText {
	return approxText{*t}
}

type approxText struct{ t driver.AttributedText }

// ApproxLayout is the TextLayout of ApproxTextEngine: the text's lines.
type ApproxLayout struct {
	Lines []string
	Size  Size
	// lines are the lines' boxes and offsets, for TextLines.
	lines []TextLine
	// attachments are the boxes of the text's attachments, cut off
	// unless shown.
	attachments []approxBox
	// carets hit-tests the lines, read when first asked.
	carets driver.LazyCarets
}

type approxBox struct {
	box   Bounds
	shown bool
}

// Release implements TextLayout.
func (*ApproxLayout) Release() {}

// TextLines implements LineLayout.
func (l *ApproxLayout) TextLines() []TextLine { return l.lines }

// HitTest returns the caret nearest p in the layout's content box.
func (l *ApproxLayout) HitTest(p Point) Caret { return l.carets.HitTest(p) }

// CaretRect returns the caret at c, as high as its line.
func (l *ApproxLayout) CaretRect(c Caret) (Bounds, bool) { return l.carets.CaretRect(c) }

// RangeRects appends to out the boxes covering UTF-16 offsets start to
// end.
func (l *ApproxLayout) RangeRects(start, end int, out []Bounds) []Bounds {
	return l.carets.RangeRects(start, end, out)
}

// Attachment implements AttachmentLayout.
func (l *ApproxLayout) Attachment(i int) (Bounds, bool) {
	if i < 0 || i >= len(l.attachments) {
		return Bounds{}, false
	}
	return l.attachments[i].box, l.attachments[i].shown
}

func (a approxText) charWidth() Points { return a.t.Style.Font.Size*0.6 + a.t.Style.LetterSpacing }
func (a approxText) lineHeight() Points {
	if h := a.t.Style.LineHeight; h > 0 {
		return h
	}
	return a.t.Style.Font.Size * 1.2
}

// approxLine is a line of approxText: its words joined by single spaces,
// and where its attachments sit.
type approxLine struct {
	text           string
	start, end     int // byte offsets of its first and last words
	width          Points
	ascent, height Points
	atts           []approxAtt
}

type approxAtt struct {
	index int    // in the text's attachments
	x     Points // from the line's start
}

// attachmentAt returns the index of the attachment at byte offset i.
func (a approxText) attachmentAt(i int) int {
	for k, at := range a.t.Attachments {
		if at.Offset == i {
			return k
		}
	}
	return -1
}

// wordWidth is the width of word, which starts at byte offset at.
func (a approxText) wordWidth(word string, at int) Points {
	var w Points
	for i, r := range word {
		if k := a.attachmentAt(at + i); r == driver.AttachmentRune && k >= 0 {
			w += a.t.Attachments[k].Size.Width
		} else {
			w += a.charWidth()
		}
	}
	return w
}

func (a approxText) lines(maxWidth Points) []approxLine {
	var out []approxLine
	cw := a.charWidth()
	lh := a.lineHeight()
	font := driver.FontExtent{Ascent: lh, XHeight: a.t.Style.Font.Size / 2}
	var line approxLine
	started := false
	start := 0 // the paragraph's byte offset
	flush := func() {
		line.ascent, line.height = lh, lh
		if !started {
			line.start, line.end = start, start
		}
		var below Points
		for _, at := range line.atts {
			above, b := a.t.Attachments[at.index].Extent(font)
			line.ascent = max(line.ascent, above)
			below = max(below, b)
		}
		line.height = line.ascent + below
		out = append(out, line)
		line, started = approxLine{}, false
	}
	place := func(word string, at int) {
		if started {
			line.text += " "
			line.width += cw
		} else {
			line.start = at
		}
		line.end = at + len(word)
		for i, r := range word {
			if k := a.attachmentAt(at + i); r == driver.AttachmentRune && k >= 0 {
				line.atts = append(line.atts, approxAtt{index: k, x: line.width})
				line.width += a.t.Attachments[k].Size.Width
			} else {
				line.width += cw
			}
		}
		line.text += word
		started = true
	}
	for _, para := range strings.Split(a.t.Text, "\n") {
		for i := 0; i < len(para); {
			if para[i] == ' ' {
				i++
				continue
			}
			j := i
			for j < len(para) && para[j] != ' ' {
				j++
			}
			word, at := para[i:j], start+i
			i = j
			if maxWidth.IsAuto() || cw <= 0 {
				place(word, at)
				continue
			}
			ww := a.wordWidth(word, at)
			if started && line.width+cw+ww > maxWidth+0.01 {
				flush()
			}
			// A word wider than the line breaks between its characters.
			for ww > maxWidth+0.01 && utf8.RuneCountInString(word) > 1 {
				fit, w := 0, Points(0)
				for k, r := range word {
					rw := cw
					if n := a.attachmentAt(at + k); r == driver.AttachmentRune && n >= 0 {
						rw = a.t.Attachments[n].Size.Width
					}
					if k > 0 && w+rw > maxWidth+0.01 {
						break
					}
					w += rw
					fit = k + utf8.RuneLen(r)
				}
				if fit >= len(word) {
					break
				}
				place(word[:fit], at)
				flush()
				word, at = word[fit:], at+fit
				ww = a.wordWidth(word, at)
			}
			place(word, at)
		}
		flush()
		start += len(para) + 1
	}
	if n := a.t.MaxLines; n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func (a approxText) size(lines []approxLine) Size {
	var s Size
	for _, l := range lines {
		s.Width = max(s.Width, l.width)
		s.Height += l.height
	}
	return s
}

func (a approxText) Measure(maxWidth Points) Size { return a.size(a.lines(maxWidth)) }

func (a approxText) Layout(size Size) driver.TextLayout {
	lines := a.lines(size.Width + 0.01)
	l := &ApproxLayout{Size: size, attachments: make([]approxBox, len(a.t.Attachments))}
	var y Points
	fs := a.t.Style.Font.Size
	for _, line := range lines {
		l.Lines = append(l.Lines, line.text)
		l.lines = append(l.lines, TextLine{Start: line.start, End: line.end,
			Bounds:   Bounds{Origin: Point{X: 0, Y: y}, Size: Size{Width: line.width, Height: line.height}},
			Ascender: line.ascent, Descender: line.height - line.ascent, CapHeight: fs * 0.7, XHeight: fs / 2})
		for _, at := range line.atts {
			att := a.t.Attachments[at.index]
			font := driver.FontExtent{Ascent: a.lineHeight(), XHeight: a.t.Style.Font.Size / 2}
			top := y + line.ascent - att.Ascent(font)
			l.attachments[at.index] = approxBox{box: Bounds{Origin: Point{X: at.x, Y: top}, Size: att.Size}, shown: true}
		}
		y += line.height
	}
	l.carets.Set(func() driver.CaretLayout { return a.carets(lines, l.lines) })
	return l
}

// carets reads the caret lines of lines, laid out at boxes: each
// character is a cluster as wide as it measures, the spaces between two
// words one character together.
func (a approxText) carets(lines []approxLine, boxes []TextLine) driver.CaretLayout {
	text := a.t.Text
	units := func(byteOffset int) int { return driver.UTF16Len(text[:min(byteOffset, len(text))]) }
	out := make(driver.CaretLayout, len(lines))
	for i, line := range lines {
		end := len(text)
		if i+1 < len(lines) {
			end = lines[i+1].start
		}
		cl := driver.CaretLine{Start: units(line.start), End: units(end), Box: boxes[i].Bounds}
		if i == 0 {
			cl.Start = 0
		}
		x, unit, space := Points(0), cl.Start, false
		if i == 0 {
			unit = units(line.start)
		}
		for k, r := range text[line.start:line.end] {
			w := a.charWidth()
			if n := a.attachmentAt(line.start + k); r == driver.AttachmentRune && n >= 0 {
				w = a.t.Attachments[n].Size.Width
			} else if r == ' ' && space {
				w = 0 // spaces collapse into one
			}
			space = r == ' '
			n := len(utf16.Encode([]rune{r}))
			cl.Clusters = append(cl.Clusters, driver.CaretCluster{Start: unit, End: unit + n, Left: x, Right: x + w})
			x += w
			unit += n
		}
		out[i] = cl
	}
	return out
}

func (approxText) Release() {}

// TextCaretAt returns the caret nearest p, a point in the text's frame, after
// the last commit: its UTF-16 offset, and upstream when it stays at the end
// of a wrapped line. False while the text is not laid out.
func (r *Ref) TextCaretAt(p Point) (offset int, upstream, ok bool) {
	tn := r.laidOut()
	if tn == nil {
		return 0, false, false
	}
	c := driver.HitTest(tn.layout, Point{X: p.X - tn.shownBox.Origin.X, Y: p.Y - tn.shownBox.Origin.Y})
	return c.Offset, c.Affinity == driver.Upstream, true
}

// TextCaretBox returns the caret's box at UTF-16 offset, a box of no width
// as tall as its line, in the text's frame; upstream takes the earlier line
// where one offset is at both.
func (r *Ref) TextCaretBox(offset int, upstream bool) (Bounds, bool) {
	tn := r.laidOut()
	if tn == nil {
		return Bounds{}, false
	}
	affinity := driver.Downstream
	if upstream {
		affinity = driver.Upstream
	}
	b, ok := driver.CaretRect(tn.layout, driver.Caret{Offset: offset, Affinity: affinity})
	b.Origin.X += tn.shownBox.Origin.X
	b.Origin.Y += tn.shownBox.Origin.Y
	return b, ok
}

// TextRangeBoxes returns the line-high boxes of the UTF-16 range from start
// to end, in the text's frame.
func (r *Ref) TextRangeBoxes(start, end int) []Bounds {
	tn := r.laidOut()
	if tn == nil {
		return nil
	}
	boxes := driver.RangeRects(tn.layout, start, end, nil)
	for i := range boxes {
		boxes[i].Origin.X += tn.shownBox.Origin.X
		boxes[i].Origin.Y += tn.shownBox.Origin.Y
	}
	return boxes
}

func (r *Ref) laidOut() *textNode {
	if r == nil || r.n == nil || r.n.deleted || r.n.text == nil || r.n.text.layout == nil {
		return nil
	}
	return r.n.text
}
