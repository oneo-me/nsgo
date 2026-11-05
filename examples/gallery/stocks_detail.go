package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"math"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// dayRange is where the last price sits in the day, in one line: the
// low–high span filled, the previous close marked, the last price a knob
// that springs on the platform.
func dayRange(cx *core.Scope, s *stock) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	q := s.quote.Get
	x := core.NewAnimatedValue(cx, 0)
	var width core.Points
	span := func() (float64, float64) {
		qq := q()
		lo, hi := math.Min(qq.low, qq.prevClose), math.Max(qq.high, qq.prevClose)
		if hi <= lo {
			hi = lo + s.tick
		}
		return lo, hi
	}
	at := func(v float64) float64 { lo, hi := span(); return (v - lo) / (hi - lo) }
	placed := false
	place := func() {
		if width == 0 {
			return
		}
		to := at(q().last) * float64(width)
		if placed {
			ui.Spring(cx, x, to, core.SpringConfig{Stiffness: 220, Damping: 24})
		} else {
			x.Set(to)
		}
		placed = true
	}
	cx.Effect(func() { q(); cx.Untrack(place) })
	p := func(v float64) string { return priceText(v, s) }
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
		ui.Tabular(ui.Textf(func() string { return "L " + p(q().low) }, 12, ui.Regular, colors.Muted)),
		core.View{OnLayout: func(b core.Bounds) { width = b.Size.Width; place() }, LabelFunc: func() string { return tr.StocksDayRange(p(q().low), p(q().high)) },
			Style: core.Style{FlexGrow: 1, FlexShrink: 1, Height: core.Pt(16), JustifyContent: core.JustifyCenter}, Children: []core.Node{
				core.View{Style: core.Style{Height: core.Pt(4), CornerRadius: 2}, StyleFunc: ui.Filled(colors.Raised)},
				core.View{StyleFunc: func() core.Style {
					l, h := at(q().low), at(q().high)
					return core.Style{Position: core.Absolute, Left: core.Pct(float32(l * 100)), Width: core.Pct(float32((h - l) * 100)), Height: core.Pt(4), CornerRadius: 2, Background: colors.Border()}
				}},
				core.View{StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Left: core.Pct(float32(at(q().prevClose) * 100)), Width: core.Pt(2), Height: core.Pt(12), Margin: core.Edges{Left: -1},
						CornerRadius: 1, Background: colors.Muted()}
				}},
				core.View{StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Left: core.Pt(-7), Width: core.Pt(14), Height: core.Pt(14), CornerRadius: 7, BorderWidth: 3, BorderColor: colors.Background(),
						Background: toneOf(cx, q().last-q().prevClose), Animated: &core.AnimatedStyle{TranslateX: x}}
				}},
			}},
		ui.Tabular(ui.Textf(func() string { return "H " + p(q().high) }, 12, ui.Regular, colors.Muted)),
	}}
}

