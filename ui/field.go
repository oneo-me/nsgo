package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Field is the platform's text field with base.Input's fields, in a
// text box's color and rim; its own style sets its size and padding.
type Field base.Input

// Render implements core.Node.
func (f Field) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	in := base.Input(f)
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
