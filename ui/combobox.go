package ui

import (
	"slices"
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/base/locales"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// Combobox is a text field that filters Options as it is typed in and
// picks from them: one value that a choice replaces, closing the list, or
// with Multiple several that a choice toggles. The list marks what is chosen
// with its rows' selected face. Options' Icon and Content go unused.
type Combobox[T comparable] struct {
	Options     []Option[T]
	Multiple    bool
	Value       func() []T
	OnChange    func(value []T)
	Placeholder string
	// Empty shows while none matches.
	Empty    string
	Disabled func() bool
	// Presentation is base's: a popup at the field by default, or a
	// sheet, which ui chooses on no device of its own.
	Presentation base.Presentation
	// Label names the field for assistive technology.
	Label string
	// TestID names the box around the field.
	TestID string
	// Style merges over the box's.
	Style core.Style
}

// Render implements core.Node.
func (c Combobox[T]) Render(cx *core.Scope) core.Node {
	row, look := listRowStyle(cx, c.Presentation)
	group, groupLook := listGroupStyle(cx)
	popup, backdrop := pickerSurface(cx, c.Presentation)
	indexes := core.NewMemoFunc(cx, func() []string {
		var out []string
		if c.Value != nil {
			for _, v := range c.Value() {
				if i := slices.IndexFunc(c.Options, func(o Option[T]) bool { return o.Value == v }); i >= 0 {
					out = append(out, strconv.Itoa(i))
				}
			}
		}
		return out
	}, slices.Equal[[]string])
	empty := c.Empty
	if empty == "" {
		empty = locales.ListNothingFound(cx)
	}
	return base.Combobox{Items: optionItems(c.Options), Multiple: c.Multiple, Value: indexes.Get, Disabled: c.Disabled, Presentation: c.Presentation, Placeholder: c.Placeholder,
		OnChange: func(v []string) {
			if c.OnChange == nil {
				return
			}
			out := make([]T, 0, len(v))
			for _, s := range v {
				if i, err := strconv.Atoi(s); err == nil {
					out = append(out, c.Options[i].Value)
				}
			}
			c.OnChange(out)
		},
		Offset: 4, Margin: 8, Label: c.Label, TestID: c.TestID, Style: core.Style{Position: core.Relative, AlignItems: core.AlignCenter}.Merge(c.Style),
		Field: func(in core.Input) core.Node {
			in.Style = in.Style.Merge(core.Style{FlexGrow: 1, FlexShrink: 1, Padding: core.Edges{Left: 10, Right: 28}})
			return Input(in)
		},
		TriggerButton: Icon(lucide.ChevronDown, iconSize-2, ColorsOf(cx).Muted),
		TriggerButtonStyle: core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Right: core.Pt(0), Width: core.Pt(28),
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		PopupStyleFunc: popup, BackdropStyle: backdrop, ListStyle: core.Style{MaxHeight: core.Pt(320)}, Empty: emptyList(cx, empty),
		ItemStyle: row, ItemStyleFunc: look, GroupStyle: group, GroupStyleFunc: groupLook,
		RenderItem: func(e base.SelectEntry) core.Node {
			return optionRow(cx, c.Options[e.Index], "")
		}}
}
