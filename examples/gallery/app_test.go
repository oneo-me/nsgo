package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// headless skips a test that checks what only the headless model shows,
// such as the commands a real window would act on, under nsgo test
// -target.
func headless(t *testing.T) {
	if target := nsgotest.Target(); target != "" {
		t.Skipf("checks the headless model, not %s", target)
	}
}

var desktopEnv = core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, Locale: "en-US"}

// startDesktop runs the gallery in a wide desktop window; a phone has
// none.
func startDesktop(t *testing.T, opts ...nsgotest.Option) *nsgotest.App {
	if target := nsgotest.Target(); target == "ios_simulator" || target == "android" {
		t.Skipf("needs a desktop window, which %s has not", target)
	}
	return nsgotest.Start(t, Root, append([]nsgotest.Option{nsgotest.Size(1200, 800), nsgotest.Environment(desktopEnv)}, opts...)...)
}

// startPhone runs the gallery at a phone's size.
func startPhone(t *testing.T) *nsgotest.App {
	return nsgotest.Start(t, Root, nsgotest.Size(390, 844), nsgotest.Environment(core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US"}))
}

// open shows a demo in the desktop layout.
func open(app *nsgotest.App, name string) { app.Press("id:nav-" + name) }

// TestGroups keeps the sidebar in NSH's order of groups.
func TestGroups(t *testing.T) {
	app := startDesktop(t)
	last := core.Points(-1)
	for _, g := range groups {
		root := app.Find("id:nav-group-" + g.title)
		app.Find("id:nav-group-" + g.title + " >> id:nav-" + g.demos[0].name)
		if root.Frame.Origin.Y <= last {
			t.Fatalf("the %s group is above the group before it", g.title)
		}
		last = root.Frame.Origin.Y
	}
}

// TestEveryDemoBuilds opens each demo in both layouts and in the dark.
func TestEveryDemoBuilds(t *testing.T) {
	wide, phone := startDesktop(t), startPhone(t)
	for _, d := range allDemos() {
		open(wide, d.name)
		if got := wide.Find("id:title").Text; got != d.name {
			t.Fatalf("title = %q, want %q", got, d.name)
		}
		wide.Find("id:demo-" + d.name)
		phone.Press("id:nav-" + d.name)
		phone.Find("id:screen-" + d.name)
		phone.Back()
		phone.Find("id:home")
	}
	// The appearance builds the gallery again around the same demo.
	env := desktopEnv
	env.Dark = true
	wide.SetEnvironment(env)
	wide.Find("id:demo-" + allDemos()[len(allDemos())-1].name)
	if got := wide.Find("id:sidebar").Style.Background; got != galleryTheme().Dark.Colors["inset"] {
		t.Fatalf("sidebar = %x, want the dark inset", got)
	}
}

// TestLayoutsKeepTheDemo moves between the layouts with a demo open.
func TestLayoutsKeepTheDemo(t *testing.T) {
	app := startDesktop(t)
	open(app, "Controls")
	app.Resize(400, 800)
	app.Find("id:screen-Controls")
	app.Back()
	app.Press("id:nav-Form")
	app.Resize(1200, 800)
	if got := app.Find("id:title").Text; got != "Form" {
		t.Fatalf("title = %q after widening, want Form", got)
	}
	if got, _ := app.Storage().GetItem(lastDemo); got != "Form" {
		t.Fatalf("remembered %q, want Form", got)
	}
}

// TestResizeKeepsTheCards moves the cards between columns, keeping what
// they show, when the pane narrows to one column.
func TestResizeKeepsTheCards(t *testing.T) {
	app := startDesktop(t)
	open(app, "Controls")
	app.Press("role:switch Notifications")
	app.Resize(900, 800)
	if !app.Find("role:switch Badges").Props.Disabled {
		t.Fatal("the switches were built again")
	}
}

func TestPhoneRestoresDemo(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 844), nsgotest.Storage(map[string]string{lastDemo: "Controls"}))
	app.Find("id:screen-Controls")
	app.Back()
	app.Find("id:home")
	if _, ok := app.Storage().GetItem(lastDemo); ok {
		t.Fatal("the index still remembers a demo")
	}
}

