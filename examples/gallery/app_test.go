package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"

	"github.com/oneo-me/nsgo/ui"
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

// open shows a demo in the desktop layout, from its tab's sidebar.
func open(app *nsgotest.App, name string) {
	app.Press("id:tab-" + tabs[tabOf(name)].id)
	app.Press("id:nav-" + name)
}

// openTab chooses a tab of the phone layout's system tab bar.
func openTab(app *nsgotest.App, i int) { app.Emit("id:tab-bar", "change", float64(i)) }

// openPhone pushes a demo in the phone layout, from its tab's index.
func openPhone(app *nsgotest.App, name string) {
	openTab(app, tabOf(name))
	app.Press("id:nav-" + name)
}

// TestTabs keeps each tab's sidebar to its demos, in order, and lets each
// tab show the demo it showed last.
func TestTabs(t *testing.T) {
	app := startDesktop(t)
	for _, tab := range tabs {
		app.Press("id:tab-" + tab.id)
		last := core.Points(-1)
		for _, g := range tab.groups {
			root := app.Find("id:nav-group-" + g.name)
			app.Find("id:nav-group-" + g.name + " >> id:nav-" + g.demos[0].name)
			if root.Frame.Origin.Y <= last {
				t.Fatalf("the %s group is above the group before it", g.name)
			}
			last = root.Frame.Origin.Y
		}
		app.Find("id:demo-" + tab.groups[0].demos[0].name)
	}
	if app.Exists("id:nav-Stocks") {
		t.Fatal("the basics tab lists the apps")
	}
	open(app, "Selection")
	app.Press("id:tab-basics")
	app.Press("id:tab-components")
	app.Find("id:demo-Selection")
	// The menu reaches a demo of another tab.
	app.Press("id:tab-basics")
	app.ChooseMenu("View", "Photos")
	app.Find("id:nav-Photos")
	app.Find("id:demo-Photos")
}

// TestPhoneTabs shows the tabs and the settings at the bottom, and the
// settings' theme.
func TestPhoneTabs(t *testing.T) {
	app := startPhone(t)
	bars := app.QueryMounted("id:tab-bar")
	if len(bars) != 1 || len(bars[0].NativeProps["items"].([]any)) != len(tabs)+1 {
		t.Fatalf("the bars are %v", bars)
	}
	openPhone(app, "Selection")
	app.Back()
	openTab(app, settingsTab())
	app.Find("id:settings-page")
	app.Find("id:settings-general-language")
	app.Press("id:theme")
	app.Press("id:theme-2")
	if got, _ := app.Storage().GetItem(themeKey); got != "dark" {
		t.Fatalf("kept %q, want dark", got)
	}
	if got := app.Find("id:settings-page").Style.Background; got != ui.Theme().Dark.Background {
		t.Fatalf("page = %x, want the dark background", got)
	}
	// One bar serves every tab, and pages pushed cover it.
	openPhone(app, "Selection")
	app.Back()
	if got := len(app.QueryMounted("id:tab-bar")); got != 1 {
		t.Fatalf("%d bars", got)
	}
	if len(app.QueryMounted("id:screen-Selection >> id:tab-bar")) != 0 {
		t.Fatal("a pushed page holds the bar")
	}
}

// TestPhoneLargeTitles titles the indexes, the settings and the demos that
// scroll with iOS's large title, and leaves the others a compact bar.
func TestPhoneLargeTitles(t *testing.T) {
	app := startPhone(t)
	if !app.Find("id:index").Screen.LargeTitle {
		t.Fatal("the index has no large title")
	}
	openPhone(app, "Selection")
	if !app.Find("id:screen-Selection").Screen.LargeTitle {
		t.Fatal("a demo that scrolls has no large title")
	}
	app.Back()
	openPhone(app, "Terminal")
	if app.Find("id:screen-Terminal").Screen.LargeTitle {
		t.Fatal("a demo that fills the page has a large title")
	}
	app.Back()
	openTab(app, settingsTab())
	if !app.Find("id:settings-page").Screen.LargeTitle {
		t.Fatal("the settings have no large title")
	}
}

