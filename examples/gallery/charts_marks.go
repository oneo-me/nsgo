package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/base/plot"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// chartMotion is the charts' motion: what the marks draw in and move with,
// and the hover's fade and glide.
func chartMotion() plot.Motion {
	ease := core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}
	return plot.Motion{
		Appear:  &core.TimingConfig{Duration: 600 * time.Millisecond, Easing: ease},
		Update:  &core.TimingConfig{Duration: 400 * time.Millisecond, Easing: ease},
		Enter:   &core.TimingConfig{Duration: 150 * time.Millisecond},
		Exit:    &core.TimingConfig{Duration: 150 * time.Millisecond},
		Pointer: &core.SpringConfig{Stiffness: 320, Damping: 32},
	}
}

// chartPalette is the series' colors.
func chartPalette(cx *core.Scope) []core.Color {
	return []core.Color{ToneAccent.Color(cx), ToneInfo.Color(cx), ToneUp.Color(cx), ToneWarn.Color(cx), ToneDanger.Color(cx)}
}

// axisLabel is an axis's labels' look.
func axisLabel(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{FontSize: 11, Color: colors.Muted(), FontVariant: core.FontTabularNums}
	}
}

// gridLine is a chart's grid lines' look.
func gridLine(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Line())} }
}

// chartTip is a tooltip showing what text says.
func chartTip(cx *core.Scope, text func() string) core.Node {
	fin, colors := ui.FinOf(cx), ui.ColorsOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return fin.Menu().Over(core.Style{Padding: core.Symmetric(4, 8), CornerRadius: 8, BorderWidth: 1})
	}, Children: []core.Node{ui.Tabular(ui.Textf(text, 12, ui.Regular, colors.Text))}}
}

// newDataButton is a button that moves a chart to its other rows.
func newDataButton(cx *core.Scope, press func()) core.Node {
	return ui.Button{Text: locales.For(cx).ChartsNewData(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: press}
}

var (
	barQuarters = []string{"Q1", "Q2", "Q3", "Q4"}
	barRows     = [][][]float64{
		{{32, 41, 38, 52}, {18, 22, 27, 25}, {44, 39, 51, 58}},
		{{28, 45, 49, 40}, {24, 19, 21, 33}, {36, 47, 42, 61}},
	}
)

// barsCard stacks two series beside a third, each in its category's color
// of an ordinal scale, and moves to new rows with the domain.
func barsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	period := core.NewSignal(cx, 0)
	hover := core.NewSignal(cx, -1)
	rows := func(i int) func() []float64 { return func() []float64 { return barRows[period.Get()][i] } }
	same := func(name string) func() []string {
		return func() []string { return strings.Split(strings.Repeat(name+",", len(barQuarters)-1)+name, ",") }
	}
	label := func() []string {
		out := make([]string, len(barQuarters))
		for i, v := range barRows[period.Get()][2] {
			out[i] = fmt.Sprintf("%.0f", v)
		}
		return out
	}
	return Card{Title: tr.ChartsBars(), Description: tr.ChartsBarsNote(), Actions: []core.Node{
		newDataButton(cx, func() { period.Set(1 - period.GetUntracked()) })}, Children: []core.Node{
		plot.Plot{
			TestID: "bars-plot", Label: tr.ChartsBars(), Count: func() int { return len(barQuarters) },
			XScale: func() plot.Scale {
				return plot.BandScale(plot.Band{Domain: barQuarters, PaddingInner: 0.3, PaddingOuter: 0.15, GroupPadding: 0.1})
			},
			ColorScale: func() plot.ColorScale {
				return plot.OrdinalColors([]string{"web", "mobile", "desktop"}, chartPalette(cx)...)
			},
			AppearGeneration: func() any { return "bars" },
			Motion:           chartMotion(),
			CrossLineStyleFunc: func() core.Style {
				return core.Style{Width: core.Pct(100), Background: ui.Alpha(colors.Text(), 0.06), CornerRadius: 4}
			},
			Tooltip: chartTip(cx, func() string { return barTip(hover.Get(), period.Get()) }),
			OnHoverChange: func(i int) {
				if i >= 0 {
					hover.Set(i)
				}
			},
			Style: core.Style{Height: core.Pt(180), Margin: core.Edges{Left: 28, Bottom: 22, Top: 8}},
			Children: []core.Node{
				plot.Grid{StyleFunc: gridLine(cx), Ticks: 4},
				plot.Bar{Value: rows(0), Stack: "stack", Color: same("web"), Stagger: 0.4, HoverDim: 0.4},
				plot.Bar{Value: rows(1), Stack: "stack", Color: same("mobile"), Stagger: 0.4, HoverDim: 0.4},
				plot.Bar{Value: rows(2), Color: same("desktop"), Corners: plot.Corners{TopLeft: 3, TopRight: 3}, Stagger: 0.4, HoverDim: 0.4,
					Label: label, LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Bottom: 2}}},
				plot.Axis{Placement: plot.AxisLeft, Ticks: 4, LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Right: 6}}},
				plot.Axis{LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Top: 4}}},
			},
		},
	}}
}

