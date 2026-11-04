package ui

import (
	"fmt"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Switch is an on and off switch in Onevra's look: base.Switch, the knob
// sliding across and the track changing color on the platform. Label is
// what assistive technology reads.
type Switch struct {
	Checked  func() bool
	OnChange func(bool)
	Label    string
}

// Render implements core.Node.
func (s Switch) Render(cx *core.Scope) core.Node {
	look := NewSwitchLook(cx, s.Checked, false)
	// More to press on every side, without taking more room.
	return base.Switch{Checked: s.Checked, OnChange: s.OnChange, Label: s.Label,
		Style:          core.Style{Margin: core.All(-8), Padding: core.All(8), Cursor: core.CursorPointer},
		TrackStyleFunc: look.Track, ThumbStyleFunc: look.Thumb}
}

// SwitchLook is Onevra's switch as the styles of base.Switch's parts: a
// track that turns to the accent and a capsule knob that slides across,
// both on the platform.
type SwitchLook struct {
	Track, Thumb func() core.Style
}

// NewSwitchLook makes a switch's look, 40 × 20 or, small, 30 × 16 for
// dense rows. on is the value the app owns; the knob follows it.
func NewSwitchLook(cx *core.Scope, on func() bool, small bool) SwitchLook {
	colors := ColorsOf(cx)
	width, height, pad := core.Points(40), core.Points(20), core.Points(2)
	if small {
		width, height = 30, 16
	}
	knobH := height - 2*pad
	knobW := knobH * 1.3
	start := 0.0
	shown := core.Untracked(cx, on)
	if shown {
		start = 1
	}
	t := core.NewAnimatedValue(cx, start)
	x := Lerp(t, 0, float64(width-2*pad-knobW))
	env := cx.Env()
	cx.Effect(func() {
		next := on()
		if next == shown {
			return
		}
		shown = next
		to := 0.0
		if next {
			to = 1
		}
		// Reduced motion: the knob and the track change in place.
		if env.ReduceMotion() {
			t.Set(to)
		} else {
			Timing(cx, t, to, switchTiming)
		}
	})
	track := func() core.Style {
		return core.Style{Width: core.Pt(width), Height: core.Pt(height), CornerRadius: height / 2, Padding: core.All(pad), Background: colors.Track(),
			Animated: &core.AnimatedStyle{Background: core.Interpolate(t, core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{colors.Track(), colors.AccentFill()}})}}
	}
	return SwitchLook{Track: track, Thumb: func() core.Style {
		return core.Style{Width: core.Pt(knobW), Height: core.Pt(knobH), CornerRadius: knobH / 2, Background: colors.Knob(),
			Shadows: FinOf(cx).Knob().Shadows, Animated: &core.AnimatedStyle{TranslateX: x}}
	}}
}

// Option is one segment of a segmented control; Count shows a quiet
// number after the label when set.
type Option[T comparable] struct {
	Value T
	Label string
	Count func() int
}

// Segmented is Onevra's segmented control: equal segments in a recessed
// capsule, the selected one on a raised thumb that slides to it, 28 pt
// tall: base.Tabs, a tab per segment, in the gallery's look. Small is the
// same control.
type Segmented[T comparable] struct {
	Options  []Option[T]
	Value    func() T
	OnChange func(T)
	Small    bool
}

