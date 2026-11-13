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

// A preferred language takes the user's region in place of its own.
func TestLocaleTagTakesTheUsersRegion(t *testing.T) {
	for _, c := range []struct{ lang, region, want string }{
		{"zh-Hans-CN", "CN", "zh-Hans-CN"},
		{"en-US", "CN", "en-CN"},
		{"en", "GB", "en-GB"},
		{"es-419", "", "es-419"},
		{"de", "", "de"},
	} {
		if got := localeTag(c.lang, c.region); got != c.want {
			t.Errorf("localeTag(%q, %q) = %q, want %q", c.lang, c.region, got, c.want)
		}
	}
}
