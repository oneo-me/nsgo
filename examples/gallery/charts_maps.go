package main

import (
	"math"
	"strings"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// factor is a portfolio's lean on a factor, scored 0 to 100, and two
// benchmarks'.
type factor struct {
	name               string
	ours, index, peers float64
}

var factors = []factor{
	{"Value", 72, 46, 64}, {"Momentum", 58, 62, 40}, {"Quality", 86, 55, 71},
	{"Size", 34, 70, 52}, {"Low vol", 64, 42, 78}, {"Yield", 48, 52, 66},
}

// factorLead is the factor the portfolio leans on most.
var factorLead = func() factor {
	lead := factors[0]
	for _, f := range factors {
		if f.ours > lead.ours {
			lead = f
		}
	}
	return lead
}()

// labelAt is where the factors' names sit: a share of the plot's side out
// from its middle.
const labelAt = 44

// away is a name's side of its place: away from the center.
func away(v float64) core.Align {
	switch {
	case v > 0.01:
		return core.AlignStart
	case v < -0.01:
		return core.AlignEnd
	}
	return core.AlignCenter
}

// factorsCard is a portfolio's factor exposure as a radar against a
// benchmark: hexagonal hairline rings and spokes, the benchmark a dashed
// outline, the portfolio an accent outline over a wash that deepens toward
// the rim, growing out of the center and morphing to the other benchmark
// (the plot's update). The factor nearest the pointer by angle, or the
// keys' factor, is read out beside it with both scores and the spread, its
// name lit at the rim.
func factorsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	peers := core.NewSignal(cx, false)
	hover := core.NewSignal(cx, -1)
	shown := func() (factor, bool) {
		if h := hover.Get(); h >= 0 && h < len(factors) {
			return factors[h], true
		}
		return factor{}, false
	}
	theirsOf := func(f factor) float64 {
		if peers.Get() {
			return f.peers
		}
		return f.index
	}
	mean := func(pick func(factor) float64) float64 {
		s := 0.0
		for _, f := range factors {
			s += pick(f)
		}
		return s / float64(len(factors))
	}
	ours := func() float64 {
		if f, ok := shown(); ok {
			return f.ours
		}
		return mean(func(f factor) float64 { return f.ours })
	}
	theirs := func() float64 {
		if f, ok := shown(); ok {
			return theirsOf(f)
		}
		return mean(theirsOf)
	}
	spread := func() float64 { return ours() - theirs() }
	benchName := func() string {
		if peers.Get() {
			return tr.ChartsPeers()
		}
		return tr.ChartsIndex()
	}
	side := core.Points(204)
	if chartPointer() {
		side = 196
	}
	rim := func() core.Length { return core.Pct(78) }
	accent := chartSeriesColor(cx, 0)
	children := []core.Node{
		plot.RadialGrid{Rings: func() []float64 { return []float64{25, 50, 75} }, OuterRadius: rim, StyleFunc: gridStyle(cx)},
		plot.RadialGrid{Rings: func() []float64 { return []float64{100} }, Spokes: true, OuterRadius: rim, StyleFunc: baselineStyle(cx)},
		plot.RadialLine{Radius: func() []float64 { return column(factors, theirsOf) }, Closed: true, OuterRadius: rim,
			StyleFunc: func() core.Style {
				return core.Style{Stroke: core.PaintColor(colors.Muted()), StrokeDash: []core.Points{3, 3}, Fill: core.PaintColor(ui.Alpha(colors.Muted(), 0.06))}
			},
			HoverDotStyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 4, BorderWidth: 1.5, BorderColor: colors.Surface(), Background: colors.Muted()}
			}},
		plot.RadialLine{Radius: func() []float64 { return column(factors, func(f factor) float64 { return f.ours }) }, Closed: true, OuterRadius: rim, Dot: true,
			DotStyleFunc: func() core.Style { return core.Style{Width: core.Pt(4), Height: core.Pt(4), CornerRadius: 2, Background: accent()} },
			BackgroundStyleFunc: func() core.Style {
				return core.Style{Gradients: []core.Gradient{{Kind: core.GradientRadial, Circle: true, Stops: []core.ColorStop{{Color: ui.Alpha(accent(), 0.04)}, {Color: ui.Alpha(accent(), 0.3)}}}}}
			},
			StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(accent()), StrokeWidth: 2} },
			HoverDotStyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, BorderWidth: 2, BorderColor: colors.Surface(), Background: accent()}
			}},
	}
	// Each factor's name past the rim at its angle, growing away from the
	// center; the hovered one lit.
	for i, f := range factors {
		angle := 2 * math.Pi * float64(i) / float64(len(factors))
		sin, cos := math.Sin(angle), math.Cos(angle)
		align := core.TextAlignCenter
		if sin > 0.01 {
			align = core.TextAlignStart
		} else if sin < -0.01 {
			align = core.TextAlignEnd
		}
		children = append(children, core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute,
			Left: core.Pct(float32(50 + labelAt*sin)), Top: core.Pct(float32(50 - labelAt*cos)), Width: core.Pt(0), Height: core.Pt(0),
			AlignItems: away(sin), JustifyContent: core.Justify(away(-cos))}, Children: []core.Node{
			core.Text{Content: f.name, Style: core.Style{Width: core.Pt(64), NumberOfLines: 1, FontSize: 10.5, FontFamily: ui.Mono, TextAlign: align},
				StyleFunc: func() core.Style {
					if hover.Get() == i {
						return core.Style{Color: accent()}
					}
					return core.Style{Color: colors.Faint()}
				}},
		}})
	}
	layout := core.Style{Wrap: core.Wrap, JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter, Gap: 12}
	panel := core.Style{AlignSelf: core.AlignStretch, FlexGrow: 1, JustifyContent: core.JustifyCenter, Gap: 12}
	if chartPointer() {
		layout.Direction, layout.Wrap, layout.Gap, panel.FlexBasis = core.Row, core.NoWrap, 20, core.Pt(0)
	}
	return Card{TestID: "card-factors", Title: tr.ChartsFactors(), Description: tr.ChartsFactorsNote(), Children: []core.Node{
		// The readout beside the radar where the card has room for it, below
		// where it has not.
		core.View{Style: layout, Children: []core.Node{
			plot.Plot{
				TestID:           "factors-plot",
				Count:            func() int { return len(factors) },
				YScale:           func() plot.Scale { return plot.LinearScale(0, 100) },
				AppearGeneration: func() any { return 1 },
				Motion: func() plot.Motion {
					m := chartMotion(ms(900), ms(550), false)
					m.Exit.Duration, m.Pointer = ms(180), &core.SpringConfig{Stiffness: 380, Damping: 34}
					return m
				}(),
				LabelFunc: func() string {
					return tr.ChartsFactorsLabel(strings.ToLower(benchName()), factorLead.name, fixed(factorLead.ours, 0))
				},
				AccessibilityValue: func() core.AccessibilityValue {
					if f, ok := shown(); ok {
						return core.AccessibilityValue{Text: f.name + ": " + fixed(f.ours, 0) + ", " + strings.ToLower(benchName()) + " " + fixed(theirsOf(f), 0)}
					}
					return core.AccessibilityValue{}
				},
				OnHoverChange: hover.Set,
				Style:         core.Style{Width: core.Pt(side), Height: core.Pt(side), Margin: core.Symmetric(10, 40)},
				Children:      children,
			},
			core.View{Style: panel, Children: []core.Node{
				core.View{Style: core.Style{Gap: 2}, Children: []core.Node{
					tipTitleText(cx, func() string {
						if f, ok := shown(); ok {
							return upper(f.name)
						}
						return upper(tr.ChartsAllSixFactors())
					}),
					baseline(8,
						core.Text{ContentFunc: func() string { return fixed(ours(), 1) }, Style: core.Style{FontSize: 26, FontWeight: ui.Semibold, LetterSpacing: -0.6, FontVariant: core.FontTabularNums},
							StyleFunc: ui.Colored(colors.Text)},
						ui.Textf(func() string {
							if _, ok := shown(); ok {
								return tr.ChartsScore()
							}
							return tr.ChartsMeanScore()
						}, 12, ui.Regular, colors.Muted)),
				}},
				core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
					chartLegend(cx, accent, constant(tr.ChartsPortfolio()), func() string { return fixed(ours(), 1) }),
					chartLegend(cx, colors.Muted, benchName, func() string { return fixed(theirs(), 1) }),
				}},
				chartFigures(cx, 0,
					chartFigure(cx, tr.ChartsSpread(), func() string { return sign(spread(), 1) }, func() Tone { return upDown(spread()) }),
					chartFigure(cx, tr.ChartsLead(), constant(factorLead.name), nil)),
			}},
		}},
		chartSwitch(tr.ChartsBenchmark(), 220, peers.Get, peers.Set,
			ui.Option[bool]{Value: false, Label: tr.ChartsVsIndex()}, ui.Option[bool]{Value: true, Label: tr.ChartsVsPeers()}),
	}}
}

