package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TestToggleGroupAndToolbar put toggle buttons in one stop of the focus.
func TestToggleGroupAndToolbar(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var bold *core.Signal[bool]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			bold = core.NewSignal(cx, false)
			return Toolbar{TestID: "bar", Label: "Format", Children: []core.Node{
				ToggleGroup{TestID: "marks", Children: []core.Node{
					ToggleButton{Text: "Bold", TestID: "bold", On: bold.Get, OnChange: bold.Set},
					ToggleButton{Text: "Italic", TestID: "italic"},
				}},
				ToolbarSeparator{TestID: "line"},
				ToolbarGroup{Label: "More", TestID: "more", Children: []core.Node{Link{Text: "Help", Href: "/help", OpenWith: base.LinkOpenApp, TestID: "help"}}},
			}}
		})
		if app.Find("bar").Props.Role != core.RoleToolbar || app.Find("marks").Props.Role != core.RoleGroup {
			t.Fatal("a toggle group in a toolbar is one of its groups")
		}
		if !app.Find("bold").Props.Focusable || app.Find("italic").Props.Focusable || app.Find("help").Props.Focusable {
			t.Fatal("the toolbar is not one stop of the focus")
		}
		app.Press("bold")
		if !bold.Get() || !app.Find("bold").Props.Pressed {
			t.Fatal("a press does not turn the toggle on")
		}
	})
}

// TestLink reports where it goes, then the press.
func TestLink(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var log []string
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return Link{Text: "Docs", Href: "/docs", OpenWith: base.LinkOpenApp, TestID: "docs",
				OnOpen: func(href string) { log = append(log, href) }, OnPress: func() { log = append(log, "press") }}
		})
		app.Press("docs")
		if len(log) != 2 || log[0] != "/docs" || log[1] != "press" || app.Find("docs").Props.Role != core.RoleLink {
			t.Fatalf("log = %v", log)
		}
	})
}

// TestPagination moves between pages, the current one selected.
func TestPagination(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var page *core.Signal[int]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			page = core.NewSignal(cx, 1)
			return Pagination{Page: page.Get, TotalPages: func() int { return 20 }, OnChange: page.Set, Label: "Pages",
				PreviousLabel: "Previous", NextLabel: "Next", TestID: "pages"}
		})
		if !app.Find("label:Previous").Props.Disabled {
			t.Fatal("Previous is not disabled on the first page")
		}
		app.Press("label:Next")
		if page.Get() != 2 || app.Find("label:Previous").Props.Disabled {
			t.Fatalf("page = %d after Next", page.Get())
		}
		app.Press(`text:"20"`)
		if page.Get() != 20 || !app.Find("label:Next").Props.Disabled {
			t.Fatalf("page = %d after the last page", page.Get())
		}
	})
}

// TestOtpInput keeps digits up to its length and reports the full code.
func TestOtpInput(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var code, done string
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return OtpInput{Length: 4, Groups: 2, Label: "Code", OnChange: func(v string) { code = v }, OnComplete: func(v string) { done = v }}
		})
		app.Type("label:Code", "12a345")
		if code != "1234" || done != "1234" {
			t.Fatalf("code = %q, done = %q", code, done)
		}
	})
}

// TestScrollbarTheme replaces the indicators, and the view's own bar goes
// with the theme.
func TestScrollbarTheme(t *testing.T) {
	headless(t)
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			base.SetScrollbarTheme(cx, ScrollbarTheme(cx))
			return core.Scroll{TestID: "list", Style: core.Style{Height: core.Pt(100), Width: core.Pt(100)},
				Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(800)}}}}
		})
		if s := app.Find("list").Scroll; !s.HideIndicator {
			t.Fatal("the theme does not replace the platform's indicator")
		}
	})
}
