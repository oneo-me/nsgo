//go:build windows

package windows

// The DirectWrite text engine (engine goroutine) and text drawing (main
// thread).
//
// Prepare shapes the text once into an IDWriteTextLayout of DirectWrite's
// shared factory, which is free-threaded; Measure breaks it again at each
// width asked. Layout makes a layout of its own at the final width and
// hands it to the main thread, which draws it with Direct2D's
// DrawTextLayout: DirectWrite's font fallback picks faces for what the
// text's font lacks, and color fonts draw emoji in color. Span colors are
// drawing effects, brushes set on the layout when it is first drawn on a
// device. Span backgrounds and decorations are boxes found by hit testing,
// at layout, and drawn by the painter, as NSH's text.rs draws them, so a
// struck-through space is struck too.
//
// Line heights are DirectWrite's own (each line as tall as its fonts and
// attachments) unless the text sets LineHeight. Then lines are laid out on
// that uniform pitch, the baseline centering the text's font in it, and a
// line whose attachments reach past it grows: it is drawn moved down by
// what the lines above it grew, clipped to its band.
//
// Attachments are inline objects of their size: DirectWrite gives them
// their place in the line and the engine their view.

import (
	"math"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
)

const (
	// unbounded is the width of a layout with no limit: small enough that
	// positions in centered or trailing text keep their fractions.
	unbounded = 1 << 14
	// unboundedHeight is the height of a layout with no limit.
	unboundedHeight = 1e7
)

// drawnText is a TextLayout of this backend, drawn by it.
type drawnText interface {
	driver.TextLayout
	draw(p *painter, x, y float32)
}

// faceMetrics are what a font's lines and decorations are placed by, in
// ems.
type faceMetrics struct {
	ascent, descent, xHeight      float32
	capHeight                     float32
	underline, underlineThickness float32 // the position above the baseline
	strike, strikeThickness       float32
}

// defaultFaceMetrics stand for a font DirectWrite cannot find.
var defaultFaceMetrics = faceMetrics{ascent: 1.07, descent: 0.26, xHeight: 0.5, capHeight: 0.7,
	underline: -0.1, underlineThickness: 0.05, strike: 0.3, strikeThickness: 0.05}

type faceKey struct {
	family string
	weight driver.FontWeight
	italic bool
}

var (
	facesMu sync.Mutex
	faces   = map[faceKey]faceMetrics{}
	// systemFonts is the font collection text looks families up in: the
	// system's, with the app's fonts when it registered any, or nil.
	systemFonts = sync.OnceValue(func() *comObject {
		if appFonts != nil {
			return appFonts
		}
		f := dwriteFactory()
		if f == nil {
			return nil
		}
		var c *comObject
		if failed(f.call(dwGetSystemFontCollection, uintptr(unsafe.Pointer(&c)), 0)) {
			return nil
		}
		return c
	})
)

var (
	typographiesMu sync.Mutex
	// typographies holds an IDWriteTypography per font variant, which
	// texts share and keep for the process.
	typographies = map[driver.FontVariant]*comObject{}
)

// typography returns the IDWriteTypography that turns on v's features,
// or nil when it has none.
func typography(v driver.FontVariant) *comObject {
	tags := v.Tags()
	if len(tags) == 0 {
		return nil
	}
	typographiesMu.Lock()
	defer typographiesMu.Unlock()
	if t, ok := typographies[v]; ok {
		return t
	}
	var t *comObject
	if f := dwriteFactory(); f == nil || failed(f.call(dwCreateTypography, uintptr(unsafe.Pointer(&t)))) {
		t = nil
	}
	for _, tag := range tags {
		// A DWRITE_FONT_FEATURE is a tag and a parameter, passed by value
		// as a text range's start and length are.
		name := uint32(tag[0]) | uint32(tag[1])<<8 | uint32(tag[2])<<16 | uint32(tag[3])<<24
		if t != nil {
			t.callRange(typographyAddFontFeature, 0, name, 1)
		}
	}
	typographies[v] = t
	return t
}

// fontName returns the family DirectWrite is asked for.
func fontName(family string) string {
	switch family {
	case "":
		return defaultFontName
	case driver.FontMonospace:
		return "Consolas"
	}
	return family
}

func fontWeight(w driver.FontWeight) uintptr {
	if w == 0 {
		return 400
	}
	return uintptr(w)
}

func fontStyle(italic bool) uintptr {
	if italic {
		return dwStyleItalic
	}
	return dwStyleNormal
}

