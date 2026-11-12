//go:build darwin

package apple

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gosmallcaps"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

// TestRegisteredFontsLayOut registers a font file from the resources
// and checks that text in its family lays out in it: in small capitals,
// which the system's font draws narrower.
func TestRegisteredFontsLayOut(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "fonts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fonts", "smallcaps.ttf"), gosmallcaps.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	root := imageload.ResourceRoot
	imageload.ResourceRoot = dir
	defer func() { imageload.ResourceRoot = root }()
	width := func(family string) driver.Points {
		p := TextEngine{}.Prepare(&driver.AttributedText{Text: "lowercase letters",
			Style: driver.TextStyle{Font: driver.Font{Family: family, Size: 20, Weight: 400}, Color: driver.RGB(0)}})
		defer p.Release()
		return p.Measure(driver.Auto).Width
	}
	RegisterFonts([]string{"fonts/smallcaps.ttf"})
	fallback := width("Go Smallcaps Unregistered")
	if got := width("Go Smallcaps"); got <= fallback*1.05 {
		t.Fatalf("Go Smallcaps measures %v, the system's font %v: not its own glyphs", got, fallback)
	}
}

// TestAFontRegisteredLaterReplacesItsFallback registers a font after its
// family was looked up, as a TSX module's import does as the app runs:
// text prepared after it lays out in it, not in the fallback found first.
func TestAFontRegisteredLaterReplacesItsFallback(t *testing.T) {
	width := func() driver.Points {
		p := TextEngine{}.Prepare(&driver.AttributedText{Text: "iiiiiiii",
			Style: driver.TextStyle{Font: driver.Font{Family: "Go Mono", Size: 20, Weight: 400}, Color: driver.RGB(0)}})
		defer p.Release()
		return p.Measure(driver.Auto).Width
	}
	before := width()
	if err := RegisterFont(gomono.TTF); err != nil {
		t.Fatal(err)
	}
	// Monospaced, its i is as wide as any letter, wider than a fallback's.
	if after := width(); after <= before*1.2 {
		t.Fatalf("Go Mono measures %v after it registered, %v before: still the fallback", after, before)
	}
}
