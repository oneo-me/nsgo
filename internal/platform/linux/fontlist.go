//go:build linux && !android

package linux

import (
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// familyNames maps a fontFamily list to fontconfig's names.
func familyNames(family string) []string {
	var names []string
	for _, f := range driver.FontFamilies(family) {
		switch f.Kind {
		case driver.FamilySerif:
			names = append(names, "serif")
		case driver.FamilyMonospace:
			names = append(names, "monospace")
		case driver.FamilySystem, driver.FamilyRounded:
			names = append(names, "sans-serif")
		default:
			names = append(names, f.Name)
		}
	}
	return names
}

// pangoFamilies is a fontFamily list as Pango takes one: separated by
// commas, so a name with a comma in it cannot be one of its families.
func pangoFamilies(family string) string {
	names := familyNames(family)
	kept := names[:0]
	for _, n := range names {
		if !strings.Contains(n, ",") {
			kept = append(kept, n)
		}
	}
	return strings.Join(kept, ",")
}

// gtkFamilies is a fontFamily list as GTK's CSS takes one.
func gtkFamilies(family string) string {
	names := familyNames(family)
	for i, n := range names {
		names[i] = strconv.Quote(n)
	}
	return strings.Join(names, ", ")
}
