package ui

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
)

// The sidebar shell is Onexa's: the sidebar reaches the window's top
// beside the window controls, its toggle after them, and leaves a window
// material showing through; the content has a header of its own and
// slides over the sidebar as it hides. The app hides the system's title
// bar for HeaderHeight and picks the material (nsgo.WindowMaterial).

// HeaderHeight is the height of the shell's headers, which the app gives
// desktop.SetTitleBar for the window controls to center in.
const HeaderHeight core.Points = 40

// headerHeight is HeaderHeight, or the window controls' height where it
// is greater.
func headerHeight(cx *core.Scope) core.Points { return max(HeaderHeight, desktop.TitleBar(cx).Top) }

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
			Height: core.Pt(headerHeight(cx) + safe.Top), Padding: p, Background: fill()}
	}}
}

// sidebarSlide is how long the sidebar takes to show or hide.
const sidebarSlide = 220 * time.Millisecond

// fillRest grows to fill its container.
var fillRest = core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)}

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
				Height: core.Pt(headerHeight(cx)), Direction: core.Row, AlignItems: core.AlignCenter, ZIndex: 2}
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

	// The handle hugs the sidebar's edge; a double click restores the
	// default width.
	var start core.Points
	handle := core.View{TestID: "sidebar-resize", Disabled: closed, OnPress: func(e core.PressEvent) {
		if e.Clicks == 2 {
			resize(def)
		}
	}, StyleFunc: func() core.Style {
		st := core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(9), ZIndex: 1}
		if open() {
			st.Left = core.Pt(width() - 5)
		} else {
			st.Left = core.Pt(-5)
		}
		if !shown.Get() {
			st.Visibility = core.VisibilityHidden
		}
		return st
	}, Children: []core.Node{base.ResizeHandle{Label: s.ResizeLabel, Edge: base.HandleLeading, Disabled: closed,
		Value: func() core.AccessibilityValue {
			return core.AccessibilityValue{Now: float32(chosen()), Min: float32(lo), Max: float32(min(hi, env.Window().Width))}
		},
		OnDragStart: func() { start = chosen() },
		OnDragMove:  func(d core.Points) { resize(min(start+d, env.Window().Width-12)) },
		OnDragEnd:   func(d core.Points) { resize(min(start+d, env.Window().Width-12)) },
		Style:       core.Style{Margin: core.Edges{Left: 4}}, StyleFunc: HandleLine(cx),
		ThumbStyle: HandleThumb(), ThumbStyleFunc: HandleThumbColor(cx)}}}

	children := []core.Node{core.View{Style: fillRest.Merge(core.Style{Direction: core.Row}), Children: []core.Node{slot, content}}, handle}
	if toggle != nil {
		children = append(children, toggle)
	}
	return core.View{TestID: s.TestID, Style: fillRest, StyleFunc: func() core.Style {
		return core.Style{Background: Chrome(cx), Color: colors.Text()}
	}, Children: children}
}

// SidebarItem is a destination of a Sidebar.
type SidebarItem struct {
	// ID is what Selected and OnSelect name the item by.
	ID     string
	Text   string
	Icon   IconName
	TestID string
}

// SidebarSection is a group of a Sidebar's items under an optional
// title.
type SidebarSection struct {
	Title  string
	Items  []SidebarItem
	TestID string
}

// Sidebar is grouped navigation, as Onexa's: the sections' items, the
// Selected one on a faint fill of the text's color, icons in the accent.
// It scrolls and leaves its background to the shell.
type Sidebar struct {
	Sections []SidebarSection
	Selected func() string
	OnSelect func(id string)
	TestID   string
}

// Render implements core.Node.
func (s Sidebar) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	desk := core.Untracked(cx, cx.Env().Desktop)
	height := core.Points(30)
	if !desk {
		height = 40
	}
	row := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(height),
		Padding: core.Symmetric(0, 8), CornerRadius: 8, Cursor: core.CursorPointer}
	selectedFill := func() core.Color { return Alpha(colors.Text(), 0.1) }
	states := func() core.Style {
		s := core.Style{Selected: &core.Style{Background: selectedFill()}, Focus: FocusRing(cx, -2)}
		if !desk {
			s.Active = &core.Style{Background: selectedFill()}
		}
		return s
	}
	var sections []core.Node
	for _, sec := range s.Sections {
		var nodes []core.Node
		if sec.Title != "" {
			title := Text(sec.Title, 11, Semibold, colors.Muted)
			title.Role, title.MaxLines = core.RoleHeader, 1
			title.Style.Padding = core.Edges{Top: 4, Right: 8, Bottom: 2, Left: 8}
			nodes = append(nodes, title)
		}
		for _, item := range sec.Items {
			label := Text(item.Text, 13, Regular, colors.Text)
			label.MaxLines, label.Style.FlexGrow, label.Style.FlexShrink = 1, 1, 1
			nodes = append(nodes, base.Button{Selected: func() bool { return s.Selected != nil && s.Selected() == item.ID },
				OnPress: func(core.PressEvent) {
					if s.OnSelect != nil {
						s.OnSelect(item.ID)
					}
				},
				Style: row, StyleFunc: states, Label: item.Text, TestID: item.TestID,
				Children: []core.Node{Icon(item.Icon, 16, colors.Accent), label}})
		}
		sections = append(sections, core.View{TestID: sec.TestID, Style: core.Style{Gap: 1}, Children: nodes})
	}
	return core.Scroll{TestID: s.TestID, Style: fillRest, Children: []core.Node{
		core.View{Style: core.Style{Padding: core.Edges{Right: 8, Bottom: 8, Left: 8}, Gap: 12}, Children: sections}}}
}

// IconButton is a square icon button with no fill at rest, Size points
// across (28 when 0), as a toolbar's: a fill under the pointer, held
// down and while Selected. IconFunc and LabelFunc replace Icon and Label,
// what assistive technology reads.
type IconButton struct {
	Icon      IconName
	IconFunc  func() IconName
	Label     string
	LabelFunc func() string
	Size      core.Points
	Selected  func() bool
	OnPress   func()
	TestID    string
}

// Render implements core.Node.
func (b IconButton) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	size := b.Size
	if size == 0 {
		size = 28
	}
	name := b.IconFunc
	if name == nil {
		name = func() IconName { return b.Icon }
	}
	return core.View{OnPress: func(core.PressEvent) { b.OnPress() }, Role: core.RoleButton, Label: b.Label, LabelFunc: b.LabelFunc,
		Selected: b.Selected, TestID: b.TestID, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff},
		Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: 8, Cursor: core.CursorPointer},
		StyleFunc: func() core.Style {
			return core.Style{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}, Focus: FocusRing(cx, -2)}
		},
		Children: []core.Node{IconOf(name, size*0.6, colors.Muted)}}
}
