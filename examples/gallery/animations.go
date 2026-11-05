package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// motionTones are the tones of the demos' dots and tiles.
func motionTones(cx *core.Scope) []func() core.Color {
	colors := ui.ColorsOf(cx)
	return []func() core.Color{colors.Accent, colors.Info, colors.Warn, colors.Danger, colors.Up}
}

// onTone is text on a tone's fill.
func onTone(cx *core.Scope) func() core.Color {
	return func() core.Color {
		if cx.Dark() {
			return ui.ColorsOf(cx).OnAccent()
		}
		return core.RGB(0xffffff)
	}
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func ease(kind core.EasingKind, mode core.EaseMode) core.Easing {
	return core.Easing{Kind: kind, Mode: mode}
}

// hero is an orbit, pulsing rings and a breathing core: three loops,
// started once.
func hero(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	spin := core.NewAnimatedValue(cx, 0)
	rings := []*core.AnimatedValue{core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)}
	breathe := core.NewAnimatedValue(cx, 0)
	var ringLoops []*core.Animation
	for _, r := range rings {
		ringLoops = append(ringLoops, core.Loop(core.Timing(r, 1, core.TimingConfig{Duration: ms(2400), Easing: ui.EaseOutCubic}), 0))
	}
	sine := ease(core.EasingSin, core.EaseModeInOut)
	run := cx.StartAnimation(core.Parallel(
		core.Loop(core.Timing(spin, 360, core.TimingConfig{Duration: ms(3200)}), 0),
		core.Stagger(ms(800), ringLoops...),
		core.Loop(core.Sequence(
			core.Timing(breathe, 1, core.TimingConfig{Duration: ms(1300), Easing: sine}),
			core.Timing(breathe, 0, core.TimingConfig{Duration: ms(1300), Easing: sine}),
		), 0),
	), nil)
	cx.Cleanup(run.Stop)

	const s core.Points = 188
	orbit := func(size core.Points, angle core.AnimatedNode, c func() core.Color, dot core.Points) core.Node {
		return core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
			return core.Style{Position: core.Absolute, Left: core.Pt((s - size) / 2), Top: core.Pt((s - size) / 2),
				Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2, BorderWidth: 1, BorderColor: colors.Line(), Animated: &core.AnimatedStyle{Rotate: angle}}
		},
			Children: []core.Node{core.View{StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Left: core.Pt(size/2 - dot/2), Top: core.Pt(-dot / 2), Width: core.Pt(dot), Height: core.Pt(dot),
					CornerRadius: dot / 2, Background: c(), Shadows: fin.Knob().Shadows}
			}}}}
	}
	var stage []core.Node
	for _, r := range rings {
		stage = append(stage, core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
			return core.Style{Position: core.Absolute, Left: core.Pt(34), Top: core.Pt(34),
				Width: core.Pt(s - 68), Height: core.Pt(s - 68), CornerRadius: (s - 68) / 2, BorderWidth: 1.5, BorderColor: colors.Accent(),
				Animated: &core.AnimatedStyle{
					Opacity: core.Interpolate(r, core.Interpolation{Input: []float64{0, 0.15, 1}, Output: []float64{0, 0.6, 0}}),
					Scale:   ui.Lerp(r, 0.35, 1.45),
				}}
		}})
	}
	stage = append(stage,
		orbit(s-12, spin, colors.Accent, 10),
		orbit(s-64, core.Mul(spin, core.Constant(-1.6)), colors.Info, 8),
		core.View{StyleFunc: func() core.Style {
			return core.Style{Position: core.Absolute, Left: core.Pt(s/2 - 26), Top: core.Pt(s/2 - 26), Width: core.Pt(52), Height: core.Pt(52), CornerRadius: 26,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: colors.AccentFill(), Shadows: fin.Accent().Shadows,
				Animated: &core.AnimatedStyle{
					Background: core.Interpolate(breathe, core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{colors.AccentFill(), colors.Up()}}),
					Scale:      ui.Lerp(breathe, 0.9, 1.08),
				}}
		}, Children: []core.Node{ui.Icon(lucide.Zap, 22, colors.OnAccent)}},
	)
	return ui.Card{Title: tr.AnimNativeMotion(), Description: tr.AnimThreeLoopsStartedOnceRun(), Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(s + 8), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(s), Height: core.Pt(s)}, Children: stage},
		}},
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			APILine{Label: tr.AnimOrbitsOneTurnIn3(), Names: "Loop"},
			APILine{Label: tr.AnimRings08SApart(), Names: "Stagger"},
			APILine{Label: tr.AnimCoreInAndOut(), Names: "Sequence"},
		}},
	}}
}

// easingCurve is a curve to animate with and its samples to draw it,
// from the platforms' own easing code.
type easingCurve struct {
	name   string
	easing core.Easing
}

