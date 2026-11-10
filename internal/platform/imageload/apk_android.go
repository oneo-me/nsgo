//go:build android

package imageload

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// apk is the APK the app runs from, opened on first use.
var apk struct {
	sync.Once
	r   *zip.ReadCloser
	err error
}

// readAPKAsset reads path from the assets of the APK the app runs from,
// where packaging puts its resources.
func readAPKAsset(path string) ([]byte, error) {
	apk.Do(func() {
		// The runtime starts the app from the extracted native libraries,
		// lib/<abi> beside the installed base.apk.
		exe, err := os.Executable()
		if err != nil {
			apk.err = err
			return
		}
		apk.r, apk.err = zip.OpenReader(filepath.Join(filepath.Dir(exe), "..", "..", "base.apk"))
	})
	r, err := apk.r, apk.err
	if err != nil {
		return nil, err
	}
	f, err := r.Open("assets/" + path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
