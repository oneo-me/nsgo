package ui

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
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

// TestCarouselLooping goes around the ends with the arrows, each slide
// keeping both its place in the loop and its scale.
func TestCarouselLooping(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var index *core.Signal[int]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			index = core.NewSignal(cx, 0)
			return core.View{Style: core.Style{Width: core.Pt(350)}, Children: []core.Node{
				Carousel{TestID: "carousel", Label: "Rooms", Count: 4, SlideWidth: 200, Looping: true, Index: index.Get, OnChange: index.Set,
					PreviousLabel: "Previous", NextLabel: "Next",
					Slide: func(cx *core.Scope, i int) core.Node {
						return core.View{TestID: "slide", Style: core.Style{Height: core.Pt(160)}}
					}},
			}}
		})
		step := core.Points(200 + 12)
		period := 4 * step
		at := func(i int) core.Point { return core.Point{X: period + core.Points(i)*step} }
		if got := app.Find("carousel").Offset; got != at(0) {
			t.Fatalf("rests at %v, want %v", got, at(0))
		}
		look := func(shown, beside string) {
			t.Helper()
			center, side := app.Find("role:group "+shown).Animated, app.Find("role:group "+beside).Animated
			if center.Scale != 1 || center.Opacity != 1 || side.Scale != 0.9 || side.Opacity != 0.5 {
				t.Fatalf("%s scales %v and fades %v, %s scales %v and fades %v", shown, center.Scale, center.Opacity, beside, side.Scale, side.Opacity)
			}
		}
		look("1 / 4", "4 / 4")
		// The last slide is the first's neighbor before it.
		last := app.Find("role:group 4 / 4")
		first := app.Find("role:group 1 / 4")
		if got := core.Points(last.Animated.TranslateX) + last.Frame.Origin.X - first.Frame.Origin.X; got != -step {
			t.Fatalf("the last slide is %v from the first, want %v", got, -step)
		}
		if app.Find("role:button Previous").Props.Disabled {
			t.Fatal("Previous is disabled on the first slide of a loop")
		}
		app.Press("role:button Previous")
		app.Advance(time.Second)
		if index.GetUntracked() != 3 || app.Find("carousel").Offset != at(3) {
			t.Fatalf("Previous around the start: index %d at %v, want 3 at %v", index.GetUntracked(), app.Find("carousel").Offset, at(3))
		}
		look("4 / 4", "1 / 4")
		app.Press("role:button Next")
		app.Advance(time.Second)
		if index.GetUntracked() != 0 || app.Find("carousel").Offset != at(0) {
			t.Fatalf("Next around the end: index %d at %v, want 0 at %v", index.GetUntracked(), app.Find("carousel").Offset, at(0))
		}
		look("1 / 4", "4 / 4")
	})
}
