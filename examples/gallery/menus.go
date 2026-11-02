package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
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
	children := []core.Node{core.Text{ContentFunc: p.label, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 14}, StyleFunc: ui.Colored(fg)}}
	if p.note != "" {
		children = append(children, ui.API{Text: p.note})
	}
	if p.mark != nil {
		children = append(children, core.View{Style: core.Style{Width: core.Pt(16), AlignItems: core.AlignCenter}, Children: []core.Node{
			core.Dynamic(func(*core.Scope) core.Node {
				switch p.mark() {
				case markCheck:
					return ui.Icon(ui.IconCheck, 14, colors.Accent)
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
			core.Text{Content: name, Style: core.Style{FlexGrow: 1, FontSize: 14}, StyleFunc: ui.Colored(colors.Text)},
			ui.Icon(ui.IconChevron, 14, colors.Faint),
		}},
		Back:     core.View{Style: back, StyleFunc: func() core.Style { return core.Style{Active: itemPressed(cx)} }, Children: []core.Node{ui.Icon(ui.IconBack, 14, colors.Accent), ui.Text(name, 14, ui.Semibold, colors.Accent)}},
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
func chip(cx *core.Scope, label func() string, name ui.IconName) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	children := []core.Node{}
	if name != "" {
		children = append(children, ui.Icon(name, 14, colors.Muted))
	}
	children = append(children, ui.Textf(label, 14, ui.Medium, colors.Text))
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
	l := newMenuLook(cx)
	events := core.NewSignalFunc(cx, []string(nil), slices.Equal)
	log := func(what string) {
		events.Update(func(es []string) []string { return append([]string{what}, es[:min(len(es), 5)]...) })
	}
	changed := func(menu string) func(bool, base.Reason) {
		return func(open bool, reason base.Reason) {
			state := "closed"
			if open {
				state = "opened"
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
		history, undone := core.NewSignalFunc(cx, []string(nil), slices.Equal), core.NewSignalFunc(cx, []string(nil), slices.Equal)
		sidebar, statusBar := core.NewSignal(cx, true), core.NewSignal(cx, false)
		zoom := core.NewSignal(cx, "Medium")
		recent := core.NewSignalFunc(cx, []string{"Budget.numbers", "Roadmap.md", "Trip photos"}, slices.Equal)
		act := func(what string) {
			history.Update(func(h []string) []string { return append(slices.Clone(h), what) })
			undone.Set(nil)
			log("chose " + what)
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
		cx.OnKey("mod+alt+n", func() { act("New document") })
		cx.OnKey("mod+alt+s", func() { act("Save") })
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
		for _, size := range []string{"Small", "Medium", "Large"} {
			sizes = append(sizes, l.item(cx, itemProps{label: label(size), keepOpen: true, onSelect: func() { zoom.Set(size) }, mark: func() mark {
				if zoom.Get() == size {
					return markDot
				}
				return markNone
			}}))
		}
		return ui.Card{Icon: ui.IconLists, Title: "Menu bar", Description: ui.Hint(cx, "Each menu a sheet; submenus take its place", "Hover across submenus; the shortcuts work too"), Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				menu("File", chip(cx, label("File"), ""),
					l.item(cx, itemProps{label: label("New document"), note: key("mod+alt+n"), onSelect: func() { act("New document") }}),
					l.item(cx, itemProps{label: label("New window"), onSelect: func() { act("New window") }}),
					l.submenu(cx, "Open recent", false,
						core.Each(recent.Get, func(s string) string { return s }, func(cx *core.Scope, name func() string, _ func() int) core.Node {
							return l.item(cx, itemProps{label: name, onSelect: func() { act("Open " + name()) }})
						}),
						separator(cx),
						l.item(cx, itemProps{label: label("Clear recent"), disabled: empty(recent), onSelect: func() { recent.Set(nil) }}),
					),
					separator(cx),
					l.item(cx, itemProps{label: label("Save"), note: key("mod+alt+s"), onSelect: func() { act("Save") }}),
					l.submenu(cx, "Export as", false,
						l.item(cx, itemProps{label: label("PDF"), onSelect: func() { act("Export PDF") }}),
						l.submenu(cx, "Image", false,
							l.item(cx, itemProps{label: label("PNG"), onSelect: func() { act("Export PNG") }}),
							l.item(cx, itemProps{label: label("JPEG"), onSelect: func() { act("Export JPEG") }}),
							l.item(cx, itemProps{label: label("WebP"), onSelect: func() { act("Export WebP") }}),
						),
						l.submenu(cx, "Web page", true, l.item(cx, itemProps{label: label("HTML")})),
					),
					separator(cx),
					l.item(cx, itemProps{label: label("Delete document"), danger: true, onSelect: func() { act("Delete document") }}),
				),
				menu("Edit", chip(cx, label("Edit"), ""),
					l.item(cx, itemProps{label: lastOf(history, "Undo"), note: key("mod+alt+z"), disabled: empty(history), onSelect: undo}),
					l.item(cx, itemProps{label: lastOf(undone, "Redo"), note: key("mod+alt+shift+z"), disabled: empty(undone), onSelect: redo}),
					separator(cx),
					groupTitle(cx, "Clipboard"),
					l.item(cx, itemProps{label: label("Cut"), onSelect: func() { act("Cut") }}),
					l.item(cx, itemProps{label: label("Copy"), onSelect: func() { act("Copy") }}),
					l.item(cx, itemProps{label: label("Paste"), onSelect: func() { act("Paste") }}),
				),
				menu("View", chip(cx, label("View"), ""), append([]core.Node{
					groupTitle(cx, "Show"),
					l.item(cx, itemProps{label: label("Sidebar"), mark: check(sidebar), keepOpen: true, onSelect: func() { toggled(sidebar) }}),
					l.item(cx, itemProps{label: label("Status bar"), mark: check(statusBar), keepOpen: true, onSelect: func() { toggled(statusBar) }}),
					separator(cx),
					groupTitle(cx, "Zoom"),
				}, sizes...)...),
			}},
			ui.Caption{LabelFunc: func() string {
				return fmt.Sprintf("Sidebar %s · status bar %s · zoom %s", onOff(sidebar.Get()), onOff(statusBar.Get()), zoom.Get())
			}},
			ui.Caption{LabelFunc: func() string {
				if h := history.Get(); len(h) > 0 {
					return "Done: " + strings.Join(h, ", ")
				}
				return "Nothing done yet"
			}},
		}}
	}

	rows := func(cx *core.Scope) core.Node {
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
				l.item(cx, itemProps{label: label("Open"), onSelect: func() { log("open " + name()) }}),
				l.item(cx, itemProps{label: func() string {
					for _, f := range files.Get() {
						if f.id == id && f.starred {
							return "Unstar"
						}
					}
					return "Star"
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
				l.item(cx, itemProps{label: label("Duplicate"), onSelect: func() {
					f := find(id)
					ext := ""
					base := f.name
					if i := strings.LastIndex(f.name, "."); i > 0 {
						base, ext = f.name[:i], f.name[i:]
					}
					c := menuFile{id: next, name: base + " copy" + ext}
					next++
					files.Update(func(fs []menuFile) []menuFile {
						at := slices.IndexFunc(fs, func(x menuFile) bool { return x.id == id })
						return slices.Insert(slices.Clone(fs), at+1, c)
					})
				}}),
				l.submenu(cx, "Move to", false,
					l.item(cx, itemProps{label: label("Documents"), onSelect: func() { log("moved " + name() + " to Documents") }}),
					l.item(cx, itemProps{label: label("Archive"), onSelect: func() { log("moved " + name() + " to Archive") }}),
				),
				separator(cx),
				l.item(cx, itemProps{label: label("Delete"), danger: true, onSelect: func() {
					files.Update(func(fs []menuFile) []menuFile {
						return slices.DeleteFunc(slices.Clone(fs), func(x menuFile) bool { return x.id == id })
					})
				}}),
			}
		}
		return ui.Card{Icon: ui.IconPhotos, Title: "On the rows of a list", Description: ui.Hint(cx, "Hold a finger on a row, or press its ⋯", "Right click a row, or its ⋯ button"), Children: []core.Node{
			core.Show{When: func() bool { return len(files.Get()) > 0 }, Else: func(*core.Scope) core.Node { return ui.Caption{Label: "No files left"} },
				Then: func(*core.Scope) core.Node {
					return core.View{Children: []core.Node{core.Each(files.Get, func(f menuFile) int { return f.id },
						func(cx *core.Scope, file func() menuFile, i func() int) core.Node {
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
											return ui.Icon(ui.IconStar, 16, colors.Accent)
										}
										return ui.Icon(ui.IconText, 16, colors.Muted)
									}),
									core.Text{ContentFunc: func() string { return file().name }, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 14}, StyleFunc: ui.Colored(colors.Text)},
									base.Menu{Align: core.AlignEnd, Offset: 4, Margin: 8, PopupStyleFunc: l.surface(cx), BackdropStyle: l.backdrop(),
										Label: "Actions for " + core.Untracked(cx, file).name,
										Trigger: core.View{Tooltip: core.Text{Content: "More actions"}, HitSlop: core.All(8),
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
		side := core.NewSignal(cx, core.PlaceBottom)
		align := core.NewSignal(cx, core.AlignStart)
		alignName := map[core.Align]string{core.AlignStart: "start", core.AlignCenter: "center", core.AlignEnd: "end"}
		return ui.Card{Icon: ui.IconComponents, Title: "Where it goes", Description: ui.Hint(cx, "Phones show menus as sheets: try a desktop", "The side and alignment; it flips where there is no room"), Children: []core.Node{
			ui.Segmented[core.Placement]{Options: []ui.Option[core.Placement]{{Value: core.PlaceTop, Label: "Top"}, {Value: core.PlaceBottom, Label: "Bottom"}, {Value: core.PlaceLeft, Label: "Left"}, {Value: core.PlaceRight, Label: "Right"}}, Value: side.Get, OnChange: side.Set, Small: true},
			ui.Segmented[core.Align]{Options: []ui.Option[core.Align]{{Value: core.AlignStart, Label: "Start"}, {Value: core.AlignCenter, Label: "Center"}, {Value: core.AlignEnd, Label: "End"}}, Value: align.Get, OnChange: align.Set, Small: true},
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Padding: core.Symmetric(24, 0)}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				p, a := side.Get(), align.Get()
				m := menu("Placed", chip(cx, label(placementName(p)+" · "+alignName[a]), ""),
					l.item(cx, itemProps{label: label("One"), onSelect: func() { log("chose One") }}),
					l.item(cx, itemProps{label: label("Two"), onSelect: func() { log("chose Two") }}),
					l.item(cx, itemProps{label: label("Three"), onSelect: func() { log("chose Three") }}),
				)
				m.Placement, m.Align, m.Offset = p, a, 6
				return m
			})}},
		}}
	}

	longList := func(cx *core.Scope) core.Node {
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
		m := menu("Time zone", chip(cx, zone.Get, ui.IconSearch),
			core.Scroll{Style: core.Style{MaxHeight: core.Pt(maxHeight)}, Children: []core.Node{core.View{Style: core.Style{Padding: core.All(6)}, Children: items}}})
		m.PopupStyle.Padding = core.Edges{}
		if !l.desktop {
			m.PopupStyle.Padding = core.Edges{Bottom: 34}
		}
		return ui.Card{Icon: ui.IconScroll, Title: "A long menu", Description: "Twenty choices in a menu that scrolls", Children: []core.Node{m}}
	}

	commands := func(cx *core.Scope) core.Node {
		open := core.NewSignal(cx, false)
		cx.OnKey("mod+shift+p", func() { toggled(open) })
		var right core.Node
		if l.desktop {
			right = ui.Badge{Text: cx.KeyLabel("mod+shift+p")}
		}
		m := menu("Commands", chip(cx, label("Commands"), ui.IconPlus),
			groupTitle(cx, "Commands"),
			l.item(cx, itemProps{label: label("New note"), onSelect: func() { log("ran New note") }}),
			l.item(cx, itemProps{label: label("Toggle theme"), onSelect: func() { log("ran Toggle theme") }}),
			l.item(cx, itemProps{label: label("Sync now"), onSelect: func() { log("ran Sync now") }}),
		)
		m.Open = open.Get
		m.OnOpenChange = func(next bool, reason base.Reason) {
			open.Set(next)
			changed("commands")(next, reason)
		}
		return ui.Card{Icon: ui.IconSpark, Title: "Opened by the app", Description: "A controlled menu: a button or a shortcut opens it", Actions: []core.Node{right}, Children: []core.Node{
			m, ui.Button{Text: "Open the commands", Variant: ui.ButtonSecondary, OnPress: func() { open.Set(true) }},
		}}
	}

	native := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		chosen := core.NewSignal(cx, "Nothing chosen yet")
		items := func() []desktop.MenuItem {
			choose := func(s string) func() { return func() { chosen.Set("Chose " + s); log("native " + s) } }
			return []desktop.MenuItem{
				{Label: "Cut", Shortcut: "mod+x", OnSelect: choose("Cut")},
				{Label: "Copy", Shortcut: "mod+c", OnSelect: choose("Copy")},
				{Label: "Paste", Disabled: true},
				{Separator: true},
				{Label: "Share", Items: []desktop.MenuItem{{Label: "Mail", OnSelect: choose("Mail")}, {Label: "Messages", OnSelect: choose("Messages")}}},
				{Label: "Pinned", Checkable: true, Checked: true, OnSelect: choose("Pinned")},
			}
		}
		show := func(at core.Point) { desktop.PopupMenu(cx, items(), at, nil) }
		return ui.Card{Icon: ui.IconMenu, Title: "The platform's menu", Description: "PopupMenu shows the system's own at a point", Actions: []core.Node{ui.Badge{Text: "Native", Tone: ui.ToneInfo}}, Children: []core.Node{
			core.View{TestID: "popup-area", Role: core.RoleButton, Label: "Show the native menu",
				StyleFunc: func() core.Style {
					return core.Style{Height: core.Pt(72), CornerRadius: ui.RadiusInset, Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border(),
						AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Active: &core.Style{Background: colors.Pressed()}}
				},
				OnPress:       func(e core.PressEvent) { show(e.PagePosition) },
				OnContextMenu: func(e core.ContextMenuEvent) { show(e.PagePosition) },
				Children:      []core.Node{ui.Text(ui.Hint(cx, "Press here", "Right click or press here"), 14, ui.Regular, colors.Muted)}},
			ui.Caption{LabelFunc: chosen.Get},
		}}
	}

	appMenu := func(cx *core.Scope) core.Node {
		children := []core.Node{
			ui.Note{Text: "SetApplicationMenu sets the menu bar on macOS, Windows and Linux: this gallery's has the roles the platform carries out (About, Hide, Undo … Select All, Minimize, Zoom) and a View menu that toggles the sidebar and opens each demo."},
			ui.Caption{Label: "Last chosen", Note: shell.chosen.Get},
		}
		if env := cx.Env(); env.Desktop() && env.OS() != core.OSMacOS && env.OS() != core.OSWeb {
			children = append(children, ui.Note{Text: "SetMenuBar hides the bar Windows and Linux draw until Alt is pressed alone."},
				ui.HStack{Gap: 8, Children: []core.Node{
					ui.Button{Text: "Always shown", Variant: ui.ButtonSecondary, Small: true, OnPress: func() { desktop.SetMenuBar(cx, desktop.MenuBarShown) }},
					ui.Button{Text: "Auto-hide", Variant: ui.ButtonSecondary, Small: true, OnPress: func() { desktop.SetMenuBar(cx, desktop.MenuBarAutoHide) }},
				}})
		}
		return ui.Card{Icon: ui.IconMenu, Title: "The application menu", Description: "The system's menu bar, set by the app", Actions: []core.Node{ui.Badge{Text: "Native", Tone: ui.ToneInfo}}, Children: children}
	}

	eventsCard := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		return ui.Card{Icon: ui.IconSystem, Title: "Events", Description: "OnOpenChange with its reason, and what was chosen", Children: []core.Node{
			core.Show{When: func() bool { return len(events.Get()) > 0 }, Else: func(*core.Scope) core.Node { return ui.Caption{Label: "Open a menu"} },
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
