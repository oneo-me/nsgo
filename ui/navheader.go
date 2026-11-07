package ui

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// navEdgeTail is how far a soft edge's blur runs past its bars.
const navEdgeTail core.Points = 24

// systemHeader reports whether a page shows the system's header over its
// content, with the system's glass. Only iOS sets it apart from what
// scrolls under it so far.
func systemHeader(cx *core.Scope) bool { return cx.Env().OS() == core.OSIOS }

// NavInset is the room to leave at the top of a page's content that does not
// scroll for a NavPage's system header, and 0 where the header takes room of
// its own. Scroll views under the header are inset already. It is tracked as
// the safe area changes.
func NavInset(cx *core.Scope) core.Points {
	if !systemHeader(cx) {
		return 0
	}
	return core.Insets(cx).Top
}

// NavHeader is the bar at the top of a page: a back button named
// BackLabel while OnBack is set, the title, and the app's views at
// either end. On iOS NavPage gives these to the system's header, whose
// back button pops the page.
type NavHeader struct {
	Title string
	// TitleFunc replaces Title as the graph changes.
	TitleFunc func() string
	OnBack    func()
	BackLabel string
	Leading   []core.Node
	Trailing  []core.Node
	// Scroll is the page's scroll view, ScrollFunc when it changes: under a
	// system header it blurs progressively toward the top.
	Scroll     *core.Ref
	ScrollFunc func() *core.Ref
	// Symbols has the system's header show its own buttons for what takes
	// presses in Trailing, one each in order, as the back button is drawn.
	Symbols func() []core.HeaderSymbol
	TestID  string
}

// Render implements core.Node.
func (h NavHeader) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var children []core.Node
	if h.OnBack != nil {
		children = append(children, IconButton{Icon: lucide.ChevronLeft, Label: h.BackLabel, Size: 32, OnPress: h.OnBack})
	}
	children = append(children, h.Leading...)
	children = append(children, core.Text{Content: h.Title, ContentFunc: h.TitleFunc, MaxLines: 1, Role: core.RoleHeader,
		Style: TypeTitle.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), StyleFunc: Colored(colors.Text)})
	children = append(children, h.Trailing...)
	return core.View{TestID: h.TestID, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
		MinHeight: core.Pt(44), Padding: core.Symmetric(0, 12), Borders: core.Edges{Bottom: 1}},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Surface(), BorderColors: core.EdgeColors{Bottom: colors.Line()}}
		}, Children: children}
}
