package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// instrument is a card on the ring: what it is called and what kind of
// part it shows, a line on it, its body and the APIs it is made of.
type instrument struct {
	title, kind, caption, apis string
	body                       func(cx *core.Scope) core.Node
}

var instruments = []instrument{
	{"Resonance", "Shader", "Two sources interfere in a WGSL shader. The slider sets their wavelength.",
		"Shader · CompileWGSL", resonance},
	{"Typeset", "Text", "The platform's text engine sets every keystroke, at any weight.",
		"Input · Text · FontWeight", typeset},
	{"Spring", "Gesture", "Throw the weight. It follows the pointer on the main thread and springs home.",
		"PanValues · Spring", spring},
	{"Switchboard", "Controls", "Native switches. What they light changes on the platform's own transitions.",
		"Switch · Transition", switchboard},
	{"Loupe", "Effect", "A shader magnifies a picture of the plate, and the button under it still takes presses.",
		"ShaderEffect · sample_content", loupe},
	{"Cadence", "Motion", "Twenty-four bars loop out of step, every frame on the platform's clock.",
		"Loop · Stagger · AnimatedStyle", cadence},
}

// card is instrument i's sheet: its number and kind over a rule, its name
// and line, its body and the APIs it is made of. What it holds slides a
// little against the sheet as the card turns, as print under glass would.
func card(cx *core.Scope, i int, in instrument, rel core.AnimatedNode) core.Node {
	parallax := core.Interpolate(rel, core.Interpolation{Input: []float64{-90, 0, 90}, Output: []float64{16, 0, -16}})
	return core.View{Style: core.Style{FlexGrow: 1, CornerRadius: cardRadius, Background: sheet, BorderWidth: 1, BorderColor: rule,
		Overflow: core.Hidden}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, Padding: core.Edges{Top: 20, Right: 24, Bottom: 18, Left: 24}, Gap: 16,
			Animated: &core.AnimatedStyle{TranslateX: parallax}}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
				mono(fmt.Sprintf("%02d", i+1), signal),
				core.View{Style: core.Style{FlexGrow: 1, Height: core.Pt(1), Background: rule}},
				mono(strings.ToUpper(in.kind), ink3),
			}},
			core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
				core.Text{Content: in.title, Role: core.RoleHeader, Style: core.Style{FontFamily: serif, FontSize: 30, Color: ink}},
				core.Text{Content: in.caption, Style: core.Style{FontSize: 12.5, LineHeight: 18, Color: ink2}},
			}},
			core.View{Style: core.Style{FlexGrow: 1, Gap: 12}, Children: []core.Node{in.body(cx)}},
			core.View{Style: core.Style{Borders: core.Edges{Top: 1}, BorderColor: rule, Padding: core.Edges{Top: 10}}, Children: []core.Node{
				mono(in.apis, ink3),
			}},
		}},
	}}
}

// plate is the recess an instrument draws in.
func plate(children ...core.Node) core.Node {
	return core.View{Style: core.Style{FlexGrow: 1, CornerRadius: 3, Overflow: core.Hidden, Background: recess,
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: children}
}

// reading is a label and its value, over a control.
func reading(label string, value func() string) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween, AlignItems: core.AlignBaseline}, Children: []core.Node{
		core.Text{Content: label, Style: core.Style{FontSize: 12, Color: ink2}},
		core.Text{ContentFunc: value, Style: core.Style{FontSize: 11, FontFamily: ui.Mono, Color: ink}},
	}}
}

// filling is a box over the whole of its parent.
var filling = core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)}

func resonance(cx *core.Scope) core.Node {
	wave := core.NewSignal(cx, 11.0)
	return core.Group{
		plate(core.Shader{Program: resonanceProgram, Label: "Interference of two ring patterns", TestID: "resonance",
			Paused: cx.Env().ReduceMotion(), Style: filling,
			UniformsFunc: func() core.Uniforms {
				return core.Uniforms{"wavelength": wave.Get(), "ink": ink, "paper": recess, "signal": signal}
			}}),
		reading("Wavelength", func() string { return fmt.Sprintf("%.1f pt", wave.Get()) }),
		ui.Slider{Value: wave.Get, OnChange: wave.Set, Min: 7, Max: 18, Step: 0.5, Label: "Wavelength", TestID: "wavelength"},
	}
}

