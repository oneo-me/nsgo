// Command gallery shows what NSGo apps are made of: core's elements and
// services and base's unstyled components, a demo each, in NSH's gallery's
// design, under three tabs. A wide window shows them atop a sidebar of the
// demos beside the selected one; a narrow one shows them, and the settings,
// in a bar at the bottom, with demos pushed over it all. Its look and
// components are package ui's, as NSH's are its nsh-gallery-ui's.
//
// It translates NSH's examples/gallery demo by demo, and departs from it
// where NSGo differs: icons are Lucide's (core.Icon) rather than glyphs
// drawn in canvases; NSH's Tailwind and standard-module demos have no
// NSGo counterpart, while Layout, Table and Accessibility show what NSGo
// has beyond NSH's gallery. Its index is laid out as NSH's after the 0.3.0
// release, a demo per element, listing the components NSGo has.
package main

import (
	_ "embed"
	"github.com/oneo-me/nsgo/icons/lucide"
	"os"
	"slices"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/i18n"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

//go:embed logo.png
var logoPNG []byte

// demo is an entry of the index: name is its ID, in English, and title
// and detail say it in a language.
type demo struct {
	name          string
	title, detail func(locales.Messages) string
	icon          func() *core.SVG
	screen        func(cx *core.Scope) core.Node
}

// group is demos by what they show: name is its ID, in English.
type group struct {
	name  string
	title func(locales.Messages) string
	demos []demo
}

// tab is a top-level section of the demos: id names it in tests, icon is
// its Lucide icon, and ios and android the system's for the system's tab
// bar.
type tab struct {
	id           string
	title        func(locales.Messages) string
	icon         func() *core.SVG
	ios, android string
	groups       []group
}

// tabs are the demos: the apps built on NSGo, base's components and ui's
// own by name, and core's primitives from the simplest with styling,
// motion and the platform's services. They are set in init because demos
// list them.
var tabs []tab

// settingsTab is the narrow layout's tab after the demos'.
func settingsTab() int { return len(tabs) }

func init() {
	primitives := []demo{
		{"Text", locales.Messages.DemoText, locales.Messages.DemoTextDetail, lucide.Type, textDemo},
		{"Image", locales.Messages.DemoImage, locales.Messages.DemoImageDetail, lucide.Image, imagesDemo},
		{"Scroll", locales.Messages.DemoScroll, locales.Messages.DemoScrollDetail, lucide.ScrollText, scrollingDemo},
		{"List", locales.Messages.DemoList, locales.Messages.DemoListDetail, lucide.List, listsDemo},
		{"Canvas", locales.Messages.DemoCanvas, locales.Messages.DemoCanvasDetail, lucide.PenTool, canvasDemo},
		{"Shader", locales.Messages.DemoShader, locales.Messages.DemoShaderDetail, lucide.Sparkles, shaderDemo},
		{"WebView and Video", locales.Messages.DemoWebViewAndVideo, locales.Messages.DemoWebViewAndVideoDetail, lucide.Film, mediaDemo},
		{"Overlay", locales.Messages.DemoOverlay, locales.Messages.DemoOverlayDetail, lucide.Layers, eventsDemo(overlayCard)},
	}
	components := []demo{
		{"Checkbox", locales.Messages.DemoCheckbox, locales.Messages.DemoCheckboxDetail, lucide.SquareCheck, controlsDemo(controls.filter)},
		{"Collapsible", locales.Messages.DemoCollapsible, locales.Messages.DemoCollapsibleDetail, lucide.ChevronsUpDown, controlsDemo(controls.disclosure)},
		{"DataTable", locales.Messages.DemoDataTable, locales.Messages.DemoDataTableDetail, lucide.Table, dataTableDemo},
		{"DiffView", locales.Messages.DemoDiffView, locales.Messages.DemoDiffViewDetail, lucide.FileDiff, diffDemo},
		{"Dock", locales.Messages.DemoDock, locales.Messages.DemoDockDetail, lucide.AppWindow, dockDemo},
		{"Editor", locales.Messages.DemoEditor, locales.Messages.DemoEditorDetail, lucide.Code, editorDemo},
		{"FileIcon", locales.Messages.DemoFileIcon, locales.Messages.DemoFileIconDetail, lucide.File, eventsDemo(fileIconCard)},
		{"HoldToConfirm", locales.Messages.DemoHoldToConfirm, locales.Messages.DemoHoldToConfirmDetail, lucide.Trash, eventsDemo(holdCard)},
		{"Input", locales.Messages.DemoInput, locales.Messages.DemoInputDetail, lucide.TextCursorInput, formDemo},
		{"Menu", locales.Messages.DemoMenu, locales.Messages.DemoMenuDetail, lucide.Menu, menusDemo},
		{"NavStack", locales.Messages.DemoNavStack, locales.Messages.DemoNavStackDetail, lucide.PanelsTopLeft, navigationDemo},
		{"Plot", locales.Messages.DemoPlot, locales.Messages.DemoPlotDetail, lucide.ChartSpline, chartsDemo},
		{"Popover", locales.Messages.DemoPopover, locales.Messages.DemoPopoverDetail, lucide.MessageSquare, eventsDemo(popoverCard)},
		{"Progress", locales.Messages.DemoProgress, locales.Messages.DemoProgressDetail, lucide.LoaderCircle, controlsDemo(controls.transfers)},
		{"Radio", locales.Messages.DemoRadio, locales.Messages.DemoRadioDetail, lucide.CircleDot, controlsDemo(controls.plansCard)},
		{"Resizable", locales.Messages.DemoResizable, locales.Messages.DemoResizableDetail, lucide.PanelLeft, resizableDemo},
		{"Select", locales.Messages.DemoSelect, locales.Messages.DemoSelectDetail, lucide.ChevronDown, eventsDemo(selectCard)},
		{"Sheet", locales.Messages.DemoSheet, locales.Messages.DemoSheetDetail, lucide.PanelBottom, eventsDemo(sheetCard)},
		{"Slider", locales.Messages.DemoSlider, locales.Messages.DemoSliderDetail, lucide.SlidersHorizontal, eventsDemo(sliderCard)},
		{"Spinner", locales.Messages.DemoSpinner, locales.Messages.DemoSpinnerDetail, lucide.RefreshCw, eventsDemo(spinnerCard)},
		{"Stepper", locales.Messages.DemoStepper, locales.Messages.DemoStepperDetail, lucide.Check, stepperDemo},
		{"Switch", locales.Messages.DemoSwitch, locales.Messages.DemoSwitchDetail, lucide.ToggleRight, controlsDemo(controls.settings)},
		{"Table", locales.Messages.DemoTable, locales.Messages.DemoTableDetail, lucide.Table, tableDemo},
		{"Tabs", locales.Messages.DemoTabs, locales.Messages.DemoTabsDetail, lucide.NotebookTabs, controlsDemo(controls.sections)},
		{"Terminal", locales.Messages.DemoTerminal, locales.Messages.DemoTerminalDetail, lucide.Terminal, terminalDemo},
		{"TextView", locales.Messages.DemoTextView, locales.Messages.DemoTextViewDetail, lucide.FileText, textViewDemo},
		{"Toast", locales.Messages.DemoToast, locales.Messages.DemoToastDetail, lucide.Bell, eventsDemo(toastCard, styledToastsCard)},
		{"ToggleButton", locales.Messages.DemoToggleButton, locales.Messages.DemoToggleButtonDetail, lucide.SlidersHorizontal, eventsDemo(toggleCard)},
		{"Tooltip", locales.Messages.DemoTooltip, locales.Messages.DemoTooltipDetail, lucide.MessageSquareText, eventsDemo(tooltipCard)},
	}
	basics := []group{
		{"Primitives", locales.Messages.GroupPrimitives, primitives},
		{"Styling", locales.Messages.GroupStyling, []demo{
			{"Styles", locales.Messages.DemoStyles, locales.Messages.DemoStylesDetail, lucide.Palette, stylesDemo},
			{"Layout", locales.Messages.DemoLayout, locales.Messages.DemoLayoutDetail, lucide.Columns2, layoutDemo},
			{"Patterns", locales.Messages.DemoPatterns, locales.Messages.DemoPatternsDetail, lucide.Component, componentsDemo},
		}},
		{"Motion", locales.Messages.GroupMotion, []demo{
			{"Animations", locales.Messages.DemoAnimations, locales.Messages.DemoAnimationsDetail, lucide.Zap, animationsDemo},
			{"Gestures", locales.Messages.DemoGestures, locales.Messages.DemoGesturesDetail, lucide.Hand, gesturesDemo},
			{"Physics", locales.Messages.DemoPhysics, locales.Messages.DemoPhysicsDetail, lucide.Atom, physicsDemo},
			{"Collapsing header", locales.Messages.DemoCollapsingHeader, locales.Messages.DemoCollapsingHeaderDetail, lucide.PanelTop, collapsingDemo},
		}},
		{"Platform", locales.Messages.GroupPlatform, []demo{
			{"System", locales.Messages.DemoSystem, locales.Messages.DemoSystemDetail, lucide.Cpu, systemDemo},
			{"Storage", locales.Messages.DemoStorage, locales.Messages.DemoStorageDetail, lucide.Database, storageDemo},
			{"Accessibility", locales.Messages.DemoAccessibility, locales.Messages.DemoAccessibilityDetail, lucide.Accessibility, accessibilityDemo},
		}},
	}
	apps := []demo{
		{"Stocks", locales.Messages.DemoStocks, locales.Messages.DemoStocksDetail, lucide.ChartCandlestick, stocksDemo},
		{"AI Chat", locales.Messages.DemoAIChat, locales.Messages.DemoAIChatDetail, lucide.MessageCircle, chatDemo},
		{"Photos", locales.Messages.DemoPhotos, locales.Messages.DemoPhotosDetail, lucide.Images, photosDemo},
	}
	tabs = []tab{
		{"apps", locales.Messages.GroupApps, lucide.LayoutGrid, "square.grid.2x2", "android:ic_menu_gallery",
			[]group{{"Apps", locales.Messages.GroupApps, apps}}},
		{"components", locales.Messages.TabComponents, lucide.Component, "puzzlepiece.extension", "android:ic_menu_agenda",
			[]group{{"Components", locales.Messages.GroupComponents, components}}},
		{"basics", locales.Messages.TabBasics, lucide.Shapes, "square.on.circle", "android:ic_menu_sort_by_size", basics},
	}
}

// allGroups is every group, in the tabs' order.
func allGroups() []group {
	var out []group
	for _, t := range tabs {
		out = append(out, t.groups...)
	}
	return out
}

// allDemos is every demo, in the index's order.
func allDemos() []demo {
	var out []demo
	for _, g := range allGroups() {
		out = append(out, g.demos...)
	}
	return out
}

// findDemo returns the demo named name.
func findDemo(name string) (demo, bool) {
	for _, d := range allDemos() {
		if d.name == name {
			return d, true
		}
	}
	return demo{}, false
}

// tabOf is the index of the tab holding the demo named name.
func tabOf(name string) int {
	for i, t := range tabs {
		for _, g := range t.groups {
			for _, d := range g.demos {
				if d.name == name {
					return i
				}
			}
		}
	}
	return 0
}

// groupOf is the title of the group holding the demo named name.
func groupOf(tr locales.Messages, name string) string {
	for _, g := range allGroups() {
		for _, d := range g.demos {
			if d.name == name {
				return g.title(tr)
			}
		}
	}
	return tr.AppGallery()
}

// route is a screen of the navigation stack: the index (no demo, no
// page), a demo, or a page a demo pushes.
type route struct {
	demo string
	page func(cx *core.Scope) base.NavPage
}

// gallery is the state the layouts share, so switching between them (a
// window resized, the appearance changed) keeps what is open.
type gallery struct {
	nav *base.Nav[route]
	// tab is the index of the tab shown: of tabs, or settingsTab in the
	// narrow layout.
	tab *core.Signal[int]
	// selected is the demo the desktop layout shows, and picked the one
	// each tab shows when selected again.
	selected *core.Signal[string]
	picked   []string
	// sidebarOpen is whether the desktop layout shows its sidebar, and
	// sidebarWidth how wide the user made it.
	sidebarOpen  *core.Signal[bool]
	sidebarWidth *core.Signal[core.Points]
	// chosen is the label of the last application menu item chosen.
	chosen *core.Signal[string]
	// language is the one chosen in the settings, "" the system's.
	language *core.Signal[string]
	// settingsOpen is whether the settings show, and settingsCategory and
	// settingsQuery what they show.
	settingsOpen                    *core.Signal[bool]
	settingsCategory, settingsQuery *core.Signal[string]
	// wasDesktop is whether the desktop layout has been shown, for the
	// switch back.
	wasDesktop bool
	desktop    func() bool
	env        *core.Env
	toasts     *toaster
}

// shell is the running gallery's state, which demos reach for the layout
// they are in and to push pages. One process runs one gallery.
var shell *gallery

// titleBar is what the window's controls take of the narrow layout's
// headers, which reach the window's top; the desktop layout's shell keeps
// clear of them itself.
func (g *gallery) titleBar(cx *core.Scope) core.Edges {
	if g.desktop() {
		return core.Edges{}
	}
	return desktop.TitleBar(cx)
}

// paneWidth is the width of the desktop layout's content pane, right of
// the sidebar.
func (g *gallery) paneWidth() core.Points {
	w := g.env.Window().Width
	if g.sidebarOpen.Get() {
		w -= g.sidebarWidth.Get()
	}
	return w
}

// push shows a page over the demo.
func (g *gallery) push(page func(cx *core.Scope) base.NavPage) { g.nav.Push(route{page: page}) }

// selectDemo shows a demo and its tab in the desktop layout, closing what
// it had pushed.
func (g *gallery) selectDemo(name string) {
	if g.nav.Depth() > 0 {
		g.nav.PopToRoot()
	}
	t := tabOf(name)
	g.picked[t] = name
	g.tab.Set(t)
	g.selected.Set(name)
}

// pushDemo shows a demo in the narrow layout, over its tab.
func (g *gallery) pushDemo(name string) {
	g.tab.Set(tabOf(name))
	g.nav.SetRoutes(route{}, route{demo: name})
}

// open shows a demo in the layout shown: selected beside the sidebar, or
// pushed over its tab.
func (g *gallery) open(name string) {
	if g.desktop() {
		g.selectDemo(name)
		return
	}
	g.pushDemo(name)
}

// lastDemo is the storage key of the demo left open, shown again on the
// next launch.
const lastDemo = "lastDemo"

// Root builds the gallery again when the window's width calls for the
// other layout, keeping its state in shell; a change of theme or
// appearance restyles it in place.
// NSGO_GALLERY_OPEN names a demo to open at start, for screenshots.
func Root(cx *core.Scope) core.Node {
	cx.SetTheme(ui.Theme())
	env := cx.Env()
	var picked []string
	for _, t := range tabs {
		picked = append(picked, t.groups[0].demos[0].name)
	}
	g := &gallery{
		nav:              base.NewNav(cx, route{}),
		tab:              core.NewSignal(cx, 0),
		selected:         core.NewSignal(cx, picked[0]),
		picked:           picked,
		sidebarOpen:      core.NewSignal(cx, true),
		sidebarWidth:     core.NewSignal(cx, sidebarWidth),
		chosen:           core.NewSignal(cx, locales.For(cx).AppNothingYet()),
		language:         core.NewSignal(cx, ""),
		settingsOpen:     core.NewSignal(cx, false),
		settingsCategory: core.NewSignal(cx, ""),
		settingsQuery:    core.NewSignal(cx, ""),
		env:              env,
		toasts:           newToaster(cx),
	}
	desktopLayout := core.NewMemo(cx, func() bool {
		limit := tabletWidth
		if env.Desktop() {
			limit = desktopWidth
		}
		return env.Window().Width >= limit
	})
	g.desktop = desktopLayout.Get
	shell = g

	hasWindow := desktop.IsAvailable(cx)
	if hasWindow {
		desktop.SetTitle(cx, locales.For(cx).AppGallery())
		desktop.SetTitleBar(cx, desktop.TitleBarHidden, ui.HeaderHeight)
	}
	rememberDemo(cx, g)
	rememberLanguage(cx, g)
	rememberAppearance(cx)
	cx.Effect(func() { desktop.SetApplicationMenu(cx, g.menu(locales.For(cx))) })

	return core.Dynamic(func(cx *core.Scope) core.Node {
		wide := desktopLayout.Get()
		// A new language builds the gallery again in it, as a new layout
		// does, keeping what is open.
		i18n.Language(cx)
		// What the layout builds mounts untracked, outside this function.
		return core.Func(func(cx *core.Scope) core.Node {
			colors := ui.ColorsOf(cx)
			var children []core.Node
			if wide {
				children = []core.Node{g.desktopLayout(cx), g.settingsDialog(cx)}
			} else {
				children = []core.Node{g.phone(cx)}
			}
			children = append(children, g.toasts.view(wide))
			return core.View{StyleFunc: func() core.Style {
				return fillPage.Merge(core.Style{Background: ui.Chrome(cx), Color: colors.Text()})
			}, Children: children}
		})
	})
}

// rememberDemo reopens the demo left open last time (selected on a
// desktop, pushed on a phone), then keeps the key current: the demo shown,
// or none when a phone is back on the tabs. A name no demo has any more is
// ignored.
func rememberDemo(cx *core.Scope, g *gallery) {
	store := cx.Storage()
	name, _ := store.GetItem(lastDemo)
	if open := os.Getenv("NSGO_GALLERY_OPEN"); open != "" {
		name = open
	}
	if d, ok := findDemo(name); ok {
		g.selectDemo(d.name)
		if !g.desktop() {
			g.pushDemo(d.name)
		}
	}
	cx.Effect(func() {
		open := g.selected.Get()
		if !g.desktop() {
			open = ""
			for _, r := range g.nav.Routes() {
				if r.demo != "" {
					open = r.demo
				}
			}
		}
		core.Untracked(cx, func() bool {
			if open != "" {
				store.SetItem(lastDemo, open)
			} else {
				store.RemoveItem(lastDemo)
			}
			return true
		})
	})
}

// toggleSidebar hides or shows the desktop layout's sidebar.
func (g *gallery) toggleSidebar() { g.sidebarOpen.Update(func(open bool) bool { return !open }) }

// phone is the narrow layout: the tabs, each index a stack of its own for
// its large title, and pushed over them all, the bar included, the demos.
func (g *gallery) phone(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	// Back from the desktop layout: what it showed stays open.
	if g.wasDesktop {
		if g.settingsOpen.GetUntracked() {
			g.settingsOpen.Set(false)
			g.tab.Set(settingsTab())
			g.nav.SetRoutes(route{})
		} else {
			g.pushDemo(g.selected.GetUntracked())
		}
	}
	g.wasDesktop = false
	shellTab := func(i int, id, title string, icon func() *core.SVG, ios, android string) ui.ShellTab {
		system := ios
		if cx.Env().OS() == core.OSAndroid {
			system = android
		}
		return ui.ShellTab{Text: title, Icon: icon, SystemIcon: system, TestID: "tab-" + id, Content: func(cx *core.Scope) core.Node {
			return core.Stack{Style: fillPage, Children: []core.Node{g.section(cx, i)}}
		}}
	}
	var sections []ui.ShellTab
	for i, t := range tabs {
		sections = append(sections, shellTab(i, t.id, t.title(tr), t.icon, t.ios, t.android))
	}
	sections = append(sections, shellTab(settingsTab(), "settings", tr.Settings(), lucide.Settings, "gearshape", "android:ic_menu_preferences"))
	title := func() string {
		if t := g.tab.Get(); t < len(tabs) {
			return tabs[t].title(tr)
		}
		return tr.Settings()
	}
	return g.nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, r route, _ func() int) base.NavPage {
		switch {
		case r.page != nil:
			return r.page(cx)
		case r.demo != "":
			return demoScreen(cx, r.demo)
		}
		return base.NavPage{TitleFunc: title, HeaderHidden: true, Children: []core.Node{
			ui.TabShell{Tabs: sections, Selected: g.tab.Get, OnChange: g.tab.Set, Label: tr.AppSections()},
		}}
	})
}

