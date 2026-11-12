package base

import (
	"slices"

	"github.com/nsgo-dev/nsgo/core"
)

// accordionContext is what an AccordionItem finds of its Accordion.
type accordionContext struct {
	value            func() []string
	multiple, locked func() bool
	change           func([]string)
}

// accordionItem is what a trigger and a panel find of their item.
type accordionItem struct {
	open, disabled func() bool
	toggle         func()
}

var (
	accordionKey     = core.NewKey[*accordionContext]("base.accordion", nil)
	accordionItemKey = core.NewKey[*accordionItem]("base.accordionItem", nil)
)

// Accordion is stacked sections that each expand and collapse under a
// header, one at a time or, with Multiple, several. Value holds the open
// AccordionItems' values: a trigger's press reports the next ones with
// OnChange. It has no look, and assistive technology hears a group. Each
// trigger is a Tab stop of its own, as gpui-base's are.
type Accordion struct {
	Value func() []string
	// Multiple leaves the others open as one opens.
	Multiple func() bool
	// Disabled disables every item.
	Disabled func() bool
	// OnChange receives the open values as they would be.
	OnChange  func(value []string)
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (a Accordion) Render(cx *core.Scope) core.Node {
	accordionKey.Provide(cx, &accordionContext{value: a.Value, multiple: a.Multiple, locked: a.Disabled,
		change: func(v []string) {
			if a.OnChange != nil {
				a.OnChange(v)
			}
		}})
	return core.View{Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, Role: core.RoleGroup, Label: a.Label, TestID: a.TestID, OnLayout: a.OnLayout, Tooltip: a.Tooltip,
		Children: a.Children}
}

// AccordionItem is one section of an Accordion: an AccordionHeader with
// its AccordionTrigger and an AccordionPanel, open while the accordion's
// Value holds its Value. It has no look or role of its own.
type AccordionItem struct {
	// Value identifies it in the accordion's: the app's, never its title.
	Value string
	// Disabled stops its trigger; it stays as the accordion's Value says.
	Disabled  func() bool
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (i AccordionItem) Render(cx *core.Scope) core.Node {
	a := accordionKey.Use(cx)
	open := core.NewMemo(cx, func() bool { return a != nil && a.value != nil && slices.Contains(a.value(), i.Value) })
	disabled := core.NewMemo(cx, func() bool { return is(i.Disabled) || a != nil && is(a.locked) })
	accordionItemKey.Provide(cx, &accordionItem{open: open.Get, disabled: disabled.Get, toggle: func() {
		if a == nil {
			return
		}
		next := []string{}
		if a.value != nil {
			next = slices.Clone(core.Untracked(cx, a.value))
		}
		switch at := slices.Index(next, i.Value); {
		case at >= 0:
			next = slices.Delete(next, at, at+1)
		case core.Untracked(cx, func() bool { return is(a.multiple) }):
			next = append(next, i.Value)
		default:
			next = []string{i.Value}
		}
		a.change(next)
	}})
	return core.View{Class: i.Class, Style: i.Style, StyleFunc: i.StyleFunc, TestID: i.TestID, OnLayout: i.OnLayout, Tooltip: i.Tooltip,
		Children: i.Children}
}

// AccordionHeader is the heading of an AccordionItem around its
// AccordionTrigger, which assistive technology moves between by level.
type AccordionHeader struct {
	// Level is its level among the page's headings, 3 when 0.
	Level     int
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (h AccordionHeader) Render(*core.Scope) core.Node {
	level := h.Level
	if level <= 0 {
		level = 3
	}
	return core.View{Class: h.Class, Style: h.Style, StyleFunc: h.StyleFunc, Role: core.RoleHeader, TestID: h.TestID, OnLayout: h.OnLayout, Tooltip: h.Tooltip,
		AccessibilityLevel: func() int { return level }, Children: h.Children}
}

// AccordionTrigger is the button in an AccordionHeader that expands and
// collapses its item, reporting the accordion's next Value. Style.Expanded
// styles it while open.
type AccordionTrigger struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t AccordionTrigger) Render(cx *core.Scope) core.Node {
	item := accordionItemKey.Use(cx)
	if item == nil {
		item = &accordionItem{open: func() bool { return false }, disabled: func() bool { return false }, toggle: func() {}}
	}
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleButton, Label: t.Label, Hint: t.Hint, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Disabled: item.disabled, Expanded: item.open, ShowsExpanded: true, OnPress: func(core.PressEvent) { item.toggle() }, Children: t.Children}
}

// AccordionPanel is the content of an AccordionItem, shown while it is
// open: a region for assistive technology, hidden from it while closed.
type AccordionPanel struct {
	// Reveal opens and closes it as a Collapsible's Reveal does. The panel
	// is in the layout while closed, zero high, so its padding and borders
	// belong on a view of its content.
	Reveal func() bool
	// KeepMounted keeps the content in the tree while closed, out of
	// sight, of the focus and of assistive technology: a video keeps its
	// place. Without it the content leaves as the panel closes and comes
	// back as it was.
	KeepMounted func() bool
	Class       string
	Style       core.Style
	StyleFunc   func() core.Style
	Children    []core.Node
	Label       string
	TestID      string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (p AccordionPanel) Render(cx *core.Scope) core.Node {
	open := func() bool { return false }
	if item := accordionItemKey.Use(cx); item != nil {
		open = item.open
	}
	mounted := followOpen(cx, open, p.Reveal)
	held := core.NewHolder(cx, func(*core.Scope) core.Node { return core.Group(p.Children) })
	style := func() core.Style {
		var s core.Style
		if p.StyleFunc != nil {
			s = p.StyleFunc()
		}
		switch {
		case !is(p.Reveal):
			if !open() {
				s.Display = core.DisplayNone
			}
		default:
			s.Overflow = core.Hidden
			if !open() {
				s.Height = core.Pt(0)
				if !mounted() {
					s.Visibility = core.VisibilityHidden
				}
			}
		}
		return s
	}
	return core.View{Class: p.Class, Style: p.Style, StyleFunc: style, Role: core.RoleRegion, Label: p.Label, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip,
		AccessibilityElementsHidden: func() bool { return !open() },
		Children:                    []core.Node{core.Show{When: func() bool { return mounted() || is(p.KeepMounted) }, Then: func(*core.Scope) core.Node { return held.Place() }}}}
}
