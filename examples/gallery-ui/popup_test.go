package ui

import (
	"testing"
	"time"

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

// TestTooltip shows a tooltip as the pointer rests on its trigger.
func TestTooltip(t *testing.T) {
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignStart, Padding: core.All(40)}, Children: []core.Node{
			Tooltip{Text: "Shared with 3 people", Trigger: core.Text{Content: "Shared"}, TestID: "shared"},
		}}
	})
	app.Hover("shared")
	app.Advance(600 * time.Millisecond)
	app.Find("Shared with 3 people")
}

// TestMenu chooses items of a menu and its submenu: in a popup on
// desktops, in a sheet on phones.
func TestMenu(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var chosen string
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			wrap := core.NewSignal(cx, false)
			return core.View{Style: core.Style{AlignItems: core.AlignStart}, Children: []core.Node{
				Menu{TestID: "menu", Trigger: Button{Text: "Edit"}, Children: []core.Node{
					MenuItem{Text: "Copy", Shortcut: "⌘C", OnSelect: func() { chosen = "copy" }},
					MenuItem{Text: "Word wrap", Checked: wrap.Get, OnSelect: func() { wrap.Set(!wrap.Get()) }},
					MenuSeparator{},
					Submenu{Text: "Share", Back: "Back", Children: []core.Node{
						MenuItem{Text: "Email", OnSelect: func() { chosen = "email" }},
					}},
					MenuItem{Text: "Delete", Danger: true, OnSelect: func() { chosen = "delete" }},
				}},
			}}
		})
		app.Press("menu")
		app.Press("role:menuitem Copy")
		if chosen != "copy" || app.Exists("role:menuitem Copy") {
			t.Fatalf("chose %q, want copy and the menu closed", chosen)
		}
		app.Press("menu")
		app.Press("role:menuitem Share")
		app.Press("role:menuitem Email")
		if chosen != "email" {
			t.Fatalf("chose %q from the submenu, want email", chosen)
		}
	})
}

// TestContextMenu opens a context menu at a secondary click.
func TestContextMenu(t *testing.T) {
	var chosen string
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return ContextMenu{TestID: "file", Trigger: core.Text{Content: "report.pdf"}, Children: []core.Node{
			MenuItem{Text: "Rename", OnSelect: func() { chosen = "rename" }},
		}}
	})
	app.RightClick("file")
	app.Press("role:menuitem Rename")
	if chosen != "rename" {
		t.Fatalf("chose %q, want rename", chosen)
	}
}

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
