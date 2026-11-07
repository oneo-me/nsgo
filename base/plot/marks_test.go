package plot_test

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base/plot"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

// desktop has Tab move the keyboard focus.
var desktop = nsgotest.Environment(driver.Environment{OS: driver.OSMacOS, PixelRatio: 2})

func col(v ...float64) func() []float64 { return func() []float64 { return v } }

func strs(v ...string) func() []string { return func() []string { return v } }

func frame(p plot.Plot, id string) core.Node {
	p.TestID = id
	if p.Style.Width.IsAuto() {
		p.Style = core.Style{Width: core.Pt(200), Height: core.Pt(100)}
	}
	return core.View{Style: core.Style{Padding: core.All(20)}, Children: []core.Node{p}}
}

func rects(ops []driver.DrawOp) []driver.DrawOp {
	var out []driver.DrawOp
	for _, op := range ops {
		if op.Kind == driver.DrawRect {
			out = append(out, op)
		}
	}
	return out
}

func TestBarsStandInBands(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:      col(10, 20),
			XScale: func() plot.Scale { return plot.BandScale(plot.Band{}) },
			YScale: func() plot.Scale { return plot.LinearScale(0, 40) },
			Children: []core.Node{
				plot.Bar{Stack: "s"},
				plot.Bar{Stack: "s", Value: col(10, 5)},
				plot.Bar{Value: col(40, 40), Label: strs("one", "two")},
			},
		}, "plot")
	})
	cs := canvases(app, "id:plot")
	if len(cs) != 3 {
		t.Fatalf("%d canvases", len(cs))
	}
	// A stack takes one place of two in each band, 100 wide.
	first, second := rects(cs[0].Ops), rects(cs[1].Ops)
	if len(first) != 2 || first[0].W != 50 || first[0].X != 0 || first[0].Y != 75 || first[0].H != 25 {
		t.Fatalf("first bar %+v", first)
	}
	if second[0].Y != 50 || second[0].H != 25 {
		t.Errorf("the second piles at %+v", second[0])
	}
	if third := rects(cs[2].Ops); third[1].X != 150 {
		t.Errorf("the other stands beside at %+v", third[1])
	}
	if !app.Exists("text:one") || !app.Exists("text:two") {
		t.Errorf("no labels:\n%s", app.Dump())
	}
}

func TestBarsAcrossAndHoverAlongY(t *testing.T) {
	var hovered []int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:              col(10, 20),
			YScale:         func() plot.Scale { return plot.BandScale(plot.Band{}) },
			XScale:         func() plot.Scale { return plot.LinearScale(0, 40) },
			CrossLineStyle: core.Style{Background: core.RGB(0x888888)},
			OnHoverChange:  func(i int) { hovered = append(hovered, i) },
			Children:       []core.Node{plot.Bar{}},
		}, "plot")
	})
	bars := rects(canvases(app, "id:plot")[0].Ops)
	if len(bars) != 2 || bars[1].Y != 50 || bars[1].W != 100 || bars[1].H != 50 {
		t.Fatalf("bars %+v", bars)
	}
	app.HoverAt("id:plot", core.Point{X: 10, Y: 80})
	if !slices.Equal(hovered, []int{1}) {
		t.Errorf("hover %v, want the lower band", hovered)
	}
}

