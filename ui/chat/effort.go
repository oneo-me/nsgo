package chat

import (
	"math"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

// chatEffortTransition is how long the motion takes to settle on a level.
const chatEffortTransition = 900 * time.Millisecond

// chatEffortFrame is how far the trigger's clock steps: its icon is always
// shown, and its halo breathes and its blocks drift slowly.
const chatEffortFrame = 0.033

// The bar's geometry.
const (
	chatEffortBar     core.Points = 18
	chatEffortNameGap core.Points = 8
	chatEffortName    core.Points = 16
	chatEffortPadding core.Points = 14
	chatEffortMinGap  core.Points = 12
)

// The motion's programs, compiled when an effort picker first shows.
var (
	chatEffortPanel = sync.OnceValue(func() *core.ShaderProgram { return core.MustCompileShader(chatEffortPanelSource) })
	chatEffortIcon  = sync.OnceValue(func() *core.ShaderProgram { return core.MustCompileShader(chatEffortIconSource) })
)

// EffortPicker is a chip with an icon, the selected level's name and a
// caret, for the composer's Accessory, that opens a panel with the name
// and a bar above the levels' names, spread from edge to edge; each level
// fills the bar to the end of its name. Pressing or dragging across the
// bar or the names, or the arrows, Home and End, choose a level and keep
// the panel open. The panel widens to fit the names.
//
// The stronger the level, the livelier all three: the icon turns violet
// and gains a breathing halo, then streams blocks of data outward; the
// bar's violet fill gains a glint, then streaming blocks, a glow, flowing
// colors, sparkles and rising sparks; the panel gains a violet wash, then
// drifting aurora, floating motes and a turning border. They are shaders
// on the platform's clock, each fading in or out as the level changes.
// Reduced motion keeps them still.
type EffortPicker struct {
	// Label names the trigger and the bar, and heads the panel; Tooltip is
	// the trigger's hover text, Label when nil.
	Label   string
	Tooltip func() string
	Icon    func() *core.SVG
	// Levels run from the weakest to the strongest; only their ID and
	// Label are used.
	Levels   []Option
	Selected func() string
	OnChange func(id string)
	// Off makes the first level turn reasoning off: the bar empties and
	// the icon dims.
	Off bool
	// Width is the panel's, 260 when 0.
	Width  core.Points
	TestID string
}

// Render implements core.Node.
func (p EffortPicker) Render(cx *core.Scope) core.Node {
	n := len(p.Levels)
	if n == 0 {
		return nil
	}
	colors := tonesOf(cx)
	open := core.NewSignal(cx, false)
	selected := func() int {
		if p.Selected != nil {
			v := p.Selected()
			for i, o := range p.Levels {
				if o.ID == v {
					return i
				}
			}
		}
		return 0
	}
	off := func() bool { return p.Off && selected() == 0 }
	level := func() string { return p.Levels[selected()].Label }
	motion := newChatEffortMotion(cx, func() float64 { return chatEffortStrength(selected(), n, p.Off) }, off, selected, open.Get)
	ink := func() core.Color {
		s := float32(chatEffortStrength(selected(), n, p.Off))
		c := ui.Mix(colors.Muted(), chatEffortInk(cx, s), min(1, s*1.5))
		if off() {
			c = ui.Alpha(c, 0.5)
		}
		return c
	}
	chip := Chip{TextFunc: level, Caret: true, Active: open.Get, Leading: func(cx *core.Scope, size core.Points) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
			core.Shader{Program: chatEffortIcon(), Style: core.Style{Position: core.Absolute, Top: core.Pt(-8), Left: core.Pt(-8), Width: core.Pt(30), Height: core.Pt(30)},
				Uniforms: core.Uniforms{"Strength": motion.strength, "Clock": core.Sub(motion.clock, core.Mod(motion.clock, core.Constant(chatEffortFrame))), "Lively": motion.lively()}},
			ui.Icon(p.Icon, 12, ink),
		}}
	}}
	style, states := chip.shape(cx)
	trigger := base.Popover{Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) }, Placement: core.PlaceTop, Align: core.AlignStart, Offset: 4, Margin: 8,
		Style: style, StyleFunc: states, Trigger: core.Group(chip.face(cx)), Label: p.Label, TestID: p.TestID, PopupStyleFunc: chatPopup(cx),
		PopupStyle: core.Style{Padding: core.All(0), MaxWidth: core.Pt(480)},
		Children:   []core.Node{core.Func(func(cx *core.Scope) core.Node { return p.panel(cx, motion, selected) })}}
	tooltip := p.Tooltip
	if tooltip == nil {
		tooltip = constant(p.Label)
	}
	trigger.Tooltip = chatTooltip(cx, tooltip)
	return trigger
}