// TestNarrowDesktop shows the stack's header at the window's top, clear of
// the window controls, with no title bar of the app's.
func TestNarrowDesktop(t *testing.T) {
	env := desktopEnv
	env.TitleBar = core.Edges{Top: 28, Left: 78}
	app := startDesktop(t, nsgotest.Size(500, 700), nsgotest.Environment(env))
	app.Press("role:tab Settings")
	app.Find("id:settings-page")
	head := app.Find("role:header Settings").Parent.Parent
	if head.Frame.Origin.Y != 0 || head.Props.WindowDrag != core.WindowDragOn {
		t.Fatalf("the header is at %v, dragging %v", head.Frame.Origin, head.Props.WindowDrag)
	}
	if x := app.Find("role:header Settings").Parent.Frame.Origin.X; x < 78 {
		t.Fatalf("the title is at %v, under the window controls", x)
	}
	if app.Exists("id:settings") {
		t.Fatal("the sidebar's footer shows")
	}
	// The bar leaves pushed pages the window.
	app.Press("role:tab Controls")
	app.Press("id:nav-Stocks")
	if app.Exists("role:tablist Sections") {
		t.Fatal("the tab bar shows over a pushed page")
	}
	app.Back()
	app.Find("role:tablist Sections")
}

// TestSettingsFollowTheLayout turns the dialog into the tab and back.
func TestSettingsFollowTheLayout(t *testing.T) {
	app := startDesktop(t)
	app.Press("id:settings")
	app.Press("id:theme")
	app.Press("id:theme-1")
	if got, _ := app.Storage().GetItem(themeKey); got != "light" {
		t.Fatalf("kept %q, want light", got)
	}
	app.Resize(400, 800)
	app.Find("id:settings-page")
	app.Resize(1200, 800)
	app.Find("id:settings-dialog")
}

func TestThemeRemembered(t *testing.T) {
	app := startDesktop(t, nsgotest.Storage(map[string]string{themeKey: "dark"}))
	app.Find("role:button Light appearance")
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
		openPhone(phone, d.name)
		phone.Find("id:screen-" + d.name)
		phone.Back()
		phone.Find("id:home")
	}
	// The appearance builds the gallery again around the same demo.
	env := desktopEnv
	env.Dark = true
	wide.SetEnvironment(env)
	wide.Find("id:demo-" + allDemos()[len(allDemos())-1].name)
	if got := wide.Find("id:shell").Style.Background; got != ui.Theme().Dark.Colors["inset"] {
		t.Fatalf("sidebar = %x, want the dark inset", got)
	}
}

// TestLayoutsKeepTheDemo moves between the layouts with a demo open.
func TestLayoutsKeepTheDemo(t *testing.T) {
	app := startDesktop(t)
	open(app, "Selection")
	app.Resize(400, 800)
	app.Find("id:screen-Selection")
	app.Back()
	app.Press("id:nav-Input")
	app.Resize(1200, 800)
	if got := app.Find("id:title").Text; got != "Input" {
		t.Fatalf("title = %q after widening, want Input", got)
	}
	if got, _ := app.Storage().GetItem(lastDemo); got != "Input" {
		t.Fatalf("remembered %q, want Input", got)
	}
}

// TestResizeKeepsTheCards moves the cards between columns, keeping what
// they show, when the pane narrows to one column.
func TestResizeKeepsTheCards(t *testing.T) {
	app := startDesktop(t)
	open(app, "Selection")
	app.Press("role:switch Disabled >> nth=0")
	app.Resize(900, 800)
	if !app.Find("role:checkbox Starred").Props.Disabled {
		t.Fatal("the cards were built again")
	}
}

