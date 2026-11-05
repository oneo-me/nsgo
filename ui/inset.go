package ui

import "github.com/oneo-me/nsgo/core"

// Inset is a recessed surface that sets its content apart. Zero Gap and Padding
// take 10 pt gaps and 12 pt padding, unless Exact takes them as given.
type Inset struct {
	Gap, Padding core.Points
	Exact        bool
	Children     []core.Node
}

// Render implements core.Node.
func (in Inset) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	gap, padding := in.Gap, in.Padding
	if !in.Exact {
		if gap == 0 {
			gap = 10
		}
		if padding == 0 {
			padding = 12
		}
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Background: colors.Inset(), CornerRadius: RadiusInset, BorderCurve: core.BorderCurveContinuous,
			Padding: core.All(padding), Gap: gap}
	}, Children: in.Children}
}
