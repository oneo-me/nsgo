package main

import (
	"path/filepath"
	"syscall"
)

// aliasOnVolume reads what an alias of path records, path being on the
// volume mounted at mnt, named volume.
func aliasOnVolume(mnt, volume, path string) (macAliasInfo, error) {
	var root, parent, file syscall.Stat_t
	for _, s := range []struct {
		path string
		st   *syscall.Stat_t
	}{{mnt, &root}, {filepath.Dir(path), &parent}, {path, &file}} {
		if err := syscall.Stat(s.path, s.st); err != nil {
			return macAliasInfo{}, err
		}
	}
	rel, err := filepath.Rel(mnt, path)
	if err != nil {
		return macAliasInfo{}, err
	}
	return macAliasInfo{
		volume:        volume,
		volumeCreated: uint32(root.Birthtimespec.Sec + macEpoch),
		parentCNID:    uint32(parent.Ino),
		parentName:    filepath.Base(filepath.Dir(path)),
		fileCNID:      uint32(file.Ino),
		fileCreated:   uint32(file.Birthtimespec.Sec + macEpoch),
		fileName:      filepath.Base(path),
		path:          "/" + filepath.ToSlash(rel),
	}, nil
}
