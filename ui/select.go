package ui

import (
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Select picks one of Options from a field that opens them as a list:
// over it on desktops, the chosen row on the field, where the keys move
// through them and letters jump, and as a sheet on touch screens. The field
// shows the chosen option's label, or Placeholder while Value matches none,
// and the list checks it unless NoCheck. The rows are made as they scroll
// in, so a list may be long. Options' Icon and Content go unused.
type Select[T comparable] struct {
	Options     []Option[T]
	Value       func() T
	OnChange    func(value T)
	Placeholder string
	NoCheck     bool
	// Searchable shows a field above the rows that filters them by label,
	// with SearchPlaceholder in it and Empty while none matches.
	Searchable        bool
	SearchPlaceholder string
	Empty             string
	// OnClear, when set, shows a button in the field while an option is
	// chosen, which asks to choose none.
	OnClear  func()
	Disabled func() bool
	// Label names the field for assistive technology.
	Label string
	// TestID names the field; each option adds "-" and its index.
	TestID string
	// Style merges over the field's, 200 pt wide.
	Style core.Style
}

// optionItems are options as a list's, valued by their index.
func optionItems[T comparable](options []Option[T]) func() []base.SelectItem {
	return func() []base.SelectItem {
		items := make([]base.SelectItem, len(options))
		for i, o := range options {
			items[i] = base.SelectItem{Value: strconv.Itoa(i), Title: o.Label, Disabled: o.Disabled != nil && o.Disabled()}
		}
		return items
	}
}

// optionRow draws option i of a list.
func optionRow[T comparable](cx *core.Scope, o Option[T], id string, checked func() bool) core.Node {
	colors := ColorsOf(cx)
	children := menuLeading(cx, nil, checked)
	if o.Leading != nil {
		children = append(children, o.Leading)
	}
	children = append(children, core.Text{Content: o.Label, MaxLines: 1, Style: TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), StyleFunc: Colored(colors.Text)})
	if o.Trailing != nil {
		children = append(children, o.Trailing)
	}
	return core.View{TestID: id, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, FlexGrow: 1}, Children: children}
}

// Render implements core.Node.
func (s Select[T]) Render(cx *core.Scope) core.Node {
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
	leading := core.Dynamic(func(*core.Scope) core.Node {
		if i := chosen.Get(); i >= 0 && s.Options[i].Leading != nil {
			return s.Options[i].Leading
		}
		return core.Group(nil)
	})
	title := func() string {
		if i := chosen.Get(); i >= 0 {
			return s.Options[i].Label
		}
		return ""
	}
	face := pickerField(cx, leading, pickerText(cx, title, s.Placeholder), lucide.ChevronDown, open.Get)
	row, look := listRowStyle(cx)
	group, groupLook := listGroupStyle(cx)
	popup, backdrop := pickerSurface(cx)
	label := s.Label
	if label == "" {
		label = s.Placeholder
	}
	root := core.Style{Width: core.Pt(200), AlignSelf: core.AlignStart}.Merge(s.Style)
	b := base.Select{Items: optionItems(s.Options), Disabled: s.Disabled, Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
		Value: func() *string {
			if i := chosen.Get(); i >= 0 {
				v := strconv.Itoa(i)
				return &v
			}
			return nil
		},
		OnChange: func(v *string) {
			if v == nil {
				if s.OnClear != nil {
					s.OnClear()
				}
				return
			}
			if i, err := strconv.Atoi(*v); err == nil && s.OnChange != nil {
				s.OnChange(s.Options[i].Value)
			}
		},
		Offset: 4, Margin: 8, RootStyle: root,
		Style:     core.Style{Group: "button", CornerRadius: RadiusControl},
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Trigger:   face, PopupStyleFunc: popup, BackdropStyle: backdrop, ListStyle: core.Style{MaxHeight: core.Pt(320)},
		ItemStyle: row, ItemStyleFunc: look, GroupStyle: group, GroupStyleFunc: groupLook, Label: label, TestID: s.TestID,
		RenderItem: func(e base.SelectEntry) core.Node {
			id := ""
			if s.TestID != "" {
				id = s.TestID + "-" + strconv.Itoa(e.Index)
			}
			var checked func() bool
			if !s.NoCheck {
				checked = func() bool { return chosen.Get() == e.Index }
			}
			return optionRow(cx, s.Options[e.Index], id, checked)
		}}
	if s.Searchable {
		b.Searchable = func() bool { return true }
		b.SearchPlaceholder, b.SearchLabel, b.SearchField = s.SearchPlaceholder, s.SearchPlaceholder, searchField
		if b.SearchLabel == "" {
			b.SearchLabel = locales.ListSearch(cx)
		}
		text := s.Empty
		if text == "" {
			text = locales.ListNothingFound(cx)
		}
		b.Empty = emptyList(cx, text)
	}
	if s.OnClear != nil {
		b.Cleanable, b.ClearButton, b.ClearButtonStyle, b.ClearButtonLabel = true, clearMark(cx), clearStyle, locales.ListClear(cx)
	}
	return b
}
