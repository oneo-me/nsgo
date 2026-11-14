package main

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// tradingDays are the n weekdays up to Friday, October 30, 2026, oldest
// first.
func tradingDays(n int) []time.Time {
	out := make([]time.Time, n)
	day := time.Date(2026, time.October, 30, 0, 0, 0, 0, time.UTC)
	for i := n - 1; i >= 0; day = day.AddDate(0, 0, -1) {
		if wd := day.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out[i] = day
			i--
		}
	}
	return out
}

// monthDay writes a day as "Oct 30".
func monthDay(d time.Time) string { return d.Format("Jan 2") }

// cents rounds v to cents.
func cents(v float64) float64 { return math.Round(v*100) / 100 }

// dayCandle is a day of a stock: its prices, its volume in millions, and its
// volume again under its direction, NaN under the other.
type dayCandle struct {
	date                          string
	open, high, low, close, volume float64
	dir                           string
	upVolume, downVolume          float64
}

// candles are a stock's days over three months: a climb, a pullback, a
// recovery; volume heavier on the days that move more.
var candles = func() []dayCandle {
	r := chartRandom(61)
	closing := 168.0
	days := tradingDays(61)
	out := make([]dayCandle, len(days))
	for i, d := range days {
		open := closing * (1 + (r()-0.5)*0.012)
		drift := 0.004
		if i < 24 {
			drift = 0.0035
		} else if i < 40 {
			drift = -0.0045
		}
		closing = open * (1 + drift + (r()-0.5)*0.026)
		high := max(open, closing) * (1 + r()*0.011)
		low := min(open, closing) * (1 - r()*0.011)
		volume := math.Round((6+r()*6)*(1+math.Abs(closing-open)/open*30)*100) / 100
		c := dayCandle{date: monthDay(d), open: cents(open), high: cents(high), low: cents(low), close: cents(closing), volume: volume,
			upVolume: math.NaN(), downVolume: math.NaN()}
		if closing >= open {
			c.dir, c.upVolume = "up", volume
		} else {
			c.dir, c.downVolume = "down", volume
		}
		out[i] = c
	}
	return out
}()

// candleSpans are the periods the candles show, in days.
var candleSpans = map[string]int{"1M": 21, "2M": 41, "3M": 61}

const (
	candleInner   = 0.3
	candleOuter   = 0.15
	priceHeight   = 168
	volumeHeight  = 44
	chartGutter   = 58
	candleTagSide = 14
)

// dirs are rows' directions.
func dirs[T any](rows []T, f func(T) string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = f(r)
	}
	return out
}

