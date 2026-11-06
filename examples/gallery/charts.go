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

// chartDayNames are the days of a week, short, in tr's language.
func chartDayNames(tr locales.Messages) []string {
	return []string{tr.ChartsMon(), tr.ChartsTue(), tr.ChartsWed(), tr.ChartsThu(), tr.ChartsFri(), tr.ChartsSat(), tr.ChartsSun()}
}

// chartWeeks are visitors per day of two weeks.
var chartWeeks = [][]float64{
	{1820, 2140, 1960, 2480, 2310, 1290, 1180},
	{1910, 2260, 2390, 2650, 2520, 1420, 1330},
}

const (
	// visitorsTop is the top of the visitors' y scale: a step above the
	// highest grid line, room for the tooltip.
	visitorsTop = 3500
	// chartAxis is the width of the y axis' labels.
	chartAxis = 24
)

// ticker is a sparkline's row: a symbol and its walk of prices.
type ticker struct {
	id     int
	symbol string
	prices []float64
}

// walk is a repeatable walk of count prices from start.
func walk(count int, seed, start float64) []float64 {
	price, s := start, seed
	out := make([]float64, count)
	for i := range out {
		s = math.Mod(s*16807, 2147483647)
		price += (math.Mod(s, 1000)/1000 - 0.48) * start * 0.02
		out[i] = price
	}
	return out
}

var tickers = func() []ticker {
	symbols := []string{"NSHL", "GPUI", "TAFY", "QJSX", "RUST", "OKLC", "SKIA", "CORE", "BASE", "WASM", "GTK4", "D2D1"}
	out := make([]ticker, 48)
	for n := range out {
		symbol := symbols[n%len(symbols)]
		if n >= len(symbols) {
			symbol += itoa(n / len(symbols))
		}
		out[n] = ticker{id: n, symbol: symbol, prices: walk(40, float64(7+n*13), float64(40+(n*37)%160))}
	}
	return out
}()

// visitors writes n with thousands separators.
func visitors(n float64) string {
	s := itoa(int(n))
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// seriesKey is a series' key, as its mark: a short line, dashed for the
// other week.
func seriesKey(cx *core.Scope, dashed bool) core.Node {
	colors := ui.ColorsOf(cx)
	if dashed {
		dash := core.View{Style: core.Style{Width: core.Pt(4), Height: core.Pt(1.5)}, StyleFunc: ui.Filled(colors.Muted)}
		return core.View{Style: core.Style{Width: core.Pt(10), Direction: core.Row, Gap: 2}, Children: []core.Node{dash, dash}}
	}
	return core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(2), CornerRadius: 1}, StyleFunc: ui.Filled(ui.ToneAccent.Shade(cx))}
}

// legendItem is the key, then the series' name.
func legendItem(cx *core.Scope, label func() string, dashed bool) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		seriesKey(cx, dashed), core.Text{ContentFunc: label, Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)},
	}}
}

// wash is a wash of c under a line: its background, fading out from the
// line's highest point (16%) to the baseline.
func wash(c core.Color) core.Style {
	return core.Style{Gradients: []core.Gradient{{Angle: 180, Stops: []core.ColorStop{{Color: ui.Alpha(c, 0.16)}, {Color: ui.Alpha(c, 0)}}}}}
}

// tipValue is a value in the tooltip, after its series' key.
func tipValue(cx *core.Scope, value func() string, dashed bool) core.Node {
	colors := ui.ColorsOf(cx)
	c := colors.Text
	if dashed {
		c = colors.Muted
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		seriesKey(cx, dashed), ui.Tabular(ui.Textf(value, 12, ui.Regular, c)),
	}}
}