func TestBarsGrowAndDim(t *testing.T) {
	var generation *core.Signal[int]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		generation = core.NewSignal(cx, 0)
		return frame(plot.Plot{
			Y:                col(10, 20),
			XScale:           func() plot.Scale { return plot.BandScale(plot.Band{}) },
			YScale:           func() plot.Scale { return plot.LinearScale(0, 20) },
			AppearGeneration: func() any { return generation.Get() },
			Motion:           plot.Motion{Appear: &core.TimingConfig{Duration: 400 * time.Millisecond}},
			Children:         []core.Node{plot.Bar{HoverDim: 0.5, Corners: plot.AllCorners(4)}},
		}, "plot")
	})
	generation.Set(1)
	app.Settle()
	mid := canvases(app, "id:plot")[0]
	if count(mid.Ops, driver.DrawArc) == 0 {
		t.Fatalf("a moving rounded bar draws no corners")
	}
	app.Advance(time.Second)
	done := canvases(app, "id:plot")[0]
	if count(done.Ops, driver.DrawAlpha) != 2 {
		t.Errorf("%d alphas, want one per bar", count(done.Ops, driver.DrawAlpha))
	}
	app.HoverAt("id:plot", core.Point{X: 150, Y: 50})
	var alphas []float32
	for _, op := range canvases(app, "id:plot")[0].Ops {
		if op.Kind == driver.DrawAlpha {
			alphas = append(alphas, op.A)
		}
	}
	if !slices.Equal(alphas, []float32{0.5, 1}) {
		t.Errorf("alphas %v, want the other row dimmed", alphas)
	}
}

func TestRuleSpansItsRow(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:        col(20, 40),
			YScale:   func() plot.Scale { return plot.LinearScale(0, 40) },
			Children: []core.Node{plot.Rule{Bases: col(10, math.NaN())}},
		}, "plot")
	})
	ops := canvases(app, "id:plot")[0].Ops
	if count(ops, driver.DrawLineTo) != 1 {
		t.Fatalf("a row without a base has a line: %d", count(ops, driver.DrawLineTo))
	}
}

func TestDotsFindThePointer(t *testing.T) {
	var hovered []int
	var rows *core.Signal[int]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		rows = core.NewSignal(cx, 0)
		return frame(plot.Plot{
			X:             col(0, 100),
			Y:             func() []float64 { return [][]float64{{0, 100}, {100, 0}}[rows.Get()] },
			XScale:        func() plot.Scale { return plot.LinearScale(0, 100) },
			YScale:        func() plot.Scale { return plot.LinearScale(0, 100) },
			Motion:        plot.Motion{Update: &core.TimingConfig{Duration: 200 * time.Millisecond}},
			OnHoverChange: func(i int) { hovered = append(hovered, i) },
			Children: []core.Node{plot.Dot{TestID: "dots", R: 10, HitRadius: 4,
				HoverDotStyle: core.Style{BorderWidth: 2}}},
		}, "plot")
	})
	arcs := func() []driver.DrawOp {
		var out []driver.DrawOp
		for _, op := range canvases(app, "id:dots")[0].Ops {
			if op.Kind == driver.DrawArc {
				out = append(out, op)
			}
		}
		return out
	}
	got := arcs()
	if len(got) != 2 || got[1].X != 200 || got[1].Y != 0 || got[1].R != 10 {
		t.Fatalf("dots %+v", got)
	}
	app.HoverAt("id:plot", core.Point{X: 195, Y: 5})
	app.HoverAt("id:plot", core.Point{X: 100, Y: 50})
	if !slices.Equal(hovered, []int{1, -1}) {
		t.Errorf("hover %v, want the dot, then none in the middle", hovered)
	}
	rows.Set(1)
	app.Advance(100 * time.Millisecond)
	if y := arcs()[1].Y; y <= 0 || y >= 100 {
		t.Errorf("a dot at %v mid-move", y)
	}
	app.Advance(time.Second)
	if y := arcs()[1].Y; y != 100 {
		t.Errorf("a dot at %v after the move", y)
	}
}

