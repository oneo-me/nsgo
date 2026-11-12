package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Sheet is a panel at the bottom over a dimmed backdrop while Open
// (base.Sheet in ui's look); a press on the backdrop, Escape or
// the back button calls OnClose.
type Sheet struct {
	Open     func() bool
	OnClose  func()
	Children []core.Node
}

// Render implements core.Node.
func (s Sheet) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	env := cx.Env()
	return base.Sheet{
		Open: s.Open,
		OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				s.OnClose()
			}
		},
		BackdropStyle: core.Style{Background: Black(0.4)},
		PopupStyleFunc: func() core.Style {
			return core.Style{Gap: 12, Padding: core.Edges{Top: 8, Right: 16, Bottom: env.SafeArea().Bottom + 16, Left: 16},
				CornerRadii: core.Corners{TopLeft: 16, TopRight: 16}, Background: colors.Surface()}
		},
		Children: append([]core.Node{core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(5), CornerRadius: 3, AlignSelf: core.AlignCenter},
			StyleFunc: Filled(colors.Border)}}, s.Children...),
	}
}
