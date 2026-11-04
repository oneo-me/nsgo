package cldr

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestFormatDate compares FormatDate with what ICU writes through Intl
// (testdata/golden.html, run in Chrome), in the Gregorian calendar, UTC.
func TestFormatDate(t *testing.T) {
	data, err := os.ReadFile("testdata/dates.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Locale, Options, Want string
		Time                  int64
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	styles := map[string]Style{"short": ShortStyle, "medium": MediumStyle, "long": LongStyle, "full": FullStyle}
	failed := 0
	for _, c := range cases {
		var o DateOptions
		for _, kv := range strings.Split(c.Options, ",") {
			k, v, _ := strings.Cut(kv, "=")
			switch k {
			case "date":
				o.Date = styles[v]
			case "time":
				o.Time = styles[v]
			case "skeleton":
				o.Skeleton = v
			}
		}
		if known(c.Locale, o) {
			continue
		}
		got := FormatDate(c.Locale, time.UnixMilli(c.Time).UTC(), o)
		if got != c.Want {
			failed++
			if failed <= 60 {
				t.Errorf("%s %s %s: got %q, want %q (%s)", c.Locale, time.UnixMilli(c.Time).UTC().Format(time.DateTime), c.Options, got, c.Want, Pattern(c.Locale, o))
			}
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d cases differ", failed, len(cases))
	}
}

// known reports the cases where Chrome differs from CLDR 48 on purpose
// or by its data: Thai's skeletons of times are "HH:mm น." in CLDR 48,
// "HH:mm" in Chrome's older data, and V8 writes Traditional Chinese's
// skeletons of times with AM and PM (下午7:45) where CLDR's patterns have
// flexible day periods (晚上7:45), as its time styles keep.
func known(locale string, o DateOptions) bool {
	hour := strings.ContainsAny(o.Skeleton, "jhH")
	switch locale {
	case "th":
		return hour
	case "zh-Hant", "zh-TW":
		return hour && !strings.Contains(o.Skeleton, "H")
	}
	return false
}