func typeset(cx *core.Scope) core.Node {
	text := core.NewSignal(cx, "Native")
	weight := core.NewSignal(cx, 500.0)
	return core.Group{
		plate(core.Text{TestID: "typeset-preview",
			ContentFunc: func() string {
				if t := text.Get(); t != "" {
					return t
				}
				return "…"
			},
			StyleFunc: func() core.Style {
				return core.Style{FontSize: 46, LineHeight: 50, FontWeight: core.FontWeight(weight.Get()), LetterSpacing: -1, Color: ink,
					NumberOfLines: 2, TextAlign: core.TextAlignCenter, Padding: core.Symmetric(0, 12)}
			}}),
		ui.Input{TestID: "typeset-input", Value: text.Get, OnChange: text.Set, Placeholder: "Type here", MaxLength: 40},
		reading("Weight", func() string {
			return fmt.Sprintf("%.0f · %d glyphs", weight.Get(), utf8.RuneCountInString(text.Get()))
		}),
		ui.Slider{Value: weight.Get, OnChange: weight.Set, Min: 100, Max: 900, Step: 100, Label: "Weight", TestID: "weight"},
	}
}

func spring(cx *core.Scope) core.Node {
	x, y := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	give := core.NewSignal(cx, 40.0)
	throws := core.NewSignal(cx, 0)
	home := func(v core.Point) {
		c := core.SpringConfig{Stiffness: 220, Damping: 6 + (100-give.GetUntracked())/100*24}
		cx.StartAnimation(core.Parallel(
			core.Spring(x, 0, core.SpringConfig{Stiffness: c.Stiffness, Damping: c.Damping, Velocity: float64(v.X)}),
			core.Spring(y, 0, core.SpringConfig{Stiffness: c.Stiffness, Damping: c.Damping, Velocity: float64(v.Y)}),
		), nil)
	}
	hair := func(w, h core.Points) core.Node {
		return core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Width: core.Pt(w), Height: core.Pt(h), Background: rule}}
	}
	// The weight's shadow stays on the plate, wider and fainter the
	// farther the weight is thrown.
	reach := core.Interpolate(core.Add(core.Mul(x, x), core.Mul(y, y)),
		core.Interpolation{Input: []float64{0, 6400}, Output: []float64{1, 1.5}})
	return core.Group{
		plate(
			hair(1, 400), hair(400, 1),
			core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Width: core.Pt(72), Height: core.Pt(72),
				CornerRadius: 36, BorderWidth: 1, BorderColor: rule}},
			core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Width: core.Pt(40), Height: core.Pt(10),
				CornerRadius: 5, Background: core.RGBA(0x1a1915, 0.14), Margin: core.Edges{Top: 58},
				Animated: &core.AnimatedStyle{TranslateX: x, Scale: reach}}},
			core.View{TestID: "weight-ball", Role: core.RoleImage, Label: "Weight", PanValues: core.AnimatedXY{X: x, Y: y},
				OnPan: func(e core.PanEvent) {
					if e.Phase == core.GestureEnded || e.Phase == core.GestureCancelled {
						throws.Update(func(n int) int { return n + 1 })
						home(e.Velocity)
					}
				},
				Style: core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 20, Background: ink,
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
					Animated: &core.AnimatedStyle{TranslateX: x, TranslateY: y}},
				Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(6), Height: core.Pt(6), CornerRadius: 3, Background: signal}}}},
		),
		reading("Give", func() string { return fmt.Sprintf("%.0f%% · %d throws", give.Get(), throws.Get()) }),
		ui.Slider{Value: give.Get, OnChange: give.Set, Min: 0, Max: 100, Step: 5, Label: "Give", TestID: "give"},
	}
}

func switchboard(cx *core.Scope) core.Node {
	names := []string{"Aurora", "Beacon", "Comet", "Drift"}
	on := make([]*core.Signal[bool], len(names))
	rows := make([]core.Node, 0, len(names))
	for i, name := range names {
		on[i] = core.NewSignal(cx, i%2 == 0)
		var edge core.Edges
		if i > 0 {
			edge.Top = 1
		}
		rows = append(rows, core.View{Style: core.Style{FlexGrow: 1, Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12,
			Padding: core.Symmetric(0, 14), Borders: edge, BorderColor: rule},
			Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					s := core.Style{Width: core.Pt(9), Height: core.Pt(9), CornerRadius: 4.5, BorderWidth: 1, BorderColor: ink3,
						Transition: core.Transition{Duration: 220 * time.Millisecond, Easing: quintOut}}
					if on[i].Get() {
						s.Background, s.BorderColor = signal, signal
					}
					return s
				}},
				core.Text{Content: name, Style: core.Style{FlexGrow: 1, FontSize: 13.5, Color: ink}},
				mono(fmt.Sprintf("CH %d", i+1), ink3),
				ui.Switch{Checked: on[i].Get, OnChange: on[i].Set, Label: name, Size: ui.SizeSmall},
			}})
	}
	live := func() int {
		n := 0
		for _, s := range on {
			if s.Get() {
				n++
			}
		}
		return n
	}
	// The meter fills cell by cell, each a beat after the one before.
	meter := make([]core.Node, 2*len(names))
	for i := range meter {
		meter[i] = core.View{StyleFunc: func() core.Style {
			s := core.Style{FlexGrow: 1, Height: core.Pt(10), Background: core.RGBA(0x1a1915, 0.08),
				Transition: core.Transition{Duration: 180 * time.Millisecond, Delay: time.Duration(i) * 30 * time.Millisecond}}
			if i < 2*live() {
				s.Background = ink
			}
			return s
		}}
	}
	return core.Group{
		core.View{Style: core.Style{FlexGrow: 1, CornerRadius: 3, Background: recess}, Children: rows},
		reading("Live", func() string { return fmt.Sprintf("%d of %d", live(), len(names)) }),
		core.View{Style: core.Style{Direction: core.Row, Gap: 3}, Children: meter},
	}
}

