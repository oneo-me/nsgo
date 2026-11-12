package ui

import (
	"math"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// NumberInput is base.NumberInput in ui's look, with buttons stacked at
// its end on desktops, a minus and a plus on touch screens. What is typed
// is read as the field is left or Return pressed, and kept to the range
// and the step; text that is no number gives the value back. A drag across
// its Leading handle scrubs it.
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
	number := func(v float64) func() *float64 { return func() *float64 { return &v } }
	in := base.NumberInput{Format: format, Parse: parse, OnCommit: true, Snap: true, Step: n.step, Disabled: n.Disabled, Label: n.Label, TestID: n.TestID,
		ControlsRight: true, IncrementLabel: n.IncrementLabel, DecrementLabel: n.DecrementLabel,
		Value: func() *float64 {
			v := 0.0
			if n.Value != nil {
				v = n.Value()
			}
			return &v
		},
		OnChange: func(v *float64) {
			if v != nil && n.OnChange != nil {
				n.OnChange(*v)
			}
		}}
	if n.Max > n.Min {
		in.Min, in.Max = number(n.Min), number(n.Max)
	}
	return n.look(cx, in)
}

// look is in in ui's look: what a base form with a number input of its
// own takes, a scrub handle it brings included.
func (n NumberInput) look(cx *core.Scope, in base.NumberInput) base.NumberInput {
	colors := ColorsOf(cx)
	in.InputStyle = TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(32), Padding: core.Symmetric(0, 10)})
	in.InputStyleFunc = func() core.Style {
		return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Now().Selection, CaretColor: colors.Accent()}
	}
	in.Style = core.Style{Direction: core.Row, AlignItems: core.AlignStretch, Height: core.Pt(ButtonHeight), Width: core.Pt(160),
		CornerRadius: RadiusControl, BorderWidth: 1, Overflow: core.Hidden, Transition: quick}
	in.StyleFunc = func() core.Style {
		return core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(),
			Hover:    &core.Style{BorderColor: Mix(colors.FieldBorder(), colors.Text(), 0.25)},
			Focus:    &core.Style{BorderColor: colors.Accent()},
			Disabled: &core.Style{Opacity: core.Alpha(0.5), Background: colors.Inset(), BorderColor: colors.Border()}}.Merge(n.Style)
	}
	if n.Leading != nil || n.LeadingText != "" {
		in.Scrub = n.handle(cx)
		if n.TestID != "" {
			in.ScrubTestID = n.TestID + "-scrub"
		}
	}
	if in.Scrub != nil {
		in.ScrubStyle = TypeLabel.Merge(core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, MinWidth: core.Pt(ButtonHeight - 6),
			Padding: core.Edges{Left: 8}, Cursor: core.CursorColResize})
		in.ScrubStyleFunc = Colored(colors.Muted)
		in.InputStyle.Padding.Left = 4
	}
	if !n.HideSteppers {
		n.steppers(cx, &in)
	}
	return in
}

// handle is what the leading handle shows.
func (n NumberInput) handle(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	if n.Leading != nil {
		return Icon(n.Leading, iconSize-2, colors.Muted)
	}
	return core.Text{Content: n.LeadingText, MaxLines: 1, Style: TypeLabel, StyleFunc: Colored(colors.Muted)}
}

// steppers draws in's buttons: stacked chevrons on desktops, a minus and a
// plus on touch screens.
func (n NumberInput) steppers(cx *core.Scope, in *base.NumberInput) {
	colors := ColorsOf(cx)
	face := core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Transition: quick, Borders: core.Edges{Left: 1},
		Disabled: &core.Style{Cursor: core.CursorDefault}}
	in.ButtonLook = func() core.Style {
		c := colors.Now()
		return core.Style{Color: c.Muted, BorderColors: core.EdgeColors{Left: c.Line, Top: c.Line},
			Hover: &core.Style{Background: c.Hover, Color: c.Text}, Active: &core.Style{Background: c.Pressed},
			Focus: FocusRing(cx, -2), Disabled: &core.Style{Color: c.Faint, Background: transparent}}
	}
	icon := func(svg func() *core.SVG) core.Node { return Icon(svg, iconSize-4, Inherited) }
	if !Touch(cx) {
		half := face.Merge(core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Width: core.Pt(22)})
		in.IncrementButton, in.DecrementButton = icon(lucide.ChevronUp), icon(lucide.ChevronDown)
		in.IncrementButtonStyle, in.DecrementButtonStyle = half, half.Merge(core.Style{Borders: core.Edges{Left: 1, Top: 1}})
		return
	}
	square := face.Merge(core.Style{Width: core.Pt(ButtonHeight)})
	in.IncrementButton, in.DecrementButton = icon(lucide.Plus), icon(lucide.Minus)
	in.IncrementButtonStyle, in.DecrementButtonStyle = square, square
	// The column's two buttons in a row, the minus first.
	in.ControlsStyle = core.Style{Direction: core.RowReverse}
}
