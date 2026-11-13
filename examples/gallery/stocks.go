package main

import (
	"fmt"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"math"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// sectorBar is a sector's mean change as a bar diverging from the middle.
func sectorBar(cx *core.Scope, lbl string, change func() float64, scale func() float64) core.Node {
	colors := ui.ColorsOf(cx)
	share := func() float64 { return math.Min(1, math.Abs(change())/scale()) }
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Height: core.Pt(22)}, Children: []core.Node{
		core.Text{Content: lbl, MaxLines: 1, Style: core.Style{Width: core.Pt(84), FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Direction: core.Row, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.View{Style: fill, Children: []core.Node{springBar(func() float64 {
				if change() < 0 {
					return share()
				}
				return 0
			}, colors.Down, true, 8, 2)}},
			core.View{Style: core.Style{Width: core.Pt(1), Height: core.Pt(14)}, StyleFunc: ui.Filled(colors.Border)},
			core.View{Style: fill, Children: []core.Node{springBar(func() float64 {
				if change() > 0 {
					return share()
				}
				return 0
			}, colors.Up, false, 8, 2)}},
		}},
		core.Text{ContentFunc: func() string { return signed(change(), 2) + "%" }, StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(56), TextAlign: core.TextAlignEnd, FontSize: 12, FontVariant: core.FontTabularNums, Color: toneOf(cx, change())}
		}},
	}}
}

// breadthPanel is advancers against decliners across the composite and
// the sectors' moves: what the strip opens to.
func breadthPanel(cx *core.Scope, m *market) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	b := m.breadth.Get
	means := core.NewMemoFunc(cx, func() []float64 {
		m.pulse.Get()
		out := make([]float64, len(sectors))
		for i, sc := range sectors {
			sum, n := 0.0, 0
			for _, s := range m.stocks {
				if s.sector == sc.id {
					sum += percentOf(s.quote.GetUntracked())
					n++
				}
			}
			out[i] = sum / float64(max(n, 1))
		}
		return out
	}, func(a, b []float64) bool { return false })
	scale := func() float64 {
		v := 0.5
		for _, x := range means.Get() {
			v = math.Max(v, math.Abs(x))
		}
		return v * 1.1
	}
	var bars []core.Node
	for i, sc := range sectors {
		bars = append(bars, sectorBar(cx, sc.label(locales.For(cx)), func() float64 { return means.Get()[i] }, scale))
	}
	segment := func(n func() int, c func() core.Color) core.Node {
		return core.View{StyleFunc: func() core.Style { return core.Style{FlexGrow: float32(n()), Background: c()} }}
	}
	return core.View{Style: core.Style{Padding: core.Edges{Right: 16, Bottom: 14, Left: 16}, Gap: 10}, Children: []core.Node{
		core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Height: core.Pt(6), CornerRadius: 3, Overflow: core.Hidden, Gap: 2}, Children: []core.Node{
				segment(func() int { return b().up }, colors.Up), segment(func() int { return b().flat }, colors.Faint), segment(func() int { return b().down }, colors.Down),
			}},
			core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
				core.Text{ContentFunc: func() string { return fmt.Sprintf("%d up", b().up) }, Style: core.Style{FlexGrow: 1, FontSize: 12, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Up)},
				ui.Tabular(ui.Textf(func() string { return tr.StocksFlatIssues(b().flat, issues) }, 12, ui.Regular, colors.Faint)),
				core.Text{ContentFunc: func() string { return tr.StocksDownCount(b().down) }, Style: core.Style{FlexGrow: 1, TextAlign: core.TextAlignEnd, FontSize: 12, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Down)},
			}},
		}},
		core.View{Style: core.Style{Gap: 2}, Children: bars},
	}}
}