func TestCellsNameTheirBandsAndStepAsAGrid(t *testing.T) {
	var hovered []int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Count:         func() int { return 4 },
			XScale:        func() plot.Scale { return plot.BandScale(plot.Band{}) },
			YScale:        func() plot.Scale { return plot.BandScale(plot.Band{}) },
			ColorScale:    func() plot.ColorScale { return plot.SequentialColors(core.RGB(0xffffff), core.RGB(0x000000)) },
			OnHoverChange: func(i int) { hovered = append(hovered, i) },
			Children: []core.Node{
				plot.Cell{TestID: "cells", X: strs("a", "b", "a", "b"), Y: strs("x", "x", "y", "y"), Value: col(0, 1, 2, 3),
					Label: strs("0", "1", "2", "3"), DarkLabelStyle: core.Style{Color: core.RGB(0xffffff)},
					HoverCellStyle: core.Style{BorderWidth: 1}},
				plot.Axis{Placement: plot.AxisLeft},
				plot.Axis{TickStyle: core.Style{Width: core.Pt(1), Height: core.Pt(4)}},
			},
		}, "plot")
	}, desktop)
	got := rects(canvases(app, "id:cells")[0].Ops)
	if len(got) != 4 || got[3].X != 100 || got[3].Y != 50 || got[3].W != 100 {
		t.Fatalf("cells %+v", got)
	}
	if !app.Exists("text:x") || !app.Exists("text:b") {
		t.Errorf("the axes do not write the categories:\n%s", app.Dump())
	}
	app.Key("tab")
	app.Key("right")
	app.Key("down")
	app.Key("right")
	app.Key("escape")
	if !slices.Equal(hovered, []int{0, 2, 3, -1}) {
		t.Errorf("keys %v", hovered)
	}
}

func TestAxisTicksFollowTheScaleAndKeepInsideThePlot(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(30)}, Children: []core.Node{plot.Plot{
			TestID: "plot",
			Y:      col(0, 1500),
			Style:  core.Style{Width: core.Pt(200), Height: core.Pt(100)},
			Children: []core.Node{
				plot.Line{TestID: "line", Curve: plot.CurveLinear},
				plot.Axis{Placement: plot.AxisLeft, Ticks: 4, LabelClass: "text-xs", LineStyle: core.Style{Width: core.Pt(1)}},
				plot.Axis{Format: func(v float64, i int) string { return "day" + string(rune('a'+i)) }},
			},
		}}}
	})
	for _, want := range []string{"text:0", "text:500", "text:1,000", "text:1,500", "text:daya", "text:dayb"} {
		if !app.Exists(want) {
			t.Errorf("no %s:\n%s", want, app.Dump())
		}
	}
	// The y axis's labels at the plot's ends keep the range inside it.
	var first driver.DrawOp
	for _, op := range canvases(app, "id:line")[0].Ops {
		if op.Kind == driver.DrawMoveTo {
			first = op
		}
	}
	if first.Y >= 100 {
		t.Errorf("the line starts at %v, on the edge the label passes", first.Y)
	}
}

func TestSecondScaleAlignsItsTicks(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y: col(0, 280e6),
			Children: []core.Node{
				plot.Grid{TestID: "grid"},
				plot.Line{Curve: plot.CurveLinear},
				plot.Line{TestID: "price", Curve: plot.CurveLinear, Y: col(172.4, 191.8), Scale: plot.ScaleY2},
				plot.Axis{Placement: plot.AxisRight, Scale: plot.ScaleY2, LabelPlacement: plot.LabelsInside},
			},
		}, "plot")
	})
	if !app.Exists("text:170") || !app.Exists("text:200") {
		t.Errorf("the right axis does not take the aligned steps:\n%s", app.Dump())
	}
	if n := count(canvases(app, "id:grid")[0].Ops, driver.DrawMoveTo); n != 4 {
		t.Errorf("the grid draws %d lines, want the aligned steps' 4", n)
	}
}

// The steps two aligned scales take change with the room the axes' labels
// leave, and how many labels there are changes that room: it has to settle.
func TestAlignedAxesSettle(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:     col(170.17, 179.278),
			Style: core.Style{Width: core.Pt(300), Height: core.Pt(200)},
			Children: []core.Node{
				plot.Line{Curve: plot.CurveLinear},
				plot.Line{Curve: plot.CurveLinear, Y: col(0, 265.72), Scale: plot.ScaleY2},
				plot.Axis{Placement: plot.AxisLeft, LabelStyle: core.Style{FontSize: 11}},
				plot.Axis{Placement: plot.AxisRight, Scale: plot.ScaleY2, LabelStyle: core.Style{FontSize: 11}},
			},
		}, "plot")
	})
	app.Find("id:plot")
}

