package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// tabsContent shows a list of each variant, named after it, of tabs of
// different widths with the third disabled.
func tabsContent(cx *core.Scope) core.Node {
	var lists []core.Node
	for v, name := range []string{"segmented", "underline"} {
		selected := core.NewSignal(cx, 0)
		lists = append(lists, Tabs[int]{Variant: TabsVariant(v), Label: name, TestID: name, Value: selected.Get, OnChange: selected.Set,
			Options: []Option[int]{{Value: 0, Label: "One", TestID: name + "-0"}, {Value: 1, Label: "A longer tab", TestID: name + "-1"},
				{Value: 2, Label: "Three", TestID: name + "-2", Disabled: func() bool { return true }}}})
	}
	return core.View{Style: core.Style{Gap: 16, AlignItems: core.AlignStart}, Children: lists}
}

// checkIndicator fails unless name's indicator lies on tab, at its look.
func checkIndicator(t *testing.T, app *nsgotest.App, name, tab string) {
	t.Helper()
	at, f := app.Find(tab).Frame, app.Find(name).Children[0].Frame
	want := at
	if name == "underline" {
		want.Origin.X, want.Size.Width = at.Origin.X+6, at.Size.Width-12
		want.Origin.Y, want.Size.Height = at.Origin.Y+at.Size.Height-2, 2
	}
	if f != want {
		t.Fatalf("%s: indicator at %+v, want %+v over %s", name, f, want, tab)
	}
}

// TestTabs selects tabs of both variants, skips a disabled one, and moves
// the one indicator onto the selected tab.
func TestTabs(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, tabsContent)
		for _, name := range []string{"segmented", "underline"} {
			checkIndicator(t, app, name, name+"-0")
			app.Press(name + "-1")
			if !app.Find(name+"-1").Props.Selected || app.Find(name+"-0").Props.Selected {
				t.Fatalf("%s: pressing the second tab did not select it alone", name)
			}
			if !app.Find(name + "-2").Props.Disabled {
				t.Fatalf("%s: the third tab is not disabled", name)
			}
			checkIndicator(t, app, name, name+"-1")
		}
		if got := app.Find("underline").Children[0].Style.Background; got != p.Accent {
			t.Fatalf("underline = %x, want the accent", got)
		}
		if got := app.Find("segmented").Style.Background; got == 0 {
			t.Fatal("the segmented list has no track")
		}
	})
}

// TestTabsEqual shares the width between the tabs.
func TestTabsEqual(t *testing.T) {
	headless(t)
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		value := core.NewSignal(cx, "a")
		return Tabs[string]{TestID: "tabs", Equal: true, Value: value.Get, OnChange: value.Set,
			Options: []Option[string]{{Value: "a", Label: "A", TestID: "a"}, {Value: "b", Label: "A much longer tab", TestID: "b"}}}
	})
	list, a, b := app.Find("tabs"), app.Find("a"), app.Find("b")
	if a.Frame.Size.Width != b.Frame.Size.Width {
		t.Fatalf("tabs %v and %v wide", a.Frame.Size.Width, b.Frame.Size.Width)
	}
	if want := 2*a.Frame.Size.Width + 2 + 2*2; list.Frame.Size.Width != want {
		t.Fatalf("list %v wide, want %v", list.Frame.Size.Width, want)
	}
}

// TestTabsContent shows an option's content in place of its label, which
// still names the tab.
func TestTabsContent(t *testing.T) {
	headless(t)
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return Tabs[int]{Options: []Option[int]{{Label: "Inbox", TestID: "inbox", Content: core.View{TestID: "slot", Style: core.Style{Width: core.Pt(8), Height: core.Pt(8)}}}}}
	})
	tab := app.Find("inbox")
	if len(tab.Children) != 1 || tab.Children[0] != app.Find("slot") || tab.Props.Label != "Inbox" {
		t.Fatalf("tab %+v", tab.Props)
	}
}

// TestTabsSlide slides the indicator on a change of tab alone.
func TestTabsSlide(t *testing.T) {
	headless(t)
	app, _ := start(t, desktopEnv, tabsContent)
	thumb := func() *nsgotest.View { return app.Find("segmented").Children[0] }
	if d := thumb().Style.Transition.Duration; d != 0 {
		t.Fatalf("the first placement slides for %v", d)
	}
	app.Press("segmented-1")
	if d := thumb().Style.Transition.Duration; d != tabsSlide.Duration {
		t.Fatalf("a change of tab slides for %v", d)
	}
	app.Press("segmented-0")
	checkIndicator(t, app, "segmented", "segmented-0")
}

// TestTabsRTL keeps the indicator on the selected tab right to left.
func TestTabsRTL(t *testing.T) {
	e := desktopEnv
	e.RTL = true
	app, _ := start(t, e, tabsContent)
	if app.Find("segmented-1").Frame.Origin.X >= app.Find("segmented-0").Frame.Origin.X {
		t.Fatal("the tabs are not laid out right to left")
	}
	for _, name := range []string{"segmented", "underline"} {
		checkIndicator(t, app, name, name+"-0")
		app.Press(name + "-1")
		checkIndicator(t, app, name, name+"-1")
	}
}
