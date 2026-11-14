package main

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

const (
	livePoints = 60
	// liveChannel is the ticks the live price's channel spans: each tick's
	// high and low of the last ten.
	liveChannel = 10
	liveHeight  = 176
	// liveGutter is the room right of the live plot: the y labels and the
	// last price's tag.
	liveGutter = 58
	// liveOpen is the session's open.
	liveOpen = 41.2
	// liveStart is the first tick's time of day, in seconds.
	liveStart = 10*3600 + 42*60
)

// livePoint is a tick: its second, its price, and the high and low of the
// ten before it.
type livePoint struct {
	t                 int
	price, high, low float64
}

// liveClock is a tick's time of day.
func liveClock(t int) string {
	s := liveStart + t
	return fmt.Sprintf("%02d:%02d:%02d", s/3600, s/60%60, s%60)
}

// liveFit is a y domain round rows, with room above and below.
func liveFit(rows []livePoint) [2]float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, r := range rows {
		lo, hi = min(lo, r.low), max(hi, r.high)
	}
	pad := max(0.2, (hi-lo)*0.3)
	return [2]float64{math.Floor((lo-pad)*10) / 10, math.Ceil((hi+pad)*10) / 10}
}

// column reads a column of rows.
func column[T any](rows []T, f func(T) float64) []float64 {
	out := make([]float64, len(rows))
	for i, r := range rows {
		out[i] = f(r)
	}
	return out
}

// liveRefit is how a re-fit of the live plot's y domain glides: its marks
// and labels over the plot's update, the newest dot with the layout.
var liveRefit = func() core.LayoutAnimation {
	t := core.LayoutTransition{Curve: core.LayoutEaseInEaseOut, Duration: 450 * time.Millisecond}
	return core.LayoutAnimation{Create: &t, Update: &t, Delete: &t}
}()