func TestSidebarToggles(t *testing.T) {
	app := startDesktop(t)
	app.Press("role:button Toggle sidebar")
	if w := app.Find("id:demo-Stocks").Parent.Frame.Size.Width; w < 1100 {
		t.Fatalf("the pane is %v wide with the sidebar hidden", w)
	}
	app.ChooseMenu("View", "Show Sidebar")
	if w := app.Find("id:demo-Stocks").Parent.Frame.Size.Width; w > 1200-sidebarWidth+1 {
		t.Fatalf("the pane is %v wide with the sidebar shown", w)
	}
}

func TestApplicationMenu(t *testing.T) {
	app := startDesktop(t)
	app.ChooseMenu("View", "Menus")
	app.Find("id:demo-Menus")
	if app.Menu[0].Items[0].Role != desktop.MenuAbout {
		t.Fatalf("app menu starts with %+v", app.Menu[0].Items[0])
	}
}

func TestControls(t *testing.T) {
	app := startDesktop(t)
	open(app, "Controls")
	app.Press("role:switch Notifications")
	app.Find("text:Notifications: off")
	if !app.Find("role:switch Badges").Props.Disabled {
		t.Fatal("Badges stays enabled without notifications")
	}
	app.Press("role:checkbox Select all")
	app.Find("text:Showing 1,650 items")
	app.Press("role:radio Team")
	app.Press("role:button Continue with Team")
	app.Find("text:Continue: Team, $12 / mo")
	app.Press("role:tab Members")
	app.Find("text:Grace Hopper")
	app.Press("id:demo-Controls >> role:button Storage")
	app.Find("text:120 GB")
}

func TestForm(t *testing.T) {
	app := startDesktop(t)
	open(app, "Form")
	app.Press("role:button Submit")
	app.Find("text:3 fields need attention")
	app.Find("text:Enter your name")
	app.Type("id:name", "Ada")
	app.Type("id:email", "ada@example.com")
	app.Type("id:password", "Analytical1!")
	app.Find("text:Strong")
	app.Press("role:button Submit")
	app.Find("text:Account created for Ada <ada@example.com>")
	app.Type("id:shout", "loud")
	if got := app.Find("id:shout").Text; got != "LOUD" {
		t.Fatalf("controlled field = %q", got)
	}
}

func TestStorage(t *testing.T) {
	app := startDesktop(t, nsgotest.Storage(map[string]string{launchesKey: "4"}))
	open(app, "Storage")
	app.Find(`text:SetItem("launches", "5")`)
	app.Type("id:note", "Milk")
	if got, _ := app.Storage().GetItem(noteKey); got != "Milk" {
		t.Fatalf("note = %q", got)
	}
	app.OnAlert = func(a core.AlertConfig) int { return 1 }
	app.Press("role:button Clear all")
	if keys := app.Storage().Keys(); len(keys) > 1 {
		t.Fatalf("keys left: %v", keys)
	}
}

func TestMenus(t *testing.T) {
	app := startDesktop(t)
	open(app, "Menus")
	// On the chip's text, as a pointer presses it.
	app.Press("text:File")
	app.Press("text:New document")
	app.Find("text:Done: New document")
	app.Find("text:chose New document")
	app.Press("label:Actions for Notes.txt")
	app.Press("text:Delete")
	if app.Exists("text:Notes.txt") {
		t.Fatal("Notes.txt was not deleted")
	}
	app.Key("mod+shift+p")
	app.Find("text:New note")
}

// TestNativePopupMenu right-clicks the area that shows the platform's own
// context menu and chooses an item of its submenu there.
func TestNativePopupMenu(t *testing.T) {
	app := startDesktop(t)
	open(app, "Menus")
	app.RightClick("id:popup-area")
	app.ChoosePopup("Share", "Mail")
	app.Find("text:Chose Mail")
}

func TestStocks(t *testing.T) {
	app := startDesktop(t)
	open(app, "Stocks")
	price := app.Find("id:stock-price").Text
	app.Advance(2 * time.Second)
	app.WaitUntil(2*time.Second, func() bool { return app.Find("id:stock-price").Text != price })
	app.Press("id:stock-LUMO")
	if n := len(app.Query("text:Lumora Optics")); n != 2 {
		t.Fatalf("Lumora Optics shows %d times, want in its row and the detail", n)
	}
	app.Press("role:button Buy")
	app.Press("role:button Review buy")
	app.Press("role:button Place buy order")
	app.Find("text:Buy order placed")
	app.Press("role:button Pause the market")
	app.Find("role:button Resume the market")
}