// candlesCard is daily candles over their volume: each day a body from
// its open to its close (a Bar with a base per row) over a wick from its
// low to its high (a Rule), green up and red down through the plot's
// ColorScale, the last close tagged in the gutter at its height. Under it
// the day's volume in the same bands, in the day's tone. The day under the
// pointer is marked on both plots and its open, high, low and close read
// out over them; a period draws its candles in again.
func candlesCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	span := core.NewSignal(cx, "2M")
	hover := core.NewSignal(cx, -1)
	// Which plot the pointer is over: the other one marks the day itself.
	over := core.NewSignal(cx, "")
	rows := core.NewMemoFunc(cx, func() []dayCandle { return candles[len(candles)-candleSpans[span.Get()]:] }, nil)
	last := func() dayCandle { r := rows.Get(); return r[len(r)-1] }
	shown := func() dayCandle {
		r := rows.Get()
		if h := hover.Get(); h >= 0 && h < len(r) {
			return r[h]
		}
		return r[len(r)-1]
	}
	// The close before a day's, what its change is measured from.
	before := func(c dayCandle) float64 {
		if i := slices.IndexFunc(candles, func(d dayCandle) bool { return d.date == c.date }); i > 0 {
			return candles[i-1].close
		}
		return c.open
	}
	change := func() float64 { return shown().close - before(shown()) }
	tone := func() core.Color {
		if change() >= 0 {
			return colors.Up()
		}
		return colors.Down()
	}
	lastTone := func() core.Color {
		if last().dir == "up" {
			return colors.Up()
		}
		return colors.Down()
	}
	domain := func() (float64, float64) {
		lo := slices.Min(column(rows.Get(), func(c dayCandle) float64 { return c.low }))
		hi := slices.Max(column(rows.Get(), func(c dayCandle) float64 { return c.high }))
		pad := (hi - lo) * 0.08
		return math.Floor(lo - pad), math.Ceil(hi + pad)
	}
	grid := func() []float64 { lo, hi := domain(); return gridTicks(lo, hi, 4) }
	y := func(v float64) float64 { lo, hi := domain(); return (1 - (v-lo)/(hi-lo)) * priceHeight }
	topVolume := func() float64 { return slices.Max(column(rows.Get(), func(c dayCandle) float64 { return c.volume })) * 1.12 }
	total := func() float64 { return (last().close/rows.Get()[0].open - 1) * 100 }
	ups := func() int {
		n := 0
		for _, c := range rows.Get() {
			if c.dir == "up" {
				n++
			}
		}
		return n
	}
	dates := func() []float64 {
		n := float64(len(rows.Get()) - 1)
		return []float64{0, math.Round(n / 3), math.Round(2 * n / 3), n}
	}
	follow := func(which string) func(int) {
		return func(i int) {
			cx.Batch(func() {
				hover.Set(i)
				if i < 0 {
					which = ""
				}
				over.Set(which)
			})
		}
	}
	// The hovered day's band on the plot the pointer is not over.
	mark := func(which string) core.Node {
		return core.Show{When: func() bool { return hover.Get() >= 0 && over.Get() != which }, Then: func(*core.Scope) core.Node {
			return core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0)}, StyleFunc: func() core.Style {
				n := len(rows.Get())
				start, _, _ := bandAt(max(hover.Get(), 0), n, candleInner, candleOuter)
				_, width, _ := bandAt(0, n, candleInner, candleOuter)
				return core.Style{Left: core.Pct(float32(start * 100)), Width: core.Pct(float32(width * 100)), Background: ui.Alpha(colors.Text(), 0.06)}
			}}
		}}
	}
	ohlc := func(label string, v func() float64) core.Node {
		return core.Text{Spans: []core.Span{{Content: label + " "}, {ContentFunc: func() string { return fixed(v(), 2) }, StyleFunc: ui.Colored(colors.Text)}},
			StyleFunc: axisStyle(cx, core.Style{})}
	}
	motion := chartMotion(ms(800), 0, true)
	values := func(f func(dayCandle) float64) func() []float64 { return func() []float64 { return column(rows.Get(), f) } }
	direction := func() []string { return dirs(rows.Get(), func(c dayCandle) string { return c.dir }) }
	band := func() plot.Scale { return plot.BandScale(plot.Band{PaddingInner: candleInner, PaddingOuter: candleOuter}) }
	return Card{TestID: "card-candles", Title: tr.ChartsCandles(), Description: tr.ChartsCandlesNote(), Children: []core.Node{
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.Text{Content: "NSHL", Style: core.Style{FontSize: 13, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
				tipTitleText(cx, func() string { return upper(shown().date) }),
			}},
			baseline(10, headline(cx, func() string { return fixed(shown().close, 2) }, nil),
				core.Text{ContentFunc: func() string { return sign(change(), 2) + "  " + pct(change()/before(shown())*100, 2) },
					Style: core.Style{FontSize: 12, FontWeight: ui.Medium, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(tone)}),
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, ColumnGap: 12, RowGap: 2}, Children: []core.Node{
				ohlc("O", func() float64 { return shown().open }), ohlc("H", func() float64 { return shown().high }),
				ohlc("L", func() float64 { return shown().low }), ohlc("C", func() float64 { return shown().close }),
				core.Text{Spans: []core.Span{{Content: "VOL "}, {ContentFunc: func() string { return fixed(shown().volume, 1) + "M" }, StyleFunc: ui.Colored(colors.Text)}},
					StyleFunc: axisStyle(cx, core.Style{})},
			}},
		}},
		core.View{Style: core.Style{Margin: core.Edges{Top: chartGap()}, Gap: 6}, Children: []core.Node{
			plot.Plot{
				TestID: "candles-plot",
				Count:  func() int { return len(rows.Get()) },
				XScale: band,
				YScale: func() plot.Scale { lo, hi := domain(); return plot.LinearScale(lo, hi) },
				ColorScale: func() plot.ColorScale {
					return plot.OrdinalColors([]string{"up", "down"}, colors.Up(), colors.Down())
				},
				AppearGeneration: func() any { return span.Get() },
				// No pointer spring: the other plot marks the same day at once,
				// so both move together.
				Motion: motion,
				LabelFunc: func() string {
					return tr.ChartsCandlesLabel(span.Get(), fixed(rows.Get()[0].open, 2), fixed(last().close, 2), pct(total(), 2))
				},
				AccessibilityValue: func() core.AccessibilityValue {
					if hover.Get() < 0 {
						return core.AccessibilityValue{}
					}
					c := shown()
					return core.AccessibilityValue{Text: tr.ChartsCandleValue(c.date, fixed(c.open, 2), fixed(c.high, 2), fixed(c.low, 2), fixed(c.close, 2))}
				},
				CrossLineStyleFunc: crossBand(cx, 0.06, 0),
				OnHoverChange:      follow("price"),
				Style:              core.Style{Height: core.Pt(priceHeight), Margin: core.Edges{Right: chartGutter}},
				Children: []core.Node{
					plot.Grid{Y: grid, StyleFunc: gridStyle(cx)},
					// The last close: a dotted level across the days.
					plot.Grid{Y: func() []float64 { return []float64{last().close} }, StyleFunc: func() core.Style {
						return core.Style{Stroke: core.PaintColor(ui.Alpha(lastTone(), 0.55)), StrokeDash: []core.Points{2, 3}}
					}},
					// Wicks under bodies, both in the day's tone.
					plot.Rule{Value: values(func(c dayCandle) float64 { return c.high }), Bases: values(func(c dayCandle) float64 { return c.low }), Color: direction,
						Stagger: 0.6, HoverDim: 0.7, Style: core.Style{StrokeWidth: 1}},
					plot.Bar{Value: values(func(c dayCandle) float64 { return c.close }), Bases: values(func(c dayCandle) float64 { return c.open }), Color: direction,
						Corners: plot.AllCorners(1), Stagger: 0.6, HoverDim: 0.7},
					mark("price"),
					// The gutter: the grid's values clear of the last close's tag.
					plot.Axis{Placement: plot.AxisRight, TickValues: func() []float64 {
						var out []float64
						for _, v := range grid() {
							if math.Abs(y(v)-y(last().close)) > candleTagSide {
								out = append(out, v)
							}
						}
						return out
					}, Format: func(v float64, _ int) string { return fixed(v, 0) }, LabelStyleFunc: axisStyle(cx, core.Style{Padding: core.Edges{Left: 10}})},
					plot.Axis{Placement: plot.AxisRight, TickValues: func() []float64 { return []float64{last().close} }, Format: func(v float64, _ int) string { return fixed(v, 2) },
						LabelStyleFunc: func() core.Style {
							return core.Style{Margin: core.Edges{Left: 6}, Padding: core.Symmetric(0, 5), LineHeight: 18, CornerRadius: 5, Background: lastTone(),
								FontSize: 10.5, FontFamily: ui.Mono, FontWeight: ui.Semibold, Color: colors.Surface(), FontVariant: core.FontTabularNums}
						}},
				},
			},
			plot.Plot{
				TestID:           "volume-plot",
				Count:            func() int { return len(rows.Get()) },
				XScale:           band,
				YScale:           func() plot.Scale { return plot.LinearScale(0, topVolume()) },
				AppearGeneration: func() any { return span.Get() },
				Motion:           motion,
				LabelFunc: func() string {
					sum := 0.0
					for _, c := range rows.Get() {
						sum += c.volume
					}
					return tr.ChartsVolumeLabel(span.Get(), fixed(sum/float64(len(rows.Get())), 1))
				},
				AccessibilityValue: func() core.AccessibilityValue {
					if hover.Get() < 0 {
						return core.AccessibilityValue{}
					}
					return core.AccessibilityValue{Text: shown().date + ": " + tr.ChartsMillion(fixed(shown().volume, 1))}
				},
				CrossLineStyleFunc: crossBand(cx, 0.06, 0),
				OnHoverChange:      follow("volume"),
				Style:              core.Style{Height: core.Pt(volumeHeight), Margin: core.Edges{Right: chartGutter, Bottom: chartBelow}},
				Children: []core.Node{
					// A day's volume in one of two series, so each takes its tone,
					// faded under the candles'.
					plot.Bar{Value: values(func(c dayCandle) float64 { return c.upVolume }), Stack: "volume", Corners: plot.Corners{TopLeft: 1, TopRight: 1}, Stagger: 0.6, HoverDim: 0.7,
						StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(ui.Alpha(colors.Up(), 0.45))} }},
					plot.Bar{Value: values(func(c dayCandle) float64 { return c.downVolume }), Stack: "volume", Corners: plot.Corners{TopLeft: 1, TopRight: 1}, Stagger: 0.6, HoverDim: 0.7,
						StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(ui.Alpha(colors.Down(), 0.45))} }},
					plot.Grid{Y: func() []float64 { return []float64{0} }, StyleFunc: baselineStyle(cx)},
					mark("volume"),
					plot.Axis{Placement: plot.AxisRight, TickValues: func() []float64 { return []float64{topVolume() / 1.12} }, Format: func(v float64, _ int) string { return fixed(v, 0) + "M" },
						LabelStyleFunc: axisStyle(cx, core.Style{Padding: core.Edges{Left: 10}})},
					plot.Axis{TickValues: dates, Format: func(v float64, _ int) string {
						if r := rows.Get(); int(v) < len(r) {
							return r[int(v)].date
						}
						return ""
					}, LabelStyleFunc: belowStyle(cx)},
				},
			},
		}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsHigh(), func() string { return fixed(slices.Max(column(rows.Get(), func(c dayCandle) float64 { return c.high })), 2) }, nil),
			chartFigure(cx, tr.ChartsLow(), func() string { return fixed(slices.Min(column(rows.Get(), func(c dayCandle) float64 { return c.low })), 2) }, nil),
			chartFigure(cx, tr.ChartsChange(), func() string { return pct(total(), 1) }, func() Tone { return upDown(total()) }),
			chartFigure(cx, tr.ChartsUpDays(), func() string { return itoa(ups()) + " / " + itoa(len(rows.Get())) }, nil)),
		chartSwitch(tr.ChartsPeriod(), 180, span.Get, func(s string) {
			cx.Batch(func() {
				span.Set(s)
				hover.Set(-1)
			})
		}, ui.Option[string]{Value: "1M", Label: "1M"}, ui.Option[string]{Value: "2M", Label: "2M"}, ui.Option[string]{Value: "3M", Label: "3M"}),
	}}
}

