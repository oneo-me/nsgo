package base_test

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// A dialog with an ending part stays until its transition is over, and comes back from where it is when opened again.
func TestDialogGoesThroughItsMotion(t *testing.T) {
	var open *core.Signal[bool]
	closed := 0
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		open = core.NewSignal(cx, true)
		return base.Dialog{Open: open.Get, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, OnClose: func() { closed++ },
			Animation: core.ModalNone, BackdropStyle: core.Style{Background: core.RGBA(0, 0.3)}, BackdropTestID: "backdrop",
			BackdropEndingStyle: core.Style{Opacity: core.Alpha(0)}, TestID: "dialog",
			PopupStyle:  core.Style{Width: core.Pt(100), Height: core.Pt(100), Transition: core.Transition{Duration: 200 * time.Millisecond, Properties: core.TransitionOpacity}},
			EndingStyle: core.Style{Opacity: core.Alpha(0)},
			Children:    []core.Node{core.Text{Content: "inside"}}}
	})
	app.Settle()
	open.Set(false)
	app.Settle()
	if got := app.Find("id:dialog").Style.Opacity; got != 0 || app.Find("id:backdrop").Style.Opacity != 0 {
		t.Fatalf("dialog opacity = %v as it goes", got)
	}
	if closed != 0 {
		t.Fatal("OnClose ran before the dialog had gone")
	}
	open.Set(true)
	app.Settle()
	app.Advance(300 * time.Millisecond)
	if !app.Exists("id:dialog") || closed != 0 {
		t.Fatal("opened again as it went, the dialog left")
	}
	open.Set(false)
	app.Advance(100 * time.Millisecond)
	if !app.Exists("id:dialog") {
		t.Fatal("the dialog left before its transition ended")
	}
	app.Advance(150 * time.Millisecond)
	if app.Exists("id:dialog") || closed != 1 {
		t.Fatalf("after its transition: shown %v, %d closes", app.Exists("id:dialog"), closed)
	}
}

func TestSheetSlidesInAndReportsAfterItsMotion(t *testing.T) {
	var log []string
	var open *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		open = core.NewSignal(cx, false)
		return base.Sheet{Open: open.Get, CloseOnSwipe: true, TestID: "sheet",
			OnOpenChange:  func(o bool, r base.Reason) { log = append(log, r.String()); open.Set(o) },
			OnOpen:        func() { log = append(log, "open") },
			OnClose:       func() { log = append(log, "close") },
			OnCancel:      func() { log = append(log, "cancel") },
			BackdropStyle: core.Style{Background: core.RGBA(0, 0.3)},
			PopupStyle:    core.Style{Height: core.Pt(300)},
			Children:      []core.Node{core.Text{Content: "inside"}}}
	})
	open.Set(true)
	app.Settle()
	panel := app.Find("id:sheet")
	if got := panel.Animated.TranslateY; got != 300 {
		t.Errorf("the panel starts %v below its place, want its height", got)
	}
	if len(log) != 0 {
		t.Errorf("log = %q before the motion ended", log)
	}
	app.Advance(300 * time.Millisecond)
	if got := app.Find("id:sheet").Animated.TranslateY; got != 0 {
		t.Errorf("the panel is %v below its place after coming in", got)
	}
	if backdrop := app.Find("id:sheet").Parent.Children[0]; backdrop.Animated.Opacity != 1 {
		t.Errorf("the backdrop's opacity is %v", backdrop.Animated.Opacity)
	}
	if !slices.Equal(log, []string{"open"}) {
		t.Errorf("log = %q, want open once in", log)
	}
	// A short drag comes back; a long one closes it.
	app.Pan("id:sheet", core.Point{Y: 20}, core.Point{Y: 50})
	app.Advance(300 * time.Millisecond)
	if !app.Exists("text:inside") || len(log) != 1 {
		t.Fatalf("a short drag: log %q", log)
	}
	app.Pan("id:sheet", core.Point{Y: 60}, core.Point{Y: 120})
	if want := []string{"open", "cancel", "swipe"}; !slices.Equal(log, want) {
		t.Errorf("log = %q, want %q", log, want)
	}
	if !app.Exists("text:inside") {
		t.Error("the sheet left before its motion out")
	}
	app.Advance(250 * time.Millisecond)
	if app.Exists("text:inside") || log[len(log)-1] != "close" {
		t.Errorf("after the motion: log %q", log)
	}
}

