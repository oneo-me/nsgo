package main

import (
	"fmt"
	"math"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

const (
	bookRow = 26
	// depthSpan is the ticks either side of the mid price the depth
	// chart spans.
	depthSpan = bookLevels + 2
	tapeShown = 8
	tapeRow   = 28
	// tapeHeld is the trades the canvas holds: the rows shown and four
	// more that slide out.
	tapeHeld = tapeShown + 4
)

// cumulative is sizes summed outward from the best price.
func cumulative(levels []level) []float64 {
	out := make([]float64, len(levels))
	sum := 0.0
	for i, l := range levels {
		sum += l.size
		out[i] = sum
	}
	return out
}

// depthBlock is cumulative depth: bids stepping down to the left, asks to
// the right, the spread hatched and the mid price dashed. It is calm on
// purpose: it redraws four times a second, the price axis is a fixed span
// around the mid, and the size axis widens at once but narrows over about
// a second.
func depthBlock(cx *core.Scope, m *market, s *stock) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	frame := core.NewSignalFunc(cx, 0, func(a, b int) bool { return false })
	cx.Effect(func() {
		if m.ticks.Get()%5 == 0 {
			frame.Set(0)
		}
	})
	book := func() orderBook {
		frame.Get()
		return core.Untracked(cx, m.book.Get)
	}
	totals := func() (float64, float64, float64) {
		b := book()
		var bids, asks float64
		for _, l := range b.bids {
			bids += l.size
		}
		for _, l := range b.asks {
			asks += l.size
		}
		imb := 0.0
		if bids+asks > 0 {
			imb = (bids - asks) / (bids + asks)
		}
		return bids, asks, imb
	}
	top := &calmRange{rate: 0.25}
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		b := book()
		if len(b.bids) == 0 {
			return
		}
		bids, asks := cumulative(b.bids), cumulative(b.asks)
		mid := (b.bids[0].price + b.asks[0].price) / 2
		lo, hi := mid-depthSpan*s.tick, mid+depthSpan*s.tick
		_, ymax := top.fit(0, math.Max(bids[len(bids)-1], asks[len(asks)-1])*1.1)
		w, h0 := float32(size.Width), float32(size.Height)
		const padTop, padBottom = 4, 18
		h := h0 - padTop - padBottom
		X := func(p float64) float32 { return float32((p-lo)/(hi-lo)) * w }
		Y := func(v float64) float32 { return padTop + h - float32(v/ymax)*h }
		d.SetStrokeColor(gridColor(cx)).SetLineWidth(1)
		for k := 1; k <= 3; k++ {
			y := padTop + h*float32(k)/4
			d.BeginPath().MoveTo(0, y).LineTo(w, y).Stroke()
		}
		bx, ax := X(b.bids[0].price), X(b.asks[0].price)
		stockHatch(d, bx, padTop, ax-bx, h, c.Border, 4)
		// A side: a step line out from the best price, filled below.
		side := func(levels []level, sums []float64, col core.Color, left bool) {
			d.BeginPath().MoveTo(X(levels[0].price), padTop+h)
			y := Y(sums[0])
			d.LineTo(X(levels[0].price), y)
			for i := 1; i < len(levels); i++ {
				d.LineTo(X(levels[i].price), y)
				y = Y(sums[i])
				d.LineTo(X(levels[i].price), y)
			}
			edge := w
			if left {
				edge = 0
			}
			d.LineTo(edge, y).LineTo(edge, padTop+h).ClosePath().SetFillColor(ui.Alpha(col, 0.16)).Fill().SetStrokeColor(col).SetLineWidth(1.5).Stroke()
		}
		side(b.bids, bids, c.Up, true)
		side(b.asks, asks, c.Down, false)
		d.SetStrokeColor(c.Muted).SetLineDash(2, 3).BeginPath().MoveTo(X(mid), padTop).LineTo(X(mid), padTop+h).Stroke().SetLineDash()
		f := sans(11, ui.Regular)
		d.SetFillColor(c.Faint).SetTextBaseline(core.BaselineTop).SetTextAlign(core.TextAlignStart).FillText(priceText(lo, s), 0, padTop+h+5, f)
		d.SetTextAlign(core.TextAlignEnd).FillText(priceText(hi, s), w, padTop+h+5, f)
		d.SetTextAlign(core.TextAlignCenter).SetFillColor(c.Text).FillText(tr.StocksMid()+priceText(mid, s), w/2, padTop+h+5, f)
	}
	return block(cx, tr.StocksDepth(), label(tr.StocksTicksCumulative(depthSpan)), nil, false,
		core.Canvas{Style: core.Style{Height: core.Pt(120)}, Draw: draw, Label: tr.StocksDepthChart()},
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, ColumnGap: 16, RowGap: 6}, Children: []core.Node{
			Legend{Color: colors.Up, Label: tr.StocksBids(), Value: func() string { b, _, _ := totals(); return compactNumber(b) }},
			Legend{Color: colors.Down, Label: tr.StocksAsks(), Value: func() string { _, a, _ := totals(); return compactNumber(a) }},
			Legend{Color: colors.Border, Label: tr.StocksImbalance(), Value: func() string { _, _, i := totals(); return signed(i*100, 0) + "%" }},
		}},
	)
}

