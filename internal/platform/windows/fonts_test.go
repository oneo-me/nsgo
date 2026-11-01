//go:build windows

package windows

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/image/font/gofont/goregular"
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
	registerFonts([]string{"fonts/smallcaps.ttf"})
	if appFonts == nil {
		t.Skip("no IDWriteFactory5")
	}
	w := &window{scale: 1}
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
// family was looked up, as a TSX module's import does as the app runs:
// the collection text looks families up in has it from then on.
func TestAFontRegisteredLaterReplacesItsFallback(t *testing.T) {
	has := func(family string) bool {
		c := systemFonts()
		if c == nil {
			t.Skip("no DirectWrite")
		}
		defer c.release()
		var index uint32
		var exists int32
		c.call(collectionFindFamilyName, uintptr(unsafe.Pointer(wide(family))), uintptr(unsafe.Pointer(&index)),
			uintptr(unsafe.Pointer(&exists)))
		return exists != 0
	}
	// "Go": DirectWrite reads a weight in a family's name as its weight.
	if has("Go") {
		t.Skip("the system has Go's fonts")
	}
	if err := (&window{}).RegisterFont(goregular.TTF); err != nil {
		t.Skip(err)
	}
	if !has("Go") {
		t.Fatal("the family is not in the collection after it registered")
	}
}