// panel is the open panel: the wash, the heading and the bar over a shader
// that fills it.
func (p EffortPicker) panel(cx *core.Scope, motion *chatEffortMotion, selected func() int) core.Node {
	colors := tonesOf(cx)
	n := len(p.Levels)
	// Names keep their selected weight's room, so selecting one does not
	// move the others.
	slots := make([]core.Points, n)
	total := core.Points(0)
	for i, o := range p.Levels {
		slots[i] = core.Points(math.Ceil(float64(cx.MeasureText(o.Label, core.Font{Size: 11.5, Weight: ui.Semibold}).Width))) + 1
		total += slots[i]
	}
	width := p.Width
	if width <= 0 {
		width = 260
	}
	width = max(width, total+core.Points(n-1)*chatEffortMinGap+2*chatEffortPadding)
	inner := width - 2*chatEffortPadding
	// Each level's fill ends where its name does; Off fills nothing.
	ends := make([]float64, n)
	stops := make([]float64, n)
	gap := core.Points(0)
	if n > 1 {
		gap = (inner - total) / core.Points(n-1)
	}
	u := core.Points(0)
	for i := range n {
		stops[i], ends[i] = float64(i), float64((u+slots[i])/inner)
		u += slots[i] + gap
	}
	ends[n-1] = 1
	if p.Off {
		ends[0] = 0
	}
	index := core.NewAnimatedValue(cx, float64(core.Untracked(cx, selected)))
	cx.Effect(func() {
		to := selected()
		cx.StartAnimation(core.Timing(index, float64(to), core.TimingConfig{Duration: chatEffortTransition * 2 / 3, Easing: ui.EaseOutCubic}), nil)
	})
	fill := core.Interpolate(index, core.Interpolation{Input: stops, Output: ends})
	bar := core.NewSignal(cx, core.Bounds{})
	hovered := core.NewSignal(cx, false)
	choose := func(i int) {
		if i >= 0 && i < n && i != core.Untracked(cx, selected) && p.OnChange != nil {
			p.OnChange(p.Levels[i].ID)
		}
	}
	rtl := core.Untracked(cx, cx.Env().RTL)
	at := func(x core.Points) int {
		w := core.Untracked(cx, bar.Get).Size.Width
		if w <= 0 || n < 2 {
			return 0
		}
		f := float64(min(max(x/w, 0), 1))
		if rtl {
			f = 1 - f
		}
		return int(math.Round(f * float64(n-1)))
	}
	var downX core.Points
	focused := core.NewSignal(cx, false)
	step := func(d int) {
		if rtl {
			d = -d
		}
		choose(min(max(core.Untracked(cx, selected)+d, 0), n-1))
	}
	keys := core.Show{When: focused.Get, Then: func(cx *core.Scope) core.Node {
		repeat := core.KeyOptions{Repeat: true}
		cx.OnKeyWith("right", repeat, func() { step(1) })
		cx.OnKeyWith("left", repeat, func() { step(-1) })
		cx.OnKeyWith("up", repeat, func() { choose(min(core.Untracked(cx, selected)+1, n-1)) })
		cx.OnKeyWith("down", repeat, func() { choose(max(core.Untracked(cx, selected)-1, 0)) })
		cx.OnKey("home", func() { choose(0) })
		cx.OnKey("end", func() { choose(n - 1) })
		return nil
	}}
	names := make([]core.Node, n)
	for i, o := range p.Levels {
		align := core.TextAlignCenter
		switch i {
		case 0:
			align = core.TextAlignStart
		case n - 1:
			align = core.TextAlignEnd
		}
		names[i] = core.Text{Content: o.Label, MaxLines: 1, Style: core.Style{Width: core.Pt(slots[i]), FontSize: 11.5, LineHeight: chatEffortName, TextAlign: align},
			StyleFunc: func() core.Style {
				if selected() == i {
					return core.Style{Color: colors.Text(), FontWeight: ui.Semibold}
				}
				return core.Style{Color: colors.Muted()}
			}}
	}
	focus := &core.Ref{}
	if !ui.Touch(cx) {
		cx.Post(focus.Focus)
	}
	control := core.View{Ref: focus, Accessible: true, Role: core.RoleAdjustable, Label: p.Label, TestID: p.testID("-bar"),
		AccessibilityValue: func() core.AccessibilityValue {
			i := selected()
			return core.AccessibilityValue{Min: 0, Max: float32(n - 1), Now: float32(i), Text: p.Levels[i].Label}
		},
		AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
		OnAccessibilityAction: func(name string) {
			i := core.Untracked(cx, selected)
			if name == core.ActionDecrement {
				choose(max(i-1, 0))
			} else {
				choose(min(i+1, n-1))
			}
		},
		OnLayout:    bar.Set,
		OnFocus:     func() { focused.Set(true) },
		OnBlur:      func() { focused.Set(false) },
		OnPress:     func(core.PressEvent) {},
		OnPressDown: func(e core.PressEvent) { downX = e.Position.X; choose(at(downX)) },
		OnHoverIn:   func() { hovered.Set(true) },
		OnHoverOut:  func() { hovered.Set(false) },
		OnPan: func(e core.PanEvent) {
			if e.Phase == core.GestureChanged {
				choose(at(downX + e.Translation.X))
			}
		},
		PanActivation: core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12},
		Style:         core.Style{Width: core.Pt(inner), Height: core.Pt(chatEffortBar + chatEffortNameGap + chatEffortName), CornerRadius: chatEffortBar / 2},
		Children: []core.Node{keys, core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Direction: core.Row, JustifyContent: core.JustifySpaceBetween},
			Children: names}}}
	label := ui.Text(p.Label, 12.5, ui.Regular, colors.Text)
	label.MaxLines = 1
	label.Style.FlexGrow, label.Style.FlexShrink, label.Style.MinWidth = 1, 1, core.Pt(0)
	value := ui.Textf(func() string { return p.Levels[selected()].Label }, 12.5, ui.Semibold, func() core.Color {
		s := float32(chatEffortStrength(selected(), n, p.Off))
		if p.Off && selected() == 0 {
			return colors.Muted()
		}
		return chatEffortInk(cx, s)
	})
	value.Style.FlexShrink = 0
	// The panel covers the popup's border, so the turning border meets the
	// glass's edge.
	return core.View{Style: core.Style{Width: core.Pt(width), Margin: core.All(-1), Padding: core.All(chatEffortPadding), Gap: 10, CornerRadius: ui.RadiusCard, Overflow: core.Hidden}, Children: []core.Node{
		core.Shader{Program: chatEffortPanel(), Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)},
			Uniforms: core.Uniforms{"Fill": fill, "Strength": motion.strength, "Clock": motion.clock, "Lively": motion.lively(), "Rtl": chatFlag(rtl), "Radius": float64(ui.RadiusCard)},
			UniformsFunc: func() core.Uniforms {
				b := bar.Get()
				rest := ui.Alpha(colors.Text(), 0.08)
				if hovered.Get() {
					rest = ui.Alpha(colors.Text(), 0.12)
				}
				return core.Uniforms{"Bar": []any{float64(b.Origin.X), float64(b.Origin.Y), float64(inner), float64(chatEffortBar)},
					"Rest": rest, "Dark": chatFlag(cx.Dark())}
			}},
		core.View{Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{label, value}},
		control,
	}}
}

