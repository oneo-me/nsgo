package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// FormRowVariant is a FormRow's look.
type FormRowVariant uint8

const (
	// FormRowDefault is a setting: the title in the text's color, and a
	// chevron at the end when it can be pressed.
	FormRowDefault FormRowVariant = iota
	// FormRowAccent is an action: the title in the accent.
	FormRowAccent
	// FormRowDanger is a destructive action, or what went wrong: the
	// title in the danger color.
	FormRowDanger
)

// FormRow is a row of a FormSection: an Icon on a tile, Title over
// Detail, then Value in the muted color and the Control. With OnPress it
// is a button that darkens while pressed.
type FormRow struct {
	Icon          func() *core.SVG
	Title, Detail string
	Value         func() string
	Control       core.Node
	OnPress       func()
	Disabled      func() bool
	Variant       FormRowVariant
	// Label names a pressable row for assistive technology, Title by
	// default.
	Label  string
	TestID string
}

// Render implements core.Node.
func (r FormRow) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	tint := colors.Text
	switch r.Variant {
	case FormRowAccent:
		tint = colors.Accent
	case FormRowDanger:
		tint = colors.Danger
	}
	var children []core.Node
	if r.Icon != nil {
		fill := colors.AccentFill
		if r.Variant == FormRowDanger {
			fill = colors.Danger
		}
		children = append(children, core.View{Style: core.Style{Width: core.Pt(formIconTile), Height: core.Pt(formIconTile), FlexShrink: 0,
			CornerRadius: 6, BorderCurve: core.BorderCurveContinuous, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: Filled(fill), Children: []core.Node{Icon(r.Icon, 14, colors.OnAccent)}})
	}
	texts := []core.Node{core.Text{Content: r.Title, Style: TypeBody,
		StyleFunc: func() core.Style { return core.Style{Color: tint()} }}}
	if r.Detail != "" {
		detail := Text(r.Detail, 11.5, Regular, colors.Muted)
		detail.Style.LineHeight = 15
		texts = append(texts, detail)
	}
	children = append(children, core.View{Style: core.Style{Gap: 1, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: texts})
	if r.Value != nil {
		value := Textf(r.Value, 13, Regular, colors.Muted)
		value.MaxLines, value.Style.FlexShrink, value.Style.MinWidth = 1, 1, core.Pt(0)
		children = append(children, value)
	}
	if r.Control != nil {
		children = append(children, core.View{Style: core.Style{FlexShrink: 0}, Children: []core.Node{r.Control}})
	}
	if r.OnPress != nil && r.Variant == FormRowDefault {
		children = append(children, Icon(lucide.ChevronRight, 13, colors.Faint))
	}
	style := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: formRowGap, MinHeight: core.Pt(formRowHeight),
		Padding: core.Symmetric(6, formRowPad)}
	if r.OnPress == nil {
		return core.View{TestID: r.TestID, Style: style, Children: children}
	}
	label := r.Label
	if label == "" {
		label = r.Title
	}
	style.Disabled = Dimmed()
	return base.Button{Label: label, TestID: r.TestID, Disabled: r.Disabled, OnPress: func(core.PressEvent) { r.OnPress() },
		Style: style,
		StyleFunc: func() core.Style {
			return core.Style{Active: &core.Style{Background: colors.Hover()}, FocusVisible: FocusRing(cx, -2)}
		},
		Children: children}
}
