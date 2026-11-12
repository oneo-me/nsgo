//go:build darwin

package apple

import (
	"errors"
	"log"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

var (
	cgFontCreateWithDataProvider      func(provider uintptr) uintptr
	ctFontManagerRegisterGraphicsFont func(font uintptr, err *uintptr) bool
	loadFonts                         sync.Once
)

// RegisterFonts registers the app's font files, paths under its
// resources, with CoreText for the process, so text finds them by their
// families. Call it before the first view.
func RegisterFonts(paths []string) {
	for _, path := range paths {
		data, err := imageload.ReadResource(path)
		if err != nil || len(data) == 0 {
			log.Printf("nsgo: font %s: %v", path, err)
			continue
		}
		if err := RegisterFont(data); err != nil {
			log.Printf("nsgo: font %s: %v", path, err)
		}
	}
}

// RegisterFont registers font file data with CoreText for the process,
// as the app runs: text prepared from then on finds its families.
func RegisterFont(data []byte) error {
	if len(data) == 0 {
		return errors.New("no font data")
	}
	loadFonts.Do(func() {
		cg := load("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
		ct := load("/System/Library/Frameworks/CoreText.framework/CoreText")
		purego.RegisterLibFunc(&cgFontCreateWithDataProvider, cg, "CGFontCreateWithDataProvider")
		purego.RegisterLibFunc(&ctFontManagerRegisterGraphicsFont, ct, "CTFontManagerRegisterGraphicsFont")
	})
	cfData := cfDataCreate(0, &data[0], len(data))
	provider := cgDataProviderCreateWithData(cfData)
	font := cgFontCreateWithDataProvider(provider)
	Release(provider)
	Release(cfData)
	if font == 0 {
		return errors.New("not a font CoreText reads")
	}
	var cfErr uintptr
	if !ctFontManagerRegisterGraphicsFont(font, &cfErr) {
		// Registered already, as when a dev reload runs Run again.
		Release(cfErr)
	}
	Release(font)
	// Families looked up before it found another face.
	fontCache.clear()
	attrsCache.clear()
	return nil
}