// levelRow is one level: its label, price, size and running total, over
// a bar of the running total from the right edge (a depth ladder).
func levelRow(cx *core.Scope, s *stock, lvl func() (level, bool), total func() float64, bid bool, n func() int, most func() float64) core.Node {
	colors := ui.ColorsOf(cx)
	tint := func() core.Color {
		if bid {
			return ui.Alpha(colors.Up(), 0.1)
		}
		return ui.Alpha(colors.Down(), 0.1)
	}
	return core.View{Style: core.Style{Height: core.Pt(bookRow), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			share := math.Min(1, total()/most())
			return core.Style{Position: core.Absolute, Right: core.Pt(0), Top: core.Pt(1), Bottom: core.Pt(1), Width: core.Pct(float32(share * 100)), Background: tint()}
		}},
		core.Text{ContentFunc: func() string {
			if bid {
				return fmt.Sprintf("B%d", n())
			}
			return fmt.Sprintf("A%d", n())
		}, Style: core.Style{Width: core.Pt(28), FontSize: 12, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Faint)},
		core.Text{ContentFunc: func() string {
			if l, ok := lvl(); ok {
				return priceText(l.price, s)
			}
			return "–"
		}, StyleFunc: func() core.Style {
			l, _ := lvl()
			return core.Style{FlexGrow: 1, FontSize: 13, FontVariant: core.FontTabularNums, Color: toneOf(cx, l.price-s.quote.Get().prevClose)}
		}},
		core.Text{ContentFunc: func() string {
			if l, ok := lvl(); ok {
				return compactNumber(l.size)
			}
			return ""
		}, Style: core.Style{Width: core.Pt(64), TextAlign: core.TextAlignEnd, FontSize: 13, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Text)},
		core.Text{ContentFunc: func() string { return compactNumber(total()) }, Style: core.Style{Width: core.Pt(64), Padding: core.Edges{Right: 6}, TextAlign: core.TextAlignEnd, FontSize: 12, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Faint)},
	}}
}

