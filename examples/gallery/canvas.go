package main

import (
	"fmt"
	"math"
	"time"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

const tau = 2 * math.Pi

func sans(size core.Points, weight core.FontWeight) core.Font {
	return core.Font{Size: size, Weight: weight}
}

// seeded is a small seeded random generator, so the fictional data is
// stable.
func seeded(seed uint32) func() float64 {
	s := seed
	return func() float64 {
		s += 0x6d2b79f5
		t := s
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return float64(t^(t>>14)) / 4294967296
	}
}

// hatchClip is diagonal hatching over the current clip.
func hatchClip(d *core.Drawing, x, y, w, h float32, c core.Color, gap float32) {
	d.SetStrokeColor(c).SetLineWidth(1).BeginPath()
	for i := -h; i < w; i += gap {
		d.MoveTo(x+i, y+h).LineTo(x+i+h, y)
	}
	d.Stroke()
}

// dashed is a dashed reference line.
func dashed(d *core.Drawing, x0, y0, x1, y1 float32, c core.Color) {
	d.Save().SetLineDash(3, 4).SetStrokeColor(c).SetLineWidth(1).BeginPath().MoveTo(x0, y0).LineTo(x1, y1).Stroke().Restore()
}

// reveal is a value that sweeps 0 to 1 once mounted, and again on
// replay: canvases clip to it.
func reveal(cx *core.Scope, d time.Duration) (*core.AnimatedValue, func()) {
	t := core.NewAnimatedValue(cx, 0)
	replay := func() {
		t.Set(0)
		ui.Timing(cx, t, 1, core.TimingConfig{Duration: d, Easing: ui.EaseOutCubic})
	}
	replay()
	return t, replay
}

func cos32(a float64) float32 { return float32(math.Cos(a)) }
func sin32(a float64) float32 { return float32(math.Sin(a)) }

var dialPresets = []struct {
	value string
	label func(locales.Messages) string
	load  float64
}{{"idle", locales.Messages.CanvasIdle, 18}, {"cruise", locales.Messages.CanvasCruise, 57}, {"peak", locales.Messages.CanvasPeak, 91}}

// dial is a tick ring, a hatched red zone, a gradient sweep and a needle,
// all bound to one animated value: springing it moves the needle on the
// platform's main thread.
func dial(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	preset := core.NewSignal(cx, "cruise")
	load := func() float64 {
		for _, p := range dialPresets {
			if p.value == preset.Get() {
				return p.load
			}
		}
		return 0
	}
	needle := core.NewAnimatedValue(cx, 0)
	ui.Spring(cx, needle, core.Untracked(cx, load), core.SpringConfig{Stiffness: 90, Damping: 9})
	choose := func(p string) {
		preset.Set(p)
		ui.Spring(cx, needle, core.Untracked(cx, load), core.SpringConfig{Stiffness: 140, Damping: 11})
	}
	a0, a1 := 0.75*math.Pi, 2.25*math.Pi
	angle := core.Interpolate(needle, core.Interpolation{Input: []float64{0, 100}, Output: []float64{a0, a1}, Extrapolate: core.ExtrapolateExtend})
	zone := func() Tone {
		switch l := load(); {
		case l >= 85:
			return ToneDanger
		case l >= 60:
			return ToneWarn
		}
		return ToneUp
	}
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		shade := ui.Black(0.3)
		if cx.Dark() {
			shade = ui.Black(0.6)
		}
		w, h := float32(size.Width), float32(size.Height)
		cxp, cy := w/2, h*0.54
		r := min(w/2-8, h*0.5) - 6
		at := func(v float64) float64 { return a0 + v/100*(a1-a0) }
		// The red zone: an annular sector, hatched.
		d.Save().BeginPath().Arc(cxp, cy, r+2, float32(at(85)), float32(a1), false).Arc(cxp, cy, r-20, float32(a1), float32(at(85)), true).ClosePath().Clip()
		zx, zy, zw, zh := cxp+0.6*r, cy-0.2*r, 0.4*r+4, r
		d.SetFillColor(ui.Alpha(c.Danger, 0.08)).FillRect(zx, zy, zw, zh)
		hatchClip(d, zx, zy, zw, zh, ui.Alpha(c.Danger, 0.55), 5)
		d.Restore()
		// The tick ring.
		d.SetLineCap(core.CapRound)
		for i := 0; i <= 50; i++ {
			a := at(float64(i * 2))
			major := i%10 == 0
			in := r - 8
			col, lw := c.Faint, float32(1)
			if major {
				in, col, lw = r-14, c.Muted, 2
			}
			if i*2 >= 85 {
				col = c.Danger
			}
			d.SetStrokeColor(col).SetLineWidth(lw).BeginPath().MoveTo(cxp+cos32(a)*in, cy+sin32(a)*in).LineTo(cxp+cos32(a)*r, cy+sin32(a)*r).Stroke()
			if major {
				d.SetFillColor(c.Faint).SetTextAlign(core.TextAlignCenter).SetTextBaseline(core.BaselineMiddle).
					FillText(itoa(i*2), cxp+cos32(a)*(r-28), cy+sin32(a)*(r-28), sans(11, ui.Regular))
			}
		}
		// The sweep: a track, then a conic gradient arc ending at the
		// needle.
		sr := r + 12
		d.SetLineWidth(5).SetLineCap(core.CapRound).SetStrokeColor(c.Raised).BeginPath().Arc(cxp, cy, sr, float32(a0), float32(a1), false).Stroke()
		d.SetStrokeGradient(core.CanvasConic(float32(a0-0.2), cxp, cy,
			core.GradientStop{Offset: 0, Color: c.Info}, core.GradientStop{Offset: 0.45, Color: c.Accent},
			core.GradientStop{Offset: 0.75, Color: c.Warn}, core.GradientStop{Offset: 0.8, Color: c.Danger}))
		d.BeginPath().Arc(cxp, cy, sr, float32(a0), float32(a0), false).Bind(core.ArgA1, angle).Stroke()
		// The readout, drawn again only when the preset changes.
		d.SetTextAlign(core.TextAlignCenter).SetTextBaseline(core.BaselineAlphabetic).SetFillColor(c.Text).
			FillText(fmt.Sprint(load()), cxp, cy+r*0.62, sans(40, ui.Semibold)).
			SetFillColor(c.Faint).FillText(tr.CanvasLoad(), cxp, cy+r*0.62+18, sans(11, ui.Regular))
		// The needle, rotated on the platform.
		d.Save().Translate(cxp, cy).Rotate(0).Bind(core.ArgA, angle).SetShadow(shade, 0, 0, 6).SetFillColor(c.Text).
			BeginPath().MoveTo(-14, -3).LineTo(r-18, 0).LineTo(-14, 3).ClosePath().Fill().Restore()
		d.SetFillColor(c.Raised).SetStrokeColor(c.Text).SetLineWidth(2).BeginPath().Arc(cxp, cy, 7, 0, tau, false).Fill().Stroke()
	}
	var opts []ui.Option[string]
	for _, p := range dialPresets {
		opts = append(opts, ui.Option[string]{Value: p.value, Label: p.label(locales.For(cx))})
	}
	return Card{Title: tr.CanvasTurbine3Load(), Description: tr.CanvasNeedleAndSweepOnOne(), Children: []core.Node{
		core.Canvas{Style: core.Style{Height: core.Pt(230)}, Draw: draw, Label: tr.CanvasLoadDial(), TestID: "dial"},
		ui.Tabs[string]{Equal: true, Options: opts, Value: preset.Get, OnChange: choose},
		core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
			core.Dynamic(func(*core.Scope) core.Node {
				z := zone()
				name := map[Tone]string{ToneDanger: tr.CanvasRedline(), ToneWarn: tr.CanvasHigh(), ToneUp: tr.CanvasNominal()}[z]
				return Stat{Label: tr.CanvasZone(), Value: label(name), Tone: z, Width: core.Pct(33)}
			}),
			Stat{Label: tr.CanvasSpool(), Value: func() string { return thousands(int(load()*128)) + tr.CanvasRpm() }, Width: core.Pct(33)},
			Stat{Label: tr.CanvasLimit(), Value: label("85 %"), Tone: ToneDanger, Width: core.Pct(33)},
		}},
	}}
}

