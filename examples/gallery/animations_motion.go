package main

import (
	"fmt"
	"math"
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

// The ready-made motions, a staggered entrance, keyframes and an aurora:
// NSH's motion cards, each motion played on the frame clock.

var motionKinds = []struct {
	kind  core.MotionKind
	label string
}{{core.MotionEnter, "Enter"}, {core.MotionExit, "Exit"}, {core.MotionAttention, "Attention"}, {core.MotionLoop, "Loop"}}

// presetTile is one preset on a little card of its own: a tap plays it.
// Exits come back after a moment; loops play until tapped again.
func presetTile(cx *core.Scope, p core.MotionPreset, tone func() core.Color, onPlay func(string)) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	m := core.NewMotion(cx, nil)
	looping := core.NewSignal(cx, false)
	var run *core.AnimationRun
	// turn is the latest tap: an exit comes back only if no tap came after.
	turn := 0
	press := func() {
		turn++
		mine := turn
		if run != nil {
			run.Stop()
		}
		if p.Kind() == core.MotionLoop {
			looping.Set(!looping.GetUntracked())
			if looping.GetUntracked() {
				onPlay(p.Name())
				run = m.Play(p.Spec(), nil)
			} else {
				onPlay("stopped")
				run = m.Play(core.MotionSpec{Timing: core.KeyframeTiming{Duration: ms(1)}, Tracks: restTracks()}, nil)
			}
			return
		}
		onPlay(p.Name())
		run = m.Play(p.Spec(), func(finished bool) {
			if p.Kind() != core.MotionExit || !finished {
				return
			}
			cx.AfterFunc(ms(450), func() {
				if mine != turn {
					return
				}
				in := core.FadeIn.Spec()
				in.Timing.Duration = ms(220)
				in.Tracks = append(restTracks(), in.Tracks...)
				run = m.Play(in, nil)
			})
		})
	}
	return base.Button{OnPress: func(core.PressEvent) { press() }, Label: p.Name(),
		Style: core.Style{FlexBasis: core.Pct(30), FlexGrow: 1, CornerRadius: 14, Active: &core.Style{Transform: core.Transform{Scale: 0.97}}},
		Children: []core.Node{core.View{StyleFunc: func() core.Style {
			border := colors.Border()
			if looping.Get() {
				border = tone()
			}
			return fin.Raised().Over(core.Style{Height: core.Pt(92), CornerRadius: 14, BorderCurve: core.BorderCurveContinuous, AlignItems: core.AlignCenter,
				JustifyContent: core.JustifyCenter, Gap: 12, Background: colors.Raised(), BorderWidth: 1, BorderColor: border})
		}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return fin.Knob().Over(core.Style{Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 9, BorderCurve: core.BorderCurveContinuous,
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: tone(), Perspective: 300, Animated: m.Style()})
			},
				Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, Opacity: core.Alpha(0.85)}, StyleFunc: ui.Filled(onTone(cx))}}},
			core.Text{Content: p.Name(), MaxLines: 1, StyleFunc: func() core.Style {
				c := colors.Muted()
				if looping.Get() {
					c = colors.Text()
				}
				return core.Style{FontSize: 12, FontFamily: ui.Mono, Color: c}
			}},
		}}}}
}

// restTracks puts every channel a preset moves back at rest.
func restTracks() []core.MotionTrack {
	var out []core.MotionTrack
	for _, c := range []core.MotionChannel{core.MotionOpacity, core.MotionScale, core.MotionScaleX, core.MotionScaleY} {
		out = append(out, core.MotionTrack{Channel: c, Stops: core.Stops(1, 1)})
	}
	for _, c := range []core.MotionChannel{core.MotionTranslateX, core.MotionTranslateY, core.MotionRotate, core.MotionRotateX, core.MotionRotateY} {
		out = append(out, core.MotionTrack{Channel: c, Stops: core.Stops(0, 0)})
	}
	return out
}

