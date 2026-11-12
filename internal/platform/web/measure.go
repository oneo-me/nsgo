//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/webstyle"
)

// A text of one style on one line is measured with a canvas's
// measureText, as NSH measures its runs: laying the measuring element out
// for each new text forces the page's layout, which held a table of
// ticking quotes to a few frames a second. Its height is its style's
// line, measured once in the element. Text the canvas cannot measure as
// the element draws it (spans, attachments, line breaks, letter spacing,
// numeric and caps variants, characters a fallback font may draw), or
// that wraps, is laid out in the element as before.

// widthsKept is the most widths kept before they are forgotten, and the
// most measurements between dropping the canvas: Blink keeps every
// string a context shaped, so a table measuring new numbers every frame
// would pile them up (NSH's MEASURE_ROUNDS).
const widthsKept = 5000

// lineFonts are what a text of one style needs to be measured on a
// canvas: its canvas font and the key of its line's height.
type lineFont struct {
	font, line string
}

// canvasFont is the style's font as a canvas takes it, and false when the
// canvas would not measure the text as the element draws it.
func canvasFont(t driver.TextStyle) (lineFont, bool) {
	if t.LetterSpacing != 0 || t.Font.Variant != 0 {
		return lineFont{}, false
	}
	style := "normal"
	if t.Font.Italic {
		style = "italic"
	}
	font := style + " " + strconv.Itoa(int(t.Font.Weight)) + " " + webstyle.Px(t.Font.Size) + " " + cssFamily(t.Font.Family)
	line := font + "/normal"
	if t.LineHeight > 0 {
		line = font + "/" + webstyle.Px(t.LineHeight)
	}
	return lineFont{font: font, line: line}, true
}

// oneLine reports whether text is one line a canvas measures as the
// element draws it: no breaks or tabs, and characters the fonts the app
// names cover (Latin), so no fallback font changes the line's height.
func oneLine(text string) bool {
	for _, c := range text {
		if c < 0x20 || c == 0x7f || c > 0x24f && c != '−' && c != '€' && c != '…' && c != '–' && c != '—' && c != '·' {
			return false
		}
	}
	return true
}

// textWidths measures and remembers texts' widths on a canvas.
type textWidths struct {
	ctx   js.Value
	font  string // the context's
	uses  int
	sizes map[[2]string]driver.Points // by font and text
	lines map[string]driver.Points    // a line's height, by font and line height
}

// forget drops what was measured: a font loaded, and what was measured
// with a fallback is stale.
func (w *textWidths) forget() {
	w.sizes, w.lines = nil, nil
}

// width is text's width in font, rounded up to the 1/64 point the page
// lays out in, so the element drawing it has room for it.
func (w *textWidths) width(d *dom, font, text string) driver.Points {
	if text == "" {
		return 0
	}
	key := [2]string{font, text}
	if v, ok := w.sizes[key]; ok {
		return v
	}
	if v, ok := d.width(font, text); ok {
		return w.keep(key, driver.Points(math.Ceil(v*64)/64))
	}
	if w.uses++; w.ctx.IsUndefined() || w.uses >= widthsKept {
		w.uses, w.font = 0, ""
		if c := js.Global().Get("OffscreenCanvas"); c.Truthy() {
			w.ctx = c.New(1, 1).Call("getContext", "2d")
		} else {
			w.ctx = js.Global().Get("document").Call("createElement", "canvas").Call("getContext", "2d")
		}
	}
	if w.font != font {
		w.font = font
		w.ctx.Set("font", font)
	}
	return w.keep(key, driver.Points(math.Ceil(w.ctx.Call("measureText", text).Get("width").Float()*64)/64))
}

func (w *textWidths) keep(key [2]string, v driver.Points) driver.Points {
	if w.sizes == nil || len(w.sizes) >= widthsKept {
		w.sizes = map[[2]string]driver.Points{}
	}
	w.sizes[key] = v
	return v
}

// quick is p's size on one line, measured on a canvas, and false when
// the canvas cannot measure it.
func (p *preparedText) quick() (driver.Size, bool) {
	if p.line.font == "" {
		return driver.Size{}, false
	}
	w := &p.s.widths
	h, ok := w.lines[p.line.line]
	if !ok {
		// The element measures the line's height once per style.
		m := p.hold()
		p.s.measureCSS.set1("max-width", "none")
		h = driver.Points(m.Call("getBoundingClientRect").Get("height").Float())
		if w.lines == nil {
			w.lines = map[string]driver.Points{}
		}
		w.lines[p.line.line] = h
	}
	return driver.Size{Width: w.width(&p.s.dom, p.line.font, p.text), Height: h}, true
}
