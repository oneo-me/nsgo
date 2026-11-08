package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// pane is a panel's title and what it does.
func pane(cx *core.Scope, title, detail string, size func() string) core.Node {
	colors := ui.ColorsOf(cx)
	children := []core.Node{ui.Text(title, 13, ui.Semibold, colors.Text), ui.Text(detail, 12, ui.Regular, colors.Muted)}
	if size != nil {
		children = append(children, core.Text{ContentFunc: size, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)})
	}
	return core.View{Style: core.Style{FlexGrow: 1, Gap: 4, Padding: core.All(16)}, Children: children}
}

// resizableDemo is base's ResizablePanelGroup and ResizablePanel in an
// editor's frame, a sidebar beside an editor over a console, each handle
// dragged within the panels' ranges; the console can be hidden and comes
// back at its size. The drags run on the platform; the sizes are the
// app's.
func resizableDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	// A phone leaves the editor room beside the sidebar.
	start := core.Points(220)
	if core.Untracked(cx, cx.Env().Window).Width < 600 {
		start = 130
	}
	sidebarValue, consoleValue := core.NewSignal(cx, float64(start)), core.NewSignal(cx, 140.0)
	sidebar := func() core.Points { return core.Points(sidebarValue.Get()) }
	consoleSize := func() core.Points { return core.Points(consoleValue.Get()) }
	setSidebar := func(p core.Points) { sidebarValue.Set(float64(p)) }
	setConsole := func(p core.Points) { consoleValue.Set(float64(p)) }
	shown := core.NewSignal(cx, true)
	return stagePage(cx,
		core.View{StyleFunc: func() core.Style {
			return core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), Overflow: core.Hidden, CornerRadius: 8, BorderWidth: 1, BorderColor: colors.Border(),
				Background: colors.Background()}
		}, Children: []core.Node{
			base.ResizablePanelGroup{TestID: "panels", HandleStyleFunc: ui.HandleLine(cx), HandleThumbStyle: ui.HandleThumb(cx), HandleThumbStyleFunc: ui.HandleThumbColor(cx), HandleLabel: tr.ResizableResizeTheSidebar(),
				Style: core.Style{FlexGrow: 1},
				Panels: []base.ResizablePanel{
					{Size: sidebar, MinSize: 120, MaxSize: 360, OnResize: setSidebar, StyleFunc: ui.Filled(colors.Raised),
						Children: []core.Node{pane(cx, tr.ResizableFiles(), tr.Resizable120To360Pt(), func() string { return tr.ResizableSidebarPt(fmt.Sprintf("%.0f", sidebar())) })}},
					{MinSize: 160, Children: []core.Node{base.ResizablePanelGroup{Axis: base.Vertical, HandleStyleFunc: ui.HandleLine(cx), HandleThumbStyle: ui.HandleThumb(cx), HandleThumbStyleFunc: ui.HandleThumbColor(cx),
						HandleLabel: tr.ResizableResizeTheConsole(), Style: core.Style{FlexGrow: 1},
						Panels: []base.ResizablePanel{
							{MinSize: 80, Children: []core.Node{pane(cx, tr.ResizableEditor(), tr.ResizableTakesTheRoomTheOthers(), nil)}},
							{Size: consoleSize, MinSize: 60, Visible: shown.Get, OnResize: setConsole, StyleFunc: ui.Filled(colors.Raised),
								Children: []core.Node{pane(cx, tr.ResizableConsole(), tr.ResizableHiddenItKeepsItsSize(), func() string { return tr.ResizableConsolePt(fmt.Sprintf("%.0f", consoleSize())) })}},
						}}}},
				}},
		}},
		Section{Title: tr.TerminalOptions(), Note: "base.ResizablePanel", Children: []core.Node{
			SwitchRow{Label: tr.ResizableConsole(), Code: "Visible", Checked: shown.Get, OnChange: shown.Set},
			numberKnob(tr.ResizableFiles(), sidebarValue, 120, 360, 10),
			numberKnob(tr.ResizableConsole(), consoleValue, 60, 300, 10),
			Note{Text: Hint(cx, tr.ResizableDragALineBetweenTwo(), tr.ResizableDragALineBetweenTwo2())},
		}})
}
