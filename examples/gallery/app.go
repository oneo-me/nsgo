// Command gallery shows what NSGo apps are made of: core's elements and
// services and base's unstyled components, a demo each, in NSH's gallery's
// design. A wide window shows a sidebar of the demos beside the selected
// one; a narrow one is a phone's index with demos pushed over it. Its
// look and components are examples/gallery-ui's, as NSH's are its
// nsh-gallery-ui's.
//
// It translates NSH's examples/gallery demo by demo, and departs from it
// where NSGo differs: icons are Lucide's (core.Icon) rather than glyphs
// drawn in canvases; NSH's Tailwind and standard-module demos have no NSGo counterpart, while
// Layout, Shader and Accessibility show what NSGo has beyond NSH's
// gallery.
package main

import (
	_ "embed"
	"os"
	"slices"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

//go:embed logo.png
var logoPNG []byte

// demo is an entry of the index.
type demo struct {
	name   string
	detail string
	icon   ui.IconName
	screen func(cx *core.Scope) core.Node
}

// group is demos by what they show.
type group struct {
	title string
	demos []demo
}

// groups are set in init because demos list them.
var groups []group

func init() {
	groups = []group{
		{"Apps", []demo{
			{"Stocks", "A live watchlist and a stock: candles, depth, the tape, a tick every 50 ms", ui.IconStocks, stocksDemo},
			{"AI Chat", "Streaming replies with reasoning, tools and attachments", ui.IconChat, chatDemo},
			{"Photos", "A long recycled list: images, pull to refresh, paging", ui.IconPhotos, photosDemo},
		}},
		{"Layout and lists", []demo{
			{"Lists", "Columns, sticky sections, a chat, horizontal lists", ui.IconLists, listsDemo},
			{"Scrolling", "Paging, snapping, zoom, scroll events, prepending, keyboard taps", ui.IconScroll, scrollingDemo},
			{"Collapsing header", "A header that shrinks to a bar as the page scrolls, headings sticking below it", ui.IconScroll, collapsingDemo},
			{"Layout", "Flexbox, wrapping, positioning, grid and block layout, direction", ui.IconLayout, layoutDemo},
			{"Navigation", "Header options, modals, the back button, safe areas", ui.IconNav, navigationDemo},
			{"Form", "Inputs, keyboard types, the keyboard over the content", ui.IconForm, formDemo},
			{"Resizable", "Panels resized by dragging the lines between them, within their ranges", ui.IconSidebar, resizableDemo},
			{"Dock", "A workspace of tabs dragged between groups and docks, split, closed, zoomed and resized", ui.IconDock, dockDemo},
		}},
		{"Touch and motion", []demo{
			{"Gestures", "Pinch, rotate, press states, hit slop, pointer events, dropped files", ui.IconGestures, gesturesDemo},
			{"Physics", "Drags and springs on the UI thread", ui.IconPhysics, physicsDemo},
			{"Animations", "Easing, sequence / parallel / stagger / loop, springs, interpolation", ui.IconAnimation, animationsDemo},
		}},
		{"Drawing and type", []demo{
			{"Canvas", "2D drawing: paths, gradients, text, images, native animation", ui.IconCanvas, canvasDemo},
			{"Charts", "A line and area plot with hover, grid lines and motion, sparklines in a list", ui.IconStocks, chartsDemo},
			{"Shader", "Fragment shaders in Go's syntax, uniforms the platform animates", ui.IconShader, shaderDemo},
			{"Styles", "Corners, borders, shadows, clipping, filters, 3D transforms", ui.IconStyles, stylesDemo},
			{"Text", "Fonts, spans, inline views, decorations, truncation, languages", ui.IconText, textDemo},
			{"Text View", "Markdown and HTML documents, extensions, streaming replies", ui.IconText, textViewDemo},
			{"Editor", "Code with highlighting, several carets, folding, find and replace, diagnostics, completion and definitions", ui.IconText, editorDemo},
			{"DataTable", "A million stocks of sixty columns, their quotes ticking every 33 ms, fixed columns, sorting, selection, resized and moved columns", ui.IconLists, dataTableDemo},
			{"Diff", "Two texts side by side or unified, changes navigated, reverted and kept, a patch", ui.IconText, diffDemo},
			{"Images", "Resize modes, tint, GIF, placeholders, blur, fade", ui.IconImage, imagesDemo},
		}},
		{"Platform", []demo{
			{"Components", "Press feedback, a sheet, layout animation, input accessory, dialog options", ui.IconComponents, componentsDemo},
			{"Overlays", "Overlay, popovers, tooltips, sheets, toasts", ui.IconOverlays, overlaysDemo},
			{"Menus", "A menu bar, submenus, checkable items, context menus on rows", ui.IconMenu, menusDemo},
			{"Controls", "Switches in settings, a filter with select all, a plan picker, tabs, collapsible sections, progress bars, a table", ui.IconControls, controlsDemo},
			{"Web and video", "Web views, video players and the app's own native views", ui.IconMedia, mediaDemo},
			{"System", "Status bar, alerts, the window, environment, timers, the clipboard, fetch", ui.IconSystem, systemDemo},
			{"Accessibility", "Labels, values, actions, groups, live regions and focus", ui.IconAccess, accessibilityDemo},
			{"Storage", "A launch count, a note saved as you type, the stored keys", ui.IconStorage, storageDemo},
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
func groupOf(name string) string {
	for _, g := range groups {
		for _, d := range g.demos {
			if d.name == name {
				return g.title
			}
		}
	}
	return "Gallery"
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
		nav:          base.NewNav(cx, route{}),
		selected:     core.NewSignal(cx, first),
		sidebarOpen:  core.NewSignal(cx, true),
		sidebarWidth: core.NewSignal(cx, sidebarWidth),
		chosen:       core.NewSignal(cx, "nothing yet"),
		env:          env,
		toasts:       newToaster(cx),
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
		desktop.SetTitle(cx, "Gallery")
		desktop.SetTitleBar(cx, desktop.TitleBarHidden, ui.HeaderHeight)
	}
	rememberDemo(cx, g)
	cx.Effect(func() { desktop.SetApplicationMenu(cx, g.menu()) })

	return core.Dynamic(func(cx *core.Scope) core.Node {
		wide := desktopLayout.Get()
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
			children = append(children, body, g.toasts.view(wide))
			return core.View{StyleFunc: func() core.Style {
				return fillPage.Merge(core.Style{Background: ui.Chrome(cx), Color: colors.Text()})
			}, Children: children}
		})
	})
}

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
			core.Text{Content: "Gallery", MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
			ui.Text("NSGo", 11, ui.Regular, colors.Muted),
		}}
}

