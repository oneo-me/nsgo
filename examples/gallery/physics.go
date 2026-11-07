package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"math"
	"time"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

const (
	physicsCard core.Points = 128
	// shortWindow is the height below which the demo scrolls, with its
	// stage shortStage tall: below it the stage and the controls under it
	// no longer fit.
	shortWindow core.Points = 640
	shortStage  core.Points = 300
)

// feels are the springs a release uses.
var feels = map[string]core.SpringConfig{
	"soft":   {Stiffness: 90, Damping: 12},
	"bouncy": {Stiffness: 180, Damping: 11},
	"firm":   {Stiffness: 420, Damping: 32},
}

// physicsDemo is a card dragged on the platform's main thread (PanValues)
// and released into a spring, a decay or the nearest corner, with the
// drag's velocity. A canvas draws a tether to it from the same animated
// values, with no engine work per frame.
func physicsDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	fin := ui.FinOf(cx)
	env := cx.Env()
	x, y := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	pressed, lift := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	mode, feel := core.NewSignal(cx, "spring"), core.NewSignal(cx, "bouncy")
	stageSize := core.NewSignal(cx, core.Size{})
	releases, speed := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
	corner := core.NewSignal(cx, "—")

	settle := func(v *core.AnimatedValue, to, velocity float64) {
		c := feels[feel.GetUntracked()]
		c.Velocity = velocity
		ui.Spring(cx, v, to, c)
	}
	corners := func() core.Point {
		s := stageSize.Get()
		return core.Point{X: max(0, s.Width/2-physicsCard/2-14), Y: max(0, s.Height/2-physicsCard/2-14)}
	}
	release := func(e core.PanEvent) {
		ui.Spring(cx, lift, 0, core.SpringConfig{Stiffness: 300, Damping: 24})
		releases.Update(func(n int) int { return n + 1 })
		vx, vy := float64(e.Velocity.X), float64(e.Velocity.Y)
		speed.Set(int(math.Round(math.Hypot(vx, vy))))
		switch mode.GetUntracked() {
		case "spring":
			settle(x, 0, vx)
			settle(y, 0, vy)
			corner.Set("—")
		case "coast":
			// Coast with the release velocity, then spring home unless
			// grabbed again.
			for _, a := range []struct {
				v *core.AnimatedValue
				s float64
			}{{x, vx}, {y, vy}} {
				cx.StartAnimation(core.Decay(a.v, core.DecayConfig{Velocity: a.s, Deceleration: 0.992}), func(done bool) {
					if done {
						settle(a.v, 0, 0)
					}
				})
			}
			corner.Set("—")
		case "snap":
			// Where the fling would carry it, then the nearest corner.
			px, py := float64(e.Values.X)+vx*0.2, float64(e.Values.Y)+vy*0.2
			c := core.Untracked(cx, corners)
			sx, sy, h, v := float64(c.X), float64(c.Y), "right", "bottom"
			if px < 0 {
				sx, h = -sx, "left"
			}
			if py < 0 {
				sy, v = -sy, "top"
			}
			settle(x, sx, vx)
			settle(y, sy, vy)
			corner.Set(v + " " + h)
		}
	}
	pulse := func() {
		cx.StartAnimation(core.Sequence(
			core.Timing(pressed, 1, core.TimingConfig{Duration: 120 * time.Millisecond, Easing: core.BezierEasing(core.EaseOut)}),
			core.Timing(pressed, 0, core.TimingConfig{Duration: 320 * time.Millisecond, Easing: core.BezierEasing(core.EaseInOut)}),
		), nil)
	}
	reset := func() {
		releases.Set(0)
		speed.Set(0)
		corner.Set("—")
		settle(x, 0, 0)
		settle(y, 0, 0)
	}

	line, border, faint, accent := colors.Line, colors.Border, colors.Faint, colors.Accent
	tether := core.Canvas{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
		Draw: func(d *core.Drawing, size core.Size) {
			w, h := float32(size.Width), float32(size.Height)
			cxp, cyp := w/2, h/2
			d.SetFillColor(line())
			for gx := float32(10); gx < w; gx += 16 {
				for gy := float32(10); gy < h; gy += 16 {
					d.FillRect(gx, gy, 1.5, 1.5)
				}
			}
			d.SetLineDash(3, 5).SetStrokeColor(border()).SetLineWidth(1).BeginPath().MoveTo(cxp, 0).LineTo(cxp, h).MoveTo(0, cyp).LineTo(w, cyp).Stroke()
			if mode.Get() == "snap" {
				c := corners()
				d.SetStrokeColor(faint())
				for _, sx := range []float32{-1, 1} {
					for _, sy := range []float32{-1, 1} {
						d.BeginPath().RoundRect(cxp+sx*float32(c.X)-float32(physicsCard)/2, cyp+sy*float32(c.Y)-float32(physicsCard)/2, float32(physicsCard), float32(physicsCard), 26).Stroke()
					}
				}
			}
			// The tether, from animated values: it follows drags and
			// springs with no engine work.
			d.SetLineDash().SetStrokeColor(accent()).SetAlpha(0.45).SetLineWidth(1.5).BeginPath().MoveTo(cxp, cyp).
				LineTo(cxp, cyp).Bind(core.ArgX, core.Add(x, core.Constant(cxp))).Bind(core.ArgY, core.Add(y, core.Constant(cyp))).Stroke()
			d.SetAlpha(1).SetFillColor(accent()).BeginPath().Arc(cxp, cyp, 4, 0, 2*math.Pi, false).Fill()
		}}

	body := func(short bool) core.Node {
		st := func() core.Style {
			st := fin.Inset().Over(core.Style{Overflow: core.Hidden, CornerRadius: ui.RadiusCard, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: colors.Border(),
				Background: colors.Inset(), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter})
			if short {
				st.Height = core.Pt(shortStage)
			} else {
				st.FlexGrow, st.FlexShrink, st.MinHeight = 1, 1, core.Pt(0)
			}
			return st
		}
		outer := core.Style{Padding: core.All(16), Gap: 12}
		var page func() core.Style
		if !short {
			outer = fillPage.Merge(outer)
			page = pageStyle(cx)
		}
		return core.View{Style: outer, StyleFunc: page, Children: []core.Node{
			Appear{Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
				IconTile{Icon: lucide.Atom, Size: 40},
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
					ui.Text(tr.PhysicsDragAndRelease(), 15, ui.Semibold, colors.Text),
					core.Text{Content: Hint(cx, tr.PhysicsTapToPulseLongPress(), tr.PhysicsClickToPulsePressAnd()), MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
				}},
				ui.Badge{Text: tr.PhysicsUiThread(), Variant: ui.BadgeAccent},
			}}}},
			core.View{OnLayout: func(b core.Bounds) { stageSize.Set(b.Size) }, StyleFunc: st, Children: []core.Node{
				tether,
				core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Width: core.Pt(physicsCard), Height: core.Pt(physicsCard), CornerRadius: 30,
						Background: tints.Accent(), Animated: &core.AnimatedStyle{Opacity: lift, TranslateX: x, TranslateY: y, Scale: ui.Lerp(lift, 1, 1.32)}}
				}},
				core.View{PanValues: core.AnimatedXY{X: x, Y: y}, TestID: "physics-card",
					OnPan: func(e core.PanEvent) {
						switch e.Phase {
						case core.GestureBegan:
							ui.Spring(cx, lift, 1, core.SpringConfig{Stiffness: 400, Damping: 26})
						case core.GestureEnded, core.GestureCancelled:
							release(e)
						}
					},
					OnPress: func(core.PressEvent) { pulse() }, OnLongPress: func(core.PressEvent) { reset() }, Role: core.RoleButton, Label: tr.PhysicsDraggableCard(),
					StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(physicsCard), Height: core.Pt(physicsCard), CornerRadius: 30, BorderCurve: core.BorderCurveContinuous,
							AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 8, Background: colors.AccentFill(),
							// The accent's highlight and glow; the color itself
							// animates.
							Shadows: fin.Accent().Shadows,
							Animated: &core.AnimatedStyle{
								Background: core.Interpolate(pressed, core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{colors.AccentFill(), colors.Info()}}),
								TranslateX: x, TranslateY: y,
								Scale:  core.Mul(ui.Lerp(pressed, 1, 1.12), ui.Lerp(lift, 1, 1.06)),
								Rotate: core.Interpolate(x, core.Interpolation{Input: []float64{-200, 200}, Output: []float64{-14, 14}}),
							}}
					},
					Children: []core.Node{ui.Icon(lucide.Hand, 28, colors.OnAccent), ui.Text(tr.PhysicsDragMe(), 13, ui.Bold, colors.OnAccent)}},
			}},
			core.View{StyleFunc: func() core.Style { return ui.CardStyle(cx).Merge(core.Style{Padding: core.All(12), Gap: 12}) }, Children: []core.Node{
				ui.Tabs[string]{Equal: true, Options: []ui.Option[string]{{Value: "spring", Label: tr.PhysicsSpring()}, {Value: "coast", Label: tr.PhysicsCoast()}, {Value: "snap", Label: tr.PhysicsSnap()}}, Value: mode.Get, OnChange: mode.Set},
				ui.Tabs[string]{Equal: true, Options: []ui.Option[string]{{Value: "soft", Label: tr.PhysicsSoft()}, {Value: "bouncy", Label: tr.PhysicsBouncy()}, {Value: "firm", Label: tr.PhysicsFirm()}}, Value: feel.Get, OnChange: feel.Set},
				core.View{Style: core.Style{Direction: core.Row, Padding: core.Symmetric(0, 4)}, Children: []core.Node{
					Stat{Label: tr.PhysicsReleases(), Value: func() string { return itoa(releases.Get()) }, Width: core.Pct(30)},
					core.Dynamic(func(*core.Scope) core.Node {
						t := ToneNeutral
						if speed.Get() > 1500 {
							t = ToneWarn
						}
						return Stat{Label: tr.PhysicsSpeed(), Value: func() string { return fmt.Sprintf("%d pt/s", speed.Get()) }, Tone: t, Width: core.Pct(34)}
					}),
					core.Dynamic(func(*core.Scope) core.Node {
						if mode.Get() == "snap" {
							return Stat{Label: tr.PhysicsCorner(), Value: corner.Get, Width: core.Pct(36)}
						}
						return Stat{Label: tr.PhysicsSpring(), Value: func() string { f := feels[feel.Get()]; return fmt.Sprintf("%.0f / %.0f", f.Stiffness, f.Damping) }, Width: core.Pct(36)}
					}),
				}},
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8, Padding: core.Symmetric(0, 4)}, Children: []core.Node{
					core.Text{ContentFunc: func() string {
						switch mode.Get() {
						case "coast":
							return tr.PhysicsCoastsToAStopThen()
						case "snap":
							return tr.PhysicsSpringsToTheCornerThe()
						}
						return tr.PhysicsSpringsHomeWithTheRelease()
					}, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
					core.Text{ContentFunc: func() string {
						if mode.Get() == "coast" {
							return tr.PhysicsDecay()
						}
						return tr.PhysicsSpring()
					}, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
				}},
			}},
		}}
	}
	// Short windows scroll, with a stage of fixed height; the rest fills.
	short := core.NewMemo(cx, func() bool { return env.Window().Height < shortWindow })
	return core.Dynamic(func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		if short.Get() {
			return core.Scroll{Style: core.Style{FlexGrow: 1}, StyleFunc: ui.Filled(colors.Background), Children: []core.Node{body(true)}}
		}
		return body(false)
	})
}
