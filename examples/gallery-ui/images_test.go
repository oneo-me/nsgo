package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// TestImage keeps its well's ratio and shows the placeholder until the
// picture loads.
func TestImage(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Width: core.Pt(300)}, Children: []core.Node{
				Image{Source: core.Src("https://example.com/coast.jpg"), Label: "Coast", TestID: "coast"},
			}}
		})
		well := app.Find("role:image Coast").Parent
		if s := well.Frame.Size; s.Width != 300 || s.Height != 200 || well.Style.Background != p.Inset {
			t.Fatalf("well = %v, %x; want 300 by 200 on the inset color", s, well.Style.Background)
		}
		if len(well.Children) != 2 {
			t.Fatalf("well holds %d views while loading, want the placeholder and the picture", len(well.Children))
		}
		app.Load("coast", core.Size{Width: 1200, Height: 800})
		if len(well.Children) != 1 {
			t.Fatal("the placeholder stays over the loaded picture")
		}
	})
}

// TestImageCompare moves the divider where it is pressed, along a drag
// and by a screen reader's steps.
func TestImageCompare(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var position *core.Signal[float32]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			position = core.NewSignal(cx, float32(50))
			return core.View{Style: core.Style{Width: core.Pt(300)}, Children: []core.Node{
				ImageCompare{TestID: "compare", Label: "Divider", Position: position.Get, OnChange: position.Set,
					BeforeLabel: "Before", AfterLabel: "After",
					Before: core.View{Style: core.Style{FlexGrow: 1, Background: core.RGB(0x336699)}},
					After:  core.View{Style: core.Style{FlexGrow: 1, Background: core.RGB(0x999999)}}},
			}}
		})
		reveal := app.Find("compare").Children[1]
		if w := reveal.Frame.Size.Width; w != 150 {
			t.Fatalf("before shows %v of 300, want 150", w)
		}
		if w := reveal.Children[0].Frame.Size.Width; w != 300 {
			t.Fatalf("before is laid out %v wide, want the frame's 300", w)
		}
		app.Pan("compare", core.Point{X: 30}, core.Point{X: 60})
		if got := position.GetUntracked(); got != 70 {
			t.Fatalf("after a drag of 60 position = %v, want 70", got)
		}
		app.AccessibilityAction("compare", core.ActionDecrement)
		if got := position.GetUntracked(); got != 65 {
			t.Fatalf("after a step down position = %v, want 65", got)
		}
		if v := app.Find("role:adjustable Divider").Props.Value; v.Now != 65 || v.Max != 100 {
			t.Fatalf("value = %+v, want 65 of 100", v)
		}
	})
}

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
		step := track.Scroll.Behavior.SnapToInterval
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
		if !app.Find("role:button Next").Props.Disabled || !app.Find("role:tab 4 / 4").Props.Selected {
			t.Fatal("the last slide leaves Next enabled or its indicator unselected")
		}
		app.Press("role:tab 1 / 4")
		app.Advance(time.Second)
		if index.GetUntracked() != 0 || app.Find("carousel").Offset.X != 0 {
			t.Fatal("the first indicator did not bring the first slide back")
		}
	})
}
