//go:build darwin

package apple

import (
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