// quoteBlock is a big figure, a direction arrow that turns over on the
// platform, the day's range and figures.
func quoteBlock(cx *core.Scope, m *market, s *stock) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	q := s.quote.Get
	change := func() float64 { qq := q(); return qq.last - qq.prevClose }
	// Price, arrow and change all follow the day's direction; the arrow
	// turns over when it flips.
	down := core.Untracked(cx, change) < 0
	start := 0.0
	if down {
		start = 1
	}
	turn := core.NewAnimatedValue(cx, start)
	cx.Effect(func() {
		if (change() < 0) == down {
			return
		}
		down = !down
		to := 0.0
		if down {
			to = 1
		}
		ui.Spring(cx, turn, to, core.SpringConfig{Stiffness: 300, Damping: 18})
	})
	p := func(v float64) string { return priceText(v, s) }
	yearLo, yearHi := math.Inf(1), math.Inf(-1)
	for _, d := range s.days {
		yearLo, yearHi = math.Min(yearLo, d.l), math.Max(yearHi, d.h)
	}
	sectorName := ""
	for _, sc := range sectors {
		if sc.id == s.sector {
			sectorName = sc.label(locales.For(cx))
		}
	}
	decimals := 2
	if s.tick < 0.01 {
		decimals = 3
	}
	toneStyle := func(size core.Points, weight core.FontWeight) func() core.Style {
		return func() core.Style {
			return core.Style{FontSize: size, FontWeight: weight, FontVariant: core.FontTabularNums, Color: toneOf(cx, change())}
		}
	}
	w := core.Pct(25)
	return core.View{Style: core.Style{Padding: core.Edges{Top: 12, Right: 16, Bottom: 14, Left: 16}, Gap: 10}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			core.Text{Content: s.name, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			ui.Text(sectorName, 13, ui.Regular, colors.Faint),
			liveDot(cx, m.running.Get, 6),
			ui.Tabular(ui.Textf(func() string { return marketTime(float64(m.clock.Get()), true) }, 12, ui.Regular, colors.Muted)),
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
			core.Text{ContentFunc: func() string { return p(q().last) }, TestID: "stock-price", StyleFunc: func() core.Style {
				return core.Style{FontSize: 36, FontWeight: ui.Semibold, LetterSpacing: -1, FontVariant: core.FontTabularNums, Color: toneOf(cx, change())}
			}},
			core.View{Style: core.Style{Animated: &core.AnimatedStyle{Rotate: ui.Lerp(turn, 0, 180)}}, Children: []core.Node{ui.Icon(lucide.ArrowUp, 20, func() core.Color { return toneOf(cx, change()) })}},
			core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignEnd, Gap: 1}, Children: []core.Node{
				core.Text{ContentFunc: func() string { return signed(percentOf(q()), 2) + "%" }, StyleFunc: toneStyle(15, ui.Semibold)},
				core.Text{ContentFunc: func() string { return signed(change(), decimals) + tr.StocksToday() }, StyleFunc: toneStyle(12, ui.Regular)},
			}},
		}},
		dayRange(cx, s),
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap}, Children: []core.Node{
			Stat{Label: tr.StocksOpen(), Value: func() string { return p(q().open) }, Width: w},
			Stat{Label: tr.StocksPrevClose(), Value: func() string { return p(q().prevClose) }, Width: w},
			Stat{Label: tr.StocksAvgPrice(), Value: func() string { qq := q(); return p(qq.turnover / math.Max(qq.volume, 1)) }, Width: w},
			Stat{Label: tr.StocksAmplitude(), Value: func() string { qq := q(); return fmt.Sprintf("%.2f%%", (qq.high-qq.low)/qq.prevClose*100) }, Width: w},
			Stat{Label: tr.StocksVolume(), Value: func() string { return compactNumber(q().volume) }, Width: w},
			Stat{Label: tr.StocksTurnover(), Value: func() string { return compactNumber(q().turnover) }, Width: w},
			Stat{Label: tr.Stocks120dHigh(), Value: label(p(yearHi)), Width: w},
			Stat{Label: tr.Stocks120dLow(), Value: label(p(yearLo)), Width: w},
		}},
	}}
}

var horizons = []int{5, 20, 60, 120}

