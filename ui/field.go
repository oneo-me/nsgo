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
