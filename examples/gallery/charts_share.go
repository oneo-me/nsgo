package main

import (
	"math"
	"strings"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// resource is a resource's load: its capacity and what is in use now, at
// the day's peak and at the week's.
type resource struct {
	name, unit string
	total      float64
	used       [3]float64
}

var resources = []resource{
	{"Compute", "vCPU", 64, [3]float64{38, 51, 59}},
	{"Memory", "GB", 256, [3]float64{141, 188, 214}},
	{"Network", "Gbps", 10, [3]float64{3.2, 7.4, 8.9}},
}

const (
	// ringWidth and ringGap are each ring's width and the gap between
	// rings.
	ringWidth = 11
	ringGap   = 5
)

// capacityCard is capacity as rings: a track per resource and over it the
// share in use, with round caps, sweeping in from 12 o'clock and then round
// to each period's load (the plot's update); the average in the middle and
// the figures beside, in the rings' colors.
func capacityCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	period := core.NewSignal(cx, 0)
	periods := []string{tr.ChartsNowPeriod(), tr.ChartsDayPeak(), tr.ChartsWeekPeak()}
	share := func(r resource) float64 { return r.used[period.Get()] / r.total }
	average := func() float64 {
		sum := 0.0
		for _, r := range resources {
			sum += share(r)
		}
		return sum / float64(len(resources))
	}
	size := core.Points(172)
	if chartPointer() {
		size = 164
	}
	outer := size/2 - 2
	var tracks, loads, rows []core.Node
	for i, r := range resources {
		o := outer - core.Points(i)*(ringWidth+ringGap)
		in, out := func() core.Length { return core.Pt(o - ringWidth) }, func() core.Length { return core.Pt(o) }
		color := chartSeriesColor(cx, i)
		tracks = append(tracks, plot.Arc{StartAngle: func() float32 { return 0 }, EndAngle: func() float32 { return 2 * math.Pi }, InnerRadius: in, OuterRadius: out,
			StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(ui.Alpha(color(), 0.13))} }})
		loads = append(loads, plot.Arc{StartAngle: func() float32 { return 0 }, EndAngle: func() float32 { return float32(share(r) * 2 * math.Pi) }, InnerRadius: in, OuterRadius: out,
			CornerRadius: ringWidth / 2, StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(color())} }})
		rows = append(rows, core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4}, StyleFunc: ui.Filled(color)},
				core.Text{Content: r.name, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{ContentFunc: func() string { return fixed(share(r)*100, 0) + "%" }, Style: core.Style{FontSize: 15, FontWeight: ui.Semibold, FontVariant: core.FontTabularNums},
					StyleFunc: ui.Colored(colors.Text)},
			}},
			core.Text{ContentFunc: func() string {
				return trimFloat(r.used[period.Get()]) + " / " + trimFloat(r.total) + " " + r.unit
			}, StyleFunc: axisStyle(cx, core.Style{Margin: core.Edges{Left: 16}})},
		}})
	}
	children := append(tracks, loads...)
	children = append(children, core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0),
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 2}, Children: []core.Node{
		core.Text{ContentFunc: func() string { return fixed(average()*100, 0) + "%" }, Style: core.Style{FontSize: 22, FontWeight: ui.Semibold, LetterSpacing: -0.4, FontVariant: core.FontTabularNums},
			StyleFunc: ui.Colored(colors.Text)},
		core.Text{Content: upper(tr.ChartsAvgLoad()), Style: core.Style{FontSize: 10, FontFamily: ui.Mono, LetterSpacing: 0.8}, StyleFunc: ui.Colored(colors.Faint)},
	}})
	layout := core.Style{AlignItems: core.AlignCenter, Gap: 16}
	figures := core.Style{AlignSelf: core.AlignStretch, JustifyContent: core.JustifyCenter, Gap: 14}
	if chartPointer() {
		layout.Direction, layout.Gap, figures.FlexGrow = core.Row, 24, 1
	}
	var options []ui.Option[int]
	for i, p := range periods {
		options = append(options, ui.Option[int]{Value: i, Label: p})
	}
	return Card{TestID: "card-capacity", Title: tr.ChartsCapacity(), Description: tr.ChartsCapacityNote(), Children: []core.Node{
		core.View{Style: layout, Children: []core.Node{
			plot.Plot{
				TestID:           "capacity-plot",
				Count:            func() int { return len(resources) },
				NoHover:          true,
				AppearGeneration: func() any { return 1 },
				Motion:           chartMotion(ms(1100), ms(700), true),
				LabelFunc: func() string {
					var parts []string
					for _, r := range resources {
						parts = append(parts, r.name+" "+fixed(share(r)*100, 0)+"%")
					}
					return tr.ChartsCapacityLabel(periods[period.Get()], strings.Join(parts, ", "))
				},
				Style:    core.Style{Width: core.Pt(size), Height: core.Pt(size)},
				Children: children,
			},
			core.View{Style: figures, Children: rows},
		}},
		chartSwitch(tr.ChartsPeriod(), 260, period.Get, period.Set, options...),
	}}
}

