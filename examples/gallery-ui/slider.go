package ui

import (
	"math"
	"strconv"

	"github.com/oneo-me/nsgo/core"
)

// Slider's geometry: a 4-point track under a white capsule knob 20 × 16,
// which grows by a third while held.
const (
	sliderTrack core.Points = 4
	sliderKnobW core.Points = 20
	sliderKnobH core.Points = 16
	// sliderTicks is the most ticks a stepped slider shows.
	sliderTicks = 60
)

// Slider picks a number from Min to Max on a track: a knob to drag, or,
// with High, two that pick a range from Value up to High. A press on the
// track moves the nearest knob there and a drag carries it on. Each knob
// is a Tab stop on desktops that the arrow keys, Page Up and Page Down,
// Home and End move, and an adjustable element for assistive technology;
// a range's knobs never cross. Step above 0 snaps the values to steps
// from Min and marks them under the track, up to 60 of them; at 0 the
// keys move a hundredth of the range.
type Slider struct {
	Value    func() float64
	OnChange func(value float64)
	// High and OnHighChange make it a range, Value its low end.
	High         func() float64
	OnHighChange func(value float64)
	// Min and Max are the range, 0 to 100 when Max is not above Min.
	Min, Max float64
	Step     float64
	Disabled func() bool
	// Label names it for assistive technology; a range's knobs add
	// LowLabel and HighLabel after it, such as "minimum" and "maximum".
	Label               string
	LowLabel, HighLabel string
	// ValueText is what assistive technology reads as a value, the
	// number when nil.
	ValueText func(value float64) string
	// TestID names the slider; its knobs add "-thumb", or "-low" and
	// "-high" in a range.
	TestID string
	Style  core.Style
}

// bounds are the slider's range.
func (s Slider) bounds() (lo, hi float64) {
	if s.Max > s.Min {
		return s.Min, s.Max
	}
	return 0, 100
}

// snap keeps v to the range and its steps.
func (s Slider) snap(v float64) float64 {
	lo, hi := s.bounds()
	v = min(max(v, lo), hi)
	if s.Step > 0 {
		n := min(math.Round((v-lo)/s.Step), math.Floor((hi-lo)/s.Step+1e-9))
		v = lo + n*s.Step
		scale := math.Pow(10, float64(decimals(s.Step)))
		v = min(math.Round(v*scale)/scale, hi)
	}
	return v
}

// keyStep is what an arrow key moves a knob by.
func (s Slider) keyStep() float64 {
	if s.Step > 0 {
		return s.Step
	}
	lo, hi := s.bounds()
	return (hi - lo) / 100
}