// tapeDay is a day of a stock's trading: its close and its volume in
// millions of shares.
type tapeDay struct {
	date          time.Time
	close, volume float64
}

// tape is two years of a stock's trading days up to Friday, October 30,
// 2026.
var tape = func() []tapeDay {
	days := tradingDays(504)
	r := chartRandom(97)
	closing := 118.0
	out := make([]tapeDay, len(days))
	for i, d := range days {
		before := closing
		drift := 0.0021
		if i < 180 {
			drift = 0.0011
		} else if i < 300 {
			drift = -0.0016
		}
		closing = cents(closing * (1 + drift + (r()-0.5)*0.03))
		// Heavier on the days that move more, and through the sell-off.
		move := math.Abs(closing/before - 1)
		heavy := 1.0
		if i >= 180 && i < 300 {
			heavy = 1.25
		}
		out[i] = tapeDay{d, closing, math.Round((32+r()*18)*(1+move*22)*heavy*10) / 10}
	}
	return out
}()

// tapeSession is a row of the tape: size days, the last day's close, the
// days' volume summed, up or down from the row before.
type tapeSession struct {
	label, title  string
	close, volume float64
	dir           string
}

// sessions are n rows of size days each, the latest last, named by name.
func sessions(n, size int, name func(time.Time) (string, string)) []tapeSession {
	var out []tapeSession
	for k := n; k >= 1; k-- {
		end := len(tape) - (k-1)*size
		days := tape[end-size : end]
		lastDay := days[len(days)-1]
		prior := days[0].close
		if end-size-1 >= 0 {
			prior = tape[end-size-1].close
		}
		label, title := name(lastDay.date)
		volume := 0.0
		for _, d := range days {
			volume += d.volume
		}
		dir := "up"
		if lastDay.close < prior {
			dir = "down"
		}
		out = append(out, tapeSession{label, title, lastDay.close, math.Round(volume*10) / 10, dir})
	}
	return out
}

