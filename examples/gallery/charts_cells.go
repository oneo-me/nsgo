package main

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// pairTickers are the holdings whose daily returns the correlation matrix
// pairs.
var pairTickers = []string{"NVDA", "AMD", "MSFT", "AAPL", "TSLA", "JPM", "XOM", "GLD"}

// exposures are each holding's loadings on six factors (the market, tech,
// semiconductors, rates, energy, gold) and the variance of its own news,
// per lookback: a factor model, so the matrix it makes is a correlation
// matrix.
var exposures = map[string][][7]float64{
	"1Y": {
		{0.7, 0.45, 0.85, 0, 0, 0, 0.25}, {0.7, 0.4, 0.85, 0, 0, 0, 0.35}, {0.7, 0.55, 0.15, 0, 0, 0, 0.3}, {0.65, 0.35, 0, 0, 0, 0, 0.5},
		{0.6, 0.25, 0, 0, 0, 0, 1.1}, {0.65, -0.05, 0, 0.6, 0.2, 0, 0.3}, {0.3, -0.25, 0, 0.3, 0.8, 0.1, 0.3}, {-0.2, -0.1, -0.05, -0.15, 0.15, 0.85, 0.3},
	},
	"3Y": {
		{0.75, 0.45, 0.7, 0, 0, 0, 0.35}, {0.75, 0.4, 0.7, 0, 0, 0, 0.45}, {0.75, 0.5, 0.1, 0, 0, 0, 0.35}, {0.75, 0.45, 0.05, 0, 0, 0, 0.4},
		{0.7, 0.3, 0, 0, 0, 0, 0.95}, {0.7, 0, 0, 0.55, 0.05, 0, 0.35}, {0.45, -0.1, 0, 0.2, 0.75, 0.05, 0.3}, {0.05, 0, 0, -0.1, 0.1, 0.8, 0.35},
	},
}

// holdingPair is a cell of the matrix: the holding of its line, of its column,
// and their correlation.
type holdingPair struct {
	a, b string
	r    float64
}

// correlations are every pair of holdings, line by line, each loading
// nudged a little: symmetric, 1 on the diagonal.
func correlations(lookback string) []holdingPair {
	seed := 37.0
	if lookback == "1Y" {
		seed = 11
	}
	r := chartRandom(seed)
	n := len(pairTickers)
	loads := make([][6]float64, n)
	own := make([]float64, n)
	for i, e := range exposures[lookback] {
		for k := range 6 {
			if e[k] != 0 {
				loads[i][k] = e[k] * (0.92 + r()*0.16)
			}
		}
		own[i] = e[6]
	}
	cov := func(i, j int) float64 {
		s := 0.0
		for k := range 6 {
			s += loads[i][k] * loads[j][k]
		}
		if i == j {
			s += own[i]
		}
		return s
	}
	var out []holdingPair
	for i, a := range pairTickers {
		for j, b := range pairTickers {
			v := 1.0
			if i != j {
				v = math.Round(cov(i, j)/math.Sqrt(cov(i, i)*cov(j, j))*100) / 100
			}
			out = append(out, holdingPair{a, b, v})
		}
	}
	return out
}

var pairCells = map[string][]holdingPair{"1Y": correlations("1Y"), "3Y": correlations("3Y")}

// pairsOf are the pairs of two holdings, each once: the cells above the
// diagonal.
func pairsOf(cells []holdingPair) []holdingPair {
	var out []holdingPair
	for _, p := range cells {
		if slices.Index(pairTickers, p.a) < slices.Index(pairTickers, p.b) {
			out = append(out, p)
		}
	}
	return out
}

// cellLabel and darkCellLabel are the cells' labels: dark on light cells,
// white on dark ones.
var (
	cellLabel     = core.RGBA(0x000000, 0.7)
	darkCellLabel = core.Style{Color: core.RGBA(0xffffff, 0.9)}
)