// rangesBlock is ranges over 5 to 120 days as dumbbells, low to high, the
// last price a line across them and the previous close dashed.
func rangesBlock(cx *core.Scope, m *market, s *stock) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	type rng struct {
		n      int
		lo, hi float64
	}
	rows := core.NewMemoFunc(cx, func() []rng {
		m.pulse.Get()
		q := s.quote.GetUntracked()
		var out []rng
		for _, n := range horizons {
			lo, hi := q.low, q.high
			for _, d := range s.days[max(0, len(s.days)-n):] {
				lo, hi = math.Min(lo, d.l), math.Max(hi, d.h)
			}
			out = append(out, rng{n, lo, hi})
		}
		return out
	}, func(a, b []rng) bool { return false })
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		r := rows.Get()
		q := s.quote.GetUntracked()
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, x := range r {
			lo, hi = math.Min(lo, x.lo), math.Max(hi, x.hi)
		}
		w := float32(size.Width)
		const left, right = 40, 6
		X := func(v float64) float32 {
			span := hi - lo
			if span == 0 {
				span = 1
			}
			return left + float32((v-lo)/span)*(w-left-right)
		}
		f := sans(11, ui.Regular)
		d.SetTextBaseline(core.BaselineMiddle)
		for i, row := range r {
			y := 10 + float32(i)*34
			d.SetFillColor(c.Muted).SetTextAlign(core.TextAlignStart).FillText(fmt.Sprintf("%dd", row.n), 0, y, f)
			d.SetStrokeColor(c.Border).SetLineWidth(3).SetLineCap(core.CapRound).BeginPath().MoveTo(X(row.lo), y).LineTo(X(row.hi), y).Stroke()
			d.SetFillColor(c.Down).BeginPath().Arc(X(row.lo), y, 4.5, 0, tau, false).Fill()
			d.SetFillColor(c.Up).BeginPath().Arc(X(row.hi), y, 4.5, 0, tau, false).Fill()
			d.SetFillColor(c.Faint).SetTextAlign(core.TextAlignStart).FillText(priceText(row.lo, s), X(row.lo)-4, y+13, f)
			d.SetTextAlign(core.TextAlignEnd).FillText(priceText(row.hi, s), X(row.hi)+4, y+13, f)
		}
		bottom := float32(len(horizons)) * 34
		d.SetStrokeColor(c.Faint).SetLineWidth(1).SetLineDash(3, 4).BeginPath().MoveTo(X(q.prevClose), 0).LineTo(X(q.prevClose), bottom).Stroke().SetLineDash()
		d.SetStrokeColor(c.Accent).SetLineWidth(2).BeginPath().MoveTo(X(q.last), 0).LineTo(X(q.last), bottom).Stroke()
		d.SetFillColor(c.Accent).SetTextAlign(core.TextAlignCenter).FillText(tr.StocksLast()+priceText(q.last, s), max(left+30, min(w-40, X(q.last))), bottom+8, f)
	}
	return block(cx, tr.StocksRanges(), label(tr.StocksLowHighEachSecond()), nil, false,
		core.Canvas{Style: core.Style{Height: core.Pt(core.Points(len(horizons)*34 + 16))}, Draw: draw, Label: tr.StocksPriceRanges()})
}

// pauseButton pauses and resumes the market.
func pauseButton(cx *core.Scope, m *market) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	return core.Dynamic(func(*core.Scope) core.Node {
		run := m.running.Get()
		face := func() core.Style {
			s := core.Style{Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 16, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
			if run {
				s = fin.Pill().Over(s)
				s.BorderWidth = 1
			} else {
				s = fin.Accent().Over(s)
				s.Background = colors.AccentFill()
			}
			return s
		}
		name, tint, lbl := lucide.Play, colors.OnAccent, tr.StocksResumeTheMarket()
		if run {
			name, tint, lbl = lucide.Pause, colors.Muted, tr.StocksPauseTheMarket()
		}
		return ui.Press{OnPress: func() {
			if m.running.GetUntracked() {
				m.halt()
			} else {
				m.start()
			}
		}, HitSlop: 6, Label: lbl, Children: []core.Node{core.View{StyleFunc: face, Children: []core.Node{ui.Icon(name, 13, tint)}}}}
	})
}

// stockScreen is a stock, pushed: its view under a header with the
// pause button.
func stockScreen(m *market, s *stock) func(cx *core.Scope) base.NavPage {
	return func(cx *core.Scope) base.NavPage {
		return screen(cx, s.code, pauseButton(cx, m), stockView(cx, m, s, false, nil))
	}
}

