package base

import (
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// SidebarItem is a destination of a Sidebar.
type SidebarItem struct {
	// ID is what Selected and OnSelect name the item by.
	ID       string
	Label    string
	Children []core.Node
	TestID   string
}

// SidebarSection is a group of a Sidebar's items: Header, the Items, then
// Footer.
type SidebarSection struct {
	Header core.Node
	Items  []SidebarItem
	Footer core.Node
	Class  string
	Style  core.Style
	TestID string
}

// Sidebar is grouped navigation that scrolls. It is one stop of Tab, as a
// macOS source list is: while it has the keyboard focus, Style.Focus on
// ItemStyle shows the selected item, Up and Down select the item before and
// after, Home and End the first and the last, scrolled into view.
type Sidebar struct {
	Sections []SidebarSection
	Selected func() string
	OnSelect func(id string)
	// ItemClass, ItemStyle and ItemStyleFunc are each item's; ContentClass
	// and ContentStyle what scrolls.
	ItemClass     string
	ItemStyle     core.Style
	ItemStyleFunc func() core.Style
	ContentClass  string
	ContentStyle  core.Style
	Class         string
	Style         core.Style
	StyleFunc     func() core.Style
	TestID        string
}

// Render implements core.Node.
func (s Sidebar) Render(cx *core.Scope) core.Node {
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
	move := func(to func(at, last int) int) func() {
		return func() {
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
	}
	item := func(item SidebarItem) core.Node {
		ref := &core.Ref{}
		order = append(order, item.ID)
		refs[item.ID] = ref
		on := func() bool { return selected() == item.ID }
		return core.View{Ref: ref, Role: core.RoleButton, Label: item.Label, Selected: on, NoFocus: true, TestID: item.TestID, Class: s.ItemClass,
			OnPress: func(core.PressEvent) { choose(item.ID) }, Style: s.ItemStyle, StyleFunc: s.ItemStyleFunc,
			PartStates: func() core.PartStates { return core.PartStates{Focused: on() && focused.Get()} },
			Children:   item.Children}
	}
	var sections []core.Node
	for _, sec := range s.Sections {
		var nodes []core.Node
		if sec.Header != nil {
			nodes = append(nodes, sec.Header)
		}
		for _, it := range sec.Items {
			nodes = append(nodes, item(it))
		}
		if sec.Footer != nil {
			nodes = append(nodes, sec.Footer)
		}
		sections = append(sections, core.View{TestID: sec.TestID, Class: sec.Class, Style: sec.Style, Children: nodes})
	}
	keys := bindWhile(focused.Get, core.KeyNotEditing, false,
		controlKey{"up", true, move(func(at, _ int) int { return at - 1 })}, controlKey{"down", true, move(func(at, _ int) int { return at + 1 })},
		controlKey{"home", true, move(func(int, int) int { return 0 })}, controlKey{"end", true, move(func(_, last int) int { return last })})
	return core.Scroll{Ref: scroll, Class: s.Class, TestID: s.TestID, Style: s.Style, StyleFunc: s.StyleFunc, OnScroll: func(e core.ScrollEvent) { offset = e.Offset.Y },
		Children: []core.Node{core.View{Ref: content, Class: s.ContentClass, OnPress: func(core.PressEvent) {},
			OnFocus: func() { focused.Set(true) }, OnBlur: func() { focused.Set(false) },
			Style: s.ContentStyle, Children: append(sections, keys)}}}
}
