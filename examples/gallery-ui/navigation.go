package ui

import (
	"github.com/oneo-me/nsgo/icons/lucide"
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/mobile"
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

// Sidebar is grouped navigation, as Onexa's: the sections' items, the
// Selected one on a faint fill of the text's color, icons in the accent.
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
		Padding: core.Symmetric(0, 8), CornerRadius: 8, Cursor: core.CursorPointer}
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
				if !desk {
					st.Active = &core.Style{Background: selectedFill()}
				}
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
	return core.Scroll{Ref: scroll, TestID: s.TestID, Style: fillRest, OnScroll: func(e core.ScrollEvent) { offset = e.Offset.Y },
		Children: []core.Node{core.View{Ref: content, OnPress: func(core.PressEvent) {},
			OnFocus: func() { focused.Set(true) }, OnBlur: func() { focused.Set(false) },
			Style: core.Style{Padding: core.Edges{Right: 12, Bottom: 12, Left: 12}, Gap: 12}, Children: append(sections, keys)}}}
}

// NavHeader is the bar at the top of a page: a back button named
// BackLabel while OnBack is set, the title, and the app's views at
// either end.
type NavHeader struct {
	Title     string
	OnBack    func()
	BackLabel string
	Leading   []core.Node
	Trailing  []core.Node
	TestID    string
}

// Render implements core.Node.
func (h NavHeader) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var children []core.Node
	if h.OnBack != nil {
		children = append(children, IconButton{Icon: lucide.ChevronLeft, Label: h.BackLabel, Size: 32, OnPress: h.OnBack})
	}
	children = append(children, h.Leading...)
	children = append(children, core.Text{Content: h.Title, MaxLines: 1, Role: core.RoleHeader,
		Style: typeTitle.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), StyleFunc: Colored(colors.Text)})
	children = append(children, h.Trailing...)
	return core.View{TestID: h.TestID, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
		MinHeight: core.Pt(HeaderHeight + 4), Padding: core.Symmetric(0, 12), Borders: core.Edges{Bottom: 1}},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Surface(), BorderColors: core.EdgeColors{Bottom: colors.Line()}}
		}, Children: children}
}

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

// Breadcrumbs is a path: its ancestors pressable, the last item the
// current one. With more than MaxItems, the ancestors after the first
// collapse into a menu under an ellipsis named MoreLabel.
type Breadcrumbs struct {
	Items []string
	// OnSelect receives the index of a pressed ancestor.
	OnSelect  func(index int)
	MaxItems  int
	MoreLabel string
	TestID    string
	Style     core.Style
}

// Render implements core.Node.
func (b Breadcrumbs) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	sep := func() core.Node { return Icon(lucide.ChevronRight, 13, colors.Faint) }
	choose := func(i int) {
		if b.OnSelect != nil {
			b.OnSelect(i)
		}
	}
	crumb := func(i int) core.Node {
		return base.Button{Role: core.RoleLink, Label: b.Items[i], OnPress: func(core.PressEvent) { choose(i) },
			Style: core.Style{Padding: core.Symmetric(2, 4), CornerRadius: radiusSmall, Cursor: core.CursorPointer, MinWidth: core.Pt(0), FlexShrink: 1},
			StyleFunc: func() core.Style {
				return core.Style{Color: colors.Muted(), Hover: &core.Style{Color: colors.Text(), Background: colors.Hover()}, Focus: FocusRing(cx, 0)}
			},
			Children: []core.Node{line(b.Items[i], typeBody, 1)}}
	}
	hidden := 0
	if b.MaxItems > 1 && len(b.Items) > b.MaxItems {
		hidden = len(b.Items) - b.MaxItems + 1
	}
	var out []core.Node
	for i := range b.Items {
		if hidden > 0 && i >= 1 && i <= hidden {
			if i == 1 {
				var items []core.Node
				for j := 1; j <= hidden; j++ {
					items = append(items, MenuItem{Text: b.Items[j], OnSelect: func() { choose(j) }})
				}
				out = append(out, sep(), Menu{Trigger: Button{Variant: ButtonGhost, Icon: lucide.Ellipsis, Label: b.MoreLabel},
					Label: b.MoreLabel, Children: items})
			}
			continue
		}
		if i > 0 {
			out = append(out, sep())
		}
		if i == len(b.Items)-1 {
			out = append(out, core.Text{Content: b.Items[i], MaxLines: 1,
				Style: typeLabel.Merge(core.Style{FlexShrink: 1, Padding: core.Symmetric(2, 4)}), StyleFunc: Colored(colors.Text)})
			continue
		}
		out = append(out, crumb(i))
	}
	return core.View{TestID: b.TestID,
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2, MinWidth: core.Pt(0)}.Merge(b.Style), Children: out}
}