// Render implements core.Node.
func (s Slider) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	lo, hi := s.bounds()
	ranged := s.High != nil
	read := func(fn func() float64) func() float64 {
		return func() float64 {
			if fn == nil {
				return lo
			}
			return min(max(fn(), lo), hi)
		}
	}
	values := []func() float64{read(s.Value)}
	changes := []func(float64){s.OnChange}
	if ranged {
		values = append(values, read(s.High))
		changes = append(changes, s.OnHighChange)
	}
	// set moves knob k to v, kept from crossing the other.
	set := func(k int, v float64) {
		if is(s.Disabled) {
			return
		}
		v = s.snap(v)
		if ranged && k == 0 {
			v = min(v, core.Untracked(cx, values[1]))
		} else if ranged {
			v = max(v, core.Untracked(cx, values[0]))
		}
		if v != core.Untracked(cx, values[k]) && changes[k] != nil {
			changes[k](v)
		}
	}
	width := core.NewSignal(cx, core.Points(0))
	// span is the width the knobs' centers travel, x a value's place.
	span := func() core.Points { return max(0, width.Get()-sliderKnobW) }
	x := func(v float64) core.Points {
		if hi <= lo {
			return sliderKnobW / 2
		}
		return sliderKnobW/2 + span()*core.Points((v-lo)/(hi-lo))
	}
	at := func(px core.Points) float64 {
		w := core.Untracked(cx, span)
		if w <= 0 {
			return lo
		}
		f := float64(min(max(px-sliderKnobW/2, 0), w) / w)
		return lo + f*(hi-lo)
	}
	// held is the knob a press or drag holds, -1 when none.
	held := core.NewSignal(cx, -1)
	focused := core.NewSignal(cx, -1)
	var downX core.Points
	pressed := false
	grab := func(px core.Points) {
		v, k := at(px), 0
		if ranged {
			a, b := core.Untracked(cx, values[0]), core.Untracked(cx, values[1])
			if v > b || math.Abs(v-b) < math.Abs(v-a) || v == b && v == a && v > lo {
				k = 1
			}
		}
		held.Set(k)
		set(k, v)
	}
	// hold is a press on knob k, which a drag carries from its center.
	hold := func(k int) {
		if !is(s.Disabled) {
			downX, pressed = core.Untracked(cx, func() core.Points { return x(values[k]()) }), true
			held.Set(k)
		}
	}
	const height = sliderKnobH + 4
	tall := core.Points(height)
	if s.Step > 0 {
		tall += 8
	}
	// A finger's room: the slider reaches 44 pt tall into its margins,
	// its parts placed down by as much.
	var reach core.Points
	if touch(cx) && tall < 44 {
		reach = (44 - tall) / 2
	}
	middle := reach + height/2
	bar := core.Style{Position: core.Absolute, Top: core.Pt(middle - sliderTrack/2), Height: core.Pt(sliderTrack), CornerRadius: sliderTrack / 2}
	children := []core.Node{
		core.View{PointerEvents: core.PointerEventsNone, Style: bar.Merge(core.Style{Left: core.Pt(sliderKnobW / 2), Right: core.Pt(sliderKnobW / 2)}),
			StyleFunc: Filled(colors.Line)},
		core.View{PointerEvents: core.PointerEventsNone, Style: bar, StyleFunc: func() core.Style {
			from, to := x(lo), x(values[0]())
			if ranged {
				from, to = to, x(values[1]())
			}
			return core.Style{Left: core.Pt(from), Width: core.Pt(max(0, to-from)), Background: colors.Accent()}
		}},
	}
	if s.Step > 0 && hi > lo {
		if n := int(math.Floor((hi - lo) / s.Step)); n <= sliderTicks {
			for i := 0; i <= n; i++ {
				v := lo + float64(i)*s.Step
				children = append(children, core.View{PointerEvents: core.PointerEventsNone,
					Style:     core.Style{Position: core.Absolute, Top: core.Pt(middle + 10), Width: core.Pt(2), Height: core.Pt(2), CornerRadius: 1},
					StyleFunc: func() core.Style { return core.Style{Left: core.Pt(x(v) - 1), Background: colors.Track()} }})
			}
		}
	}
	names := []string{s.Label}
	ids := []string{s.TestID + "-thumb"}
	if ranged {
		names = []string{s.Label + " " + s.LowLabel, s.Label + " " + s.HighLabel}
		ids = []string{s.TestID + "-low", s.TestID + "-high"}
	}
	if s.TestID == "" {
		ids = make([]string, len(ids))
	}
	for k := range values {
		children = append(children, s.knob(cx, k, names[k], ids[k], values, set, hold, x, middle, held, focused))
	}
	return core.View{TestID: s.TestID, Disabled: s.Disabled, NoFocus: true,
		OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) },
		OnPressDown: func(e core.PressEvent) {
			if !is(s.Disabled) {
				downX, pressed = e.Position.X, true
				grab(downX)
			}
		},
		OnPressUp: func(core.PressEvent) { pressed = false; held.Set(-1) },
		OnPan: func(e core.PanEvent) {
			switch e.Phase {
			case core.GestureBegan:
				if !pressed {
					hold(0)
				}
				pressed = false
			case core.GestureChanged:
				if k := held.GetUntracked(); k >= 0 {
					set(k, at(downX+e.Translation.X))
				}
			default:
				held.Set(-1)
			}
		},
		PanActivation: core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12},
		Style: core.Style{Height: core.Pt(tall + 2*reach), Margin: core.Symmetric(-reach, 0), MinWidth: core.Pt(80), Cursor: core.CursorPointer,
			Disabled: dimmed()}.Merge(s.Style),
		Children: children}
}