func TestStocksOnAPhone(t *testing.T) {
	app := startPhone(t)
	app.Press("id:nav-Stocks")
	app.Press("id:stock-ORBX")
	app.Find("text:Orbex Freight")
	app.Back()
	app.Find("id:watchlist")
}

func TestChatStreamsAScript(t *testing.T) {
	app := startPhone(t)
	app.Press("id:nav-AI Chat")
	app.Press("label:Think it through >> nth=0")
	app.Advance(20 * time.Second)
	app.Find("text:The ball costs")
	app.Find("role:button Regenerate")
	app.Find("text:Thought for")
	app.Press("role:button Copy")
	if !strings.Contains(app.Clipboard, "0.05") {
		t.Fatalf("copied %q", app.Clipboard)
	}
	app.Type("id:composer", "Hello")
	app.Press("role:button Send")
	app.Advance(20 * time.Second)
	app.Find("text:This assistant is simulated")
}

func TestChatRetriesAFailure(t *testing.T) {
	app := startPhone(t)
	app.Press("id:nav-AI Chat")
	app.Press("label:Fail a reply")
	app.Advance(10 * time.Second)
	app.Find("text:error 529")
	app.Press("role:button Retry")
	app.Advance(20 * time.Second)
	app.Find("text:fewer meetings next sprint")
}

func TestListsAndScrolling(t *testing.T) {
	app := startDesktop(t)
	open(app, "Lists")
	app.Find("id:grid")
	app.Press("role:tab More")
	app.Scroll("id:more", 200)
	app.Press("role:button Add")
	app.Find("text:Item 1")
	app.Press("role:tab Chat")
	app.Type("id:chat-draft", "Ship it today")
	app.Submit("id:chat-draft")
	app.Find("text:Ship it today")
	app.Press("role:tab Page")
	app.Scroll("id:page", 400)

	open(app, "Scrolling")
	app.Press("role:button Add 3 posts on top")
	app.Find("text:New 3")
	app.Find("text:33 posts")
}

func TestGesturesAndFileDrop(t *testing.T) {
	app := startDesktop(t)
	open(app, "Gestures")
	app.Pan("id:photo", core.Point{X: 30, Y: 40})
	app.Find("text:50 pt")
	app.Press("id:press-pad")
	app.Find("text:press · pressOut · pressIn")
	app.DropFiles("id:drop-zone", "file:///tmp/a%20b.txt", "blob:http://localhost/1234#notes.md")
	if got := app.Find("id:dropped").Text; got != "a b.txt, notes.md" {
		t.Fatalf("dropped = %q", got)
	}
}

func TestMedia(t *testing.T) {
	app := startDesktop(t)
	open(app, "Web and video")
	app.PostMessage("id:bridge", "hello from the page")
	app.Find("text:hello from the page")
	app.Play("id:player", core.VideoInfo{Duration: 5 * time.Second, Size: core.Size{Width: 640, Height: 360}})
	app.Press("role:button Play")
	if v := app.Find("id:player").Video; v.Paused || !v.Muted || !v.Repeat {
		t.Fatalf("video = %+v", v)
	}
	app.Progress("id:player", 2*time.Second, 5*time.Second)
	app.Find("text:0:02")
	app.Press("role:button Restart")
	if s := app.Find("id:player").Seeks; len(s) != 1 || s[0] != 0 {
		t.Fatalf("seeks = %v", s)
	}
	slider := app.Find("id:native-slider")
	if slider.Kind != nsgotest.KindNative || slider.Component != sliderComponent || slider.NativeProps["value"] != 0.4 {
		t.Fatalf("native view %+v", slider)
	}
	app.Emit("id:native-slider", "change", 0.25)
	app.Find("text:Last event: change 0.25")
}

func TestSystem(t *testing.T) {
	headless(t)
	app := startDesktop(t)
	open(app, "System")
	app.Press("role:button Minimize")
	app.Press("role:button Full screen")
	if !slices.ContainsFunc(app.Window, func(c core.WindowCommand) bool { return c.Action == core.WindowToggleFullScreen }) {
		t.Fatalf("window commands = %+v", app.Window)
	}
	app.Press("role:button Copy text")
	if app.Clipboard != "Hello from NSGo" {
		t.Fatalf("clipboard = %q", app.Clipboard)
	}
	app.Press("role:button Copy logo")
	if !bytes.Equal(app.ClipboardImage, logoPNG) {
		t.Fatal("the logo was not copied")
	}
	app.Press("role:button Paste image")
	app.Find("id:pasted-image")
	app.Press("role:button The website")
	app.Find("text:Opened https://github.com/oneo-me/nsgo")
	app.OnAlert = func(core.AlertConfig) int { return 1 }
	app.Press("role:button Confirm")
	app.Find("text:Delete")
}

