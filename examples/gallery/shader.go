package main

import (
	"fmt"
	"time"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// signalField is Onevra's SignalField background on core.Shader: a sparse
// grid of dots that breathe with slow noise, gather light around the
// pointer and pulse outward where it is pressed, behind its children. Its
// middle stays calmer than its edges, where content usually sits.
//
// Everything that moves is an animated value, so the platform's frame
// clock draws it with no engine work per frame: the drift's clock loops
// and each ripple's age runs on a timing. The glow is set where the
// pointer is with each move, as the batch is drawn the moment it arrives,
// and fades only as the pointer leaves.
type signalField struct {
	// Tint is the dots' resting color, Accent what the pointer and
	// ripples light.
	Tint, Accent func() core.Color
	// Intensity scales the pattern's contrast and Scale its spacing; zero
	// means 1.
	Intensity, Scale func() float64
	Style            core.Style
	StyleFunc        func() core.Style
	Children         []core.Node
}

// maxRipples is how many ripples show at once; a new press replaces the
// oldest.
const maxRipples = 6

const (
	// rippleLife is how long a ripple shows, as the shader fades it.
	rippleLife = 1400 * time.Millisecond
	// timeWrap keeps the clock small enough for float precision; the drift
	// is slow, so its jump once in this period goes unnoticed.
	timeWrap = 3600
	// driftStep is the clock's step the drift moves by. Each step draws
	// the whole field again, so the slow drift moves 20 times a second
	// rather than at the display's rate; ripples spread at the display's.
	driftStep = 0.05
)

var signalProgram = core.MustCompileShader(signalSource)

// ripple is a slot of the field's ripples.
type ripple struct {
	x, y float64
	live bool
}

func (f signalField) Render(cx *core.Scope) core.Node {
	reduce := cx.Env().ReduceMotion()
	clock := core.NewAnimatedValue(cx, 0)
	if !reduce {
		cx.StartAnimation(core.Loop(core.Timing(clock, timeWrap, core.TimingConfig{Duration: timeWrap * time.Second}), 0), nil)
	}
	drift := core.Sub(clock, core.Mod(clock, core.Constant(driftStep)))
	px, py, presence := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	inside := false

	var ages [maxRipples]*core.AnimatedValue
	for i := range ages {
		ages[i] = core.NewAnimatedValue(cx, 0)
	}
	slots := core.NewSignalFunc(cx, [maxRipples]ripple{}, nil)
	next := 0
	press := func(p core.Point) {
		if reduce {
			return
		}
		i := next
		next = (next + 1) % maxRipples
		rs := slots.GetUntracked()
		rs[i] = ripple{x: float64(p.X), y: float64(p.Y), live: true}
		slots.Set(rs)
		ages[i].Set(0)
		life := rippleLife.Seconds()
		cx.StartAnimation(core.Timing(ages[i], life, core.TimingConfig{Duration: rippleLife}), func(finished bool) {
			if finished {
				rs := slots.GetUntracked()
				rs[i].live = false
				slots.Set(rs)
			}
		})
	}
	follow := func(p core.Point) {
		px.Set(float64(p.X))
		py.Set(float64(p.Y))
		if !inside {
			// The glow shows with the first move inside, where the
			// pointer is, in the same batch.
			inside = true
			presence.Set(1)
		}
	}
	leave := func() {
		inside = false
		cx.StartAnimation(core.Timing(presence, 0, core.TimingConfig{Duration: 600 * time.Millisecond}), nil)
	}
	positive := func(fn func() float64) float64 {
		if fn == nil {
			return 1
		}
		return max(fn(), 0.01)
	}

	style := f.Style
	style.Overflow = core.Hidden
	shader := core.Shader{
		Program: signalProgram,
		Style:   core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)},
		Uniforms: core.Uniforms{
			"Time":    drift,
			"Pointer": []core.AnimatedNode{px, py, presence},
		},
		UniformsFunc: func() core.Uniforms {
			rs := slots.Get()
			lanes := make([]any, 0, 4*maxRipples)
			for i, r := range rs {
				w := 0.0
				if r.live {
					w = 1
				}
				lanes = append(lanes, r.x, r.y, ages[i], w)
			}
			return core.Uniforms{"Ripples": lanes, "Intensity": positive(f.Intensity), "Scale": positive(f.Scale),
				"Tint": f.Tint(), "Accent": f.Accent()}
		},
	}
	return core.View{Style: style, StyleFunc: f.StyleFunc,
		// The field only listens: presses still reach its children.
		OnMouseMove: follow,
		OnHoverOut:  leave,
		OnTouch: func(e core.TouchEvent) {
			if e.Phase != core.TouchStart {
				return
			}
			for _, t := range e.Touches {
				if t.Changed {
					press(t.Position)
				}
			}
		},
		Children: append([]core.Node{shader}, f.Children...),
	}
}

