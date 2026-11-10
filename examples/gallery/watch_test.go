package main

import (
	"fmt"
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/ui"
)

// startWatch runs root on an Apple Watch.
func startWatch(t *testing.T, root func(cx *core.Scope) core.Node) *nsgotest.App {
	if nsgotest.Target() != "watchos_simulator" {
		t.Skip("needs an Apple Watch: nsgo test -target watchos_simulator")
	}
	return nsgotest.Start(t, root)
}

// TestWatchPressAndScroll presses a button, scrolls a list longer than
// the screen to its end and presses the last row, which showed only once
// scrolled to.
func TestWatchPressAndScroll(t *testing.T) {
	count := 0
	var pressed string
	app := startWatch(t, func(cx *core.Scope) core.Node {
		n := core.NewSignal(cx, 0)
		rows := []core.Node{ui.Button{Text: "Add", TestID: "add", OnPress: func() {
			count++
			n.Set(count)
		}}, core.Text{TestID: "count", ContentFunc: func() string { return fmt.Sprint(n.Get()) }}}
		for i := range 40 {
			id := fmt.Sprintf("row-%d", i)
			rows = append(rows, ui.Button{Text: id, TestID: id, OnPress: func() { pressed = id }})
		}
		return core.Scroll{TestID: "list", Style: core.Style{FlexGrow: 1},
			Children: []core.Node{core.View{Style: core.Style{Padding: core.All(8), Gap: 8}, Children: rows}}}
	})
	app.Press("id:add")
	app.Press("id:add")
	if count != 2 || app.Find("id:count").Text != "2" {
		t.Fatalf("two presses counted %d, showing %q", count, app.Find("id:count").Text)
	}
	app.Screenshot("watch-list")
	app.Scroll("id:list", 5000)
	app.Press("id:row-39")
	if pressed != "row-39" {
		t.Fatalf("pressed %q, want the last row", pressed)
	}
}

// TestWatchInput types into a field the system's text input edits, and
// submits it.
func TestWatchInput(t *testing.T) {
	var submitted string
	app := startWatch(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(8)}, Children: []core.Node{
			ui.Input{TestID: "name", Placeholder: "Name", OnSubmit: func(s string) { submitted = s }},
		}}
	})
	app.Focus("id:name")
	app.Type("id:name", "Ann")
	app.Submit("id:name")
	if submitted != "Ann" {
		t.Fatalf("submitted %q, want Ann", submitted)
	}
}

// TestWatchAlert shows an alert and answers it.
func TestWatchAlert(t *testing.T) {
	answer := -2
	app := startWatch(t, func(cx *core.Scope) core.Node {
		return ui.Button{Text: "Ask", TestID: "ask", OnPress: func() {
			cx.ShowAlert(core.AlertConfig{Title: "Delete?", Actions: []core.AlertAction{
				{Label: "Cancel", Role: core.AlertCancel}, {Label: "Delete", Role: core.AlertDestructive}}},
				func(i int) { answer = i })
		}}
	})
	app.Press("id:ask")
	app.Answer(1)
	if answer != 1 {
		t.Fatalf("the alert answered %d, want 1", answer)
	}
}

// TestWatchGallery opens the gallery's first demo from its home page on a
// watch's screen.
func TestWatchGallery(t *testing.T) {
	app := startWatch(t, Root)
	app.Screenshot("watch-gallery")
}