func (p EffortPicker) testID(suffix string) string {
	if p.TestID == "" {
		return ""
	}
	return p.TestID + suffix
}

func chatFlag(on bool) float64 {
	if on {
		return 1
	}
	return 0
}

// chatEffortMotion is the animated level the icon and the panel draw:
// values the platform moves, so changing the level blends the effects.
type chatEffortMotion struct {
	strength, clock *core.AnimatedValue
	still           bool
}

func (m *chatEffortMotion) lively() float64 { return chatFlag(!m.still) }

// newChatEffortMotion moves strength to its level's and keeps the clock
// running while something shows it: the panel, or the icon's halo. The
// clock's pace rises a little with the level, without moving what it drives.
func newChatEffortMotion(cx *core.Scope, strength func() float64, off func() bool, level func() int, open func() bool) *chatEffortMotion {
	m := &chatEffortMotion{strength: core.NewAnimatedValue(cx, core.Untracked(cx, strength)), clock: core.NewAnimatedValue(cx, 0), still: ui.ReduceMotion(cx)}
	cx.Effect(func() {
		to := strength()
		level()
		cx.StartAnimation(core.Timing(m.strength, to, core.TimingConfig{Duration: chatEffortTransition, Easing: ui.EaseOutCubic}), nil)
	})
	if m.still {
		return m
	}
	const hour = 3600
	var run *core.AnimationRun
	var seconds, pace float64
	var since time.Time
	stop := func() {
		if run != nil {
			run.Stop()
			run = nil
			seconds = math.Mod(seconds+cx.Now().Sub(since).Seconds()*pace, hour)
		}
	}
	cx.Effect(func() {
		s := float32(strength())
		show := open() && s > 0.001 || s > 0.4
		core.Untracked(cx, func() struct{} {
			stop()
			if show {
				pace, since = 1+0.8*float64(chatSmoothstep(0.6, 1, s)), cx.Now()
				m.clock.Set(seconds)
				run = cx.StartAnimation(core.Timing(m.clock, seconds+hour, core.TimingConfig{Duration: time.Duration(hour / pace * float64(time.Second)), Easing: linearEasing}), nil)
			}
			return struct{}{}
		})
	})
	return m
}