// accounts are a quarter's income accounts; flowPairs what flows between
// them: two segments into revenue, revenue into gross profit and its cost,
// gross profit into operating income and expenses, operating income into
// net income and tax.
var (
	accounts  = []string{"Products", "Services", "Revenue", "Gross profit", "Cost", "Operating", "OpEx", "Net income", "Tax"}
	flowPairs = [][2]int{{0, 2}, {1, 2}, {2, 3}, {2, 4}, {3, 5}, {3, 6}, {5, 7}, {5, 8}}
	// flowed is what flows along each pair, in billions, per quarter.
	flowed = map[string][]float64{
		"Q3": {61.6, 23.9, 39.7, 45.8, 23.4, 16.3, 19.9, 3.5},
		"Q4": {68.2, 25.1, 43.6, 49.7, 27.0, 16.6, 23.1, 3.9},
	}
	// accountSeries is each account's series color: what is kept in the
	// accent, the segments in graphite, what is spent quiet.
	accountSeries = []int{1, 2, 0, 0, 3, 0, 3, 0, 3}
)

const (
	// flowLeft and flowRight are the room left of the segments' names and
	// right of the results'.
	flowLeft  = 58
	flowRight = 64
	// flowNarrow is the distance between columns below which two-word names
	// break over two lines, so names over neighbouring columns do not run
	// into each other.
	flowNarrow = 72
)

