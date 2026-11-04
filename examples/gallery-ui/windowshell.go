package ui

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
)

// The window shells fill the window with the app's content under a
// header of their own, which drags the window: the sidebar reaches the
// window's top beside the window controls, keeping clear of them, and
// leaves a window material showing through. The app hides the system's
// title bar for HeaderHeight with desktop.SetTitleBar and picks the
// material (nsgo.WindowMaterial).

// HeaderHeight is the height of the shell's headers, which the app gives
// desktop.SetTitleBar for the window controls to center in.
const HeaderHeight core.Points = 40

// ShellHeaderHeight is a shell's header's height: HeaderHeight, or the
// window controls' height where it is greater.
func ShellHeaderHeight(cx *core.Scope) core.Points {
	return max(HeaderHeight, desktop.TitleBar(cx).Top)
}

// Chrome is the background of the window's chrome (a title bar, the
// sidebar): none while a window material shows through it, read in a
// StyleFunc.
func Chrome(cx *core.Scope) core.Color {
	if desktop.ShowsMaterial(cx) {
		return 0
	}
	return ColorsOf(cx).Inset()
}

// controlsEnd is where the window controls and the safe area end at the
// window's start.
func controlsEnd(cx *core.Scope) core.Points { return desktop.TitleBar(cx).Left + core.Insets(cx).Left }

// shellHeader is a header row, whose background drags the window. It
// keeps clear of the safe area, at its start of lead points more, and at
// its end of the window controls while trail says so.
func shellHeader(cx *core.Scope, testID string, lead func() core.Points, trail bool, fill func() core.Color, children []core.Node) core.Node {
	return core.View{TestID: testID, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, Children: children, StyleFunc: func() core.Style {
		safe := core.Insets(cx)
		p := core.Edges{Top: safe.Top, Left: 12 + lead(), Right: 12}
		if trail {
			p.Right += desktop.TitleBar(cx).Right + safe.Right
		}
		return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexShrink: 0,
			Height: core.Pt(ShellHeaderHeight(cx) + safe.Top), Padding: p, Background: fill()}
	}}
}

// sidebarSlide is how long the sidebar takes to show or hide.
const sidebarSlide = 220 * time.Millisecond

// SidebarShell is a window of a sidebar beside the content, each under a
// header: the sidebar's toggle stays after the window controls, the
// app's SidebarActions end the sidebar's header, and its Header and
// Actions are the content's. Hiding the sidebar slides the content over
// it, the sidebar keeping its width; its edge is a handle the user drags
// to resize it and double-clicks to restore DefaultWidth. SidebarFooter
// is a bar at the sidebar's bottom, with no line above it.
//
// Open and Width are the app's, set from OnOpenChange and OnWidthChange.
// The sidebar and its header are the chrome (Chrome), the content and
// its header on the page's background.
type SidebarShell struct {
	Sidebar        []core.Node
	SidebarActions []core.Node
	SidebarFooter  []core.Node
	Header         []core.Node
	Actions        []core.Node
	Content        []core.Node
	Open           func() bool
	OnOpenChange   func(open bool)
	// ToggleLabel names the toggle, which shows while OnOpenChange is set.
	ToggleLabel string
	// Width is the sidebar's width, DefaultWidth (240 points when 0)
	// where it is nil or 0. The user drags it between MinWidth (160 when
	// 0) and MaxWidth (none when 0) on the handle named ResizeLabel.
	Width              func() core.Points
	OnWidthChange      func(width core.Points)
	DefaultWidth       core.Points
	MinWidth, MaxWidth core.Points
	ResizeLabel        string
	TestID             string
}

