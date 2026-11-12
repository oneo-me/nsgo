package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// TestButton presses each variant, 28 points tall on every device, and an
// icon's button as a square.
func TestButton(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var pressed []string
		variants := []ButtonVariant{ButtonPrimary, ButtonSecondary, ButtonOutline, ButtonGhost}
		names := []string{"primary", "secondary", "outline", "ghost"}
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			var row []core.Node
			for i, v := range variants {
				row = append(row, Button{Variant: v, Text: names[i], TestID: names[i], OnPress: func() { pressed = append(pressed, names[i]) }})
			}
			row = append(row,
				Button{Icon: lucide.Plus, Label: "Add", TestID: "icon", Variant: ButtonOutline},
				Button{Text: "Off", TestID: "disabled", Disabled: func() bool { return true }, OnPress: func() { pressed = append(pressed, "off") }})
			return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8, AlignItems: core.AlignStart}, Children: row}
		})
		for _, n := range names {
			app.Press(n)
			if h := app.Find(n).Frame.Size.Height; h != ButtonHeight {
				t.Fatalf("%s takes presses %v tall, want %v", n, h, ButtonHeight)
			}
			if h := app.Find(n).Children[0].Frame.Size.Height; h != ButtonHeight {
				t.Fatalf("%s face = %v tall, want %v", n, h, ButtonHeight)
			}
		}
		if len(pressed) != len(names) {
			t.Fatalf("pressed %v, want each variant", pressed)
		}
		if got := app.Find("primary").Children[0].Style.Background; got != p.AccentFill {
			t.Fatalf("primary = %x, want the accent", got)
		}
		if got := app.Find("ghost").Children[0].Style.Background; got != 0 {
			t.Fatalf("ghost = %x at rest, want no fill", got)
		}
		if f := app.Find("icon").Children[0].Frame.Size; f.Width != ButtonHeight || f.Height != ButtonHeight {
			t.Fatalf("icon's button = %v, want a %v square", f, ButtonHeight)
		}
		if app.Find("role:button Add").Props.Label != "Add" {
			t.Fatal("the icon's button has no label")
		}
		if !app.Find("disabled").Props.Disabled {
			t.Fatal("the disabled button is not reported disabled")
		}
	})
}
