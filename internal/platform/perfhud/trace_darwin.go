//go:build darwin

package perfhud

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// DrawTrace draws the performance panel's trace of s into ctx's w × h
// points (engine.PerfTrace); flipped says ctx's y axis points down.
func DrawTrace(ctx uintptr, s engine.PerfSnapshot, w, h float64, flipped bool) {
	byColor := map[driver.Color][]apple.CGRect{}
	var order []driver.Color
	for _, b := range engine.PerfTrace(s, w, h) {
		y := b.Y
		if !flipped {
			y = h - b.Y - b.H
		}
		if _, ok := byColor[b.Color]; !ok {
			order = append(order, b.Color)
		}
		byColor[b.Color] = append(byColor[b.Color], apple.Rect(b.X, y, b.W, b.H))
	}
	for _, c := range order {
		color := apple.CGColor(c)
		apple.FillRects(ctx, byColor[c], color)
		apple.Release(color)
	}
}