func TestPhoneRestoresDemo(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 844), nsgotest.Storage(map[string]string{lastDemo: "Selection"}))
	app.Find("id:screen-Selection")
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
	app.ChooseMenu("View", "Menu")
	app.Find("id:demo-Menu")
	if app.Menu[0].Items[0].Role != desktop.MenuAbout {
		t.Fatalf("app menu starts with %+v", app.Menu[0].Items[0])
	}
}

// TestControls uses a control of each page, each reporting its event.
func TestControls(t *testing.T) {
	app := startDesktop(t)
	open(app, "Selection")
	app.Press("role:radio Banana")
	app.Find("text:Radio: Banana")
	app.Press("role:checkbox Starred")
	app.Find("text:Checkbox: off")
	open(app, "Tabs")
	app.Press("role:tab Activity")
	app.Find("text:Tabs: 1")
	open(app, "Stepper and Breadcrumbs")
	app.Find("id:stage-Breadcrumbs")
	open(app, "Progress")
	if now := app.Find("role:progressbar Progress >> nth=0").Props.Value.Now; now != 40 {
		t.Fatalf("the bar stands at %v%%", now)
	}
}

func TestForm(t *testing.T) {
	app := startDesktop(t)
	open(app, "Input")
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
	open(app, "Menu")
	// On the chip's text, as a pointer presses it.
	app.Press("id:demo-Menu >> text:File")
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
	open(app, "Menu")
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
	openPhone(app, "Stocks")
	app.Press("id:stock-ORBX")
	app.Find("text:Orbex Freight")
	app.Back()
	app.Find("id:watchlist")
}

func TestChatStreamsAScript(t *testing.T) {
	app := startPhone(t)
	openPhone(app, "AI Chat")
	app.Press("label:Think it through >> nth=0")
	app.Advance(20 * time.Second)
	app.Find("text:The ball costs")
	app.Find("role:button Regenerate")
	app.Press("role:button Worked for")
	app.Press("role:button Ran 1 step")
	app.Find("role:button Thought for")
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
	openPhone(app, "AI Chat")
	app.Press("label:Fail a reply")
	app.Advance(10 * time.Second)
	app.Find("text:error 529")
	app.Press("role:button Retry")
	app.Advance(20 * time.Second)
	app.Find("text:fewer meetings next sprint")
}

