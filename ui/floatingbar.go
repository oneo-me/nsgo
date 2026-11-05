package ui

import "github.com/oneo-me/nsgo/core"

// FloatingBar is a capsule of controls floating over content, such as
// icon buttons, on the glass of a bar.
type FloatingBar struct {
	Children []core.Node
	// Label names the bar for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (b FloatingBar) Render(cx *core.Scope) core.Node {
	return core.View{TestID: b.TestID, Label: b.Label,
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.All(4), MaxWidth: core.Pct(100),
			CornerRadius: RadiusFull, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return FinOf(cx).Glass().Over(core.Style{Color: ColorsOf(cx).Text()}).Merge(b.Style)
		}, Children: b.Children}
}
