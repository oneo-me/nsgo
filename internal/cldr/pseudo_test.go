package cldr

import (
	"testing"
	"time"
)

func TestPseudolocalize(t *testing.T) {
	for _, c := range []struct{ tag, in, want string }{
		{PseudoAccents, "Sort by " + ArgStart + "name" + ArgEnd, "[Śóŕť ƀý name ~~~]"},
		{PseudoAccents, "Undo", "[Úńďó ~~]"},
		{PseudoBidi, "Sort by " + ArgStart + "name" + ArgEnd, "‏‮Sort‬‏ ‏‮by‬‏ ‏name‏"},
		{"en", "Sort by " + ArgStart + "name" + ArgEnd, "Sort by name"},
	} {
		if got := Pseudolocalize(c.tag, c.in); got != c.want {
			t.Errorf("Pseudolocalize(%q, %q) = %q, want %q", c.tag, c.in, got, c.want)
		}
	}
}

func TestZones(t *testing.T) {
	day := time.Date(2026, 1, 5, 13, 7, 0, 0, time.FixedZone("", 8*3600))
	india := time.Date(2026, 1, 5, 13, 7, 0, 0, time.FixedZone("", 5*3600+1800))
	for _, c := range []struct {
		got, want string
	}{
		{FormatDate("en", day, DateOptions{Time: LongStyle}), "1:07:00 PM GMT+8"},
		{FormatDate("en", day, DateOptions{Time: FullStyle}), "1:07:00 PM GMT+08:00"},
		{FormatDate("en", india, DateOptions{Time: LongStyle}), "1:07:00 PM GMT+5:30"},
		{FormatDate("en", day.UTC(), DateOptions{Time: LongStyle}), "5:07:00 AM GMT"},
		{FormatDate("fr", day, DateOptions{Time: LongStyle}), "13:07:00 UTC+8"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
