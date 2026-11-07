package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
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

// sidebarSections are an open section, one the app can close, and a third.
func sidebarSections(open func() bool, onOpen func(bool)) []SidebarSection {
	return []SidebarSection{
		{Items: []SidebarItem{{ID: "inbox", Text: "Inbox", Icon: lucide.Bell, TestID: "inbox"}}},
		{Title: "Projects", Collapsible: true, Open: open, OnOpenChange: onOpen, Items: []SidebarItem{
			{ID: "web", Text: "Website", Icon: lucide.Globe, TestID: "web"},
			{ID: "docs", Text: "Docs", Icon: lucide.Globe, TestID: "docs"}}},
		{Title: "Archive", Items: []SidebarItem{{ID: "old", Text: "Old", Icon: lucide.Bell, TestID: "old"}}},
	}
}

// TestSidebarKeysFollowAPress moves the selection with the keys after a
// click, which leaves the focus where it was on desktops.
func TestSidebarKeysFollowAPress(t *testing.T) {
	var selected *core.Signal[string]
	app, _ := start(t, envs()[0].Environment, func(cx *core.Scope) core.Node {
		selected = core.NewSignal(cx, "inbox")
		return core.View{Style: core.Style{Height: core.Pt(400)}, Children: []core.Node{
			Sidebar{Selected: selected.Get, OnSelect: selected.Set, Sections: sidebarSections(nil, nil)}}}
	})
	app.Key("down")
	if got := selected.GetUntracked(); got != "inbox" {
		t.Fatalf("the keys moved a sidebar never pressed to %q", got)
	}
	app.Press("inbox")
	for _, want := range []string{"web", "docs", "old", "old"} {
		app.Key("down")
		if got := selected.GetUntracked(); got != want {
			t.Fatalf("down selected %q, want %q", got, want)
		}
	}
	app.Key("up")
	if got := selected.GetUntracked(); got != "docs" {
		t.Fatalf("up selected %q, want docs", got)
	}
	app.Key("home")
	if got := selected.GetUntracked(); got != "inbox" {
		t.Fatalf("home selected %q", got)
	}
	app.Key("end")
	if got := selected.GetUntracked(); got != "old" {
		t.Fatalf("end selected %q", got)
	}
}

// TestSidebarCollapses folds a section by its title and passes over its
// items with the keys, from the items around them and from one inside.
func TestSidebarCollapses(t *testing.T) {
	var selected *core.Signal[string]
	var reported []bool
	app, _ := start(t, envs()[0].Environment, func(cx *core.Scope) core.Node {
		selected = core.NewSignal(cx, "inbox")
		return core.View{Style: core.Style{Height: core.Pt(400)}, Children: []core.Node{
			Sidebar{Selected: selected.Get, OnSelect: selected.Set,
				Sections: sidebarSections(nil, func(open bool) { reported = append(reported, open) })}}}
	})
	toggle := "role:button Projects"
	if !app.Find(toggle).Props.Expanded {
		t.Fatal("a section starts collapsed")
	}
	app.Press("inbox")
	app.Press(toggle)
	app.Settle()
	app.FastForward(time.Second)
	if app.Find(toggle).Props.Expanded || app.Exists("web") {
		t.Fatal("the title did not fold its items away")
	}
	app.Key("down")
	if got := selected.GetUntracked(); got != "old" {
		t.Fatalf("down selected %q, past the folded items to old", got)
	}
	app.Key("up")
	if got := selected.GetUntracked(); got != "inbox" {
		t.Fatalf("up selected %q, past the folded items to inbox", got)
	}
	selected.Set("web")
	app.Settle()
	app.Key("down")
	if got := selected.GetUntracked(); got != "old" {
		t.Fatalf("down from a folded item selected %q", got)
	}
	app.Press(toggle)
	app.Settle()
	if !app.Exists("web") || !slices.Equal(reported, []bool{false, true}) {
		t.Fatalf("the title did not open again: %v", reported)
	}
}

// TestSidebarSectionStates starts a section closed, and leaves one the app
// holds to the app.
func TestSidebarSectionStates(t *testing.T) {
	var open *core.Signal[bool]
	app, _ := start(t, envs()[0].Environment, func(cx *core.Scope) core.Node {
		open = core.NewSignal(cx, true)
		sections := sidebarSections(open.Get, nil)
		sections[2].Collapsible, sections[2].DefaultClosed = true, true
		return core.View{Style: core.Style{Height: core.Pt(400)}, Children: []core.Node{Sidebar{Sections: sections}}}
	})
	if app.Exists("old") {
		t.Fatal("DefaultClosed showed the items")
	}
	app.Press("role:button Projects")
	app.Settle()
	if !app.Exists("web") {
		t.Fatal("a section the app holds closed by itself")
	}
	open.Set(false)
	app.Settle()
	app.FastForward(time.Second)
	if app.Exists("web") {
		t.Fatal("a section did not follow the app")
	}
}

// TestSidebarTitleTakesNoRoom keeps a collapsible section where a plain
// one is, and shows its arrow only under the pointer on desktops.
func TestSidebarTitleTakesNoRoom(t *testing.T) {
	sidebar := func(e core.Environment, collapsible bool) *nsgotest.App {
		app, _ := start(t, e, func(*core.Scope) core.Node {
			sections := sidebarSections(nil, nil)
			sections[1].Collapsible, sections[2].Collapsible = collapsible, collapsible
			return core.View{Style: core.Style{Height: core.Pt(400)}, Children: []core.Node{Sidebar{Sections: sections}}}
		})
		return app
	}
	plain, folding := sidebar(desktopEnv, false), sidebar(desktopEnv, true)
	for _, id := range []string{"inbox", "web", "docs", "old"} {
		if a, b := plain.Find(id).Frame.Origin, folding.Find(id).Frame.Origin; a.X != b.X {
			t.Fatalf("%s sits at %v in a collapsible section, %v in a plain one", id, b, a)
		}
		if a, b := absoluteY(plain.Find(id)), absoluteY(folding.Find(id)); a != b {
			t.Fatalf("%s sits at y %v in a collapsible section, %v in a plain one", id, b, a)
		}
	}
	arrow := func(app *nsgotest.App) float32 {
		return float32(app.Find("role:button Projects").Children[1].Style.Opacity)
	}
	if got := arrow(folding); got != 0 {
		t.Fatalf("the arrow's opacity at rest is %v", got)
	}
	folding.Hover("role:button Projects")
	if got := arrow(folding); got != 1 {
		t.Fatalf("the arrow's opacity under the pointer is %v", got)
	}
	folding.HoverOut()
	if got := arrow(folding); got != 0 {
		t.Fatalf("the arrow's opacity after the pointer left is %v", got)
	}
	if got := arrow(sidebar(phoneEnv, true)); got != 1 {
		t.Fatalf("the arrow's opacity on a touch screen is %v", got)
	}
}

// absoluteY is where v sits in the window.
func absoluteY(v *nsgotest.View) driver.Points {
	y := driver.Points(0)
	for ; v != nil; v = v.Parent {
		y += v.Frame.Origin.Y
	}
	return y
}