// motionPresets are the ready-made motions by what they are for, each a
// tap away.
func motionPresets(cx *core.Scope) core.Node {
	kind := core.NewSignal(cx, core.MotionAttention)
	status := core.NewSignal(cx, "tap a tile")
	var chips []core.Node
	for _, k := range motionKinds {
		chips = append(chips, chipTab(cx, k.label, func() bool { return kind.Get() == k.kind }, func() {
			kind.Set(k.kind)
			status.Set("tap a tile")
		}))
	}
	return ui.Card{Icon: ui.IconSpark, Title: "Motion presets", Description: fmt.Sprintf("%d ready-made motions, played natively", len(core.MotionPresets())), Actions: []core.Node{ui.Badge{TextFunc: status.Get, Tone: ui.ToneAccent}}, Children: []core.Node{
		core.Scroll{Horizontal: true, HideIndicator: true, Children: []core.Node{
			base.Tabs{Label: "Kinds", Style: core.Style{Direction: core.Row, Gap: 8, Padding: core.Symmetric(4, 4)}, Children: chips}}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			var tiles []core.Node
			for _, p := range core.MotionPresets() {
				if p.Kind() == kind.Get() {
					tones := motionTones(cx)
					tiles = append(tiles, presetTile(cx, p, tones[len(tiles)%len(tones)], status.Set))
				}
			}
			return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8}, Children: tiles}
		}),
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			ui.APILine{Label: "Channels for a view", Names: "NewMotion"},
			ui.APILine{Label: "A preset", Names: "Motion.Play(Shake.Spec())"},
		}},
	}}
}

var staggerBars = []float32{0.5, 0.78, 0.42, 1, 0.62, 0.88, 0.55}

// staggerSteps is how many steps item i of count waits, counted from
// from (0 first, 1 center, 2 last), as Stagger counts them: the middle
// pair of an even count together.
func staggerSteps(i, count, from int) int {
	switch from {
	case 0:
		return i
	case 2:
		return count - 1 - i
	}
	return int(math.Floor(math.Abs(float64(i) - float64(count-1)/2)))
}

// staggerBar is a bar and its cap, entering delay in.
func staggerBar(cx *core.Scope, height float32, tone func() core.Color, delay int) core.Node {
	barSpec := core.FadeInUp.Spec()
	barSpec.Timing.Delay, barSpec.Timing.Duration = ms(delay), ms(520)
	capSpec := core.PopIn.Spec()
	capSpec.Timing.Delay = ms(delay + 220)
	bar, capm := core.NewMotion(cx, &barSpec), core.NewMotion(cx, &capSpec)
	return core.View{Style: core.Style{FlexGrow: 1, Height: core.Pct(100), JustifyContent: core.JustifyEnd, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, Animated: capm.Style()}, StyleFunc: ui.Filled(tone)},
		core.View{Style: core.Style{AlignSelf: core.AlignStretch, Height: core.Pct(height * 78), CornerRadius: 10, BorderCurve: core.BorderCurveContinuous,
			Animated: bar.Style()}, StyleFunc: func() core.Style {
			return core.Style{Gradients: []core.Gradient{core.LinearGradient(180, tone(), ui.Alpha(tone(), 0.35))}}
		}},
	}}
}

// staggeredEntrance is items entering one after another, from either end
// or the middle: a motion per item, each with its own delay.
func staggeredEntrance(cx *core.Scope) core.Node {
	from := core.NewSignal(cx, 1)
	wave := core.NewSignal(cx, 0)
	replay := func(origin int) {
		from.Set(origin)
		wave.Update(func(w int) int { return w + 1 })
	}
	return ui.Card{Icon: ui.IconLists, Title: "Staggered entrance", Description: "NewMotion(enter with a delay) per item", Actions: []core.Node{ui.Badge{Text: "70 ms apart"}}, Children: []core.Node{
		core.Dynamic(func(cx *core.Scope) core.Node {
			wave.Get()
			origin := from.Get()
			tones := motionTones(cx)
			var bars []core.Node
			for i, h := range staggerBars {
				bars = append(bars, staggerBar(cx, h, tones[i%len(tones)], staggerSteps(i, len(staggerBars), origin)*70))
			}
			return core.View{Style: core.Style{Height: core.Pt(140), Direction: core.Row, AlignItems: core.AlignEnd, Gap: 8}, Children: bars}
		}),
		base.Tabs{Label: "Origin", Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			chipTab(cx, "From first", func() bool { return from.Get() == 0 }, func() { replay(0) }),
			chipTab(cx, "Center", func() bool { return from.Get() == 1 }, func() { replay(1) }),
			chipTab(cx, "Last", func() bool { return from.Get() == 2 }, func() { replay(2) }),
		}},
		ui.Button{Text: "Replay", Variant: ui.ButtonSecondary, Icon: ui.IconPlay, OnPress: func() { replay(from.GetUntracked()) }},
	}}
}

const (
	moon   = 52
	phases = 8
)