// correlationCard is how a portfolio's holdings move together: a
// correlation matrix of their daily returns as a heatmap, a Cell per pair
// in the bands of its two holdings (band scales with the tickers as their
// domain), colored along a diverging ColorScale from −1 through 0 to +1,
// its coefficient written in it, white on the dark cells and left out where
// it does not fit. The pair under the pointer or the keys (which step
// across the grid) is ringed and read out over the plot, the other cells
// fading; another lookback glides every cell to its new color.
func correlationCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	lookback := core.NewSignal(cx, "1Y")
	hover := core.NewSignal(cx, -1)
	cells := func() []holdingPair { return pairCells[lookback.Get()] }
	shown := func() (holdingPair, bool) {
		if h := hover.Get(); h >= 0 && h < len(cells()) {
			return cells()[h], true
		}
		return holdingPair{}, false
	}
	pairs := func() []holdingPair { return pairsOf(cells()) }
	mean := func() float64 {
		s := 0.0
		for _, p := range pairs() {
			s += p.r
		}
		return s / float64(len(pairs()))
	}
	closest := func() holdingPair { return slices.MaxFunc(pairs(), func(a, b holdingPair) int { return cmpFloat(a.r, b.r) }) }
	hedge := func() holdingPair { return slices.MinFunc(pairs(), func(a, b holdingPair) int { return cmpFloat(a.r, b.r) }) }
	kinship := func(p holdingPair) string {
		switch {
		case p.a == p.b:
			return tr.ChartsItself()
		case p.r >= 0.7:
			return tr.ChartsMoveClosely()
		case p.r >= 0.4:
			return tr.ChartsMoveTogether()
		case p.r >= 0.15:
			return tr.ChartsLooselyRelated()
		case p.r > -0.15:
			return tr.ChartsBarelyRelated()
		}
		return tr.ChartsMoveApart()
	}
	width := core.NewSignal(cx, core.Points(0))
	// A phone's narrow card: smaller tickers, and the coefficients leave the
	// cells too small for them.
	compact := core.NewMemo(cx, func() bool { w := width.Get(); return w > 0 && w < 320 })
	tick := func() core.Style {
		size := core.Points(10.5)
		if compact.Get() {
			size = 9
		}
		return axisText(cx).Merge(core.Style{FontSize: size, Color: colors.Muted()})
	}
	gutter := func() core.Points {
		if compact.Get() {
			return 34
		}
		return 40
	}
	// The negatives in the warning tone, apart from the accent's positives.
	scale := func() []core.Color { return []core.Color{colors.Warn(), colors.Line(), chartSeries(cx)[0]} }
	band := func() plot.Scale { return plot.BandScale(plot.Band{Domain: pairTickers, PaddingInner: 0.06}) }
	return Card{TestID: "card-correlation", Title: tr.ChartsCorrelation(), Description: tr.ChartsCorrelationNote(), Children: []core.Node{
		readout(cx,
			func() string {
				if p, ok := shown(); ok {
					return p.a + " × " + p.b
				}
				return upper(tr.ChartsHoldings(itoa(len(pairTickers)))) + " · " + lookback.Get()
			},
			func() string {
				if p, ok := shown(); ok {
					return minus(p.r, 2)
				}
				return minus(mean(), 2)
			}, nil,
			func() string {
				if p, ok := shown(); ok {
					return kinship(p)
				}
				return tr.ChartsMeanOfPairs(itoa(len(pairs())))
			},
			note(cx, func() string {
				switch _, ok := shown(); {
				case !ok:
					return tr.ChartsTheLower()
				case lookback.Get() == "1Y":
					return tr.ChartsDailyOverAYear()
				}
				return tr.ChartsDailyOverThreeYears()
			}, colors.Faint)),
		ramp(cx, scale, constant3("−1", "0", "+1"), core.Style{Margin: core.Edges{Top: 10}}),
		core.View{Style: core.Style{Margin: core.Edges{Top: chartGap() + 4}}, OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: []core.Node{plot.Plot{
			TestID:     "correlation-plot",
			Count:      func() int { return len(cells()) },
			XScale:     band,
			YScale:     band,
			ColorScale: func() plot.ColorScale { return plot.DivergingColors(scale()...).Over(-1, 1) },
			AppearGeneration: func() any { return 1 },
			Motion: func() plot.Motion {
				m := chartMotion(ms(900), ms(650), false)
				m.Exit.Duration, m.Pointer = ms(180), &core.SpringConfig{Stiffness: 380, Damping: 34}
				return m
			}(),
			LabelFunc: func() string {
				c, h := closest(), hedge()
				return tr.ChartsCorrelationLabel(itoa(len(pairTickers)), lookback.Get(), c.a, c.b, minus(c.r, 2), h.a, h.b, minus(h.r, 2))
			},
			AccessibilityValue: func() core.AccessibilityValue {
				if p, ok := shown(); ok {
					return core.AccessibilityValue{Text: tr.ChartsPairValue(p.a, p.b, minus(p.r, 2))}
				}
				return core.AccessibilityValue{}
			},
			OnHoverChange: hover.Set,
			StyleFunc: func() core.Style {
				return core.Style{AspectRatio: 1, Margin: core.Edges{Left: gutter(), Top: 20}}
			},
			Children: []core.Node{
				core.Dynamic(func(cx *core.Scope) core.Node {
					size := core.Points(10)
					if compact.Get() {
						size = 8.5
					}
					return plot.Cell{X: func() []string { return dirs(cells(), func(p holdingPair) string { return p.b }) }, Y: func() []string { return dirs(cells(), func(p holdingPair) string { return p.a }) },
						Value: func() []float64 { return column(cells(), func(p holdingPair) float64 { return p.r }) }, Corners: plot.AllCorners(3), Stagger: 0.5, HoverDim: 0.65,
						Label: func() []string { return dirs(cells(), func(p holdingPair) string { return minus(p.r, 2) }) },
						LabelStyle: core.Style{FontSize: size, FontVariant: core.FontTabularNums, Color: cellLabel}, DarkLabelStyle: darkCellLabel,
						HoverCellStyleFunc: ringStyle(cx, 4)}
				}),
				plot.Axis{Placement: plot.AxisTop, LabelStyleFunc: func() core.Style { return tick().Merge(core.Style{LineHeight: 14, Padding: core.Edges{Bottom: 6}}) }},
				plot.Axis{Placement: plot.AxisLeft, LabelStyleFunc: func() core.Style {
					return tick().Merge(core.Style{Width: core.Pt(gutter()), Padding: core.Edges{Right: 8}, TextAlign: core.TextAlignEnd})
				}},
			},
		}}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsClosest(), func() string { c := closest(); return c.a + "·" + c.b + " " + minus(c.r, 2) }, nil),
			chartFigure(cx, tr.ChartsBestHedge(), func() string { h := hedge(); return h.a + "·" + h.b + " " + minus(h.r, 2) }, func() Tone {
				if hedge().r < 0 {
					return ToneDown
				}
				return ToneNeutral
			}),
			chartFigure(cx, tr.ChartsMean(), func() string { return fixed(mean(), 2) }, nil)),
		chartSwitch(tr.ChartsLookback(), 180, lookback.Get, lookback.Set,
			ui.Option[string]{Value: "1Y", Label: tr.Charts1Year()}, ui.Option[string]{Value: "3Y", Label: tr.Charts3Years()}),
	}}
}