// orderBookBlock is asks above bids, five levels or ten (the change
// animates the layout), the bid and ask balance springing on top.
func orderBookBlock(cx *core.Scope, m *market, s *stock) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	depth := core.NewSignal(cx, 5)
	type sums struct{ bids, asks []float64 }
	sm := core.NewMemoFunc(cx, func() sums { b := m.book.Get(); return sums{cumulative(b.bids), cumulative(b.asks)} }, func(a, b sums) bool { return false })
	at := func(xs []float64, i int) float64 {
		if i >= 0 && i < len(xs) {
			return xs[i]
		}
		return 0
	}
	// The bars' scale: the deeper side's total, eased like the depth
	// chart's.
	scale := &calmRange{rate: 0.05}
	most := core.NewMemo(cx, func() float64 {
		x, n := sm.Get(), depth.Get()-1
		_, hi := scale.fit(0, math.Max(1, math.Max(at(x.bids, n), at(x.asks, n))))
		return hi
	})
	share := func() float64 {
		x, n := sm.Get(), depth.Get()-1
		b, a := at(x.bids, n), at(x.asks, n)
		if b+a == 0 {
			return 0.5
		}
		return b / (b + a)
	}
	spread := func() float64 {
		b := m.book.Get()
		if len(b.asks) == 0 || len(b.bids) == 0 {
			return 0
		}
		return b.asks[0].price - b.bids[0].price
	}
	toggle := func() {
		cx.AnimateLayout(core.LayoutSpringPreset)
		if depth.GetUntracked() == 5 {
			depth.Set(bookLevels)
		} else {
			depth.Set(5)
		}
	}
	lvl := func(levels func() []level, i func() int) func() (level, bool) {
		return func() (level, bool) {
			ls := levels()
			k := i()
			if k < 0 || k >= len(ls) {
				return level{}, false
			}
			return ls[k], true
		}
	}
	indices := func() []int {
		out := make([]int, depth.Get())
		for i := range out {
			out[i] = i
		}
		return out
	}
	return block(cx, tr.StocksOrderBook(), nil, ui.Press{OnPress: toggle, HitSlop: 8, Label: tr.StocksLevels(), Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(30), Padding: core.Symmetric(0, 12), CornerRadius: ui.RadiusControl, JustifyContent: core.JustifyCenter}, StyleFunc: ui.Filled(colors.Raised),
			Children: []core.Node{ui.Tabular(ui.Textf(func() string {
				if depth.Get() == 5 {
					return "5 → 10"
				}
				return "10 → 5"
			}, 12, ui.Semibold, colors.Text))}},
	}}, false,
		core.View{Style: core.Style{Gap: 5}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3, Overflow: core.Hidden}, StyleFunc: ui.Filled(colors.Down), Children: []core.Node{springBar(share, colors.Up, false, 6, 0)}},
			core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
				core.Text{ContentFunc: func() string { return tr.StocksBidShare(share()) }, Style: core.Style{FlexGrow: 1, FontSize: 12, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Up)},
				ui.Tabular(ui.Textf(func() string { return tr.StocksSpread() + priceText(spread(), s) }, 12, ui.Regular, colors.Faint)),
				core.Text{ContentFunc: func() string { return tr.StocksAskShare(1 - share()) }, Style: core.Style{FlexGrow: 1, TextAlign: core.TextAlignEnd, FontSize: 12, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Down)},
			}},
		}},
		core.View{Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Gap: 10, Padding: core.Edges{Bottom: 4}}, Children: []core.Node{
				head(cx, tr.StocksLvl(), 28, 0, core.TextAlignStart), head(cx, tr.StocksPrice(), 0, 1, core.TextAlignStart), head(cx, tr.StocksSize(), 64, 0, core.TextAlignEnd), head(cx, tr.StocksTotal(), 64, 0, core.TextAlignEnd),
			}},
			core.Each(indices, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
				k := func() int { return depth.Get() - 1 - i() }
				return levelRow(cx, s, lvl(func() []level { return m.book.Get().asks }, k), func() float64 { return at(sm.Get().asks, k()) }, false,
					func() int { return depth.Get() - i() }, most.Get)
			}),
			core.View{Style: core.Style{Height: core.Pt(1), Margin: core.Symmetric(3, 0)}, StyleFunc: ui.Filled(colors.Border)},
			core.Each(indices, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
				return levelRow(cx, s, lvl(func() []level { return m.book.Get().bids }, i), func() float64 { return at(sm.Get().bids, i()) }, true,
					func() int { return i() + 1 }, most.Get)
			}),
		}},
	)
}

// tapeBlock is the newest trades on top, blocks highlighted: one canvas
// redrawn four times a second, the new rows sliding in by an offset the
// platform animates.
func tapeBlock(cx *core.Scope, m *market, s *stock) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	shown := core.NewSignalFunc(cx, []trade(nil), func(a, b []trade) bool { return false })
	offset := core.NewAnimatedValue(cx, 0)
	top := -1
	// Four times a second: what came in since, and the slide that shows it.
	cx.Effect(func() {
		if m.ticks.Get()%5 != 0 {
			return
		}
		cx.Untrack(func() {
			ts := m.trades.GetUntracked()
			fresh := 0
			if top >= 0 {
				fresh = -1
				for i, t := range ts {
					if t.id <= top {
						fresh = i
						break
					}
				}
			}
			if len(ts) > 0 {
				top = ts[0].id
			}
			shown.Set(ts[:min(len(ts), tapeHeld)])
			k := tapeHeld
			if fresh >= 0 {
				k = min(4, fresh)
			}
			if k > 0 {
				offset.Set(float64(-k * tapeRow))
				ui.Timing(cx, offset, 0, core.TimingConfig{Duration: ms(220), Easing: ui.EaseOutCubic})
			}
		})
	})
	stats := func() (float64, float64, int) {
		var buys, sells float64
		blocks := 0
		for _, t := range m.trades.Get() {
			if t.side > 0 {
				buys += t.size
			} else {
				sells += t.size
			}
			if t.size >= blockSize {
				blocks++
			}
		}
		return buys, sells, blocks
	}
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		w, h := float32(size.Width), float32(size.Height)
		d.Save().BeginPath().Rect(0, 0, w, h).Clip().Translate(0, 0).Bind(core.ArgY, offset).SetTextBaseline(core.BaselineMiddle)
		for i, t := range shown.Get() {
			y := float32(i)*tapeRow + tapeRow/2
			isBlock := t.size >= blockSize
			col := c.Down
			if t.side > 0 {
				col = c.Up
			}
			if isBlock {
				d.SetFillColor(tints.Accent()).BeginPath().RoundRect(0, float32(i)*tapeRow+2, w, tapeRow-4, 8).Fill()
			} else if i > 0 {
				d.SetStrokeColor(gridColor(cx)).SetLineWidth(1).BeginPath().MoveTo(0, float32(i)*tapeRow).LineTo(w, float32(i)*tapeRow).Stroke()
			}
			d.SetTextAlign(core.TextAlignStart).SetFillColor(c.Faint).FillText(marketTime(t.t, true), 8, y, sans(12, ui.Regular))
			d.SetFillColor(col).BeginPath().Arc(84, y, 3, 0, tau, false).Fill()
			d.FillText(priceText(t.price, s), 96, y, sans(14, ui.Regular))
			sizeFont, sizeColor := sans(14, ui.Regular), c.Text
			if isBlock {
				sizeFont, sizeColor = sans(14, ui.Semibold), c.Accent
			}
			d.SetTextAlign(core.TextAlignEnd).SetFillColor(sizeColor).FillText(compactNumber(t.size), w-8, y, sizeFont)
			if isBlock {
				d.FillText(tr.StocksBlock(), w-70, y, sans(10, ui.Semibold))
			}
		}
		d.Restore()
	}
	return block(cx, tr.StocksTimeSales(), label(tr.StocksBlocks()+compactNumber(blockSize)), nil, false,
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, ColumnGap: 16, RowGap: 6}, Children: []core.Node{
			Legend{Color: colors.Up, Label: tr.StocksAtAsk(), Value: func() string { b, _, _ := stats(); return compactNumber(b) }},
			Legend{Color: colors.Down, Label: tr.StocksAtBid(), Value: func() string { _, sl, _ := stats(); return compactNumber(sl) }},
			Legend{Color: colors.Accent, Label: tr.StocksBlocks2(), Value: func() string { _, _, n := stats(); return itoa(n) }},
		}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 10, Padding: core.Symmetric(0, 8)}, Children: []core.Node{
			head(cx, tr.StocksTime(), 80, 0, core.TextAlignStart), head(cx, tr.StocksPrice(), 0, 1, core.TextAlignStart), head(cx, tr.StocksSize(), 0, 0, core.TextAlignEnd),
		}},
		core.Canvas{Style: core.Style{Height: core.Pt(tapeShown * tapeRow)}, Draw: draw, Label: tr.StocksRecentTrades(), TestID: "tape"},
	)
}

