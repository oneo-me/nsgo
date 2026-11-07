package base

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time { return NewDate(y, m, d, TimeOfDay{}) }

func TestRangesStartEndAndStartAgain(t *testing.T) {
	a, b := date(2025, 2, 10), date(2025, 2, 12)
	v := DateValue{Range: true}.picked(a)
	if v != (DateValue{Range: true, Start: a}) {
		t.Fatalf("started: %+v", v)
	}
	v = v.picked(b)
	if v != (DateValue{Range: true, Start: a, End: b}) || !v.Complete() {
		t.Fatalf("ended: %+v", v)
	}
	if v.picked(a) != (DateValue{Range: true, Start: a}) {
		t.Fatal("a complete range does not start again")
	}
	if (DateValue{Range: true, Start: b}).picked(a) != (DateValue{Range: true, Start: a}) {
		t.Fatal("a day before the start does not start there")
	}
}

func TestMonthsShowTheirWeeks(t *testing.T) {
	august := date(2025, 8, 1)
	if got := weekCount(august, 0); got != 6 {
		t.Errorf("from Sunday: %d weeks", got)
	}
	if got := firstShown(august, 0); got != date(2025, 7, 27) {
		t.Errorf("first shown: %v", got)
	}
	if got := weekCount(august, 1); got != 5 {
		t.Errorf("from Monday: %d weeks", got)
	}
	if got := firstShown(august, 1); got != date(2025, 7, 28) {
		t.Errorf("first shown: %v", got)
	}
}

func TestMatchers(t *testing.T) {
	sunday := date(2026, 10, 25)
	if !DaysOfWeek(0, 6)(sunday) || DaysOfWeek(6)(sunday) {
		t.Error("DaysOfWeek")
	}
	i := DatesOutside(date(2026, 10, 10), date(2026, 10, 20))
	if !i(date(2026, 10, 9)) || i(date(2026, 10, 10)) || !i(date(2026, 10, 21)) {
		t.Error("DatesOutside")
	}
	r := DatesBetween(date(2026, 10, 10), time.Time{})
	if r(date(2026, 10, 9)) || !r(date(2030, 1, 1)) {
		t.Error("DatesBetween")
	}
}

func TestAddMonthsStaysInShortMonths(t *testing.T) {
	if got := addMonths(date(2026, 1, 31), 1); got != date(2026, 2, 28) {
		t.Errorf("got %v", got)
	}
	if got := addMonths(date(2026, 3, 31), -12); got != date(2025, 3, 31) {
		t.Errorf("got %v", got)
	}
}

func TestStrftime(t *testing.T) {
	at := NewDate(2026, 10, 9, TimeOfDay{Hour: 15, Minute: 4, Second: 5})
	for format, want := range map[string]string{
		"%d.%m.%Y": "09.10.2026", "%Y/%m/%d %I:%M %p": "2026/10/09 03:04 PM", "%H:%M:%S": "15:04:05", "%y %j %%": "26 282 %",
		"%a %b": "Fri Oct",
	} {
		if got := strftime(at, format); got != want {
			t.Errorf("%q = %q, want %q", format, got, want)
		}
	}
}

func TestSegmentsEditATime(t *testing.T) {
	e := newSegmentEditor(TimeOfDay{Hour: 9, Minute: 30}, TimeMinute, HourCycle12)
	if got := e.text(); got != "09:30 AM" {
		t.Fatalf("text = %q", got)
	}
	// A 2 fits no second digit of an hour and moves on.
	e.inputDigit(2)
	if e.time.Hour != 2 || e.segment != SegmentMinute {
		t.Fatalf("after 2: %+v", e)
	}
	e.inputDigit(4)
	e.inputDigit(5)
	if e.time.Minute != 45 || e.segment != SegmentPeriod {
		t.Fatalf("after 45: %+v", e)
	}
	e.inputPeriod(true)
	if e.time.Hour != 14 {
		t.Fatalf("PM: %+v", e)
	}
	// 12 AM is midnight and 12 PM noon.
	e.selectSegment(SegmentHour)
	e.inputDigit(1)
	e.inputDigit(2)
	if e.time.Hour != 12 {
		t.Fatalf("12 PM: %+v", e)
	}
	// Steps go around.
	m := newSegmentEditor(TimeOfDay{Minute: 59}, TimeMinute, HourCycle23)
	m.selectSegment(SegmentMinute)
	m.step(1)
	if m.time.Minute != 0 {
		t.Fatalf("step past 59: %+v", m)
	}
}
