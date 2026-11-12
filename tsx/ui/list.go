package ui

import (
	"slices"
	"strconv"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// syncSelection keeps s and what JS holds alike: selected, when set, sets
// s as it changes, and change hears what the user picks.
func syncSelection(cx *core.Scope, s *ui.Selection, selected func() []string, change func([]string)) {
	strs := func() []string {
		keys := s.Keys()
		out := make([]string, 0, len(keys))
		for _, k := range keys {
			if str, ok := k.(string); ok {
				out = append(out, str)
			}
		}
		slices.Sort(out)
		return out
	}
	if selected != nil {
		cx.Effect(func() {
			want := slices.Sorted(slices.Values(selected()))
			if core.Untracked(cx, func() bool { return slices.Equal(want, strs()) }) {
				return
			}
			keys := make([]any, len(want))
			for i, k := range want {
				keys[i] = k
			}
			core.Untracked(cx, func() bool { s.Set(keys...); return true })
		})
	}
	if change != nil {
		last := strs()
		cx.Effect(func() {
			now := strs()
			if slices.Equal(now, last) {
				return
			}
			last = now
			core.Untracked(cx, func() bool { change(now); return true })
		})
	}
}

// List is ui.List: Row gets a row's index, Key names rows, by their index
// when nil, and the selection is those names.
type List struct {
	Count            func() int
	Key              func(i int) string
	Row              func(cx *core.Scope, index func() int) core.Node
	Multiple         bool
	Selected         func() []string
	OnSelectedChange func(selected []string)
	OnActivate       func(index int)
	TapSelects       bool
	RowLabel         func(i int) string
	Columns          int
	EstimatedSize    core.Points
	Gap              core.Points
	Header           core.Node
	Footer           core.Node
	Empty            core.Node
	OnLoadMore       func()
	Label            string
	Ref              *core.Ref
	TestID           string
	Style            core.Style
	RowStyle         core.Style
}

// Render implements core.Node.
func (l List) Render(cx *core.Scope) core.Node {
	selection := ui.NewSelection(cx, l.Multiple)
	syncSelection(cx, selection, l.Selected, l.OnSelectedChange)
	return ui.List{Count: l.Count, Selection: selection, OnActivate: l.OnActivate, TapSelects: l.TapSelects, RowLabel: l.RowLabel,
		Columns: l.Columns, EstimatedSize: l.EstimatedSize, Gap: l.Gap, Header: l.Header, Footer: l.Footer, Empty: l.Empty,
		OnLoadMore: l.OnLoadMore, Label: l.Label, Ref: l.Ref, TestID: l.TestID, Style: l.Style, RowStyle: l.RowStyle,
		Key: func(i int) any {
			if l.Key != nil {
				return l.Key(i)
			}
			return strconv.Itoa(i)
		},
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			return l.Row(cx, row.Index)
		}}
}

// TreeNode is an item of a Tree: its ID, what it shows, and its children.
type TreeNode struct {
	ID       string
	Text     string
	Icon     func() *core.SVG
	Children []TreeNode
}

// Tree is ui.Tree of data: each of Items shows its Text after its Icon. It
// holds what is expanded itself, from DefaultExpanded, and the selection
// is the selected items' IDs.
type Tree struct {
	Items            func() []TreeNode
	DefaultExpanded  []string
	Selected         func() []string
	OnSelectedChange func(selected []string)
	OnActivate       func(id string)
	ToggleLabel      string
	EstimatedSize    core.Points
	Empty            core.Node
	Label            string
	TestID           string
	Style            core.Style
}

// Render implements core.Node.
func (t Tree) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	open := core.NewSignalFunc(cx, slices.Clone(t.DefaultExpanded), slices.Equal)
	selection := ui.NewSelection(cx, false)
	syncSelection(cx, selection, t.Selected, t.OnSelectedChange)
	nodes := core.NewMemoFunc(cx, func() map[string]TreeNode {
		m := map[string]TreeNode{}
		var walk func(items []TreeNode)
		walk = func(items []TreeNode) {
			for _, n := range items {
				m[n.ID] = n
				walk(n.Children)
			}
		}
		if t.Items != nil {
			walk(t.Items())
		}
		return m
	}, func(a, b map[string]TreeNode) bool { return false })
	keys := func(items []TreeNode) []any {
		out := make([]any, len(items))
		for i, n := range items {
			out[i] = n.ID
		}
		return out
	}
	tree := ui.Tree{Label: t.Label, ToggleLabel: t.ToggleLabel, EstimatedSize: t.EstimatedSize, Empty: t.Empty, TestID: t.TestID, Style: t.Style,
		Selection: selection,
		Roots: func() []any {
			if t.Items == nil {
				return nil
			}
			return keys(t.Items())
		},
		Children: func(k any) []any { return keys(nodes.Get()[k.(string)].Children) },
		Expanded: func(k any) bool { return slices.Contains(open.Get(), k.(string)) },
		OnExpand: func(k any, on bool) {
			next := slices.DeleteFunc(slices.Clone(open.GetUntracked()), func(s string) bool { return s == k })
			if on {
				next = append(next, k.(string))
			}
			open.Set(next)
		},
		RowLabel: func(k any) string { return nodes.GetUntracked()[k.(string)].Text },
		Row: func(cx *core.Scope, item ui.TreeItem) core.Node {
			node := func() TreeNode { return nodes.Get()[item.Key().(string)] }
			return ui.HStack{Gap: 6, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node {
					if n := node(); n.Icon != nil {
						return ui.IconOf(n.Icon, 14, colors.Muted)
					}
					return nil
				}),
				ui.Textf(func() string { return node().Text }, 13, ui.Regular, colors.Text)}}
		}}
	if t.OnActivate != nil {
		tree.OnActivate = func(k any) { t.OnActivate(k.(string)) }
	}
	return tree
}