// shadowAt is where the shadow of phase k sits: it slides off to the
// right as the moon waxes, then comes back from the left as it wanes.
func shadowAt(k int) core.Points {
	if k <= phases/2 {
		return core.Points(k) / (phases / 2) * moon
	}
	return -core.Points(phases-k) / (phases / 2) * moon
}

// keyframesMotions is sprites and ticks with steps, and a like made of
// chained motions.
func keyframesMotions(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	fin := ui.FinOf(cx)
	sprite, tick := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	cx.StartAnimation(core.Parallel(
		core.ValueKeyframes(sprite, core.KeyframeTiming{Duration: ms(1600), Easing: core.StepsEasing(phases, core.JumpEnd), Iterations: core.Forever},
			core.Stops(0, -moon*phases)...),
		core.ValueKeyframes(tick, core.KeyframeTiming{Duration: ms(12000), Easing: core.StepsEasing(12, core.JumpEnd), Iterations: core.Forever},
			core.Stops(0, 360)...),
	), nil)

	heart, bubble := core.NewMotion(cx, nil), core.NewMotion(cx, nil)
	bubble.Value(core.MotionOpacity).Set(0)
	liked, count := core.NewSignal(cx, false), core.NewSignal(cx, 41)
	like := func() {
		on := !liked.GetUntracked()
		liked.Set(on)
		count.Update(func(n int) int {
			if on {
				return n + 1
			}
			return n - 1
		})
		if !on {
			heart.Play(core.HeadShake.Spec(), nil)
			return
		}
		back := ease(core.EasingBack, core.EaseModeOut)
		bubble.Play(core.MotionSpec{Timing: core.KeyframeTiming{Duration: ms(900), Easing: ease(core.EasingQuad, core.EaseModeOut)}, Tracks: []core.MotionTrack{
			{Channel: core.MotionOpacity, Stops: []core.KeyframeStop{core.StopAt(0, 0), core.StopAt(0.3, 1).Eased(back), core.StopAt(1, 0)}},
			{Channel: core.MotionTranslateY, Stops: []core.KeyframeStop{core.StopAt(0, 0), core.StopAt(0.3, -18).Eased(back), core.StopAt(1, -46)}},
			{Channel: core.MotionScale, Stops: []core.KeyframeStop{core.StopAt(0, 0.6), core.StopAt(0.3, 1).Eased(back), core.StopAt(1, 1)}},
		}}, nil)
		pop := core.PopIn.Spec()
		pop.Timing.Duration = ms(360)
		heart.Play(pop, func(finished bool) {
			if finished {
				heart.Play(core.Heartbeat.Spec(), nil)
			}
		})
	}

	charge := core.NewAnimatedValue(cx, 0)
	width := core.NewSignal(cx, core.Points(0))
	charging := func() {
		charge.Set(0)
		cx.StartAnimation(core.ValueKeyframes(charge, core.KeyframeTiming{Duration: ms(1500), Easing: core.StepsEasing(5, core.JumpEnd)}, core.Stops(0, 1)...), nil)
	}
	caption := func(s string) core.Node {
		return core.Text{Content: s, Style: core.Style{FontSize: 11, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)}
	}

	var frames []core.Node
	for k := range phases {
		frames = append(frames, core.View{Style: core.Style{Width: core.Pt(moon), Height: core.Pt(moon), CornerRadius: moon / 2, Overflow: core.Hidden}, StyleFunc: ui.Filled(colors.Warn),
			Children: []core.Node{core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(shadowAt(k)), Top: core.Pt(0), Width: core.Pt(moon), Height: core.Pt(moon),
				CornerRadius: moon / 2}, StyleFunc: ui.Filled(colors.Raised)}}})
	}
	dial := []core.Node{}
	for k := range 12 {
		mark := core.Points(3)
		if k%3 == 0 {
			mark = 6
		}
		dial = append(dial, core.View{PointerEvents: core.PointerEventsNone,
			Style: core.Style{Position: core.Absolute, Left: core.Pt(moon/2 - 2.25), Top: core.Pt(2), Width: core.Pt(1.5), Height: core.Pt(moon - 7),
				Transform: core.Transform{Rotate: float32(k * 30)}},
			Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(mark)}, StyleFunc: ui.Filled(colors.Faint)}}})
	}
	dial = append(dial, core.View{PointerEvents: core.PointerEventsNone,
		Style: core.Style{Position: core.Absolute, Left: core.Pt(moon/2 - 2.75), Top: core.Pt(7), Width: core.Pt(2.5), Height: core.Pt(moon - 17),
			Animated: &core.AnimatedStyle{Rotate: tick}},
		Children: []core.Node{core.View{Style: core.Style{Height: core.Pt((moon - 17) / 2), CornerRadius: 1.25}, StyleFunc: ui.Filled(colors.Accent)}}})
	var segments []core.Node
	for k := range 5 {
		s := core.Style{FlexGrow: 1}
		var sf func() core.Style
		if k > 0 {
			s.Borders = core.Edges{Left: 2}
			sf = func() core.Style { return core.Style{BorderColors: core.EdgeColors{Left: colors.Surface()}} }
		}
		segments = append(segments, core.View{Style: s, StyleFunc: sf})
	}

	return ui.Card{Icon: ui.IconAnimation, Title: "Keyframes", Description: "Steps, keyframes with easings of their own, chained motions", Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignStart, JustifyContent: core.JustifySpaceAround}, Children: []core.Node{
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(moon), Height: core.Pt(moon), CornerRadius: moon / 2, Overflow: core.Hidden}, StyleFunc: ui.Filled(colors.Raised), Children: []core.Node{
					core.View{Style: core.Style{Direction: core.Row, Width: core.Pt(moon * phases), Animated: &core.AnimatedStyle{TranslateX: sprite}}, Children: frames},
				}},
				caption("steps(8)"),
			}},
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return core.Style{Width: core.Pt(moon), Height: core.Pt(moon), CornerRadius: moon / 2, BorderWidth: 1.5, BorderColor: colors.Border(), Background: colors.Raised()}
				},
					Children: dial},
				caption("steps(12)"),
			}},
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				base.Button{OnPress: func(core.PressEvent) { like() }, Label: "Like", Selected: liked.Get, Style: core.Style{CornerRadius: moon / 2}, Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						bg, border := colors.Raised(), colors.Border()
						if liked.Get() {
							bg, border = tints.Warn(), colors.Warn()
						}
						return core.Style{Width: core.Pt(moon), Height: core.Pt(moon), CornerRadius: moon / 2, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
							Background: bg, BorderWidth: 1, BorderColor: border}
					}, Children: []core.Node{
						core.View{Style: core.Style{Animated: heart.Style()}, Children: []core.Node{ui.Icon(ui.IconStar, 24, func() core.Color {
							if liked.Get() {
								return colors.Warn()
							}
							return colors.Muted()
						})}},
					}},
				}},
				core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(-4), Right: core.Pt(-10), Animated: bubble.Style()},
					Children: []core.Node{ui.Text("+1", 13, ui.Bold, colors.Warn)}},
				core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) + " likes" },
					Style: core.Style{FontSize: 11, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
			}},
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
			core.View{OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) },
				StyleFunc: func() core.Style {
					return core.Style{FlexGrow: 1, Height: core.Pt(22), CornerRadius: 7, Overflow: core.Hidden, Background: colors.Background(), Shadows: fin.Track().Shadows}
				},
				Children: []core.Node{
					core.Dynamic(func(*core.Scope) core.Node {
						w := float64(width.Get())
						return core.View{StyleFunc: func() core.Style {
							return core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(core.Points(w)),
								Gradients: []core.Gradient{core.LinearGradient(90, colors.Up(), colors.Accent())},
								Animated:  &core.AnimatedStyle{TranslateX: core.Add(core.Mul(charge, core.Constant(w)), core.Constant(-w))}}
						}}
					}),
					core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0),
						Direction: core.Row}, Children: segments},
				}},
			ui.Button{Text: "Charge", Small: true, Variant: ui.ButtonSecondary, OnPress: charging},
		}},
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			ui.APILine{Label: "A sprite sheet, frame by frame", Names: "StepsEasing(8)"},
			ui.APILine{Label: "+1: three stops, one with its easing", Names: "StopAt(…).Eased(…)"},
			ui.APILine{Label: "Like: pop in, then a heartbeat", Names: "Play(…, done)"},
			ui.APILine{Label: "Charge in five jumps", Names: "ValueKeyframes"},
		}},
	}}
}

