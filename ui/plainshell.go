package ui

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
)

// PlainShell is a window of the app's content under a header: the app's
// Leading content, its Title where Leading is nil, and its Actions at the
// end. Content starts below the header, or under it with BehindHeader,
// keeping its controls clear with ShellHeaderHeight. The header leaves a
// window material showing through, and is on the page's background where
// none shows, as the content is.
type PlainShell struct {
	Title        string
	Leading      []core.Node
	Actions      []core.Node
	Content      []core.Node
	BehindHeader bool
	TestID       string
}

// Render implements core.Node.
func (p PlainShell) Render(cx *core.Scope) core.Node {
	hideTitleBar(cx)
	colors := ColorsOf(cx)
	lead := p.Leading
	if lead == nil && p.Title != "" {
		lead = []core.Node{core.Text{Content: p.Title, MaxLines: 1, Role: core.RoleHeader,
			Style: TypeLabel.Merge(core.Style{FlexShrink: 1}), StyleFunc: Colored(colors.Text)}}
	}
	head := []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexGrow: 1,
		FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pct(100), Overflow: core.Hidden}, Children: lead}}
	head = append(head, p.Actions...)
	page := func() core.Color {
		if desktop.ShowsMaterial(cx) {
			return 0
		}
		return colors.Background()
	}
	fill := page
	if p.BehindHeader {
		fill = func() core.Color { return 0 }
	}
	bar := shellHeader(cx, "shell-header", func() core.Points { return controlsEnd(cx) }, true, fill, head)
	body := core.View{Style: FillRest, StyleFunc: Filled(colors.Background), Children: p.Content}
	children := []core.Node{bar, body}
	if p.BehindHeader {
		bar = core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0)},
			Children: []core.Node{bar}}
		children = []core.Node{body, bar}
	}
	return core.View{TestID: p.TestID, Style: FillRest, StyleFunc: func() core.Style {
		return core.Style{Background: page(), Color: colors.Text()}
	}, Children: children}
}
