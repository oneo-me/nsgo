package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// AlertAction is an action of an AlertDialog, a Button of its Variant,
// or filled with Color.
type AlertAction struct {
	Text    string
	Variant ButtonVariant
	Color   func() core.Color
	OnPress func()
	TestID  string
}

// AlertDialog asks the user to confirm or acknowledge something: a Dialog
// whose actions close it, kept open by presses outside it.
type AlertDialog struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	// OnClose follows it being gone, once its motion ended.
	OnClose     func()
	Title       string
	Description string
	Actions     []AlertAction
	TestID      string
}

// Render implements core.Node.
func (a AlertDialog) Render(*core.Scope) core.Node {
	var footer []core.Node
	for _, action := range a.Actions {
		footer = append(footer, Button{Variant: action.Variant, Color: action.Color, Text: action.Text, TestID: action.TestID, OnPress: func() {
			if action.OnPress != nil {
				action.OnPress()
			}
			if a.OnOpenChange != nil {
				a.OnOpenChange(false, base.ReasonSelect)
			}
		}})
	}
	return Dialog{Open: a.Open, OnOpenChange: a.OnOpenChange, OnClose: a.OnClose, Title: a.Title, Description: a.Description, Footer: footer,
		NoCloseOnBackdropPress: true, TestID: a.TestID}
}
