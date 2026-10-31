//go:build darwin

package apple

import (
	"structs"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/driver"
)

// What CoreText draws besides glyphs: attachments' placeholders sized by
// run delegates, span backgrounds, strikethrough and wavy underlines,
// which CoreText has no attribute for.

var (
	ctRunDelegateCreate     func(callbacks *runDelegateCallbacks, refCon uintptr) uintptr
	ctFontGetXHeight        func(uintptr) float64
	ctFontGetUnderlineThick func(uintptr) float64
	ctFontGetUnderlinePos   func(uintptr) float64
	cgContextSetLineDash    func(ctx uintptr, phase float64, lengths *float64, count uintptr)

	attrRunDelegate uintptr
)

func loadInline(ct, cg uintptr) {
	reg := purego.RegisterLibFunc
	reg(&ctRunDelegateCreate, ct, "CTRunDelegateCreate")
	reg(&ctFontGetXHeight, ct, "CTFontGetXHeight")
	reg(&ctFontGetUnderlineThick, ct, "CTFontGetUnderlineThickness")
	reg(&ctFontGetUnderlinePos, ct, "CTFontGetUnderlinePosition")
	reg(&cgContextSetLineDash, cg, "CGContextSetLineDash")
	attrRunDelegate = constant(ct, "kCTRunDelegateAttributeName")
	delegateCallbacks = runDelegateCallbacks{
		version:    1, // kCTRunDelegateVersion1
		dealloc:    purego.NewCallback(func(ref uintptr) { extents.Delete(ref) }),
		getAscent:  purego.NewCallback(func(ref uintptr) cgFloat { return cgFloat{V: extentOf(ref).above} }),
		getDescent: purego.NewCallback(func(ref uintptr) cgFloat { return cgFloat{V: extentOf(ref).below} }),
		getWidth:   purego.NewCallback(func(ref uintptr) cgFloat { return cgFloat{V: extentOf(ref).width} }),
	}
}

// runDelegateCallbacks is CTRunDelegateCallbacks. It lives in a global, so
// the collector never moves it.
type runDelegateCallbacks struct {
	_                                        structs.HostLayout
	version                                  int
	dealloc, getAscent, getDescent, getWidth uintptr
}

var delegateCallbacks runDelegateCallbacks

// cgFloat returns a CGFloat from a callback: purego returns floats from
// callbacks only inside structs, which the ABI returns in a float
// register.
type cgFloat struct {
	_ structs.HostLayout
	V float64
}

// extents holds each run delegate's metrics by its refCon, until
// CoreText deallocates the delegate.
var (
	extents    sync.Map // uintptr → extent
	nextExtent uintptr
	extentMu   sync.Mutex
)

type extent struct{ above, below, width float64 }

func extentOf(ref uintptr) extent {
	e, _ := extents.Load(ref)
	x, _ := e.(extent)
	return x
}

// runDelegate returns a new CTRunDelegate of e. The caller releases it.
func runDelegate(e extent) uintptr {
	extentMu.Lock()
	nextExtent++
	ref := nextExtent
	extentMu.Unlock()
	extents.Store(ref, e)
	return ctRunDelegateCreate(&delegateCallbacks, ref)
}

// fontExtent returns the metrics an attachment is placed against in st's
// font.
func fontExtent(st driver.TextStyle) driver.FontExtent {
	f := Font(st.Font)
	defer Release(f)
	return driver.FontExtent{
		Ascent:  driver.Points(ctFontGetAscent(f)),
		Descent: driver.Points(ctFontGetDescent(f)),
		XHeight: driver.Points(ctFontGetXHeight(f)),
	}
}

// decoration is a UTF-16 range drawn on: a span's background box, a
// strikethrough, or an underline CoreText cannot draw.
type decoration struct {
	start, end int
	color      driver.Color
	strike     bool
	style      driver.TextDecorationStyle
	radius     float64
	// ascent and descent bound a background; offset is a strike's height
	// above the baseline and thickness its width.
	ascent, descent, offset, thickness float64
}

