package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// HoverCard previews what Trigger points at on a surface beside it: after
// the pointer rests or the focus stays on a desktop, as a press on a touch
// screen. Trigger keeps its own role, presses and look.
type HoverCard struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	// OpenDelay and CloseDelay are base.HoverCard's.
	OpenDelay, CloseDelay time.Duration
	Placement             core.Placement
	Align                 core.Align
	Trigger               core.Node
	Children              []core.Node
	TestID                string
	// Style merges over the surface's.
	Style core.Style
}

// Render implements core.Node.
func (h HoverCard) Render(cx *core.Scope) core.Node {
	return base.HoverCard{Open: h.Open, OnOpenChange: h.OnOpenChange, OpenDelay: h.OpenDelay, CloseDelay: h.CloseDelay,
		Placement: h.Placement, Align: h.Align, Offset: 6, Margin: 8, Trigger: h.Trigger, Children: h.Children, TestID: h.TestID,
		PopupStyle: core.Style{Padding: core.All(12), Gap: 8, MinWidth: core.Pt(200), MaxWidth: core.Pt(320), Transition: quick},
		PopupStyleFunc: func() core.Style {
			return FinOf(cx).Menu().Over(core.Style{CornerRadius: RadiusCard, BorderWidth: 1, Color: ColorsOf(cx).Text()}).Merge(h.Style)
		},
		StartingStyle: core.Style{Opacity: core.Alpha(0)}, EndingStyle: core.Style{Opacity: core.Alpha(0)}}
}