func freshData(rand func() float64) []int {
	out := make([]int, 14)
	for i := range out {
		s := math.Sin(float64(i) / 2.2)
		out[i] = int(math.Round(28 + 40*s*s + rand()*26))
	}
	return out
}

// areaChart is a gradient fill, a line, a dashed average, a hatched
// window and labels measured with MeasureText; new data sweeps in on the
// platform.
func areaChart(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	data := core.NewSignalFunc(cx, freshData(seeded(7)), func(a, b []int) bool { return false })
	sweep, replay := reveal(cx, 1100*time.Millisecond)
	last := func() int { d := data.Get(); return d[len(d)-1] }
	avg := func() int {
		sum := 0
		for _, v := range data.Get() {
			sum += v
		}
		return int(math.Round(float64(sum) / float64(len(data.Get()))))
	}
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		values := data.Get()
		w0, h0 := float32(size.Width), float32(size.Height)
		f := sans(11, ui.Regular)
		labelW := d.MeasureText("100", f)
		left, right, top, bottom := labelW+8, float32(4), float32(16), float32(20)
		w, h := w0-left-right, h0-top-bottom
		x := func(i int) float32 { return left + float32(i)/float32(len(values)-1)*w }
		y := func(v float64) float32 { return top + h - float32(v/100)*h }
		// Grid and labels.
		d.SetTextBaseline(core.BaselineMiddle)
		for _, v := range []float64{0, 50, 100} {
			dashed(d, left, y(v), w0-right, y(v), c.Line)
			l := fmt.Sprint(v)
			d.SetFillColor(c.Faint).FillText(l, left-8-d.MeasureText(l, f), y(v), f)
		}
		// The deploy window: hatched.
		wx, ww := x(9), x(11)-x(9)
		d.Save().BeginPath().Rect(wx, top, ww, h).Clip()
		hatchClip(d, wx, top, ww, h, ui.Alpha(c.Warn, 0.28), 6)
		d.Restore()
		d.SetFillColor(c.Warn).SetTextBaseline(core.BaselineAlphabetic).FillText("deploy", wx+2, top-4, f)
		// Day labels.
		d.SetFillColor(c.Faint)
		for _, i := range []int{0, 7, 13} {
			l := fmt.Sprintf("d-%d", 13-i)
			if i == 13 {
				l = "now"
			}
			lw := d.MeasureText(l, f)
			d.FillText(l, min(w0-right-lw, max(left, x(i)-lw/2)), h0-4, f)
		}
		// Everything the data draws sweeps in left to right (a clip the
		// platform animates).
		d.Save().BeginPath().Rect(left-6, 0, 0, h0).Bind(core.ArgW, core.Mul(sweep, core.Constant(w+12))).Clip()
		d.SetFillGradient(core.CanvasLinear(0, top, 0, top+h, core.GradientStop{Offset: 0, Color: ui.Alpha(c.Accent, 0.26)}, core.GradientStop{Offset: 1, Color: ui.Alpha(c.Accent, 0)}))
		d.BeginPath().MoveTo(x(0), y(float64(values[0])))
		for i, v := range values[1:] {
			d.LineTo(x(i+1), y(float64(v)))
		}
		d.LineTo(x(len(values)-1), top+h).LineTo(x(0), top+h).ClosePath().Fill()
		d.BeginPath()
		for i, v := range values {
			if i == 0 {
				d.MoveTo(x(i), y(float64(v)))
			} else {
				d.LineTo(x(i), y(float64(v)))
			}
		}
		d.SetStrokeColor(c.Accent).SetLineWidth(2).SetLineJoin(core.JoinRound).Stroke()
		for i, v := range values {
			r := float32(2.5)
			if i == len(values)-1 {
				r = 4.5
			}
			d.BeginPath().Arc(x(i), y(float64(v)), r, 0, tau, false).SetFillColor(c.Inset).Fill().SetStrokeColor(c.Accent).SetLineWidth(1.5).Stroke()
		}
		d.Restore()
		// The average, with a pill label measured to fit.
		mean := float64(avg())
		dashed(d, left, y(mean), w0-right, y(mean), c.Muted)
		l := tr.CanvasAvg(mean)
		lw := d.MeasureText(l, f) + 12
		d.SetFillColor(c.Raised).BeginPath().RoundRect(w0-right-lw, y(mean)-9, lw, 18, 9).Fill()
		d.SetFillColor(c.Text).SetTextBaseline(core.BaselineMiddle).FillText(l, w0-right-lw+6, y(mean), f)
	}
	return Card{Title: tr.CanvasRelayThroughput(), Description: tr.CanvasAreaAverageHatching(), Actions: []core.Node{ui.Button{Text: tr.CanvasNew(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: func() {
		data.Set(freshData(func() float64 { return float64(time.Now().UnixNano()%1000) / 1000 }))
		replay()
	}}}, Children: []core.Node{
		core.Dynamic(func(*core.Scope) core.Node {
			l, a := last(), avg()
			t, sign := ToneUp, "+"
			if l < a {
				t, sign = ToneDown, "−"
			}
			return Figure{Value: label(fmt.Sprintf("%d.%d", l, (l*7)%10)), Unit: "k/s", Note: label(tr.CanvasVsAverage(fmt.Sprintf("%s%d", sign, max(l-a, a-l)))), Tone: t}
		}),
		core.Canvas{Style: core.Style{Height: core.Pt(180)}, Draw: draw, Label: tr.CanvasThroughputChart()},
		core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{
			Legend{Color: colors.Accent, Label: tr.CanvasMessages()}, Legend{Color: colors.Muted, Label: tr.CanvasAverage()}, Legend{Color: colors.Warn, Label: tr.CanvasDeploy()},
		}},
	}}
}

