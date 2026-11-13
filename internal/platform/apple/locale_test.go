//go:build darwin && !ios

package apple

import "testing"

// The system's regional settings read as Intl names them.
func TestLocaleSettingsReadTheSystems(t *testing.T) {
	s := LocaleSettings()
	if s.FirstDayOfWeek < 1 || s.FirstDayOfWeek > 7 {
		t.Errorf("first day of the week %d", s.FirstDayOfWeek)
	}
	switch s.HourCycle {
	case "h11", "h12", "h23", "h24":
	default:
		t.Errorf("hour cycle %q", s.HourCycle)
	}
	switch s.MeasurementSystem {
	case "metric", "us", "uk":
	default:
		t.Errorf("measurement system %q", s.MeasurementSystem)
	}
}
