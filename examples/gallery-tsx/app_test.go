package main

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// names are the demos, in the sidebar's order.
var names = []string{"Button", "Link", "Toolbar", "Input", "Checkbox", "Select", "DatePicker", "ColorPicker", "Slider", "Badge", "Avatar", "Image",
	"Carousel", "List", "Table", "DataTable", "CommitGraph", "Plot", "Accordion", "Resizable", "Dock", "Progress", "Toast", "Dialog", "Popover",
	"Command", "Menu", "Tabs", "Breadcrumbs", "Sidebar", "NavStack", "Editor", "DiffView", "TextView"}

// open starts the gallery on the demo named name.
func open(t *testing.T, name string) *nsgotest.App {
	t.Helper()
	app := nsgotest.Start(t, Root, nsgotest.Size(1080, 760))
	app.Press("id:nav-" + name)
	app.WaitFor("id:demo-"+name, 5*time.Second)
	return app
}

func has(t *testing.T, app *nsgotest.App, texts ...string) {
	t.Helper()
	got := app.Texts()
	for _, want := range texts {
		if !slices.Contains(got, want) {
			t.Errorf("no %q among %q", want, got)
		}
	}
}

// TestEveryDemoOpens shows each demo, then each again in dark.
func TestEveryDemoOpens(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(1080, 760))
	for range 2 {
		for _, name := range names {
			app.Press("id:nav-" + name)
			app.WaitFor("id:demo-"+name, 5*time.Second)
			if got := app.Find("id:title").Text; got != name {
				t.Errorf("the title says %q on %s", got, name)
			}
		}
		app.Press("id:appearance")
	}
}

func TestButtonReports(t *testing.T) {
	app := open(t, "Button")
	app.Press("id:stage-Button >> text:accent")
	has(t, app, "Pressed accent")
}

func TestInput(t *testing.T) {
	app := open(t, "Input")
	app.Type("id:input", "hello")
	if got := app.Find("id:input-value").Text; got != "hello" {
		t.Errorf("the value reads %q", got)
	}
}

func TestSelect(t *testing.T) {
	app := open(t, "Select")
	if got := app.Find("id:select-value").Text; got != "banana" {
		t.Errorf("the value starts %q", got)
	}
	app.Press("id:select")
	app.Press("text:Mango")
	if got := app.Find("id:select-value").Text; got != "mango" {
		t.Errorf("the value reads %q", got)
	}
}

func TestListSelects(t *testing.T) {
	app := open(t, "List")
	app.Press("text:Task 2")
	if got := app.Find("id:list-selected").Text; got != "task-1" {
		t.Errorf("selected reads %q", got)
	}
}

func TestTreeExpands(t *testing.T) {
	app := open(t, "List")
	if app.Exists("text:app >> nth=1") || app.Exists("text:guide.md") {
		t.Fatal("docs starts open")
	}
	has(t, app, "app", "docs", "README.md")
}

// TestLabelsShow keeps a selection control's text where no children are given.
func TestLabelsShow(t *testing.T) {
	app := open(t, "Checkbox")
	has(t, app, "Select all", "Receipts", "email")
}

// TestDatesCross picks a day, which reaches TSX as an ISO date.
func TestDatesCross(t *testing.T) {
	app := open(t, "DatePicker")
	app.Press("id:card-Calendar >> text:15")
	has(t, app, "2026-11-15")
}

// TestRolesAreClasses reads a role ui's theme adds through a class.
func TestRolesAreClasses(t *testing.T) {
	app := open(t, "Carousel")
	if bg := app.Find("text:Mountains").Parent.Style.Background; bg == (core.Color(0)) {
		t.Error("bg-inset paints nothing")
	}
}

func TestDialog(t *testing.T) {
	app := open(t, "Dialog")
	app.Press("text:Open a dialog")
	has(t, app, "Rename the project")
	app.Press("text:Save")
	app.Advance(time.Second)
	app.WaitForGone("text:Rename the project", 2*time.Second)
	has(t, app, "Saved")
}

func TestToast(t *testing.T) {
	app := open(t, "Toast")
	app.Press("text:Show a toast")
	app.WaitFor("text:Toast 1", 2*time.Second)
	app.Press("text:Dismiss all")
	app.Advance(time.Second)
	if app.Exists("text:Toast 1") {
		t.Error("the toast stays")
	}
}

func TestMenu(t *testing.T) {
	app := open(t, "Menu")
	app.Press("text:File")
	app.Press("text:New")
	has(t, app, "New")
}

func TestCommand(t *testing.T) {
	app := open(t, "Command")
	app.Press("text:New file")
	has(t, app, "Ran new")
}

func TestDataTableSorts(t *testing.T) {
	app := open(t, "DataTable")
	has(t, app, "T0000")
	app.Press("id:sort:price")
	if got := app.Find("id:table-sort").Text; got != "price ascending" {
		t.Errorf("the sort reads %q", got)
	}
}

func TestNavStack(t *testing.T) {
	app := open(t, "NavStack")
	app.Press("text:Push")
	app.WaitFor("id:page-2", 3*time.Second)
}

func TestAppearance(t *testing.T) {
	app := open(t, "Badge")
	light := app.Find("id:demo-Badge").Style.Background
	app.Press("id:appearance")
	if dark := app.Find("id:demo-Badge").Style.Background; dark == light || dark == (core.Color(0)) {
		t.Errorf("the page is %#x in light and %#x in dark", light, dark)
	}
}

func TestNarrowPushesDemos(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 800))
	app.Press("id:nav-Badge")
	app.WaitFor("id:demo-Badge", 3*time.Second)
	has(t, app, "accent")
}
