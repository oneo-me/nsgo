// Command appclip is a café's app with an App Clip: the app, here, keeps
// the user's orders, and the clip, in package clip, orders at a table
// from the link its code opens, without installing the app.
package main

import (
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

func main() { nsgo.Run(Root, nsgo.WindowSize(390, 640)) }

func Root(cx *core.Scope) core.Node {
	page := func() core.Style {
		colors := cx.Theme().Light
		if cx.Dark() {
			colors = cx.Theme().Dark
		}
		return core.Style{Width: core.Pct(100), Height: core.Pct(100), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 12, Padding: core.All(24),
			Background: colors.Background, Color: colors.Text}
	}
	return core.View{StyleFunc: page, Children: []core.Node{
		core.Text{Content: "Cafe", Style: core.Style{FontSize: 34, FontWeight: core.FontWeightBold}, TestID: "title"},
		core.Text{Content: "Scan a table's code to order there, with or without the app.", Style: core.Style{Opacity: core.Alpha(0.6), TextAlign: core.TextAlignCenter}},
		ui.Button{Text: "Order to go", TestID: "order"},
	}}
}
