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
// or Placeholder while Value matches none, and the menu checks it.
// Options' Icon, Disabled and Content go unused.
type Select[T comparable] struct {
	Options     []Option[T]
	Value       func() T
	OnChange    func(value T)
	Placeholder string
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
	chosen := func() (string, bool) {
		if s.Value == nil {
			return s.Placeholder, false
		}
		v := s.Value()
		for _, o := range s.Options {
			if o.Value == v {
				return o.Label, true
			}
		}
		return s.Placeholder, false
	}
	text := core.Text{MaxLines: 1, Style: TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}),
		ContentFunc: func() string { t, _ := chosen(); return t },
		StyleFunc: func() core.Style {
			if _, ok := chosen(); ok {
				return core.Style{Color: colors.Text()}
			}
			return core.Style{Color: colors.Faint()}
		}}
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
		Children: []core.Node{text, Icon(lucide.ChevronDown, iconSize-2, colors.Muted)}}
	items := make([]core.Node, len(s.Options))
	for i, o := range s.Options {
		id := ""
		if s.TestID != "" {
			id = s.TestID + "-" + strconv.Itoa(i)
		}
		items[i] = MenuItem{Text: o.Label, TestID: id,
			Checked: func() bool { return s.Value != nil && s.Value() == o.Value },
			OnSelect: func() {
				if s.OnChange != nil && (s.Value == nil || s.Value() != o.Value) {
					s.OnChange(o.Value)
				}
			}}
	}
	popup, backdrop := menuSurface(cx)
	label := s.Label
	if label == "" {
		label = s.Placeholder
	}
	return base.Menu{Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) }, Offset: 4, Margin: 8,
		Style: core.Style{Group: "button", Width: core.Pt(200), CornerRadius: RadiusControl}.
			Merge(slop(cx, ButtonHeight)).Merge(s.Style),
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Trigger:   face, PopupStyleFunc: popup, BackdropStyle: backdrop, Children: items, Label: label, TestID: s.TestID}
}
