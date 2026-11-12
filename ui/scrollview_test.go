package ui

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

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
				BottomBar: []core.Node{IconButton{Icon: lucide.Plus, Label: "Add", OnPress: func() {}}}, BottomBarLabel: "Actions"}
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
