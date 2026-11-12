//go:build windows && nsgotest

package windows

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// TestTestHostChoosesNativeMenus chooses a menu bar item and an item of a
// context menu tracking, as nsgo test -target does.
func TestTestHostChoosesNativeMenus(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	var root *core.Scope
	chose := make(chan string, 4)
	app := func(cx *core.Scope) core.Node {
		root = cx
		cx.SetApplicationMenu([]core.MenuItem{{Label: "File", Items: []core.MenuItem{
			{Label: "New", OnSelect: func() { chose <- "New" }}}}})
		return core.View{Style: core.Style{FlexGrow: 1}}
	}
	hwnd := startApp(t, "nsgo-menus", app, 300, 200)
	var ts *testSurface
	var newID uint32
	for deadline := time.Now().Add(5 * time.Second); newID == 0 && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		onMain(t, hwnd, func(w *window) {
			ts = &testSurface{w: w}
			if len(w.chrome.entries) > 0 {
				newID = w.chrome.entries[len(w.chrome.entries)-1].id
			}
		})
	}
	if newID == 0 {
		t.Fatal("no menu bar")
	}
	if err := ts.ChooseMenu(newID); err != nil {
		t.Fatal(err)
	}
	if got := <-chose; got != "New" {
		t.Errorf("chose %q", got)
	}
	root.Post(func() {
		root.PopupMenu([]core.MenuItem{{Label: "Copy", OnSelect: func() { chose <- "Copy" }}}, core.Point{X: 20, Y: 20}, nil)
	})
	var copyID uint32
	for deadline := time.Now().Add(5 * time.Second); copyID == 0 && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		onMain(t, hwnd, func(w *window) {
			if w.popupTracking && len(w.popupShown) > 0 {
				copyID = w.popupShown[0].ID
			}
		})
	}
	if copyID == 0 {
		t.Fatal("no context menu tracks")
	}
	if err := ts.ChoosePopup(copyID); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-chose:
		if got != "Copy" {
			t.Errorf("chose %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Error("the context menu's item was not chosen")
	}
}
