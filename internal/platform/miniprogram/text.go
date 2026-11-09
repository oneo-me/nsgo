package miniprogram

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/webstyle"
)

// Text is laid out on the engine's side: the logic layer measures runs
// on an offscreen canvas (Measurer), lines break here, greedily after
// spaces and around CJK characters, and the render layer draws each run
// where it was placed, never wrapping it (white-space: pre). Runs of a
// line keep their spans' styles on one baseline, and attachments take
// their placeholders' places.

// Measurer measures text in the host's fonts.
type Measurer interface {
	// Width is the advance of text in font, a CSS font shorthand.
	Width(font, text string) float64
}

// defaultFamily is what texts without a family of their own are measured
// and drawn in. It is named rather than sans-serif, as the logic layer's
// canvas and the render layer resolve sans-serif to different fonts
// (Helvetica and PingFang SC in WeChat DevTools on a Mac), so lines
// measured in one would not fit in the other.
const defaultFamily = `"PingFang SC", "Helvetica Neue", Helvetica, Arial, sans-serif`

// monospaceFamily is the CSS of driver.FontMonospace.
const monospaceFamily = `Menlo, "SF Mono", Consolas, "Roboto Mono", monospace`

// family returns the CSS font-family of a family name.
func family(name string) string {
	switch name {
	case "":
		return defaultFamily
	case driver.FontMonospace:
		return monospaceFamily
	}
	var list []string
	for _, f := range driver.FontFamilies(name) {
		switch f.Kind {
		case driver.FamilyMonospace:
			list = append(list, monospaceFamily)
		case driver.FamilySerif:
			list = append(list, "serif")
		case driver.FamilySystem, driver.FamilyRounded:
			list = append(list, defaultFamily)
		default:
			list = append(list, strconv.Quote(f.Name))
		}
	}
	return strings.Join(append(list, defaultFamily), ", ")
}

// fontCSS is a font as declarations: the render layer drops a font
// shorthand in inline styles.
func fontCSS(f driver.Font) string {
	italic := "normal"
	if f.Italic {
		italic = "italic"
	}
	return "font-style:" + italic + ";font-weight:" + strconv.Itoa(int(f.Weight)) +
		";font-size:" + webstyle.Px(f.Size) + ";font-family:" + family(f.Family) + ";"
}

// canvasFont is a font as the canvas's font shorthand.
func canvasFont(f driver.Font) string {
	italic := "normal"
	if f.Italic {
		italic = "italic"
	}
	return italic + " " + strconv.Itoa(int(f.Weight)) + " " + webstyle.Px(f.Size) + " " + family(f.Family)
}

// runStyle is how the runs of one span are measured and drawn.
type runStyle struct {
	font          string // canvas font
	css           string // the run's declarations
	size          driver.Points
	lineHeight    driver.Points
	letterSpacing driver.Points
}

func newRunStyle(t driver.TextStyle) runStyle {
	css := fontCSS(t.Font) + "color:" + webstyle.Color(t.Color) + ";"
	if t.Decoration != 0 {
		css += "text-decoration:" + webstyle.Decoration(t.Decoration) + " " + webstyle.DecorationStyles[t.DecorationStyle]
		if t.DecorationColor != 0 {
			css += " " + webstyle.Color(t.DecorationColor)
		}
		css += ";"
	}
	if t.LetterSpacing != 0 {
		css += "letter-spacing:" + webstyle.Px(t.LetterSpacing) + ";"
	}
	if t.Font.Variant&driver.FontTabularNums != 0 {
		css += "font-variant-numeric:tabular-nums;"
	}
	lh := t.LineHeight
	if lh <= 0 {
		lh = t.Font.Size * 1.2
	}
	return runStyle{font: canvasFont(t.Font), css: css, size: t.Font.Size, lineHeight: lh, letterSpacing: t.LetterSpacing}
}

// above is how far the baseline is below the top of the style's line box:
// the glyphs sit around its middle, a baseline about 0.3 em below it.
func (r *runStyle) above() driver.Points { return r.lineHeight/2 + 0.3*r.size }

// TextEngine is driver.TextEngine over a Measurer.
type TextEngine struct {
	m      Measurer
	widths map[widthKey]driver.Points
}

type widthKey struct{ font, text string }

// NewTextEngine returns an engine measuring with m.
func NewTextEngine(m Measurer) *TextEngine {
	return &TextEngine{m: m, widths: map[widthKey]driver.Points{}}
}

