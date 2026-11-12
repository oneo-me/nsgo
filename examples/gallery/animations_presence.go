package main

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// exits is a card in a Presence: hidden, it stays until its motion's exit
// ends, which the card plays itself.
func exits(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	fin := ui.FinOf(cx)
	shown := core.NewSignal(cx, true)
	tone := motionTones(cx)[1]
	return Card{Title: tr.AnimExits(), Description: tr.AnimWhatARegionHeldLeaves(), Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(96), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
			core.Presence{When: shown.Get, Then: func(cx *core.Scope) core.Node {
				enter := core.FadeInUp.Spec()
				m := core.NewMotion(cx, &enter)
				m.Exit(core.FadeOutDown.Spec())
				return core.View{TestID: "exit-card", StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Height: core.Pt(56), Padding: core.Symmetric(0, 20), CornerRadius: 14,
						BorderCurve: core.BorderCurveContinuous, JustifyContent: core.JustifyCenter, Background: tone(), Shadows: fin.Knob().Shadows,
						Animated: m.Style()}
				}, Children: []core.Node{ui.Text(tr.AnimChangesSaved(), 14, ui.Semibold, onTone(cx))}}
			}},
		}},
		APILine{Label: tr.AnimLeavesWithItsOwnMotion(), Names: "Presence · Scope.OnExit · Motion.Exit"},
		ui.Cell{Child: core.Dynamic(func(*core.Scope) core.Node {
			l := tr.AnimHide()
			if !shown.Get() {
				l = tr.AnimShow()
			}
			return ui.Button{Text: l, TestID: "exit-toggle", Variant: ui.ButtonSecondary, OnPress: func() { toggled(shown) }}
		})},
	}}
}

// sharedElements are tiles that open into a panel: the panel and its tile
// share a MatchID, so the tile grows into it.
func sharedElements(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	fin := ui.FinOf(cx)
	tones := motionTones(cx)
	open := core.NewSignal(cx, -1)
	choose := func(i int) {
		cx.AnimateLayout(core.LayoutSpringPreset)
		open.Set(i)
	}
	const tiles = 4
	tile := func(i int, style core.Style, onPress func()) core.Node {
		return core.View{MatchID: "tile-" + itoa(i), TestID: "tile-" + itoa(i), OnPress: func(core.PressEvent) { onPress() },
			StyleFunc: func() core.Style {
				s := style
				s.CornerRadius, s.BorderCurve, s.Background, s.Shadows = 12, core.BorderCurveContinuous, tones[i%len(tones)](), fin.Knob().Shadows
				s.JustifyContent, s.AlignItems = core.JustifyCenter, core.AlignCenter
				return s
			}, Children: []core.Node{ui.Text(itoa(i+1), 15, ui.Semibold, onTone(cx))}}
	}
	return Card{Title: tr.AnimSharedElements(), Description: tr.AnimViewsOfOneMatchIDAreOne(), Children: []core.Node{
		core.View{Style: core.Style{Gap: 10}, Children: []core.Node{
			core.View{Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				i := open.Get()
				if i < 0 {
					return nil
				}
				return tile(i, core.Style{Height: core.Pt(120)}, func() { choose(-1) })
			})}},
			core.View{Style: core.Style{Direction: core.Row, Gap: 10}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				shown := open.Get()
				var row []core.Node
				for i := range tiles {
					if i != shown {
						row = append(row, tile(i, core.Style{Width: core.Pt(48), Height: core.Pt(48)}, func() { choose(i) }))
					}
				}
				return core.Group(row)
			})}},
		}},
		APILine{Label: tr.AnimATileOpensIntoItsPanel(), Names: "View.MatchID · AnimateLayout"},
	}}
}

// resistingDrags is a knob a drag moves along a track, resisted past its
// ends by a node on the frame clock, and sprung back into it on release.
func resistingDrags(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	const reach, knob = 80, 44
	x := core.NewAnimatedValue(cx, 0)
	shown := core.Resist(x, -reach, reach, 160)
	return Card{Title: tr.AnimResistingDrags(), Description: tr.AnimADragSlowsPastItsEnds(), Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(72), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(2*reach + knob), Height: core.Pt(knob), CornerRadius: knob / 2, BorderWidth: 1},
				StyleFunc: func() core.Style { return core.Style{Background: colors.Raised(), BorderColor: colors.Border()} }},
			core.View{TestID: "resist-knob", PanValues: core.AnimatedXY{X: x},
				OnPan: func(e core.PanEvent) {
					if e.Phase == core.GestureEnded || e.Phase == core.GestureCancelled {
						to := min(max(float64(e.Values.X), -reach), reach)
						ui.Spring(cx, x, to, core.SpringConfig{Stiffness: 320, Damping: 26, Velocity: float64(e.Velocity.X)})
					}
				},
				StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Width: core.Pt(knob), Height: core.Pt(knob), CornerRadius: knob / 2,
						Background: colors.Accent(), Shadows: fin.Knob().Shadows, Animated: &core.AnimatedStyle{TranslateX: shown}}
				}},
		}},
		APILine{Label: tr.AnimOnTheFrameClockNoCommits(), Names: "Resist · PanValues"},
	}}
}