// tapeSpans are the tape's periods: their unit and rows.
var tapeSpans = map[string][]tapeSession{
	"1M": sessions(22, 1, func(d time.Time) (string, string) { return monthDay(d), d.Format("Jan 2, 2006") }),
	"6M": sessions(26, 5, func(d time.Time) (string, string) { return monthDay(d), "→ " + monthDay(d) }),
	"2Y": sessions(24, 21, func(d time.Time) (string, string) { return d.Format("Jan ’06"), d.Format("Jan 2006") }),
}

const (
	tapeHeight = 176
	// chartSide is the width of each side's axis labels.
	chartSide = 40
)

// shares writes millions of shares as a chart writes them: 40M, 1.2B.
func shares(m float64) string {
	switch {
	case m == 0:
		return "0"
	case m >= 1000:
		digits := 1
		if math.Mod(m, 1000) == 0 {
			digits = 0
		}
		return fixed(m/1000, digits) + "B"
	}
	return fixed(m, 0) + "M"
}

// axisTitle is a scale's title over its axis.
func axisTitle(cx *core.Scope, text string) core.Node {
	return core.Text{Content: upper(text), Style: core.Style{FontSize: 10, FontFamily: ui.Mono, LetterSpacing: 0.8}, StyleFunc: ui.Colored(ui.ColorsOf(cx).Faint)}
}

