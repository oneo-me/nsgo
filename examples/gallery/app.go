// Command gallery shows what NSGo apps are made of: core's elements and
// services and base's unstyled components, a demo each, in NSH's gallery's
// design. A wide window shows a sidebar of the demos beside the selected
// one; a narrow one is a phone's index with demos pushed over it. Its
// look and components are examples/gallery-ui's, as NSH's are its
// nsh-gallery-ui's.
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
	"os"
	"slices"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/i18n"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

//go:embed logo.png
var logoPNG []byte

// demo is an entry of the index: name is its ID, in English, and title
// and detail say it in a language.
type demo struct {
	name          string
	title, detail func(locales.Messages) string
	icon          ui.IconName
	screen        func(cx *core.Scope) core.Node
}

// group is demos by what they show: name is its ID, in English.
type group struct {
	name  string
	title func(locales.Messages) string
	demos []demo
}

// groups are the demos: the apps built on NSGo, core's primitives from the
// simplest, base's components and gallery-ui's own by name, styling, motion and the platform's
// services, as NSH's gallery lists them after its 0.3.0 release. They are
// set in init because demos list them.
var groups []group

func init() {
	primitives := []demo{
		{"Text", locales.Messages.DemoText, locales.Messages.DemoTextDetail, ui.IconText, textDemo},
		{"Image", locales.Messages.DemoImage, locales.Messages.DemoImageDetail, ui.IconImage, imagesDemo},
		{"Scroll", locales.Messages.DemoScroll, locales.Messages.DemoScrollDetail, ui.IconScroll, scrollingDemo},
		{"List", locales.Messages.DemoList, locales.Messages.DemoListDetail, ui.IconLists, listsDemo},
		{"Canvas", locales.Messages.DemoCanvas, locales.Messages.DemoCanvasDetail, ui.IconCanvas, canvasDemo},
		{"Shader", locales.Messages.DemoShader, locales.Messages.DemoShaderDetail, ui.IconShader, shaderDemo},
		{"WebView and Video", locales.Messages.DemoWebViewAndVideo, locales.Messages.DemoWebViewAndVideoDetail, ui.IconMedia, mediaDemo},
		{"Overlay", locales.Messages.DemoOverlay, locales.Messages.DemoOverlayDetail, ui.IconOverlays, eventsDemo(overlayCard)},
	}
	components := []demo{
		{"Checkbox", locales.Messages.DemoCheckbox, locales.Messages.DemoCheckboxDetail, ui.IconCheckbox, controlsDemo(controls.filter)},
		{"Collapsible", locales.Messages.DemoCollapsible, locales.Messages.DemoCollapsibleDetail, ui.IconCollapsible, controlsDemo(controls.disclosure)},
		{"DataTable", locales.Messages.DemoDataTable, locales.Messages.DemoDataTableDetail, ui.IconTable, dataTableDemo},
		{"DiffView", locales.Messages.DemoDiffView, locales.Messages.DemoDiffViewDetail, ui.IconDiff, diffDemo},
		{"Dock", locales.Messages.DemoDock, locales.Messages.DemoDockDetail, ui.IconDock, dockDemo},
		{"Editor", locales.Messages.DemoEditor, locales.Messages.DemoEditorDetail, ui.IconCode, editorDemo},
		{"FileIcon", locales.Messages.DemoFileIcon, locales.Messages.DemoFileIconDetail, ui.IconFile, eventsDemo(fileIconCard)},
		{"HoldToConfirm", locales.Messages.DemoHoldToConfirm, locales.Messages.DemoHoldToConfirmDetail, ui.IconTrash, eventsDemo(holdCard)},
		{"Input", locales.Messages.DemoInput, locales.Messages.DemoInputDetail, ui.IconForm, formDemo},
		{"Menu", locales.Messages.DemoMenu, locales.Messages.DemoMenuDetail, ui.IconMenu, menusDemo},
		{"NavStack", locales.Messages.DemoNavStack, locales.Messages.DemoNavStackDetail, ui.IconNav, navigationDemo},
		{"Plot", locales.Messages.DemoPlot, locales.Messages.DemoPlotDetail, ui.IconChart, chartsDemo},
		{"Popover", locales.Messages.DemoPopover, locales.Messages.DemoPopoverDetail, ui.IconPopover, eventsDemo(popoverCard)},
		{"Progress", locales.Messages.DemoProgress, locales.Messages.DemoProgressDetail, ui.IconProgress, controlsDemo(controls.transfers)},
		{"Radio", locales.Messages.DemoRadio, locales.Messages.DemoRadioDetail, ui.IconRadio, controlsDemo(controls.plansCard)},
		{"Resizable", locales.Messages.DemoResizable, locales.Messages.DemoResizableDetail, ui.IconSidebar, resizableDemo},
		{"Sheet", locales.Messages.DemoSheet, locales.Messages.DemoSheetDetail, ui.IconSheet, eventsDemo(sheetCard)},
		{"Stepper", locales.Messages.DemoStepper, locales.Messages.DemoStepperDetail, ui.IconCheck, stepperDemo},
		{"Switch", locales.Messages.DemoSwitch, locales.Messages.DemoSwitchDetail, ui.IconSwitch, controlsDemo(controls.settings)},
		{"Table", locales.Messages.DemoTable, locales.Messages.DemoTableDetail, ui.IconTable, tableDemo},
		{"Tabs", locales.Messages.DemoTabs, locales.Messages.DemoTabsDetail, ui.IconTabs, controlsDemo(controls.sections)},
		{"TextView", locales.Messages.DemoTextView, locales.Messages.DemoTextViewDetail, ui.IconFileText, textViewDemo},
		{"Toast", locales.Messages.DemoToast, locales.Messages.DemoToastDetail, ui.IconBell, eventsDemo(toastCard, styledToastsCard)},
		{"ToggleButton", locales.Messages.DemoToggleButton, locales.Messages.DemoToggleButtonDetail, ui.IconControls, eventsDemo(toggleCard)},
		{"Tooltip", locales.Messages.DemoTooltip, locales.Messages.DemoTooltipDetail, ui.IconTooltip, eventsDemo(tooltipCard)},
	}
	groups = []group{
		{"Apps", locales.Messages.GroupApps, []demo{
			{"Stocks", locales.Messages.DemoStocks, locales.Messages.DemoStocksDetail, ui.IconStocks, stocksDemo},
			{"AI Chat", locales.Messages.DemoAIChat, locales.Messages.DemoAIChatDetail, ui.IconChat, chatDemo},
			{"Photos", locales.Messages.DemoPhotos, locales.Messages.DemoPhotosDetail, ui.IconPhotos, photosDemo},
		}},
		{"Primitives", locales.Messages.GroupPrimitives, primitives},
		{"Components", locales.Messages.GroupComponents, components},
		{"Styling", locales.Messages.GroupStyling, []demo{
			{"Styles", locales.Messages.DemoStyles, locales.Messages.DemoStylesDetail, ui.IconStyles, stylesDemo},
			{"Layout", locales.Messages.DemoLayout, locales.Messages.DemoLayoutDetail, ui.IconLayout, layoutDemo},
			{"Patterns", locales.Messages.DemoPatterns, locales.Messages.DemoPatternsDetail, ui.IconComponents, componentsDemo},
		}},
		{"Motion", locales.Messages.GroupMotion, []demo{
			{"Animations", locales.Messages.DemoAnimations, locales.Messages.DemoAnimationsDetail, ui.IconAnimation, animationsDemo},
			{"Gestures", locales.Messages.DemoGestures, locales.Messages.DemoGesturesDetail, ui.IconGestures, gesturesDemo},
			{"Physics", locales.Messages.DemoPhysics, locales.Messages.DemoPhysicsDetail, ui.IconPhysics, physicsDemo},
			{"Collapsing header", locales.Messages.DemoCollapsingHeader, locales.Messages.DemoCollapsingHeaderDetail, ui.IconHeader, collapsingDemo},
		}},
		{"Platform", locales.Messages.GroupPlatform, []demo{
			{"System", locales.Messages.DemoSystem, locales.Messages.DemoSystemDetail, ui.IconSystem, systemDemo},
			{"Storage", locales.Messages.DemoStorage, locales.Messages.DemoStorageDetail, ui.IconStorage, storageDemo},
			{"Accessibility", locales.Messages.DemoAccessibility, locales.Messages.DemoAccessibilityDetail, ui.IconAccess, accessibilityDemo},
		}},
	}
}

