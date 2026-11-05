package ui

import "github.com/oneo-me/nsgo/core"

// Panel is the card every other is made of: Head on the page, then the
// surface with the Children and the Footer after a hairline. Flush lays
// the children edge to edge, clipped to the surface's inner corners. OnPress
// makes the surface a button named Label.
type Panel struct {
	Head     core.Node
	Flush    bool
	Children []core.Node
	Footer   []core.Node
	OnPress  func()
	Label    string
	TestID   string
	// Style merges over the surface's.
	Style core.Style
}

// Render implements core.Node.
func (p Panel) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	const inner = RadiusCard - 1
	var parts []core.Node
	if len(p.Children) > 0 {
		body := core.Style{Gap: 12, Padding: core.All(14)}
		if p.Flush {
			r := core.Corners{TopLeft: inner, TopRight: inner}
			if len(p.Footer) == 0 {
				r.BottomLeft, r.BottomRight = inner, inner
			}
			body = core.Style{Overflow: core.Hidden, CornerRadii: r, BorderCurve: core.BorderCurveContinuous}
		}
		parts = append(parts, core.View{Style: body, Children: p.Children})
	}
	if len(p.Footer) > 0 {
		s := core.Style{Gap: 6, Padding: core.Symmetric(10, 14)}
		if len(parts) > 0 {
			s.Borders = core.Edges{Top: 1}
		}
		parts = append(parts, core.View{Style: s,
			StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }, Children: p.Footer})
	}
	surface := core.View{StyleFunc: func() core.Style { return CardStyle(cx).Merge(p.Style) }, Children: parts}
	if p.OnPress != nil {
		surface.Role, surface.Label, surface.OnPress = core.RoleButton, p.Label, func(core.PressEvent) { p.OnPress() }
		surface.Style = core.Style{Cursor: core.CursorPointer, Transition: quick}
		surface.StyleFunc = func() core.Style {
			colors := ColorsOf(cx)
			return CardStyle(cx).Merge(core.Style{Hover: &core.Style{BorderColor: colors.FieldBorder()},
				Active: &core.Style{Background: colors.Raised()}, Focus: FocusRing(cx, 2)}).Merge(p.Style)
		}
	}
	if p.Head == nil {
		surface.TestID = p.TestID
		return surface
	}
	return core.View{TestID: p.TestID, Style: core.Style{Gap: 8}, Children: []core.Node{p.Head, surface}}
}