func TestAccessibility(t *testing.T) {
	app := startDesktop(t)
	open(app, "Accessibility")
	app.AccessibilityAction("id:a11y-quantity", core.ActionIncrement)
	if v := app.Find("id:a11y-quantity").Props.Value; v.Now != 3 {
		t.Fatalf("value = %+v", v)
	}
	app.AccessibilityAction("id:a11y-message", "archive")
	app.Find("text:archive")
	app.Press("role:button Search apples")
	if f := app.AccessibilityFocus; f == nil || f.Props.TestID != "a11y-result" {
		t.Fatalf("focus = %+v", f)
	}
}

func TestToasts(t *testing.T) {
	app := startDesktop(t)
	open(app, "Overlays")
	app.Press("role:button Delete the draft")
	app.Find("id:draft >> text:Draft deleted")
	app.Press("role:button Undo")
	app.Advance(time.Second)
	app.WaitForGone("id:draft", 2*time.Second)
	if got := app.Find("id:last-event").Text; got != "toast: undone" {
		t.Fatalf("last event = %q, want the action that closed the toast", got)
	}
	app.Press("role:button Copy the link")
	app.Find("text:The link is on the clipboard (1)")
	app.Advance(6 * time.Second)
	app.WaitForGone("text:The link is on the clipboard (1)", 2*time.Second)
}

func TestCollapsingHeader(t *testing.T) {
	app := startDesktop(t)
	open(app, "Collapsing header")
	app.Find("id:header >> text:Recent")
	app.Find("text:Week 1")
	if app.Exists("text:Week 20") {
		t.Fatal("the list built rows far off the screen")
	}
	app.Scroll("id:page", 8000)
	app.Find("text:Week 20")
	app.Find("id:footer >> text:That's all 160 entries")
}

// TestCharts hovers the week's plot, switches weeks and finds the
// sparklines, a plot per row of a list.
func TestCharts(t *testing.T) {
	app := startDesktop(t)
	open(app, "Charts")
	app.Find("text:Sun, the last day")
	plot := "role:image Visitors per day, from 1,820 on Monday to 1,180 on Sunday"
	app.Find(plot)
	// The pointer at the middle: Thursday, the fourth of seven days.
	app.Hover(plot)
	app.Find("text:Plot: Thu, 2,480")
	app.Find("text:2,480 visitors")
	app.Press("role:button Show this week")
	app.Find("text:Plot: this week")
	app.Find("text:1,330 visitors")
	if len(app.Query("text:NSHL")) == 0 {
		t.Fatal("no sparkline rows")
	}
	if n := len(app.Query("role:image")); n < 5 {
		t.Fatalf("%d images, want the week's plot and a sparkline per row", n)
	}
}

// TestDataTable selects a row of the million stocks, sorts by ID and
// jumps far down.
func TestDataTable(t *testing.T) {
	app := startDesktop(t)
	open(app, "DataTable")
	app.Find("text:rows 1–")
	app.Press("text:Apple 1 >> nth=0")
	app.Find("text:Row 1 >> nth=0")
	// Ascending, then descending: the last stock first.
	app.Press("id:sort:id")
	app.Press("id:sort:id")
	app.Find("text:1000000 >> nth=0")
	app.Press("id:sort:id")
	app.Press("role:button Go to row 500000")
	app.Find("text:Tesla 500000")
	app.Press("text:Cells")
	app.Press("text:Tesla 500000")
	app.Find("text:Cell row 500000, Name")
}

// TestTable finds the invoices' table and its roles on Controls.
func TestTable(t *testing.T) {
	app := startDesktop(t)
	open(app, "Controls")
	app.Find("role:table Recent invoices")
	if n := len(app.Query("role:columnheader")); n < 4 {
		t.Fatalf("%d column headers, want the invoices' 4 and the total", n)
	}
}