// axisTitles are each scale's title over its own axis.
func axisTitles(cx *core.Scope, left, right string) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween, Margin: core.Edges{Top: chartGap() + 2, Bottom: 6}},
		Children: []core.Node{axisTitle(cx, left), axisTitle(cx, right)}}
}

// volumeCard is volume and price on two y scales: a bar of the shares
// traded per day, week or month on the left scale, in the tone of the
// close's move, and the close as a line on the right one (the line's
// ScaleY2). Neither scale names a domain: the plot widens both to the same
// nice steps, so one grid (a Grid without values, which follows them)
// serves both axes. The row under the pointer reads out both values; a
// period moves both scales to their new domains as its bars and line draw
// in.
func volumeCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	span := core.NewSignal(cx, "6M")
	hover := core.NewSignal(cx, -1)
	rows := func() []tapeSession { return tapeSpans[span.Get()] }
	unit := func() string {
		switch span.Get() {
		case "1M":
			return tr.ChartsDayUnit()
		case "6M":
			return tr.ChartsWeekUnit()
		}
		return tr.ChartsMonthUnit()
	}
	at := func() int {
		if h := hover.Get(); h >= 0 && h < len(rows()) {
			return h
		}
		return len(rows()) - 1
	}
	shown := func() tapeSession { return rows()[at()] }
	prior := func() float64 {
		if i := at(); i > 0 {
			return rows()[i-1].close
		}
		return rows()[0].close
	}
	change := func() float64 { return (shown().close/prior() - 1) * 100 }
	total := func() float64 { r := rows(); return (r[len(r)-1].close/r[0].close - 1) * 100 }
	average := func() float64 {
		sum := 0.0
		for _, s := range rows() {
			sum += s.volume
		}
		return sum / float64(len(rows()))
	}
	dates := func() []float64 {
		n := float64(len(rows()) - 1)
		return []float64{0, math.Round(n / 3), math.Round(2 * n / 3), n}
	}
	title := func(s tapeSession) string {
		if span.Get() == "6M" {
			return tr.ChartsWeekTo(s.label)
		}
		return s.title
	}
	shownTone := func() core.Color {
		if shown().dir == "up" {
			return colors.Up()
		}
		return colors.Down()
	}
	return Card{TestID: "card-volume-price", Title: tr.ChartsVolumePrice(), Description: tr.ChartsVolumePriceNote(), Children: []core.Node{
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.Text{Content: "ORBX", Style: core.Style{FontSize: 13, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
				tipTitleText(cx, func() string { return upper(title(shown())) }),
			}},
			baseline(10, headline(cx, func() string { return "$" + fixed(shown().close, 2) }, nil),
				core.Text{ContentFunc: func() string { return pct(change(), 2) }, Style: core.Style{FontSize: 12, FontWeight: ui.Medium, FontVariant: core.FontTabularNums},
					StyleFunc: func() core.Style { return core.Style{Color: upDown(change()).Color(cx)} }}),
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, ColumnGap: 16, RowGap: 4, Margin: core.Edges{Top: 4}}, Children: []core.Node{
				chartLegend(cx, func() core.Color { return ui.Alpha(shownTone(), 0.6) }, constant(tr.ChartsVolume()), func() string { return shares(shown().volume) }),
				lineKey(cx, chartSeriesColor(cx, 0), tr.ChartsClose(), func() string { return "$" + fixed(shown().close, 2) }),
			}},
		}},
		axisTitles(cx, tr.ChartsShares(), "USD"),
		plot.Plot{
			TestID: "volume-price-plot",
			Count:  func() int { return len(rows()) },
			XScale: func() plot.Scale { return plot.BandScale(plot.Band{PaddingInner: 0.28, PaddingOuter: 0.14}) },
			ColorScale: func() plot.ColorScale {
				return plot.OrdinalColors([]string{"up", "down"}, ui.Alpha(colors.Up(), 0.5), ui.Alpha(colors.Down(), 0.5))
			},
			AppearGeneration: func() any { return span.Get() },
			Motion:           chartMotion(ms(700), ms(600), false),
			LabelFunc:        func() string { return tr.ChartsVolumePriceLabel(span.Get(), pct(total(), 1), shares(average()), unit()) },
			AccessibilityValue: func() core.AccessibilityValue {
				if hover.Get() < 0 {
					return core.AccessibilityValue{}
				}
				return core.AccessibilityValue{Text: tr.ChartsVolumePriceValue(title(shown()), "$"+fixed(shown().close, 2), shares(shown().volume))}
			},
			CrossLineStyleFunc: crossBand(cx, 0.05, 0),
			DotStyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(9), Height: core.Pt(9), CornerRadius: 4.5, BorderWidth: 2, BorderColor: chartSeries(cx)[0], Background: colors.Surface()}
			},
			OnHoverChange: hover.Set,
			Style:         core.Style{Height: core.Pt(tapeHeight), Margin: core.Edges{Left: chartSide, Right: chartSide, Bottom: chartBelow}},
			Children: []core.Node{
				// No values: the lines follow the aligned steps both axes share.
				plot.Grid{StyleFunc: gridStyle(cx)},
				plot.Bar{Value: func() []float64 { return column(rows(), func(s tapeSession) float64 { return s.volume }) },
					Color:   func() []string { return dirs(rows(), func(s tapeSession) string { return s.dir }) },
					Corners: plot.Corners{TopLeft: 1.5, TopRight: 1.5}, Stagger: 0.5, HoverDim: 0.6},
				plot.Line{Y: func() []float64 { return column(rows(), func(s tapeSession) float64 { return s.close }) }, Scale: plot.ScaleY2, Curve: plot.CurveLinear,
					StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(chartSeries(cx)[0]), StrokeWidth: 2} }},
				plot.Axis{Placement: plot.AxisLeft, Format: func(v float64, _ int) string { return shares(v) }, LabelStyleFunc: gutterStyle(cx, chartSide, false)},
				plot.Axis{Placement: plot.AxisRight, Scale: plot.ScaleY2, Format: func(v float64, _ int) string { return fixed(v, 0) },
					LabelStyleFunc: axisStyle(cx, core.Style{Width: core.Pt(chartSide), Padding: core.Edges{Left: 8}})},
				plot.Axis{TickValues: dates, Format: func(v float64, _ int) string {
					if r := rows(); int(v) < len(r) {
						return r[int(v)].label
					}
					return ""
				}, LabelStyleFunc: belowStyle(cx)},
			},
		},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsChange(), func() string { return pct(total(), 1) }, func() Tone { return upDown(total()) }),
			core.Dynamic(func(cx *core.Scope) core.Node {
				return chartFigure(cx, tr.ChartsAvgPer(unit()), func() string { return shares(average()) }, nil)
			}),
			chartFigure(cx, tr.ChartsHigh(), func() string { return "$" + fixed(slices.Max(column(rows(), func(s tapeSession) float64 { return s.close })), 2) }, nil)),
		chartSwitch(tr.ChartsPeriod(), 180, span.Get, func(s string) {
			cx.Batch(func() {
				span.Set(s)
				hover.Set(-1)
			})
		}, ui.Option[string]{Value: "1M", Label: "1M"}, ui.Option[string]{Value: "6M", Label: "6M"}, ui.Option[string]{Value: "2Y", Label: "2Y"}),
	}}
}

