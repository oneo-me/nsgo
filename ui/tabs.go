package ui

import (
	"fmt"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TabsVariant is a tab list's look.
type TabsVariant uint8

const (
	// TabsSegmented sets the selected tab on a raised thumb in a recessed
	// capsule: the default.
	TabsSegmented TabsVariant = iota
	// TabsUnderline marks the selected tab with an accent underline.
	TabsUnderline
)

// tabsSlide is how the indicator moves to the selected tab.
var tabsSlide = core.Transition{Duration: 180 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionLayout}

// Tabs picks one of Options: a tab list whose content the app shows beside
// it, or a segmented control. One indicator marks the selected tab and
// slides to the next, resizing to it.
type Tabs[T comparable] struct {
	Variant TabsVariant
	Options []Option[T]
	Value   func() T
	// OnChange receives the value of a pressed tab other than the selected
	// one.
	OnChange func(T)
	// Equal fills the width with tabs of one width, instead of each
	// fitting its content.
	Equal bool
	// Label names the list for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (t Tabs[T]) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	underline := t.Variant == TabsUnderline
	selected := func() int {
		if t.Value == nil {
			return 0
		}
		v := t.Value()
		for i, o := range t.Options {
			if o.Value == v {
				return i
			}
		}
		return -1
	}
	list := core.Style{Direction: core.Row, Gap: 4}
	tab := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
		Padding: core.Symmetric(8, 12), CornerRadius: RadiusControl, FontSize: TypeBody.FontSize, LineHeight: TypeBody.LineHeight, FontWeight: Regular}
	if !underline {
		list.Padding, list.Gap, list.CornerRadius = core.All(2), 2, RadiusFull
		tab.Height, tab.Padding, tab.CornerRadius = core.Pt(ButtonHeight), core.Symmetric(0, 12), ButtonHeight/2
	}
	if t.Equal {
		// A grid's tracks are one width whether the list fills its parent or
		// fits the widest tab.
		list.Display, list.GridTemplateColumns = core.DisplayGrid, core.GridTracks(fmt.Sprintf("repeat(%d, minmax(0, 1fr))", max(1, len(t.Options))))
	} else if !underline {
		list.AlignSelf = core.AlignStart
	}
	width := core.NewSignal(cx, core.Points(0))
	frames := make([]*core.Signal[core.Bounds], len(t.Options))
	// Only a change of tab slides: the indicator follows a tab that a
	// resize moves at once.
	sliding := core.NewSignal(cx, false)
	last := core.Untracked(cx, selected)
	cx.Effect(func() {
		if i := selected(); i != last {
			last = i
			sliding.Set(true)
		}
	})
	env := cx.Env()
	indicator := core.View{PointerEvents: core.PointerEventsNone, OnTransitionEnd: func() { sliding.Set(false) }, StyleFunc: func() core.Style {
		var f core.Bounds
		if i := selected(); i >= 0 {
			f = frames[i].Get()
		}
		left := f.Origin.X
		// Right to left, Left is the distance from the list's right.
		if env.RTL() {
			left = width.Get() - f.Origin.X - f.Size.Width
		}
		s := core.Style{Position: core.Absolute}
		if sliding.Get() {
			s.Transition = tabsSlide
		}
		if underline {
			const inset, height = 6, 2
			s.Left, s.Top, s.Width, s.Height = core.Pt(left+inset), core.Pt(f.Origin.Y+f.Size.Height-height), core.Pt(max(0, f.Size.Width-2*inset)), core.Pt(height)
			s.CornerRadius, s.Background = height/2, colors.Accent()
			return s
		}
		s.Left, s.Top, s.Width, s.Height = core.Pt(left), core.Pt(f.Origin.Y), core.Pt(f.Size.Width), core.Pt(f.Size.Height)
		s.CornerRadius = f.Size.Height / 2
		return FinOf(cx).Pill().Over(s)
	}}
	tabLook := func() core.Style {
		on := core.Style{Color: colors.Text()}
		// Only the thumb's tab is heavier: an underline never moves the labels.
		if !underline {
			on.FontWeight = Semibold
		}
		return core.Style{Color: Alpha(colors.Text(), 0.6), Selected: &on, Focus: FocusRing(cx, -2), Disabled: Dimmed()}
	}
	for i := range frames {
		frames[i] = core.NewSignal(cx, core.Bounds{})
	}
	tabs := []core.Node{indicator}
	for i, o := range t.Options {
		children := []core.Node{o.Content}
		if o.Content == nil {
			children = nil
			if o.Icon != nil {
				children = append(children, Icon(o.Icon, iconSize, Inherited))
			}
			if o.Label != "" {
				children = append(children, core.Text{Content: o.Label, MaxLines: 1})
			}
		}
		tabs = append(tabs, base.Tab{Selected: func() bool { return selected() == i }, Disabled: o.Disabled,
			OnPress: func(core.PressEvent) {
				if t.OnChange != nil && core.Untracked(cx, selected) != i {
					t.OnChange(o.Value)
				}
			},
			OnLayout: frames[i].Set, Style: tab, StyleFunc: tabLook, Children: children, Label: o.Label, TestID: o.TestID})
	}
	return base.Tabs{Style: list, StyleFunc: func() core.Style {
		if underline {
			return t.Style
		}
		return core.Style{Background: Alpha(colors.Text(), 0.06)}.Merge(t.Style)
	}, OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: tabs, Label: t.Label, TestID: t.TestID}
}
