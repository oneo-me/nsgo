//go:build js && wasm

package harmonyos

import (
	"encoding/json"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// textEngine measures text with the platform's measurer, the one ArkUI lays
// its own text out with, so a measured line is the line the render layer
// draws (docs/architecture/harmonyos.md measures them identical).
type textEngine struct {
	bridge js.Value
	// cache keeps what the host measured: a round trip costs milliseconds,
	// and a commit that lays out a list again measures the same rows.
	cache map[measureKey]driver.Size
}

// measureKey is what tells one measurement from another: the text, the
// font fields the host measures with, and the width it wraps at.
type measureKey struct {
	text   string
	size   driver.Points
	weight driver.FontWeight
	italic bool
	family string
	width  driver.Points
}

// prepare shapes text once, as the engine asks; measuring is the host's.
func (e *textEngine) prepare(t *driver.AttributedText) driver.PreparedText {
	return &prepared{engine: e, text: t}
}

type prepared struct {
	engine *textEngine
	text   *driver.AttributedText
}

// Measure returns the text's size wrapped at maxWidth, which Auto leaves
// unwrapped.
func (p *prepared) Measure(maxWidth driver.Points) driver.Size {
	return p.engine.measure(p.text.Text, p.text.Style, maxWidth)
}

// Layout returns the text as it will be drawn: the render layer lays it out
// with ArkUI, at the frame the engine measured for it.
func (p *prepared) Layout(size driver.Size) driver.TextLayout { return &layout{size: size} }

// Release frees nothing: the text is the engine's.
func (p *prepared) Release() {}

// layout is text left to the render layer to draw.
type layout struct{ size driver.Size }

func (l *layout) Release() {}

// measure asks the host for the size of text in style, wrapped at maxWidth.
func (e *textEngine) measure(text string, style driver.TextStyle, maxWidth driver.Points) driver.Size {
	f := style.Font
	if f.Size == 0 {
		f.Size = 16
	}
	w := float64(0)
	if maxWidth > 0 {
		w = float64(maxWidth)
	}
	key := measureKey{text: text, size: f.Size, weight: f.Weight, italic: f.Italic, family: f.Family, width: driver.Points(w)}
	if size, ok := e.cache[key]; ok {
		return size
	}
	out := e.bridge.Call("measure", text, float64(f.Size), float64(f.Weight), f.Italic, f.Family, w).String()
	var size [2]float64
	if err := json.Unmarshal([]byte(out), &size); err != nil {
		return driver.Size{Width: maxWidth, Height: f.Size * 1.2}
	}
	size2 := driver.Size{Width: driver.Points(size[0]), Height: driver.Points(size[1])}
	if maxWidth > 0 && size2.Width > maxWidth {
		size2.Width = maxWidth
	}
	if e.cache == nil || len(e.cache) >= 4096 {
		e.cache = make(map[measureKey]driver.Size, 256)
	}
	e.cache[key] = size2
	return size2
}