// chatEffortStrength places a level from 0, the weakest, or Off and the
// weakest after it, to 1, the strongest.
func chatEffortStrength(selected, n int, off bool) float64 {
	if off {
		selected, n = max(selected-1, 0), n-1
	}
	if n <= 1 {
		return 1
	}
	return float64(selected) / float64(n-1)
}

// chatEffortInk is readable violet text, warming toward fuchsia with
// strength.
func chatEffortInk(cx *core.Scope, strength float32) core.Color {
	if cx.Dark() {
		return ui.Mix(core.RGB(0xa78bfa), core.RGB(0xe879f9), 0.5*strength)
	}
	return ui.Mix(core.RGB(0x7c3aed), core.RGB(0xc026d3), 0.5*strength)
}

func chatSmoothstep(lo, hi, v float32) float32 {
	x := min(max((v-lo)/(hi-lo), 0), 1)
	return x * x * (3 - 2*x)
}

// chatEffortShared is what both programs share: the violets, from indigo
// to fuchsia, and a fixed pseudo-random fraction for an index.
const chatEffortShared = `
func hash(i float) float {
	return fract(sin(i*12.9898+78.233) * 43758.5453)
}

func indigo() vec3 { return vec3(0.388, 0.4, 0.945) }
func violet() vec3 { return vec3(0.545, 0.361, 0.965) }
func purple() vec3 { return vec3(0.659, 0.333, 0.969) }
func fuchsia() vec3 { return vec3(0.851, 0.275, 0.937) }

func tone(i float) vec3 {
	m := mod(i, 4)
	if m < 1 {
		return indigo()
	}
	if m < 2 {
		return violet()
	}
	if m < 3 {
		return fuchsia()
	}
	return purple()
}

// cycle samples the violets at f, so a flow repeats seamlessly.
func cycle(f float) vec3 {
	x := fract(f) * 4
	return mix(tone(floor(x)), tone(floor(x)+1), fract(x))
}

func phase(clock, period float) float {
	return mod(clock, period) / period
}

func over(dst vec4, c vec3, a float) vec4 {
	a = clamp(a, 0, 1)
	return vec4(c*a, a) + dst*(1-a)
}
`

