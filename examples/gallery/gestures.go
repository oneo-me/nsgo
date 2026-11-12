package main

import (
	"fmt"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"math"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// dots is a dot matrix behind a stage, drawn once.
func dots(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	c := colors.Line
	return core.Canvas{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
		Draw: func(d *core.Drawing, size core.Size) {
			d.SetFillColor(c())
			for x := float32(8); x < float32(size.Width); x += 14 {
				for y := float32(8); y < float32(size.Height); y += 14 {
					d.FillRect(x, y, 1.5, 1.5)
				}
			}
		}}
}

// stage is the darker surface a demo plays on, for a StyleFunc.
func stage(cx *core.Scope, height core.Points) core.Style {
	colors := ui.ColorsOf(cx)
	return core.Style{Height: core.Pt(height), CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, Overflow: core.Hidden,
		Background: colors.Background(), BorderWidth: 1, BorderColor: colors.Line(), Shadows: ui.FinOf(cx).Track().Shadows}
}

// centered is a stage with its content centered, for a StyleFunc.
func centered(cx *core.Scope, height core.Points) func() core.Style {
	return func() core.Style {
		return stage(cx, height).Merge(core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter})
	}
}

// knobShadows are the highlight and shadow of a small colored knob, read
// in a StyleFunc.
func knobShadows(cx *core.Scope) []core.BoxShadow {
	if cx.Dark() {
		return []core.BoxShadow{ui.Inner(0, 1, 0, ui.White(0.35)), ui.Shadow(0, 3, 10, 0, ui.Black(0.45))}
	}
	return ui.FinOf(cx).Knob().Shadows
}

// transformable is one photo dragged, pinched and rotated at the same
// time, all on the platform's main thread.
func transformable(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	x, y := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	scale, angle := core.NewAnimatedValue(cx, 1), core.NewAnimatedValue(cx, 0)
	type result struct {
		scale, angle float32
		drag         int
	}
	last := core.NewSignal(cx, result{scale: 1})
	reset := func() {
		cx.StartAnimation(core.Parallel(core.Spring(x, 0, core.SpringConfig{}), core.Spring(y, 0, core.SpringConfig{}),
			core.Spring(scale, 1, core.SpringConfig{}), core.Spring(angle, 0, core.SpringConfig{})), nil)
		last.Set(result{scale: 1})
	}
	degrees := core.Mul(angle, core.Constant(180/math.Pi))
	return Card{Title: tr.GestPanPinchAndRotation(), Description: tr.GestThreeRecognizersOnOneView(), Children: []core.Node{
		core.View{StyleFunc: centered(cx, 260), Children: []core.Node{
			dots(cx),
			core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Width: core.Pt(200), Height: core.Pt(150), CornerRadius: 18,
					BorderWidth: 1, BorderColor: colors.Faint(), BorderStyle: core.BorderDashed}
			}},
			core.View{TestID: "photo", PanValues: core.AnimatedXY{X: x, Y: y}, PinchValue: scale, RotationValue: angle,
				OnPan: func(e core.PanEvent) {
					if e.Phase == core.GestureEnded {
						r := last.GetUntracked()
						r.drag = int(math.Round(math.Hypot(float64(e.Translation.X), float64(e.Translation.Y))))
						last.Set(r)
					}
				},
				OnPinch: func(e core.GestureEvent) {
					if e.Phase == core.GestureEnded {
						r := last.GetUntracked()
						r.scale = e.Value
						last.Set(r)
					}
				},
				OnRotation: func(e core.GestureEvent) {
					if e.Phase == core.GestureEnded {
						r := last.GetUntracked()
						r.angle = e.Value * 180 / math.Pi
						last.Set(r)
					}
				},
				StyleFunc: func() core.Style {
					return core.Style{Width: core.Pt(200), Height: core.Pt(150), CornerRadius: 18, BorderCurve: core.BorderCurveContinuous, Overflow: core.Hidden,
						BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised(),
						Animated: &core.AnimatedStyle{TranslateX: x, TranslateY: y, Scale: scale, Rotate: degrees}}
				},
				Children: []core.Node{core.Image{Source: core.Src("https://picsum.photos/seed/nsh-card/400/300"), Style: core.Style{FlexGrow: 1}}}},
		}},
		core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
			Stat{Label: tr.GestDrag(), Value: func() string { return fmt.Sprintf("%d pt", last.Get().drag) }, Width: core.Pct(33)},
			Stat{Label: tr.GestScale(), Value: func() string { return fmt.Sprintf("%.2f×", last.Get().scale) }, Tone: ToneAccent, Width: core.Pct(33)},
			Stat{Label: tr.GestAngle(), Value: func() string { return fmt.Sprintf("%d°", int(math.Round(float64(last.Get().angle)))) }, Tone: ToneInfo, Width: core.Pct(34)},
		}},
		Note{Text: Hint(cx, tr.GestPinchAndRotateWithTwo(), tr.GestDragWithTheMousePinch())},
		ui.Button{Text: tr.GestReset(), Variant: ui.ButtonSecondary, OnPress: reset},
	}}
}