// constant3 is labels that never change.
func constant3(labels ...string) func() []string { return func() []string { return labels } }

// calYears are the years of the returns calendar; this year's months run
// up to the last full one.
var (
	calYears  = []string{"2020", "2021", "2022", "2023", "2024", "2025", "2026"}
	monthsNow = 10
	months    = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
)

// drift, swing and shocks are each year's mean monthly return, a month's
// swing around it, and the months markets remember.
var (
	calDrift = map[string][]float64{"SPX": {1.3, 2.1, -1.6, 1.8, 1.8, 1.3, 0.9}, "NVDA": {5, 4.5, -4, 7, 5, 3.5, 2}}
	calSwing = map[string]float64{"SPX": 7, "NVDA": 26}
	calShock = map[string]map[string]float64{
		"SPX":  {"2020 Mar": -12.5, "2020 Apr": 12.7, "2022 Jun": -8.4, "2022 Sep": -9.3, "2022 Oct": 8, "2023 Nov": 8.9, "2025 Apr": -0.8},
		"NVDA": {"2020 Mar": -2.4, "2022 Sep": -19.6, "2023 May": 36.3, "2024 Feb": 28.6, "2025 Apr": 0.4},
	}
)

// monthReturn is a month's return, in percent.
type monthReturn struct {
	year, month string
	ret         float64
}