var easingCurves = []easingCurve{
	{"linear", core.Easing{}},
	{"ease", core.BezierEasing(core.Ease)},
	{"quad out", ease(core.EasingQuad, core.EaseModeOut)},
	{"cubic inOut", ease(core.EasingCubic, core.EaseModeInOut)},
	{"sin in", ease(core.EasingSin, core.EaseModeIn)},
	{"circle out", ease(core.EasingCircle, core.EaseModeOut)},
	{"exp inOut", ease(core.EasingExp, core.EaseModeInOut)},
	{"back out", ease(core.EasingBack, core.EaseModeOut)},
	{"elastic out", core.Easing{Kind: core.EasingElastic, Mode: core.EaseModeOut, Param: 1}},
	{"bounce out", ease(core.EasingBounce, core.EaseModeOut)},
}

// hatch is diagonal hatching inside a rectangle.
func hatch(d *core.Drawing, c core.Color, x, y, w, h float32) {
	d.Save().BeginPath().Rect(x, y, w, h).Clip()
	d.SetStrokeColor(c).SetLineWidth(1).BeginPath()
	for i := -h; i < w; i += 7 {
		d.MoveTo(x+i, y+h).LineTo(x+i+h, y)
	}
	d.Stroke().Restore()
}

// chipTab is a selectable capsule, a tab of the curves (36 pt, with hit
// slop up to 44).
func chipTab(cx *core.Scope, label string, active func() bool, onPress func()) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	return base.Tab{Selected: active, OnPress: func(core.PressEvent) { onPress() }, Label: label,
		Style: core.Style{Margin: core.All(-4), Padding: core.All(4), Transition: core.Transition{Duration: ms(150), Easing: core.EaseOut, Properties: core.TransitionTransform},
			Active: &core.Style{Transform: core.Transform{Scale: 0.97}}},
		Children: []core.Node{core.View{StyleFunc: func() core.Style {
			s := core.Style{Height: core.Pt(36), Padding: core.Symmetric(0, 14), CornerRadius: 18, JustifyContent: core.JustifyCenter}
			if active() {
				s = fin.Light().Over(s)
				s.Background = colors.Light()
			} else {
				s = fin.Raised().Over(s)
				s.Background, s.BorderWidth, s.BorderColor = colors.Raised(), 1, colors.Border()
			}
			return s
		}, Children: []core.Node{core.Text{Content: label, StyleFunc: func() core.Style {
			c := colors.Text
			if active() {
				c = colors.OnLight
			}
			return core.Style{FontSize: 13, FontWeight: ui.Medium, Color: c()}
		}}}}}}
}

