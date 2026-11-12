package ui

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
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
					Children: []core.Node{Input{Label: "Name", DefaultValue: "report.pdf"}},
					Footer: []core.Node{Button{Variant: ButtonSecondary, Text: "Cancel", TestID: "cancel", OnPress: func() { open.Set(false) }},
						Button{Variant: ButtonPrimary, Text: "Rename"}}},
				AlertDialog{TestID: "alert", Title: "Delete report.pdf?", Description: "This cannot be undone.",
					Open: alert.Get, OnOpenChange: func(v bool, _ base.Reason) { alert.Set(v) },
					Actions: []AlertAction{{Text: "Cancel", Variant: ButtonSecondary, TestID: "keep"},
						{Text: "Delete", Color: ColorsOf(cx).Danger, TestID: "confirm", OnPress: func() { confirmed = true }}}},
			}}
		})
		app.Press("open")
		d := app.Find("dialog")
		if e.Idiom == core.IdiomDesktop {
			if d.Style.CornerRadius != RadiusDialog {
				t.Fatal("the desktop dialog is not a rounded panel")
			}
			if _, _, _, a := d.Style.BorderColor.Components(); a != 1 || len(d.Style.Effects.Shadows) == 0 {
				t.Fatalf("the desktop dialog's rim %v is not solid over a shadow", d.Style.BorderColor)
			}
		} else if d.Style.Background != p.Surface || d.Style.Corners.BottomLeft != 0 || d.Style.Corners.TopLeft == 0 {
			t.Fatal("the phone dialog is not a sheet from the bottom")
		}
		app.Find("Rename file")
		app.Press("cancel")
		if e.Idiom == core.IdiomDesktop {
			app.Find("Rename file") // it fades out
			app.Advance(200 * time.Millisecond)
		}
		app.WaitForGone("Rename file", time.Second)
		app.Press("open-alert")
		app.Press("confirm")
		app.Advance(200 * time.Millisecond)
		app.WaitForGone("Delete report.pdf?", time.Second)
		if !confirmed {
			t.Fatal("the alert's action did not run")
		}
	})
}
