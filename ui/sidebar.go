package ui

import (
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// SidebarItem is a destination of a Sidebar.
type SidebarItem struct {
	// ID is what Selected and OnSelect name the item by.
	ID     string
	Text   string
	Icon   func() *core.SVG
	TestID string
}

// SidebarSection is a group of a Sidebar's items under an optional
// title.
type SidebarSection struct {
	Title string
	Items []SidebarItem
	// Actions end the title's row; Note follows the items, for a section
	// that has none yet.
	Actions core.Node
	Note    core.Node
	// Collapsible makes a titled section's title a button that folds its
	// items and Note away; Actions stay. Open makes that the app's, set
	// from OnOpenChange, with its own state otherwise, open unless
	// DefaultClosed.
	Collapsible   bool
	Open          func() bool
	OnOpenChange  func(open bool)
	DefaultClosed bool
	TestID        string
}

// Sidebar is grouped navigation: the sections' items, the Selected one on
// a faint fill of the text's color, icons in the accent.
// It scrolls and leaves its background to the shell.
//
// It is one stop of Tab, as a macOS source list is: while it has the
// keyboard focus, the selected item is ringed. While it has the focus, or
// after a press in it until the focus goes elsewhere, Up and Down select
// the item before and after, Home and End the first and the last, scrolled
// into view, passing over collapsed sections' items. A collapsible
// section's title is a stop of its own.
type Sidebar struct {
	Sections []SidebarSection
	Selected func() string
	OnSelect func(id string)
	TestID   string
}

// Render implements core.Node.
func (s Sidebar) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	const height core.Points = 32
	selected := func() string {
		if s.Selected == nil {
			return ""
		}
		return s.Selected()
	}
	choose := func(id string) {
		if s.OnSelect != nil {
			s.OnSelect(id)
		}
	}
	// focused is the keyboard focus on the sidebar; held counts the times
	// it took the keys, 0 while it holds none, so taking them again binds
	// them anew, above a list's.
	focused, held := core.NewSignal(cx, false), core.NewSignal(cx, 0)
	take := func() { held.Update(func(n int) int { return n + 1 }) }
	// The items in order, whether each shows, and where each is: what the
	// keys move through and scroll into view.
	var order []string
	shown := map[string]func() bool{}
	refs := map[string]*core.Ref{}
	scroll, content := &core.Ref{}, &core.Ref{}
	var offset core.Points
	reveal := func(id string) {
		r, ok := refs[id].Bounds()
		c, ok2 := content.Bounds()
		view, ok3 := scroll.Bounds()
		if !ok || !ok2 || !ok3 {
			return
		}
		top := r.Origin.Y - c.Origin.Y
		bottom := top + r.Size.Height
		switch {
		case top < offset:
			scroll.ScrollTo(core.Point{Y: top}, true)
		case bottom > offset+view.Size.Height:
			scroll.ScrollTo(core.Point{Y: bottom - view.Size.Height}, true)
		}
	}
	// walk selects the first item showing from at by step.
	walk := func(at, step int) {
		for i := at; i >= 0 && i < len(order); i += step {
			if core.Untracked(cx, shown[order[i]]) {
				choose(order[i])
				reveal(order[i])
				return
			}
		}
	}
	// move walks from the selected item, or the first when none is.
	move := func(step int) func() {
		return func() {
			at := slices.Index(order, core.Untracked(cx, selected))
			if at < 0 {
				walk(0, 1)
				return
			}
			walk(at+step, step)
		}
	}
	item := func(it SidebarItem, open func() bool) core.Node {
		ref := &core.Ref{}
		order = append(order, it.ID)
		shown[it.ID], refs[it.ID] = open, ref
		on := func() bool { return selected() == it.ID }
		label := Text(it.Text, 13, Regular, colors.Text)
		label.MaxLines, label.Style.FlexGrow, label.Style.FlexShrink = 1, 1, 1
		return core.View{Ref: ref, Role: core.RoleButton, Label: it.Text, Selected: on, NoFocus: true, TestID: it.TestID,
			OnPress: func(core.PressEvent) {
				take()
				choose(it.ID)
			},
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(height),
				Padding: core.Symmetric(0, 8), CornerRadius: 8, Cursor: core.CursorDefault},
			StyleFunc: func() core.Style {
				return core.Style{Selected: &core.Style{Background: Alpha(colors.Text(), 0.1)}, Focus: FocusRing(cx, -2)}
			},
			PartStates: func() core.PartStates { return core.PartStates{Focused: on() && focused.Get()} },
			Children:   []core.Node{Icon(it.Icon, 16, colors.Accent), label}}
	}
	var sections []core.Node
	for _, sec := range s.Sections {
		open, toggle := sec.opener(cx)
		var heading core.Node
		if sec.Title != "" {
			title := Text(sec.Title, 11, Bold, colors.Muted)
			title.Role, title.MaxLines = core.RoleHeader, 1
			title.Style.Padding = core.Edges{Top: 4, Right: 8, Bottom: 2, Left: 8}
			if sec.Actions != nil || sec.Collapsible {
				title.Style.FlexGrow, title.Style.FlexShrink = 1, 1
			}
			heading = title
			if sec.Collapsible {
				title.Role = core.RoleNone
				title.Style.Padding.Right = 8 + iconSize
				heading = base.Button{Label: sec.Title, Expanded: open, OnPress: func(core.PressEvent) { take(); toggle() },
					Style:     core.Style{Group: sidebarTitle, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), CornerRadius: 8, Cursor: core.CursorDefault},
					StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, -2)} },
					Children:  []core.Node{title, sidebarArrow(cx, open)}}
			}
			if sec.Actions != nil {
				heading = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Edges{Right: 4}}, Children: []core.Node{heading, sec.Actions}}
			}
		}
		var nodes []core.Node
		for _, it := range sec.Items {
			nodes = append(nodes, item(it, open))
		}
		if sec.Note != nil {
			nodes = append(nodes, core.View{Style: core.Style{Padding: core.Symmetric(4, 8)}, Children: []core.Node{sec.Note}})
		}
		if sec.Title != "" && sec.Collapsible {
			sections = append(sections, base.Collapsible{Open: open, Reveal: func() bool { return true }, TestID: sec.TestID, Children: []core.Node{heading},
				Content: core.View{Style: core.Style{Padding: core.Edges{Top: 4}, Gap: 4}, Children: nodes}})
			continue
		}
		if heading != nil {
			nodes = append([]core.Node{heading}, nodes...)
		}
		sections = append(sections, core.View{TestID: sec.TestID, Style: core.Style{Gap: 4}, Children: nodes})
	}
	keys := core.Show{When: func() bool { return held.Get() > 0 }, Then: func(cx *core.Scope) core.Node {
		repeat := core.KeyOptions{When: core.KeyNotEditing, Repeat: true}
		cx.OnKeyWith("up", repeat, move(-1))
		cx.OnKeyWith("down", repeat, move(1))
		cx.OnKeyWith("home", repeat, func() { walk(0, 1) })
		cx.OnKeyWith("end", repeat, func() { walk(len(order)-1, -1) })
		return nil
	}}
	return core.Scroll{Ref: scroll, TestID: s.TestID, Style: FillRest, OnScroll: func(e core.ScrollEvent) { offset = e.Offset.Y },
		Children: []core.Node{core.View{Ref: content, OnPress: func(core.PressEvent) { take() },
			OnFocus: func() { focused.Set(true); take() }, OnBlur: func() { focused.Set(false); held.Set(0) },
			Style: core.Style{Padding: core.Edges{Right: 12, Bottom: 12, Left: 12}, Gap: 12}, Children: append(sections, keys)}}}
}

