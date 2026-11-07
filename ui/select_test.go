package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// TestSelect shows the placeholder until a choice, then the chosen
// option's label, selected without a check in the menu it is chosen from.
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
		app.Press("role:option Pear")
		if fruit.Get() != "pear" || !app.Exists("text:Pear") || app.Exists("text:Pick a fruit") {
			t.Fatalf("chose %q, want pear shown in the field", fruit.Get())
		}
		app.Press("fruit")
		if !app.Find("role:option Pear").Props.Selected || app.Find("role:option Apple").Props.Selected {
			t.Fatal("the chosen option's row is not selected")
		}
		if n := len(app.Find("fruit-1").Children); n != 1 {
			t.Fatalf("a row has %d children, want its text, with no check", n)
		}
	})
}

// TestSelectOptionSlots shows an option's Leading in its row and in the
// field while chosen, and its Trailing only in its row, where a button
// leaves the choice and the menu alone.
func TestSelectOptionSlots(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var sound *core.Signal[string]
		played := ""
		option := func(id string) Option[string] {
			return Option[string]{Value: id, Label: id, Leading: core.View{TestID: "mark-" + id, Style: core.Style{Width: core.Pt(14), Height: core.Pt(14)}},
				Trailing: IconButton{Icon: lucide.Play, Label: "Play " + id, Size: 24, TestID: "play-" + id, OnPress: func() { played = id }}}
		}
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			sound = core.NewSignal(cx, "bell")
			return core.View{Style: core.Style{AlignItems: core.AlignStart}, Children: []core.Node{
				Select[string]{TestID: "sound", Label: "Sound", Value: sound.Get, OnChange: sound.Set,
					Options: []Option[string]{option("bell"), option("chime")}},
			}}
		})
		if !app.Exists("mark-bell") || app.Exists("mark-chime") || app.Exists("play-bell") {
			t.Fatal("the field does not show just the chosen option's Leading")
		}
		app.Press("sound")
		if !app.Exists("mark-chime") {
			t.Fatal("no Leading in the menu")
		}
		if n := len(app.Find("sound-1").Children); n != 3 {
			t.Fatalf("a row has %d children, want Leading, text and Trailing", n)
		}
		app.Press("play-chime")
		if played != "chime" || sound.Get() != "bell" || !app.Exists("role:option chime") {
			t.Fatalf("played %q with %q chosen, want chime played, bell chosen and the menu open", played, sound.Get())
		}
		app.Press("role:option chime")
		if sound.Get() != "chime" || !app.Exists("mark-chime") || app.Exists("mark-bell") {
			t.Fatalf("chose %q, want chime's Leading in the field", sound.Get())
		}
	})
}
