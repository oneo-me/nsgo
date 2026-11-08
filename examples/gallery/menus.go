package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// label is a static text.
func label(s string) func() string { return func() string { return s } }

// chip is a pressable that opens a menu.
func chip(cx *core.Scope, label func() string, name func() *core.SVG) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	children := []core.Node{}
	if name != nil {
		children = append(children, ui.Icon(name, 14, colors.Muted))
	}
	children = append(children, ui.Textf(label, 13, ui.Medium, colors.Text))
	return core.View{StyleFunc: func() core.Style {
		return fin.Raised().Over(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, AlignSelf: core.AlignStart,
			Padding: core.Symmetric(0, 12), Height: core.Pt(34), JustifyContent: core.JustifyCenter, CornerRadius: ui.RadiusControl, Background: colors.Raised(),
			Active: &core.Style{Background: colors.Border()}, Hover: &core.Style{Background: colors.Border()}})
	}, Children: children}
}

var zones = []string{
	"UTC−10 Honolulu", "UTC−8 Los Angeles", "UTC−7 Denver", "UTC−6 Chicago", "UTC−5 New York", "UTC−3 São Paulo",
	"UTC±0 London", "UTC+1 Berlin", "UTC+2 Cairo", "UTC+3 Moscow", "UTC+4 Dubai", "UTC+5:30 Mumbai", "UTC+7 Bangkok",
	"UTC+8 Hong Kong", "UTC+8 Shanghai", "UTC+8 Singapore", "UTC+9 Tokyo", "UTC+9 Seoul", "UTC+10 Sydney", "UTC+12 Auckland",
}

type menuFile struct {
	id      int
	name    string
	starred bool
}