// liveCard is a minute of a price, a tick a second: the newest point is
// the plot's right edge, its dot breathing while the stream runs, its price
// tagged in the gutter at its height and the window's high written over
// its point (a Label each, placed by the plot's scales), inside a channel
// of each tick's high and low over the last ten (an area with Lows: a
// band). The y domain holds still while the window fits in it and re-fits
// only when the price leaves it or uses too little of it, so the grid does
// not jump with every tick; a re-fit glides, the marks and labels on the
// plot's update, the newest dot with a layout animation of the same length.
func liveCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	next := chartRandom(29)
	t, price := 0, liveOpen
	var recent []float64
	step := func() livePoint {
		price = max(1, price+(next()-0.485)*0.16)
		t++
		rounded := math.Round(price*100) / 100
		recent = append(recent, rounded)
		if len(recent) > liveChannel {
			recent = recent[1:]
		}
		return livePoint{t, rounded, slices.Max(recent), slices.Min(recent)}
	}
	first := make([]livePoint, livePoints)
	for i := range first {
		first[i] = step()
	}
	points := core.NewSignalFunc(cx, first, nil)
	domain := core.NewSignal(cx, liveFit(first))
	running := core.NewSignal(cx, true)
	hover := core.NewSignal(cx, -1)
	tip := core.NewSignal(cx, livePoints-1)
	var tick func()
	tick = func() {
		cx.AfterFunc(time.Second, tick)
		if !running.GetUntracked() {
			return
		}
		rows := append(slices.Clone(points.GetUntracked()[1:]), step())
		d := domain.GetUntracked()
		fit := liveFit(rows)
		lo, hi := slices.Min(column(rows, func(r livePoint) float64 { return r.low })), slices.Max(column(rows, func(r livePoint) float64 { return r.high }))
		refit := lo < d[0] || hi > d[1] || hi-lo < (d[1]-d[0])*0.3
		if refit && !cx.Env().ReduceMotion() {
			cx.AnimateLayout(liveRefit)
		}
		cx.Batch(func() {
			points.Set(rows)
			if refit {
				domain.Set(fit)
			}
		})
	}
	cx.AfterFunc(time.Second, tick)

	last := func() float64 { p := points.Get(); return p[len(p)-1].price }
	change := func() float64 { return last() - liveOpen }
	tone := func() core.Color {
		if change() >= 0 {
			return colors.Up()
		}
		return colors.Down()
	}
	y := func(v float64) core.Points {
		d := domain.Get()
		return core.Points((1 - (v-d[0])/(d[1]-d[0])) * liveHeight)
	}
	grid := func() []float64 { d := domain.Get(); return gridTicks(d[0], d[1], 4) }
	prices := func() []float64 { return column(points.Get(), func(p livePoint) float64 { return p.price }) }
	high := func() float64 { return slices.Max(prices()) }
	low := func() float64 { return slices.Min(prices()) }
	// The row of the window's high (its first, if it repeats), its text
	// centered over it but kept off the plot's ends.
	peak := func() int { return slices.Index(prices(), high()) }
	peakAlign := func() plot.TextAlign {
		switch p := peak(); {
		case p < 8:
			return plot.TextLeft
		case p > livePoints-9:
			return plot.TextRight
		}
		return plot.TextCenter
	}
	breathing := func() bool { return running.Get() && !cx.Env().ReduceMotion() }
	dot := func(size core.Points, c func() core.Color, ring bool) core.Node {
		return core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(-size / 2), Top: core.Pt(-size / 2), Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2},
			StyleFunc: func() core.Style {
				if ring {
					return core.Style{Background: c(), BorderWidth: 2, BorderColor: colors.Inset()}
				}
				return core.Style{Background: c()}
			}}
	}
	ping := func(size core.Points, c func() core.Color) core.Node {
		return core.Show{When: breathing, Then: func(*core.Scope) core.Node {
			v := dot(size, c, false).(core.View)
			v.Class = "animate-ping"
			return v
		}}
	}
	liveTone := func() core.Color {
		if running.Get() {
			return colors.Up()
		}
		return colors.Faint()
	}
	head := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignStart}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, Gap: 6}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.Text{Content: "NSHL", Style: core.Style{FontSize: 13, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
				// Live: a dot that breathes while the stream runs.
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 5}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(6), Height: core.Pt(6)}, Children: []core.Node{
						core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(3), Top: core.Pt(3)}, Children: []core.Node{ping(6, colors.Up), dot(6, liveTone, false)}},
					}},
					core.Text{ContentFunc: func() string {
						if running.Get() {
							return upper(tr.ChartsLive())
						}
						return upper(tr.ChartsPaused())
					}, Style: core.Style{FontSize: 10, FontFamily: ui.Mono, LetterSpacing: 0.8}, StyleFunc: ui.Colored(liveTone)},
				}},
			}},
			baseline(10,
				core.Text{ContentFunc: func() string { return fixed(last(), 2) }, Style: core.Style{FontSize: 30, FontWeight: ui.Semibold, LetterSpacing: -0.6, FontVariant: core.FontTabularNums},
					StyleFunc: ui.Colored(colors.Text)},
				core.Text{ContentFunc: func() string { return sign(change(), 2) + "  " + pct(change()/liveOpen*100, 2) },
					Style: core.Style{FontSize: 13, FontWeight: ui.Medium, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(tone)}),
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			text := tr.ChartsPause()
			if !running.Get() {
				text = tr.ChartsResume()
			}
			return ui.Button{Text: text, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: func() { running.Set(!running.GetUntracked()) }}
		}),
	}}
	ago := []string{"−60s", "−45s", "−30s", "−15s", tr.ChartsNow()}
	seconds := func() []float64 { return column(points.Get(), func(p livePoint) float64 { return float64(p.t) }) }
	return Card{TestID: "card-live", Title: tr.ChartsLivePrice(), Description: tr.ChartsLivePriceNote(), Children: []core.Node{
		head,
		spaced(chartGap(), plot.Plot{
			TestID: "live-plot",
			Y: prices,
			// The ticks along their seconds, so a new one slides the window
			// on, each point keeping its place.
			X:      seconds,
			XScale: func() plot.Scale { s := seconds(); return plot.LinearScale(s[0], s[len(s)-1]) },
			YScale: func() plot.Scale { d := domain.Get(); return plot.LinearScale(d[0], d[1]) },
			// The appear runs once; the stream moves the marks on.
			AppearGeneration: func() any { return 1 },
			Motion: plot.Motion{
				Appear:  &core.TimingConfig{Duration: ms(900), Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}},
				Update:  &core.TimingConfig{Duration: ms(450), Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeInOut}},
				Enter:   &core.TimingConfig{Duration: ms(120)},
				Exit:    &core.TimingConfig{Duration: ms(160)},
				Pointer: &core.SpringConfig{Stiffness: 380, Damping: 36},
			},
			LabelFunc: func() string { return tr.ChartsLiveLabel(fixed(last(), 2), pct(change()/liveOpen*100, 2)) },
			AccessibilityValue: func() core.AccessibilityValue {
				h := hover.Get()
				if h < 0 || h >= livePoints {
					return core.AccessibilityValue{}
				}
				p := points.Get()[h]
				return core.AccessibilityValue{Text: liveClock(p.t) + ", " + fixed(p.price, 2)}
			},
			CrossLineStyleFunc: crossLine(cx),
			DotStyle:           hoverDotShape,
			TooltipPlacement:   plot.TooltipPointer,
			TooltipOffset:      10,
			Tooltip: tipCard(cx,
				tipTitleText(cx, func() string { return liveClock(points.Get()[tip.Get()].t) }),
				tipValue(cx, func() string { return fixed(points.Get()[tip.Get()].price, 2) })),
			OnHoverChange: func(i int) {
				hover.Set(i)
				if i >= 0 {
					tip.Set(i)
				}
			},
			Style: core.Style{Height: core.Pt(liveHeight), Margin: core.Edges{Right: liveGutter, Bottom: chartBelow}},
			Children: []core.Node{
				plot.Grid{Y: grid, StyleFunc: gridStyle(cx)},
				// The open: a faint dashed reference, while it is in view.
				plot.Grid{Y: func() []float64 {
					if d := domain.Get(); liveOpen > d[0] && liveOpen < d[1] {
						return []float64{liveOpen}
					}
					return nil
				}, StyleFunc: referenceStyle(cx)},
				plot.Area{BackgroundStyleFunc: func() core.Style { return wash(tone(), 0.2) }, HoverDotStyle: hidden},
				// The channel: each tick's high and low of the ten before it,
				// the line running inside.
				plot.Area{Y1: func() []float64 { return column(points.Get(), func(p livePoint) float64 { return p.high }) },
					Lows:      func() []float64 { return column(points.Get(), func(p livePoint) float64 { return p.low }) },
					StyleFunc: func() core.Style { return core.Style{Fill: core.PaintColor(ui.Alpha(tone(), 0.12))} }, HoverDotStyle: hidden},
				plot.Line{StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(tone()), StrokeWidth: 1.75} },
					HoverDotStyleFunc: hoverDot(cx, tone)},
				// The newest point: a dot ringed in the inset's color, its halo
				// breathing.
				core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pct(100), Width: core.Pt(0), Height: core.Pt(0)},
					StyleFunc: func() core.Style { return core.Style{Top: core.Pt(y(last()))} },
					Children: []core.Node{ping(12, func() core.Color { return ui.Alpha(tone(), 0.5) }), dot(9, tone, true)}},
				// The gutter: the grid's values clear of the last price's tag.
				plot.Axis{Placement: plot.AxisRight, TickValues: func() []float64 {
					var out []float64
					for _, v := range grid() {
						if math.Abs(float64(y(v)-y(last()))) > 14 {
							out = append(out, v)
						}
					}
					return out
				}, Format: func(v float64, _ int) string { return fixed(v, 2) }, LabelStyleFunc: axisStyle(cx, core.Style{Padding: core.Edges{Left: 10}})},
				// The window's high over its point, and the last price's tag at
				// its height in the gutter.
				plot.Label{Texts: func() []plot.PlotText {
					return []plot.PlotText{
						{Text: tr.ChartsHigh() + " " + fixed(high(), 2), X: seconds()[peak()], Y: high(), Align: peakAlign(), Class: "-translate-y-full",
							// On the card's color, so a grid line near the high
							// passes behind it.
							Style: axisText(cx).Merge(core.Style{Color: colors.Muted(), Margin: core.Edges{Top: -6}, Padding: core.Symmetric(0, 4), CornerRadius: 3, Background: colors.Surface()})},
						{Text: fixed(last(), 2), X: seconds()[livePoints-1], Y: last(), Class: "-translate-y-1/2",
							Style: core.Style{Margin: core.Edges{Left: 6}, Padding: core.Symmetric(0, 6), LineHeight: 20, CornerRadius: 6, Background: tone(),
								FontSize: 11, FontFamily: ui.Mono, FontWeight: ui.Semibold, Color: colors.Surface(), FontVariant: core.FontTabularNums}},
					}
				}},
				// The window's seconds under it, the first and last at its ends.
				plot.Axis{TickValues: func() []float64 {
					s := seconds()
					now := s[len(s)-1]
					return []float64{now - 59, now - 45, now - 30, now - 15, now}
				}, Format: func(_ float64, i int) string { return ago[min(i, len(ago)-1)] }, LabelStyleFunc: belowStyle(cx)},
			},
		}),
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsOpen(), func() string { return fixed(liveOpen, 2) }, nil),
			chartFigure(cx, tr.ChartsHigh(), func() string { return fixed(high(), 2) }, nil),
			chartFigure(cx, tr.ChartsLow(), func() string { return fixed(low(), 2) }, nil),
			chartFigure(cx, tr.ChartsRange(), func() string { return fixed((high()-low())/low()*100, 2) + "%" }, nil)),
	}}
}