func TestMenusOnTouchScreensAreSheetsWithLevels(t *testing.T) {
	var picked string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		item := func(label string) core.Node {
			return base.MenuItem{Label: label, OnSelect: func() { picked = label },
				Children: []core.Node{core.Text{Content: label}}}
		}
		return base.Menu{Trigger: core.Text{Content: "More"}, Label: "More", TestID: "menu",
			Children: []core.Node{
				item("Copy"),
				base.Submenu{Label: "Share", Trigger: core.Text{Content: "Share"},
					Back: core.Text{Content: "‹ Back"}, Children: []core.Node{item("Mail")}},
			}}
	})
	app.Press("label:More")
	app.Advance(300 * time.Millisecond)
	if app.Exists("text:Mail") || !app.Exists("text:Copy") {
		t.Fatal("the sheet shows more than its first level")
	}
	app.Press("label:Share")
	if app.Exists("text:Copy") || !app.Exists("text:Mail") || !app.Exists("text:‹ Back") {
		t.Fatal("the submenu does not show in place of its menu's items")
	}
	app.Back()
	if !app.Exists("text:Copy") || app.Exists("text:Mail") {
		t.Fatal("back did not return a level")
	}
	app.Press("label:Share")
	app.Press("text:‹ Back")
	app.Press("label:Share")
	app.Press("label:Mail")
	app.Advance(250 * time.Millisecond)
	if picked != "Mail" || app.Exists("text:Copy") {
		t.Errorf("picked %q, menu still shown %v", picked, app.Exists("text:Copy"))
	}
}

func TestContextMenusOpenAtThePointerOrAfterAHold(t *testing.T) {
	var reasons []string
	menu := func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			base.ContextMenu{TestID: "area", Style: core.Style{Height: core.Pt(200)},
				OnOpenChange: func(_ bool, r base.Reason) { reasons = append(reasons, r.String()) },
				PopupStyle:   core.Style{Width: core.Pt(80)},
				Children:     []core.Node{base.MenuItem{Label: "Inspect", TestID: "inspect", Style: core.Style{Height: core.Pt(20)}}}},
		}}
	}
	app := startDesktop(t, menu)
	app.RightClick("id:area")
	popup := app.Find("id:inspect").Parent
	if got := popup.Frame.Origin; got.Y != 100 {
		t.Errorf("popup at %v, want below the pointer at the area's middle", got)
	}

	phone := nsgotest.Start(t, menu)
	reasons = nil
	phone.Drag("id:area")
	phone.Advance(time.Second)
	if phone.Exists("label:Inspect") || len(reasons) != 0 {
		t.Fatalf("a tap opened the menu: %v", reasons)
	}
	// A finger moving away is no hold.
	phone.Drag("id:area", core.Point{Y: 30})
	phone.Advance(time.Second)
	if len(reasons) != 0 {
		t.Fatalf("a drag opened the menu: %v", reasons)
	}
}

func TestPopoversOnTouchScreensTakeThePressOutside(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.Popover{Trigger: core.Text{Content: "Pop"}, Label: "Pop", DefaultOpen: true,
			Children: []core.Node{core.Text{Content: "inside"}}}
	})
	root := app.Find("label:Pop").Parent
	backdrop := root.Children[1]
	if backdrop.Frame != root.Frame || len(backdrop.Children) != 1 {
		t.Fatalf("the backdrop covers %v of %v", backdrop.Frame, root.Frame)
	}
	if backdrop.Children[0].Events == 0 {
		t.Error("the backdrop takes no presses")
	}
}

