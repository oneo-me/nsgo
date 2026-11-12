// Package commitgraph is base/commitgraph's styled form: a repository's
// commits as a virtualized list beside their revision graph, in the look of
// Onegit's All Commits page.
package commitgraph

import (
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/base/commitgraph"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// RefKind is what a ref names.
type RefKind uint8

const (
	// RefBranch is a local branch.
	RefBranch RefKind = iota
	// RefRemote is a remote's branch.
	RefRemote
	// RefTag is a tag.
	RefTag
	// RefHead is the checkout: its branch, or HEAD detached.
	RefHead
)

// Ref is a label at a commit. Current marks in bold what the checkout is
// on, such as its upstream; a RefHead always is.
type Ref struct {
	Name    string
	Kind    RefKind
	Current bool
}

// Commit is a row of a View.
type Commit struct {
	Hash string
	// Parents are the commit's parents' hashes, the first parent first.
	Parents []string
	// Short is the abbreviated hash, the first 7 characters of Hash when
	// empty.
	Short   string
	Subject string
	// More tells that the message goes on after Subject.
	More bool
	// Avatar is the URL of the author's picture; without it the author's
	// initials show.
	Avatar string
	Author string
	Date   time.Time
	Refs   []Ref
	// Head marks the checked out commit, whose subject is bold.
	Head bool
}

// View lists Commits, newest first with each after its children as
// `git log --date-order` lists them, beside their revision graph. A merge's
// node collapses or expands the commits it brings in. Rows narrower than
// 560 points leave out the author, hash and date. Give it a height, or let
// it fill what the app sizes.
type View struct {
	Commits func() []Commit
	// Selection holds the chosen commits' hashes.
	Selection *ui.Selection
	// OnActivate receives the commit Enter or a double click activates.
	OnActivate func(Commit)
	// OnLoadMore runs as the end of the rows comes near, for the next page.
	OnLoadMore func()
	// Empty shows while there are no commits.
	Empty core.Node
	// Label names the list for assistive technology.
	Label  string
	Ref    *core.Ref
	TestID string
	Style  core.Style
}

// narrowWidth is the width below which rows show only refs and subjects.
const narrowWidth = 560

// Render implements core.Node.
func (v View) Render(cx *core.Scope) core.Node {
	commits := func() []Commit {
		if v.Commits == nil {
			return nil
		}
		return v.Commits()
	}
	history := commitgraph.NewHistory(cx, func() []commitgraph.Commit {
		list := commits()
		out := make([]commitgraph.Commit, len(list))
		for i, c := range list {
			out[i] = commitgraph.Commit{Hash: c.Hash, Parents: c.Parents}
		}
		return out
	}, v.Selection)
	at := func(i int) Commit {
		if i, list := history.At(i).Index, commits(); i >= 0 && i < len(list) {
			return list[i]
		}
		return Commit{}
	}
	var activate func(int)
	if v.OnActivate != nil {
		activate = func(i int) { v.OnActivate(core.Untracked(cx, func() Commit { return at(i) })) }
	}
	wide := core.NewSignal(cx, true)
	return core.View{TestID: v.TestID, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)}.Merge(v.Style),
		OnLayout: func(b core.Bounds) { wide.Set(b.Size.Width >= narrowWidth) },
		Children: []core.Node{ui.List{Label: v.Label, Ref: v.Ref, Selection: v.Selection, Count: history.Len, OnActivate: activate,
			OnLoadMore: v.OnLoadMore, Empty: v.Empty, EstimatedSize: commitgraph.RowHeight,
			Key:      func(i int) any { return history.At(i).Hash },
			RowLabel: func(i int) string { return at(i).Subject },
			RowStyle: core.Style{Height: core.Pt(commitgraph.RowHeight), MinHeight: core.Pt(commitgraph.RowHeight), Padding: core.Symmetric(0, 4), Gap: 0},
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				return commitRow{history: history, item: func() commitgraph.Item { return history.At(row.Index()) },
					commit: func() Commit { return at(row.Index()) }, wide: wide.Get}
			},
			Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)}}}}
}