// easingPlot is one curve plotted, with a dot that rides it: x runs
// linearly, y with the curve, both on the platform.
func easingPlot(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	selected := core.NewSignal(cx, len(easingCurves)-3)
	px, py := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	var running *core.AnimationRun
	play := func() {
		if running != nil {
			running.Stop()
		}
		px.Set(0)
		py.Set(0)
		c := easingCurves[selected.GetUntracked()]
		running = cx.StartAnimation(core.Sequence(core.Delay(ms(120)), core.Parallel(
			core.Timing(px, 1, core.TimingConfig{Duration: ms(1100)}),
			core.Timing(py, 1, core.TimingConfig{Duration: ms(1100), Easing: c.easing}),
		)), nil)
	}
	const lo, hi = -0.18, 1.24
	textC, lineC, faintC, tint, accentC, lightC := colors.Text, colors.Line, colors.Faint, tints.Accent, colors.Accent, colors.Light
	draw := func(d *core.Drawing, size core.Size) {
		w, h := float32(size.Width), float32(size.Height)
		l, r, t, b := float32(30), w-12, float32(8), h-8
		y := func(v float64) float32 { return b - float32((v-lo)/(hi-lo))*(b-t) }
		x := func(v float64) float32 { return l + float32(v)*(r-l) }
		// Zones outside 0 to 1: where a curve overshoots.
		hatch(d, ui.Alpha(textC(), 0.06), l, t, r-l, y(1)-t)
		hatch(d, ui.Alpha(textC(), 0.06), l, y(0), r-l, b-y(0))
		d.SetStrokeColor(lineC()).SetLineWidth(1)
		for _, q := range []float64{0.25, 0.5, 0.75} {
			d.BeginPath().MoveTo(x(q), t).LineTo(x(q), b).Stroke()
		}
		d.SetLineDash(3, 4).SetStrokeColor(faintC())
		for _, v := range []float64{0, 1} {
			d.BeginPath().MoveTo(l, y(v)).LineTo(r, y(v)).Stroke()
		}
		d.SetLineDash()
		font := core.Font{Size: 11}
		d.SetFillColor(faintC()).SetTextBaseline(core.BaselineMiddle).FillText("1.0", 2, y(1), font).FillText("0", 2, y(0), font)
		// The curve, filled faintly beneath.
		e := easingCurves[selected.Get()].easing
		const n = 96
		d.BeginPath().MoveTo(x(0), y(0))
		for i := 0; i <= n; i++ {
			p := float64(i) / n
			d.LineTo(x(p), y(e.At(p)))
		}
		d.LineTo(x(1), y(0)).ClosePath().SetFillColor(tint()).SetAlpha(0.6).Fill().SetAlpha(1)
		d.BeginPath().MoveTo(x(0), y(e.At(0)))
		for i := 1; i <= n; i++ {
			p := float64(i) / n
			d.LineTo(x(p), y(e.At(p)))
		}
		d.SetStrokeColor(accentC()).SetLineWidth(2).SetLineJoin(core.JoinRound).Stroke()
		// The playhead and the dot, from animated values.
		hx := core.Add(core.Mul(px, core.Constant(r-l)), core.Constant(l))
		hy := core.Add(core.Mul(py, core.Constant(-(b-t)/(hi-lo))), core.Constant(b+lo*(b-t)/(hi-lo)))
		d.SetStrokeColor(ui.Alpha(textC(), 0.22)).SetLineWidth(1)
		d.BeginPath().MoveTo(l, t).Bind(core.ArgX, hx).LineTo(l, b).Bind(core.ArgX, hx).Stroke()
		d.SetLineDash(2, 3).BeginPath().MoveTo(l, b).Bind(core.ArgY, hy).LineTo(l, b).Bind(core.ArgX, hx).Bind(core.ArgY, hy).Stroke().SetLineDash()
		d.SetFillColor(lightC()).BeginPath().Arc(l, b, 5, 0, 2*math.Pi, false).Bind(core.ArgX, hx).Bind(core.ArgY, hy).Fill()
		d.SetStrokeColor(accentC()).BeginPath().Arc(l, b, 9, 0, 2*math.Pi, false).Bind(core.ArgX, hx).Bind(core.ArgY, hy).Stroke()
		d.SetFillColor(accentC()).BeginPath().Arc(l, b, 3.5, 0, 2*math.Pi, false).Bind(core.ArgY, hy).Fill()
	}
	var chips []core.Node
	for i, c := range easingCurves {
		chips = append(chips, chipTab(cx, c.name, func() bool { return selected.Get() == i }, func() { selected.Set(i); play() }))
	}
	return ui.Card{Title: tr.AnimEasing(), Description: tr.AnimCurvesAreDataEvaluatedNatively(), Children: []core.Node{
		core.Canvas{Style: core.Style{Height: core.Pt(196)}, Draw: draw, Label: tr.AnimEasingCurve()},
		core.Scroll{Horizontal: true, HideIndicator: true, Children: []core.Node{
			base.Tabs{Label: tr.AnimCurves(), Style: core.Style{Direction: core.Row, Gap: 8, Padding: core.Symmetric(4, 4)}, Children: chips}}},
		ui.Button{Text: tr.AnimPlay(), Icon: lucide.Play, OnPress: play},
	}}
}

// easingRace is every curve at once: ten timings in one Parallel.
func easingRace(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	var dots []*core.AnimatedValue
	for range easingCurves {
		dots = append(dots, core.NewAnimatedValue(cx, 0))
	}
	width := core.NewSignal(cx, core.Points(0))
	there := core.NewSignal(cx, false)
	play := func() {
		toggled(there)
		to := 0.0
		if there.GetUntracked() {
			to = 1
		}
		var steps []*core.Animation
		for i, d := range dots {
			steps = append(steps, core.Timing(d, to, core.TimingConfig{Duration: ms(1200), Easing: easingCurves[i].easing}))
		}
		cx.StartAnimation(core.Parallel(steps...), nil)
	}
	tones := motionTones(cx)
	var rows []core.Node
	for i, c := range easingCurves {
		lane := core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Height: core.Pt(12), JustifyContent: core.JustifyCenter}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(2), CornerRadius: 1}, StyleFunc: ui.Filled(colors.Raised)},
			core.View{StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Width: core.Pt(12), Height: core.Pt(12), CornerRadius: 6,
					Background: tones[i%len(tones)](), Shadows: fin.Knob().Shadows,
					Animated: &core.AnimatedStyle{TranslateX: core.Mul(dots[i], core.Constant(max(0, width.Get()-12)))}}
			}},
		}}
		if i == 0 {
			lane.OnLayout = func(b core.Bounds) { width.Set(b.Size.Width) }
		}
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(26), Gap: 10}, Children: []core.Node{
			core.Text{Content: c.name, MaxLines: 1, Style: core.Style{Width: core.Pt(84), FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			lane,
		}})
	}
	return ui.Card{Title: tr.AnimSideBySide(), Description: tr.AnimTenTimingsStartedTogether(), Actions: []core.Node{ui.Badge{Text: "1.2 s"}}, Children: []core.Node{
		core.View{Style: core.Style{Gap: 2}, Children: rows},
		APILine{Label: tr.AnimOneGroupOneStart(), Names: "Parallel"},
		core.Dynamic(func(*core.Scope) core.Node {
			l := tr.AnimPlay()
			if there.Get() {
				l = tr.AnimReverse()
			}
			return ui.Button{Text: l, Variant: ui.ButtonSecondary, Icon: lucide.Play, OnPress: play}
		}),
	}}
}