// allDemos is every demo, in the index's order.
func allDemos() []demo {
	var out []demo
	for _, g := range groups {
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

// groupOf is the title of the group holding the demo named name.
func groupOf(tr locales.Messages, name string) string {
	for _, g := range groups {
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
	// selected is the demo the desktop layout shows.
	selected *core.Signal[string]
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

// selectDemo shows a demo in the desktop layout, closing what it had
// pushed.
func (g *gallery) selectDemo(name string) {
	if g.nav.Depth() > 0 {
		g.nav.PopToRoot()
	}
	g.selected.Set(name)
}

// open shows a demo in the layout shown: selected beside the sidebar, or
// pushed over the index.
func (g *gallery) open(name string) {
	if g.desktop() {
		g.selectDemo(name)
		return
	}
	g.nav.SetRoutes(route{}, route{demo: name})
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
	first := groups[0].demos[0].name
	g := &gallery{
		nav:              base.NewNav(cx, route{}),
		selected:         core.NewSignal(cx, first),
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
	cx.Effect(func() { desktop.SetApplicationMenu(cx, g.menu(locales.For(cx))) })

	return core.Dynamic(func(cx *core.Scope) core.Node {
		wide := desktopLayout.Get()
		// A new language builds the gallery again in it, as a new layout
		// does, keeping what is open.
		i18n.Language(cx)
		// What the layout builds mounts untracked, outside this function.
		return core.Func(func(cx *core.Scope) core.Node {
			colors := ui.ColorsOf(cx)
			body := g.phone(cx)
			if wide {
				body = g.desktopLayout(cx)
			}
			// The desktop layout's shell draws its own headers.
			var children []core.Node
			if hasWindow && !wide {
				children = append(children, g.titleBar(cx))
			}
			children = append(children, body, g.settingsDialog(cx), g.toasts.view(wide))
			return core.View{StyleFunc: func() core.Style {
				return fillPage.Merge(core.Style{Background: ui.Chrome(cx), Color: colors.Text()})
			}, Children: children}
		})
	})
}

// headerActions lays out the buttons at the end of the index's header.
var headerActions = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4}

// rememberDemo reopens the demo left open last time (selected on a
// desktop, pushed on a phone), then keeps the key current: the demo shown,
// or none when a phone is back on the index. A name no demo has any more
// is ignored.
func rememberDemo(cx *core.Scope, g *gallery) {
	store := cx.Storage()
	name, _ := store.GetItem(lastDemo)
	if open := os.Getenv("NSGO_GALLERY_OPEN"); open != "" {
		name = open
	}
	if d, ok := findDemo(name); ok {
		g.selected.Set(d.name)
		if !g.desktop() {
			g.nav.SetRoutes(route{}, route{demo: d.name})
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

// titleBar is window chrome the app draws over the phone layout; the
// platform keeps its own window controls.
func (g *gallery) titleBar(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, TestID: "title-bar",
		StyleFunc: func() core.Style {
			tb := desktop.TitleBar(cx)
			return core.Style{Height: core.Pt(max(tb.Top, ui.HeaderHeight)), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
				Padding: core.Edges{Left: tb.Left + 12, Right: tb.Right + 12}, Background: ui.Chrome(cx),
				Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Line()}}
		},
		Children: []core.Node{
			logo(cx, 20),
			core.Text{Content: locales.For(cx).AppGallery(), MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
			ui.Text("NSGo", 11, ui.Regular, colors.Muted),
		}}
}

// toggleSidebar hides or shows the desktop layout's sidebar.
func (g *gallery) toggleSidebar() { g.sidebarOpen.Update(func(open bool) bool { return !open }) }

// phone is the narrow layout: the index, demos pushed over it.
func (g *gallery) phone(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	// Back from the desktop layout: the demo it showed stays open.
	if g.wasDesktop {
		g.nav.SetRoutes(route{}, route{demo: g.selected.GetUntracked()})
	}
	g.wasDesktop = false
	return g.nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, r route, _ func() int) base.NavPage {
		switch {
		case r.page != nil:
			return r.page(cx)
		case r.demo != "":
			d, _ := findDemo(r.demo)
			return base.NavPage{Title: d.title(tr), TestID: "screen-" + d.name, HeaderStyleFunc: headerStyle(cx),
				Header:    pageHeader(cx, pageHeaderProps{title: d.title(tr)}),
				StyleFunc: pageStyle(cx),
				Children: []core.Node{core.View{Style: fillPage, StyleFunc: pageStyle(cx),
					Children: []core.Node{core.Func(func(cx *core.Scope) core.Node { return d.screen(cx) })}}}}
		}
		return base.NavPage{Title: tr.AppGallery(), TestID: "index", HeaderStyleFunc: headerStyle(cx),
			HeaderRight: core.View{Style: headerActions, Children: []core.Node{g.barSettingsButton(cx), barThemeToggle(cx)}},
			Header: pageHeader(cx, pageHeaderProps{title: tr.AppGallery(), root: true,
				right: core.View{Style: headerActions, Children: []core.Node{g.settingsButton(cx), themeToggle(cx)}}}),
			Children: []core.Node{g.home(cx)}}
	})
}

// home is the phone layout's index of the demos by group.
func (g *gallery) home(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	var nodes []core.Node
	for i, grp := range groups {
		var rows []core.Node
		for j, d := range grp.demos {
			rows = append(rows, demoRow(cx, d, j == 0, func() { g.nav.Push(route{demo: d.name}) }))
		}
		nodes = append(nodes, ui.Appear{Index: i + 1, Children: []core.Node{core.View{TestID: "group-" + grp.name, Style: core.Style{Gap: 8}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Padding: core.Symmetric(0, 4)}, Children: []core.Node{
				core.Text{Content: grp.title(tr), Role: core.RoleHeader, Style: core.Style{FlexGrow: 1, FontSize: 13, FontWeight: ui.Semibold,
					LetterSpacing: 0.4, TextTransform: core.TextTransformUppercase}, StyleFunc: ui.Colored(colors.Muted)},
				core.Text{Content: itoa(len(grp.demos)), Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
			}},
			core.View{Style: core.Style{CornerRadius: ui.RadiusCard, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, Overflow: core.Hidden},
				StyleFunc: func() core.Style { return core.Style{Background: colors.Surface(), BorderColor: colors.Border()} }, Children: rows},
		}}}})
	}
	return core.Scroll{TestID: "home", Style: core.Style{FlexGrow: 1}, StyleFunc: pageStyle(cx), Children: []core.Node{
		core.View{Style: core.Style{Padding: core.Edges{Top: 16, Right: 16, Bottom: 40, Left: 16}, PaddingEnv: core.EnvEdges{Bottom: core.EnvSafeArea}, Gap: 20}, Children: nodes},
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
	return ui.Press{OnPress: open, Scale: 0.98, Label: d.title(tr), TestID: "nav-" + d.name, Children: []core.Node{
		core.View{Style: s, StyleFunc: func() core.Style {
			if first {
				return core.Style{}
			}
			return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}}
		}, Children: []core.Node{
			ui.IconTile{Icon: d.icon, Size: 38},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				ui.Text(d.title(tr), 16, ui.Medium, colors.Text),
				core.Text{Content: d.detail(tr), MaxLines: 2, Style: core.Style{FontSize: 13, LineHeight: 18}, StyleFunc: ui.Colored(colors.Muted)},
			}},
			ui.Icon(ui.IconChevron, 16, colors.Faint),
		}},
	}}
}

