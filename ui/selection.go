package ui

import "github.com/oneo-me/nsgo/core"

// The selection controls style base's parts by the states the control
// reports: its indicator through PartStates, its marks and label through
// a GroupStyle under the control's group.
const selectionGroup = "selection"

// selectionRow is a selection control's row: its part, then its label,
// all of it pressable.
var selectionRow = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(24),
	Cursor: core.CursorPointer, CornerRadius: RadiusSmall, Group: selectionGroup, Disabled: &core.Style{Cursor: core.CursorDefault}}

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
	return []core.Node{core.Text{Content: content, Style: TypeBody.Merge(core.Style{FlexShrink: 1}), StyleFunc: func() core.Style {
		return core.Style{Color: colors.Text(), GroupStyle: &core.GroupStyle{Name: selectionGroup, Disabled: &core.Style{Color: colors.Faint()}}}
	}}}
}