// sidebarTitle is the group of a collapsible section's title.
const sidebarTitle = "sidebar-title"

// sidebarArrow is a collapsible title's arrow, over the title's end so it
// takes no room, showing while the pointer or the focus is on the title
// and always on touch screens, which have no pointer to bring it.
func sidebarArrow(cx *core.Scope, open func() bool) core.Node {
	// The title's padding sits it a point low.
	s := core.Style{Position: core.Absolute, Top: core.Pt(2), Bottom: core.Pt(0), Right: core.Pt(8), JustifyContent: core.JustifyCenter, PointerEvents: core.PointerEventsNone}
	if !Touch(cx) {
		s.Opacity = core.Alpha(0)
		s.Transition = core.Transition{Duration: quick.Duration, Easing: core.EaseOut, Properties: core.TransitionOpacity}
		shown := &core.Style{Opacity: core.Alpha(1)}
		s.GroupStyle = &core.GroupStyle{Name: sidebarTitle, Hover: shown, Focus: shown}
	}
	return core.View{Style: s, Children: []core.Node{disclosureArrow(cx, open)}}
}

// opener is whether the section's items show, and what flips that.
func (sec SidebarSection) opener(cx *core.Scope) (open func() bool, toggle func()) {
	if !sec.Collapsible || sec.Title == "" {
		return func() bool { return true }, func() {}
	}
	own := core.NewSignal(cx, !sec.DefaultClosed)
	open = own.Get
	if sec.Open != nil {
		open = sec.Open
	}
	return open, func() {
		next := !core.Untracked(cx, open)
		own.Set(next)
		if sec.OnOpenChange != nil {
			sec.OnOpenChange(next)
		}
	}
}
