//go:build windows

package windows

import (
	"errors"
	"log"
	"sync"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

// An app's font files join DirectWrite's system fonts in a collection of
// NSGo's own, which every text format looks families up in: an in-memory
// font file loader holds the files' bytes, and a font set builder makes
// one set of the system's fonts and those files. It needs IDWriteFactory5,
// Windows 10 1703 or later; earlier Windows shows the system's fonts.

var iidDWriteFactory5 = parseGUID("958db99a-be2a-4f09-af7d-65189803d1d3")

const (
	dwRegisterFontFileLoader        = 13
	dw3GetSystemFontSet             = 35
	dw3CreateFontCollectionFromSet  = 37
	dw5CreateFontSetBuilder         = 43
	dw5CreateInMemoryFontFileLoader = 44
	memoryLoaderCreateReference     = 4
	fontSetBuilderAddFontSet        = 5
	fontSetBuilderCreateFontSet     = 6
	fontSetBuilder1AddFontFile      = 7
)

// appFonts is the system's fonts and the app's, nil without app fonts or
// when DirectWrite cannot make the collection; fontData are the app's
// fonts it holds, in order. The engine thread and the main thread read
// appFonts, so it changes under fontsMu.
var (
	fontsMu  sync.Mutex
	appFonts *comObject
	fontData [][]byte
)

// registerFonts makes appFonts of the app's font files, paths under its
// resources. Call it before the first text.
func registerFonts(paths []string) {
	var add [][]byte
	for _, p := range paths {
		data, err := imageload.ReadResource(p)
		if err != nil || len(data) == 0 {
			log.Printf("nsgo: font %s: %v", p, err)
			continue
		}
		add = append(add, data)
	}
	if len(add) > 0 {
		if err := addFonts(add...); err != nil {
			log.Printf("nsgo: fonts: %v", err)
		}
	}
}

// RegisterFont implements driver.FontRegistrar: appFonts is made again
// with it, and the families looked up before look again.
func (w *window) RegisterFont(data []byte) error { return addFonts(data) }

// addFonts makes appFonts of the system's fonts, the app's so far and
// data's, and has the families looked up before look again.
func addFonts(data ...[]byte) error {
	err := buildFonts(data)
	// After fontsMu, which metricsOf takes under facesMu.
	facesMu.Lock()
	clear(faces)
	facesMu.Unlock()
	return err
}

func buildFonts(data [][]byte) error {
	fontsMu.Lock()
	defer fontsMu.Unlock()
	f := dwriteFactory()
	if f == nil {
		return errors.New("no DirectWrite")
	}
	f5 := f.query(&iidDWriteFactory5)
	if f5 == nil {
		return errors.New("fonts need Windows 10 1703 or later")
	}
	defer f5.release()
	var loader, builder, system, set *comObject
	if failed(f5.call(dw5CreateInMemoryFontFileLoader, uintptr(unsafe.Pointer(&loader)))) {
		return errors.New("DirectWrite cannot load fonts from memory")
	}
	// DirectWrite keeps the registered loader, which keeps the bytes.
	f5.call(dwRegisterFontFileLoader, loader.ptr())
	if failed(f5.call(dw5CreateFontSetBuilder, uintptr(unsafe.Pointer(&builder)))) {
		return errors.New("DirectWrite cannot build a font set")
	}
	defer builder.release()
	if !failed(f5.call(dw3GetSystemFontSet, uintptr(unsafe.Pointer(&system)))) {
		builder.call(fontSetBuilderAddFontSet, system.ptr())
		system.release()
	}
	var bad error
	all := append(append([][]byte(nil), fontData...), data...)
	for i, d := range all {
		var file *comObject
		// With no owner object, DirectWrite copies the bytes.
		if failed(loader.call(memoryLoaderCreateReference, f5.ptr(), uintptr(unsafe.Pointer(&d[0])),
			uintptr(len(d)), 0, uintptr(unsafe.Pointer(&file)))) ||
			failed(builder.call(fontSetBuilder1AddFontFile, file.ptr())) {
			if i >= len(fontData) {
				bad = errors.New("DirectWrite cannot read it")
			}
		}
		if file != nil {
			file.release()
		}
	}
	if failed(builder.call(fontSetBuilderCreateFontSet, uintptr(unsafe.Pointer(&set)))) {
		return errors.New("DirectWrite cannot make the font set")
	}
	defer set.release()
	var c *comObject
	if failed(f5.call(dw3CreateFontCollectionFromSet, set.ptr(), uintptr(unsafe.Pointer(&c)))) {
		return errors.New("DirectWrite cannot make the collection")
	}
	if appFonts != nil {
		appFonts.release()
	}
	appFonts, fontData = c, all
	return bad
}

// fontCollection is appFonts, held for the caller, which releases it.
func fontCollection() *comObject {
	fontsMu.Lock()
	defer fontsMu.Unlock()
	if appFonts == nil {
		return nil
	}
	appFonts.addRef()
	return appFonts
}