// demoScreen is a demo pushed over the tabs.
func demoScreen(cx *core.Scope, name string) base.NavPage {
	tr := locales.For(cx)
	d, _ := findDemo(name)
	p := base.NavPage{Title: d.title(tr), TestID: "screen-" + d.name, HeaderStyleFunc: headerStyle(cx),
		Header:    pageHeader(cx, pageHeaderProps{title: d.title(tr)}),
		StyleFunc: pageStyle(cx),
		Children: []core.Node{core.View{Style: fillPage, StyleFunc: pageStyle(cx),
			Children: []core.Node{core.Func(func(cx *core.Scope) core.Node { return d.screen(cx) })}}}}
	if fixedDemos[d.name] {
		return p
	}
	return largeTitle(cx, p)
}

// section is tab i's page: its index, or the settings.
func (g *gallery) section(cx *core.Scope, i int) base.NavPage {
	tr := locales.For(cx)
	if i == settingsTab() {
		return largeTitle(cx, base.NavPage{Title: tr.Settings(), TestID: "settings-page", HeaderStyleFunc: headerStyle(cx),
			Header:    pageHeader(cx, pageHeaderProps{title: tr.Settings(), root: true}),
			StyleFunc: pageStyle(cx),
			Children:  []core.Node{g.settingsPage(cx)}})
	}
	title := tabs[i].title(tr)
	return largeTitle(cx, base.NavPage{Title: title, TestID: "index", HeaderStyleFunc: headerStyle(cx),
		Header:    pageHeader(cx, pageHeaderProps{title: title, root: true}),
		StyleFunc: pageStyle(cx),
		Children:  []core.Node{g.home(cx, i)}})
}