// composition is Sequence, Parallel, Stagger and Loop, with stop and
// reset.
func composition(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	tones := motionTones(cx)
	ys := make([]*core.AnimatedValue, len(tones))
	for i := range ys {
		ys[i] = core.NewAnimatedValue(cx, 0)
	}
	spin, fade := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 1)
	type state struct {
		label string
		tone  ui.Tone
	}
	st := core.NewSignal(cx, state{"idle", ui.ToneNeutral})
	var bounces []*core.Animation
	for _, y := range ys {
		bounces = append(bounces, core.Sequence(
			core.Timing(y, -46, core.TimingConfig{Duration: ms(220), Easing: ease(core.EasingQuad, core.EaseModeOut)}),
			core.Spring(y, 0, core.SpringConfig{Stiffness: 320, Damping: 11}),
		))
	}
	stagger := core.Stagger(ms(70), bounces...)
	sequence := core.Sequence(
		core.Timing(spin, 1, core.TimingConfig{Duration: ms(520), Easing: ease(core.EasingCubic, core.EaseModeInOut)}),
		core.Delay(ms(200)),
		core.Parallel(
			core.Timing(fade, 0.25, core.TimingConfig{Duration: ms(300)}),
			core.Timing(spin, 0, core.TimingConfig{Duration: ms(640), Easing: ease(core.EasingBack, core.EaseModeOut)}),
		),
		core.Timing(fade, 1, core.TimingConfig{Duration: ms(300)}),
	)
	sine := ease(core.EasingSin, core.EaseModeInOut)
	pulse := core.Loop(core.Sequence(
		core.Timing(fade, 0.3, core.TimingConfig{Duration: ms(420), Easing: sine}),
		core.Timing(fade, 1, core.TimingConfig{Duration: ms(420), Easing: sine}),
	), 0)
	var runs []*core.AnimationRun
	cx.Cleanup(func() {
		for _, r := range runs {
			r.Stop()
		}
	})
	run := func(name string, a *core.Animation) func() {
		return func() {
			st.Set(state{name + tr.AnimRunning(), ui.ToneInfo})
			runs = append(runs, cx.StartAnimation(a, func(finished bool) {
				if finished {
					st.Set(state{name + tr.AnimFinished(), ui.ToneAccent})
				} else {
					st.Set(state{name + tr.AnimStopped(), ui.ToneWarn})
				}
			}))
		}
	}
	var tiles []core.Node
	for i, c := range tones {
		tiles = append(tiles, core.View{StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 12, BorderCurve: core.BorderCurveContinuous, Background: c(),
				Shadows: fin.Knob().Shadows, Animated: &core.AnimatedStyle{Opacity: fade, TranslateY: ys[i], Rotate: ui.Lerp(spin, 0, float64(i+1)*36)}}
		}})
	}
	return ui.Card{Title: tr.AnimComposition(), Description: tr.AnimEachStepStartsAsThe(), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node { s := st.Get(); return ui.Badge{Text: s.label, Tone: s.tone} })}, Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(116), Direction: core.Row, JustifyContent: core.JustifyCenter, AlignItems: core.AlignEnd, Gap: 12, Padding: core.Edges{Bottom: 14}},
			Children: append([]core.Node{core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(14),
					Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Border()}, BorderStyle: core.BorderDashed}
			}}}, tiles...)},
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: ui.Button{Text: "Stagger", Variant: ui.ButtonSecondary, OnPress: run("stagger", stagger)}},
			ui.Cell{Child: ui.Button{Text: "Sequence", Variant: ui.ButtonSecondary, OnPress: run("sequence", sequence)}},
			ui.Cell{Child: ui.Button{Text: "Loop", Variant: ui.ButtonSecondary, OnPress: run("loop", pulse)}},
		}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: ui.Button{Text: tr.AnimStop(), Variant: ui.ButtonGhost, Icon: lucide.Pause, OnPress: func() {
				for _, r := range runs {
					r.Stop()
				}
			}}},
			ui.Cell{Child: ui.Button{Text: tr.AnimReset(), Variant: ui.ButtonGhost, OnPress: func() {
				for _, r := range runs {
					r.Reset()
				}
			}}},
		}},
	}}
}