// commitRow is one row's graph, refs, subject, author, hash and date.
type commitRow struct {
	history *commitgraph.History
	item    func() commitgraph.Item
	commit  func() Commit
	wide    func() bool
}

// Render implements core.Node.
func (r commitRow) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	refs := core.NewMemoFunc(cx, func() []Ref { return r.commit().Refs }, slices.Equal[[]Ref])
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: []core.Node{
		commitgraph.Graph{History: r.history, Item: r.item, Color: lineColor,
			MergeStyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: ui.Alpha(lineColor(r.item().Row.Line), 0.22)}}
			}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0),
			Gap: 6, Padding: core.Edges{Left: 4}}, Children: []core.Node{
			core.Show{When: func() bool { return len(refs.Get()) > 0 }, Then: func(cx *core.Scope) core.Node {
				// Refs give way to the subject rather than hiding it.
				return core.Dynamic(func(cx *core.Scope) core.Node {
					return refPills{refs: refs.Get(), column: func() int { return r.item().Row.Column }}
				})
			}},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(120), Gap: 4}, Children: []core.Node{
				core.Text{ContentFunc: func() string { return r.commit().Subject }, MaxLines: 1, Style: core.Style{FontSize: 13, FlexShrink: 1, MinWidth: core.Pt(0)},
					StyleFunc: func() core.Style {
						s := core.Style{Color: colors.Text()}
						if r.commit().Head {
							s.FontWeight = ui.Bold
						}
						return s
					}},
				core.Show{When: func() bool { return r.commit().More }, Then: func(cx *core.Scope) core.Node {
					return ui.Icon(lucide.CornerDownLeft, 12, colors.Faint)
				}},
			}},
			core.Show{When: r.wide, Then: func(cx *core.Scope) core.Node { return r.details(cx) }},
		}},
	}}
}

// details is a wide row's author, hash and date.
func (r commitRow) details(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	type author struct{ name, avatar string }
	who := core.NewMemo(cx, func() author { return author{r.commit().Author, r.commit().Avatar} })
	muted := func(fn func() string, width core.Points, style core.Style) core.Node {
		return core.Text{ContentFunc: fn, MaxLines: 1, StyleFunc: ui.Colored(colors.Muted),
			Style: core.Style{FontSize: 12, Width: core.Pt(width), FlexShrink: 0}.Merge(style)}
	}
	return core.Group{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Width: core.Pt(140), FlexShrink: 0, Gap: 6}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node {
				a := who.Get()
				return ui.Avatar{Name: a.name, Image: core.ImageSource{URI: a.avatar}, Size: 16}
			}),
			core.Text{ContentFunc: func() string { return who.Get().name }, MaxLines: 1, StyleFunc: ui.Colored(colors.Muted),
				Style: core.Style{FontSize: 12, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}},
		}},
		muted(func() string { return shortHash(r.commit()) }, 76, core.Style{FontFamily: ui.Mono}),
		muted(func() string { return commitgraph.FormatDate(cx, r.commit().Date) }, 124, core.Style{TextAlign: core.TextAlignEnd, FontVariant: core.FontTabularNums}),
	}
}

// shortHash is the hash a row shows.
func shortHash(c Commit) string {
	if c.Short != "" {
		return c.Short
	}
	return c.Hash[:min(7, len(c.Hash))]
}

// refPills labels the refs at a commit in the graph's column. A branch takes
// that column's color, so a tip's label matches the hue of its own line, and
// a tag one hue of its own.
type refPills struct {
	refs   []Ref
	column func() int
}

