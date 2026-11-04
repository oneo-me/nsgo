package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
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

// Dialog is a modal surface over a dimmed app, centered on desktops and
// a sheet from the bottom on touch screens: a title, a description, the
// app's children and its Footer of actions.
type Dialog struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	Title        string
	Description  string
	Children     []core.Node
	// Footer holds the actions, at the end of a row on desktops, the
	// last first in a stack across a sheet.
	Footer                 []core.Node
	NoCloseOnEscape        bool
	NoCloseOnBackdropPress bool
	// Label names the dialog for assistive technology, Title when empty.
	Label  string
	TestID string
	// Style merges over the surface's.
	Style core.Style
}

// Render implements core.Node.
func (d Dialog) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := d.Label
	if label == "" {
		label = d.Title
	}
	var body []core.Node
	if d.Title != "" || d.Description != "" {
		var head []core.Node
		if d.Title != "" {
			head = append(head, core.Text{Content: d.Title, Role: core.RoleHeader, Style: typeTitle, StyleFunc: Colored(colors.Text)})
		}
		if d.Description != "" {
			head = append(head, core.Text{Content: d.Description, Style: typeBody, StyleFunc: Colored(colors.Muted)})
		}
		body = append(body, core.View{Style: core.Style{Gap: 4}, Children: head})
	}
	body = append(body, d.Children...)
	if touch(cx) {
		if d.Footer != nil {
			body = append(body, core.View{Style: core.Style{Direction: core.ColumnReverse, Gap: 8, Margin: core.Edges{Top: 8}},
				Children: d.Footer})
		}
		return base.Sheet{Open: d.Open, OnOpenChange: d.OnOpenChange, NoCloseOnEscape: d.NoCloseOnEscape,
			NoCloseOnBackdropPress: d.NoCloseOnBackdropPress, CloseOnSwipe: !d.NoCloseOnBackdropPress,
			BackdropStyle: scrim, PopupStyleFunc: func() core.Style { return sheetPanel(cx, core.PlaceBottom).Merge(d.Style) },
			Children: sheetContent(cx, core.PlaceBottom, body), Label: label, TestID: d.TestID}
	}
	if d.Footer != nil {
		body = append(body, core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, JustifyContent: core.JustifyEnd,
			Gap: 8, Margin: core.Edges{Top: 8}}, Children: d.Footer})
	}
	return base.Dialog{Open: d.Open, OnOpenChange: d.OnOpenChange, Animation: core.ModalFade, NoCloseOnEscape: d.NoCloseOnEscape,
		NoCloseOnBackdropPress: d.NoCloseOnBackdropPress, BackdropStyle: scrim,
		PopupStyle:     core.Style{Width: core.Pct(90), MaxWidth: core.Pt(480), Padding: core.All(20), Gap: 12},
		PopupStyleFunc: func() core.Style { return surface(cx).Merge(d.Style) },
		Children:       body, Label: label, TestID: d.TestID}
}

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
	Title        string
	Description  string
	Actions      []AlertAction
	TestID       string
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
	return Dialog{Open: a.Open, OnOpenChange: a.OnOpenChange, Title: a.Title, Description: a.Description, Footer: footer,
		NoCloseOnBackdropPress: true, TestID: a.TestID, Style: core.Style{MaxWidth: core.Pt(420)}}
}
