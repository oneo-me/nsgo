package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// ContextMenu opens its items about Trigger, as a secondary click or a
// finger resting on it asks: at the pointer, or as a sheet.
type ContextMenu struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	Disabled     bool
	// Presentation is base's: a popup at the pointer by default, or a
	// sheet, which ui chooses on no device of its own.
	Presentation base.Presentation
	Trigger      core.Node
	Children     []core.Node
	TestID       string
	// Style is the trigger's.
	Style core.Style
}

// Render implements core.Node.
func (m ContextMenu) Render(cx *core.Scope) core.Node {
	popup, backdrop := menuSurface(cx, m.Presentation == base.PresentSheet)
	return base.ContextMenu{Open: m.Open, OnOpenChange: m.OnOpenChange, Disabled: m.Disabled, Presentation: m.Presentation,
		Offset: 2, Margin: 8,
		Style: m.Style, PopupStyleFunc: popup, BackdropStyle: backdrop, Trigger: m.Trigger, Children: m.Children, TestID: m.TestID}
}