// marketStrip is the market in one line, on the 16 pt gutters: the
// composite and its change, the clock and the market's control. A press opens breadth
// and sectors with a layout animation; the chevron turns on the platform.
func marketStrip(cx *core.Scope, m *market) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	opened := core.NewSignal(cx, false)
	turn := core.NewAnimatedValue(cx, 0)
	change := func() float64 { return (m.composite.Get() - indexBase) / indexBase * 100 }
	toggle := func() {
		cx.AnimateLayout(core.LayoutSpringPreset)
		toggled(opened)
		to := 0.0
		if opened.GetUntracked() {
			to = 1
		}
		ui.Spring(cx, turn, to, ui.SettleSpring)
	}
	return core.View{Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(0, 16), Height: core.Pt(56)}, Children: []core.Node{
			core.View{OnPress: func(core.PressEvent) { toggle() }, Role: core.RoleButton, Label: tr.StocksBreadth(), Expanded: opened.Get,
				Style: core.Style{FlexGrow: 1, FlexShrink: 1, Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Height: core.Pt(56)}, Children: []core.Node{
					core.Text{Content: tr.StocksComposite(), MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
					core.Text{ContentFunc: func() string { return grouped(m.composite.Get(), 2) }, MaxLines: 1, Style: core.Style{FontSize: 13, FontWeight: ui.Semibold, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Text)},
					core.Text{ContentFunc: func() string { return signed(change(), 2) + "%" }, StyleFunc: func() core.Style {
						return core.Style{FontSize: 13, FontWeight: ui.Medium, FontVariant: core.FontTabularNums, Color: toneOf(cx, change())}
					}},
					core.View{Style: core.Style{Animated: &core.AnimatedStyle{Rotate: ui.Lerp(turn, 90, -90)}}, Children: []core.Node{ui.Icon(lucide.ChevronRight, 12, colors.Faint)}},
				}},
			ui.Tabular(ui.Textf(func() string { return marketTime(float64(m.clock.Get()), true) }, 12, ui.Regular, colors.Faint)),
			marketControl(cx, m),
		}},
		core.Show{When: opened.Get, Then: func(cx *core.Scope) core.Node { return breadthPanel(cx, m) }},
	}}
}

const (
	// sparkPoints is the points a sparkline draws: few enough for a calm,
	// readable curve; sparkSmooth the minutes each averages over.
	sparkPoints = 24
	sparkSmooth = 4
	// ring is the newest point's ring at its widest: the canvas reaches
	// this far past the plot on every side, so it is never clipped.
	ring   = 10
	sparkW = 104
	sparkH = 38
	// pulseGap is the shortest time between two pulses of a row, so each
	// one plays out.
	pulseGap = 600 * time.Millisecond
)

