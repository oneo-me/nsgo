package base

import (
	"github.com/oneo-me/nsgo/core"
)

// SliderValue is a slider's value: one number, or a range of two with a
// thumb each, whose start is never past its end.
type SliderValue struct {
	// Start is a range's start, End a range's end or a single value.
	Start, End float64
	Range      bool
}

// SingleValue is one value, one thumb.
func SingleValue(v float64) SliderValue { return SliderValue{Start: v, End: v} }

// RangeValue is a range of two values, a thumb each.
func RangeValue(start, end float64) SliderValue {
	return SliderValue{Start: start, End: end, Range: true}
}

// held is v as a slider shows it: in r, a range's ends in order.
func (v SliderValue) held(r numberRange) SliderValue {
	if !v.Range {
		return SingleValue(r.clamp(v.End))
	}
	a, b := r.clamp(v.Start), r.clamp(v.End)
	return RangeValue(min(a, b), max(a, b))
}

// of is the value of a thumb.
func (v SliderValue) of(start bool) float64 {
	if start {
		return v.Start
	}
	return v.End
}

// with is v with a thumb at x; a range's thumbs never cross.
func (v SliderValue) with(start bool, x float64) SliderValue {
	switch {
	case !v.Range:
		return SingleValue(x)
	case start:
		return RangeValue(min(x, v.End), v.End)
	}
	return RangeValue(v.Start, max(x, v.Start))
}

// SliderScale is how values spread along a slider's track.
type SliderScale uint8

const (
	// SliderLinear spreads them evenly.
	SliderLinear SliderScale = iota
	// SliderLogarithmic spreads them by orders of magnitude, for a volume,
	// a frequency, a zoom. It needs Min above 0, else it is linear.
	SliderLogarithmic
)

