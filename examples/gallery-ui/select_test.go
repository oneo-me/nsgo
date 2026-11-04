package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestSelect shows the placeholder until a choice, then the chosen
// option's label, checked in the menu it is chosen from.
func TestSelect(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var fruit *core.Signal[string]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			fruit = core.NewSignal(cx, "")
			return core.View{Style: core.Style{AlignItems: core.AlignStart}, Children: []core.Node{
				Select[string]{TestID: "fruit", Label: "Fruit", Placeholder: "Pick a fruit", Value: fruit.Get, OnChange: fruit.Set,
					Options: []Option[string]{{Value: "apple", Label: "Apple"}, {Value: "pear", Label: "Pear"}}},
			}}
		})
		if !app.Exists("text:Pick a fruit") {
			t.Fatal("no placeholder before a choice")
		}
		app.Press("fruit")
		app.Press("role:menuitem Pear")
		if fruit.Get() != "pear" || !app.Exists("text:Pear") || app.Exists("text:Pick a fruit") {
			t.Fatalf("chose %q, want pear shown in the field", fruit.Get())
		}
		app.Press("fruit")
		check := func(id string) float32 { return app.Find(id).Children[0].Style.Opacity }
		if check("fruit-1") == 0 || check("fruit-0") != 0 {
			t.Fatal("the chosen option is not checked")
		}
	})
}
