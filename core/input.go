package core

import (
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/layout"
)

// Composition is an input method's composition in a text field.
type Composition struct {
	// Composing is false once the composition ended.
	Composing bool
	// Text is what is being composed; empty once it ended.
	Text string
}

const (
	KeyboardAppearanceDefault KeyboardAppearance = iota
	KeyboardAppearanceLight
	KeyboardAppearanceDark
)

// SetSelection selects start to end, in UTF-16 units, in the element, a
// text field; equal offsets place the caret.
func (r *Ref) SetSelection(start, end int) {
	if r.n == nil || r.n.deleted || r.n.input == nil {
		return
	}
	r.t.commands = append(r.t.commands, driver.MountItem{Kind: driver.ItemSelection, ID: r.n.id, Selection: TextRange{Start: start, End: end}})
	r.t.changed = true
}

// defaultInputWidth is a field's width when its style sets none.
const defaultInputWidth Points = 160

func (p *preparedInput) set(e driver.TextEngine, at driver.AttributedText) bool {
	if p.prepared != nil && at.Equal(&p.text) {
		return false
	}
	p.release()
	p.text = at
	p.prepared = e.Prepare(&p.text)
	return true
}

func (p *preparedInput) release() {
	if p.prepared != nil {
		p.prepared.Release()
		p.prepared = nil
	}
}

// inputText is s as field c lays it out. A multiline field's trailing
// line break still takes a line.
func inputText(s string, c *driver.InputConfig) driver.AttributedText {
	at := driver.AttributedText{Text: s, Style: c.Style, MaxLines: 1}
	if c.Multiline {
		at.MaxLines = 0
		if strings.HasSuffix(s, "\n") {
			at.Text += "​"
		}
	}
	return at
}

// prepareInput prepares what measures field n: a multiline field's value
// or placeholder, and the value whose size OnContentSizeChange reports.
func (t *Tree) prepareInput(n *node) {
	in := n.input
	c := &in.config
	if c.Multiline {
		s := in.value
		if s == "" {
			s = c.Placeholder
		}
		if in.measured.set(t.text, inputText(s, c)) {
			t.markLayout(n)
		}
	} else if in.measured.prepared != nil {
		in.measured.release()
		t.markLayout(n)
	}
	if in.contentSize != nil && in.value != "" {
		in.sized.set(t.text, inputText(in.value, c))
	} else {
		in.sized.release()
	}
}

// measure is a field's content size: a line at the default width, or a
// multiline field's text wrapped at its width.
func (in *inputNode) measure(known layout.Size[layout.Opt], available layout.Size[layout.AvailableSpace]) Size {
	m := Size{Width: defaultInputWidth, Height: ceil(in.lineH, in.ratio)}
	if in.measured.prepared == nil {
		return m
	}
	width := defaultInputWidth
	if available.Width.Kind == layout.SpaceDefinite {
		width = Points(available.Width.Value)
	} else if known.Width.Valid {
		width = Points(known.Width.Value)
	}
	m.Height = max(m.Height, ceil(in.measured.prepared.Measure(max(width, 0)).Height, in.ratio))
	return m
}

// reportContentSize queues OnContentSizeChange when the size of field n's
// value changed: wrapped at its content width when multiline, one empty
// line's when it is empty.
func (t *Tree) reportContentSize(n *node) {
	in := n.input
	if in.contentSize == nil {
		return
	}
	size := Size{Width: 0, Height: ceil(in.lineH, t.ratio)}
	if in.sized.prepared != nil {
		width := Auto
		if in.config.Multiline {
			width = in.config.Content.Size.Width
		}
		m := in.sized.prepared.Measure(width)
		size = Size{Width: ceil(m.Width, t.ratio), Height: ceil(m.Height, t.ratio)}
	}
	if in.reportedOK && size == in.reported {
		return
	}
	in.reported, in.reportedOK = size, true
	fn := in.contentSize
	t.after(n, func() { fn(size) })
}

// The caret and pastes of text inputs, as NSH's onCaretMove and onPaste
// report them.

// CaretEvent is where an input's caret is: Bounds in the input's view,
// as wide as the platform draws the caret and as tall as its line, and
// PageBounds the same in the surface, which an Overlay's AnchorBounds
// takes. Empty Bounds mean the platform could not place it yet.
type CaretEvent struct {
	Bounds     Bounds
	PageBounds Bounds
}
