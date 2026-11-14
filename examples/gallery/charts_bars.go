package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// sale is a month's sales, in thousands, online and in store.
type sale struct {
	month         string
	online, store float64
}

var sales = []sale{{"Jan", 42, 31}, {"Feb", 51, 28}, {"Mar", 48, 36}, {"Apr", 63, 33}, {"May", 58, 40}, {"Jun", 71, 38}, {"Jul", 66, 44}}

const (
	// salesTop is the top of the y scale: a step above the highest grid
	// line, room for the labels over the stacks.
	salesTop    = 130
	salesHeight = 168
)

// rightAxis is the labels of a y axis right of a plot, as d3 writes them.
func rightAxis(cx *core.Scope, values ...float64) plot.Axis {
	return plot.Axis{Placement: plot.AxisRight, TickValues: func() []float64 { return values }, LabelStyleFunc: gutterStyle(cx, chartAxisGap+8, true)}
}

// salesCard is sales per month as bars in bands, side by side or stacked,
// growing in one after another, a value over each (the total over a
// stack), the month under the pointer a faint band over its bars while the
// others fade.
func salesCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	stacked := core.NewSignal(cx, false)
	hover := core.NewSignal(cx, -1)
	shown := func() sale {
		if h := hover.Get(); h >= 0 && h < len(sales) {
			return sales[h]
		}
		return sales[len(sales)-1]
	}
	total := func(s sale) float64 { return s.online + s.store }
	k := func(v float64) string { return fixed(v, 0) + "k" }
	best := sales[0]
	for _, s := range sales {
		if total(s) > total(best) {
			best = s
		}
	}
	labelStyle := func() core.Style {
		return core.Style{FontSize: 10.5, FontFamily: ui.Mono, Padding: core.Edges{Bottom: 3}, Color: colors.Muted(), FontVariant: core.FontTabularNums}
	}
	values := func(f func(sale) float64) func() []float64 { return func() []float64 { return column(sales, f) } }
	labels := func(f func(sale) float64) func() []string {
		return func() []string {
			out := make([]string, len(sales))
			for i, s := range sales {
				out[i] = fixed(f(s), 0)
			}
			return out
		}
	}
	// Upright bars, their tips rounded; a stack only at its top.
	tip := plot.Corners{TopLeft: 3, TopRight: 3}
	bars := core.Dynamic(func(cx *core.Scope) core.Node {
		online := plot.Bar{Value: values(func(s sale) float64 { return s.online }), Corners: tip, Stagger: 0.4, HoverDim: 0.55,
			Label: labels(func(s sale) float64 { return s.online }), LabelStyleFunc: labelStyle,
			StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(chartSeries(cx)[0])} }}
		store := plot.Bar{Value: values(func(s sale) float64 { return s.store }), Corners: tip, Stagger: 0.4, HoverDim: 0.55,
			Label: labels(func(s sale) float64 { return s.store }), LabelStyleFunc: labelStyle,
			StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(chartSeries(cx)[1])} }}
		if stacked.Get() {
			// Stacked, the top bar's label is the month's total.
			online.Stack, online.Corners, online.Label = "sales", plot.Corners{}, nil
			store.Stack, store.Label = "sales", labels(total)
		}
		return core.Group{online, store}
	})
	return Card{TestID: "card-sales", Title: tr.ChartsSales(), Description: tr.ChartsSalesNote(), Children: []core.Node{
		baseline(10, headline(cx, func() string { return k(total(shown())) }, nil),
			note(cx, func() string {
				if hover.Get() < 0 {
					return tr.ChartsTheLastMonth(shown().month)
				}
				return shown().month
			}, colors.Muted)),
		keys(8,
			chartLegend(cx, chartSeriesColor(cx, 0), constant(tr.ChartsOnline()), func() string { return k(shown().online) }),
			chartLegend(cx, chartSeriesColor(cx, 1), constant(tr.ChartsInStore()), func() string { return k(shown().store) })),
		spaced(chartGap(), plot.Plot{
			TestID: "sales-plot",
			Count:  func() int { return len(sales) },
			// Bands with half their gap at the ends: their middles divide the
			// width evenly, each month's label centered under its band.
			XScale: func() plot.Scale {
				return plot.BandScale(plot.Band{PaddingInner: 0.3, PaddingOuter: 0.15, MaxBandWidth: 56, GroupPadding: 0.12})
			},
			YScale: func() plot.Scale { return plot.LinearScale(0, salesTop) },
			AppearGeneration: func() any { return stacked.Get() },
			Motion:           chartMotion(ms(600), 0, false),
			Label:            tr.ChartsSalesLabel(best.month),
			AccessibilityValue: func() core.AccessibilityValue {
				if hover.Get() < 0 {
					return core.AccessibilityValue{}
				}
				s := shown()
				return core.AccessibilityValue{Text: tr.ChartsSalesValue(s.month, k(s.online), k(s.store))}
			},
			// The band of the month under the pointer: a faint tint over it.
			CrossLineStyleFunc: crossBand(cx, 0.05, 0),
			OnHoverChange:      hover.Set,
			Style:              core.Style{Height: core.Pt(salesHeight), Margin: core.Edges{Right: chartAxisGap + 8, Bottom: chartBelow}},
			Children: []core.Node{
				plot.Grid{Y: func() []float64 { return []float64{40, 80, 120} }, StyleFunc: gridStyle(cx)},
				plot.Grid{Y: func() []float64 { return []float64{0} }, StyleFunc: baselineStyle(cx)},
				bars,
				// The grid's values right of it, and a month under each band.
				rightAxis(cx, 0, 40, 80, 120),
				plot.Axis{Format: func(v float64, _ int) string { return sales[int(v)].month }, LabelStyleFunc: belowStyle(cx)},
			},
		}),
		chartSwitch(tr.ChartsLayout(), 220, stacked.Get, stacked.Set,
			ui.Option[bool]{Value: false, Label: tr.ChartsSideBySide()}, ui.Option[bool]{Value: true, Label: tr.ChartsStacked()}),
	}}
}

