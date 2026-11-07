package ui

import (
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Pagination is the pages of a set in a row of buttons, the current one in
// the accent, with chevrons for Previous and Next: base.Pagination's first,
// last and neighbouring pages around ellipses. Compact keeps the chevrons
// and Children, a pager for a narrow screen.
type Pagination struct {
	Page         func() int
	TotalPages   func() int
	VisiblePages int
	Compact      func() bool
	Disabled     func() bool
	OnChange     func(page int)
	// Label names it for assistive technology, PreviousLabel and NextLabel
	// its chevrons.
	Label, PreviousLabel, NextLabel string
	Children                        []core.Node
	TestID                          string
	Style                           core.Style
}

// Render implements core.Node.
func (p Pagination) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	item := func(i base.PaginationItem) core.Node {
		switch i.Kind {
		case base.PaginationPrevious:
			return Icon(lucide.ChevronLeft, iconSize, Inherited)
		case base.PaginationNext:
			return Icon(lucide.ChevronRight, iconSize, Inherited)
		case base.PaginationEllipsis:
			return core.Text{Content: "…", Style: TypeBody}
		}
		return core.Text{Content: strconv.Itoa(i.Page), Style: TypeBody}
	}
	return base.Pagination{Page: p.Page, TotalPages: p.TotalPages, VisiblePages: p.VisiblePages, Compact: p.Compact, Disabled: p.Disabled,
		OnChange: p.OnChange, Label: p.Label, PreviousLabel: p.PreviousLabel, NextLabel: p.NextLabel, RenderItem: item, TestID: p.TestID, Children: p.Children,
		Style: core.Style{AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, Gap: 2}.Merge(p.Style),
		ItemStyle: core.Style{MinWidth: core.Pt(ButtonHeight), Height: core.Pt(ButtonHeight), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: RadiusControl, Transition: quick, Disabled: Dimmed()},
		ItemStyleFunc: func() core.Style {
			c := cx.Palette()
			return core.Style{Color: colors.Text(), Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()},
				Focus:    FocusRing(cx, 2),
				Selected: &core.Style{Background: c.Accent, Color: colors.OnAccent(), Hover: &core.Style{Background: c.AccentHover}}}
		}}
}
