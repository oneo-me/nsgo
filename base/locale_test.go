package base_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/i18n"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// i18n.Regional is what the user set, else their region's.
func TestRegionalFollowsTheUser(t *testing.T) {
	var got []i18n.RegionalSettings
	for _, env := range []core.Environment{
		{Locale: "en-US"},
		{Locale: "en-US", LocaleSettings: core.LocaleSettings{FirstDayOfWeek: 1, HourCycle: "h23", MeasurementSystem: "metric"}},
		{Locale: "en-US", LocaleSettings: core.LocaleSettings{Region: "DE"}},
	} {
		nsgotest.Start(t, func(cx *core.Scope) core.Node {
			got = append(got, i18n.Regional(cx))
			return core.View{}
		}, nsgotest.Environment(env))
	}
	want := []i18n.RegionalSettings{
		{FirstDayOfWeek: 7, HourCycle: "h12", MeasurementSystem: "us"},
		{FirstDayOfWeek: 1, HourCycle: "h23", MeasurementSystem: "metric"},
		{FirstDayOfWeek: 1, HourCycle: "h23", MeasurementSystem: "metric"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("environment %d: %+v, want %+v", i, got[i], want[i])
		}
	}
}

// A calendar starts its weeks and a time field keeps its hours as the user
// reads them, unless the app sets them.
func TestCalendarsAndTimesFollowTheUser(t *testing.T) {
	env := core.Environment{Locale: "en-US", LocaleSettings: core.LocaleSettings{FirstDayOfWeek: 1, HourCycle: "h23"}}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			base.Calendar{TestID: "user", DefaultValue: base.DateValue{Start: base.NewDate(2026, 11, 12, base.TimeOfDay{})}},
			base.Calendar{TestID: "sunday", CalendarConfig: base.CalendarConfig{FirstDayOfWeek: 7},
				DefaultValue: base.DateValue{Start: base.NewDate(2026, 11, 12, base.TimeOfDay{})}},
			base.TimeField{Label: "Time", DefaultValue: base.TimeOfDay{Hour: 13, Minute: 5}},
		}}
	}, nsgotest.Environment(env))
	// The first weekday's label: its number, Sunday 0.
	first := func(id string) string {
		return app.Find("id:" + id).Children[1].Children[0].Children[0].Children[0].Children[0].Text
	}
	if u, s := first("user"), first("sunday"); u != "1" || s != "0" {
		t.Errorf("weeks start on %q for the user, %q set to Sunday\n%s", u, s, app.Dump())
	}
	if app.Exists("text:PM") {
		t.Error("the time field shows a day period for a 24-hour user")
	}
}

// A dialog is announced with its role, its title and its description; a
// field the app rejects says so and takes its aria-invalid: style.
func TestDialogsAndFieldsTellScreenReaders(t *testing.T) {
	var bad *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		bad = core.NewSignal(cx, false)
		return core.View{Children: []core.Node{
			core.Input{TestID: "email", Label: "Email", AccessibilityDescription: "Your work address", Invalid: bad.Get,
				Class: "aria-invalid:bg-[#ff0000]", Style: core.Style{Width: core.Pt(100), Height: core.Pt(20)}},
			base.Dialog{Open: func() bool { return true }, Label: "Delete file?", Role: core.RoleAlertDialog,
				AccessibilityDescription: "This cannot be undone.", TestID: "dialog",
				PopupStyle: core.Style{Width: core.Pt(100), Height: core.Pt(100)}},
		}}
	})
	d := app.Find("id:dialog")
	if d.Props.Role != core.RoleAlertDialog || d.Props.Label != "Delete file?" || d.Props.Description != "This cannot be undone." {
		t.Errorf("dialog props %+v", d.Props)
	}
	if p := app.FindMounted("id:email").Props; p.Invalid || p.Description != "Your work address" {
		t.Errorf("field props before rejecting %+v", p)
	}
	bad.Set(true)
	app.Settle()
	if v := app.FindMounted("id:email"); !v.Props.Invalid || v.Style.Background != core.RGB(0xff0000) {
		t.Errorf("a rejected field: invalid %v, background %x", v.Props.Invalid, v.Style.Background)
	}
}