var ridgeDays = []func(locales.Messages) string{locales.Messages.CanvasMon, locales.Messages.CanvasTue, locales.Messages.CanvasWed,
	locales.Messages.CanvasThu, locales.Messages.CanvasFri, locales.Messages.CanvasSat, locales.Messages.CanvasSun}

var ridges = func() [][]float64 {
	out := make([][]float64, len(ridgeDays))
	for d := range ridgeDays {
		rand := seeded(uint32(100 + d))
		peaks := []float64{6 + rand()*4, 12 + rand()*3, 17 + rand()*4}
		weights := []float64{0.5 + rand()*0.5, 0.4 + rand()*0.6, 0.3 + rand()*0.7}
		if d >= 5 {
			for k := range weights {
				weights[k] *= 0.55
			}
		}
		series := make([]float64, 25)
		for h := range series {
			s := 0.04
			for k, p := range peaks {
				s += weights[k] * math.Exp(-(float64(h)-p)*(float64(h)-p)/6)
			}
			series[h] = s
		}
		out[d] = series
	}
	return out
}()

// ridgeline is one smooth curve per day, overlapping, each a low-alpha
// fill.
func ridgeline(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	sweep, replay := reveal(cx, 1400*time.Millisecond)
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		width, height := float32(size.Width), float32(size.Height)
		f := sans(11, ui.Regular)
		left := d.MeasureText(tr.CanvasMon(), f) + 10
		const bottom = 18
		step := (height - bottom - 30) / float32(len(ridgeDays)-1)
		x := func(h float64) float32 { return left + float32(h/24)*(width-left) }
		// Peak window and hour labels.
		dashed(d, x(12), 0, x(12), height-bottom, c.Border)
		dashed(d, x(14), 0, x(14), height-bottom, c.Border)
		d.SetFillColor(c.Faint).SetTextBaseline(core.BaselineAlphabetic)
		for _, h := range []int{0, 6, 12, 18, 24} {
			l := fmt.Sprintf("%02d", h)
			lw := d.MeasureText(l, f)
			d.FillText(l, min(width-lw, max(left, x(float64(h))-lw/2)), height-3, f)
		}
		d.Save().BeginPath().Rect(0, 0, 0, height).Bind(core.ArgW, core.Mul(sweep, core.Constant(width))).Clip()
		for di, series := range ridges {
			base := 30 + float32(di)*step
			y := func(v float64) float32 { return base - float32(v)*46 }
			d.BeginPath().MoveTo(x(0), base).LineTo(x(0), y(series[0]))
			for h := 1; h < len(series); h++ {
				mx := (x(float64(h-1)) + x(float64(h))) / 2
				d.QuadTo(x(float64(h-1)), y(series[h-1]), mx, (y(series[h-1])+y(series[h]))/2)
			}
			d.LineTo(x(24), y(series[24])).LineTo(x(24), base).ClosePath()
			// Occlude the ridge behind, then a low-alpha fill and the line.
			d.SetFillColor(c.Inset).Fill()
			fillC, line := ui.Alpha(c.Accent, 0.1+float32(di)*0.025), c.Accent
			if di >= 5 {
				fillC, line = ui.Alpha(c.Info, 0.18), c.Info
			}
			d.SetFillColor(fillC).Fill().SetStrokeColor(line).SetLineWidth(1.25).Stroke()
		}
		d.Restore()
		d.SetFillColor(c.Muted)
		for di, day := range ridgeDays {
			d.FillText(day(tr), 0, 30+float32(di)*step-2, f)
		}
	}
	return Card{Title: tr.CanvasFootfallByHour(), Description: tr.CanvasRidgelineOneCurveADay(), Actions: []core.Node{ui.Button{Text: tr.CanvasReplay(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: replay}}, Children: []core.Node{
		Caption{Label: tr.CanvasGateBLastWeek(), Note: label(tr.CanvasPeak12001400())},
		core.Canvas{Style: core.Style{Height: core.Pt(210)}, Draw: draw, Label: tr.CanvasFootfallRidgeline()},
		core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{Legend{Color: colors.Accent, Label: tr.CanvasWeekday()}, Legend{Color: colors.Info, Label: tr.CanvasWeekend()}}},
	}}
}

