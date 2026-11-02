package main

import (
	"fmt"
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

// base.DockArea owns the workspace's behavior: tab drags into another
// group, to a group's edge (a new split) or along a tab bar, all run on
// the platform; the splits' and docks' handles; selection, closing and
// what may move. The layout is the app's data: the area reports every edit
// as the whole new layout and the app passes it back. Each panel's content
// is the app's too, rendered once per key, so it keeps its state wherever
// the panel moves.

// dockTitles are the panels' titles, by key.
var dockTitles = map[string]string{"files": "Files", "readme": "README.md", "main": "main.go", "preview": "Preview", "terminal": "Terminal"}

func dockTitle(panel string) string {
	if t, ok := dockTitles[panel]; ok {
		return t
	}
	return panel
}

// dockNames are the docks' names, by placement.
var dockNames = map[base.DockPlacement]string{base.DockLeft: "Files", base.DockBottom: "Terminal"}

// initialDock is the workspace at first: two editor groups side by side,
// the files on the left and a terminal under them.
func initialDock() base.DockLayout {
	editors := base.DockTabs("readme", "main")
	editors.Active = 1
	center := base.DockSplit(base.Horizontal, editors, base.DockTabs("preview"))
	center.Sizes = []core.Points{300, 0}
	return base.DockLayout{
		Center: center,
		Left:   &base.DockRegion{Root: base.DockTabs("files"), Size: 200},
		Bottom: &base.DockRegion{Root: base.DockTabs("terminal"), Size: 160},
	}
}

// describeDock is a layout as a line of text: row(...) and column(...)
// for splits, [a, b] for tab groups.
func describeNode(n base.DockNode) string {
	if n.Tabs {
		titles := make([]string, len(n.Panels))
		for i, p := range n.Panels {
			titles[i] = dockTitle(p)
		}
		return "[" + strings.Join(titles, ", ") + "]"
	}
	parts := make([]string, len(n.Children))
	for i, c := range n.Children {
		parts[i] = describeNode(c)
	}
	axis := "row"
	if n.Axis == base.Vertical {
		axis = "column"
	}
	return axis + "(" + strings.Join(parts, " ") + ")"
}

func describeRegion(name string, r *base.DockRegion) string {
	if r == nil {
		return name + ": none"
	}
	state := "open"
	if r.Closed {
		state = "closed"
	}
	return fmt.Sprintf("%s: %s %.0f pt, %s", name, describeNode(r.Root), r.Size, state)
}

// firstSlot is the size of the center's first slot, when it is a split.
func firstSlot(n base.DockNode) string {
	switch {
	case n.Tabs:
		return "—"
	case len(n.Sizes) == 0 || n.Sizes[0] == 0:
		return "auto"
	}
	return fmt.Sprintf("%.0f pt", n.Sizes[0])
}

// dockSources are the lines of an editor's file.
var dockSources = map[string][]string{
	"main":   {"package main", "", "func main() {", "    hopper.Run(hopper.App{})", "}"},
	"readme": {"# Hopper", "", "A dockable workspace: drag a tab", "to another group or to an edge."},
}

// editorPanel is an editor: a count of its edits, which survives every
// move, over its file's lines.
func editorPanel(cx *core.Scope, panel string) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	edits := core.NewSignal(cx, 0)
	var lines []core.Node
	for i, l := range dockSources[panel] {
		lines = append(lines, core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{
			core.Text{Content: itoa(i + 1), Style: core.Style{Width: core.Pt(16), TextAlign: core.TextAlignEnd, FontFamily: ui.Mono, FontSize: 12}, StyleFunc: ui.Colored(colors.Faint)},
			core.Text{Content: l, Style: core.Style{FlexGrow: 1, FontFamily: ui.Mono, FontSize: 12}, StyleFunc: ui.Colored(colors.Code)},
		}})
	}
	return core.View{Style: core.Style{FlexGrow: 1, Gap: 16, Padding: core.All(16)}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
			base.Button{OnPress: func(core.PressEvent) { edits.Update(func(n int) int { return n + 1 }) }, Label: "Edit " + dockTitle(panel),
				StyleFunc: func() core.Style {
					return fin.Accent().Over(core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, CornerRadius: 8, Padding: core.Symmetric(0, 12),
						Background: colors.AccentFill(), Transition: core.Transition{Duration: 150e6}, Active: &core.Style{Transform: core.Transform{Scale: 0.97}}})
				},
				Children: []core.Node{ui.Icon(ui.IconPlus, 14, colors.OnAccent), ui.Text("Edit", 13, ui.Semibold, colors.OnAccent)}},
			ui.Textf(func() string { return fmt.Sprintf("%s: %s", dockTitle(panel), count(edits.Get(), "edit")) }, 14, ui.Regular, colors.Muted),
		}},
		core.View{Style: core.Style{Gap: 4}, Children: lines},
	}}
}

