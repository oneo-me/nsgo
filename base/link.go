package base

import "github.com/oneo-me/nsgo/core"

// LinkOpenWith is how a Link opens its Href.
type LinkOpenWith uint8

const (
	// LinkOpenSystem opens a URL of another app or site outside the app
	// (core.Scope.OpenURL); any other Href, a path of the app's, is only
	// reported.
	LinkOpenSystem LinkOpenWith = iota
	// LinkOpenApp opens nothing: OnOpen is how it opens.
	LinkOpenApp
)

// Link is content that goes somewhere when pressed, with the link role and
// no look of its own. In a Toolbar it is one of its items.
type Link struct {
	// Href is where it goes, a URL or a path of the app's, never shown.
	Href     string
	HrefFunc func() string
	// OpenWith says who opens Href on a press; OnOpen hears it either way.
	OpenWith LinkOpenWith
	Disabled func() bool
	// OnOpen hears it open its Href, before OnPress.
	OnOpen func(href string)
	// OnPress hears the press, after it opened its Href.
	OnPress   func(core.PressEvent)
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// HitSlop and HitSlopFunc are how far outside it presses count.
	HitSlop     core.Edges
	HitSlopFunc func() core.Edges
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (l Link) Render(cx *core.Scope) core.Node {
	href := func() string {
		if l.HrefFunc != nil {
			return core.Untracked(cx, l.HrefFunc)
		}
		return l.Href
	}
	v := core.View{Class: l.Class, Style: l.Style, StyleFunc: l.StyleFunc, Role: core.RoleLink, Label: l.Label, Hint: l.Hint, TestID: l.TestID, OnLayout: l.OnLayout, Tooltip: l.Tooltip,
		HitSlop: l.HitSlop, HitSlopFunc: l.HitSlopFunc, Disabled: l.Disabled, Children: l.Children,
		OnPress: func(e core.PressEvent) {
			if to := href(); to != "" {
				if l.OnOpen != nil {
					l.OnOpen(to)
				}
				if l.OpenWith == LinkOpenSystem && core.CheckURL(to) == nil {
					cx.OpenURL(to, nil)
				}
			}
			if l.OnPress != nil {
				l.OnPress(e)
			}
		}}
	return barItem(cx, v, nil)
}
