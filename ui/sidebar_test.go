package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// TestSidebar selects an item of a grouped sidebar.
func TestSidebar(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			selected := core.NewSignal(cx, "inbox")
			return core.View{Style: core.Style{Height: core.Pt(400)}, Children: []core.Node{
				Sidebar{TestID: "sidebar", Selected: selected.Get, OnSelect: selected.Set, Sections: []SidebarSection{
					{Items: []SidebarItem{{ID: "inbox", Text: "Inbox", Icon: lucide.Bell, TestID: "inbox"}}},
					{Title: "Projects", Items: []SidebarItem{{ID: "web", Text: "Website", Icon: lucide.Globe, TestID: "web"}}},
				}}}}
		})
		if got := app.Find("inbox").Style.Background; got != Alpha(p.Text, 0.1) {
			t.Fatalf("selected row = %x, want the faint text color", got)
		}
		app.Press("web")
		if !app.Find("web").Props.Selected || app.Find("inbox").Props.Selected {
			t.Fatal("pressing Website did not select it alone")
		}
		app.Find("Projects")
	})
}
