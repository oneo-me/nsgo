//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Pressable views keep the arrow, as on the desktops: only Style.Cursor
// names a cursor.
func TestFeelCursorOnlyFromStyle(t *testing.T) {
	press := driver.EventMask(1 << driver.EventPress)
	for _, tc := range []struct {
		name   string
		v      view
		cursor string
	}{
		{"plain", view{}, ""},
		{"pressable", view{events: press}, ""},
		{"long press", view{events: driver.EventMask(1 << driver.EventLongPress)}, ""},
		{"text", view{style: driver.ViewStyle{Cursor: driver.CursorText}}, "text"},
		{"pressable with a cursor", view{events: press, style: driver.ViewStyle{Cursor: driver.CursorColResize}}, "col-resize"},
	} {
		css := string(tc.v.feelCSS(nil))
		got := ""
		for _, d := range strings.Split(css, ";") {
			if name, value, ok := strings.Cut(d, ":"); ok && name == "cursor" {
				got = value
			}
		}
		if got != tc.cursor {
			t.Errorf("%s: cursor %q, want %q (%s)", tc.name, got, tc.cursor, css)
		}
	}
}

// Run in a browser, which lays out the alert: nsgo test -target web.
func TestAlertButtonsKeepTheArrow(t *testing.T) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("no document here: run with nsgo test -target web")
	}
	s := &surface{doc: doc}
	s.env.ReduceMotion = true
	s.showAlert(&driver.AlertConfig{ID: 1, Title: "Title", Actions: []driver.AlertAction{
		{Label: "Cancel", Role: driver.AlertCancel}, {Label: "OK", Role: driver.AlertDefault},
	}})
	a := s.alerts[0]
	defer a.layer.Call("remove")
	for _, b := range a.buttons {
		if c := js.Global().Call("getComputedStyle", b).Get("cursor").String(); c != "default" && c != "auto" {
			t.Errorf("%s: cursor %q", b.Get("textContent").String(), c)
		}
	}
}
