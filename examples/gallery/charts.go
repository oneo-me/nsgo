package main

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/ui"
)

// chartsDemo is what an app builds from base/plot's parts and its own
// views, with no chart component in between. A live price streamed a
// tick a second, its newest point breathing and tagged in the gutter; an
// order book's depth as steps either side of the spread; a portfolio's
// value over its underwater plot, the worst run shaded and the hovered
// day marked on both; capacity rings that sweep in for each period;
// revenue as a donut whose hovered slice lifts; sales as bars side by
// side or stacked; bars that span a range of their row (a base per row):
// a quarter's profit bridge, an onboarding funnel centered on its middle
// and daily candles over their volume; dots: holdings as bubbles of their
// market cap by risk and return, and analysts' price targets as a dot
// plot; volume and price, and revenue and margin, on two y scales;
// heatmaps: a correlation matrix, a calendar of monthly returns and a
// week of a market's trades per half hour; factor exposure as a radar; a
// quarter's income as flows; a market map and an allocation as treemaps;
// and a watchlist of sparklines that draw in once. Values on the axes are
// small monospaced figures the plots' Axis parts lay along their edges,
// grids hairlines, tooltips floating cards with a caps title. The cards
// are in the charts_*.go files.
func chartsDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, liveCard, depthCard, drawdownCard, capacityCard, revenueCard, salesCard, bridgeCard,
		funnelCard, candlesCard, riskCard, targetsCard, volumeCard, marginsCard, correlationCard, seasonalityCard,
		activityCard, factorsCard, flowsCard, marketCard, allocationCard, watchlistCard)
}

// chartRandom is a repeatable stream of numbers in [0, 1).
func chartRandom(seed float64) func() float64 {
	s := seed
	return func() float64 {
		s = math.Mod(s*16807, 2147483647)
		return math.Mod(s, 100000) / 100000
	}
}

// fixed writes n with digits decimals.
func fixed(n float64, digits int) string { return strconv.FormatFloat(n, 'f', digits, 64) }

// count writes n rounded, with thousands separators.
func count(n float64) string { return thousands(int(math.Round(n))) }

// sign writes n with an explicit plus or a true minus.
func sign(n float64, digits int) string {
	if n >= 0 {
		return "+" + fixed(n, digits)
	}
	return "−" + fixed(-n, digits)
}

// pct writes n as a signed percentage.
func pct(n float64, digits int) string { return sign(n, digits) + "%" }

// minus writes n with a true minus.
func minus(n float64, digits int) string {
	if n < 0 {
		return "−" + fixed(-n, digits)
	}
	return fixed(n, digits)
}

// gridTicks are values at a nice step (1, 2, 2.5 or 5 × 10^k) strictly
// inside lo..hi, about want of them.
func gridTicks(lo, hi float64, want int) []float64 {
	raw := (hi - lo) / float64(want)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	step := raw
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if m*mag >= raw {
			step = m * mag
			break
		}
	}
	var out []float64
	for v := math.Ceil(lo/step) * step; v < hi; v += step {
		if v > lo {
			out = append(out, math.Round(v/step)*step)
		}
	}
	return out
}

// bandAt is where band i of n starts and how wide it is, and the step
// between two bands' starts, as shares of the plot's width: the band
// scale's own arithmetic, so a view of the app's lines up with a band.
func bandAt(i, n int, inner, outer float64) (start, width, step float64) {
	step = 1 / (float64(n) - inner + 2*outer)
	return step * (outer + float64(i)), step * (1 - inner), step
}

// chartBelow is the room under a plot for its x labels: 8 points, then a
// line of axis text.
const chartBelow = 8 + 13

// chartAxisGap is the width of the y axis' labels beside a plot.
const chartAxisGap = 28

// chartPointer is whether the charts lay out for a pointer: smaller
// switches, and figures beside the rings rather than under them.
func chartPointer() bool { return shell.desktop() }

// chartGap is the room between a card's parts.
func chartGap() core.Points {
	if chartPointer() {
		return 10
	}
	return 12
}

// chartSeries are the series' colors, in order: the accent first, then
// graphite from dark to light, so one series leads and the rest stay as
// quiet as the interface. Rises and falls take the up and down tones.
func chartSeries(cx *core.Scope) []core.Color {
	c := ui.ColorsOf(cx).Now()
	gray := func(a float32) core.Color { return ui.Alpha(c.Text, a).Over(c.Surface) }
	return []core.Color{c.Accent, gray(0.62), gray(0.42), gray(0.28), gray(0.18)}
}

