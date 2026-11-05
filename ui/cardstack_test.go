package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// TestCardStack decides cards by a throw and by the buttons, brings one
// back, and shows the empty view after the last.
func TestCardStack(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var decided []CardDecision
		var index *core.Signal[int]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			index = core.NewSignal(cx, 0)
			var items []CardStackItem
			for _, city := range []string{"Lisbon", "Kyoto", "Oslo"} {
				items = append(items, CardStackItem{ID: city, Label: city, Content: core.Text{Content: city}})
			}
			return CardStack{TestID: "deck", Label: "Cities", Items: items, Index: index.Get, LeftLabel: "Pass", RightLabel: "Keep",
				Empty:    core.Text{Content: "All done"},
				OnDecide: func(i int, d CardDecision) { decided = append(decided, d); index.Set(i + 1) }}
		})
		app.Pan("deck", core.Point{X: 60}, core.Point{X: 180})
		app.Advance(time.Second)
		if len(decided) != 1 || decided[0] != CardRight || index.GetUntracked() != 1 {
			t.Fatalf("after a throw right: %v, index %d", decided, index.GetUntracked())
		}
		app.Pan("deck", core.Point{X: -40})
		app.Advance(time.Second)
		if len(decided) != 1 {
			t.Fatal("a short drag decided the card")
		}
		app.Press("role:button Pass")
		app.Advance(time.Second)
		if len(decided) != 2 || decided[1] != CardLeft {
			t.Fatalf("after Pass: %v", decided)
		}
		index.Set(1)
		app.Settle()
		app.Find("label:Kyoto")
		app.AccessibilityAction("deck", "right")
		app.Advance(time.Second)
		app.Press("role:button Keep")
		app.Advance(time.Second)
		app.Find("All done")
		if !app.Find("role:button Keep").Props.Disabled {
			t.Fatal("Keep is enabled with no cards left")
		}
	})
}