// ball is a ball dragged as the touch responder (positions go through the
// engine). Its responder keeps the page from scrolling while it is
// dragged.
func ball(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	pos := core.NewSignal(cx, core.Point{})
	type state struct {
		label string
		tone  Tone
	}
	st := core.NewSignal(cx, state{tr.GestIdle(), ToneNeutral})
	grab := core.NewAnimatedValue(cx, 0)
	var start core.Point
	return Card{Title: tr.GestTouchResponder(), Description: tr.GestPositionsGoThroughTheEngine(), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node { s := st.Get(); return ui.Badge{Text: s.label, Variant: s.tone.Badge()} })}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style { return stage(cx, 200) }, Children: []core.Node{
			dots(cx),
			core.View{TestID: "ball", Responder: &core.Responder{
				Claims: []core.TouchClaim{{}},
				OnGrant: func(core.ResponderEvent) {
					start = pos.GetUntracked()
					ui.Spring(cx, grab, 1, core.SpringConfig{Stiffness: 420, Damping: 24})
					st.Set(state{tr.GestDragging(), ToneAccent})
				},
				OnMove: func(e core.ResponderEvent) {
					pos.Set(core.Point{X: start.X + e.Gesture.Delta.X, Y: start.Y + e.Gesture.Delta.Y})
				},
				OnRelease: func(e core.ResponderEvent) {
					ui.Spring(cx, grab, 0, core.SpringConfig{Stiffness: 300, Damping: 20})
					v := e.Gesture.Velocity
					st.Set(state{fmt.Sprintf("%.2f, %.2f pt/ms", v.X/1000, v.Y/1000), ToneInfo})
				},
				OnTerminate: func(core.ResponderEvent) {
					ui.Spring(cx, grab, 0, core.SpringConfig{})
					st.Set(state{tr.GestTakenOver(), ToneWarn})
				},
			}, StyleFunc: func() core.Style {
				p := pos.Get()
				return core.Style{Position: core.Absolute, Left: core.Pct(50), Top: core.Pt(70), Margin: core.Edges{Left: -30}, Width: core.Pt(60), Height: core.Pt(60),
					CornerRadius: 30, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: colors.Info(), Shadows: knobShadows(cx),
					Transform: core.Transform{TranslateX: p.X, TranslateY: p.Y},
					Animated: &core.AnimatedStyle{
						Background: core.Interpolate(grab, core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{colors.Info(), colors.AccentFill()}}),
						Scale:      ui.Lerp(grab, 1, 1.15),
					}}
			}, Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(18), Height: core.Pt(18), CornerRadius: 9, Background: core.RGBA(0x1e1408, 0.35)}}}},
		}},
		Caption{Label: tr.GestPosition(), Note: func() string { p := pos.Get(); return fmt.Sprintf("%.0f, %.0f", p.X, p.Y) }},
		ui.Button{Text: tr.GestCenter(), Variant: ui.ButtonSecondary, OnPress: func() { pos.Set(core.Point{}) }},
	}}
}

type finger struct {
	id   int
	x, y core.Points
}