// levels are the dot levels, from none to full, read where ui.ColorsOf's
// colors are.
func levels(cx *core.Scope) []core.Color {
	c := ui.ColorsOf(cx).Now()
	return []core.Color{c.Raised, ui.Alpha(c.Accent, 0.22), ui.Alpha(c.Accent, 0.45), ui.Alpha(c.Accent, 0.7), c.Accent}
}

// dotMatrix is uptime per hour block, a pulse on the newest dot (a loop
// on the platform).
func dotMatrix(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	pulse := core.NewAnimatedValue(cx, 0)
	run := cx.StartAnimation(core.Loop(core.Timing(pulse, 1, core.TimingConfig{Duration: ms(1400), Easing: ease(core.EasingQuad, core.EaseModeOut)}), 0), nil)
	cx.Cleanup(run.Stop)
	draw := func(d *core.Drawing, size core.Size) {
		c, lv := colors.Now(), levels(cx)
		width, height := float32(size.Width), float32(size.Height)
		const rows, gap = 7, 12.5
		cols := int(width / gap)
		ox, oy := (width-float32(cols-1)*gap)/2, (height-float32(rows-1)*gap)/2
		rand := seeded(42)
		var lastX, lastY float32
		for col := range cols {
			for r := range rows {
				n := rand()
				incident := (col == 6 && r == 2) || (col == 17 && r == 5) || (col == cols-4 && r == 3)
				fillC := lv[min(4, int(math.Floor(n*n*5+float64(col)/float64(cols)*1.6)))]
				if incident {
					fillC = c.Danger
				}
				lastX, lastY = ox+float32(col)*gap, oy+float32(r)*gap
				d.SetFillColor(fillC).BeginPath().Arc(lastX, lastY, 3.6, 0, tau, false).Fill()
			}
		}
		// The pulse: radius and alpha follow the loop.
		d.Save().SetAlpha(0.9).Bind(core.ArgA, ui.Lerp(pulse, 0.9, 0)).SetStrokeColor(c.Accent).SetLineWidth(1.5).
			BeginPath().Arc(lastX, lastY, 4, 0, tau, false).Bind(core.ArgR, ui.Lerp(pulse, 4, 11)).Stroke().Restore()
	}
	var swatches []core.Node
	for i := range len(levels(cx)) {
		swatches = append(swatches, core.View{Style: core.Style{Width: core.Pt(9), Height: core.Pt(9), CornerRadius: 2.5},
			StyleFunc: ui.Filled(func() core.Color { return levels(cx)[i] })})
	}
	return Card{Title: tr.CanvasBeaconUptime(), Description: tr.CanvasDotMatrixOneColumnA(), Children: []core.Node{
		Figure{Value: label("99.94"), Unit: "%", Note: label(tr.Canvas3Incidents()), Tone: ToneDanger},
		core.Canvas{Style: core.Style{Height: core.Pt(96)}, Draw: draw, Label: tr.CanvasUptimeMatrix()},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: append(append([]core.Node{ui.Text(tr.CanvasLess(), 13, ui.Regular, colors.Muted)}, swatches...),
			core.Text{Content: tr.CanvasMore(), Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)}, Legend{Color: colors.Danger, Label: tr.CanvasIncident()})},
	}}
}

