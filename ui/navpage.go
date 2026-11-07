package ui

import (
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/mobile"
)

// NavPage returns p, a page of a base.Nav, on the page's background under
// header, a NavHeader whose title and ends default to p's, unless p has a
// Header of its own. On Android the system header hides for it, and it
// reaches under the status bar. On iOS the system's transparent header shows
// header's title and trailing views over the page, which scrolls under it.
func NavPage(cx *core.Scope, p base.NavPage, header NavHeader) base.NavPage {
	colors := ColorsOf(cx)
	app := p.StyleFunc
	p.StyleFunc = func() core.Style {
		s := core.Style{Background: colors.Background()}
		if app != nil {
			s = s.Merge(app())
		}
		return s
	}
	if systemHeader(cx) {
		return systemNavPage(cx, p, header)
	}
	if p.Header == nil {
		if header.Title == "" {
			header.Title = p.Title
		}
		if header.TitleFunc == nil {
			header.TitleFunc = p.TitleFunc
		}
		if header.Leading == nil && p.HeaderLeft != nil {
			header.Leading = []core.Node{p.HeaderLeft}
		}
		if header.Trailing == nil && p.HeaderRight != nil {
			header.Trailing = []core.Node{p.HeaderRight}
		}
		p.Header = header
	}
	if mobile.IsAvailable(cx) {
		// The system mounts the header views in its bar even while hidden.
		p.HeaderHidden, p.HeaderLeft, p.HeaderRight, p.HeaderTitle = true, nil, nil, nil
		top := core.View{Style: core.Style{PaddingEnv: core.EnvEdges{Top: core.EnvSafeArea, Right: core.EnvSafeArea, Left: core.EnvSafeArea}},
			StyleFunc: Filled(colors.Surface), Children: []core.Node{p.Header}}
		p.Header, p.Children = nil, append([]core.Node{top}, p.Children...)
	}
	return p
}

// systemNavPage shows header in the system's header, transparent over p,
// with a bar under it that blurs the page's scroll view as far as the header
// reaches, the blur's tail included.
func systemNavPage(cx *core.Scope, p base.NavPage, header NavHeader) base.NavPage {
	colors := ColorsOf(cx)
	p.HeaderTransparent, p.Header = true, nil
	p.HeaderStyleFunc = func() core.Style { return core.Style{Color: colors.Text(), TintColor: colors.Text()} }
	if header.Title != "" {
		p.Title = header.Title
	}
	if header.TitleFunc != nil {
		p.TitleFunc = header.TitleFunc
	}
	if len(header.Leading) > 0 {
		p.HeaderLeft = navItems(header.Leading)
	}
	if len(header.Trailing) > 0 {
		p.HeaderRight = navItems(header.Trailing)
		p.IOS.HeaderRightSymbols = header.Symbols
	}
	scroll := header.ScrollFunc
	if scroll == nil {
		scroll = func() *core.Ref { return header.Scroll }
	}
	bar := core.Dynamic(func(cx *core.Scope) core.Node {
		return core.View{EdgeBar: core.EdgeBar{Scroll: scroll(), Side: core.SideTop}, PointerEvents: core.PointerEventsNone,
			Style:     core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Left: core.Pt(0)},
			StyleFunc: func() core.Style { return core.Style{Height: core.Pt(max(0, core.Insets(cx).Top-navEdgeTail))} }}
	})
	p.Children = append(slices.Clip(p.Children), bar)
	return p
}

// navItems lays out views of the system header in a row.
func navItems(children []core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter}, Children: children}
}