var dockFiles = []struct {
	name   string
	depth  int
	folder bool
}{{"cmd/", 0, true}, {"cmd/app.go", 1, false}, {"cmd/lib.go", 1, false}, {"go.mod", 0, false}}

// dockPanel is what a panel shows, by key.
func dockPanel(cx *core.Scope, panel string) core.Node {
	colors := ui.ColorsOf(cx)
	switch panel {
	case "readme", "main":
		return editorPanel(cx, panel)
	case "files":
		var rows []core.Node
		for _, f := range dockFiles {
			dot := colors.Faint
			if f.folder {
				dot = colors.Accent
			}
			rows = append(rows, core.View{StyleFunc: func() core.Style {
				return core.Style{Height: core.Pt(28), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, CornerRadius: 6,
					Padding: core.Edges{Left: 8 + core.Points(f.depth)*14, Right: 8}, Hover: &core.Style{Background: colors.Hover()}}
			}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(6), Height: core.Pt(6), CornerRadius: 3}, StyleFunc: ui.Filled(dot)},
				core.Text{Content: f.name, Style: core.Style{FontFamily: ui.Mono, FontSize: 12}, StyleFunc: ui.Colored(colors.Text)},
			}})
		}
		return core.View{Style: core.Style{FlexGrow: 1, Gap: 2, Padding: core.All(8)}, Children: rows}
	case "preview":
		return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 12, Padding: core.All(16)}, Children: []core.Node{
			ui.IconTile{Icon: ui.IconSpark, Size: 40},
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 2}, Children: []core.Node{
				ui.Text("Live preview", 14, ui.Semibold, colors.Text), ui.Text("Follows the editor beside it", 12, ui.Regular, colors.Muted)}},
		}}
	}
	monoText := func(s string, c func() core.Color) core.Node {
		return core.Text{Content: s, Style: core.Style{FontFamily: ui.Mono, FontSize: 12}, StyleFunc: ui.Colored(c)}
	}
	return core.View{Style: core.Style{FlexGrow: 1, Gap: 4, Padding: core.All(12)}, Children: []core.Node{
		monoText("$ go run .", colors.Text), monoText("   building hopper", colors.Muted), monoText("Finished in 1.92s", colors.Accent),
	}}
}