// TestEditor types in the editor, finds a word and opens the large file.
// TestEditor opens NSH's Editor demo: the simulated language server's
// problems in the dock, a hover card with a fix that edits the text, the
// find bar and another language's sample.
func TestEditor(t *testing.T) {
	headless(t)
	app := startDesktop(t)
	open(app, "Editor")
	app.Find("text:interface Order {")
	app.WaitFor("text:Problems 6", 5*time.Second)
	app.Find("text:1 error")
	app.Find("text:4 warnings")
	// The misspelled method: its problem in a hover card, whose fix edits
	// the text as one step.
	row := app.Find("id:editor >> text:receipt.prnt();")
	charW := row.Frame.Size.Width / core.Points(len("receipt.prnt();"))
	app.HoverAt("id:editor >> text:receipt.prnt();", core.Point{X: charW * 9.5, Y: row.Frame.Size.Height / 2})
	app.Advance(200 * time.Millisecond)
	app.WaitFor("id:hover-card", 5*time.Second)
	app.Find("id:hover-card >> text:Property 'prnt' does not exist on type 'Receipt'. Did you mean 'print'?")
	app.Press("id:hover-card >> role:button Change spelling to 'print'")
	app.Find("id:editor >> text:receipt.print();")
	app.Advance(400 * time.Millisecond)
	app.WaitFor("text:Problems 5", 5*time.Second)
	app.Find("text:0 errors")
	app.Press("role:button Find and replace")
	app.Type("label:Find", "order")
	app.Find("id:find-bar >> text:matches")
	app.Key("Escape")
	app.Settle()
	if app.Exists("id:find-bar") {
		t.Error("Escape left the find bar open")
	}
	app.Press("label:Language")
	app.Press("text:Rust")
	app.Find("text:fn word_counts(text: &str) -> Vec<(&str, usize)> {")
	app.Find("text:Problems 1")
	app.Find("text:main.rs")
}

// TestDiff counts the changes, reverts one and shows the patch.
func TestDiff(t *testing.T) {
	app := startDesktop(t)
	open(app, "Diff")
	app.Find("text:var ErrNotFound")
	app.Find("text:· 7 changes")
	app.Press("role:button Revert >> nth=0")
	app.Find("text:· 6 changes")
	app.Press("role:button Show a patch")
	app.Find("text:store.go")
	app.Find("text:old_test.go → store_test.go")
}

// TestAppearanceRestylesInPlace switches each demo to dark: every view
// stays (nothing is built again) and takes the colors a fresh dark build
// shows.
func TestAppearanceRestylesInPlace(t *testing.T) {
	headless(t)
	dark := desktopEnv
	dark.Dark = true
	for _, d := range allDemos() {
		t.Run(d.name, func(t *testing.T) {
			// The market's random prices color its charts.
			drawings := d.name != "Stocks"
			shown := "id:demo-" + d.name
			app := startDesktop(t)
			open(app, d.name)
			app.FastForward(2 * time.Second)
			before := viewIDs(rootOf(app.Find(shown)))
			app.Press("role:button Dark appearance")
			got := snapshot(app, shown, drawings)
			checkKept(t, before, got)
			// The gallery keeps its state in a global: the fresh build
			// starts after the switched one is read.
			fresh := startDesktop(t, nsgotest.Environment(dark))
			open(fresh, d.name)
			fresh.FastForward(2 * time.Second)
			checkLooks(t, got, snapshot(fresh, shown, drawings))
		})
	}
}

// TestPhoneAppearanceRestylesInPlace switches the phone layout to dark:
// with the index's toggle, and as the system turns dark under a pushed
// demo.
func TestPhoneAppearanceRestylesInPlace(t *testing.T) {
	headless(t)
	phone := core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US"}
	dark := phone
	dark.Dark = true
	for _, demo := range []string{"", "Controls"} {
		start := func(env core.Environment) (*nsgotest.App, string) {
			app := nsgotest.Start(t, Root, nsgotest.Size(390, 844), nsgotest.Environment(env))
			if demo == "" {
				return app, "id:home"
			}
			app.Press("id:nav-" + demo)
			return app, "id:screen-" + demo
		}
		app, shown := start(phone)
		app.FastForward(2 * time.Second)
		before := viewIDs(rootOf(app.Find(shown)))
		if demo == "" {
			app.Press("role:button Dark appearance")
		} else {
			app.SetEnvironment(dark)
		}
		got := snapshot(app, shown, true)
		checkKept(t, before, got)
		fresh, _ := start(dark)
		fresh.FastForward(2 * time.Second)
		checkLooks(t, got, snapshot(fresh, shown, true))
	}
}

