// Package flow is base/flow's styled form: a directed diagram in ui's
// look, its lines in the muted text's color, the hovered and selected
// node's in the accent, and Card, a node's default face.
package flow

import (
	baseflow "github.com/nsgo-dev/nsgo/base/flow"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// The diagram's data and camera are base/flow's.
type (
	Direction = baseflow.Direction
	Node      = baseflow.Node
	Edge      = baseflow.Edge
	Data      = baseflow.Data
	State     = baseflow.State
)

const (
	TopToBottom = baseflow.TopToBottom
	LeftToRight = baseflow.LeftToRight
)

// View is base/flow's View in ui's look; its fields are base's. While
// there are no nodes Empty shows, else EmptyText, an em dash when empty,
// in the muted text's color.
type View struct {
	Data             func() Data
	Direction        func() Direction
	NodeGap, RankGap core.Points
	Selected         func() string
	OnSelect         func(id string)
	OnOpen           func(id string)
	State            *State
	Empty, Overlay   core.Node
	EmptyText        string
	Label            string
	LabelFunc        func() string
	Style            core.Style
	TestID           string
}

// Render implements core.Node.
func (v View) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	empty := v.Empty
	if empty == nil {
		text := v.EmptyText
		if text == "" {
			text = "—"
		}
		empty = core.Text{Content: text, Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted)}
	}
	return baseflow.View{Data: v.Data, Direction: v.Direction, NodeGap: v.NodeGap, RankGap: v.RankGap,
		Selected: v.Selected, OnSelect: v.OnSelect, OnOpen: v.OnOpen, State: v.State, Empty: empty, Overlay: v.Overlay,
		Label: v.Label, LabelFunc: v.LabelFunc, Style: v.Style, TestID: v.TestID,
		LookFunc: func() baseflow.Look {
			return baseflow.Look{Line: ui.Mix(colors.Muted(), colors.Background(), .45), Emphasis: colors.Accent().Faded(.85),
				Label: colors.Muted(), LabelBackground: colors.Background(), LabelBorder: colors.Border().Faded(.7)}
		},
		NodeStyleFunc: func() core.Style {
			return core.Style{CornerRadius: CardRadius, FocusVisible: ui.FocusRing(cx, 2)}
		},
		RingStyleFunc: func() core.Style {
			return core.Style{BorderWidth: 2, BorderColor: colors.Accent().Faded(.7), CornerRadius: CardRadius + 3}
		}}
}