func TestDomainsGlideWithUpdate(t *testing.T) {
	var top *core.Signal[float64]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		top = core.NewSignal(cx, 100.0)
		return frame(plot.Plot{
			Y:      col(0, 50),
			YScale: func() plot.Scale { return plot.LinearScale(0, top.Get()) },
			Motion: plot.Motion{Update: &core.TimingConfig{Duration: 200 * time.Millisecond}},
			Children: []core.Node{
				plot.Line{TestID: "line", Curve: plot.CurveLinear},
				plot.Label{Texts: func() []plot.PlotText { return []plot.PlotText{{Text: "peak", X: 1, Y: 50, Align: plot.TextRight}} }},
			},
		}, "plot")
	})
	transform := func() int { return count(canvases(app, "id:line")[0].Ops, driver.DrawTransform) }
	if transform() != 0 {
		t.Fatal("a line at rest records through a transform")
	}
	top.Set(200)
	app.Advance(100 * time.Millisecond)
	if transform() != 1 {
		t.Errorf("a line records no transform while its domain moves")
	}
	var scale float32
	for _, op := range canvases(app, "id:line")[0].Ops {
		if op.Kind == driver.DrawTransform {
			scale = op.Y2
		}
	}
	if scale <= 1 || scale >= 2 {
		t.Errorf("the domain shows at %v of the new one", scale)
	}
	app.Advance(time.Second)
	if transform() != 0 {
		t.Errorf("the transform stays after the move")
	}
	if !app.Exists("text:peak") {
		t.Error("no label")
	}
}

func TestReversedScalesRunBack(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:        col(0, 100),
			XScale:   func() plot.Scale { return plot.PointScale().Reversed() },
			YScale:   func() plot.Scale { return plot.LinearScale(0, 100).Reversed() },
			Children: []core.Node{plot.Line{TestID: "line", Curve: plot.CurveLinear}},
		}, "plot")
	})
	var at []driver.DrawOp
	for _, op := range canvases(app, "id:line")[0].Ops {
		if op.Kind == driver.DrawMoveTo || op.Kind == driver.DrawLineTo {
			at = append(at, op)
		}
	}
	if len(at) != 2 || at[0].X != 200 || at[0].Y != 0 || at[1].X != 0 || at[1].Y != 100 {
		t.Errorf("points %+v", at)
	}
}

func TestLinesTakeTheirCategorysColor(t *testing.T) {
	red, blue := core.RGB(0xff0000), core.RGB(0x0000ff)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:          col(0, 1),
			ColorScale: func() plot.ColorScale { return plot.OrdinalColors([]string{"a", "b"}, red, blue) },
			Children: []core.Node{
				plot.Line{TestID: "b", Color: strs("b", "b")},
				plot.Line{TestID: "own", Color: strs("a", "a"), Style: core.Style{Stroke: core.PaintColor(blue)}},
			},
		}, "plot")
	})
	colorOf := func(id string) driver.Color {
		var c driver.Color
		for _, op := range canvases(app, id)[0].Ops {
			if op.Kind == driver.DrawStrokeColor {
				c = op.Color
			}
		}
		return c
	}
	if colorOf("id:b") != blue {
		t.Errorf("category color %08x", uint32(colorOf("id:b")))
	}
	if colorOf("id:own") != blue {
		t.Errorf("the app's stroke loses: %08x", uint32(colorOf("id:own")))
	}
}

