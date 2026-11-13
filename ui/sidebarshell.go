package ui

import (
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// sidebarSlide is how long the sidebar takes to show or hide.
const sidebarSlide = 220 * time.Millisecond

// SidebarShellVariant is how a SidebarShell shows its content.
type SidebarShellVariant uint8

const (
	// SidebarShellPlain puts the content on the page's background from
	// the sidebar to the window's edges. It is the default.
	SidebarShellPlain SidebarShellVariant = iota
	// SidebarShellInset floats the content as a rounded card on the page's
	// background at 90%, the window keeping a margin of 5 points around
	// the sidebar and the card, and between them, while the sidebar
	// shows; the card's edge and shadow lift it off the chrome there, in
	// place of the sidebar's line. The margin, the edge and the shadow go
	// as the sidebar hides, the window controls and the toggle moving with
	// it as it slides.
	SidebarShellInset
)

// The margin of SidebarShellInset and its card's corners.
const (
	shellInset       core.Points = 5
	shellInsetRadius core.Points = 10
)

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
// its header on the page's background, and the right panel on that
// background at 80%. Variant chooses how the content shows on the chrome,
// and ChromeStyle, when set, paints the chrome in place of Chrome, such
// as with a gradient; it is a StyleFunc. Backdrop is drawn over the
// chrome, filling the window behind the sidebar and the content, such as
// a texture; it takes no presses.
type SidebarShell struct {
	Variant        SidebarShellVariant
	ChromeStyle    func() core.Style
	Backdrop       []core.Node
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
	// Right is a panel at the content's end under RightHeader, and
	// RightOpen whether it shows, with a toggle ending the content's
	// header, named RightToggleLabel, while OnRightOpenChange is set. The
	// content slides over it as over the sidebar, and draws its line, a
	// handle of its own named RightResizeLabel, on its edge; RightWidth,
	// RightDefaultWidth (280 when 0), RightMinWidth (200 when 0) and
	// RightMaxWidth (none when 0) work as the sidebar's do.
	Right, RightHeader                 []core.Node
	RightOpen                          func() bool
	OnRightOpenChange                  func(open bool)
	RightToggleLabel, RightResizeLabel string
	RightWidth                         func() core.Points
	OnRightWidthChange                 func(width core.Points)
	RightDefaultWidth                  core.Points
	RightMinWidth, RightMaxWidth       core.Points
	TestID                             string
}

// Render implements core.Node.
func (s SidebarShell) Render(cx *core.Scope) core.Node {
	hideTitleBar(cx)
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

	shown := slideShown(cx, open)
	inset := s.Variant == SidebarShellInset
	// margin is the inset variant's around the sidebar and the content,
	// which slides as the sidebar does; shift follows it a frame at a time
	// for the window controls and the toggle, which the slide leaves.
	margin := func() core.Points {
		if inset && open() {
			return shellInset
		}
		return 0
	}
	shift := core.NewSignal(cx, core.Untracked(cx, margin))
	if inset {
		insetControls(cx, margin, shift)
	}

	// The toggle follows the window controls, which keep a margin of
	// their own, or the window's edge at a margin; the headers keep clear
	// of it.
	toggleStart := func() core.Points {
		if desktop.TitleBar(cx).Left > 0 {
			return controlsEnd(cx)
		}
		return controlsEnd(cx) + 12 + shift.Get()
	}
	const toggleSize, toggleIcon = 32, 16.25
	reserve := func() core.Points { return controlsEnd(cx) }
	var toggle core.Node
	if s.OnOpenChange != nil {
		toggle = core.View{StyleFunc: func() core.Style {
			return core.Style{Position: core.Absolute, Top: core.Pt(core.Insets(cx).Top + shift.Get()), Left: core.Pt(toggleStart()),
				Height: core.Pt(ShellHeaderHeight(cx)), Direction: core.Row, AlignItems: core.AlignCenter, ZIndex: 2}
		}, Children: []core.Node{IconButton{Icon: lucide.PanelLeft, Label: s.ToggleLabel, Size: toggleSize, IconSize: toggleIcon,
			OnPress: func() { s.OnOpenChange(!open()) }}}}
		reserve = func() core.Points { return toggleStart() - 12 + toggleSize + 8 }
	}

	sidebar := []core.Node{
		shellHeader(cx, "sidebar-header", func() core.Points { return reserve() - margin() }, false, func() core.Color { return 0 },
			append([]core.Node{Spacer{}}, s.SidebarActions...)),
		core.View{Style: FillRest, Children: s.Sidebar},
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

	var right, rightHandle core.Node
	if s.Right != nil {
		right, rightHandle = s.rightPanel(cx)
	}
	head := []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
		FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pct(100), Overflow: core.Hidden}, Children: s.Header}}
	headerFill := colors.Background
	if inset {
		headerFill = func() core.Color { return 0 }
	}
	var content core.Node = core.View{TestID: "content", Style: FillRest.Merge(core.Style{Overflow: core.Hidden}), StyleFunc: Filled(colors.Background), Children: []core.Node{
		shellHeader(cx, "content-header", func() core.Points {
			if open() {
				return 0
			}
			return reserve()
		}, true, headerFill, append(append(head, s.Actions...), s.rightToggle()...)),
		core.View{Style: FillRest, Children: s.Content},
		rightHandle,
	}}
	if inset {
		content = insetCard(cx, content.(core.View), open, margin)
	}

	// The handle's band, 3 points across, centers its line on the
	// sidebar's last point; a double click restores the default width.
	// The inset card's edge is the line there.
	line := HandleLine(cx)
	if inset {
		line = func() core.Style { return core.Style{} }
	}
	var start core.Points
	handle := core.View{TestID: "sidebar-resize", Disabled: closed, StyleFunc: func() core.Style {
		st := core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(3), ZIndex: 1}
		if open() {
			// The band centers in the gap before the inset card.
			st.Left = core.Pt(margin() + width() + margin()/2 - 2)
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
		Style:         core.Style{Margin: core.Edges{Left: 1}}, StyleFunc: line,
		ThumbStyle: HandleThumb(cx), ThumbStyleFunc: HandleThumbColor(cx)}}}

	row := []core.Node{slot, content}
	if right != nil {
		row = append(row, right)
	}
	var children []core.Node
	if s.Backdrop != nil {
		children = append(children, core.View{TestID: "backdrop", Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0),
			Right: core.Pt(0), Bottom: core.Pt(0), PointerEvents: core.PointerEventsNone}, Children: s.Backdrop})
	}
	children = append(children, core.View{Style: FillRest.Merge(core.Style{Direction: core.Row}), StyleFunc: func() core.Style {
		return core.Style{Margin: core.All(margin())}
	}, Children: row}, handle)
	if toggle != nil {
		children = append(children, toggle)
	}
	return core.View{TestID: s.TestID, Style: FillRest, StyleFunc: func() core.Style {
		if s.ChromeStyle != nil {
			return s.ChromeStyle().Merge(core.Style{Color: colors.Text()})
		}
		return core.Style{Background: Chrome(cx), Color: colors.Text()}
	}, Children: children}
}