// metricsOf returns the metrics of the face font is set in, the default
// family's when the system has no such family.
func metricsOf(font driver.Font) faceMetrics {
	key := faceKey{fontName(font.Family), font.Weight, font.Italic}
	facesMu.Lock()
	defer facesMu.Unlock()
	if m, ok := faces[key]; ok {
		return m
	}
	m := defaultFaceMetrics
	if c := systemFonts(); c != nil {
		var index uint32
		var exists int32
		name := wide(key.family)
		c.call(collectionFindFamilyName, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&index)),
			uintptr(unsafe.Pointer(&exists)))
		if exists == 0 {
			c.call(collectionFindFamilyName, uintptr(unsafe.Pointer(wide(defaultFontName))), uintptr(unsafe.Pointer(&index)),
				uintptr(unsafe.Pointer(&exists)))
		}
		var family, f *comObject
		if exists != 0 && !failed(c.call(collectionGetFontFamily, uintptr(index), uintptr(unsafe.Pointer(&family)))) {
			if !failed(family.call(familyGetFirstMatchingFont, fontWeight(key.weight), dwStretchNormal, fontStyle(key.italic),
				uintptr(unsafe.Pointer(&f)))) {
				var fm fontMetrics
				f.call(fontGetMetrics, uintptr(unsafe.Pointer(&fm)))
				if em := float32(fm.designUnitsPerEm); em > 0 {
					m = faceMetrics{
						ascent: float32(fm.ascent) / em, descent: float32(fm.descent) / em, xHeight: float32(fm.xHeight) / em,
						capHeight: float32(fm.capHeight) / em,
						underline: float32(fm.underlinePosition) / em, underlineThickness: float32(fm.underlineThickness) / em,
						strike: float32(fm.strikethroughPosition) / em, strikeThickness: float32(fm.strikethroughThickness) / em,
					}
					if m.xHeight <= 0 {
						m.xHeight = 0.5
					}
				}
				f.release()
			}
			family.release()
		}
	}
	faces[key] = m
	return m
}

// textFormat returns a format of font, or nil.
func textFormat(font driver.Font) *comObject {
	f := dwriteFactory()
	if f == nil {
		return nil
	}
	var format *comObject
	if failed(f.callF(dwCreateTextFormat, 1<<5, uintptr(unsafe.Pointer(wide(fontName(font.Family)))), appFonts.ptr(),
		fontWeight(font.Weight), fontStyle(font.Italic), dwStretchNormal, f32(max(float32(font.Size), 0.1)),
		uintptr(unsafe.Pointer(wide(""))), uintptr(unsafe.Pointer(&format)))) {
		return nil
	}
	return format
}

// inlineObject is an attachment's placeholder: an IDWriteInlineObject of
// the attachment's size that draws nothing, as its view is drawn as a
// child of the text's. DirectWrite holds it by reference counting.
type inlineObject struct {
	vtbl    *[7]uintptr
	refs    int32
	metrics inlineMetrics
}

var (
	inlineVtbl = [7]uintptr{
		syscall.NewCallback(inlineQueryInterface),
		syscall.NewCallback(inlineAddRef),
		syscall.NewCallback(inlineRelease),
		syscall.NewCallback(inlineDraw),
		syscall.NewCallback(inlineGetMetrics),
		syscall.NewCallback(inlineGetOverhang),
		syscall.NewCallback(inlineGetBreakConditions),
	}
	inlinesMu sync.Mutex
	inlines   = map[*inlineObject]struct{}{}
)

// newInline returns a placeholder holding one reference, the caller's.
func newInline(m inlineMetrics) *inlineObject {
	o := &inlineObject{vtbl: &inlineVtbl, refs: 1, metrics: m}
	inlinesMu.Lock()
	inlines[o] = struct{}{}
	inlinesMu.Unlock()
	return o
}

func (o *inlineObject) ptr() uintptr { return uintptr(unsafe.Pointer(o)) }

func inlineQueryInterface(o *inlineObject, iid *guid, out *unsafe.Pointer) uintptr {
	if *iid == iidUnknown || *iid == iidDWriteInline {
		*out = unsafe.Pointer(o)
		inlineAddRef(o)
		return sOK
	}
	*out = nil
	return eNoInterface
}

func inlineAddRef(o *inlineObject) uintptr {
	inlinesMu.Lock()
	defer inlinesMu.Unlock()
	o.refs++
	return uintptr(o.refs)
}

func inlineRelease(o *inlineObject) uintptr {
	inlinesMu.Lock()
	defer inlinesMu.Unlock()
	if o.refs--; o.refs == 0 {
		delete(inlines, o)
	}
	return uintptr(o.refs)
}

// inlineDraw draws nothing. Its float arguments are not where a Go
// callback reads them, but it reads none.
func inlineDraw(o *inlineObject, context, renderer, a, b, c, d, e uintptr) uintptr { return sOK }

func inlineGetMetrics(o *inlineObject, out *inlineMetrics) uintptr {
	*out = o.metrics
	return sOK
}

func inlineGetOverhang(o *inlineObject, out *overhangMetrics) uintptr {
	*out = overhangMetrics{}
	return sOK
}

func inlineGetBreakConditions(o *inlineObject, before, after *uint32) uintptr {
	*before, *after = 0, 0 // DWRITE_BREAK_CONDITION_NEUTRAL
	return sOK
}

// textRun is a range of the text in one style, in UTF-16 units.
type textRun struct {
	start, end uint32
	style      driver.TextStyle
	bg         driver.TextBackground
}

// textAttachment is an attachment's place and extents, in points.
type textAttachment struct {
	pos          uint32 // its UTF-16 offset
	size         driver.Size
	top          float32 // how far its top is above the baseline
	above, below float32
}