// through is what flows through account i: the most into or out of it.
func through(q string, i int) float64 {
	var in, out float64
	for k, p := range flowPairs {
		if p[0] == i {
			out += flowed[q][k]
		}
		if p[1] == i {
			in += flowed[q][k]
		}
	}
	return max(in, out)
}

// billions writes billions of dollars.
func billions(v float64) string { return "$" + fixed(v, 1) + "B" }

// flowsCard is two quarters' income as flows: each account a hairline node
// named beside it, each flow a ribbon fading from its source's color to its
// target's, what is kept in the accent and what is spent quiet, drawn in
// from the left. The account under the pointer or the arrow keys is read
// out over the flows, the links away from it fading; the quarters' flows
// move into each other.
func flowsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	quarter := core.NewSignal(cx, "Q3")
	hover := core.NewSignal(cx, -1)
	revenue := func() float64 { return through(quarter.Get(), 2) }
	shown := func() int {
		if h := hover.Get(); h >= 0 && h < len(accounts) {
			return h
		}
		return 2
	}
	margin := func(i int) string { return fixed(through(quarter.Get(), i)/revenue()*100, 1) + "%" }
	// The distance between the five columns, once laid out.
	column := core.NewSignal(cx, core.Points(math.Inf(1)))
	shade := func(i int) core.Color { return chartSeries(cx)[accountSeries[i]] }
	links := func() []plot.SankeyLink {
		out := make([]plot.SankeyLink, len(flowPairs))
		for k, p := range flowPairs {
			out[k] = plot.SankeyLink{Source: p[0], Target: p[1], Value: flowed[quarter.Get()][k]}
		}
		return out
	}
	return Card{TestID: "card-flows", Title: tr.ChartsFlows(), Description: tr.ChartsFlowsNote(), Children: []core.Node{
		readout(cx,
			func() string { return upper(accounts[shown()]) },
			func() string { return billions(through(quarter.Get(), shown())) }, nil,
			func() string { return tr.ChartsOfQuarterRevenue(margin(shown()), quarter.Get()) }),
		core.View{Style: core.Style{Margin: core.Edges{Bottom: 6}}, OnLayout: func(b core.Bounds) { column.Set((b.Size.Width - flowLeft - flowRight - 3) / 4) }, Children: []core.Node{plot.Plot{
			TestID:           "flows-plot",
			Count:            func() int { return len(accounts) },
			AppearGeneration: func() any { return 1 },
			Motion: func() plot.Motion {
				m := chartMotion(ms(1100), ms(600), false)
				m.Exit.Duration, m.Pointer = ms(180), &core.SpringConfig{Stiffness: 420, Damping: 40}
				return m
			}(),
			LabelFunc: func() string {
				return tr.ChartsFlowsLabel(quarter.Get(), billions(revenue()), billions(through(quarter.Get(), 7)), margin(7))
			},
			AccessibilityValue: func() core.AccessibilityValue {
				if h := hover.Get(); h >= 0 {
					return core.AccessibilityValue{Text: accounts[h] + ": " + billions(through(quarter.Get(), h)) + ", " + tr.ChartsOfRevenue(margin(h))}
				}
				return core.AccessibilityValue{}
			},
			OnHoverChange: func(i int) { hover.Set(i) },
			// Room for the segments' names left of them, the results' right and
			// the rest's above.
			Style: func() core.Style {
				height := core.Points(220)
				if chartPointer() {
					height = 196
				}
				return core.Style{Height: core.Pt(height), Margin: core.Edges{Top: chartGap() + 18, Left: flowLeft, Right: flowRight}}
			}(),
			Children: []core.Node{plot.Sankey{Links: links, NodeAlign: plot.SankeyLeft, NodeWidth: 3, NodePadding: 20, NodeCornerRadius: 1.5, HoverDim: 0.7,
				NodeStyleOf: func(row int) core.Style { return core.Style{Fill: core.PaintColor(shade(row))} },
				LinkBackgroundStyleOf: func(link int) core.Style {
					p := flowPairs[link]
					return core.Style{Gradients: []core.Gradient{{Angle: 90, Stops: []core.ColorStop{{Color: ui.Alpha(shade(p[0]), 0.3)}, {Color: ui.Alpha(shade(p[1]), 0.3)}}}}}
				},
				NodeLabel: func() []string {
					if column.Get() >= flowNarrow {
						return accounts
					}
					out := make([]string, len(accounts))
					for i, a := range accounts {
						out[i] = strings.Replace(a, " ", "\n", 1)
					}
					return out
				},
				NodeLabelStyleFunc: axisColored(cx, core.Style{Padding: core.All(5), NumberOfLines: 2}, colors.Muted),
			}},
		}}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsRevenueStep(), func() string { return billions(revenue()) }, nil),
			chartFigure(cx, tr.ChartsGrossMargin(), func() string { return margin(3) }, nil),
			chartFigure(cx, tr.ChartsOperating(), func() string { return margin(5) }, nil),
			chartFigure(cx, tr.ChartsNetMargin(), func() string { return margin(7) }, fixedTone(ToneUp))),
		chartSwitch(tr.ChartsQuarter(), 160, quarter.Get, func(q string) {
			cx.Batch(func() {
				quarter.Set(q)
				hover.Set(-1)
			})
		}, ui.Option[string]{Value: "Q3", Label: "Q3"}, ui.Option[string]{Value: "Q4", Label: "Q4"}),
	}}
}

