package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestSelection checks a checkbox, an indeterminate one and a radio of a
// group, their parts following them.
func TestSelection(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			checked, mixed, choice := core.NewSignal(cx, false), core.NewSignal(cx, true), core.NewSignal(cx, "a")
			radio := func(v string) core.Node {
				return Radio{Text: "Option " + v, TestID: "radio-" + v, Checked: func() bool { return choice.Get() == v },
					OnChange: func(bool) { choice.Set(v) }}
			}
			return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
				Checkbox{Text: "Remember", TestID: "check", Checked: checked.Get, OnChange: checked.Set},
				Checkbox{Text: "All", TestID: "all", Checked: func() bool { return false }, Indeterminate: mixed.Get,
					OnChange: func(bool) { mixed.Set(false) }},
				Checkbox{Text: "Locked", TestID: "locked", Disabled: func() bool { return true }},
				RadioGroup{Label: "Options", Children: []core.Node{radio("a"), radio("b")}},
			}}
		})
		box := func() *nsgotest.View { return app.Find("check").Children[0] }
		if box().Style.Background != p.Field {
			t.Fatalf("unchecked box = %x, want the field", box().Style.Background)
		}
		app.Press("check")
		if !app.Find("check").Props.Checked || box().Style.Background != p.AccentFill {
			t.Fatalf("checked box = %x, want the accent's fill", box().Style.Background)
		}
		if all := app.Find("all"); !all.Props.Indeterminate || all.Children[0].Style.Background != p.AccentFill {
			t.Fatal("the indeterminate box is not filled")
		}
		app.Press("all")
		if app.Find("all").Props.Indeterminate {
			t.Fatal("pressing the indeterminate box kept it so")
		}
		app.Press("role:radio Option b")
		if !app.Find("radio-b").Props.Checked || app.Find("radio-a").Props.Checked {
			t.Fatal("pressing option b did not check it alone")
		}
		if ring := app.Find("radio-b").Children[0]; ring.Style.Background != p.AccentFill {
			t.Fatalf("checked ring = %x, want the accent's fill", ring.Style.Background)
		}
		if l := app.Find("locked"); !l.Props.Disabled || l.Children[0].Style.Opacity >= 1 {
			t.Fatal("the disabled checkbox is not Dimmed")
		}
	})
}
