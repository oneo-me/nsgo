package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestToggleButton turns on and off with presses, filled with the accent
// and reported selected while on.
func TestToggleButton(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var on *core.Signal[bool]
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			on = core.NewSignal(cx, false)
			return ToggleButton{Text: "Bold", TestID: "bold", On: on.Get, OnChange: on.Set}
		})
		app.Press("bold")
		if b := app.Find("bold"); !on.Get() || !b.Props.Selected || b.Children[0].Style.Background != p.Accent {
			t.Fatal("a press does not turn the toggle on, selected and filled with the accent")
		}
		app.Press("bold")
		if b := app.Find("bold"); on.Get() || b.Props.Selected || b.Children[0].Style.Background == p.Accent {
			t.Fatal("a second press does not turn the toggle off")
		}
	})
}
