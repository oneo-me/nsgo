// Command gallery shows what NSGo apps are made of: core's elements and
// services, and the components of base and ui, in NSH's gallery's design,
// under two tabs. Components has the apps and a page per component, grouped
// by what they do: its base form with every part's style written out in
// neutral colors, then its ui form with its props to set, or a whole page
// for a complex one with its options beside it. Basics has what they are
// made of. A wide window shows the tabs atop a sidebar of the demos beside
// the selected one; a narrow one shows them,
// and the settings, in a bar at the bottom, with demos pushed over it all.
// Its look and components are package ui's, as NSH's are its
// nsh-gallery-ui's.
//
// It translates NSH's examples/gallery demo by demo, and departs from it
// where NSGo differs: icons are Lucide's (core.Icon) rather than glyphs
// drawn in canvases; NSH's Tailwind and standard-module demos have no
// NSGo counterpart, while Layout, Table and Accessibility show what NSGo
// has beyond NSH's gallery.
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

// tabs are the demos. Components holds the apps built on NSGo above a page
// per family of components, each with its ui form and then its base form,
// by what they do;
// Basics holds what they are made of: core's primitives from the simplest,
// styling, interaction and the platform's services. They are set in init
// because demos list them.
var tabs []tab

// settingsTab is the narrow layout's tab after the demos'.
func settingsTab() int { return len(tabs) }