// bridgeQuarters are the bridge's quarters, and statements each quarter's
// revenue and what each line after it adds or takes, in millions: cost of
// sales, R&D, SG&A, other income (a loss in 3Q), tax.
var (
	bridgeQuarters = []string{"1Q26", "2Q26", "3Q26", "4Q26"}
	statements     = map[string][6]float64{
		"1Q26": {118.4, -46.2, -19.8, -24.1, 2.6, -7.4},
		"2Q26": {126.9, -48.5, -20.6, -25.3, 1.8, -8.1},
		"3Q26": {131.2, -52.9, -21.9, -26.0, -1.4, -6.6},
		"4Q26": {142.7, -54.1, -22.8, -27.4, 3.9, -9.2},
	}
)

// bridgeStep is a step of a bridge: a total stands on zero, every other
// step runs from where the one before it ends to where it leaves the
// running total.
type bridgeStep struct {
	name, short, kind string
	start, end        float64
}

// amount is what a step shows: a total's level, any other step's change.
func (s bridgeStep) amount() float64 {
	if s.kind == "total" {
		return s.end
	}
	return s.end - s.start
}

// bridge is a quarter's steps, named in tr's words.
func bridge(tr locales.Messages, q string) []bridgeStep {
	st := statements[q]
	var steps []bridgeStep
	level := st[0]
	total := func(name, short string) { steps = append(steps, bridgeStep{name, short, "total", 0, level}) }
	line := func(name, short string, v float64) {
		end := math.Round((level+v)*10) / 10
		kind := "increase"
		if v < 0 {
			kind = "decrease"
		}
		steps = append(steps, bridgeStep{name, short, kind, level, end})
		level = end
	}
	total(tr.ChartsRevenueStep(), tr.ChartsRev())
	line(tr.ChartsCostOfSales(), "COGS", st[1])
	total(tr.ChartsGrossProfit(), tr.ChartsGross())
	line("R&D", "R&D", st[2])
	line("SG&A", "SG&A", st[3])
	other := tr.ChartsOtherIncome()
	if st[4] < 0 {
		other = tr.ChartsOtherLoss()
	}
	line(other, tr.ChartsOther(), st[4])
	line(tr.ChartsTax(), tr.ChartsTax(), st[5])
	total(tr.ChartsNetProfit(), tr.ChartsNet())
	return steps
}

const (
	bridgeInner  = 0.32
	bridgeOuter  = 0.16
	bridgeTop    = 160
	bridgeHeight = 176
)

// millions writes millions of dollars, signedMillions a change of them.
func millions(v float64) string { return "$" + fixed(v, 1) + "M" }

