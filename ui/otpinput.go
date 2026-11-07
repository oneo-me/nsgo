package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// OtpInput is a one-time code in bordered cells, the one that takes the
// next digit ringed in the accent: base.OtpInput's one native field over
// them, with the system's code autofill.
type OtpInput struct {
	Length int
	// Value makes the code controlled; DefaultValue starts its own.
	Value        func() string
	DefaultValue string
	Masked       bool
	Disabled     func() bool
	// Groups is how many groups the cells split into.
	Groups               int
	OnChange, OnComplete func(value string)
	// Label is what assistive technology says for the field.
	Label  string
	Ref    *base.OtpInputRef
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (o OtpInput) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	return base.OtpInput{Length: o.Length, Value: o.Value, DefaultValue: o.DefaultValue, Masked: o.Masked, Disabled: o.Disabled, Groups: o.Groups,
		OnChange: o.OnChange, OnComplete: o.OnComplete, Label: o.Label, Ref: o.Ref, TestID: o.TestID,
		Style:      core.Style{Direction: core.Row, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, Gap: 12}.Merge(o.Style),
		GroupStyle: core.Style{Direction: core.Row, Gap: 6},
		CellStyle: TypeBody.Merge(core.Style{Width: core.Pt(ButtonHeight + 8), Height: core.Pt(ButtonHeight + 8), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: RadiusControl, BorderWidth: 1, Transition: quick, Disabled: Dimmed()}),
		CellStyleFunc: func() core.Style {
			return core.Style{Color: colors.Text(), Background: colors.Field(), BorderColor: colors.FieldBorder(),
				Focus: &core.Style{BorderColor: colors.Accent()}}
		},
		CaretStyle:     core.Style{Width: core.Pt(1.5), Height: core.Pt(16)},
		CaretStyleFunc: Filled(colors.Accent)}
}