// preparedText is text shaped once, measured on the engine goroutine.
type preparedText struct {
	w           *window
	units       []uint16
	offsets     []int // each unit's UTF-8 offset, and the text's length last
	style       driver.TextStyle
	runs        []textRun
	attachments []textAttachment
	maxLines    int
	ellipsize   driver.EllipsizeMode
	shadow      driver.TextShadow
	// cut is the text, kept when it ellipsizes at its head or middle,
	// which DirectWrite's trimming cannot: Layout cuts it, its last line's
	// rest when it has several (rest).
	cut  *driver.AttributedText
	rest bool
	// step is the uniform line height, 0 for DirectWrite's own, and
	// baseline where the baseline is in each of its lines.
	step, baseline float32
	layout         *comObject // the engine's, for measuring
	inlines        []*inlineObject
}

// Prepare implements core.TextEngine on the engine goroutine.
func (w *window) Prepare(t *driver.AttributedText) driver.PreparedText {
	pt := &preparedText{w: w, style: t.Style, maxLines: t.MaxLines, ellipsize: t.Ellipsize, shadow: t.Shadow}
	if t.Ellipsize == driver.EllipsizeHead || t.Ellipsize == driver.EllipsizeMiddle {
		switch {
		case t.MaxLines == 1 && !strings.ContainsAny(t.Text, "\n\r"):
			c := *t
			pt.cut = &c
		case t.MaxLines > 1:
			c := *t
			pt.cut, pt.rest = &c, true
		}
	}
	units16 := func(s string) {
		for i, r := range s {
			n := len(pt.units)
			pt.units = utf16.AppendRune(pt.units, r)
			for range len(pt.units) - n {
				pt.offsets = append(pt.offsets, i)
			}
		}
	}
	units16(t.Text)
	pt.offsets = append(pt.offsets, len(t.Text))
	// unit returns the UTF-16 offset of byte offset b.
	unit := func(b int) uint32 {
		lo, hi := 0, len(pt.offsets)-1
		for lo < hi {
			mid := (lo + hi) / 2
			if pt.offsets[mid] < b {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return uint32(lo)
	}
	at := 0
	add := func(start, end int, style driver.TextStyle, bg driver.TextBackground) {
		if end > start {
			pt.runs = append(pt.runs, textRun{unit(start), unit(end), style, bg})
		}
	}
	for _, sp := range t.Spans {
		start, end := min(max(sp.Start, at), len(t.Text)), min(max(sp.End, at), len(t.Text))
		add(at, start, t.Style, driver.TextBackground{})
		add(start, end, sp.Style, sp.Background)
		at = max(at, end)
	}
	add(at, len(t.Text), t.Style, driver.TextBackground{})
	for _, a := range t.Attachments {
		if a.Offset < 0 || a.Offset >= len(t.Text) {
			continue
		}
		if r, _ := utf8.DecodeRuneInString(t.Text[a.Offset:]); r != driver.AttachmentRune {
			continue
		}
		font := pt.runAt(unit(a.Offset)).style.Font
		m := metricsOf(font)
		size := float32(font.Size)
		extent := driver.FontExtent{Ascent: driver.Points(m.ascent * size), Descent: driver.Points(m.descent * size),
			XHeight: driver.Points(m.xHeight * size)}
		above, below := a.Extent(extent)
		pt.attachments = append(pt.attachments, textAttachment{pos: unit(a.Offset), size: a.Size,
			top: float32(a.Ascent(extent)), above: float32(above), below: float32(below)})
	}
	if t.Style.LineHeight > 0 {
		m := metricsOf(t.Style.Font)
		size := float32(t.Style.Font.Size)
		pt.step = float32(t.Style.LineHeight)
		// The font centered in the line.
		pt.baseline = (pt.step-(m.ascent+m.descent)*size)/2 + m.ascent*size
	}
	for _, a := range pt.attachments {
		pt.inlines = append(pt.inlines, newInline(inlineMetrics{width: float32(a.size.Width),
			height: a.above + a.below, baseline: a.above}))
	}
	pt.layout = pt.build()
	return pt
}

// runAt returns the run holding unit i.
func (pt *preparedText) runAt(i uint32) *textRun {
	for k := range pt.runs {
		if i < pt.runs[k].end {
			return &pt.runs[k]
		}
	}
	if n := len(pt.runs); n > 0 {
		return &pt.runs[n-1]
	}
	return &textRun{style: pt.style}
}

// build returns a layout of the text with no width limit, or nil.
func (pt *preparedText) build() *comObject {
	format := textFormat(pt.style.Font)
	if format == nil {
		return nil
	}
	defer format.release()
	align := uintptr(dwAlignLeading)
	switch pt.style.Align {
	case driver.TextAlignCenter:
		align = dwAlignCenter
	case driver.TextAlignEnd:
		align = dwAlignTrailing
	case driver.TextAlignJustify:
		align = dwAlignJustified
	}
	format.call(formatSetTextAlignment, align)
	if pt.style.RTL {
		// Leading and trailing alignments follow it.
		format.call(formatSetReadingDirection, dwReadingRTL)
	}
	if pt.maxLines == 1 {
		// A text limited to one line does not wrap: it is truncated.
		format.call(formatSetWordWrapping, dwNoWrap)
	}
	if pt.step > 0 {
		format.callF(formatSetLineSpacing, 0b110, dwLineSpacingUniform, f32(pt.step), f32(pt.baseline))
	}
	if pt.maxLines > 0 && pt.ellipsize != driver.EllipsizeClip {
		var sign *comObject
		dwriteFactory().call(dwCreateEllipsisTrimmingSign, format.ptr(), uintptr(unsafe.Pointer(&sign)))
		trim := trimming{granularity: dwTrimmingCharacter}
		format.call(formatSetTrimming, uintptr(unsafe.Pointer(&trim)), sign.ptr())
		sign.release()
	}
	units := pt.units
	if len(units) == 0 {
		units = []uint16{0}
	}
	var layout *comObject
	if failed(dwriteFactory().callF(dwCreateTextLayout, 0b11000, uintptr(unsafe.Pointer(&units[0])), uintptr(len(pt.units)),
		format.ptr(), f32(unbounded), f32(unboundedHeight), uintptr(unsafe.Pointer(&layout)))) {
		return nil
	}
	spacing := layout.query(&iidDWriteTextLayout1)
	for _, r := range pt.runs {
		n := r.end - r.start
		f := r.style.Font
		name := wide(fontName(f.Family))
		layout.callRange(layoutSetFontFamilyName, 0, r.start, n, uintptr(unsafe.Pointer(name)))
		layout.callRange(layoutSetFontWeight, 0, r.start, n, fontWeight(f.Weight))
		layout.callRange(layoutSetFontStyle, 0, r.start, n, fontStyle(f.Italic))
		layout.callRange(layoutSetFontSize, 1, r.start, n, f32(max(float32(f.Size), 0.1)))
		if ls := float32(r.style.LetterSpacing); ls != 0 && spacing != nil {
			spacing.callRange(layoutSetCharacterSpacing, 0b111, r.start, n, f32(0), f32(ls), f32(0))
		}
		if t := typography(f.Variant); t != nil {
			layout.callRange(layoutSetTypography, 0, r.start, n, t.ptr())
		}
	}
	spacing.release()
	for i, a := range pt.attachments {
		layout.callRange(layoutSetInlineObject, 0, a.pos, 1, pt.inlines[i].ptr())
	}
	return layout
}

// lineMetricsOf returns layout's lines.
func lineMetricsOf(layout *comObject) []lineMetrics {
	var count uint32
	layout.call(layoutGetLineMetrics, 0, 0, uintptr(unsafe.Pointer(&count)))
	if count == 0 {
		return nil
	}
	lines := make([]lineMetrics, count)
	layout.call(layoutGetLineMetrics, uintptr(unsafe.Pointer(&lines[0])), uintptr(count), uintptr(unsafe.Pointer(&count)))
	return lines[:min(int(count), len(lines))]
}

// lineBox is a line where it is drawn: its units, its top and baseline,
// and how far it moves down from where DirectWrite put it.
type lineBox struct {
	start, end        uint32 // end includes trailing spaces and the line break
	trailing, newline uint32
	top, height       float32
	baseline          float32
	shift             float32
	dwTop, dwHeight   float32
}

// stack places the lines of a layout.
func (pt *preparedText) stack(lines []lineMetrics) []lineBox {
	boxes := make([]lineBox, len(lines))
	var start uint32
	var top, dwTop float32
	k := 0 // the next attachment
	for i, l := range lines {
		b := lineBox{start: start, end: start + l.length, trailing: l.trailingWhitespace, newline: l.newline,
			top: top, height: l.height, baseline: top + l.baseline, dwTop: dwTop, dwHeight: l.height}
		if pt.step > 0 {
			above, below := pt.baseline, pt.step-pt.baseline
			for ; k < len(pt.attachments) && pt.attachments[k].pos < b.end; k++ {
				above = max(above, pt.attachments[k].above)
				below = max(below, pt.attachments[k].below)
			}
			b.height, b.baseline = above+below, top+above
			b.shift = b.baseline - (dwTop + l.baseline)
		}
		boxes[i] = b
		start, top, dwTop = b.end, top+b.height, dwTop+l.height
	}
	return boxes
}

// shown returns how many of n lines show.
func (pt *preparedText) shown(n int) int {
	if pt.maxLines > 0 {
		return min(n, pt.maxLines)
	}
	return n
}

// inkWidth returns how wide units from start to end are drawn in layout.
func inkWidth(layout *comObject, start, end uint32) (left, right float32) {
	left, right = float32(math.Inf(1)), float32(math.Inf(-1))
	if end <= start {
		return 0, 0
	}
	var buf [16]hitTestMetrics
	var count uint32
	layout.callF(layoutHitTestTextRange, 0b1100, uintptr(start), uintptr(end-start), f32(0), f32(0),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&count)))
	for _, m := range buf[:min(int(count), len(buf))] {
		left, right = min(left, m.left), max(right, m.left+m.width)
	}
	if right < left {
		return 0, 0
	}
	return left, right
}

