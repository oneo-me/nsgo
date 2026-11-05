package main

import (
	_ "embed"
	"github.com/oneo-me/nsgo/icons/lucide"
	"math"
	"strconv"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// The gallery's shell: its layouts, demo pages and headers, and helpers
// its demos share. Its look and components are package ui's.

// The layouts' sizes.
const (
	// sidebarWidth is the width the desktop layout's sidebar starts at.
	sidebarWidth core.Points = 220
	// desktopWidth is the narrowest window that takes the desktop layout
	// on desktops; tabletWidth on other devices (an iPad in landscape).
	desktopWidth core.Points = 700
	tabletWidth  core.Points = 900
)

// Shared styles.
var (
	row  = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}
	fill = core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}
)

// columnsFor is how many columns at least min pt wide fit in width, with
// gap between them and 16 pt gutters: 1 to max.
func columnsFor(min core.Points, max int, width core.Points, gap core.Points) int {
	n := int(math.Floor(float64((width - 32 + gap) / (min + gap))))
	return clampInt(n, 1, max)
}

func clampInt(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

func clamp(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

// themeToggle switches the app between light and dark, whatever the
// system shows: an icon button, as a toolbar's.
func themeToggle(cx *core.Scope) core.Node {
	name, label := toggleFace(cx)
	return ui.IconButton{IconFunc: name, LabelFunc: label, Size: 30, OnPress: func() { flipAppearance(cx) }}
}

// barThemeToggle is the theme toggle in the system's bar (iOS, Android),
// where the system draws the button (iOS's Liquid Glass): the item is the
// icon alone.
func barThemeToggle(cx *core.Scope) core.Node {
	name, label := toggleFace(cx)
	return ui.Press{OnPress: func() { flipAppearance(cx) }, Scale: 0.92, LabelFunc: label, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(34), Height: core.Pt(34), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{ui.IconOf(name, 18, ui.ColorsOf(cx).Text)}},
	}}
}

// toggleFace is a theme toggle's icon and label: the appearance it
// switches to.
func toggleFace(cx *core.Scope) (name func() *core.SVG, label func() string) {
	tr := locales.For(cx)
	name = func() *core.SVG {
		if cx.Dark() {
			return lucide.Sun()
		}
		return lucide.Moon()
	}
	label = func() string {
		if cx.Dark() {
			return tr.UiLightAppearance()
		}
		return tr.UiDarkAppearance()
	}
	return name, label
}

// flipAppearance forces the appearance other than the one shown.
func flipAppearance(cx *core.Scope) {
	next := core.AppearanceDark
	if cx.Dark() {
		next = core.AppearanceLight
	}
	cx.SetAppearance(next)
}

//go:embed assets/logo-dark.svg
var logoDark []byte

//go:embed assets/logo-light.svg
var logoLight []byte

// logo is the gallery's mark in the appearance shown (deeper oranges on
// light backgrounds).
func logo(cx *core.Scope, size core.Points) core.Node {
	return core.Image{SourceFunc: func() core.ImageSource {
		data := logoLight
		if cx.Dark() {
			data = logoDark
		}
		return core.ImageSource{Data: data, Size: core.Size{Width: 100, Height: 100}}
	}, Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)}}
}

// card builders take the demo's scope, so a demo page lists its cards as
// functions built when shown.
type cardFunc func(cx *core.Scope) core.Node

// pageProps are a demo page's options.
type pageProps struct {
	// columns is the most columns on a desktop (default 2).
	columns int
	// bottom is the space under the last card on phones (default 32).
	bottom core.Points
	// still keeps the cards from fading in.
	still    bool
	behavior core.ScrollBehavior
	// behaviorFunc, when set, replaces behavior as it changes.
	behaviorFunc func() core.ScrollBehavior
	// after follows the cards inside the scroll view (a demo's modals).
	after []core.Node
	ref   *core.Ref
}

