package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// An AppImage, <slug>-<version>-linux-<arch>.AppImage, runs on any Linux
// with FUSE without installing: AppImage's type 2 runtime, a static
// executable nsgo downloads once, followed by a SquashFS image of the
// Linux package (squashfs.go) as an AppDir, with AppRun, a link to the
// app, and the desktop entry and icon at its top. GTK 4 comes from the
// system, as for the Debian and Arch packages.

// appImageRuntimes are AppImage's runtime for each architecture, Go's,
// under its MIT license.
var appImageRuntimes = map[string]pinned{
	"amd64": {"appimage-runtime-20251108-x86_64",
		"https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime-x86_64",
		"2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d"},
	"arm64": {"appimage-runtime-20251108-aarch64",
		"https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime-aarch64",
		"00cbdfcf917cc6c0ff6d3347d59e0ca1f7f45a6df1a428a0d6d8a78664d87444"},
}

// linuxPackageSuffixes name the packages written beside the Linux
// package, which none of them holds.
var linuxPackageSuffixes = []string{".deb", ".pkg.tar.gz", ".AppImage", ".tar.gz"}

// isLinuxPackage reports whether a file at the top of the Linux package
// is one of the packages written from it.
func isLinuxPackage(rel string) bool {
	for _, s := range linuxPackageSuffixes {
		if strings.HasSuffix(rel, s) {
			return true
		}
	}
	return false
}

// buildAppImage writes the AppImage of the Linux package in dir, built for
// arch, Go's, beside it, and returns its path.
func buildAppImage(ctx context.Context, p *project, dir, arch string) (string, error) {
	runtimePath, err := appImageRuntimes[arch].fetch(ctx)
	if err != nil {
		return "", fmt.Errorf("AppImage's runtime: %w", err)
	}
	stage(ctx, "writing the AppImage")
	entries, err := appDirEntries(p, dir)
	if err != nil {
		return "", err
	}
	root, err := sqfsTree(entries)
	if err != nil {
		return "", err
	}
	image, err := squashfs(root, uint32(time.Now().Unix()))
	if err != nil {
		return "", err
	}
	runtime, err := os.ReadFile(runtimePath)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, p.releaseName("linux_"+goArchName(arch), ".AppImage"))
	return path, os.WriteFile(path, append(runtime, image...), 0o755)
}

// appDirEntries are the AppImage's files: the Linux package in dir, with
// AppRun starting the app and .DirIcon its icon.
func appDirEntries(p *project, dir string) ([]linuxPackageEntry, error) {
	entries, err := linuxDirEntries(dir)
	if err != nil {
		return nil, err
	}
	entries = append(entries, linuxPackageEntry{name: "AppRun", link: p.Name, mode: 0o777})
	if _, err := os.Stat(filepath.Join(dir, p.ID+".png")); err == nil {
		entries = append(entries, linuxPackageEntry{name: ".DirIcon", link: p.ID + ".png", mode: 0o777})
	}
	return entries, nil
}

// linuxDirEntries are the files of the Linux package in dir, by their
// paths in it, without the packages written beside them.
func linuxDirEntries(dir string) ([]linuxPackageEntry, error) {
	var entries []linuxPackageEntry
	err := filepath.WalkDir(dir, func(file string, d fs.DirEntry, err error) error {
		if err != nil || file == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, file)
		rel = filepath.ToSlash(rel)
		if !strings.Contains(rel, "/") && isLinuxPackage(rel) {
			return nil
		}
		if d.IsDir() {
			entries = append(entries, linuxPackageEntry{name: rel, mode: 0o755, dir: true})
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(file)
			if err != nil {
				return err
			}
			entries = append(entries, linuxPackageEntry{name: rel, link: link, mode: 0o777})
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		mode := int64(0o644)
		if info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		entries = append(entries, linuxPackageEntry{name: rel, data: data, mode: mode})
		return nil
	})
	return entries, err
}