// width is text's advance in style, its letter spacing included.
func (e *TextEngine) width(st *runStyle, text string) driver.Points {
	if text == "" {
		return 0
	}
	k := widthKey{st.font, text}
	w, ok := e.widths[k]
	if !ok {
		if len(e.widths) > 1<<14 {
			clear(e.widths)
		}
		w = driver.Points(e.m.Width(st.font, text))
		e.widths[k] = w
	}
	return w + st.letterSpacing*driver.Points(utf8.RuneCountInString(text))
}

// segment is a stretch of a piece in one style, or an attachment.
type segment struct {
	start, end int // bytes of the text
	style      int
	width      driver.Points
	attachment int // its index, or -1
}

// piece is what no line breaks inside: a word with its trailing spaces,
// or a CJK character.
type piece struct {
	segments []segment
	width    driver.Points
	// trimmed leaves the trailing spaces out: a line's end does not
	// count them.
	trimmed driver.Points
}

// prepared is a text shaped for measuring.
type prepared struct {
	e      *TextEngine
	t      driver.AttributedText
	styles []runStyle
	// paragraphs are the text split at line breaks, each its pieces.
	paragraphs [][]piece
	// starts are the paragraphs' byte offsets.
	starts []int
}

// Prepare implements driver.TextEngine.
func (e *TextEngine) Prepare(t *driver.AttributedText) driver.PreparedText {
	p := &prepared{e: e, t: *t}
	// The text's style, then the spans' that differ: spans of one look,
	// as a link in the text's, share it and their runs join.
	p.styles = append(p.styles, newRunStyle(t.Style))
	spanStyle := make([]int, len(t.Spans))
	for k, sp := range t.Spans {
		st := newRunStyle(sp.Style)
		spanStyle[k] = len(p.styles)
		for i := range p.styles {
			if p.styles[i] == st {
				spanStyle[k] = i
				break
			}
		}
		if spanStyle[k] == len(p.styles) {
			p.styles = append(p.styles, st)
		}
	}
	// Each byte's style: the span's, else the text's.
	styleAt := func(i int) int {
		for k, sp := range t.Spans {
			if i >= sp.Start && i < sp.End {
				return spanStyle[k]
			}
		}
		return 0
	}
	attachmentAt := map[int]int{}
	for k, a := range t.Attachments {
		attachmentAt[a.Offset] = k
	}
	start := 0
	for _, para := range strings.Split(t.Text, "\n") {
		p.starts = append(p.starts, start)
		var pieces []piece
		for _, b := range breaks(para) {
			var pc piece
			for i := b[0]; i < b[1]; {
				r, n := utf8.DecodeRuneInString(para[i:])
				at := start + i
				st := styleAt(at)
				if k, ok := attachmentAt[at]; ok && r == driver.AttachmentRune {
					pc.segments = append(pc.segments, segment{start: at, end: at + n, style: st,
						width: t.Attachments[k].Size.Width, attachment: k})
				} else if last := len(pc.segments) - 1; last >= 0 && pc.segments[last].style == st && pc.segments[last].attachment < 0 {
					pc.segments[last].end = at + n
				} else {
					pc.segments = append(pc.segments, segment{start: at, end: at + n, style: st, attachment: -1})
				}
				i += n
			}
			for k := range pc.segments {
				sg := &pc.segments[k]
				if sg.attachment < 0 {
					sg.width = e.width(&p.styles[sg.style], t.Text[sg.start:sg.end])
				}
				w := sg.width
				if k == len(pc.segments)-1 && sg.attachment < 0 {
					w = e.width(&p.styles[sg.style], strings.TrimRightFunc(t.Text[sg.start:sg.end], unicode.IsSpace))
				}
				pc.trimmed = pc.width + w
				pc.width += sg.width
			}
			pieces = append(pieces, pc)
		}
		p.paragraphs = append(p.paragraphs, pieces)
		start += len(para) + 1
	}
	return p
}

// cjk reports whether a line may break before and after r: CJK
// ideographs, kana, hangul and their punctuation.
func cjk(r rune) bool {
	return r >= 0x2E80 && r <= 0x9FFF || r >= 0xAC00 && r <= 0xD7AF || r >= 0xF900 && r <= 0xFAFF ||
		r >= 0xFE30 && r <= 0xFE4F || r >= 0xFF00 && r <= 0xFFEF || r >= 0x20000 && r <= 0x3FFFF
}