// spanEdges returns where units from start to end begin and end on their
// line, trailing spaces included, which hit testing a range leaves out.
func spanEdges(layout *comObject, start, end uint32) (left, right float32) {
	if end <= start {
		return 0, 0
	}
	edge := func(pos uint32, trailing uintptr) float32 {
		var x, y float32
		var m hitTestMetrics
		layout.call(layoutHitTestTextPosition, uintptr(pos), trailing, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)),
			uintptr(unsafe.Pointer(&m)))
		return x
	}
	a, b := edge(start, 0), edge(end-1, 1)
	return min(a, b), max(a, b)
}

func (pt *preparedText) Measure(maxWidth driver.Points) driver.Size {
	if pt.layout == nil {
		return driver.Size{}
	}
	limit := float32(unbounded)
	if !maxWidth.IsAuto() {
		limit = max(float32(maxWidth), 0)
	}
	pt.layout.callF(layoutSetMaxWidth, 1, f32(limit))
	boxes := pt.stack(lineMetricsOf(pt.layout))
	shown := pt.shown(len(boxes))
	var width, height float32
	for _, b := range boxes[:shown] {
		height += b.height
	}
	if shown < len(boxes) {
		for _, b := range boxes[:shown] {
			l, r := spanEdges(pt.layout, b.start, b.end-b.newline)
			width = max(width, r-l)
		}
	} else {
		var m textMetrics
		pt.layout.call(layoutGetMetrics, uintptr(unsafe.Pointer(&m)))
		width = m.widthWithTrailing
	}
	// Whole points: frames are rounded to them, and a frame narrower than
	// the measured text would wrap it again.
	width = float32(math.Ceil(float64(width) - 1e-3))
	if limit < unbounded {
		width = min(width, limit)
	}
	return driver.Size{Width: driver.Points(width), Height: driver.Points(math.Ceil(float64(height) - 1e-3))}
}

