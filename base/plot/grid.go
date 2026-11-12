package plot

import (
	"math"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// Grid is lines across a Plot, as gpui-base's Grid: vertical at the
// values X of its x scale (row indices on a point scale) and horizontal at
// the values Y of the y scale Scale names, from edge to edge, each on whole
// device pixels so a thin line stays sharp. With neither X nor Y, it
// follows the ticks of the scale the values go along, where an Axis of that
// scale puts them: about Ticks of them, or the axis's default ones, the
// aligned step of a dual-axis chart, so one grid serves both axes. It
// strokes with its paint style (Stroke, StrokeWidth, StrokeDash; inherited
// from the plot), the text color where none is set. Its values do not count
// in the domains, and each line moves with its scale's domain.
type Grid struct {
	X, Y      func() []float64
	Scale     ValueScale
	Ticks     int
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
}

// Render implements core.Node.
func (g Grid) Render(cx *core.Scope) core.Node {
	plot := plotOf(cx, "Grid")
	values := func(f func() []float64) []float64 {
		if f == nil {
			return nil
		}
		return f()
	}
	return markLayer(parts.Of(g.Class, g.Style, g.StyleFunc), g.TestID, markCanvas(plot, room, part{},
		func(d *core.Drawing, plot *plotState, _ float32) {
			d.StrokeFromView(core.PaintCurrentColor).LineWidthFromView().LineDashFromView().BeginPath()
			if plot == nil {
				return
			}
			size := plot.size.Get()
			w, h := float32(size.Width), float32(size.Height)
			xs, ys := values(g.X), values(g.Y)
			// Without values, the ticks along the values' axis.
			if g.X == nil && g.Y == nil {
				rows := plot.rows()
				if plot.bandAxis.Get() == axisY {
					xs = tickValues(plot.xMap.Get(), rows, g.Ticks)
				} else {
					ys = tickValues(plot.yMapOf(g.Scale), rows, g.Ticks)
				}
			}
			// A line on the middle of a device pixel row or column: a
			// hairline 1 point wide on a 1x screen is one pixel, not two
			// halves.
			ratio := plot.ratio()
			sharp := func(v float32) float32 { return (float32(math.Floor(float64(v*ratio))) + 0.5) / ratio }
			xMap, yMap := plot.xMap.Get(), plot.yMapOf(g.Scale)
			plot.traced(d, true, noScale, func() {
				for _, v := range xs {
					if at, ok := xMap.tick(v); ok {
						d.MoveTo(sharp(at), 0).LineTo(sharp(at), h)
					}
				}
			})
			plot.traced(d, false, g.Scale, func() {
				for _, v := range ys {
					if at, ok := yMap.tick(v); ok {
						d.MoveTo(0, sharp(at)).LineTo(w, sharp(at))
					}
				}
			})
			d.Stroke()
		}))
}