func barTip(row, period int) string {
	if row < 0 {
		row = 0
	}
	r := barRows[period]
	return fmt.Sprintf("%s  %.0f · %.0f · %.0f", barQuarters[row], r[0][row], r[1][row], r[2][row])
}

// bridgeSteps are a quarter's changes, each row a base of its own: where the
// last one ended.
var bridgeSteps = []struct {
	name  string
	delta float64
}{{"Start", 120}, {"New", 38}, {"Upsell", 21}, {"Churn", -27}, {"FX", -9}}

// bridgeCard is a waterfall, a bar per row from its base to its value, with
// a text at each step's end and a rule to the final total.
func bridgeCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	bases, ends, names, labels := make([]float64, len(bridgeSteps)), make([]float64, len(bridgeSteps)), make([]string, len(bridgeSteps)), make([]string, len(bridgeSteps))
	kinds := make([]string, len(bridgeSteps))
	at := 0.0
	for i, s := range bridgeSteps {
		bases[i], ends[i], names[i] = at, at+s.delta, s.name
		at += s.delta
		labels[i], kinds[i] = fmt.Sprintf("%+.0f", s.delta), "up"
		if s.delta < 0 {
			kinds[i] = "down"
		}
	}
	bases[0], labels[0], kinds[0] = 0, "120", "total"
	return Card{Title: tr.ChartsBridge(), Description: tr.ChartsBridgeNote(), Children: []core.Node{plot.Plot{
		TestID: "bridge-plot", Label: tr.ChartsBridge(), Count: func() int { return len(bridgeSteps) },
		XScale: func() plot.Scale {
			return plot.BandScale(plot.Band{Domain: names, PaddingInner: 0.35, PaddingOuter: 0.2})
		},
		YScale: func() plot.Scale { return plot.LinearScale(0, 200) },
		ColorScale: func() plot.ColorScale {
			return plot.OrdinalColors([]string{"total", "up", "down"}, ToneInfo.Color(cx), ToneUp.Color(cx), ToneDanger.Color(cx))
		},
		AppearGeneration: func() any { return "bridge" },
		Motion:           chartMotion(),
		Style:            core.Style{Height: core.Pt(160), Margin: core.Edges{Left: 28, Bottom: 22, Top: 8}},
		Children: []core.Node{
			plot.Grid{StyleFunc: gridLine(cx), Ticks: 4},
			plot.Bar{Bases: func() []float64 { return bases }, Value: func() []float64 { return ends }, Color: func() []string { return kinds },
				Corners: plot.AllCorners(2), Stagger: 0.6, HoverDim: 0.5, Label: func() []string { return labels },
				LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Bottom: 2}}},
			plot.Axis{Placement: plot.AxisLeft, Ticks: 4, LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Right: 6}}},
			plot.Axis{LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Top: 4}}},
		},
	}}}
}

// ohlc is a session of prices.
type ohlc struct{ open, high, low, close, volume float64 }

var ohlcRows = func() []ohlc {
	price, s := 172.0, 11.0
	next := func() float64 {
		s = math.Mod(s*16807, 2147483647)
		return math.Mod(s, 1000)/1000 - 0.47
	}
	out := make([]ohlc, 28)
	for i := range out {
		open, shut := price, price+next()*4
		out[i] = ohlc{open, max(open, shut) + math.Abs(next())*2, min(open, shut) - math.Abs(next())*2, shut, 40 + (next()+0.5)*220}
		price = shut
	}
	return out
}()

