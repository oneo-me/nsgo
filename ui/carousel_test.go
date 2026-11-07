package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// TestCarousel centers the chosen slide, follows the user's scroll and
// stops the arrows at the ends.
func TestCarousel(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var index *core.Signal[int]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			index = core.NewSignal(cx, 0)
			return core.View{Style: core.Style{Width: core.Pt(350)}, Children: []core.Node{
				Carousel{TestID: "carousel", Label: "Rooms", Count: 4, Index: index.Get, OnChange: index.Set,
					PreviousLabel: "Previous", NextLabel: "Next",
					Slide: func(cx *core.Scope, i int) core.Node {
						return core.View{Style: core.Style{Height: core.Pt(160)}}
					}},
			}}
		})
		track := app.Find("carousel")
		step := track.Scroll.Behavior.SnapToOffsets[1]
		if step != 280+12 {
			t.Fatalf("snaps every %v, want a slide of 280 and its gap", step)
		}
		if !app.Find("role:button Previous").Props.Disabled {
			t.Fatal("Previous is enabled on the first slide")
		}
		app.Press("role:button Next")
		app.Advance(time.Second)
		if index.GetUntracked() != 1 || app.Find("carousel").Offset.X != step {
			t.Fatalf("after Next: index %d at %v, want 1 at %v", index.GetUntracked(), app.Find("carousel").Offset.X, step)
		}
		app.DragScroll("carousel", core.Point{X: step * 3}, 0)
		app.Advance(time.Second)
		if index.GetUntracked() != 3 {
			t.Fatalf("after scrolling to the end index = %d, want 3", index.GetUntracked())
		}
		if !app.Find("role:button Next").Props.Disabled || !app.Find("role:button 4 / 4").Props.Selected {
			t.Fatal("the last slide leaves Next enabled or its indicator unselected")
		}
		app.Press("role:button 1 / 4")
		app.Advance(time.Second)
		if index.GetUntracked() != 0 || app.Find("carousel").Offset.X != 0 {
			t.Fatal("the first indicator did not bring the first slide back")
		}
	})
}
