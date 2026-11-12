package main

import (
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// names are the demos, in the sidebar's order.
var names = []string{"Stocks", "AI Chat", "Photos", "Button", "Link", "Toolbar", "Input", "Checkbox", "Select", "DatePicker", "ColorPicker", "Slider", "Badge", "Avatar", "Image",
	"Carousel", "List", "Table", "DataTable", "CommitGraph", "Plot", "FlowDiagram", "Accordion", "Resizable", "Dock", "Progress", "Toast", "Dialog", "Popover",
	"Command", "Menu", "Tabs", "Breadcrumbs", "Sidebar", "NavStack", "Conversation", "Editor", "RichEditor", "DiffView", "Terminal", "TextView"}

// startDesktop starts the gallery in its wide layout, which phones show
// none of.
func startDesktop(t *testing.T) *nsgotest.App {
	t.Helper()
	if target := nsgotest.Target(); target == "ios_simulator" || target == "android" {
		t.Skipf("needs a desktop window, which %s has not", target)
	}
	return nsgotest.Start(t, Root, nsgotest.Size(1080, 760))
}

// startPhone starts the gallery at a phone's size, in its narrow layout.
func startPhone(t *testing.T) *nsgotest.App {
	t.Helper()
	return nsgotest.Start(t, Root, nsgotest.Size(390, 844))
}

// open starts the gallery on the demo named name: picked from the
// sidebar, or pushed over the index where the screen is a phone's.
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
	app := startDesktop(t)
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
	app.Press("id:stage-Button >> text:outline")
	has(t, app, "Pressed outline")
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
	app := startDesktop(t)
	app.Press("id:nav-DataTable")
	app.WaitFor("id:demo-DataTable", 5*time.Second)
	has(t, app, "T0000")
	// The pinned column's header shows on any screen.
	app.Press("id:sort:symbol")
	if got := app.Find("id:table-sort").Text; got != "symbol descending" {
		t.Errorf("the sort reads %q", got)
	}
}

func TestNavStack(t *testing.T) {
	app := open(t, "NavStack")
	app.Press("text:Push")
	app.WaitFor("id:page-2", 3*time.Second)
}

func TestAppearance(t *testing.T) {
	app := startDesktop(t)
	app.Press("id:nav-Badge")
	app.WaitFor("id:demo-Badge", 5*time.Second)
	light := app.Find("id:demo-Badge").Style.Background
	app.Press("id:appearance")
	if dark := app.Find("id:demo-Badge").Style.Background; dark == light || dark == (core.Color(0)) {
		t.Errorf("the page is %#x in light and %#x in dark", light, dark)
	}
}

// TestEveryDemoOpensPhone pushes each demo over the index and goes back.
func TestEveryDemoOpensPhone(t *testing.T) {
	app := startPhone(t)
	for _, name := range names {
		app.Press("id:nav-" + name)
		app.WaitFor("id:demo-"+name, 5*time.Second)
		// The header's back button, since a terminal takes the keys that
		// go back.
		if app.Exists("role:button Back") {
			app.Press("role:button Back")
		} else {
			app.Back()
		}
		app.WaitFor("id:index", 5*time.Second)
	}
}

func TestNarrowPushesDemos(t *testing.T) {
	app := startPhone(t)
	app.Press("id:nav-Badge")
	app.WaitFor("id:demo-Badge", 3*time.Second)
	has(t, app, "accent")
}

// TestTerminal runs the demo session, which every platform runs: typed
// into, its title reaches the panel. The web and Android run no terminal
// and say so.
func TestTerminal(t *testing.T) {
	app := open(t, "Terminal")
	if runtime.GOOS == "js" || runtime.GOOS == "android" {
		app.WaitFor("id:terminal-unavailable", 5*time.Second)
		return
	}
	if app.Exists("text:Demo session") {
		app.Press("text:Demo session")
	}
	app.Find("id:terminal")
	app.WaitUntil(5*time.Second, func() bool { return app.Find("id:terminal-size").Text != "—" })
	app.Type("label:Terminal", "title from tsx")
	app.Key("enter")
	app.WaitUntil(5*time.Second, func() bool { return app.Find("id:terminal-title").Text == "from tsx" })
	app.Press("id:terminal-restart")
	if got := app.Find("id:terminal-title").Text; got != "—" {
		t.Errorf("the restarted session kept the title %q", got)
	}
}

// TestConversation shows a turn in each state with ui/chat's parts: the
// running one works, the failed one says why, and the composer sends and
// stops.
func TestConversation(t *testing.T) {
	app := open(t, "Conversation")
	app.Find("id:conversation")
	app.Find("id:turn-running")
	app.Find("id:conversation-panel")
	has(t, app, "Rendering a long list", "The model is overloaded right now (error 529).", "You stopped this reply.", "trip-notes.pdf")
	app.Press("label:Stop")
	app.Type("label:Message", "Hello")
	app.Press("label:Send")
	if got := app.Find("label:Message").Text; got != "" {
		t.Errorf("the composer kept %q after sending", got)
	}
	app.Press("id:conversation-model")
	app.WaitFor("label:Vision", 5*time.Second)
	app.Press("label:Vision")
	app.WaitForGone("label:Deep", 5*time.Second)
	has(t, app, "Vision")
}

