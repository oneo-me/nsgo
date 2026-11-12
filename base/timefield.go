package base

import (
	"fmt"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// TimeOfDay is a time of day.
type TimeOfDay struct{ Hour, Minute, Second int }

// timeOf is t's time of day.
func timeOf(t time.Time) TimeOfDay { return TimeOfDay{t.Hour(), t.Minute(), t.Second()} }

// on is day at t.
func (t TimeOfDay) on(day time.Time) time.Time {
	if day.IsZero() {
		return day
	}
	return NewDate(day.Year(), day.Month(), day.Day(), t)
}

// ParseTimeOfDay reads "09:30" or "09:30:15".
func ParseTimeOfDay(text string) (TimeOfDay, bool) {
	for _, layout := range []string{time.TimeOnly, "15:04"} {
		if t, err := time.Parse(layout, text); err == nil {
			return timeOf(t), true
		}
	}
	return TimeOfDay{}, false
}

// String is "09:30", or "09:30:15" with seconds.
func (t TimeOfDay) String() string {
	if t.Second == 0 {
		return t.Format(TimeMinute)
	}
	return t.Format(TimeSecond)
}

// Format is the time at p.
func (t TimeOfDay) Format(p TimePrecision) string {
	if p == TimeSecond {
		return fmt.Sprintf("%02d:%02d:%02d", t.Hour, t.Minute, t.Second)
	}
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

// TimePrecision is how fine a time is: a DatePicker's zero holds none, and
// a TimeField's is TimeMinute.
type TimePrecision uint8

const (
	TimeNone TimePrecision = iota
	TimeMinute
	TimeSecond
)

// truncate drops what p does not hold.
func (p TimePrecision) truncate(t TimeOfDay) TimeOfDay {
	if p != TimeSecond {
		t.Second = 0
	}
	return t
}

// HourCycle is a clock's hours.
type HourCycle uint8

const (
	HourCycle23 HourCycle = iota
	HourCycle12
)

// format is a strftime format of the time.
func (p TimePrecision) format(c HourCycle) string {
	switch {
	case p == TimeSecond && c == HourCycle12:
		return "%I:%M:%S %p"
	case p == TimeSecond:
		return "%H:%M:%S"
	case c == HourCycle12:
		return "%I:%M %p"
	}
	return "%H:%M"
}

// TimeSegment is a part of a TimeField.
type TimeSegment uint8

const (
	SegmentHour TimeSegment = iota
	SegmentMinute
	SegmentSecond
	SegmentPeriod
)

// TimeFieldSegmentEntry is what RenderSegment draws.
type TimeFieldSegmentEntry struct {
	Segment TimeSegment
	Value   int
	Label   string
}

// segmentEditor edits a time a segment at a time, as gpui-base's.
type segmentEditor struct {
	time      TimeOfDay
	precision TimePrecision
	cycle     HourCycle
	segment   TimeSegment
	// pending is the first digit of two, -1 none.
	pending int
}

func newSegmentEditor(t TimeOfDay, p TimePrecision, c HourCycle) segmentEditor {
	return segmentEditor{time: p.truncate(t), precision: p, cycle: c, pending: -1}
}

func (e segmentEditor) segments() []TimeSegment {
	s := []TimeSegment{SegmentHour, SegmentMinute}
	if e.precision == TimeSecond {
		s = append(s, SegmentSecond)
	}
	if e.cycle == HourCycle12 {
		s = append(s, SegmentPeriod)
	}
	return s
}

func (e segmentEditor) bounds(s TimeSegment) (lo, hi int) {
	switch {
	case s == SegmentHour && e.cycle == HourCycle12:
		return 1, 12
	case s == SegmentHour:
		return 0, 23
	case s == SegmentPeriod:
		return 0, 1
	}
	return 0, 59
}

func (e segmentEditor) value(s TimeSegment) int {
	switch s {
	case SegmentHour:
		if e.cycle == HourCycle12 {
			return (e.time.Hour+11)%12 + 1
		}
		return e.time.Hour
	case SegmentMinute:
		return e.time.Minute
	case SegmentSecond:
		return e.time.Second
	}
	return e.time.Hour / 12
}

func (e segmentEditor) label(s TimeSegment) string {
	if s == SegmentPeriod {
		if e.value(s) == 0 {
			return "AM"
		}
		return "PM"
	}
	return fmt.Sprintf("%02d", e.value(s))
}

// text is the time as it shows.
func (e segmentEditor) text() string {
	out := ""
	for i, s := range e.segments() {
		switch {
		case s == SegmentPeriod:
			out += " "
		case i > 0:
			out += ":"
		}
		out += e.label(s)
	}
	return out
}

func (e segmentEditor) withValue(s TimeSegment, v int) TimeOfDay {
	t := e.time
	switch s {
	case SegmentHour:
		if e.cycle == HourCycle12 {
			// 12 AM is midnight and 12 PM is noon.
			t.Hour = v%12 + t.Hour/12*12
		} else {
			t.Hour = v
		}
	case SegmentMinute:
		t.Minute = v
	case SegmentSecond:
		t.Second = v
	default:
		t.Hour = t.Hour%12 + v*12
	}
	return t
}

func (e *segmentEditor) setFormat(p TimePrecision, c HourCycle) {
	e.precision, e.cycle, e.time = p, c, p.truncate(e.time)
	if !slices.Contains(e.segments(), e.segment) {
		e.segment = SegmentHour
	}
	e.pending = -1
}

func (e *segmentEditor) setTime(t TimeOfDay) {
	if t = e.precision.truncate(t); t != e.time {
		e.time, e.pending = t, -1
	}
}

func (e *segmentEditor) selectSegment(s TimeSegment) {
	if slices.Contains(e.segments(), s) {
		e.segment, e.pending = s, -1
	}
}

func (e *segmentEditor) moveSegment(by int) {
	s := e.segments()
	if i := slices.Index(s, e.segment) + by; i >= 0 && i < len(s) {
		e.selectSegment(s[i])
	}
}

func (e *segmentEditor) step(delta int) bool {
	e.pending = -1
	lo, hi := e.bounds(e.segment)
	span := hi - lo + 1
	return e.replace(((e.value(e.segment)-lo+delta)%span+span)%span + lo)
}

func (e *segmentEditor) inputDigit(d int) bool {
	if e.segment == SegmentPeriod {
		return false
	}
	lo, hi := e.bounds(e.segment)
	v, complete := d, d*10 > hi
	if first := e.pending; first >= 0 {
		e.pending = -1
		if n := first*10 + d; n >= lo && n <= hi {
			v, complete = n, true
		}
	}
	if !complete {
		e.pending = d
	}
	changed := e.replace(v)
	if complete {
		e.moveSegment(1)
	}
	return changed
}

func (e *segmentEditor) inputPeriod(pm bool) bool {
	if e.segment != SegmentPeriod {
		return false
	}
	v := 0
	if pm {
		v = 1
	}
	return e.replace(v)
}

func (e *segmentEditor) clearSegment() bool {
	e.pending = -1
	return e.replace(0)
}

func (e *segmentEditor) replace(v int) bool {
	t := e.withValue(e.segment, v)
	changed := t != e.time
	e.time = t
	return changed
}

// timeConfig is what a TimeField and a DatePicker's time take.
type timeConfig struct {
	precision     TimePrecision
	cycle         HourCycle
	disabled      func() bool
	label         string
	segment       part
	separator     part
	renderSegment func(TimeFieldSegmentEntry) core.Node
}

// timeField is one transparent native field over the segments, as an
// OtpInput's: what it gains or loses goes to the selected segment, then it
// holds the time again.
func timeField(cx *core.Scope, c timeConfig, value func() TimeOfDay, changed func(TimeOfDay)) []core.Node {
	if c.precision == TimeNone {
		c.precision = TimeMinute
	}
	editor := core.NewSignal(cx, newSegmentEditor(core.Untracked(cx, value), c.precision, c.cycle))
	text := core.NewSignal(cx, editor.GetUntracked().text())
	var echoes []string
	focused := core.NewSignal(cx, false)
	field := new(core.Ref)
	// A time the app sets shows unless it is the user's own coming back.
	cx.Effect(func() {
		t := value()
		cx.Untrack(func() {
			if otpEchoed(&echoes, t.String()) {
				return
			}
			echoes = nil
			e := editor.GetUntracked()
			e.setTime(t)
			editor.Set(e)
		})
	})
	cx.Effect(func() {
		now := editor.Get().text()
		cx.Untrack(func() { text.Set(now) })
	})
	edit := func(f func(e *segmentEditor) bool) {
		e := editor.GetUntracked()
		did := f(&e)
		editor.Set(e)
		if did {
			if len(echoes) == otpEchoes {
				echoes = echoes[1:]
			}
			echoes = append(echoes, e.time.String())
			changed(e.time)
		}
	}
	move := func(by int) func() {
		return func() {
			e := editor.GetUntracked()
			e.moveSegment(by)
			editor.Set(e)
		}
	}
	typed := func(typed string) {
		deleted, inserted := otpDiff(text.GetUntracked(), typed)
		if inserted == "" && deleted > 0 {
			edit((*segmentEditor).clearSegment)
		}
		for _, r := range inserted {
			if d, ok := otpDigit(r); ok {
				edit(func(e *segmentEditor) bool { return e.inputDigit(int(d - '0')) })
			} else if r == 'a' || r == 'A' || r == 'p' || r == 'P' {
				pm := r == 'p' || r == 'P'
				edit(func(e *segmentEditor) bool { return e.inputPeriod(pm) })
			}
		}
		// Back to the time as it shows, through the typed text first, so
		// the field hears it even when the time is the same.
		now := editor.GetUntracked().text()
		text.Set(typed)
		cx.Post(func() {
			text.Set(now)
			field.SetSelection(utf16Len(now), utf16Len(now))
		})
	}
	enabled := func() bool { return focused.Get() && !is(c.disabled) }
	keys := bindWhile(enabled, core.KeyAlways, false,
		controlKey{"up", true, func() { edit(func(e *segmentEditor) bool { return e.step(1) }) }},
		controlKey{"down", true, func() { edit(func(e *segmentEditor) bool { return e.step(-1) }) }},
		controlKey{"left", true, move(-1)}, controlKey{"right", true, move(1)})
	// Tab walks the segments and leaves the field from the last one, Shift
	// with it from the first: unbound there.
	atEnd := func(last bool) func() bool {
		return func() bool {
			e, s := editor.Get(), editor.Get().segments()
			i := 0
			if last {
				i = len(s) - 1
			}
			return enabled() && e.segment != s[i]
		}
	}
	tab := bindWhile(atEnd(true), core.KeyAlways, false, controlKey{"tab", false, move(1)})
	shiftTab := bindWhile(atEnd(false), core.KeyAlways, false, controlKey{"shift+tab", false, move(-1)})
	native := core.Input{Ref: field, Label: c.label, Disabled: c.disabled, KeyboardType: core.KeyboardNumber, NoAutoCorrect: true,
		AutoCapitalize: core.AutoCapitalizeNone, HideCaret: true,
		Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
			Padding: core.All(0), BorderWidth: 0, Background: core.Transparent, Color: core.Transparent,
			SelectionColor: core.Transparent},
		Value: text.Get, OnChange: typed,
		OnSelectionChange: func(r core.TextRange) {
			if end := utf16Len(text.GetUntracked()); r.Start != end || r.End != end {
				field.SetSelection(end, end)
			}
		},
		OnFocus: func() { focused.Set(true) }, OnBlur: func() { focused.Set(false) }}
	nodes := []core.Node{native, keys, tab, shiftTab}
	segments := editor.GetUntracked().segments()
	disabledPart := func() core.PartStates { return core.PartStates{Disabled: is(c.disabled)} }
	for i, s := range segments {
		if i > 0 && s != SegmentPeriod {
			nodes = append(nodes, core.View{AccessibilityElementsHidden: func() bool { return true }, NoFocus: true,
				PartStates: disabledPart, Class: c.separator.Class, Style: c.separator.Style,
				Children: []core.Node{core.Text{Content: ":"}}})
		}
		nodes = append(nodes, core.Func(func(cx *core.Scope) core.Node {
			return segmentView(cx, c, editor, focused, field, s, edit)
		}))
	}
	return nodes
}

func segmentView(cx *core.Scope, c timeConfig, editor *core.Signal[segmentEditor], focused *core.Signal[bool], field *core.Ref,
	s TimeSegment, edit func(func(*segmentEditor) bool)) core.Node {
	entry := core.NewMemo(cx, func() TimeFieldSegmentEntry {
		e := editor.Get()
		return TimeFieldSegmentEntry{Segment: s, Value: e.value(s), Label: e.label(s)}
	})
	selected := core.NewMemo(cx, func() bool { return focused.Get() && editor.Get().segment == s })
	var child core.Node = core.Text{ContentFunc: func() string { return entry.Get().Label }}
	if c.renderSegment != nil {
		child = core.Dynamic(func(*core.Scope) core.Node { return c.renderSegment(entry.Get()) })
	}
	return core.View{AccessibilityElementsHidden: func() bool { return true }, NoFocus: true,
		PartStates: func() core.PartStates { return core.PartStates{Disabled: is(c.disabled), Focused: selected.Get()} },
		Class:      c.segment.Class, Style: c.segment.Style, StyleFunc: c.segment.Fn,
		OnPress: func(core.PressEvent) {
			if core.Untracked(cx, func() bool { return is(c.disabled) }) {
				return
			}
			if selected.GetUntracked() && s == SegmentPeriod {
				edit(func(e *segmentEditor) bool { return e.inputPeriod(e.value(SegmentPeriod) == 0) })
			} else {
				e := editor.GetUntracked()
				e.selectSegment(s)
				editor.Set(e)
			}
			field.Focus()
		},
		Children: []core.Node{child}}
}

// TimeField is a time of day edited a segment at a time: the hour, the
// minute, with TimeSecond the second, with HourCycle12 AM or PM. The keys
// edit the selected segment and move between them, and a press selects
// one. The app lays the field out and styles its parts, or draws them.
type TimeField struct {
	Value        func() TimeOfDay
	DefaultValue TimeOfDay
	Precision    TimePrecision
	HourCycle    HourCycle
	Disabled     func() bool
	Label        string

	Class        string
	Style        core.Style
	StyleFunc    func() core.Style
	SegmentClass string
	SegmentStyle core.Style
	// SegmentStyleFunc is each segment's style following what it reads.
	SegmentStyleFunc func() core.Style
	SeparatorClass   string
	SeparatorStyle   core.Style
	RenderSegment    func(TimeFieldSegmentEntry) core.Node

	OnChange func(value TimeOfDay)
	TestID   string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t TimeField) Render(cx *core.Scope) core.Node {
	own := core.NewSignal(cx, t.DefaultValue)
	value := func() TimeOfDay {
		if t.Value != nil {
			return t.Value()
		}
		return own.Get()
	}
	changed := func(v TimeOfDay) {
		if t.Value == nil {
			own.Set(v)
		}
		if t.OnChange != nil {
			t.OnChange(v)
		}
	}
	c := timeConfig{precision: t.Precision, cycle: t.HourCycle, disabled: t.Disabled, label: t.Label,
		segment:   parts.Of(t.SegmentClass, t.SegmentStyle, t.SegmentStyleFunc),
		separator: part{Class: t.SeparatorClass, Style: t.SeparatorStyle}, renderSegment: t.RenderSegment}
	return core.View{Class: t.Class, Style: calendarRow.Merge(t.Style), StyleFunc: t.StyleFunc, Disabled: t.Disabled, TestID: t.TestID,
		OnLayout: t.OnLayout, Tooltip: t.Tooltip, Children: timeField(cx, c, value, changed)}
}