func init() {
	type m = locales.Messages
	apps := []demo{
		{"Stocks", m.DemoStocks, m.DemoStocksDetail, lucide.ChartCandlestick, stocksDemo},
		{"AI Chat", m.DemoAIChat, m.DemoAIChatDetail, lucide.MessageCircle, chatDemo},
		{"Photos", m.DemoPhotos, m.DemoPhotosDetail, lucide.Images, photosDemo},
	}
	actions := []demo{
		{"Button", m.DemoButton, m.DemoButtonDetail, lucide.MousePointerClick, eventsDemo(buttonCard, iconButtonCard, toggleButtonCard, toggleGroupCard, holdPlayground)},
		{"Link", m.DemoLink, m.DemoLinkDetail, lucide.Link, eventsDemo(linkCard)},
		{"Toolbar", m.DemoToolbar, m.DemoToolbarDetail, lucide.PanelTop, eventsDemo(toolbarCard)},
	}
	inputs := []demo{
		{"Input", m.DemoInput, m.DemoInputDetail, lucide.TextCursorInput, formDemo},
		{"Checkbox", m.DemoCheckbox, m.DemoCheckboxDetail, lucide.SquareCheck, eventsDemo(checkboxCard, radioCard, switchCard)},
		{"Select", m.DemoSelect, m.DemoSelectDetail, lucide.ChevronsUpDown, eventsDemo(selectPlayground, comboboxPlayground)},
		{"DatePicker", m.DemoDatePicker, m.DemoDatePickerDetail, lucide.Calendar, eventsDemo(datePickerPlayground, calendarPlayground, timeFieldPlayground)},
		{"ColorPicker", m.DemoColorPicker, m.DemoColorPickerDetail, lucide.Palette, eventsDemo(colorPickerPlayground, colorWellPlayground)},
		{"Slider", m.DemoSlider, m.DemoSliderDetail, lucide.SlidersHorizontal, eventsDemo(sliderPlayground, ratingCard)},
	}
	display := []demo{
		{"Badge", m.DemoBadge, m.DemoBadgeDetail, lucide.Tag, eventsDemo(badgeCard)},
		{"Avatar", m.DemoAvatar, m.DemoAvatarDetail, lucide.User, eventsDemo(avatarCard, avatarGroupCard)},
		{"Image", m.DemoImage, m.DemoImageDetail, lucide.Image, eventsDemo(imagePlayground, imageCompareCard)},
		{"Carousel", m.DemoCarousel, m.DemoCarouselDetail, lucide.GalleryHorizontal, eventsDemo(carouselCard)},
	}
	data := []demo{
		{"List", m.DemoList, m.DemoListDetail, lucide.List, eventsDemo(listCard, treeCard)},
		{"Table", m.DemoTable, m.DemoTableDetail, lucide.Table, eventsDemo(plainCard(tableCard))},
		{"DataTable", m.DemoDataTable, m.DemoDataTableDetail, lucide.Table, dataTableDemo},
		{"Plot", m.DemoPlot, m.DemoPlotDetail, lucide.ChartSpline, chartsDemo},
	}
	containers := []demo{
		{"Accordion", m.DemoAccordion, m.DemoAccordionDetail, lucide.Rows3, eventsDemo(accordionCard, collapsiblePlayground)},
		{"Resizable", m.DemoResizable, m.DemoResizableDetail, lucide.PanelLeft, resizableDemo},
		{"Dock", m.DemoDock, m.DemoDockDetail, lucide.AppWindow, dockDemo},
	}
	feedback := []demo{
		{"Progress", m.DemoProgress, m.DemoProgressDetail, lucide.LoaderCircle, eventsDemo(progressCard, progressRingCard, spinnerPlayground, meterCard, skeletonCard)},
		{"Toast", m.DemoToast, m.DemoToastDetail, lucide.Bell, eventsDemo(toastPlayground)},
	}
	popups := []demo{
		{"Dialog", m.DemoDialog, m.DemoDialogDetail, lucide.AppWindow, eventsDemo(dialogCard, alertDialogCard, sheetCard)},
		{"Popover", m.DemoPopover, m.DemoPopoverDetail, lucide.MessageSquare, eventsDemo(popoverPlayground, tooltipPlayground, hoverCardPlayground)},
		{"Command", m.DemoCommand, m.DemoCommandDetail, lucide.Search, eventsDemo(commandPlayground)},
		{"Menu", m.DemoMenu, m.DemoMenuDetail, lucide.Menu, menusDemo},
	}
	navigation := []demo{
		{"Tabs", m.DemoTabs, m.DemoTabsDetail, lucide.NotebookTabs, eventsDemo(tabsCard)},
		{"Breadcrumbs", m.DemoBreadcrumbs, m.DemoBreadcrumbsDetail, lucide.ChevronRight, eventsDemo(breadcrumbsCard, paginationCard, stepperPlayground)},
		{"Sidebar", m.DemoSidebar, m.DemoSidebarDetail, lucide.PanelLeft, eventsDemo(sidebarCard)},
		{"NavStack", m.DemoNavStack, m.DemoNavStackDetail, lucide.PanelsTopLeft, navigationDemo},
	}
	documents := []demo{
		{"Conversation", m.DemoConversation, m.DemoConversationDetail, lucide.MessagesSquare, conversationDemo},
		{"Editor", m.DemoEditor, m.DemoEditorDetail, lucide.Code, editorDemo},
		{"DiffView", m.DemoDiffView, m.DemoDiffViewDetail, lucide.FileDiff, diffDemo},
		{"Terminal", m.DemoTerminal, m.DemoTerminalDetail, lucide.Terminal, terminalDemo},
		{"TextView", m.DemoTextView, m.DemoTextViewDetail, lucide.FileText, textViewDemo},
	}
	basics := []group{
		{"Primitives", m.GroupPrimitives, []demo{
			{"Text", m.DemoText, m.DemoTextDetail, lucide.Type, textDemo},
			{"Images", m.DemoImages, m.DemoImagesDetail, lucide.Image, imagesDemo},
			{"Scroll", m.DemoScroll, m.DemoScrollDetail, lucide.ScrollText, scrollingDemo},
			{"Lists", m.DemoLists, m.DemoListsDetail, lucide.List, listsDemo},
			{"Canvas", m.DemoCanvas, m.DemoCanvasDetail, lucide.PenTool, canvasDemo},
			{"Shader", m.DemoShader, m.DemoShaderDetail, lucide.Sparkles, shaderDemo},
			{"WebView and Video", m.DemoWebViewAndVideo, m.DemoWebViewAndVideoDetail, lucide.Film, mediaDemo},
			{"Overlay", m.DemoOverlay, m.DemoOverlayDetail, lucide.Layers, eventsDemo(overlayCard)},
		}},
		{"Styling", m.GroupStyling, []demo{
			{"Styles", m.DemoStyles, m.DemoStylesDetail, lucide.Palette, stylesDemo},
			{"Layout", m.DemoLayout, m.DemoLayoutDetail, lucide.Columns2, layoutDemo},
		}},
		{"Interaction", m.GroupInteraction, []demo{
			{"Animations", m.DemoAnimations, m.DemoAnimationsDetail, lucide.Zap, animationsDemo},
			{"Gestures", m.DemoGestures, m.DemoGesturesDetail, lucide.Hand, gesturesDemo},
			{"Physics", m.DemoPhysics, m.DemoPhysicsDetail, lucide.Atom, physicsDemo},
			{"Collapsing header", m.DemoCollapsingHeader, m.DemoCollapsingHeaderDetail, lucide.PanelTop, collapsingDemo},
			{"Patterns", m.DemoPatterns, m.DemoPatternsDetail, lucide.Component, componentsDemo},
		}},
		{"Platform", m.GroupPlatform, []demo{
			{"System", m.DemoSystem, m.DemoSystemDetail, lucide.Cpu, systemDemo},
			{"Storage", m.DemoStorage, m.DemoStorageDetail, lucide.Database, storageDemo},
			{"Accessibility", m.DemoAccessibility, m.DemoAccessibilityDetail, lucide.Accessibility, accessibilityDemo},
		}},
	}
	tabs = []tab{
		{"components", m.TabComponents, lucide.Component, "puzzlepiece.extension", "android:ic_menu_agenda", []group{
			{"Apps", m.GroupApps, apps}, {"Actions", m.GroupActions, actions}, {"Inputs", m.GroupInputs, inputs}, {"Display", m.GroupDisplay, display}, {"Data", m.GroupData, data},
			{"Containers", m.GroupContainers, containers}, {"Feedback", m.GroupFeedback, feedback}, {"Popups", m.GroupPopups, popups}, {"Navigation", m.GroupNavigation, navigation},
			{"Documents", m.GroupDocuments, documents},
		}},
		{"basics", m.TabBasics, lucide.Shapes, "square.on.circle", "android:ic_menu_sort_by_size", basics},
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
	// root is the scope whose theme the settings change.
	root *core.Scope
	nav  *base.Nav[route]
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
	// searchOpen is whether the demos' search shows.
	searchOpen *core.Signal[bool]
	// toastPlacement, toastReverse and toastSwipe are how toasts show, as
	// the Toast demo sets them.
	toastPlacement *core.Signal[ui.ToastPlacement]
	toastReverse   *core.Signal[bool]
	toastSwipe     *core.Signal[ui.ToastSwipe]
	// wasDesktop is whether the desktop layout has been shown, for the
	// switch back.
	wasDesktop bool
	desktop    func() bool
	env        *core.Env
	toasts     *ui.Toaster
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
	rememberTheme(cx)
	env := cx.Env()
	var picked []string
	for _, t := range tabs {
		picked = append(picked, t.groups[0].demos[0].name)
	}
	g := &gallery{
		root:             cx,
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
		searchOpen:       core.NewSignal(cx, false),
		env:              env,
		toastPlacement:   core.NewSignal(cx, ui.ToastBottomEnd),
		toastReverse:     core.NewSignal(cx, false),
		toastSwipe:       core.NewSignal(cx, ui.ToastSwipeEdge),
		toasts:           ui.NewToaster(cx),
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
			children = append(children, g.searchDialog(cx), g.toastView(wide))
			return core.View{StyleFunc: func() core.Style {
				return fillPage.Merge(core.Style{Background: ui.Chrome(cx), Color: colors.Text()})
			}, Children: children}
		})
	})
}

