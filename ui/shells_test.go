package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// windowEnv is a macOS window whose hidden title bar keeps 80 points for
// the traffic lights.
var windowEnv = func() core.Environment {
	e := desktopEnv
	e.TitleBar = core.Edges{Top: 28, Left: 80}
	return e
}()

// shell runs content under ui's theme alone, filling the
// window, as an app's root does, and returns the colors it shows.
func shell(t *testing.T, e core.Environment, content func(cx *core.Scope) core.Node) (*nsgotest.App, Palette) {
	t.Helper()
	var shown Palette
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.SetTheme(DefaultTheme())
		shown = core.Untracked(cx, ColorsOf(cx).Now)
		return core.Func(content)
	}, size(e), nsgotest.Environment(e))
	return app, shown
}

// within finds the view under v with testID, seen or not.
func within(v *nsgotest.View, testID string) *nsgotest.View {
	if v.Props.TestID == testID {
		return v
	}
	for _, c := range v.Children {
		if f := within(c, testID); f != nil {
			return f
		}
	}
	return nil
}

// x is v's distance from the start of its ancestor matching selector.
func x(app *nsgotest.App, selector string, v *nsgotest.View) core.Points {
	top := app.Find(selector)
	var out core.Points
	for ; v != nil && v != top; v = v.Parent {
		out += v.Frame.Origin.X
	}
	return out
}

// y is v's distance from the top of its ancestor matching selector.
func y(app *nsgotest.App, selector string, v *nsgotest.View) core.Points {
	top := app.Find(selector)
	var out core.Points
	for ; v != nil && v != top; v = v.Parent {
		out += v.Frame.Origin.Y
	}
	return out
}

// TestSidebarShell keeps the toggle after the window controls, slides
// the content over the sidebar, which keeps its width, and resizes the
// sidebar for the app.
func TestSidebarShell(t *testing.T) {
	open := true
	var width core.Points = 240
	app, p := shell(t, windowEnv, func(cx *core.Scope) core.Node {
		o, w := core.NewSignal(cx, open), core.NewSignal(cx, width)
		return SidebarShell{TestID: "shell", ToggleLabel: "Toggle sidebar", ResizeLabel: "Resize sidebar",
			Open: o.Get, OnOpenChange: func(v bool) { open = v; o.Set(v) },
			Width: w.Get, OnWidthChange: func(v core.Points) { width = v; w.Set(v) },
			Sidebar:       []core.Node{Sidebar{TestID: "nav", Sections: []SidebarSection{{Items: []SidebarItem{{ID: "a", Text: "Inbox"}}}}}},
			SidebarFooter: []core.Node{Button{Text: "Settings"}},
			Header:        []core.Node{core.Text{Content: "Inbox", TestID: "title"}},
			Actions:       []core.Node{Button{Text: "Compose"}},
			Content:       []core.Node{core.Text{Content: "Body"}}}
	})
	toggleX := func() core.Points { return x(app, "shell", app.Find("role:button Toggle sidebar")) }
	if got := toggleX(); got != 80 {
		t.Fatalf("toggle at %v, want right after the window controls", got)
	}
	if got := app.Find("sidebar-header").Frame.Size.Height; got != HeaderHeight {
		t.Fatalf("header height = %v, want %v", got, HeaderHeight)
	}
	if got := app.Find("shell").Style.Background; got != p.Inset {
		t.Fatalf("chrome = %x, want the inset color without a material", got)
	}
	app.Find("sidebar-panel >> role:button Settings")
	if got := app.Find("sidebar-panel").Frame.Size.Width; got != 240 {
		t.Fatalf("sidebar width = %v, want 240", got)
	}
	if got := x(app, "shell", app.Find("title")); got != 240+12 {
		t.Fatalf("title at %v, want at the content's start", got)
	}
	app.AccessibilityAction("role:adjustable Resize sidebar", "increment")
	if width != 250 {
		t.Fatalf("width = %v after a step, want 250", width)
	}

	app.Press("role:button Toggle sidebar")
	if open {
		t.Fatal("the toggle left the sidebar open")
	}
	if got := within(app.Find("shell"), "sidebar-slot").Frame.Size.Width; got != 0 {
		t.Fatalf("slot = %v, want closed", got)
	}
	if got := within(app.Find("shell"), "sidebar-panel").Frame.Size.Width; got != 250 {
		t.Fatalf("hiding sidebar = %v wide, want its width kept", got)
	}
	if got := toggleX(); got != 80 {
		t.Fatalf("toggle moved to %v", got)
	}
	if want := core.Points(80 + 32 + 8); x(app, "shell", app.Find("title")) != want {
		t.Fatalf("title at %v, want %v, after the toggle", x(app, "shell", app.Find("title")), want)
	}
	app.Advance(time.Second)
	if app.Exists("sidebar-header") || app.Exists("role:adjustable Resize sidebar") {
		t.Fatal("the hidden sidebar is still seen")
	}
	app.Press("role:button Toggle sidebar")
	if !open || within(app.Find("shell"), "sidebar-slot").Frame.Size.Width != 250 {
		t.Fatal("the sidebar came back without its width")
	}
}