func signedMillions(v float64) string {
	if v >= 0 {
		return "+$" + fixed(v, 1) + "M"
	}
	return "−$" + fixed(-v, 1) + "M"
}

// bridgeCard is a quarter's profit bridge, as a waterfall: revenue, what
// each line of the income statement takes or adds, gross and net profit as
// totals. Each step is a Bar from its row's base (where the running total
// stood) to its value (where the step leaves it), colored by kind through
// the plot's ColorScale, its signed change written at its end; dashed
// connectors carry each level to the next step. The step under the pointer
// or the arrow keys is read out over the plot and its share of revenue;
// the quarters move into each other on the plot's update, the bars, their
// labels and the connectors together.
func bridgeCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	quarter := core.NewSignal(cx, "4Q26")
	hover := core.NewSignal(cx, -1)
	steps := core.NewMemoFunc(cx, func() []bridgeStep { return bridge(tr, quarter.Get()) }, nil)
	// The plot's width, for the gap between two bands a connector spans.
	plotWidth := core.NewSignal(cx, core.Points(0))
	gap := func() core.Points {
		_, width, step := bandAt(0, 8, bridgeInner, bridgeOuter)
		return core.Points(step-width) * plotWidth.Get()
	}
	revenue := func() float64 { return steps.Get()[0].end }
	net := func() float64 { s := steps.Get(); return s[len(s)-1].end }
	margin := func(v float64) string { return fixed(v/revenue()*100, 1) + "%" }
	shown := func() (bridgeStep, bool) {
		if h := hover.Get(); h >= 0 && h < len(steps.Get()) {
			return steps.Get()[h], true
		}
		return bridgeStep{}, false
	}
	stepColor := func() core.Color {
		s, ok := shown()
		switch {
		case !ok:
			return colors.Text()
		case s.kind == "increase":
			return colors.Up()
		case s.kind == "decrease":
			return colors.Down()
		}
		return colors.Muted()
	}
	written := func(s bridgeStep) string {
		if s.kind == "total" {
			return millions(s.end)
		}
		return signedMillions(s.amount())
	}
	values := func(f func(bridgeStep) float64) func() []float64 { return func() []float64 { return column(steps.Get(), f) } }
	var options []ui.Option[string]
	for _, q := range bridgeQuarters {
		options = append(options, ui.Option[string]{Value: q, Label: q})
	}
	return Card{TestID: "card-bridge", Title: tr.ChartsBridge(), Description: tr.ChartsBridgeNote(), Children: []core.Node{
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			tipTitleText(cx, func() string {
				if s, ok := shown(); ok {
					return upper(s.name)
				}
				return upper(tr.ChartsNetProfit())
			}),
			baseline(10,
				headline(cx, func() string {
					if s, ok := shown(); ok {
						return written(s)
					}
					return millions(net())
				}, stepColor),
				note(cx, func() string {
					if s, ok := shown(); ok {
						return tr.ChartsOfRevenue(margin(math.Abs(s.amount())))
					}
					return tr.ChartsNetMarginOf(margin(net()))
				}, colors.Muted)),
		}},
		keys(8,
			chartLegend(cx, chartSeriesColor(cx, 1), constant(tr.ChartsTotal()), nil),
			chartLegend(cx, colors.Up, constant(tr.ChartsAdds()), nil),
			chartLegend(cx, colors.Down, constant(tr.ChartsTakes()), nil)),
		core.View{Style: core.Style{Margin: core.Edges{Top: chartGap()}}, OnLayout: func(b core.Bounds) { plotWidth.Set(b.Size.Width - chartAxisGap - 8) }, Children: []core.Node{plot.Plot{
			TestID: "bridge-plot",
			Count:  func() int { return len(steps.Get()) },
			XScale: func() plot.Scale { return plot.BandScale(plot.Band{PaddingInner: bridgeInner, PaddingOuter: bridgeOuter}) },
			YScale: func() plot.Scale { return plot.LinearScale(0, bridgeTop) },
			// Each step's kind takes its color: totals graphite, rises and
			// falls in the day's tones.
			ColorScale: func() plot.ColorScale {
				return plot.OrdinalColors([]string{"total", "increase", "decrease"}, chartSeries(cx)[1], colors.Up(), colors.Down())
			},
			AppearGeneration: func() any { return 1 },
			Motion:           chartMotion(ms(900), ms(550), false),
			LabelFunc:        func() string { return tr.ChartsBridgeLabel(quarter.Get(), millions(revenue()), millions(net()), margin(net())) },
			AccessibilityValue: func() core.AccessibilityValue {
				if s, ok := shown(); ok {
					return core.AccessibilityValue{Text: s.name + ": " + written(s)}
				}
				return core.AccessibilityValue{}
			},
			CrossLineStyleFunc: crossBand(cx, 0.05, 0),
			OnHoverChange:      hover.Set,
			Style:              core.Style{Height: core.Pt(bridgeHeight), Margin: core.Edges{Right: chartAxisGap + 8, Bottom: chartBelow}},
			Children: []core.Node{
				plot.Grid{Y: func() []float64 { return []float64{40, 80, 120} }, StyleFunc: gridStyle(cx)},
				// Connectors: each step's level carried across the gap to the
				// next step, placed on the scales (from a band's right edge,
				// half a band past its middle) so they follow the plot's ranges
				// and glide with a new quarter.
				plot.Label{Texts: func() []plot.PlotText {
					s := steps.Get()
					out := make([]plot.PlotText, len(s)-1)
					for i := range out {
						out[i] = plot.PlotText{X: float64(i) + (1-bridgeInner)/2, Y: s[i].end, Style: core.Style{Width: core.Pt(gap()), Height: core.Pt(0),
							Borders: core.Edges{Top: 1}, BorderStyle: core.BorderDashed, BorderColors: core.EdgeColors{Top: ui.Alpha(colors.Text(), 0.3)}}}
					}
					return out
				}},
				plot.Grid{Y: func() []float64 { return []float64{0} }, StyleFunc: baselineStyle(cx)},
				plot.Bar{Value: values(func(s bridgeStep) float64 { return s.end }), Bases: values(func(s bridgeStep) float64 { return s.start }),
					Color:   func() []string { s := steps.Get(); out := make([]string, len(s)); for i, x := range s { out[i] = x.kind }; return out },
					Corners: plot.AllCorners(2), Stagger: 0.5, HoverDim: 0.6,
					Label: func() []string {
						s := steps.Get()
						out := make([]string, len(s))
						for i, x := range s {
							if x.kind == "total" {
								out[i] = fixed(x.end, 1)
							} else {
								out[i] = sign(x.amount(), 1)
							}
						}
						return out
					},
					LabelStyleFunc: func() core.Style {
						return core.Style{FontSize: 10.5, FontFamily: ui.Mono, Padding: core.Symmetric(3, 0), Color: colors.Muted(), FontVariant: core.FontTabularNums}
					}},
				rightAxis(cx, 0, 40, 80, 120),
				plot.Axis{Format: func(v float64, _ int) string {
					if s := steps.Get(); int(v) < len(s) {
						return s[int(v)].short
					}
					return ""
				}, LabelStyleFunc: belowStyle(cx)},
			},
		}}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsRevenueStep(), func() string { return millions(revenue()) }, nil),
			chartFigure(cx, tr.ChartsGross(), func() string { return margin(steps.Get()[2].end) }, nil),
			chartFigure(cx, tr.ChartsOperating(), func() string { return margin(steps.Get()[4].end) }, nil),
			chartFigure(cx, tr.ChartsNet(), func() string { return margin(net()) }, fixedTone(ToneUp))),
		chartSwitch(tr.ChartsQuarter(), 260, quarter.Get, quarter.Set, options...),
	}}
}