// breaks returns where lines may break in s, as the byte ranges of its
// pieces: after the spaces following a word, and around each CJK
// character.
func breaks(s string) [][2]int {
	var out [][2]int
	start, spaces := 0, false
	for i, r := range s {
		switch {
		case cjk(r) || r == driver.AttachmentRune:
			if i > start {
				out = append(out, [2]int{start, i})
			}
			out = append(out, [2]int{i, i + utf8.RuneLen(r)})
			start, spaces = i+utf8.RuneLen(r), false
		case unicode.IsSpace(r):
			spaces = true
		case spaces:
			out = append(out, [2]int{start, i})
			start, spaces = i, false
		}
	}
	if start < len(s) || len(out) == 0 {
		out = append(out, [2]int{start, len(s)})
	}
	return out
}

// line is a line while it is broken, then laid out.
type line struct {
	segments []segment
	width    driver.Points
	trimmed  driver.Points
	start    int // its first byte
	end      int // past its last
	cut      bool
}

// lines breaks the text at maxWidth (Auto: only at line breaks), with
// the line limit applied; cut marks a last line that lost what followed.
func (p *prepared) lines(maxWidth driver.Points) []line {
	limit := !maxWidth.IsAuto()
	var out []line
	for k, pieces := range p.paragraphs {
		ln := line{start: p.starts[k], end: p.starts[k]}
		push := func() {
			out = append(out, ln)
			ln = line{start: ln.end, end: ln.end}
		}
		for _, pc := range pieces {
			if limit && len(ln.segments) > 0 && ln.width+pc.trimmed > maxWidth+0.01 {
				push()
			}
			// A piece wider than the line breaks between its characters.
			if limit && len(ln.segments) == 0 && pc.trimmed > maxWidth+0.01 {
				for _, part := range p.split(pc, maxWidth) {
					if len(ln.segments) > 0 {
						push()
					}
					ln.add(part)
				}
				continue
			}
			ln.add(pc)
		}
		out = append(out, ln)
	}
	if n := p.t.MaxLines; n > 0 && len(out) > n {
		out = out[:n]
		out[n-1].cut = true
	}
	return out
}

func (ln *line) add(pc piece) {
	if len(pc.segments) == 0 {
		return
	}
	if len(ln.segments) == 0 {
		ln.start = pc.segments[0].start
	}
	ln.segments = append(ln.segments, pc.segments...)
	ln.trimmed = ln.width + pc.trimmed
	ln.width += pc.width
	ln.end = pc.segments[len(pc.segments)-1].end
}

// split cuts pc into pieces that fit maxWidth, at least a character each.
func (p *prepared) split(pc piece, maxWidth driver.Points) []piece {
	var out []piece
	var cur piece
	for _, sg := range pc.segments {
		if sg.attachment >= 0 {
			if len(cur.segments) > 0 && cur.width+sg.width > maxWidth {
				out, cur = append(out, cur), piece{}
			}
			cur.segments = append(cur.segments, sg)
			cur.width += sg.width
			cur.trimmed = cur.width
			continue
		}
		st := &p.styles[sg.style]
		for i := sg.start; i < sg.end; {
			_, n := utf8.DecodeRuneInString(p.t.Text[i:])
			w := p.e.width(st, p.t.Text[i:i+n])
			if len(cur.segments) > 0 && cur.width+w > maxWidth+0.01 {
				out, cur = append(out, cur), piece{}
			}
			if last := len(cur.segments) - 1; last >= 0 && cur.segments[last].style == sg.style && cur.segments[last].attachment < 0 {
				cur.segments[last].end = i + n
				cur.segments[last].width += w
			} else {
				cur.segments = append(cur.segments, segment{start: i, end: i + n, style: sg.style, width: w, attachment: -1})
			}
			cur.width += w
			cur.trimmed = cur.width
			i += n
		}
	}
	if len(cur.segments) > 0 {
		out = append(out, cur)
	}
	return out
}

// metrics returns a line's height and how far its baseline is below its
// top: the runs' line boxes share the baseline, and an attachment
// reserves its extent around it.
func (p *prepared) metrics(ln *line) (height, baseline driver.Points) {
	var above, below driver.Points
	for _, sg := range ln.segments {
		st := &p.styles[sg.style]
		if sg.attachment >= 0 {
			a, b := p.t.Attachments[sg.attachment].Extent(p.extent(st))
			above, below = max(above, a), max(below, b)
			continue
		}
		above, below = max(above, st.above()), max(below, st.lineHeight-st.above())
	}
	if above+below <= 0 {
		st := &p.styles[0]
		return st.lineHeight, st.above()
	}
	return above + below, above
}