// shaderDemo is core.Shader: fragment shaders written in Go's syntax,
// compiled for each platform's GPU, their uniforms animated values the
// platform moves. NSH's gallery has no shaders; this page is NSGo's.
func shaderDemo(cx *core.Scope) core.Node {
	background := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		presses := core.NewSignal(cx, 0)
		scale, intensity := core.NewSignal(cx, 1.0), core.NewSignal(cx, 1.0)
		cycle := func(s *core.Signal[float64], values ...float64) func() {
			return func() {
				cur := s.GetUntracked()
				for i, v := range values {
					if v == cur {
						s.Set(values[(i+1)%len(values)])
						return
					}
				}
				s.Set(values[0])
			}
		}
		field := signalField{
			Tint: colors.Muted, Accent: colors.Accent, Scale: scale.Get, Intensity: intensity.Get,
			StyleFunc: func() core.Style {
				return core.Style{Height: core.Pt(320), CornerRadius: ui.RadiusControl, Background: colors.Background(), BorderWidth: 1, BorderColor: colors.Line(),
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 12, Padding: core.All(16)}
			},
			Children: []core.Node{
				core.Text{Content: tr.ShaderSignalField(), Role: core.RoleHeader, Style: core.Style{FontSize: 22, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{Content: Hint(cx, tr.ShaderPressAnywhereOnTheField(), tr.ShaderMoveThePointerOverThe()),
					Style: core.Style{FontSize: 13, TextAlign: core.TextAlignCenter}, StyleFunc: ui.Colored(colors.Muted)},
				ui.Button{Text: tr.ShaderStart(), Variant: ui.ButtonAccent, OnPress: func() { presses.Update(func(n int) int { return n + 1 }) }},
				ui.Textf(func() string { return tr.ShaderStartPressed(presses.Get()) }, 13, ui.Regular, colors.Faint),
			},
		}
		return ui.Card{Title: tr.ShaderBackground(), Description: tr.ShaderAClockAndRipplesThe(), Children: []core.Node{
			field,
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Cell{Child: ui.Button{Text: tr.ShaderScale(), Variant: ui.ButtonSecondary, OnPress: cycle(scale, 1, 1.5, 0.75)}},
				ui.Cell{Child: ui.Button{Text: tr.ShaderIntensity(), Variant: ui.ButtonSecondary, OnPress: cycle(intensity, 1, 2, 0.5)}},
			}},
			Caption{Label: tr.ShaderUniforms(), Note: func() string {
				return tr.ShaderScaleIntensity(fmt.Sprintf("%.2g", scale.Get()), fmt.Sprintf("%.2g", intensity.Get()))
			}},
		}}
	}
	images := func(cx *core.Scope) core.Node {
		tr :=
			// A wave runs through the logo, read as the shader's image.
			locales.For(cx)

		wave := core.NewAnimatedValue(cx, 0)
		run := cx.StartAnimation(core.Loop(core.Timing(wave, 2*3.14159265, core.TimingConfig{Duration: 2 * time.Second}), 0), nil)
		cx.Cleanup(run.Stop)
		return ui.Card{Title: tr.ShaderImages(), Description: tr.ShaderWhatImage0atToImage3atRead(), Children: []core.Node{
			core.View{Style: core.Style{AlignItems: core.AlignCenter, Padding: core.Symmetric(8, 0)}, Children: []core.Node{
				core.Shader{Program: rippleProgram, Images: []core.ImageSource{{Data: logoPNG}}, Uniforms: core.Uniforms{"Phase": wave},
					Label: tr.ShaderRipplingLogo(), TestID: "image-shader",
					Style: core.Style{Width: core.Pt(160), Height: core.Pt(160), CornerRadius: 12, Overflow: core.Hidden}},
			}},
			APILine{Label: tr.ShaderItsRowsWaveWithA(), Names: "Shader.Images"},
		}}
	}
	source := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Card{Title: tr.ShaderTheSource(), Description: tr.ShaderGoSSyntaxCheckedAnd(), Children: []core.Node{
			core.Scroll{Horizontal: true, Children: []core.Node{core.Text{Content: rippleSource,
				Style: core.Style{FontFamily: ui.Mono, FontSize: 12, LineHeight: 18}, StyleFunc: ui.Colored(colors.Code)}}},
			APILine{Label: tr.ShaderCompiledOnceAsTheProgram(), Names: "MustCompileShader"},
		}}
	}
	return demoPage(cx, pageProps{}, background, images, source)
}