// funnelReached is how many of each channel reached each stage, from a
// first visit to a first trade.
var funnelReached = map[string][]float64{
	"app": {96800, 27110, 19240, 10870, 7830},
	"web": {87400, 15250, 9170, 4150, 2410},
	"all": {184200, 42360, 28410, 15020, 10240},
}

var funnelStages = []string{"Visits", "Sign-ups", "KYC verified", "Funded", "First trade"}

// funnelStage is a stage: how many reached it and the share of the stage
// before that did.
type funnelStage struct {
	name        string
	count, kept float64
}

func funnelOf(channel string) []funnelStage {
	r := funnelReached[channel]
	out := make([]funnelStage, len(r))
	for i, c := range r {
		kept := 1.0
		if i > 0 {
			kept = c / r[i-1]
		}
		out[i] = funnelStage{funnelStages[i], c, kept}
	}
	return out
}

const (
	// funnelInner and funnelOuter make bars as high as the gaps between
	// them, where the necks run.
	funnelInner = 0.5
	funnelOuter = 0.08
	// funnelLeft and funnelRight are the columns beside the bars: each
	// stage's name over its count, and what it keeps.
	funnelLeft  = 86
	funnelRight = 54
	// funnelMinHalf is half the narrowest bar, in points: a stage of none
	// would be this wide, so a last stage of a few traders stays a bar.
	funnelMinHalf = 5
	// funnelRadius is the bars' corners, which the necks' edges follow.
	funnelRadius  = 4
	funnelAppear  = 900
	funnelStagger = 0.5
)

