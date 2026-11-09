// Command widgets is a counter whose widgets show its count: on the Mac's
// desktop and in Notification Center, and on the Home Screen and the Lock
// Screen of an iPhone or an iPad. Each change writes the widgets' timeline
// with package widget, and the medium widget counts down to the next hour,
// which the system shows without the app.
package main

import (
	"strconv"
	"time"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
	"github.com/oneo-me/nsgo/widget"
)

func main() { nsgo.Run(Root, nsgo.WindowSize(420, 360)) }

// countKey keeps the count in the app's storage.
const countKey = "count"

func Root(cx *core.Scope) core.Node {
	storage := cx.Storage()
	saved, _ := storage.GetItem(countKey)
	n, _ := strconv.Atoi(saved)
	count := core.NewSignal(cx, n)
	status := core.NewSignal(cx, "")
	cx.Effect(func() {
		n := count.Get()
		storage.SetItem(countKey, strconv.Itoa(n))
		msg := "Add the Counter widget from the system's widgets."
		if err := widget.Update("counter", entry(n, time.Now())); err != nil {
			msg = err.Error()
		}
		status.Set(msg)
	})
	add := func(d int) func() { return func() { count.Update(func(n int) int { return n + d }) } }
	// The page takes the theme's colors, which follow the appearance.
	page := func() core.Style {
		colors := cx.Theme().Light
		if cx.Dark() {
			colors = cx.Theme().Dark
		}
		return core.Style{Width: core.Pct(100), Height: core.Pct(100), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 16, Padding: core.All(24),
			Background: colors.Background, Color: colors.Text}
	}
	return core.View{StyleFunc: page, Children: []core.Node{
		core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }, Style: core.Style{FontSize: 56, FontWeight: core.FontWeightBold, FontVariant: core.FontTabularNums}, TestID: "count"},
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Button{Text: "−", Variant: ui.ButtonSecondary, OnPress: add(-1), TestID: "decrement"},
			ui.Button{Text: "Reset", Variant: ui.ButtonSecondary, Disabled: func() bool { return count.Get() == 0 }, OnPress: func() { count.Set(0) }, TestID: "reset"},
			ui.Button{Text: "+", OnPress: add(1), TestID: "increment"},
		}},
		core.Text{ContentFunc: status.Get, Style: core.Style{FontSize: 12, Opacity: core.Alpha(0.6), TextAlign: core.TextAlignCenter}, MaxLines: 3, TestID: "status"},
	}}
}

// entry is what the widgets show of the count, in each family.
func entry(n int, now time.Time) widget.Entry {
	count := strconv.Itoa(n)
	label := widget.Text{Content: "Count", Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}}
	number := widget.Text{Content: count, Style: widget.Style{FontSize: 44, FontWeight: core.FontWeightBold, FontVariant: core.FontTabularNums, Accented: true}}
	small := widget.Column{Style: widget.Style{AlignItems: widget.AlignStart, FillWidth: true, FillHeight: true}, Children: []widget.View{
		label, widget.Spacer{}, number,
	}}
	hour := now.Truncate(time.Hour).Add(time.Hour)
	return widget.Entry{
		View: small,
		Families: map[widget.Family]widget.View{
			widget.Medium: widget.Row{Style: widget.Style{Gap: 16, FillWidth: true, FillHeight: true}, Children: []widget.View{
				small,
				widget.Column{Style: widget.Style{AlignItems: widget.AlignEnd, Gap: 4}, Children: []widget.View{
					widget.Text{Content: "Next hour in", Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}},
					widget.Date{Time: hour, Format: widget.Timer, Style: widget.Style{TextStyle: widget.Title2, FontVariant: core.FontTabularNums, TextAlign: core.TextAlignEnd}},
					widget.Progress{Value: float64(now.Minute()) / 60, Style: widget.Style{Width: 120, Color: widget.Accent}},
				}},
			}},
			widget.Circular: widget.Stack{Children: []widget.View{
				widget.Gauge{Value: float64(n%10) / 10, Style: widget.Style{FillWidth: true, FillHeight: true}},
				widget.Text{Content: count, Style: widget.Style{FontSize: 18, FontWeight: core.FontWeightBold}},
			}},
			widget.Rectangular: widget.Column{Style: widget.Style{AlignItems: widget.AlignStart}, Children: []widget.View{
				widget.Text{Content: "Count", Style: widget.Style{TextStyle: widget.Headline}},
				widget.Text{Content: count, Style: widget.Style{TextStyle: widget.Title, FontVariant: core.FontTabularNums}},
			}},
			widget.Inline: widget.Row{Style: widget.Style{Gap: 4}, Children: []widget.View{
				widget.Symbol{Name: "number"}, widget.Text{Content: "Count " + count},
			}},
		},
	}
}
