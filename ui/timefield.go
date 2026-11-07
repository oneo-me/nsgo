package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TimeField is a time of day in a field 28 pt tall, edited a segment at a
// time: digits, the arrows, and a press on a segment.
type TimeField struct {
	Value        func() base.TimeOfDay
	DefaultValue base.TimeOfDay
	Precision    base.TimePrecision
	HourCycle    base.HourCycle
	Disabled     func() bool
	OnChange     func(value base.TimeOfDay)
	// Label names it for assistive technology.
	Label  string
	TestID string
	// Style merges over the field's.
	Style core.Style
}

// timeBox is a time field's box and its segments' looks.
func timeBox(cx *core.Scope) (core.Style, func() core.Style, core.Style, func() core.Style) {
	colors := ColorsOf(cx)
	box := core.Style{Position: core.Relative, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, Height: core.Pt(ButtonHeight),
		Padding: core.Symmetric(0, 6), BorderWidth: 1, CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous, Disabled: Dimmed()}
	segment := TypeBody.Merge(core.Style{Padding: core.Symmetric(1, 3), CornerRadius: 4})
	return box, func() core.Style {
			return core.Style{Background: colors.Field(), BorderColor: colors.FieldBorder(), FocusWithin: &core.Style{BorderColor: colors.Accent()}}
		}, segment, func() core.Style {
			return core.Style{Color: colors.Text(), Focus: &core.Style{Background: colors.AccentFill(), Color: colors.OnAccent()}}
		}
}

// Render implements core.Node.
func (t TimeField) Render(cx *core.Scope) core.Node {
	box, look, segment, segmentLook := timeBox(cx)
	return base.TimeField{Value: t.Value, DefaultValue: t.DefaultValue, Precision: t.Precision, HourCycle: t.HourCycle, Disabled: t.Disabled,
		OnChange: t.OnChange, Label: t.Label, TestID: t.TestID, Style: box.Merge(t.Style), StyleFunc: look,
		SegmentStyle: segment, SegmentStyleFunc: segmentLook, SeparatorStyle: TypeBody}
}
