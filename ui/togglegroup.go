package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// ToggleGroup is a row of ToggleButtons, one Tab stop whose arrows move
// between them: the app keeps which are on, any number or exactly one.
type ToggleGroup struct {
	Disabled func() bool
	Children []core.Node
	Label    string
	TestID   string
	Style    core.Style
}

// Render implements core.Node.
func (g ToggleGroup) Render(*core.Scope) core.Node {
	return base.ToggleGroup{Disabled: g.Disabled, Label: g.Label, TestID: g.TestID, Children: g.Children,
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, Gap: 4}.Merge(g.Style)}
}