// Layout lays the text out at size in a layout of its own, which the
// main thread draws and releases.
func (pt *preparedText) Layout(size driver.Size) driver.TextLayout {
	// The most characters that fit around the ellipsis, as a text of
	// their own.
	fits := func(c *driver.AttributedText) bool {
		p := pt.w.Prepare(c)
		defer p.Release()
		return p.Measure(driver.Auto).Width <= size.Width+0.01
	}
	var cut driver.AttributedText
	switch {
	case pt.cut == nil:
	case pt.rest:
		// The lines before the last as they wrap, the rest cut.
		whole := *pt.cut
		whole.MaxLines = 0
		p := pt.w.Prepare(&whole)
		l := p.Layout(size).(*textLayout)
		lines := l.TextLines()
		l.Release()
		p.Release()
		if len(lines) > pt.cut.MaxLines {
			cut = pt.cut.CutRest(lines[pt.cut.MaxLines-1].Start, fits)
		}
	case pt.Measure(driver.Auto).Width > size.Width:
		cut = pt.cut.CutToFit(fits)
	}
	if cut.Text != "" {
		p := pt.w.Prepare(&cut)
		defer p.Release()
		l := p.Layout(size).(*textLayout)
		// Attachments by the app's indices: those cut out are hidden.
		if len(pt.cut.Attachments) > 0 {
			boxes := make([]attachmentBox, len(pt.cut.Attachments))
			for k, o := range cut.Origin {
				if k < len(l.attachments) && o < len(boxes) {
					boxes[o] = l.attachments[k]
				}
			}
			l.attachments = boxes
		}
		return l
	}
	l := &textLayout{color: pt.style.Color, offsets: pt.offsets, attachments: make([]attachmentBox, len(pt.attachments)),
		shadow: pt.shadow, runs: pt.runs, style: pt.style}
	layout := pt.build()
	if layout == nil {
		return l
	}
	l.layout = layout
	l.carets.Set(l.caretLines)
	// A little over the width, so that rounding does not wrap what fit.
	layout.callF(layoutSetMaxWidth, 1, f32(max(float32(size.Width), 0)+0.01))
	boxes := pt.stack(lineMetricsOf(layout))
	shown := pt.shown(len(boxes))
	if shown < len(boxes) {
		// DirectWrite trims what does not fit its height and ends the
		// last line that does with the ellipsis.
		var h float32
		for _, b := range boxes[:shown] {
			h += b.dwHeight
		}
		layout.callF(layoutSetMaxHeight, 1, f32(h+0.01))
	}
	l.lines = boxes[:shown]
	if pt.maxLines > 0 && pt.ellipsize == driver.EllipsizeClip && len(l.lines) > 0 {
		last := l.lines[len(l.lines)-1]
		l.clip = rectF{0, 0, float32(size.Width), last.top + last.height}
	}
	for i, b := range l.lines {
		if n := len(l.groups); n > 0 && l.groups[n-1].shift == b.shift {
			l.groups[n-1].to = i + 1
		} else {
			l.groups = append(l.groups, lineGroup{from: i, to: i + 1, shift: b.shift})
		}
	}
	for i, a := range pt.attachments {
		line := -1
		for k, b := range l.lines {
			if a.pos >= b.start && a.pos < b.end {
				line = k
			}
		}
		if line < 0 {
			continue
		}
		var x, y float32
		var m hitTestMetrics
		layout.call(layoutHitTestTextPosition, uintptr(a.pos), 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)),
			uintptr(unsafe.Pointer(&m)))
		if m.isTrimmed != 0 {
			continue
		}
		l.attachments[i] = attachmentBox{shown: true, box: driver.Bounds{
			Origin: driver.Point{X: driver.Points(x), Y: driver.Points(l.lines[line].baseline - a.top)}, Size: a.size}}
	}
	thin := float32(1 / pt.w.scale)
	for _, r := range pt.runs {
		if visible(r.style.Color) && r.style.Color != l.color {
			l.colors = append(l.colors, colorRange{r.start, r.end - r.start, r.style.Color})
		}
		bg := visible(r.bg.Color)
		deco := r.style.Decoration & (driver.TextDecorationUnderline | driver.TextDecorationLineThrough)
		lineColor := r.style.Color
		if r.style.DecorationColor != 0 {
			lineColor = r.style.DecorationColor
		}
		if !bg && deco == 0 {
			continue
		}
		m := metricsOf(r.style.Font)
		size := float32(r.style.Font.Size)
		for _, b := range l.lines {
			if bg {
				// Hanging spaces at the line's end are not painted.
				if x0, x1 := inkWidth(layout, max(r.start, b.start), min(r.end, b.end-b.trailing-b.newline)); x1 > x0 {
					box := rectF{x0, b.baseline - m.ascent*size, x1, b.baseline + m.descent*size}
					radius := max(min(float32(r.bg.Radius), (box.bottom-box.top)/2), 0)
					l.backgrounds = append(l.backgrounds, backgroundBox{box, r.bg.Color, radius})
				}
			}
			if deco == 0 {
				continue
			}
			x0, x1 := spanEdges(layout, max(r.start, b.start), min(r.end, b.end-b.newline))
			if x1 <= x0 {
				continue
			}
			line := func(position, thickness float32) {
				t := max(thickness*size, thin)
				if dt := float32(r.style.DecorationThickness); dt > 0 {
					t = dt
				}
				y := b.baseline - position*size
				l.decorations = append(l.decorations, decoration{rectF{x0, y - t/2, x1, y + t/2}, lineColor, r.style.DecorationStyle})
			}
			if deco&driver.TextDecorationUnderline != 0 {
				position := m.underline
				if off, ok := r.style.UnderlineOffset.Points(); ok {
					// The app's offset puts the line's top that far below
					// the baseline.
					t := max(m.underlineThickness*size, thin)
					if dt := float32(r.style.DecorationThickness); dt > 0 {
						t = dt
					}
					position = -(float32(off) + t/2) / size
				}
				line(position, m.underlineThickness)
			}
			if deco&driver.TextDecorationLineThrough != 0 {
				line(m.strike, m.strikeThickness)
			}
		}
	}
	return l
}

