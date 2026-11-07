package ui

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// DatePicker is a date, a range or with TimePrecision a date and time in a
// field that opens a Calendar: a popup at it on desktops, a sheet on touch
// screens. The field shows the value in DateFormat, strftime's, or
// Placeholder while there is none.
type DatePicker struct {
	Value        func() base.DateValue
	DefaultValue base.DateValue
	Range        bool
	// TimePrecision edits a date's time too, HourCycle its clock.
	TimePrecision base.TimePrecision
	HourCycle     base.HourCycle
	// DefaultTime is a new date's time.
	DefaultTime base.TimeOfDay
	// TimeLabel says what the time field is, in the popup.
	TimeLabel string
	// Presets are values offered beside the calendar.
	Presets        []base.DatePreset
	NumberOfMonths int
	// FirstDayOfWeek starts the weeks, Sunday 0.
	FirstDayOfWeek int
	// DisabledMatcher says which days take no press.
	DisabledMatcher func(day time.Time) bool
	DateFormat      string
	Placeholder     string
	// Cleanable shows a button in the field while a date is chosen.
	Cleanable    bool
	Disabled     func() bool
	OnChange     func(value base.DateValue)
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	// Label names the field for assistive technology.
	Label  string
	TestID string
	// Style merges over the field's, 200 pt wide.
	Style core.Style
}

// Render implements core.Node.
func (d DatePicker) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	own := core.NewSignal(cx, d.DefaultValue)
	value := func() base.DateValue {
		if d.Value != nil {
			return d.Value()
		}
		return own.Get()
	}
	ownOpen := core.NewSignal(cx, false)
	open := d.Open
	if open == nil {
		open = ownOpen.Get
	}
	format := d.DateFormat
	if format == "" {
		format = "%Y-%m-%d"
		if d.TimePrecision == base.TimeSecond {
			format += " %H:%M:%S"
		} else if d.TimePrecision != base.TimeNone {
			format += " %H:%M"
		}
	}
	text := func() string { return value().Format(format) }
	box, look, segment, segmentLook := timeBox(cx)
	popup, backdrop := pickerSurface(cx)
	label := d.Label
	if label == "" {
		label = d.Placeholder
	}
	b := base.DatePicker{Value: value, Range: d.Range, Open: open, Disabled: d.Disabled, TimePrecision: d.TimePrecision, HourCycle: d.HourCycle,
		DefaultTime: d.DefaultTime, Presets: d.Presets, Cleanable: d.Cleanable,
		CalendarConfig: calendarConfig(cx, d.NumberOfMonths, d.FirstDayOfWeek, d.DisabledMatcher),
		CalendarStyle:  calendarStyle(d.NumberOfMonths),
		OnOpenChange: func(v bool, r base.Reason) {
			ownOpen.Set(v)
			if d.OnOpenChange != nil {
				d.OnOpenChange(v, r)
			}
		},
		OnChange: func(v base.DateValue) {
			own.Set(v)
			if d.OnChange != nil {
				d.OnChange(v)
			}
		},
		Offset: 4, Margin: 8, Label: label, TestID: d.TestID, RootStyle: core.Style{Width: core.Pt(200), AlignSelf: core.AlignStart}.Merge(d.Style),
		Style: core.Style{Group: "button", CornerRadius: RadiusControl}, StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Trigger:        pickerField(cx, nil, pickerText(cx, text, d.Placeholder), lucide.Calendar, open),
		PopupStyle:     core.Style{Padding: core.All(12), Gap: 8},
		PopupStyleFunc: popup, BackdropStyle: backdrop, ContentStyle: core.Style{Direction: core.Row, Gap: 12},
		PresetsStyle: core.Style{Gap: 2, MinWidth: core.Pt(96)},
		PresetStyle:  TypeBody.Merge(core.Style{Padding: core.Symmetric(6, 8), CornerRadius: RadiusControl - 2}),
		PresetStyleFunc: func() core.Style {
			return core.Style{Color: colors.Text(), Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}
		},
		TimeStyle: core.Style{Margin: core.Edges{Top: 8}}, TimeFieldStyle: box, TimeFieldStyleFunc: look,
		SegmentStyle: segment, SegmentStyleFunc: segmentLook, SeparatorStyle: TypeBody, TimeFieldLabel: d.TimeLabel}
	if d.TimeLabel != "" {
		b.TimeLabel = core.Text{Content: d.TimeLabel, Style: TypeBody, StyleFunc: Colored(colors.Muted)}
	}
	if d.Cleanable {
		b.ClearButton, b.ClearButtonStyle, b.ClearButtonLabel = clearMark(cx), clearStyle, locales.ListClear(cx)
	}
	return b
}
