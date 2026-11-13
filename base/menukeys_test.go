package base_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// menuApp is a menu of n items, the third disabled and the last a
// submenu of two, in a window h high.
func menuApp(t *testing.T, n int, h core.Points, chosen *[]string) *nsgotest.App {
	return nsgotest.Start(t, func(cx *core.Scope) core.Node {
		var items []core.Node
		for i := range n - 1 {
			label := fmt.Sprint("Item ", i)
			items = append(items, base.MenuItem{Label: label, Disabled: func() bool { return i == 2 },
				OnSelect: func() { *chosen = append(*chosen, label) },
				Style:    core.Style{Height: core.Pt(30), Width: core.Pt(120)}})
		}
		items = append(items, base.Submenu{Label: "More", Style: core.Style{Height: core.Pt(30), Width: core.Pt(120)},
			Children: []core.Node{
				base.MenuItem{Label: "Sub A", OnSelect: func() { *chosen = append(*chosen, "Sub A") }, Style: core.Style{Height: core.Pt(30), Width: core.Pt(120)}},
				base.MenuItem{Label: "Sub B", OnSelect: func() { *chosen = append(*chosen, "Sub B") }, Style: core.Style{Height: core.Pt(30), Width: core.Pt(120)}},
			}})
		return core.View{Style: core.Style{Padding: core.All(10)}, Children: []core.Node{
			base.Menu{Label: "Menu", Trigger: core.Text{Content: "Menu"}, Style: core.Style{Height: core.Pt(20)},
				PopupStyle: core.Style{Padding: core.All(4)}, Children: items},
		}}
	}, nsgotest.Size(400, h), nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop, OS: core.OSMacOS}))
}

func menuFocus(app *nsgotest.App) string {
	for _, l := range []string{"Menu", "More", "Sub A", "Sub B", "Copy", "Paste"} {
		if app.Exists("label:"+l) && app.Find("label:"+l).Focused {
			return l
		}
	}
	for i := range 40 {
		l := fmt.Sprint("Item ", i)
		if app.Exists("label:"+l) && app.Find("label:"+l).Focused {
			return l
		}
	}
	return ""
}

// The keyboard opening a menu focuses its first item; Down and Up move
// through the enabled items around the ends, Home and End to the ends,
// Right and Left into a submenu and out, and Return chooses.
func TestMenuKeys(t *testing.T) {
	var chosen []string
	app := menuApp(t, 5, 600, &chosen)
	app.Key("tab")
	app.Key("enter")
	steps := []struct{ key, want string }{
		{"", "Item 0"}, {"down", "Item 1"}, {"down", "Item 3"}, {"down", "More"}, {"down", "Item 0"},
		{"up", "More"}, {"home", "Item 0"}, {"end", "More"}, {"right", "Sub A"}, {"down", "Sub B"},
		{"left", "More"},
	}
	for _, s := range steps {
		if s.key != "" {
			app.Key(s.key)
		}
		if got := menuFocus(app); got != s.want {
			t.Fatalf("after %q the focus is on %q, want %q", s.key, got, s.want)
		}
	}
	if app.Exists("label:Sub A") {
		t.Error("Left left the submenu open")
	}
	app.Key("up")
	app.Key("enter")
	if len(chosen) != 1 || chosen[0] != "Item 3" || app.Exists("label:Item 3") {
		t.Errorf("Return chose %v, the menu open: %v", chosen, app.Exists("label:Item 3"))
	}
	if menuFocus(app) != "Menu" {
		t.Errorf("the focus went to %q, not back to the trigger", menuFocus(app))
	}
}

// A menu a press opened focuses nothing, and the keys start at an end or
// at the item the pointer came over.
func TestMenuKeysAfterAPress(t *testing.T) {
	var chosen []string
	app := menuApp(t, 5, 600, &chosen)
	app.Press("label:Menu")
	if got := menuFocus(app); got != "" {
		t.Fatalf("a press focused %q", got)
	}
	app.Key("up")
	if got := menuFocus(app); got != "More" {
		t.Fatalf("Up after a press focused %q", got)
	}
	app.Hover("label:Item 1")
	app.Key("down")
	if got := menuFocus(app); got != "Item 3" {
		t.Fatalf("Down after hovering Item 1 focused %q", got)
	}
}

