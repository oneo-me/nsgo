package ui

import "github.com/oneo-me/nsgo/core"

// Card is a titled container: a header on the page of a bold Title, a
// Description and Actions at its end, then a flat bordered surface holding
// the Children and an optional Footer.
type Card struct {
	Title       string
	Description string
	// Actions end the header (a switch, a small button, a value).
	Actions  []core.Node
	Children []core.Node
	// Footer follows the children on its own line: a status or a caption.
	Footer []core.Node
	// Flush lays the children edge to edge, without the surface's padding
	// and gaps (rows, a full-bleed stage).
	Flush bool
	// OnPress makes the surface a button named Label, else Title, which
	// takes the presses of what it holds.
	OnPress func()
	Label   string
	TestID  string
	// Style merges over the surface's.
	Style core.Style
}

// CardStyle is a card's surface, flat as Onevra's are, read in a
// StyleFunc.
func CardStyle(cx *core.Scope) core.Style {
	colors := ColorsOf(cx)
	return core.Style{Background: colors.Surface(), CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous,
		BorderWidth: 1, BorderColor: colors.Border()}
}

// Render implements core.Node.
func (c Card) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var head core.Node
	if c.Title != "" {
		title := Text(c.Title, 13, Bold, colors.Text)
		title.Role = core.RoleHeader
		titles := []core.Node{title}
		if c.Description != "" {
			description := Text(c.Description, 12, Regular, colors.Muted)
			description.MaxLines = 2
			titles = append(titles, description)
		}
		nodes := append([]core.Node{core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: titles}}, c.Actions...)
		head = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 10, Padding: core.Symmetric(0, 2)}, Children: nodes}
	}
	label := c.Label
	if label == "" {
		label = c.Title
	}
	return Panel{TestID: c.TestID, Head: head, Flush: c.Flush, Children: c.Children, Footer: c.Footer, Style: c.Style,
		OnPress: c.OnPress, Label: label}
}