// chartSeriesColor is the ith series' color.
func chartSeriesColor(cx *core.Scope, i int) func() core.Color {
	return func() core.Color { return chartSeries(cx)[i%5] }
}

// chartViolet is a category's color the theme has no tone for.
var chartViolet = core.RGB(0x8e51ff)

// chartCategories are colors for categories that must read apart at a
// glance (sectors, asset classes), in order.
func chartCategories(cx *core.Scope) []core.Color {
	colors := ui.ColorsOf(cx)
	return []core.Color{colors.Accent(), colors.Info(), colors.Warn(), chartViolet, chartSeries(cx)[2]}
}

// axisText is the values on the axes: small, monospaced, tabular and
// faint, so the marks lead.
func axisText(cx *core.Scope) core.Style {
	return core.Style{FontSize: 10.5, FontFamily: ui.Mono, Color: ui.ColorsOf(cx).Faint(), FontVariant: core.FontTabularNums}
}

// axisStyle is axisText over s, read where it is drawn.
func axisStyle(cx *core.Scope, s core.Style) func() core.Style {
	return func() core.Style { return axisText(cx).Merge(s) }
}

// axisColored is axisStyle in color, read where it is drawn.
func axisColored(cx *core.Scope, s core.Style, color func() core.Color) func() core.Style {
	return func() core.Style { return axisText(cx).Merge(s).Merge(core.Style{Color: color()}) }
}

// belowStyle is an axis' labels under a plot: a line of axis text 8
// points below its edge.
func belowStyle(cx *core.Scope) func() core.Style {
	return axisStyle(cx, core.Style{LineHeight: 13, Padding: core.Edges{Top: 8}})
}

// gutterStyle is the labels of a y axis Width wide beside a plot, toward
// it at Align's side.
func gutterStyle(cx *core.Scope, width core.Points, right bool) func() core.Style {
	if right {
		return axisStyle(cx, core.Style{Width: core.Pt(width), Padding: core.Edges{Left: 8}, TextAlign: core.TextAlignEnd})
	}
	return axisStyle(cx, core.Style{Width: core.Pt(width), Padding: core.Edges{Right: 8}, TextAlign: core.TextAlignEnd})
}

// tipTitle is a tooltip's title over its values, and a readout's: a
// small, monospaced, muted caps label.
func tipTitle(cx *core.Scope) core.Style {
	return core.Style{FontSize: 10, FontFamily: ui.Mono, LetterSpacing: 0.8, Color: ui.ColorsOf(cx).Muted()}
}

// tipTitleText is a tipTitle of text.
func tipTitleText(cx *core.Scope, text func() string) core.Node {
	return core.Text{ContentFunc: text, StyleFunc: func() core.Style { return tipTitle(cx) }}
}

// tipCard is a floating tooltip's card: the menu's surface, 10-point
// corners.
func tipCard(cx *core.Scope, children ...core.Node) core.Node {
	fin := ui.FinOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return fin.Menu().Over(core.Style{BorderWidth: 1, CornerRadius: 10, Padding: core.Symmetric(8, 10), Gap: 6})
	}, Children: children}
}

// tipValue is a tooltip's value under its title.
func tipValue(cx *core.Scope, text func() string) core.Node {
	return ui.Tabular(ui.Textf(text, 13, ui.Medium, ui.ColorsOf(cx).Text))
}

// wash is a wash under a line: c at top opacity fading out to the
// baseline.
func wash(c core.Color, top float32) core.Style {
	return core.Style{Gradients: []core.Gradient{{Angle: 180, Stops: []core.ColorStop{{Color: ui.Alpha(c, top)}, {Color: ui.Alpha(c, 0)}}}}}
}

// gridStyle is a grid's hairlines.
func gridStyle(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Line())} }
}

// baselineStyle is a baseline, the grid's zero or its foot: firmer than
// the grid.
func baselineStyle(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Border())} }
}

// referenceStyle is a reference (an open, a target): a dashed line of
// the text's color at 28%, quieter than the marks.
func referenceStyle(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Stroke: core.PaintColor(ui.Alpha(colors.Text(), 0.28)), StrokeDash: []core.Points{2, 4}}
	}
}