// chartsDemo shows base's Plot with PlotLine, PlotArea and PlotGrid,
// drawn natively from the app's rows: a week of visitors over the other
// week (dashed), with a legend, compact y labels and the hover's cross
// line, dots (each series' own, in its color, ringed in the card's) and
// tooltip gliding on the frame clock, and a list of sparklines, each with
// a wash in its tone and its change beside it, that draw in as their rows
// come. Washes fade out toward the baseline (an area's background, a
// gradient inside its shape). Grid lines are hairlines with a firmer
// baseline; the tooltip floats in the room above the highest value and
// stays within the plot's width at the first and last days.
func chartsDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	chartDays, chartWeekNames := chartDayNames(tr), []string{tr.ChartsLastWeek(), tr.ChartsThisWeek()}
	last := core.NewSignal(cx, "—")
	week := core.NewSignal(cx, 0)
	pointer := shell.desktop()
	gap := core.Points(12)
	label, detail := core.Points(15), core.Points(13)
	if pointer {
		gap, label, detail = 10, 14, 12
	}

	visitorsCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		hover := core.NewSignal(cx, -1)
		// The last day hovered: the tooltip keeps it while it fades out.
		tip := core.NewSignal(cx, 0)
		rows := func() []float64 { return chartWeeks[week.Get()] }
		other := func() []float64 { return chartWeeks[1-week.Get()] }
		shown := func() int {
			if h := hover.Get(); h >= 0 {
				return h
			}
			return len(rows()) - 1
		}
		var yLabels []core.Node
		for _, v := range []float64{0, 1000, 2000, 3000} {
			s := "0"
			if v > 0 {
				s = fmt.Sprintf("%gk", v/1000)
			}
			yLabels = append(yLabels, core.Text{Content: s, Style: core.Style{Position: core.Absolute, Right: core.Pt(0), FontVariant: core.FontTabularNums,
				Top: core.Pt(core.Points((1-v/visitorsTop)*160 - 8)), FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)})
		}
		var days []core.Node
		for _, d := range chartDays {
			days = append(days, ui.Text(d, 12, ui.Regular, colors.Muted))
		}
		switchAlign := core.AlignStretch
		if pointer {
			switchAlign = core.AlignStart
		}
		switchLabel := func() string {
			if week.Get() == 0 {
				return tr.ChartsShowThisWeek()
			}
			return tr.ChartsShowLastWeek()
		}
		return ui.Card{Title: tr.ChartsVisitors(), Description: tr.ChartsAWeekOverTheOther(), Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node {
				tr := locales.For(cx)
				day := chartDays[shown()]
				if hover.Get() < 0 {
					day += tr.ChartsTheLastDay()
				}
				return Caption{Label: day, Note: func() string { return visitors(rows()[shown()]) + tr.ChartsVisitors2() }}
			}),
			core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{
				legendItem(cx, func() string { return chartWeekNames[week.Get()] }, false),
				legendItem(cx, func() string { return chartWeekNames[1-week.Get()] }, true),
			}},
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				plot.Plot{
					TestID: "visitors-plot",
					Y:      rows,
					// Room above the highest week for the tooltip.
					YScale:           func() plot.Scale { return plot.LinearScale(0, visitorsTop) },
					AppearGeneration: week.Get,
					Motion: plot.Motion{
						Appear:  &core.TimingConfig{Duration: 600 * time.Millisecond, Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}},
						Enter:   &core.TimingConfig{Duration: 150 * time.Millisecond},
						Exit:    &core.TimingConfig{Duration: 150 * time.Millisecond},
						Pointer: &core.SpringConfig{Stiffness: 320, Damping: 32},
					},
					LabelFunc: func() string {
						r := rows()
						return tr.ChartsVisitorsPerDay(visitors(r[0]), visitors(r[6]))
					},
					AccessibilityValue: func() core.AccessibilityValue {
						h := hover.Get()
						if h < 0 {
							return core.AccessibilityValue{}
						}
						return core.AccessibilityValue{Text: tr.ChartsHoverValue(chartDays[h], visitors(rows()[h]), visitors(other()[h]), strings.ToLower(chartWeekNames[1-week.Get()]))}
					},
					CrossLineStyleFunc: func() core.Style { return core.Style{Width: core.Pt(1), Background: ui.Alpha(colors.Text(), 0.3)} },
					// The dots' shape; each week's line gives its own its color.
					DotStyle:      core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, BorderWidth: 2},
					TooltipOffset: 4,
					Tooltip: core.View{StyleFunc: func() core.Style {
						return fin.Menu().Over(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10,
							Padding: core.Symmetric(0, 8), Height: core.Pt(26), CornerRadius: 8, BorderWidth: 1})
					},
						Children: []core.Node{
							tipValue(cx, func() string { return visitors(rows()[tip.Get()]) }, false),
							tipValue(cx, func() string { return visitors(other()[tip.Get()]) }, true),
						}},
					OnHoverChange: func(i int) {
						hover.Set(i)
						if i < 0 {
							last.Set(tr.ChartsPlotLeft())
							return
						}
						tip.Set(i)
						last.Set(tr.ChartsPlotEvent(chartDays[i], visitors(rows()[i])))
					},
					Style: core.Style{FlexGrow: 1, Height: core.Pt(160)},
					Children: []core.Node{
						plot.Grid{Y: func() []float64 { return []float64{1000, 2000, 3000} }, StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Line())} }},
						// The line over it has the dot.
						plot.Area{BackgroundStyleFunc: func() core.Style { return wash(ui.ToneAccent.Color(cx)) }, HoverDotStyle: core.Style{Display: core.DisplayNone}},
						plot.Grid{Y: func() []float64 { return []float64{0} }, StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Border())} }},
						// Each week's dot in its line's color, ringed in the
						// card's, so it cuts out of the line.
						plot.Line{Y: other, StyleFunc: func() core.Style {
							return core.Style{Stroke: core.PaintColor(colors.Muted()), StrokeWidth: 1.5, StrokeDash: []core.Points{4, 4}}
						},
							HoverDotStyleFunc: func() core.Style { return core.Style{Background: colors.Muted(), BorderColor: colors.Surface()} }},
						plot.Line{StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(ui.ToneAccent.Color(cx)), StrokeWidth: 2} },
							HoverDotStyleFunc: func() core.Style {
								return core.Style{Background: ui.ToneAccent.Color(cx), BorderColor: colors.Surface()}
							}},
					},
				},
				// The y axis' labels at the grid's values, compact.
				core.View{Style: core.Style{Width: core.Pt(chartAxis), Height: core.Pt(160)}, Children: yLabels},
			}},
			core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween, Margin: core.Edges{Right: chartAxis + 8}},
				Children: days},
			core.View{Style: core.Style{Direction: core.Row, AlignSelf: switchAlign},
				Children: []core.Node{core.Dynamic(func(cx *core.Scope) core.Node {
					tr := locales.For(cx)
					var flex float32
					if !pointer {
						flex = 1
					}
					return ui.Button{Text: switchLabel(), Variant: ui.ButtonSecondary, Small: pointer, Flex: flex, OnPress: func() {
						week.Set(1 - week.GetUntracked())
						hover.Set(-1)
						if week.GetUntracked() == 0 {
							last.Set(tr.ChartsPlotLastWeek())
						} else {
							last.Set(tr.ChartsPlotThisWeek())
						}
					}}
				})}},
		}}
	}

	sparklines := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Card{Title: tr.ChartsSparklines(), Description: tr.ChartsAPlotPerRowOf(), Children: []core.Node{
			core.List{TestID: "sparklines", Count: func() int { return len(tickers) }, Key: func(i int) any { return tickers[i].id },
				EstimatedSize: 44, Style: core.Style{Height: core.Pt(44 * 6)},
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					colors := ui.ColorsOf(cx)
					t := func() ticker { return tickers[r.Index()] }
					first := func() float64 { return t().prices[0] }
					closing := func() float64 { p := t().prices; return p[len(p)-1] }
					up := func() bool { return closing() >= first() }
					tn := func() ui.Tone {
						if up() {
							return ui.ToneUp
						}
						return ui.ToneDown
					}
					change := func() float64 { return (closing() - first()) / first() * 100 }
					changeText := func() string {
						sign := "−"
						if up() {
							sign = "+"
						}
						return fmt.Sprintf("%s%.2f%%", sign, math.Abs(change()))
					}
					return core.View{Style: core.Style{Height: core.Pt(44), Direction: core.Row, AlignItems: core.AlignCenter, Gap: gap}, Children: []core.Node{
						core.Text{ContentFunc: func() string { return t().symbol }, Style: core.Style{Width: core.Pt(64), FontSize: label, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)},
						plot.Plot{
							Y:                func() []float64 { return t().prices },
							NoHover:          true,
							AppearGeneration: func() int { return t().id },
							Motion:           plot.Motion{Appear: &core.TimingConfig{Duration: 400 * time.Millisecond, Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}}},
							LabelFunc: func() string {
								dir := "down"
								if up() {
									dir = "up"
								}
								return fmt.Sprintf("%s, %s %.2f%%", t().symbol, dir, math.Abs(change()))
							},
							Style: core.Style{FlexGrow: 1, Height: core.Pt(28)},
							Children: []core.Node{
								// A wash under the line, in its tone, fading out.
								plot.Area{BackgroundStyleFunc: func() core.Style { return wash(tn().Color(cx)) }},
								plot.Line{StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(tn().Color(cx)), StrokeWidth: 1.5} }},
							},
						},
						// The close over its change: an explicit plus, a true minus.
						core.View{Style: core.Style{Width: core.Pt(72), AlignItems: core.AlignEnd, Gap: 2}, Children: []core.Node{
							core.Text{ContentFunc: func() string { return fmt.Sprintf("%.2f", closing()) }, Style: core.Style{FontSize: label, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Text)},
							core.Text{ContentFunc: changeText, Style: core.Style{FontVariant: core.FontTabularNums}, StyleFunc: func() core.Style { return core.Style{FontSize: detail, Color: tn().Color(cx)} }},
						}},
					}}
				}},
		}}
	}

	return stagePage(cx, demoPage(cx, pageProps{flush: true}, visitorsCard, sparklines),
		Section{Title: tr.TerminalOptions(), Note: "plot.Plot", Children: []core.Node{
			choiceKnob("Y", week, ui.Option[int]{Value: 0, Label: chartWeekNames[0]}, ui.Option[int]{Value: 1, Label: chartWeekNames[1]}),
		}},
		Section{Title: tr.ChartsLastEvent(), Note: "OnHoverChange", Children: []core.Node{
			core.Text{ContentFunc: last.Get, Style: core.Style{FontSize: 13, FontFamily: ui.Mono}, StyleFunc: ui.Colored(ui.ColorsOf(cx).Text)},
		}})
}