func TestKeysMoveTheHover(t *testing.T) {
	var hovered []int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:             col(1, 2, 3),
			XScale:        func() plot.Scale { return plot.PointScale().Reversed() },
			OnHoverChange: func(i int) { hovered = append(hovered, i) },
			Children:      []core.Node{plot.Line{}},
		}, "plot")
	}, desktop)
	app.Key("tab")
	// Reversed, Right moves toward the first row, from the last one.
	app.Key("left")
	app.Key("left")
	app.Key("end")
	app.Key("home")
	app.Key("escape")
	if !slices.Equal(hovered, []int{0, 1, 2, 0, -1}) {
		t.Errorf("keys %v", hovered)
	}
}

func TestPieSlicesAreCanvases(t *testing.T) {
	var hovered []int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:             col(1, 0, 1),
			Motion:        plot.Motion{Pointer: &core.SpringConfig{Stiffness: 300, Damping: 30}},
			OnHoverChange: func(i int) { hovered = append(hovered, i) },
			Children: []core.Node{plot.Pie{TestID: "pie", InnerRadius: func() core.Length { return core.Pct(50) },
				HoverLift: 4, HoverDim: 0.4, CornerRadius: 2, Label: strs("one", "", "two"),
				LabelLineStyle: core.Style{Stroke: core.PaintColor(core.RGB(0x888888))}}},
		}, "plot")
	})
	if n := len(canvases(app, "id:pie")); n != 4 {
		t.Fatalf("%d canvases, want two slices and two leader lines", n)
	}
	if !app.Exists("text:one") || !app.Exists("text:two") {
		t.Errorf("no labels:\n%s", app.Dump())
	}
	// Right of the center, in the ring.
	app.HoverAt("id:plot", core.Point{X: 135, Y: 50})
	app.HoverAt("id:plot", core.Point{X: 100, Y: 50})
	app.HoverAt("id:plot", core.Point{X: 65, Y: 50})
	if !slices.Equal(hovered, []int{0, -1, 2}) {
		t.Errorf("hover %v, want a slice, the hole, the other", hovered)
	}
}

func TestArcSweepsToItsNewValue(t *testing.T) {
	var end *core.Signal[float32]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		end = core.NewSignal[float32](cx, 1)
		return frame(plot.Plot{
			Motion: plot.Motion{Update: &core.TimingConfig{Duration: 200 * time.Millisecond}},
			Children: []core.Node{plot.Arc{TestID: "arc", EndAngle: end.Get,
				InnerRadius: func() core.Length { return core.Pt(30) }, OuterRadius: func() core.Length { return core.Pt(40) }}},
		}, "plot")
	})
	rim := func() float32 {
		for _, op := range canvases(app, "id:arc")[0].Ops {
			if op.Kind == driver.DrawArc {
				return op.A1 - op.A0
			}
		}
		return 0
	}
	if math.Abs(float64(rim()-1)) > 1e-3 {
		t.Fatalf("rim spans %v", rim())
	}
	end.Set(2)
	app.Settle()
	app.Advance(100 * time.Millisecond)
	if r := rim(); r <= 1 || r >= 2 {
		t.Errorf("mid-move rim %v", r)
	}
	// A change while it moves goes on from where it is drawn.
	end.Set(3)
	app.Settle()
	app.Advance(time.Second)
	if r := rim(); math.Abs(float64(r-3)) > 1e-3 {
		t.Errorf("rim %v after the moves", r)
	}
}

