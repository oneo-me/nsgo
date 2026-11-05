package ui

import (
	"math"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// TestProgress fills a determinate bar to its value and sweeps an
// indeterminate one.
func TestProgress(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 16}, Children: []core.Node{
				Progress{Label: "Upload", TestID: "upload", Value: func() float32 { return 40 }},
				Progress{Label: "Sync", TestID: "sync", Indeterminate: func() bool { return true }},
			}}
		})
		bar := app.Find("upload")
		track, fill := bar.Children[0], bar.Children[0].Children[0]
		if r := fill.Frame.Size.Width / track.Frame.Size.Width; math.Abs(float64(r)-0.4) > 0.01 {
			t.Fatalf("fill = %v of the track, want 0.4", r)
		}
		if bar.Props.Value.Now != 40 {
			t.Fatalf("value = %v, want 40", bar.Props.Value.Now)
		}
		sweep := func() core.Points { return app.Find("sync").Children[0].Children[0].Style.Transform.TranslateX }
		before := sweep()
		app.FastForward(time.Second)
		if sweep() == before {
			t.Fatal("the indeterminate bar does not sweep")
		}
	})
}
