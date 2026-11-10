package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Input is the platform's text field with base.Input's fields, styled as
// a text box 28 pt tall, or Multiline growing from three lines: its rim
// darker under the pointer and the accent while focused.
type Input base.Input

// Render implements core.Node.
func (f Input) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	in := base.Input(f)
	focused := core.NewSignal(cx, false)
	onFocus, onBlur := in.OnFocus, in.OnBlur
	in.OnFocus = func() {
		focused.Set(true)
		if onFocus != nil {
			onFocus()
		}
	}
	in.OnBlur = func() {
		focused.Set(false)
		if onBlur != nil {
			onBlur()
		}
	}
	shape := core.Style{Height: core.Pt(ButtonHeight), Padding: core.Symmetric(0, 10)}
	if in.Multiline {
		shape = core.Style{MinHeight: core.Pt(ButtonHeight + 2*TypeBody.LineHeight), Padding: core.Symmetric(6, 10),
			TextAlignVertical: core.TextAlignVerticalTop}
	}
	in.StyleFunc = func() core.Style {
		s := core.Style{PlaceholderColor: colors.Faint(), SelectionColor: cx.Palette().Selection, CaretColor: colors.Accent(),
			FontSize: 13, Color: colors.Text(), Background: colors.Field(),
			BorderWidth: 1, BorderColor: colors.FieldBorder(), CornerRadius: RadiusControl,
			Hover: &core.Style{BorderColor: Mix(colors.FieldBorder(), colors.Text(), 0.25)}}
		if focused.Get() {
			s.BorderColor, s.Hover = colors.Accent(), nil
		}
		return shape.Merge(s)
	}
	return in
}
