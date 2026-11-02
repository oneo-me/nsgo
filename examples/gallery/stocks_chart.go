package main

import (
	"fmt"
	"math"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

type period string

// periods are the chart's periods, intraday named in tr's language.
func periods(tr locales.Messages) []ui.Option[period] {
	return []ui.Option[period]{{Value: "intraday", Label: tr.StocksIntraday()}, {Value: "1m", Label: "1m"}, {Value: "5m", Label: "5m"}, {Value: "15m", Label: "15m"}, {Value: "1D", Label: "1D"}}
}

// averages are the moving averages the candles show.
func averages(cx *core.Scope) []struct {
	n int
	c func() core.Color
} {
	colors := ui.ColorsOf(cx)
	return []struct {
		n int
		c func() core.Color
	}{{5, colors.Accent}, {10, colors.Info}, {20, colors.Warn}}
}

// bodies are candle bodies in points by period.
var bodies = map[period]float32{"intraday": 1, "1m": 5, "5m": 7, "15m": 9, "1D": 5}

const (
	chartRight  = 54
	chartTop    = 8
	chartBottom = 18
	chartHeight = 216
)

// candlesOf is a period's candles: minutes merged into 5 or 15, or the
// days with today last.
func candlesOf(s *stock, p period) []candle {
	minutes := s.minutes.Get()
	switch p {
	case "1m", "intraday":
		return minutes
	case "1D":
		q := s.quote.Get()
		return append(append([]candle(nil), s.days...), candle{t: 0, o: q.open, h: q.high, l: q.low, c: q.last, v: q.volume, a: q.turnover / q.volume})
	}
	span := 300.0
	if p == "15m" {
		span = 900
	}
	var out []candle
	for _, m := range minutes {
		t := m.t - math.Mod(m.t-marketOpen, span)
		if k := len(out) - 1; k >= 0 && out[k].t == t {
			c := &out[k]
			c.h, c.l, c.c, c.v, c.a = math.Max(c.h, m.h), math.Min(c.l, m.l), m.c, c.v+m.v, m.a
		} else {
			m.t = t
			out = append(out, m)
		}
	}
	return out
}

// average is the mean close of the n candles ending at i; false before
// there are n.
func average(data []candle, i, n int) (float64, bool) {
	if i < n-1 || i >= len(data) {
		return 0, false
	}
	sum := 0.0
	for k := i - n + 1; k <= i; k++ {
		sum += data[k].c
	}
	return sum / float64(n), true
}

func candleLabel(tr locales.Messages, c candle, p period) string {
	if p == "1D" {
		if c.t == 0 {
			return tr.StocksToday2()
		}
		return tr.StocksDaysAgo(int(c.t))
	}
	return marketTime(c.t, false)
}

// chartFrame is the geometry of one drawing, shared with the crosshair.
type chartFrame struct {
	first, count            int
	lo, hi                  float64
	mainH, volTop, volH, pw float32
}

// stockChart is the stock's chart: the intraday time-share line with its
// average price, or candles with MA5, 10 and 20, both over volume. Drag
// sideways for the crosshair; its lines move on the platform and the
// readout floats beside it.
func stockChart(cx *core.Scope, s *stock, running func() bool, height core.Points) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	if height == 0 {
		height = chartHeight
	}
	per := core.NewSignal(cx, period("intraday"))
	cursor := core.NewSignal(cx, -1)
	width := core.NewSignal(cx, core.Points(0))
	live := livePulse(cx, running)
	// The live point's ring, built once: redraws reuse the same nodes.
	ringAlpha, ringRadius := ui.Lerp(live, 0.45, 0), ui.Lerp(live, 3, 13)
	// The minutes are one slice changed in place: every run is a change.
	data := core.NewMemoFunc(cx, func() []candle { return candlesOf(s, per.Get()) }, func(a, b []candle) bool { return false })
	step := func() float32 { return bodies[per.Get()] + 2 }
	intraday := func() bool { return per.Get() == "intraday" }
	// span is the minutes the intraday axis spans: the session so far (an
	// hour at least) and a narrow margin ahead, never past the close.
	span := func() int { return int(math.Min(session, math.Ceil(math.Max(60, float64(len(data.Get())))*1.06))) }
	view := func() (int, int) {
		if intraday() {
			return 0, span()
		}
		n := max(1, int((float32(width.Get())-chartRight)/step()))
		return max(0, len(data.Get())-n), n
	}
	X := func(i int) float32 {
		pw := float32(width.Get()) - chartRight
		if intraday() {
			return float32(i) / float32(span()) * pw
		}
		first, _ := view()
		return float32(i-first)*step() + step()/2
	}
	var frame *chartFrame
	// Drawn again on every tick: about a second to settle after the data
	// narrows.
	scale := &calmRange{rate: 0.06}
	cx.Effect(func() { per.Get(); scale.reset() })

	// The crosshair: two hairlines and a dot placed by animated values.
	cxv, cyv, shown := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	pick := func(x core.Points) {
		if x < 0 {
			cursor.Set(-1)
			ui.Timing(cx, shown, 0, core.TimingConfig{Duration: ms(180)})
			return
		}
		cx.Untrack(func() {
			d := data.Get()
			if len(d) == 0 {
				return
			}
			pw := float32(width.Get()) - chartRight
			first, _ := view()
			var raw int
			if intraday() {
				raw = int(math.Round(float64(float32(x) / pw * float32(span()))))
			} else {
				raw = first + int(float32(x)/step())
			}
			i := max(first, min(len(d)-1, raw))
			if cursor.Get() < 0 {
				ui.Timing(cx, shown, 1, core.TimingConfig{Duration: ms(140)})
			}
			cursor.Set(i)
			cxv.Set(float64(X(i)))
			if frame != nil {
				cyv.Set(float64(chartTop + float32((frame.hi-d[i].c)/(frame.hi-frame.lo))*frame.mainH))
			}
		})
	}
	focus := func() candle {
		d := data.Get()
		i := cursor.Get()
		if i < 0 || i >= len(d) {
			i = len(d) - 1
		}
		if i < 0 {
			return candle{}
		}
		return d[i]
	}
	p := func(v float64) string { return priceText(v, s) }
	prevClose := func() float64 { return s.quote.Get().prevClose }
	readoutLeft := func() core.Points {
		i := cursor.Get()
		if i < 0 {
			return 0
		}
		x, w := X(i), float32(width.Get())
		if x > w/2 {
			return core.Points(max(0, x-150))
		}
		return core.Points(min(w-140, x+10))
	}

	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		grid := gridColor(cx)
		dd := data.Get()
		first, n := view()
		end := min(len(dd), first+n)
		if end <= first {
			return
		}
		w, h := float32(size.Width), float32(size.Height)
		pw := w - chartRight
		const volH = 40
		mainH := h - chartTop - chartBottom - volH - 12
		volTop := chartTop + mainH + 12
		lo, hi := math.Inf(1), math.Inf(-1)
		if intraday() {
			lo, hi = prevClose(), prevClose()
		}
		for i := first; i < end; i++ {
			lo, hi = math.Min(lo, dd[i].l), math.Max(hi, dd[i].h)
		}
		margin := (hi - lo) * 0.08
		if margin == 0 {
			margin = hi * 0.01
		}
		// Intraday the range is the day's, which only widens; candles
		// scroll, so their range eases in rather than jumping.
		rlo, rhi := lo-margin, hi+margin
		if !intraday() {
			rlo, rhi = scale.fit(rlo, rhi)
		}
		f := &chartFrame{first: first, count: n, lo: rlo, hi: rhi, mainH: mainH, volTop: volTop, volH: volH, pw: pw}
		frame = f
		Y := func(v float64) float32 { return chartTop + float32((f.hi-v)/(f.hi-f.lo))*mainH }
		f11 := sans(11, ui.Regular)

		// The grid and the price axis, in percent from the close intraday.
		d.SetTextBaseline(core.BaselineMiddle).SetLineWidth(1)
		for k := 0; k <= 4; k++ {
			v := f.hi - (f.hi-f.lo)*float64(k)/4
			y := Y(v)
			d.SetStrokeColor(grid).BeginPath().MoveTo(0, y).LineTo(pw, y).Stroke()
			lbl := p(v)
			if intraday() {
				lbl = signed((v-prevClose())/prevClose()*100, 2) + "%"
			}
			d.SetFillColor(c.Faint).SetTextAlign(core.TextAlignEnd).FillText(lbl, w, y, f11)
		}
		d.SetStrokeColor(grid).BeginPath().MoveTo(0, volTop+volH).LineTo(pw, volTop+volH).Stroke()

		if intraday() {
			now := X(len(dd) - 1)
			last := dd[len(dd)-1].c
			tone := toneOf(cx, last-prevClose())
			stockHatch(d, now+4, chartTop, pw-now-4, mainH, grid, 7)
			hline(d, 0, pw, Y(prevClose()), c.Faint)
			// The price area and line.
			d.SetFillGradient(core.CanvasLinear(0, chartTop, 0, chartTop+mainH, core.GradientStop{Offset: 0, Color: ui.Alpha(tone, 0.26)}, core.GradientStop{Offset: 1, Color: ui.Alpha(tone, 0)}))
			d.BeginPath().MoveTo(X(0), chartTop+mainH)
			for i, cd := range dd {
				d.LineTo(X(i), Y(cd.c))
			}
			d.LineTo(now, chartTop+mainH).ClosePath().Fill()
			d.SetLineJoin(core.JoinRound).BeginPath()
			for i, cd := range dd {
				if i == 0 {
					d.MoveTo(X(i), Y(cd.c))
				} else {
					d.LineTo(X(i), Y(cd.c))
				}
			}
			d.SetStrokeColor(tone).SetLineWidth(1.5).Stroke()
			// The day's average price.
			d.BeginPath()
			for i, cd := range dd {
				if i == 0 {
					d.MoveTo(X(i), Y(cd.a))
				} else {
					d.LineTo(X(i), Y(cd.a))
				}
			}
			d.SetStrokeColor(c.Warn).SetLineWidth(1.1).Stroke()
			// Volume.
			vw := max(1, pw/float32(span())-0.8)
			vmax := 0.0
			for _, cd := range dd {
				vmax = math.Max(vmax, cd.v)
			}
			if vmax == 0 {
				vmax = 1
			}
			for i, cd := range dd {
				vh := float32(cd.v/vmax) * volH
				col := c.Down
				if cd.c >= cd.o {
					col = c.Up
				}
				d.SetFillColor(ui.Alpha(col, 0.6)).FillRect(X(i)-vw/2, volTop+volH-vh, vw, vh)
			}
			// The live point, pulsing on the platform.
			ly := Y(last)
			d.SetFillColor(tone).SetAlpha(0.45).Bind(core.ArgA, ringAlpha).BeginPath().Arc(now, ly, 3, 0, tau, false).Bind(core.ArgR, ringRadius).Fill()
			d.SetAlpha(1).BeginPath().Arc(now, ly, 3, 0, tau, false).Fill()
			// The open, the middle and the end of the span.
			d.SetFillColor(c.Faint).SetTextBaseline(core.BaselineTop)
			for _, k := range []struct {
				at    float32
				align core.TextAlign
			}{{0, core.TextAlignStart}, {0.5, core.TextAlignCenter}, {1, core.TextAlignEnd}} {
				d.SetTextAlign(k.align).FillText(marketTime(marketOpen+math.Round(float64(k.at)*float64(span()))*60, false), k.at*pw, volTop+volH+5, f11)
			}
		} else {
			body := bodies[per.Get()]
			vmax := 0.0
			for i := first; i < end; i++ {
				vmax = math.Max(vmax, dd[i].v)
			}
			if vmax == 0 {
				vmax = 1
			}
			for i := first; i < end; i++ {
				cd := dd[i]
				x := X(i)
				col := c.Down
				if cd.c >= cd.o {
					col = c.Up
				}
				d.SetStrokeColor(col).SetLineWidth(1).BeginPath().MoveTo(x, Y(cd.h)).LineTo(x, Y(cd.l)).Stroke()
				top := Y(math.Max(cd.o, cd.c))
				bh := max(1, Y(math.Min(cd.o, cd.c))-top)
				if cd.c >= cd.o {
					d.SetFillColor(c.Inset).FillRect(x-body/2, top, body, bh).StrokeRect(x-body/2+0.5, top+0.5, body-1, max(0, bh-1))
				} else {
					d.SetFillColor(col).FillRect(x-body/2, top, body, bh)
				}
				vh := float32(cd.v/vmax) * volH
				d.SetFillColor(ui.Alpha(col, 0.5)).FillRect(x-body/2, volTop+volH-vh, body, vh)
			}
			d.SetLineWidth(1.2).SetLineJoin(core.JoinRound)
			for _, a := range averages(cx) {
				d.SetStrokeColor(a.c()).BeginPath()
				started := false
				for i := first; i < end; i++ {
					v, ok := average(dd, i, a.n)
					if !ok {
						continue
					}
					if started {
						d.LineTo(X(i), Y(v))
					} else {
						d.MoveTo(X(i), Y(v))
					}
					started = true
				}
				d.Stroke()
			}
			d.SetFillColor(c.Faint).SetTextBaseline(core.BaselineTop)
			cnt := end - first
			for _, k := range []struct {
				at    int
				align core.TextAlign
			}{{0, core.TextAlignStart}, {cnt / 2, core.TextAlignCenter}, {cnt - 1, core.TextAlignEnd}} {
				d.SetTextAlign(k.align).FillText(candleLabel(tr, dd[first+k.at], per.Get()), min(pw, max(0, X(first+k.at))), volTop+volH+5, f11)
			}
		}

		// The last price on the axis.
		last := dd[len(dd)-1].c
		ly := max(chartTop+8, min(chartTop+mainH-8, Y(last)))
		ref := dd[len(dd)-1].o
		if intraday() {
			ref = prevClose()
		}
		tone := toneOf(cx, last-ref)
		if !intraday() {
			hline(d, 0, pw, ly, ui.Alpha(tone, 0.7), 3, 3)
		}
		d.SetFillColor(tone).BeginPath().RoundRect(pw+2, ly-9, chartRight-2, 18, 5).Fill()
		d.SetFillColor(c.Inset).SetTextAlign(core.TextAlignEnd).SetTextBaseline(core.BaselineMiddle).FillText(p(last), w-4, ly, sans(11, ui.Semibold))
	}

	legends := core.Dynamic(func(*core.Scope) core.Node {
		if intraday() {
			return core.Group{
				core.Dynamic(func(*core.Scope) core.Node {
					return ui.Legend{Color: func() core.Color { return toneOf(cx, focus().c-prevClose()) }, Label: tr.StocksPrice2(), Value: func() string { return p(focus().c) }}
				}),
				ui.Legend{Color: colors.Warn, Label: tr.StocksAvg(), Value: func() string { return p(focus().a) }},
				ui.Legend{Color: colors.Faint, Label: tr.StocksVol(), Value: func() string { return compactNumber(focus().v) }},
			}
		}
		var out core.Group
		for _, a := range averages(cx) {
			out = append(out, ui.Legend{Color: a.c, Label: fmt.Sprintf("MA%d", a.n), Value: func() string {
				i := cursor.Get()
				if i < 0 {
					i = len(data.Get()) - 1
				}
				if v, ok := average(data.Get(), i, a.n); ok {
					return p(v)
				}
				return "–"
			}})
		}
		return out
	})

	canvas := scrubbing(core.View{OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Style: core.Style{Height: core.Pt(height)}, Children: []core.Node{
		core.Canvas{Style: core.Style{Height: core.Pt(height)}, Draw: draw, Label: s.code + tr.StocksChart(), TestID: "chart"},
	}}, pick)
	return core.View{Style: core.Style{Gap: 10}, Children: []core.Node{
		ui.Segmented[period]{Options: periods(tr), Value: per.Get, OnChange: per.Set, Small: true},
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, ColumnGap: 14, RowGap: 6}, Children: []core.Node{legends}},
		core.View{Style: core.Style{Height: core.Pt(height)}, Children: []core.Node{
			canvas,
			// The crosshair over the canvas, moved by animated values.
			core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0),
				Animated: &core.AnimatedStyle{Opacity: shown}}, Children: []core.Node{
				core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(chartTop), Bottom: core.Pt(chartBottom), Left: core.Pt(0), Width: core.Pt(1),
					Animated: &core.AnimatedStyle{TranslateX: cxv}}, StyleFunc: ui.Filled(colors.Muted)},
				core.View{StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(chartRight), Top: core.Pt(0), Height: core.Pt(1), Background: ui.Alpha(colors.Muted(), 0.6),
						Animated: &core.AnimatedStyle{TranslateY: cyv}}
				}},
				core.View{StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Top: core.Pt(-4), Left: core.Pt(-4), Width: core.Pt(9), Height: core.Pt(9), CornerRadius: 4.5, Background: colors.Text(),
						BorderWidth: 2, BorderColor: colors.Inset(), Animated: &core.AnimatedStyle{TranslateX: cxv, TranslateY: cyv}}
				}},
				core.Show{When: func() bool { return cursor.Get() >= 0 }, Then: func(*core.Scope) core.Node {
					return core.View{StyleFunc: func() core.Style {
						return core.Style{Position: core.Absolute, Top: core.Pt(chartTop + 2), Left: core.Pt(readoutLeft()), Width: core.Pt(140), Padding: core.All(10), Gap: 3,
							CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, Background: ui.Alpha(colors.Surface(), 0.94), BorderWidth: 1, BorderColor: colors.Border()}
					}, Children: []core.Node{
						ui.Textf(func() string { return candleLabel(tr, focus(), per.Get()) }, 12, ui.Regular, colors.Muted),
						core.Text{Spans: []core.Span{
							{ContentFunc: func() string { return p(focus().c) }},
							{ContentFunc: func() string { return " " + signed((focus().c-prevClose())/prevClose()*100, 2) + "%" }, Style: core.Style{FontSize: 12, FontWeight: ui.Regular}},
						}, Style: core.Style{FontSize: 17, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
						core.Text{ContentFunc: func() string {
							f := focus()
							if intraday() {
								return tr.StocksAvgVol(p(f.a), compactNumber(f.v))
							}
							return fmt.Sprintf("O %s H %s\nL %s V %s", p(f.o), p(f.h), p(f.l), compactNumber(f.v))
						}, Style: core.Style{FontSize: 11, LineHeight: 15}, StyleFunc: ui.Colored(colors.Muted)},
					}}
				}},
			}},
		}},
	}}
}
