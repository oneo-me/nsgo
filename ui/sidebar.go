package ui

import (
	"slices"

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
	Title  string
	Items  []SidebarItem
	TestID string
}

// Sidebar is grouped navigation: the sections' items, the Selected one on
// a faint fill of the text's color, icons in the accent.
// It scrolls and leaves its background to the shell.
//
// It is one stop of Tab, as a macOS source list is: while it has the
// keyboard focus, the selected item is ringed, Up and Down select the item
// before and after, Home and End the first and the last, scrolled into
// view.
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
	height := core.Points(32)
	if !desk {
		height = 40
	}
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
	focused := core.NewSignal(cx, false)
	row := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(height),
		Padding: core.Symmetric(0, 8), CornerRadius: 8, Cursor: core.CursorDefault}
	selectedFill := func() core.Color { return Alpha(colors.Text(), 0.1) }

	// The items in order, and where each is: what the keys move through
	// and scroll into view.
	var order []string
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
	// move selects the item at to(at, last): at is the selected one's
	// place, -1 when it is not shown.
	move := func(to func(at, last int) int) {
		if len(order) == 0 {
			return
		}
		at := slices.Index(order, core.Untracked(cx, selected))
		i := min(max(to(at, len(order)-1), 0), len(order)-1)
		if i == at {
			return
		}
		choose(order[i])
		reveal(order[i])
	}

	item := func(item SidebarItem) core.Node {
		ref := &core.Ref{}
		order = append(order, item.ID)
		refs[item.ID] = ref
		on := func() bool { return selected() == item.ID }
		label := Text(item.Text, 13, Regular, colors.Text)
		label.MaxLines, label.Style.FlexGrow, label.Style.FlexShrink = 1, 1, 1
		return core.View{Ref: ref, Role: core.RoleButton, Label: item.Text, Selected: on, NoFocus: true, TestID: item.TestID,
			OnPress: func(core.PressEvent) { choose(item.ID) },
			Style:   row, StyleFunc: func() core.Style {
				st := core.Style{Selected: &core.Style{Background: selectedFill()}}
				if on() && focused.Get() {
					st = st.Merge(*FocusRing(cx, -2))
				}
				return st
			},
			Children: []core.Node{Icon(item.Icon, 16, colors.Accent), label}}
	}
	var sections []core.Node
	for _, sec := range s.Sections {
		var nodes []core.Node
		if sec.Title != "" {
			title := Text(sec.Title, 11, Bold, colors.Muted)
			title.Role, title.MaxLines = core.RoleHeader, 1
			title.Style.Padding = core.Edges{Top: 4, Right: 8, Bottom: 2, Left: 8}
			nodes = append(nodes, title)
		}
		for _, it := range sec.Items {
			nodes = append(nodes, item(it))
		}
		sections = append(sections, core.View{TestID: sec.TestID, Style: core.Style{Gap: 1}, Children: nodes})
	}

	// The keys are the list's only while it has the focus: elsewhere the
	// arrows are the page's.
	keys := core.Show{When: focused.Get, Then: func(cx *core.Scope) core.Node {
		repeat := core.KeyOptions{When: core.KeyNotEditing, Repeat: true}
		cx.OnKeyWith("up", repeat, func() { move(func(at, _ int) int { return at - 1 }) })
		cx.OnKeyWith("down", repeat, func() { move(func(at, _ int) int { return at + 1 }) })
		cx.OnKeyWith("home", repeat, func() { move(func(int, int) int { return 0 }) })
		cx.OnKeyWith("end", repeat, func() { move(func(_, last int) int { return last }) })
		return core.Group{}
	}}
	return core.Scroll{Ref: scroll, TestID: s.TestID, Style: FillRest, OnScroll: func(e core.ScrollEvent) { offset = e.Offset.Y },
		Children: []core.Node{core.View{Ref: content, OnPress: func(core.PressEvent) {},
			OnFocus: func() { focused.Set(true) }, OnBlur: func() { focused.Set(false) },
			Style: core.Style{Padding: core.Edges{Right: 12, Bottom: 12, Left: 12}, Gap: 12}, Children: append(sections, keys)}}}
}