// crossLine is the line under the pointer over a line or an area: a
// hairline of the text's color at 22%.
func crossLine(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style { return core.Style{Width: core.Pt(1), Background: ui.Alpha(colors.Text(), 0.22)} }
}

// crossBand is the band under the pointer over bars or cells: a faint
// band of the text's color the width of a band.
func crossBand(cx *core.Scope, a float32, radius core.Points) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Width: core.Pct(100), Height: core.Pct(100), Background: ui.Alpha(colors.Text(), a), CornerRadius: radius}
	}
}

// hoverDotShape is the hover's dot: 10 points, ringed.
var hoverDotShape = core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, BorderWidth: 2}

// hoverDot is the hover's dot on a line: its series' color, ringed in the
// inset's, so it cuts out of the line.
func hoverDot(cx *core.Scope, color func() core.Color) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style { return core.Style{Background: color(), BorderColor: colors.Inset()} }
}

// hidden hides a mark's hover dot.
var hidden = core.Style{Display: core.DisplayNone}

// ringStyle is a hovered bubble's or cell's ring: the text's color a
// little out from its edge.
func ringStyle(cx *core.Scope, radius core.Points) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{CornerRadius: radius, OutlineWidth: 1.5, OutlineOffset: 1.5, OutlineColor: colors.Text()}
	}
}

// chartMotion is a plot's motion: an appear of appear, an update of
// update (none at 0), the hover's fade and, unless still, its glide.
func chartMotion(appear, update time.Duration, still bool) plot.Motion {
	m := plot.Motion{
		Appear: &core.TimingConfig{Duration: appear, Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}},
		Enter:  &core.TimingConfig{Duration: 120 * time.Millisecond},
		Exit:   &core.TimingConfig{Duration: 160 * time.Millisecond},
	}
	if update > 0 {
		m.Update = &core.TimingConfig{Duration: update, Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeInOut}}
	}
	if !still {
		m.Pointer = &core.SpringConfig{Stiffness: 320, Damping: 32}
	}
	return m
}


// headline is a card's large figure, in color or the text's.
func headline(cx *core.Scope, text func() string, color func() core.Color) core.Node {
	colors := ui.ColorsOf(cx)
	if color == nil {
		color = colors.Text
	}
	return core.Text{ContentFunc: text, Style: core.Style{FontSize: 22, FontWeight: ui.Semibold, LetterSpacing: -0.4, FontVariant: core.FontTabularNums},
		StyleFunc: func() core.Style { return core.Style{Color: color()} }}
}

// note is a quieter line beside or under a headline, in color.
func note(cx *core.Scope, text func() string, color func() core.Color) core.Node {
	t := core.Text{ContentFunc: text, Style: core.Style{FontSize: 12, FlexShrink: 1, FontVariant: core.FontTabularNums},
		StyleFunc: func() core.Style { return core.Style{Color: color()} }}
	return t
}

// baseline lays its children on a row along their text's baseline.
func baseline(gap core.Points, children ...core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: gap}, Children: children}
}

// readout is a title over a headline and a note, a card's head.
func readout(cx *core.Scope, title func() string, value func() string, valueColor func() core.Color, aside func() string, extra ...core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	children := []core.Node{tipTitleText(cx, title), baseline(10, headline(cx, value, valueColor), note(cx, aside, colors.Muted))}
	return core.View{Style: core.Style{Gap: 4}, Children: append(children, extra...)}
}

// chartFigure is a label over its value, in a row of figures under a
// chart.
func chartFigure(cx *core.Scope, label string, value func() string, tone func() Tone) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 4}, Children: []core.Node{
		core.Text{Content: upper(label), Style: core.Style{FontSize: 10, FontFamily: ui.Mono, LetterSpacing: 0.8, NumberOfLines: 1}, StyleFunc: ui.Colored(colors.Faint)},
		core.Text{ContentFunc: value, Style: core.Style{FontSize: 14, FontWeight: ui.Medium, FontVariant: core.FontTabularNums, NumberOfLines: 1}, StyleFunc: func() core.Style {
			if tone == nil {
				return core.Style{Color: colors.Text()}
			}
			return core.Style{Color: tone().Foreground(cx)()}
		}},
	}}
}