var routesLatency = []struct {
	name          string
	before, after float64
}{{"North Gate", 182, 96}, {"Harbor", 240, 131}, {"Old Mill", 128, 104}, {"Ridge", 205, 88}, {"Lowfield", 96, 71}, {"Quarry", 151, 160}}

// dumbbell is before and after per row; the after dots travel on the
// platform, one value staggered through each row's input range.
func dumbbell(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	t, replay := reveal(cx, 1500*time.Millisecond)
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		width, height := float32(size.Width), float32(size.Height)
		f13, f11 := sans(13, ui.Regular), sans(11, ui.Regular)
		var left float32
		for _, r := range routesLatency {
			left = max(left, d.MeasureText(r.name, f13))
		}
		left += 12
		right := d.MeasureText("+00%", f11) + 10
		const most = 250
		x := func(v float64) float32 { return left + float32(v/most)*(width-left-right) }
		d.SetTextBaseline(core.BaselineAlphabetic)
		for _, v := range []float64{0, 100, 200} {
			dashed(d, x(v), 0, x(v), height-18, c.Line)
			l := fmt.Sprint(v)
			d.SetFillColor(c.Faint).FillText(l, x(v)-d.MeasureText(l, f11)/2, height-3, f11)
		}
		for i, r := range routesLatency {
			cy := 15 + float32(i)*30
			start := float64(i) * 0.07
			after := core.Interpolate(t, core.Interpolation{Input: []float64{start, start + 0.6}, Output: []float64{float64(x(r.before)), float64(x(r.after))}, Extrapolate: core.ExtrapolateClamp})
			worse := r.after > r.before
			tn, bar := c.Accent, ui.Alpha(c.Accent, 0.45)
			if worse {
				tn, bar = c.Danger, ui.Alpha(c.Danger, 0.5)
			}
			d.SetFillColor(c.Muted).SetTextBaseline(core.BaselineMiddle).FillText(r.name, 0, cy, f13)
			d.SetStrokeColor(bar).SetLineWidth(3).SetLineCap(core.CapRound).BeginPath().MoveTo(x(r.before), cy).LineTo(x(r.before), cy).Bind(core.ArgX, after).Stroke()
			d.SetFillColor(c.Inset).SetStrokeColor(c.Muted).SetLineWidth(1.5).BeginPath().Arc(x(r.before), cy, 4.5, 0, tau, false).Fill().Stroke()
			d.SetFillColor(tn).BeginPath().Arc(x(r.before), cy, 5.5, 0, tau, false).Bind(core.ArgX, after).Fill()
			pct := int(math.Round((r.after - r.before) / r.before * 100))
			l := fmt.Sprintf("%d%%", pct)
			if pct > 0 {
				l = "+" + l
			}
			d.SetFillColor(tn).FillText(l, width-d.MeasureText(l, f11), cy, f11)
		}
	}
	return Card{Title: tr.CanvasRouteLatency(), Description: tr.CanvasDumbbellBeforeAfter(), Actions: []core.Node{ui.Button{Text: tr.CanvasReplay(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: replay}}, Children: []core.Node{
		core.Canvas{Style: core.Style{Height: core.Pt(core.Points(30*len(routesLatency) + 24))}, Draw: draw, Label: tr.CanvasRouteLatency()},
		core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{
			Legend{Color: colors.Muted, Label: tr.CanvasBefore()}, Legend{Color: colors.Accent, Label: tr.CanvasAfter()}, Legend{Color: colors.Danger, Label: tr.CanvasRegressed()},
		}},
	}}
}