func (pt *preparedText) Release() {
	releaseAll(&pt.layout)
	for _, o := range pt.inlines {
		inlineRelease(o)
	}
	pt.inlines = nil
}

// colorRange is a span's color, set as a drawing effect.
type colorRange struct {
	start, length uint32
	color         driver.Color
}

// backgroundBox is a span's background on one line, in the text's points.
type backgroundBox struct {
	r      rectF
	color  driver.Color
	radius float32
}

// decoration is an underline or a strikethrough on one line: its line's
// box, centered on it.
type decoration struct {
	r     rectF
	color driver.Color
	style driver.TextDecorationStyle
}

// draw paints d moved by x, y in its style: a solid line fills its box,
// the others are drawn along its middle.
func (d decoration) draw(p *painter, x, y float32) {
	r := rectF{d.r.left + x, d.r.top + y, d.r.right + x, d.r.bottom + y}
	t := r.bottom - r.top
	switch d.style {
	case driver.DecorationDouble:
		p.fillRect(rectF{r.left, r.top - t, r.right, r.bottom - t}, d.color)
		p.fillRect(rectF{r.left, r.top + t, r.right, r.bottom + t}, d.color)
	case driver.DecorationDotted, driver.DecorationDashed:
		on, step := t, 2*t
		if d.style == driver.DecorationDashed {
			on, step = 3*t, 5*t
		}
		for a := r.left; a < r.right; a += step {
			p.fillRect(rectF{a, r.top, min(a+on, r.right), r.bottom}, d.color)
		}
	case driver.DecorationWavy:
		// A band along a sine, a wavelength four thicknesses long.
		amp, wave := max(t, 1), 4*max(t, 1)
		mid := (r.top + r.bottom) / 2
		var top, bottom []pointF
		for a := r.left; ; a += wave / 8 {
			a = min(a, r.right)
			dy := amp * float32(math.Sin(float64(2*math.Pi*(a-r.left)/wave)))
			top = append(top, pointF{a, mid + dy - t/2})
			bottom = append(bottom, pointF{a, mid + dy + t/2})
			if a >= r.right {
				break
			}
		}
		for i := len(bottom) - 1; i >= 0; i-- {
			top = append(top, bottom[i])
		}
		if g := p.polygon(top); g != nil {
			p.fillGeometry(g, d.color)
			g.release()
		}
	default:
		p.fillRect(r, d.color)
	}
}

type attachmentBox struct {
	box   driver.Bounds // in the content box, in points
	shown bool
}

// lineGroup is lines drawn together, moved down by shift.
type lineGroup struct {
	from, to int
	shift    float32
}