// fixedTone is a tone a figure takes always.
func fixedTone(t Tone) func() Tone { return func() Tone { return t } }

// chartFigures lays figures in a row over a hairline, Top under what is
// above.
func chartFigures(cx *core.Scope, top core.Points, figures ...core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{Direction: core.Row, Gap: 12, Padding: core.Edges{Top: 12}, Borders: core.Edges{Top: 1}, Margin: core.Edges{Top: top}},
		StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }, Children: figures}
}

// chartSwitch is a card's segments: small tabs width wide at the start
// with a pointer, else regular ones across the card.
func chartSwitch[T comparable](label string, width core.Points, value func() T, set func(T), options ...ui.Option[T]) core.Node {
	tabs := ui.Tabs[T]{Label: label, Options: options, Value: value, OnChange: set}
	s := core.Style{Margin: core.Edges{Top: chartGap() + 4}}
	if chartPointer() {
		tabs.Size = ui.SizeSmall
		s.AlignSelf, s.Width = core.AlignStart, core.Pt(width)
	} else {
		tabs.Equal = true
	}
	tabs.Style = core.Style{Width: core.Pct(100)}
	return core.View{Style: s, Children: []core.Node{tabs}}
}

// chartLegend is a colored square and a label with an optional value.
func chartLegend(cx *core.Scope, color func() core.Color, label func() string, value func() string) core.Node {
	colors := ui.ColorsOf(cx)
	spans := []core.Span{{ContentFunc: label}}
	if value != nil {
		spans = append(spans, core.Span{ContentFunc: func() string { return " " + value() }, Style: core.Style{FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Text)})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(9), Height: core.Pt(9), CornerRadius: 2.5}, StyleFunc: ui.Filled(color)},
		core.Text{Spans: spans, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
	}}
}

// lineKey is a line's swatch in a legend, a short stroke with a dot on
// it, and its label with an optional value.
func lineKey(cx *core.Scope, color func() core.Color, label string, value func() string) core.Node {
	colors := ui.ColorsOf(cx)
	spans := []core.Span{{Content: label}}
	if value != nil {
		spans = append(spans, core.Span{ContentFunc: func() string { return " " + value() }, Style: core.Style{FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Text)})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(14), Height: core.Pt(2), CornerRadius: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, StyleFunc: ui.Filled(color),
			Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(6), Height: core.Pt(6), CornerRadius: 3}, StyleFunc: ui.Filled(color)}}},
		core.Text{Spans: spans, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
	}}
}

// dotKey is a round key of a legend, a dot of color or a ring of it when
// hollow, and its label.
func dotKey(cx *core.Scope, color func() core.Color, label string, hollow bool, size core.Points) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2}, StyleFunc: func() core.Style {
			if hollow {
				return core.Style{BorderWidth: 1.5, BorderColor: color()}
			}
			return core.Style{Background: color()}
		}},
		ui.Text(label, 12, ui.Regular, colors.Muted),
	}}
}

// keys lays legend keys in a row that wraps, top under what is above.
func keys(top core.Points, children ...core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, ColumnGap: 14, RowGap: 6, Margin: core.Edges{Top: top}},
		Children: children}
}

// ramp is a color ramp's key: its colors left to right over the labels
// of its ends (and middle).
func ramp(cx *core.Scope, colors func() []core.Color, labels func() []string, s core.Style) core.Node {
	return core.View{Style: core.Style{Gap: 4}.Merge(s), Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3}, StyleFunc: func() core.Style {
			var stops []core.ColorStop
			for _, c := range colors() {
				stops = append(stops, core.ColorStop{Color: c})
			}
			return core.Style{Gradients: []core.Gradient{{Angle: 90, Stops: stops}}}
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			var texts []core.Node
			for _, l := range labels() {
				texts = append(texts, core.Text{Content: l, StyleFunc: axisStyle(cx, core.Style{FontSize: 10})})
			}
			return core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: texts}
		}),
	}}
}

// upper is s in capitals, for caps labels.
func upper(s string) string { return strings.ToUpper(s) }

// spaced puts top's room above a node.
func spaced(top core.Points, children ...core.Node) core.Node {
	return core.View{Style: core.Style{Margin: core.Edges{Top: top}}, Children: children}
}