const (
	// depthMid is the book's middle price, its levels a tick apart on each
	// side.
	depthMid    = 100.0
	depthTick   = 0.02
	depthLevels = 40
	depthHeight = 168
)

// depthLevel is a level of the book: its price and the size summed out to
// it on its side, NaN on the other.
type depthLevel struct{ price, bid, ask float64 }

// depthSides are each side's sizes, thicker away from the middle, with a
// few walls.
var depthSides = func() (bids, asks []float64) {
	r := chartRandom(83)
	side := func() []float64 {
		out := make([]float64, depthLevels)
		for k := range out {
			wall := 1.0
			if r() > 0.9 {
				wall = 3.2
			}
			out[k] = math.Round((30 + r()*140) * (1 + float64(k)/14) * wall)
		}
		return out
	}
	return side(), side()
}

// depthBook is the levels within n ticks of the best prices, from the
// lowest bid up: each side's size summed from the best price out.
func depthBook(n int) []depthLevel {
	bidSizes, askSizes := depthSides()
	nan := math.NaN()
	out := make([]depthLevel, 2*n)
	sum := 0.0
	for k := range n {
		sum += bidSizes[k]
		out[n-1-k] = depthLevel{math.Round((depthMid-depthTick-float64(k)*depthTick)*100) / 100, sum, nan}
	}
	sum = 0
	for k := range n {
		sum += askSizes[k]
		out[n+k] = depthLevel{math.Round((depthMid+depthTick+float64(k)*depthTick)*100) / 100, nan, sum}
	}
	return out
}

