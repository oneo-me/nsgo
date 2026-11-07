package ui

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// NavHeader is the bar at the top of a page: a back button named
// BackLabel while OnBack is set, the title, and the app's views at
// either end.
type NavHeader struct {
	Title     string
	OnBack    func()
	BackLabel string
	Leading   []core.Node
	Trailing  []core.Node
	TestID    string
}

// Render implements core.Node.
func (h NavHeader) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var children []core.Node
	if h.OnBack != nil {
		children = append(children, IconButton{Icon: lucide.ChevronLeft, Label: h.BackLabel, Size: 32, OnPress: h.OnBack})
	}
	children = append(children, h.Leading...)
	children = append(children, core.Text{Content: h.Title, MaxLines: 1, Role: core.RoleHeader,
		Style: TypeTitle.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), StyleFunc: Colored(colors.Text)})
	children = append(children, h.Trailing...)
	return core.View{TestID: h.TestID, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
		MinHeight: core.Pt(44), Padding: core.Symmetric(0, 12), Borders: core.Edges{Bottom: 1}},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Surface(), BorderColors: core.EdgeColors{Bottom: colors.Line()}}
		}, Children: children}
}
