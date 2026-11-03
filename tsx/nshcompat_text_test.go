package tsx_test

import (
	"math"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
)

// nshTextEngine measures text as NSH's headless runs do (nsh-core's
// text.rs ApproxTextEngine), which the frames of the compat scripts were
// recorded with: every character, spaces too, 0.55 of the first span's
// font size wide, lines 1.2 of it tall, as many as the width needs
// whatever the words, never wrapping at a space. It lays out as
// core.ApproxTextEngine does, which only carets and spans read.
type nshTextEngine struct{}

func (nshTextEngine) Prepare(t *driver.AttributedText) driver.PreparedText {
	return nshText{t: t, approx: core.ApproxTextEngine{}.Prepare(t)}
}

type nshText struct {
	t      *driver.AttributedText
	approx driver.PreparedText
}

func (p nshText) Measure(maxWidth driver.Points) driver.Size {
	font := p.font()
	// The order of the products is NSH's: it decides the last bit.
	width := float32(utf8.RuneCountInString(p.t.Text)) * font * 0.55
	line := font * 1.2
	lines := float32(1)
	if !maxWidth.IsAuto() && width > float32(maxWidth) && maxWidth > 0 {
		width, lines = float32(maxWidth), float32(math.Ceil(float64(width/float32(maxWidth))))
	}
	if p.t.MaxLines > 0 {
		lines = min(lines, float32(p.t.MaxLines))
	}
	return driver.Size{Width: driver.Points(math.Ceil(float64(width))), Height: driver.Points(math.Ceil(float64(line * lines)))}
}

func (p nshText) Layout(size driver.Size) driver.TextLayout {
	return nshLayout{ApproxLayout: p.approx.Layout(size).(*core.ApproxLayout), t: p.t, font: p.font()}
}

func (p nshText) font() float32 {
	font := float32(p.t.Style.Font.Size)
	if len(p.t.Spans) > 0 && p.t.Spans[0].Start == 0 {
		font = float32(p.t.Spans[0].Style.Font.Size)
	}
	if font == 0 {
		font = 14
	}
	return font
}

// nshLayout places a text's attachments as NSH's headless engine does,
// on one line: after as many characters, each 0.55 of the font size wide,
// their tops their ascent above the line's bottom, the baseline.
type nshLayout struct {
	*core.ApproxLayout
	t    *driver.AttributedText
	font float32
}

func (l nshLayout) Attachment(i int) (driver.Bounds, bool) {
	if i < 0 || i >= len(l.t.Attachments) {
		return driver.Bounds{}, false
	}
	a := l.t.Attachments[i]
	index := utf8.RuneCountInString(l.t.Text[:min(a.Offset, len(l.t.Text))])
	line := l.font * 1.2
	ascent := a.Ascent(driver.FontExtent{Ascent: driver.Points(line), XHeight: driver.Points(l.font * 0.5)})
	return driver.Bounds{Origin: driver.Point{X: driver.Points(float32(index) * l.font * 0.55),
		Y: max(driver.Points(line)-ascent, 0)}, Size: a.Size}, true
}

func (p nshText) Release() { p.approx.Release() }