// Slider picks a value, or a range of two, along a track. It owns the
// presses and drags, WAI-ARIA's slider keys on each thumb, the step and
// the scale, and each thumb's role, label and value for assistive
// technology, and has no look. Its parts are a track, the indicator inside
// it whose length the values map onto, the range and the thumbs on it,
// then its children.
type Slider struct {
	// Value makes it controlled; without it the slider keeps its own, from
	// DefaultValue.
	Value        func() SliderValue
	DefaultValue SliderValue
	// Min, Max and Step are what it takes, 0, 100 and 1 when MaxFunc and
	// Max are unset; NoStep takes any value.
	Min, Max func() float64
	Step     func() float64
	Scale    func() SliderScale
	// Axis is the direction of the track, min at the start, or at the
	// bottom of a vertical one. Read as it is made.
	Axis     Axis
	Disabled func() bool
	// Label is what assistive technology says for the thumb, on a range the
	// end's; StartLabel for a range's start, Label by default.
	Label      string
	StartLabel string
	// OnChange receives the value asked for, as many times as it changes
	// during a drag; OnChangeEnd the value it ended at, once.
	OnChange       func(value SliderValue)
	OnChangeEnd    func(value SliderValue)
	Class          string
	Style          core.Style
	StyleFunc      func() core.Style
	TrackClass     string
	TrackStyle     core.Style
	IndicatorClass string
	IndicatorStyle core.Style
	RangeClass     string
	RangeStyle     core.Style
	ThumbClass     string
	ThumbStyle     core.Style
	// IndicatorStyleFunc, RangeStyleFunc and ThumbStyleFunc are the parts'
	// styles following what they read.
	IndicatorStyleFunc func() core.Style
	RangeStyleFunc     func() core.Style
	ThumbStyleFunc     func() core.Style
	// ValueText is what assistive technology reads as a value, the number
	// by its step's decimals when nil.
	ValueText func(value float64) string
	// Marks show on the indicator, under the range and the thumbs: a
	// stepped slider's ticks, placed by the app in percents of its length.
	Marks    []core.Node
	Children []core.Node
	TestID   string
	// ThumbRef is the thumb's, on a range the end's.
	ThumbRef *core.Ref
	// ThumbTestID and StartThumbTestID name the thumb, on a range the
	// end's, and a range's start.
	ThumbTestID, StartThumbTestID string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// sliderDrag is a pointer's drag, from the grant to the release.
type sliderDrag struct {
	start bool
	// from is where it started along the indicator, in points from the
	// min end.
	from      float64
	value     SliderValue
	moved     bool
	undecided bool
}

// sliderCtl is what the slider's parts and listeners share.
type sliderCtl struct {
	s        Slider
	cx       *core.Scope
	own      *core.Signal[SliderValue]
	pending  *core.Signal[*SliderValue]
	mirrored func() bool
	rail     core.Bounds
	drag     *sliderDrag
}

func (c *sliderCtl) rng() numberRange {
	lo, hi, step := 0.0, 100.0, 1.0
	if c.s.Min != nil {
		lo = c.s.Min()
	}
	if c.s.Max != nil {
		hi = c.s.Max()
	}
	if c.s.Step != nil {
		step = c.s.Step()
	}
	scale := linearScale
	if c.s.Scale != nil && c.s.Scale() == SliderLogarithmic {
		scale = logScale
	}
	return newNumberRange(lo, hi, step, scale)
}

// shown is the value shown: a drag's, else the app's, else its own.
func (c *sliderCtl) shown() SliderValue {
	var v SliderValue
	switch p := c.pending.Get(); {
	case p != nil:
		v = *p
	case c.s.Value != nil:
		v = c.s.Value()
	default:
		v = c.own.Get()
	}
	return v.held(c.rng())
}

func (c *sliderCtl) current() SliderValue { return core.Untracked(c.cx, c.shown) }

func (c *sliderCtl) fraction(start bool) float64 { return c.rng().fraction(c.shown().of(start)) }

func (c *sliderCtl) request(v SliderValue) {
	if c.s.Value == nil {
		c.own.Set(v)
	}
	if c.s.OnChange != nil {
		c.s.OnChange(v)
	}
}

func (c *sliderCtl) end(v SliderValue) {
	if c.s.OnChangeEnd != nil {
		c.s.OnChangeEnd(v)
	}
}

// nudge moves a thumb from a key or a screen reader: a whole change.
func (c *sliderCtl) nudge(start bool, move func(numberRange, float64) float64) {
	if core.Untracked(c.cx, func() bool { return is(c.s.Disabled) }) {
		return
	}
	r := core.Untracked(c.cx, c.rng)
	now := c.current()
	if next := now.with(start, move(r, now.of(start))); next != now {
		c.request(next)
		c.end(next)
	}
}

func (c *sliderCtl) length() float64 {
	if c.s.Axis == Vertical {
		return float64(c.rail.Size.Height)
	}
	return float64(c.rail.Size.Width)
}

// along is points from the indicator's min end of p, a point in the track.
func (c *sliderCtl) along(p core.Point) float64 {
	switch {
	case c.s.Axis == Vertical:
		return float64(c.rail.Origin.Y + c.rail.Size.Height - p.Y)
	case core.Untracked(c.cx, c.mirrored):
		return float64(c.rail.Origin.X + c.rail.Size.Width - p.X)
	}
	return float64(p.X - c.rail.Origin.X)
}

// offset is how far a drag went toward max.
func (c *sliderCtl) offset(e core.ResponderEvent) float64 {
	switch {
	case c.s.Axis == Vertical:
		return -float64(e.Gesture.Delta.Y)
	case core.Untracked(c.cx, c.mirrored):
		return -float64(e.Gesture.Delta.X)
	}
	return float64(e.Gesture.Delta.X)
}

// pressTrack has the nearer thumb jump to the value under the pointer,
// and a drag go on from there.
func (c *sliderCtl) pressTrack(e core.ResponderEvent) {
	now := c.current()
	at := c.along(e.Position)
	start := false
	if now.Range {
		s, t := core.Untracked(c.cx, func() float64 { return c.fraction(true) }), core.Untracked(c.cx, func() float64 { return c.fraction(false) })
		start = at < (s+t)/2*c.length()
	}
	c.drag = &sliderDrag{start: start, from: at, value: now}
	c.dragTo(0)
}

// pressThumb starts a drag that moves a thumb by the pointer's offset, so
// it does not jump under it.
func (c *sliderCtl) pressThumb(start bool) {
	now := c.current()
	from := core.Untracked(c.cx, func() float64 { return c.fraction(start) }) * c.length()
	c.drag = &sliderDrag{start: start, from: from, value: now, undecided: now.Range && now.Start == now.End}
}

// dragTo moves the drag offset points toward max since it started.
func (c *sliderCtl) dragTo(offset float64) {
	d := c.drag
	if d == nil || c.length() <= 0 {
		return
	}
	r := core.Untracked(c.cx, c.rng)
	v := r.snap(r.valueAt((d.from + offset) / c.length()))
	// Pressed on two thumbs at one value: the one that can go the
	// pointer's way, which the drag then keeps.
	if d.undecided && d.value.Range && v != d.value.Start {
		d.start, d.undecided = v < d.value.Start, false
	}
	d.moved = true
	next := d.value.with(d.start, v)
	changed := next != d.value
	d.value = next
	if changed {
		// Shown at once: no wait for the app's Value.
		c.pending.Set(&next)
		c.request(next)
	}
}

// release ends the drag; the app has answered every change by then, so
// its value shows again.
func (c *sliderCtl) release(core.ResponderEvent) {
	d := c.drag
	c.drag = nil
	if d != nil && d.moved {
		c.end(d.value)
	}
	c.pending.Set(nil)
}

// responder is the drags of a part.
func (c *sliderCtl) responder(grant func(core.ResponderEvent)) *core.Responder {
	return &core.Responder{Keep: true, Claims: []core.TouchClaim{{Ask: func(core.ResponderEvent) bool {
		return !core.Untracked(c.cx, func() bool { return is(c.s.Disabled) })
	}}}, OnGrant: grant, OnMove: func(e core.ResponderEvent) { c.dragTo(c.offset(e)) },
		OnRelease: c.release, OnTerminate: c.release}
}

// a11yValue is what assistive technology gets of a thumb: its value
// between what it can take.
func (c *sliderCtl) a11yValue(start bool) core.AccessibilityValue {
	r, v := c.rng(), c.shown()
	lo, hi := r.min, r.max
	if v.Range && start {
		hi = v.End
	} else if v.Range {
		lo = v.Start
	}
	now := v.of(start)
	text := r.format(now)
	if c.s.ValueText != nil {
		text = c.s.ValueText(now)
	}
	return core.AccessibilityValue{Min: float32(lo), Max: float32(hi), Now: float32(now), Text: text}
}

// thumb is a thumb, centered on its value on the indicator.
func (c *sliderCtl) thumb(start bool, focused *core.Signal[int]) core.Node {
	s := c.s
	label, id, ref := s.Label, s.ThumbTestID, s.ThumbRef
	if start {
		id, ref = s.StartThumbTestID, nil
		if s.StartLabel != "" {
			label = s.StartLabel
		}
	}
	base := core.Style{Position: core.Absolute, Transform: core.Transform{TranslateXPercent: -50, TranslateYPercent: -50}}
	if s.Axis == Horizontal {
		base.Top = core.Pct(50)
	} else {
		base.Left = core.Pct(50)
	}
	which := 1
	if start {
		which = 2
	}
	env := c.cx.Env()
	var said *core.AccessibilityValue
	step := func(by float64) func() {
		return func() {
			c.nudge(start, func(r numberRange, v float64) float64 { return r.stepBy(v, by) })
		}
	}
	return core.View{Class: s.ThumbClass, Style: base.Merge(s.ThumbStyle), Disabled: s.Disabled, Role: core.RoleAdjustable, Label: label, TestID: id, Ref: ref, Accessible: true,
		StyleFunc: func() core.Style {
			var look core.Style
			if s.ThumbStyleFunc != nil {
				look = s.ThumbStyleFunc()
			}
			f := float32(c.fraction(start))
			switch {
			case s.Axis == Vertical:
				look.Top = core.Pct((1 - f) * 100)
			case c.mirrored():
				look.Left = core.Pct((1 - f) * 100)
			default:
				look.Left = core.Pct(f * 100)
			}
			return look
		},
		// While a drag moves it and no screen reader runs, the value waits
		// for the drag's end: no props item per move.
		AccessibilityValue: func() core.AccessibilityValue {
			if c.pending.Get() != nil && !env.ScreenReader() && said != nil {
				return *said
			}
			v := c.a11yValue(start)
			said = &v
			return v
		},
		AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
		OnAccessibilityAction: func(name string) {
			switch name {
			case core.ActionIncrement:
				step(1)()
			case core.ActionDecrement:
				step(-1)()
			}
		},
		// A press makes it a Tab stop; the drag is the responder's.
		OnPress: func(core.PressEvent) {},
		OnFocus: func() { focused.Set(which) },
		OnBlur: func() {
			if focused.GetUntracked() == which {
				focused.Set(0)
			}
		},
		Responder: c.responder(func(core.ResponderEvent) { c.pressThumb(start) })}
}

// Render implements core.Node.
func (s Slider) Render(cx *core.Scope) core.Node {
	env := cx.Env()
	c := &sliderCtl{s: s, cx: cx, own: core.NewSignal(cx, s.DefaultValue), pending: core.NewSignal[*SliderValue](cx, nil),
		mirrored: func() bool { return s.Axis == Horizontal && env.RTL() }}
	focused := core.NewSignal(cx, 0)
	nudge := func(move func(numberRange, float64) float64) func() {
		return func() {
			if f := focused.GetUntracked(); f != 0 {
				c.nudge(f == 2, move)
			}
		}
	}
	by := func(steps float64) func() {
		return nudge(func(r numberRange, v float64) float64 { return r.stepBy(v, steps) })
	}
	page := func(d float64) func() { return nudge(func(r numberRange, v float64) float64 { return r.page(v, d) }) }
	keys := bindWhile(func() bool { return focused.Get() != 0 }, core.KeyNotEditing, s.Axis == Horizontal,
		controlKey{"right", true, by(1)}, controlKey{"up", true, by(1)}, controlKey{"left", true, by(-1)}, controlKey{"down", true, by(-1)},
		controlKey{"pageup", true, page(1)}, controlKey{"pagedown", true, page(-1)},
		controlKey{"home", false, nudge(func(r numberRange, _ float64) float64 { return r.min })},
		controlKey{"end", false, nudge(func(r numberRange, _ float64) float64 { return r.max })})
	part := func() core.PartStates { return core.PartStates{Disabled: is(s.Disabled)} }
	rangeBase := core.Style{Position: core.Absolute}
	if s.Axis == Horizontal {
		rangeBase.Top, rangeBase.Bottom = core.Pt(0), core.Pt(0)
	} else {
		rangeBase.Left, rangeBase.Right = core.Pt(0), core.Pt(0)
	}
	rangeView := core.View{Class: s.RangeClass, Style: rangeBase.Merge(s.RangeStyle), PointerEvents: core.PointerEventsNone, PartStates: part,
		StyleFunc: func() core.Style {
			var look core.Style
			if s.RangeStyleFunc != nil {
				look = s.RangeStyleFunc()
			}
			start := float32(0)
			if c.shown().Range {
				start = float32(c.fraction(true))
			}
			end := float32(c.fraction(false))
			switch {
			case s.Axis == Vertical:
				look.Bottom, look.Top = core.Pct(start*100), core.Pct((1-end)*100)
			case c.mirrored():
				look.Left, look.Right = core.Pct((1-end)*100), core.Pct(start*100)
			default:
				look.Left, look.Right = core.Pct(start*100), core.Pct((1-end)*100)
			}
			return look
		}}
	parts := append(append([]core.Node(nil), s.Marks...), rangeView,
		core.Show{When: func() bool { return c.shown().Range }, Then: func(*core.Scope) core.Node { return c.thumb(true, focused) }},
		c.thumb(false, focused))
	indicator := core.View{Class: s.IndicatorClass, Style: s.IndicatorStyle, StyleFunc: s.IndicatorStyleFunc, PointerEvents: core.PointerEventsBoxNone, PartStates: part,
		OnLayout: func(b core.Bounds) { c.rail = b },
		Children: parts}
	track := core.View{Class: s.TrackClass, Style: core.Style{Group: "slider-track"}.Merge(s.TrackStyle), PartStates: part, NoFocus: true,
		Responder: c.responder(c.pressTrack), Children: []core.Node{indicator}}
	return core.View{Class: s.Class, Style: s.Style, StyleFunc: s.StyleFunc, Disabled: s.Disabled, TestID: s.TestID, OnLayout: s.OnLayout, Tooltip: s.Tooltip,
		Children: append([]core.Node{track, keys}, s.Children...)}
}
