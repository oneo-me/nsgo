// Package library finds libghostty-vt for the terminal: the file
// $NSGO_GHOSTTY_VT names, the one the nsgo tool put into the app, or on
// desktops the build for the platform the package embeds, written once
// into the user's cache.
//
// The builds in lib/ come from Ghostty's sources at the commit the
// terminal binds, built by `go run ./tools/libghostty`. The nsgo tool
// signs them into iOS apps as GhosttyVT.framework and into macOS apps'
// Frameworks.
package library

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Find returns the libghostty-vt to load, the first of:
//
//   - the file $NSGO_GHOSTTY_VT names;
//   - the one in the app: an iOS app's GhosttyVT.framework, a macOS app's
//     Frameworks, or beside the executable;
//   - on desktops, the build this package embeds for the platform,
//     written once into the user's cache
//     (<cache>/nsgo/libghostty-vt/<sha256>/).
func Find() (string, error) {
	if p := os.Getenv("NSGO_GHOSTTY_VT"); p != "" {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		paths := []string{filepath.Join(dir, "Frameworks", "GhosttyVT.framework", "GhosttyVT")}
		if name != "" {
			paths = append(paths, filepath.Join(dir, "..", "Frameworks", name), filepath.Join(dir, name))
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	if embedded == nil {
		if runtime.GOOS == "ios" {
			return "", fmt.Errorf("terminal: the app has no GhosttyVT.framework: build it with the nsgo tool")
		}
		return "", fmt.Errorf("terminal: libghostty-vt has no build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return extract()
}

// extract writes the embedded build into the user's cache, through a
// temporary file beside it so that no process loads a partial one, and
// returns its path.
func extract() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(embedded)
	path := filepath.Join(cache, "nsgo", "libghostty-vt", hex.EncodeToString(sum[:8]), name)
	if fi, err := os.Stat(path); err == nil && fi.Size() == int64(len(embedded)) {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), name+".*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(embedded)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o755)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return "", fmt.Errorf("terminal: writing libghostty-vt: %w", err)
	}
	return path, nil
}