// insetCard is SidebarShellInset's card around the content, which clips
// it: apart from the sidebar by margin, with an edge and a shadow while
// the sidebar is open. The shadow is the card's and the clip the
// content's, so it falls outside.
func insetCard(cx *core.Scope, content core.View, open func() bool, margin func() core.Points) core.Node {
	colors := ColorsOf(cx)
	content.TestID = ""
	content.Style.CornerRadius = shellInsetRadius - 1
	content.StyleFunc = nil
	return core.View{TestID: "content", Style: FillRest, Children: []core.Node{content}, StyleFunc: func() core.Style {
		st := core.Style{Background: Alpha(colors.Background(), 0.9), CornerRadius: shellInsetRadius, BorderWidth: 1,
			BorderColor: Alpha(colors.Border(), 0), Margin: core.Edges{Left: margin()}}
		if open() {
			st.BorderColor = colors.Line()
			st.Shadows = []core.BoxShadow{
				{Offset: core.Point{Y: 1}, Blur: 2, Color: Black(0.05)},
				{Offset: core.Point{Y: 4}, Blur: 16, Color: Black(0.07)},
			}
		}
		return st
	}}
}

// slideShown is whether a panel that open shows is seen: it hides once the
// content has slid over it, out of the pointer's and screen readers' way.
// Only a change of open slides: open may follow more of the app's state,
// whose other changes would otherwise animate whatever moves with them.
func slideShown(cx *core.Scope, open func() bool) *core.Signal[bool] {
	env := cx.Env()
	was := core.Untracked(cx, open)
	shown := core.NewSignal(cx, was)
	var stop func() bool
	cx.Effect(func() {
		o := open()
		if o == was {
			return
		}
		was = o
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
	return shown
}

// insetControls moves shift to margin a frame at a time as the sidebar
// slides, and the window controls with it: margin changes in the pass
// that animates the layout, which the controls are no part of.
func insetControls(cx *core.Scope, margin func() core.Points, shift *core.Signal[core.Points]) {
	env := cx.Env()
	const frames = 13 // sidebarSlide at 60 frames a second
	var stop func() bool
	cx.Effect(func() {
		to := margin()
		cx.Untrack(func() {
			if stop != nil {
				stop()
				stop = nil
			}
			from := shift.Get()
			if from == to || env.ReduceMotion() {
				shift.Set(to)
				return
			}
			var step func(i int)
			step = func(i int) {
				t := float32(i) / frames
				// Ease in and out, as the layout slides.
				t = t * t * (3 - 2*t)
				shift.Set(from + (to-from)*core.Points(t))
				if i < frames {
					stop = cx.AfterFunc(sidebarSlide/frames, func() { step(i + 1) })
				}
			}
			step(1)
		})
	})
	if !desktop.IsAvailable(cx) {
		return
	}
	cx.Effect(func() {
		d := shift.Get()
		desktop.SetWindowControls(cx, desktop.WindowControls{Offset: core.Point{X: d, Y: d}})
	})
	cx.Cleanup(func() { desktop.SetWindowControls(cx, desktop.WindowControls{}) })
}

// rightToggle is the toggle of the right panel, when it has one.
func (s SidebarShell) rightToggle() []core.Node {
	if s.Right == nil || s.OnRightOpenChange == nil {
		return nil
	}
	return []core.Node{IconButton{Icon: lucide.PanelRight, Label: s.RightToggleLabel, Size: 32, IconSize: 16.25,
		OnPress: func() { s.OnRightOpenChange(!s.rightOpen()) }}}
}

func (s SidebarShell) rightOpen() bool { return s.RightOpen != nil && s.RightOpen() }

// rightPanel is the panel at the content's end, which stays against the
// window's edge as the content slides over it, and the handle on the
// content's edge, whose line moves with the content.
func (s SidebarShell) rightPanel(cx *core.Scope) (panel, handle core.Node) {
	colors := ColorsOf(cx)
	env := cx.Env()
	def, lo, hi := s.RightDefaultWidth, s.RightMinWidth, s.RightMaxWidth
	if lo == 0 {
		lo = 200
	}
	if hi == 0 {
		hi = core.Points(math.MaxFloat32)
	}
	hi = max(hi, lo)
	if def == 0 {
		def = 280
	}
	def = min(max(def, lo), hi)
	own := core.NewSignal(cx, def)
	chosen := func() core.Points {
		if s.RightWidth == nil {
			return own.Get()
		}
		if w := s.RightWidth(); w > 0 {
			return min(max(w, lo), hi)
		}
		return def
	}
	width := func() core.Points { return max(min(chosen(), env.Window().Width-12), 0) }
	resize := func(w core.Points) {
		w = min(max(w, lo), hi)
		if s.RightWidth == nil {
			own.Set(w)
		}
		if s.OnRightWidthChange != nil {
			s.OnRightWidthChange(w)
		}
	}
	shown := slideShown(cx, s.rightOpen)
	closed := func() bool { return !s.rightOpen() }
	var start core.Points
	panel = core.View{TestID: "right-slot", StyleFunc: func() core.Style {
		var w core.Points
		if s.rightOpen() {
			w = width()
		}
		return core.Style{Width: core.Pt(w), FlexShrink: 0, Height: core.Pct(100), AlignItems: core.AlignEnd, Overflow: core.Hidden}
	}, Children: []core.Node{core.Show{When: shown.Get, Then: func(cx *core.Scope) core.Node {
		head := []core.Node{shellHeader(cx, "right-header", func() core.Points { return 0 }, false, func() core.Color { return 0 }, s.RightHeader)}
		return core.View{TestID: "right-panel", Disabled: closed, StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(width()), FlexShrink: 0, Height: core.Pct(100), MinHeight: core.Pt(0), Background: Alpha(colors.Background(), 0.8), Color: colors.Text()}
		}, Children: append(head, core.View{Style: FillRest, Children: s.Right})}
	}}}}
	handle = core.View{TestID: "right-resize", Disabled: closed, StyleFunc: func() core.Style {
		st := core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Right: core.Pt(0), Width: core.Pt(3), ZIndex: 1}
		if !shown.Get() {
			st.Visibility = core.VisibilityHidden
		}
		return st
	}, Children: []core.Node{base.ResizeHandle{Label: s.RightResizeLabel, Edge: base.HandleTrailing, Disabled: closed,
		Value: func() core.AccessibilityValue {
			return core.AccessibilityValue{Now: float32(chosen()), Min: float32(lo), Max: float32(min(hi, env.Window().Width))}
		},
		OnDragStart:   func() { start = chosen() },
		OnDragMove:    func(d core.Points) { resize(min(start-d, env.Window().Width-12)) },
		OnDragEnd:     func(d core.Points) { resize(min(start-d, env.Window().Width-12)) },
		OnDoublePress: func() { resize(def) },
		StyleFunc:     HandleLine(cx), ThumbStyle: HandleThumb(cx), ThumbStyleFunc: HandleThumbColor(cx)}}}
	return panel, handle
}
