package base

import (
	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// DatePreset is a labelled value a DatePicker offers beside its calendar.
type DatePreset struct {
	Label string
	Value DateValue
}

// DatePicker is a date, a range, or with TimePrecision a date and time
// picked in a popup at its trigger, a sheet on touch screens. A complete
// date is the value and closes it. With a time the popup stays on desktops,
// each edit reported, until the selected day is pressed again; a tap on a
// touch screen picks the day at its time and closes. Return opens and
// closes it from the focused trigger, Delete and Backspace clear.
type DatePicker struct {
	Value        func() DateValue
	DefaultValue DateValue
	Range        bool
	Open         func() bool
	DefaultOpen  bool
	Disabled     func() bool
	// DateFormat is strftime's, for what assistive technology hears.
	DateFormat    string
	TimePrecision TimePrecision
	HourCycle     HourCycle
	DefaultTime   TimeOfDay
	Cleanable     bool
	Presets       []DatePreset
	CalendarConfig

	Placement core.Placement
	// Align lines the popup up with its trigger, at its start when 0.
	Align          core.Align
	Offset, Margin core.Points

	Class     string // the trigger's
	Style     core.Style
	StyleFunc func() core.Style
	// RootClass and RootStyle are the view around the trigger and its
	// clear button.
	RootClass      string
	RootStyle      core.Style
	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	BackdropClass  string
	BackdropStyle  core.Style
	ContentClass   string
	ContentStyle   core.Style
	PresetsClass   string
	PresetsStyle   core.Style
	PresetClass    string
	PresetStyle    core.Style
	// PresetStyleFunc is each preset's style following what it reads.
	PresetStyleFunc func() core.Style
	CalendarClass   string
	CalendarStyle   core.Style
	TimeClass       string
	TimeStyle       core.Style
	TimeLabel       core.Node
	TimeFieldClass  string
	TimeFieldStyle  core.Style
	// TimeFieldStyleFunc is the time field's style following what it
	// reads, and SegmentStyleFunc each segment's.
	TimeFieldStyleFunc func() core.Style
	SegmentStyleFunc   func() core.Style
	TimeFieldLabel     string
	SegmentClass       string
	SegmentStyle       core.Style
	SeparatorClass     string
	SeparatorStyle     core.Style
	RenderSegment      func(TimeFieldSegmentEntry) core.Node
	ClearButtonClass   string
	ClearButtonStyle   core.Style
	ClearButton        core.Node
	ClearButtonLabel   string
	Trigger            core.Node

	OnChange     func(value DateValue)
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	OnCancel     func()
	Label        string
	TestID       string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// displayFormat is what assistive technology hears without a DateFormat.
func displayFormat(p TimePrecision, c HourCycle) string {
	if p == TimeNone {
		return "%Y/%m/%d"
	}
	return "%Y/%m/%d " + p.format(c)
}

// editedTimes are the times the user left at, for the value they were left
// on: they show until it changes.
type editedTimes struct {
	at         DateValue
	start, end TimeOfDay
}

// Render implements core.Node.
func (d DatePicker) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, d.Open, d.DefaultOpen, nil, nil, nil)
	own := core.NewSignal(cx, d.DefaultValue)
	current := core.NewMemo(cx, func() DateValue {
		if d.Value != nil {
			return d.Value()
		}
		return own.Get()
	})
	edited := core.NewSignal[*editedTimes](cx, nil)
	times := core.NewMemo(cx, func() [2]TimeOfDay {
		v := current.Get()
		if e := edited.Get(); e != nil && e.at == v {
			return [2]TimeOfDay{e.start, e.end}
		}
		start := d.DefaultTime
		if v.IsSet() {
			start = timeOf(v.Start)
		}
		end := start
		if !v.End.IsZero() {
			end = timeOf(v.End)
		}
		return [2]TimeOfDay{start, end}
	})
	setTimes := func(start, end TimeOfDay) {
		edited.Set(&editedTimes{at: current.GetUntracked(), start: start, end: end})
	}
	date := core.NewMemo(cx, func() DateValue { return current.Get().dateOnly().shaped(d.Range) })
	editsTime := d.TimePrecision != TimeNone && !d.Range
	anchor := new(core.Ref)
	width := core.NewSignal[core.Points](cx, -1)
	focused := core.NewSignal(cx, false)
	closedBy := Reason(0)
	disabled := func() bool { return is(d.Disabled) }
	change := func(next bool, r Reason) {
		if next && core.Untracked(cx, disabled) || core.Untracked(cx, st.open) == next {
			return
		}
		if !next {
			closedBy = r
		}
		if d.OnOpenChange != nil {
			d.OnOpenChange(next, r)
		}
		if st.own != nil {
			st.own.Set(next)
		}
	}
	report := func(next DateValue) {
		if d.Value == nil {
			own.Set(next)
		}
		if d.OnChange != nil {
			d.OnChange(next)
		}
	}
	withTimes := func(v DateValue) DateValue {
		t := times.GetUntracked()
		return DateValue{Range: v.Range, Start: t[0].on(v.Start), End: t[1].on(v.End)}
	}
	clear := func() {
		setTimes(d.DefaultTime, d.DefaultTime)
		report(DateValue{Range: d.Range})
		change(false, ReasonSelect)
	}
	toggle := func() { change(!core.Untracked(cx, st.open), ReasonTriggerPress) }
	clearClosed := func() {
		if !core.Untracked(cx, st.open) {
			clear()
		}
	}
	keys := bindWhile(func() bool { return focused.Get() && !disabled() }, core.KeyNotEditing, false,
		controlKey{key: "enter", run: toggle}, controlKey{key: "delete", run: clearClosed}, controlKey{key: "backspace", run: clearClosed})
	content := func(onSheet bool) []core.Node {
		state := contentState{date: date, times: times, editsTime: editsTime, change: change, report: report, withTimes: withTimes, setTimes: setTimes}
		return []core.Node{core.Func(func(cx *core.Scope) core.Node { return d.content(cx, onSheet, state) })}
	}
	popup := dropdown{anchor: anchor, onContent: true, width: width, fit: true,
		frame: dropdownFrame{placement: d.Placement, align: cmpAlign(d.Align), offset: d.Offset, margin: d.Margin,
			popup:    parts.Of(d.PopupClass, d.PopupStyle, d.PopupStyleFunc),
			backdrop: parts.Part{Class: d.BackdropClass, Style: d.BackdropStyle}},
		open: st.open, content: content,
		dismiss: func(r Reason) {
			if d.OnCancel != nil {
				d.OnCancel()
			}
			change(false, r)
		},
		shown: func(shown bool) {
			if shown {
				if d.OnOpen != nil {
					d.OnOpen()
				}
				return
			}
			if d.OnClose != nil {
				d.OnClose()
			}
			if core.Untracked(cx, cx.Env().Desktop) && closedBy != ReasonPressOutside {
				anchor.Focus()
			}
			closedBy = 0
		}}
	trigger := core.View{Ref: anchor, Class: d.Class, Style: d.Style, StyleFunc: d.StyleFunc, Label: d.Label, TestID: d.TestID, Role: core.RoleCombobox,
		Disabled: d.Disabled, Focusable: func() bool { return !disabled() }, Expanded: st.open, ShowsExpanded: true,
		AccessibilityValue: func() core.AccessibilityValue {
			format := d.DateFormat
			if format == "" {
				format = displayFormat(d.TimePrecision, d.HourCycle)
			}
			v, t := date.Get(), times.Get()
			return core.AccessibilityValue{Text: DateValue{Range: v.Range, Start: t[0].on(v.Start), End: t[1].on(v.End)}.Format(format)}
		},
		OnLayout: func(core.Bounds) {
			if b, ok := anchor.ContentBounds(); ok {
				width.Set(b.Size.Width)
			}
		},
		OnFocus: func() { focused.Set(true) }, OnBlur: func() { focused.Set(false) },
		OnPress:  func(core.PressEvent) { toggle() },
		Children: []core.Node{d.Trigger}}
	clearButton := core.Show{When: func() bool { return d.Cleanable && date.Get().IsSet() }, Then: func(*core.Scope) core.Node {
		return core.View{Role: core.RoleButton, Label: d.ClearButtonLabel, NoFocus: true, Disabled: d.Disabled,
			Class: d.ClearButtonClass, Style: d.ClearButtonStyle,
			OnPress:  func(core.PressEvent) { clear() },
			Children: []core.Node{d.ClearButton}}
	}}
	return core.View{Class: d.RootClass, Style: d.RootStyle, OnLayout: d.OnLayout, Tooltip: d.Tooltip,
		Children: []core.Node{trigger, clearButton, keys, popup.node(cx)}}
}