// ohlcColumn reads a column of the ohlcRows.
func ohlcColumn(f func(ohlc) float64) func() []float64 {
	return func() []float64 {
		out := make([]float64, len(ohlcRows))
		for i, c := range ohlcRows {
			out[i] = f(c)
		}
		return out
	}
}

// candlesCard puts volumes on a second scale under ohlcRows, a rule from the
// low to the high under a bar from the open to the close, the two scales'
// ticks aligned so one grid serves both axes, with a label on the last
// close. Its plot takes the keys: Left and Right step through the sessions.
func candlesCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	hover := core.NewSignal(cx, len(ohlcRows)-1)
	dir := func() []string {
		out := make([]string, len(ohlcRows))
		for i, c := range ohlcRows {
			out[i] = "up"
			if c.close < c.open {
				out[i] = "down"
			}
		}
		return out
	}
	last := len(ohlcRows) - 1
	return Card{Title: tr.ChartsCandles(), Description: tr.ChartsCandlesNote(), Children: []core.Node{plot.Plot{
		TestID: "ohlcRows-plot", Label: tr.ChartsCandles(), Count: func() int { return len(ohlcRows) },
		XScale: func() plot.Scale { return plot.BandScale(plot.Band{PaddingInner: 0.3}) },
		ColorScale: func() plot.ColorScale {
			return plot.OrdinalColors([]string{"up", "down"}, ToneUp.Color(cx), ToneDanger.Color(cx))
		},
		AppearGeneration: func() any { return "ohlcRows" },
		Motion:           chartMotion(),
		CrossLineStyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(1), Background: ui.Alpha(colors.Text(), 0.3)}
		},
		Tooltip: chartTip(cx, func() string {
			c := ohlcRows[hover.Get()]
			return fmt.Sprintf("O %.2f  H %.2f  L %.2f  C %.2f", c.open, c.high, c.low, c.close)
		}),
		TooltipPlacement: plot.TooltipPointer, TooltipOffset: 8,
		OnHoverChange: func(i int) {
			if i >= 0 {
				hover.Set(i)
			}
		},
		Style: core.Style{Height: core.Pt(200), Margin: core.Edges{Left: 34, Right: 34, Bottom: 8, Top: 8}},
		Children: []core.Node{
			plot.Grid{StyleFunc: gridLine(cx)},
			// A thick rule keeps the volumes in the middle of the bands, under
			// the candles; a bar would stand beside them.
			plot.Rule{Value: ohlcColumn(func(c ohlc) float64 { return c.volume }), Scale: plot.ScaleY2,
				StyleFunc: func() core.Style {
					return core.Style{Stroke: core.PaintColor(ui.Alpha(colors.Muted(), 0.25)), StrokeWidth: 7}
				}},
			plot.Rule{Value: ohlcColumn(func(c ohlc) float64 { return c.high }), Bases: ohlcColumn(func(c ohlc) float64 { return c.low }),
				Color: dir, Stagger: 0.5},
			plot.Bar{Value: ohlcColumn(func(c ohlc) float64 { return c.close }), Bases: ohlcColumn(func(c ohlc) float64 { return c.open }),
				Color: dir, Stagger: 0.5},
			plot.Axis{Placement: plot.AxisLeft, LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Right: 6}}},
			plot.Axis{Placement: plot.AxisRight, Scale: plot.ScaleY2, LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Left: 6}}},
			plot.Label{Texts: func() []plot.PlotText {
				return []plot.PlotText{{Text: fmt.Sprintf("%.2f", ohlcRows[last].close), X: float64(last), Y: ohlcRows[last].close, Align: plot.TextRight,
					Style: core.Style{Margin: core.Edges{Right: 6}}}}
			}, StyleFunc: axisLabel(cx)},
		},
	}}}
}

var (
	shareNames = []string{"Direct", "Search", "Social", "Email", "Referral"}
	shareRows  = [][]float64{{42, 28, 14, 10, 6}, {30, 34, 20, 6, 10}}
)