// fixedDemos are the demos that fill the page rather than scroll under a
// large title.
var fixedDemos = map[string]bool{"Stocks": true, "AI Chat": true, "Photos": true, "List": true, "DataTable": true, "DiffView": true, "Dock": true,
	"Editor": true, "Resizable": true, "Terminal": true, "Physics": true, "Collapsing header": true}

// largeTitle gives page p iOS's large title, over no background until the
// content scrolls under the system's blurred bar.
func largeTitle(cx *core.Scope, p base.NavPage) base.NavPage {
	if cx.Env().OS() != core.OSIOS {
		return p
	}
	colors := ui.ColorsOf(cx)
	p.HeaderLargeTitle = true
	p.HeaderStyleFunc = func() core.Style { return core.Style{Color: colors.Text(), TintColor: colors.Accent()} }
	return p
}

// home is tab i's index of the demos by group, titled where it has more
// than one.
func (g *gallery) home(cx *core.Scope, i int) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	var nodes []core.Node
	for n, grp := range tabs[i].groups {
		var rows []core.Node
		for j, d := range grp.demos {
			rows = append(rows, demoRow(cx, d, j == 0, func() { g.nav.Push(route{demo: d.name}) }))
		}
		var parts []core.Node
		if len(tabs[i].groups) > 1 {
			parts = append(parts, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Padding: core.Symmetric(0, 4)}, Children: []core.Node{
				core.Text{Content: grp.title(tr), Role: core.RoleHeader, Style: core.Style{FlexGrow: 1, FontSize: 13, FontWeight: ui.Semibold,
					LetterSpacing: 0.4, TextTransform: core.TextTransformUppercase}, StyleFunc: ui.Colored(colors.Muted)},
				core.Text{Content: itoa(len(grp.demos)), Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
			}})
		}
		parts = append(parts, core.View{Style: core.Style{CornerRadius: ui.RadiusCard, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, Overflow: core.Hidden},
			StyleFunc: func() core.Style { return core.Style{Background: colors.Surface(), BorderColor: colors.Border()} }, Children: rows})
		nodes = append(nodes, Appear{Index: n + 1, Children: []core.Node{core.View{TestID: "group-" + grp.name, Style: core.Style{Gap: 8}, Children: parts}}})
	}
	return core.Scroll{TestID: "home", Style: fillPage, StyleFunc: pageStyle(cx), Children: []core.Node{
		core.View{Style: core.Style{Gap: 20}, Children: nodes, StyleFunc: func() core.Style {
			return core.Style{Padding: core.Edges{Top: 16, Right: 16, Bottom: 16 + ui.TabBarInset(cx), Left: 16}}
		}},
	}}
}