func TestDesktopPopupsComeInAndGoThroughTheirPhases(t *testing.T) {
	var log []string
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		return base.Popover{Trigger: core.Text{Content: "Pop"}, Label: "Pop",
			OnOpen: func() { log = append(log, "open") }, OnClose: func() { log = append(log, "close") },
			PopupClass: "transition-opacity duration-200", StartingClass: "opacity-0", EndingClass: "opacity-0",
			Children: []core.Node{core.Text{Content: "inside"}}}
	})
	opacity := func() float32 { return app.Find("text:inside").Parent.Style.Opacity }
	app.Press("label:Pop")
	if got := opacity(); got != 0 {
		t.Errorf("the popup starts at opacity %v", got)
	}
	app.Advance(60 * time.Millisecond)
	if got := opacity(); got != 1 || !slices.Equal(log, []string{"open"}) {
		t.Errorf("past its first frame: opacity %v, log %q", got, log)
	}
	app.Press("label:Pop")
	if got := opacity(); got != 0 || len(log) != 1 {
		t.Errorf("closing: opacity %v, log %q", got, log)
	}
	app.Advance(199 * time.Millisecond)
	if !app.Exists("text:inside") {
		t.Fatal("the popup left before its transition ended")
	}
	app.Advance(time.Millisecond)
	if app.Exists("text:inside") || !slices.Equal(log, []string{"open", "close"}) {
		t.Errorf("after its transition: log %q", log)
	}
	// Opened again on its way out, it stays.
	app.Press("label:Pop")
	app.Advance(60 * time.Millisecond)
	app.Press("label:Pop")
	app.Advance(100 * time.Millisecond)
	app.Press("label:Pop")
	app.Advance(time.Second)
	if !app.Exists("text:inside") || opacity() != 1 {
		t.Error("a popup opened again on its way out left")
	}
}

func TestDesktopPopupsGoAtOnceWithReducedMotion(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.Menu{Trigger: core.Text{Content: "More"}, Label: "More",
			PopupClass: "transition-opacity duration-200", StartingClass: "opacity-0", EndingClass: "opacity-0",
			Children: []core.Node{base.MenuItem{Label: "Copy", Children: []core.Node{core.Text{Content: "Copy"}}}}}
	}, nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop, ReduceMotion: true}))
	app.Press("label:More")
	if got := app.Find("text:Copy").Parent.Parent.Style.Opacity; got != 1 {
		t.Errorf("the menu starts at opacity %v", got)
	}
	app.Press("label:Copy")
	if app.Exists("text:Copy") {
		t.Error("the menu stayed to go")
	}
}

func TestViewTooltipsShowOnHoverOnDesktops(t *testing.T) {
	app := startDesktop(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(40)}, Children: []core.Node{
			core.View{TestID: "target", Style: core.Style{Width: core.Pt(50), Height: core.Pt(20)},
				Tooltip: core.Text{Content: "a hint", TestID: "hint"}},
		}}
	})
	app.Hover("id:target")
	if app.Exists("id:hint") {
		t.Fatal("the tooltip showed at once")
	}
	app.Advance(600 * time.Millisecond)
	hint := app.Find("id:hint")
	if pe := hint.Parent.Props.PointerEvents; pe != core.PointerEventsNone {
		t.Errorf("the tooltip takes the pointer (%v)", pe)
	}
	if hint.Parent.Frame.Origin.Y+hint.Parent.Frame.Size.Height > 40 {
		t.Errorf("the tooltip at %v is not above its view", hint.Parent.Frame)
	}
	app.HoverOut()
	if app.Exists("id:hint") {
		t.Error("the tooltip stayed after the pointer left")
	}
}