// shareCard is a donut whose values move with the plot's update, the slice
// under the pointer lifted and the others dimmed, with labels on leader
// lines, and the row under the pointer in its hole.
func shareCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	period := core.NewSignal(cx, 0)
	hover := core.NewSignal(cx, -1)
	rows := func() []float64 { return shareRows[period.Get()] }
	center := func() string {
		if h := hover.Get(); h >= 0 {
			return fmt.Sprintf("%s %.0f%%", shareNames[h], rows()[h])
		}
		return ""
	}
	labels := func() []string {
		out := make([]string, len(shareNames))
		for i, n := range shareNames {
			out[i] = fmt.Sprintf("%s %.0f%%", n, rows()[i])
		}
		return out
	}
	return Card{Title: tr.ChartsShare(), Description: tr.ChartsShareNote(), Actions: []core.Node{
		newDataButton(cx, func() { period.Set(1 - period.GetUntracked()) })}, Children: []core.Node{plot.Plot{
		TestID: "share-plot", Label: tr.ChartsShare(), Y: rows,
		ColorScale:       func() plot.ColorScale { return plot.OrdinalColors(shareNames, chartPalette(cx)...) },
		AppearGeneration: func() any { return "share" },
		Motion:           chartMotion(),
		OnHoverChange:    hover.Set,
		Style:            core.Style{Height: core.Pt(220), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Children: []core.Node{
			plot.Pie{Color: func() []string { return shareNames }, InnerRadius: func() core.Length { return core.Pct(62) },
				OuterRadius: func() core.Length { return core.Pct(72) }, PadAngle: func() float32 { return 0.03 }, CornerRadius: 3,
				HoverLift: 5, HoverDim: 0.35, Label: labels, LabelStyleFunc: axisLabel(cx), LabelGap: 12,
				LabelLineStyleOf: func(int) core.Style { return core.Style{Stroke: core.PaintColor(colors.Border())} }},
			core.Text{ContentFunc: center, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Text), PointerEvents: core.PointerEventsNone},
		},
	}}}
}

// gaugeCard is two arcs: a track, and a value that sweeps round to its new
// end on the plot's update.
func gaugeCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	load := core.NewSignal(cx, 0.62)
	const start, turn = float32(-2.2), float32(4.4)
	inner, outer := func() core.Length { return core.Pct(78) }, func() core.Length { return core.Pct(100) }
	return Card{Title: tr.ChartsGauge(), Description: tr.ChartsGaugeNote(), Actions: []core.Node{
		newDataButton(cx, func() {
			next := load.GetUntracked() + 0.27
			load.Set(next - math.Floor(next))
		})}, Children: []core.Node{plot.Plot{
		TestID: "gauge-plot", Label: tr.ChartsGauge(), NoHover: true, AppearGeneration: func() any { return "gauge" }, Motion: chartMotion(),
		Style: core.Style{Height: core.Pt(140), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Children: []core.Node{
			plot.Arc{StartAngle: func() float32 { return start }, EndAngle: func() float32 { return start + turn }, InnerRadius: inner, OuterRadius: outer,
				CornerRadius: 8, StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(colors.Line())} }},
			plot.Arc{StartAngle: func() float32 { return start }, EndAngle: func() float32 { return start + turn*float32(load.Get()) }, InnerRadius: inner, OuterRadius: outer,
				CornerRadius: 8, StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(ToneAccent.Color(cx))} }},
			ui.Tabular(ui.Textf(func() string { return fmt.Sprintf("%.0f%%", load.Get()*100) }, 20, ui.Medium, colors.Text)),
		},
	}}}
}

// bubbles are companies: a return, a risk and a size.
var bubbles = func() (out [][3]float64) {
	s := 29.0
	next := func() float64 {
		s = math.Mod(s*16807, 2147483647)
		return math.Mod(s, 1000) / 1000
	}
	for range 24 {
		out = append(out, [3]float64{next()*30 + 4, next()*22 - 6, next() * next() * 900})
	}
	return out
}()

func bubbleColumn(i int) func() []float64 {
	return func() []float64 {
		out := make([]float64, len(bubbles))
		for row, b := range bubbles {
			out[row] = b[i]
		}
		return out
	}
}

