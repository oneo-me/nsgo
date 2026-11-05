package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/mobile"
)

// NavPage returns p, a page of a base.Nav, on the page's background under
// header, a NavHeader whose title and ends default to p's, unless p has a
// Header of its own. On iOS and Android the system header hides for it,
// and it reaches under the status bar.
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
	if p.Header == nil {
		if header.Title == "" {
			header.Title = p.Title
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
