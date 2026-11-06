package ui

import (
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Select picks one of Options from a field that opens them as a menu:
// beside it on desktops, where the arrow keys move through them, and as
// a sheet on touch screens. The field shows the chosen option's label,
// or Placeholder while Value matches none, and the menu checks it unless
// NoCheck.
// Options' Icon, Disabled and Content go unused.
type Select[T comparable] struct {
	Options     []Option[T]
	Value       func() T
	OnChange    func(value T)
	Placeholder string
	NoCheck     bool
	// Label names the field for assistive technology.
	Label string
	// TestID names the field; each option adds "-" and its index.
	TestID string
	// Style merges over the field's, 200 pt wide.
	Style core.Style
}

// Render implements core.Node.
func (s Select[T]) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	open := core.NewSignal(cx, false)
	chosen := core.NewMemo(cx, func() int {
		if s.Value != nil {
			v := s.Value()
			for i, o := range s.Options {
				if o.Value == v {
					return i
				}
			}
		}
		return -1
	})
	text := core.Text{MaxLines: 1, Style: TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}),
		ContentFunc: func() string {
			if i := chosen.Get(); i >= 0 {
				return s.Options[i].Label
			}
			return s.Placeholder
		},
		StyleFunc: func() core.Style {
			if chosen.Get() >= 0 {
				return core.Style{Color: colors.Text()}
			}
			return core.Style{Color: colors.Faint()}
		}}
	leading := core.Dynamic(func(*core.Scope) core.Node {
		if i := chosen.Get(); i >= 0 && s.Options[i].Leading != nil {
			return s.Options[i].Leading
		}
		return core.Group(nil)
	})
	face := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Height: core.Pt(ButtonHeight),
		Padding: core.Edges{Left: 10, Right: 8}, CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1,
		Transition: quick},
		StyleFunc: func() core.Style {
			bg := colors.Surface()
			border := colors.FieldBorder()
			if open.Get() {
				border = colors.Accent()
			}
			return core.Style{Background: bg, BorderColor: border,
				GroupStyle: &core.GroupStyle{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}}
		},
		Children: []core.Node{leading, text, Icon(lucide.ChevronDown, iconSize-2, colors.Muted)}}
	items := make([]core.Node, len(s.Options))
	for i, o := range s.Options {
		id := ""
		if s.TestID != "" {
			id = s.TestID + "-" + strconv.Itoa(i)
		}
		item := MenuItem{Text: o.Label, Leading: o.Leading, Trailing: o.Trailing, TestID: id,
			OnSelect: func() {
				if s.OnChange != nil && (s.Value == nil || s.Value() != o.Value) {
					s.OnChange(o.Value)
				}
			}}
		if !s.NoCheck {
			item.Checked = func() bool { return s.Value != nil && s.Value() == o.Value }
		}
		items[i] = item
	}
	popup, backdrop := menuSurface(cx)
	label := s.Label
	if label == "" {
		label = s.Placeholder
	}
	return base.Menu{Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) }, Offset: 4, Margin: 8,
		Style:     core.Style{Group: "button", Width: core.Pt(200), CornerRadius: RadiusControl}.Merge(s.Style),
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Trigger:   face, PopupStyleFunc: popup, BackdropStyle: backdrop, Children: items, Label: label, TestID: s.TestID}
}