// scatterCard is a bubble chart on two linear scales, the dots' areas in
// proportion to their sizes, the one nearest the pointer ringed.
func scatterCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	hover := core.NewSignal(cx, 0)
	return Card{Title: tr.ChartsScatter(), Description: tr.ChartsScatterNote(), Children: []core.Node{plot.Plot{
		TestID: "scatter-plot", Label: tr.ChartsScatter(), X: bubbleColumn(0), Y: bubbleColumn(1),
		XScale: func() plot.Scale { return plot.LinearScale(0, 40) }, YScale: func() plot.Scale { return plot.LinearScale(-10, 20) },
		AppearGeneration: func() any { return "scatter" }, Motion: chartMotion(),
		DotStyleFunc: func() core.Style { return core.Style{CornerRadius: 999, BorderWidth: 2, BorderColor: colors.Text()} },
		Tooltip: chartTip(cx, func() string {
			b := bubbles[hover.Get()]
			return fmt.Sprintf("%.1f, %.1f · %.0f", b[0], b[1], b[2])
		}), TooltipPlacement: plot.TooltipPointer, TooltipOffset: 10,
		OnHoverChange: func(i int) {
			if i >= 0 {
				hover.Set(i)
			}
		},
		Style: core.Style{Height: core.Pt(200), Margin: core.Edges{Left: 28, Bottom: 22, Top: 8, Right: 8}},
		Children: []core.Node{
			plot.Grid{StyleFunc: gridLine(cx), X: func() []float64 { return []float64{0, 10, 20, 30, 40} }, Y: func() []float64 { return []float64{-10, 0, 10, 20} }},
			plot.Dot{Sizes: bubbleColumn(2), RScale: &plot.RadiusScale{Min: 2, Max: 16}, Stagger: 0.7, HoverDim: 0.5,
				StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(ui.Alpha(ToneAccent.Color(cx), 0.6))} }},
			plot.Axis{Placement: plot.AxisLeft, TickValues: func() []float64 { return []float64{-10, 0, 10, 20} }, LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Right: 6}}},
			plot.Axis{Ticks: 5, LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Top: 4}}},
		},
	}}}
}

var (
	heatMonths = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	heatYears  = []string{"2022", "2023", "2024", "2025"}
)

// heatCell is a month's return.
func heatCell(year, month int) float64 {
	return math.Round((math.Sin(float64(year*7+month)*1.3)*4.2+math.Cos(float64(month*3-year))*2.1)*10) / 10
}

// heatmapCard is a calendar of returns: a cell per month on two band scales
// of categories, a diverging ramp about zero, the value written where it
// fits and light on dark cells. The keys step across the grid.
func heatmapCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	n := len(heatMonths) * len(heatYears)
	xs, ys, values, labels := make([]string, n), make([]string, n), make([]float64, n), make([]string, n)
	for y := range heatYears {
		for m := range heatMonths {
			i := y*len(heatMonths) + m
			xs[i], ys[i], values[i] = heatMonths[m], heatYears[y], heatCell(y, m)
			labels[i] = fmt.Sprintf("%.1f", values[i])
		}
	}
	band := func(domain []string) func() plot.Scale {
		return func() plot.Scale { return plot.BandScale(plot.Band{Domain: domain, PaddingInner: 0.08}) }
	}
	return Card{Title: tr.ChartsHeatmap(), Description: tr.ChartsHeatmapNote(), Children: []core.Node{plot.Plot{
		TestID: "heatmap-plot", Label: tr.ChartsHeatmap(), Count: func() int { return n },
		XScale: band(heatMonths), YScale: band(heatYears),
		ColorScale: func() plot.ColorScale {
			return plot.DivergingColors(ToneDanger.Color(cx), colors.Surface(), ToneUp.Color(cx))
		},
		AppearGeneration: func() any { return "heatmap" }, Motion: chartMotion(),
		Style: core.Style{Height: core.Pt(150), Margin: core.Edges{Left: 34, Bottom: 22}},
		Children: []core.Node{
			plot.Cell{X: func() []string { return xs }, Y: func() []string { return ys }, Value: func() []float64 { return values },
				Corners: plot.AllCorners(3), Stagger: 0.6, HoverDim: 0.3, Label: func() []string { return labels },
				LabelStyle: core.Style{FontSize: 11, FontVariant: core.FontTabularNums}, DarkLabelStyle: core.Style{Color: core.RGB(0xffffff)},
				HoverCellStyleFunc: func() core.Style {
					return core.Style{CornerRadius: 3, BorderWidth: 2, BorderColor: colors.Text()}
				}},
			plot.Axis{Placement: plot.AxisLeft, LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Right: 6}}},
			plot.Axis{LabelStyleFunc: axisLabel(cx), LabelStyle: core.Style{Padding: core.Edges{Top: 4}}},
		},
	}}}
}

var (
	radarAxes = []string{"Speed", "Reach", "Depth", "Cost", "Care", "Trust"}
	radarRows = [][]float64{{72, 58, 81, 44, 66, 90}, {48, 83, 52, 76, 88, 61}}
)

