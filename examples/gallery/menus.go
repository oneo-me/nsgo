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

// The look of every menu here is the gallery's: base draws none.

// menuLook is the menus' look on the device: popups on desktops, sheets
// at the bottom on touch screens.
type menuLook struct {
	desktop bool
	height  core.Points
}

func newMenuLook(cx *core.Scope) menuLook {
	l := menuLook{desktop: cx.Env().Desktop(), height: 46}
	if l.desktop {
		l.height = 32
	}
	return l
}

// surface is the popup on desktops, the sheet at the bottom on touch
// screens, for a PopupStyleFunc.
func (l menuLook) surface(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		if l.desktop {
			return ui.FinOf(cx).Raised().Over(core.Style{Background: colors.Raised(), CornerRadius: ui.RadiusInset, BorderWidth: 1, BorderColor: colors.Border(),
				Padding: core.All(6), MinWidth: core.Pt(220)})
		}
		return core.Style{Background: colors.Surface(), CornerRadii: core.Corners{TopLeft: 20, TopRight: 20},
			Padding: core.Edges{Top: 10, Right: 10, Bottom: 34, Left: 10}}
	}
}

func (menuLook) backdrop() core.Style { return core.Style{Background: ui.Black(0.45)} }

// itemHovered and itemPressed are an item under the pointer and pressed:
// an orange tint with a hairline of it, read in a StyleFunc.
func itemHovered(cx *core.Scope) *core.Style {
	if cx.Dark() {
		return &core.Style{Background: core.RGBA(0xed986e, 0.08), BorderColor: core.RGBA(0xed986e, 0.22)}
	}
	return &core.Style{Background: core.RGBA(0xd4581b, 0.08), BorderColor: core.RGBA(0xd4581b, 0.24)}
}

func itemPressed(cx *core.Scope) *core.Style {
	if cx.Dark() {
		return &core.Style{Background: core.RGBA(0xed986e, 0.14), BorderColor: core.RGBA(0xed986e, 0.32)}
	}
	return &core.Style{Background: core.RGBA(0xd4581b, 0.14), BorderColor: core.RGBA(0xd4581b, 0.34)}
}

// itemProps are a menu row: a mark at the trailing edge (a check, a
// dot), the label and a note (a shortcut, a value).
type itemProps struct {
	label    func() string
	onSelect func()
	note     string
	mark     func() mark
	disabled func() bool
	danger   bool
	keepOpen bool
}

// markDot is a menu row's choice of one.
const markDot = markDash

func (l menuLook) rowStyle() core.Style {
	return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Height: core.Pt(l.height), Padding: core.Symmetric(0, 8),
		CornerRadius: 7, BorderWidth: 1, BorderColor: transparent}
}

func (l menuLook) item(cx *core.Scope, p itemProps) core.Node {
	colors := ui.ColorsOf(cx)
	fg := colors.Text
	if p.danger {
		fg = colors.Danger
	}
	s := l.rowStyle()
	s.Disabled = &core.Style{Opacity: core.Alpha(0.35)}
	states := func() core.Style { return core.Style{Hover: itemHovered(cx), Active: itemPressed(cx)} }
	children := []core.Node{core.Text{ContentFunc: p.label, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(fg)}}
	if p.note != "" {
		children = append(children, API{Text: p.note})
	}
	if p.mark != nil {
		children = append(children, core.View{Style: core.Style{Width: core.Pt(16), AlignItems: core.AlignCenter}, Children: []core.Node{
			core.Dynamic(func(*core.Scope) core.Node {
				switch p.mark() {
				case markCheck:
					return ui.Icon(lucide.Check, 14, colors.Accent)
				case markDot:
					return core.View{Style: core.Style{Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 4}, StyleFunc: ui.Filled(colors.Accent)}
				}
				return core.Group{}
			}),
		}})
	}
	return base.MenuItem{OnSelect: p.onSelect, Disabled: p.disabled, NoCloseOnSelect: p.keepOpen, Style: s, StyleFunc: states, Children: children}
}

// label is a static item label.
func label(s string) func() string { return func() string { return s } }