func TestRadarDrawsItsGridAndFindsItsRows(t *testing.T) {
	var hovered []int
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return frame(plot.Plot{
			Y:             col(4, 8, 6, 2),
			YScale:        func() plot.Scale { return plot.LinearScale(0, 10) },
			OnHoverChange: func(i int) { hovered = append(hovered, i) },
			DotStyle:      core.Style{Width: core.Pt(4), Height: core.Pt(4)},
			Children: []core.Node{
				plot.RadialGrid{TestID: "grid", Rings: col(5, 10), Spokes: true},
				plot.RadialLine{TestID: "line", Closed: true, Dot: true, Style: core.Style{Fill: core.PaintColor(core.RGB(0xff0000))}},
			},
		}, "plot")
	})
	grid := canvases(app, "id:grid")[0].Ops
	if n := count(grid, driver.DrawMoveTo); n != 6 {
		t.Errorf("%d subpaths, want two rings and four spokes", n)
	}
	if n := count(canvases(app, "id:line")[0].Ops, driver.DrawClosePath); n != 2 {
		t.Errorf("%d closes, want the area's and the line's", n)
	}
	app.HoverAt("id:plot", core.Point{X: 130, Y: 50})
	if !slices.Equal(hovered, []int{1}) {
		t.Errorf("hover %v, want the row to the right", hovered)
	}
}

func TestSankeyDrawsNodesAndRibbons(t *testing.T) {
	var hovered []int
	var links *core.Signal[[]plot.SankeyLink]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		links = core.NewSignalFunc(cx, []plot.SankeyLink{{Source: 0, Target: 1, Value: 5}, {Source: 0, Target: 2, Value: 5}}, nil)
		return frame(plot.Plot{
			Count:         func() int { return 3 },
			Motion:        plot.Motion{Update: &core.TimingConfig{Duration: 200 * time.Millisecond}},
			OnHoverChange: func(i int) { hovered = append(hovered, i) },
			Children: []core.Node{plot.Sankey{TestID: "sankey", Links: links.Get, NodeLabel: strs("in", "a", "b"),
				ValueLabel: func(row int, v float64) string {
					if row == 0 {
						return "ten"
					}
					return ""
				}, HoverDim: 0.5}},
		}, "plot")
	})
	if n := len(canvases(app, "id:sankey")); n != 5 {
		t.Fatalf("%d canvases, want two ribbons and three nodes", n)
	}
	if !app.Exists("text:in") || !app.Exists("text:ten") {
		t.Errorf("no labels:\n%s", app.Dump())
	}
	app.HoverAt("id:plot", core.Point{X: 12, Y: 50})
	if !slices.Equal(hovered, []int{0}) {
		t.Errorf("hover %v", hovered)
	}
	links.Set([]plot.SankeyLink{{Source: 0, Target: 1, Value: 9}, {Source: 0, Target: 2, Value: 1}})
	app.Settle()
	app.Advance(time.Second)
	if n := len(canvases(app, "id:sankey")); n != 5 {
		t.Errorf("%d canvases after a move", n)
	}
	links.Set([]plot.SankeyLink{{Source: 1, Target: 0, Value: 1}, {Source: 0, Target: 1, Value: 1}})
	app.Settle()
	if n := len(canvases(app, "id:sankey")); n != 0 {
		t.Errorf("a cycle draws %d canvases", n)
	}
}