var quarterNames = []string{"Q1 25", "Q2 25", "Q3 25", "Q4 25", "Q1 26", "Q2 26", "Q3 26", "Q4 26"}

// segments are two segments' revenue per quarter in billions and their
// net margin in percent: a growing cloud business, and devices that lose
// money outside the holiday quarter.
var segments = map[string][2][]float64{
	"cloud":   {{4.1, 4.4, 4.8, 5.3, 5.7, 6.2, 6.6, 7.2}, {18.0, 19.5, 21.0, 22.4, 23.0, 24.8, 25.5, 27.1}},
	"devices": {{3.2, 2.9, 3.4, 4.6, 3.1, 2.8, 3.3, 4.9}, {-6.5, -9.2, -2.1, 5.8, -4.0, -7.4, 1.2, 8.6}},
}

const marginHeight = 184

// marginsCard is revenue bars on the left scale and the net margin as a
// line with dots on the right one. Both domains are the plot's: aligned,
// and when the margin goes below zero (devices) both scales put zero on
// the same grid line, the revenue axis writing no values under it.
// Switching the segment moves both scales and the marks to the new values.
func marginsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	segment := core.NewSignal(cx, "devices")
	hover := core.NewSignal(cx, -1)
	revenue := func() []float64 { return segments[segment.Get()][0] }
	margins := func() []float64 { return segments[segment.Get()][1] }
	at := func() int {
		if h := hover.Get(); h >= 0 && h < len(quarterNames) {
			return h
		}
		return len(quarterNames) - 1
	}
	year := func() float64 {
		sum := 0.0
		for _, v := range revenue()[4:] {
			sum += v
		}
		return sum
	}
	losses := func() int {
		n := 0
		for _, m := range margins() {
			if m < 0 {
				n++
			}
		}
		return n
	}
	name := func() string {
		if segment.Get() == "cloud" {
			return tr.ChartsCloud()
		}
		return tr.ChartsDevices()
	}
	series := func(i int) func() core.Color { return chartSeriesColor(cx, i) }
	return Card{TestID: "card-margins", Title: tr.ChartsMargins(), Description: tr.ChartsMarginsNote(), Children: []core.Node{
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			tipTitleText(cx, func() string { return upper(strings.Replace(quarterNames[at()], " ", " 20", 1)) }),
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, ColumnGap: 16, RowGap: 4, Margin: core.Edges{Top: 2}}, Children: []core.Node{
				chartLegend(cx, func() core.Color { return ui.Alpha(series(1)(), 0.5) }, constant(tr.ChartsRevenueStep()), func() string { return "$" + fixed(revenue()[at()], 1) + "B" }),
				lineKey(cx, series(0), tr.ChartsNetMargin(), func() string { return pct(margins()[at()], 1) }),
			}},
		}},
		axisTitles(cx, "USD", tr.ChartsMargin()),
		plot.Plot{
			TestID:           "margins-plot",
			Count:            func() int { return len(quarterNames) },
			XScale:           func() plot.Scale { return plot.BandScale(plot.Band{PaddingInner: 0.4, PaddingOuter: 0.2, MaxBandWidth: 44}) },
			AppearGeneration: func() any { return 0 },
			Motion:           chartMotion(ms(700), ms(600), false),
			LabelFunc:        func() string { return tr.ChartsMarginsLabel(name(), "$"+fixed(year(), 1)+"B", itoa(losses())) },
			AccessibilityValue: func() core.AccessibilityValue {
				if hover.Get() < 0 {
					return core.AccessibilityValue{}
				}
				return core.AccessibilityValue{Text: tr.ChartsMarginsValue(quarterNames[at()], "$"+fixed(revenue()[at()], 1)+"B", fixed(margins()[at()], 1)+"%")}
			},
			CrossLineStyleFunc: crossBand(cx, 0.05, 0),
			OnHoverChange:      hover.Set,
			Style:              core.Style{Height: core.Pt(marginHeight), Margin: core.Edges{Left: chartSide, Right: chartSide, Bottom: 2*chartBelow + 2}},
			Children: []core.Node{
				plot.Grid{StyleFunc: gridStyle(cx)},
				// Zero, the same line on both scales.
				plot.Grid{Y: func() []float64 { return []float64{0} }, StyleFunc: baselineStyle(cx)},
				plot.Bar{Value: revenue, Corners: plot.Corners{TopLeft: 3, TopRight: 3}, Stagger: 0.4, HoverDim: 0.55,
					StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(ui.Alpha(series(1)(), 0.5))} }},
				plot.Line{Y: margins, Scale: plot.ScaleY2, Curve: plot.CurveLinear, Dot: true,
					DotStyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 3.5, BorderWidth: 1.5, BorderColor: series(0)(), Background: colors.Surface()}
					},
					HoverDotStyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(11), Height: core.Pt(11), CornerRadius: 5.5, BorderWidth: 2, BorderColor: series(0)(), Background: colors.Surface()}
					},
					StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(series(0)()), StrokeWidth: 2} }},
				plot.Axis{Placement: plot.AxisLeft, Format: func(v float64, _ int) string {
					switch {
					case v < 0:
						return ""
					case v == 0:
						return "$0"
					}
					return "$" + fixed(v, 0) + "B"
				}, LabelStyleFunc: gutterStyle(cx, chartSide, false)},
				plot.Axis{Placement: plot.AxisRight, Scale: plot.ScaleY2, Format: func(v float64, _ int) string { return minus(v, 0) + "%" },
					LabelStyleFunc: axisStyle(cx, core.Style{Width: core.Pt(chartSide), Padding: core.Edges{Left: 8}})},
				// The quarters, and under them each year in the middle of its
				// four, so the labels fit a phone.
				plot.Axis{Format: func(v float64, _ int) string { return quarterNames[int(v)][:2] }, LabelStyleFunc: belowStyle(cx)},
				plot.Axis{TickValues: func() []float64 { return []float64{1.5, 5.5} }, Format: func(v float64, _ int) string {
					if v < 4 {
						return "2025"
					}
					return "2026"
				}, LabelStyleFunc: func() core.Style { return belowStyle(cx)().Merge(core.Style{Padding: core.Edges{Top: chartBelow + 2}, Color: colors.Muted()}) }},
			},
		},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsLast4Q(), func() string { return "$" + fixed(year(), 1) + "B" }, nil),
			chartFigure(cx, tr.ChartsBestMargin(), func() string { return pct(slices.Max(margins()), 1) }, fixedTone(ToneUp)),
			chartFigure(cx, tr.ChartsLossQuarters(), func() string { return itoa(losses()) + " / " + itoa(len(quarterNames)) }, func() Tone {
				if losses() > 0 {
					return ToneDown
				}
				return ToneNeutral
			})),
		chartSwitch(tr.ChartsSegment(), 220, segment.Get, segment.Set,
			ui.Option[string]{Value: "cloud", Label: tr.ChartsCloud()}, ui.Option[string]{Value: "devices", Label: tr.ChartsDevices()}),
	}}
}