// touchPad is the raw touch events, every finger drawn where it is.
func touchPad(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	type entry struct{ name, text string }
	log := core.NewSignalFunc(cx, []entry(nil), slices.Equal)
	fingers := core.NewSignalFunc(cx, []finger(nil), slices.Equal)
	names := map[core.TouchPhase]string{core.TouchStart: "start", core.TouchMove: "move", core.TouchEnd: "end", core.TouchCancel: "cancel"}
	return Card{Title: tr.GestTouchEvents(), Description: Hint(cx, tr.GestStartMoveEndAndCancel(), tr.GestStartMoveEndAndCancel2()), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
		n := len(fingers.Get())
		t := ToneNeutral
		if n > 0 {
			t = ToneAccent
		}
		return ui.Badge{Text: tr.GestDown(n), Variant: t.Badge()}
	})}, Children: []core.Node{
		core.View{TestID: "touch-pad", OnTouch: func(e core.TouchEvent) {
			var down []finger
			var at core.Touch
			for _, t := range e.Touches {
				if t.Changed {
					at = t
				}
				if e.Phase == core.TouchCancel || (e.Phase == core.TouchEnd && t.Changed) {
					continue
				}
				down = append(down, finger{t.ID, t.Position.X, t.Position.Y})
			}
			fingers.Set(down)
			log.Update(func(l []entry) []entry {
				out := append([]entry{{names[e.Phase], tr.GestTouchLine(fmt.Sprintf("%.0f, %.0f", at.Position.X, at.Position.Y), fmt.Sprintf("%.0f, %.0f", at.PagePosition.X, at.PagePosition.Y), len(down))}}, l...)
				return out[:min(len(out), 4)]
			})
		}, StyleFunc: centered(cx, 150), Children: []core.Node{
			dots(cx),
			core.Show{When: func() bool { return len(fingers.Get()) == 0 }, Then: func(*core.Scope) core.Node {
				return ui.Text(Hint(cx, tr.GestTouchHereTryTwoFingers(), tr.GestPressAndDragHereWith()), 13, ui.Regular, colors.Muted)
			}},
			core.Each(fingers.Get, func(f finger) int { return f.id }, func(cx *core.Scope, f func() finger, _ func() int) core.Node {
				colors := ui.ColorsOf(cx)
				tints := ui.TintsOf(cx)
				return core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Left: core.Pt(f().x - 26), Top: core.Pt(f().y - 26), Width: core.Pt(52), Height: core.Pt(52), CornerRadius: 26,
						BorderWidth: 2, BorderColor: colors.Accent(), Background: tints.Accent()}
				}}
			}),
		}},
		core.View{Style: core.Style{Gap: 4, MinHeight: core.Pt(72)}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
			l := log.Get()
			if len(l) == 0 {
				return ui.Text(tr.GestNoTouchesYet(), 13, ui.Regular, colors.Faint)
			}
			var rows []core.Node
			for i, e := range l {
				c, n := colors.Faint, colors.Faint
				if i == 0 {
					c, n = colors.Text, colors.Accent
				}
				rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
					core.Text{Content: e.name, Style: core.Style{Width: core.Pt(52), FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(n)},
					core.Text{Content: e.text, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(c)},
				}})
			}
			return core.Group(rows)
		})}},
	}}
}