// shot is the views from a root down: where each is, what it shows, how
// it looks, and which have no size (a hidden screen's canvases draw when
// shown).
type shot struct {
	ids          map[uint64]string
	where, texts []string
	looks        []string
	unsized      []bool
}

// snapshot reads the views of the window showing selector.
func snapshot(app *nsgotest.App, selector string, drawings bool) shot {
	root := rootOf(app.Find(selector))
	s := shot{ids: viewIDs(root), looks: looks(root, drawings)}
	for _, v := range views(root) {
		s.where = append(s.where, where(v))
		s.texts = append(s.texts, v.Text)
		s.unsized = append(s.unsized, v.Frame.Size == driver.Size{})
	}
	return s
}

// checkKept fails when a view of before is gone from after.
func checkKept(t *testing.T, before map[uint64]string, after shot) {
	t.Helper()
	for id, where := range before {
		if _, ok := after.ids[id]; !ok {
			t.Fatalf("%s was built again", where)
		}
	}
}

// checkLooks fails when a view looks other than in a fresh build, unless
// it shows other data (live prices) or has no size.
func checkLooks(t *testing.T, got, want shot) {
	t.Helper()
	if len(got.looks) != len(want.looks) {
		t.Fatalf("%d views after the switch, %d in a dark build", len(got.looks), len(want.looks))
	}
	for i := range got.looks {
		if got.looks[i] != want.looks[i] && got.texts[i] == want.texts[i] && !got.unsized[i] {
			a, b := differ(got.looks[i], want.looks[i])
			t.Fatalf("%s after the switch: …%s…\nin a dark build: …%s…", got.where[i], a, b)
		}
	}
}

// differ is where a and b first differ, with some context.
func differ(a, b string) (string, string) {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	from := max(0, i-80)
	return a[from:min(len(a), i+80)], b[from:min(len(b), i+80)]
}

// rootOf is the root view above v.
func rootOf(v *nsgotest.View) *nsgotest.View {
	for v.Parent != nil {
		v = v.Parent
	}
	return v
}

// viewIDs are the views from v down, with where each is.
func viewIDs(v *nsgotest.View) map[uint64]string {
	ids := map[uint64]string{}
	for _, v := range views(v) {
		ids[uint64(v.ID)] = where(v)
	}
	return ids
}

// views are the views from v down, in order.
func views(v *nsgotest.View) []*nsgotest.View {
	out := []*nsgotest.View{v}
	for _, c := range v.Children {
		out = append(out, views(c)...)
	}
	return out
}

// where names a view for a failure: its kind, text and the nearest test
// ID above it.
func where(v *nsgotest.View) string {
	id := ""
	for a := v; a != nil && id == ""; a = a.Parent {
		id = a.Props.TestID
	}
	return fmt.Sprintf("view %d (kind %d, %q, in %q)", v.ID, v.Kind, v.Text, id)
}

// looks are the colors and styles each view from v down shows, in order,
// and its drawing's colors with drawings.
func looks(v *nsgotest.View, drawings bool) []string {
	var out []string
	var walk func(*nsgotest.View, string)
	walk = func(v *nsgotest.View, path string) {
		// A drawing's colors; where it draws follows its data.
		type paint struct {
			Kind  driver.DrawKind
			Color driver.Color
			Stops any
		}
		var ops []paint
		for _, op := range v.Ops {
			if !drawings {
				break
			}
			p := paint{Kind: op.Kind, Color: op.Color}
			if op.Gradient != nil {
				p.Stops = op.Gradient.Stops
			}
			ops = append(ops, p)
		}
		// JSON follows the pointers in the styles.
		look, err := json.Marshal(struct {
			Kind  driver.ViewKind
			Style driver.ViewStyle
			Text  driver.TextStyle
			Spans []driver.TextSpan
			Ops   []paint
		}{v.Kind, v.Style, v.TextStyle, v.Spans, ops})
		if err != nil {
			panic(err)
		}
		out = append(out, path+" "+string(look))
		for i, c := range v.Children {
			walk(c, fmt.Sprintf("%s/%d", path, i))
		}
	}
	walk(v, "")
	return out
}