// demoRow is a demo in the phone layout's index.
func demoRow(cx *core.Scope, d demo, first bool, open func()) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	s := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14, Padding: core.All(12)}
	if !first {
		s.Borders = core.Edges{Top: 1}
	}
	return core.View{OnPress: func(core.PressEvent) { open() }, Role: core.RoleButton, Label: d.title(tr), TestID: "nav-" + d.name, Children: []core.Node{
		core.View{Style: s, StyleFunc: func() core.Style {
			if first {
				return core.Style{}
			}
			return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}}
		}, Children: []core.Node{
			IconTile{Icon: d.icon, Size: 38},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				ui.Text(d.title(tr), 16, ui.Medium, colors.Text),
				core.Text{Content: d.detail(tr), MaxLines: 2, Style: core.Style{FontSize: 13, LineHeight: 18}, StyleFunc: ui.Colored(colors.Muted)},
			}},
			ui.Icon(lucide.ChevronRight, 16, colors.Faint),
		}},
	}}
}

// desktopLayout is a sidebar of the demos and the selected one beside it,
// no index to go back to. What a demo pushes opens over it in the content
// pane, with a back button.
func (g *gallery) desktopLayout(cx *core.Scope) core.Node {
	// From the narrow layout: show the demo it had open, or the settings
	// over the selected one.
	if t := g.tab.GetUntracked(); t == settingsTab() {
		g.settingsOpen.Set(true)
		g.selectDemo(g.selected.GetUntracked())
	} else {
		name := g.picked[t]
		for _, r := range core.Untracked(cx, g.nav.Routes) {
			if r.demo != "" {
				name = r.demo
			}
		}
		g.selectDemo(name)
	}
	g.wasDesktop = true
	// The application menu's View › Show Sidebar takes the key where
	// there is a menu bar.
	if !desktop.IsAvailable(cx) {
		cx.OnKey("mod+b", g.toggleSidebar)
	}
	tr := locales.For(cx)
	return ui.SidebarShell{TestID: "shell", Sidebar: g.sidebar(tr), SidebarFooter: []core.Node{g.settingsButton(cx), ui.Spacer{}, themeToggle(cx)},
		Open: g.sidebarOpen.Get, OnOpenChange: g.sidebarOpen.Set, ToggleLabel: tr.AppToggleSidebar(),
		Width: g.sidebarWidth.Get, OnWidthChange: g.sidebarWidth.Set, DefaultWidth: sidebarWidth, MinWidth: 212, MaxWidth: 360,
		ResizeLabel: tr.AppResizeSidebar(),
		Header:      []core.Node{g.demoTitle()},
		Content: []core.Node{g.nav.Stack(fillPage, func(cx *core.Scope, r route, _ func() int) base.NavPage {
			if r.page != nil {
				return r.page(cx)
			}
			return base.NavPage{HeaderHidden: true, Children: []core.Node{g.demoPane()}}
		})}}
}