// radarCard is two closed radial lines over rings and spokes, moved to new
// rows with the plot's update.
func radarCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	flip := core.NewSignal(cx, 0)
	outer := func() core.Length { return core.Pct(92) }
	rows := func(i int) func() []float64 { return func() []float64 { return radarRows[(i+flip.Get())%2] } }
	hover := core.NewSignal(cx, 0)
	line := func(tone Tone, i int) core.Node {
		return plot.RadialLine{Radius: rows(i), Closed: true, OuterRadius: outer,
			StyleFunc: func() core.Style {
				c := tone.Color(cx)
				return core.Style{Stroke: core.PaintColor(c), StrokeWidth: 1.5, Fill: core.PaintColor(ui.Alpha(c, 0.14))}
			},
			HoverDotStyleFunc: func() core.Style { return core.Style{Background: tone.Color(cx)} }}
	}
	return Card{Title: tr.ChartsRadar(), Description: tr.ChartsRadarNote(), Actions: []core.Node{
		newDataButton(cx, func() { flip.Set(1 - flip.GetUntracked()) })}, Children: []core.Node{plot.Plot{
		TestID: "radar-plot", Label: tr.ChartsRadar(), Count: func() int { return len(radarAxes) },
		YScale:           func() plot.Scale { return plot.LinearScale(0, 100) },
		AppearGeneration: func() any { return "radar" }, Motion: chartMotion(),
		DotStyle: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4},
		Tooltip:  chartTip(cx, func() string { return radarAxes[hover.Get()] }),
		OnHoverChange: func(i int) {
			if i >= 0 {
				hover.Set(i)
			}
		},
		Style: core.Style{Height: core.Pt(220)},
		Children: []core.Node{
			plot.RadialGrid{Rings: func() []float64 { return []float64{25, 50, 75, 100} }, Spokes: true, StyleFunc: gridLine(cx), OuterRadius: outer},
			plot.RadialGrid{Rings: func() []float64 { return []float64{100} }, Circular: true, StyleFunc: gridLine(cx), OuterRadius: outer},
			line(ToneAccent, 0), line(ToneInfo, 1),
		},
	}}}
}

var (
	flowNames = []string{"Ads", "Search", "Referral", "Trial", "Signup", "Paid", "Team", "Left"}
	flowRows  = [][]plot.SankeyLink{
		{{Source: 0, Target: 3, Value: 40}, {Source: 1, Target: 3, Value: 55}, {Source: 1, Target: 4, Value: 30}, {Source: 2, Target: 4, Value: 25},
			{Source: 3, Target: 5, Value: 45}, {Source: 3, Target: 7, Value: 50}, {Source: 4, Target: 5, Value: 30}, {Source: 4, Target: 6, Value: 25}},
		{{Source: 0, Target: 3, Value: 25}, {Source: 1, Target: 3, Value: 70}, {Source: 1, Target: 4, Value: 18}, {Source: 2, Target: 4, Value: 42},
			{Source: 3, Target: 5, Value: 62}, {Source: 3, Target: 7, Value: 33}, {Source: 4, Target: 5, Value: 20}, {Source: 4, Target: 6, Value: 40}},
	}
)

// flowsCard is a sankey whose nodes take their tones, its ribbons a
// gradient from their source's color to their target's.
func flowsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	period := core.NewSignal(cx, 0)
	shade := func(row int) core.Color {
		palette := chartPalette(cx)
		return palette[row%len(palette)]
	}
	links := func() []plot.SankeyLink { return flowRows[period.Get()] }
	return Card{Title: tr.ChartsFlows(), Description: tr.ChartsFlowsNote(), Actions: []core.Node{
		newDataButton(cx, func() { period.Set(1 - period.GetUntracked()) })}, Children: []core.Node{plot.Plot{
		TestID: "flows-plot", Label: tr.ChartsFlows(), Count: func() int { return len(flowNames) },
		AppearGeneration: func() any { return "flows" }, Motion: chartMotion(),
		Style: core.Style{Height: core.Pt(260), Margin: core.Edges{Left: 56, Right: 48, Top: 36}},
		Children: []core.Node{plot.Sankey{Links: links, NodeWidth: 10, NodePadding: 40, NodeCornerRadius: 2, HoverDim: 0.7,
			NodeStyleOf: func(row int) core.Style { return core.Style{Fill: core.PaintColor(shade(row))} },
			LinkBackgroundStyleOf: func(link int) core.Style {
				l := links()[link]
				return core.Style{Gradients: []core.Gradient{{Angle: 90, Stops: []core.ColorStop{
					{Color: ui.Alpha(shade(l.Source), 0.35)}, {Color: ui.Alpha(shade(l.Target), 0.35)}}}}}
			},
			NodeLabel:  func() []string { return flowNames },
			ValueLabel: func(_ int, v float64) string { return fmt.Sprintf("%.0f", v) },
			NodeLabelStyleFunc: func() core.Style {
				s := axisLabel(cx)()
				s.Padding = core.All(4)
				return s
			}}},
	}}}
}

