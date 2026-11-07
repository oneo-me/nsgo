package ui

import (
	"strconv"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Calendar is a month of days to pick a date from, or with Range a start
// and an end, in Onevra's fixed-size look: a selected day in the accent,
// today ringed, the days around the month muted. Its title toggles jump to
// months and years, and its days are one Tab stop whose keys move through
// them, turning the months.
type Calendar struct {
	Value          func() base.DateValue
	DefaultValue   base.DateValue
	Range          bool
	NumberOfMonths int
	// FirstDayOfWeek starts the weeks, Sunday 0.
	FirstDayOfWeek int
	// DisabledMatcher says which days take no press.
	DisabledMatcher func(day time.Time) bool
	OnChange        func(value base.DateValue)
	TestID          string
	// Style merges over the calendar's.
	Style core.Style
}

const (
	// calendarWidth is a month's width: seven days 32 points wide.
	calendarWidth core.Points = 224
	// calendarBodyHeight is the weekdays' 24 points and six weeks of 28,
	// which the months and the years fill too.
	calendarBodyHeight core.Points = 192
	calendarGap        core.Points = 16
)

// calendarStyle is the width of a calendar of months, whatever it shows.
func calendarStyle(months int) core.Style {
	n := core.Points(max(months, 1))
	return core.Style{Width: core.Pt(n*calendarWidth + (n-1)*calendarGap)}
}

// calendarConfig is a calendar's parts in Onevra's look.
func calendarConfig(cx *core.Scope, months, firstDay int, disabled func(time.Time) bool) base.CalendarConfig {
	colors := ColorsOf(cx)
	cell := core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: RadiusControl - 2, Disabled: Dimmed()}
	strong := func() core.Style {
		return core.Style{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}, Focus: FocusRing(cx, -1)}
	}
	return base.CalendarConfig{NumberOfMonths: months, FirstDayOfWeek: firstDay, DisabledMatcher: disabled,
		PreviousLabel: locales.CalendarPrevious(cx), NextLabel: locales.CalendarNext(cx),
		HeaderStyle: core.Style{Height: core.Pt(32), Margin: core.Edges{Bottom: 8}},
		BodyStyle:   core.Style{Height: core.Pt(calendarBodyHeight), Gap: calendarGap},
		ItemStyle:   cell,
		ItemStyleOf: func(kind base.CalendarItemKind) core.Style {
			switch kind {
			case base.CalendarWeekday:
				return core.Style{MinHeight: core.Pt(16), Margin: core.Edges{Bottom: 8}, Disabled: &core.Style{}}
			case base.CalendarPrevious, base.CalendarNext:
				return strong().Merge(core.Style{Width: core.Pt(28), Height: core.Pt(32)})
			case base.CalendarMonthToggle, base.CalendarYearToggle:
				return strong().Merge(core.Style{Height: core.Pt(32), Padding: core.Symmetric(0, 6)})
			case base.CalendarDay:
				return strong().Merge(core.Style{MinHeight: core.Pt(28), BorderWidth: 1, BorderColor: core.Transparent, Color: colors.Text(),
					Selected: &core.Style{Background: colors.AccentFill(), Color: colors.OnAccent()},
					Data: []core.DataStyle{{Name: "outside", Style: &core.Style{Color: colors.Faint()}},
						{Name: "range-middle", Style: &core.Style{Background: Alpha(colors.Accent(), 0.16), Color: colors.Text()}},
						{Name: "today", Style: &core.Style{BorderColor: colors.Accent()}}}})
			}
			return strong().Merge(core.Style{MinHeight: core.Pt(36), Color: colors.Text(),
				Selected: &core.Style{Background: colors.AccentFill(), Color: colors.OnAccent()}})
		},
		RenderItem: func(e base.CalendarEntry) core.Node { return calendarItem(cx, e, months > 1) }}
}

// calendarItem is what an item of a calendar says.
func calendarItem(cx *core.Scope, e base.CalendarEntry, titled bool) core.Node {
	colors := ColorsOf(cx)
	text := func(s string, style core.Style, color func() core.Color) core.Node {
		return core.Text{Content: s, MaxLines: 1, Style: TypeBody.Merge(style), StyleFunc: Colored(color)}
	}
	day := func(month int) time.Time {
		return time.Date(e.Year, time.Month(max(month, 1)), 1, 0, 0, 0, 0, time.UTC)
	}
	switch e.Kind {
	case base.CalendarPrevious:
		return Icon(lucide.ChevronLeft, iconSize, colors.Muted)
	case base.CalendarNext:
		return Icon(lucide.ChevronRight, iconSize, colors.Muted)
	case base.CalendarMonthToggle:
		if titled {
			return text(locales.CalendarMonth(cx, day(e.Month)), core.Style{FontWeight: Medium}, colors.Text)
		}
		return text(locales.CalendarMonthName(cx, day(e.Month)), core.Style{FontWeight: Medium}, colors.Text)
	case base.CalendarYearToggle:
		return text(strconv.Itoa(e.Value), core.Style{FontWeight: Medium}, colors.Text)
	case base.CalendarWeekday:
		// 2023-01-01 is a Sunday.
		return text(locales.CalendarWeekday(cx, time.Date(2023, 1, 1+e.Value, 0, 0, 0, 0, time.UTC)), TypeCaption, colors.Muted)
	case base.CalendarMonth:
		return core.Text{Content: locales.CalendarMonthName(cx, day(e.Value)), Style: TypeBody}
	case base.CalendarYear:
		return core.Text{Content: strconv.Itoa(e.Value), Style: TypeBody}
	}
	// Its color is the item's, which follows the selection.
	return core.Text{Content: strconv.Itoa(e.Value), Style: TypeBody}
}

// Render implements core.Node.
func (c Calendar) Render(cx *core.Scope) core.Node {
	return base.Calendar{Value: c.Value, DefaultValue: c.DefaultValue, Range: c.Range, OnChange: c.OnChange, TestID: c.TestID,
		CalendarConfig: calendarConfig(cx, c.NumberOfMonths, c.FirstDayOfWeek, c.DisabledMatcher),
		Style:          core.Style{AlignSelf: core.AlignStart}.Merge(calendarStyle(c.NumberOfMonths)).Merge(c.Style)}
}