// decorations lists what t's spans draw besides glyphs, in UTF-16
// offsets.
func decorations(t *driver.AttributedText, toUTF16 []int) []decoration {
	type run struct {
		start, end int
		style      driver.TextStyle
		background driver.TextBackground
	}
	var runs []run
	if len(t.Spans) == 0 {
		runs = append(runs, run{0, len(t.Text), t.Style, driver.TextBackground{}})
	}
	for _, sp := range t.Spans {
		runs = append(runs, run{sp.Start, sp.End, sp.Style, sp.Background})
	}
	var out []decoration
	for _, r := range runs {
		start, end := min(max(r.start, 0), len(t.Text)), min(max(r.end, 0), len(t.Text))
		strike := r.style.Decoration&driver.TextDecorationLineThrough != 0
		under := r.style.Decoration&driver.TextDecorationUnderline != 0 && drawsUnderline(r.style)
		if end <= start || (!strike && !under && r.background.Color == 0) {
			continue
		}
		f := Font(r.style.Font)
		d := decoration{start: toUTF16[start], end: toUTF16[end],
			ascent: ctFontGetAscent(f), descent: ctFontGetDescent(f)}
		if r.background.Color != 0 {
			bg := d
			bg.color, bg.radius = r.background.Color, float64(r.background.Radius)
			out = append(out, bg)
		}
		lineColor := r.style.Color
		if r.style.DecorationColor != 0 {
			lineColor = r.style.DecorationColor
		}
		d.strike, d.color, d.style = true, lineColor, r.style.DecorationStyle
		d.thickness = max(ctFontGetUnderlineThick(f), 1)
		if t := r.style.DecorationThickness; t > 0 {
			d.thickness = float64(t)
		}
		if strike {
			d.offset = ctFontGetXHeight(f) / 2
			out = append(out, d)
		}
		if under {
			// offset is the line's middle above the baseline: the font's
			// underline position, which a wave hangs below, or the app's
			// offset of its top below the baseline.
			switch off, ok := r.style.UnderlineOffset.Points(); {
			case ok:
				d.offset = -float64(off) - d.thickness/2
			case r.style.DecorationStyle == driver.DecorationWavy:
				d.offset = ctFontGetUnderlinePos(f) - d.thickness
			default:
				d.offset = ctFontGetUnderlinePos(f)
			}
			out = append(out, d)
		}
		Release(f)
	}
	return out
}

// drawsUnderline reports whether st's underline is drawn here rather
// than by CoreText, which has no wavy line, thickness or offset.
func drawsUnderline(st driver.TextStyle) bool {
	return st.DecorationStyle == driver.DecorationWavy || st.DecorationThickness > 0 || !st.UnderlineOffset.IsAuto()
}

// box is a decoration placed in a text's box: from its top left.
type box struct {
	rect   CGRect
	color  driver.Color
	radius float64
	style  driver.TextDecorationStyle
}

// place puts l's decorations on its lines, and its attachments.
func (l *Layout) place(decos []decoration, atts []driver.TextAttachment, fonts []driver.FontExtent, toUTF16 []int) {
	lines, baselines, bases := l.placed()
	for _, d := range decos {
		for i, line := range lines {
			r := ctLineGetStringRange(line)
			from, to := r.Location+bases[i], r.Location+r.Length+bases[i]
			a, b := max(d.start, from), min(d.end, to)
			// Hanging spaces and breaks at a line's end are not painted.
			for b > a && l.blankAt(b-1) {
				b--
			}
			if a >= b {
				continue
			}
			x0 := ctLineOffsetForIndex(line, a-bases[i], nil)
			x1 := ctLineOffsetForIndex(line, b-bases[i], nil)
			if x1 < x0 {
				x0, x1 = x1, x0
			}
			o := baselines[i]
			if d.strike {
				l.over = append(l.over, box{rect: Rect(o.X+x0, o.Y-d.offset-d.thickness/2, x1-x0, d.thickness),
					color: d.color, style: d.style})
				continue
			}
			h := d.ascent + d.descent
			l.under = append(l.under, box{rect: Rect(o.X+x0, o.Y-d.ascent, x1-x0, h), color: d.color,
				radius: min(d.radius, h/2)})
		}
	}
	l.attachments = make([]attachmentBox, len(atts))
	for k, at := range atts {
		idx := toUTF16[at.Offset]
		for i, line := range lines {
			r := ctLineGetStringRange(line)
			if idx < r.Location+bases[i] || idx >= r.Location+r.Length+bases[i] {
				continue
			}
			var asc, desc, lead float64
			width := ctLineGetTypographicBounds(line, &asc, &desc, &lead)
			x := ctLineOffsetForIndex(line, idx-bases[i], nil)
			if x+float64(at.Size.Width) > width+0.5 {
				break // behind the ellipsis
			}
			o := baselines[i]
			top := o.Y - float64(at.Ascent(fonts[k]))
			l.attachments[k] = attachmentBox{driver.Bounds{
				Origin: driver.Point{X: driver.Points(o.X + x), Y: driver.Points(top)}, Size: at.Size}, true}
			break
		}
	}
}

