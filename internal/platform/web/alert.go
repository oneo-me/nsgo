//go:build js && wasm

package web

import (
	"fmt"
	"slices"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// alert is an open alert: a dialog over the whole page, answered once.
type alert struct {
	id     uint32
	cancel int // the action Escape chooses, or -1
	// cancelable: Escape and a press outside close it without an action.
	cancelable bool
	layer      js.Value
	buttons    []js.Value
	order      []int    // the action of each button
	restore    js.Value // what had focus before
	listeners  listeners
}

// showAlert draws c over the page and its app, which keep running.
func (s *surface) showAlert(c *driver.AlertConfig) {
	ink, panel, line, muted, danger := "#1f2328", "#ffffff", "#d0d7de", "#59636e", "#cf222e"
	if s.env.Dark {
		ink, panel, line, muted, danger = "#e6edf3", "#2d333b", "#444c56", "#9198a1", "#ff7b72"
	}
	add := func(tag string, parent js.Value, style map[string]string) js.Value {
		el := s.doc.Call("createElement", tag)
		setStyle(el, style)
		parent.Call("appendChild", el)
		return el
	}
	a := &alert{id: c.ID, cancel: -1, cancelable: c.Cancelable, restore: s.doc.Get("activeElement")}
	// An action sheet rises from the bottom with its actions stacked.
	align, direction, justify := "center", "row", "flex-end"
	if c.Sheet {
		align, direction, justify = "flex-end", "column", "stretch"
	}
	a.layer = add("div", s.doc.Get("body"), map[string]string{
		"position": "fixed", "inset": "0", "z-index": "2147483647", "box-sizing": "border-box",
		"display": "flex", "align-items": align, "justify-content": "center", "padding": "24px",
		"background": "rgba(0,0,0,0.35)", "font-family": fontFamily, "color": ink,
	})
	box := add("div", a.layer, map[string]string{
		"box-sizing": "border-box", "width": "100%", "max-width": "340px", "padding": "20px",
		"display": "flex", "flex-direction": "column", "gap": "8px",
		"background": panel, "border-radius": "14px", "box-shadow": "0 12px 40px rgba(0,0,0,0.3)",
	})
	if s.rtl {
		a.layer.Call("setAttribute", "dir", "rtl")
	}
	box.Call("setAttribute", "role", "alertdialog")
	box.Call("setAttribute", "aria-modal", "true")
	text := func(id, content string, style map[string]string) {
		if content == "" {
			return
		}
		el := add("div", box, style)
		el.Set("id", fmt.Sprintf("nsgo-alert-%d-%s", c.ID, id))
		el.Set("textContent", content)
		attr := map[string]string{"title": "aria-labelledby", "message": "aria-describedby"}[id]
		box.Call("setAttribute", attr, el.Get("id"))
	}
	text("title", c.Title, map[string]string{"font-size": "17px", "font-weight": "600"})
	text("message", c.Message, map[string]string{
		"font-size": "14px", "line-height": "1.4", "color": muted, "white-space": "pre-wrap",
	})
	row := add("div", box, map[string]string{
		"display": "flex", "flex-direction": direction, "flex-wrap": "wrap", "justify-content": justify,
		"gap": "8px", "margin-top": "12px",
	})
	// An alert without buttons still needs a way out: a dismissal.
	actions, dismiss := c.Actions, len(c.Actions) == 0
	if dismiss {
		actions = []driver.AlertAction{{Label: "OK"}}
		a.cancelable = true
	}
	// order[i] is the action of button i: an action sheet's cancel
	// action goes last.
	order := make([]int, 0, len(actions))
	for i, act := range actions {
		if !c.Sheet || act.Role != driver.AlertCancel {
			order = append(order, i)
		}
	}
	for i, act := range actions {
		if c.Sheet && act.Role == driver.AlertCancel {
			order = append(order, i)
		}
	}
	a.order = order
	focus := -1
	for at, i := range order {
		act := actions[i]
		b := add("button", row, map[string]string{
			"font": "inherit", "font-size": "14px", "padding": "8px 16px", "border-radius": "8px",
			"border": "1px solid " + line, "background": panel, "color": ink,
		})
		b.Set("type", "button")
		b.Set("textContent", act.Label)
		switch act.Role {
		case driver.AlertDefault:
			b.Get("style").Call("setProperty", "font-weight", "600")
			if focus < 0 {
				focus = at
			}
		case driver.AlertCancel:
			if a.cancel < 0 {
				a.cancel = i
				if focus < 0 {
					focus = -2 - at
				}
			}
		case driver.AlertDestructive:
			b.Get("style").Call("setProperty", "color", danger)
		}
		index := i
		if dismiss {
			index = -1
		}
		a.listeners.on(b, "click", func(js.Value) { s.answer(a, index) })
		a.buttons = append(a.buttons, b)
	}
	if focus < 0 {
		// The cancel button, else the first.
		focus = max(-2-focus, 0)
	}
	// Presses stay in the alert, and so does focus, which Tab cycles
	// through its buttons.
	a.listeners.on(a.layer, "mousedown", func(e js.Value) {
		if e.Get("target").Get("tagName").String() != "BUTTON" {
			e.Call("preventDefault")
		}
	})
	a.listeners.on(a.layer, "click", func(e js.Value) {
		if e.Get("target").Equal(a.layer) && a.cancelable {
			s.answer(a, a.cancel)
		}
	})
	a.listeners.on(a.layer, "keydown", func(e js.Value) {
		if e.Get("key").String() != "Tab" {
			return
		}
		e.Call("preventDefault")
		step := 1
		if e.Get("shiftKey").Bool() {
			step = -1
		}
		i := slices.IndexFunc(a.buttons, func(b js.Value) bool { return b.Equal(s.doc.Get("activeElement")) })
		n := len(a.buttons)
		a.buttons[((i+step)%n+n)%n].Call("focus")
	})
	s.alerts = append(s.alerts, a)
	a.buttons[focus].Call("focus")
	if !s.env.ReduceMotion {
		animate(a.layer, map[string]any{"opacity": 0}, 150, "ease-out")
	}
}

// answer closes a with the action at index, or -1, and tells the engine.
func (s *surface) answer(a *alert, index int) {
	i := slices.Index(s.alerts, a)
	if i < 0 {
		return
	}
	s.alerts = slices.Delete(s.alerts, i, i+1)
	a.listeners.release()
	a.layer.Call("remove")
	if r := a.restore; r.Truthy() && r.Get("isConnected").Bool() {
		r.Call("focus", map[string]any{"preventScroll": true})
	}
	s.engine.Dispatch(driver.Event{Kind: driver.EventAlert, Count: a.id, Index: index})
}