// pressEvents lights each press event's chip as it arrives.
func pressEvents(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	names := []string{"pressIn", "press", "longPress", "pressOut"}
	flashes := make([]*core.AnimatedValue, len(names))
	for i := range flashes {
		flashes[i] = core.NewAnimatedValue(cx, 0)
	}
	n := core.NewSignal(cx, 0)
	log := core.NewSignalFunc(cx, []string(nil), slices.Equal)
	fire := func(name string) {
		n.Update(func(c int) int { return c + 1 })
		log.Update(func(l []string) []string {
			out := append([]string{name}, l...)
			return out[:min(len(out), 6)]
		})
		if i := slices.Index(names, name); i >= 0 {
			flashes[i].Set(1)
			ui.Timing(cx, flashes[i], 0, core.TimingConfig{Duration: 700 * time.Millisecond, Easing: core.Easing{Kind: core.EasingQuad, Mode: core.EaseModeOut}})
		}
	}
	var chips []core.Node
	for i, name := range names {
		chips = append(chips, core.View{StyleFunc: func() core.Style {
			return core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Height: core.Pt(30), CornerRadius: 15, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised(), Shadows: fin.Raised().Shadows,
				Animated: &core.AnimatedStyle{Background: core.Interpolate(flashes[i], core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{colors.Raised(), colors.AccentFill()}})}}
		},
			Children: []core.Node{core.Text{Content: name, Style: core.Style{FontSize: 11, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)}}})
	}
	return Card{Title: tr.GestPressEvents(), Description: tr.GestInPressLongPressAnd(), Actions: []core.Node{ui.Badge{TextFunc: func() string { return itoa(n.Get()) }}}, Children: []core.Node{
		core.View{Role: core.RoleButton, Label: tr.GestPressEvents(), TestID: "press-pad",
			OnPressDown: func(core.PressEvent) { fire("pressIn") }, OnPressUp: func(core.PressEvent) { fire("pressOut") },
			OnPress: func(core.PressEvent) { fire("press") }, OnLongPress: func(core.PressEvent) { fire("longPress") },
			StyleFunc: func() core.Style {
				return fin.Light().Over(core.Style{Height: core.Pt(64), CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, Background: colors.Light(),
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Active: &core.Style{Opacity: core.Alpha(0.55)}})
			},
			Children: []core.Node{ui.Text(Hint(cx, tr.GestTapHoldOrSlideOff(), tr.GestClickHoldOrSlideOff()), 13, ui.Semibold, colors.OnLight)}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 6}, Children: chips},
		core.Text{ContentFunc: func() string {
			if l := log.Get(); len(l) > 0 {
				return strings.Join(l, " · ")
			}
			return tr.GestNoEventsYet()
		}, MaxLines: 1, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
		core.View{Style: core.Style{Direction: core.Row, Gap: 10}, Children: []core.Node{
			core.View{Role: core.RoleButton, Label: tr.GestRipple(), OnPress: func(core.PressEvent) { fire("ripple") }, OnLongPress: func(core.PressEvent) { fire(tr.GestFastLongPress()) },
				DelayLongPress: 250 * time.Millisecond, Android: core.AndroidViewProps{Ripple: &core.Ripple{Color: ui.Alpha(colors.AccentFill(), 0.25)}},
				StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{FlexGrow: 1, Height: core.Pt(56), CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, Background: colors.Raised(),
						BorderWidth: 1, BorderColor: colors.Border(), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 2, Active: &core.Style{Background: colors.Border()}})
				},
				Children: []core.Node{ui.Text(tr.GestRipple(), 13, ui.Regular, colors.Text), ui.Text(tr.GestLongPressAfter250Ms(), 12, ui.Regular, colors.Faint)}},
			core.View{Role: core.RoleButton, Label: tr.GestStar(), OnPress: func(core.PressEvent) { fire("borderless") },
				Android:  core.AndroidViewProps{Ripple: &core.Ripple{Borderless: true, Radius: 40}},
				Style:    core.Style{Width: core.Pt(56), Height: core.Pt(56), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Active: &core.Style{Opacity: core.Alpha(0.4)}},
				Children: []core.Node{ui.Icon(lucide.Star, 24, colors.Warn)}},
		}},
		core.Text{Content: tr.GestPressableTextDimmedWhilePressed(), OnPress: func(core.PressEvent) { fire("text") }, Role: core.RoleLink,
			Style: core.Style{AlignSelf: core.AlignStart, FontSize: 13, Padding: core.Symmetric(12, 0)}, StyleFunc: ui.Colored(colors.Accent)},
	}}
}

// hitSlop is a tiny dot with a ring showing the 30 pt around it that
// also takes taps.
func hitSlop(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	taps := core.NewSignal(cx, 0)
	ring := core.NewAnimatedValue(cx, 0)
	sine := core.Easing{Kind: core.EasingSin, Mode: core.EaseModeInOut}
	run := cx.StartAnimation(core.Loop(core.Sequence(
		core.Timing(ring, 1, core.TimingConfig{Duration: 1400 * time.Millisecond, Easing: sine}),
		core.Timing(ring, 0, core.TimingConfig{Duration: 1400 * time.Millisecond, Easing: sine}),
	), 0), nil)
	cx.Cleanup(run.Stop)
	return Card{Title: tr.GestHitSlop(), Description: Hint(cx, tr.GestA12PtDotThat(), tr.GestA12PtDotThat2()), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
		t := ToneNeutral
		if taps.Get() > 0 {
			t = ToneAccent
		}
		return ui.Badge{Text: tr.GestTaps(taps.Get()), Variant: t.Badge()}
	})}, Children: []core.Node{
		core.View{StyleFunc: centered(cx, 140), Children: []core.Node{
			dots(cx),
			core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Width: core.Pt(72), Height: core.Pt(72), CornerRadius: 36,
					BorderWidth: 1, BorderStyle: core.BorderDashed, BorderColor: colors.Danger(), Background: tints.Danger(),
					Animated: &core.AnimatedStyle{Opacity: ui.Lerp(ring, 0.45, 1), Scale: ui.Lerp(ring, 0.96, 1.04)}}
			}},
			ui.Press{HitSlop: 30, Scale: 0.8, OnPress: func() { taps.Update(func(n int) int { return n + 1 }) }, Label: tr.GestTinyDot(), Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(12), Height: core.Pt(12), CornerRadius: 6},
					StyleFunc: func() core.Style { return core.Style{Background: colors.Danger(), Shadows: knobShadows(cx)} }},
			}},
		}},
		APILine{Label: Hint(cx, tr.GestTheDashedRingIsThe(), tr.GestTheDashedRingIsThe2()), Names: "HitSlop: All(30)"},
	}}
}

