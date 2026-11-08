package ui

import (
	"slices"
	"strconv"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// SidebarItem is a destination of a Sidebar.
type SidebarItem struct {
	// ID is what Selected and OnSelect name the item by.
	ID string
	// Text labels it, shown after Icon unless Content replaces both.
	Text    string
	Icon    func() *core.SVG
	Content core.Node
	// Expanded marks an item disclosing nested items, which the app lists
	// after it while it returns true.
	Expanded func() bool
	// Menu is its context menu's items.
	Menu   []core.Node
	TestID string
}

// SidebarSection is a group of a Sidebar's items under an optional
// title.
type SidebarSection struct {
	Title string
	Items []SidebarItem
	// Trailing ends the title, such as a count of the items; a collapsible
	// title's arrow takes its place while the pointer or the focus is on
	// it. Actions end the title's row, after the title. Note follows the
	// items, for a section that has none yet.
	Trailing core.Node
	Actions  core.Node
	Note     core.Node
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
	// SectionsFunc replaces Sections with ones that change: the items are
	// built anew as it does, keeping the scroll position, the keys and the
	// sections' own open state.
	SectionsFunc func() []SidebarSection
	Selected     func() string
	OnSelect     func(id string)
	TestID       string
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
	// focused is the keyboard focus on the sidebar; taking counts the
	// times it took the keys, 0 while it holds none, so taking them again
	// binds them anew, above a list's.
	focused, taking := core.NewSignal(cx, false), core.NewSignal(cx, 0)
	take := func() { taking.Update(func(n int) int { return n + 1 }) }
	// The sections by key in order, each one's items, whether each item
	// shows, and where each is: what the keys move through and scroll
	// into view.
	var keys []string
	ids := map[string][]string{}
	shown := map[string]func() bool{}
	refs := map[string]*core.Ref{}
	order := func() []string {
		var all []string
		for _, k := range keys {
			all = append(all, ids[k]...)
		}
		return all
	}
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
		all := order()
		for i := at; i >= 0 && i < len(all); i += step {
			if core.Untracked(cx, shown[all[i]]) {
				choose(all[i])
				reveal(all[i])
				return
			}
		}
	}
	// move walks from the selected item, or the first when none is.
	move := func(step int) func() {
		return func() {
			at := slices.Index(order(), core.Untracked(cx, selected))
			if at < 0 {
				walk(0, 1)
				return
			}
			walk(at+step, step)
		}
	}
	item := func(it SidebarItem, open func() bool) core.Node {
		ref := &core.Ref{}
		shown[it.ID], refs[it.ID] = open, ref
		on := func() bool { return selected() == it.ID }
		children := []core.Node{it.Content}
		if it.Content == nil {
			label := Text(it.Text, 13, Regular, colors.Text)
			label.MaxLines, label.Style.FlexGrow, label.Style.FlexShrink = 1, 1, 1
			children = []core.Node{Icon(it.Icon, 16, colors.Accent), label}
		}
		row := core.View{Ref: ref, Role: core.RoleButton, Label: it.Text, Selected: on, Expanded: it.Expanded, NoFocus: true, TestID: it.TestID,
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
			Children:   children}
		if it.Menu == nil {
			return row
		}
		return ContextMenu{Trigger: row, Children: it.Menu}
	}
	// section is a section kept by its key while the sections change: its
	// title, arrow and open state stay, and its items, Trailing and
	// Actions follow.
	section := func(cx *core.Scope, at func() keyedSection) core.Node {
		first := core.Untracked(cx, at)
		key, sec := first.key, first.sec
		now := func() SidebarSection { return at().sec }
		open, toggle := sec.opener(cx, func() SidebarSection { return core.Untracked(cx, at).sec })
		held, hold := sidebarHeld(cx)
		follow := func(part func(SidebarSection) core.Node) core.Node {
			return core.Dynamic(func(cx *core.Scope) core.Node { return part(now()) })
		}
		trailing := follow(func(sec SidebarSection) core.Node { return sec.Trailing })
		var heading core.Node
		if sec.Title != "" {
			title := Text(sec.Title, 11, Bold, colors.Muted)
			title.Role, title.MaxLines = core.RoleHeader, 1
			title.Style.Padding = core.Symmetric(3, 8)
			title.Style.FlexGrow, title.Style.FlexShrink = 1, 1
			heading = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)},
				Children: []core.Node{title, sidebarTrailing(cx, trailing, nil)}}
			if sec.Collapsible {
				title.Role = core.RoleNone
				title.Style.Padding.Right = 4
				heading = base.Button{Label: sec.Title, Expanded: open, OnPress: func(core.PressEvent) { take(); hold(); toggle() },
					Style:     core.Style{Group: sidebarTitle, Direction: core.Row, AlignItems: core.AlignCenter, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), CornerRadius: 8, Cursor: core.CursorDefault},
					StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, -2)} },
					Children:  []core.Node{title, sidebarTrailing(cx, trailing, held), sidebarArrow(cx, open, held)}}
			}
			heading = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter},
				StyleFunc: func() core.Style {
					if now().Actions == nil {
						return core.Style{}
					}
					return core.Style{Padding: core.Edges{Right: 4}}
				},
				Children: []core.Node{heading, follow(func(sec SidebarSection) core.Node { return sec.Actions })}}
		}
		items := core.Dynamic(func(cx *core.Scope) core.Node {
			sec := now()
			var nodes []core.Node
			cx.Untrack(func() {
				for _, id := range ids[key] {
					delete(shown, id)
					delete(refs, id)
				}
				var list []string
				for _, it := range sec.Items {
					list = append(list, it.ID)
					nodes = append(nodes, item(it, open))
				}
				ids[key] = list
				if sec.Note != nil {
					nodes = append(nodes, core.View{Style: core.Style{Padding: core.Symmetric(4, 8)}, Children: []core.Node{sec.Note}})
				}
			})
			return core.Group(nodes)
		})
		if sec.Title != "" && sec.Collapsible {
			return base.Collapsible{Open: open, Reveal: func() bool { return true }, TestID: sec.TestID, Children: []core.Node{heading},
				Content: core.View{Style: core.Style{Padding: core.Edges{Top: 2}, Gap: 2}, Children: []core.Node{items}}}
		}
		nodes := []core.Node{items}
		if heading != nil {
			nodes = []core.Node{heading, items}
		}
		return core.View{TestID: sec.TestID, Style: core.Style{Gap: 2}, Children: nodes}
	}
	list := func() []SidebarSection { return s.Sections }
	if s.SectionsFunc != nil {
		list = s.SectionsFunc
	}
	body := core.Each(func() []keyedSection {
		var all []keyedSection
		var next []string
		for i, sec := range list() {
			k := sec.key(i)
			all, next = append(all, keyedSection{k, sec}), append(next, k)
		}
		keys = next
		return all
	}, func(k keyedSection) string { return k.key }, func(cx *core.Scope, at func() keyedSection, _ func() int) core.Node { return section(cx, at) })
	bindings := core.Show{When: func() bool { return taking.Get() > 0 }, Then: func(cx *core.Scope) core.Node {
		repeat := core.KeyOptions{When: core.KeyNotEditing, Repeat: true}
		cx.OnKeyWith("up", repeat, move(-1))
		cx.OnKeyWith("down", repeat, move(1))
		cx.OnKeyWith("home", repeat, func() { walk(0, 1) })
		cx.OnKeyWith("end", repeat, func() { walk(len(order())-1, -1) })
		return nil
	}}
	return core.Scroll{Ref: scroll, TestID: s.TestID, Style: FillRest, OnScroll: func(e core.ScrollEvent) { offset = e.Offset.Y },
		Children: []core.Node{core.View{Ref: content, OnPress: func(core.PressEvent) { take() },
			OnFocus: func() { focused.Set(true); take() }, OnBlur: func() { focused.Set(false); taking.Set(0) },
			Style: core.Style{Padding: core.Edges{Right: 10, Bottom: 12, Left: 10}, Gap: 10}, Children: []core.Node{body, bindings}}}}
}