// sidebar is the desktop layout's tabs above the selected one's list of
// the demos by group, titled where it has more than one. A list hidden
// keeps its scroll position.
func (g *gallery) sidebar(tr locales.Messages) []core.Node {
	var options []ui.Option[int]
	var lists []core.Node
	for i, t := range tabs {
		options = append(options, ui.Option[int]{Value: i, Label: t.title(tr), TestID: "tab-" + t.id})
		var sections []ui.SidebarSection
		for _, grp := range t.groups {
			var entries []ui.SidebarItem
			for _, d := range grp.demos {
				entries = append(entries, ui.SidebarItem{ID: d.name, Text: d.title(tr), Icon: d.icon, TestID: "nav-" + d.name})
			}
			section := ui.SidebarSection{Items: entries, TestID: "nav-group-" + grp.name}
			if len(t.groups) > 1 {
				section.Title = grp.title(tr)
			}
			sections = append(sections, section)
		}
		lists = append(lists, core.View{Children: []core.Node{ui.Sidebar{TestID: "sidebar", Sections: sections, Selected: g.selected.Get, OnSelect: g.selectDemo}},
			StyleFunc: func() core.Style {
				s := ui.FillRest
				if g.tab.Get() != i {
					s.Display = core.DisplayNone
				}
				return s
			}})
	}
	return []core.Node{
		core.View{Style: core.Style{FlexShrink: 0, Padding: core.Edges{Right: 12, Bottom: 8, Left: 12}}, Children: []core.Node{
			ui.Segmented[int]{Options: options, Value: g.tab.Get, OnChange: func(i int) { g.selectDemo(g.picked[i]) }, Compact: true, Label: tr.AppSections()},
		}},
		core.View{Style: ui.FillRest, Children: lists},
	}
}

