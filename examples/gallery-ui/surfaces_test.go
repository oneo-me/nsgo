package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// TestCard draws a card on the surface, and a pressable one as a
// button.
func TestCard(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		pressed := 0
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
				Card{TestID: "card", Title: "Storage", Description: "12 GB of 50 GB used",
					Actions:  []core.Node{Button{Variant: ButtonGhost, Icon: IconMore, Label: "More"}},
					Children: []core.Node{Progress{Label: "Used", Value: func() float32 { return 24 }}},
					Footer:   []core.Node{Button{Text: "Manage"}}},
				Card{Title: "Open project", OnPress: func() { pressed++ }, Children: []core.Node{core.Text{Content: "Onexa"}}},
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

// TestScrollView fades the content at an edge only while more of it
// lies past that edge, and floats its bars over the content.
func TestScrollView(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			var rows []core.Node
			for range 30 {
				rows = append(rows, core.View{Style: core.Style{Height: core.Pt(40)}})
			}
			return ScrollView{TestID: "scroll", Style: core.Style{Height: core.Pt(300)}, Children: rows,
				BottomBar: []core.Node{IconButton{Icon: IconPlus, Label: "Add", OnPress: func() {}}}, BottomBarLabel: "Actions"}
		})
		frame := app.Find("scroll").Parent
		top, bottom := frame.Children[1], frame.Children[2]
		if top.Animated.Opacity != 0 || bottom.Animated.Opacity != 1 {
			t.Fatalf("at the top: fades %v and %v, want 0 and 1", top.Animated.Opacity, bottom.Animated.Opacity)
		}
		app.Scroll("scroll", 200)
		app.Advance(100 * time.Millisecond)
		if top.Animated.Opacity != 1 || bottom.Animated.Opacity != 1 {
			t.Fatalf("in the middle: fades %v and %v, want both", top.Animated.Opacity, bottom.Animated.Opacity)
		}
		app.ScrollTo("scroll", core.Point{Y: app.Find("scroll").Content.Height - 300})
		app.Advance(100 * time.Millisecond)
		if bottom.Animated.Opacity != 0 {
			t.Fatalf("at the end: bottom fade %v, want 0", bottom.Animated.Opacity)
		}
		bar := app.Find("role:button Add")
		if f := app.Find("scroll").Frame; bar.Frame.Origin.Y+bar.Frame.Size.Height > f.Origin.Y+f.Size.Height {
			t.Fatal("the bottom bar is below the scroll view")
		}
	})
}

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
