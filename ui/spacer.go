package ui

import "github.com/oneo-me/nsgo/core"

// Spacer takes the free space of a row.
type Spacer struct{}

// Render implements core.Node.
func (Spacer) Render(*core.Scope) core.Node { return core.View{Style: core.Style{FlexGrow: 1}} }