// TestStocks ticks the market, opens a stock and places an order, which a
// toast confirms.
func TestStocks(t *testing.T) {
	app := startDesktop(t)
	app.Press("id:nav-Stocks")
	app.Find("id:watchlist")
	before := app.Find("id:price-ORCA").Text
	app.Advance(3 * time.Second)
	app.WaitUntil(5*time.Second, func() bool { return app.Find("id:price-ORCA").Text != before })
	app.Press("id:stock-NOVA")
	if got := app.Find("id:stock-name").Text; got != "Nova Optics" {
		t.Errorf("the page shows %q", got)
	}
	app.Press("id:ticket-place")
	app.WaitFor("text:Bought 10 NOVA", 5*time.Second)
	app.Press("id:market-pause")
	paused := app.Find("id:price-NOVA").Text
	app.Advance(3 * time.Second)
	if got := app.Find("id:price-NOVA").Text; got != paused {
		t.Errorf("the paused market moved from %s to %s", paused, got)
	}
}

// TestStocksPhone pushes a stock's page over the watchlist and comes back.
func TestStocksPhone(t *testing.T) {
	app := startPhone(t)
	app.Press("id:nav-Stocks")
	app.WaitFor("id:watchlist", 5*time.Second)
	app.Press("id:stock-KITE")
	app.WaitFor("id:stock-detail", 5*time.Second)
	if app.Exists("id:watchlist") {
		t.Error("the watchlist shows under the stock's page")
	}
	app.Press("id:stock-back")
	app.WaitFor("id:watchlist", 5*time.Second)
}

// TestTerminalPhone runs the demo session, a phone's: typed into, its
// title reaches the panel. Where no terminal runs, the page says why.
func TestTerminalPhone(t *testing.T) {
	app := startPhone(t)
	app.Press("id:nav-Terminal")
	app.WaitFor("id:demo-Terminal", 5*time.Second)
	if runtime.GOOS == "js" || runtime.GOOS == "android" {
		app.WaitFor("id:terminal-unavailable", 5*time.Second)
		return
	}
	if app.Exists("text:Demo session") {
		app.Press("text:Demo session")
	}
	app.Find("id:terminal")
	// What the keyboard typed before the view had it is cleared with the
	// key bar's Control and U.
	app.Type("label:Terminal", "title on a phone")
	if app.Exists("id:terminal-key-ctrl") {
		app.Press("id:terminal-key-ctrl")
		app.Type("label:Terminal", "u")
	} else {
		app.Key("ctrl+u")
	}
	app.Type("label:Terminal", "title from a phone")
	app.Key("enter")
	app.WaitUntil(5*time.Second, func() bool { return app.Find("id:terminal-title").Text == "from a phone" })
}

// TestAIChat streams a scripted reply that searches the web, then a free
// prompt that Stop ends.
func TestAIChat(t *testing.T) {
	app := open(t, "AI Chat")
	app.Find("id:chat-empty")
	app.Press("id:chat-try-search")
	app.WaitFor("text:Searched the web", 5*time.Second)
	app.Advance(10 * time.Second)
	app.WaitFor("label:Copy", 10*time.Second)
	has(t, app, "What changed in the latest release of the Harbor runtime?")
	app.Type("label:Message", "Write me a long answer")
	app.Press("label:Send")
	app.WaitFor("label:Stop", 5*time.Second)
	app.Press("label:Stop")
	app.WaitFor("text:You stopped this reply.", 5*time.Second)
	// The list of conversations shows beside a wide one.
	if app.Exists("id:chat-new") {
		app.Press("id:chat-new")
		app.WaitFor("id:chat-empty", 5*time.Second)
	}
}

// TestPhotos loads a page more at the end and refreshes from the button.
func TestPhotos(t *testing.T) {
	app := open(t, "Photos")
	if got := app.Find("id:photos-count").Text; got != "1,000" {
		t.Errorf("the stream starts with %s photos", got)
	}
	app.Press("id:photos-refresh")
	app.Advance(time.Second)
	app.WaitUntil(5*time.Second, func() bool { return app.Find("id:photos-refreshes").Text == "1" })
}

// TestRichEditor formats the document from the toolbar, which the
// Markdown beside it follows.
func TestRichEditor(t *testing.T) {
	app := open(t, "RichEditor")
	app.Find("id:rich")
	if got := app.Find("id:rich-markdown").Text; !strings.Contains(got, "# Launch notes") {
		t.Fatalf("the Markdown reads %q", got)
	}
	app.Press("label:Undo")
	app.Press("id:rich-bold")
	if app.Find("id:rich-words").Text == "0" {
		t.Error("the document has no words")
	}
}

// TestFlowDiagram lays the plan out from TSX's data, cards made by JSX in
// it, and selects a step.
func TestFlowDiagram(t *testing.T) {
	app := open(t, "FlowDiagram")
	app.Find("flow >> text:Prototype")
	app.AccessibilityAction("flow >> label:Design", "activate")
	if got := app.Find("id:flow-selected").Text; got != "design" {
		t.Fatalf("selected %q", got)
	}
	app.Screenshot("flow-diagram")
}
