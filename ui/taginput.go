package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// TagInput is base.TagInput in Onevra's look: bordered chips in a field.
type TagInput struct {
	Tags     func() []string
	OnChange func(tags []string)
	// Accept, when set, keeps out the tags it rejects, which stay typed.
	Accept      func(tag string) bool
	Placeholder string
	Disabled    func() bool
	// RemoveLabel names a tag's remove button for assistive technology,
	// such as "Remove design".
	RemoveLabel func(tag string) string
	// Label names the field for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (in TagInput) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	const chipSize = ButtonHeight - 8
	return base.TagInput{Tags: in.Tags, OnChange: in.OnChange, Accept: in.Accept, Placeholder: in.Placeholder, Disabled: in.Disabled,
		RemoveLabel: in.RemoveLabel, Label: in.Label, TestID: in.TestID,
		RemoveIcon: Icon(lucide.X, TypeCaption.FontSize, Inherited),
		RemoveStyle: core.Style{Width: core.Pt(chipSize - 6), Height: core.Pt(chipSize - 6), CornerRadius: RadiusFull,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Transition: quick},
		RemoveStyleFunc: func() core.Style {
			return core.Style{Color: colors.Muted(), Hover: &core.Style{Background: colors.Pressed(), Color: colors.Text()}, Focus: FocusRing(cx, 0)}
		},
		TagStyle: TypeBody.Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2, Height: core.Pt(chipSize),
			Padding: core.Edges{Left: 8, Right: 2}, CornerRadius: RadiusSmall, BorderWidth: 1, MinWidth: core.Pt(0), FlexShrink: 1, Cursor: core.CursorDefault}),
		TagStyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.Border(), Background: colors.Raised(), Color: colors.Text()}
		},
		TextStyle: core.Style{FlexShrink: 1},
		InputStyle: TypeBody.Merge(core.Style{FlexGrow: 1, FlexBasis: core.Pt(80), MinWidth: core.Pt(80), Height: core.Pt(chipSize),
			Padding: core.Symmetric(0, 4)}),
		InputStyleFunc: func() core.Style {
			return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Now().Selection, CaretColor: colors.Accent()}
		},
		Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 4, MinHeight: core.Pt(ButtonHeight),
			Padding: core.Symmetric(3, 4), CornerRadius: RadiusControl, BorderWidth: 1, Cursor: core.CursorText, Transition: quick},
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(), Color: colors.Text(),
				Hover:    &core.Style{BorderColor: Mix(colors.FieldBorder(), colors.Text(), 0.25)},
				Focus:    &core.Style{BorderColor: colors.Accent()},
				Disabled: &core.Style{Opacity: core.Alpha(0.5), Background: colors.Inset(), BorderColor: colors.Border()}}.Merge(in.Style)
		}}
}
