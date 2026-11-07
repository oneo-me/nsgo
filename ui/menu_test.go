package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

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
					MenuItem{Text: "Delete", Variant: MenuItemDanger, OnSelect: func() { chosen = "delete" }},
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