// toastView is where the toasts show, over the narrow layout's tab bar.
func (g *gallery) toastView(wide bool) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		t := ui.Toasts{Toaster: g.toasts, Placement: g.toastPlacement.Get(), Reverse: g.toastReverse.Get(), Swipe: g.toastSwipe.Get(),
			CloseLabel: locales.For(cx).ToastClose(), TestID: "toasts"}
		if !wide && t.Placement >= ui.ToastBottom && t.Placement <= ui.ToastBottomEnd {
			t.Inset = tabBarHeight
		}
		return t
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
var fixedDemos = map[string]bool{"Stocks": true, "AI Chat": true, "Conversation": true, "Photos": true, "Lists": true, "DataTable": true, "DiffView": true, "Dock": true,
	"Editor": true, "Plot": true, "Resizable": true, "Terminal": true, "TextView": true, "Physics": true, "Collapsing header": true}

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

// home is tab i's index of the demos by group.
func (g *gallery) home(cx *core.Scope, i int) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	var nodes []core.Node
	for n, grp := range tabs[i].groups {
		var rows []core.Node
		for j, d := range grp.demos {
			rows = append(rows, demoRow(cx, d, j == 0, func() { g.nav.Push(route{demo: d.name}) }))
		}
		parts := []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Padding: core.Symmetric(0, 4)}, Children: []core.Node{
				core.Text{Content: grp.title(tr), Role: core.RoleHeader, Style: core.Style{FlexGrow: 1, FontSize: 13, FontWeight: ui.Semibold,
					LetterSpacing: 0.4, TextTransform: core.TextTransformUppercase}, StyleFunc: ui.Colored(colors.Muted)},
				core.Text{Content: itoa(len(grp.demos)), Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
			}},
			core.View{Style: core.Style{CornerRadius: ui.RadiusCard, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, Overflow: core.Hidden},
				StyleFunc: func() core.Style { return core.Style{Background: colors.Surface(), BorderColor: colors.Border()} }, Children: rows},
		}
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
	return ui.SidebarShell{TestID: "shell", SidebarActions: []core.Node{g.sidebarTabs(tr)}, Sidebar: append([]core.Node{g.searchButton(cx, tr)}, g.sidebar(tr)...), SidebarFooter: []core.Node{g.settingsButton(cx), ui.Spacer{}, themeToggle(cx)},
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

// sidebarTabs are the desktop layout's small tabs, at the end of the
// sidebar's header: icons named in tooltips, as words in every language
// would not fit beside the window controls and the sidebar's toggle.
func (g *gallery) sidebarTabs(tr locales.Messages) core.Node {
	var options []ui.Option[int]
	for i, t := range tabs {
		title := t.title(tr)
		options = append(options, ui.Option[int]{Value: i, Label: title, TestID: "tab-" + t.id,
			Content: core.Func(func(*core.Scope) core.Node {
				return ui.Tooltip{Text: title, Trigger: ui.Icon(t.icon, 14, ui.Inherited)}
			})})
	}
	return ui.Tabs[int]{Size: ui.SizeSmall, Options: options, Value: g.tab.Get, OnChange: func(i int) { g.selectDemo(g.picked[i]) }, Label: tr.AppSections(),
		Style: core.Style{AlignSelf: core.AlignCenter}}
}

// sidebar is the selected tab's list of the demos by group. A list hidden
// keeps its scroll position.
func (g *gallery) sidebar(tr locales.Messages) []core.Node {
	var lists []core.Node
	for i, t := range tabs {
		var sections []ui.SidebarSection
		for _, grp := range t.groups {
			var entries []ui.SidebarItem
			for _, d := range grp.demos {
				entries = append(entries, ui.SidebarItem{ID: d.name, Text: d.title(tr), Icon: d.icon, TestID: "nav-" + d.name})
			}
			sections = append(sections, ui.SidebarSection{Title: grp.title(tr), Collapsible: true, Items: entries, TestID: "nav-group-" + grp.name})
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
	return []core.Node{core.View{Style: ui.FillRest, Children: lists}}
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
