//go:build darwin

package apple

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
)

// TestCanvasLayerDrawsWhatAFullReplayDoes plays the split samples whole
// and over a layer of their static part, and compares the pixels.
func TestCanvasLayerDrawsWhatAFullReplayDoes(t *testing.T) {
	for _, s := range canvastest.Split(TextEngine{}) {
		t.Run(s.Name, func(t *testing.T) {
			d := s.Drawing
			split := driver.SplitCanvas(d)
			if !split.Static(d) {
				t.Fatalf("split %+v keeps nothing", split)
			}
			w, h := float64(s.Width), float64(s.Height)
			draw := func(over bool) *image.RGBA {
				ctx, img := bitmap(s.Width, s.Height)
				defer Release(ctx)
				DrawTopDown(ctx, h, true, func(baseUp bool) {
					host := CanvasHost{Ratio: canvastest.Scale, BaseUp: baseUp}
					if !over {
						Play(ctx, d.Ops, host)
						return
					}
					l := NewCanvasLayer(ctx, d, d.Ops, split, w, h, canvastest.Scale, host)
					if l == nil {
						t.Fatal("no layer")
					}
					defer l.Release()
					l.Draw(ctx, w, h)
					PlayOver(ctx, d.Ops, split, host)
				})
				return img
			}
			whole, over := draw(false), draw(true)
			if dir := os.Getenv("NSGO_CANVAS_PNG"); dir != "" {
				for name, img := range map[string]*image.RGBA{"whole": whole, "over": over} {
					if f, err := os.Create(dir + "/apple-split-" + s.Name + "-" + name + ".png"); err == nil {
						png.Encode(f, img)
						f.Close()
					}
				}
			}
			if n, at := canvastest.Differ(whole, over); n > 0 {
				t.Errorf("%d pixels differ from a full replay, the first at %v", n, at)
			}
		})
	}
}
