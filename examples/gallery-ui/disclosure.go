package ui

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TabsVariant is a tab list's look.
type TabsVariant uint8

const (
	// TabsUnderline underlines the selected tab on a line under the list:
	// the default, for sections of a page.
	TabsUnderline TabsVariant = iota
	// TabsSegmented raises the selected tab on a sunken track, for views
	// of the same content.
	TabsSegmented
)

// TabItem is one tab of Tabs.
type TabItem struct {
	Text     string
	Icon     IconName
	Disabled func() bool
	TestID   string
}

// Tabs is a tab list: the app keeps which tab is Selected, by index, and
// shows its content beside the list.
type Tabs struct {
	Variant  TabsVariant
	Items    []TabItem
	Selected func() int
	// OnChange receives the index of a pressed tab other than the
	// selected one.
	OnChange func(index int)
	// Label names the list for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (t Tabs) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	list := core.Style{Direction: core.Row, AlignItems: core.AlignStretch}
	tab := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
		Cursor: core.CursorPointer, Transition: quick, Disabled: &core.Style{Cursor: core.CursorDefault}}
	segmented := t.Variant == TabsSegmented
	if segmented {
		list.CornerRadius, list.Padding, list.Gap, list.BorderWidth = RadiusControl, core.All(2), 2, 1
		tab.Height, tab.Padding, tab.CornerRadius, tab.BorderWidth = core.Pt(ButtonHeight-4), core.Symmetric(0, 12), RadiusControl-2, 1
	} else {
		list.Gap, list.Borders = 16, core.Edges{Bottom: 1}
		tab.Height, tab.Padding = core.Pt(ButtonHeight+4), core.Symmetric(0, 4)
		tab.Borders, tab.Margin = core.Edges{Bottom: 2}, core.Edges{Bottom: -1}
	}
	tabLook := func() core.Style {
		s := core.Style{Color: colors.Muted(), Hover: &core.Style{Color: colors.Text()}, Focus: FocusRing(cx, -2),
			Disabled: &core.Style{Color: colors.Faint()}}
		if segmented {
			s.BorderColor = transparent
			s.Selected = &core.Style{Background: colors.Raised(), BorderColor: colors.Border(), Shadows: FinOf(cx).Raised().Shadows,
				Color: colors.Text()}
		} else {
			s.BorderColors = core.EdgeColors{Bottom: transparent}
			s.Selected = &core.Style{BorderColors: core.EdgeColors{Bottom: colors.Accent()}, Color: colors.Text()}
		}
		return s
	}
	selected := func() int {
		if t.Selected == nil {
			return 0
		}
		return t.Selected()
	}
	var tabs []core.Node
	for i, item := range t.Items {
		var children []core.Node
		if item.Icon != "" {
			children = append(children, Icon(item.Icon, iconSize, inherited))
		}
		if item.Text != "" {
			children = append(children, line(item.Text, typeLabel, 1))
		}
		tabs = append(tabs, base.Tab{Selected: func() bool { return selected() == i }, Disabled: item.Disabled,
			OnPress: func(core.PressEvent) {
				if t.OnChange != nil && core.Untracked(cx, selected) != i {
					t.OnChange(i)
				}
			},
			Style: tab, StyleFunc: tabLook, Children: children, Label: item.Text, TestID: item.TestID})
	}
	return base.Tabs{Style: list, StyleFunc: func() core.Style {
		if segmented {
			return core.Style{Background: colors.Inset(), BorderColor: colors.Line()}.Merge(t.Style)
		}
		return core.Style{BorderColors: core.EdgeColors{Bottom: colors.Line()}}.Merge(t.Style)
	}, Children: tabs, Label: t.Label, TestID: t.TestID}
}

// Collapsible is a titled region whose Content the trigger row shows and
// hides; the content is made the first time it opens and kept after.
type Collapsible struct {
	Open         func() bool
	OnOpenChange func(open bool)
	Text         string
	// Trigger replaces Text in the trigger row, before its chevron.
	Trigger  []core.Node
	Content  core.Node
	Disabled func() bool
	TestID   string
	Style    core.Style
}

// Render implements core.Node.
func (c Collapsible) Render(cx *core.Scope) core.Node {
	return base.Collapsible{Open: c.Open, Content: core.View{Style: core.Style{Padding: core.Edges{Top: 4, Bottom: 8}},
		Children: []core.Node{c.Content}}, Style: c.Style, TestID: c.TestID,
		Children: []core.Node{collapsibleTrigger(cx, c, core.Style{Padding: core.Symmetric(0, 8)})}}
}