// listed is a listed stock: its market cap in billions at the start of the
// periods and its change over a day, a week and a month, in percent.
type listed struct {
	ticker, name string
	cap          float64
	change       [3]float64
}

var marketSectors = []struct {
	sector string
	stocks []listed
}{
	{"Technology", []listed{
		{"NVDA", "NVIDIA", 4300, [3]float64{2.41, 5.8, 11.2}}, {"MSFT", "Microsoft", 3700, [3]float64{-0.62, 1.4, 4.1}},
		{"AAPL", "Apple", 3600, [3]float64{0.35, -1.9, 6.3}}, {"AVGO", "Broadcom", 1600, [3]float64{3.12, 7.4, 14.8}},
		{"ORCL", "Oracle", 720, [3]float64{-2.84, -5.1, -9.6}}, {"AMD", "AMD", 380, [3]float64{1.76, 9.2, 3.4}},
		{"CRM", "Salesforce", 240, [3]float64{-1.21, -3.3, -7.8}}, {"ADBE", "Adobe", 150, [3]float64{-0.44, 0.6, -4.2}},
	}},
	{"Communication", []listed{
		{"GOOGL", "Alphabet", 3100, [3]float64{1.05, 3.2, 8.7}}, {"META", "Meta", 1800, [3]float64{-1.48, 2.1, -3.6}},
		{"NFLX", "Netflix", 480, [3]float64{0.22, -2.7, 5.1}}, {"DIS", "Disney", 200, [3]float64{-0.31, -1.1, 1.9}},
	}},
	{"Consumer", []listed{
		{"AMZN", "Amazon", 2400, [3]float64{0.86, -0.8, 2.6}}, {"TSLA", "Tesla", 1400, [3]float64{-3.65, -8.4, 18.9}},
		{"COST", "Costco", 420, [3]float64{0.12, 0.9, -1.4}}, {"WMT", "Walmart", 820, [3]float64{0.48, 1.6, 3.8}},
		{"HD", "Home Depot", 380, [3]float64{-0.73, -2.2, -4.9}}, {"MCD", "McDonald's", 220, [3]float64{0.05, 0.4, 1.1}},
	}},
	{"Financials", []listed{
		{"BRK.B", "Berkshire", 1050, [3]float64{0.18, 0.7, 2.2}}, {"JPM", "JPMorgan", 830, [3]float64{1.34, 2.9, 6.6}},
		{"V", "Visa", 670, [3]float64{-0.27, 1.2, 3.1}}, {"MA", "Mastercard", 520, [3]float64{-0.39, 0.8, 2.4}},
		{"BAC", "Bank of America", 390, [3]float64{1.92, 3.8, 7.9}}, {"GS", "Goldman Sachs", 240, [3]float64{2.65, 4.4, 9.8}},
	}},
	{"Healthcare", []listed{
		{"LLY", "Eli Lilly", 820, [3]float64{-1.94, -4.6, 7.2}}, {"UNH", "UnitedHealth", 300, [3]float64{-4.12, -9.3, -16.4}},
		{"JNJ", "Johnson & Johnson", 460, [3]float64{0.41, 1.3, 2.8}}, {"ABBV", "AbbVie", 380, [3]float64{-0.58, 0.2, -2.1}},
		{"MRK", "Merck", 210, [3]float64{-1.07, -2.8, -6.3}},
	}},
	{"Energy", []listed{
		{"XOM", "Exxon Mobil", 470, [3]float64{-0.96, 1.8, -3.2}}, {"CVX", "Chevron", 300, [3]float64{-1.33, 0.9, -4.7}},
		{"COP", "ConocoPhillips", 120, [3]float64{-2.18, -0.6, -8.1}},
	}},
}

// mapRange is how far a period's changes run before their color is full:
// a day's 3%, a month's 12%.
var mapRange = []float64{3, 6, 12}

// mapRow is a market map's row: a sector (no parent), or a stock in one,
// its cap at the end of the period and its change over it.
type mapRow struct {
	id, parent, name string
	cap, chg         float64
}

func mapRows(period int) []mapRow {
	var out []mapRow
	for _, m := range marketSectors {
		out = append(out, mapRow{id: m.sector, name: m.sector, cap: math.NaN(), chg: math.NaN()})
	}
	for _, m := range marketSectors {
		for _, s := range m.stocks {
			c := s.change[period]
			out = append(out, mapRow{s.ticker, m.sector, s.name, s.cap * (1 + c/100), c})
		}
	}
	return out
}

