// Command clip is the hello example's App Clip: a link such as
// https://hello.example.com/?name=Ada opens it, and it greets the name the
// link gives, in the app's languages.
package main

import (
	"net/url"
	"strings"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/examples/hello/locales"
	"github.com/oneo-me/nsgo/mobile"
	"github.com/oneo-me/nsgo/ui"
)

func main() { nsgo.Run(Root, nsgo.WindowSize(390, 640)) }

// nameIn is the name a link gives, "" when it gives none.
func nameIn(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Query().Get("name"))
}

func Root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	colors := ui.ColorsOf(cx)
	greeting := func() string {
		who := nameIn(mobile.OpenedURL(cx))
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
			ui.Button{Text: locales.SayHello(cx), OnPress: func() { count.Update(func(n int) int { return n + 1 }) }, TestID: "say"},
			core.Text{ContentFunc: func() string { return locales.Said(cx, count.Get()) },
				StyleFunc: func() core.Style { return ui.TypeCaption.Merge(core.Style{Color: colors.Muted()}) }, TestID: "said"},
		},
	}
}