// desktopLayout is a sidebar of the demos and the selected one beside it,
// no index to go back to. What a demo pushes opens over it in the content
// pane, with a back button.
func (g *gallery) desktopLayout(cx *core.Scope) core.Node {
	// From the phone layout: show the demo it had open.
	routes := core.Untracked(cx, g.nav.Routes)
	for _, r := range routes {
		if r.demo != "" {
			g.selected.Set(r.demo)
		}
	}
	if len(routes) > 1 {
		g.nav.SetRoutes(routes[0])
	}
	g.wasDesktop = true
	// The application menu's View › Show Sidebar takes the key where
	// there is a menu bar.
	if !desktop.IsAvailable(cx) {
		cx.OnKey("mod+b", g.toggleSidebar)
	}
	tr := locales.For(cx)
	return ui.SidebarShell{TestID: "shell", Sidebar: []core.Node{g.sidebar(tr)}, SidebarFooter: []core.Node{g.settingsButton(cx), ui.Spacer{}, themeToggle(cx)},
		Open: g.sidebarOpen.Get, OnOpenChange: g.sidebarOpen.Set, ToggleLabel: tr.AppToggleSidebar(),
		Width: g.sidebarWidth.Get, OnWidthChange: g.sidebarWidth.Set, DefaultWidth: sidebarWidth, MinWidth: 200, MaxWidth: 360,
		ResizeLabel: tr.AppResizeSidebar(),
		Header:      []core.Node{g.demoTitle()},
		Content: []core.Node{g.nav.Stack(fillPage, func(cx *core.Scope, r route, _ func() int) base.NavPage {
			if r.page != nil {
				return r.page(cx)
			}
			return base.NavPage{HeaderHidden: true, Children: []core.Node{g.demoPane()}}
		})}}
}

// sidebar is the desktop layout's list of the demos by group.
func (g *gallery) sidebar(tr locales.Messages) core.Node {
	var sections []ui.SidebarSection
	for _, grp := range groups {
		var items []ui.SidebarItem
		for _, d := range grp.demos {
			items = append(items, ui.SidebarItem{ID: d.name, Text: d.title(tr), Icon: d.icon, TestID: "nav-" + d.name})
		}
		sections = append(sections, ui.SidebarSection{Title: grp.title(tr), Items: items, TestID: "nav-group-" + grp.name})
	}
	return ui.Sidebar{TestID: "sidebar", Sections: sections, Selected: g.selected.Get, OnSelect: g.selectDemo}
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