// funnelHalf is half the widest stage, everyone's visits: every channel on
// the same scale.
var funnelHalf = funnelReached["all"][0] / 2

// funnelTiers are each stage's depth of the series color, the top stage
// deepest.
var funnelTiers = []float32{1, 0.8, 0.64, 0.5, 0.38}

// funnelHeight is the funnel's height.
func funnelHeight() core.Points {
	if chartPointer() {
		return 204
	}
	return 216
}

// funnelRow is where stage i's bar is down the plot, in points: its top,
// height and middle, and the middle of the gap under it.
func funnelRow(i int) (top, height, middle, gap core.Points) {
	start, width, step := bandAt(i, len(funnelStages), funnelInner, funnelOuter)
	h := float64(funnelHeight())
	top, height = core.Points(start*h), core.Points(width*h)
	return top, height, top + height/2, top + height + core.Points((step-width)*h/2)
}

// funnelHalfFor is a stage's half width as a share of a plot width wide:
// from funnelMinHalf points for none to the whole width for everyone's
// visits, in proportion.
func funnelHalfFor(count float64, width core.Points) float64 {
	if width > 2*funnelMinHalf {
		return (funnelMinHalf + (float64(width)/2-funnelMinHalf)*(count/funnelReached["all"][0])) / float64(width)
	}
	return count / (4 * funnelHalf)
}

// shareText writes a share as a percentage.
func shareText(v float64) string { return fixed(v*100, 1) + "%" }

