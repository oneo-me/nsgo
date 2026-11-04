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

// Wrap lays its children out in a row that wraps, 8 pt apart.
type Wrap struct{ Children []core.Node }

// Render implements core.Node.
func (w Wrap) Render(*core.Scope) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 8}, Children: w.Children}
}

// Spacer takes the free space of a row.
type Spacer struct{}

// Render implements core.Node.
func (Spacer) Render(*core.Scope) core.Node { return core.View{Style: core.Style{FlexGrow: 1}} }

// Cell is an equal share of a row for its Child.
type Cell struct{ Child core.Node }

// Render implements core.Node.
func (c Cell) Render(*core.Scope) core.Node {
	return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0)}, Children: []core.Node{c.Child}}
}
