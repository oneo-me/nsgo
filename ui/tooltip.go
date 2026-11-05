package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
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
}

// Render implements core.Node.
func (p Tooltip) Render(cx *core.Scope) core.Node {
	return base.Tooltip{Open: p.Open, OnOpenChange: p.OnOpenChange, Placement: p.Placement, Trigger: p.Trigger,
		Children: []core.Node{tooltipBubble(cx, p.Text)}, TestID: p.TestID}
}