func TestListsAndScrolling(t *testing.T) {
	app := startDesktop(t)
	open(app, "List")
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

	open(app, "Scroll")
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
	open(app, "WebView and Video")
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
	app.Press("id:demo-System >> role:button Confirm")
	app.Find("text:Delete")

	dir := t.TempDir()
	report := filepath.Join(dir, "report.pdf")
	os.WriteFile(report, []byte("%PDF-1.7 report"), 0o644)
	app.PickFiles = func(r core.FilesRequest) []core.File { return []core.File{{Path: report}} }
	app.Press("role:button PDFs and images")
	if r := app.FilePicks[0]; r.Limit != core.MaxPicked || !slices.Equal(r.Types, []string{"application/pdf", "image/*"}) {
		t.Fatalf("pick = %+v", r)
	}
	if got := app.Find("id:picked-file-0").Text; got != "report.pdf · application/pdf · 0 KB" {
		t.Fatalf("picked file = %q", got)
	}
	app.PickDirectories = func(core.DirectoriesRequest) []core.Directory { return []core.Directory{{Path: dir}} }
	app.Press("role:button Pick a folder")
	if got := app.Find("id:picked-folder-0").Text; app.DirectoryPicks[0].Limit != 1 || got != filepath.Base(dir)+" · "+dir {
		t.Fatalf("picked folder = %q, pick %+v", got, app.DirectoryPicks[0])
	}
	app.PickDirectories = nil
	app.Press("role:button Pick a folder")
	app.Find("text:No folder picked")
	saved := filepath.Join(dir, "copy.pdf")
	app.SaveFile = func(core.SaveRequest) string { return saved }
	app.Press("role:button Save the file picked")
	app.Find("text:Saved report.pdf")
	if b, _ := os.ReadFile(saved); string(b) != "%PDF-1.7 report" {
		t.Fatalf("saved %q", b)
	}
	app.Press("role:button Share the file picked")
	app.Find("text:Share sheet shown")
	if r := app.Shares[0]; len(r.Files) != 1 || r.Files[0] != (core.SharedFile{Path: report, Name: "report.pdf", MIME: "application/pdf"}) ||
		r.Text != "Hello from NSGo" || r.From.Size.Width == 0 {
		t.Fatalf("share = %+v", r)
	}
	app.Press("role:button Open a text file")
	app.Find("text:Opened in another app")
	if f := app.OpenedFiles[0]; f.Name != "Hello from NSGo.txt" || f.MIME != "text/plain" {
		t.Fatalf("opened %+v", f)
	}
	app.OpenFile = func(core.SharedFile) bool { return false }
	app.Press("role:button Open a text file")
	app.Find("text:No app opens it")
	app.SaveFile = nil
	app.Press("role:button Save a text file")
	app.Find("text:Nothing saved")
	if r := app.Saves[1]; r.Name != "Hello from NSGo.txt" || r.MIME != "text/plain" || string(r.Data) != "Hello from NSGo\n" {
		t.Fatalf("save = %+v", r)
	}
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
	open(app, "Toast")
	app.Press("role:switch Action")
	app.Press("id:toast-show")
	app.Find("id:toasts >> text:Changes saved")
	app.Press("role:button Undo")
	app.Advance(time.Second)
	app.WaitForGone("text:Changes saved", 2*time.Second)
	if got := app.Find("id:last-event").Text; got != "toast: undone" {
		t.Fatalf("last event = %q, want the action that closed the toast", got)
	}
	app.Press("id:toast-show")
	app.Find("text:Changes saved")
	app.Advance(6 * time.Second)
	app.WaitForGone("text:Changes saved", 2*time.Second)
}

// TestToastOptions moves the toasts from the Toast demo, each option
// applying to the one toast system every demo shows through.
func TestToastOptions(t *testing.T) {
	app := startDesktop(t)
	open(app, "Toast")
	app.Press("id:toast-show")
	bottom := app.Find("id:toasts").Parent.Frame.Origin.Y
	if x := app.Find("id:toasts").Frame.Origin.X; x < 600 {
		t.Fatalf("the toasts at the bottom end are at x %v", x)
	}
	app.Press("id:knob-Placement")
	app.Press("text:TopStart")
	if top := app.Find("id:toasts").Parent.Frame.Origin.Y; top >= bottom {
		t.Fatalf("the toasts moved from y %v to %v, want the top", bottom, top)
	}
	if x := app.Find("id:toasts").Frame.Origin.X; x != 16 {
		t.Fatalf("the toasts at the top start are at x %v", x)
	}
	shell.toastPlacement.Set(ui.ToastEnd)
	app.Settle()
	if at := app.Find("id:toasts").Frame.Origin; at.X < 600 || at.Y < 200 {
		t.Fatalf("the toasts at the end are at %v, want the middle of the right side", at)
	}
	if got := len(app.QueryMounted("id:toasts")); got != 1 {
		t.Fatalf("%d toast stacks", got)
	}
}