func TestTreemapTilesItsRows(t *testing.T) {
	var hovered []int
	var share *core.Signal[float64]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		share = core.NewSignal(cx, 3.0)
		return frame(plot.Plot{
			Count:         func() int { return 4 },
			Motion:        plot.Motion{Update: &core.TimingConfig{Duration: 200 * time.Millisecond}},
			ColorScale:    func() plot.ColorScale { return plot.OrdinalColors([]string{"g"}, core.RGB(0x336699)) },
			OnHoverChange: func(i int) { hovered = append(hovered, i) },
			Children: []core.Node{plot.Treemap{TestID: "tree",
				ID: strs("g", "a", "b", "c"), Parent: strs("", "g", "g", ""),
				Value:        func() []float64 { return []float64{0, share.Get(), 1, 4} },
				Color:        strs("g", "g", "g", "g"),
				PaddingInner: []core.Points{2}, PaddingTop: []core.Points{16},
				GroupClassOf: func(int) string { return "rounded" }, GroupStyle: core.Style{Fill: core.PaintColor(core.RGB(0x222222))},
				Label: strs("", "alpha", "beta", "gamma"), GroupLabel: strs("gee", "", "", ""),
				ValueLabel: func(row int, v float64) string {
					if row == 3 {
						return "4.0"
					}
					return ""
				},
				HoverDim: 0.5, HoverStyle: core.Style{BorderWidth: 1},
			}},
		}, "plot")
	}, desktop)
	cs := canvases(app, "id:tree")
	if len(cs) != 2 {
		t.Fatalf("%d canvases, want groups under leaves", len(cs))
	}
	if n := len(rects(cs[1].Ops)); n != 3 {
		t.Errorf("%d leaves", n)
	}
	if !app.Exists("text:gamma") || !app.Exists("text:gee") {
		t.Errorf("no labels:\n%s", app.Dump())
	}
	app.HoverAt("id:plot", core.Point{X: 150, Y: 50})
	app.Key("tab")
	app.Key("home")
	if len(hovered) != 2 {
		t.Errorf("hover %v", hovered)
	}
	share.Set(7)
	app.Settle()
	app.Advance(100 * time.Millisecond)
	moving := rects(canvases(app, "id:tree")[1].Ops)
	app.Advance(time.Second)
	done := rects(canvases(app, "id:tree")[1].Ops)
	if slices.EqualFunc(moving, done, func(a, b driver.DrawOp) bool { return a.X == b.X && a.W == b.W }) {
		t.Errorf("the nodes do not move: %+v", done)
	}
}

func TestAppearScopeRemembersFinishedPlots(t *testing.T) {
	var shown, generation *core.Signal[int]
	var scope *core.Signal[string]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		shown, generation = core.NewSignal(cx, 1), core.NewSignal(cx, 0)
		scope = core.NewSignal(cx, "a")
		return plot.AppearScope{ID: func() any { return scope.Get() }, Children: []core.Node{core.Show{
			When: func() bool { return shown.Get() == 1 },
			Then: func(*core.Scope) core.Node {
				return frame(plot.Plot{
					Y:                col(1, 2),
					AppearGeneration: func() any { return generation.Get() },
					Motion:           plot.Motion{Appear: &core.TimingConfig{Duration: 200 * time.Millisecond}},
					Children:         []core.Node{plot.Line{Curve: plot.CurveLinear}},
				}, "plot")
			}}}}
	})
	clips := func() int { return count(canvases(app, "id:plot")[0].Ops, driver.DrawClip) }
	app.Advance(time.Second)
	shown.Set(0)
	app.Settle()
	shown.Set(1)
	app.Settle()
	if clips() != 0 {
		t.Error("a finished generation draws in again")
	}
	// A new generation replays, and going back to the first does too.
	generation.Set(1)
	app.Settle()
	if clips() == 0 {
		t.Error("a new generation does not draw in")
	}
	app.Advance(time.Second)
	generation.Set(0)
	app.Settle()
	if clips() == 0 {
		t.Error("the forgotten generation does not draw in")
	}
	app.Advance(time.Second)
	// A new ID forgets.
	shown.Set(0)
	app.Settle()
	scope.Set("b")
	app.Settle()
	shown.Set(1)
	app.Settle()
	if clips() == 0 {
		t.Error("a new scope ID does not forget")
	}
}

func TestOutsideAPlotAMarkDrawsNothing(t *testing.T) {
	nodes := []core.Node{plot.Bar{}, plot.Rule{}, plot.Dot{}, plot.Cell{}, plot.Pie{}, plot.Arc{}, plot.RadialLine{}, plot.RadialGrid{},
		plot.Sankey{}, plot.Treemap{}, plot.Axis{}, plot.Label{}}
	app := nsgotest.Start(t, func(*core.Scope) core.Node { return core.View{TestID: "root", Children: nodes} })
	if d := app.Dump(); strings.Contains(d, "Canvas") && len(canvases(app, "id:root")) > 2 {
		t.Errorf("marks draw outside a plot:\n%s", d)
	}
}