// rippleProgram stretches image 0 over the view, its rows waving.
var rippleProgram = core.MustCompileShader(rippleSource)

// rippleSource is the image shader's source.
const rippleSource = `package main

var Phase float

func Fragment(dst vec4, pos vec2) vec4 {
	uv := pos / rectSize()
	uv.x += 0.03 * sin(uv.y*12 + Phase)
	return image0At(uv * image0Size())
}
`

// signalSource is Onevra's SignalField shader.
const signalSource = `package main

var Time float
var Pointer vec3
var Ripples [6]vec4
var Tint vec4
var Accent vec4
var Intensity float
var Scale float

func hash(p vec2) float {
	p3 := fract(vec3(p.x, p.y, p.x) * 0.1031)
	p3 += dot(p3, p3.yzx+33.33)
	return fract((p3.x + p3.y) * p3.z)
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3 - 2*f)
	a := hash(i)
	b := hash(i + vec2(1, 0))
	c := hash(i + vec2(0, 1))
	d := hash(i + vec2(1, 1))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	spacing := 22 * Scale
	center := (floor(pos/spacing) + 0.5) * spacing

	// Two slow octaves of noise breathe across the field.
	t := Time
	n := noise(center/(260*Scale)+vec2(t*0.035, t*0.021))*0.65 + noise(center/(110*Scale)-vec2(t*0.05, -t*0.03))*0.35
	ambient := smoothstep(0.3, 0.85, n)

	// The middle, where content sits, stays calmer than the edges.
	edge := length((center - size*0.5) / (size * 0.5))
	calm := mix(0.45, 1.0, smoothstep(0.2, 1.1, edge))

	d := center - Pointer.xy
	glow := Pointer.z * exp(-dot(d, d)/(2*150*150))

	// Ripples light the dots on an expanding ring and push them outward.
	ring := 0.0
	push := vec2(0)
	for i := 0; i < 6; i++ {
		rp := Ripples[i]
		if rp.w > 0 {
			v := center - rp.xy
			dist := length(v)
			fade := 1 - rp.z/1.4
			k := (dist - rp.z*520) / 34
			band := exp(-k*k) * fade * fade * rp.w
			ring += band
			if dist > 0.001 {
				push += v / dist * band
			}
		}
	}
	ring = min(ring, 1.0)
	shift := push * spacing * 0.22
	if length(shift) > spacing*0.3 {
		shift = normalize(shift) * spacing * 0.3
	}

	radius := (0.45 + 0.35*ambient + 0.7*glow + 0.9*ring) * min(Scale, 1.6)
	cover := 1 - smoothstep(radius-0.6, radius+0.6, distance(pos, center+shift))
	a := clamp(Intensity*(0.05+0.10*ambient*calm+0.32*glow+0.4*ring), 0.0, 1.0) * cover * Tint.a
	col := mix(Tint.rgb, Accent.rgb, clamp(glow*0.9+ring, 0.0, 1.0))

	// A faint light under the pointer and ripples, between the dots.
	pd := pos - Pointer.xy
	h := Intensity * Accent.a * (0.035*Pointer.z*exp(-dot(pd, pd)/(2*190*190)) + 0.025*ring)
	return vec4(col*a, a) + vec4(Accent.rgb*h, h)*(1-a)
}
`