// TestPlaygrounds sets a prop and sees the component change, and a stage
// page keeps its panel beside the component until the pane is too narrow.
func TestPlaygrounds(t *testing.T) {
	app := startDesktop(t)
	open(app, "Buttons")
	if app.Find("id:button").Props.Disabled {
		t.Fatal("the button starts disabled")
	}
	app.Press("id:stage-Button >> role:button Save")
	app.Find("text:Button pressed")
	app.Press("role:switch Disabled >> nth=0")
	if !app.Find("id:button").Props.Disabled {
		t.Fatal("the Disabled knob left the button enabled")
	}
	open(app, "DataTable")
	if s, p := app.Find("id:stage"), app.Find("id:stage-panel"); p.Frame.Origin.X < s.Frame.Origin.X+s.Frame.Size.Width {
		t.Fatalf("the panel is at %v, inside the stage to %v", p.Frame.Origin.X, s.Frame.Size.Width)
	}
	app.Resize(800, 800)
	if s, p := app.Find("id:stage"), app.Find("id:stage-panel"); p.Frame.Origin.Y < s.Frame.Origin.Y+s.Frame.Size.Height {
		t.Fatalf("the panel is at %v, beside the stage", p.Frame.Origin)
	}
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
	open(app, "Plot")
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

// TestTable finds the invoices' table and its roles.
func TestTable(t *testing.T) {
	app := startDesktop(t)
	open(app, "Lists and Tables")
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
	app.Press("id:demo-Editor >> label:Language")
	app.Press("text:Rust")
	app.Find("text:fn word_counts(text: &str) -> Vec<(&str, usize)> {")
	app.Find("text:Problems 1")
	app.Find("text:main.rs")
}

// TestDiff counts the changes, reverts one and shows the patch.
func TestDiff(t *testing.T) {
	app := startDesktop(t)
	open(app, "DiffView")
	app.Find("text:var ErrNotFound")
	app.Find("text:· 7 changes")
	app.Press("role:button Revert >> nth=0")
	app.Find("text:· 6 changes")
	app.Press("role:switch Show a patch")
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
			fresh := startDesktop(t, nsgotest.Environment(dark), nsgotest.Storage(map[string]string{themeKey: "dark"}))
			open(fresh, d.name)
			fresh.FastForward(2 * time.Second)
			checkLooks(t, got, snapshot(fresh, shown, drawings))
		})
	}
}