var springKinds = []struct {
	label              func(locales.Messages) string
	stiffness, damping float64
	tone               ui.Tone
}{
	{locales.Messages.AnimGentle, 120, 14, ui.ToneInfo},
	{locales.Messages.AnimWobbly, 180, 8, ui.ToneWarn},
	{locales.Messages.AnimStiff, 400, 30, ui.ToneAccent},
}

const springTime = 1800 * time.Millisecond

// springs are springs with different settings, released together; the
// plot's dots are the springs themselves.
func springs(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	xs := make([]*core.AnimatedValue, len(springKinds))
	for i := range xs {
		xs[i] = core.NewAnimatedValue(cx, 0)
	}
	clock := core.NewAnimatedValue(cx, 0)
	width := core.NewSignal(cx, core.Points(0))
	there := core.NewSignal(cx, false)
	play := func() {
		toggled(there)
		w := float64(max(1, width.GetUntracked()-28))
		to := 0.0
		if there.GetUntracked() {
			to = w
		}
		clock.Set(0)
		steps := []*core.Animation{core.Timing(clock, 1, core.TimingConfig{Duration: springTime})}
		for i, x := range xs {
			steps = append(steps, core.Spring(x, to, core.SpringConfig{Stiffness: springKinds[i].stiffness, Damping: springKinds[i].damping}))
		}
		cx.StartAnimation(core.Parallel(steps...), nil)
	}
	// The response of each spring from 0 to 1, from the platforms' own
	// spring code.
	responses := make([][]float64, len(springKinds))
	for i, s := range springKinds {
		responses[i] = core.SampleSpring(0, 1, core.SpringConfig{Stiffness: s.stiffness, Damping: s.damping})
	}
	lineC, faintC := colors.Line, colors.Faint
	draw := func(d *core.Drawing, size core.Size) {
		cw, h := float32(size.Width), float32(size.Height)
		l, r, t, b := float32(4), cw-4, float32(10), h-10
		y := func(v float64) float32 { return b - float32(v/1.5)*(b-t) }
		// A dot matrix behind, then the target.
		d.SetFillColor(lineC())
		for gx := l; gx <= r; gx += 12 {
			for gy := t; gy <= b; gy += 12 {
				d.FillRect(gx, gy, 1.5, 1.5)
			}
		}
		d.SetLineDash(3, 4).SetStrokeColor(faintC()).BeginPath().MoveTo(l, y(1)).LineTo(r, y(1)).Stroke().SetLineDash()
		w := float64(max(1, width.Get()-28))
		fromEnd := !there.Get()
		for i, s := range springKinds {
			pts := responses[i]
			// The samples end as the spring rests; the plot shows 1.8 s,
			// held at the target after that.
			limit := int(springTime.Seconds()*60) + 1
			c := s.tone.Color(cx)
			d.SetStrokeColor(c).SetLineWidth(1.75).BeginPath()
			for k := 0; k < limit; k++ {
				v := pts[min(k, len(pts)-1)]
				if fromEnd {
					v = 1 - v
				}
				px := l + float32(k)/float32(max(1, limit-1))*(r-l)
				if k == 0 {
					d.MoveTo(px, y(v))
				} else {
					d.LineTo(px, y(v))
				}
			}
			d.Stroke()
			// The live spring value, at the clock's x.
			d.SetFillColor(c).BeginPath().Arc(l, b, 4, 0, 2*math.Pi, false).
				Bind(core.ArgX, core.Add(core.Mul(clock, core.Constant(r-l)), core.Constant(l))).
				Bind(core.ArgY, core.Add(core.Mul(core.Div(xs[i], core.Constant(w)), core.Constant(-(b-t)/1.5)), core.Constant(b))).Fill()
		}
	}
	var legends, tracks []core.Node
	for i, s := range springKinds {
		legends = append(legends, Legend{Color: s.tone.Shade(cx), Label: s.label(locales.For(cx)), Value: label(fmt.Sprintf("%.0f / %.0f", s.stiffness, s.damping))})
		track := core.View{StyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(28), JustifyContent: core.JustifyCenter, CornerRadius: 14, Background: colors.Background(), Shadows: fin.Track().Shadows}
		},
			Children: []core.Node{core.View{StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Left: core.Pt(2), Top: core.Pt(2), Width: core.Pt(24), Height: core.Pt(24), CornerRadius: 12,
					Background: s.tone.Color(cx), Shadows: fin.Knob().Shadows, Animated: &core.AnimatedStyle{TranslateX: xs[i]}}
			}}}}
		if i == 0 {
			track.OnLayout = func(b core.Bounds) { width.Set(b.Size.Width) }
		}
		tracks = append(tracks, track)
	}
	return ui.Card{Title: tr.AnimSprings(), Description: tr.AnimStiffnessAndDampingMass1(), Children: []core.Node{
		core.Canvas{Style: core.Style{Height: core.Pt(150)}, Draw: draw, Label: tr.AnimSpringResponses()},
		core.View{Style: core.Style{Direction: core.Row, Gap: 14, Wrap: core.Wrap}, Children: legends},
		core.View{Style: core.Style{Gap: 8}, Children: tracks},
		ui.Button{Text: tr.AnimRelease(), OnPress: play},
	}}
}

