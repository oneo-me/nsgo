package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// ContextMenu opens its items about Trigger: at the pointer on a
// secondary click, or as a sheet after a finger rests on it.
type ContextMenu struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	Disabled     bool
	Trigger      core.Node
	Children     []core.Node
	TestID       string
	// Style is the trigger's.
	Style core.Style
}

// Render implements core.Node.
func (m ContextMenu) Render(cx *core.Scope) core.Node {
	popup, backdrop := menuSurface(cx)
	return base.ContextMenu{Open: m.Open, OnOpenChange: m.OnOpenChange, Disabled: m.Disabled, Offset: 2, Margin: 8,
		Style: m.Style, PopupStyleFunc: popup, BackdropStyle: backdrop, Trigger: m.Trigger, Children: m.Children, TestID: m.TestID}
}