// sizeOf is a level's size on its side.
func (l depthLevel) size() float64 {
	if math.IsNaN(l.bid) {
		return l.ask
	}
	return l.bid
}

// depthCard is an order book's depth: the bids' size summed from the best
// bid down on the left, the asks' from the best ask up on the right, as
// steps (a level's size holds until the next one), the spread a gap in the
// middle. The side under the pointer shows its dot and its sum; the bar
// under the plot weighs one side against the other.
func depthCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	ticks := core.NewSignal(cx, 20)
	rows := core.NewMemoFunc(cx, func() []depthLevel { return depthBook(ticks.Get()) }, nil)
	hover := core.NewSignal(cx, -1)
	tip := core.NewSignal(cx, 0)
	lo := func() float64 { return rows.Get()[0].price - depthTick }
	hi := func() float64 { r := rows.Get(); return r[len(r)-1].price + depthTick }
	top := func() float64 { return slices.Max(column(rows.Get(), depthLevel.size)) * 1.18 }
	bids := func() float64 { return rows.Get()[0].bid }
	asks := func() float64 { r := rows.Get(); return r[len(r)-1].ask }
	shown := func() depthLevel {
		r := rows.Get()
		if t := tip.Get(); t < len(r) {
			return r[t]
		}
		return r[0]
	}
	bidSide := func() bool { return !math.IsNaN(shown().bid) }
	// A side's dot shows only while the pointer is over that side.
	dot := func(bid bool, c func() core.Color) func() core.Style {
		return func() core.Style {
			o := float32(0)
			if hover.Get() >= 0 && bidSide() == bid {
				o = 1
			}
			return core.Style{Background: c(), BorderColor: colors.Inset(), Opacity: core.Alpha(o)}
		}
	}
	values := func(f func(depthLevel) float64) func() []float64 { return func() []float64 { return column(rows.Get(), f) } }
	side := func(bid bool, c func() core.Color) []core.Node {
		value := values(func(l depthLevel) float64 { return l.ask })
		if bid {
			value = values(func(l depthLevel) float64 { return l.bid })
		}
		return []core.Node{
			plot.Area{Y1: value, Curve: plot.CurveStepAfter, BackgroundStyleFunc: func() core.Style { return wash(c(), 0.22) }, HoverDotStyle: hidden},
			plot.Line{Y: value, Curve: plot.CurveStepAfter, StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(c()), StrokeWidth: 1.5} },
				HoverDotStyleFunc: dot(bid, c)},
		}
	}
	share := func(v float64) string { return fixed(v/(bids()+asks())*100, 0) + "%" }
	grow := core.Transition{Duration: ms(500), Easing: core.EaseInOut, Properties: core.TransitionLayout}
	children := []core.Node{
		plot.Grid{Y: func() []float64 { return gridTicks(0, top(), 3) }, StyleFunc: gridStyle(cx)},
		// The middle: a dashed hairline in the spread.
		core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Borders: core.Edges{Left: 1}, BorderStyle: core.BorderDashed},
			StyleFunc: func() core.Style {
				return core.Style{Left: core.Pct(float32((depthMid - lo()) / (hi() - lo()) * 100)), BorderColors: core.EdgeColors{Left: ui.Alpha(colors.Text(), 0.2)}}
			}},
	}
	children = append(children, side(true, colors.Up)...)
	children = append(children, side(false, colors.Down)...)
	children = append(children,
		plot.Grid{Y: func() []float64 { return []float64{0} }, StyleFunc: baselineStyle(cx)},
		// The prices at the ends, and the middle a shade firmer.
		plot.Axis{TickValues: func() []float64 { return []float64{lo(), hi()} }, Format: func(v float64, _ int) string { return fixed(v, 2) }, LabelStyleFunc: belowStyle(cx)},
		plot.Axis{TickValues: func() []float64 { return []float64{depthMid} }, Format: func(v float64, _ int) string { return fixed(v, 2) },
			LabelStyleFunc: func() core.Style { return belowStyle(cx)().Merge(core.Style{Color: colors.Muted()}) }},
	)
	sideColor := func() core.Color {
		if bidSide() {
			return colors.Up()
		}
		return colors.Down()
	}
	return Card{TestID: "card-depth", Title: tr.ChartsDepth(), Description: tr.ChartsDepthNote(), Children: []core.Node{
		baseline(10, headline(cx, func() string { return fixed(depthMid, 2) }, nil),
			note(cx, func() string {
				return tr.ChartsMidSpread(fixed(depthTick*2, 2), fixed(depthTick*2/depthMid*100, 2)+"%")
			}, colors.Muted)),
		spaced(chartGap(), plot.Plot{
			TestID: "depth-plot",
			Count:  func() int { return len(rows.Get()) },
			X:      values(func(l depthLevel) float64 { return l.price }),
			XScale: func() plot.Scale { return plot.LinearScale(lo(), hi()) },
			YScale: func() plot.Scale { return plot.LinearScale(0, top()) },
			// Another range draws the book in again.
			AppearGeneration: func() any { return ticks.Get() },
			Motion: plot.Motion{
				Appear:  &core.TimingConfig{Duration: ms(700), Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}},
				Enter:   &core.TimingConfig{Duration: ms(120)},
				Exit:    &core.TimingConfig{Duration: ms(160)},
				Pointer: &core.SpringConfig{Stiffness: 420, Damping: 40},
			},
			LabelFunc: func() string { return tr.ChartsDepthLabel(itoa(ticks.Get()), count(bids()), count(asks())) },
			AccessibilityValue: func() core.AccessibilityValue {
				if hover.Get() < 0 {
					return core.AccessibilityValue{}
				}
				if bidSide() {
					return core.AccessibilityValue{Text: tr.ChartsBidsTo(fixed(shown().price, 2), count(shown().size()))}
				}
				return core.AccessibilityValue{Text: tr.ChartsAsksTo(fixed(shown().price, 2), count(shown().size()))}
			},
			CrossLineStyleFunc: crossLine(cx),
			DotStyle:           hoverDotShape,
			TooltipOffset:      6,
			Tooltip: tipCard(cx,
				core.Text{ContentFunc: func() string {
					if bidSide() {
						return upper(tr.ChartsBids()) + " ≥ " + fixed(shown().price, 2)
					}
					return upper(tr.ChartsAsks()) + " ≤ " + fixed(shown().price, 2)
				}, StyleFunc: func() core.Style { return tipTitle(cx).Merge(core.Style{Color: sideColor()}) }},
				tipValue(cx, func() string { return count(shown().size()) })),
			OnHoverChange: func(i int) {
				hover.Set(i)
				if i >= 0 {
					tip.Set(i)
				}
			},
			Style:    core.Style{Height: core.Pt(depthHeight), Margin: core.Edges{Bottom: chartBelow}},
			Children: children,
		}),
		// The balance: each side's share of what the book holds in view.
		core.View{Style: core.Style{Margin: core.Edges{Top: chartGap() + 4}, Gap: 8}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
				ui.Tabular(ui.Textf(func() string { return tr.ChartsBids() + " " + share(bids()) }, 12, ui.Regular, colors.Up)),
				ui.Tabular(ui.Textf(func() string { return share(asks()) + " " + tr.ChartsAsks() }, 12, ui.Regular, colors.Down)),
			}},
			core.View{Style: core.Style{Direction: core.Row, Gap: 3, Height: core.Pt(4)}, Children: []core.Node{
				core.View{Style: core.Style{CornerRadius: 2, Transition: grow}, StyleFunc: func() core.Style { return core.Style{FlexGrow: float32(bids()), Background: colors.Up()} }},
				core.View{Style: core.Style{CornerRadius: 2, Transition: grow}, StyleFunc: func() core.Style { return core.Style{FlexGrow: float32(asks()), Background: colors.Down()} }},
			}},
		}},
		chartSwitch(tr.ChartsRange(), 260, ticks.Get, ticks.Set,
			ui.Option[int]{Value: 10, Label: tr.ChartsTicks("10")}, ui.Option[int]{Value: 20, Label: tr.ChartsTicks("20")}, ui.Option[int]{Value: 40, Label: tr.ChartsTicks("40")}),
	}}
}

