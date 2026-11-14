package main

import (
	"math"
	"slices"
	"sort"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// riskPeriods are the periods a holding's annualized volatility and return
// are measured over.
var riskPeriods = []string{"1Y", "3Y", "5Y"}

var riskSectors = []string{"Technology", "Financials", "Energy", "Healthcare", "Consumer"}

// riskHolding is a holding: its market cap in billions and, per period,
// its annualized volatility and return in percent.
type riskHolding struct {
	ticker, name, sector string
	cap                  float64
	p                    [3][2]float64
}

var riskHoldings = []riskHolding{
	{"AAPL", "Apple", "Technology", 3600, [3][2]float64{{25, 12}, {23, 15}, {27, 21}}},
	{"MSFT", "Microsoft", "Technology", 3700, [3][2]float64{{21, 19}, {22, 17}, {25, 19}}},
	{"NVDA", "NVIDIA", "Technology", 4300, [3][2]float64{{48, 36}, {52, 44}, {54, 46}}},
	{"AMD", "Advanced Micro Devices", "Technology", 380, [3][2]float64{{55, 41}, {50, 9}, {53, 26}}},
	{"JPM", "JPMorgan Chase", "Financials", 830, [3][2]float64{{22, 31}, {21, 24}, {25, 19}}},
	{"GS", "Goldman Sachs", "Financials", 240, [3][2]float64{{29, 42}, {26, 27}, {29, 21}}},
	{"V", "Visa", "Financials", 670, [3][2]float64{{18, 15}, {18, 13}, {21, 12}}},
	{"XOM", "Exxon Mobil", "Energy", 470, [3][2]float64{{20, 3}, {23, 6}, {29, 22}}},
	{"CVX", "Chevron", "Energy", 300, [3][2]float64{{22, -3}, {24, 1}, {30, 15}}},
	{"LLY", "Eli Lilly", "Healthcare", 820, [3][2]float64{{33, 12}, {30, 41}, {29, 37}}},
	{"UNH", "UnitedHealth", "Healthcare", 300, [3][2]float64{{42, -26}, {30, -4}, {31, 4}}},
	{"JNJ", "Johnson & Johnson", "Healthcare", 460, [3][2]float64{{15, 14}, {14.5, 2}, {16, 5}}},
	{"PFE", "Pfizer", "Healthcare", 140, [3][2]float64{{25, -6}, {23, -14}, {25, -4}}},
	// In the middle of the plot over three years, where a pointer lands.
	{"AMZN", "Amazon", "Consumer", 2400, [3][2]float64{{31, 8}, {34, 11}, {33, 10}}},
	{"COST", "Costco", "Consumer", 420, [3][2]float64{{19, 4}, {20, 26}, {21, 24}}},
	{"NKE", "Nike", "Consumer", 95, [3][2]float64{{36, -15}, {33, -20}, {31, -10}}},
	{"MCD", "McDonald's", "Consumer", 220, [3][2]float64{{16, 2}, {17, 7}, {18, 11}}},
}

// spx is the index the holdings are measured against, per period.
var spx = [3][2]float64{{16, 16}, {15.5, 18}, {17.5, 14}}

// riskRow is a plotted row: a holding, its cap sizing its bubble, or the
// index, its bench placing its marker; NaN for what a row has not.
type riskRow struct {
	ticker, name, sector string
	vol, ret, cap, bench float64
}

// riskOrder is the rows' tickers in the order of their three-year
// volatility, so the keys step from the left.
var riskOrder = func() []string {
	type v struct {
		ticker string
		vol    float64
	}
	var all []v
	for _, h := range riskHoldings {
		all = append(all, v{h.ticker, h.p[1][0]})
	}
	all = append(all, v{"SPX", spx[1][0]})
	sort.SliceStable(all, func(i, j int) bool { return all[i].vol < all[j].vol })
	out := make([]string, len(all))
	for i, a := range all {
		out[i] = a.ticker
	}
	return out
}()

func riskRows(period int) []riskRow {
	out := make([]riskRow, len(riskOrder))
	nan := math.NaN()
	for i, t := range riskOrder {
		if t == "SPX" {
			out[i] = riskRow{t, "S&P 500", "", spx[period][0], spx[period][1], nan, spx[period][1]}
			continue
		}
		h := riskHoldings[slices.IndexFunc(riskHoldings, func(h riskHolding) bool { return h.ticker == t })]
		out[i] = riskRow{t, h.name, h.sector, h.p[period][0], h.p[period][1], h.cap, nan}
	}
	return out
}

// riskFree is the rate a Sharpe ratio is measured over, in percent.
const riskFree = 4

func sharpe(r riskRow) float64 { return (r.ret - riskFree) / r.vol }

// capOf writes a market cap in billions.
func capOf(b float64) string {
	if b >= 1000 {
		return "$" + fixed(b/1000, 1) + "T"
	}
	return "$" + fixed(b, 0) + "B"
}

var (
	volDomain = [2]float64{8, 60}
	retDomain = [2]float64{-36, 58}
	retTicks  = []float64{-20, 0, 20, 40}
)

// riskCard is risk against return: a portfolio's holdings as bubbles (a
// Dot with a radius per row through a RadiusScale, so a bubble's area is
// its market cap), each holding's annualized volatility along x and return
// up y, colored by sector through the plot's ColorScale, translucent with
// a ring in the card's color so overlaps stay apart. The S&P 500 is a
// hollow marker of its own (a second Dot that reads only the index's row),
// and dashed lines through it split the plot into quadrants. A period moves
// every bubble to its new place; the holding under the pointer or the
// arrow keys is ringed and read out over the plot, the others fading.
func riskCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	period := core.NewSignal(cx, 1)
	hover := core.NewSignal(cx, -1)
	rows := core.NewMemoFunc(cx, func() []riskRow { return riskRows(period.Get()) }, nil)
	holdings := func() []riskRow {
		var out []riskRow
		for _, r := range rows.Get() {
			if r.ticker != "SPX" {
				out = append(out, r)
			}
		}
		return out
	}
	index := func() riskRow {
		r := rows.Get()
		return r[slices.IndexFunc(r, func(r riskRow) bool { return r.ticker == "SPX" })]
	}
	shown := func() (riskRow, bool) {
		if h := hover.Get(); h >= 0 && h < len(rows.Get()) {
			return rows.Get()[h], true
		}
		return riskRow{}, false
	}
	mean := func(f func(riskRow) float64) float64 {
		sum := 0.0
		for _, r := range holdings() {
			sum += f(r)
		}
		return sum / float64(len(holdings()))
	}
	best := func() riskRow {
		return slices.MaxFunc(holdings(), func(a, b riskRow) int { return cmpFloat(sharpe(a), sharpe(b)) })
	}
	ahead := func() int {
		n := 0
		for _, r := range holdings() {
			if sharpe(r) > sharpe(index()) {
				n++
			}
		}
		return n
	}
	volTicks := []float64{20, 40}
	height := core.Points(248)
	if chartPointer() {
		volTicks, height = []float64{10, 20, 30, 40, 50}, 236
	}
	plotWidth := core.NewSignal(cx, core.Points(0))
	narrow := core.NewMemo(cx, func() bool { w := plotWidth.Get(); return w > 0 && w < 420 })
	sectorColor := func(i int) func() core.Color { return func() core.Color { return chartCategories(cx)[i] } }
	var sectorKeys []core.Node
	for i, s := range riskSectors {
		sectorKeys = append(sectorKeys, dotKey(cx, sectorColor(i), s, false, 9))
	}
	sectorKeys = append(sectorKeys, dotKey(cx, colors.Text, "S&P 500", true, 9))
	// On the card's color, so the grid and the index's lines pass under the
	// words.
	corner := func() core.Style {
		return axisText(cx).Merge(core.Style{Padding: core.Symmetric(4, 6), Background: colors.Surface(), CornerRadius: 4})
	}
	column := func(f func(riskRow) float64) func() []float64 {
		return func() []float64 {
			r := rows.Get()
			out := make([]float64, len(r))
			for i, x := range r {
				out[i] = f(x)
			}
			return out
		}
	}
	ring := ringStyle(cx, 999)
	return Card{TestID: "card-risk", Title: tr.ChartsRisk(), Description: tr.ChartsRiskNote(), Children: []core.Node{
		readout(cx,
			func() string {
				if r, ok := shown(); ok {
					if r.ticker == "SPX" {
						return "S&P 500 · " + upper(tr.ChartsBenchmark())
					}
					return r.ticker + " · " + upper(r.name)
				}
				return upper(tr.ChartsPortfolio()) + " · " + riskPeriods[period.Get()]
			},
			func() string {
				if r, ok := shown(); ok {
					return pct(r.ret, 1)
				}
				return pct(mean(func(r riskRow) float64 { return r.ret }), 1)
			}, nil,
			func() string {
				if r, ok := shown(); ok {
					return tr.ChartsVolSharpe(fixed(r.vol, 1)+"%", fixed(sharpe(r), 2))
				}
				return tr.ChartsMeanReturn(fixed(mean(func(r riskRow) float64 { return r.vol }), 1) + "%")
			},
			note(cx, func() string {
				r, ok := shown()
				switch {
				case !ok:
					return tr.ChartsHoldingsWeight(itoa(len(riskHoldings)), itoa(riskFree)+"%")
				case r.ticker == "SPX":
					return tr.ChartsTheIndex()
				}
				return tr.ChartsSectorCap(r.sector, capOf(r.cap))
			}, colors.Faint)),
		keys(10, sectorKeys...),
		core.View{Style: core.Style{Margin: core.Edges{Top: chartGap() + 4}}, OnLayout: func(b core.Bounds) { plotWidth.Set(b.Size.Width) }, Children: []core.Node{
			// The y axis' title, over its ticks on the right.
			core.Text{Content: tr.ChartsReturnAnnualized() + " ↑", Style: core.Style{AlignSelf: core.AlignEnd, Margin: core.Edges{Bottom: 6}}, StyleFunc: axisStyle(cx, core.Style{})},
			plot.Plot{
				TestID: "risk-plot",
				Count:  func() int { return len(rows.Get()) },
				X:      column(func(r riskRow) float64 { return r.vol }),
				XScale: func() plot.Scale { return plot.LinearScale(volDomain[0], volDomain[1]) },
				YScale: func() plot.Scale { return plot.LinearScale(retDomain[0], retDomain[1]) },
				// Translucent sector fills, so a bubble under another still
				// shows.
				ColorScale: func() plot.ColorScale {
					shades := make([]core.Color, len(riskSectors))
					for i, c := range chartCategories(cx) {
						shades[i] = ui.Alpha(c, 0.78)
					}
					return plot.OrdinalColors(riskSectors, shades...)
				},
				AppearGeneration: func() any { return 1 },
				Motion:           chartMotion(ms(1000), ms(700), false),
				LabelFunc: func() string {
					return tr.ChartsRiskLabel(itoa(len(riskHoldings)), riskPeriods[period.Get()], best().ticker, itoa(ahead()))
				},
				AccessibilityValue: func() core.AccessibilityValue {
					if r, ok := shown(); ok {
						return core.AccessibilityValue{Text: tr.ChartsRiskValue(r.name, pct(r.ret, 1), fixed(r.vol, 1)+"%", fixed(sharpe(r), 2))}
					}
					return core.AccessibilityValue{}
				},
				OnHoverChange: hover.Set,
				Style:         core.Style{Height: core.Pt(height), Margin: core.Edges{Right: chartAxisGap + 8, Bottom: chartBelow}},
				Children: []core.Node{
					plot.Grid{X: func() []float64 { return volTicks }, Y: func() []float64 { return retTicks }, StyleFunc: gridStyle(cx)},
					plot.Grid{Y: func() []float64 { return []float64{0} }, StyleFunc: baselineStyle(cx)},
					// The index's risk and return: the quadrants' edges.
					plot.Grid{X: func() []float64 { return []float64{index().vol} }, Y: func() []float64 { return []float64{index().ret} },
						StyleFunc: func() core.Style {
							return core.Style{Stroke: core.PaintColor(ui.Alpha(colors.Text(), 0.3)), StrokeDash: []core.Points{2, 4}}
						}},
					plot.Label{Texts: func() []plot.PlotText {
						return []plot.PlotText{
							{Text: tr.ChartsLessRisk(), X: volDomain[0], Y: retDomain[1], Style: corner()},
							{Text: tr.ChartsMoreRisk(), X: volDomain[1], Y: retDomain[0], Align: plot.TextRight, Class: "-translate-y-full", Style: corner()},
						}
					}},
					// The holdings: a bubble's area its market cap, the bigger
					// ones under the smaller.
					core.Dynamic(func(cx *core.Scope) core.Node {
						// Smaller bubbles on a narrow plot, so the crowded middle stays
						// apart.
						radii := &plot.RadiusScale{Min: 4, Max: 20}
						if narrow.Get() {
							radii = &plot.RadiusScale{Min: 3, Max: 12}
						}
						return plot.Dot{Value: column(func(r riskRow) float64 { return r.ret }), Sizes: column(func(r riskRow) float64 { return r.cap }), RScale: radii,
							Color: func() []string { return dirs(rows.Get(), func(r riskRow) string { return r.sector }) }, Stagger: 0.5, HoverDim: 0.78, HitRadius: 6,
							StyleFunc:         func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Surface()), StrokeWidth: 1.5} },
							HoverDotStyleFunc: ring}
					}),
					// The index: a hollow marker in the text's color over a halo
					// in the card's, lifting it out of the bubbles it sits among.
					plot.Dot{Value: column(func(r riskRow) float64 { return r.bench }), R: 9, HoverDim: 0.7,
						StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(colors.Surface())} }, HoverDotStyle: hidden},
					plot.Dot{Value: column(func(r riskRow) float64 { return r.bench }), R: 5, HoverDim: 0.7,
						StyleFunc: func() core.Style {
							return core.Style{Fill: core.PaintColor(colors.Surface()), Stroke: core.PaintColor(colors.Text()), StrokeWidth: 2}
						}, HoverDotStyleFunc: ring},
					plot.Axis{Placement: plot.AxisRight, TickValues: func() []float64 { return retTicks }, Format: func(v float64, _ int) string {
						if v == 0 {
							return "0"
						}
						return pct(v, 0)
					}, LabelStyleFunc: gutterStyle(cx, chartAxisGap+8, true)},
					plot.Axis{TickValues: func() []float64 { return volTicks }, Format: func(v float64, _ int) string { return fixed(v, 0) + "%" }, LabelStyleFunc: belowStyle(cx)},
				},
			},
			// The x axis' title, under its ticks' end.
			core.Text{Content: tr.ChartsVolatility() + " →", Style: core.Style{AlignSelf: core.AlignEnd, Margin: core.Edges{Right: chartAxisGap + 8, Top: 2}},
				StyleFunc: axisStyle(cx, core.Style{})},
		}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsBestSharpe(), func() string { return best().ticker + " " + fixed(sharpe(best()), 2) }, fixedTone(ToneUp)),
			chartFigure(cx, "S&P 500", func() string { return pct(index().ret, 1) }, nil),
			chartFigure(cx, tr.ChartsAhead(), func() string { return itoa(ahead()) + " / " + itoa(len(riskHoldings)) }, nil)),
		chartSwitch(tr.ChartsPeriod(), 180, period.Get, period.Set,
			ui.Option[int]{Value: 0, Label: "1Y"}, ui.Option[int]{Value: 1, Label: "3Y"}, ui.Option[int]{Value: 2, Label: "5Y"}),
	}}
}