// textLayout is text laid out at its final size, drawn on the main
// thread.
type textLayout struct {
	layout      *comObject
	color       driver.Color
	offsets     []int
	lines       []lineBox
	groups      []lineGroup
	colors      []colorRange
	backgrounds []backgroundBox
	decorations []decoration
	attachments []attachmentBox
	// runs and style are the text's, for the fonts of its lines.
	runs  []textRun
	style driver.TextStyle
	// clip, when not empty, cuts the lines a clipped text shows.
	clip rectF
	// shadow is drawn behind the glyphs: their outline recorded, then
	// shaded by an effect, in shadowGeneration.
	shadow                   driver.TextShadow
	shadowList, shadowEffect *comObject
	shadowImage              *comObject
	shadowGeneration         uint64
	// The brushes of colors, set as drawing effects in generation.
	brushes    []*comObject
	generation uint64
	// carets hit-tests the lines, read from the layout when first asked.
	carets driver.LazyCarets
}

// HitTest implements driver.TextHitTester.
func (l *textLayout) HitTest(p driver.Point) driver.Caret { return l.carets.HitTest(p) }

// CaretRect implements driver.TextHitTester.
func (l *textLayout) CaretRect(c driver.Caret) (driver.Bounds, bool) { return l.carets.CaretRect(c) }

// RangeRects implements driver.TextHitTester.
func (l *textLayout) RangeRects(start, end int, out []driver.Bounds) []driver.Bounds {
	return l.carets.RangeRects(start, end, out)
}

// caretLines reads where the lines' clusters are from DirectWrite: each
// cluster's box and bidi level at its first unit, line by line, without
// the line's break, its y the line's box.
func (l *textLayout) caretLines() driver.CaretLayout {
	out := make(driver.CaretLayout, 0, len(l.lines))
	for _, b := range l.lines {
		cl := driver.CaretLine{Start: int(b.start), End: int(b.end),
			Box: driver.Bounds{Origin: driver.Point{Y: driver.Points(b.top)}, Size: driver.Size{Height: driver.Points(b.height)}}}
		left, right := spanEdges(l.layout, b.start, b.end-b.trailing-b.newline)
		cl.Box.Origin.X, cl.Box.Size.Width = driver.Points(left), driver.Points(right-left)
		for pos := b.start; pos < b.end-b.newline; {
			var x, y float32
			var m hitTestMetrics
			l.layout.call(layoutHitTestTextPosition, uintptr(pos), 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)),
				uintptr(unsafe.Pointer(&m)))
			end := max(m.position+m.length, pos+1)
			cl.Clusters = append(cl.Clusters, driver.CaretCluster{Start: int(pos), End: int(end),
				Left: driver.Points(m.left), Right: driver.Points(m.left + m.width), RTL: m.bidiLevel&1 != 0})
			pos = end
		}
		cl.RTL = l.style.RTL
		out = append(out, cl)
	}
	return out
}

// Attachment implements core.AttachmentLayout.
func (l *textLayout) Attachment(i int) (driver.Bounds, bool) {
	if i < 0 || i >= len(l.attachments) {
		return driver.Bounds{}, false
	}
	return l.attachments[i].box, l.attachments[i].shown
}

// Release implements core.TextLayout on the main thread.
func (l *textLayout) Release() {
	l.carets.Drop()
	releaseAll(&l.layout, &l.shadowImage, &l.shadowEffect, &l.shadowList)
	for i := range l.brushes {
		releaseAll(&l.brushes[i])
	}
	l.brushes = nil
}

// effects sets the spans' colors as brushes of the current device.
func (l *textLayout) effects(p *painter) {
	if l.generation == gfxGeneration && len(l.brushes) == len(l.colors) {
		return
	}
	for i := range l.brushes {
		releaseAll(&l.brushes[i])
	}
	l.brushes = l.brushes[:0]
	for _, c := range l.colors {
		col := colorOf(c.color)
		var brush *comObject
		p.dc.call(rtCreateSolidColorBrush, uintptr(unsafe.Pointer(&col)), 0, uintptr(unsafe.Pointer(&brush)))
		l.brushes = append(l.brushes, brush)
		l.layout.callRange(layoutSetDrawingEffect, 0, c.start, c.length, brush.ptr())
	}
	l.generation = gfxGeneration
}

// draw paints the span backgrounds, the glyphs, then the decorations, at
// x, y in the view; attachments are the text view's children.
func (l *textLayout) draw(p *painter, x, y float32) {
	if l.layout == nil {
		return
	}
	for _, b := range l.backgrounds {
		p.fillRounded(rectF{b.r.left + x, b.r.top + y, b.r.right + x, b.r.bottom + y}, b.radius, b.color)
	}
	l.effects(p)
	if !l.clip.empty() {
		p.pushClipRect(rectF{l.clip.left + x, l.clip.top + y, l.clip.right + x, l.clip.bottom + y})
	}
	if l.shadow.Color != 0 {
		l.drawShadow(p, x, y)
	}
	for _, g := range l.groups {
		clip := len(l.groups) > 1
		if clip {
			first, last := l.lines[g.from], l.lines[g.to-1]
			p.pushClipRect(rectF{-unbounded, y + first.top, unbounded, y + last.top + last.height})
		}
		p.dc.callPoint(rtDrawTextLayout, x, y+g.shift, l.layout.ptr(), p.solid(l.color).ptr(), d2dDrawTextEnableColorFont)
		if clip {
			p.pop()
		}
	}
	for _, d := range l.decorations {
		d.draw(p, x, y)
	}
	if !l.clip.empty() {
		p.pop()
	}
	runtime.KeepAlive(l)
}