// equityDay is a day of a portfolio: its value and how far it sits under
// its best so far, in percent.
type equityDay struct {
	day             int
	value, drawdown float64
}

// equity is a portfolio's value over 120 days: a rally, a slide, a
// recovery.
var equity = func() []equityDay {
	r := chartRandom(5)
	value := 10000.0
	top := value
	out := make([]equityDay, 120)
	for day := range out {
		drift := 0.0058
		if day < 38 {
			drift = 0.004
		} else if day < 66 {
			drift = -0.0062
		}
		value *= 1 + drift + (r()-0.5)*0.018
		top = max(top, value)
		out[day] = equityDay{day, math.Round(value), (value/top - 1) * 100}
	}
	return out
}()

const (
	equityHeight = 140
	waterHeight  = 72
	// troughLabel is the room the low's label takes under its dot: a gap,
	// then a line of axis text.
	troughLabel = 5 + 13
)

var (
	equityTrough = slices.MinFunc(equity, func(a, b equityDay) int { return cmpFloat(a.drawdown, b.drawdown) })
	equityPeak   = func() equityDay {
		best := equity[0]
		for _, d := range equity[:equityTrough.day+1] {
			if d.value > best.value {
				best = d
			}
		}
		return best
	}()
	// worstRun is the days of the worst run, from its high to its low.
	worstRun  = equity[equityPeak.day : equityTrough.day+1]
	recovered = func() int {
		for _, d := range equity[equityTrough.day+1:] {
			if d.value >= equityPeak.value {
				return d.day
			}
		}
		return -1
	}()
	underwater = func() float64 {
		n := 0
		for _, d := range equity {
			if d.drawdown < -0.05 {
				n++
			}
		}
		return float64(n) / float64(len(equity))
	}()
	// ddFloor is the underwater plot's floor: deep enough that the low's
	// label fits between its dot and the floor.
	ddFloor  = math.Floor(equityTrough.drawdown * waterHeight / (waterHeight - troughLabel))
	valueLo  = slices.Min(column(equity, func(d equityDay) float64 { return d.value })) * 0.97
	valueHi  = slices.Max(column(equity, func(d equityDay) float64 { return d.value })) * 1.03
	equityUp = (equity[len(equity)-1].value/equity[0].value - 1) * 100
)