// marketCard is a market map: the S&P 500's largest stocks as a Treemap of
// sectors, each stock's area its market cap and its color the period's
// change along a diverging ColorScale (losses red, gains green, flat the
// card's inset gray), its ticker and change centered on it where they fit,
// light on the dark tiles. Each sector's name sits in a band over its
// stocks. The stock or sector under the pointer or the arrow keys is read
// out over the map, the others fading; another period recolors the tiles
// and moves them to their new caps.
func marketCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	period := core.NewSignal(cx, 0)
	hover := core.NewSignal(cx, -1)
	rows := core.NewMemoFunc(cx, func() []mapRow { return mapRows(period.Get()) }, nil)
	periods := []string{"1D", "1W", "1M"}
	stocks := func(sector string) []mapRow {
		var out []mapRow
		for _, r := range rows.Get() {
			if r.parent != "" && (sector == "" || r.parent == sector) {
				out = append(out, r)
			}
		}
		return out
	}
	// weighted is a set of stocks' cap-weighted change.
	weighted := func(of []mapRow) float64 {
		var s, c float64
		for _, r := range of {
			s, c = s+r.chg*r.cap, c+r.cap
		}
		return s / c
	}
	capSum := func(of []mapRow) float64 {
		s := 0.0
		for _, r := range of {
			s += r.cap
		}
		return s
	}
	advancing := func() int {
		n := 0
		for _, r := range stocks("") {
			if r.chg > 0 {
				n++
			}
		}
		return n
	}
	type reading struct {
		title string
		value float64
		note  string
	}
	read := func() reading {
		all := stocks("")
		h := hover.Get()
		if h < 0 || h >= len(rows.Get()) {
			return reading{"S&P 500 · " + upper(tr.ChartsTop(itoa(len(all)))) + " · " + periods[period.Get()], weighted(all),
				tr.ChartsAdvancing(itoa(advancing()), itoa(len(all)-advancing()), capOf(capSum(all)))}
		}
		r := rows.Get()[h]
		if r.parent == "" {
			of := stocks(r.id)
			return reading{upper(r.name) + " · " + upper(tr.ChartsStocks(itoa(len(of)))), weighted(of), tr.ChartsCapWeighted(capOf(capSum(of)))}
		}
		return reading{r.id + " · " + upper(r.name), r.chg, tr.ChartsSectorCap(r.parent, capOf(r.cap))}
	}
	toneColor := func(v float64) core.Color {
		switch {
		case v > 0:
			return colors.Up()
		case v < 0:
			return colors.Down()
		}
		return colors.Text()
	}
	height := core.Points(340)
	if chartPointer() {
		height = 300
	}
	keyStyle := core.Style{Margin: core.Edges{Top: 10}, Gap: 4}
	if chartPointer() {
		keyStyle.AlignSelf, keyStyle.Width = core.AlignStart, core.Pt(260)
	}
	return Card{TestID: "card-market", Title: tr.ChartsMarketMap(), Description: tr.ChartsMarketMapNote(), Children: []core.Node{
		readout(cx, func() string { return read().title }, func() string { return pct(read().value, 2) }, func() core.Color { return toneColor(read().value) },
			func() string { return read().note }),
		plot.Plot{
			TestID: "market-plot",
			Count:  func() int { return len(rows.Get()) },
			ColorScale: func() plot.ColorScale {
				r := mapRange[period.Get()]
				return plot.DivergingColors(colors.Down(), colors.Inset(), colors.Up()).Over(-r, r)
			},
			AppearGeneration: func() any { return 1 },
			Motion: func() plot.Motion {
				m := chartMotion(ms(900), ms(650), false)
				m.Exit.Duration, m.Pointer = ms(180), &core.SpringConfig{Stiffness: 380, Damping: 36}
				return m
			}(),
			LabelFunc: func() string {
				return tr.ChartsMarketMapLabel(itoa(len(stocks(""))), itoa(len(marketSectors)), periods[period.Get()], itoa(advancing()))
			},
			AccessibilityValue: func() core.AccessibilityValue {
				if h := hover.Get(); h >= 0 && h < len(rows.Get()) {
					r := read()
					return core.AccessibilityValue{Text: rows.Get()[h].name + ": " + pct(r.value, 2) + ", " + r.note}
				}
				return core.AccessibilityValue{}
			},
			OnHoverChange: hover.Set,
			Style:         core.Style{Height: core.Pt(height), Margin: core.Edges{Top: chartGap() + 4}},
			Children: []core.Node{plot.Treemap{
				ID:         func() []string { return dirs(rows.Get(), func(r mapRow) string { return r.id }) },
				Parent:     func() []string { return dirs(rows.Get(), func(r mapRow) string { return r.parent }) },
				Value:      func() []float64 { return column(rows.Get(), func(r mapRow) float64 { return r.cap }) },
				ColorValue: func() []float64 { return column(rows.Get(), func(r mapRow) float64 { return r.chg }) },
				PaddingInner: []core.Points{3, 1}, PaddingTop: []core.Points{18}, Corners: plot.AllCorners(2), Stagger: 0.4, HoverDim: 0.65,
				Label:          func() []string { return dirs(rows.Get(), func(r mapRow) string { return r.id }) },
				ValueLabel:     func(row int, _ float64) string { return pct(rows.Get()[row].chg, 2) },
				LabelPlacement: plot.TreemapLabelCenter,
				LabelStyle:     core.Style{Padding: core.All(4), FontSize: 11.5, FontWeight: ui.Semibold, LetterSpacing: 0.2},
				ValueLabelStyle: core.Style{FontSize: 10, FontWeight: ui.Regular, Opacity: core.Alpha(0.85), FontVariant: core.FontTabularNums},
				DarkLabelStyle:  core.Style{Color: core.RGB(0xffffff)},
				GroupLabel:      func() []string { return dirs(rows.Get(), func(r mapRow) string { return r.name }) },
				GroupLabelStyle: core.Style{FontSize: 10.5, FontFamily: ui.Mono, LineHeight: 18, LetterSpacing: 0.6, Padding: core.Symmetric(0, 4)},
				HoverStyleFunc: func() core.Style {
					return core.Style{CornerRadius: 2, OutlineWidth: 2, OutlineColor: colors.Text()}
				},
			}},
		},
		// The color's key: the period's range, from its full loss to its full
		// gain.
		core.View{Style: keyStyle, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(8), CornerRadius: 4}, StyleFunc: func() core.Style {
				return core.Style{Gradients: []core.Gradient{{Angle: 90, Stops: []core.ColorStop{{Color: colors.Down()}, {Color: colors.Inset()}, {Color: colors.Up()}}}}}
			}},
			core.Dynamic(func(cx *core.Scope) core.Node {
				r := mapRange[period.Get()]
				var texts []core.Node
				for _, v := range []float64{-r, -r / 2, 0, r / 2, r} {
					text := "0"
					if v != 0 {
						digits := 0
						if v != math.Trunc(v) {
							digits = 1
						}
						text = pct(v, digits)
					}
					texts = append(texts, core.Text{Content: text, StyleFunc: axisStyle(cx, core.Style{})})
				}
				return core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: texts}
			}),
		}},
		chartSwitch(tr.ChartsPeriod(), 220, period.Get, period.Set,
			ui.Option[int]{Value: 0, Label: tr.ChartsDayPeriod()}, ui.Option[int]{Value: 1, Label: tr.ChartsWeekPeriod()}, ui.Option[int]{Value: 2, Label: tr.ChartsMonthPeriod()}),
	}}
}

