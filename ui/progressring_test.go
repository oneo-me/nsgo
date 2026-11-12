package ui

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestProgressRing reads its value as Progress does, and the spinners
// turn while work goes on.
func TestProgressRing(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{
				ProgressRing{Label: "Upload", TestID: "upload", Value: func() float32 { return 40 }},
				Spinner{Label: "Sync", TestID: "ring"},
				Spinner{Label: "Load", TestID: "spokes", Variant: SpinnerSpokes, Size: 20},
			}}
		}, nsgotest.ManualFrames())
		if v := app.Find("upload").Props.Value; v.Now != 40 || v.Max != 100 {
			t.Fatalf("value = %v of %v, want 40 of 100", v.Now, v.Max)
		}
		if f := app.Find("spokes").Frame.Size; f.Width != 20 || f.Height != 20 {
			t.Fatalf("spokes are %v, want 20 pt across", f)
		}
		turn := func(id string) float32 { return app.Find(id).Animated.Rotate }
		before := []float32{turn("ring"), turn("spokes")}
		app.Advance(350 * time.Millisecond)
		if turn("ring") == before[0] || turn("spokes") == before[1] {
			t.Fatal("the spinners stand still")
		}
	})
}