// dialFace is a dial's ticks and labels, drawn once.
func dialFace(cx *core.Scope, size core.Points) core.Node {
	colors := ui.ColorsOf(cx)
	muted, border, faint := colors.Muted, colors.Border, colors.Faint
	return core.Canvas{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)}, Draw: func(d *core.Drawing, s core.Size) {
		c := float32(s.Width) / 2
		for i := 0; i <= 40; i++ {
			a := float64(-120+i*6)*math.Pi/180 - math.Pi/2
			major := i%10 == 0
			r0 := c - 7
			col, w := border, float32(1)
			if major {
				r0, col, w = c-12, muted, 1.5
			}
			cos, sin := float32(math.Cos(a)), float32(math.Sin(a))
			d.SetStrokeColor(col()).SetLineWidth(w).BeginPath().MoveTo(c+cos*r0, c+sin*r0).LineTo(c+cos*(c-2), c+sin*(c-2)).Stroke()
		}
		d.SetFillColor(faint()).SetTextAlign(core.TextAlignCenter).SetTextBaseline(core.BaselineMiddle)
		for i, l := range []string{"0", "1", "2"} {
			a := float64(-120+i*120)*math.Pi/180 - math.Pi/2
			d.FillText(l, c+float32(math.Cos(a))*(c-24), c+float32(math.Sin(a))*(c-24), core.Font{Size: 11})
		}
	}}
}

// interpolations are colors, angles and scales from one value; outside
// the range they extend or clamp.
func interpolations(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	t := core.NewAnimatedValue(cx, 0)
	at := core.NewSignal(cx, 0)
	goTo := func(v int) {
		at.Set(v)
		ui.Spring(cx, t, float64(v), core.SpringConfig{Stiffness: 140, Damping: 16})
	}
	const d core.Points = 128
	return ui.Card{Title: tr.AnimInterpolation(), Description: tr.AnimOneValueDrivesColorAngle(), Actions: []core.Node{ui.Badge{TextFunc: func() string { return fmt.Sprintf("t = %d", at.Get()) }, Tone: ui.ToneAccent}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(d), Height: core.Pt(d)}, Children: []core.Node{
				dialFace(cx, d),
				core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(d/2 - 1.5), Top: core.Pt(14), Width: core.Pt(3), Height: core.Pt(d - 28),
					Animated: &core.AnimatedStyle{Rotate: core.Interpolate(t, core.Interpolation{Input: []float64{0, 2}, Output: []float64{-120, 120}, Extrapolate: core.ExtrapolateExtend})}},
					Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(d/2 - 22), CornerRadius: 1.5}, StyleFunc: ui.Filled(colors.Accent)}}},
				core.View{StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Left: core.Pt(d/2 - 6), Top: core.Pt(d/2 - 6), Width: core.Pt(12), Height: core.Pt(12), CornerRadius: 6,
						Background: colors.Light(), Shadows: fin.Light().Shadows}
				}},
			}},
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return core.Style{Width: core.Pt(64), Height: core.Pt(64), CornerRadius: 16, BorderCurve: core.BorderCurveContinuous, Background: colors.Info(),
						Animated: &core.AnimatedStyle{
							Background: core.Interpolate(t, core.Interpolation{Input: []float64{0, 1, 2}, Colors: []core.Color{colors.Info(), colors.Accent(), colors.Danger()}}),
							Rotate:     core.Interpolate(t, core.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 180}, ExtrapolateRight: core.ExtrapolateExtend}),
						}}
				}},
				core.Text{Content: "extend", Style: core.Style{FontSize: 11, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
			}},
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(64), Height: core.Pt(64), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(52), Height: core.Pt(52), CornerRadius: 26,
						Animated: &core.AnimatedStyle{Scale: core.Interpolate(t, core.Interpolation{Input: []float64{0, 1}, Output: []float64{0.6, 1.2}, Extrapolate: core.ExtrapolateClamp})}}, StyleFunc: ui.Filled(colors.Up)},
				}},
				core.Text{Content: "clamp", Style: core.Style{FontSize: 11, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
			}},
		}},
		ui.Segmented[int]{Options: []ui.Option[int]{{Value: 0, Label: "t = 0"}, {Value: 1, Label: "t = 1"}, {Value: 2, Label: "t = 2"}}, Value: at.Get, OnChange: goTo},
		core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
			Caption{Label: tr.AnimColorThroughThreeStops(), Note: label(tr.AnimAt01And2())},
			Caption{Label: tr.AnimRotationKeepsGoingPast1(), Note: label(tr.Anim360At2())},
			Caption{Label: tr.AnimScaleStopsAt12(), Note: label("clamp")},
		}},
	}}
}

