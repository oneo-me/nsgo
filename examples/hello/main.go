// Command hello is the smallest app worth packaging: it greets the name
// typed into it and counts its greetings, in English and Chinese, and its
// App Clip (clip) greets the name a link gives. Build it for every target,
// mini programs included, to try the packages and installers (go tool
// nsgo build examples/hello).
package main

import (
	"strings"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/examples/hello/locales"
	"github.com/oneo-me/nsgo/ui"
)

func main() { nsgo.Run(Root, nsgo.WindowSize(420, 320)) }

func Root(cx *core.Scope) core.Node {
	name := core.NewSignal(cx, "")
	count := core.NewSignal(cx, 0)
	colors := ui.ColorsOf(cx)
	greeting := func() string {
		who := strings.TrimSpace(name.Get())
		if who == "" {
			who = locales.World(cx)
		}
		return locales.Greeting(cx, who)
	}
	return core.View{
		StyleFunc: func() core.Style {
			return core.Style{Width: core.Pct(100), Height: core.Pct(100), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Gap: 16, Padding: core.All(24), Background: cx.Palette().Background, Color: colors.Text()}
		},
		Children: []core.Node{
			core.Text{ContentFunc: greeting, Style: ui.TypeHeading.Merge(core.Style{FontSize: 28, LineHeight: 34, TextAlign: core.TextAlignCenter}), TestID: "greeting"},
			core.View{Style: core.Style{Width: core.Pt(240)}, Children: []core.Node{
				ui.Input{Value: name.Get, OnChange: name.Set, Placeholder: locales.NamePlaceholder(cx), TestID: "name"},
			}},
			ui.Button{Text: locales.SayHello(cx), OnPress: func() { count.Update(func(n int) int { return n + 1 }) }, TestID: "say"},
			core.Text{ContentFunc: func() string { return locales.Said(cx, count.Get()) },
				StyleFunc: func() core.Style { return ui.TypeCaption.Merge(core.Style{Color: colors.Muted()}) }, TestID: "said"},
		},
	}
}