// toggleSidebar hides or shows the desktop layout's sidebar.
func (g *gallery) toggleSidebar() { g.sidebarOpen.Update(func(open bool) bool { return !open }) }

// phone is the narrow layout: the index, demos pushed over it.
func (g *gallery) phone(cx *core.Scope) core.Node {
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
			return base.NavPage{Title: d.name, TestID: "screen-" + d.name, HeaderStyleFunc: headerStyle(cx),
				Header:    pageHeader(cx, pageHeaderProps{title: d.name}),
				StyleFunc: pageStyle(cx),
				Children: []core.Node{core.View{Style: fillPage, StyleFunc: pageStyle(cx),
					Children: []core.Node{core.Func(func(cx *core.Scope) core.Node { return d.screen(cx) })}}}}
		}
		return base.NavPage{Title: "Gallery", TestID: "index", HeaderStyleFunc: headerStyle(cx), HeaderRight: barThemeToggle(cx),
			Header:   pageHeader(cx, pageHeaderProps{title: "Gallery", root: true, right: themeToggle(cx)}),
			Children: []core.Node{g.home(cx)}}
	})
}

// home is the phone layout's index of the demos by group.
func (g *gallery) home(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	var nodes []core.Node
	for i, grp := range groups {
		var rows []core.Node
		for j, d := range grp.demos {
			rows = append(rows, demoRow(cx, d, j == 0, func() { g.nav.Push(route{demo: d.name}) }))
		}
		nodes = append(nodes, ui.Appear{Index: i + 1, Children: []core.Node{core.View{TestID: "group-" + grp.title, Style: core.Style{Gap: 8}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Padding: core.Symmetric(0, 4)}, Children: []core.Node{
				core.Text{Content: grp.title, Role: core.RoleHeader, Style: core.Style{FlexGrow: 1, FontSize: 13, FontWeight: ui.Semibold,
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
	colors := ui.ColorsOf(cx)
	s := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14, Padding: core.All(12)}
	if !first {
		s.Borders = core.Edges{Top: 1}
	}
	return ui.Press{OnPress: open, Scale: 0.98, Label: d.name, TestID: "nav-" + d.name, Children: []core.Node{
		core.View{Style: s, StyleFunc: func() core.Style {
			if first {
				return core.Style{}
			}
			return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}}
		}, Children: []core.Node{
			ui.IconTile{Icon: d.icon, Size: 38},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				ui.Text(d.name, 16, ui.Medium, colors.Text),
				core.Text{Content: d.detail, MaxLines: 2, Style: core.Style{FontSize: 13, LineHeight: 18}, StyleFunc: ui.Colored(colors.Muted)},
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
	return ui.SidebarShell{TestID: "shell", Sidebar: []core.Node{g.sidebar()}, SidebarFooter: []core.Node{ui.Spacer{}, themeToggle(cx)},
		Open: g.sidebarOpen.Get, OnOpenChange: g.sidebarOpen.Set, ToggleLabel: "Toggle sidebar",
		Width: g.sidebarWidth.Get, OnWidthChange: g.sidebarWidth.Set, DefaultWidth: sidebarWidth, MinWidth: 200, MaxWidth: 360,
		ResizeLabel: "Resize sidebar",
		Header:      []core.Node{g.demoTitle()}, Actions: []core.Node{g.demoGroup()},
		Content: []core.Node{g.nav.Stack(fillPage, func(cx *core.Scope, r route, _ func() int) base.NavPage {
			if r.page != nil {
				return r.page(cx)
			}
			return base.NavPage{HeaderHidden: true, Children: []core.Node{g.demoPane()}}
		})}}
}

// sidebar is the desktop layout's list of the demos by group.
func (g *gallery) sidebar() core.Node {
	var sections []ui.SidebarSection
	for _, grp := range groups {
		var items []ui.SidebarItem
		for _, d := range grp.demos {
			items = append(items, ui.SidebarItem{ID: d.name, Text: d.name, Icon: d.icon, TestID: "nav-" + d.name})
		}
		sections = append(sections, ui.SidebarSection{Title: grp.title, Items: items, TestID: "nav-group-" + grp.title})
	}
	return ui.Sidebar{TestID: "sidebar", Sections: sections, Selected: g.selected.Get, OnSelect: g.selectDemo}
}

// demoTitle is the selected demo's name and what it shows, in the
// content's header.
func (g *gallery) demoTitle() core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		d, _ := findDemo(g.selected.Get())
		return core.Func(func(cx *core.Scope) core.Node {
			colors := ui.ColorsOf(cx)
			title := ui.Text(d.name, 14, ui.Semibold, colors.Text)
			title.MaxLines, title.Role, title.TestID, title.Style.FlexShrink = 1, core.RoleHeader, "title", 0
			detail := ui.Text(d.detail, 12, ui.Regular, colors.Muted)
			detail.MaxLines, detail.Style.FlexShrink = 1, 1
			return ui.HStack{Gap: 8, Children: []core.Node{ui.Icon(d.icon, 16, colors.Accent), title, detail}}
		})
	})
}

// demoGroup is the selected demo's group, at the end of the content's
// header.
func (g *gallery) demoGroup() core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		return ui.Badge{Text: groupOf(g.selected.Get())}
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
func (g *gallery) menu() []desktop.MenuItem {
	selected := g.selected.Get()
	choose := func(label string, fn func()) func() {
		return func() { g.chosen.Set(label); fn() }
	}
	view := []desktop.MenuItem{
		{Label: "Show Sidebar", Shortcut: "mod+b", Checkable: true, Checked: g.sidebarOpen.Get(), OnSelect: choose("Show Sidebar", g.toggleSidebar)},
		{Separator: true},
	}
	for i, d := range allDemos() {
		item := desktop.MenuItem{Label: d.name, Checkable: true, Checked: d.name == selected, OnSelect: choose(d.name, func() { g.open(d.name) })}
		if i < 9 {
			item.Shortcut = "mod+" + itoa(i+1)
		}
		view = append(view, item)
	}
	return []desktop.MenuItem{
		{Label: "Gallery", Items: []desktop.MenuItem{
			{Role: desktop.MenuAbout}, {Separator: true},
			{Role: desktop.MenuHide}, {Role: desktop.MenuHideOthers}, {Role: desktop.MenuShowAll}, {Separator: true},
			{Role: desktop.MenuQuit},
		}},
		{Label: "Edit", Items: []desktop.MenuItem{
			{Role: desktop.MenuUndo}, {Role: desktop.MenuRedo}, {Separator: true},
			{Role: desktop.MenuCut}, {Role: desktop.MenuCopy}, {Role: desktop.MenuPaste}, {Role: desktop.MenuSelectAll},
		}},
		{Label: "View", Items: view},
		{Label: "Window", Items: []desktop.MenuItem{
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
	nsgo.Run(Root, append(opts, nsgo.WindowMaterial(desktop.WindowMaterialSidebar))...)
}