func monthly(symbol string) []monthReturn {
	seed := 29.0
	if symbol == "SPX" {
		seed = 7
	}
	r := chartRandom(seed)
	var out []monthReturn
	for y, year := range calYears {
		n := 12
		if year == "2026" {
			n = monthsNow
		}
		for _, m := range months[:n] {
			ret := calDrift[symbol][y] + (r()-0.5)*calSwing[symbol]
			if shock, ok := calShock[symbol][year+" "+m]; ok {
				ret = shock
			}
			out = append(out, monthReturn{year, m, math.Round(ret*10) / 10})
		}
	}
	return out
}

var monthlyReturns = map[string][]monthReturn{"SPX": monthly("SPX"), "NVDA": monthly("NVDA")}

// yearReturn is a year's return: its months compounded.
func yearReturn(rows []monthReturn, year string) float64 {
	p := 1.0
	for _, r := range rows {
		if r.year == year {
			p *= 1 + r.ret/100
		}
	}
	return (p - 1) * 100
}

// tenths is a cell's text: the return without its percent sign, a tenth
// of a percent below 10 and whole above, so a big month fits its cell as
// the rest do.
func tenths(v float64) string {
	if math.Abs(v) >= 10 {
		return minus(v, 0)
	}
	return minus(v, 1)
}