// covered is a stock's analysts: today's price, their lowest, mean and
// highest 12-month targets, the mean three months ago, and how many cover
// it.
type covered struct {
	ticker                                string
	price, low, mean, high, prior, analysts float64
}

var coverage = map[string][]covered{
	"semis": {
		{"NVDA", 186, 140, 228, 300, 210, 64}, {"AMD", 238, 150, 255, 350, 190, 45}, {"AVGO", 342, 280, 405, 460, 380, 41},
		{"TSM", 298, 250, 345, 400, 320, 18}, {"INTC", 38, 20, 36, 52, 28, 38}, {"QCOM", 168, 130, 185, 230, 188, 34},
	},
	"software": {
		{"MSFT", 512, 450, 625, 710, 600, 58}, {"ORCL", 258, 175, 330, 400, 290, 40}, {"CRM", 242, 200, 330, 430, 345, 50},
		{"ADBE", 345, 280, 430, 560, 460, 36}, {"NOW", 905, 750, 1130, 1300, 1100, 42}, {"PLTR", 182, 100, 160, 255, 130, 25},
	},
	"banks": {
		{"JPM", 305, 255, 330, 375, 315, 26}, {"BAC", 51, 44, 57, 63, 54, 25}, {"WFC", 82, 70, 90, 101, 86, 24},
		{"C", 98, 80, 112, 128, 102, 22}, {"GS", 780, 560, 790, 940, 700, 24}, {"MS", 152, 120, 158, 180, 160, 25},
	},
}

