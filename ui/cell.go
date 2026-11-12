package ui

import "github.com/nsgo-dev/nsgo/core"

// Cell is an equal share of a row for its Child.
type Cell struct{ Child core.Node }

// Render implements core.Node.
func (c Cell) Render(*core.Scope) core.Node {
	return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0)}, Children: []core.Node{c.Child}}
}
