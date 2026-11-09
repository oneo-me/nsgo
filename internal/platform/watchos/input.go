//go:build ios

package watchos

import (
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/field"
)

// A text field on a watch is edited in the system's text input, which a
// TextField the host puts over the view opens when tapped: Go draws the
// field's text or placeholder in its style, and the host reports what the
// user entered.

func (s *surface) configureInput(v *view, c *driver.InputConfig) {
	if v.field == nil {
		return
	}
	v.field.Config = *c
	s.relayoutField(v)
	s.sendInput(v)
}

// sendInput sends the field's placeholder, kind and text to the host.
func (s *surface) sendInput(v *view) {
	c := &v.field.Config
	s.w.op(opInput, v.id)
	s.w.str(c.Placeholder)
	s.w.flag(c.Masked)
	s.w.flag(c.Disabled || c.Readonly)
	s.w.str(v.value)
}

// relayoutField lays out what the field shows in its content box: its
// text, bullets for a masked one, or its placeholder.
func (s *surface) relayoutField(v *view) {
	if v.field == nil {
		return
	}
	c := &v.field.Config
	text, st := v.value, c.Style
	if c.Masked {
		text = strings.Repeat("•", utf8.RuneCountInString(text))
	}
	if text == "" {
		text = c.Placeholder
		if c.PlaceholderColor != 0 {
			st.Color = c.PlaceholderColor
		} else {
			st.Color = st.Color.Faded(0.5)
		}
	}
	v.shown.Release()
	v.shown = nil
	if text != "" {
		at := &driver.AttributedText{Text: text, Style: st}
		if !c.Multiline {
			at.MaxLines, at.Ellipsize = 1, driver.EllipsizeTail
		}
		p := apple.TextEngine{}.Prepare(at)
		v.shown, _ = p.Layout(c.Content.Size).(*apple.Layout)
		p.Release()
	}
	s.touched(v)
	s.redraw[v] = true
}

// drawField draws the field's text in its content box.
func (s *surface) drawField(v *view, ctx uintptr, baseUp bool) {
	if v.shown == nil {
		return
	}
	box := v.field.Config.Content
	v.shown.Draw(ctx, float64(v.shownFrame().Size.Height), cgRect(box), baseUp)
}

// Host.swift's text events.
const (
	textChanged uint8 = iota
	textSubmitted
	textFocused
	textBlurred
)

func hostText(id uint32, kind uint8, data *byte, n int) {
	v := s.view(driver.NodeID(id))
	if v == nil || v.field == nil {
		return
	}
	text := ""
	if n > 0 {
		text = string(unsafe.Slice(data, n))
	}
	c := &v.field.Config
	if !c.Multiline {
		text = field.SingleLine(text)
	}
	switch kind {
	case textChanged:
		s.enter(v, text)
	case textSubmitted:
		s.enter(v, text)
		v.report(driver.Event{Kind: driver.EventSubmitEditing, Text: v.value})
	case textFocused:
		s.focusField(v)
	case textBlurred:
		s.blurField(v)
	}
	s.flush()
}

// enter takes text the user left in field v, within its length.
func (s *surface) enter(v *view, text string) {
	if max := v.field.Config.MaxLength; max > 0 && utf16Len(text) > max {
		return
	}
	if text != v.value {
		v.value = text
		v.field.Edited(text)
		s.relayoutField(v)
	}
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r >= 0x10000 {
			n++
		}
	}
	return n
}

// focusField reports that field v took the focus, and the one that had it
// lost it.
func (s *surface) focusField(v *view) {
	if s.focused == v {
		return
	}
	if s.focused != nil {
		s.blurField(s.focused)
	}
	s.focused = v
	v.report(driver.Event{Kind: driver.EventFocus})
}

func (s *surface) blurField(v *view) {
	if s.focused == v {
		s.focused = nil
	}
	v.report(driver.Event{Kind: driver.EventBlur})
}

func cgRect(b driver.Bounds) apple.CGRect {
	return apple.Rect(float64(b.Origin.X), float64(b.Origin.Y), float64(b.Size.Width), float64(b.Size.Height))
}

func cgPoint(p driver.Point) apple.CGPoint { return apple.CGPoint{X: float64(p.X), Y: float64(p.Y)} }

func onMain(fn func()) { apple.OnMain(fn) }