// demoPage is a demo's page of cards, each fading in after the one
// before. On phones one column on 12 pt gutters; in the desktop layout the
// cards are dealt left to right into as many columns as fit the pane (at
// least 320 pt each), on 16 pt gutters.
func demoPage(cx *core.Scope, p pageProps, cards ...cardFunc) core.Node {
	colors := ui.ColorsOf(cx)
	build := func(i int, c cardFunc) core.Node {
		node := core.Func(func(cx *core.Scope) core.Node { return c(cx) })
		if p.still {
			return node
		}
		return Appear{Index: i, Children: []core.Node{node}}
	}
	bottom := p.bottom
	if bottom == 0 {
		bottom = 32
	}
	most := p.columns
	if most == 0 {
		most = 2
	}
	var body core.Node
	if !shell.desktop() {
		var nodes []core.Node
		for i, c := range cards {
			nodes = append(nodes, build(i, c))
		}
		body = core.View{Style: core.Style{Padding: core.Edges{Top: 12, Right: 12, Bottom: bottom, Left: 12}, Gap: 20}, Children: nodes}
	} else {
		// The cards are built once and move between the columns, which
		// are built again when their number changes.
		held := make([]*core.Holder, len(cards))
		for i, c := range cards {
			held[i] = core.NewHolder(cx, func(*core.Scope) core.Node { return build(i, c) })
		}
		columns := core.NewMemo(cx, func() int { return columnsFor(320, most, shell.paneWidth(), 12) })
		body = core.Dynamic(func(cx *core.Scope) core.Node {
			n := columns.Get()
			return core.Func(func(cx *core.Scope) core.Node {
				var cols []core.Node
				for column := range n {
					var nodes []core.Node
					for i, h := range held {
						if i%n == column {
							nodes = append(nodes, h.Place())
						}
					}
					cols = append(cols, core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), Gap: 20}, Children: nodes})
				}
				return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignStart, Gap: 12,
					Padding: core.Edges{Top: 16, Right: 16, Bottom: 32, Left: 16}, Width: core.Pct(100),
					MaxWidth: core.Pt(core.Points(n)*560 + 32), AlignSelf: core.AlignCenter}, Children: cols}
			})
		})
	}
	return core.Scroll{Ref: p.ref, TestID: "demo-scroll", Behavior: p.behavior, BehaviorFunc: p.behaviorFunc, Style: core.Style{FlexGrow: 1, FlexShrink: 1},
		StyleFunc: func() core.Style { return core.Style{Background: colors.Background()} },
		Children:  append([]core.Node{body}, p.after...)}
}

// pageHeaderProps are a page header's options.
type pageHeaderProps struct {
	title string
	// root is the bottom page of its stack: no back button.
	root bool
	// onBack is what back does (default: pop the page).
	onBack func()
	// left replaces the back button, center the title.
	left, center, right core.Node
	// background is the bar's fill (default the page's background).
	background func() core.Color
	// tint is the back button's color (default the accent).
	tint func() core.Color
	// transparent lays it over the content: no fill, no hairline.
	transparent bool
}

