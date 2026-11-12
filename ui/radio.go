package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

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
	dot := core.View{Style: core.Style{Width: core.Pt(iconSize * 3 / 8), Height: core.Pt(iconSize * 3 / 8), CornerRadius: RadiusFull,
		Display: core.DisplayNone, GroupStyle: &core.GroupStyle{Name: selectionGroup, Checked: &core.Style{Display: core.DisplayFlex}}},
		StyleFunc: Filled(colors.OnAccent)}
	ring := core.View{Style: core.Style{Width: core.Pt(iconSize), Height: core.Pt(iconSize), CornerRadius: RadiusFull, BorderWidth: 1, FlexShrink: 0,
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
		StyleFunc: func() core.Style { return core.Style{FocusVisible: FocusRing(cx, 2)} },
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
