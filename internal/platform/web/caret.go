//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
)

// The caret and pastes of text fields. The caret is found in a hidden
// copy of the field, with its font, box and wrapping, that holds its text
// up to the caret, as NSH's web runtime finds it.

// caretMoved reports where v's caret is, while its field has the focus.
func (s *surface) caretMoved(v *view) {
	el := v.inner
	if !v.events.Has(driver.EventCaretMove) || !el.Equal(s.doc.Get("activeElement")) || el.Get("selectionEnd").IsNull() {
		return
	}
	cs := js.Global().Call("getComputedStyle", el)
	mirror := s.doc.Call("createElement", "div")
	style := map[string]string{"position": "absolute", "visibility": "hidden", "top": "0", "left": "0",
		"overflow": "hidden", "box-sizing": "border-box", "white-space": "pre", "overflow-wrap": "normal"}
	if el.Get("tagName").String() == "TEXTAREA" {
		style["white-space"], style["overflow-wrap"] = "pre-wrap", "break-word"
	}
	for _, p := range []string{"font-family", "font-size", "font-weight", "font-style", "font-stretch",
		"font-variant", "font-feature-settings", "font-kerning", "letter-spacing", "line-height", "padding-top", "padding-right", "padding-bottom",
		"padding-left", "border-top-width", "border-right-width", "border-bottom-width", "border-left-width",
		"text-indent", "text-transform", "word-spacing", "tab-size", "direction", "text-align"} {
		style[p] = cs.Call("getPropertyValue", p).String()
	}
	style["width"] = cs.Call("getPropertyValue", "width").String()
	setStyle(mirror, style)
	value := el.Get("value").String()
	end := el.Get("selectionEnd").Int()
	if el.Get("selectionDirection").String() == "backward" {
		end = el.Get("selectionStart").Int()
	}
	before := utf16Prefix(value, end)
	if el.Get("type").String() == "password" {
		before = strings.Repeat("•", len([]rune(before)))
	}
	mirror.Set("textContent", before)
	marker := s.doc.Call("createElement", "span")
	marker.Set("textContent", "​")
	mirror.Call("appendChild", marker)
	s.host.Call("appendChild", mirror)
	x := marker.Get("offsetLeft").Float() - el.Get("scrollLeft").Float()
	y := marker.Get("offsetTop").Float() - el.Get("scrollTop").Float()
	h := marker.Get("offsetHeight").Float()
	mirror.Call("remove")
	// In the input's view: the field sits at its content box.
	fr, vr := el.Call("getBoundingClientRect"), v.el.Call("getBoundingClientRect")
	x += fr.Get("left").Float() - vr.Get("left").Float()
	y += fr.Get("top").Float() - vr.Get("top").Float()
	s.send(v, driver.Event{Kind: driver.EventCaretMove, Point: driver.Point{X: driver.Points(x), Y: driver.Points(y)},
		Size: driver.Size{Width: 2, Height: driver.Points(h)}})
}

// utf16Prefix returns s up to UTF-16 offset n.
func utf16Prefix(s string, n int) string {
	units := 0
	for i, r := range s {
		if units >= n {
			return s[:i]
		}
		units += len(string(r))/4 + 1
	}
	return s
}

// pasted reports what the user pastes into v's field, and keeps its text
// out when the field only reports pastes. Files' bytes are read first, so
// a paste with files comes after the text it inserts.
func (s *surface) pasted(v *view, e js.Value) {
	if !v.events.Has(driver.EventPaste) {
		return
	}
	data := e.Get("clipboardData")
	if !data.Truthy() {
		return
	}
	p := &driver.PasteEvent{Text: data.Call("getData", "text/plain").String(), HTML: data.Call("getData", "text/html").String()}
	if c := v.input.config; c != nil && c.PasteBehavior == driver.PasteReport {
		e.Call("preventDefault")
	}
	files := data.Get("files")
	n := files.Get("length").Int()
	if n == 0 {
		s.send(v, driver.Event{Kind: driver.EventPaste, Data: p})
		return
	}
	p.Files = make([]driver.PastedFile, n)
	left := n
	for i := range n {
		f := files.Index(i)
		p.Files[i] = driver.PastedFile{Name: f.Get("name").String(), MIME: f.Get("type").String()}
		var then js.Func
		then = js.FuncOf(func(_ js.Value, args []js.Value) any {
			then.Release()
			b := js.Global().Get("Uint8Array").New(args[0])
			p.Files[i].Data = make([]byte, b.Get("length").Int())
			js.CopyBytesToGo(p.Files[i].Data, b)
			if left--; left == 0 {
				s.send(v, driver.Event{Kind: driver.EventPaste, Data: p})
			}
			return nil
		})
		f.Call("arrayBuffer").Call("then", then)
	}
}