// sparkline is today in 104 × 38 pt: a smooth line through a moving
// average of the minutes' closes ending at the live price, a soft fill
// under it that fades in from the left, the previous close dotted and a
// dot at the newest point that rings on each change. The y range eases
// instead of jumping.
func sparkline(cx *core.Scope, s *stock) core.Node {
	colors := ui.ColorsOf(cx)
	scale := &calmRange{rate: 0.1}
	rv := core.NewAnimatedValue(cx, 1)
	ringRadius, haloAlpha, strokeAlpha := ui.Lerp(rv, 3, ring), ui.Lerp(rv, 0.35, 0), ui.Lerp(rv, 0.9, 0)
	seen := core.Untracked(cx, s.quote.Get).changedAt
	var fired time.Time
	cx.Effect(func() {
		c := s.quote.Get().changedAt
		if c == seen {
			return
		}
		seen = c
		// A pulse at most every 600 ms, so each one plays out.
		if time.Since(fired) < pulseGap {
			return
		}
		fired = time.Now()
		rv.Set(0)
		ui.Timing(cx, rv, 1, core.TimingConfig{Duration: ms(700), Easing: ui.EaseOutCubic})
	})
	ys := make([]float64, 0, sparkPoints)
	muted := colors.Muted
	draw := func(d *core.Drawing, size core.Size) {
		minutes := s.minutes.Get()
		q := s.quote.Get()
		w, h := float32(size.Width), float32(size.Height)
		// Each point: the mean close of the minutes before it; the newest
		// point is the last trade.
		n := min(sparkPoints, len(minutes))
		if n == 0 {
			return
		}
		ys = ys[:0]
		for k := range n {
			end := int(math.Round(float64(k) / float64(max(1, n-1)) * float64(len(minutes)-1)))
			start := max(0, end-sparkSmooth+1)
			sum := 0.0
			for i := start; i <= end; i++ {
				sum += minutes[i].c
			}
			ys = append(ys, sum/float64(end-start+1))
		}
		ys[n-1] = q.last
		lo, hi := q.prevClose, q.prevClose
		for _, v := range ys {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		rlo, rhi := scale.fit(lo, hi)
		left, right, bottom := float32(ring), w-ring, h-ring
		Y := func(v float64) float32 {
			span := rhi - rlo
			if span == 0 {
				span = 1
			}
			return ring + 2 + float32((rhi-v)/span)*(bottom-ring-4)
		}
		X := func(k int) float32 { return left + float32(k)/float32(max(1, n-1))*(right-left) }
		tone := toneOf(cx, q.last-q.prevClose)
		// A smooth curve: quadratic segments through the midpoints.
		curve := func() {
			d.MoveTo(X(0), Y(ys[0]))
			for k := 1; k < n-1; k++ {
				d.QuadTo(X(k), Y(ys[k]), (X(k)+X(k+1))/2, (Y(ys[k])+Y(ys[k+1]))/2)
			}
			d.LineTo(X(n-1), Y(ys[n-1]))
		}
		// The fill: down from the line's top, strictly under the curve...
		top := bottom
		for _, v := range ys {
			top = min(top, Y(v))
		}
		d.SetFillGradient(core.CanvasLinear(0, top, 0, bottom, core.GradientStop{Offset: 0, Color: ui.Alpha(tone, 0.22)}, core.GradientStop{Offset: 1, Color: ui.Alpha(tone, 0)}))
		d.BeginPath()
		curve()
		d.LineTo(X(n-1), bottom).LineTo(X(0), bottom).ClosePath().Fill()
		// ...and faded in over the first fifth, erased, not overdrawn.
		fadeTo := left + (right-left)*0.2
		d.SetComposite(core.CompositeDestinationOut).
			SetFillGradient(core.CanvasLinear(left, 0, fadeTo, 0, core.GradientStop{Offset: 0, Color: core.RGBA(0, 1)}, core.GradientStop{Offset: 1, Color: core.RGBA(0, 0)})).
			FillRect(0, 0, fadeTo, h).SetComposite(core.CompositeSourceOver)
		// The previous close, dotted.
		d.SetStrokeColor(ui.Alpha(muted(), 0.4)).SetLineWidth(1).SetLineDash(1, 3).BeginPath().MoveTo(left, Y(q.prevClose)).LineTo(right, Y(q.prevClose)).Stroke().SetLineDash()
		d.BeginPath()
		curve()
		d.SetStrokeColor(tone).SetLineWidth(1.75).SetLineJoin(core.JoinRound).SetLineCap(core.CapRound).Stroke()
		// The newest point and its pulse: a soft halo and a ring that grow
		// and fade on the platform.
		x, y := X(n-1), Y(ys[n-1])
		d.SetFillColor(tone).SetStrokeColor(tone)
		d.SetAlpha(0.35).Bind(core.ArgA, haloAlpha).BeginPath().Arc(x, y, 3, 0, tau, false).Bind(core.ArgR, ringRadius).Fill()
		d.SetAlpha(0.9).Bind(core.ArgA, strokeAlpha).SetLineWidth(1.5).BeginPath().Arc(x, y, 3, 0, tau, false).Bind(core.ArgR, ringRadius).Stroke()
		d.SetAlpha(1).BeginPath().Arc(x, y, 3, 0, tau, false).Fill()
	}
	return core.Canvas{Style: core.Style{Width: core.Pt(sparkW + ring*2), Height: core.Pt(sparkH + ring*2), Margin: core.All(-ring)}, Draw: draw}
}

// stockRow is a watchlist row: code and name, today's sparkline, the
// price and its change. In the desktop layout the rows are a sidebar's,
// the one shown beside the list on a rounded fill; on phones hairlines
// divide them.
func stockRow(cx *core.Scope, s *stock, first func() bool, onPress func(), wide bool, active func() bool) core.Node {
	colors := ui.ColorsOf(cx)
	q := s.quote.Get
	change := func() float64 { return percentOf(q()) }
	st := core.Style{Height: core.Pt(64), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(0, 16)}
	var states func() core.Style
	var divider core.Node
	if wide {
		st.Height, st.Margin, st.Padding = core.Pt(60), core.Symmetric(1, 8), core.Symmetric(0, 8)
		st.CornerRadius, st.BorderCurve = 10, core.BorderCurveContinuous
		states = func() core.Style { return core.Style{Selected: &core.Style{Background: ui.Alpha(colors.Text(), 0.1)}} }
	} else {
		divider = core.Show{When: func() bool { return !first() }, Then: func(*core.Scope) core.Node {
			return core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(16), Right: core.Pt(0), Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)}
		}}
	}
	return core.View{OnPress: func(core.PressEvent) { onPress() }, Role: core.RoleButton, Label: s.name, Selected: active, TestID: "stock-" + s.code,
		Style: st, StyleFunc: states, Children: []core.Node{
			divider,
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 3}, Children: []core.Node{
				core.Text{Content: s.code, MaxLines: 1, Style: core.Style{FontSize: 16, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{Content: s.name, MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			}},
			sparkline(cx, s),
			core.View{Style: core.Style{Width: core.Pt(84), AlignItems: core.AlignEnd, Gap: 3}, Children: []core.Node{
				core.Text{ContentFunc: func() string { return priceText(q().last, s) }, MaxLines: 1, Style: core.Style{FontSize: 16, FontWeight: ui.Semibold, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{ContentFunc: func() string { return signed(change(), 2) + "%" }, MaxLines: 1, StyleFunc: func() core.Style {
					return core.Style{FontSize: 13, FontWeight: ui.Medium, FontVariant: core.FontTabularNums, Color: toneOf(cx, change())}
				}},
			}},
		}}
}

// watchlistWidth is the watchlist's width beside the stock in the
// desktop layout.
const watchlistWidth = 380

// stocksDemo is the watchlist: quotes that change every tick under a
// one-line market strip; sorted filters sort again once a second, the
// rows gliding to their places. A stock opens its details, pushed on phones, beside
// the list on desktops; leaving the demo stops the market.
func stocksDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	wide := shell.desktop()
	m := newMarket(cx)
	m.start()
	filter := core.NewSignal(cx, "all")
	ordered := func(f string) []*stock {
		change := func(s *stock) float64 { return percentOf(s.quote.GetUntracked()) }
		var out []*stock
		switch f {
		case "all":
			return m.stocks
		case "gainers":
			for _, s := range m.stocks {
				if change(s) > 0 {
					out = append(out, s)
				}
			}
			slices.SortFunc(out, func(a, b *stock) int { return cmpFloat(change(b), change(a)) })
		case "losers":
			for _, s := range m.stocks {
				if change(s) < 0 {
					out = append(out, s)
				}
			}
			slices.SortFunc(out, func(a, b *stock) int { return cmpFloat(change(a), change(b)) })
		default:
			out = slices.Clone(m.stocks)
			slices.SortFunc(out, func(a, b *stock) int {
				return cmpFloat(b.quote.GetUntracked().turnover, a.quote.GetUntracked().turnover)
			})
			out = out[:10]
		}
		return out
	}
	rows := core.NewSignalFunc(cx, m.stocks, slices.Equal)
	cx.Effect(func() {
		f := filter.Get()
		m.pulse.Get()
		cx.Untrack(func() {
			next := ordered(f)
			if slices.Equal(next, rows.GetUntracked()) {
				return
			}
			cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)
			rows.Set(next)
		})
	})
	counts := core.NewMemo(cx, func() [2]int {
		m.pulse.Get()
		var c [2]int
		for _, s := range m.stocks {
			switch ch := percentOf(s.quote.GetUntracked()); {
			case ch > 0:
				c[0]++
			case ch < 0:
				c[1]++
			}
		}
		return c
	})
	// Stable options, so the segments stay mounted; the counts follow the
	// once-a-second counts.
	options := []ui.Option[string]{
		{Value: "all", Label: tr.StocksAll(), Content: countedTab(cx, tr.StocksAll(), func() int { return len(m.stocks) })},
		{Value: "gainers", Label: tr.StocksGainers(), Content: countedTab(cx, tr.StocksGainers(), func() int { return counts.Get()[0] })},
		{Value: "losers", Label: tr.StocksLosers(), Content: countedTab(cx, tr.StocksLosers(), func() int { return counts.Get()[1] })},
		{Value: "active", Label: tr.StocksActive()},
	}
	// Desktop: the stock beside the list.
	shown := core.NewSignal(cx, m.stocks[0])
	watchlist := core.View{StyleFunc: func() core.Style { return fillPage.Merge(core.Style{Background: colors.Background()}) }, Children: []core.Node{
		marketStrip(cx, m),
		core.View{Style: core.Style{Padding: core.Edges{Right: 16, Bottom: 6, Left: 16}}, Children: []core.Node{ui.Tabs[string]{Equal: true, Options: options, Value: filter.Get, OnChange: filter.Set}}},
		core.Scroll{Style: core.Style{FlexGrow: 1}, TestID: "watchlist", Children: []core.Node{
			core.View{Children: []core.Node{
				core.Each(rows.Get, func(s *stock) string { return s.code }, func(cx *core.Scope, s func() *stock, i func() int) core.Node {
					st := core.Untracked(cx, s)
					return stockRow(cx, st, func() bool { return i() == 0 }, func() {
						if wide {
							shown.Set(st)
						} else {
							shell.push(stockScreen(m, st))
						}
					}, wide, func() bool { return wide && shown.Get() == st })
				}),
				core.Show{When: func() bool { return len(rows.Get()) == 0 }, Then: func(*core.Scope) core.Node {
					return core.Text{Content: tr.StocksNothingHereRightNow(), Style: core.Style{Padding: core.Symmetric(32, 0), TextAlign: core.TextAlignCenter, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)}
				}},
			}},
			core.Text{ContentFunc: func() string {
				st := m.stats.Get()
				return tr.StocksSimulatedFooter(rateLabel(m.rate.Get()), fmt.Sprintf("%.2f", st[0]), fmt.Sprintf("%.0f", st[1]))
			}, Style: core.Style{Padding: core.Edges{Top: 16, Right: 16, Bottom: 40, Left: 16}, FontSize: 12, LineHeight: 17, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Faint)},
		}},
	}}
	if !wide {
		return watchlist
	}
	// The book and the tape go beside the chart when the stock has the
	// room.
	split := core.NewMemo(cx, func() bool { return shell.paneWidth()-watchlistWidth >= 860 })
	return core.View{StyleFunc: func() core.Style {
		return fillPage.Merge(core.Style{Direction: core.Row, Background: colors.Background()})
	}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(watchlistWidth), Borders: core.Edges{Right: 1}, BorderColors: core.EdgeColors{Right: colors.Line()}}
		}, Children: []core.Node{watchlist}},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: []core.Node{core.Dynamic(func(cx *core.Scope) core.Node {
			st := shown.Get()
			return core.Func(func(cx *core.Scope) core.Node { return stockView(cx, m, st, true, split.Get) })
		})}},
	}}
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