// hidingHeader is a header that hides while the list scrolls up and comes
// back as it scrolls down: DiffClamp of the scroll offset, on the
// platform.
func hidingHeader(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	scroll := core.NewAnimatedValue(cx, 0)
	const h core.Points = 52
	header := core.DiffClamp(scroll, 0, float64(h))
	var rows []core.Node
	for n := 1; n <= 30; n++ {
		bar := colors.Raised
		if n%3 == 0 {
			bar = colors.Accent
		}
		rows = append(rows, core.View{StyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(48), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(0, 12),
				CornerRadius: 12, Background: colors.Surface(), BorderWidth: 1, BorderColor: colors.Line()}
		}, Children: []core.Node{
			core.Text{Content: fmt.Sprintf("%02d", n), Style: core.Style{Width: core.Pt(24), FontSize: 13}, StyleFunc: ui.Colored(colors.Faint)},
			core.Text{Content: tr.NavRow(n), Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
			core.View{Style: core.Style{Width: core.Pt(core.Points(40 + (n*37)%50)), Height: core.Pt(4), CornerRadius: 2}, StyleFunc: ui.Filled(bar)},
		}})
	}
	return ui.Card{Title: tr.AnimHidingHeader(), Description: tr.AnimHidesAsTheListScrolls(), Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(300), CornerRadius: ui.RadiusControl, Overflow: core.Hidden, Background: colors.Background(), Shadows: fin.Track().Shadows}
		}, Children: []core.Node{
			core.Scroll{Style: core.Style{FlexGrow: 1}, OffsetValues: core.AnimatedXY{Y: scroll}, TestID: "hiding-list", Children: []core.Node{
				core.View{Style: core.Style{Padding: core.Edges{Top: h + 8, Right: 8, Bottom: 8, Left: 8}, Gap: 6}, Children: rows},
			}},
			core.View{StyleFunc: func() core.Style {
				return fin.Raised().Over(core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0), Height: core.Pt(h), Direction: core.Row,
					AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(0, 14), Background: colors.Raised(), Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Border()},
					Animated: &core.AnimatedStyle{
						TranslateY: core.Mul(header, core.Constant(-1)),
						Opacity:    core.Interpolate(header, core.Interpolation{Input: []float64{0, float64(h)}, Output: []float64{1, 0}, Easing: ease(core.EasingQuad, core.EaseModeIn)}),
					}})
			}, Children: []core.Node{
				ui.Icon(lucide.Search, 16, colors.Muted),
				core.Text{Content: tr.AnimSearch(), Style: core.Style{FlexGrow: 1, FontSize: 13, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
				API{Text: fmt.Sprintf("DiffClamp(0, %.0f)", h)},
			}},
		}},
	}}
}