// pointerEventsCard is an overlay that lets taps through to the button
// under it.
func pointerEventsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	through := core.NewSignal(cx, 0)
	warn := colors.Warn
	hatch := func() core.Color { return ui.Alpha(warn(), 0.28) }
	return Card{Title: tr.GestPointerEvents(), Description: Hint(cx, tr.GestAnOverlayThatLetsTaps(), tr.GestAnOverlayThatLetsClicks()), Actions: []core.Node{ui.Badge{TextFunc: func() string { return itoa(through.Get()) }, Variant: ui.BadgeUp}}, Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(88)}, Children: []core.Node{
			ui.Press{OnPress: func() { through.Update(func(n int) int { return n + 1 }) }, Scale: 0.98, Label: tr.GestButtonUnderAnOverlay(), Style: core.Style{FlexGrow: 1}, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return core.Style{FlexGrow: 1, CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, Background: tints.Up(), BorderWidth: 1, BorderColor: colors.Up(),
						AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
				}, Children: []core.Node{ui.Text(tr.GestButtonUnderAnOverlay(), 13, ui.Semibold, colors.Up)}},
			}},
			core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
				Children: []core.Node{core.Canvas{Style: core.Style{FlexGrow: 1}, Draw: func(d *core.Drawing, size core.Size) {
					w, h := float32(size.Width), float32(size.Height)
					// A hatched band across the middle, framed with dashes.
					d.Save().BeginPath().Rect(w*0.18, 0, w*0.64, h).Clip()
					d.SetStrokeColor(hatch()).SetLineWidth(1).BeginPath()
					for i := -h; i < w; i += 8 {
						d.MoveTo(i, h).LineTo(i+h, 0)
					}
					d.Stroke().Restore()
					d.SetLineDash(4, 4).SetStrokeColor(warn()).BeginPath().RoundRect(w*0.18+0.5, 0.5, w*0.64-1, h-1, 10).Stroke()
				}}}},
		}},
		APILine{Label: Hint(cx, tr.GestTapsOnTheHatchedBand(), tr.GestClicksOnTheHatchedBand()), Names: "PointerEventsNone"},
	}}
}

// hover is the mouse over a view: it comes in, moves and leaves.
func hover(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	over := core.NewSignal(cx, false)
	at := core.NewSignal(cx, core.Point{})
	return Card{Title: tr.GestHover(), Description: Hint(cx, tr.GestAPointerOverAView(), tr.GestTheMouseOverAView()), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
		if over.Get() {
			return ui.Badge{Text: tr.GestOver(), Variant: ui.BadgeAccent}
		}
		return ui.Badge{Text: tr.GestAway()}
	})}, Children: []core.Node{
		core.View{TestID: "hover-pad", OnMouseEnter: func(core.MouseEvent) { over.Set(true) }, OnMouseLeave: func(core.MouseEvent) { over.Set(false) },
			OnMouseMove: func(e core.MouseEvent) { at.Set(e.Position) },
			StyleFunc: func() core.Style {
				return stage(cx, 100).Merge(core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Hover: &core.Style{Background: colors.Surface()}})
			},
			Children: []core.Node{ui.Textf(func() string {
				if !over.Get() {
					return tr.GestPointerAway()
				}
				p := at.Get()
				return tr.GestPointerAt(fmt.Sprintf("%.0f, %.0f", p.X, p.Y))
			}, 13, ui.Regular, colors.Muted)}},
		APILine{Label: tr.GestInMoveAndOut(), Names: "OnMouseEnter · OnMouseMove · OnMouseLeave"},
	}}
}