// submenu is the row that opens a submenu, and the row a sheet shows to
// come back.
func (l menuLook) submenu(cx *core.Scope, name string, disabled bool, children ...core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	row := l.rowStyle()
	row.Gap = 0
	if disabled {
		row.Opacity = core.Alpha(0.35)
	}
	back := l.rowStyle()
	back.Gap = 6
	return base.Submenu{Disabled: func() bool { return disabled }, Label: name,
		Trigger: core.View{Style: row, StyleFunc: func() core.Style { return core.Style{Hover: itemHovered(cx), Active: itemPressed(cx)} }, Children: []core.Node{
			core.Text{Content: name, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
			ui.Icon(lucide.ChevronRight, 14, colors.Faint),
		}},
		Back:     core.View{Style: back, StyleFunc: func() core.Style { return core.Style{Active: itemPressed(cx)} }, Children: []core.Node{ui.Icon(lucide.ChevronLeft, 14, colors.Accent), ui.Text(name, 13, ui.Semibold, colors.Accent)}},
		Children: children}
}

// groupTitle is a group's title: not an item, nothing to press.
func groupTitle(cx *core.Scope, s string) core.Node {
	colors := ui.ColorsOf(cx)
	return core.Text{Content: s, Style: core.Style{FontSize: 11, FontWeight: ui.Semibold, LetterSpacing: 0.6,
		TextTransform: core.TextTransformUppercase, Padding: core.Edges{Top: 8, Right: 8, Bottom: 4, Left: 8}}, StyleFunc: ui.Colored(colors.Faint)}
}

func separator(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return base.MenuSeparator{Style: core.Style{Height: core.Pt(1), Margin: core.Symmetric(4, 4)}, StyleFunc: ui.Filled(colors.Line)}
}

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

// menusDemo is base's menus as complete as an app's: a menu bar whose
// menus have shortcuts, submenus three levels deep, checkable items and a
// choice of one, group titles, disabled and destructive items; menus on
// the rows of a list; where a popup goes; a long menu that scrolls; a menu
// a shortcut opens. On desktops they are popups with cascading submenus,
// on touch screens sheets whose submenus take the place of their items.
// Then the platform's own menus: the application menu and a popup menu.
func menusDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	l := newMenuLook(cx)
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
	key := func(k string) string {
		if l.desktop {
			return cx.KeyLabel(k)
		}
		return ""
	}
	menu := func(name string, trigger core.Node, children ...core.Node) base.Menu {
		return base.Menu{Offset: 4, Margin: 8, PopupStyleFunc: l.surface(cx), BackdropStyle: l.backdrop(), OnOpenChange: changed(name),
			Label: name, Trigger: trigger, Children: children}
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
		lastOf := func(s *core.Signal[[]string], verb string) func() string {
			return func() string {
				if h := s.Get(); len(h) > 0 {
					return verb + " " + h[len(h)-1]
				}
				return verb
			}
		}
		empty := func(s *core.Signal[[]string]) func() bool { return func() bool { return len(s.Get()) == 0 } }
		check := func(s *core.Signal[bool]) func() mark {
			return func() mark {
				if s.Get() {
					return markCheck
				}
				return markNone
			}
		}
		var sizes []core.Node
		for _, size := range []string{tr.MenusSmall(), tr.MenusMedium(), tr.MenusLarge()} {
			sizes = append(sizes, l.item(cx, itemProps{label: label(size), keepOpen: true, onSelect: func() { zoom.Set(size) }, mark: func() mark {
				if zoom.Get() == size {
					return markDot
				}
				return markNone
			}}))
		}
		return ui.Card{Title: tr.MenusMenuBar(), Description: Hint(cx, tr.MenusEachMenuASheetSubmenus(), tr.MenusHoverAcrossSubmenusTheShortcuts()), Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				menu(tr.MenusFile(), chip(cx, label(tr.MenusFile()), nil),
					l.item(cx, itemProps{label: label(tr.MenusNewDocument()), note: key("mod+alt+n"), onSelect: func() { act(tr.MenusNewDocument()) }}),
					l.item(cx, itemProps{label: label(tr.MenusNewWindow()), onSelect: func() { act(tr.MenusNewWindow()) }}),
					l.submenu(cx, tr.MenusOpenRecent(), false,
						core.Each(recent.Get, func(s string) string { return s }, func(cx *core.Scope, name func() string, _ func() int) core.Node {
							tr := locales.For(cx)
							return l.item(cx, itemProps{label: name, onSelect: func() { act(tr.MenusOpen() + name()) }})
						}),
						separator(cx),
						l.item(cx, itemProps{label: label(tr.MenusClearRecent()), disabled: empty(recent), onSelect: func() { recent.Set(nil) }}),
					),
					separator(cx),
					l.item(cx, itemProps{label: label(tr.MenusSave()), note: key("mod+alt+s"), onSelect: func() { act(tr.MenusSave()) }}),
					l.submenu(cx, tr.MenusExportAs(), false,
						l.item(cx, itemProps{label: label(tr.MenusPdf()), onSelect: func() { act(tr.MenusExportPdf()) }}),
						l.submenu(cx, tr.MenusImage(), false,
							l.item(cx, itemProps{label: label(tr.MenusPng()), onSelect: func() { act(tr.MenusExportPng()) }}),
							l.item(cx, itemProps{label: label(tr.MenusJpeg()), onSelect: func() { act(tr.MenusExportJpeg()) }}),
							l.item(cx, itemProps{label: label(tr.MenusWebp()), onSelect: func() { act(tr.MenusExportWebp()) }}),
						),
						l.submenu(cx, tr.MenusWebPage(), true, l.item(cx, itemProps{label: label(tr.MenusHtml())})),
					),
					separator(cx),
					l.item(cx, itemProps{label: label(tr.MenusDeleteDocument()), danger: true, onSelect: func() { act(tr.MenusDeleteDocument()) }}),
				),
				menu(tr.MenusEdit(), chip(cx, label(tr.MenusEdit()), nil),
					l.item(cx, itemProps{label: lastOf(history, tr.MenusUndo()), note: key("mod+alt+z"), disabled: empty(history), onSelect: undo}),
					l.item(cx, itemProps{label: lastOf(undone, tr.MenusRedo()), note: key("mod+alt+shift+z"), disabled: empty(undone), onSelect: redo}),
					separator(cx),
					groupTitle(cx, tr.MenusClipboard()),
					l.item(cx, itemProps{label: label(tr.MenusCut()), onSelect: func() { act(tr.MenusCut()) }}),
					l.item(cx, itemProps{label: label(tr.MenusCopy()), onSelect: func() { act(tr.MenusCopy()) }}),
					l.item(cx, itemProps{label: label(tr.MenusPaste()), onSelect: func() { act(tr.MenusPaste()) }}),
				),
				menu(tr.MenusView(), chip(cx, label(tr.MenusView()), nil), append([]core.Node{
					groupTitle(cx, tr.MenusShow()),
					l.item(cx, itemProps{label: label(tr.MenusSidebar()), mark: check(sidebar), keepOpen: true, onSelect: func() { toggled(sidebar) }}),
					l.item(cx, itemProps{label: label(tr.MenusStatusBar()), mark: check(statusBar), keepOpen: true, onSelect: func() { toggled(statusBar) }}),
					separator(cx),
					groupTitle(cx, tr.MenusZoom()),
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
				l.item(cx, itemProps{label: label(tr.MenusOpen2()), onSelect: func() { log(tr.MenusOpen3() + name()) }}),
				l.item(cx, itemProps{label: func() string {
					for _, f := range files.Get() {
						if f.id == id && f.starred {
							return tr.MenusUnstar()
						}
					}
					return tr.MenusStar()
				}, onSelect: func() {
					files.Update(func(fs []menuFile) []menuFile {
						out := slices.Clone(fs)
						for i := range out {
							if out[i].id == id {
								out[i].starred = !out[i].starred
							}
						}
						return out
					})
				}}),
				l.item(cx, itemProps{label: label(tr.MenusDuplicate()), onSelect: func() {
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
				}}),
				l.submenu(cx, tr.MenusMoveTo(), false,
					l.item(cx, itemProps{label: label(tr.MenusDocuments()), onSelect: func() { log(tr.MenusMoved() + name() + tr.MenusToDocuments()) }}),
					l.item(cx, itemProps{label: label(tr.MenusArchive()), onSelect: func() { log(tr.MenusMoved() + name() + tr.MenusToArchive()) }}),
				),
				separator(cx),
				l.item(cx, itemProps{label: label(tr.MenusDelete()), danger: true, onSelect: func() {
					files.Update(func(fs []menuFile) []menuFile {
						return slices.DeleteFunc(slices.Clone(fs), func(x menuFile) bool { return x.id == id })
					})
				}}),
			}
		}
		return ui.Card{Title: tr.MenusOnTheRowsOfA(), Description: Hint(cx, tr.MenusHoldAFingerOnA(), tr.MenusRightClickARowOr()), Children: []core.Node{
			core.Show{When: func() bool { return len(files.Get()) > 0 }, Else: func(*core.Scope) core.Node { return Caption{Label: tr.MenusNoFilesLeft()} },
				Then: func(*core.Scope) core.Node {
					return core.View{Children: []core.Node{core.Each(files.Get, func(f menuFile) int { return f.id },
						func(cx *core.Scope, file func() menuFile, i func() int) core.Node {
							tr := locales.For(cx)
							colors := ui.ColorsOf(cx)
							id := core.Untracked(cx, file).id
							starred := func() bool { return file().starred }
							return base.ContextMenu{Margin: 8, PopupStyleFunc: l.surface(cx), BackdropStyle: l.backdrop(), OnOpenChange: changed(core.Untracked(cx, file).name),
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
									base.Menu{Align: core.AlignEnd, Offset: 4, Margin: 8, PopupStyleFunc: l.surface(cx), BackdropStyle: l.backdrop(),
										Label: tr.MenusActionsFor() + core.Untracked(cx, file).name,
										Trigger: core.View{Tooltip: core.Text{Content: tr.MenusMoreActions()}, HitSlop: core.All(8),
											Style:    core.Style{Padding: core.Symmetric(0, 8), Height: core.Pt(28), JustifyContent: core.JustifyCenter, Active: &core.Style{Opacity: core.Alpha(0.5)}},
											Children: []core.Node{ui.Text("⋯", 18, ui.Regular, colors.Muted)}},
										Children: actions(id)},
								}},
								Children: actions(id)}
						})}}
				}},
		}}
	}

	placement := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		side := core.NewSignal(cx, core.PlaceBottom)
		align := core.NewSignal(cx, core.AlignStart)
		alignName := map[core.Align]string{core.AlignStart: "start", core.AlignCenter: "center", core.AlignEnd: "end"}
		return ui.Card{Title: tr.MenusWhereItGoes(), Description: Hint(cx, tr.MenusPhonesShowMenusAsSheets(), tr.MenusTheSideAndAlignmentIt()), Children: []core.Node{
			ui.Segmented[core.Placement]{Options: []ui.Option[core.Placement]{{Value: core.PlaceTop, Label: tr.MenusTop()}, {Value: core.PlaceBottom, Label: tr.MenusBottom()}, {Value: core.PlaceLeft, Label: tr.MenusLeft()}, {Value: core.PlaceRight, Label: tr.MenusRight()}}, Value: side.Get, OnChange: side.Set, Small: true},
			ui.Segmented[core.Align]{Options: []ui.Option[core.Align]{{Value: core.AlignStart, Label: tr.MenusStart()}, {Value: core.AlignCenter, Label: tr.MenusCenter()}, {Value: core.AlignEnd, Label: tr.MenusEnd()}}, Value: align.Get, OnChange: align.Set, Small: true},
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Padding: core.Symmetric(24, 0)}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				p, a := side.Get(), align.Get()
				m := menu(tr.MenusPlaced(), chip(cx, label(placementName(p)+" · "+alignName[a]), nil),
					l.item(cx, itemProps{label: label(tr.MenusOne()), onSelect: func() { log(tr.MenusChoseOne()) }}),
					l.item(cx, itemProps{label: label(tr.MenusTwo()), onSelect: func() { log(tr.MenusChoseTwo()) }}),
					l.item(cx, itemProps{label: label(tr.MenusThree()), onSelect: func() { log(tr.MenusChoseThree()) }}),
				)
				m.Placement, m.Align, m.Offset = p, a, 6
				return m
			})}},
		}}
	}

	longList := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		zone := core.NewSignal(cx, "UTC+8 Hong Kong")
		var items []core.Node
		for _, z := range zones {
			items = append(items, l.item(cx, itemProps{label: label(z), onSelect: func() { zone.Set(z) }, mark: func() mark {
				if zone.Get() == z {
					return markCheck
				}
				return markNone
			}}))
		}
		maxHeight := core.Points(360)
		if l.desktop {
			maxHeight = 280
		}
		m := menu(tr.MenusTimeZone(), chip(cx, zone.Get, lucide.Search),
			core.Scroll{Style: core.Style{MaxHeight: core.Pt(maxHeight)}, Children: []core.Node{core.View{Style: core.Style{Padding: core.All(6)}, Children: items}}})
		m.PopupStyle.Padding = core.Edges{}
		if !l.desktop {
			m.PopupStyle.Padding = core.Edges{Bottom: 34}
		}
		return ui.Card{Title: tr.MenusALongMenu(), Description: tr.MenusTwentyChoicesInAMenu(), Children: []core.Node{m}}
	}

	commands := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		open := core.NewSignal(cx, false)
		cx.OnKey("mod+shift+p", func() { toggled(open) })
		var right core.Node
		if l.desktop {
			right = ui.Badge{Text: cx.KeyLabel("mod+shift+p")}
		}
		m := menu(tr.MenusCommands(), chip(cx, label(tr.MenusCommands()), lucide.Plus),
			groupTitle(cx, tr.MenusCommands()),
			l.item(cx, itemProps{label: label(tr.MenusNewNote()), onSelect: func() { log(tr.MenusRanNewNote()) }}),
			l.item(cx, itemProps{label: label(tr.MenusToggleTheme()), onSelect: func() { log(tr.MenusRanToggleTheme()) }}),
			l.item(cx, itemProps{label: label(tr.MenusSyncNow()), onSelect: func() { log(tr.MenusRanSyncNow()) }}),
		)
		m.Open = open.Get
		m.OnOpenChange = func(next bool, reason base.Reason) {
			open.Set(next)
			changed("commands")(next, reason)
		}
		return ui.Card{Title: tr.MenusOpenedByTheApp(), Description: tr.MenusAControlledMenuAButton(), Actions: []core.Node{right}, Children: []core.Node{
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
		return ui.Card{Title: tr.MenusThePlatformSMenu(), Description: tr.MenusPopupmenuShowsTheSystemS(), Actions: []core.Node{ui.Badge{Text: tr.MenusNative2(), Tone: ui.ToneInfo}}, Children: []core.Node{
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
					ui.Button{Text: tr.MenusAlwaysShown(), Variant: ui.ButtonSecondary, Small: true, OnPress: func() { desktop.SetMenuBar(cx, desktop.MenuBarShown) }},
					ui.Button{Text: tr.MenusAutoHide(), Variant: ui.ButtonSecondary, Small: true, OnPress: func() { desktop.SetMenuBar(cx, desktop.MenuBarAutoHide) }},
				}})
		}
		return ui.Card{Title: tr.MenusTheApplicationMenu(), Description: tr.MenusTheSystemSMenuBar(), Actions: []core.Node{ui.Badge{Text: tr.MenusNative2(), Tone: ui.ToneInfo}}, Children: children}
	}

	eventsCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Card{Title: tr.MenusEvents(), Description: tr.MenusOnopenchangeWithItsReasonAnd(), Children: []core.Node{
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

	return demoPage(cx, pageProps{}, menuBar, rows, placement, longList, commands, native, appMenu, eventsCard)
}