// extent is the font around an attachment in st.
func (p *prepared) extent(st *runStyle) driver.FontExtent {
	return driver.FontExtent{Ascent: 0.8 * st.size, Descent: 0.2 * st.size, XHeight: 0.5 * st.size}
}

// Measure implements driver.PreparedText. Widths round up, as native text
// stacks do: a box exactly this wide must not wrap the text again.
func (p *prepared) Measure(maxWidth driver.Points) driver.Size {
	var s driver.Size
	for _, ln := range p.lines(maxWidth) {
		h, _ := p.metrics(&ln)
		s.Width = max(s.Width, ln.trimmed)
		s.Height += h
	}
	s.Width = driver.Points(math.Ceil(float64(s.Width)*100-0.001) / 100)
	return s
}

// Release implements driver.PreparedText.
func (*prepared) Release() {}

// run is a stretch of a line in one style, at its place in the content
// box: the top of its own line box.
type run struct {
	text   string
	x, y   driver.Points
	height driver.Points
	style  int
}

// TextLayout is the text laid out at its final size: what the render
// layer draws.
type TextLayout struct {
	styles []string // the runs' CSS
	runs   []run
	lines  []driver.TextLine
	boxes  []attachmentBox
	carets driver.LazyCarets
}

type attachmentBox struct {
	box   driver.Bounds
	shown bool
}

// Layout implements driver.PreparedText.
func (p *prepared) Layout(size driver.Size) driver.TextLayout {
	l := &TextLayout{boxes: make([]attachmentBox, len(p.t.Attachments))}
	for _, st := range p.styles {
		l.styles = append(l.styles, st.css)
	}
	lines := p.lines(size.Width + 0.01)
	if n := len(lines); n > 0 && lines[n-1].cut && p.t.MaxLines == 1 &&
		(p.t.Ellipsize == driver.EllipsizeHead || p.t.Ellipsize == driver.EllipsizeMiddle) {
		// Cut at its head or middle: the most characters that fit.
		cut := p.t.CutToFit(func(c *driver.AttributedText) bool {
			return p.e.Prepare(c).Measure(driver.Auto).Width <= size.Width
		})
		return p.e.Prepare(&cut).Layout(size)
	}
	if n := len(lines); n > 0 && lines[n-1].cut && p.t.Ellipsize != driver.EllipsizeClip {
		p.ellipsize(&lines[n-1], size.Width)
	}
	var y driver.Points
	type laid struct {
		x, height, baseline driver.Points
	}
	placed := make([]laid, len(lines))
	for i := range lines {
		ln := &lines[i]
		h, base := p.metrics(ln)
		x := p.lineStart(ln.trimmed, size.Width)
		placed[i] = laid{x, h, base}
		cx := x
		for _, sg := range ln.segments {
			st := &p.styles[sg.style]
			if sg.attachment >= 0 {
				a := p.t.Attachments[sg.attachment]
				top := y + base - a.Ascent(p.extent(st))
				l.boxes[sg.attachment] = attachmentBox{box: driver.Bounds{Origin: driver.Point{X: cx, Y: top}, Size: a.Size}, shown: true}
				cx += sg.width
				continue
			}
			text := ellipsis
			if sg.start >= 0 {
				text = p.t.Text[sg.start:sg.end]
			}
			if n := len(l.runs) - 1; n >= 0 && l.runs[n].style == sg.style && l.runs[n].y == y+base-st.above() &&
				abs(l.runs[n].x+p.e.width(st, l.runs[n].text)-cx) < 0.5 {
				l.runs[n].text += text
			} else {
				l.runs = append(l.runs, run{text: text, x: cx, y: y + base - st.above(), height: st.lineHeight, style: sg.style})
			}
			cx += sg.width
		}
		if n := len(l.runs) - 1; n >= 0 {
			l.runs[n].text = strings.TrimRightFunc(l.runs[n].text, unicode.IsSpace)
		}
		fs := p.styles[0].size
		l.lines = append(l.lines, driver.TextLine{Start: ln.start, End: ln.end,
			Bounds:   driver.Bounds{Origin: driver.Point{X: x, Y: y}, Size: driver.Size{Width: ln.trimmed, Height: h}},
			Ascender: base, Descender: h - base, CapHeight: 0.7 * fs, XHeight: 0.5 * fs})
		y += h
	}
	l.carets.Set(func() driver.CaretLayout { return p.carets(lines, l.lines) })
	return l
}

