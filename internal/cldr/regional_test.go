package cldr

import (
	"testing"
	"time"
)

func TestRegionalOf(t *testing.T) {
	for _, c := range []struct {
		locale string
		want   Regional
	}{
		{"en-US", Regional{7, "h12", "us"}},
		{"en-GB", Regional{1, "h23", "uk"}},
		{"de-DE", Regional{1, "h23", "metric"}},
		{"ar-EG", Regional{6, "h12", "metric"}},
		{"en-US-u-fw-mon-hc-h23-ms-metric", Regional{1, "h23", "metric"}},
		{"zh", Regional{7, "h23", "metric"}},
	} {
		if got := RegionalOf(c.locale); got != c.want {
			t.Errorf("RegionalOf(%q) = %+v, want %+v", c.locale, got, c.want)
		}
	}
}

// A -u-hc- in the locale sets the hours of styles and skeletons.
func TestFormatDateTakesTheHourCycle(t *testing.T) {
	at := time.Date(2026, 1, 5, 13, 7, 0, 0, time.UTC)
	for _, c := range []struct {
		locale string
		o      DateOptions
		want   string
	}{
		{"en-US", DateOptions{Time: ShortStyle}, "1:07 PM"},
		{"en-US-u-hc-h23", DateOptions{Time: ShortStyle}, "13:07"},
		{"en-US-u-hc-h23", DateOptions{Skeleton: "jm"}, "13:07"},
		{"de-DE-u-hc-h12", DateOptions{Time: ShortStyle}, "1:07 PM"},
		{"de-DE-u-hc-h12", DateOptions{Skeleton: "jm"}, "1:07 PM"},
	} {
		if got := FormatDate(c.locale, at, c.o); got != c.want {
			t.Errorf("FormatDate(%q, %+v) = %q, want %q", c.locale, c.o, got, c.want)
		}
	}
}

// The day period goes where the language writes it: before the hours in
// Chinese and Korean, after them elsewhere, and leaves with its spaces.
func TestHourCyclesAcrossLanguages(t *testing.T) {
	at := time.Date(2026, 1, 5, 9, 7, 0, 0, time.UTC)
	for _, c := range []struct {
		locale string
		o      DateOptions
		want   string
	}{
		{"zh-CN-u-hc-h12", DateOptions{Skeleton: "jm"}, "上午9:07"},
		{"ko-KR-u-hc-h23", DateOptions{Time: ShortStyle}, "09:07"},
		{"ja-JP-u-hc-h12", DateOptions{Skeleton: "jm"}, "午前9:07"},
		{"en-US-u-hc-h23", DateOptions{Date: ShortStyle, Time: ShortStyle}, "1/5/26, 09:07"},
	} {
		if got := FormatDate(c.locale, at, c.o); got != c.want {
			t.Errorf("FormatDate(%q, %+v) = %q, want %q", c.locale, c.o, got, c.want)
		}
	}
	// A cycle the language already reads changes nothing.
	if a, b := FormatDate("zh-CN-u-hc-h23", at, DateOptions{Skeleton: "jm"}), FormatDate("zh-CN", at, DateOptions{Skeleton: "jm"}); a != b {
		t.Errorf("zh-CN in its own cycle wrote %q, not %q", a, b)
	}
}
