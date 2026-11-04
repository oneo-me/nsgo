package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestSkeleton pulses its bars in a wave, reads as one element, and
// gives way to the content once it loads.
func TestSkeleton(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var loading *core.Signal[bool]
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			loading = core.NewSignal(cx, true)
			return SkeletonContent{Loading: loading.Get, Placeholder: Skeleton{Lines: 4, Avatar: true, Label: "Loading", TestID: "skeleton"},
				Children: []core.Node{core.Text{Content: "Loaded"}}}
		})
		s := app.Find("label:Loading")
		bars := s.Children[1].Children
		if len(bars) != 4 {
			t.Fatalf("skeleton has %d bars, want 4", len(bars))
		}
		if bars[0].Style.Background != p.Border || bars[2].Style.Animation.Delay <= bars[1].Style.Animation.Delay {
			t.Fatal("the bars do not pulse in a wave of the border color")
		}
		loading.Set(false)
		app.Settle()
		app.Find("Loaded")
		if app.Exists("label:Loading") {
			t.Fatal("the skeleton stays after loading")
		}
	})
}