type attachmentBox struct {
	box   driver.Bounds
	shown bool
}

// blankAt reports whether UTF-16 offset i is a space or a line break.
func (l *Layout) blankAt(i int) bool {
	if i < 0 || i >= len(l.toUTF8)-1 {
		return false
	}
	c := l.text[l.toUTF8[i]]
	return c == ' ' || c == '\n' || c == '\r' || c == '\t'
}

// Attachment implements core.AttachmentLayout.
func (l *Layout) Attachment(i int) (driver.Bounds, bool) {
	if l == nil || i < 0 || i >= len(l.attachments) {
		return driver.Bounds{}, false
	}
	return l.attachments[i].box, l.attachments[i].shown
}

// DrawBackgrounds fills the backgrounds of l's spans in box of a flipped
// context, behind the selection and the glyphs.
func (l *Layout) DrawBackgrounds(ctx uintptr, at CGRect) {
	if l == nil || ctx == 0 {
		return
	}
	fillBoxes(ctx, l.under, at)
}

// drawLines draws decoration lines, each in its box's style: a solid box
// is the line, and the others are drawn along its middle.
func drawLines(ctx uintptr, lines []box, at CGRect) {
	for _, b := range lines {
		r := b.rect
		r.Origin.X += at.Origin.X
		r.Origin.Y += at.Origin.Y
		t := r.Size.Height
		x0, x1, y := r.Origin.X, r.Origin.X+r.Size.Width, r.Origin.Y+t/2
		color := CGColor(b.color)
		cgContextSaveGState(ctx)
		cgContextSetFillColor(ctx, color)
		cgContextSetStrokeColor(ctx, color)
		cgContextSetLineWidth(ctx, t)
		switch b.style {
		case driver.DecorationDouble:
			cgContextAddRect(ctx, Rect(x0, y-1.5*t, x1-x0, t))
			cgContextAddRect(ctx, Rect(x0, y+0.5*t, x1-x0, t))
			cgContextFillPath(ctx)
		case driver.DecorationDotted, driver.DecorationDashed:
			dash := [2]float64{t, t}
			if b.style == driver.DecorationDashed {
				dash = [2]float64{3 * t, 2 * t}
			}
			cgContextSetLineDash(ctx, 0, &dash[0], 2)
			cgContextMoveToPoint(ctx, x0, y)
			cgContextAddLineToPoint(ctx, x1, y)
			cgContextStrokePath(ctx)
		case driver.DecorationWavy:
			amp, step := max(t, 1), 2*max(t, 1)
			cgContextMoveToPoint(ctx, x0, y)
			up := true
			for x := x0; x < x1; x += step {
				dy := amp
				if up {
					dy = -amp
				}
				cgContextAddQuadCurve(ctx, x+step/2, y+2*dy, min(x+step, x1), y)
				up = !up
			}
			cgContextStrokePath(ctx)
		default:
			cgContextAddRect(ctx, r)
			cgContextFillPath(ctx)
		}
		cgContextRestoreGState(ctx)
		Release(color)
	}
}

func fillBoxes(ctx uintptr, boxes []box, at CGRect) {
	for _, b := range boxes {
		r := b.rect
		r.Origin.X += at.Origin.X
		r.Origin.Y += at.Origin.Y
		color := CGColor(b.color)
		cgContextSaveGState(ctx)
		cgContextSetFillColor(ctx, color)
		if b.radius > 0 {
			path := cgPathCreateWithRoundedRect(r, b.radius, b.radius, 0)
			cgContextAddPath(ctx, path)
			Release(path)
		} else {
			cgContextAddRect(ctx, r)
		}
		cgContextFillPath(ctx)
		cgContextRestoreGState(ctx)
		Release(color)
	}
}
