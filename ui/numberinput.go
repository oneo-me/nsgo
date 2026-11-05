package ui

import (
	"math"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// scrubDistance is how far a drag on a NumberInput's handle moves to
// change the value by a step.
const scrubDistance = 4

// NumberInput is a field of a number with buttons that step it: stacked
// at its end on desktops, a minus and a plus on touch screens. The arrow
// keys step it while it has the keyboard focus, and a drag across its
// Leading handle scrubs it. What is typed is read as the field is left
// or Return pressed, and kept to the range and the step; text that is no
// number gives the value back.
type NumberInput struct {
	Value    func() float64
	OnChange func(value float64)
	// Min and Max are the range, when Max is above Min.
	Min, Max float64
	// Step is what the buttons, the keys and the scrub change the value
	// by, and what it is rounded to from Min; 1 when 0.
	Step float64
	// Format writes the value, with as many decimals as Step has when
	// nil; Parse reads it back, strconv.ParseFloat's when nil.
	Format func(value float64) string
	Parse  func(text string) (float64, bool)
	// Leading is the scrub handle's icon, or LeadingText its text, such
	// as "W"; without either the field has no handle. Its test ID is the
	// field's with "-scrub".
	Leading     func() *core.SVG
	LeadingText string
	// HideSteppers leaves the buttons out.
	HideSteppers bool
	// IncrementLabel and DecrementLabel name the buttons for assistive
	// technology.
	IncrementLabel, DecrementLabel string
	Disabled                       func() bool
	// Label names the field for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// step is the value's step.
func (n NumberInput) step() float64 {
	if n.Step > 0 {
		return n.Step
	}
	return 1
}

// fit keeps v to the range and the step.
func (n NumberInput) fit(v float64) float64 {
	step, from := n.step(), 0.0
	ranged := n.Max > n.Min
	if ranged {
		from = n.Min
	}
	v = from + math.Round((v-from)/step)*step
	if ranged {
		v = min(max(v, n.Min), n.Max)
	}
	// Steps such as 0.1 leave binary noise; the step's decimals hold
	// the value.
	scale := math.Pow(10, float64(decimals(step)))
	return math.Round(v*scale) / scale
}

// decimals is how many digits after the point step needs.
func decimals(step float64) int {
	s := strconv.FormatFloat(step, 'f', -1, 64)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

// Render implements core.Node.
func (n NumberInput) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	value := func() float64 {
		if n.Value == nil {
			return 0
		}
		return n.Value()
	}
	format := n.Format
	if format == nil {
		d := decimals(n.step())
		format = func(v float64) string { return strconv.FormatFloat(v, 'f', d, 64) }
	}
	parse := n.Parse
	if parse == nil {
		parse = func(s string) (float64, bool) {
			v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
		}
	}
	set := func(v float64) {
		if v = n.fit(v); v != value() && n.OnChange != nil {
			n.OnChange(v)
		}
	}
	by := func(steps float64) { set(value() + steps*n.step()) }
	at := func(edge float64) func() bool {
		return func() bool { return is(n.Disabled) || n.Max > n.Min && value() == edge }
	}
	// While the field has the focus it shows what is typed.
	focused, draft := core.NewSignal(cx, false), core.NewSignal(cx, "")
	commit := func() {
		if v, ok := parse(draft.Get()); ok {
			set(v)
		}
		draft.Set(format(value()))
	}
	field := base.Input{Label: n.Label, Disabled: n.Disabled, KeyboardType: core.KeyboardDecimal,
		Value: func() string {
			if focused.Get() {
				return draft.Get()
			}
			return format(value())
		},
		OnChange: draft.Set, OnSubmit: func(string) { commit() },
		OnFocus: func() { draft.Set(format(value())); focused.Set(true) },
		OnBlur:  func() { commit(); focused.Set(false) },
		Style:   TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(32), Padding: core.Symmetric(0, 10)}),
		StyleFunc: func() core.Style {
			return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Now().Selection}
		}}
	var children []core.Node
	if n.Leading != nil || n.LeadingText != "" {
		children = append(children, n.handle(cx, value, set))
		field.Style.Padding.Left = 4
	}
	children = append(children, field, core.Show{When: focused.Get, Then: func(cx *core.Scope) core.Node {
		// The arrows move the caret in other fields; this one steps.
		opts := core.KeyOptions{When: core.KeyAlways, Repeat: true}
		cx.OnKeyWith("up", opts, func() { by(1); draft.Set(format(value())) })
		cx.OnKeyWith("down", opts, func() { by(-1); draft.Set(format(value())) })
		return nil
	}})
	if !n.HideSteppers {
		children = append(children, n.steppers(cx, by, at))
	}
	return core.View{TestID: n.TestID, Children: children,
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignStretch, Height: core.Pt(ButtonHeight), Width: core.Pt(160),
			CornerRadius: RadiusControl, BorderWidth: 1, Overflow: core.Hidden, Transition: quick},
		StyleFunc: func() core.Style {
			s := core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(),
				Hover: &core.Style{BorderColor: Mix(colors.FieldBorder(), colors.Text(), 0.25)}}
			switch {
			case is(n.Disabled):
				s = core.Style{Opacity: core.Alpha(0.5), Background: colors.Inset(), BorderColor: colors.Border()}
			case focused.Get():
				s.BorderColor, s.Hover = colors.Accent(), nil
			}
			return s.Merge(n.Style)
		}}
}

