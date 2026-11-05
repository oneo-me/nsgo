package plot_test

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base/plot"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

// canvases are the canvases inside what selector names, in tree order.
func canvases(app *nsgotest.App, selector string) []*nsgotest.View {
	var out []*nsgotest.View
	var walk func(v *nsgotest.View)
	walk = func(v *nsgotest.View) {
		if v.Kind == driver.KindCanvas {
			out = append(out, v)
		}
		for _, c := range v.Children {
			walk(c)
		}
	}
	walk(app.Find(selector))
	return out
}

func count(ops []driver.DrawOp, kind driver.DrawKind) int {
	n := 0
	for _, op := range ops {
		if op.Kind == kind {
			n++
		}
	}
	return n
}

func TestPlotDrawsItsMarksAndFollowsThePointer(t *testing.T) {
	var hovered []int
	var week *core.Signal[int]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		week = core.NewSignal(cx, 0)
		rows := [][]float64{{10, 20, 30, 40, 50}, {50, 40, 30, 20, 10}}
		return core.View{Style: core.Style{Padding: core.All(20)}, Children: []core.Node{
			plot.Plot{
				TestID:           "plot",
				Label:            "Visitors",
				Y:                func() []float64 { return rows[week.Get()] },
				YScale:           func() plot.Scale { return plot.LinearScale(0, 100) },
				Style:            core.Style{Width: core.Pt(200), Height: core.Pt(100)},
				CrossLineStyle:   core.Style{Width: core.Pt(1), Background: core.RGB(0x888888)},
				DotStyle:         core.Style{Width: core.Pt(8), Height: core.Pt(8)},
				Tooltip:          core.View{TestID: "tip", Style: core.Style{Width: core.Pt(60), Height: core.Pt(20)}},
				TooltipOffset:    4,
				OnHoverChange:    func(i int) { hovered = append(hovered, i) },
				AppearGeneration: week.Get,
				Motion:           plot.Motion{Appear: &core.TimingConfig{Duration: 300 * time.Millisecond}},
				Children: []core.Node{
					plot.Grid{Y: func() []float64 { return []float64{0, 50, 100} }},
					plot.Area{BackgroundStyle: core.Style{Gradients: []core.Gradient{{Angle: 180,
						Stops: []core.ColorStop{{Color: core.RGB(0x00ff00)}, {Color: core.RGBA(0x00ff00, 0)}}}}},
						HoverDotStyle: core.Style{Display: core.DisplayNone}},
					plot.Line{Curve: plot.CurveLinear, Style: core.Style{Stroke: core.PaintColor(core.RGB(0xff0000))}},
				},
			},
		}}
	})
	plot := app.Find("id:plot")
	if plot.Props.Role != core.RoleImage || plot.Props.Label != "Visitors" {
		t.Fatalf("plot role %v label %q, want one image", plot.Props.Role, plot.Props.Label)
	}
	cs := canvases(app, "id:plot")
	if len(cs) != 4 {
		t.Fatalf("%d canvases, want the grid's, the area's background and shape, the line's", len(cs))
	}
	grid, background, area, line := cs[0], cs[1], cs[2], cs[3]
	if n := count(grid.Ops, driver.DrawMoveTo); n != 3 {
		t.Errorf("grid draws %d lines, want 3", n)
	}
	if background.Style.Background != 0 || !background.Style.Effects.Empty() {
		t.Errorf("the area's background view draws its background as a box")
	}
	if count(background.Ops, driver.DrawFillGradient) != 1 {
		t.Errorf("the area's background fills no gradient inside its shape")
	}
	if count(area.Ops, driver.DrawFill) != 1 || count(area.Ops, driver.DrawStroke) != 1 {
		t.Errorf("the area does not fill its shape and stroke its edge")
	}
	// The line, linear: a move and four segments, in its own red, after
	// the appear's clip while the marks draw in.
	if n := count(line.Ops, driver.DrawLineTo); n < 4 {
		t.Errorf("line has %d segments, want 4", n)
	}
	app.Advance(time.Second)
	line = canvases(app, "id:plot")[3]
	if count(line.Ops, driver.DrawClip) != 0 {
		t.Errorf("the line still clips after the appear")
	}
	var stroke driver.Color
	for _, op := range line.Ops {
		if op.Kind == driver.DrawStrokeColor {
			stroke = op.Color
		}
	}
	if stroke != core.RGB(0xff0000) {
		t.Errorf("line strokes %08x, want its own red", uint32(stroke))
	}

	// The row nearest the pointer along x: rows are 50 points apart.
	app.HoverAt("id:plot", core.Point{X: 60, Y: 50})
	app.HoverAt("id:plot", core.Point{X: 190, Y: 50})
	app.HoverOut()
	if !slices.Equal(hovered, []int{1, 4, -1}) {
		t.Errorf("hover changes %v, want 1, 4 and none", hovered)
	}
	// The tooltip stays inside the plot at the last row: centered on the
	// cross line at 200 it would overflow by 30.
	app.HoverAt("id:plot", core.Point{X: 190, Y: 50})
	tip := app.Find("id:tip")
	if x := tip.Parent.Parent.Animated.TranslateX; math.Abs(float64(x-140)) > 0.5 {
		t.Errorf("tooltip at %v, want 140, its right edge on the plot's", x)
	}
	week.Set(1)
	app.Settle()
	if line := canvases(app, "id:plot")[3]; count(line.Ops, driver.DrawClip) == 0 {
		t.Errorf("a new generation does not draw the marks in again")
	}
}