// chatEffortIconSource draws behind the trigger's icon: a breathing halo
// in the upper levels, and at the top blocks of data leaving its edge.
const chatEffortIconSource = `package main

var Strength float
var Clock float
var Lively float
` + chatEffortShared + `
func Fragment(dst vec4, pos vec2) vec4 {
	p := pos - rectSize()/2
	s := Strength
	out := vec4(0)
	halo := smoothstep(0.4, 1, s)
	if halo > 0 {
		pulse := 0.6
		if Lively > 0 {
			pulse = 0.5 + 0.5*sin(Clock*1.3)
		}
		r := 7 + 1.5*pulse
		out = over(out, violet(), halo*(0.12+0.18*pulse)*clamp(r-length(p)+0.5, 0, 1))
	}
	top := smoothstep(0.75, 1, s)
	if top > 0 && Lively > 0 {
		for j := 0; j < 10; j++ {
			fj := float(j)
			seed := hash(fj + 211)
			period := 1.4 + 1.2*seed
			t := Clock + seed*5
			f := phase(t, period)
			trip := mod(floor(t/period), 1000003)
			angle := 6.28318 * hash(fj*19+trip)
			d := 5 + f*(7+3*seed)
			a := top * smoothstep(0, 0.15, f) * (1 - f) * (0.6 + 0.4*seed)
			size := (1.6 + 1.2*seed) * (1 - 0.4*f)
			center := vec2(d*cos(angle), 0.8*d*sin(angle))
			q := abs(p-center) - size/2
			if max(q.x, q.y) < 0 {
				out = over(out, mix(tone(fj), vec3(1), 0.2), a)
			}
		}
	}
	return out
}
`

