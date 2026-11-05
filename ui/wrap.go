package ui

import "github.com/oneo-me/nsgo/core"

// Wrap lays its children out in a row that wraps, 8 pt apart.
type Wrap struct{ Children []core.Node }

// Render implements core.Node.
func (w Wrap) Render(*core.Scope) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 8}, Children: w.Children}
}