// pageHeader is a page's header where the app draws it (desktops, the
// web): a back button while the page is above another, the title and
// right. Phones show their system header with the screen's title instead.
func pageHeader(cx *core.Scope, p pageHeaderProps) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tint := p.tint
	if tint == nil {
		tint = colors.Accent
	}
	back := func() bool { return p.left == nil && !p.root && shell.nav.Depth() > 0 }
	style := func() core.Style {
		s := core.Style{Height: core.Pt(44), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(0, 12)}
		switch {
		case p.transparent:
		case p.background != nil:
			s.Background = p.background()
		default:
			s.Background, s.Borders, s.BorderColors = colors.Background(), core.Edges{Bottom: 1}, core.EdgeColors{Bottom: colors.Line()}
		}
		return s
	}
	var children []core.Node
	if p.left != nil {
		children = append(children, p.left)
	}
	children = append(children, core.Show{When: back, Then: func(*core.Scope) core.Node {
		return core.View{
			OnPress: func(core.PressEvent) {
				if p.onBack != nil {
					p.onBack()
				} else {
					shell.nav.Pop(1)
				}
			},
			Role: core.RoleButton, Label: tr.UiBack(), HitSlop: core.All(6),
			Style: core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2,
				Padding: core.Edges{Left: 4, Right: 10}, CornerRadius: ui.RadiusControl},
			StyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}
			},
			Children: []core.Node{ui.Icon(lucide.ChevronLeft, 18, tint), ui.Text(tr.UiBack(), 13, ui.Medium, tint)},
		}
	}})
	center := p.center
	if center == nil {
		title := ui.Text(p.title, 15, ui.Semibold, colors.Text)
		title.MaxLines, title.Role, title.Style.FlexGrow, title.Style.FlexShrink = 1, core.RoleHeader, 1, 1
		center = title
	}
	children = append(children, core.View{StyleFunc: func() core.Style {
		var pad core.Points = 4
		if back() || p.left != nil {
			pad = 0
		}
		return core.Style{FlexGrow: 1, FlexShrink: 1, Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, pad)}
	}, Children: []core.Node{center}})
	if p.right != nil {
		children = append(children, p.right)
	}
	return core.View{StyleFunc: style, Children: children}
}

// headerStyle is the system header's colors on every page (iOS,
// Android), for a HeaderStyleFunc.
func headerStyle(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Background: colors.Background(), Color: colors.Text(), TintColor: colors.Accent()}
	}
}

// pageStyle is a page's background, for a StyleFunc.
func pageStyle(cx *core.Scope) func() core.Style {
	return func() core.Style { return core.Style{Background: ui.ColorsOf(cx).Background()} }
}

// screen is a page a demo pushes: the system header on phones, the app's
// own elsewhere.
func screen(cx *core.Scope, title string, right core.Node, children ...core.Node) base.NavPage {
	return base.NavPage{Title: title, HeaderStyleFunc: headerStyle(cx), HeaderRight: right,
		Header:    pageHeader(cx, pageHeaderProps{title: title, right: right}),
		StyleFunc: pageStyle(cx),
		Children:  children}
}

// itoa formats an integer.
func itoa(n int) string { return strconv.Itoa(n) }

// every calls fn every d until cx ends or stop is called. It starts
// under cx, as later does, so a press handler that remakes its own button
// (Send becoming Pause) does not end it.
func every(cx *core.Scope, d time.Duration, fn func()) (stop func()) {
	stopped := false
	var cancel func() bool
	var tick func()
	tick = func() {
		if stopped {
			return
		}
		fn()
		if !stopped {
			cancel = cx.AfterFunc(d, tick)
		}
	}
	stop = func() {
		stopped = true
		if cancel != nil {
			cancel()
		}
	}
	cx.Post(func() {
		if !stopped {
			cx.Cleanup(stop)
			cancel = cx.AfterFunc(d, tick)
		}
	})
	return stop
}

// toggled flips a boolean signal.
func toggled(s *core.Signal[bool]) { s.Update(func(v bool) bool { return !v }) }

// thousands formats n with commas, as en-US does.
func thousands(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		return "-" + thousands(-n)
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// transparent is a color that draws nothing; a Color of 0 means unset.
var transparent = core.RGBA(0xffffff, 0)

// fillPage fills what is left of its parent along both axes and shrinks
// to it: a page whose scroll views scroll within it instead of growing it
// past the window (React Native's flex items do not shrink by default).
var fillPage = core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0)}

// later runs fn after d under cx itself. Scope.AfterFunc belongs to the
// current owner, which in a press handler is the pressed view's: a timer
// started there ends when that view goes, as the chat's empty state does
// once a reply starts. A Post runs under cx, so the timer lives as long
// as it does.
func later(cx *core.Scope, d time.Duration, fn func()) {
	cx.Post(func() { cx.AfterFunc(d, fn) })
}
