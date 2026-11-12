package ui

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
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
		return float32(app.Find("role:button Projects").Children[2].Style.Opacity)
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

// TestSidebarSectionsChange lists items added after the sidebar showed,
// in a section still folded as the user left it, its title the same view
// with its count following, the keys moving through them, and an item's
// Content in place of its text.
func TestSidebarSectionsChange(t *testing.T) {
	var items *core.Signal[[]string]
	var selected *core.Signal[string]
	app, _ := start(t, envs()[0].Environment, func(cx *core.Scope) core.Node {
		items, selected = core.NewSignalFunc(cx, []string{"a"}, slices.Equal), core.NewSignal(cx, "a")
		sections := func() []SidebarSection {
			var list []SidebarItem
			for _, id := range items.Get() {
				list = append(list, SidebarItem{ID: id, Text: id, Content: Text("row "+id, 13, Regular, Inherited), TestID: id})
			}
			return []SidebarSection{{Items: list}, {Title: "Later", Collapsible: true, Items: []SidebarItem{{ID: "z", Text: "z", TestID: "z"}},
				Trailing: Text(strconv.Itoa(len(list)), 11, Regular, Inherited)}}
		}
		return core.View{Style: core.Style{Height: core.Pt(400)}, Children: []core.Node{
			Sidebar{Selected: selected.Get, OnSelect: selected.Set, SectionsFunc: sections}}}
	})
	app.Find("row a")
	app.Press("role:button Later")
	app.Settle()
	app.FastForward(time.Second)
	title := app.Find("role:button Later").ID
	items.Set([]string{"a", "b"})
	app.Settle()
	if !app.Exists("b") || app.Exists("z") {
		t.Fatal("the new item did not show, or the folded section opened again")
	}
	if app.Find("role:button Later").ID != title || !app.Exists("2") {
		t.Fatal("the title was built again, or its count did not follow")
	}
	app.Press("a")
	app.Key("down")
	if got := selected.GetUntracked(); got != "b" {
		t.Fatalf("down selected %q, want the new item", got)
	}
}

// TestSidebarTrailing shows a collapsible title's count at its end until
// the pointer brings the arrow there in its place, the count lingering
// out of sight a moment after it leaves.
func TestSidebarTrailing(t *testing.T) {
	app, _ := start(t, desktopEnv, func(*core.Scope) core.Node {
		sections := sidebarSections(nil, nil)
		sections[1].Trailing = Text("2", 11, Regular, Inherited)
		return core.View{Style: core.Style{Width: core.Pt(220), Height: core.Pt(400)}, Children: []core.Node{Sidebar{Sections: sections}}}
	})
	title := app.Find("role:button Projects")
	trailing, arrow := title.Children[1], title.Children[2]
	if trailing.Style.Opacity != 1 || arrow.Style.Opacity != 0 {
		t.Fatalf("at rest the count's opacity is %v and the arrow's %v", trailing.Style.Opacity, arrow.Style.Opacity)
	}
	if end, arrowEnd := trailing.Frame.Origin.X+trailing.Frame.Size.Width-8, title.Frame.Size.Width-8; end != arrowEnd ||
		arrow.Frame.Origin.X+arrow.Frame.Size.Width != arrowEnd {
		t.Fatalf("the count ends at %v and the arrow at %v, want both at %v", end, arrow.Frame.Origin.X+arrow.Frame.Size.Width, arrowEnd)
	}
	// At rest, the count waits to come back and the arrow to go, past a
	// moment's report of the pointer gone; the pointer swaps them at once.
	if trailing.Style.Transition.Delay != sidebarLinger || arrow.Style.Transition.Delay != sidebarLinger {
		t.Fatalf("the count comes back after %v and the arrow goes after %v", trailing.Style.Transition.Delay, arrow.Style.Transition.Delay)
	}
	app.Hover("role:button Projects")
	title = app.Find("role:button Projects")
	if title.Children[1].Style.Opacity != 0 || title.Children[2].Style.Opacity != 1 {
		t.Fatal("the pointer did not swap the count for the arrow")
	}
	if title.Children[1].Style.Transition.Delay != 0 || title.Children[2].Style.Transition.Delay != 0 {
		t.Fatal("the pointer waited to swap the count for the arrow")
	}
}

// TestSidebarTitleHoldsItsArrow keeps a pressed title's arrow, and its
// count away, through the fold, wherever the platform says the pointer is.
func TestSidebarTitleHoldsItsArrow(t *testing.T) {
	app, _ := start(t, desktopEnv, func(*core.Scope) core.Node {
		sections := sidebarSections(nil, nil)
		sections[1].Trailing = Text("2", 11, Regular, Inherited)
		return core.View{Style: core.Style{Width: core.Pt(220), Height: core.Pt(400)}, Children: []core.Node{Sidebar{Sections: sections}}}
	})
	app.Press("role:button Projects")
	app.HoverOut()
	title := app.Find("role:button Projects")
	if title.Children[1].Style.Opacity != 0 || title.Children[2].Style.Opacity != 1 {
		t.Fatal("the count came back while the section folded")
	}
	app.FastForward(sidebarHold)
	title = app.Find("role:button Projects")
	if title.Children[1].Style.Opacity != 1 || title.Children[2].Style.Opacity != 0 {
		t.Fatal("the count did not come back after the fold")
	}
}
