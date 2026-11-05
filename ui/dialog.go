package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

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
			head = append(head, core.Text{Content: d.Title, Role: core.RoleHeader, Style: TypeTitle, StyleFunc: Colored(colors.Text)})
		}
		if d.Description != "" {
			head = append(head, core.Text{Content: d.Description, Style: TypeBody, StyleFunc: Colored(colors.Muted)})
		}
		body = append(body, core.View{Style: core.Style{Gap: 4}, Children: head})
	}
	body = append(body, d.Children...)
	if Touch(cx) {
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