// flowBlock is bought against sold volume by order size: who is moving
// the price.
func flowBlock(cx *core.Scope, m *market) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	sum := func(xs [4]float64) float64 { return xs[0] + xs[1] + xs[2] + xs[3] }
	most := func() float64 {
		f := m.flow.Get()
		v := 1.0
		for i := range 4 {
			v = math.Max(v, math.Max(f.buy[i], f.sell[i]))
		}
		return v
	}
	net := func() float64 { f := m.flow.Get(); return sum(f.buy) - sum(f.sell) }
	big := func() float64 { f := m.flow.Get(); return f.buy[2] + f.buy[3] - f.sell[2] - f.sell[3] }
	plus := func(v float64) string {
		if v >= 0 {
			return "+" + compactNumber(v)
		}
		return compactNumber(v)
	}
	var rows []core.Node
	for i, b := range flowBuckets {
		buy := func() float64 { return m.flow.Get().buy[i] }
		sell := func() float64 { return m.flow.Get().sell[i] }
		rows = append(rows, core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
				core.Text{Spans: []core.Span{{Content: b.label(locales.For(cx))}, {Content: " " + b.note, Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Faint)}},
					Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
				ui.Textf(func() string { return compactNumber(sell()) }, 12, ui.Regular, colors.Down),
				ui.Text("/", 12, ui.Regular, colors.Faint),
				ui.Textf(func() string { return compactNumber(buy()) }, 12, ui.Regular, colors.Up),
			}},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter}, Children: []core.Node{
				core.View{Style: fill, Children: []core.Node{springBar(func() float64 { return sell() / most() }, colors.Down, true, 10, 3)}},
				core.View{Style: core.Style{Width: core.Pt(2), Height: core.Pt(18), Margin: core.Symmetric(0, 2), CornerRadius: 1}, StyleFunc: ui.Filled(colors.Border)},
				core.View{Style: fill, Children: []core.Node{springBar(func() float64 { return buy() / most() }, colors.Up, false, 10, 3)}},
			}},
		}})
	}
	return block(cx, tr.StocksTradeFlow(), label(tr.StocksSoldBought()), nil, false,
		core.Text{Spans: []core.Span{
			{ContentFunc: func() string { return plus(net()) }},
			{ContentFunc: func() string { return tr.StocksNet() + plus(big()) + tr.StocksLargeOrders() }, Style: core.Style{FontSize: 13, FontWeight: ui.Regular},
				StyleFunc: ui.Colored(func() core.Color {
					if big() < 0 {
						return colors.Down()
					}
					return colors.Up()
				})},
		}, Style: core.Style{FontSize: 22, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
		core.View{Style: core.Style{Gap: 10}, Children: rows},
	)
}
