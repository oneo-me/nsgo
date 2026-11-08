package ui

import (
	"fmt"
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Tree is base.Tree in Onevra's look: the app's nested items in rows
// indented by depth, each branch with a chevron that expands it, as a
// double press does. It keeps no state: the app says which keys are
// expanded and sets them from OnExpand, and only the children of what is
// expanded are read. A press only selects, the rows a multiple Selection
// holds by Shift and Mod as in a List, which assistive technology hears
// for the cursor's row alone.
type Tree struct {
	// Roots returns the top items' keys, comparable values such as IDs.
	Roots func() []any
	// Children returns an item's children, none for a leaf.
	Children func(key any) []any
	Expanded func(key any) bool
	OnExpand func(key any, expanded bool)
	// Row builds an item's content after its chevron; read it as
	// signals, since a cell shows other items as it scrolls.
	Row        func(cx *core.Scope, item TreeItem) core.Node
	Selection  *Selection
	OnActivate func(key any)
	// RowLabel is what assistive technology reads for an item, its text
	// when nil.
	RowLabel func(key any) string
	// ToggleLabel names the chevron for assistive technology, such as
	// "Expand".
	ToggleLabel   string
	EstimatedSize core.Points
	Empty         core.Node
	Label         string
	Ref           *core.Ref
	TestID        string
	Style         core.Style
	RowStyle      core.Style
	// RowStyleFunc is List's.
	RowStyleFunc func() core.Style
}

// TreeItem is the item a Tree's cell shows, as signals.
type TreeItem struct {
	Row core.ListRow
	// Key, Depth (0 for a root) and Branch, whether it has children.
	Key      func() any
	Depth    func() int
	Branch   func() bool
	Expanded func() bool
}

// Render implements core.Node.
func (tr Tree) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	expanded := func(key any) bool { return tr.Expanded != nil && tr.Expanded(key) }
	children := func(key any) []any {
		if tr.Children == nil {
			return nil
		}
		return tr.Children(key)
	}
	id := func(key any) string { return fmt.Sprintf("%T:%v", key, key) }
	byID := map[string]any{}
	var ids []string
	// standIn is a child no row shows: it makes a collapsed branch a
	// folder without reading its children's children.
	standIn := func(id string) []base.TreeItem { return []base.TreeItem{{ID: id + "\x00"}} }
	var items func(keys []any) []base.TreeItem
	items = func(keys []any) []base.TreeItem {
		out := make([]base.TreeItem, 0, len(keys))
		for _, key := range keys {
			item := base.TreeItem{ID: id(key)}
			byID[item.ID] = key
			if tr.RowLabel != nil {
				item.Label = tr.RowLabel(key)
			}
			if kids := children(key); len(kids) > 0 {
				if expanded(key) {
					ids = append(ids, item.ID)
					item.Children = items(kids)
				} else {
					item.Children = standIn(item.ID)
				}
			}
			out = append(out, item)
		}
		return out
	}
	// One walk of what shows gives the items and what is expanded.
	walked := core.NewMemoFunc(cx, func() []base.TreeItem {
		ids = nil
		if tr.Roots == nil {
			return nil
		}
		return items(tr.Roots())
	}, func(a, b []base.TreeItem) bool { return false })
	selection := tr.Selection
	if selection == nil {
		selection = NewSelection(cx, false)
	}
	// shownKeys are the rows' keys in order: what the walk holds is what
	// shows, but for the stand-ins.
	shownKeys := func() []any {
		var keys []any
		var push func(items []base.TreeItem)
		push = func(items []base.TreeItem) {
			for i := range items {
				if key, ok := byID[items[i].ID]; ok {
					keys = append(keys, key)
					push(items[i].Children)
				}
			}
		}
		push(walked.GetUntracked())
		return keys
	}
	desktop := cx.Env().Desktop
	// choose selects key as a press does: in a multiple Selection Shift
	// takes the rows from the anchor and Mod toggles it.
	choose := func(key any, e core.PressEvent) {
		if !selection.Multiple() {
			selection.Set(key)
			return
		}
		switch m := e.Modifiers; {
		case m.Shift && selection.Anchor() != nil:
			anchor := selection.Anchor()
			keys := shownKeys()
			from, to := slices.Index(keys, anchor), slices.Index(keys, key)
			if from < 0 {
				from = to
			}
			selection.SetRange(anchor, key, keys[min(from, to):max(from, to)+1]...)
		case m.Platform || m.Control || !core.Untracked(cx, desktop):
			selection.Toggle(key)
		default:
			selection.Set(key)
		}
	}
	const indent = iconSize + 4
	surface := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(RowHeight - 8), Padding: core.Symmetric(0, 4),
		CornerRadius: RadiusControl, MinWidth: core.Pt(0), Cursor: core.CursorDefault}.Merge(tr.RowStyle)
	row := func(cx *core.Scope, r base.TreeRow) core.Node {
		key := func() any { return byID[r.Entry().ID] }
		item := TreeItem{Row: r.Row, Key: key, Depth: func() int { return r.Entry().Depth },
			Branch: func() bool { return r.Entry().Folder }, Expanded: r.Expanded}
		chevron := base.Button{Label: tr.ToggleLabel, NoFocus: true, Expanded: item.Expanded,
			AccessibilityElementsHidden: func() bool { return !item.Branch() },
			OnPress:                     func(core.PressEvent) { r.Toggle() },
			Style: core.Style{Width: core.Pt(iconSize), Height: core.Pt(iconSize), FlexShrink: 0, CornerRadius: RadiusSmall,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: func() core.Style {
				s := core.Style{Color: colors.Muted()}
				if !item.Branch() {
					s.Opacity, s.Cursor = core.Alpha(0), core.CursorDefault
				}
				if item.Expanded() {
					s.Transform = core.Transform{Rotate: 90}
				}
				return s
			},
			Children: []core.Node{Icon(lucide.ChevronRight, iconSize-3, Inherited)}}
		if !ReduceMotion(cx) {
			chevron.Style.Transition = core.Transition{Duration: quick.Duration, Easing: core.EaseOut, Properties: core.TransitionTransform}
		}
		content := []core.Node{chevron}
		if tr.Row != nil {
			content = append(content, tr.Row(cx, item))
		}
		// The surface fills the row and takes its presses, which would
		// expand a folder in base.Tree, and shows every row selected.
		return core.View{Style: surface, StyleFunc: rowLook(cx, true, core.Style{}, tr.RowStyleFunc),
			Selected: func() bool { return selection.Has(key()) },
			OnPress: func(e core.PressEvent) {
				choose(key(), e)
				r.TakeKeys()
				if e.Clicks == 2 {
					r.Toggle()
					if tr.OnActivate != nil {
						tr.OnActivate(key())
					}
				}
			},
			Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, FlexGrow: 1, MinWidth: core.Pt(0)},
				StyleFunc: func() core.Style { return core.Style{Padding: core.Edges{Left: core.Points(item.Depth()) * indent}} },
				Children:  content}}}
	}
	var action func(string)
	if tr.OnActivate != nil {
		action = func(id string) { tr.OnActivate(byID[id]) }
	}
	onExpand := func(id string, open bool) {
		if tr.OnExpand != nil {
			tr.OnExpand(byID[id], open)
		}
	}
	size := tr.EstimatedSize
	if size == 0 {
		size = RowHeight
	}
	return base.Tree{Items: walked.Get,
		Expanded: func() []string {
			walked.Get()
			return ids
		},
		Selected: func() *string {
			if key := selection.Cursor(); key != nil {
				s := id(key)
				return &s
			}
			return nil
		},
		OnSelectedChange: func(id string) { selection.Set(byID[id]) },
		OnExpand:         func(id string) { onExpand(id, true) },
		OnCollapse:       func(id string) { onExpand(id, false) },
		OnAction:         action,
		Row:              row,
		EstimatedSize:    size, Empty: tr.Empty, Label: tr.Label, ListRef: tr.Ref, TestID: tr.TestID,
		Style:     core.Style{MinHeight: core.Pt(0), CornerRadius: RadiusControl}.Merge(tr.Style),
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		ListStyle: core.Style{Padding: core.All(4)},
		ItemStyle: core.Style{CornerRadius: RadiusControl, MinWidth: core.Pt(0)},
		ItemStyleFunc: func() core.Style {
			return core.Style{Focus: &core.Style{OutlineWidth: 2, OutlineColor: colors.Now().Info, OutlineOffset: -2}}
		}}
}