// funnelCard is an onboarding funnel: each stage a bar centered on the
// middle (a Bar from minus to plus half its width, a base per row),
// deepest at the top and lighter down, and between two stages a soft neck
// from one bar's ends to the next one's (a Canvas under the bars, its ends
// animated values), so the stages read as one flow. Beside the bars, each
// stage's name over its count, and what it keeps of the stage before; in
// the gaps, what is lost there, the biggest drop in the down tone. The bars
// grow out of the middle top down, the necks and labels fading in after
// them; a channel moves the bars and the necks together on the same scale.
// The stage under the pointer or the arrow keys stays lit and is read out
// over the plot.
func funnelCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	channel := core.NewSignal(cx, "all")
	hover := core.NewSignal(cx, -1)
	rows := core.NewMemoFunc(cx, func() []funnelStage { return funnelOf(channel.Get()) }, nil)
	n := len(funnelStages)
	shown := func() (funnelStage, bool) {
		if h := hover.Get(); h >= 0 && h < n {
			return rows.Get()[h], true
		}
		return funnelStage{}, false
	}
	overall := func() float64 { r := rows.Get(); return r[n-1].count / r[0].count }
	// The stage that keeps the least of the one before it.
	leak := func() funnelStage {
		r := rows.Get()
		worst := r[1]
		for _, s := range r[2:] {
			if s.kept < worst.kept {
				worst = s
			}
		}
		return worst
	}
	lit := func(i int) bool { h := hover.Get(); return h < 0 || h == i }
	still := cx.Env().ReduceMotion()
	height := funnelHeight()
	// The plot's width, for the narrowest stage; the necks' ends follow it
	// at once (a resize moves nothing).
	plotWidth := core.NewSignal(cx, core.Points(0))
	// The necks' ends: each bar's half width, moved with the bars.
	halves := make([]*core.AnimatedValue, n)
	for i, c := range funnelReached["all"] {
		halves[i] = core.NewAnimatedValue(cx, c/(4*funnelHalf))
	}
	cx.Effect(func() {
		width := plotWidth.Get()
		for i, c := range funnelReached[core.Untracked(cx, channel.Get)] {
			halves[i].Set(funnelHalfFor(c, width))
		}
	})
	start := float64(0)
	if still {
		start = 1
	}
	// Each neck in after the bar under it has grown, dimmed with the hover
	// unless it runs into the hovered stage; each row's labels in once its
	// bar is half grown.
	necksIn, necksLit, labelsIn := make([]*core.AnimatedValue, n-1), make([]*core.AnimatedValue, n-1), make([]*core.AnimatedValue, n)
	for i := range labelsIn {
		labelsIn[i] = core.NewAnimatedValue(cx, start)
	}
	for k := range necksIn {
		necksIn[k], necksLit[k] = core.NewAnimatedValue(cx, start), core.NewAnimatedValue(cx, 1)
	}
	growsAt := func(i int) float64 { return funnelAppear * funnelStagger * float64(i) / float64(n-1) }
	if !still {
		out := core.Easing{Kind: core.EasingCubic, Mode: core.EaseModeOut}
		for i, v := range labelsIn {
			ui.Timing(cx, v, 1, core.TimingConfig{Duration: ms(360), Delay: ms(int(growsAt(i)) + 280), Easing: out})
		}
		for k, v := range necksIn {
			ui.Timing(cx, v, 1, core.TimingConfig{Duration: ms(420), Delay: ms(int(growsAt(k+1)) + 360), Easing: out})
		}
	}
	move := func(v *core.AnimatedValue, to float64, c core.TimingConfig) {
		if cx.Env().ReduceMotion() {
			v.Set(to)
		} else {
			ui.Timing(cx, v, to, c)
		}
	}
	onHover := func(i int) {
		hover.Set(i)
		for k, v := range necksLit {
			to := 0.35
			if i < 0 || i == k+1 {
				to = 1
			}
			move(v, to, core.TimingConfig{Duration: ms(160), Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}})
		}
	}
	switchTo := func(next string) {
		channel.Set(next)
		for i, c := range funnelReached[next] {
			move(halves[i], funnelHalfFor(c, plotWidth.GetUntracked()), core.TimingConfig{Duration: ms(550), Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeInOut}})
		}
	}
	// A row's fade with the hover.
	dim := func(i int) core.Style {
		if lit(i) {
			return core.Style{Opacity: core.Alpha(1)}
		}
		return core.Style{Opacity: core.Alpha(0.35)}
	}
	fading := core.Transition{Duration: ms(150), Properties: core.TransitionOpacity | core.TransitionBackground}
	tier := func(i int) core.Color { return ui.Alpha(chartSeries(cx)[0], funnelTiers[i]) }
	channels := []ui.Option[string]{{Value: "all", Label: tr.ChartsAll()}, {Value: "app", Label: tr.ChartsApp()}, {Value: "web", Label: tr.ChartsWeb()}}
	channelName := func() string {
		for _, c := range channels {
			if c.Value == channel.Get() {
				return c.Label
			}
		}
		return ""
	}
	var names, kept []core.Node
	for i, name := range funnelStages {
		_, _, middle, _ := funnelRow(i)
		names = append(names, core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(middle - 16), Height: core.Pt(32),
			JustifyContent: core.JustifyCenter, Animated: &core.AnimatedStyle{Opacity: labelsIn[i]}}, Children: []core.Node{
			core.View{Style: core.Style{AlignItems: core.AlignEnd, Gap: 2, Transition: fading}, StyleFunc: func() core.Style { return dim(i) }, Children: []core.Node{
				core.Text{Content: name, Style: core.Style{NumberOfLines: 1, FontSize: 11}, StyleFunc: ui.Colored(colors.Muted)},
				core.Text{ContentFunc: func() string { return count(rows.Get()[i].count) }, Style: core.Style{FontSize: 13, FontWeight: ui.Medium, FontVariant: core.FontTabularNums},
					StyleFunc: ui.Colored(colors.Text)},
			}},
		}})
		kept = append(kept, core.View{Style: core.Style{Position: core.Absolute, Right: core.Pt(0), Top: core.Pt(middle - 9), Height: core.Pt(18),
			Animated: &core.AnimatedStyle{Opacity: labelsIn[i]}}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(18), JustifyContent: core.JustifyCenter, Padding: core.Symmetric(0, 6), CornerRadius: 9, Transition: fading},
				StyleFunc: func() core.Style {
					bg := ui.Alpha(colors.Text(), 0.06)
					if hover.Get() == i {
						bg = ui.Alpha(chartSeries(cx)[0], 0.22)
					}
					return dim(i).Merge(core.Style{Background: bg})
				}, Children: []core.Node{core.Text{ContentFunc: func() string {
					if i == 0 {
						return "100%"
					}
					return shareText(rows.Get()[i].kept)
				}, StyleFunc: axisColored(cx, core.Style{}, colors.Text)}}},
		}})
	}
	for k := range n - 1 {
		_, _, _, gap := funnelRow(k)
		kept = append(kept, core.View{Style: core.Style{Position: core.Absolute, Right: core.Pt(0), Top: core.Pt(gap - 7), Height: core.Pt(14), JustifyContent: core.JustifyCenter,
			Animated: &core.AnimatedStyle{Opacity: necksIn[k]}}, Children: []core.Node{
			core.Text{ContentFunc: func() string { return "−" + shareText(1-rows.Get()[k+1].kept) }, Style: core.Style{Transition: fading}, StyleFunc: func() core.Style {
				c := colors.Faint()
				if leak().name == funnelStages[k+1] {
					c = colors.Down()
				}
				return axisText(cx).Merge(core.Style{FontSize: 10, Padding: core.Edges{Right: 6}, Color: c}).Merge(dim(k + 1))
			}},
		}})
	}
	// The necks, beside the bars: from one bar's ends to the next one's,
	// curving through the gap. Their edges at the bars follow the bars'
	// rounded corners, half a point into them, so no seam shows.
	necks := core.Canvas{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), PointerEvents: core.PointerEventsNone},
		Draw: func(d *core.Drawing, size core.Size) {
			width := float64(size.Width)
			mid := core.Constant(width / 2)
			const r, e = funnelRadius, 0.5
			const quarter = math.Pi / 2
			c := chartSeries(cx)[0]
			for k := range n - 1 {
				top0, h0, _, _ := funnelRow(k)
				y0, y1 := float32(top0+h0), func() float32 { t, _, _, _ := funnelRow(k + 1); return float32(t) }()
				ym := (y0 + y1) / 2
				a, b := core.Mul(halves[k], core.Constant(width)), core.Mul(halves[k+1], core.Constant(width))
				aR, aL, bR, bL := core.Add(a, mid), core.Sub(mid, a), core.Add(b, mid), core.Sub(mid, b)
				in := func(x core.AnimatedNode, by float64) core.AnimatedNode { return core.Add(x, core.Constant(by)) }
				d.SetFillGradient(core.CanvasLinear(0, y0, 0, y1, core.GradientStop{Offset: 0, Color: ui.Alpha(c, 0.22*funnelTiers[k])},
					core.GradientStop{Offset: 1, Color: ui.Alpha(c, 0.22*funnelTiers[k+1])}))
				d.SetAlpha(1).Bind(core.ArgA, core.Mul(necksIn[k], necksLit[k]))
				d.BeginPath()
				// Along the bar above's bottom, its corners included, right to
				// left.
				d.MoveTo(0, y0-r).Bind(core.ArgX, aR)
				d.Arc(0, y0-r, r-e, 0, quarter, false).Bind(core.ArgX, in(aR, -r))
				d.Arc(0, y0-r, r-e, quarter, 2*quarter, false).Bind(core.ArgX, in(aL, r))
				d.LineTo(0, y0-r).Bind(core.ArgX, aL)
				// Down its left end and through the gap to the bar below.
				d.LineTo(0, y0).Bind(core.ArgX, aL)
				d.CubicTo(0, ym, 0, ym, 0, y1).Bind(core.ArgX1, aL).Bind(core.ArgX2, bL).Bind(core.ArgX, bL)
				d.LineTo(0, y1+r).Bind(core.ArgX, bL)
				// Along the bar below's top, left to right.
				d.Arc(0, y1+r, r-e, 2*quarter, 3*quarter, false).Bind(core.ArgX, in(bL, r))
				d.Arc(0, y1+r, r-e, 3*quarter, 4*quarter, false).Bind(core.ArgX, in(bR, -r))
				d.LineTo(0, y1+r).Bind(core.ArgX, bR)
				// Up its right end and back.
				d.LineTo(0, y1).Bind(core.ArgX, bR)
				d.CubicTo(0, ym, 0, ym, 0, y0).Bind(core.ArgX1, bR).Bind(core.ArgX2, aR).Bind(core.ArgX, aR)
				d.ClosePath().Fill()
			}
			d.SetAlpha(1)
		}}
	barEnd := func(side float64) func() []float64 {
		return func() []float64 {
			width := plotWidth.Get()
			out := make([]float64, n)
			for i, s := range rows.Get() {
				out[i] = side * funnelHalfFor(s.count, width) * 2 * funnelHalf
			}
			return out
		}
	}
	tiers := make([]string, n)
	for i := range tiers {
		tiers[i] = strconv.Itoa(i)
	}
	return Card{TestID: "card-funnel", Title: tr.ChartsFunnel(), Description: tr.ChartsFunnelNote(), Children: []core.Node{
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			tipTitleText(cx, func() string {
				if s, ok := shown(); ok {
					return upper(s.name)
				}
				return upper(tr.ChartsVisitsToFirstTrade())
			}),
			baseline(10,
				headline(cx, func() string {
					if s, ok := shown(); ok {
						return count(s.count)
					}
					return shareText(overall())
				}, nil),
				note(cx, func() string {
					r := rows.Get()
					s, ok := shown()
					switch {
					case !ok:
						return tr.ChartsTradersOfVisits(count(r[n-1].count), count(r[0].count))
					case hover.Get() == 0:
						return tr.ChartsTopOfTheFunnel()
					}
					return tr.ChartsKeptOf(shareText(s.kept), strings.ToLower(r[hover.Get()-1].name), shareText(s.count/r[0].count))
				}, colors.Muted)),
		}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 10, Margin: core.Edges{Top: chartGap() + 6}}, Children: []core.Node{
			// Each stage's name over its count, end-aligned on its bar's
			// middle.
			core.View{Style: core.Style{Width: core.Pt(funnelLeft), Height: core.Pt(height)}, Children: names},
			core.View{Style: core.Style{FlexGrow: 1, Height: core.Pt(height)}, OnLayout: func(b core.Bounds) { plotWidth.Set(b.Size.Width) }, Children: []core.Node{plot.Plot{
				TestID: "funnel-plot",
				Count:  func() int { return n },
				// Stages down the plot, the counts across it, centered on zero.
				XScale: func() plot.Scale { return plot.LinearScale(-funnelHalf, funnelHalf) },
				YScale: func() plot.Scale { return plot.BandScale(plot.Band{PaddingInner: funnelInner, PaddingOuter: funnelOuter}) },
				// One hue, deepest at the top stage.
				ColorScale: func() plot.ColorScale {
					shades := make([]core.Color, n)
					for i := range shades {
						shades[i] = tier(i)
					}
					return plot.OrdinalColors(tiers, shades...)
				},
				AppearGeneration: func() any { return 1 },
				Motion:           chartMotion(ms(funnelAppear), ms(550), false),
				LabelFunc: func() string {
					r := rows.Get()
					return tr.ChartsFunnelLabel(channelName(), count(r[0].count), count(r[n-1].count), shareText(overall()))
				},
				AccessibilityValue: func() core.AccessibilityValue {
					if s, ok := shown(); ok {
						return core.AccessibilityValue{Text: tr.ChartsFunnelValue(s.name, count(s.count), shareText(s.kept))}
					}
					return core.AccessibilityValue{}
				},
				CrossLineStyleFunc: crossBand(cx, 0.045, 6),
				OnHoverChange:      onHover,
				Style:              core.Style{Height: core.Pt(height)},
				Children: []core.Node{
					necks,
					plot.Bar{Bases: barEnd(-1), Value: barEnd(1), Color: func() []string { return tiers }, Corners: plot.AllCorners(funnelRadius),
						Stagger: funnelStagger, HoverDim: 0.65},
				},
			}}},
			// What each stage keeps of the one before, on its row; what is
			// lost, in the gap above it.
			core.View{Style: core.Style{Width: core.Pt(funnelRight), Height: core.Pt(height)}, Children: kept},
		}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsOverall(), func() string { return shareText(overall()) }, fixedTone(ToneUp)),
			chartFigure(cx, tr.ChartsBiggestDrop(), func() string { return leak().name }, nil),
			chartFigure(cx, tr.ChartsLostThere(), func() string { return "−" + shareText(1-leak().kept) }, fixedTone(ToneDown))),
		chartSwitch(tr.ChartsChannel(), 200, channel.Get, switchTo, channels...),
	}}
}
