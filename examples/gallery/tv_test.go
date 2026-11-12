package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/ui"
)

// startTV runs root on an Apple TV, whose remote moves a focus.
func startTV(t *testing.T, root func(cx *core.Scope) core.Node) *nsgotest.App {
	if nsgotest.Target() != "tvos_simulator" {
		t.Skip("needs an Apple TV: nsgo test -target tvos_simulator")
	}
	return nsgotest.Start(t, root)
}

// focusedID is the test ID of the view with the focus, or "".
func focusedID(app *nsgotest.App, ids ...string) string {
	for _, id := range ids {
		if app.Find("id:" + id).Focused {
			return id
		}
	}
	return ""
}

// focusWithin reports whether v or a view inside it has the focus.
func focusWithin(v *nsgotest.View) bool {
	if v.Focused {
		return true
	}
	for _, c := range v.Children {
		if focusWithin(c) {
			return true
		}
	}
	return false
}

// TestTVRemote moves the focus between buttons with the remote's arrows
// and presses the focused one with its select button.
func TestTVRemote(t *testing.T) {
	var pressed []string
	app := startTV(t, func(cx *core.Scope) core.Node {
		button := func(id string) core.Node {
			return ui.Button{Text: id, TestID: id, OnPress: func() { pressed = append(pressed, id) }}
		}
		return core.View{Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter, Gap: 40},
			Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Gap: 40}, Children: []core.Node{button("a"), button("b")}},
				button("c"),
			}}
	})
	ids := []string{"a", "b", "c"}
	if got := focusedID(app, ids...); got != "a" {
		t.Fatalf("the focus starts on %q, want the first button", got)
	}
	app.Key("right")
	if got := focusedID(app, ids...); got != "b" {
		t.Fatalf("right moved the focus to %q, want b", got)
	}
	app.Key("down")
	if got := focusedID(app, ids...); got != "c" {
		t.Fatalf("down moved the focus to %q, want c", got)
	}
	app.Key("enter")
	if len(pressed) != 1 || pressed[0] != "c" {
		t.Fatalf("select pressed %v, want [c]", pressed)
	}
}

// TestTVModalFocus moves the focus into a dialog as it opens, keeps it
// there whichever way the remote moves it, and gives it back to the
// button that opened it as the remote's Menu button closes the dialog.
func TestTVModalFocus(t *testing.T) {
	app := startTV(t, func(cx *core.Scope) core.Node {
		open := core.NewSignal(cx, false)
		return core.View{Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter, Gap: 40},
			Children: []core.Node{
				ui.Button{Text: "Other", TestID: "other", OnPress: func() {}},
				ui.Button{Text: "Open", TestID: "open", OnPress: func() { open.Set(true) }},
				ui.Dialog{Open: open.Get, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, Title: "Dialog", TestID: "dialog", Children: []core.Node{
					ui.Button{Text: "Inside", TestID: "inside", OnPress: func() {}},
				}},
			}}
	})
	app.Key("down")
	if got := focusedID(app, "other", "open"); got != "open" {
		t.Fatalf("down moved the focus to %q, want open", got)
	}
	app.Key("enter")
	if !focusWithin(app.Find("id:dialog")) {
		t.Fatal("the dialog opened without the focus")
	}
	for _, key := range []string{"up", "down", "left", "right"} {
		app.Key(key)
		if !focusWithin(app.Find("id:dialog")) {
			t.Fatalf("%s moved the focus out of the dialog", key)
		}
	}
	app.Key("escape")
	app.FastForward(time.Second)
	if app.Exists("id:inside") {
		t.Fatal("Menu left the dialog open")
	}
	if got := focusedID(app, "other", "open"); got != "open" {
		t.Fatalf("the focus went back to %q, want the button that opened the dialog", got)
	}
}