// Render implements core.Node.
func (s SidebarShell) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	env := cx.Env()
	open := s.Open
	if open == nil {
		open = func() bool { return true }
	}
	closed := func() bool { return !open() }
	def, lo, hi := s.DefaultWidth, s.MinWidth, s.MaxWidth
	if lo == 0 {
		lo = 160
	}
	if hi == 0 {
		hi = core.Points(math.MaxFloat32)
	}
	hi = max(hi, lo)
	if def == 0 {
		def = 240
	}
	def = min(max(def, lo), hi)
	own := core.NewSignal(cx, def)
	chosen := func() core.Points {
		if s.Width == nil {
			return own.Get()
		}
		if w := s.Width(); w > 0 {
			return min(max(w, lo), hi)
		}
		return def
	}
	// width is what shows: the handle stays within the window.
	width := func() core.Points { return max(min(chosen(), env.Window().Width-12), 0) }
	resize := func(w core.Points) {
		w = min(max(w, lo), hi)
		if s.Width == nil {
			own.Set(w)
		}
		if s.OnWidthChange != nil {
			s.OnWidthChange(w)
		}
	}

	// shown is whether the sidebar is seen: it hides once the content has
	// slid over it, out of the pointer's and screen readers' way.
	shown := core.NewSignal(cx, core.Untracked(cx, open))
	first := true
	var stop func() bool
	cx.Effect(func() {
		o := open()
		if first {
			first = false
			return
		}
		cx.Untrack(func() {
			if !env.ReduceMotion() {
				cx.AnimateLayout(core.LayoutAnimation{Update: &core.LayoutTransition{Curve: core.LayoutEaseInEaseOut, Duration: sidebarSlide}})
			}
			if stop != nil {
				stop()
			}
			if o {
				shown.Set(true)
				return
			}
			stop = cx.AfterFunc(sidebarSlide, func() { shown.Set(open()) })
		})
	})

	// The toggle follows the window controls, which keep a margin of
	// their own, or the window's edge at a margin; the headers keep clear
	// of it.
	toggleStart := func() core.Points {
		if desktop.TitleBar(cx).Left > 0 {
			return controlsEnd(cx)
		}
		return controlsEnd(cx) + 12
	}
	const toggleSize = 28
	reserve := func() core.Points { return controlsEnd(cx) }
	var toggle core.Node
	if s.OnOpenChange != nil {
		toggle = core.View{StyleFunc: func() core.Style {
			return core.Style{Position: core.Absolute, Top: core.Pt(core.Insets(cx).Top), Left: core.Pt(toggleStart()),
				Height: core.Pt(ShellHeaderHeight(cx)), Direction: core.Row, AlignItems: core.AlignCenter, ZIndex: 2}
		}, Children: []core.Node{IconButton{Icon: IconSidebar, Label: s.ToggleLabel, Size: toggleSize, Selected: open,
			OnPress: func() { s.OnOpenChange(!open()) }}}}
		reserve = func() core.Points { return toggleStart() - 12 + toggleSize + 8 }
	}

	sidebar := []core.Node{
		shellHeader(cx, "sidebar-header", reserve, false, func() core.Color { return 0 },
			append([]core.Node{Spacer{}}, s.SidebarActions...)),
		core.View{Style: fillRest, Children: s.Sidebar},
	}
	if s.SidebarFooter != nil {
		sidebar = append(sidebar, core.View{TestID: "sidebar-footer", Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4,
			FlexShrink: 0, Padding: core.All(8)}, Children: s.SidebarFooter})
	}
	// The slot narrows over the sidebar, which keeps its width.
	slot := core.View{TestID: "sidebar-slot", StyleFunc: func() core.Style {
		var w core.Points
		if open() {
			w = width()
		}
		return core.Style{Width: core.Pt(w), FlexShrink: 0, Overflow: core.Hidden}
	}, Children: []core.Node{core.View{TestID: "sidebar-panel", Disabled: closed, Children: sidebar, StyleFunc: func() core.Style {
		st := core.Style{Width: core.Pt(width()), Height: core.Pct(100), MinHeight: core.Pt(0)}
		if !shown.Get() {
			st.Visibility = core.VisibilityHidden
		}
		return st
	}}}}

	head := []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
		FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pct(100), Overflow: core.Hidden}, Children: s.Header}}
	content := core.View{Style: fillRest, StyleFunc: Filled(colors.Background), Children: []core.Node{
		shellHeader(cx, "content-header", func() core.Points {
			if open() {
				return 0
			}
			return reserve()
		}, true, func() core.Color { return colors.Background() }, append(head, s.Actions...)),
		core.View{Style: fillRest, Children: s.Content},
	}}

	// The handle's band, 3 points across, centers its line on the
	// sidebar's last point; a double click restores the default width.
	var start core.Points
	handle := core.View{TestID: "sidebar-resize", Disabled: closed, StyleFunc: func() core.Style {
		st := core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(3), ZIndex: 1}
		if open() {
			st.Left = core.Pt(width() - 2)
		} else {
			st.Left = core.Pt(-2)
		}
		if !shown.Get() {
			st.Visibility = core.VisibilityHidden
		}
		return st
	}, Children: []core.Node{base.ResizeHandle{Label: s.ResizeLabel, Edge: base.HandleLeading, Disabled: closed,
		Value: func() core.AccessibilityValue {
			return core.AccessibilityValue{Now: float32(chosen()), Min: float32(lo), Max: float32(min(hi, env.Window().Width))}
		},
		OnDragStart:   func() { start = chosen() },
		OnDragMove:    func(d core.Points) { resize(min(start+d, env.Window().Width-12)) },
		OnDragEnd:     func(d core.Points) { resize(min(start+d, env.Window().Width-12)) },
		OnDoublePress: func() { resize(def) },
		Style:         core.Style{Margin: core.Edges{Left: 1}}, StyleFunc: HandleLine(cx),
		ThumbStyle: HandleThumb(cx), ThumbStyleFunc: HandleThumbColor(cx)}}}

	children := []core.Node{core.View{Style: fillRest.Merge(core.Style{Direction: core.Row}), Children: []core.Node{slot, content}}, handle}
	if toggle != nil {
		children = append(children, toggle)
	}
	return core.View{TestID: s.TestID, Style: fillRest, StyleFunc: func() core.Style {
		return core.Style{Background: Chrome(cx), Color: colors.Text()}
	}, Children: children}
}

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
	colors := ColorsOf(cx)
	lead := p.Leading
	if lead == nil && p.Title != "" {
		lead = []core.Node{core.Text{Content: p.Title, MaxLines: 1, Role: core.RoleHeader,
			Style: typeLabel.Merge(core.Style{FlexShrink: 1}), StyleFunc: Colored(colors.Text)}}
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
	body := core.View{Style: fillRest, StyleFunc: Filled(colors.Background), Children: p.Content}
	children := []core.Node{bar, body}
	if p.BehindHeader {
		bar = core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0)},
			Children: []core.Node{bar}}
		children = []core.Node{body, bar}
	}
	return core.View{TestID: p.TestID, Style: fillRest, StyleFunc: func() core.Style {
		return core.Style{Background: page(), Color: colors.Text()}
	}, Children: children}
}