// chatEffortPanelSource fills the panel: its wash, aurora, motes and
// turning border, and the bar with its fill and effects, in a coordinate
// from the start of the bar that mirrors right to left. The glow stays
// within the panel's padding.
const chatEffortPanelSource = `package main

var Bar vec4
var Rest vec4
var Fill float
var Strength float
var Clock float
var Lively float
var Rtl float
var Dark float
var Radius float
` + chatEffortShared + `
func box(p, center, half vec2, r float) float {
	q := abs(p-center) - half + r
	return length(max(q, vec2(0))) + min(max(q.x, q.y), 0) - r
}

func panel(pos, size vec2) vec4 {
	out := vec4(0)
	s := Strength
	wash := smoothstep(0.2, 1, s)
	if wash <= 0 {
		return out
	}
	boost := mix(1, 1.5, Dark)
	out = over(out, violet(), clamp((pos.y/size.y-0.25)/0.75, 0, 1)*0.1*wash*boost)
	// Aurora: soft glows wandering slowly, more as the level rises.
	for j := 0; j < 3; j++ {
		k := float(j)
		shown := smoothstep(0.45+0.15*k, 1, s)
		if shown <= 0 {
			continue
		}
		at := vec2(0.3+0.2*k, 0.8)
		if Lively > 0 {
			at = vec2(0.5+0.4*sin(Clock*0.23+k*2.1), 0.7+0.25*sin(Clock*0.31+k*1.3))
		}
		d := length(pos - at*size)
		c := violet()
		if j == 1 {
			c = fuchsia()
		} else if j == 2 {
			c = indigo()
		}
		out = over(out, c, 0.16*shown*boost*exp(-d*d/(2*30*30)))
	}
	top := smoothstep(0.75, 1, s)
	if top <= 0 || Lively <= 0 {
		return out
	}
	// Motes float up through the panel and fade.
	for j := 0; j < 12; j++ {
		fj := float(j)
		seed := hash(fj + 101)
		period := 5 + 4*seed
		t := Clock + seed*11
		f := phase(t, period)
		trip := mod(floor(t/period), 1000003)
		center := vec2(12+(size.x-24)*hash(fj*7+trip)+6*sin(t*1.1), size.y*(1-f))
		a := top * sin(f*3.14159) * (0.35 + 0.4*seed)
		out = over(out, mix(tone(fj), vec3(1), 0.3), a*clamp(0.8+1.2*seed-length(pos-center)+0.5, 0, 1))
	}
	// The border turns through the violets.
	edge := abs(box(pos, size/2, size/2-0.75, Radius-0.75))
	if edge < 1.5 {
		c := pos - size/2
		angle := atan2(c.x, -c.y)/6.28318 + 0.5
		out = over(out, cycle(angle-phase(Clock, 9)), 0.75*top*clamp(1.25-edge, 0, 1))
	}
	return out
}

func effects(out vec4, u vec2, end, vis float) vec4 {
	s := Strength
	w := Bar.z
	h := Bar.w
	// Blocks of data drift along it at a fixed pace, fading where the fill
	// ends, more of them as the level rises, with trails at the top.
	if s > 0.4 {
		trail := smoothstep(0.8, 1, s)
		loop := w + 24
		for j := 0; j < 20; j++ {
			fj := float(j)
			seed := hash(fj)
			shown := clamp((s-0.4-0.5*fj/20)/0.1, 0, 1)
			if shown <= 0 {
				continue
			}
			x := mod(Clock*(18+22*seed)+seed*loop*7, loop) - 12
			lane := 0.5
			if mod(fj, 3) == 0 {
				lane = 0.28
			} else if mod(fj, 3) == 2 {
				lane = 0.72
			}
			for k := 0; k < 3; k++ {
				fk := float(k)
				a := shown * 0.65 * vis
				if k > 0 {
					a *= trail * (0.5 - 0.18*fk)
				}
				xk := x - fk*4.8
				a *= clamp(min((xk+3)/9, (end-xk)/9), 0, 1)
				q := abs(u-vec2(xk, h*lane)) - 1.5
				if max(q.x, q.y) < 0 {
					out = over(out, vec3(1), a)
				}
			}
		}
	}
	// A soft highlight sweeps along it, and at the top a second one back.
	band := max(w*0.25, 32)
	sweep := vis * smoothstep(0.1, 0.4, s) * (0.12 + 0.28*s)
	if sweep > 0 {
		d := abs(u.x-(-band+phase(Clock, 4.5)*(w+2*band))) / band
		out = over(out, vec3(1), sweep*clamp(1-d, 0, 1))
	}
	back := vis * smoothstep(0.75, 1, s) * 0.3
	if back > 0 {
		d := abs(u.x-(-band+(1-phase(Clock, 6.5))*(w+2*band))) / band
		out = over(out, vec3(1), back*clamp(1-d, 0, 1))
	}
	// Four-pointed stars twinkle at the top levels, each somewhere new.
	shown := smoothstep(0.75, 1, s) * vis
	if shown > 0 {
		for j := 0; j < 6; j++ {
			fj := float(j)
			seed := hash(fj + 17)
			period := 2.4 + 1.6*seed
			t := Clock + seed*7
			glow := sin(phase(t, period) * 3.14159)
			glow *= glow
			trip := mod(floor(t/period), 1000003)
			x := w * hash(fj*31+trip)
			a := shown * glow * clamp((end-x)/12, 0, 1)
			r := (2 + 2.5*seed) * glow
			q := abs(u-vec2(x, 4+(h-8)*hash(fj*47+trip)))
			if a > 0.01 && r > 0 && sqrt(q.x)+sqrt(q.y) < sqrt(r) {
				out = over(out, vec3(1), 0.95*a)
			}
		}
	}
	return out
}

func sparks(out vec4, u vec2, end float) vec4 {
	shown := smoothstep(0.75, 1, Strength)
	if shown <= 0 {
		return out
	}
	for j := 0; j < 14; j++ {
		fj := float(j)
		seed := hash(fj + 53)
		period := 2.2 + 1.6*seed
		t := Clock + seed*9
		f := phase(t, period)
		trip := mod(floor(t/period), 1000003)
		x := Bar.z * hash(fj*13+trip)
		a := shown * clamp((end-x)/12, 0, 1) * sin(f*3.14159)
		center := vec2(x+3*sin(t*1.3), 2-f*(10+8*seed))
		r := (0.7 + 0.8*seed) * (1 - 0.4*f)
		out = over(out, mix(tone(fj), vec3(1), 0.45), a*clamp(r-length(u-center)+0.5, 0, 1))
	}
	return out
}

func Fragment(dst vec4, pos vec2) vec4 {
	out := panel(pos, rectSize())
	s := Strength
	w := Bar.z
	h := Bar.w
	r := h / 2
	u := pos - Bar.xy
	if Rtl > 0 {
		u.x = w - u.x
	}
	end := w * clamp(Fill, 0, 1)
	// The fill is one pill from the start; shorter than its own cap it
	// keeps the cap's shape and fades.
	vis := clamp(end/(2*r), 0, 1)
	fillEnd := max(end, 2*r)
	body := box(u, vec2(fillEnd/2, r), vec2(fillEnd/2, r), r)
	flow := 0.0
	if Lively > 0 {
		flow = phase(Clock, 16)
	}
	top := smoothstep(0.75, 1, s)
	glow := smoothstep(0.45, 1, s) * vis
	if glow > 0 && body > 0 {
		pulse := 0.8
		if Lively > 0 {
			pulse = 0.7 + 0.3*sin(Clock*1.4)
		}
		out = over(out, violet(), 0.55*glow*pulse*exp(-body*body/(2*4*4)))
		if top > 0 {
			out = over(out, cycle(0.5-flow), 0.45*top*pulse*vis*exp(-body*body/(2*6*6)))
		}
	}
	track := box(u, vec2(w/2, r), vec2(w/2, r), r)
	out = over(out, Rest.rgb/max(Rest.a, 0.001), Rest.a*clamp(0.5-track, 0, 1))
	if vis > 0 {
		f := clamp(u.x/fillEnd, 0, 1)
		c := mix(mix(indigo(), violet(), 0.25), mix(violet(), fuchsia(), 0.6*s), f)
		if top > 0 {
			c = mix(c, cycle(f-flow), top)
		}
		inside := clamp(0.5-body, 0, 1)
		if inside > 0 {
			fill := over(vec4(0), c, vis)
			if u.y < r {
				// A gloss across the top gives it depth.
				fill = over(fill, vec3(1), 0.22*vis*(1-u.y/r))
			}
			if Lively > 0 {
				fill = effects(fill, u, end, vis)
			}
			out = fill*inside + out*(1-fill.a*inside)
		}
	}
	if Lively > 0 {
		out = sparks(out, u, end)
	}
	return out
}
`
