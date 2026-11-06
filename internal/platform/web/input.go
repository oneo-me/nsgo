//go:build js && wasm

package web

import (
	"strconv"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/field"
)

// Text fields follow NSH's web backend: an <input>, or a <textarea> when
// multiline, in the view's element, which draws the background and
// border. The browser keeps maxLength itself; Return submits unless it
// breaks the line (SubmitBehavior), and Shift or Alt with Return always
// break a textarea's. An input method's composition is followed with the
// composition events, and a value the app sets during one waits for its
// end, since setting the value would end the composition and commit the
// text as typed.

// fieldCSS colors placeholders that have a color.
const fieldCSS = `.nsgo-placeholder::placeholder { color: var(--nsgo-placeholder); opacity: 1; }`

var inputModes = map[driver.KeyboardType]string{
	driver.KeyboardEmail:   "email",
	driver.KeyboardNumber:  "numeric",
	driver.KeyboardDecimal: "decimal",
	driver.KeyboardPhone:   "tel",
	driver.KeyboardURL:     "url",
}

var enterHints = map[driver.ReturnKeyType]string{
	driver.ReturnKeyDone: "done", driver.ReturnKeyGo: "go", driver.ReturnKeyNext: "next",
	driver.ReturnKeySearch: "search", driver.ReturnKeySend: "send",
}

var capitalize = map[driver.AutoCapitalize]string{
	driver.AutoCapitalizeSentences: "sentences", driver.AutoCapitalizeNone: "off",
	driver.AutoCapitalizeWords: "words", driver.AutoCapitalizeCharacters: "characters",
}

// webInput is a text field's element and what it tracks.
type webInput struct {
	fs        field.State
	listeners listeners
	props     *driver.ViewProps // the last, which a new element takes
	config    *driver.InputConfig
	look      classSlot // the field's class of its text style (classes.go)
}

// createInput puts an <input> in v's element.
func (s *surface) createInput(v *view) {
	v.input = &webInput{fs: field.State{ID: v.id, Dispatch: s.engine.Dispatch}}
	s.setFieldElement(v, "input")
}

// setFieldElement makes v's field an element of tag, keeping its text.
func (s *surface) setFieldElement(v *view, tag string) {
	in := v.input
	value := ""
	if !v.inner.IsUndefined() && !v.inner.IsNull() {
		value = v.inner.Get("value").String()
		in.listeners.release()
		v.inner.Call("remove")
	}
	el := s.element(tag)
	el.Get("classList").Call("add", "nsgo-field")
	if tag == "input" {
		el.Set("type", "text")
	}
	s.freeClass(&in.look)
	in.look = classSlot{} // the new element has none
	el.Set("value", value)
	v.inner = el
	v.el.Call("appendChild", el)
	if in.props != nil {
		setViewProps(v, in.props)
	}
	on := func(name string, fn func(e js.Value)) { in.listeners.on(el, name, fn) }
	on("input", func(js.Value) {
		in.fs.Edited(el.Get("value").String())
		s.fieldSelection(v)
	})
	on("compositionstart", func(e js.Value) { s.compose(v, true, eventData(e)) })
	on("compositionupdate", func(e js.Value) { s.compose(v, true, eventData(e)) })
	on("compositionend", func(js.Value) {
		// The committed text goes first; browsers differ in whether its
		// input event comes before or after this.
		in.fs.Edited(el.Get("value").String())
		s.compose(v, false, "")
	})
	on("keydown", func(e js.Value) {
		key := e.Get("key").String()
		composing := e.Get("isComposing").Bool() || e.Get("keyCode").Int() == 229
		switch {
		case key == "Enter":
			if composing {
				return
			}
			lineBreak := tag == "textarea" && (e.Get("shiftKey").Bool() || e.Get("altKey").Bool())
			insert, blur := in.fs.Return(el.Get("value").String(), lineBreak)
			if insert {
				in.fs.Key("Enter")
				return
			}
			e.Call("preventDefault")
			if blur {
				el.Call("blur")
			}
		case key == "Backspace" || key == "Escape" || len([]rune(key)) == 1 && !composing:
			in.fs.Key(key)
		}
	})
	for _, name := range []string{"select", "keyup", "pointerup"} {
		on(name, func(js.Value) { s.fieldSelection(v) })
	}
	for _, name := range []string{"select", "keyup", "pointerup", "input", "scroll", "focus"} {
		on(name, func(js.Value) { s.caretMoved(v) })
	}
	on("paste", func(e js.Value) { s.pasted(v, e) })
	on("focus", func(js.Value) { s.send(v, driver.Event{Kind: driver.EventFocus}) })
	on("blur", func(js.Value) {
		s.compose(v, false, "")
		s.send(v, driver.Event{Kind: driver.EventBlur})
	})
}