// holding is a row of the market: a sector has no parent and takes the sum
// of its stocks.
type holding struct {
	id, sector string
	cap, move  float64
}

var (
	marketRows = []holding{{id: "Tech"}, {id: "Energy"}, {id: "Health"},
		{"NSGO", "Tech", 310, 2.4}, {"GPUI", "Tech", 190, -1.1}, {"TAFY", "Tech", 120, 0.6}, {"QJSX", "Tech", 70, -3.2},
		{"OILS", "Energy", 160, -0.8}, {"WIND", "Energy", 90, 1.9}, {"GRID", "Energy", 40, 0.2},
		{"CURE", "Health", 140, 3.1}, {"LABS", "Health", 80, -2.2}, {"VAXX", "Health", 50, 0.9}}
	marketDays = []float64{1, 1.6}
)

func marketColumn[T any](f func(holding) T) func() []T {
	return func() []T {
		out := make([]T, len(marketRows))
		for i, s := range marketRows {
			out[i] = f(s)
		}
		return out
	}
}

// marketCard is a treemap of sectors and their stocks, shaded by the day's
// change along a diverging ramp, the sectors named in bands of their own.
func marketCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	day := core.NewSignal(cx, 0)
	// Another day moves the money between the sectors.
	caps := func() []float64 {
		d := day.Get()
		out := make([]float64, len(marketRows))
		for i, s := range marketRows {
			out[i] = s.cap
			if s.sector == "Tech" {
				out[i] = s.cap / marketDays[d]
			}
		}
		return out
	}
	return Card{Title: tr.ChartsMarket(), Description: tr.ChartsMarketNote(), Actions: []core.Node{
		newDataButton(cx, func() { day.Set(1 - day.GetUntracked()) })}, Children: []core.Node{plot.Plot{
		TestID: "market-plot", Label: tr.ChartsMarket(), Count: func() int { return len(marketRows) },
		ColorScale: func() plot.ColorScale {
			return plot.DivergingColors(ToneDanger.Color(cx), colors.Muted(), ToneUp.Color(cx)).Over(-4, 4)
		},
		AppearGeneration: func() any { return "market" }, Motion: chartMotion(),
		Style: core.Style{Height: core.Pt(240)},
		Children: []core.Node{plot.Treemap{
			ID: marketColumn(func(s holding) string { return s.id }), Parent: marketColumn(func(s holding) string { return s.sector }),
			Value: caps, ColorValue: marketColumn(func(s holding) float64 {
				if s.sector == "" {
					return math.NaN()
				}
				return s.move
			}),
			PaddingInner: []core.Points{4, 2}, PaddingTop: []core.Points{18}, Corners: plot.AllCorners(3), Stagger: 0.4, HoverDim: 0.3,
			Label:      marketColumn(func(s holding) string { return s.id }),
			ValueLabel: func(row int, _ float64) string { return fmt.Sprintf("%+.1f%%", marketRows[row].move) },
			GroupLabel: marketColumn(func(s holding) string { return s.id }),
			LabelStyle: core.Style{Padding: core.All(5), FontSize: 11}, ValueLabelStyle: core.Style{FontSize: 11},
			DarkLabelStyle:  core.Style{Color: core.RGB(0xffffff)},
			GroupLabelStyle: core.Style{FontSize: 11, Padding: core.Edges{Left: 2}},
			GroupStyle:      core.Style{Fill: core.PaintNone},
			HoverStyleFunc: func() core.Style {
				return core.Style{CornerRadius: 3, BorderWidth: 2, BorderColor: colors.Text()}
			},
		}},
	}}}
}