// Render implements core.Node.
func (sc Segmented[T]) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	width := core.NewSignal(cx, core.Points(0))
	x := core.NewAnimatedValue(cx, 0)
	index := func() int {
		v := sc.Value()
		for i, o := range sc.Options {
			if o.Value == v {
				return i
			}
		}
		return 0
	}
	// The track's padding: the segments share what is left of the
	// measured width.
	const pad = 2
	segment := func() core.Points { return (width.Get() - 2*pad) / core.Points(max(1, len(sc.Options))) }
	height := ButtonHeight + 2*pad
	// The first placement jumps; later ones slide.
	placed := false
	env := cx.Env()
	cx.Effect(func() {
		to := float64(core.Points(index()) * segment())
		if width.Get() <= 0 {
			return
		}
		if placed && !env.ReduceMotion() {
			Spring(cx, x, to, SettleSpring)
		} else {
			x.Set(to)
		}
		placed = true
	})
	children := []core.Node{core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
		return FinOf(cx).Pill().Over(core.Style{Position: core.Absolute, Top: core.Pt(pad), Left: core.Pt(pad), Bottom: core.Pt(pad),
			Width: core.Pt(max(0, segment())), CornerRadius: (height - 2*pad) / 2,
			Animated: &core.AnimatedStyle{TranslateX: x}})
	}}}
	for _, o := range sc.Options {
		selected := func() bool { return sc.Value() == o.Value }
		label := []core.Node{core.Text{Content: o.Label, MaxLines: 1, StyleFunc: func() core.Style {
			c, w := Alpha(colors.Text(), 0.6), Regular
			if selected() {
				c, w = colors.Text(), Semibold
			}
			return core.Style{FontSize: 13, FontWeight: w, Color: c}
		}}}
		if o.Count != nil {
			label = append(label, Textf(func() string { return fmt.Sprint(o.Count()) }, 12, Regular, colors.Faint))
		}
		children = append(children, base.Tab{Selected: selected, OnPress: func(core.PressEvent) { sc.OnChange(o.Value) }, Label: o.Label,
			Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 5,
				Padding: core.Symmetric(0, 12), Cursor: core.CursorPointer},
			Children: label})
	}
	return core.View{OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: []core.Node{base.Tabs{
		Style:     core.Style{Height: core.Pt(height), Direction: core.Row, Padding: core.All(pad), CornerRadius: height / 2},
		StyleFunc: func() core.Style { return core.Style{Background: Alpha(colors.Text(), 0.06)} },
		Children:  children,
	}}}
}

// The selection controls style base's parts by the states the control
// reports: its indicator through PartStates, its marks and label through
// a GroupStyle under the control's group.
const selectionGroup = "selection"

// Checkbox is checked, unchecked or indeterminate, with Text or the app's
// Children beside its box.
type Checkbox struct {
	Checked func() bool
	// Indeterminate is neither checked nor unchecked, as "select all" over
	// a partial selection; a press then asks for checked.
	Indeterminate func() bool
	Disabled      func() bool
	// OnChange receives the value a press asks for.
	OnChange func(checked bool)
	Text     string
	// Children replace Text.
	Children []core.Node
	// Label is what assistive technology reads, Text when empty.
	Label  string
	Hint   string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (c Checkbox) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	shown := func(on func(*core.GroupStyle, *core.Style)) core.Style {
		gs := &core.GroupStyle{Name: selectionGroup}
		on(gs, &core.Style{Display: core.DisplayFlex})
		return core.Style{Display: core.DisplayNone, GroupStyle: gs}
	}
	check := core.View{Style: shown(func(gs *core.GroupStyle, s *core.Style) { gs.Checked = s }),
		Children: []core.Node{Icon(IconCheck, iconSize-4, colors.OnAccent)}}
	dash := core.View{Style: core.Style{Width: core.Pt(iconSize / 2), Height: core.Pt(2), CornerRadius: 1}.
		Merge(shown(func(gs *core.GroupStyle, s *core.Style) { gs.Indeterminate = s })), StyleFunc: Filled(colors.OnAccent)}
	box := core.Style{Width: core.Pt(iconSize), Height: core.Pt(iconSize), CornerRadius: radiusSmall, BorderWidth: 1,
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Transition: quick, Disabled: &core.Style{Opacity: core.Alpha(0.45)}}
	label := c.Label
	if label == "" {
		label = c.Text
	}
	return base.Checkbox{Checked: c.Checked, Indeterminate: c.Indeterminate, Disabled: c.Disabled, OnChange: c.OnChange,
		Style: selectionRow.Merge(c.Style), StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		IndicatorStyle: box, IndicatorStyleFunc: func() core.Style {
			on := &core.Style{Background: colors.AccentFill(), BorderColor: colors.AccentFill()}
			return core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(), Shadows: FinOf(cx).Raised().Shadows,
				GroupStyle: &core.GroupStyle{Name: selectionGroup, Hover: &core.Style{BorderColor: colors.Faint()}},
				Checked:    on, Indeterminate: on}
		},
		Indicator: core.View{Children: []core.Node{check, dash}},
		Children:  selectionLabel(cx, c.Text, c.Children), Label: label, Hint: c.Hint, TestID: c.TestID}
}