// handle is the leading handle a drag across scrubs the value with; the
// steppers and the keys do as much for assistive technology.
func (n NumberInput) handle(cx *core.Scope, value func() float64, set func(float64)) core.Node {
	colors := ColorsOf(cx)
	var from float64
	var content core.Node
	if n.Leading != nil {
		content = Icon(n.Leading, iconSize-2, colors.Muted)
	} else {
		content = core.Text{Content: n.LeadingText, MaxLines: 1, Style: TypeLabel, StyleFunc: Colored(colors.Muted)}
	}
	id := ""
	if n.TestID != "" {
		id = n.TestID + "-scrub"
	}
	return core.View{TestID: id, ImportantForAccessibility: core.ImportantNoHideDescendants,
		PanActivation: core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12},
		OnPan: func(e core.PanEvent) {
			if is(n.Disabled) {
				return
			}
			switch e.Phase {
			case core.GestureBegan:
				from = value()
			case core.GestureChanged, core.GestureEnded:
				set(from + math.Round(float64(e.Translation.X)/scrubDistance)*n.step())
			}
		},
		Style: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, MinWidth: core.Pt(ButtonHeight - 6),
			Padding: core.Edges{Left: 8}, Cursor: core.CursorColResize},
		Children: []core.Node{content}}
}

// steppers are the buttons that step the value: stacked chevrons on
// desktops, a minus and a plus on touch screens.
func (n NumberInput) steppers(cx *core.Scope, by func(float64), at func(float64) func() bool) core.Node {
	colors := ColorsOf(cx)
	button := func(icon func() *core.SVG, label string, steps float64, disabled func() bool, s core.Style) core.Node {
		return base.Button{Label: label, Disabled: disabled, OnPress: func(core.PressEvent) { by(steps) },
			Style: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Transition: quick, Borders: core.Edges{Left: 1}, Disabled: &core.Style{Cursor: core.CursorDefault}}.Merge(s),
			StyleFunc: func() core.Style {
				c := colors.Now()
				return core.Style{Color: c.Muted, BorderColors: core.EdgeColors{Left: c.Line, Top: c.Line},
					Hover: &core.Style{Background: c.Hover, Color: c.Text}, Active: &core.Style{Background: c.Pressed},
					Focus: FocusRing(cx, -2), Disabled: &core.Style{Color: c.Faint, Background: transparent}}
			},
			Children: []core.Node{Icon(icon, iconSize-4, Inherited)}}
	}
	if !Touch(cx) {
		half := core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Width: core.Pt(22)}
		return core.View{Children: []core.Node{
			button(lucide.ChevronUp, n.IncrementLabel, 1, at(n.Max), half),
			button(lucide.ChevronDown, n.DecrementLabel, -1, at(n.Min), half.Merge(core.Style{Borders: core.Edges{Left: 1, Top: 1}})),
		}}
	}
	square := core.Style{Width: core.Pt(ButtonHeight)}
	return core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
		button(lucide.Minus, n.DecrementLabel, -1, at(n.Min), square),
		button(lucide.Plus, n.IncrementLabel, 1, at(n.Max), square),
	}}
}