// contentState is what the popup's content reads of its picker.
type contentState struct {
	date      *core.Memo[DateValue]
	times     *core.Memo[[2]TimeOfDay]
	editsTime bool
	change    func(bool, Reason)
	report    func(DateValue)
	withTimes func(DateValue) DateValue
	setTimes  func(start, end TimeOfDay)
}

// content is the presets, the calendar and the time.
func (d DatePicker) content(cx *core.Scope, onSheet bool, s contentState) core.Node {
	g := newCalendarGrid(cx, d.CalendarConfig, s.date.Get)
	if !onSheet {
		g.focusDays()
	}
	picked := func(v DateValue) {
		// On desktops the popup stays for the time, a press on the selected
		// day again confirming it. A touch has no double click: a tap picks
		// the day at its time and closes the sheet.
		if s.editsTime && !onSheet {
			if v == s.date.GetUntracked() {
				s.change(false, ReasonSelect)
				return
			}
			s.report(s.withTimes(v))
			return
		}
		s.report(s.withTimes(v))
		s.change(false, ReasonSelect)
	}
	var children []core.Node
	if len(d.Presets) > 0 {
		buttons := make([]core.Node, len(d.Presets))
		for i, p := range d.Presets {
			buttons[i] = core.View{Role: core.RoleButton, Label: p.Label, NoFocus: true, Class: d.PresetClass, Style: d.PresetStyle, StyleFunc: d.PresetStyleFunc,
				OnPress: func(core.PressEvent) {
					start, end := d.DefaultTime, d.DefaultTime
					if p.Value.IsSet() {
						start = timeOf(p.Value.Start)
					}
					if !p.Value.End.IsZero() {
						end = timeOf(p.Value.End)
					}
					s.setTimes(start, end)
					s.report(p.Value)
					s.change(false, ReasonSelect)
				},
				Children: []core.Node{core.Text{Content: p.Label}}}
		}
		children = append(children, core.View{Class: d.PresetsClass, Style: d.PresetsStyle, Children: buttons})
	}
	column := []core.Node{core.View{Class: d.CalendarClass, Style: d.CalendarStyle, Children: g.nodes(picked)}}
	if s.editsTime {
		changed := func(t TimeOfDay) {
			s.setTimes(t, s.times.GetUntracked()[1])
			if v := s.date.GetUntracked(); v.Complete() {
				s.report(s.withTimes(v))
			}
		}
		c := timeConfig{precision: d.TimePrecision, cycle: d.HourCycle, disabled: d.Disabled, label: d.TimeFieldLabel,
			segment:   parts.Of(d.SegmentClass, d.SegmentStyle, d.SegmentStyleFunc),
			separator: part{Class: d.SeparatorClass, Style: d.SeparatorStyle}, renderSegment: d.RenderSegment}
		var row []core.Node
		if d.TimeLabel != nil {
			row = append(row, d.TimeLabel)
		}
		row = append(row, core.View{Class: d.TimeFieldClass, Style: calendarRow.Merge(d.TimeFieldStyle), StyleFunc: d.TimeFieldStyleFunc, Disabled: d.Disabled,
			Children: timeField(cx, c, func() TimeOfDay { return s.times.Get()[0] }, changed)})
		column = append(column, core.View{Class: d.TimeClass, Children: row,
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween}.Merge(d.TimeStyle)})
	}
	children = append(children, core.View{Children: column})
	return core.View{Class: d.ContentClass, Style: d.ContentStyle, Children: children}
}
