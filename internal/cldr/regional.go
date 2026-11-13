package cldr

import (
	"strings"

	"golang.org/x/text/language"
)

// Regional is how a locale's region reads weeks, times and measures,
// as CLDR 48's supplemental data has it, Intl's names for its values:
// FirstDayOfWeek 1 for Monday to 7 for Sunday (ISO 8601), HourCycle
// "h11", "h12", "h23" or "h24", MeasurementSystem "metric", "us" or "uk".
type Regional struct {
	FirstDayOfWeek    int
	HourCycle         string
	MeasurementSystem string
}

// RegionalOf is locale's regional preferences: what its Unicode extension
// sets (-u-fw-, -u-hc-, -u-ms-), else its region's, likely or given, else
// its language's hours.
func RegionalOf(locale string) Regional {
	t, _ := language.Parse(strings.ReplaceAll(locale, "_", "-"))
	region, _ := t.Region()
	r := Regional{FirstDayOfWeek: firstDayOf(region.String()), HourCycle: cycleOf(hourLetter(dataAt(find(locale)))),
		MeasurementSystem: measurementOf(region.String())}
	if fw := weekdays[t.TypeForKey("fw")]; fw > 0 {
		r.FirstDayOfWeek = fw
	}
	if hc := t.TypeForKey("hc"); hc == "h11" || hc == "h12" || hc == "h23" || hc == "h24" {
		r.HourCycle = hc
	}
	switch t.TypeForKey("ms") {
	case "metric":
		r.MeasurementSystem = "metric"
	case "ussystem":
		r.MeasurementSystem = "us"
	case "uksystem":
		r.MeasurementSystem = "uk"
	}
	return r
}

// weekdays are the -u-fw- values' ISO 8601 numbers.
var weekdays = map[string]int{"mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6, "sun": 7}

// firstDayOf is CLDR's weekData firstDay of region, Monday where it
// names none.
func firstDayOf(region string) int {
	switch {
	case strings.Contains(sundayRegions, region) && len(region) == 2:
		return 7
	case strings.Contains(saturdayRegions, region) && len(region) == 2:
		return 6
	case region == "MV":
		return 5
	}
	return 1
}

// The regions whose weeks start on Sunday and on Saturday, CLDR 48's
// weekData.
const (
	sundayRegions = "AG AS BD BR BS BT BW BZ CA CN CO DM DO ET GT GU HK HN ID IL IN JM JP KE KH KR LA MH MM MO MT MX MZ " +
		"NI NP PA PE PH PK PR PT PY SA SG SV TH TT TW UM US VE VI WS YE ZA ZW"
	saturdayRegions = "AE AF BH DJ DZ EG IQ IR JO KW LY OM QA SD SY"
)

// measurementOf is CLDR's measurementData of region.
func measurementOf(region string) string {
	switch region {
	case "US", "LR", "MM", "PR", "AS", "GU", "MP", "VI", "UM":
		return "us"
	case "GB":
		return "uk"
	}
	return "metric"
}

// cycleOf is the hour cycle of an hour letter.
func cycleOf(letter byte) string {
	switch letter {
	case 'h':
		return "h12"
	case 'K':
		return "h11"
	case 'k':
		return "h24"
	}
	return "h23"
}

// hourOf is the hour letter of a -u-hc- value in locale, 0 for none.
func hourOf(locale string) byte {
	_, ext, ok := strings.Cut(strings.ToLower(locale), "-u-")
	if !ok {
		return 0
	}
	keys := strings.Split(ext, "-")
	for i, k := range keys {
		if k == "hc" && i+1 < len(keys) {
			switch keys[i+1] {
			case "h11":
				return 'K'
			case "h12":
				return 'h'
			case "h23":
				return 'H'
			case "h24":
				return 'k'
			}
		}
	}
	return 0
}