// Render implements core.Node.
func (p refPills) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	pills := make([]core.Node, len(p.refs))
	for i, ref := range p.refs {
		weight := ui.Regular
		if ref.Current || ref.Kind == RefHead {
			weight = ui.Bold
		}
		pills[i] = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(18), FlexShrink: 0, Gap: 4,
			Padding: core.Symmetric(0, 6), CornerRadius: 5, BorderWidth: 1},
			StyleFunc: func() core.Style {
				border, background := refColors(p.column(), cx.Dark())
				if ref.Kind == RefTag {
					border, background = tagColors(cx.Dark())
				}
				return core.Style{BorderColor: border, Background: background}
			},
			Children: []core.Node{ui.Icon(refIcon(ref.Kind), 11, colors.Text), ui.Text(ref.Name, 11, weight, colors.Text)}}
	}
	return core.View{Children: pills, Style: core.Style{Direction: core.Row, FlexShrink: 1, MinWidth: core.Pt(0), MaxWidth: core.Pt(320), Gap: 4, Overflow: core.Hidden}}
}

// refIcon leads a ref's label.
func refIcon(kind RefKind) func() *core.SVG {
	switch kind {
	case RefRemote:
		return lucide.Cloud
	case RefTag:
		return lucide.Tag
	case RefHead:
		return lucide.CircleDot
	}
	return lucide.GitBranch
}

// The graph's colors, Onegit's. A line keeps its color in both appearances;
// labels are muted in the dark one, wrapping around as the lines do.
var (
	linePalette = []core.Color{
		core.RGB(0xFF9502), core.RGB(0xFFCC00), core.RGB(0xFF3B30), core.RGB(0xA2845E),
		core.RGB(0x64DA38), core.RGB(0x1CADF8), core.RGB(0xCB73E1), core.RGB(0x8E8E91),
		core.RGB(0xFF2968), core.RGB(0x30D5C8), core.RGB(0x5856D6), core.RGB(0xB4D435),
		core.RGB(0xFF6F61),
	}
	refBackgroundsLight = []core.Color{
		core.RGB(0xFFF2DD), core.RGB(0xFFF9DC), core.RGB(0xFFE5E4), core.RGB(0xF3F0EA),
		core.RGB(0xE9FBE4), core.RGB(0xDFF5FF), core.RGB(0xFAECFC), core.RGB(0xF1F1F1),
		core.RGB(0xFFE3EC), core.RGB(0xDEF7F5), core.RGB(0xE5E5F7), core.RGB(0xF2F9DE),
		core.RGB(0xFFE5E2),
	}
	refBordersDark = []core.Color{
		core.RGB(0xF28B1F), core.RGB(0xBF9A2F), core.RGB(0xCB2327), core.RGB(0xA68357),
		core.RGB(0x27A649), core.RGB(0x0082BA), core.RGB(0x9D53A0), core.RGB(0x6D6D6E),
		core.RGB(0xCC1F56), core.RGB(0x20A89E), core.RGB(0x4A48B0), core.RGB(0x8DAA28),
		core.RGB(0xE05A4D),
	}
	refBackgroundsDark = []core.Color{
		core.RGB(0x5D3D14), core.RGB(0x5B4C0E), core.RGB(0x5F2425), core.RGB(0x433A32),
		core.RGB(0x285224), core.RGB(0x13445B), core.RGB(0x503455), core.RGB(0x3D3D3F),
		core.RGB(0x601E35), core.RGB(0x1A4A45), core.RGB(0x272650), core.RGB(0x3D4E1A),
		core.RGB(0x5D2A24),
	}
)

// lineColor is line's color.
func lineColor(line int) core.Color { return linePalette[max(line, 0)%len(linePalette)] }

// refColors returns the border and background of a label at a commit in the
// graph's column.
func refColors(column int, dark bool) (border, background core.Color) {
	i := max(column, 0) % len(linePalette)
	if dark {
		return refBordersDark[i], refBackgroundsDark[i]
	}
	return linePalette[i], refBackgroundsLight[i]
}

// tagColors returns the border and background of a tag's label.
func tagColors(dark bool) (border, background core.Color) {
	if dark {
		return core.RGB(0xB5B1F6), core.RGB(0x3E3F4E)
	}
	return core.RGB(0xB5B1F6), core.RGB(0xF6F5FE)
}
