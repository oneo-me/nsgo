//go:build linux && !android

package linux

import "testing"

// The session's POSIX categories give the region of its formats and its
// measurement system, apart from the language.
func TestLocaleSettingsFollowThePOSIXCategories(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_TIME", "de_DE.UTF-8")
	t.Setenv("LC_MEASUREMENT", "en_US.UTF-8")
	s := localeSettings()
	if s.Region != "DE" || s.MeasurementSystem != "us" {
		t.Errorf("settings %+v, want DE's formats and U.S. measures", s)
	}
	t.Setenv("LC_ALL", "en_GB.UTF-8@euro")
	if s := localeSettings(); s.Region != "GB" || s.MeasurementSystem != "uk" {
		t.Errorf("LC_ALL over the rest: %+v", s)
	}
	switch s.HourCycle {
	case "", "h12", "h23":
	default:
		t.Errorf("GNOME's clock read as %q", s.HourCycle)
	}
}
