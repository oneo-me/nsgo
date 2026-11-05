package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Breadcrumbs is a path: its ancestors pressable, the last item the
// current one. With more than MaxItems, the ancestors after the first
// collapse into a menu under an ellipsis named MoreLabel.
type Breadcrumbs struct {
	Items []string
	// OnSelect receives the index of a pressed ancestor.
	OnSelect  func(index int)
	MaxItems  int
	MoreLabel string
	TestID    string
	Style     core.Style
}

// Render implements core.Node.
func (b Breadcrumbs) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	sep := func() core.Node { return Icon(lucide.ChevronRight, 13, colors.Faint) }
	choose := func(i int) {
		if b.OnSelect != nil {
			b.OnSelect(i)
		}
	}
	crumb := func(i int) core.Node {
		return base.Button{Role: core.RoleLink, Label: b.Items[i], OnPress: func(core.PressEvent) { choose(i) },
			Style: core.Style{Padding: core.Symmetric(2, 4), CornerRadius: RadiusSmall, MinWidth: core.Pt(0), FlexShrink: 1},
			StyleFunc: func() core.Style {
				return core.Style{Color: colors.Muted(), Hover: &core.Style{Color: colors.Text(), Background: colors.Hover()}, Focus: FocusRing(cx, 0)}
			},
			Children: []core.Node{line(b.Items[i], TypeBody, 1)}}
	}
	hidden := 0
	if b.MaxItems > 1 && len(b.Items) > b.MaxItems {
		hidden = len(b.Items) - b.MaxItems + 1
	}
	var out []core.Node
	for i := range b.Items {
		if hidden > 0 && i >= 1 && i <= hidden {
			if i == 1 {
				var items []core.Node
				for j := 1; j <= hidden; j++ {
					items = append(items, MenuItem{Text: b.Items[j], OnSelect: func() { choose(j) }})
				}
				out = append(out, sep(), Menu{Trigger: Button{Variant: ButtonGhost, Icon: lucide.Ellipsis, Label: b.MoreLabel},
					Label: b.MoreLabel, Children: items})
			}
			continue
		}
		if i > 0 {
			out = append(out, sep())
		}
		if i == len(b.Items)-1 {
			out = append(out, core.Text{Content: b.Items[i], MaxLines: 1,
				Style: TypeLabel.Merge(core.Style{FlexShrink: 1, Padding: core.Symmetric(2, 4)}), StyleFunc: Colored(colors.Text)})
			continue
		}
		out = append(out, crumb(i))
	}
	return core.View{TestID: b.TestID,
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2, MinWidth: core.Pt(0)}.Merge(b.Style), Children: out}
}
