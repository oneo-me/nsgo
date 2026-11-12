package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Tooltip shows Text by its trigger as the pointer rests on it, on
// desktops, or while the app opens it.
type Tooltip struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	Text         string
	Trigger      core.Node
	// Placement is above the trigger when 0.
	Placement core.Placement
	TestID    string
	// Style is the trigger's box, within its parent's layout.
	Style core.Style
}

// Render implements core.Node.
func (p Tooltip) Render(cx *core.Scope) core.Node {
	return base.Tooltip{Open: p.Open, OnOpenChange: p.OnOpenChange, Placement: p.Placement, Style: p.Style, Trigger: p.Trigger,
		Children: []core.Node{tooltipBubble(cx, p.Text)}, TestID: p.TestID}
}