func loupe(cx *core.Scope) core.Node {
	presses := core.NewSignal(cx, 0)
	return core.Group{
		core.View{Style: core.Style{FlexGrow: 1, CornerRadius: 3, Overflow: core.Hidden}, Children: []core.Node{
			core.ShaderEffect{Program: loupeProgram, TestID: "loupe", Paused: cx.Env().ReduceMotion(),
				Uniforms: core.Uniforms{"ink": ink}, Style: core.Style{FlexGrow: 1},
				Children: []core.Node{
					core.View{Style: core.Style{FlexGrow: 1, Background: recess, Padding: core.All(16), Gap: 6, JustifyContent: core.JustifyCenter}, Children: []core.Node{
						core.Text{Content: "Aa Gg Rr", Style: core.Style{FontFamily: serif, FontSize: 40, Color: ink}},
						core.Text{Content: "The quick brown fox jumps over the lazy dog. 0123456789",
							Style: core.Style{FontSize: 11, LineHeight: 15, Color: ink2}},
						core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Margin: core.Edges{Top: 8}}, Children: []core.Node{
							ui.Button{Text: "Press", Variant: ui.ButtonPrimary, Size: ui.SizeSmall, TestID: "loupe-press",
								OnPress: func() { presses.Update(func(n int) int { return n + 1 }) }},
							mono("UNDER THE GLASS", ink3),
						}},
					}},
				}},
		}},
		reading("Presses", func() string { return fmt.Sprint(presses.Get()) }),
	}
}

func cadence(cx *core.Scope) core.Node {
	const count = 24
	tempo := core.NewSignal(cx, 84.0)
	levels := make([]*core.AnimatedValue, count)
	for i := range levels {
		levels[i] = core.NewAnimatedValue(cx, 0.12)
	}
	var run *core.AnimationRun
	start := func(bpm float64) {
		if run != nil {
			run.Stop()
		}
		if cx.Env().ReduceMotion() {
			return
		}
		beat := time.Duration(float64(time.Minute) / bpm)
		rise := core.TimingConfig{Duration: beat * 2 / 5, Easing: core.BezierEasing(lift)}
		fall := core.TimingConfig{Duration: beat * 3 / 5, Easing: core.BezierEasing(drop)}
		steps := make([]*core.Animation, count)
		for i, v := range levels {
			v.Set(0.12)
			steps[i] = core.Loop(core.Sequence(core.Timing(v, 1, rise), core.Timing(v, 0.12, fall)), 0)
		}
		run = cx.StartAnimation(core.Stagger(beat/count*2, steps...), nil)
	}
	start(tempo.GetUntracked())

	bars := make([]core.Node, count)
	for i, v := range levels {
		color := ink
		if i == 0 {
			color = signal
		}
		bars[i] = core.View{Style: core.Style{Width: core.Pt(3), Height: core.Pt(120), Background: color,
			TransformOrigin: core.ObjectPosition{Y: core.PositionAxis{Offset: core.Pct(100)}},
			Animated:        &core.AnimatedStyle{ScaleY: v}}}
	}
	return core.Group{
		plate(core.View{TestID: "bars", Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 5, Height: core.Pt(120)}, Children: bars}),
		reading("Tempo", func() string { return fmt.Sprintf("%.0f bpm", tempo.Get()) }),
		ui.Slider{Value: tempo.Get, Min: 50, Max: 160, Step: 2, Label: "Tempo", TestID: "tempo",
			OnChange: func(v float64) { tempo.Set(v); start(v) }},
	}
}