// collapsibleTrigger is the row that opens and closes c: its title and a
// chevron that turns as it opens.
func collapsibleTrigger(cx *core.Scope, c Collapsible, s core.Style) core.Node {
	colors := ColorsOf(cx)
	title := c.Trigger
	if title == nil {
		title = []core.Node{line(c.Text, typeLabel.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), 0)}
	}
	chevron := core.Style{FlexShrink: 0}
	if !reduceMotion(cx) {
		chevron.Transition = core.Transition{Duration: 160 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform}
	}
	turn := core.View{Style: chevron, StyleFunc: func() core.Style {
		if is(c.Open) {
			return core.Style{Transform: core.Transform{Rotate: 180}}
		}
		return core.Style{}
	}, Children: []core.Node{Icon(IconChevronDown, iconSize, colors.Muted)}}
	return base.Button{Expanded: func() bool { return is(c.Open) }, Disabled: c.Disabled, Label: c.Text,
		OnPress: func(core.PressEvent) {
			if c.OnOpenChange != nil {
				c.OnOpenChange(!core.Untracked(cx, func() bool { return is(c.Open) }))
			}
		},
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(ButtonHeight + 4),
			CornerRadius: RadiusControl, Cursor: core.CursorPointer, Transition: quick, Disabled: dimmed()}.Merge(s),
		StyleFunc: func() core.Style {
			return core.Style{Color: colors.Text(), Hover: &core.Style{Background: colors.Hover()},
				Active: &core.Style{Background: colors.Pressed()}, Focus: FocusRing(cx, -2)}
		},
		Children: append(append([]core.Node(nil), title...), turn)}
}

// AccordionItem is one section of an Accordion.
type AccordionItem struct {
	Text     string
	Content  core.Node
	Disabled func() bool
	TestID   string
}

// Accordion is a card of Collapsibles divided by hairlines. The app keeps
// which are open: one at a time, or any.
type Accordion struct {
	Items        []AccordionItem
	Open         func(index int) bool
	OnOpenChange func(index int, open bool)
	TestID       string
	Style        core.Style
}

// Render implements core.Node.
func (a Accordion) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var sections []core.Node
	for i, item := range a.Items {
		c := Collapsible{Text: item.Text, Disabled: item.Disabled,
			Open: func() bool { return a.Open != nil && a.Open(i) },
			OnOpenChange: func(open bool) {
				if a.OnOpenChange != nil {
					a.OnOpenChange(i, open)
				}
			}}
		section := base.Collapsible{Open: c.Open, TestID: item.TestID,
			Content:  core.View{Style: core.Style{Padding: core.Edges{Left: 14, Right: 14, Bottom: 14}}, Children: []core.Node{item.Content}},
			Children: []core.Node{collapsibleTrigger(cx, c, core.Style{Padding: core.Symmetric(8, 14), CornerRadius: RadiusCard - 1})}}
		if i > 0 {
			section.Style = core.Style{Borders: core.Edges{Top: 1}}
			section.StyleFunc = func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }
		}
		sections = append(sections, section)
	}
	return core.View{TestID: a.TestID, StyleFunc: func() core.Style { return CardStyle(cx).Merge(a.Style) }, Children: sections}
}

// Progress is a bar of how far a task is, Value from 0 to 100, or with
// Indeterminate one under way, a segment sweeping across it.
type Progress struct {
	Value         func() float32
	Indeterminate func() bool
	// Label names the task for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (p Progress) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	height := core.Points(6)
	if touch(cx) {
		height = 8
	}
	bar := func() core.Style {
		return core.Style{Height: core.Pt(height), CornerRadius: radiusFull, Background: colors.Accent()}
	}
	still := reduceMotion(cx)
	var sweep *core.Signal[bool]
	if p.Indeterminate != nil && !still {
		sweep = core.NewSignal(cx, false)
		const period = 900 * time.Millisecond
		var tick func()
		tick = func() {
			if is(p.Indeterminate) {
				sweep.Set(!sweep.GetUntracked())
			}
			cx.AfterFunc(period, tick)
		}
		cx.AfterFunc(0, tick)
	}
	return base.Progress{Value: p.Value, Indeterminate: p.Indeterminate, Label: p.Label, TestID: p.TestID, Style: p.Style,
		TrackStyle:     core.Style{Height: core.Pt(height), CornerRadius: radiusFull, BorderWidth: 0, Overflow: core.Hidden},
		TrackStyleFunc: Filled(colors.Inset),
		IndicatorStyleFunc: func() core.Style {
			if !is(p.Indeterminate) {
				return bar().Merge(core.Style{Transition: core.Transition{Duration: 200 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionLayout}})
			}
			if still {
				return bar().Merge(core.Style{Width: core.Pct(100), Animation: core.KeyframeAnimation{Keyframes: core.KeyframesPulse}})
			}
			s := bar().Merge(core.Style{Width: core.Pct(100.0 / 3),
				Transition: core.Transition{Duration: 900 * time.Millisecond, Easing: core.EaseInOut, Properties: core.TransitionTransform}})
			if sweep.Get() {
				s.Transform.TranslateXPercent = 200
			}
			return s
		}}
}
