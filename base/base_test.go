package base_test

import (
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

func TestPopupsNavAndAlerts(t *testing.T) {
	var reasons []string
	var chosen string
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		nav := NewNavHelper(cx)
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			base.Menu{TestID: "menu", Trigger: core.Text{Content: "File"}, PopupStyle: core.Style{Width: core.Pt(100)},
				OnOpenChange: func(open bool, r base.Reason) { reasons = append(reasons, r.String()) },
				Children: []core.Node{
					base.MenuItem{Label: "Open", Style: core.Style{Height: core.Pt(20)}, OnSelect: func() { nav.Push("detail") }},
					base.MenuItem{Label: "Quit", Style: core.Style{Height: core.Pt(20)}, OnSelect: func() {
						base.ShowAlertDialog(cx, base.AlertDialog{Title: "Quit?", Actions: []core.AlertAction{
							{Label: "Cancel", Role: core.AlertCancel}, {Label: "Quit", Role: core.AlertDestructive}}},
							func(label string) { chosen = label })
					}},
				}},
			nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, _ func() int) base.NavPage {
				return base.NavPage{Title: route, TestID: "page-" + route,
					Children: []core.Node{core.Text{Content: "page " + route}}}
			}),
		}}
	})
	app.Press("text:File")
	app.Press("label:Open")
	app.Find("text:page detail")
	if app.Exists("label:Open") {
		t.Fatal("choosing an item left the menu open")
	}
	app.Back()
	app.Find("text:page home")
	if app.Exists("text:page detail") {
		t.Fatal("back did not pop the page")
	}

	app.Press("text:File")
	app.Press("label:Quit")
	if alerts := app.Alerts(); len(alerts) != 1 || alerts[0].Title != "Quit?" {
		t.Fatalf("alerts = %+v", alerts)
	}
	app.Answer(1)
	if chosen != "Quit" {
		t.Fatalf("chosen = %q", chosen)
	}
	want := []string{"triggerPress", "select", "triggerPress", "select"}
	if len(reasons) != len(want) {
		t.Fatalf("reasons = %v, want %v", reasons, want)
	}
}

func NewNavHelper(cx *core.Scope) *base.Nav[string] { return base.NewNav(cx, "home") }

func TestPopupsReportChangesAndEveryCancel(t *testing.T) {
	var log []string
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignStart}, Children: []core.Node{
			base.Popover{TestID: "pop", Trigger: core.Text{Content: "Pop"}, DefaultOpen: true,
				NoCloseOnEscape: true, NoCloseOnPressOutside: true,
				OnOpenChange: func(open bool, r base.Reason) { log = append(log, "change "+r.String()) },
				OnCancel:     func() { log = append(log, "cancel") },
				PopupStyle:   core.Style{Width: core.Pt(20), Height: core.Pt(20)},
				Children:     []core.Node{core.Text{Content: "inside"}}},
			base.Tooltip{TestID: "tip", Trigger: core.Text{Content: "Tip"},
				OnOpenChange: func(open bool, r base.Reason) { log = append(log, "tooltip "+r.String()) }},
			base.Menu{TestID: "menu", Trigger: core.Text{Content: "Menu"}, Style: core.Style{Width: core.Pt(200)},
				PopupStyle: core.Style{Width: core.Pt(50)},
				Children:   []core.Node{base.MenuItem{Label: "Item", TestID: "item", Style: core.Style{Height: core.Pt(20)}}}},
		}}
	})
	app.Back()
	app.Hover("id:tip")
	app.HoverOut()
	// Pressing the menu's trigger presses outside the popover.
	app.Press("text:Menu")
	if want := []string{"cancel", "cancel"}; !slices.Equal(log, want) {
		t.Fatalf("log = %q, want both attempts to close cancelled and no requests that change nothing", log)
	}
	app.Find("text:inside")
	menu, popup := app.Find("id:menu"), app.Find("id:item").Parent
	if popup.Frame.Origin.X != menu.Frame.Origin.X {
		t.Errorf("menu at x %v, want at its trigger's start %v", popup.Frame.Origin.X, menu.Frame.Origin.X)
	}
}

// startDesktop runs app as on a desktop, where popups follow the pointer.
func startDesktop(t *testing.T, app func(cx *core.Scope) core.Node) *nsgotest.App {
	return nsgotest.Start(t, app, nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop}))
}

// Rows of years fill a body's height, and a short page keeps the empty ones
// so that its years stay the size of a full page's.
func TestCalendarShortPageKeepsItsRows(t *testing.T) {
	day := base.NewDate(2001, 2, 3, base.TimeOfDay{})
	app := startDesktop(t, func(*core.Scope) core.Node {
		return base.Calendar{TestID: "calendar", DefaultValue: base.DateValue{Start: day},
			CalendarConfig: base.CalendarConfig{YearRange: [2]int{2000, 2003}, ItemStyle: core.Style{MinHeight: core.Pt(30)},
				BodyStyle: core.Style{Height: core.Pt(210)}}}
	})
	high := app.Find("id:calendar").Frame.Size.Height
	app.Press("label:2001")
	if got := app.Find("id:calendar").Frame.Size.Height; got != high {
		t.Fatalf("years are %v high, days %v", got, high)
	}
	if got := app.Find("label:2000").Frame.Size.Height; got < 52 || got > 53 {
		t.Fatalf("a year of four rows of 210 is %v high", got)
	}
}
