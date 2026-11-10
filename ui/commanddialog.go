package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
)

// CommandDialog hosts a Command in a modal surface, or a sheet on touch
// screens: 640 points wide and its rows 440 tall. It takes the
// keyboard as it opens and gives it back as it closes; the app binds what
// opens it, and closes it from the palette's OnSelect. With
// ExpandOnSearch it fits the field until the query holds more than spaces.
type CommandDialog struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	// OnClose follows it being gone, once its motion ended.
	OnClose func()
	// Command is the palette, whose Label, when set, names the dialog.
	Command Command
	// Inset is the Dialog's.
	Inset  core.Points
	TestID string
	// Style merges over the surface's.
	Style core.Style
}

// Render implements core.Node.
func (d CommandDialog) Render(cx *core.Scope) core.Node {
	c := d.Command
	c.AutoFocus = true
	c.Style = core.Style{FlexGrow: 1, FlexShrink: 1, Height: core.Pt(440)}.Merge(c.Style)
	cancel := c.OnCancel
	c.OnCancel = func() {
		if d.OnOpenChange != nil {
			d.OnOpenChange(false, base.ReasonCancel)
		}
		if cancel != nil {
			cancel()
		}
	}
	style := d.Style
	if !Touch(cx) {
		style = core.Style{Width: core.Pt(640)}.Merge(style)
	}
	label := c.Label
	if label == "" {
		label = c.SearchLabel
	}
	if label == "" {
		label = locales.CommandSearch(cx)
	}
	return Dialog{Open: d.Open, OnOpenChange: d.OnOpenChange, OnClose: d.OnClose, Bare: true, Inset: d.Inset, Style: style, Label: label, TestID: d.TestID,
		Children: []core.Node{c}}
}