var assetClasses = []string{"Equities", "Bonds", "Alternatives", "Cash"}

// assetSeries is each class's series color.
var assetSeries = map[string]int{"Equities": 0, "Bonds": 1, "Alternatives": 3, "Cash": 2}

// position is a holding: its asset class and its value in thousands, today
// and a year ago (NaN: not held then).
type position struct {
	ticker, name, class string
	now, year           float64
}

var positions = func() []position {
	nan := math.NaN()
	return []position{
		{"VOO", "S&P 500 ETF", "Equities", 182, 140}, {"QQQ", "Nasdaq 100 ETF", "Equities", 96, 64}, {"VXUS", "Intl. stocks", "Equities", 58, 61},
		{"NVDA", "NVIDIA", "Equities", 41, nan}, {"ARKK", "Innovation ETF", "Equities", nan, 22},
		{"BND", "Total bond", "Bonds", 74, 88}, {"TLT", "20Y Treasuries", "Bonds", 31, nan}, {"TIP", "TIPS", "Bonds", 18, 24},
		{"GLD", "Gold", "Alternatives", 34, 21}, {"VNQ", "Real estate", "Alternatives", 16, 19}, {"BTC", "Bitcoin", "Alternatives", 12, 5},
		{"SGOV", "T-bills", "Cash", 44, 52}, {"USD", "Cash", "Cash", 14, 18},
	}
}()

// allocationRow is a row of the allocation: an asset class (no parent, its
// value NaN) or a position in one.
type allocationRow struct {
	id, parent, name, class string
	value                   float64
}

func allocationRows(lastYear bool) []allocationRow {
	var out []allocationRow
	for _, c := range assetClasses {
		out = append(out, allocationRow{c, "", c, c, math.NaN()})
	}
	for _, p := range positions {
		v := p.now
		if lastYear {
			v = p.year
		}
		if !math.IsNaN(v) {
			out = append(out, allocationRow{p.ticker, p.class, p.name, p.class, v})
		}
	}
	return out
}

// kUSD writes thousands of dollars.
func kUSD(k float64) string {
	if k >= 1000 {
		return "$" + fixed(k/1000, 2) + "M"
	}
	return "$" + fixed(k, 0) + "k"
}

