package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
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

// TestSelectOptionSlots shows an option's Leading in its row and in the
// field while chosen, and its Trailing only in its row, where a button
// leaves the choice and the menu alone; NoCheck drops the check.
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
				Select[string]{TestID: "sound", Label: "Sound", NoCheck: true, Value: sound.Get, OnChange: sound.Set,
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
			t.Fatalf("a row has %d children, want Leading, text and Trailing, with no check", n)
		}
		app.Press("play-chime")
		if played != "chime" || sound.Get() != "bell" || !app.Exists("role:menuitem chime") {
			t.Fatalf("played %q with %q chosen, want chime played, bell chosen and the menu open", played, sound.Get())
		}
		app.Press("role:menuitem chime")
		if sound.Get() != "chime" || !app.Exists("mark-chime") || app.Exists("mark-bell") {
			t.Fatalf("chose %q, want chime's Leading in the field", sound.Get())
		}
	})
}
