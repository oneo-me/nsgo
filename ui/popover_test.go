package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TestPopover opens a popover from its trigger and closes it with Escape.
func TestPopover(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			open := core.NewSignal(cx, false)
			return core.View{Style: core.Style{AlignItems: core.AlignStart}, Children: []core.Node{
				Popover{Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) }, TestID: "filters",
					Trigger: Button{Text: "Filters"}, Children: []core.Node{core.Text{Content: "Popover body"}}},
			}}
		})
		app.Press("filters")
		if surface := app.Find("Popover body").Parent; surface.Style.CornerRadius != RadiusCard {
			t.Fatalf("popover radius = %v, want a card's", surface.Style.CornerRadius)
		}
		app.Key("Escape")
		if app.Exists("Popover body") {
			t.Fatal("Escape left the popover open")
		}
	})
}
