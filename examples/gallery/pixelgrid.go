package main

import (
	"math"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/ui"
)

// pixelGridSpacing is the distance between a PixelGrid's lines.
const pixelGridSpacing = 22

// PixelGrid is a box of Children on Onevra's line grid, turned 45° about
// its center: the page's background shows through, so a component looks
// as it does in an app. Only the grid clips to the box.
type PixelGrid struct {
	TestID   string
	Style    core.Style
	Children []core.Node
}

// Render implements core.Node.
func (g PixelGrid) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	draw := func(d *core.Drawing, size core.Size) {
		w, h := float64(size.Width), float64(size.Height)
		lines := int(math.Ceil(math.Hypot(w, h) / 2 / pixelGridSpacing))
		reach := float32(lines * pixelGridSpacing)
		d.Translate(float32(w/2), float32(h/2)).Rotate(math.Pi / 4).BeginPath()
		for i := -lines; i <= lines; i++ {
			at := float32(i * pixelGridSpacing)
			d.MoveTo(at, -reach).LineTo(at, reach).MoveTo(-reach, at).LineTo(reach, at)
		}
		d.SetStrokeColor(ui.Alpha(colors.Text(), 0.07)).SetLineWidth(1).Stroke()
	}
	grid := core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0), Overflow: core.Hidden},
		Children: []core.Node{core.Canvas{Style: core.Style{FlexGrow: 1}, Draw: draw}}}
	return core.View{TestID: g.TestID, Style: g.Style, StyleFunc: ui.Filled(colors.Background), Children: append([]core.Node{grid}, g.Children...)}
}
