package ui

import "github.com/oneo-me/nsgo/core"

// HStack lays its children out in a row with a gap, centered across.
type HStack struct {
	Gap      core.Points
	Children []core.Node
}

// Render implements core.Node.
func (h HStack) Render(*core.Scope) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: h.Gap}, Children: h.Children}
}
