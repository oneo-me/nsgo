//go:build linux && !android

package linux

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

var (
	fontconfig struct {
		once    sync.Once
		err     error
		addFile func(config uintptr, file *byte) int32
	}
	// Pango's: the default font map the main thread draws with, and what
	// tells it fontconfig's fonts changed.
	pangoCairoFontMapGetDefault func() uintptr
	pangoFcFontMapConfigChanged func(fontMap uintptr)
)

// loadFontconfig loads fontconfig's application fonts.
func loadFontconfig() error {
	fontconfig.once.Do(func() {
		lib, err := purego.Dlopen("libfontconfig.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			fontconfig.err = err
			return
		}
		purego.RegisterLibFunc(&fontconfig.addFile, lib, "FcConfigAppFontAddFile")
	})
	return fontconfig.err
}

// registerFonts adds the app's font files, paths under its resources, to
// fontconfig's application fonts, which Pango finds by their families.
// It runs before GTK and the text engine make their font maps.
func registerFonts(paths []string) {
	if len(paths) == 0 {
		return
	}
	if err := loadFontconfig(); err != nil {
		log.Printf("nsgo: fonts: %v", err)
		return
	}
	addFile := fontconfig.addFile
	for _, p := range paths {
		file := filepath.Join(imageload.ResourceRoot, filepath.FromSlash(p))
		if addFile(0, cstr(file)) == 0 {
			log.Printf("nsgo: font %s: fontconfig cannot read %s", p, file)
		}
	}
}

// RegisterFont implements driver.FontRegistrar, on the engine thread:
// fontconfig takes a file, so the font is written to the user's cache
// first; the engine's Pango contexts are made again with it, and the
// main thread's font map is told fontconfig's fonts changed.
func (w *window) RegisterFont(data []byte) error {
	if err := loadFontconfig(); err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	dir := filepath.Join(cache, "nsgo", "fonts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	file := filepath.Join(dir, hex.EncodeToString(sum[:12])+".font")
	if _, err := os.Stat(file); err != nil {
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return err
		}
	}
	if fontconfig.addFile(0, cstr(file)) == 0 {
		return errors.New("fontconfig cannot read it")
	}
	for i, m := range w.measureMap {
		if m != 0 {
			gObjectUnref(m)
			w.measureMap[i] = 0
		}
	}
	w.hitMu.Lock()
	for i, m := range w.hitMap {
		if m != 0 {
			gObjectUnref(m)
			w.hitMap[i] = 0
		}
	}
	w.hitMu.Unlock()
	if pangoFcFontMapConfigChanged == nil {
		return nil
	}
	if w.handle == 0 {
		pangoFcFontMapConfigChanged(pangoCairoFontMapGetDefault())
		return nil
	}
	w.post(func() { pangoFcFontMapConfigChanged(pangoCairoFontMapGetDefault()) })
	return nil
}