// knob is knob k of the slider: the capsule the pointer drags, and the
// element the keys and assistive technology move.
func (s Slider) knob(cx *core.Scope, k int, name, id string, values []func() float64, set func(int, float64),
	hold func(int), x func(float64) core.Points, middle core.Points, held, focused *core.Signal[int]) core.Node {
	colors := ColorsOf(cx)
	lo, hi := s.bounds()
	value := values[k]
	by := func(d float64) { set(k, core.Untracked(cx, value)+d) }
	page := max(s.keyStep(), (hi-lo)/10)
	// The ends a knob may reach: the range's, or the other knob.
	from, to := func() float64 { return lo }, func() float64 { return hi }
	if len(values) == 2 && k == 0 {
		to = values[1]
	} else if len(values) == 2 {
		from = values[0]
	}
	keys := core.Show{When: func() bool { return focused.Get() == k }, Then: func(cx *core.Scope) core.Node {
		repeat := core.KeyOptions{Repeat: true}
		cx.OnKeyWith("right", repeat, func() { by(s.keyStep()) })
		cx.OnKeyWith("up", repeat, func() { by(s.keyStep()) })
		cx.OnKeyWith("left", repeat, func() { by(-s.keyStep()) })
		cx.OnKeyWith("down", repeat, func() { by(-s.keyStep()) })
		cx.OnKeyWith("pageup", repeat, func() { by(page) })
		cx.OnKeyWith("pagedown", repeat, func() { by(-page) })
		cx.OnKey("home", func() { set(k, core.Untracked(cx, from)) })
		cx.OnKey("end", func() { set(k, core.Untracked(cx, to)) })
		return nil
	}}
	v := core.View{TestID: id, Accessible: true, Role: core.RoleAdjustable, Label: name, Disabled: s.Disabled,
		AccessibilityValue: func() core.AccessibilityValue {
			now := value()
			text := strconv.FormatFloat(now, 'f', -1, 64)
			if s.ValueText != nil {
				text = s.ValueText(now)
			}
			return core.AccessibilityValue{Min: float32(from()), Max: float32(to()), Now: float32(now), Text: text}
		},
		AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
		OnAccessibilityAction: func(name string) {
			switch name {
			case core.ActionIncrement:
				by(s.keyStep())
			case core.ActionDecrement:
				by(-s.keyStep())
			}
		},
		Style: core.Style{Position: core.Absolute, Top: core.Pt(middle - sliderKnobH/2), Width: core.Pt(sliderKnobW), Height: core.Pt(sliderKnobH),
			CornerRadius: sliderKnobH / 2, Transition: core.Transition{Duration: quick.Duration, Easing: core.EaseOut, Properties: core.TransitionTransform}},
		StyleFunc: func() core.Style {
			st := FinOf(cx).Knob().Over(core.Style{Left: core.Pt(x(value()) - sliderKnobW/2), Background: colors.Knob(),
				BorderWidth: 1, BorderColor: Alpha(colors.Text(), 0.08), Focus: FocusRing(cx, 2)})
			if held.Get() == k {
				st.Transform = core.Transform{Scale: 1.3}
				st.Shadows = []core.BoxShadow{Shadow(0, 6, 14, -3, Black(0.18))}
			}
			return st
		},
		OnPressDown: func(core.PressEvent) { hold(k) },
		OnPressUp:   func(core.PressEvent) { held.Set(-1) },
		Children:    []core.Node{keys}}
	if !touch(cx) {
		// A press focuses the knob, which the slider's press moves.
		v.OnPress = func(core.PressEvent) {}
		v.OnFocus = func() { focused.Set(k) }
		v.OnBlur = func() {
			if focused.GetUntracked() == k {
				focused.Set(-1)
			}
		}
	}
	return v
}
