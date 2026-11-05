package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TestDialog opens a dialog, centered on desktops and a sheet on phones,
// and closes it from its footer; an alert dialog keeps open on its
// backdrop and closes on an action.
func TestDialog(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var confirmed bool
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			open, alert := core.NewSignal(cx, false), core.NewSignal(cx, false)
			return core.View{Style: core.Style{AlignItems: core.AlignStart, Gap: 8}, Children: []core.Node{
				Button{Text: "Rename", TestID: "open", OnPress: func() { open.Set(true) }},
				Button{Text: "Delete", TestID: "open-alert", OnPress: func() { alert.Set(true) }},
				Dialog{TestID: "dialog", Title: "Rename file", Description: "Choose a new name.",
					Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
					Children: []core.Node{Input{Input: base.Input{Label: "Name", DefaultValue: "report.pdf"}}},
					Footer: []core.Node{Button{Variant: ButtonSecondary, Text: "Cancel", TestID: "cancel", OnPress: func() { open.Set(false) }},
						Button{Variant: ButtonAccent, Text: "Rename"}}},
				AlertDialog{TestID: "alert", Title: "Delete report.pdf?", Description: "This cannot be undone.",
					Open: alert.Get, OnOpenChange: func(v bool, _ base.Reason) { alert.Set(v) },
					Actions: []AlertAction{{Text: "Cancel", Variant: ButtonSecondary, TestID: "keep"},
						{Text: "Delete", Color: ColorsOf(cx).Danger, TestID: "confirm", OnPress: func() { confirmed = true }}}},
			}}
		})
		app.Press("open")
		d := app.Find("dialog")
		if d.Style.Background != p.Surface {
			t.Fatalf("dialog = %x, want the surface", d.Style.Background)
		}
		if e.Idiom == core.IdiomDesktop {
			if d.Style.CornerRadius != RadiusCard {
				t.Fatal("the desktop dialog is not a rounded card")
			}
		} else if d.Style.Corners.BottomLeft != 0 || d.Style.Corners.TopLeft == 0 {
			t.Fatal("the phone dialog is not a sheet from the bottom")
		}
		app.Find("Rename file")
		app.Press("cancel")
		app.WaitForGone("Rename file", time.Second)
		app.Press("open-alert")
		app.Press("confirm")
		app.WaitForGone("Delete report.pdf?", time.Second)
		if !confirmed {
			t.Fatal("the alert's action did not run")
		}
	})
}