// TestPhoneAppearanceRestylesInPlace switches the phone layout to dark:
// with the settings' theme, and as the system turns dark under a pushed
// demo.
func TestPhoneAppearanceRestylesInPlace(t *testing.T) {
	headless(t)
	phone := core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US"}
	dark := phone
	dark.Dark = true
	for _, demo := range []string{"", "Selection"} {
		var stored map[string]string
		start := func(env core.Environment) (*nsgotest.App, string) {
			app := nsgotest.Start(t, Root, nsgotest.Size(390, 844), nsgotest.Environment(env), nsgotest.Storage(stored))
			if demo == "" {
				// Both builds have the settings' tab.
				openTab(app, settingsTab())
				openTab(app, 0)
				return app, "id:home"
			}
			openPhone(app, demo)
			return app, "id:screen-" + demo
		}
		app, shown := start(phone)
		app.FastForward(2 * time.Second)
		before := viewIDs(rootOf(app.Find(shown)))
		if demo == "" {
			openTab(app, settingsTab())
			app.Press("id:theme")
			app.Press("id:theme-2")
			openTab(app, 0)
		} else {
			app.SetEnvironment(dark)
		}
		got := snapshot(app, shown, true)
		checkKept(t, before, got)
		if demo == "" {
			stored = map[string]string{themeKey: "dark"}
		}
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

// TestLanguageSetting chooses a language in the settings, which the
// setting says its words in, base's components follow and the layout's
// direction too, and keeps it.
func TestLanguageSetting(t *testing.T) {
	app := startDesktop(t)
	ltr := app.Find("id:settings").Frame.Origin.X
	app.Press("id:settings")
	app.Find("id:settings-dialog")
	app.Press("id:language")
	app.Find("text:简体中文")
	app.Press("text:العربية")
	if got, _ := app.Storage().GetItem(languageKey); got != "ar" {
		t.Errorf("kept %q, want ar", got)
	}
	// The settings stay open in the new language.
	app.Find("text:الإعدادات")
	// The sidebar's footer runs from the right: the settings button,
	// first, moves toward its end.
	if rtl := app.Find("id:settings").Frame.Origin.X; rtl <= ltr {
		t.Errorf("the settings button at %v right to left, %v left to right", rtl, ltr)
	}
	app.Press("id:language")
	app.Press("text:اتباع النظام")
	if _, ok := app.Storage().GetItem(languageKey); ok {
		t.Error("following the system keeps a language")
	}
	if got := app.Find("id:settings").Frame.Origin.X; got != ltr {
		t.Errorf("back to the system's English, the button is at %v, was %v", got, ltr)
	}
	app.Press("id:settings-close")
	app.Advance(200 * time.Millisecond)
	app.WaitForGone("id:settings-dialog", time.Second)
}

// TestSettingsSearch finds the language setting by a language's name and
// says when nothing matches.
func TestSettingsSearch(t *testing.T) {
	app := startDesktop(t)
	app.Press("id:settings")
	app.Type("id:settings-search", "简体")
	app.Find("text:Results")
	app.Find("id:settings-general-language")
	app.Type("id:settings-search", "nothing like it")
	app.Find("text:No settings match.")
}

// TestLanguageRemembered starts in the language chosen last time.
func TestLanguageRemembered(t *testing.T) {
	app := startDesktop(t, nsgotest.Storage(map[string]string{languageKey: "zh-Hans"}))
	app.Press("id:settings")
	app.Press("id:language")
	app.Find("text:跟随系统")
}

// TestGalleryLanguages shows the gallery in the user's language: the
// sidebar, a demo and base's own words.
func TestGalleryLanguages(t *testing.T) {
	for _, c := range []struct{ locale, nav, title, sort string }{
		{"zh-CN", "选择", "苹果", "按ID排序"},
		{"zh-TW", "選擇", "蘋果", "依ID排序"},
		{"ar-EG", "التحديد", "تفاح", "الفرز حسب المعرّف"},
	} {
		t.Run(c.locale, func(t *testing.T) {
			env := desktopEnv
			env.Locale = c.locale
			app := nsgotest.Start(t, Root, nsgotest.Size(1200, 800), nsgotest.Environment(env))
			open(app, "Selection")
			app.Find("id:nav-Selection >> text:" + c.nav)
			app.Find("id:demo-Selection >> text:" + c.title + " >> nth=0")
			open(app, "DataTable")
			app.Find("label:" + c.sort)
		})
	}
}

// TestSidebarResizes drags the sidebar's edge and restores its default
// width on a double click.
func TestSidebarResizes(t *testing.T) {
	app := startDesktop(t)
	// Stocks keeps drawing, which spaces presses past a double click.
	open(app, "Input")
	width := func() driver.Points { return app.Find("id:sidebar-slot").Frame.Size.Width }
	// The band is 3 points across, its line on the sidebar's last point.
	band := app.Find("id:sidebar-resize")
	if line := band.Frame.Origin.X + band.Children[0].Children[0].Frame.Origin.X; band.Frame.Size.Width != 3 || line != sidebarWidth-1 {
		t.Fatalf("the band is %v, its line at %v, want 3 wide with the line at %v", band.Frame, line, sidebarWidth-1)
	}
	app.Drag("id:sidebar-resize >> role:adjustable", driver.Point{X: 60})
	if w := width(); w != sidebarWidth+60 {
		t.Fatalf("the sidebar is %v wide after a drag of 60, want %v", w, sidebarWidth+60)
	}
	app.Press("id:sidebar-resize >> role:adjustable")
	app.Press("id:sidebar-resize >> role:adjustable")
	if w := width(); w != sidebarWidth {
		t.Fatalf("the sidebar is %v wide after a double click, want %v", w, sidebarWidth)
	}
}

// TestSidebarKeys tabs to the sidebar, one stop, whose arrows move the
// selection through the demos.
func TestSidebarKeys(t *testing.T) {
	app := startDesktop(t)
	for range 4 {
		if app.Find("id:sidebar").Children[0].Focused {
			break
		}
		app.Key("tab")
	}
	if !app.Find("id:sidebar").Children[0].Focused {
		t.Fatal("tab does not reach the sidebar")
	}
	app.Key("down")
	if got := app.Find("id:title").Text; got != "AI Chat" {
		t.Fatalf("down from Stocks selected %q, want AI Chat", got)
	}
	app.Key("end")
	if got, want := app.Find("id:title").Text, "TextView"; got != want {
		t.Fatalf("end selected %q, want %q", got, want)
	}
	app.Key("home")
	if got := app.Find("id:title").Text; got != "Stocks" {
		t.Fatalf("home selected %q, want Stocks", got)
	}
}

// TestPseudoLocales chooses en-XA, which says the gallery's and base's
// messages accented and bracketed, then ar-XB, which lays it out right to
// left.
func TestPseudoLocales(t *testing.T) {
	app := startDesktop(t)
	ltr := app.Find("id:settings").Frame.Origin.X
	app.Press("id:settings")
	app.Press("id:language")
	app.Press("text:[Éńğĺíśĥ ~~~]")
	app.Find("text:[Śéťťíńğś ~~~]")
	app.Press("id:language")
	app.Press("text:\u200f\u202eEnglish\u202c\u200f")
	if got := app.Find("id:settings").Frame.Origin.X; got <= ltr {
		t.Errorf("in ar-XB the settings button is at %v, %v left to right", got, ltr)
	}
}

// TestTerminal runs the shell, types a command that sets the title, which
// the panel shows, and restarts the shell.
func TestTerminal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("runs a Unix shell")
	}
	app := startDesktop(t)
	open(app, "Terminal")
	app.Find("id:terminal")
	// The view sized the screen, which the panel reports.
	app.WaitUntil(5*time.Second, func() bool { return app.Find("id:terminal-size").Text != "0 × 0" })
	app.Type("label:Terminal", `printf '\033]0;%s\007' nsgo-terminal-test`)
	app.Key("enter")
	titled := func() bool { return app.Find("id:terminal-title").Text == "nsgo-terminal-test" }
	app.WaitUntil(10*time.Second, titled)
	app.Screenshot("terminal")
	app.Press("text:Dracula")
	app.Press("text:Bar")
	app.Screenshot("terminal-dracula")
	app.Press("text:Restart the shell")
	if titled() {
		t.Error("the restarted shell kept the old title")
	}
}