// Radio is one option of a RadioGroup, with Text or the app's Children
// beside its ring.
type Radio struct {
	Checked  func() bool
	Disabled func() bool
	// OnChange receives true when the unchecked radio is pressed: check it
	// and uncheck the others.
	OnChange func(checked bool)
	Text     string
	Children []core.Node
	Label    string
	Hint     string
	TestID   string
	Style    core.Style
}

// Render implements core.Node.
func (r Radio) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	dot := core.View{Style: core.Style{Width: core.Pt(iconSize * 3 / 8), Height: core.Pt(iconSize * 3 / 8), CornerRadius: radiusFull,
		Display: core.DisplayNone, GroupStyle: &core.GroupStyle{Name: selectionGroup, Checked: &core.Style{Display: core.DisplayFlex}}},
		StyleFunc: Filled(colors.OnAccent)}
	ring := core.View{Style: core.Style{Width: core.Pt(iconSize), Height: core.Pt(iconSize), CornerRadius: radiusFull, BorderWidth: 1, FlexShrink: 0,
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Transition: quick},
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(), Shadows: FinOf(cx).Raised().Shadows,
				GroupStyle: &core.GroupStyle{Name: selectionGroup, Hover: &core.Style{BorderColor: colors.Faint()},
					Checked:  &core.Style{Background: colors.AccentFill(), BorderColor: colors.AccentFill()},
					Disabled: &core.Style{Opacity: core.Alpha(0.45)}}}
		},
		Children: []core.Node{dot}}
	label := r.Label
	if label == "" {
		label = r.Text
	}
	return base.Radio{Checked: r.Checked, Disabled: r.Disabled, OnChange: r.OnChange, Style: selectionRow.Merge(r.Style),
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Children:  append([]core.Node{ring}, selectionLabel(cx, r.Text, r.Children)...), Label: label, Hint: r.Hint, TestID: r.TestID}
}

// RadioGroup holds the Radios of one choice, stacked or in a row.
type RadioGroup struct {
	Horizontal bool
	Children   []core.Node
	// Label names the choice for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (g RadioGroup) Render(*core.Scope) core.Node {
	s := core.Style{Gap: 4}
	if g.Horizontal {
		s = core.Style{Direction: core.Row, Wrap: core.Wrap, ColumnGap: 16, RowGap: 4}
	}
	return base.RadioGroup{Style: s.Merge(g.Style), Children: g.Children, Label: g.Label, TestID: g.TestID}
}

// selectionRow is a selection control's row: its part, then its label,
// all of it pressable.
var selectionRow = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(24),
	Cursor: core.CursorPointer, CornerRadius: radiusSmall, Group: selectionGroup, Disabled: &core.Style{Cursor: core.CursorDefault}}

// selectionLabel is a selection control's label: the app's children, or
// its text, faint while the control is disabled.
func selectionLabel(cx *core.Scope, content string, children []core.Node) []core.Node {
	if children != nil {
		return children
	}
	if content == "" {
		return nil
	}
	colors := ColorsOf(cx)
	return []core.Node{core.Text{Content: content, Style: typeBody.Merge(core.Style{FlexShrink: 1}), StyleFunc: func() core.Style {
		return core.Style{Color: colors.Text(), GroupStyle: &core.GroupStyle{Name: selectionGroup, Disabled: &core.Style{Color: colors.Faint()}}}
	}}}
}