// demoTitle is the selected demo's group and name, as a path, in the
// content's header.
func (g *gallery) demoTitle() core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		name := g.selected.Get()
		d, _ := findDemo(name)
		return core.Func(func(cx *core.Scope) core.Node {
			colors := ui.ColorsOf(cx)
			tr := locales.For(cx)
			group := ui.Text(groupOf(tr, name), 13, ui.Bold, colors.Text)
			group.MaxLines = 1
			title := ui.Text(d.title(tr), 13, ui.Bold, colors.Text)
			title.MaxLines, title.Role, title.TestID, title.Style.FlexShrink = 1, core.RoleHeader, "title", 1
			return ui.HStack{Gap: 4, Children: []core.Node{group, ui.Text("/", 13, ui.Bold, colors.Text), title}}
		})
	})
}

// demoPane is the selected demo.
func (g *gallery) demoPane() core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		d, _ := findDemo(g.selected.Get())
		return core.Func(func(cx *core.Scope) core.Node {
			return core.View{TestID: "demo-" + d.name, Style: fillPage, StyleFunc: pageStyle(cx), Children: []core.Node{d.screen(cx)}}
		})
	})
}

// menu is the application menu: the app menu and Edit of roles, View
// (the sidebar and a demo each, with shortcuts) and Window.
func (g *gallery) menu(tr locales.Messages) []desktop.MenuItem {
	selected := g.selected.Get()
	choose := func(label string, fn func()) func() {
		return func() { g.chosen.Set(label); fn() }
	}
	view := []desktop.MenuItem{
		{Label: tr.AppShowSidebar(), Shortcut: "mod+b", Checkable: true, Checked: g.sidebarOpen.Get(), OnSelect: choose(tr.AppShowSidebar(), g.toggleSidebar)},
		{Separator: true},
	}
	for i, d := range allDemos() {
		item := desktop.MenuItem{Label: d.title(tr), Checkable: true, Checked: d.name == selected, OnSelect: choose(d.title(tr), func() { g.open(d.name) })}
		if i < 9 {
			item.Shortcut = "mod+" + itoa(i+1)
		}
		view = append(view, item)
	}
	return []desktop.MenuItem{
		{Label: tr.AppGallery(), Items: []desktop.MenuItem{
			{Role: desktop.MenuAbout}, {Separator: true},
			{Role: desktop.MenuHide}, {Role: desktop.MenuHideOthers}, {Role: desktop.MenuShowAll}, {Separator: true},
			{Role: desktop.MenuQuit},
		}},
		{Label: tr.AppEdit(), Items: []desktop.MenuItem{
			{Role: desktop.MenuUndo}, {Role: desktop.MenuRedo}, {Separator: true},
			{Role: desktop.MenuCut}, {Role: desktop.MenuCopy}, {Role: desktop.MenuPaste}, {Role: desktop.MenuSelectAll},
		}},
		{Label: tr.AppView(), Items: view},
		{Label: tr.AppWindow(), Items: []desktop.MenuItem{
			{Role: desktop.MenuMinimize}, {Role: desktop.MenuZoom}, {Role: desktop.MenuToggleFullScreen}, {Separator: true},
			{Role: desktop.MenuClose},
		}},
	}
}

// osName is the platform's name, as the sidebar shows it.
func osName(os core.OS) string {
	names := map[core.OS]string{core.OSMacOS: "macOS", core.OSIOS: "iOS", core.OSAndroid: "Android", core.OSWindows: "Windows", core.OSLinux: "Linux", core.OSWeb: "web"}
	if n, ok := names[os]; ok {
		return n
	}
	return "unknown"
}

func main() {
	// The Go fonts' small capitals and Pacifico, shipped in
	// resources/fonts.
	opts := []nsgo.Option{nsgo.Fonts("fonts/Go-Smallcaps.ttf", "fonts/Go-Smallcaps-Italic.ttf", "fonts/Pacifico-Regular.ttf")}
	names := make([]string, 0, len(components))
	for name := range components {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		opts = append(opts, nsgo.Component(name, components[name]))
	}
	nsgo.Run(Root, append(opts, nsgo.WindowSize(1080, 760), nsgo.WindowMaterial(desktop.WindowMaterialSidebar))...)
}
