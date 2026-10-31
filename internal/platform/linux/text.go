//go:build linux && !android

package linux

import (
	"math"
	"slices"
	"unicode/utf8"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Pango units per point. Pango sizes, widths and spacing use these.
const pangoScale = 1024

const (
	pangoAlignLeft   = 0
	pangoAlignCenter = 1
	pangoAlignRight  = 2

	pangoDirectionLTR = 0
	pangoDirectionRTL = 1

	pangoWrapWord     = 0
	pangoWrapChar     = 1
	pangoWrapWordChar = 2

	pangoEllipsizeNone   = 0
	pangoEllipsizeStart  = 1
	pangoEllipsizeMiddle = 2
	pangoEllipsizeEnd    = 3

	pangoStyleNormal  = 0
	pangoStyleOblique = 1
	pangoStyleItalic  = 2

	pangoUnderlineSingle = 1
	pangoUnderlineDouble = 2
	pangoUnderlineError  = 4 // wavy
)

// textSource is what a text is laid out from: plain data, so the engine
// thread and the main thread each build a PangoLayout of their own.
type textSource struct {
	text     string
	style    driver.TextStyle
	spans    []driver.TextSpan
	maxLines int
	// ellipsize cuts what maxLines leaves out; a clipped text is measured
	// as if it ended with an ellipsis and drawn whole, clipped to its box.
	ellipsize driver.EllipsizeMode
	shadow    driver.TextShadow
	// atts are the text's attachments and fonts the metrics of the font
	// around each, which places it.
	atts  []driver.TextAttachment
	fonts []driver.FontExtent
}

// textLayout is a prepared text laid out at its final width. The engine
// measures it with Pango on its own thread; the main thread builds the
// PangoLayout it draws from the same plain data, so both use one font
// configuration.
type textLayout struct {
	textSource
	width  driver.Points
	height driver.Points

	// under are the spans' background boxes, from the content box's top
	// left; attachments are where the attachments went.
	under       []rect
	underColors []driver.Color
	attachments []attachmentBox
	// over are the decoration lines Pango cannot draw, drawn over the
	// glyphs.
	over []decorationLine

	// lines are the lines OnTextLayout reports.
	lines []driver.TextLine
	// carets hit-tests the lines, read when first asked.
	carets driver.LazyCarets

	// The main thread's PangoLayout, made when first drawn and freed by
	// Release.
	layout uintptr
}

// attachmentBox is an attachment's box in the content box, unless a line
// limit cut it off.
type attachmentBox struct {
	box   driver.Bounds
	shown bool
}

// preparedText is a text shaped on the engine thread.
type preparedText struct {
	textSource
	w      *window
	layout uintptr // the engine thread's PangoLayout, reused across measures
	width  driver.Points
	height driver.Points
}

// Prepare implements core.TextEngine.
func (w *window) Prepare(t *driver.AttributedText) driver.PreparedText {
	src := textSource{text: t.Text, style: t.Style, spans: t.Spans, maxLines: t.MaxLines, ellipsize: t.Ellipsize,
		shadow: t.Shadow}
	if len(t.Attachments) > 0 {
		src.atts = append([]driver.TextAttachment(nil), t.Attachments...)
		src.fonts = make([]driver.FontExtent, len(t.Attachments))
		for i, at := range t.Attachments {
			src.fonts[i] = w.fontExtent(src.styleAt(at.Offset).Font)
		}
	}
	return &preparedText{w: w, textSource: src}
}

// styleAt is the style of the span at byte offset i: for an attachment,
// the font it is placed against.
func (s *textSource) styleAt(i int) driver.TextStyle {
	for _, sp := range s.spans {
		if sp.Start <= i && i < sp.End {
			return sp.Style
		}
	}
	return s.style
}

// measureContext returns the Pango context the engine thread measures
// with, made on first use from a font map of its own. It is never the
// default context, which belongs to the main thread.
func (w *window) measureContext(rtl bool) uintptr {
	i := 0
	if rtl {
		i = 1
	}
	if w.measureMap[i] == 0 {
		w.measureMap[i] = pangoFontMapCreateContext(pangoCairoFontMapNew())
		pangoContextSetBaseDir(w.measureMap[i], textDirection(rtl))
	}
	return w.measureMap[i]
}

// textDirection is the PangoDirection of a base direction.
func textDirection(rtl bool) int32 {
	if rtl {
		return pangoDirectionRTL
	}
	return pangoDirectionLTR
}

// fontExtent returns the vertical metrics of f, from a line of "x" in
// it: its ascent and descent, and the top of the x's ink. The engine
// thread calls it; it caches what it measured.
func (w *window) fontExtent(f driver.Font) driver.FontExtent {
	if e, ok := w.extents[f]; ok {
		return e
	}
	l := pangoLayoutNew(w.measureContext(false))
	attrs := pangoAttrListNew()
	addStyle(attrs, 1, 0, 1, driver.TextStyle{Font: f}, nil)
	pangoLayoutSetAttributes(l, attrs)
	pangoAttrListUnref(attrs)
	pangoLayoutSetText(l, cstr("x"), 1)
	var ink, logical pangoRectangle
	pangoLayoutGetExtents(l, &ink, &logical)
	base := pangoLayoutGetBaseline(l)
	gObjectUnref(l)
	e := driver.FontExtent{
		Ascent:  fromPango(base - logical.Y),
		Descent: fromPango(logical.Y + logical.Height - base),
		XHeight: fromPango(base - ink.Y),
	}
	if w.extents == nil {
		w.extents = map[driver.Font]driver.FontExtent{}
	}
	w.extents[f] = e
	return e
}

// pangoRectangle is PangoRectangle, in Pango units.
type pangoRectangle struct{ X, Y, Width, Height int32 }

func toPango(p driver.Points) int32   { return int32(math.Round(float64(p * pangoScale))) }
func fromPango(v int32) driver.Points { return driver.Points(v) / pangoScale }

// newLayout makes a PangoLayout on the current thread with the text and
// attributes of s, to measure.
func newLayout(context uintptr, s *textSource, width driver.Points) uintptr {
	l := pangoLayoutNew(context)
	applyLayout(l, s, width, false)
	return l
}

// applyLayout gives l its text, attributes and box; drawn, a clipped
// text keeps every line.
func applyLayout(l uintptr, s *textSource, width driver.Points, drawn bool) {
	text := s.text
	if text == "" {
		text = " "
	}
	attrs := textAttributes(text, s)
	pangoLayoutSetAttributes(l, attrs)
	pangoAttrListUnref(attrs)
	pangoLayoutSetText(l, cstr(text), int32(len(text)))
	pangoLayoutSetWidth(l, s.pangoWidth(width, drawn))
	// The text's own base direction, not its first letter's; start and
	// end follow it.
	pangoLayoutSetAutoDir(l, false)
	if drawn {
		pangoContextSetBaseDir(pangoLayoutGetContext(l), textDirection(s.style.RTL))
		pangoLayoutContextChanged(l)
	}
	start, end := int32(pangoAlignLeft), int32(pangoAlignRight)
	if s.style.RTL {
		start, end = end, start
	}
	switch s.style.Align {
	case driver.TextAlignCenter:
		pangoLayoutSetAlignment(l, pangoAlignCenter)
	case driver.TextAlignEnd:
		pangoLayoutSetAlignment(l, end)
	default:
		pangoLayoutSetAlignment(l, start)
	}
	pangoLayoutSetJustify(l, s.style.Align == driver.TextAlignJustify)
	if s.maxLines > 1 {
		pangoLayoutSetWrap(l, pangoWrapWord)
	}
	if s.maxLines > 0 && !(drawn && s.ellipsize == driver.EllipsizeClip) {
		mode := int32(pangoEllipsizeEnd)
		switch s.ellipsize {
		case driver.EllipsizeHead:
			mode = pangoEllipsizeStart
		case driver.EllipsizeMiddle:
			mode = pangoEllipsizeMiddle
		}
		pangoLayoutSetEllipsize(l, mode)
		pangoLayoutSetHeight(l, -int32(s.maxLines))
	}
}

// pangoWidth is the width Pango wraps and ellipsizes s at: none for a
// clipped line drawn whole, which its box cuts.
func (s *textSource) pangoWidth(width driver.Points, drawn bool) int32 {
	if width.IsAuto() || (drawn && s.maxLines == 1 && s.ellipsize == driver.EllipsizeClip) {
		return -1
	}
	return int32(width*pangoScale + 0.5)
}

// placeholderLen is the length in bytes of an attachment's placeholder.
var placeholderLen = len(string(driver.AttachmentRune))

// textAttributes returns the Pango attributes of text: the base style
// over all of it, then each span over its bytes, then the attachments'
// placeholders. A placeholder is a shape of the attachment's width that
// draws nothing: its logical rectangle is what its line reserves (the
// attachment's Extent), its ink the attachment, so its line grows where
// the attachment reaches past the font. Line heights leave placeholders
// out, as they would hold a line to them.
func textAttributes(text string, s *textSource) uintptr {
	var holes [][2]int
	for _, at := range s.atts {
		if at.Offset >= 0 && at.Offset+placeholderLen <= len(text) {
			holes = append(holes, [2]int{at.Offset, at.Offset + placeholderLen})
		}
	}
	list := pangoAttrListNew()
	addStyle(list, len(text), 0, len(text), s.style, holes)
	for _, sp := range s.spans {
		addStyle(list, len(text), sp.Start, sp.End, sp.Style, holes)
	}
	for i, at := range s.atts {
		if at.Offset < 0 || at.Offset+placeholderLen > len(text) {
			continue
		}
		top := at.Ascent(s.fonts[i])
		above, below := at.Extent(s.fonts[i])
		ink := pangoRectangle{0, -toPango(top), toPango(at.Size.Width), toPango(at.Size.Height)}
		logical := pangoRectangle{0, -toPango(above), toPango(at.Size.Width), toPango(above + below)}
		for _, attr := range []uintptr{
			pangoAttrShapeNew(&ink, &logical),
			// Lines are not drawn through an attachment.
			pangoAttrUnderlineNew(0),
			pangoAttrStrikethroughNew(0),
		} {
			setAttributeRange(attr, uint32(at.Offset), uint32(at.Offset+placeholderLen))
			pangoAttrListInsert(list, attr)
		}
	}
	return list
}

// addStyle adds s's font, color, spacing and decoration over start:end,
// clipped to textLen bytes. Its line height leaves out holes, which are
// in order.
func addStyle(list uintptr, textLen, start, end int, s driver.TextStyle, holes [][2]int) {
	start, end = min(max(start, 0), textLen), min(max(end, 0), textLen)
	if end <= start {
		return
	}
	add := func(attr uintptr) {
		if attr == 0 {
			return
		}
		setAttributeRange(attr, uint32(start), uint32(end))
		pangoAttrListInsert(list, attr)
	}
	f := s.Font
	if f.Family != "" {
		add(pangoAttrFamilyNew(cstr(f.Family)))
	}
	if f.Size > 0 {
		add(pangoAttrSizeNew(int32(f.Size*pangoScale + 0.5)))
	}
	if f.Weight > 0 {
		add(pangoAttrWeightNew(int32(f.Weight)))
	}
	if f.Italic {
		add(pangoAttrStyleNew(pangoStyleItalic))
	}
	if features := fontFeatures(f.Variant); features != "" {
		add(pangoAttrFontFeaturesNew(cstr(features)))
	}
	add(pangoAttrForegroundNew(rgb16(s.Color)))
	if s.LetterSpacing != 0 {
		add(pangoAttrLetterSpacingNew(int32(s.LetterSpacing * pangoScale)))
	}
	if s.LineHeight > 0 {
		from := start
		for _, h := range append(holes, [2]int{end, end}) {
			to := min(max(h[0], from), end)
			if to > from {
				attr := pangoAttrLineHeightAbsoluteNew(int32(s.LineHeight * pangoScale))
				setAttributeRange(attr, uint32(from), uint32(to))
				pangoAttrListInsert(list, attr)
			}
			from = max(from, h[1])
		}
	}
	// Pango draws no dotted or dashed lines, no thickness and no offset,
	// and strikes through solid: placeDecorations draws those lines.
	if s.Decoration&driver.TextDecorationUnderline != 0 && !drawsUnderline(s) {
		line := int32(pangoUnderlineSingle)
		switch s.DecorationStyle {
		case driver.DecorationDouble:
			line = pangoUnderlineDouble
		case driver.DecorationWavy:
			line = pangoUnderlineError
		}
		add(pangoAttrUnderlineNew(line))
		if s.DecorationColor != 0 {
			add(pangoAttrUnderlineColorNew(rgb16(s.DecorationColor)))
		}
	}
	if s.Decoration&driver.TextDecorationLineThrough != 0 && !drawsStrike(s) {
		add(pangoAttrStrikethroughNew(1))
		if s.DecorationColor != 0 {
			add(pangoAttrStrikethroughColorNew(rgb16(s.DecorationColor)))
		}
	}
}

// drawsUnderline and drawsStrike report whether s's underline or
// strikethrough is drawn by placeDecorations rather than by Pango.
func drawsUnderline(s driver.TextStyle) bool {
	return s.DecorationStyle == driver.DecorationDotted || s.DecorationStyle == driver.DecorationDashed ||
		s.DecorationThickness > 0 || !s.UnderlineOffset.IsAuto()
}

func drawsStrike(s driver.TextStyle) bool {
	return s.DecorationStyle > driver.DecorationSolid || s.DecorationThickness > 0
}

// rgb16 returns c's channels as Pango's 16-bit ones.
func rgb16(c driver.Color) (r, g, b uint16) {
	return uint16(c>>24&0xff) * 0x101, uint16(c>>16&0xff) * 0x101, uint16(c>>8&0xff) * 0x101
}

// setAttributeRange fills a PangoAttribute's byte range, the first three
// fields of its public struct.
func setAttributeRange(attr uintptr, start, end uint32) {
	*(*uint32)(unsafe.Add(cPointer(attr), 8)) = start
	*(*uint32)(unsafe.Add(cPointer(attr), 12)) = end
}

// measure shapes and measures the text at width, in Pango units of the
// engine thread's font map.
func (p *preparedText) measure(width driver.Points) driver.Size {
	if p.layout == 0 {
		p.layout = newLayout(p.w.measureContext(p.style.RTL), &p.textSource, width)
	} else {
		pangoLayoutSetWidth(p.layout, p.pangoWidth(width, false))
	}
	var w, h int32
	pangoLayoutGetPixelSize(p.layout, &w, &h)
	return driver.Size{Width: driver.Points(w), Height: driver.Points(h)}
}

// Measure implements core.PreparedText.
func (p *preparedText) Measure(maxWidth driver.Points) driver.Size {
	size := p.measure(maxWidth)
	p.width, p.height = size.Width, size.Height
	return size
}

// Layout implements core.PreparedText.
func (p *preparedText) Layout(size driver.Size) driver.TextLayout {
	// Measure once at the final width so the engine's layout and the
	// main thread's agree on the text's height.
	measured := p.measure(size.Width)
	src := p.textSource
	src.spans = append([]driver.TextSpan(nil), p.spans...)
	l := &textLayout{textSource: src, width: size.Width, height: measured.Height}
	p.placeBackgrounds(l)
	p.placeDecorations(l)
	p.placeAttachments(l)
	p.placeLines(l)
	w := p.w
	l.carets.Set(func() driver.CaretLayout { return l.caretLines(w) })
	return l
}

// HitTest implements driver.TextHitTester.
func (l *textLayout) HitTest(p driver.Point) driver.Caret { return l.carets.HitTest(p) }

// CaretRect implements driver.TextHitTester.
func (l *textLayout) CaretRect(c driver.Caret) (driver.Bounds, bool) { return l.carets.CaretRect(c) }

// RangeRects implements driver.TextHitTester.
func (l *textLayout) RangeRects(start, end int, out []driver.Bounds) []driver.Bounds {
	return l.carets.RangeRects(start, end, out)
}

// caretLines reads where the lines' characters are from a PangoLayout of
// the text as the main thread draws it, made on a context of hit
// testing's own so either thread may ask: each cluster's logical extents,
// its direction from the sign of its position's width.
func (l *textLayout) caretLines(w *window) driver.CaretLayout {
	w.hitMu.Lock()
	defer w.hitMu.Unlock()
	i := 0
	if l.style.RTL {
		i = 1
	}
	if w.hitMap[i] == 0 {
		w.hitMap[i] = pangoFontMapCreateContext(pangoCairoFontMapNew())
		pangoContextSetBaseDir(w.hitMap[i], textDirection(l.style.RTL))
	}
	pl := pangoLayoutNew(w.hitMap[i])
	defer gObjectUnref(pl)
	applyLayout(pl, &l.textSource, l.width, true)
	text := l.text
	units := make([]int, len(text)+1) // UTF-16 offsets by byte offset
	for k, r := range text {
		n := 1
		if r >= 0x10000 {
			n = 2
		}
		for b := k + 1; b <= k+utf8.RuneLen(r) && b <= len(text); b++ {
			units[b] = units[k] + n
		}
	}
	unit := func(b int) int { return units[min(max(b, 0), len(text))] }
	var out driver.CaretLayout
	iter := pangoLayoutGetIter(pl)
	defer pangoLayoutIterFree(iter)
	var line, lineTo uintptr
	for {
		if at := pangoLayoutIterGetLine(iter); at != line {
			line = at
			from := int(*(*int32)(unsafe.Add(cPointer(at), 8)))
			to := from + int(*(*int32)(unsafe.Add(cPointer(at), 12)))
			lineTo = uintptr(to)
			var ink, logical pangoRectangle
			pangoLayoutIterGetLineExtents(iter, &ink, &logical)
			out = append(out, driver.CaretLine{Start: unit(from), End: unit(to), RTL: l.style.RTL,
				Box: driver.Bounds{Origin: driver.Point{X: fromPango(logical.X), Y: fromPango(logical.Y)},
					Size: driver.Size{Width: fromPango(logical.Width), Height: fromPango(logical.Height)}}})
		}
		if pangoLayoutIterGetRun(iter) != 0 && text != "" {
			idx := pangoLayoutIterGetIndex(iter)
			var ink, logical pangoRectangle
			pangoLayoutIterGetClusterExt(iter, &ink, &logical)
			var pos pangoRectangle
			pangoLayoutIndexToPos(pl, idx, &pos)
			cl := &out[len(out)-1]
			cl.Clusters = append(cl.Clusters, driver.CaretCluster{Start: int(idx), End: int(lineTo),
				Left: fromPango(logical.X), Right: fromPango(logical.X + logical.Width), RTL: pos.Width < 0})
		}
		if pangoLayoutIterNextCluster(iter) == 0 {
			break
		}
	}
	// The clusters came in visual order: each reaches to the next in
	// logical order, the last to the line's break.
	for k := range out {
		cl := &out[k]
		cs := cl.Clusters
		slices.SortFunc(cs, func(a, b driver.CaretCluster) int { return a.Start - b.Start })
		for j := range cs {
			if j+1 < len(cs) {
				cs[j].End = cs[j+1].Start
			}
			end := min(cs[j].End, len(text))
			for end > cs[j].Start && (text[end-1] == '\n' || text[end-1] == '\r') {
				end--
			}
			cs[j].Start, cs[j].End = unit(cs[j].Start), unit(end)
		}
		// The break itself takes no caret.
		for n := len(cs); n > 0 && cs[n-1].End <= cs[n-1].Start; n = len(cs) {
			cs = cs[:n-1]
		}
		cl.Clusters = cs
	}
	return out
}

// placeLines lists the layout's lines for OnTextLayout, with the metrics
// of the font each starts in.
func (p *preparedText) placeLines(l *textLayout) {
	if p.text == "" {
		return
	}
	iter := pangoLayoutGetIter(p.layout)
	defer pangoLayoutIterFree(iter)
	for {
		line := pangoLayoutIterGetLine(iter)
		from := int(*(*int32)(unsafe.Add(cPointer(line), 8)))
		to := from + int(*(*int32)(unsafe.Add(cPointer(line), 12)))
		var ink, logical pangoRectangle
		pangoLayoutIterGetLineExtents(iter, &ink, &logical)
		baseline := pangoLayoutIterGetBaseline(iter)
		font := p.styleAt(from).Font
		ext := p.w.fontExtent(font)
		l.lines = append(l.lines, driver.TextLine{Start: min(from, len(p.text)), End: min(to, len(p.text)),
			Bounds: driver.Bounds{Origin: driver.Point{X: fromPango(logical.X), Y: fromPango(logical.Y)},
				Size: driver.Size{Width: fromPango(logical.Width), Height: fromPango(logical.Height)}},
			Ascender: fromPango(baseline - logical.Y), Descender: fromPango(logical.Y + logical.Height - baseline),
			CapHeight: p.w.capHeight(font), XHeight: ext.XHeight})
		if pangoLayoutIterNextLine(iter) == 0 {
			break
		}
	}
}

// TextLines implements core.LineLayout.
func (l *textLayout) TextLines() []driver.TextLine { return l.lines }

// capHeight returns the height of f's capitals, from the ink of an "H".
// The engine thread calls it; it caches what it measured.
func (w *window) capHeight(f driver.Font) driver.Points {
	if h, ok := w.capHeights[f]; ok {
		return h
	}
	l := pangoLayoutNew(w.measureContext(false))
	attrs := pangoAttrListNew()
	addStyle(attrs, 1, 0, 1, driver.TextStyle{Font: f}, nil)
	pangoLayoutSetAttributes(l, attrs)
	pangoAttrListUnref(attrs)
	pangoLayoutSetText(l, cstr("H"), 1)
	var ink, logical pangoRectangle
	pangoLayoutGetExtents(l, &ink, &logical)
	h := fromPango(pangoLayoutGetBaseline(l) - ink.Y)
	gObjectUnref(l)
	if w.capHeights == nil {
		w.capHeights = map[driver.Font]driver.Points{}
	}
	w.capHeights[f] = h
	return h
}

// blank reports whether c is a space or a line break, which a line's
// end hangs and a span's background leaves out.
func blank(c byte) bool { return c == ' ' || c == '\n' || c == '\r' || c == '\t' }

// placeBackgrounds puts a box per line behind each span with a
// background, from its font's ascent to its descent.
func (p *preparedText) placeBackgrounds(l *textLayout) {
	has := false
	for _, sp := range p.spans {
		has = has || sp.Background.Color&0xff != 0
	}
	if !has || p.text == "" {
		return
	}
	iter := pangoLayoutGetIter(p.layout)
	defer pangoLayoutIterFree(iter)
	for {
		line := pangoLayoutIterGetLine(iter)
		from := int(*(*int32)(unsafe.Add(cPointer(line), 8)))
		to := from + int(*(*int32)(unsafe.Add(cPointer(line), 12)))
		baseline := fromPango(pangoLayoutIterGetBaseline(iter))
		for _, sp := range p.spans {
			bg := sp.Background
			if bg.Color&0xff == 0 {
				continue
			}
			a, b := max(sp.Start, from), min(sp.End, to, len(p.text))
			for b > a && blank(p.text[b-1]) {
				b--
			}
			if a >= b {
				continue
			}
			font := p.w.fontExtent(sp.Style.Font)
			h := float64(font.Ascent + font.Descent)
			var ranges uintptr
			var n int32
			pangoLayoutLineGetXRanges(line, int32(a), int32(b), &ranges, &n)
			if ranges == 0 {
				continue
			}
			xs := unsafe.Slice((*int32)(cPointer(ranges)), 2*n)
			for i := 0; i < int(n); i++ {
				x0, x1 := fromPango(xs[2*i]), fromPango(xs[2*i+1])
				if x1 <= x0 {
					continue
				}
				l.under = append(l.under, rect{x: float64(x0), y: float64(baseline - font.Ascent),
					w: float64(x1 - x0), h: h, r: min(float64(bg.Radius), h/2)})
				l.underColors = append(l.underColors, bg.Color)
			}
			gFree(ranges)
		}
		if pangoLayoutIterNextLine(iter) == 0 {
			break
		}
	}
}

// lineMetrics are where a font puts its decoration lines: each line's
// top above the baseline, and its thickness.
type lineMetrics struct {
	underline, underlineThickness driver.Points
	strike, strikeThickness       driver.Points
}

// fontLines returns f's decoration lines. The engine thread calls it;
// it caches what it read.
func (w *window) fontLines(f driver.Font) lineMetrics {
	if m, ok := w.lineMetrics[f]; ok {
		return m
	}
	desc := fontDescription(f)
	metrics := pangoContextGetMetrics(w.measureContext(false), desc, 0)
	pangoFontDescriptionFree(desc)
	m := lineMetrics{
		underline: fromPango(pangoFontMetricsUnderlinePos(metrics)), underlineThickness: fromPango(pangoFontMetricsUnderlineThick(metrics)),
		strike: fromPango(pangoFontMetricsStrikePos(metrics)), strikeThickness: fromPango(pangoFontMetricsStrikeThick(metrics)),
	}
	pangoFontMetricsUnref(metrics)
	if w.lineMetrics == nil {
		w.lineMetrics = map[driver.Font]lineMetrics{}
	}
	w.lineMetrics[f] = m
	return m
}

// decorationLine is a decoration line placed in the content box: its
// box's top left, width and thickness.
type decorationLine struct {
	x, y, w, t float64
	color      driver.Color
	style      driver.TextDecorationStyle
}

// placeDecorations puts the underlines and strikethroughs Pango cannot
// draw on each line their runs reach.
func (p *preparedText) placeDecorations(l *textLayout) {
	type run struct {
		start, end int
		style      driver.TextStyle
	}
	var runs []run
	if len(p.spans) == 0 {
		runs = append(runs, run{0, len(p.text), p.style})
	}
	for _, sp := range p.spans {
		runs = append(runs, run{sp.Start, sp.End, sp.Style})
	}
	runs = slices.DeleteFunc(runs, func(r run) bool {
		d := r.style.Decoration
		return !(d&driver.TextDecorationUnderline != 0 && drawsUnderline(r.style) ||
			d&driver.TextDecorationLineThrough != 0 && drawsStrike(r.style))
	})
	if len(runs) == 0 || p.text == "" {
		return
	}
	iter := pangoLayoutGetIter(p.layout)
	defer pangoLayoutIterFree(iter)
	for {
		line := pangoLayoutIterGetLine(iter)
		from := int(*(*int32)(unsafe.Add(cPointer(line), 8)))
		to := from + int(*(*int32)(unsafe.Add(cPointer(line), 12)))
		baseline := float64(fromPango(pangoLayoutIterGetBaseline(iter)))
		for _, r := range runs {
			a, b := max(r.start, from), min(r.end, to, len(p.text))
			for b > a && blank(p.text[b-1]) {
				b--
			}
			if a >= b {
				continue
			}
			st := r.style
			m := p.w.fontLines(st.Font)
			color := st.Color
			if st.DecorationColor != 0 {
				color = st.DecorationColor
			}
			var placed []decorationLine
			thick := func(font driver.Points) float64 {
				if st.DecorationThickness > 0 {
					return float64(st.DecorationThickness)
				}
				return float64(max(font, 1))
			}
			if st.Decoration&driver.TextDecorationUnderline != 0 && drawsUnderline(st) {
				top := baseline - float64(m.underline)
				if off, ok := st.UnderlineOffset.Points(); ok {
					top = baseline + float64(off)
				}
				placed = append(placed, decorationLine{y: top, t: thick(m.underlineThickness), color: color, style: st.DecorationStyle})
			}
			if st.Decoration&driver.TextDecorationLineThrough != 0 && drawsStrike(st) {
				t := thick(m.strikeThickness)
				// The font's line stays centered where it was.
				top := baseline - float64(m.strike) + float64(max(m.strikeThickness, 1))/2 - t/2
				placed = append(placed, decorationLine{y: top, t: t, color: color, style: st.DecorationStyle})
			}
			var ranges uintptr
			var n int32
			pangoLayoutLineGetXRanges(line, int32(a), int32(b), &ranges, &n)
			if ranges == 0 {
				continue
			}
			xs := unsafe.Slice((*int32)(cPointer(ranges)), 2*n)
			for i := 0; i < int(n); i++ {
				x0, x1 := float64(fromPango(xs[2*i])), float64(fromPango(xs[2*i+1]))
				if x1 <= x0 {
					continue
				}
				for _, d := range placed {
					d.x, d.w = x0, x1-x0
					l.over = append(l.over, d)
				}
			}
			gFree(ranges)
		}
		if pangoLayoutIterNextLine(iter) == 0 {
			break
		}
	}
}

// draw draws the line with the content box's top left at x, y: a box,
// two boxes a thickness apart, or a dotted, dashed or wavy stroke along
// its middle.
func (d decorationLine) draw(cr uintptr, x, y float64) {
	x0, top, t := x+d.x, y+d.y, d.t
	x1, mid := x0+d.w, top+t/2
	cairoSave(cr)
	setColor(cr, d.color)
	switch d.style {
	case driver.DecorationDouble:
		cairoRectangle(cr, x0, top-t, d.w, t)
		cairoRectangle(cr, x0, top+t, d.w, t)
		cairoFill(cr)
	case driver.DecorationDotted, driver.DecorationDashed:
		dash := [2]float64{t, t}
		if d.style == driver.DecorationDashed {
			dash = [2]float64{3 * t, 2 * t}
		}
		cairoSetLineWidth(cr, t)
		cairoSetDash(cr, &dash[0], 2, 0)
		cairoMoveTo(cr, x0, mid)
		cairoLineTo(cr, x1, mid)
		cairoStroke(cr)
	case driver.DecorationWavy:
		amp := max(1.5*t, 2)
		step := 2 * amp
		cairoSetLineWidth(cr, t)
		cairoMoveTo(cr, x0, mid)
		up := true
		for cx := x0; cx < x1; cx += step {
			nx := min(cx+step, x1)
			peak := mid + amp
			if up {
				peak = mid - amp
			}
			// The quadratic through peak, as a cubic.
			cairoCurveTo(cr, cx+2*((cx+nx)/2-cx)/3, mid+2*(peak-mid)/3, nx+2*((cx+nx)/2-nx)/3, mid+2*(peak-mid)/3, nx, mid)
			up = !up
		}
		cairoStroke(cr)
	default:
		cairoRectangle(cr, x0, top, d.w, t)
		cairoFill(cr)
	}
	cairoRestore(cr)
}

// placeAttachments finds each attachment's placeholder among the
// layout's runs: its top is its ascent above the baseline. One that is
// not a run of its own was cut off by the line limit or the ellipsis.
func (p *preparedText) placeAttachments(l *textLayout) {
	if len(p.atts) == 0 {
		return
	}
	l.attachments = make([]attachmentBox, len(p.atts))
	iter := pangoLayoutGetIter(p.layout)
	defer pangoLayoutIterFree(iter)
	for {
		if run := pangoLayoutIterGetRun(iter); run != 0 {
			item := *(*uintptr)(cPointer(run))
			offset := int(*(*int32)(cPointer(item)))
			length := int(*(*int32)(unsafe.Add(cPointer(item), 4)))
			for i, at := range p.atts {
				if at.Offset != offset || length != placeholderLen {
					continue
				}
				var logical pangoRectangle
				pangoLayoutIterGetRunExtents(iter, nil, &logical)
				top := fromPango(pangoLayoutIterGetBaseline(iter)) - at.Ascent(p.fonts[i])
				l.attachments[i] = attachmentBox{shown: true, box: driver.Bounds{
					Origin: driver.Point{X: fromPango(logical.X), Y: top}, Size: at.Size}}
			}
		}
		if pangoLayoutIterNextRun(iter) == 0 {
			break
		}
	}
}

// Attachment implements core.AttachmentLayout.
func (l *textLayout) Attachment(i int) (driver.Bounds, bool) {
	if i < 0 || i >= len(l.attachments) {
		return driver.Bounds{}, false
	}
	return l.attachments[i].box, l.attachments[i].shown
}

// Release implements core.PreparedText.
func (p *preparedText) Release() {
	if p.layout != 0 {
		gObjectUnref(p.layout)
		p.layout = 0
	}
}

// ensure makes the PangoLayout the main thread draws, once, within cr.
func (l *textLayout) ensure(cr uintptr) {
	if l.layout != 0 {
		pangoCairoUpdateLayout(cr, l.layout)
		return
	}
	l.layout = pangoCairoCreateLayout(cr)
	applyLayout(l.layout, &l.textSource, l.width, true)
	pangoCairoUpdateLayout(cr, l.layout)
}

// draw paints the text with its top-left at x, y: its spans'
// backgrounds, then its glyphs and their lines.
func (l *textLayout) draw(cr uintptr, x, y driver.Points) {
	if l.text == "" {
		return
	}
	for i, r := range l.under {
		r.x += float64(x)
		r.y += float64(y)
		setColor(cr, l.underColors[i])
		roundRect(cr, r)
		cairoFill(cr)
	}
	l.ensure(cr)
	clip := l.maxLines > 0 && l.ellipsize == driver.EllipsizeClip
	if clip {
		cairoSave(cr)
		cairoRectangle(cr, float64(x), float64(y), float64(l.width), float64(l.height))
		cairoClip(cr)
	}
	cairoMoveTo(cr, float64(x), float64(y))
	pangoCairoShowLayout(cr, l.layout)
	for _, d := range l.over {
		d.draw(cr, float64(x), float64(y))
	}
	if clip {
		cairoRestore(cr)
	}
}

// indexAt returns the byte offset of the character nearest x, y, in the
// text's points.
func (l *textLayout) indexAt(x, y driver.Points) int {
	if l.layout == 0 {
		return 0
	}
	var index, trailing int32
	pangoLayoutXyToIndex(l.layout, int32(x*pangoScale), int32(y*pangoScale), &index, &trailing)
	return int(index)
}

// Height returns the height the text laid out at.
func (l *textLayout) Height() driver.Points { return l.height }

// Release implements core.TextLayout on the main thread.
func (l *textLayout) Release() {
	if l.layout != 0 {
		gObjectUnref(l.layout)
		l.layout = 0
	}
}
