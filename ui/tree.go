package ui

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Tree is the app's nested items as a List of the expanded ones,
// indented by depth, each branch with a chevron that expands it. It
// keeps no state: the app says which keys are expanded and sets them
// from OnExpand. On top of a List's keys, Right expands a branch or moves
// into it, and Left collapses it or moves to its parent.
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

// treeEntry is an item of a tree's expanded rows.
type treeEntry struct {
	key    any
	depth  int
	parent int
	branch bool
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
	rows := core.NewMemoFunc(cx, func() []treeEntry {
		var out []treeEntry
		var walk func(keys []any, depth, parent int)
		walk = func(keys []any, depth, parent int) {
			for _, key := range keys {
				kids := children(key)
				out = append(out, treeEntry{key: key, depth: depth, parent: parent, branch: len(kids) > 0})
				if len(kids) > 0 && expanded(key) {
					walk(kids, depth+1, len(out)-1)
				}
			}
		}
		if tr.Roots != nil {
			walk(tr.Roots(), 0, -1)
		}
		return out
	}, func(a, b []treeEntry) bool { return false })
	entry := func(i int) treeEntry {
		r := rows.Get()
		if i < 0 || i >= len(r) {
			return treeEntry{parent: -1}
		}
		return r[i]
	}
	toggle := func(i int, open bool) {
		if e := entry(i); e.branch && tr.OnExpand != nil {
			tr.OnExpand(e.key, open)
		}
	}
	const indent = iconSize + 4
	row := func(cx *core.Scope, r core.ListRow) core.Node {
		item := TreeItem{Row: r,
			Key:      func() any { return entry(r.Index()).key },
			Depth:    func() int { return entry(r.Index()).depth },
			Branch:   func() bool { return entry(r.Index()).branch },
			Expanded: func() bool { e := entry(r.Index()); return e.branch && expanded(e.key) }}
		chevron := core.View{Role: core.RoleButton, Label: tr.ToggleLabel, NoFocus: true, Expanded: item.Expanded,
			AccessibilityElementsHidden: func() bool { return !item.Branch() },
			OnPress:                     func(core.PressEvent) { toggle(r.Index(), !item.Expanded()) },
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
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, FlexGrow: 1, MinWidth: core.Pt(0)},
			StyleFunc: func() core.Style { return core.Style{Padding: core.Edges{Left: core.Points(item.Depth()) * indent}} },
			Children:  content}
	}
	var label func(int) string
	if tr.RowLabel != nil {
		label = func(i int) string { return tr.RowLabel(entry(i).key) }
	}
	var activate func(int)
	if tr.OnActivate != nil {
		activate = func(i int) { tr.OnActivate(entry(i).key) }
	}
	rowStyle := core.Style{Padding: core.Symmetric(0, 4), MinHeight: core.Pt(RowHeight(cx) - 8)}.Merge(tr.RowStyle)
	return List{Count: func() int { return len(rows.Get()) }, Key: func(i int) any { return entry(i).key }, Row: row,
		Selection: tr.Selection, OnActivate: activate, RowLabel: label, EstimatedSize: tr.EstimatedSize, Empty: tr.Empty,
		Label: tr.Label, Ref: tr.Ref, TestID: tr.TestID, Style: tr.Style, RowStyle: rowStyle,
		expanded: func(i int) bool { e := entry(i); return e.branch && expanded(e.key) },
		keys: func(cx *core.Scope, k *listKeys) {
			repeat := core.KeyOptions{Repeat: true}
			cx.OnKeyWith("right", repeat, func() {
				i := k.cursor()
				e := entry(i)
				switch {
				case !e.branch:
				case !expanded(e.key):
					toggle(i, true)
				default:
					k.moveTo(i+1, false)
				}
			})
			cx.OnKeyWith("left", repeat, func() {
				i := k.cursor()
				e := entry(i)
				if e.branch && expanded(e.key) {
					toggle(i, false)
				} else if e.parent >= 0 {
					k.moveTo(e.parent, false)
				}
			})
		}}
}
