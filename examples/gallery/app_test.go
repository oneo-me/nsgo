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
	"testing/fstest"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/i18n"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/nsgo-dev/nsgo/base/richeditor"
	"github.com/nsgo-dev/nsgo/ui"
	uirich "github.com/nsgo-dev/nsgo/ui/richeditor"
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

// showBase shows the base form of the playground titled title, on the tab
// beside its ui form's.
func showBase(app *nsgotest.App, title string) { app.Press("id:package-" + title + " >> role:tab Base") }

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
	open(app, "Checkbox")
	app.Press("id:tab-basics")
	app.Press("id:tab-components")
	app.Find("id:demo-Checkbox")
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
	openPhone(app, "Checkbox")
	app.Back()
	openTab(app, settingsTab())
	app.Find("id:settings-page")
	app.Find("id:settings-general-language")
	app.Press("id:theme")
	app.Press("id:theme-2")
	if got, _ := app.Storage().GetItem(themeKey); got != "dark" {
		t.Fatalf("kept %q, want dark", got)
	}
	if got := app.Find("id:settings-page").Style.Background; got != core.DefaultTheme().Dark.Background {
		t.Fatalf("page = %x, want the dark background", got)
	}
	// One bar serves every tab, and pages pushed cover it.
	openPhone(app, "Checkbox")
	app.Back()
	if got := len(app.QueryMounted("id:tab-bar")); got != 1 {
		t.Fatalf("%d bars", got)
	}
	if len(app.QueryMounted("id:screen-Checkbox >> id:tab-bar")) != 0 {
		t.Fatal("a pushed page holds the bar")
	}
}

// TestColorThemeChosen shows and keeps the built-in theme chosen in the
// settings.
func TestColorThemeChosen(t *testing.T) {
	app := startPhone(t)
	openTab(app, settingsTab())
	if got := app.Find("id:settings-page").Style.Background; got != core.DefaultTheme().Light.Background {
		t.Fatalf("page = %x, want the default background", got)
	}
	app.Press("id:color-theme")
	app.Press("id:color-theme-1")
	if got, _ := app.Storage().GetItem(colorThemeKey); got != warmDusk {
		t.Fatalf("kept %q, want %q", got, warmDusk)
	}
	if got := app.Find("id:settings-page").Style.Background; got != ui.WarmDuskTheme().Light.Background {
		t.Fatalf("page = %x, want Warm Dusk's background", got)
	}
}

