//go:build windows

package windows

import "testing"

// The regional format with the user's changes reads as Intl names it.
func TestLocaleSettingsReadTheUsers(t *testing.T) {
	s := localeSettings()
	if s.FirstDayOfWeek < 1 || s.FirstDayOfWeek > 7 {
		t.Errorf("first day of the week %d", s.FirstDayOfWeek)
	}
	if s.HourCycle != "h12" && s.HourCycle != "h23" {
		t.Errorf("hour cycle %q", s.HourCycle)
	}
	if s.MeasurementSystem != "metric" && s.MeasurementSystem != "us" {
		t.Errorf("measurement system %q", s.MeasurementSystem)
	}
	t.Logf("%+v", s)
}
