package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Popups share base's contract: Open makes them controlled, and
// OnOpenChange reports each request to open or close with its reason.
// They are drawn on one surface, and on touch screens dialogs come up as
// sheets from the bottom.

// Popover shows its children on a surface beside its trigger, which
// Trigger's look and content draw. Pressing the trigger opens and closes
// it; pressing outside or Escape closes it.
type Popover struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	// Trigger is the trigger's look and content; its OnPress and Disabled
	// go unused.
	Trigger   Button
	Placement core.Placement
	Align     core.Align
	Children  []core.Node
	// Label names the trigger, Trigger's label when empty.
	Label  string
	TestID string
	// Style merges over the surface's.
	Style core.Style
}

// Render implements core.Node.
func (p Popover) Render(cx *core.Scope) core.Node {
	style, focus, face := p.Trigger.parts(cx)
	return base.Popover{Open: p.Open, OnOpenChange: p.OnOpenChange, Placement: p.Placement, Align: p.Align, Offset: 4, Margin: 8,
		Style: style, StyleFunc: focus, Trigger: face,
		PopupStyle: core.Style{Padding: core.All(12), Gap: 8, MinWidth: core.Pt(200), MaxWidth: core.Pt(360)},
		PopupStyleFunc: func() core.Style {
			return FinOf(cx).Menu().Over(core.Style{CornerRadius: RadiusCard, BorderWidth: 1, Color: ColorsOf(cx).Text()}).Merge(p.Style)
		},
		Children: p.Children, Label: triggerLabel(p.Label, p.Trigger), TestID: p.TestID}
}

// triggerLabel is a popup trigger's name: label, else its button's.
func triggerLabel(label string, b Button) string {
	if label != "" {
		return label
	}
	return b.label()
}
