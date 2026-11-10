//go:build !darwin

package main

import "errors"

// aliasOnVolume needs a Mac, which disk images are built on.
func aliasOnVolume(mnt, volume, path string) (macAliasInfo, error) {
	return macAliasInfo{}, errors.New("disk images are built on macOS")
}