// TestPhoneLargeTitles titles the indexes, the settings and the demos that
// scroll with iOS's large title, and leaves the others a compact bar.
func TestPhoneLargeTitles(t *testing.T) {
	app := startPhone(t)
	if !app.Find("id:index").Screen.LargeTitle {
		t.Fatal("the index has no large title")
	}
	openPhone(app, "Checkbox")
	if !app.Find("id:screen-Checkbox").Screen.LargeTitle {
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
	app.Press("role:tab Components")
	app.Press("id:nav-Stocks")
	if app.Exists("role:tablist Sections") {
		t.Fatal("the tab bar shows over a pushed page")
	}
	app.Back()
	app.Find("role:tablist Sections")
}

// TestSettingsMenuItem opens the settings from the application menu, as
// the dialog in the wide layout and the tab in the narrow one.
func TestSettingsMenuItem(t *testing.T) {
	app := startDesktop(t)
	app.ChooseMenu("Gallery", "Settings…")
	app.Find("id:settings-dialog")
	app.Press("id:settings-close")
	app.Advance(200 * time.Millisecond)
	app.WaitForGone("id:settings-dialog", time.Second)
	app.Resize(400, 800)
	app.ChooseMenu("Gallery", "Settings…")
	app.Find("id:settings-page")
}

// TestSettingsKey opens the settings with their key where no menu bar
// takes it.
func TestSettingsKey(t *testing.T) {
	app := startPhone(t)
	app.Key(settingsKey)
	app.Find("id:settings-page")
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

// TestThemeMenu chooses an appearance from the sidebar and keeps it, then
// follows the system again.
func TestThemeMenu(t *testing.T) {
	app := startDesktop(t)
	app.Press("id:theme-menu")
	app.Press("role:menuitem Dark")
	if got, _ := app.Storage().GetItem(themeKey); got != "dark" {
		t.Fatalf("kept %q, want dark", got)
	}
	app.Press("id:theme-menu")
	app.Press("role:menuitem System")
	if _, ok := app.Storage().GetItem(themeKey); ok {
		t.Error("following the system keeps a theme")
	}
}

func TestThemeRemembered(t *testing.T) {
	app := startDesktop(t, nsgotest.Storage(map[string]string{themeKey: "dark"}))
	app.Press("id:settings")
	app.Find("id:theme >> text:Dark")
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
	if got := wide.Find("id:shell").Style.Background; got != core.DefaultTheme().Dark.Colors["inset"] {
		t.Fatalf("sidebar = %x, want the dark inset", got)
	}
}

// TestLayoutsKeepTheDemo moves between the layouts with a demo open.
// TestUIOnlyMarks marks the cards of components base has no form of,
// inside their pages and on no other.
func TestUIOnlyMarks(t *testing.T) {
	app := startDesktop(t)
	open(app, "Button")
	if app.Exists("id:ui-only-Button") {
		t.Fatal("Button is marked though base has one")
	}
	open(app, "Badge")
	app.Find("id:demo-Badge >> id:ui-only-Badge")
}

// TestBaseListForm chooses a row of the List page's base form, whose card
// is not marked as ui's alone.
func TestBaseListForm(t *testing.T) {
	app := startDesktop(t)
	open(app, "List")
	if app.Exists("id:ui-only-List") {
		t.Fatal("List is marked though base has one")
	}
	showBase(app, "List")
	row := "id:stage-base.List >> role:option Grace Hopper"
	app.Press(row)
	if !app.Find(row).Props.Selected {
		t.Fatal("base.List did not choose the row pressed")
	}
	app.Key("down")
	if !app.Find("id:stage-base.List >> role:option Alan Turing").Props.Selected {
		t.Fatal("base.List's keys did not move the choice")
	}
}

// TestFormsCenter centers a form with a width of its own on its stage, and
// one without in a column of formWidth.
func TestFormsCenter(t *testing.T) {
	app := startDesktop(t)
	open(app, "Input")
	for _, name := range []string{"NumberInput", "base.NumberInput"} {
		if name == "base.NumberInput" {
			showBase(app, "NumberInput")
		}
		stage, box := app.Find("id:stage-"+name).Frame, app.Find("id:form-"+name).Frame
		if got := box.Size.Width; got >= stage.Size.Width-32 {
			t.Fatalf("%s is %v wide, the stage's whole width", name, got)
		}
		left := box.Origin.X - stage.Origin.X
		if right := stage.Size.Width - left - box.Size.Width; left-right > 1 || right-left > 1 {
			t.Fatalf("%s leaves %v and %v beside it", name, left, right)
		}
	}
	open(app, "Slider")
	stage, box := app.Find("id:stage-Slider").Frame, app.Find("id:form-Slider").Frame
	if want := min(stage.Size.Width-32, formWidth); box.Size.Width != want {
		t.Fatalf("the slider is %v wide, want %v", box.Size.Width, want)
	}
}

func TestLayoutsKeepTheDemo(t *testing.T) {
	app := startDesktop(t)
	open(app, "Checkbox")
	app.Resize(400, 800)
	app.Find("id:screen-Checkbox")
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
	open(app, "Checkbox")
	app.Press("role:switch Disabled >> nth=0")
	app.Resize(900, 800)
	if !app.Find("id:stage-Checkbox >> role:checkbox Starred").Props.Disabled {
		t.Fatal("the cards were built again")
	}
}

func TestPhoneRestoresDemo(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 844), nsgotest.Storage(map[string]string{lastDemo: "Checkbox"}))
	app.Find("id:screen-Checkbox")
	app.Back()
	app.Find("id:home")
	if _, ok := app.Storage().GetItem(lastDemo); ok {
		t.Fatal("the index still remembers a demo")
	}
}

