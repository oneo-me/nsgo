//go:build darwin

package apple

import (
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestFontListTakesTheFirstFamilyInstalled(t *testing.T) {
	family := func(list string) string {
		font := Font(driver.Font{Family: list, Size: 12})
		defer Release(font)
		name := ctFontCopyFamilyName(font)
		defer Release(name)
		return GoString(objc.ID(name))
	}
	if got := family(`"No Such Family", Menlo, monospace`); got != "Menlo" {
		t.Errorf("the list set in %q, want Menlo", got)
	}
	if got := family("Menlo"); got != "Menlo" {
		t.Errorf("one family set in %q", got)
	}
}

func TestStyledFacesOfNamedFamilies(t *testing.T) {
	face := func(f driver.Font) string {
		font := Font(f)
		defer Release(font)
		name := ctFontCopyPostScriptName(font)
		defer Release(name)
		return GoString(objc.ID(name))
	}
	for _, family := range []string{driver.FontMonospace, "Menlo", `"No Such Family", Menlo`} {
		for want, f := range map[string]driver.Font{
			"Menlo-Bold":       {Family: family, Size: 12, Weight: 700},
			"Menlo-Italic":     {Family: family, Size: 12, Italic: true},
			"Menlo-BoldItalic": {Family: family, Size: 12, Weight: 700, Italic: true},
		} {
			if family == driver.FontMonospace && face(driver.Font{Family: family, Size: 12}) != "Menlo-Regular" {
				continue // the user's fixed-pitch font is not Menlo
			}
			if got := face(f); got != want {
				t.Errorf("%s: %+v sets in %s", want, f, got)
			}
		}
	}
	if got := face(driver.Font{Size: 12, Weight: 700}); !strings.Contains(got, "Bold") && !strings.Contains(got, "SFNS") {
		t.Errorf("the system's bold sets in %s", got)
	}
}
