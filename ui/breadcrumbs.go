package ui

import (
	"math"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/base/locales"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// Breadcrumbs is a path filling its parent's width: its ancestors
// pressable, the last item the current one. When it does not fit, the
// ancestors after the first collapse into a menu under an ellipsis, as few
// as make it fit; the first and last items stay and truncate if still too
// wide.
type Breadcrumbs struct {
	Items []string
	// OnSelect receives the index of a pressed ancestor.
	OnSelect func(index int)
	TestID   string
	Style    core.Style
}

const (
	// breadcrumbInset is an item's padding on each side.
	breadcrumbInset core.Points = 4
	// breadcrumbGap is the row's gap, either side of a separator.
	breadcrumbGap core.Points = 2
	// breadcrumbSeparator is a separator's side.
	breadcrumbSeparator core.Points = 13
	// breadcrumbMore is the ellipsis's side, an item's height so that
	// collapsing keeps the row's.
	breadcrumbMore core.Points = 22
	// breadcrumbMinWidth is as far as an ancestor truncates, leaving a
	// letter and its ellipsis.
	breadcrumbMinWidth core.Points = 28
)

// Render implements core.Node.
func (b Breadcrumbs) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	sep := func() core.Node { return Icon(lucide.ChevronRight, breadcrumbSeparator, colors.Faint) }
	choose := func(i int) {
		if b.OnSelect != nil {
			b.OnSelect(i)
		}
	}
	crumb := func(i int) core.Node {
		return base.Button{Role: core.RoleLink, Label: b.Items[i], OnPress: func(core.PressEvent) { choose(i) },
			Style: core.Style{Padding: core.Symmetric(2, breadcrumbInset), CornerRadius: RadiusSmall, MinWidth: core.Pt(min(b.itemWidth(cx, i), breadcrumbMinWidth)), FlexShrink: 1},
			StyleFunc: func() core.Style {
				return core.Style{Color: colors.Muted(), Hover: &core.Style{Color: colors.Text(), Background: colors.Hover()}, Focus: FocusRing(cx, 0)}
			},
			Children: []core.Node{line(b.Items[i], TypeBody, 1)}}
	}
	more := locales.BreadcrumbsMore(cx)
	width := core.NewSignal(cx, core.Points(0))
	collapsed := core.NewMemo(cx, func() int { return b.hidden(cx, width.Get()) })
	return core.View{TestID: b.TestID, OnLayout: func(f core.Bounds) { width.Set(f.Size.Width) },
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: breadcrumbGap, Width: core.Pct(100), MinWidth: core.Pt(0), FlexShrink: 1}.Merge(b.Style),
		Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
			hidden := collapsed.Get()
			var out []core.Node
			for i := range b.Items {
				if hidden > 0 && i >= 1 && i <= hidden {
					if i == 1 {
						var items []core.Node
						for j := 1; j <= hidden; j++ {
							items = append(items, MenuItem{Text: b.Items[j], OnSelect: func() { choose(j) }})
						}
						out = append(out, sep(), Menu{Trigger: Button{Variant: ButtonGhost, Icon: lucide.Ellipsis, Tint: colors.Muted, Label: more, Side: breadcrumbMore},
							Label: more, Children: items})
					}
					continue
				}
				if i > 0 {
					out = append(out, sep())
				}
				if i == len(b.Items)-1 {
					out = append(out, core.Text{Content: b.Items[i], MaxLines: 1,
						Style: TypeLabel.Merge(core.Style{FlexShrink: 1, Padding: core.Symmetric(2, breadcrumbInset)}), StyleFunc: Colored(colors.Text)})
					continue
				}
				out = append(out, crumb(i))
			}
			return core.Group(out)
		})}}
}

// hidden returns how many ancestors after the first collapse for the path
// to fit width, none before its first layout says how wide it is.
func (b Breadcrumbs) hidden(cx *core.Scope, width core.Points) int {
	if width <= 0 || len(b.Items) < 3 {
		return 0
	}
	joint := breadcrumbSeparator + 2*breadcrumbGap
	widths := make([]core.Points, len(b.Items))
	total := core.Points(len(b.Items)-1) * joint
	for i := range b.Items {
		widths[i] = b.itemWidth(cx, i)
		total += widths[i]
	}
	if total <= width {
		return 0
	}
	// The ellipsis and its separator stand in for the hidden items and
	// theirs.
	total += breadcrumbMore + joint
	hidden := 0
	for hidden < len(b.Items)-2 && total > width {
		hidden++
		total -= widths[hidden] + joint
	}
	return hidden
}

// itemWidth returns how wide the item at index i is untruncated.
func (b Breadcrumbs) itemWidth(cx *core.Scope, i int) core.Points {
	font := core.Font{Size: TypeBody.FontSize}
	if i == len(b.Items)-1 {
		font = core.Font{Size: TypeLabel.FontSize, Weight: TypeLabel.FontWeight}
	}
	return core.Points(math.Ceil(float64(cx.MeasureText(b.Items[i], font).Width))) + 2*breadcrumbInset
}