// allocationCard is a portfolio's allocation: asset classes holding their
// positions, as a Treemap whose groups are drawn too, a wash of their
// class's color showing in their padding and in the band that names them,
// the positions in it at full color (an ordinal ColorScale of the asset
// classes), named at their top left with their weight. The position or
// class under the pointer gets a tooltip by it; a year ago the portfolio
// held other positions, and the switch moves the tiles to their old
// values, growing the ones bought since and shrinking the ones sold.
func allocationCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	lastYear := core.NewSignal(cx, false)
	tip := core.NewSignal(cx, 0)
	rows := core.NewMemoFunc(cx, func() []allocationRow { return allocationRows(lastYear.Get()) }, nil)
	total := func() float64 {
		s := 0.0
		for _, r := range rows.Get() {
			if !math.IsNaN(r.value) {
				s += r.value
			}
		}
		return s
	}
	valueOf := func(r allocationRow) float64 {
		if !math.IsNaN(r.value) {
			return r.value
		}
		s := 0.0
		for _, p := range rows.Get() {
			if p.parent == r.id {
				s += p.value
			}
		}
		return s
	}
	positionsOf := func(id string) int {
		n := 0
		for _, p := range rows.Get() {
			if p.parent == id {
				n++
			}
		}
		return n
	}
	weight := func(k float64) string { return fixed(k/total()*100, 1) + "%" }
	shown := func() allocationRow {
		if t := tip.Get(); t < len(rows.Get()) {
			return rows.Get()[t]
		}
		return rows.Get()[0]
	}
	classColor := func(class string) core.Color { return chartSeries(cx)[assetSeries[class]] }
	var figures []core.Node
	for i, c := range assetClasses {
		figures = append(figures, chartFigure(cx, c, func() string { return weight(valueOf(rows.Get()[i])) }, nil))
	}
	height := core.Points(300)
	if chartPointer() {
		height = 260
	}
	return Card{TestID: "card-allocation", Title: tr.ChartsAllocation(), Description: tr.ChartsAllocationNote(), Children: []core.Node{
		chartFigures(cx, 0, figures...),
		plot.Plot{
			TestID: "allocation-plot",
			Count:  func() int { return len(rows.Get()) },
			ColorScale: func() plot.ColorScale {
				shades := make([]core.Color, len(assetClasses))
				for i, c := range assetClasses {
					shades[i] = classColor(c)
				}
				return plot.OrdinalColors(assetClasses, shades...)
			},
			AppearGeneration: func() any { return 1 },
			Motion: func() plot.Motion {
				m := chartMotion(ms(900), ms(700), false)
				m.Pointer = &core.SpringConfig{Stiffness: 380, Damping: 36}
				return m
			}(),
			TooltipPlacement: plot.TooltipPointer,
			TooltipOffset:    12,
			Tooltip: tipCard(cx,
				tipTitleText(cx, func() string {
					s := shown()
					if s.parent == "" {
						return upper(s.name) + " · " + upper(tr.ChartsPositions(itoa(positionsOf(s.id))))
					}
					return s.id + " · " + upper(s.class)
				}),
				ui.Textf(func() string { return shown().name }, 13, ui.Medium, colors.Text),
				ui.Tabular(ui.Textf(func() string { return tr.ChartsOfThePortfolio(kUSD(valueOf(shown())), weight(valueOf(shown()))) }, 12, ui.Regular, colors.Muted))),
			LabelFunc: func() string { return tr.ChartsAllocationLabel(kUSD(total()), itoa(len(assetClasses))) },
			AccessibilityValue: func() core.AccessibilityValue {
				s := shown()
				return core.AccessibilityValue{Text: s.name + ": " + kUSD(valueOf(s)) + ", " + weight(valueOf(s))}
			},
			OnHoverChange: func(i int) {
				if i >= 0 {
					tip.Set(i)
				}
			},
			Style: core.Style{Height: core.Pt(height), Margin: core.Edges{Top: chartGap() + 4}},
			Children: []core.Node{plot.Treemap{
				ID:     func() []string { return dirs(rows.Get(), func(r allocationRow) string { return r.id }) },
				Parent: func() []string { return dirs(rows.Get(), func(r allocationRow) string { return r.parent }) },
				Value:  func() []float64 { return column(rows.Get(), func(r allocationRow) float64 { return r.value }) },
				Color:  func() []string { return dirs(rows.Get(), func(r allocationRow) string { return r.class }) },
				PaddingInner: []core.Points{6, 2}, PaddingOuter: []core.Points{3}, PaddingTop: []core.Points{20}, Corners: plot.AllCorners(4), HoverDim: 0.6,
				// The groups in their class's color, a wash under their positions.
				GroupClass: "opacity-15",
				Label:        func() []string { return dirs(rows.Get(), func(r allocationRow) string { return r.id }) },
				ValueLabel:   func(_ int, v float64) string { return weight(v) },
				LabelStyle: core.Style{Padding: core.Symmetric(6, 8), FontSize: 12, FontWeight: ui.Semibold},
				ValueLabelStyle: core.Style{FontSize: 11, FontWeight: ui.Regular, Opacity: core.Alpha(0.8), FontVariant: core.FontTabularNums},
				DarkLabelStyle:  core.Style{Color: core.RGB(0xffffff)},
				GroupLabel:      func() []string { return dirs(rows.Get(), func(r allocationRow) string { return r.name }) },
				GroupLabelStyle: core.Style{Padding: core.Symmetric(0, 8), FontSize: 11, FontWeight: ui.Medium, LineHeight: 20},
				HoverStyleFunc: func() core.Style {
					return core.Style{CornerRadius: 6, OutlineWidth: 2, OutlineColor: colors.Text()}
				},
			}},
		},
		chartSwitch(tr.ChartsView(), 220, lastYear.Get, lastYear.Set,
			ui.Option[bool]{Value: false, Label: tr.ChartsToday()}, ui.Option[bool]{Value: true, Label: tr.ChartsAYearAgo()}),
	}}
}