// drawShadow draws the shadow of the glyphs: the layout recorded on the
// second context, shaded in the shadow's color by Direct2D's shadow
// effect, its blur a standard deviation half the radius, as Core
// Graphics blurs.
func (l *textLayout) drawShadow(p *painter, x, y float32) {
	if l.shadowImage == nil || l.shadowGeneration != gfxGeneration {
		releaseAll(&l.shadowImage, &l.shadowEffect, &l.shadowList)
		l.shadowGeneration = gfxGeneration
		bake := p.g.bake
		if failed(bake.call(dcCreateCommandList, uintptr(unsafe.Pointer(&l.shadowList)))) {
			return
		}
		bake.call(dcSetTarget, l.shadowList.ptr())
		bake.callF(rtSetDpi, 0b11, f32(96*p.scale), f32(96*p.scale))
		bake.call(rtBeginDraw)
		t := identity.d2d()
		bake.call(rtSetTransform, uintptr(unsafe.Pointer(&t)))
		col := colorOf(driver.RGB(0))
		var brush *comObject
		bake.call(rtCreateSolidColorBrush, uintptr(unsafe.Pointer(&col)), 0, uintptr(unsafe.Pointer(&brush)))
		if brush != nil {
			for _, g := range l.groups {
				bake.callPoint(rtDrawTextLayout, 0, g.shift, l.layout.ptr(), brush.ptr(), d2dDrawTextEnableColorFont)
			}
			brush.release()
		}
		hr := bake.call(rtEndDraw, 0, 0)
		bake.call(dcSetTarget, 0)
		if failed(hr) || failed(l.shadowList.call(commandListClose)) ||
			failed(p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(&clsidShadow)), uintptr(unsafe.Pointer(&l.shadowEffect)))) {
			releaseAll(&l.shadowEffect, &l.shadowList)
			return
		}
		const standardDeviation, color = 0, 1
		sigma := max(float32(l.shadow.Radius)/2, 0)
		c := colorOf(l.shadow.Color)
		l.shadowEffect.call(effectSetValue, standardDeviation, d2dPropertyFloat, uintptr(unsafe.Pointer(&sigma)), 4)
		l.shadowEffect.call(effectSetValue, color, d2dPropertyVector4, uintptr(unsafe.Pointer(&c)), 16)
		l.shadowEffect.call(effectSetInput, 0, l.shadowList.ptr(), 1)
		l.shadowEffect.call(effectGetOutput, uintptr(unsafe.Pointer(&l.shadowImage)))
		if l.shadowImage == nil {
			return
		}
	}
	at := pointF{x + float32(l.shadow.Offset.X), y + float32(l.shadow.Offset.Y)}
	p.dc.call(dcDrawImage, l.shadowImage.ptr(), uintptr(unsafe.Pointer(&at)), 0, d2dInterpolationLinear, 0)
}

// fonts returns the metrics and size of the font unit i is set in.
func (l *textLayout) fonts(i uint32) (faceMetrics, float32) {
	f := l.style.Font
	for _, r := range l.runs {
		if i >= r.start && i < r.end {
			f = r.style.Font
			break
		}
	}
	return metricsOf(f), float32(f.Size)
}

// TextLines implements core.LineLayout: each line's units, box and the
// metrics of the font it starts in.
func (l *textLayout) TextLines() []driver.TextLine {
	out := make([]driver.TextLine, 0, len(l.lines))
	end := len(l.offsets) - 1
	for _, b := range l.lines {
		left, right := float32(0), float32(0)
		if l.layout != nil {
			left, right = spanEdges(l.layout, b.start, b.end-b.trailing-b.newline)
		}
		m, size := l.fonts(b.start)
		out = append(out, driver.TextLine{
			Start: l.offsets[min(int(b.start), end)], End: l.offsets[min(int(b.end), end)],
			Bounds: driver.Bounds{Origin: driver.Point{X: driver.Points(left), Y: driver.Points(b.top)},
				Size: driver.Size{Width: driver.Points(right - left), Height: driver.Points(b.height)}},
			Ascender: driver.Points(b.baseline - b.top), Descender: driver.Points(b.top + b.height - b.baseline),
			CapHeight: driver.Points(m.capHeight * size), XHeight: driver.Points(m.xHeight * size),
		})
	}
	return out
}

// indexAt returns the UTF-8 offset of the character at x, y, in points
// from the content's origin, or -1.
func (l *textLayout) indexAt(x, y float32) int {
	if l.layout == nil {
		return -1
	}
	for _, b := range l.lines {
		if y < b.top || y >= b.top+b.height {
			continue
		}
		var trailing, inside int32
		var m hitTestMetrics
		l.layout.callF(layoutHitTestPoint, 0b11, f32(x), f32(y-b.shift), uintptr(unsafe.Pointer(&trailing)),
			uintptr(unsafe.Pointer(&inside)), uintptr(unsafe.Pointer(&m)))
		if inside == 0 || int(m.position) >= len(l.offsets) {
			return -1
		}
		return l.offsets[m.position]
	}
	return -1
}