// across is where day i is across both plots: their rows are evenly
// spread.
func across(i int) core.Length { return core.Pct(float32(float64(i) / float64(len(equity)-1) * 100)) }

// drawdownCard is a portfolio's value over its drawdown: the equity curve,
// the worst run from a high to its low shaded on it, and under it the
// underwater plot, how far each day sits below the best day before it. The
// day under the pointer is marked on both plots and read out above them.
func drawdownCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	hover := core.NewSignal(cx, -1)
	// Which plot the pointer is over: the other one marks the day itself.
	over := core.NewSignal(cx, "")
	shown := func() equityDay {
		if h := hover.Get(); h >= 0 && h < len(equity) {
			return equity[h]
		}
		return equity[len(equity)-1]
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
	// Where the hovered day is on each plot, from its own scale.
	height := func(which string) core.Points {
		if which == "equity" {
			return core.Points((1 - (shown().value-valueLo)/(valueHi-valueLo)) * equityHeight)
		}
		return core.Points(shown().drawdown / ddFloor * waterHeight)
	}
	mark := func(which string, c func() core.Color) core.Node {
		return core.Show{When: func() bool { return hover.Get() >= 0 && over.Get() != which }, Then: func(*core.Scope) core.Node {
			return core.Group{
				core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(1)},
					StyleFunc: func() core.Style { return core.Style{Left: across(max(hover.Get(), 0)), Background: ui.Alpha(colors.Text(), 0.22)} }},
				core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Width: core.Pt(0), Height: core.Pt(0)},
					StyleFunc: func() core.Style { return core.Style{Left: across(max(hover.Get(), 0)), Top: core.Pt(height(which))} },
					Children: []core.Node{core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(-4), Top: core.Pt(-4), Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, BorderWidth: 2},
						StyleFunc: func() core.Style { return core.Style{BorderColor: colors.Inset(), Background: c()} }}}},
			}
		}}
	}
	motion := chartMotion(ms(1000), 0, true)
	days := func() []float64 { return column(equity, func(d equityDay) float64 { return float64(d.day) }) }
	hatch := func() core.Style {
		c := ui.Alpha(colors.Danger(), 0.32)
		return core.Style{Gradients: []core.Gradient{{Angle: -45, Repeating: true, Stops: []core.ColorStop{
			{Color: c, At: core.Pt(0)}, {Color: c, At: core.Pt(1)}, {Color: transparent, At: core.Pt(1)}, {Color: transparent, At: core.Pt(6)}}}}}
	}
	troughText := pct(equityTrough.drawdown, 1)
	return Card{TestID: "card-drawdown", Title: tr.ChartsDrawdown(), Description: tr.ChartsDrawdownNote(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 10}, Children: []core.Node{
			headline(cx, func() string { return "$" + count(shown().value) }, nil),
			note(cx, func() string {
				if shown().drawdown < -0.05 {
					return tr.ChartsFromTheHigh(pct(shown().drawdown, 1))
				}
				return tr.ChartsAtAHigh()
			}, func() core.Color {
				if shown().drawdown < -0.05 {
					return colors.Danger()
				}
				return colors.Muted()
			}),
			core.View{Style: core.Style{FlexGrow: 1}},
			core.Text{ContentFunc: func() string { return upper(tr.ChartsDay(itoa(shown().day + 1))) }, StyleFunc: axisColored(cx, core.Style{}, colors.Muted)},
		}},
		core.View{Style: core.Style{Margin: core.Edges{Top: chartGap()}, Gap: 6}, Children: []core.Node{
			plot.Plot{
				TestID: "equity-plot",
				Y:      func() []float64 { return column(equity, func(d equityDay) float64 { return d.value }) },
				// Days on a linear scale, so the worst run's own rows land on
				// their days.
				X:                days,
				XScale:           func() plot.Scale { return plot.LinearScale(0, float64(len(equity)-1)) },
				YScale:           func() plot.Scale { return plot.LinearScale(valueLo, valueHi) },
				AppearGeneration: func() any { return 1 },
				// No pointer spring: the other plot marks the same day at once,
				// so both move together.
				Motion: motion,
				Label:  tr.ChartsEquityLabel(pct(equityUp, 1), troughText, itoa(equityPeak.day+1), itoa(equityTrough.day+1)),
				AccessibilityValue: func() core.AccessibilityValue {
					if hover.Get() < 0 {
						return core.AccessibilityValue{}
					}
					return core.AccessibilityValue{Text: tr.ChartsDay(itoa(shown().day+1)) + ", $" + count(shown().value) + ", " + pct(shown().drawdown, 1)}
				},
				CrossLineStyleFunc: crossLine(cx),
				DotStyle:           hoverDotShape,
				OnHoverChange:      follow("equity"),
				Style:              core.Style{Height: core.Pt(equityHeight)},
				Children: []core.Node{
					// The worst run, from its high to its low, its value hatched
					// under the curve.
					core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: across(equityPeak.day), Top: core.Pt(0), Bottom: core.Pt(0),
						Width: core.Pct(float32(float64(equityTrough.day-equityPeak.day) / float64(len(equity)-1) * 100))},
						StyleFunc: func() core.Style { return core.Style{Background: ui.Alpha(colors.Danger(), 0.04)} },
						Children: []core.Node{core.Text{Content: upper(tr.ChartsMax()) + " " + troughText, Style: core.Style{Position: core.Absolute, Left: core.Pt(6), Top: core.Pt(6)},
							StyleFunc: axisColored(cx, core.Style{}, colors.Danger)}}},
					plot.Grid{Y: func() []float64 { return gridTicks(valueLo, valueHi, 3) }, StyleFunc: gridStyle(cx)},
					plot.Area{BackgroundStyleFunc: func() core.Style { return wash(colors.Accent(), 0.16) }, HoverDotStyle: hidden},
					// Hairline stripes 6 points apart, falling to the right.
					plot.Area{X: func() []float64 { return column(worstRun, func(d equityDay) float64 { return float64(d.day) }) },
						Y1:                  func() []float64 { return column(worstRun, func(d equityDay) float64 { return d.value }) },
						BackgroundStyleFunc: hatch, HoverDotStyle: hidden},
					plot.Line{StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Accent()), StrokeWidth: 1.75} },
						HoverDotStyleFunc: hoverDot(cx, colors.Accent)},
					mark("equity", colors.Accent),
				},
			},
			// Underwater: hanging from zero, deeper red the further down.
			plot.Plot{
				TestID:             "underwater-plot",
				Y:                  func() []float64 { return column(equity, func(d equityDay) float64 { return d.drawdown }) },
				YScale:             func() plot.Scale { return plot.LinearScale(ddFloor, 0) },
				AppearGeneration:   func() any { return 1 },
				Motion:             motion,
				Label:              tr.ChartsUnderwaterLabel(troughText, itoa(equityTrough.day+1)),
				AccessibilityValue: func() core.AccessibilityValue {
					if hover.Get() < 0 {
						return core.AccessibilityValue{}
					}
					return core.AccessibilityValue{Text: tr.ChartsDay(itoa(shown().day+1)) + ", " + tr.ChartsFromTheHigh(pct(shown().drawdown, 1))}
				},
				CrossLineStyleFunc: crossLine(cx),
				DotStyle:           core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, BorderWidth: 2},
				OnHoverChange:      follow("water"),
				Style:              core.Style{Height: core.Pt(waterHeight), Margin: core.Edges{Bottom: chartBelow}},
				Children: []core.Node{
					plot.Area{Y0: func() float64 { return 0 }, BackgroundStyleFunc: func() core.Style {
						return core.Style{Gradients: []core.Gradient{{Angle: 180, Stops: []core.ColorStop{{Color: ui.Alpha(colors.Danger(), 0.04)}, {Color: ui.Alpha(colors.Danger(), 0.26)}}}}}
					}, HoverDotStyle: hidden},
					plot.Line{StyleFunc: func() core.Style { return core.Style{Stroke: core.PaintColor(colors.Danger()), StrokeWidth: 1.25} },
						HoverDotStyleFunc: hoverDot(cx, colors.Danger)},
					// The floor's value over its edge, and the low marked where
					// it is.
					plot.Axis{Placement: plot.AxisLeft, LabelPlacement: plot.LabelsInside, TickValues: func() []float64 { return []float64{ddFloor} },
						Format: func(v float64, _ int) string { return minus(v, 0) + "%" }, LabelStyleFunc: axisStyle(cx, core.Style{Padding: core.Edges{Bottom: 2}})},
					core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: across(equityTrough.day), Top: core.Pt(core.Points(equityTrough.drawdown / ddFloor * waterHeight)),
						Width: core.Pt(0), Height: core.Pt(0)}, Children: []core.Node{
						core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(-3.5), Top: core.Pt(-3.5), Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 3.5, BorderWidth: 1.5},
							StyleFunc: func() core.Style { return core.Style{BorderColor: colors.Inset(), Background: colors.Danger()} }},
						// Under the dot, where the curve never is (the low is its
						// lowest point); a width of its own, centered on the dot.
						core.Text{Content: troughText, Style: core.Style{Position: core.Absolute, Left: core.Pt(-32), Top: core.Pt(5), Width: core.Pt(64), TextAlign: core.TextAlignCenter},
							StyleFunc: axisColored(cx, core.Style{LineHeight: 13}, colors.Danger)},
					}},
					mark("water", colors.Danger),
					// The days under both plots: the first, then every 30th.
					plot.Axis{TickValues: func() []float64 { return []float64{0, 29, 59, 89, 119} }, Format: func(v float64, _ int) string {
						if v == 0 {
							return tr.ChartsDay("1")
						}
						return itoa(int(v) + 1)
					}, LabelStyleFunc: belowStyle(cx)},
				},
			},
		}},
		chartFigures(cx, chartGap()+4,
			chartFigure(cx, tr.ChartsReturn(), constant(pct(equityUp, 1)), func() Tone { return upDown(equityUp) }),
			chartFigure(cx, tr.ChartsWorst(), constant(troughText), fixedTone(ToneDanger)),
			chartFigure(cx, tr.ChartsRecovery(), func() string {
				if recovered < 0 {
					return "—"
				}
				return tr.ChartsDays(itoa(recovered - equityTrough.day))
			}, nil),
			chartFigure(cx, tr.ChartsUnderwater(), constant(fixed(underwater*100, 0)+"%"), nil)),
	}}
}

// upDown is the tone of a change: up for none or a rise, down for a fall.
func upDown(v float64) Tone {
	if v >= 0 {
		return ToneUp
	}
	return ToneDown
}