// watchSymbols are a watchlist's symbols.
var watchSymbols = [][2]string{
	{"AAPL", "Apple"}, {"MSFT", "Microsoft"}, {"NVDA", "NVIDIA"}, {"AMZN", "Amazon"}, {"GOOGL", "Alphabet"}, {"META", "Meta Platforms"},
	{"TSLA", "Tesla"}, {"AVGO", "Broadcom"}, {"ORCL", "Oracle"}, {"AMD", "Advanced Micro Devices"}, {"NFLX", "Netflix"}, {"ADBE", "Adobe"},
	{"CRM", "Salesforce"}, {"QCOM", "Qualcomm"}, {"INTC", "Intel"}, {"IBM", "IBM"}, {"UBER", "Uber"}, {"SHOP", "Shopify"},
}

// watchSpans are the watchlist's periods and the closes each shows.
var watchSpans = []struct {
	name   string
	points int
}{{"1D", 39}, {"1W", 35}, {"1M", 22}}

// watchWalk is a walk of n closes from 100, repeatable for its seed.
func watchWalk(seed float64, n int) []float64 {
	next := chartRandom(seed)
	v := 100.0
	out := make([]float64, n)
	for i := range out {
		v *= 1 + (next()-0.49)*0.024
		out[i] = v
	}
	return out
}

// watched is a row of the watchlist: its last price and its closes per
// period.
type watched struct {
	symbol, name string
	last         float64
	closes       [3][]float64
}

var watchlist = func() []watched {
	out := make([]watched, len(watchSymbols))
	for i, s := range watchSymbols {
		w := watched{symbol: s[0], name: s[1], last: 40 + chartRandom(float64(97*(i+1)))()*460}
		for j, p := range watchSpans {
			w.closes[j] = watchWalk(float64(7919*(i+1)+104729*j), p.points)
		}
		out[i] = w
	}
	return out
}()

// watchRow is a row's height; five show at once.
const watchRow = 52

// watchlistCard is a watchlist whose rows a list makes as they scroll in:
// each row's sparkline draws in from the left the first time it shows for
// a period, and stays whole when the row scrolls away and back, as the
// AppearScope around the list remembers it; a new period draws the rows in
// again. The line and its wash in the day's tone, the open a dotted
// hairline.
func watchlistCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	span := core.NewSignal(cx, 0)
	width := core.Points(88)
	if chartPointer() {
		width = 112
	}
	var options []ui.Option[int]
	for i, s := range watchSpans {
		options = append(options, ui.Option[int]{Value: i, Label: s.name})
	}
	return Card{TestID: "card-watchlist", Title: tr.ChartsWatchlist(), Description: tr.ChartsWatchlistNote(), Children: []core.Node{
		plot.AppearScope{ID: func() any { return "watchlist" }, Children: []core.Node{core.List{TestID: "watchlist", Count: func() int { return len(watchlist) },
			Key: func(i int) any { return watchlist[i].symbol }, EstimatedSize: watchRow, Style: core.Style{Height: core.Pt(watchRow * 5)},
			Row: func(cx *core.Scope, r core.ListRow) core.Node {
				colors := ui.ColorsOf(cx)
				w := func() watched { return watchlist[r.Index()] }
				closes := func() []float64 { return w().closes[span.Get()] }
				change := func() float64 { c := closes(); return (c[len(c)-1]/c[0] - 1) * 100 }
				tone := func() core.Color { return upDown(change()).Color(cx) }
				return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Height: core.Pt(watchRow), Borders: core.Edges{Bottom: 1}},
					StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Bottom: colors.Line()}} }, Children: []core.Node{
						core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
							core.Text{ContentFunc: func() string { return w().symbol }, Style: core.Style{FontSize: 13, FontWeight: ui.Semibold, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)},
							core.Text{ContentFunc: func() string { return w().name }, Style: core.Style{NumberOfLines: 1, FontSize: 11.5}, StyleFunc: ui.Colored(colors.Faint)},
						}},
						plot.Plot{
							Y:                closes,
							NoHover:          true,
							AppearGeneration: func() any { return w().symbol + " " + watchSpans[span.Get()].name },
							Motion:           plot.Motion{Appear: &core.TimingConfig{Duration: ms(700), Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}}},
							LabelFunc:        func() string { return w().symbol + ", " + watchSpans[span.Get()].name + ": " + pct(change(), 2) },
							Style:            core.Style{Width: core.Pt(width), Height: core.Pt(28)},
							Children: []core.Node{
								plot.Grid{Y: func() []float64 { return closes()[:1] }, StyleFunc: func() core.Style {
									return core.Style{Stroke: core.PaintColor(colors.Line()), StrokeDash: []core.Points{1, 3}}
								}},
								plot.Area{BackgroundStyleFunc: func() core.Style { return wash(tone(), 0.16) }, HoverDotStyle: hidden},
								plot.Line{StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(tone()), StrokeWidth: 1.5} }},
							},
						},
						core.View{Style: core.Style{Width: core.Pt(72), AlignItems: core.AlignEnd, Gap: 2}, Children: []core.Node{
							core.Text{ContentFunc: func() string { return fixed(w().last, 2) }, Style: core.Style{FontSize: 13, FontWeight: ui.Medium, FontVariant: core.FontTabularNums},
								StyleFunc: ui.Colored(colors.Text)},
							core.Text{ContentFunc: func() string { return pct(change(), 2) }, Style: core.Style{FontSize: 11.5, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(tone)},
						}},
					}}
			}}}},
		chartSwitch(tr.ChartsPeriod(), 180, span.Get, span.Set, options...),
	}}
}