// auroraBlob is where a blob rests, its colors in each appearance and the
// path it drifts along, there and back.
type auroraBlob struct {
	left        float32 // percent
	top, size   core.Points
	light, dark uint32
	duration    int
	tracks      []core.MotionTrack
}

func drift(c core.MotionChannel, from, to float64) core.MotionTrack {
	return core.MotionTrack{Channel: c, Stops: core.Stops(from, to)}
}

var auroraBlobs = []auroraBlob{
	{-18, -70, 250, 0xff5fa2, 0xd946ef, 7000, []core.MotionTrack{drift(core.MotionTranslateX, 0, 90), drift(core.MotionTranslateY, 0, 50), drift(core.MotionScale, 1, 1.3)}},
	{48, -60, 230, 0x38d9ff, 0x0ea5e9, 9000, []core.MotionTrack{drift(core.MotionTranslateX, 0, -80), drift(core.MotionTranslateY, 0, 70), drift(core.MotionScale, 1.1, 0.85)}},
	{8, 90, 270, 0xffc93c, 0x6366f1, 8000, []core.MotionTrack{drift(core.MotionTranslateX, 0, 70), drift(core.MotionTranslateY, 0, -60), drift(core.MotionScale, 0.9, 1.2)}},
	{58, 110, 210, 0x7cf29a, 0x14b8a6, 6000, []core.MotionTrack{drift(core.MotionTranslateX, 0, -60), drift(core.MotionTranslateY, 0, -40),
		drift(core.MotionRotate, 0, 40), drift(core.MotionScale, 1, 1.25)}},
}

