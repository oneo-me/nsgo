package ui

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
)

// The window shells fill the window with the app's content under a
// header of their own, which drags the window: the sidebar reaches the
// window's top beside the window controls, keeping clear of them, and
// leaves a window material showing through. A shell hides the system's
// title bar for HeaderHeight itself, so every app's window controls
// center in the same header; the app picks the material
// (nsgo.WindowMaterial).

// HeaderHeight is the height of the shell's headers, which an app gives
// desktop.SetTitleBar for a title bar of its own to match them.
const HeaderHeight core.Points = 48

// ShellHeaderHeight is a shell's header's height: HeaderHeight, or the
// window controls' height where it is greater.
func ShellHeaderHeight(cx *core.Scope) core.Points {
	return max(HeaderHeight, desktop.TitleBar(cx).Top)
}

// hideTitleBar gives a desktop window's title bar over to the shell's
// header.
func hideTitleBar(cx *core.Scope) {
	if desktop.IsAvailable(cx) {
		desktop.SetTitleBar(cx, desktop.TitleBarHidden, HeaderHeight)
	}
}

// Chrome is the background of the window's chrome (a title bar, the
// sidebar): none while a window material shows through it, read in a
// StyleFunc.
func Chrome(cx *core.Scope) core.Color {
	if desktop.ShowsMaterial(cx) {
		return 0
	}
	return ColorsOf(cx).Inset()
}

// controlsEnd is where the window controls and the safe area end at the
// window's start.
func controlsEnd(cx *core.Scope) core.Points { return desktop.TitleBar(cx).Left + core.Insets(cx).Left }

// shellHeader is a header row, whose background drags the window. It
// keeps clear of the safe area, at its start of lead points more, and at
// its end of the window controls while trail says so.
func shellHeader(cx *core.Scope, testID string, lead func() core.Points, trail bool, fill func() core.Color, children []core.Node) core.Node {
	return core.View{TestID: testID, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, Children: children, StyleFunc: func() core.Style {
		safe := core.Insets(cx)
		p := core.Edges{Top: safe.Top, Left: 12 + lead(), Right: 12}
		if trail {
			p.Right += desktop.TitleBar(cx).Right + safe.Right
		}
		return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexShrink: 0,
			Height: core.Pt(ShellHeaderHeight(cx) + safe.Top), Padding: p, Background: fill()}
	}}
}