// seasonalityCard is a calendar of monthly returns: a Cell per month in
// the band of its month across (the x scale's domain, January to December,
// so this year's months to come stay blank) and of its year down, red for
// a loss and green for a gain along a diverging ColorScale about 0 over the
// largest move either way, keyed by a ramp under the header. The months
// are the top axis' labels, the years the left one's, and each year's
// return the right one's. The month under the pointer or the keys is read
// out; another symbol glides every month to its new color.
func seasonalityCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	symbol := core.NewSignal(cx, "SPX")
	hover := core.NewSignal(cx, -1)
	rows := func() []monthReturn { return monthlyReturns[symbol.Get()] }
	shown := func() (monthReturn, bool) {
		if h := hover.Get(); h >= 0 && h < len(rows()) {
			return rows()[h], true
		}
		return monthReturn{}, false
	}
	name := func() string {
		if symbol.Get() == "SPX" {
			return "S&P 500"
		}
		return symbol.Get()
	}
	// The scale's own domain: the largest move either way, even about 0.
	reach := func() float64 { return slices.Max(column(rows(), func(r monthReturn) float64 { return math.Abs(r.ret) })) }
	total := func() float64 {
		p := 1.0
		for _, r := range rows() {
			p *= 1 + r.ret/100
		}
		return (p - 1) * 100
	}
	best := func() monthReturn {
		return slices.MaxFunc(rows(), func(a, b monthReturn) int { return cmpFloat(a.ret, b.ret) })
	}
	worst := func() monthReturn {
		return slices.MinFunc(rows(), func(a, b monthReturn) int { return cmpFloat(a.ret, b.ret) })
	}
	gains := func() int {
		n := 0
		for _, r := range rows() {
			if r.ret > 0 {
				n++
			}
		}
		return n
	}
	width := core.NewSignal(cx, core.Points(0))
	// A phone's narrow card: smaller labels, the returns left out of cells
	// too small for them.
	compact := core.NewMemo(cx, func() bool { w := width.Get(); return w > 0 && w < 340 })
	tick := func() core.Style {
		size := core.Points(10.5)
		if compact.Get() {
			size = 9
		}
		return axisText(cx).Merge(core.Style{FontSize: size})
	}
	gutter := func() core.Points {
		if compact.Get() {
			return 34
		}
		return 36
	}
	totals := func() core.Points {
		if compact.Get() {
			return 46
		}
		return 54
	}
	scale := func() []core.Color { return []core.Color{colors.Down(), colors.Line(), colors.Up()} }
	month := func(r monthReturn) string { return r.month + " " + r.year }
	return Card{TestID: "card-seasonality", Title: tr.ChartsSeasonality(), Description: tr.ChartsSeasonalityNote(), Children: []core.Node{
		readout(cx,
			func() string {
				if r, ok := shown(); ok {
					return upper(month(r))
				}
				return upper(name()) + " · 2020–2026"
			},
			func() string {
				if r, ok := shown(); ok {
					return pct(r.ret, 1)
				}
				return pct(total(), 1)
			},
			func() core.Color {
				r, ok := shown()
				switch {
				case !ok:
					return colors.Text()
				case r.ret >= 0:
					return colors.Up()
				}
				return colors.Down()
			},
			func() string {
				if r, ok := shown(); ok {
					return name() + " · " + r.year + " " + pct(yearReturn(rows(), r.year), 1)
				}
				return tr.ChartsTotalReturn()
			}),
		core.Dynamic(func(cx *core.Scope) core.Node {
			return ramp(cx, scale, func() []string { return []string{"−" + fixed(reach(), 1) + "%", "0", "+" + fixed(reach(), 1) + "%"} },
				core.Style{Margin: core.Edges{Top: 10, Left: gutter(), Right: totals()}})
		}),
		core.View{Style: core.Style{Margin: core.Edges{Top: chartGap() + 4}}, OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: []core.Node{plot.Plot{
			TestID: "seasonality-plot",
			Count:  func() int { return len(rows()) },
			XScale: func() plot.Scale { return plot.BandScale(plot.Band{Domain: months, PaddingInner: 0.1}) },
			YScale: func() plot.Scale { return plot.BandScale(plot.Band{Domain: calYears, PaddingInner: 0.12}) },
			ColorScale: func() plot.ColorScale {
				return plot.DivergingColors(scale()...).Over(-reach(), reach())
			},
			AppearGeneration: func() any { return 1 },
			Motion: func() plot.Motion {
				m := chartMotion(ms(1000), ms(650), false)
				m.Exit.Duration, m.Pointer = ms(180), &core.SpringConfig{Stiffness: 380, Damping: 34}
				return m
			}(),
			LabelFunc: func() string {
				return tr.ChartsSeasonalityLabel(name(), month(best()), pct(best().ret, 1), month(worst()), pct(worst().ret, 1))
			},
			AccessibilityValue: func() core.AccessibilityValue {
				if r, ok := shown(); ok {
					return core.AccessibilityValue{Text: month(r) + ": " + pct(r.ret, 1)}
				}
				return core.AccessibilityValue{}
			},
			OnHoverChange: hover.Set,
			StyleFunc: func() core.Style {
				row := core.Points(27)
				if compact.Get() {
					row = 22
				}
				return core.Style{Height: core.Pt(core.Points(len(calYears)) * row), Margin: core.Edges{Left: gutter(), Right: totals(), Top: 20}}
			},
			Children: []core.Node{
				core.Dynamic(func(cx *core.Scope) core.Node {
					cell := plot.Cell{X: func() []string { return dirs(rows(), func(r monthReturn) string { return r.month }) },
						Y:       func() []string { return dirs(rows(), func(r monthReturn) string { return r.year }) },
						Value:   func() []float64 { return column(rows(), func(r monthReturn) float64 { return r.ret }) },
						Corners: plot.AllCorners(3), Stagger: 0.6, HoverDim: 0.6,
						LabelStyle: core.Style{FontSize: 9.5, FontVariant: core.FontTabularNums, Color: cellLabel}, DarkLabelStyle: darkCellLabel,
						HoverCellStyleFunc: ringStyle(cx, 4)}
					// A phone's cells are only the colors: a gain written in some
					// and a loss left out of others would read as missing months.
					if !compact.Get() {
						cell.Label = func() []string { return dirs(rows(), func(r monthReturn) string { return tenths(r.ret) }) }
					}
					return cell
				}),
				plot.Axis{Placement: plot.AxisTop, LabelStyleFunc: func() core.Style { return tick().Merge(core.Style{LineHeight: 14, Padding: core.Edges{Bottom: 6}}) }},
				plot.Axis{Placement: plot.AxisLeft, LabelStyleFunc: func() core.Style {
					return tick().Merge(core.Style{Width: core.Pt(gutter()), Padding: core.Edges{Right: 8}, TextAlign: core.TextAlignEnd})
				}},
				plot.Axis{Placement: plot.AxisRight, Format: func(v float64, _ int) string { return pct(yearReturn(rows(), calYears[int(v)]), 1) },
					LabelStyleFunc: func() core.Style {
						return tick().Merge(core.Style{Width: core.Pt(totals()), Padding: core.Edges{Left: 8}, TextAlign: core.TextAlignEnd, Color: colors.Muted()})
					}},
			},
		}}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsBestMonth(), func() string { return month(best()) + " " + pct(best().ret, 1) }, fixedTone(ToneUp)),
			chartFigure(cx, tr.ChartsWorstMonth(), func() string { return month(worst()) + " " + pct(worst().ret, 1) }, fixedTone(ToneDown)),
			chartFigure(cx, tr.ChartsUpMonths(), func() string { return itoa(gains()) + " / " + itoa(len(rows())) }, nil)),
		chartSwitch(tr.ChartsSymbol(), 180, symbol.Get, symbol.Set,
			ui.Option[string]{Value: "SPX", Label: "S&P 500"}, ui.Option[string]{Value: "NVDA", Label: "NVDA"}),
	}}
}

