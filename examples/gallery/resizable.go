package main

import (
	"fmt"
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

func roundSizes(sizes []core.Points) string {
	parts := make([]string, len(sizes))
	for i, s := range sizes {
		parts[i] = fmt.Sprintf("%.0f", s)
	}
	return strings.Join(parts, " · ")
}

// pane is a panel's title and what it does.
func pane(cx *core.Scope, title, detail string, size func() string) core.Node {
	colors := ui.ColorsOf(cx)
	children := []core.Node{ui.Text(title, 14, ui.Semibold, colors.Text), ui.Text(detail, 12, ui.Regular, colors.Muted)}
	if size != nil {
		children = append(children, core.Text{ContentFunc: size, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)})
	}
	return core.View{Style: core.Style{FlexGrow: 1, Gap: 4, Padding: core.All(16)}, Children: children}
}

// resizableDemo is base's ResizablePanelGroup and ResizablePanel in an
// editor's frame, a sidebar beside an editor over a console, each handle
// dragged within the panels' ranges; the console can be hidden and comes
// back at its size. The drags run on the platform; the sizes are the
// app's, reported when a drag ends.
func resizableDemo(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	// A phone leaves the editor room beside the sidebar.
	start := core.Points(220)
	if core.Untracked(cx, cx.Env().Window).Width < 600 {
		start = 130
	}
	sidebar, consoleSize := core.NewSignal(cx, start), core.NewSignal(cx, core.Points(140))
	shown := core.NewSignal(cx, true)
	last := core.NewSignal(cx, "—")
	return core.View{StyleFunc: func() core.Style {
		return fillPage.Merge(core.Style{Gap: 12, Background: colors.Background(), Padding: core.All(16)})
	}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			core.Text{Content: ui.Hint(cx, "Drag a line between two panels with your finger", "Drag a line between two panels"),
				Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 14}, StyleFunc: ui.Colored(colors.Muted)},
			base.Button{OnPress: func(core.PressEvent) { toggled(shown) }, Selected: shown.Get, Label: "Console",
				StyleFunc: func() core.Style {
					return core.Style{CornerRadius: 6, BorderWidth: 1, BorderColor: colors.Border(), Padding: core.Symmetric(6, 12),
						Active: &core.Style{Opacity: core.Alpha(0.7)}, Selected: &core.Style{Background: colors.Raised()}}
				},
				Children: []core.Node{ui.Text("Console", 13, ui.Medium, colors.Text)}},
		}},
		core.View{StyleFunc: func() core.Style {
			return core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(240), Overflow: core.Hidden, CornerRadius: 8, BorderWidth: 1, BorderColor: colors.Border(),
				Background: colors.Background()}
		}, Children: []core.Node{
			base.ResizablePanelGroup{TestID: "panels", HandleStyleFunc: ui.HandleLine(cx), HandleThumbStyle: ui.HandleThumb(), HandleThumbStyleFunc: ui.HandleThumbColor(cx), HandleLabel: "Resize the sidebar",
				Style:    core.Style{FlexGrow: 1},
				OnResize: func(s []core.Points) { last.Set("Row: " + roundSizes(s)) },
				Panels: []base.ResizablePanel{
					{Size: sidebar.Get, MinSize: 120, MaxSize: 360, OnResize: sidebar.Set, StyleFunc: ui.Filled(colors.Raised),
						Children: []core.Node{pane(cx, "Files", "120 to 360 pt", func() string { return fmt.Sprintf("Sidebar %.0f pt", sidebar.Get()) })}},
					{MinSize: 160, Children: []core.Node{base.ResizablePanelGroup{Axis: base.Vertical, HandleStyleFunc: ui.HandleLine(cx), HandleThumbStyle: ui.HandleThumb(), HandleThumbStyleFunc: ui.HandleThumbColor(cx),
						HandleLabel: "Resize the console", Style: core.Style{FlexGrow: 1},
						OnResize: func(s []core.Points) { last.Set("Column: " + roundSizes(s)) },
						Panels: []base.ResizablePanel{
							{MinSize: 80, Children: []core.Node{pane(cx, "Editor", "Takes the room the others leave", nil)}},
							{Size: consoleSize.Get, MinSize: 60, Visible: shown.Get, OnResize: consoleSize.Set, StyleFunc: ui.Filled(colors.Raised),
								Children: []core.Node{pane(cx, "Console", "Hidden, it keeps its size", func() string { return fmt.Sprintf("Console %.0f pt", consoleSize.Get()) })}},
						}}}},
				}},
		}},
		core.Text{ContentFunc: func() string { return "Last resize: " + last.Get() }, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
	}}
}
