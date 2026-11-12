package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// FormField is a row of a FormSection that edits text: Title, then the
// Input with no box of its own, filling the rest of the row and aligned
// to its end. The Input's Label is Title unless it has one.
type FormField struct {
	Title  string
	Input  base.Input
	TestID string
}

// Render implements core.Node.
func (f FormField) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	in := f.Input
	if in.Label == "" {
		in.Label = f.Title
	}
	in.Style = core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(ButtonHeight), FontSize: 13,
		TextAlign: core.TextAlignEnd, Disabled: Dimmed()}.Merge(in.Style)
	in.StyleFunc = func() core.Style {
		return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: cx.Palette().Selection, CaretColor: colors.Accent()}
	}
	title := Text(f.Title, 13, Regular, colors.Text)
	title.Style.FlexShrink = 0
	return core.View{TestID: f.TestID, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: formRowGap,
		MinHeight: core.Pt(formRowHeight), Padding: core.Symmetric(6, formRowPad)},
		Children: []core.Node{title, in}}
}
