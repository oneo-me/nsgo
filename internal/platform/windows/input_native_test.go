//go:build windows

package windows

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// editingApp is a multiline field capped at 12 characters above a
// single-line field. The title shows the note (line breaks as /), its
// height, the last key and selection, and the second field's submits
// and blurs.
func editingApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		note, line := core.NewSignal(cx, ""), core.NewSignal(cx, "")
		height := core.NewSignal(cx, driver.Points(0))
		key := core.NewSignal(cx, "")
		sel := core.NewSignal(cx, driver.TextRange{})
		submitted, blurred := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		cx.Effect(func() {
			if note.Get() == "" {
				return // the window keeps its title until the test types
			}
			cx.SetTitle(fmt.Sprintf("%s|%s|%v|%s|%d-%d|%d|%d", title, strings.ReplaceAll(note.Get(), "\n", "/"),
				height.Get() > 24, key.Get(), sel.Get().Start, sel.Get().End, submitted.Get(), blurred.Get()))
		})
		return core.View{
			Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240), Padding: driver.All(10), Gap: 10,
				Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Input{Value: note.Get, OnChange: note.Set, Multiline: true, MaxLength: 12, AutoFocus: true,
					OnKeyPress: key.Set, OnSelectionChange: sel.Set,
					OnContentSizeChange: func(s driver.Size) { height.Set(s.Height) },
					Style:               core.Style{Width: driver.Pt(300), MinHeight: driver.Pt(30), FontSize: 16}},
				core.Input{Value: line.Get, OnChange: line.Set,
					OnSubmit: func(string) { submitted.Update(func(n int) int { return n + 1 }) },
					OnBlur:   func() { blurred.Update(func(n int) int { return n + 1 }) },
					Style:    core.Style{Width: driver.Pt(300), Height: driver.Pt(30), FontSize: 16}},
			},
		}
	}
}

// TestDrawnFieldTakesLinesLimitsAndSubmits types lines into a drawn
// multiline field, which Return breaks and MaxLength caps, moves its
// selection from the app, then submits a single line, which loses the
// focus.
func TestDrawnFieldTakesLinesLimitsAndSubmits(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-editing"
	hwnd := startApp(t, title, editingApp(title), 320, 240)

	typeText(hwnd, "one")
	pressKey(hwnd, vkReturn, '\r')
	typeText(hwnd, "two")
	// Two lines are taller than one; the last key and the caret show.
	waitForTitle(t, hwnd, title+"|one/two|true|o|7-7|", 5*time.Second)
	// Twelve characters at most.
	typeText(hwnd, "threefour")
	waitForTitle(t, hwnd, title+"|one/twothree|", 5*time.Second)

	// The app's selection, then Up moves to the first line.
	onMain(t, hwnd, func(w *window) {
		in := w.focusedField()
		in.selectRange(driver.TextRange{Start: 1, End: 3})
		if s, e := in.model.selection(); s != 1 || e != 3 {
			t.Errorf("selection %d-%d, want 1-3", s, e)
		}
		w.editKey(vkDown, false, false, false)
		if in.model.caret <= 4 {
			t.Errorf("Down left the caret at %d on the first line", in.model.caret)
		}
	})

	// Tab to the line; Return submits it and takes its focus.
	onMain(t, hwnd, func(w *window) { w.tab(false) })
	typeText(hwnd, "x")
	pressKey(hwnd, vkReturn, '\r')
	got := waitForTitle(t, hwnd, title+"|one/twothree|", 5*time.Second)
	deadline := time.Now().Add(5 * time.Second)
	for !strings.HasSuffix(got, "|1|1") && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		got = windowText(hwnd)
	}
	if !strings.HasSuffix(got, "|1|1") {
		t.Fatalf("title %q, want one submit and one blur", got)
	}
}
