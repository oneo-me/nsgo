package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TestSidebar selects an item of a grouped sidebar.
func TestSidebar(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			selected := core.NewSignal(cx, "inbox")
			return core.View{Style: core.Style{Height: core.Pt(400)}, Children: []core.Node{
				Sidebar{TestID: "sidebar", Selected: selected.Get, OnSelect: selected.Set, Sections: []SidebarSection{
					{Items: []SidebarItem{{ID: "inbox", Text: "Inbox", Icon: IconBell, TestID: "inbox"}}},
					{Title: "Projects", Items: []SidebarItem{{ID: "web", Text: "Website", Icon: IconGlobe, TestID: "web"}}},
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

// TestBreadcrumbs presses an ancestor, and one collapsed into the menu.
func TestBreadcrumbs(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var chosen []int
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			path := []string{"Home", "Documents", "Projects", "Onexa", "Notes"}
			return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
				Breadcrumbs{Items: path[:3], OnSelect: func(i int) { chosen = append(chosen, i) }},
				Breadcrumbs{Items: path, MaxItems: 3, MoreLabel: "More", OnSelect: func(i int) { chosen = append(chosen, i) }},
			}}
		})
		app.Press("role:link Documents")
		if app.Exists("role:link Projects") {
			t.Fatal("the current item is pressable")
		}
		app.Press("role:button More")
		app.Press("role:menuitem Projects")
		if len(chosen) != 2 || chosen[0] != 1 || chosen[1] != 2 {
			t.Fatalf("chose %v, want [1 2]", chosen)
		}
	})
}

// TestNavPage gives a page without a header of its own a themed one with
// a back button, below the status bar where the system has a header.
func TestNavPage(t *testing.T) {
	phone := phoneEnv
	phone.HeaderHeight, phone.SafeArea = 44, core.Edges{Top: 47, Bottom: 34}
	for _, e := range []core.Environment{desktopEnv, phone} {
		popped := false
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			nav := base.NewNav(cx, "home", "detail")
			return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, _ func() int) base.NavPage {
				return NavPage(cx, base.NavPage{Title: "Detail " + route, TestID: "page-" + route},
					NavHeader{TestID: "header", OnBack: func() { popped = true }, BackLabel: "Back"})
			})
		})
		header := app.Find("page-detail >> header")
		if header.Style.Background != p.Surface {
			t.Fatalf("%v: header = %x, want the surface", e.OS, header.Style.Background)
		}
		if e.OS == core.OSIOS && header.Frame.Origin.Y != 47 {
			t.Fatalf("header top = %v, want below the status bar", header.Frame.Origin.Y)
		}
		app.Find("Detail detail")
		app.Press("role:button Back")
		if !popped {
			t.Fatalf("%v: the back button did nothing", e.OS)
		}
	}
}