// clock is transforms, save and restore, and a second hand the platform
// turns.
func clock(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	seconds := core.NewAnimatedValue(cx, 0)
	run := cx.StartAnimation(core.Loop(core.Timing(seconds, 60, core.TimingConfig{Duration: time.Minute}), 0), nil)
	cx.Cleanup(run.Stop)
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		r := float32(size.Width) / 2
		d.Translate(r, r)
		d.SetFillColor(c.Surface).SetStrokeColor(c.Border).SetLineWidth(1.5).BeginPath().Arc(0, 0, r-2, 0, tau, false).Fill().Stroke()
		for i := range 60 {
			col, lw, l := c.Faint, float32(1), float32(12)
			if i%5 == 0 {
				col, lw, l = c.Muted, 2, 17
			}
			d.Save().Rotate(float32(i)/60*tau).SetStrokeColor(col).SetLineWidth(lw).BeginPath().MoveTo(r-8, 0).LineTo(r-l, 0).Stroke().Restore()
		}
		hand := func(angle float64, length, w float32) {
			d.Save().Rotate(float32(angle-math.Pi/2)).SetStrokeColor(c.Text).SetLineWidth(w).SetLineCap(core.CapRound).BeginPath().MoveTo(-8, 0).LineTo(length, 0).Stroke().Restore()
		}
		hand(10.0/12*tau, r*0.48, 4.5)
		hand(10.0/60*tau, r*0.7, 3)
		d.Save().Rotate(0).Bind(core.ArgA, core.Mul(seconds, core.Constant(tau/60))).Rotate(-math.Pi/2).
			SetStrokeColor(c.Accent).SetLineWidth(1.5).BeginPath().MoveTo(-14, 0).LineTo(r*0.8, 0).Stroke().Restore()
		d.SetFillColor(c.Accent).BeginPath().Arc(0, 0, 4, 0, tau, false).Fill()
	}
	return Card{Title: tr.CanvasTransforms(), Description: tr.CanvasTranslateRotateSaveRestore(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16}, Children: []core.Node{
			core.Canvas{Style: core.Style{Width: core.Pt(150), Height: core.Pt(150)}, Draw: draw, Label: tr.CanvasClock()},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 8}, Children: []core.Node{
				Caption{Label: tr.CanvasTicks(), Note: label(tr.Canvas60Rotations())}, Caption{Label: tr.CanvasHands(), Note: label(tr.CanvasSaveRestore())}, Caption{Label: tr.CanvasSecondHand(), Note: label(tr.CanvasALoop())},
			}},
		}},
	}}
}

var pathGlyphs = []struct {
	name   func(locales.Messages) string
	d      string
	tone   Tone
	stroke bool
}{
	{locales.Messages.CanvasHeart, "M12 21s-7-4.35-9.5-8.5C.5 8.5 3 4 7 4c2 0 3.5 1 5 3 1.5-2 3-3 5-3 4 0 6.5 4.5 4.5 8.5C19 16.65 12 21 12 21z", ToneDanger, false},
	{locales.Messages.CanvasStar, "M12 2l3.09 6.26L22 9.27l-5 4.87L18.18 21 12 17.27 5.82 21 7 14.14l-5-4.87 6.91-1.01z", ToneWarn, false},
	{locales.Messages.CanvasBolt, "M13 2L3 14h7l-1 8 10-12h-7z", ToneInfo, false},
	{locales.Messages.CanvasCheck, "M20 6L9 17l-5-5", ToneAccent, true},
}