// slots are the half hours of a trading day, from the open.
var (
	slots    = []string{"9:30", "10:00", "10:30", "11:00", "11:30", "12:00", "12:30", "13:00", "13:30", "14:00", "14:30", "15:00", "15:30"}
	weekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri"}
	// intraday is how busy each half hour is against the open: busy at
	// either end of the day, quiet in its middle, a market's lunch closed
	// (NaN).
	intraday = map[string][]float64{
		"US": {1, 0.74, 0.6, 0.52, 0.46, 0.42, 0.4, 0.41, 0.45, 0.5, 0.58, 0.72, 1.05},
		"HK": {1, 0.7, 0.56, 0.5, 0.58, math.NaN(), math.NaN(), 0.66, 0.5, 0.46, 0.5, 0.6, 0.84},
	}
	// busy is each weekday against Tuesday: Fridays busiest.
	busy = []float64{0.92, 1, 0.97, 1.03, 1.1}
	// opening is thousands of trades in the open's half hour.
	opening = map[string]float64{"US": 46, "HK": 21}
)

// halfHour is a half hour's trades, in thousands.
type halfHour struct {
	day, slot string
	trades    float64
}

// activity is a week of a market's trades per half hour: a row per half
// hour it is open.
func activity(market string) []halfHour {
	seed := 13.0
	if market == "US" {
		seed = 5
	}
	r := chartRandom(seed)
	var out []halfHour
	for d, day := range weekdays {
		for i, slot := range slots {
			shape := intraday[market][i]
			if math.IsNaN(shape) {
				continue
			}
			out = append(out, halfHour{day, slot, math.Round(opening[market]*shape*busy[d]*(0.9+r()*0.2)*10) / 10})
		}
	}
	return out
}