// dockDemo is base.DockArea: editor tabs in a center split, files in a
// left dock and a terminal in a bottom dock; tabs dragged between groups
// and to their edges, closed, the docks toggled and resized. Narrower
// than 600 pt it shows one group at a time and presents a dock at a time,
// as a drawer or a sheet over the editors.
func dockDemo(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	layout := core.NewSignalFunc(cx, initialDock(), func(a, b base.DockLayout) bool { return false })
	locked, closing := core.NewSignal(cx, false), core.NewSignal(cx, true)
	last, zoomed := core.NewSignal(cx, "—"), core.NewSignal(cx, "")
	wide := core.Untracked(cx, cx.Env().Window).Width >= 600
	showLeft, showBottom := core.NewSignal(cx, wide), core.NewSignal(cx, wide)
	area := &base.DockAreaRef{}
	// current is the panel shown last, whose group the zoom takes.
	current := "main"
	pointer := cx.Env().Desktop()
	pad, gap := core.Points(16), core.Points(12)
	if shell.desktop() {
		pad, gap = 24, 16
	}
	status := func(fn func() string, strong bool) core.Node {
		c := colors.Muted
		if strong {
			c = colors.Text
		}
		return core.Text{ContentFunc: fn, Style: core.Style{FontFamily: ui.Mono, FontSize: 11}, StyleFunc: ui.Colored(c)}
	}
	return core.View{StyleFunc: func() core.Style {
		return fillPage.Merge(core.Style{Background: colors.Background(), Padding: core.All(pad), Gap: gap})
	}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			ui.Tool{Label: "Toggle files", Text: "Files", On: showLeft.Get, OnPress: func() { area.ToggleDock(base.DockLeft) }},
			ui.Tool{Label: "Toggle terminal", Text: "Terminal", On: showBottom.Get, OnPress: func() { area.ToggleDock(base.DockBottom) }},
			ui.Tool{Label: "Lock the layout", Text: "Lock", On: locked.Get, OnPress: func() { toggled(locked) }},
			ui.Tool{Label: "Show close buttons", Text: "Close", On: closing.Get, OnPress: func() { toggled(closing) }},
			ui.Tool{Label: "Zoom the group", Text: "Zoom", On: func() bool { return zoomed.Get() != "" }, OnPress: func() {
				if zoomed.GetUntracked() != "" {
					area.ToggleZoom(0)
				} else {
					g, _ := area.GroupOf(current)
					area.ToggleZoom(g)
				}
			}},
			ui.Tool{Label: "Reset the layout", Text: "Reset", OnPress: func() { layout.Set(initialDock()) }},
			core.Text{Content: ui.Hint(cx, "Hold a tab to reorder it", "Drag a tab onto another group, or to a group's edge to split it"),
				Style: core.Style{MinWidth: core.Pt(160), FlexGrow: 1, FlexShrink: 1, TextAlign: core.TextAlignEnd, FontSize: 13}, StyleFunc: ui.Colored(colors.Faint)},
		}},
		core.View{StyleFunc: func() core.Style {
			return fin.Card().Over(core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(320), BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Surface(),
				CornerRadius: ui.RadiusCard, BorderCurve: core.BorderCurveContinuous, Padding: core.All(5), Gap: 5})
		}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return fin.Inset().Over(core.Style{FlexGrow: 1, FlexShrink: 1, Overflow: core.Hidden, BorderWidth: 1, BorderColor: colors.Line(), Background: colors.Inset(),
					CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous})
			}, Children: []core.Node{core.Dynamic(func(cx *core.Scope) core.Node {
				panels := map[string]base.DockPanelOptions{}
				if !closing.Get() {
					for k := range dockTitles {
						panels[k] = base.DockPanelOptions{NoClose: true}
					}
				}
				return core.Func(func(cx *core.Scope) core.Node {
					return dockArea(cx, area, layout, locked, last, zoomed, showLeft, showBottom, panels, pointer, &current)
				})
			})}},
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, ColumnGap: 20, RowGap: 4, Padding: core.Edges{Top: 2, Right: 10, Bottom: 4, Left: 10}}, Children: []core.Node{
				status(func() string { return "Center: " + describeNode(layout.Get().Center) }, true),
				status(func() string { return "First slot: " + firstSlot(layout.Get().Center) }, false),
				status(func() string { return describeRegion("Left", layout.Get().Left) }, false),
				status(func() string { return describeRegion("Bottom", layout.Get().Bottom) }, false),
				status(func() string {
					var names []string
					if showLeft.Get() {
						names = append(names, "Files")
					}
					if showBottom.Get() {
						names = append(names, "Terminal")
					}
					if len(names) == 0 {
						return "Showing docks: none"
					}
					return "Showing docks: " + strings.Join(names, ", ")
				}, false),
				status(func() string {
					if z := zoomed.Get(); z != "" {
						return "Zoomed: " + dockTitle(z)
					}
					return "Zoomed: none"
				}, false),
				status(func() string { return "Last event: " + last.Get() }, false),
			}},
		}},
	}}
}