// lineStart is where a line as wide as width starts in a box as wide as
// box, as the text aligns.
func (p *prepared) lineStart(width, box driver.Points) driver.Points {
	align := p.t.Style.Align
	if p.t.Style.RTL {
		switch align {
		case driver.TextAlignStart, driver.TextAlignJustify:
			align = driver.TextAlignEnd
		case driver.TextAlignEnd:
			align = driver.TextAlignStart
		}
	}
	switch align {
	case driver.TextAlignCenter:
		return (box - width) / 2
	case driver.TextAlignEnd, driver.TextAlignRight:
		return box - width
	}
	return 0
}

// ellipsis ends a line the line limit cut.
const ellipsis = "…"

// ellipsize ends ln with an ellipsis, dropping characters from its end
// until it fits width.
func (p *prepared) ellipsize(ln *line, width driver.Points) {
	if len(ln.segments) == 0 {
		return
	}
	last := &ln.segments[len(ln.segments)-1]
	st := &p.styles[last.style]
	ew := p.e.width(st, ellipsis)
	for len(ln.segments) > 0 {
		sg := &ln.segments[len(ln.segments)-1]
		if sg.attachment >= 0 || sg.end <= sg.start {
			ln.width -= sg.width
			ln.segments = ln.segments[:len(ln.segments)-1]
			continue
		}
		text := strings.TrimRightFunc(p.t.Text[sg.start:sg.end], unicode.IsSpace)
		w := ln.width - sg.width + p.e.width(&p.styles[sg.style], text)
		if w+ew <= width+0.01 || len(text) == 0 {
			// The ellipsis follows the last character, not its spaces.
			sg.end = sg.start + len(text)
			ln.width = w
			sg.width = p.e.width(&p.styles[sg.style], text)
			break
		}
		_, n := utf8.DecodeLastRuneInString(text)
		sg.end = sg.start + len(text) - n
		ln.width -= sg.width
		sg.width = p.e.width(&p.styles[sg.style], p.t.Text[sg.start:sg.end])
		ln.width += sg.width
	}
	// The ellipsis follows as a segment of its own: a run the text does
	// not hold, which the layout writes in place.
	ln.segments = append(ln.segments, segment{start: -1, style: last.style, width: ew, attachment: -1})
	ln.width += ew
	ln.trimmed = ln.width
}

// carets reads the caret lines of lines, laid out at boxes: each
// character a cluster as wide as it measures.
func (p *prepared) carets(lines []line, boxes []driver.TextLine) driver.CaretLayout {
	text := p.t.Text
	units := func(i int) int { return driver.UTF16Len(text[:min(i, len(text))]) }
	out := make(driver.CaretLayout, len(lines))
	for i, ln := range lines {
		end := len(text)
		if i+1 < len(lines) {
			end = lines[i+1].start
		}
		cl := driver.CaretLine{Start: units(ln.start), End: units(end), Box: boxes[i].Bounds}
		x := boxes[i].Bounds.Origin.X
		for _, sg := range ln.segments {
			if sg.start < 0 {
				continue
			}
			st := &p.styles[sg.style]
			for k, r := range text[sg.start:sg.end] {
				at := sg.start + k
				w := sg.width
				if sg.attachment < 0 {
					w = p.e.width(st, string(r))
				}
				u := units(at)
				cl.Clusters = append(cl.Clusters, driver.CaretCluster{Start: u, End: u + driver.UTF16Len(string(r)), Left: x, Right: x + w})
				x += w
			}
		}
		out[i] = cl
	}
	return out
}

// Release implements driver.TextLayout.
func (*TextLayout) Release() {}

// TextLines implements core.LineLayout.
func (l *TextLayout) TextLines() []driver.TextLine { return l.lines }

// Attachment implements core.AttachmentLayout.
func (l *TextLayout) Attachment(i int) (driver.Bounds, bool) {
	if i < 0 || i >= len(l.boxes) {
		return driver.Bounds{}, false
	}
	return l.boxes[i].box, l.boxes[i].shown
}

// HitTest implements driver.TextHitTester.
func (l *TextLayout) HitTest(p driver.Point) driver.Caret { return l.carets.HitTest(p) }

// CaretRect implements driver.TextHitTester.
func (l *TextLayout) CaretRect(c driver.Caret) (driver.Bounds, bool) { return l.carets.CaretRect(c) }

// RangeRects implements driver.TextHitTester.
func (l *TextLayout) RangeRects(start, end int, out []driver.Bounds) []driver.Bounds {
	return l.carets.RangeRects(start, end, out)
}

func abs(v driver.Points) driver.Points {
	if v < 0 {
		return -v
	}
	return v
}
