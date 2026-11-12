package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base/dock"
	"github.com/nsgo-dev/nsgo/core"
)

// TestDockArea selects a tab, which takes the surface, and closes a
// closable panel from its tab.
func TestDockArea(t *testing.T) {
	app, p := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return DockArea{TestID: "dock", Style: core.Style{Height: core.Pt(400)},
			DefaultLayout: dock.Layout{Center: dock.Tabs("editor", "notes"),
				Left: &dock.Region{Root: dock.Tabs("files"), Size: 200}},
			Panels: map[string]dock.PanelOptions{"editor": {NoClose: true}},
			RenderTab: func(cx *core.Scope, tab dock.Tab) core.Node {
				return DockTabLabel{Tab: tab, Text: tab.Key, CloseLabel: "Close " + tab.Key}
			},
			RenderPanel: func(cx *core.Scope, key string) core.Node { return core.Text{Content: key + " panel"} }}
	})
	app.Press("role:tab notes")
	tab := app.Find("role:tab notes")
	if !tab.Props.Selected || tab.Style.Background != p.Surface {
		t.Fatalf("selected tab = %x, want the surface", tab.Style.Background)
	}
	app.Find("notes panel")
	app.Press("role:button Close notes")
	if app.Exists("role:tab notes") {
		t.Fatal("the close button left the tab")
	}
	if app.Exists("role:button Close editor") {
		t.Fatal("a panel that is not closable has a close button")
	}
}