// menusDemo is ui's menus as complete as an app's: a menu bar whose
// menus have shortcuts, submenus three levels deep, checkable items and a
// choice of one, group titles, disabled and destructive items; menus on
// the rows of a list; where a popup goes; a long menu that scrolls; a menu
// a shortcut opens. On desktops they are popups with cascading submenus,
// on touch screens sheets whose submenus take the place of their items.
// Then the platform's own menus: the application menu and a popup menu.
func menusDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	onDesktop := cx.Env().Desktop()
	events := core.NewSignalFunc(cx, []string(nil), slices.Equal)
	log := func(what string) {
		events.Update(func(es []string) []string { return append([]string{what}, es[:min(len(es), 5)]...) })
	}
	changed := func(menu string) func(bool, base.Reason) {
		return func(open bool, reason base.Reason) {
			state := tr.MenusClosed()
			if open {
				state = tr.MenusOpened()
			}
			log(fmt.Sprintf("%s %s · %s", menu, state, reason))
		}
	}
	menu := func(name string, face core.Node, children ...core.Node) ui.Menu {
		return ui.Menu{OnOpenChange: changed(name), Label: name, Face: face, Children: children}
	}

	barMenu := func(name string, children ...core.Node) ui.Menu {
		return ui.Menu{OnOpenChange: changed(name), Label: name, Trigger: ui.Button{Text: name, Variant: ui.ButtonGhost}, Children: children}
	}

	menuBar := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		history, undone := core.NewSignalFunc(cx, []string(nil), slices.Equal), core.NewSignalFunc(cx, []string(nil), slices.Equal)
		sidebar, statusBar := core.NewSignal(cx, true), core.NewSignal(cx, false)
		zoom := core.NewSignal(cx, tr.MenusMedium())
		recent := core.NewSignalFunc(cx, []string{"Budget.numbers", "Roadmap.md", "Trip photos"}, slices.Equal)
		act := func(what string) {
			history.Update(func(h []string) []string { return append(slices.Clone(h), what) })
			undone.Set(nil)
			log(tr.MenusChose() + what)
		}
		move := func(from, to *core.Signal[[]string], verb string) {
			f := from.GetUntracked()
			if len(f) == 0 {
				return
			}
			last := f[len(f)-1]
			from.Set(slices.Clone(f[:len(f)-1]))
			to.Update(func(t []string) []string { return append(slices.Clone(t), last) })
			log(verb + " " + last)
		}
		undo, redo := func() { move(history, undone, "undid") }, func() { move(undone, history, "redid") }
		// The shortcuts the items show work without the menu too.
		cx.OnKey("mod+alt+n", func() { act(tr.MenusNewDocument()) })
		cx.OnKey("mod+alt+s", func() { act(tr.MenusSave()) })
		cx.OnKey("mod+alt+z", undo)
		cx.OnKey("mod+alt+shift+z", redo)
		lastOf := func(s *core.Signal[[]string], verb string) string {
			if h := s.Get(); len(h) > 0 {
				return verb + " " + h[len(h)-1]
			}
			return verb
		}
		empty := func(s *core.Signal[[]string]) func() bool { return func() bool { return len(s.Get()) == 0 } }
		var sizes []core.Node
		for _, size := range []string{tr.MenusSmall(), tr.MenusMedium(), tr.MenusLarge()} {
			sizes = append(sizes, ui.MenuItem{Text: size, Checked: func() bool { return zoom.Get() == size }, NoCloseOnSelect: true, OnSelect: func() { zoom.Set(size) }})
		}
		return Card{Title: tr.MenusMenuBar(), Description: Hint(cx, tr.MenusEachMenuASheetSubmenus(), tr.MenusHoverAcrossSubmenusTheShortcuts()), Children: []core.Node{
			ui.Toolbar{Label: tr.MenusMenuBar(), Children: []core.Node{
				barMenu(tr.MenusFile(),
					ui.MenuItem{Text: tr.MenusNewDocument(), Shortcut: "mod+alt+n", OnSelect: func() { act(tr.MenusNewDocument()) }},
					ui.MenuItem{Text: tr.MenusNewWindow(), OnSelect: func() { act(tr.MenusNewWindow()) }},
					ui.Submenu{Text: tr.MenusOpenRecent(), Back: tr.MenusOpenRecent(), Children: []core.Node{
						core.Each(recent.Get, func(s string) string { return s }, func(cx *core.Scope, name func() string, _ func() int) core.Node {
							tr := locales.For(cx)
							return ui.MenuItem{Text: core.Untracked(cx, name), OnSelect: func() { act(tr.MenusOpen() + name()) }}
						}),
						ui.MenuSeparator{},
						ui.MenuItem{Text: tr.MenusClearRecent(), Disabled: empty(recent), OnSelect: func() { recent.Set(nil) }},
					}},
					ui.MenuSeparator{},
					ui.MenuItem{Text: tr.MenusSave(), Shortcut: "mod+alt+s", OnSelect: func() { act(tr.MenusSave()) }},
					ui.Submenu{Text: tr.MenusExportAs(), Back: tr.MenusExportAs(), Children: []core.Node{
						ui.MenuItem{Text: tr.MenusPdf(), OnSelect: func() { act(tr.MenusExportPdf()) }},
						ui.Submenu{Text: tr.MenusImage(), Back: tr.MenusImage(), Children: []core.Node{
							ui.MenuItem{Text: tr.MenusPng(), OnSelect: func() { act(tr.MenusExportPng()) }},
							ui.MenuItem{Text: tr.MenusJpeg(), OnSelect: func() { act(tr.MenusExportJpeg()) }},
							ui.MenuItem{Text: tr.MenusWebp(), OnSelect: func() { act(tr.MenusExportWebp()) }},
						}},
						ui.Submenu{Text: tr.MenusWebPage(), Back: tr.MenusWebPage(), Disabled: func() bool { return true }, Children: []core.Node{ui.MenuItem{Text: tr.MenusHtml()}}},
					}},
					ui.MenuSeparator{},
					ui.MenuItem{Text: tr.MenusDeleteDocument(), Variant: ui.MenuItemDanger, OnSelect: func() { act(tr.MenusDeleteDocument()) }},
				),
				barMenu(tr.MenusEdit(),
					core.Dynamic(func(*core.Scope) core.Node {
						return ui.MenuItem{Text: lastOf(history, tr.MenusUndo()), Shortcut: "mod+alt+z", Disabled: empty(history), OnSelect: undo}
					}),
					core.Dynamic(func(*core.Scope) core.Node {
						return ui.MenuItem{Text: lastOf(undone, tr.MenusRedo()), Shortcut: "mod+alt+shift+z", Disabled: empty(undone), OnSelect: redo}
					}),
					ui.MenuSeparator{},
					ui.MenuLabel{Text: tr.MenusClipboard()},
					ui.MenuItem{Text: tr.MenusCut(), OnSelect: func() { act(tr.MenusCut()) }},
					ui.MenuItem{Text: tr.MenusCopy(), OnSelect: func() { act(tr.MenusCopy()) }},
					ui.MenuItem{Text: tr.MenusPaste(), OnSelect: func() { act(tr.MenusPaste()) }},
				),
				barMenu(tr.MenusView(), append([]core.Node{
					ui.MenuLabel{Text: tr.MenusShow()},
					ui.MenuItem{Text: tr.MenusSidebar(), Checked: sidebar.Get, NoCloseOnSelect: true, OnSelect: func() { toggled(sidebar) }},
					ui.MenuItem{Text: tr.MenusStatusBar(), Checked: statusBar.Get, NoCloseOnSelect: true, OnSelect: func() { toggled(statusBar) }},
					ui.MenuSeparator{},
					ui.MenuLabel{Text: tr.MenusZoom()},
				}, sizes...)...),
			}},
			Caption{LabelFunc: func() string {
				return tr.MenusViewState(onOff(tr, sidebar.Get()), onOff(tr, statusBar.Get()), zoom.Get())
			}},
			Caption{LabelFunc: func() string {
				if h := history.Get(); len(h) > 0 {
					return tr.MenusDone() + strings.Join(h, ", ")
				}
				return tr.MenusNothingDoneYet()
			}},
		}}
	}

	rows := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		next := 5
		files := core.NewSignalFunc(cx, []menuFile{{1, "Invoice March.pdf", false}, {2, "Holiday.jpg", true}, {3, "Notes.txt", false}, {4, "Slides.key", false}}, slices.Equal)
		find := func(id int) menuFile {
			for _, f := range files.GetUntracked() {
				if f.id == id {
					return f
				}
			}
			return menuFile{}
		}
		actions := func(id int) []core.Node {
			name := func() string { return find(id).name }
			return []core.Node{
				ui.MenuItem{Text: tr.MenusOpen2(), OnSelect: func() { log(tr.MenusOpen3() + name()) }},
				core.Dynamic(func(*core.Scope) core.Node {
					text := tr.MenusStar()
					for _, f := range files.Get() {
						if f.id == id && f.starred {
							text = tr.MenusUnstar()
						}
					}
					return ui.MenuItem{Text: text, OnSelect: func() {
						files.Update(func(fs []menuFile) []menuFile {
							out := slices.Clone(fs)
							for i := range out {
								if out[i].id == id {
									out[i].starred = !out[i].starred
								}
							}
							return out
						})
					}}
				}),
				ui.MenuItem{Text: tr.MenusDuplicate(), OnSelect: func() {
					f := find(id)
					ext := ""
					base := f.name
					if i := strings.LastIndex(f.name, "."); i > 0 {
						base, ext = f.name[:i], f.name[i:]
					}
					c := menuFile{id: next, name: base + tr.MenusCopy2() + ext}
					next++
					files.Update(func(fs []menuFile) []menuFile {
						at := slices.IndexFunc(fs, func(x menuFile) bool { return x.id == id })
						return slices.Insert(slices.Clone(fs), at+1, c)
					})
				}},
				ui.Submenu{Text: tr.MenusMoveTo(), Back: tr.MenusMoveTo(), Children: []core.Node{
					ui.MenuItem{Text: tr.MenusDocuments(), OnSelect: func() { log(tr.MenusMoved() + name() + tr.MenusToDocuments()) }},
					ui.MenuItem{Text: tr.MenusArchive(), OnSelect: func() { log(tr.MenusMoved() + name() + tr.MenusToArchive()) }},
				}},
				ui.MenuSeparator{},
				ui.MenuItem{Text: tr.MenusDelete(), Variant: ui.MenuItemDanger, OnSelect: func() {
					files.Update(func(fs []menuFile) []menuFile {
						return slices.DeleteFunc(slices.Clone(fs), func(x menuFile) bool { return x.id == id })
					})
				}},
			}
		}
		return Card{Title: tr.MenusOnTheRowsOfA(), Description: Hint(cx, tr.MenusHoldAFingerOnA(), tr.MenusRightClickARowOr()), Children: []core.Node{
			core.Show{When: func() bool { return len(files.Get()) > 0 }, Else: func(*core.Scope) core.Node { return Caption{Label: tr.MenusNoFilesLeft()} },
				Then: func(*core.Scope) core.Node {
					return core.View{Children: []core.Node{core.Each(files.Get, func(f menuFile) int { return f.id },
						func(cx *core.Scope, file func() menuFile, i func() int) core.Node {
							tr := locales.For(cx)
							colors := ui.ColorsOf(cx)
							id := core.Untracked(cx, file).id
							starred := func() bool { return file().starred }
							return ui.ContextMenu{OnOpenChange: changed(core.Untracked(cx, file).name),
								Trigger: core.View{StyleFunc: func() core.Style {
									s := core.Style{MinHeight: core.Pt(48), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}
									if i() != 0 {
										s.Borders, s.BorderColors = core.Edges{Top: 1}, core.EdgeColors{Top: colors.Line()}
									}
									return s
								}, Children: []core.Node{
									core.Dynamic(func(*core.Scope) core.Node {
										if starred() {
											return ui.Icon(lucide.Star, 16, colors.Accent)
										}
										return ui.Icon(lucide.Type, 16, colors.Muted)
									}),
									core.Text{ContentFunc: func() string { return file().name }, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
									ui.Menu{Align: core.AlignEnd, Label: tr.MenusActionsFor() + core.Untracked(cx, file).name,
										Face: core.View{Tooltip: core.Text{Content: tr.MenusMoreActions()}, HitSlop: core.All(8),
											Style:    core.Style{Padding: core.Symmetric(0, 8), Height: core.Pt(28), JustifyContent: core.JustifyCenter, Active: &core.Style{Opacity: core.Alpha(0.5)}},
											Children: []core.Node{ui.Text("⋯", 18, ui.Regular, colors.Muted)}},
										Children: actions(id)},
								}},
								Children: actions(id)}
						})}}
				}},
		}}
	}

	longList := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		zone := core.NewSignal(cx, "UTC+8 Hong Kong")
		var items []core.Node
		for _, z := range zones {
			items = append(items, ui.MenuItem{Text: z, Checked: func() bool { return zone.Get() == z }, OnSelect: func() { zone.Set(z) }})
		}
		maxHeight := core.Points(360)
		if onDesktop {
			maxHeight = 280
		}
		m := menu(tr.MenusTimeZone(), chip(cx, zone.Get, lucide.Search), items...)
		m.MaxHeight = maxHeight
		return Card{Title: tr.MenusALongMenu(), Description: tr.MenusTwentyChoicesInAMenu(), Children: []core.Node{m}}
	}

	commands := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		open := core.NewSignal(cx, false)
		cx.OnKey("mod+shift+p", func() { toggled(open) })
		var right core.Node
		if onDesktop {
			right = ui.Badge{Text: cx.KeyLabel("mod+shift+p")}
		}
		m := menu(tr.MenusCommands(), chip(cx, label(tr.MenusCommands()), lucide.Plus),
			ui.MenuLabel{Text: tr.MenusCommands()},
			ui.MenuItem{Text: tr.MenusNewNote(), OnSelect: func() { log(tr.MenusRanNewNote()) }},
			ui.MenuItem{Text: tr.MenusToggleTheme(), OnSelect: func() { log(tr.MenusRanToggleTheme()) }},
			ui.MenuItem{Text: tr.MenusSyncNow(), OnSelect: func() { log(tr.MenusRanSyncNow()) }},
		)
		m.Open = open.Get
		m.OnOpenChange = func(next bool, reason base.Reason) {
			open.Set(next)
			changed("commands")(next, reason)
		}
		return Card{Title: tr.MenusOpenedByTheApp(), Description: tr.MenusAControlledMenuAButton(), Actions: []core.Node{right}, Children: []core.Node{
			m, ui.Button{Text: tr.MenusOpenTheCommands(), Variant: ui.ButtonSecondary, OnPress: func() { open.Set(true) }},
		}}
	}

	native := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		chosen := core.NewSignal(cx, tr.MenusNothingChosenYet())
		items := func() []desktop.MenuItem {
			choose := func(s string) func() { return func() { chosen.Set(tr.MenusChose2() + s); log(tr.MenusNative() + s) } }
			return []desktop.MenuItem{
				{Label: tr.MenusCut(), Shortcut: "mod+x", OnSelect: choose(tr.MenusCut())},
				{Label: tr.MenusCopy(), Shortcut: "mod+c", OnSelect: choose(tr.MenusCopy())},
				{Label: tr.MenusPaste(), Disabled: true},
				{Separator: true},
				{Label: tr.MenusShare(), Items: []desktop.MenuItem{{Label: tr.MenusMail(), OnSelect: choose(tr.MenusMail())}, {Label: tr.MenusMessages(), OnSelect: choose(tr.MenusMessages())}}},
				{Label: tr.MenusPinned(), Checkable: true, Checked: true, OnSelect: choose(tr.MenusPinned())},
			}
		}
		show := func(at core.Point) { desktop.PopupMenu(cx, items(), at, nil) }
		return Card{Title: tr.MenusThePlatformSMenu(), Description: tr.MenusPopupmenuShowsTheSystemS(), Actions: []core.Node{ui.Badge{Text: tr.MenusNative2(), Variant: ui.BadgeInfo}}, Children: []core.Node{
			core.View{TestID: "popup-area", Role: core.RoleButton, Label: tr.MenusShowTheNativeMenu(),
				StyleFunc: func() core.Style {
					return core.Style{Height: core.Pt(72), CornerRadius: ui.RadiusInset, Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border(),
						AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Active: &core.Style{Background: colors.Pressed()}}
				},
				OnPress:       func(e core.PressEvent) { show(e.PagePosition) },
				OnContextMenu: func(e core.ContextMenuEvent) { show(e.PagePosition) },
				Children:      []core.Node{ui.Text(Hint(cx, tr.MenusPressHere(), tr.MenusRightClickOrPressHere()), 13, ui.Regular, colors.Muted)}},
			Caption{LabelFunc: chosen.Get},
		}}
	}

	appMenu := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		children := []core.Node{
			Note{Text: tr.MenusSetapplicationmenuSetsTheMenuBar()},
			Caption{Label: tr.MenusLastChosen(), Note: shell.chosen.Get},
		}
		if env := cx.Env(); env.Desktop() && env.OS() != core.OSMacOS && env.OS() != core.OSWeb {
			children = append(children, Note{Text: tr.MenusSetmenubarHidesTheBarWindows()},
				ui.HStack{Gap: 8, Children: []core.Node{
					ui.Button{Text: tr.MenusAlwaysShown(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: func() { desktop.SetMenuBar(cx, desktop.MenuBarShown) }},
					ui.Button{Text: tr.MenusAutoHide(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: func() { desktop.SetMenuBar(cx, desktop.MenuBarAutoHide) }},
				}})
		}
		return Card{Title: tr.MenusTheApplicationMenu(), Description: tr.MenusTheSystemSMenuBar(), Actions: []core.Node{ui.Badge{Text: tr.MenusNative2(), Variant: ui.BadgeInfo}}, Children: children}
	}

	eventsCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return Card{Title: tr.MenusEvents(), Description: tr.MenusOnopenchangeWithItsReasonAnd(), Children: []core.Node{
			core.Show{When: func() bool { return len(events.Get()) > 0 }, Else: func(*core.Scope) core.Node { return Caption{Label: tr.MenusOpenAMenu()} },
				Then: func(*core.Scope) core.Node {
					return core.View{Style: core.Style{Gap: 4}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
						var lines []core.Node
						for i, e := range events.Get() {
							c := colors.Faint
							if i == 0 {
								c = colors.Text
							}
							lines = append(lines, core.Text{Content: e, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(c)})
						}
						return core.View{Style: core.Style{Gap: 4}, Children: lines}
					})}}
				}},
		}}
	}

	return demoPage(cx, pageProps{}, menuPlayground, contextMenuCard, menuBar, rows, longList, commands, native, appMenu, eventsCard)
}