// TestSidebarTabsFit keeps the small tabs centered in the sidebar's header,
// past the window controls and the toggle at its narrowest, in every
// language.
func TestSidebarTabsFit(t *testing.T) {
	for _, locale := range []string{"en-US", "zh-Hans", "ar-XB"} {
		env := desktopEnv
		env.Locale = locale
		env.TitleBar = core.Edges{Top: 28, Left: 78}
		app := startDesktop(t, nsgotest.Environment(env))
		head := app.Find("id:sidebar-header").Frame
		list := app.Find("id:tab-components").Parent.Frame
		const narrowest, clearOfToggle = 212, 12 + 78 + 40
		if room := core.Points(narrowest - clearOfToggle - 12); list.Size.Width > room {
			t.Fatalf("%s: the tabs are %v wide, with %v of room", locale, list.Size.Width, room)
		}
		if h := app.Find("id:tab-components").Frame.Size.Height; h != ui.SizeSmall.Height() {
			t.Fatalf("%s: a tab is %v tall, want the small size", locale, h)
		}
		if top, want := list.Origin.Y, (head.Size.Height-list.Size.Height)/2; top != want {
			t.Fatalf("%s: the tabs are %v down the header, want centered at %v", locale, top, want)
		}
		app.Press("id:tab-basics")
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

// TestControls uses a control of each page, each showing what it was set
// to.
func TestControls(t *testing.T) {
	app := startDesktop(t)
	open(app, "Checkbox")
	radio := "id:stage-RadioGroup · Radio >> role:radio Banana"
	app.Press(radio)
	if !app.Find(radio).Props.Checked {
		t.Fatal("the radio pressed is not checked")
	}
	checkbox := "id:stage-Checkbox >> role:checkbox Starred"
	app.Press(checkbox)
	if app.Find(checkbox).Props.Checked {
		t.Fatal("the checkbox stayed checked")
	}
	open(app, "Tabs")
	tab := "id:stage-Tabs >> role:tab Activity >> nth=0"
	app.Press(tab)
	if !app.Find(tab).Props.Selected {
		t.Fatal("the tab pressed is not selected")
	}
	open(app, "Breadcrumbs")
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
	app.Find("role:button Retry")
	// Reasoning folds itself when it ends, apart from the turn's steps.
	app.WaitForGone("text:/The quick answer is 0.10/", time.Second)
	app.Press("role:button Thought for")
	app.Find("text:/The quick answer is 0.10/")
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

func TestChatCallsATool(t *testing.T) {
	app := startPhone(t)
	openPhone(app, "AI Chat")
	app.Press("label:Search the web >> nth=0")
	app.Advance(20 * time.Second)
	app.Find("text:Cold starts are roughly")
	app.Press("role:button Worked for")
	app.Press("role:button Ran 1 step")
	app.Find("role:button Web search")
}

func TestChatWaitsForTheConnection(t *testing.T) {
	app := startPhone(t)
	openPhone(app, "AI Chat")
	app.Press("label:Go offline")
	app.Advance(time.Second)
	app.Find("text:You're offline")
	app.Advance(20 * time.Second)
	app.Find("text:Tuesday, November 3")
}

func TestChatStopsAReply(t *testing.T) {
	app := startPhone(t)
	openPhone(app, "AI Chat")
	app.Press("label:Tell me a short story")
	app.Advance(2 * time.Second)
	app.Press("role:button Stop")
	app.Find("text:You stopped this reply")
}

// fakeCLI puts a CLI on PATH that runs script, reading the prompt first.
func fakeCLI(t *testing.T, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake CLI is a shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\ncat >/dev/null\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestChatRunsCodex(t *testing.T) {
	fakeCLI(t, "codex", `echo '{"type":"item.completed","item":{"type":"agent_message","text":"Hello from Codex"}}'`)
	app := startDesktop(t)
	open(app, "AI Chat")
	app.Find("text:Chat with your local Claude or Codex CLI")
	app.Type("id:composer", "Hello")
	app.Press("role:button Send")
	app.WaitFor("text:Hello from Codex", 10*time.Second)
	app.WaitFor("role:button Copy", 10*time.Second)
}

func TestChatRunsClaude(t *testing.T) {
	fakeCLI(t, "claude", `echo '{"type":"stream_event","event":{"delta":{"type":"text_delta","text":"Hello from "}}}'
echo '{"type":"stream_event","event":{"delta":{"type":"text_delta","text":"Claude"}}}'
echo '{"type":"result","result":"Hello from Claude"}'`)
	app := startDesktop(t)
	open(app, "AI Chat")
	app.Press("text:Codex")
	app.Press("text:Claude")
	app.Type("id:composer", "Hello")
	app.Press("role:button Send")
	app.WaitFor("text:Hello from Claude", 10*time.Second)
}

func TestChatRetriesAFailedCLI(t *testing.T) {
	fakeCLI(t, "codex", `echo '{"type":"error","message":"not signed in"}'`)
	app := startDesktop(t)
	open(app, "AI Chat")
	app.Type("id:composer", "Hello")
	app.Press("role:button Send")
	app.WaitFor("text:not signed in", 10*time.Second)
	fakeCLI(t, "codex", `echo '{"type":"item.completed","item":{"type":"agent_message","text":"Back again"}}'`)
	app.Press("role:button Retry")
	app.WaitFor("text:Back again", 10*time.Second)
}

func TestChatStopsACLI(t *testing.T) {
	fakeCLI(t, "codex", `exec sleep 60`)
	app := startDesktop(t)
	open(app, "AI Chat")
	app.Type("id:composer", "Hello")
	app.Press("role:button Send")
	app.Press("role:button Stop")
	app.Find("text:You stopped this reply")
}

func TestConversationShowsItsParts(t *testing.T) {
	app := startPhone(t)
	openPhone(app, "Conversation")
	app.Find("text:error 529")
	app.Find("text:You stopped this reply")
	app.Find("text:Waiting for a connection")
	app.Find("text:Wrote 5 files")
	app.Press("text:Show more")
	app.Find("text:timeline.png")

	app.Find("id:conversation-panel")
	app.Press("role:button Remove file >> nth=0")
	app.Find("role:button Remove file")
	app.Find("role:button Stop")
	app.Press("role:button Stop")
	app.Find("role:button Send")
	app.Press("id:conversation-model")
	app.Press("role:button Deep")
	app.Find("id:conversation-model")
	app.Find("text:main")
}

func TestConversationShowsItsToolbarOnADesktop(t *testing.T) {
	app := startDesktop(t)
	open(app, "Conversation")
	app.Find("text:Standard")
	app.Find("text:Worktree")
	app.Find("text:MacBook Pro")
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
	app.Find("text:0.25")
}

// TestSystemSwitch shows the platform's switch in the Switch playground
// and turns it off, with the platform's input on macOS and iOS.
func TestSystemSwitch(t *testing.T) {
	var app *nsgotest.App
	switch nsgotest.Target() {
	case "ios_simulator":
		app = startPhone(t)
		openPhone(app, "Checkbox")
	case "", "macos":
		app = startDesktop(t)
		open(app, "Checkbox")
	default:
		t.Skip("macOS and iOS alone have a system switch")
	}
	app.Press("role:switch System")
	native := func() *nsgotest.View { return app.FindMounted("id:form-Switch").Children[0].Children[0] }
	if v := native(); v.Component != driver.SwitchComponent || v.NativeProps["checked"] != true {
		t.Fatalf("switch %q with %v, want the system's, on", v.Component, v.NativeProps)
	}
	sw := fmt.Sprintf("view:%d", native().ID)
	if nsgotest.Target() == "" {
		app.Emit(sw, "change", false)
	} else {
		app.Press(sw)
	}
	if native().NativeProps["checked"] != false {
		t.Fatal("the system switch stayed on")
	}
}

// TestWebInput clicks the inline page's button and link as the user,
// whose trusted click posts to the app and saves the page's note, where a
// platform gives web views trusted input and saves their downloads.
func TestWebInput(t *testing.T) {
	if nsgotest.Target() != "macos" {
		t.Skip("trusted web input and downloads are macOS's")
	}
	app := startDesktop(t)
	open(app, "WebView and Video")
	app.WaitFor("id:bridge", 10*time.Second)
	// The page posts where its button is as it loads.
	time.Sleep(2 * time.Second)
	app.Press("id:click-page")
	app.WaitFor("text:hello from the page", 10*time.Second)
	app.Press("id:save-note")
	app.WaitFor("text:note.txt, 19 B", 10*time.Second)
}

func TestSystemNotificationsAndBadge(t *testing.T) {
	headless(t)
	app := startDesktop(t)
	open(app, "System")
	app.Press("id:show-notification")
	app.Find("text:Shown: no")
	app.Press("id:ask-notifications")
	app.Find("text:granted")
	app.Press("id:show-notification")
	app.Find("text:Shown: yes")
	if len(app.Notifications) != 1 || app.Notifications[0].ID != "gallery" {
		t.Fatalf("notifications = %+v", app.Notifications)
	}
	app.PressNotification("gallery")
	app.Find("text:Pressed: gallery")
	app.Press("id:badge-up")
	app.Press("id:badge-up")
	if app.Badge != 2 {
		t.Errorf("badge = %d", app.Badge)
	}
	app.Press("id:badge-clear")
	if app.Badge != 0 {
		t.Errorf("badge after clearing = %d", app.Badge)
	}
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
	app.Find("text:Opened https://github.com/nsgo-dev/nsgo")
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
	app.WaitFor("text:"+filepath.Base(dir)+" · 1 item", 5*time.Second)
	if app.DirectoryPicks[0].Limit != 1 {
		t.Fatalf("pick %+v", app.DirectoryPicks[0])
	}
	// A folder without a path, as Android and the web pick, reads
	// through its FS.
	app.PickDirectories = func(core.DirectoriesRequest) []core.Directory {
		return []core.Directory{{Name: "Music", FS: fstest.MapFS{"a.flac": {}, "b/c.flac": {}}}}
	}
	app.Press("role:button Pick a folder")
	app.WaitFor("text:Music · 2 items", 5*time.Second)
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

// TestBaseForms shows a component's ui form, then its base form on the tab
// beside it, with base's behavior under the look its page writes out and
// one knob for a prop both have.
func TestBaseForms(t *testing.T) {
	app := startDesktop(t)
	open(app, "Button")
	app.Find("id:stage-Button")
	if app.Exists("id:stage-base.Button") {
		t.Fatal("the base form shows before its tab is chosen")
	}
	app.Press("role:switch Disabled >> nth=0")
	if !app.Find("id:button").Props.Disabled {
		t.Fatal("the Disabled knob left the ui form enabled")
	}
	showBase(app, "Button")
	if app.Exists("id:stage-Button") {
		t.Fatal("the ui form stayed beside the base form")
	}
	if !app.Find("id:base-button").Props.Disabled {
		t.Fatal("the Disabled knob left the base form enabled")
	}
	open(app, "Checkbox")
	showBase(app, "Switch")
	sw := "id:stage-base.Switch >> role:switch Starred"
	app.Press(sw)
	if app.Find(sw).Props.Checked {
		t.Fatal("base.Switch stayed on")
	}
	open(app, "Dialog")
	showBase(app, "Dialog")
	app.Press("id:stage-base.Dialog >> role:button Open")
	app.Find("id:base-dialog")
	open(app, "Menu")
	showBase(app, "Menu · Submenu")
	app.Press("id:stage-base.Menu >> role:button Open")
	app.Press("role:menuitem Copy")
	app.Find("text:base.Menu: Copy")
}

// TestPlaygrounds sets a prop and sees the component change, and a stage
// page keeps its panel beside the component until the pane is too narrow.
func TestPlaygrounds(t *testing.T) {
	app := startDesktop(t)
	open(app, "Button")
	if app.Find("id:button").Props.Disabled {
		t.Fatal("the button starts disabled")
	}
	for _, name := range []string{"Primary", "Secondary", "Outline", "Ghost", "Small", "Default", "Large"} {
		app.Find("id:stage-Button >> role:button " + name)
	}
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

// TestCharts hovers the week's plot, switches weeks with its knob and finds the
// sparklines, a plot per row of a list.
func TestCharts(t *testing.T) {
	app := startDesktop(t)
	open(app, "Plot")
	app.Find("text:Sun, the last day")
	plot := "role:image Visitors per day, from 1,820 on Monday to 1,180 on Sunday"
	app.Find(plot)
	// The pointer at the middle: Thursday, the fourth of seven days.
	app.Hover(plot)
	app.Find("text:2,480 visitors")
	app.Press("role:tab This week")
	app.Find("text:1,330 visitors")
	if len(app.Query("text:NSHL")) == 0 {
		t.Fatal("no sparkline rows")
	}
	if n := len(app.Query("role:image")); n < 5 {
		t.Fatalf("%d images, want the week's plot and a sparkline per row", n)
	}
	// A card for each of the other marks.
	app.Find("id:bars-plot")
	for _, title := range []string{"Bars", "Bridge", "Candles", "Share", "Gauge", "Scatter", "Heatmap", "Radar", "Flows", "Market"} {
		app.Find("role:image " + title)
	}
	app.Press("role:button New data >> nth=0")
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
	open(app, "Table")
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

// TestDiff counts the changes, stages and discards one, and shows the
// patch.
func TestDiff(t *testing.T) {
	app := startDesktop(t)
	open(app, "DiffView")
	app.Find("text:var ErrNotFound")
	app.Find("text:· 7 changes")
	app.OnAlert = func(core.AlertConfig) int { return 1 }
	app.Hover("text:var ErrNotFound")
	app.Press("role:button Stage")
	app.Find("text:· 6 changes")
	app.Find("text:Staged")
	app.Hover("text:/safe for concurrent use/ >> nth=0")
	app.Press("role:button Discard Changes…")
	app.Find("text:· 5 changes")
	app.Find("text:Discarded")
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
			// The market's random prices color its charts, and a shell
			// answers the terminal when it will.
			drawings := d.name != "Stocks" && d.name != "Terminal"
			shown := "id:demo-" + d.name
			app := startDesktop(t)
			open(app, d.name)
			app.FastForward(2 * time.Second)
			before := viewIDs(rootOf(app.Find(shown)))
			app.Press("id:theme-menu")
			app.Press("role:menuitem Dark")
			// Its status bar's tabs follow the appearance: they end their
			// slide.
			if d.name == "System" {
				app.Advance(200 * time.Millisecond)
			}
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
	for _, demo := range []string{"", "Checkbox"} {
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
// setting says its words in, and keeps it.
func TestLanguageSetting(t *testing.T) {
	app := startDesktop(t)
	app.Press("id:settings")
	app.Find("id:settings-dialog")
	app.Press("id:language")
	app.Press("text:简体中文")
	if got, _ := app.Storage().GetItem(languageKey); got != "zh-Hans" {
		t.Errorf("kept %q, want zh-Hans", got)
	}
	// The settings stay open in the new language.
	app.Find("text:设置")
	app.Press("id:language")
	app.Press("text:跟随系统")
	if _, ok := app.Storage().GetItem(languageKey); ok {
		t.Error("following the system keeps a language")
	}
	app.Find("text:Settings")
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
	env := desktopEnv
	env.Locale = "zh-CN"
	app := nsgotest.Start(t, Root, nsgotest.Size(1200, 800), nsgotest.Environment(env))
	open(app, "Checkbox")
	app.Find("id:nav-Checkbox >> text:复选框")
	app.Find("id:demo-Checkbox >> text:苹果 >> nth=0")
	open(app, "DataTable")
	app.Find("label:按ID排序")
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

// TestPseudoLocales starts in en-XA, which says the gallery's and base's
// messages accented and bracketed, then in ar-XB, which lays it out right
// to left.
func TestPseudoLocales(t *testing.T) {
	ltr := startDesktop(t).Find("id:settings").Frame.Origin.X
	env := desktopEnv
	env.Locale = i18n.PseudoAccents
	app := startDesktop(t, nsgotest.Environment(env))
	app.Press("id:settings")
	app.Find("text:[Śéťťíńğś ~~~]")
	env.Locale = i18n.PseudoBidi
	if got := startDesktop(t, nsgotest.Environment(env)).Find("id:settings").Frame.Origin.X; got <= ltr {
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
	// Once another field takes the focus, the terminal's keys leave it.
	app.Press("id:search")
	app.Type("id:search-dialog >> role:textbox", "ab")
	app.Key("backspace")
	if got := app.Find("id:search-dialog >> role:textbox").Text; got != "a" {
		t.Errorf("Backspace left the search %q: the terminal kept its keys", got)
	}
	app.Key("escape")
	app.FastForward(time.Second)
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

// Each picker's page shows its base form.
func TestPickerPages(t *testing.T) {
	for name, title := range map[string]string{"Select": "base.Combobox", "DatePicker": "base.DatePicker",
		"ColorPicker": "base.ColorPicker", "Popover": "base.HoverCard", "Command": "base.Command"} {
		t.Run(name, func(t *testing.T) {
			app := startDesktop(t)
			open(app, name)
			showBase(app, strings.TrimPrefix(title, "base."))
			app.Find("id:stage-" + title)
		})
	}
}

// A base form picks in the running gallery: a fruit, a color the ui form
// shows too, then a command.
func TestPickersPick(t *testing.T) {
	app := startDesktop(t)
	open(app, "Select")
	showBase(app, "Select")
	app.Press("id:stage-base.Select >> role:combobox")
	app.Press("role:option Apple")
	app.Find("id:stage-base.Select >> text:Apple")
	open(app, "ColorPicker")
	showBase(app, "ColorPicker")
	app.Clear("id:stage-base.ColorPicker >> label:Hex")
	app.Type("id:stage-base.ColorPicker >> label:Hex", "#ef4444\n")
	app.Press("id:package-ColorPicker >> role:tab Styled")
	if got := app.Find("id:stage-ColorPicker >> label:Hex color").Text; !strings.Contains(strings.ToLower(got), "ef4444") {
		t.Fatalf("the ui form shows %q", got)
	}
	open(app, "Command")
	showBase(app, "Command")
	app.Type("id:stage-base.Command >> label:Command", "sheet")
	app.Key("enter")
	app.Find("text:base.Command: sheet")
}

// The sidebar's search and its key find a demo in any language and go to it,
// starting over each time.
func TestSearchGoesToADemo(t *testing.T) {
	app := startDesktop(t)
	app.Press("id:search")
	app.Type("id:search-dialog >> role:textbox", "datatable")
	app.Key("enter")
	app.FastForward(time.Second)
	app.Find("id:demo-DataTable")
	app.Key("mod+k")
	if got := app.Find("id:search-dialog >> role:textbox").Text; got != "" {
		t.Fatalf("the search kept %q", got)
	}
	first := app.Find("id:search-dialog >> role:option Stocks").Style.Background
	app.Key("down")
	app.Settle()
	if app.Find("id:search-dialog >> role:option AI Chat").Style.Background != first {
		t.Fatal("Down did not move the highlight")
	}
	app.Key("up")
	app.Settle()
	if app.Find("id:search-dialog >> role:option Stocks").Style.Background != first {
		t.Fatal("Up did not move the highlight back")
	}
	app.Key("escape")
	app.FastForward(time.Second)
	if app.Exists("id:search-dialog") {
		t.Fatal("Escape left the search open")
	}
}

// ui's forms pick in the running gallery.
func TestPickerForms(t *testing.T) {
	app := startDesktop(t)
	open(app, "Select")
	app.Press("id:stage-Select >> role:combobox")
	app.Press("role:option Banana")
	app.Find("id:stage-Select >> text:Banana")
	app.Press("id:stage-Combobox >> role:button")
	app.Press("role:option Cherry")
	app.Find("id:stage-Combobox >> text:Cherry")
	open(app, "DatePicker")
	day := "id:stage-Calendar >> label:2026-01-15"
	app.Press(day)
	if !app.Find(day).Props.Selected {
		t.Fatal("the day pressed is not selected")
	}
	app.Press("id:stage-DatePicker >> role:combobox")
	// The page's calendar shows it too; the popup is the last.
	app.Press("label:2026-01-20 >> nth=1")
	app.Find("id:stage-DatePicker >> text:/2026/")
	open(app, "ColorPicker")
	app.Clear("id:stage-ColorPicker >> label:Hex color")
	app.Type("id:stage-ColorPicker >> label:Hex color", "22c55e\n")
	showBase(app, "ColorPicker")
	if got := app.Find("id:stage-base.ColorPicker >> label:Hex").Text; !strings.Contains(strings.ToLower(got), "22c55e") {
		t.Fatalf("the base form shows %q", got)
	}
	open(app, "Command")
	app.Press("id:stage-Command >> role:option Sheet")
	app.Find("text:Command: sheet")
	app.Press("id:open-command-dialog")
	app.Type("id:command-dialog >> role:textbox", "sheet")
	app.Key("enter")
	app.Find("text:CommandDialog: sheet")
}

func TestRichEditorDemo(t *testing.T) {
	app := startDesktop(t)
	open(app, "RichEditor")
	app.Find("text:Release notes")
	app.Find("role:toolbar Formatting")
	app.Press("text:Release notes")
	app.Type("role:textbox", " @al")
	app.Find("role:menuitem >> nth=0")
	app.Key("enter")
	if app.Exists("role:menuitem") {
		t.Fatal("the mention list stays after choosing")
	}
	app.Find("text:@Alan Turing")
	app.Press("role:toolbar Formatting >> role:button Undo")
	if app.Exists("text:@Alan Turing") {
		t.Fatal("undo left the mention")
	}
}

func TestRichEditorDemoDragsBlocks(t *testing.T) {
	app := startDesktop(t)
	open(app, "RichEditor")
	app.Press("text:Release notes")
	app.Settle()
	below := func() bool {
		return app.Find("text:Release notes").Frame.Origin.Y > app.Find(`text:"Type Markdown as you go: # and a space makes a heading, - a list, 1. a numbered one, > a quote, and @ mentions someone."`).Frame.Origin.Y
	}
	app.Drag("role:button Drag to move the block", core.Point{}, core.Point{Y: 40}, core.Point{Y: 90})
	app.Settle()
	if !below() {
		t.Fatal("the heading did not move below the paragraph after it")
	}
	app.Press("role:toolbar Formatting >> role:button Undo")
	app.Settle()
	if below() {
		t.Fatal("undo left the heading below")
	}
}

func TestRichEditorDemoOnPhones(t *testing.T) {
	app := startPhone(t)
	openPhone(app, "RichEditor")
	app.Find("text:Release notes")
	app.Find("id:rich-editor-content")
	app.Press("text:Release notes")
	app.Type("role:textbox", "!")
	if !strings.Contains(strings.Join(app.Texts(), "\n"), "!") {
		t.Fatal("what was typed does not show")
	}
}

// TestRichEditorTakesTyping lays a document out and types into it on the
// platform's text engine and input, under nsgo test -target.
func TestRichEditorTakesTyping(t *testing.T) {
	s, err := richeditor.NewSession(richeditor.SessionConfig{
		Extensions: append(richeditor.StarterKit(richeditor.StarterOptions{}), richeditor.TableKit(richeditor.TableOptions{})),
		Markdown:   "# Title\n\nSome **bold** text\n\n| a | b |\n| --- | --- |\n| c | d |\n"})
	if err != nil {
		t.Fatal(err)
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.SetTheme(core.DefaultTheme())
		return uirich.View{Editor: s, Label: "Doc", Style: core.Style{Width: core.Pt(360), Height: core.Pt(400)}}
	})
	app.Find("text:Title")
	if h := app.Find("text:Title").Frame.Size.Height; h < 30 {
		t.Fatalf("a heading's line is %v tall", h)
	}
	if a, d := app.Find(`text:"a"`).Parent, app.Find(`text:"d"`).Parent; a.Frame.Origin.X == d.Frame.Origin.X {
		t.Fatalf("the table's cells stack: %+v %+v", a.Frame, d.Frame)
	}
	app.Press("text:Some bold text")
	app.Settle()
	if _, ok := s.State().Selection.(*richeditor.TextSelection); !ok || s.State().Selection.FromPos().Parent().TextContent() != "Some bold text" {
		t.Fatalf("a press landed in %q", s.State().Selection.FromPos().Parent().TextContent())
	}
	app.Type("role:textbox", "!")
	if !strings.Contains(s.Text(), "!") {
		t.Fatalf("typing left %q", s.Text())
	}
	// An input method composes in the line while the document waits, and
	// its commit is one undo step.
	before := s.Text()
	changes := 0
	stop := s.Subscribe(func(_ *richeditor.State, trs []*richeditor.Transaction) {
		for _, tr := range trs {
			if tr.DocChanged() {
				changes++
			}
		}
	})
	defer stop()
	app.Compose("role:textbox", "ni")
	app.Find("text:Some bold text!ni")
	if s.Text() != before {
		t.Fatalf("the document did not wait for the composition: %q", s.Text())
	}
	app.Compose("role:textbox", "你")
	app.Find("text:Some bold text!你")
	app.Commit("role:textbox", "你")
	app.Settle()
	if !strings.Contains(s.Text(), "Some bold text!你") || app.Exists("text:Some bold text!你你") {
		t.Fatalf("the commit left %q", s.Text())
	}
	if changes != 1 {
		t.Fatalf("the commit took %d changes", changes)
	}
}

// TestWebEvaluate runs a script in the inline page, a promise it settles,
// and shows its result as JSON: answered by the platform's web view on a
// target, by the test as the page would headless.
func TestWebEvaluate(t *testing.T) {
	app := startDesktop(t)
	open(app, "WebView and Video")
	app.WaitFor("id:bridge", 10*time.Second)
	if nsgotest.Target() != "" {
		// The page loads before it can answer.
		time.Sleep(2 * time.Second)
	}
	app.Press("id:run-script")
	if nsgotest.Target() == "" {
		app.AnswerScript("id:bridge", `{"elements":true,"sum":42}`, "")
	}
	app.WaitFor(`text:{"elements":true,"sum":42}`, 10*time.Second)
}
