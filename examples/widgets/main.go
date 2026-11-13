// Command widgets is a counter whose widgets show its count: on the Mac's
// desktop and in Notification Center, and on the Home Screen and the Lock
// Screen of an iPhone or an iPad. Each change writes the widgets' timeline
// with package widget, and the medium widget counts down to the next hour,
// which the system shows without the app. Its Live Activity shows the
// count on the Lock Screen and in the Dynamic Island, or as a Live Update
// on Android, with buttons that change it without opening the app on iOS.
package main

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/nsgo-dev/nsgo/widget"
)

func main() {
	// Before nsgo.Run: iOS starts the app in the background for a press,
	// without its views.
	widget.OnAction(pressed)
	nsgo.Run(Root, nsgo.WindowSize(420, 360))
}

// countKey keeps the count in the app's storage.
const countKey = "count"

// activityKind is the counter's Live Activity.
const activityKind = "counter"

// presses hands the counts the Live Activity's buttons set to the views,
// which keep the last while they do not run.
var presses struct {
	sync.Mutex
	set     func(n int)
	pending *int
}

// pressed sets the count a button of the activity names, "set:<n>", so a
// press needs nothing of the app's views: it updates the activity itself,
// as the system holds it until the handler returns.
func pressed(p widget.ActivityPress) {
	n, err := strconv.Atoi(strings.TrimPrefix(p.Action, "set:"))
	if err != nil {
		return
	}
	p.Activity.Update(activityContent(n))
	presses.Lock()
	set := presses.set
	if set == nil {
		presses.pending = &n
	}
	presses.Unlock()
	if set != nil {
		set(n)
	}
}

func Root(cx *core.Scope) core.Node {
	storage := cx.Storage()
	saved, _ := storage.GetItem(countKey)
	n, _ := strconv.Atoi(saved)
	count := core.NewSignal(cx, n)
	presses.Lock()
	if presses.pending != nil {
		count.Set(*presses.pending)
	}
	presses.set = func(n int) { cx.Post(func() { count.Set(n) }) }
	presses.Unlock()
	status := core.NewSignal(cx, "")
	var running *widget.Activity
	for _, a := range widget.Activities() {
		if a.Kind() == activityKind {
			running = a
		}
	}
	activity := core.NewSignal(cx, running)
	cx.Effect(func() {
		n := count.Get()
		storage.SetItem(countKey, strconv.Itoa(n))
		msg := "Add the Counter widget from the system's widgets."
		if err := widget.Update("counter", entry(n, time.Now())); err != nil {
			msg = err.Error()
		}
		if a := activity.Get(); a != nil {
			if err := a.Update(activityContent(n)); err != nil {
				activity.Set(nil)
			}
		}
		status.Set(msg)
	})
	toggle := func() {
		if a := activity.Get(); a != nil {
			a.End(nil, widget.DismissNow)
			activity.Set(nil)
			return
		}
		a, err := widget.StartActivity(activityKind, activityContent(count.Get()))
		if err != nil {
			status.Set(err.Error())
			return
		}
		activity.Set(a)
	}
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
		core.Dynamic(func(cx *core.Scope) core.Node {
			text := "Start Live Activity"
			if activity.Get() != nil {
				text = "End Live Activity"
			}
			return ui.Button{Text: text, Variant: ui.ButtonSecondary, OnPress: toggle, TestID: "activity"}
		}),
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

// activityContent is what the Live Activity shows of the count: its
// fields, which Android shows too, and on iOS the default views they make,
// with the count larger in the expanded Dynamic Island.
func activityContent(n int) widget.ActivityContent {
	count := strconv.Itoa(n)
	return widget.ActivityContent{
		Title: "Count", Body: "Change it from here.", Status: count, Symbol: "number.circle.fill",
		ShowProgress: true, Progress: float64((n%10+10)%10) / 10, Tint: widget.RGB(0x30d158),
		Actions: []widget.ActivityAction{
			{ID: "set:" + strconv.Itoa(n-1), Title: "Less", Symbol: "minus"},
			{ID: "set:" + strconv.Itoa(n+1), Title: "More", Symbol: "plus"},
		},
		Trailing: widget.Text{Content: count, Style: widget.Style{FontSize: 34, FontWeight: core.FontWeightBold, FontVariant: core.FontTabularNums, Padding: core.Edges{Right: 6}}},
	}
}