// A menu taller than the room goes where it fits whole, else on the side
// with more room held to it, its items scrolling; Page Down and Page Up go
// a view at a time, stopping at the ends.
func TestMenuHeldToItsRoom(t *testing.T) {
	var chosen []string
	app := menuApp(t, 30, 300, &chosen)
	app.Key("tab")
	app.Key("enter")
	menu := app.Find("role:menu")
	if b := menu.Frame; b.Origin.Y+b.Size.Height > 300 || b.Size.Height < 200 {
		t.Fatalf("the menu at %v is not held to the window", b)
	}
	app.Key("pagedown")
	first := menuFocus(app)
	app.Key("pagedown")
	second := menuFocus(app)
	if first == "Item 0" || first == second {
		t.Errorf("Page Down went to %q, then %q", first, second)
	}
	if y := menu.Children[0].Offset.Y; y <= 0 {
		t.Errorf("%q was not scrolled into view (offset %v)", second, y)
	}
	for range 20 {
		app.Key("pagedown")
	}
	if got := menuFocus(app); got != "More" {
		t.Errorf("Page Down stopped at %q, not the last item", got)
	}
	for range 20 {
		app.Key("pageup")
	}
	if got := menuFocus(app); got != "Item 0" {
		t.Errorf("Page Up stopped at %q, not the first item", got)
	}
}

// A menu whose trigger sits low opens above it, whole, where it fits
// there: the side is picked at its natural height, not the room below.
func TestMenuOpensWhereItFitsWhole(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		var items []core.Node
		for i := range 6 {
			items = append(items, base.MenuItem{Label: fmt.Sprint("Item ", i), Style: core.Style{Height: core.Pt(30), Width: core.Pt(120)}})
		}
		return core.View{Style: core.Style{Padding: core.Edges{Top: 300}}, Children: []core.Node{
			base.Menu{Label: "Menu", Trigger: core.Text{Content: "Menu"}, Style: core.Style{Height: core.Pt(20)}, Children: items},
		}}
	}, nsgotest.Size(400, 400), nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop, OS: core.OSMacOS}))
	app.Press("label:Menu")
	menu := app.Find("role:menu")
	if b := menu.Frame; b.Origin.Y+b.Size.Height > 300 || b.Size.Height < 180 {
		t.Errorf("the menu at %v is not whole above its trigger", b)
	}
}

// Escape in a submenu closes it alone, the focus back on its trigger;
// the keys go on in the menu.
func TestSubmenuEscapeGivesTheFocusBack(t *testing.T) {
	var chosen []string
	app := menuApp(t, 5, 600, &chosen)
	app.Key("tab")
	app.Key("enter")
	app.Key("end")
	app.Key("right")
	if got := menuFocus(app); got != "Sub A" {
		t.Fatalf("Right focused %q", got)
	}
	app.Key("escape")
	if app.Exists("label:Sub A") || !app.Exists("label:Item 0") {
		t.Fatal("Escape did not close the submenu alone")
	}
	if got := menuFocus(app); got != "More" {
		t.Errorf("the focus went to %q, not the submenu's trigger", got)
	}
}

// A menu on its way out takes no keys: a second Return chooses nothing
// again, the focus being back on the trigger.
func TestAMenuGoingTakesNoKeys(t *testing.T) {
	var chosen []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.Menu{Label: "Menu", Trigger: core.Text{Content: "Menu"}, Style: core.Style{Height: core.Pt(20)},
			EndingStyle: core.Style{Opacity: core.Alpha(0)},
			PopupStyle:  core.Style{Transition: core.Transition{Duration: 300 * time.Millisecond, Properties: core.TransitionOpacity}},
			Children: []core.Node{
				base.MenuItem{Label: "Copy", OnSelect: func() { chosen = append(chosen, "Copy") }, Style: core.Style{Height: core.Pt(30), Width: core.Pt(100)}},
				base.MenuItem{Label: "Paste", OnSelect: func() { chosen = append(chosen, "Paste") }, Style: core.Style{Height: core.Pt(30), Width: core.Pt(100)}},
			}}
	}, nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop, OS: core.OSMacOS}))
	app.Key("tab")
	app.Key("enter")
	app.Key("enter")
	if !app.Exists("label:Copy") {
		t.Fatal("the menu left before its exit")
	}
	app.Key("down")
	if menuFocus(app) != "Menu" {
		t.Errorf("Down on the way out moved the focus to %q", menuFocus(app))
	}
	app.Advance(time.Second)
	if len(chosen) != 1 || chosen[0] != "Copy" {
		t.Errorf("chosen %v, want Copy once", chosen)
	}
}