// layoutAnimation is layout changes animated by the platform in the next
// commit: moves, inserts and removals.
func layoutAnimation(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	column := core.NewSignal(cx, false)
	preset := core.NewSignal(cx, "spring")
	next := 6
	items := core.NewSignalFunc(cx, []int{1, 2, 3, 4, 5}, slices.Equal)
	presets := map[string]core.LayoutAnimation{"spring": core.LayoutSpringPreset, "ease": core.LayoutEaseInEaseOutPreset, "linear": core.LayoutLinearPreset}
	change := func(fn func()) {
		cx.AnimateLayout(presets[preset.GetUntracked()])
		fn()
	}
	tones := motionTones(cx)
	return ui.Card{Title: tr.AnimLayoutAnimation(), Description: tr.AnimFramesInsertsAndRemovalsOf(), Actions: []core.Node{ui.Badge{TextFunc: func() string { return tr.AnimViews(len(items.Get())) }}}, Children: []core.Node{
		ui.Segmented[string]{Options: []ui.Option[string]{{Value: "spring", Label: tr.AnimSpring()}, {Value: "ease", Label: tr.AnimEaseInOut()}, {Value: "linear", Label: tr.AnimLinear()}}, Value: preset.Get, OnChange: preset.Set, Small: true},
		core.View{StyleFunc: func() core.Style {
			d := core.Row
			if column.Get() {
				d = core.Column
			}
			return core.Style{MinHeight: core.Pt(64), Direction: d, Wrap: core.Wrap, Gap: 8}
		}, Children: []core.Node{core.Each(items.Get, func(n int) int { return n }, func(cx *core.Scope, id func() int, _ func() int) core.Node {
			fin := ui.FinOf(cx)
			n := core.Untracked(cx, id)
			return core.View{StyleFunc: func() core.Style {
				s := core.Style{CornerRadius: 12, BorderCurve: core.BorderCurveContinuous, Padding: core.Symmetric(0, 10), JustifyContent: core.JustifyCenter,
					Background: tones[n%len(tones)](), Shadows: fin.Knob().Shadows}
				if column.Get() {
					s.Height = core.Pt(30)
				} else {
					s.Height, s.Width, s.FlexGrow = core.Pt(64), core.Pt(50), 1
				}
				return s
			}, Children: []core.Node{ui.Text(itoa(n), 13, ui.Semibold, onTone(cx))}}
		})}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: core.Dynamic(func(*core.Scope) core.Node {
				l := tr.AnimColumn()
				if column.Get() {
					l = tr.AnimRow()
				}
				return ui.Button{Text: l, OnPress: func() { change(func() { toggled(column) }) }}
			})},
			ui.Cell{Child: ui.Button{Text: tr.AnimShuffle(), Variant: ui.ButtonSecondary, OnPress: func() {
				change(func() {
					items.Update(func(s []int) []int {
						s = slices.Clone(s)
						rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
						return s
					})
				})
			}}},
		}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: ui.Button{Text: tr.AnimAdd(), Variant: ui.ButtonSecondary, Icon: lucide.Plus, Disabled: func() bool { return len(items.Get()) >= 8 }, OnPress: func() {
				change(func() { items.Update(func(s []int) []int { return append(slices.Clone(s), next) }); next++ })
			}}},
			ui.Cell{Child: ui.Button{Text: tr.AnimRemove(), Variant: ui.ButtonGhost, Disabled: func() bool { return len(items.Get()) <= 1 }, OnPress: func() {
				change(func() { items.Update(func(s []int) []int { return slices.Clone(s[1:]) }) })
			}}},
		}},
	}}
}

// transitions are style changes the platform animates: no engine work
// per frame.
func transitions(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	moved := core.NewSignal(cx, false)
	curves := []struct {
		name  string
		curve core.Curve
	}{{"Linear", core.Linear}, {"Ease", core.Ease}, {"EaseIn", core.EaseIn}, {"EaseOut", core.EaseOut}, {"EaseInOut", core.EaseInOut}}
	var lanes []core.Node
	for _, c := range curves {
		lanes = append(lanes, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Height: core.Pt(24)}, Children: []core.Node{
			core.Text{Content: c.name, Style: core.Style{Width: core.Pt(76), FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Height: core.Pt(12), JustifyContent: core.JustifyCenter}, Children: []core.Node{
				core.View{Style: core.Style{Height: core.Pt(2), CornerRadius: 1}, StyleFunc: ui.Filled(colors.Raised)},
				core.View{StyleFunc: func() core.Style {
					s := core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Width: core.Pt(12), Height: core.Pt(12), CornerRadius: 6, Background: colors.Accent(),
						Shadows: fin.Knob().Shadows, Transition: core.Transition{Duration: ms(900), Easing: c.curve, Properties: core.TransitionLayout}}
					if moved.Get() {
						s.Left = core.Pct(90)
					}
					return s
				}},
			}},
		}})
	}
	return ui.Card{Title: tr.AnimTransitions(), Description: tr.AnimAStyleSTransitionAnimates(), Actions: []core.Node{ui.Badge{Text: "NSGo"}}, Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(96), AlignItems: core.AlignStart, JustifyContent: core.JustifyCenter}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				s := core.Style{Width: core.Pt(64), Height: core.Pt(64), CornerRadius: 14, Background: colors.AccentFill(), Shadows: fin.Accent().Shadows,
					Transition: core.Transition{Duration: ms(450), Easing: core.EaseInOut}}
				if moved.Get() {
					s.Background, s.CornerRadius = colors.Info(), 32
					s.Transform = core.Transform{TranslateX: 140, Rotate: 180, Scale: 0.8}
					s.Opacity = core.Alpha(0.75)
				}
				return s
			}},
		}},
		core.View{Style: core.Style{Gap: 2}, Children: lanes},
		APILine{Label: tr.AnimColorCornersTransformOpacityAndFrame(), Names: "Style.Transition · TransitionLayout"},
		ui.Button{Text: tr.AnimToggle(), Variant: ui.ButtonSecondary, OnPress: func() { toggled(moved) }},
	}}
}

// animationsDemo is the platform's animation graph: loops, ready-made
// motions and keyframes, groups, easing curves, springs, interpolation,
// DiffClamp, layout animation and style transitions.
func animationsDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, hero, motionPresets, staggeredEntrance, keyframesMotions, aurora, easingPlot, easingRace, composition, springs, interpolations,
		hidingHeader, layoutAnimation, transitions)
}