func eventData(e js.Value) string {
	if d := e.Get("data"); d.Type() == js.TypeString {
		return d.String()
	}
	return ""
}

func (s *surface) compose(v *view, composing bool, text string) {
	el := v.inner
	if value, apply := v.input.fs.Compose(composing, text, el.Get("value").String()); apply && el.Get("value").String() != value {
		el.Set("value", value)
	}
}

// fieldSelection reports the selection the user left.
func (s *surface) fieldSelection(v *view) {
	el := v.inner
	if !v.input.fs.Events.Has(driver.EventSelectionChange) || el.Get("selectionStart").IsNull() {
		return
	}
	v.input.fs.Selected(driver.TextRange{Start: el.Get("selectionStart").Int(), End: el.Get("selectionEnd").Int()})
}

// inputValue is ItemInputValue.
func (s *surface) inputValue(v *view, text string, count uint32) {
	// Setting the text the field shows would move the caret.
	if v.input.fs.Value(text, count) && v.inner.Get("value").String() != text {
		v.inner.Set("value", text)
	}
}

// selectInput is ItemSelection.
func (s *surface) selectInput(v *view, r driver.TextRange) {
	n := driver.UTF16Len(v.inner.Get("value").String())
	start := min(max(r.Start, 0), n)
	end := min(max(r.End, start), n)
	v.input.fs.SetSelection(driver.TextRange{Start: start, End: end})
	v.inner.Call("setSelectionRange", start, end)
}

func (s *surface) setInput(v *view, c *driver.InputConfig) {
	if v.kind != driver.KindInput {
		return
	}
	v.input.config = c
	in := v.input
	tag := "input"
	if c.Multiline {
		tag = "textarea"
	}
	if v.inner.Get("localName").String() != tag {
		s.setFieldElement(v, tag)
	}
	in.fs.Config = *c
	el := v.inner
	setStyle(el, frameStyle(c.Content))
	if !c.Multiline {
		kind := "text"
		if c.Masked {
			kind = "password"
		}
		if el.Get("type").String() != kind {
			el.Set("type", kind)
		}
	}
	el.Set("placeholder", c.Placeholder)
	el.Set("readOnly", c.Readonly)
	el.Set("disabled", c.Disabled)
	setAttr(el, "inputmode", inputModes[c.KeyboardType])
	// A textarea that submits shows the return key's action too.
	hint := ""
	if c.Submit() != driver.SubmitBehaviorNewline {
		hint = enterHints[c.ReturnKey]
		if hint == "" {
			hint = "enter"
		}
	}
	setAttr(el, "enterkeyhint", hint)
	setAttr(el, "autocapitalize", capitalize[c.AutoCapitalize])
	el.Set("spellcheck", !c.NoAutoCorrect)
	autocorrect := ""
	if c.NoAutoCorrect {
		autocorrect = "off"
	}
	setAttr(el, "autocorrect", autocorrect)
	setAttr(el, "autocomplete", c.AutoComplete.String())
	maxLength := ""
	if c.MaxLength > 0 {
		maxLength = strconv.Itoa(c.MaxLength)
	}
	setAttr(el, "maxlength", maxLength)
	css := fontStyle(c.Style)
	switch {
	case c.HideCaret:
		css["caret-color"] = "transparent"
	case c.Caret() != 0:
		css["caret-color"] = cssColor(c.Caret())
	}
	classes := el.Get("classList")
	if c.SelectionColor != 0 {
		s.selectionStyle()
		classes.Call("add", "nsgo-selection")
		css["--nsgo-selection"] = cssColor(c.SelectionColor.Faded(0.3))
	} else {
		classes.Call("remove", "nsgo-selection")
	}
	if c.PlaceholderColor != 0 {
		s.fieldStyle()
		classes.Call("add", "nsgo-placeholder")
		css["--nsgo-placeholder"] = cssColor(c.PlaceholderColor)
	} else {
		classes.Call("remove", "nsgo-placeholder")
	}
	s.setClass(el, &in.look, s.sortedCSS(css))
}

// fieldStyle adds fieldCSS to the page once.
func (s *surface) fieldStyle() {
	if s.fieldCSS {
		return
	}
	s.fieldCSS = true
	el := s.doc.Call("createElement", "style")
	el.Set("textContent", fieldCSS)
	s.doc.Get("head").Call("appendChild", el)
}
