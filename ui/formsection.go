package ui

import "github.com/oneo-me/nsgo/core"

// FormSection is related rows on a rounded card, divided by hairlines
// from the start of the titles above them, as an inset grouped list of iOS
// Settings: Title above the card and Footer below it, in the muted color.
// Its rows are FormRow and FormField, or the app's own nodes, which pad
// themselves.
type FormSection struct {
	Title, Footer string
	Children      []core.Node
	TestID        string
	Style         core.Style
}

// Render implements core.Node.
func (s FormSection) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var rows []core.Node
	for i, child := range s.Children {
		if i > 0 {
			rows = append(rows, core.View{Style: core.Style{Height: core.Pt(1), Margin: core.Edges{Left: formRowLead(s.Children[i-1])}},
				StyleFunc: Filled(colors.Line)})
		}
		rows = append(rows, child)
	}
	var nodes []core.Node
	if s.Title != "" {
		title := Text(s.Title, 12, Medium, colors.Muted)
		title.Role = core.RoleHeader
		title.Style.Padding = core.Symmetric(0, formRowPad)
		nodes = append(nodes, title)
	}
	nodes = append(nodes, core.View{Style: core.Style{CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous, Overflow: core.Hidden},
		StyleFunc: func() core.Style { return core.Style{Background: Alpha(colors.Text(), 0.045)} }, Children: rows})
	if s.Footer != "" {
		footer := Text(s.Footer, 11.5, Regular, colors.Muted)
		footer.Style.LineHeight, footer.Style.Padding = 16, core.Symmetric(0, formRowPad)
		nodes = append(nodes, footer)
	}
	return core.View{TestID: s.TestID, Style: core.Style{Gap: 6}.Merge(s.Style), Children: nodes}
}

// The rows' metrics: their least height, their padding at the sides and
// the tile an icon sits on.
const (
	formRowHeight core.Points = 40
	formRowPad    core.Points = 12
	formRowGap    core.Points = 10
	formIconTile  core.Points = 24
)

// formRowLead is where the hairline below row starts: the start of its
// title, past an icon, as iOS insets a cell's separator.
func formRowLead(row core.Node) core.Points {
	if r, ok := row.(FormRow); ok && r.Icon != nil {
		return formRowPad + formIconTile + formRowGap
	}
	return formRowPad
}
