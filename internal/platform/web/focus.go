//go:build js && wasm

package web

import (
	"slices"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Keyboard focus for pressable views, as NSH's web runtime keeps it:
// they and Focusable views are Tab stops unless disabled, hidden or NoFocus, their focus
// state follows the keyboard focus (:focus-visible), and a click never
// moves the focus. The views under the newest modal are inert, and the
// focus one took comes back when it closes.

// covered is what a modal made inert, and what had the focus before it.
type covered struct {
	modal *view
	inert []js.Value
	focus js.Value
}

func (s *surface) listenFocus(ls *listeners) {
	ls.on(s.host, "focusin", func(e js.Value) {
		if v := s.viewOf(e.Get("target")); v != nil && v.kind != driver.KindInput &&
			e.Get("target").Call("matches", ":focus-visible").Bool() {
			v.keyFocused = true
			s.send(v, driver.Event{Kind: driver.EventFocus})
		}
	})
	ls.on(s.host, "focusout", func(e js.Value) {
		if v := s.viewOf(e.Get("target")); v != nil && v.keyFocused {
			v.keyFocused = false
			s.send(v, driver.Event{Kind: driver.EventBlur})
		}
	})
	// A click presses without focusing, as in native apps.
	ls.on(s.host, "mousedown", func(e js.Value) {
		if v := s.target(e.Get("target")); v != nil && v.kind != driver.KindInput && v.pressable() {
			e.Call("preventDefault")
		}
	})
}

// cover makes what modal v covers inert, remembering the focus.
func (s *surface) cover(v *view) {
	c := covered{modal: v, focus: s.doc.Get("activeElement")}
	for el := v.outer(); el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		for sib := el.Get("previousElementSibling"); sib.Truthy(); sib = sib.Get("previousElementSibling") {
			if !sib.Get("inert").Bool() {
				sib.Set("inert", true)
				c.inert = append(c.inert, sib)
			}
		}
	}
	s.covers = append(s.covers, c)
}

// uncover gives back what modal v covered once it is gone, and the focus
// when it fell to the page.
func (s *surface) uncover(v *view) {
	i := slices.IndexFunc(s.covers, func(c covered) bool { return c.modal == v })
	if i < 0 {
		return
	}
	c := s.covers[i]
	s.covers = slices.Delete(s.covers, i, i+1)
	for _, el := range c.inert {
		el.Set("inert", false)
	}
	active := s.doc.Get("activeElement")
	fell := !active.Truthy() || active.Equal(s.doc.Get("body")) || !active.Get("isConnected").Bool()
	if fell && c.focus.Truthy() && c.focus.Get("isConnected").Bool() {
		c.focus.Call("focus", map[string]any{"preventScroll": true})
	}
}
