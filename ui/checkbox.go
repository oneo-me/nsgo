package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

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
		Children: []core.Node{Icon(lucide.Check, iconSize-4, colors.OnAccent)}}
	dash := core.View{Style: core.Style{Width: core.Pt(iconSize / 2), Height: core.Pt(2), CornerRadius: 1}.
		Merge(shown(func(gs *core.GroupStyle, s *core.Style) { gs.Indeterminate = s })), StyleFunc: Filled(colors.OnAccent)}
	box := core.Style{Width: core.Pt(iconSize), Height: core.Pt(iconSize), CornerRadius: RadiusSmall, BorderWidth: 1,
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