func TestAlertDialogsPresentAsActionSheets(t *testing.T) {
	var chosen = "unset"
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.Button{Label: "Share", Style: core.Style{Height: core.Pt(20)}, OnPress: func(core.PressEvent) {
			base.ShowAlertDialog(cx, base.AlertDialog{Title: "Share", Presentation: base.AlertPresentActionSheet,
				Actions: []core.AlertAction{{Label: "Save"}, {Label: "Cancel", Role: core.AlertCancel}}},
				func(label string) { chosen = label })
		}}
	})
	app.Press("label:Share")
	if a := app.Alerts(); len(a) != 1 || !a[0].Sheet || !a[0].Cancelable {
		t.Fatalf("alerts %+v, want a cancelable action sheet", a)
	}
	app.Answer(-1)
	if chosen != "" {
		t.Errorf("a dismissal chose %q", chosen)
	}
}

// A press on a dialog's blank space stays in it, off the backdrop under it.
func TestDialogKeepsPressesOffItsBackdrop(t *testing.T) {
	var reasons []base.Reason
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.Dialog{Open: func() bool { return true }, OnOpenChange: func(_ bool, r base.Reason) { reasons = append(reasons, r) },
			BackdropStyle: core.Style{Background: core.RGB(0)}, BackdropTestID: "backdrop", PopupStyle: core.Style{Width: core.Pt(200), Height: core.Pt(100)}, TestID: "dialog"}
	}, nsgotest.Size(400, 300))
	app.Press("id:dialog")
	if len(reasons) != 0 {
		t.Fatalf("the press closed the dialog: %v", reasons)
	}
	app.Press("id:backdrop")
	if len(reasons) != 1 || reasons[0] != base.ReasonBackdropPress {
		t.Fatalf("reasons = %v", reasons)
	}
}

// items of a list as a phone or a desktop shows them.
func fruit() []base.SelectItem {
	return []base.SelectItem{{Value: "apple", Title: "Apple"}, {Value: "pear", Title: "Pear", Disabled: true}, {Value: "plum", Title: "Plum"}}
}

// A desktop's select opens on its keys, moves past disabled items, chooses
// with Return and gives the keyboard back to its trigger.
func TestSelectKeysOnADesktop(t *testing.T) {
	var value *string
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		return base.Select{Items: fruit, OnChange: func(v *string) { value = v }, TestID: "root", Label: "Fruit", ItemClass: "focus:h-[40px]",
			Trigger: core.Text{Content: "Trigger"}}
	}, nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop, OS: core.OSMacOS}))
	app.Key("Tab")
	app.Key("down")
	if !app.Exists("text:Plum") {
		t.Fatal("Down did not open the list")
	}
	app.Key("down")
	app.Key("enter")
	if value == nil || *value != "plum" {
		t.Fatalf("value = %v, want plum, past the disabled pear", value)
	}
	if app.Exists("text:Plum") {
		t.Fatal("Return left the list open")
	}
}

// A menu's trigger joins the toolbar around it, and with one of the bar's
// menus open the pointer over another moves there.
func TestMenusInAToolbar(t *testing.T) {
	item := func(text string) []core.Node {
		return []core.Node{base.MenuItem{Label: text, Children: []core.Node{core.Text{Content: text}}}}
	}
	app := nsgotest.Start(t, func(*core.Scope) core.Node {
		return base.Toolbar{Children: []core.Node{
			base.Menu{Label: "File", Trigger: core.Text{Content: "FileMenu"}, Children: item("Open")},
			base.Menu{Label: "Edit", Trigger: core.Text{Content: "EditMenu"}, Children: item("Copy")},
		}}
	}, nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop, OS: core.OSMacOS}))
	app.Press("label:File")
	if !app.Exists("text:Open") {
		t.Fatal("the menu did not open")
	}
	app.Hover("label:Edit")
	app.Settle()
	if app.Exists("text:Open") || !app.Exists("text:Copy") {
		t.Fatal("the pointer over the other trigger did not move the menu there")
	}
}
