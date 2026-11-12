package ui

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// pickerField is the look of what opens a list or a calendar: a field 28
// pt tall with text, an optional leading part, and a mark at its end. Its
// rim takes the accent while open says so.
func pickerField(cx *core.Scope, leading, text core.Node, mark func() *core.SVG, open func() bool) core.Node {
	colors := ColorsOf(cx)
	children := []core.Node{}
	if leading != nil {
		children = append(children, leading)
	}
	children = append(children, text, Icon(mark, iconSize-2, colors.Muted))
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Height: core.Pt(ButtonHeight),
		Padding: core.Edges{Left: 10, Right: 8}, CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1,
		Transition: quick},
		StyleFunc: func() core.Style {
			border := colors.FieldBorder()
			if open() {
				border = colors.Accent()
			}
			return core.Style{Background: colors.Surface(), BorderColor: border,
				GroupStyle: &core.GroupStyle{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}}
		},
		Children: children}
}

// pickerText is a field's words: value, else placeholder in the faint
// color.
func pickerText(cx *core.Scope, value func() string, placeholder string) core.Text {
	colors := ColorsOf(cx)
	return core.Text{MaxLines: 1, Style: TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}),
		ContentFunc: func() string {
			if v := value(); v != "" {
				return v
			}
			return placeholder
		},
		StyleFunc: func() core.Style {
			if value() != "" {
				return core.Style{Color: colors.Text()}
			}
			return core.Style{Color: colors.Faint()}
		}}
}

// pickerSurface is a dropdown's panel on desktops and a sheet's on touch
// screens, with what goes behind the sheet.
func pickerSurface(cx *core.Scope) (func() core.Style, core.Style) { return menuSurface(cx) }

// listRowStyle is a dropdown's row: as a menu's, with the row the keys or
// the pointer are on lit by its Focus style and a chosen one by Selected.
func listRowStyle(cx *core.Scope) (core.Style, func() core.Style) {
	colors := ColorsOf(cx)
	s, _ := menuRow(cx)
	s.JustifyContent = core.JustifyCenter
	return s, func() core.Style {
		return core.Style{Color: colors.Text(), Focus: &core.Style{Background: colors.Hover()},
			Selected: &core.Style{Background: Alpha(colors.Text(), 0.1)}}
	}
}

// listGroupStyle is a dropdown's section header.
func listGroupStyle(cx *core.Scope) (core.Style, func() core.Style) {
	return TypeCaption.Merge(core.Style{Padding: core.Edges{Top: 6, Right: 8, Bottom: 3, Left: 8}}), Colored(ColorsOf(cx).Muted)
}

// clearMark is what clears a field.
func clearMark(cx *core.Scope) core.Node { return Icon(lucide.X, iconSize-2, ColorsOf(cx).Muted) }

// clearStyle places a field's clear button over its end, before its mark.
var clearStyle = core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Right: core.Pt(26), Width: core.Pt(20),
	AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}

// searchField is a list's search field: ui's Input.
func searchField(in core.Input) core.Node {
	in.Style = in.Style.Merge(core.Style{Margin: core.Edges{Bottom: 4}})
	return Input(in)
}

// emptyList is what a list shows while nothing matches.
func emptyList(cx *core.Scope, text string) core.Node {
	return core.Text{Content: text, Style: TypeBody.Merge(core.Style{Padding: core.All(12), TextAlign: core.TextAlignCenter}),
		StyleFunc: Colored(ColorsOf(cx).Muted)}
}