// keyedSection is a section under the key that keeps it while the
// sections change.
type keyedSection struct {
	key string
	sec SidebarSection
}

// key is what keeps the section at i while the sections change: its title
// and whether it folds, or its place for one with no title.
func (sec SidebarSection) key(i int) string {
	if sec.Title == "" {
		return "#" + strconv.Itoa(i)
	}
	return strconv.FormatBool(sec.Collapsible) + ":" + sec.Title
}

// sidebarTitle is the group of a collapsible section's title.
const sidebarTitle = "sidebar-title"

// sidebarLinger is how long a title's arrow stays after the pointer and
// the focus leave, before Trailing comes back.
const sidebarLinger = 150 * time.Millisecond

// sidebarHold is how long a title's arrow stays after a press, whatever
// the pointer: past the longest fold and the content leaving after it,
// whose many views coming and going can keep the platform from reporting
// the pointer still there.
const sidebarHold = 700 * time.Millisecond

// sidebarHeld is whether a title's arrow is held after a press, and what
// holds it anew.
func sidebarHeld(cx *core.Scope) (held func() bool, hold func()) {
	on := core.NewSignal(cx, false)
	var stop func() bool
	return on.Get, func() {
		if stop != nil {
			stop()
		}
		on.Set(true)
		stop = cx.AfterFunc(sidebarHold, func() { on.Set(false) })
	}
}

