package main

import (
	"math"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

// gridSpacing is the distance between a grid's lines.
const gridSpacing = 22

// grid is a box of children on the gallery's stage: a line grid turned
// 45° about its center over the page's background, so a component looks
// as it does in the gallery. Only the grid clips to the box.
type grid struct {
	style    core.Style
	children []core.Node
}

func (g grid) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	draw := func(d *core.Drawing, size core.Size) {
		w, h := float64(size.Width), float64(size.Height)
		lines := int(math.Ceil(math.Hypot(w, h) / 2 / gridSpacing))
		reach := float32(lines * gridSpacing)
		d.Translate(float32(w/2), float32(h/2)).Rotate(math.Pi / 4).BeginPath()
		for i := -lines; i <= lines; i++ {
			at := float32(i * gridSpacing)
			d.MoveTo(at, -reach).LineTo(at, reach).MoveTo(-reach, at).LineTo(reach, at)
		}
		d.SetStrokeColor(ui.Alpha(colors.Text(), 0.07)).SetLineWidth(1).Stroke()
	}
	lines := core.View{PointerEvents: core.PointerEventsNone,
		Style:    core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0), Overflow: core.Hidden},
		Children: []core.Node{core.Canvas{Style: core.Style{FlexGrow: 1}, Draw: draw}}}
	return core.View{Style: g.style, StyleFunc: ui.Filled(colors.Background), Children: append([]core.Node{lines}, g.children...)}
}