// stockView is a stock: quote and chart on the first screen, then the
// book, depth, the tape, the flow and the ranges, all live. Buy and Sell
// at the bottom open the order ticket. wide, the desktop layout beside
// the watchlist, heads it with the code, draws the chart taller and,
// while split holds, puts the book, the tape, depth and flow in a column
// right of the quote, chart and ranges.
func stockView(cx *core.Scope, m *market, s *stock, wide bool, split func() bool) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	env := cx.Env()
	m.focus(s)
	cx.Cleanup(func() { m.focus(nil) })
	sd := core.NewSignal(cx, side("buy"))
	open := core.NewSignal(cx, false)
	bn := newBanner(cx)
	trade := func(x side) { sd.Set(x); open.Set(true) }
	footer := func() core.Node {
		return core.Text{ContentFunc: func() string {
			st := m.stats.Get()
			return tr.StocksSimulatedStats(fmt.Sprintf("%.2f", st[0]), fmt.Sprintf("%.0f", st[1]))
		}, StyleFunc: func() core.Style {
			return core.Style{Padding: core.Edges{Top: 8, Right: 16, Bottom: 96 + env.SafeArea().Bottom, Left: 16}, FontSize: 12, FontVariant: core.FontTabularNums, Color: colors.Faint()}
		}}
	}
	if split == nil {
		split = func() bool { return false }
	}
	ticketView := func(cx *core.Scope) core.Node {
		return ticket(cx, open.Get, s, sd, func(title, detail string, x side) {
			open.Set(false)
			bn.show(title, detail, x)
		})
	}
	var body core.Node
	if !wide {
		body = core.Scroll{Style: core.Style{FlexGrow: 1}, TestID: "stock-scroll", Children: []core.Node{
			quoteBlock(cx, m, s),
			block(cx, "", nil, nil, false, stockChart(cx, s, m.running.Get, 0)),
			orderBookBlock(cx, m, s),
			tapeBlock(cx, m, s),
			depthBlock(cx, m, s),
			flowBlock(cx, m),
			rangesBlock(cx, m, s),
			footer(),
		}}
	} else {
		body = core.Scroll{Style: core.Style{FlexGrow: 1}, TestID: "stock-scroll", Children: []core.Node{
			core.Text{Content: s.code, Style: core.Style{Padding: core.Edges{Top: 14, Right: 16, Left: 16}, FontSize: 22, FontWeight: ui.Bold, LetterSpacing: -0.3}, StyleFunc: ui.Colored(colors.Text)},
			core.View{StyleFunc: func() core.Style {
				if split() {
					return core.Style{Direction: core.Row, AlignItems: core.AlignStart}
				}
				return core.Style{Direction: core.Column, AlignItems: core.AlignStretch}
			}, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					if split() {
						return core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}
					}
					return core.Style{}
				}, Children: []core.Node{
					quoteBlock(cx, m, s),
					block(cx, "", nil, nil, false, stockChart(cx, s, m.running.Get, 340)),
					rangesBlock(cx, m, s),
				}},
				core.View{StyleFunc: func() core.Style {
					if split() {
						return core.Style{Width: core.Pt(380), Borders: core.Edges{Left: 1}, BorderColors: core.EdgeColors{Left: colors.Line()}}
					}
					return core.Style{}
				}, Children: []core.Node{orderBookBlock(cx, m, s), tapeBlock(cx, m, s), depthBlock(cx, m, s), flowBlock(cx, m)}},
			}},
			footer(),
		}}
	}
	var sheetNode core.Node
	if wide {
		// A card in the window on desktops: Escape or a click outside
		// closes it.
		sheetNode = base.Dialog{Open: open.Get, Presentation: core.ModalFormSheet, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, Label: tr.StocksOrderTicket(),
			BackdropStyle: core.Style{Background: ui.Black(0.4)},
			PopupStyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(420), MaxWidth: core.Pct(100), Padding: core.All(20), CornerRadius: ui.RadiusCard, Background: colors.Surface(), Shadows: fin.Menu().Shadows}
			},
			Children: []core.Node{core.Func(ticketView)}}
	} else {
		sheetNode = ui.Sheet{Open: open.Get, OnClose: func() { open.Set(false) }, Children: []core.Node{core.Func(ticketView)}}
	}
	return core.View{StyleFunc: func() core.Style { return fillPage.Merge(core.Style{Background: colors.Background()}) }, Children: []core.Node{
		body,
		// The trade bar, over the content: glass, the page showing through
		// blurred as it scrolls under.
		core.View{StyleFunc: func() core.Style {
			return fin.Glass().Over(core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10,
				Padding: core.Edges{Top: 10, Right: 16, Bottom: 10 + env.SafeArea().Bottom, Left: 16}, Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: fin.Glass().Border}})
		}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(92), Gap: 1}, Children: []core.Node{
				ui.Tabular(ui.Textf(func() string { return priceText(s.quote.Get().last, s) }, 13, ui.Semibold, colors.Text)),
				core.Text{ContentFunc: func() string { return signed(percentOf(s.quote.Get()), 2) + "%" }, StyleFunc: func() core.Style {
					return core.Style{FontSize: 12, FontVariant: core.FontTabularNums, Color: toneOf(cx, percentOf(s.quote.Get()))}
				}},
			}},
			ui.Cell{Child: ui.Button{Text: tr.StocksSell(), Variant: ui.ButtonSecondary, OnPress: func() { trade("sell") }}},
			ui.Cell{Child: ui.Button{Text: tr.StocksBuy(), OnPress: func() { trade("buy") }}},
		}},
		bn.view,
		sheetNode,
	}}
}