// sidebarFade fades a title's arrow or Trailing in or out after delay.
func sidebarFade(delay time.Duration) core.Transition {
	return core.Transition{Duration: quick.Duration, Delay: delay, Easing: core.EaseOut, Properties: core.TransitionOpacity}
}

// sidebarArrow is a collapsible title's arrow, over the title's end so it
// takes no room, showing while the pointer or the focus is on the title or
// it is held, and always on touch screens, which have no pointer to bring
// it.
func sidebarArrow(cx *core.Scope, open, held func() bool) core.Node {
	// The title's padding sits it a point low.
	s := core.Style{Position: core.Absolute, Top: core.Pt(2), Bottom: core.Pt(0), Right: core.Pt(8), JustifyContent: core.JustifyCenter, PointerEvents: core.PointerEventsNone}
	if !Touch(cx) {
		s.Opacity = core.Alpha(0)
		s.Transition = sidebarFade(sidebarLinger)
		shown := &core.Style{Opacity: core.Alpha(1), Transition: sidebarFade(0)}
		s.GroupStyle = &core.GroupStyle{Name: sidebarTitle, Hover: shown, Focus: shown}
	}
	return core.View{Style: s, StyleFunc: heldStyle(held, core.Alpha(1)), Children: []core.Node{disclosureArrow(cx, open)}}
}

// sidebarTrailing is a title's Trailing, at its end. Under a collapsible
// title's arrow, held being its, it is at least as wide and fades as the
// arrow shows, or, on touch screens, where the arrow always shows, stands
// before it.
func sidebarTrailing(cx *core.Scope, node core.Node, held func() bool) core.Node {
	s := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyEnd, FlexShrink: 0, Padding: core.Edges{Top: 2, Right: 8}, PointerEvents: core.PointerEventsNone}
	var style func() core.Style
	if held != nil {
		s.MinWidth = core.Pt(8 + iconSize)
		if Touch(cx) {
			s.Padding.Right = 8 + iconSize + 4
		} else {
			s.Transition = sidebarFade(sidebarLinger)
			hidden := &core.Style{Opacity: core.Alpha(0), Transition: sidebarFade(0)}
			s.GroupStyle = &core.GroupStyle{Name: sidebarTitle, Hover: hidden, Focus: hidden}
			style = heldStyle(held, core.Alpha(0))
		}
	}
	return core.View{Style: s, StyleFunc: style, Children: []core.Node{node}}
}

// heldStyle shows opacity while held, as the pointer on the title does.
func heldStyle(held func() bool, opacity core.Opacity) func() core.Style {
	return func() core.Style {
		if !held() {
			return core.Style{}
		}
		return core.Style{Opacity: opacity, Transition: sidebarFade(0)}
	}
}

// opener is whether the section's items show, and what flips that: the
// app's Open while now, the section as it is now, has one, its own state
// otherwise.
func (sec SidebarSection) opener(cx *core.Scope, now func() SidebarSection) (open func() bool, toggle func()) {
	if !sec.Collapsible || sec.Title == "" {
		return func() bool { return true }, func() {}
	}
	own := core.NewSignal(cx, !sec.DefaultClosed)
	open = func() bool {
		if o := now().Open; o != nil {
			return o()
		}
		return own.Get()
	}
	return open, func() {
		next := !core.Untracked(cx, open)
		own.Set(next)
		if on := now().OnOpenChange; on != nil {
			on(next)
		}
	}
}