// paths are SVG path data, parsed by core.
func paths(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	var tiles []core.Node
	for _, g := range pathGlyphs {
		p := core.MustParsePath(g.d)
		tiles = append(tiles, core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return fin.Raised().Over(core.Style{Width: core.Pt(60), Height: core.Pt(60), CornerRadius: 18, BorderCurve: core.BorderCurveContinuous, Background: colors.Raised(),
					BorderWidth: 1, BorderColor: colors.Border(), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter})
			}, Children: []core.Node{
				core.Canvas{Style: core.Style{Width: core.Pt(30), Height: core.Pt(30)}, Label: g.name(locales.For(cx)), Draw: func(d *core.Drawing, _ core.Size) {
					c := g.tone.Color(cx)
					d.Scale(30.0/24, 30.0/24)
					if g.stroke {
						d.SetStrokeColor(c).SetLineWidth(2.5).SetLineCap(core.CapRound).SetLineJoin(core.JoinRound).StrokePath(p)
					} else {
						d.SetFillColor(c).FillPath(p, core.NonZero)
					}
				}},
			}},
			ui.Text(g.name(locales.For(cx)), 13, ui.Regular, colors.Muted),
		}})
	}
	return Card{Title: tr.CanvasPaths(), Description: tr.CanvasSvgPathDataParsedBy(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: tiles},
	}}
}

// compositing is clipping, shadows, DestinationOut, dashes and radial
// gradients.
func compositing(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	img := core.ImageSource{Data: logoPNG}
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		shade := ui.Black(0.25)
		if cx.Dark() {
			shade = ui.Black(0.7)
		}
		width, height := float32(size.Width), float32(size.Height)
		cy := height / 2
		// An image clipped to a circle, with a ring.
		d.Save().BeginPath().Arc(46, cy, 42, 0, tau, false).Clip().SetFillColor(c.Raised).FillRect(0, 0, 92, height).DrawImageIn(img, 10, cy-36, 72, 72).Restore()
		d.SetStrokeColor(c.Border).SetLineWidth(1.5).BeginPath().Arc(46, cy, 42, 0, tau, false).Stroke()
		// A shadowed card with a hole punched through.
		mid := width / 2
		d.Save().SetShadow(shade, 0, 6, 14).
			SetFillGradient(core.CanvasLinear(mid-46, 0, mid+46, height, core.GradientStop{Offset: 0, Color: c.AccentFill}, core.GradientStop{Offset: 1, Color: core.RGB(0xb84a15)})).
			BeginPath().RoundRect(mid-46, cy-40, 92, 80, 16).Fill().Restore()
		d.SetComposite(core.CompositeDestinationOut).BeginPath().Arc(mid, cy, 18, 0, tau, false).Fill().SetComposite(core.CompositeSourceOver)
		// A radial gradient with a dashed outline.
		rx := width - 46
		d.SetFillGradient(core.CanvasRadial(rx-10, cy-12, 4, rx, cy, 40,
			core.GradientStop{Offset: 0, Color: core.RGB(0xffe2a8)}, core.GradientStop{Offset: 0.55, Color: c.Warn}, core.GradientStop{Offset: 1, Color: ui.Alpha(c.Danger, 0.2)})).
			BeginPath().Arc(rx, cy, 38, 0, tau, false).Fill()
		d.SetLineDash(5, 5).SetStrokeColor(c.Warn).SetLineWidth(1.5).BeginPath().Arc(rx, cy, 43, 0, tau, false).Stroke()
	}
	return Card{Title: tr.CanvasCompositing(), Description: tr.CanvasClipShadowDestinationoutDashes(), Children: []core.Node{
		core.Canvas{Style: core.Style{Height: core.Pt(120)}, Draw: draw, Label: tr.CanvasCompositing()},
		core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			ui.Text(tr.CanvasClippedImage(), 13, ui.Regular, colors.Muted), ui.Text(tr.CanvasCutOut(), 13, ui.Regular, colors.Muted), ui.Text(tr.CanvasRadialGradient(), 13, ui.Regular, colors.Muted),
		}},
	}}
}