// TestSidebarShellScrolls keeps a sidebar taller than the window within
// it, its items scrolling under the sidebar's header.
func TestSidebarShellScrolls(t *testing.T) {
	var items []SidebarItem
	for i := range 40 {
		items = append(items, SidebarItem{ID: fmt.Sprint(i), Text: fmt.Sprint("Item ", i), TestID: fmt.Sprint("item-", i)})
	}
	app, _ := shell(t, windowEnv, func(cx *core.Scope) core.Node {
		return SidebarShell{TestID: "shell", Sidebar: []core.Node{Sidebar{TestID: "nav", Sections: []SidebarSection{{Items: items}}}}}
	})
	if got := app.Find("nav").Frame.Size.Height; got != 760-HeaderHeight {
		t.Fatalf("sidebar = %v tall, want the window's height under its header", got)
	}
	if got := y(app, "shell", app.Find("item-39")); got < 760 {
		t.Fatalf("the last item at %v without scrolling, want below the window", got)
	}
	app.Scroll("nav", 10000)
	scroll := app.Find("nav")
	if got, want := scroll.Offset.Y, scroll.Content.Height-scroll.Frame.Size.Height; got != want {
		t.Fatalf("scrolled to %v, want its end at %v", got, want)
	}
}

// TestShellMaterial leaves the chrome to a window material and keeps
// the content opaque.
func TestShellMaterial(t *testing.T) {
	e := windowEnv
	e.WindowMaterial = core.WindowMaterialSidebar
	app, p := shell(t, e, func(cx *core.Scope) core.Node {
		return SidebarShell{TestID: "shell", Content: []core.Node{core.Text{Content: "Body"}}}
	})
	if got := app.Find("shell").Style.Background; got != 0 {
		t.Fatalf("chrome = %x, want the material", got)
	}
	if got := app.Find("content-header").Style.Background; got != p.Background {
		t.Fatalf("content header = %x, want the background", got)
	}
}

// TestPlainShell shows the title in the header, clear of the window
// controls at both ends, and the content below it or under it.
func TestPlainShell(t *testing.T) {
	for _, behind := range []bool{false, true} {
		e := windowEnv
		e.TitleBar.Right = 40
		app, _ := shell(t, e, func(cx *core.Scope) core.Node {
			return PlainShell{TestID: "shell", Title: "Notes", BehindHeader: behind,
				Actions: []core.Node{Button{Text: "Share"}},
				Content: []core.Node{core.View{TestID: "body", Style: core.Style{FlexGrow: 1}}}}
		})
		lead, share := app.Find("shell-header >> role:header Notes").Parent.Frame, app.Find("shell-header >> role:button Share").Frame
		if end := 1000 - share.Origin.X - share.Size.Width; lead.Origin.X != 80+12 || end != 40+12 {
			t.Fatalf("header starts at %v and ends %v before the edge, want clear of the window controls", lead.Origin.X, end)
		}
		top := app.Find("body").Parent.Frame.Origin.Y
		if !behind && top != HeaderHeight || behind && top != 0 {
			t.Fatalf("content top = %v with BehindHeader %v", top, behind)
		}
	}
}

// TestShellTitleBar hides the title bar for the header on its own, in
// every shell.
func TestShellTitleBar(t *testing.T) {
	for name, content := range map[string]core.Node{"sidebar": SidebarShell{}, "plain": PlainShell{}} {
		app, _ := shell(t, windowEnv, func(cx *core.Scope) core.Node { return content })
		var bar *core.WindowCommand
		for i := range app.Window {
			if app.Window[i].Action == core.WindowSetTitleBar {
				bar = &app.Window[i]
			}
		}
		if bar == nil || bar.TitleBar != core.TitleBarHidden || bar.Height != HeaderHeight {
			t.Fatalf("%s shell's title bar command = %+v, want hidden for %v", name, bar, HeaderHeight)
		}
	}
}

// TestShellPhone keeps the header clear of the status bar and leaves the
// window alone.
func TestShellPhone(t *testing.T) {
	e := phoneEnv
	e.SafeArea = core.Edges{Top: 47, Bottom: 34}
	app, _ := shell(t, e, func(cx *core.Scope) core.Node { return PlainShell{Title: "Notes"} })
	if len(app.Window) != 0 {
		t.Fatalf("window commands on a phone: %+v", app.Window)
	}
	if got := app.Find("shell-header >> role:header Notes").Parent.Frame.Origin.Y; got != 47 {
		t.Fatalf("header top = %v, want the safe area", got)
	}
}