// target is a plotted stock: its targets as upsides over today's price, in
// percent.
type target struct {
	ticker                              string
	price, mean, analysts               float64
	lowUp, meanUp, highUp, priorUp, rev float64
	dir                                 string
}

func upside(target, price float64) float64 { return (target/price - 1) * 100 }

// targets are a list's stocks, the most upside first.
func targets(list string) []target {
	var out []target
	for _, c := range coverage[list] {
		dir := "up"
		if c.mean < c.price {
			dir = "down"
		}
		out = append(out, target{c.ticker, c.price, c.mean, c.analysts, upside(c.low, c.price), upside(c.mean, c.price), upside(c.high, c.price),
			upside(c.prior, c.price), (c.mean/c.prior - 1) * 100, dir})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].meanUp > out[j].meanUp })
	return out
}

// dollars writes a price.
func dollars(v float64) string {
	switch {
	case v >= 1000:
		return "$" + count(v)
	case v < 100:
		return "$" + fixed(v, 2)
	}
	return "$" + fixed(v, 0)
}

func median(values []float64) float64 {
	s := slices.Sorted(slices.Values(values))
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

// targetsCard is analysts' price targets as a dot plot across a band y
// scale, one stock a row: a Rule from the lowest target to the highest
// (the street's spread), a hollow Dot at the mean target three months ago
// and a filled one at today's, in the tone of its direction, all as upside
// over today's price, a firmer line at zero. The stock under the pointer
// or the arrow keys is read out over the plot; another list moves the dots
// and ranges to its stocks'.
func targetsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	list := core.NewSignal(cx, "semis")
	hover := core.NewSignal(cx, -1)
	rows := core.NewMemoFunc(cx, func() []target { return targets(list.Get()) }, nil)
	shown := func() (target, bool) {
		if h := hover.Get(); h >= 0 && h < len(rows.Get()) {
			return rows.Get()[h], true
		}
		return target{}, false
	}
	lists := []ui.Option[string]{{Value: "semis", Label: tr.ChartsSemis()}, {Value: "software", Label: tr.ChartsSoftware()}, {Value: "banks", Label: tr.ChartsBanks()}}
	name := func() string {
		for _, l := range lists {
			if l.Value == list.Get() {
				return l.Label
			}
		}
		return ""
	}
	values := func(f func(target) float64) func() []float64 { return func() []float64 { return column(rows.Get(), f) } }
	medianUp := func() float64 { return median(column(rows.Get(), func(t target) float64 { return t.meanUp })) }
	top := func() target { return rows.Get()[0] }
	widest := func() target {
		return slices.MaxFunc(rows.Get(), func(a, b target) int { return cmpFloat(a.highUp-a.lowUp, b.highUp-b.lowUp) })
	}
	raised := func() int {
		n := 0
		for _, t := range rows.Get() {
			if t.rev > 0 {
				n++
			}
		}
		return n
	}
	ticks := []float64{-50, 0, 50}
	row, dot := core.Points(34), core.Points(6)
	if chartPointer() {
		ticks, row, dot = []float64{-50, -25, 0, 25, 50, 75}, 30, 5.5
	}
	var lines []float64
	for _, v := range ticks {
		if v != 0 {
			lines = append(lines, v)
		}
	}
	return Card{TestID: "card-targets", Title: tr.ChartsTargets(), Description: tr.ChartsTargetsNote(), Children: []core.Node{
		readout(cx,
			func() string {
				if t, ok := shown(); ok {
					return t.ticker + " · " + upper(tr.ChartsAnalysts(fixed(t.analysts, 0)))
				}
				return upper(name()) + " · " + upper(tr.ChartsStocks(itoa(len(rows.Get()))))
			},
			func() string {
				if t, ok := shown(); ok {
					return dollars(t.mean)
				}
				return pct(medianUp(), 1)
			}, nil,
			func() string {
				if t, ok := shown(); ok {
					return tr.ChartsFromPrice(pct(t.meanUp, 1), dollars(t.price))
				}
				return tr.ChartsMedianUpside()
			},
			note(cx, func() string {
				if t, ok := shown(); ok {
					return tr.ChartsTargetRange(dollars(t.price*(1+t.lowUp/100)), dollars(t.price*(1+t.highUp/100)), pct(t.rev, 1))
				}
				return tr.ChartsRaised(itoa(raised()), itoa(len(rows.Get())))
			}, colors.Faint)),
		keys(10,
			dotKey(cx, colors.Up, tr.ChartsMeanTarget(), false, 9),
			dotKey(cx, colors.Muted, tr.Charts3MonthsAgo(), true, 7),
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(14), Height: core.Pt(2), CornerRadius: 1}, StyleFunc: func() core.Style { return core.Style{Background: ui.Alpha(colors.Text(), 0.3)} }},
				ui.Text(tr.ChartsLowToHigh(), 12, ui.Regular, colors.Muted),
			}}),
		spaced(chartGap()+4, plot.Plot{
			TestID: "targets-plot",
			Count:  func() int { return len(rows.Get()) },
			// Stocks down the plot, upside across it.
			XScale: func() plot.Scale { return plot.LinearScale(-55, 85) },
			YScale: func() plot.Scale { return plot.BandScale(plot.Band{PaddingInner: 0.3, PaddingOuter: 0.15}) },
			ColorScale: func() plot.ColorScale {
				return plot.OrdinalColors([]string{"up", "down"}, colors.Up(), colors.Down())
			},
			AppearGeneration: func() any { return 1 },
			Motion:           chartMotion(ms(900), ms(600), false),
			LabelFunc:        func() string { return tr.ChartsTargetsLabel(name(), pct(medianUp(), 1), top().ticker, pct(top().meanUp, 1)) },
			AccessibilityValue: func() core.AccessibilityValue {
				if t, ok := shown(); ok {
					return core.AccessibilityValue{Text: tr.ChartsTargetsValue(t.ticker, dollars(t.mean), pct(t.meanUp, 1), dollars(t.price))}
				}
				return core.AccessibilityValue{}
			},
			CrossLineStyleFunc: crossBand(cx, 0.04, 6),
			OnHoverChange:      hover.Set,
			StyleFunc: func() core.Style {
				return core.Style{Height: core.Pt(core.Points(len(rows.Get())) * row), Margin: core.Edges{Left: 44, Bottom: chartBelow}}
			},
			Children: []core.Node{
				plot.Grid{X: func() []float64 { return lines }, StyleFunc: gridStyle(cx)},
				// Today's price: no upside.
				plot.Grid{X: func() []float64 { return []float64{0} }, StyleFunc: baselineStyle(cx)},
				plot.Rule{Value: values(func(t target) float64 { return t.highUp }), Bases: values(func(t target) float64 { return t.lowUp }), Stagger: 0.4, HoverDim: 0.65,
					StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(ui.Alpha(colors.Text(), 0.3)), StrokeWidth: 2} }},
				plot.Dot{Value: values(func(t target) float64 { return t.priorUp }), R: 3.5, Stagger: 0.4, HoverDim: 0.65, HoverDotStyle: hidden,
					StyleFunc: func() core.Style {
						return core.Style{Fill: core.PaintColor(colors.Surface()), Stroke: core.PaintColor(colors.Muted()), StrokeWidth: 1.5}
					}},
				// The pointer finds a stock's dot from anywhere on its row.
				plot.Dot{Value: values(func(t target) float64 { return t.meanUp }), R: dot, Color: func() []string { return dirs(rows.Get(), func(t target) string { return t.dir }) },
					Stagger: 0.4, HoverDim: 0.65, HitRadius: row,
					StyleFunc:         func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Surface()), StrokeWidth: 2} },
					HoverDotStyleFunc: ringStyle(cx, 999)},
				plot.Axis{Placement: plot.AxisLeft, Format: func(v float64, _ int) string {
					if r := rows.Get(); int(v) < len(r) {
						return r[int(v)].ticker
					}
					return ""
				}, LabelStyleFunc: func() core.Style { return gutterStyle(cx, 44, false)().Merge(core.Style{Padding: core.Edges{Right: 10}, Color: colors.Muted()}) }},
				plot.Axis{TickValues: func() []float64 { return ticks }, Format: func(v float64, _ int) string {
					if v == 0 {
						return tr.ChartsToday()
					}
					return pct(v, 0)
				}, LabelStyleFunc: belowStyle(cx)},
			},
		}),
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsMostUpside(), func() string { return top().ticker + " " + pct(top().meanUp, 0) }, func() Tone {
				if top().dir == "up" {
					return ToneUp
				}
				return ToneDown
			}),
			chartFigure(cx, tr.ChartsWidest(), func() string { return widest().ticker }, nil),
			chartFigure(cx, tr.ChartsRaisedFigure(), func() string { return itoa(raised()) + " / " + itoa(len(rows.Get())) }, nil)),
		chartSwitch(tr.ChartsCoverage(), 240, list.Get, list.Set, lists...),
	}}
}