var activities = map[string][]halfHour{"US": activity("US"), "HK": activity("HK")}

// thousandsOf writes thousands of trades.
func thousandsOf(v float64) string { return fixed(v, 1) + "k" }

// slotEnd is the half hour after slot: its end.
func slotEnd(slot string) string {
	h, m, _ := strings.Cut(slot, ":")
	hour, _ := strconv.Atoi(h)
	if m == "30" {
		return itoa(hour+1) + ":00"
	}
	return h + ":30"
}

// activityCard is when a market trades: a week of trades per half hour as
// a heatmap, a Cell per half hour a market is open, colored along a
// sequential ColorScale from the quietest (the card's line) to the busiest
// (the accent), with round corners and no text. The x scale's domain is
// every half hour of the day, so Hong Kong's lunch break stays blank, and
// its cells shrink away when the market changes while the others glide to
// their new colors. The half hour under the pointer or the keys is ringed
// and read out.
func activityCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	market := core.NewSignal(cx, "US")
	hover := core.NewSignal(cx, -1)
	rows := func() []halfHour { return activities[market.Get()] }
	shown := func() (halfHour, bool) {
		if h := hover.Get(); h >= 0 && h < len(rows()) {
			return rows()[h], true
		}
		return halfHour{}, false
	}
	dayTotal := func(day string) float64 {
		s := 0.0
		for _, r := range rows() {
			if r.day == day {
				s += r.trades
			}
		}
		return s
	}
	perDay := func() float64 {
		s := 0.0
		for _, r := range rows() {
			s += r.trades
		}
		return s / float64(len(weekdays))
	}
	busiest := func() halfHour {
		return slices.MaxFunc(rows(), func(a, b halfHour) int { return cmpFloat(a.trades, b.trades) })
	}
	quietest := func() halfHour {
		return slices.MinFunc(rows(), func(a, b halfHour) int { return cmpFloat(a.trades, b.trades) })
	}
	meanAt := func(slot string) float64 {
		s, n := 0.0, 0
		for _, r := range rows() {
			if r.slot == slot {
				s, n = s+r.trades, n+1
			}
		}
		return s / float64(n)
	}
	width := core.NewSignal(cx, core.Points(0))
	compact := core.NewMemo(cx, func() bool { w := width.Get(); return w > 0 && w < 340 })
	tick := func() core.Style {
		size := core.Points(10.5)
		if compact.Get() {
			size = 9
		}
		return axisText(cx).Merge(core.Style{FontSize: size})
	}
	scale := func() []core.Color { return []core.Color{colors.Line(), chartSeries(cx)[0]} }
	return Card{TestID: "card-activity", Title: tr.ChartsActivity(), Description: tr.ChartsActivityNote(), Children: []core.Node{
		readout(cx,
			func() string {
				if h, ok := shown(); ok {
					return upper(h.day) + " " + h.slot + "–" + slotEnd(h.slot)
				}
				return market.Get() + " · " + upper(tr.ChartsTradesPerHalfHour())
			},
			func() string {
				if h, ok := shown(); ok {
					return thousandsOf(h.trades)
				}
				return thousandsOf(perDay())
			}, nil,
			func() string {
				if h, ok := shown(); ok {
					return tr.ChartsTradesOfDay(fixed(h.trades/dayTotal(h.day)*100, 1)+"%", h.day)
				}
				return tr.ChartsTradesADay()
			},
			note(cx, func() string {
				if market.Get() == "HK" {
					return tr.ChartsHoursHK()
				}
				return tr.ChartsHoursUS()
			}, colors.Faint)),
		ramp(cx, scale, func() []string {
			return []string{tr.ChartsQuietest(thousandsOf(quietest().trades)), tr.ChartsBusiest(thousandsOf(busiest().trades))}
		}, core.Style{Margin: core.Edges{Top: 10, Left: 36}}),
		core.View{Style: core.Style{Margin: core.Edges{Top: chartGap() + 4}}, OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: []core.Node{plot.Plot{
			TestID: "activity-plot",
			Count:  func() int { return len(rows()) },
			XScale: func() plot.Scale { return plot.BandScale(plot.Band{Domain: slots, PaddingInner: 0.14}) },
			YScale: func() plot.Scale { return plot.BandScale(plot.Band{Domain: weekdays, PaddingInner: 0.14}) },
			ColorScale: func() plot.ColorScale { return plot.SequentialColors(scale()...) },
			AppearGeneration: func() any { return 1 },
			Motion: func() plot.Motion {
				m := chartMotion(ms(900), ms(600), false)
				m.Exit.Duration, m.Pointer = ms(180), &core.SpringConfig{Stiffness: 380, Damping: 34}
				return m
			}(),
			LabelFunc: func() string {
				b, q := busiest(), quietest()
				return tr.ChartsActivityLabel(market.Get(), b.day, b.slot, thousandsOf(b.trades), q.day, q.slot)
			},
			AccessibilityValue: func() core.AccessibilityValue {
				if h, ok := shown(); ok {
					return core.AccessibilityValue{Text: tr.ChartsActivityValue(h.day, h.slot, thousandsOf(h.trades))}
				}
				return core.AccessibilityValue{}
			},
			OnHoverChange: hover.Set,
			StyleFunc: func() core.Style {
				row := core.Points(28)
				if compact.Get() {
					row = 24
				}
				return core.Style{Height: core.Pt(core.Points(len(weekdays)) * row), Margin: core.Edges{Left: 36, Top: 20}}
			},
			Children: []core.Node{
				plot.Cell{X: func() []string { return dirs(rows(), func(h halfHour) string { return h.slot }) },
					Y:       func() []string { return dirs(rows(), func(h halfHour) string { return h.day }) },
					Value:   func() []float64 { return column(rows(), func(h halfHour) float64 { return h.trades }) },
					Corners: plot.AllCorners(4), Stagger: 0.6, HoverDim: 0.6, HoverCellStyleFunc: ringStyle(cx, 5)},
				plot.Axis{Placement: plot.AxisTop, TickValues: func() []float64 {
					if compact.Get() {
						return []float64{0, 4, 8, 12}
					}
					return []float64{0, 2, 4, 6, 8, 10, 12}
				}, LabelStyleFunc: func() core.Style { return tick().Merge(core.Style{LineHeight: 14, Padding: core.Edges{Bottom: 6}}) }},
				plot.Axis{Placement: plot.AxisLeft, LabelStyleFunc: func() core.Style {
					return tick().Merge(core.Style{Width: core.Pt(36), Padding: core.Edges{Right: 8}, TextAlign: core.TextAlignEnd})
				}},
			},
		}}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsBusiestFigure(), func() string { b := busiest(); return b.day + " " + b.slot }, nil),
			chartFigure(cx, tr.ChartsQuietestFigure(), func() string { q := quietest(); return q.day + " " + q.slot }, nil),
			chartFigure(cx, tr.ChartsOpenVs(), func() string { return fixed(meanAt("9:30")/meanAt("13:30"), 1) + "×" }, nil)),
		chartSwitch(tr.ChartsMarket(), 120, market.Get, market.Set,
			ui.Option[string]{Value: "US", Label: "US"}, ui.Option[string]{Value: "HK", Label: "HK"}),
	}}
}
