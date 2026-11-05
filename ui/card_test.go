package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// TestCard draws a card on the surface, and a pressable one as a
// button.
func TestCard(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		pressed := 0
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
				Card{TestID: "card", Title: "Storage", Description: "12 GB of 50 GB used",
					Actions:  []core.Node{Button{Variant: ButtonGhost, Icon: lucide.Ellipsis, Label: "More"}},
					Children: []core.Node{Progress{Label: "Used", Value: func() float32 { return 24 }}},
					Footer:   []core.Node{Button{Text: "Manage"}}},
				Card{Title: "Open project", OnPress: func() { pressed++ }, Children: []core.Node{core.Text{Content: "NSGo"}}},
			}}
		})
		card := app.Find("card").Children[1]
		if card.Style.Background != p.Surface || card.Style.CornerRadius != RadiusCard {
			t.Fatalf("card = %x, radius %v; want the surface", card.Style.Background, card.Style.CornerRadius)
		}
		app.Find("role:header Storage")
		app.Find("role:button More")
		app.Press("role:button Open project")
		if pressed != 1 {
			t.Fatalf("pressed %d times, want 1", pressed)
		}
	})
}