// TestTerminalPhone runs the demo session on a phone, which runs no local
// programs: typed through the key bar and the keyboard, its title reaches
// the panel; a long press opens the context menu.
func TestTerminalPhone(t *testing.T) {
	app := startPhone(t)
	openPhone(app, "Terminal")
	app.Find("id:terminal")
	app.Find("id:terminal-key-escape")
	app.Type("label:Terminal", "title on a phone")
	app.Press("id:terminal-key-ctrl")
	app.Type("label:Terminal", "u")
	app.Type("label:Terminal", "title from the phone")
	app.Key("enter")
	app.WaitUntil(5*time.Second, func() bool { return app.Find("id:terminal-title").Text == "from the phone" })
	app.Screenshot("terminal-phone")
	app.LongPress("id:terminal-screen")
	// The menu's hold is timed on the test's clock.
	app.Advance(time.Second)
	app.WaitFor("text:Paste", 5*time.Second)
	app.Screenshot("terminal-phone-menu")
}

// TestTerminalPhoneSelect long-presses a word of a full-screen program's
// rows: it is selected, which the context menu's Copy follows.
func TestTerminalPhoneSelect(t *testing.T) {
	app := startPhone(t)
	openPhone(app, "Terminal")
	app.Find("id:terminal")
	app.Type("label:Terminal", "top")
	app.Key("enter")
	time.Sleep(300 * time.Millisecond)
	app.Settle()
	app.LongPress("id:terminal-screen")
	app.Advance(time.Second)
	app.WaitFor("text:Paste", 5*time.Second)
	app.Screenshot("terminal-phone-select")
	for v := app.Find("text:Copy"); v != nil; v = v.Parent {
		if v.Props.Disabled {
			t.Error("Copy is disabled with a word selected")
		}
	}
}
