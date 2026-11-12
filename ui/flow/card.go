package flow

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// CardRadius rounds Card's corners; the selection ring follows it.
const CardRadius core.Points = 14

// CardVariant is a card's border.
type CardVariant uint8

const (
	// CardDefault is the theme's soft border. It is the default.
	CardDefault CardVariant = iota
	// CardAccent is a border of the accent, as for a step under way.
	CardAccent
)

// Card is a node's default face: a softly shadowed, rounded surface 240
// points wide unless Width says otherwise, every region a slot. Header is
// a small row at the top, such as a status; Leading comes before the
// body, such as an icon tile; Body is the main content, else Title and
// Description show, each on one line; Trailing comes after the body, such
// as a badge; Footer is a small muted row at the bottom, such as counts.
type Card struct {
	Width              core.Points
	Title, Description string
	Header, Leading    core.Node
	Body, Trailing     core.Node
	Footer             core.Node
	Variant            CardVariant
}

// The card's small type, at 0.85 of the body's size.
var cardSmall = core.Style{FontSize: 11, LineHeight: 15}

// Render implements core.Node.
func (c Card) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	width := c.Width
	if width <= 0 {
		width = 240
	}
	body := c.Body
	if body == nil && (c.Title != "" || c.Description != "") {
		var lines core.Group
		if c.Title != "" {
			lines = append(lines, core.Text{Content: c.Title, MaxLines: 1, Style: ui.TypeBody.Merge(core.Style{FontWeight: ui.Semibold}),
				StyleFunc: ui.Colored(colors.Text)})
		}
		if c.Description != "" {
			lines = append(lines, core.Text{Content: c.Description, MaxLines: 1, Style: cardSmall, StyleFunc: ui.Colored(colors.Muted)})
		}
		body = lines
	}
	var rows []core.Node
	if c.Header != nil {
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}.Merge(cardSmall),
			Children: []core.Node{c.Header}})
	}
	if c.Leading != nil || body != nil || c.Trailing != nil {
		var main []core.Node
		if c.Leading != nil {
			main = append(main, c.Leading)
		}
		var inner []core.Node
		if body != nil {
			inner = []core.Node{body}
		}
		main = append(main, core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 2}, Children: inner})
		if c.Trailing != nil {
			main = append(main, c.Trailing)
		}
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, Gap: 10}, Children: main})
	}
	if c.Footer != nil {
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}.Merge(cardSmall),
			StyleFunc: ui.Colored(colors.Muted), Children: []core.Node{c.Footer}})
	}
	return core.View{Style: core.Style{Width: core.Pt(width), Padding: core.All(12), Gap: 10, CornerRadius: CardRadius, BorderWidth: 1},
		StyleFunc: func() core.Style {
			border := colors.Border().Faded(.7)
			if c.Variant == CardAccent {
				border = colors.Accent().Faded(.6)
			}
			return core.Style{Background: colors.Surface(), BorderColor: border,
				Shadows: []core.BoxShadow{ui.Shadow(0, 2, 12, 0, ui.Black(.05))}}
		},
		Children: rows}
}
