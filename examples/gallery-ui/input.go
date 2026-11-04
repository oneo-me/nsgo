package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Field is an Input in Onevra's text box: the field's color and a rim;
// the input's own style sets its size and padding.
type Field struct{ Input base.Input }

// Render implements core.Node.
func (f Field) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	in := f.Input
	in.Style = core.Style{FontSize: 13}.Merge(in.Style)
	own := in.StyleFunc
	in.StyleFunc = func() core.Style {
		s := core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Accent()}
		if own != nil {
			s = s.Merge(own())
		}
		return s
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, BorderColor: colors.FieldBorder(), Background: colors.Field()}
	}, Children: []core.Node{in}}
}

// Input is a base.Input styled as Onevra's text box, 28 pt tall, or
// Multiline growing from three lines: its rim darker under the pointer
// and the accent while focused.
type Input struct{ Input base.Input }

// Render implements core.Node.
func (f Input) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	in := f.Input
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
		shape = core.Style{MinHeight: core.Pt(ButtonHeight + 2*typeBody.LineHeight), Padding: core.Symmetric(6, 10),
			TextAlignVertical: core.TextAlignVerticalTop}
	}
	in.StyleFunc = func() core.Style {
		border := colors.FieldBorder
		if focused.Get() {
			border = colors.Accent
		}
		return shape.Merge(core.Style{PlaceholderColor: colors.Faint(), SelectionColor: cx.Palette().Selection,
			FontSize: 13, Color: colors.Text(), Background: colors.Field(),
			BorderWidth: 1, BorderColor: border(), CornerRadius: RadiusControl,
			Hover: &core.Style{BorderColor: Mix(colors.FieldBorder(), colors.Text(), 0.25)}})
	}
	return in
}
