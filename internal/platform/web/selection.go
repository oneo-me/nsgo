//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Text selection is the browser's own. A text with SelectableOn is
// user-select text and one with SelectableOff none; SelectableAuto
// keeps the page's default, which selects text outside pressable views.
// A selection area is user-select text throughout, so the browser's
// selection, handles and long presses included, runs across its texts.
// Cmd or Ctrl+A selects the area or text the selection is in, Escape
// clears it, and ::selection takes each text's SelectionColor.

// selectionCSS styles the highlight of texts with a selection color.
const selectionCSS = `.nsgo-selection::selection, .nsgo-selection *::selection {
	background: var(--nsgo-selection);
}`

// setSelection applies a view's selection props.
func (s *surface) setSelection(v *view, p *driver.ViewProps) {
	if v.kind == driver.KindText {
		select_ := map[driver.Selectable]string{driver.SelectableOn: "text", driver.SelectableOff: "none"}[p.Selectable]
		v.textCSS.set1("user-select", select_)
		v.textCSS.set1("-webkit-user-select", select_)
		v.selectable = p.Selectable
		if p.SelectionColor != v.selColor {
			v.selColor = p.SelectionColor
			s.selectionStyle()
			if p.SelectionColor == 0 {
				v.inner.Get("classList").Call("remove", "nsgo-selection")
			} else {
				v.inner.Get("classList").Call("add", "nsgo-selection")
				v.textCSS.set1("--nsgo-selection", cssColor(p.SelectionColor))
			}
		}
	}
	if p.SelectionArea != v.area {
		v.area = p.SelectionArea
		select_ := ""
		if v.area {
			select_ = "text"
		}
		v.css().set1("user-select", select_)
		v.css().set1("-webkit-user-select", select_)
	}
}

// selectionStyle adds selectionCSS to the page once.
func (s *surface) selectionStyle() {
	if s.selectionCSS {
		return
	}
	s.selectionCSS = true
	el := s.doc.Call("createElement", "style")
	el.Set("textContent", selectionCSS)
	s.doc.Get("head").Call("appendChild", el)
}

// selectKey selects the whole area or text the selection is in on Cmd or
// Ctrl+A, rather than the page, and clears the selection on Escape. It
// reports whether it took the key.
func (s *surface) selectKey(e js.Value) bool {
	if e.Get("isComposing").Bool() || editing(s.doc.Get("activeElement")) {
		return false
	}
	sel := js.Global().Call("getSelection")
	if !sel.Truthy() || sel.Get("rangeCount").Int() == 0 {
		return false
	}
	node := sel.Get("anchorNode")
	if !node.Truthy() || !s.host.Call("contains", node).Bool() {
		return false
	}
	key := e.Get("key").String()
	switch {
	case key == "Escape" && !sel.Get("isCollapsed").Bool():
		sel.Call("removeAllRanges")
	case (key == "a" || key == "A") && (e.Get("metaKey").Bool() || e.Get("ctrlKey").Bool()) && !e.Get("altKey").Bool():
		scope := s.selectionScope(node)
		if !scope.Truthy() {
			return false
		}
		sel.Call("selectAllChildren", scope)
	default:
		return false
	}
	e.Call("preventDefault")
	return true
}

// selectionScope returns the element Select All selects for a selection
// in node: the outermost area around it, or its selectable text.
func (s *surface) selectionScope(node js.Value) js.Value {
	scope := js.Null()
	el := node
	if el.Get("nodeType").Int() != 1 {
		el = el.Get("parentElement")
	}
	for ; el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		v := s.viewOf(el)
		switch {
		case v == nil:
		case v.area:
			scope = v.el
		case v.kind == driver.KindText && v.selectable == driver.SelectableOn && !scope.Truthy():
			scope = v.inner
		}
	}
	return scope
}

// copySelection writes a selection across texts as core.SelectionModel
// copies one: each text's selected part, attachments left out, texts on
// one row joined by a tab and the others by a line break. The browser
// would run the absolutely placed texts together.
func (s *surface) copySelection(e js.Value) {
	sel := js.Global().Call("getSelection")
	if !sel.Truthy() || sel.Get("rangeCount").Int() == 0 || sel.Get("isCollapsed").Bool() {
		return
	}
	r := sel.Call("getRangeAt", 0)
	if !s.host.Call("contains", r.Get("commonAncestorContainer")).Bool() {
		return
	}
	var parts []string
	var rows []js.Value
	texts := s.host.Call("querySelectorAll", ".nsgo-text")
	for i := range texts.Length() {
		el := texts.Index(i)
		if !r.Call("intersectsNode", el).Bool() || !s.userSelects(el) || el.Call("closest", ".nsgo-attachment").Truthy() {
			continue
		}
		part := s.doc.Call("createRange")
		part.Call("selectNodeContents", el)
		if r.Call("compareBoundaryPoints", 0, part).Int() > 0 { // START_TO_START
			part.Call("setStart", r.Get("startContainer"), r.Get("startOffset"))
		}
		if r.Call("compareBoundaryPoints", 2, part).Int() < 0 { // END_TO_END
			part.Call("setEnd", r.Get("endContainer"), r.Get("endOffset"))
		}
		text := strings.ReplaceAll(part.Call("toString").String(), "￼", "")
		if text == "" {
			continue
		}
		parts = append(parts, text)
		rows = append(rows, el.Call("getBoundingClientRect"))
	}
	if len(parts) < 2 {
		return // the browser's copy is right
	}
	var out strings.Builder
	for i, p := range parts {
		if i > 0 {
			a, b := rows[i-1], rows[i]
			if a.Get("top").Float() < b.Get("bottom").Float() && b.Get("top").Float() < a.Get("bottom").Float() {
				out.WriteByte('\t')
			} else {
				out.WriteByte('\n')
			}
		}
		out.WriteString(p)
	}
	e.Get("clipboardData").Call("setData", "text/plain", out.String())
	e.Call("preventDefault")
}

// userSelects reports whether the browser selects text in el: the
// nearest element setting user-select decides.
func (s *surface) userSelects(el js.Value) bool {
	for ; el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		switch el.Get("style").Call("getPropertyValue", "user-select").String() {
		case "none":
			return false
		case "text":
			return true
		}
	}
	return true
}
