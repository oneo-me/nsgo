//go:build !android

package imageload

import "errors"

// readAPKAsset reads an APK's assets, which only Android has: elsewhere
// the zip reader stays out of apps.
func readAPKAsset(string) ([]byte, error) { return nil, errors.ErrUnsupported }