// sketch draws again from the engine on each move.
func sketch(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	strokes := core.NewSignalFunc(cx, [][]core.Point(nil), func(a, b [][]core.Point) bool { return false })
	draw := func(d *core.Drawing, size core.Size) {
		c := colors.Now()
		width, height := float32(size.Width), float32(size.Height)
		// A dot grid to draw on.
		d.SetFillColor(c.Border)
		for x := float32(8); x < width; x += 16 {
			for y := float32(8); y < height; y += 16 {
				d.FillRect(x-0.75, y-0.75, 1.5, 1.5)
			}
		}
		all := strokes.Get()
		if len(all) == 0 {
			d.SetFillColor(c.Faint).SetTextAlign(core.TextAlignCenter).SetTextBaseline(core.BaselineMiddle).FillText(tr.CanvasDrawHere(), width/2, height/2, sans(14, ui.Regular))
		}
		d.SetStrokeColor(c.Accent).SetLineWidth(3).SetLineCap(core.CapRound).SetLineJoin(core.JoinRound)
		for _, s := range all {
			d.BeginPath()
			for i, p := range s {
				if i == 0 {
					d.MoveTo(float32(p.X), float32(p.Y))
				} else {
					d.LineTo(float32(p.X), float32(p.Y))
				}
			}
			d.Stroke()
		}
	}
	return Card{Title: tr.CanvasSketch(), Description: Hint(cx, tr.CanvasTouchDrawFromTheEngine(), tr.CanvasDragDrawFromTheEngine()), Actions: []core.Node{ui.Button{Text: tr.CanvasClear(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, Disabled: func() bool { return len(strokes.Get()) == 0 }, OnPress: func() { strokes.Set(nil) }}}, Children: []core.Node{
		core.View{TestID: "sketch", Responder: &core.Responder{Claims: []core.TouchClaim{{}}, Keep: true,
			OnGrant: func(e core.ResponderEvent) {
				strokes.Update(func(s [][]core.Point) [][]core.Point { return append(s, []core.Point{e.Position}) })
			},
			OnMove: func(e core.ResponderEvent) {
				strokes.Update(func(s [][]core.Point) [][]core.Point {
					if len(s) == 0 {
						return s
					}
					s[len(s)-1] = append(s[len(s)-1], e.Position)
					return s
				})
			},
		}, Children: []core.Node{core.Canvas{Style: core.Style{Height: core.Pt(200)}, Draw: draw, Label: tr.CanvasSketchPad()}}},
		Caption{Label: tr.CanvasStrokes(), Note: func() string {
			n := 0
			for _, s := range strokes.Get() {
				n += len(s)
			}
			return tr.CanvasStrokesPoints(len(strokes.Get()), n)
		}},
	}}
}

// hitTests names each bar of a chart for assistive technology and tests,
// and selects the bar a press lands in.
func hitTests(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	values := []float32{0.5, 0.8, 0.35, 0.65}
	selected := core.NewSignal(cx, -1)
	var last *core.Drawing
	shapes := make([]*core.Path, len(values))
	lineC, accentC := colors.Raised, colors.Accent
	draw := func(d *core.Drawing, size core.Size) {
		w, h := float32(size.Width), float32(size.Height)
		slot := w / float32(len(values))
		for i, v := range values {
			x, bh := float32(i)*slot+slot*0.15, v*(h-24)
			shapes[i] = new(core.Path).RoundRect(x, h-12-bh, slot*0.7, bh, 6)
			fillC := lineC
			if selected.Get() == i {
				fillC = accentC
			}
			d.SetFillColor(fillC()).FillPath(shapes[i], core.NonZero)
			d.Node(core.CanvasNode{Bounds: core.Bounds{Origin: core.Point{X: core.Points(x), Y: core.Points(h - 12 - bh)}, Size: core.Size{Width: core.Points(slot * 0.7), Height: core.Points(bh)}},
				Role: core.RoleButton, Label: tr.CanvasBarValue(i+1, float64(v)), TestID: fmt.Sprintf("bar-%d", i+1), Selected: selected.Get() == i})
		}
		last = d
	}
	pressed := func(e core.PressEvent) {
		for i, p := range shapes {
			if last != nil && p != nil && last.PathContains(p, float32(e.Position.X), float32(e.Position.Y), core.NonZero) {
				selected.Set(i)
				return
			}
		}
		selected.Set(-1)
	}
	return Card{Title: tr.CanvasNodesAndHitTests(), Description: tr.CanvasBarsScreenReadersSeeA(), Actions: []core.Node{ui.Badge{Text: "NSGo"}}, Children: []core.Node{
		core.Canvas{Style: core.Style{Height: core.Pt(140)}, Draw: draw, OnPress: pressed, Label: tr.CanvasSelectableBars(), TestID: "bars"},
		Caption{Label: tr.CanvasSelected(), Note: func() string {
			if s := selected.Get(); s >= 0 {
				return tr.CanvasBar(s + 1)
			}
			return "none"
		}},
	}}
}

// canvasDemo is core.Canvas: a 2D context recorded on the engine and
// played by the platform, its arguments bound to animated values.
func canvasDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{columns: 3}, dial, areaChart, ridgeline, dotMatrix, dumbbell, clock, paths, compositing, sketch, hitTests)
}