func dockArea(cx *core.Scope, area *base.DockAreaRef, layout *core.Signal[base.DockLayout], locked *core.Signal[bool], last, zoomed *core.Signal[string],
	showLeft, showBottom *core.Signal[bool], panels map[string]base.DockPanelOptions, pointer bool, current *string) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	return base.DockArea{Ref: area, TestID: "dock", Layout: layout.Get, OnLayoutChange: layout.Set, Locked: locked.Get, Panels: panels,
		OnPanelRemove: func(p string) { last.Set("Closed " + dockTitle(p)) },
		OnPanelActiveChange: func(p string, active bool) {
			if active {
				*current = p
				last.Set("Showing " + dockTitle(p))
			}
		},
		OnZoomChange: zoomed.Set,
		OnDockOpenChange: func(p base.DockPlacement, open bool) {
			switch p {
			case base.DockLeft:
				showLeft.Set(open)
			case base.DockBottom:
				showBottom.Set(open)
			}
		},
		Style:          core.Style{FlexGrow: 1},
		GroupStyleFunc: func() core.Style { return core.Style{FlexGrow: 1, Background: colors.Inset()} },
		TabBarStyleFunc: func() core.Style {
			return core.Style{Direction: core.Row, Gap: 4, Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Line()}}
		},
		TabStyle:     core.Style{Group: "tab"},
		ContentStyle: core.Style{FlexGrow: 1},
		HandleLabel:  "Resize",
		// A hairline that stays as it is; the thumb, a pill on it, grows
		// in under the pointer and further while dragged.
		HandleStyleFunc:  func() core.Style { return core.Style{Background: colors.Line()} },
		HandleThumbStyle: ui.HandleThumb(), HandleThumbStyleFunc: ui.HandleThumbColor(cx),
		DockStyleFunc: func() core.Style { return core.Style{Background: colors.Inset()} },
		DropIndicatorStyleFunc: func() core.Style {
			return core.Style{CornerRadius: 8, BorderWidth: 2, BorderColor: colors.Accent(), Background: tints.Accent()}
		},
		DrawerStyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(260), Borders: core.Edges{Right: 1}, BorderColors: core.EdgeColors{Right: colors.Border()}, Background: colors.Surface()}
		},
		SheetStyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(240), CornerRadii: core.Corners{TopLeft: 16, TopRight: 16}, Borders: core.Edges{Top: 1},
				BorderColors: core.EdgeColors{Top: colors.Border()}, Background: colors.Surface()}
		},
		BackdropStyle: core.Style{Background: ui.Black(0.4)},
		// Sized to its title, as wide on either side of it: the shown tab
		// is lit and underlined in orange. With a pointer, the close
		// button fades in over the tab's end while the pointer is on it;
		// a touch screen keeps it beside the shown tab's title.
		RenderTab: func(cx *core.Scope, tab base.DockTab) core.Node {
			colors := ui.ColorsOf(cx)
			children := []core.Node{core.Text{Content: dockTitle(tab.Key), MaxLines: 1, StyleFunc: func() core.Style {
				s := core.Style{MaxWidth: core.Pt(160), FontSize: 13, FontWeight: ui.Medium, Color: colors.Muted(), Transition: core.Transition{Duration: 150e6}}
				if tab.Selected() {
					s.Color = colors.Text()
				} else {
					s.GroupStyle = &core.GroupStyle{Name: "tab", Hover: &core.Style{Color: colors.Text()}}
				}
				return s
			}}}
			if tab.Closable {
				closeStyle := core.Style{Width: core.Pt(20), Height: core.Pt(20), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 6,
					Margin: core.Edges{Right: -6}, Active: &core.Style{Opacity: core.Alpha(0.7)}}
				if pointer {
					closeStyle = core.Style{Position: core.Absolute, Right: core.Pt(4), Top: core.Pt(8), Width: core.Pt(20), Height: core.Pt(20),
						AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 6,
						Opacity: core.Alpha(0.001), Transition: core.Transition{Duration: 150e6},
						GroupStyle: &core.GroupStyle{Name: "tab", Hover: &core.Style{Opacity: core.Alpha(1)}}}
				}
				closeColors := func() core.Style {
					if !pointer {
						return core.Style{}
					}
					return core.Style{Background: colors.Inset(), Hover: &core.Style{Background: colors.Raised()}}
				}
				children = append(children, core.Show{When: func() bool { return pointer || tab.Selected() }, Then: func(*core.Scope) core.Node {
					return base.Button{OnPress: func(core.PressEvent) { area.ClosePanel(tab.Key) }, Label: "Close " + dockTitle(tab.Key), Style: closeStyle, StyleFunc: closeColors,
						Children: []core.Node{ui.Text("×", 13, ui.Regular, colors.Muted)}}
				}})
			}
			children = append(children, core.Show{When: tab.Selected, Then: func(*core.Scope) core.Node {
				return core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Bottom: core.Pt(0), Left: core.Pt(12), Right: core.Pt(12),
					Height: core.Pt(2), CornerRadius: 1}, StyleFunc: ui.Filled(colors.Accent)}
			}})
			return core.View{TestID: "tab-" + tab.Key, Style: core.Style{Height: core.Pt(36), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.Symmetric(0, 12)},
				Children: children}
		},
		RenderDock: func(cx *core.Scope, d base.DockInfo) core.Node {
			colors := ui.ColorsOf(cx)
			name := dockNames[d.Placement]
			return core.View{StyleFunc: func() core.Style {
				return core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Borders: core.Edges{Bottom: 1},
					BorderColors: core.EdgeColors{Bottom: colors.Line()}, Padding: core.Edges{Left: 12, Right: 4}}
			}, Children: []core.Node{
				core.Text{Content: name, Style: core.Style{FlexGrow: 1, FontSize: 11, FontWeight: ui.Semibold, TextTransform: core.TextTransformUppercase, LetterSpacing: 0.8}, StyleFunc: ui.Colored(colors.Faint)},
				// Toggled: at compact width that dismisses the drawer, in
				// the full presentation it closes the open dock.
				base.Button{OnPress: func(core.PressEvent) { area.ToggleDock(d.Placement) }, Label: "Hide " + name,
					StyleFunc: func() core.Style {
						return core.Style{CornerRadius: 6, Padding: core.Symmetric(2, 8), Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Opacity: core.Alpha(0.7)}}
					},
					Children: []core.Node{ui.Text("Hide", 12, ui.Medium, colors.Muted)}},
			}}
		},
		RenderDragPreview: func(cx *core.Scope, tab base.DockTab) core.Node {
			colors := ui.ColorsOf(cx)
			fin := ui.FinOf(cx)
			return core.View{StyleFunc: func() core.Style {
				return fin.Raised().Over(core.Style{CornerRadius: 8, BorderWidth: 1, BorderColor: colors.Border(), Padding: core.Symmetric(6, 12), Background: colors.Raised()})
			},
				Children: []core.Node{ui.Text(dockTitle(tab.Key), 13, ui.Medium, colors.Text)}}
		},
		RenderPanel: dockPanel,
		RenderEmpty: func(cx *core.Scope) core.Node {
			colors := ui.ColorsOf(cx)
			return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 4}, Children: []core.Node{
				ui.Text("Every editor is closed", 14, ui.Semibold, colors.Text), ui.Text("Reset brings them back", 12, ui.Regular, colors.Muted)}}
		},
	}
}