// fileDrop is files dragged in from other apps, on desktops and the web.
func fileDrop(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	over := core.NewSignal(cx, false)
	dropped := core.NewSignalFunc(cx, []string(nil), slices.Equal)
	return Card{Title: tr.GestDroppedFiles(), Description: tr.GestFilesDraggedInFromOther(), Actions: []core.Node{ui.Badge{TextFunc: func() string { return tr.GestFiles(len(dropped.Get())) }}}, Children: []core.Node{
		core.View{TestID: "drop-zone", Label: tr.GestDropFilesHere(),
			OnFileDragEnter: func() { over.Set(true) }, OnFileDragLeave: func() { over.Set(false) },
			OnFileDrop: func(uris []string) { over.Set(false); dropped.Set(uris) },
			StyleFunc: func() core.Style {
				s := stage(cx, 120).Merge(core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 2, BorderStyle: core.BorderDashed})
				if over.Get() {
					s.BorderColor, s.Background = colors.Accent(), tints.Accent()
				}
				return s
			},
			Children: []core.Node{ui.Textf(func() string {
				if over.Get() {
					return tr.GestReleaseToDrop()
				}
				return Hint(cx, tr.GestDesktopsAndTheWebTake(), tr.GestDropFilesHere())
			}, 13, ui.Regular, colors.Muted)}},
		core.Text{TestID: "dropped", ContentFunc: func() string {
			uris := dropped.Get()
			if len(uris) == 0 {
				return tr.GestNothingDroppedYet()
			}
			names := make([]string, len(uris))
			for i, u := range uris {
				names[i] = fileName(u)
			}
			return strings.Join(names, ", ")
		}, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
		APILine{Label: tr.GestOnTheWebBlobUrls(), Names: "OnFileDrop"},
	}}
}

// fileName is the name a dropped file's URL gives it.
func fileName(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	if u.Fragment != "" {
		return u.Fragment
	}
	return path.Base(u.Path)
}

// accessibleRows are elements labelled for VoiceOver and TalkBack.
func accessibleRows(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	last := core.NewSignal(cx, "")
	items := []struct {
		label, hint string
		icon        func() *core.SVG
	}{{tr.GestPlay(), tr.GestStartsTheSong(), lucide.Play}, {tr.GestLike(), tr.GestAddsToFavorites(), lucide.Star}, {tr.GestNotify(), tr.GestTurnsOnAlerts(), lucide.Bell}}
	var rows []core.Node
	for i, it := range items {
		rows = append(rows, core.View{Accessible: true, Role: core.RoleButton, Label: it.label, Hint: it.hint, OnPress: func(core.PressEvent) { last.Set(it.label) },
			Children: []core.Node{Row{First: i == 0, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node {
					if last.Get() == it.label {
						return core.View{StyleFunc: func() core.Style {
							return fin.Raised().Over(core.Style{Width: core.Pt(34), Height: core.Pt(34), CornerRadius: 10, BorderWidth: 1, BorderColor: colors.Border(),
								Background: colors.Raised(), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter})
						}, Children: []core.Node{ui.Icon(it.icon, 17, colors.Accent)}}
					}
					return IconTile{Icon: it.icon, Size: 34}
				}),
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 1}, Children: []core.Node{ui.Text(it.label, 13, ui.Regular, colors.Text), ui.Text(it.hint, 12, ui.Regular, colors.Muted)}},
				core.Text{Content: "button", Style: core.Style{FontSize: 11, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
			}}}})
	}
	return Card{Title: tr.GestAccessibility(), Description: tr.GestRolesLabelsAndHintsFor(), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
		if l := last.Get(); l != "" {
			return ui.Badge{Text: l, Variant: ui.BadgeInfo}
		}
		return core.Group{}
	})}, Children: rows}
}

// gesturesDemo is the platform's gestures: pans, pinches and rotations
// moving values on its main thread, the touch responder, raw touches,
// press events, hit slop, pointer events, hover and dropped files.
func gesturesDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, transformable, ball, hover, touchPad, pressEvents, hitSlop, pointerEventsCard, fileDrop, accessibleRows)
}