// trimFloat writes v without a fraction it does not have.
func trimFloat(v float64) string {
	if v == math.Trunc(v) {
		return fixed(v, 0)
	}
	return fixed(v, 1)
}

// region is a region's revenue in a quarter, in thousands.
type region struct {
	name    string
	revenue float64
}

// regions are a quarter's revenue by region, the largest first.
var regions = []region{{"North America", 482}, {"EMEA", 312}, {"APAC", 268}, {"Latin America", 104}, {"Other", 74}}

var regionsTotal = func() (sum float64) {
	for _, r := range regions {
		sum += r.revenue
	}
	return sum
}()

// money writes thousands of dollars.
func money(k float64) string {
	if k >= 1000 {
		return "$" + fixed(k/1000, 2) + "M"
	}
	return "$" + fixed(k, 0) + "k"
}

// portion is k's share of the quarter's revenue.
func portion(k float64) string { return fixed(k/regionsTotal*100, 1) + "%" }

// revenueCard is revenue by region as a donut: 2-point gaps of even width,
// softly rounded corners, the hovered slice lifted and the others dimmed,
// its value in the hole, a legend with tabular figures at the trailing edge
// whose hovered row stands out; it closes into a pie, the hole's radius
// moving on the plot's update as the ring draws in, its shares written
// outside at the ends of leader lines in the slices' colors.
func revenueCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	hover := core.NewSignal(cx, -1)
	donut := core.NewSignal(cx, true)
	// The ring's size, its rim 8 points in (room for the lift), the gaps
	// between slices 2 points wide there; as a pie it draws in to leave room
	// for its labels outside.
	ring := core.Points(176)
	if chartPointer() {
		ring = 168
	}
	rim := ring/2 - 8
	pieRim := core.Points(math.Round(float64(rim) * 0.66))
	// The ring turned so its two smallest slices, the last, meet at 3
	// o'clock: their labels stack there instead of crowding at the top.
	smallest := regions[3].revenue + regions[4].revenue
	start := float32(math.Pi/2 - 2*math.Pi*(regionsTotal-smallest/2)/regionsTotal)
	names := make([]string, len(regions))
	for i, r := range regions {
		names[i] = r.name
	}
	shown := func() (region, bool) {
		if h := hover.Get(); h >= 0 && h < len(regions) {
			return regions[h], true
		}
		return region{}, false
	}
	var legend []core.Node
	for i, r := range regions {
		height := core.Points(36)
		if chartPointer() {
			height = 30
		}
		legend = append(legend, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Height: core.Pt(height), Padding: core.Symmetric(0, 8), CornerRadius: 6,
			Transition: core.Transition{Duration: ms(150), Properties: core.TransitionOpacity | core.TransitionBackground}},
			StyleFunc: func() core.Style {
				s := core.Style{Opacity: core.Alpha(1)}
				if h := hover.Get(); h == i {
					s.Background = ui.Alpha(colors.Text(), 0.05)
				} else if h >= 0 {
					s.Opacity = core.Alpha(0.45)
				}
				return s
			}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 2}, StyleFunc: ui.Filled(chartSeriesColor(cx, i))},
				core.Text{Content: r.name, Style: core.Style{NumberOfLines: 1, FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{Content: money(r.revenue), Style: core.Style{FontSize: 13, FontVariant: core.FontTabularNums, TextAlign: core.TextAlignEnd}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{Content: portion(r.revenue), Style: core.Style{Width: core.Pt(48), FontSize: 13, FontVariant: core.FontTabularNums, TextAlign: core.TextAlignEnd}, StyleFunc: ui.Colored(colors.Muted)},
			}})
	}
	layout := core.Style{Wrap: core.Wrap, JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter, Gap: 16}
	legendStyle := core.Style{AlignSelf: core.AlignStretch, FlexGrow: 1, JustifyContent: core.JustifyCenter, Gap: 2}
	if chartPointer() {
		layout.Direction, layout.Wrap, layout.Gap, legendStyle.FlexBasis = core.Row, core.NoWrap, 24, core.Pt(0)
	}
	button := core.Style{Direction: core.Row, Margin: core.Edges{Top: chartGap() + 4}}
	flex := float32(1)
	if chartPointer() {
		button.AlignSelf, flex = core.AlignStart, 0
	}
	return Card{TestID: "card-revenue", Title: tr.ChartsRevenue(), Description: tr.ChartsRevenueNote(), Children: []core.Node{
		// The legend sits beside the ring where the card has room for its
		// names and below it where it has not, never cutting a name short.
		core.View{Style: layout, Children: []core.Node{
			plot.Plot{
				TestID:           "revenue-plot",
				Y:                func() []float64 { return column(regions, func(r region) float64 { return r.revenue }) },
				Label:            tr.ChartsRevenueLabel(money(regionsTotal), regions[0].name, portion(regions[0].revenue)),
				AccessibilityValue: func() core.AccessibilityValue {
					if r, ok := shown(); ok {
						return core.AccessibilityValue{Text: r.name + ": " + money(r.revenue) + ", " + portion(r.revenue)}
					}
					return core.AccessibilityValue{}
				},
				AppearGeneration: func() any { return 1 },
				Motion: func() plot.Motion {
					m := chartMotion(ms(900), ms(450), false)
					m.Exit.Duration, m.Pointer = ms(180), &core.SpringConfig{Stiffness: 420, Damping: 40}
					return m
				}(),
				OnHoverChange: hover.Set,
				ColorScale:    func() plot.ColorScale { return plot.OrdinalColors(names, chartSeries(cx)...) },
				Style:         core.Style{Width: core.Pt(ring), Height: core.Pt(ring)},
				Children: []core.Node{
					plot.Pie{
						Color:      func() []string { return names },
						StartAngle: func() float32 { return start },
						EndAngle:   func() float32 { return start + 2*math.Pi },
						InnerRadius: func() core.Length {
							if donut.Get() {
								return core.Pt(rim * 0.62)
							}
							return core.Pt(0)
						},
						OuterRadius: func() core.Length {
							if donut.Get() {
								return core.Pt(rim)
							}
							return core.Pt(pieRim)
						},
						PadAngle:     func() float32 { return float32(2 / rim) },
						CornerRadius: 3, HoverLift: 6, HoverDim: 0.64,
						Label: func() []string {
							if donut.Get() {
								return nil
							}
							out := make([]string, len(regions))
							for i, r := range regions {
								out[i] = fixed(math.Round(r.revenue/regionsTotal*100), 0) + "%"
							}
							return out
						},
						LabelGap: 10,
						LabelStyleFunc: func() core.Style {
							return core.Style{Padding: core.Symmetric(0, 4), FontSize: 11, FontFamily: ui.Mono, Color: colors.Muted(), FontVariant: core.FontTabularNums}
						},
						LabelLineStyleOf: func(row int) core.Style {
							return core.Style{Stroke: core.PaintColor(chartSeries(cx)[row%5]), StrokeWidth: 1}
						},
					},
					// The total in the hole fades as it opens and closes.
					core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0),
						AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 2, Transition: core.Transition{Duration: ms(300), Properties: core.TransitionOpacity}},
						StyleFunc: func() core.Style {
							if donut.Get() {
								return core.Style{Opacity: core.Alpha(1)}
							}
							return core.Style{Opacity: core.Alpha(0)}
						}, Children: []core.Node{
							core.Text{ContentFunc: func() string {
								if r, ok := shown(); ok {
									return upper(r.name)
								}
								return upper(tr.ChartsTotal())
							}, Style: core.Style{FontSize: 10, FontFamily: ui.Mono, LetterSpacing: 0.8}, StyleFunc: ui.Colored(colors.Faint)},
							core.Text{ContentFunc: func() string {
								if r, ok := shown(); ok {
									return money(r.revenue)
								}
								return money(regionsTotal)
							}, Style: core.Style{FontSize: 20, FontWeight: ui.Semibold, LetterSpacing: -0.4, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Text)},
						}},
				},
			},
			core.View{Style: legendStyle, Children: legend},
		}},
		core.View{Style: button, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
			text := tr.ChartsShowAsPie()
			if !donut.Get() {
				text = tr.ChartsShowAsDonut()
			}
			size := ui.SizeSmall
			if !chartPointer() {
				size = ui.SizeDefault
			}
			return ui.Button{Text: text, Variant: ui.ButtonSecondary, Size: size, Flex: flex, OnPress: func() {
				cx.Batch(func() {
					donut.Set(!donut.GetUntracked())
					hover.Set(-1)
				})
			}}
		})}},
	}}
}