// TestTabShellSystemBar takes the system's bar on a phone where every tab
// names its icon, and the drawn bar elsewhere.
func TestTabShellSystemBar(t *testing.T) {
	selected := 0
	tabs := func(cx *core.Scope) core.Node {
		sel := core.NewSignal(cx, 0)
		return TabShell{Label: "Sections", Selected: sel.Get, OnChange: func(i int) { selected = i; sel.Set(i) }, Tabs: []ShellTab{
			{Text: "Home", SystemIcon: "house", Content: func(cx *core.Scope) core.Node { return core.Text{Content: "Home page"} }},
			{Text: "Search", SystemIcon: "magnifyingglass", Content: func(cx *core.Scope) core.Node { return core.Text{Content: "Search page"} }},
		}}
	}
	phone := core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US"}
	app, _ := shell(t, phone, tabs)
	if app.Exists("role:tablist Sections") {
		t.Fatal("a phone draws the bar")
	}
	app.Emit("id:tab-bar", "change", float64(1))
	if selected != 1 || !app.Exists("Search page") {
		t.Fatal("the system's bar did not switch sections")
	}
	wide, _ := shell(t, desktopEnv, tabs)
	wide.Find("role:tablist Sections")
}

// TestTabShell switches sections, keeping the first built while hidden,
// and hides the bar under the keyboard.
func TestTabShell(t *testing.T) {
	built, selected := 0, 0
	tabs := func(cx *core.Scope) core.Node {
		sel := core.NewSignal(cx, 0)
		return TabShell{Label: "Sections", Selected: sel.Get, OnChange: func(i int) { selected = i; sel.Set(i) }, Tabs: []ShellTab{
			{Text: "Home", Content: func(cx *core.Scope) core.Node {
				built++
				return core.Text{Content: "Home page"}
			}},
			{Text: "Search", Content: func(cx *core.Scope) core.Node { return core.Text{Content: "Search page"} }},
		}}
	}
	app, p := shell(t, desktopEnv, tabs)
	app.Find("Home page")
	if app.Exists("Search page") {
		t.Fatal("a section shows before it is selected")
	}
	app.Press("role:tablist Sections >> role:tab Search")
	if got := app.Find("role:tab Search >> Search").TextStyle.Color; got != p.Accent {
		t.Fatalf("selected tab = %x, want the accent", got)
	}
	if selected != 1 || !app.Exists("Search page") || app.Exists("Home page") {
		t.Fatal("the tab did not switch sections")
	}
	app.Press("role:tab Home")
	if app.Find("Home page"); built != 1 {
		t.Fatalf("the first section was built %d times, want once", built)
	}
	e := desktopEnv
	e.KeyboardHeight = 300
	app.SetEnvironment(e)
	if app.Exists("role:tablist Sections") {
		t.Fatal("the bar shows under the keyboard")
	}

	// A phone shows the drawn bar too, not the system's.
	selected = 0
	app, _ = shell(t, phoneEnv, tabs)
	app.Press("role:tablist Sections >> role:tab Search")
	if selected != 1 || !app.Exists("Search page") {
		t.Fatal("the phone's tab did not switch sections")
	}
}

// The content slides over the right panel, which stays against the window's
// edge, and draws the panel's handle on its own edge.
func TestSidebarShellRightPanel(t *testing.T) {
	var set func(bool)
	app, _ := shell(t, windowEnv, func(cx *core.Scope) core.Node {
		o := core.NewSignal(cx, true)
		set = o.Set
		return SidebarShell{Content: []core.Node{core.View{Style: FillRest}}, Right: []core.Node{core.View{Style: FillRest}},
			RightOpen: o.Get, OnRightOpenChange: o.Set, RightResizeLabel: "Resize",
			RightWidth: func() core.Points { return 300 }, TestID: "shell"}
	})
	app.Settle()
	if got := x(app, "shell", app.Find("id:right-resize")) + 3; got != x(app, "shell", app.Find("id:right-slot")) {
		t.Fatalf("the handle ends at %v, not at the panel", got)
	}
	app.Post(func() { set(false) })
	app.Advance(sidebarSlide / 2)
	if got := within(app.Find("shell"), "right-slot").Frame.Size.Width; got != 0 {
		t.Fatalf("slot %v wide", got)
	}
	if got := within(app.Find("shell"), "right-panel").Frame.Size.Width; got != 300 {
		t.Fatalf("the panel narrowed to %v as the content slid over it", got)
	}
	app.Advance(sidebarSlide)
	if within(app.Find("shell"), "right-panel") != nil {
		t.Fatal("the panel stayed after the slide")
	}
}

// TestTabShellArrows moves between the tabs with the arrows, one Tab stop,
// where the system's tab bar is not available.
func TestTabShellArrows(t *testing.T) {
	var at *core.Signal[int]
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		at = core.NewSignal(cx, 0)
		page := func(*core.Scope) core.Node { return core.View{} }
		return TabShell{Tabs: []ShellTab{{Text: "One", Content: page}, {Text: "Two", Content: page}}, Selected: at.Get, OnChange: at.Set}
	})
	if !app.Find("role:tab One").Props.Focusable || app.Find("role:tab Two").Props.Focusable {
		t.Fatal("the tab bar is not one stop of the focus")
	}
	app.Key("tab")
	app.Key("right")
	if at.Get() != 1 {
		t.Fatalf("Right moved to tab %d", at.Get())
	}
}