// aurora is a getgradients-style background: blurred radial gradients,
// each drifting on a looping motion of its own, clipped in a card.
func aurora(cx *core.Scope) core.Node {
	ink := func() core.Color {
		if cx.Dark() {
			return core.RGB(0xffffff)
		}
		return core.RGB(0x1b1035)
	}
	var blobs []core.Node
	for _, b := range auroraBlobs {
		m := core.NewMotion(cx, nil)
		m.Play(core.MotionSpec{Tracks: b.tracks, Timing: core.KeyframeTiming{Duration: ms(b.duration), Easing: ease(core.EasingSin, core.EaseModeInOut),
			Iterations: core.Forever, Direction: core.PlayAlternate}}, nil)
		blobs = append(blobs, core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pct(b.left), Top: core.Pt(b.top),
			Width: core.Pt(b.size), Height: core.Pt(b.size), CornerRadius: b.size / 2, Filters: []core.Filter{core.Blur(28)}, Animated: m.Style()},
			StyleFunc: func() core.Style {
				c, blend := core.RGB(b.light), core.BlendMultiply
				if cx.Dark() {
					c, blend = core.RGB(b.dark), core.BlendScreen
				}
				g := core.Gradient{Kind: core.GradientRadial, Circle: true, Stops: []core.ColorStop{
					{Color: ui.Alpha(c, 0.9), At: core.Pct(0)}, {Color: ui.Alpha(c, 0.5), At: core.Pct(40)}, {Color: ui.Alpha(c, 0), At: core.Pct(70)}}}
				return core.Style{Gradients: []core.Gradient{g}, BlendMode: blend}
			}})
	}
	blobs = append(blobs, core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0),
		Padding: core.All(20), JustifyContent: core.JustifyEnd, Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{AlignSelf: core.AlignStart, Padding: core.Symmetric(0, 10), Height: core.Pt(24), CornerRadius: 12, JustifyContent: core.JustifyCenter,
			BorderWidth: 1}, StyleFunc: func() core.Style {
			glass := ui.Alpha(core.RGB(0xffffff), 0.55)
			if cx.Dark() {
				glass = ui.Alpha(core.RGB(0xffffff), 0.12)
			}
			return core.Style{Background: glass, BorderColor: ui.Alpha(ink(), 0.12)}
		}, Children: []core.Node{
			core.Text{Content: "4 × NewMotion", Style: core.Style{FontSize: 11, FontFamily: ui.Mono}, StyleFunc: ui.Colored(ink)}}},
		core.Text{Content: "Aurora", Style: core.Style{FontSize: 34, FontWeight: ui.Bold, LetterSpacing: -0.8}, StyleFunc: ui.Colored(ink)},
		core.Text{Content: "Each light drifts on its own loop, there and back", Style: core.Style{FontSize: 14},
			StyleFunc: ui.Colored(func() core.Color { return ui.Alpha(ink(), 0.7) })},
	}})
	return ui.Card{Icon: ui.IconStyles, Title: "Aurora", Description: "Blurred gradients on looping motions", Flush: true, Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(260), CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous, Overflow: core.Hidden},
			StyleFunc: func() core.Style {
				if cx.Dark() {
					return core.Style{Background: core.RGB(0x0a0716)}
				}
				return core.Style{Background: core.RGB(0xfff8f3)}
			}, Children: blobs},
	}}
}
