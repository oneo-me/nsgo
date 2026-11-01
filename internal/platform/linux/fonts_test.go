//go:build linux && !android

package linux

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gosmallcaps"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

// TestRegisteredFontsLayOut registers a font file from the resources and
// checks that text in its family lays out in it: in small capitals, which
// the system's fallback draws narrower.
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
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	registerFonts([]string{"fonts/smallcaps.ttf"})
	w := &window{}
	width := func(family string) driver.Points {
		p := w.Prepare(&driver.AttributedText{Text: "lowercase letters",
			Style: driver.TextStyle{Font: driver.Font{Family: family, Size: 20, Weight: 400}, Color: driver.RGB(0)}})
		defer p.Release()
		return p.Measure(driver.Auto).Width
	}
	fallback := width("Go Smallcaps Unregistered")
	if got := width("Go Smallcaps"); got <= fallback*1.05 {
		t.Fatalf("Go Smallcaps measures %v, the fallback %v: not its own glyphs", got, fallback)
	}
}

// TestAFontRegisteredLaterReplacesItsFallback registers a font after its
// family was measured, as a TSX module's import does as the app runs:
// text prepared after it lays out in it.
func TestAFontRegisteredLaterReplacesItsFallback(t *testing.T) {
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	w := &window{}
	// Not a monospaced font, which fontconfig's own rules would stand in
	// for by name.
	width := func() driver.Points {
		p := w.Prepare(&driver.AttributedText{Text: "iiiiiiii mmmm",
			Style: driver.TextStyle{Font: driver.Font{Family: "Go Medium", Size: 20, Weight: 500}, Color: driver.RGB(0)}})
		defer p.Release()
		return p.Measure(driver.Auto).Width
	}
	before := width()
	if err := w.RegisterFont(gomedium.TTF); err != nil {
		t.Fatal(err)
	}
	if after := width(); math.Abs(float64(after-before)) < float64(before)*0.03 {
		t.Fatalf("Go Medium measures %v after it registered, %v before: still the fallback", after, before)
	}
}
