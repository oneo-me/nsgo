// Command clip is the App Clip of the café's app: a table's code opens
// https://cafe.example.com/table?n=12, and the clip orders to that table.
package main

import (
	"net/url"
	"strconv"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/mobile"
	"github.com/nsgo-dev/nsgo/ui"
)

func main() { nsgo.Run(Root, nsgo.WindowSize(390, 640)) }

// table is the table the link names, "" when none does.
func table(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	if n, err := strconv.Atoi(u.Query().Get("n")); err == nil && n > 0 {
		return strconv.Itoa(n)
	}
	return ""
}

func Root(cx *core.Scope) core.Node {
	cups := core.NewSignal(cx, 0)
	page := func() core.Style {
		colors := cx.Theme().Light
		if cx.Dark() {
			colors = cx.Theme().Dark
		}
		return core.Style{Width: core.Pct(100), Height: core.Pct(100), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 12, Padding: core.All(24),
			Background: colors.Background, Color: colors.Text}
	}
	where := func() string {
		if n := table(mobile.OpenedURL(cx)); n != "" {
			return "Table " + n
		}
		return "Scan a table's code"
	}
	return core.View{StyleFunc: page, Children: []core.Node{
		core.Text{ContentFunc: where, Style: core.Style{FontSize: 34, FontWeight: core.FontWeightBold}, TestID: "table"},
		core.Text{ContentFunc: func() string { return mobile.OpenedURL(cx) }, Style: core.Style{FontSize: 12, Opacity: core.Alpha(0.6)}, TestID: "link"},
		core.Text{ContentFunc: func() string { return strconv.Itoa(cups.Get()) + " × Flat white" }, Style: core.Style{FontVariant: core.FontTabularNums}, TestID: "cups"},
		ui.Button{Text: "Add a flat white", Disabled: func() bool { return table(mobile.OpenedURL(cx)) == "" },
			OnPress: func() { cups.Update(func(n int) int { return n + 1 }) }, TestID: "add"},
	}}
}
