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

// TestApplicationMenuBuildsAndAnswersItsShortcut shows the app menu as a
// GTK menu bar and runs the item a native key press matches.
func TestApplicationMenuBuildsAndAnswersItsShortcut(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-menu-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	app := func(cx *core.Scope) core.Node {
		bumped := core.NewSignal(cx, 0)
		cx.SetApplicationMenu([]core.MenuItem{
			{Label: "File", Items: []core.MenuItem{
				{Label: "Bump", Shortcut: "mod+b", OnSelect: func() { bumped.Update(func(n int) int { return n + 1 }) }},
				{Separator: true},
				{Label: "Quit", Role: driver.MenuQuit},
			}},
		})
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d", title, bumped.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100)}}
	}
	w := startTestWindow(t, engine.App(app), Options{Title: title, Width: 200, Height: 100})
	waitForTitle(t, w, title+"|0")
	w.onLoop(t, func() {
		if w.menuBar == 0 {
			t.Error("the application menu has no GTK menu bar")
		}
		if gtkWidgetGetParent(w.menuBar) != w.box {
			t.Error("the menu bar is not above the surface")
		}
	})
	w.onLoop(t, func() { keyPressedCallback(0, 'b', 0, gdkControl, w.handle) })
	waitForTitle(t, w, title+"|1")

	// An app rebuilds its menu whenever a checkable item changes. The
	// rebuilds must not run out of purego callbacks or GTK actions.
	w.onLoop(t, func() {
		entries := w.chrome.menu
		for range 250 {
			w.setMenu(entries)
		}
		if w.menuBar == 0 {
			t.Error("the menu bar went away after rebuilding")
		}
	})
}
