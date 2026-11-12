//go:build linux && !android

package linux

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// editingApp is a multiline field capped at 12 characters and a single
// line. The title shows the note (line breaks as /), its content height
// being above one line's, its selection, and the line's submits and blurs.
func editingApp(title string) engine.App {
	return func(cx *core.Scope) core.Node {
		note, line := core.NewSignal(cx, ""), core.NewSignal(cx, "")
		height := core.NewSignal(cx, driver.Points(0))
		sel := core.NewSignal(cx, driver.TextRange{})
		submitted, blurred := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|%s|%v|%d-%d|%d|%d", title, strings.ReplaceAll(note.Get(), "\n", "/"),
				height.Get() > 30, sel.Get().Start, sel.Get().End, submitted.Get(), blurred.Get()))
		})
		return core.View{
			Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240), Padding: driver.All(10), Gap: 10,
				Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Input{Value: note.Get, OnChange: note.Set, Multiline: true, MaxLength: 12, TestID: "note",
					OnSelectionChange:   sel.Set,
					OnContentSizeChange: func(s driver.Size) { height.Set(s.Height) },
					Style:               core.Style{Width: driver.Pt(300), MinHeight: driver.Pt(30), FontSize: 16}},
				core.Input{Value: line.Get, OnChange: line.Set, TestID: "line",
					OnSubmit: func(string) { submitted.Update(func(n int) int { return n + 1 }) },
					OnBlur:   func() { blurred.Update(func(n int) int { return n + 1 }) },
					Style:    core.Style{Width: driver.Pt(300), Height: driver.Pt(30), FontSize: 16}},
			},
		}
	}
}

// TestEditorsTakeLinesLimitsAndSubmit inserts into a GtkTextView, which
// MaxLength cuts, selects in it from the app, and submits a GtkEntry,
// which loses the focus.
func TestEditorsTakeLinesLimitsAndSubmit(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-editing"
	w := startTestWindow(t, editingApp(title), Options{Title: title, Width: 320, Height: 240})
	waitForTitle(t, w, title+"||false|0-0|0|0")
	var note, line *view
	w.onLoop(t, func() {
		for _, v := range w.entries {
			if v.textView != 0 {
				note = v
			} else if v.kind == driver.KindInput {
				line = v
			}
		}
		w.focusEntry(note)
		gtkTextBufferInsertAtCursor(note.buffer, cstr("one\ntwo"), -1)
	})
	waitForTitle(t, w, title+"|one/two|true|7-7|0|0")
	w.onLoop(t, func() { gtkTextBufferInsertAtCursor(note.buffer, cstr("threefour"), -1) })
	waitForTitle(t, w, title+"|one/twothree|true|12-12|0|0")
	w.onLoop(t, func() {
		w.selectRange(note, driver.TextRange{Start: 1, End: 3})
		if s, e := w.selection(note); s != 1 || e != 3 {
			t.Errorf("selection %d-%d, want 1-3", s, e)
		}
		w.focusEntry(line)
		gtkEditableSetText(line.entry, cstr("x"))
		gSignalEmitByName(line.entry, cstr("activate"))
	})
	waitForTitle(t, w, title+"|one/twothree|true|12-12|1|1")
}

// TestKeysCaretAndPasteInAField focuses a field that reports its caret
// and pastes: a plain key binding leaves the field its typing, one that
// asks fires there and once while held, the caret is reported, and a
// paste is reported with the clipboard's text, kept out of the field.
func TestKeysCaretAndPasteInAField(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-keys"
	app := func(cx *core.Scope) core.Node {
		x, down := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		caret := core.NewSignal(cx, false)
		pasted := core.NewSignal(cx, "")
		cx.OnKey("x", func() { x.Update(func(n int) int { return n + 1 }) })
		cx.OnKeyWith("down", core.KeyOptions{When: core.KeyAlways}, func() { down.Update(func(n int) int { return n + 1 }) })
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|%d|%d|%v|%s", title, x.Get(), down.Get(), caret.Get(), pasted.Get()))
		})
		return core.View{Style: core.Style{Padding: driver.All(20)}, Children: []core.Node{
			core.Input{Style: core.Style{Height: driver.Pt(30)}, PasteBehavior: driver.PasteReport,
				OnCaretMove: func(e core.CaretEvent) { caret.Set(e.Bounds.Size.Height > 0) },
				OnPaste:     func(e driver.PasteEvent) { pasted.Set(e.Text) }},
		}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 320, Height: 200})
	waitForTitle(t, w, title+"|0|0|false|")
	var field *view
	w.onLoop(t, func() {
		for _, v := range w.entries {
			if v.kind == driver.KindInput {
				field = v
			}
		}
		w.focusEntry(field)
	})
	waitForTitle(t, w, title+"|0|0|true|")
	w.onLoop(t, func() {
		keyPressedCallback(0, 'x', 53, 0, w.handle) // the field types it
		keyReleasedCallback(0, 'x', 53, 0, w.handle)
		keyPressedCallback(0, gdkDown, 116, 0, w.handle)
		keyPressedCallback(0, gdkDown, 116, 0, w.handle) // held: once
	})
	waitForTitle(t, w, title+"|0|1|true|")
	w.onLoop(t, func() {
		w.setClipboard("pasted text")
		gSignalEmitByName(gtkEditableGetDelegate(field.entry), cstr("paste-clipboard"))
	})
	waitForTitle(t, w, title+"|0|1|true|pasted text")
	w.onLoop(t, func() {
		if got := w.editorText(field); got != "" {
			t.Errorf("the field reads %q, want the paste kept out", got)
		}
	})
}
