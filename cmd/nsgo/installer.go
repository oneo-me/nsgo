package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// packageSlug is what the files of an app's installers and its Linux
// packages are named by: its name in lowercase ASCII words joined by
// hyphens, as release pages and package managers keep it whole, else the
// last part of its ID for a name with no such words.
func (p *project) packageSlug() string {
	if s := slug(p.Name); s != "" {
		return s
	}
	id := p.ID
	if i := strings.LastIndexByte(id, '.'); i >= 0 {
		id = id[i+1:]
	}
	if s := slug(id); s != "" {
		return s
	}
	return "app"
}

// slug keeps s's ASCII letters and digits, in lowercase, joined by
// hyphens where anything else was.
func slug(s string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			hyphen = false
		} else {
			hyphen = true
		}
	}
	return b.String()
}

// displayName is the app's name as users see it: its catalogs' appName in
// the source language, else nsgo.yaml's name.
func (p *project) displayName() (string, error) {
	n, err := p.appNames()
	if err != nil || n == nil {
		return p.Name, err
	}
	return n.names[n.source], nil
}

// resolveInstallerFiles makes the paths of the installers' pictures and
// icons absolute and checks they exist and are of a kind they take.
func (p *project) resolveInstallerFiles() error {
	for _, f := range []struct {
		key  string
		path *string
		exts []string
	}{
		{"macos.dmg_background", &p.MacOS.DMGBackground, []string{".png"}},
		{"macos.dmg_icon", &p.MacOS.DMGIcon, []string{".icns", ".png"}},
		{"windows.installer_icon", &p.Windows.InstallerIcon, []string{".ico", ".png"}},
		{"windows.uninstaller_icon", &p.Windows.UninstallerIcon, []string{".ico", ".png"}},
	} {
		if *f.path == "" {
			continue
		}
		*f.path = p.resolve(*f.path)
		if _, err := os.Stat(*f.path); err != nil {
			return fmt.Errorf("%s: %w", f.key, err)
		}
		if ext := strings.ToLower(filepath.Ext(*f.path)); !slices.Contains(f.exts, ext) {
			return fmt.Errorf("%s: %s is not %s", f.key, rel(*f.path), strings.Join(f.exts, " or "))
		}
	}
	return nil
}

// iconFile is an installer's icon in the format ext names (.icns or
// .ico): path's own when it is one, else path's PNG or, without a path,
// the app's icon converted; nil without either.
func (p *project) iconFile(path, ext string) ([]byte, error) {
	if strings.EqualFold(filepath.Ext(path), ext) {
		return os.ReadFile(path)
	}
	img, err := p.iconImage()
	if path != "" {
		img, err = readIcon(path)
	}
	if img == nil || err != nil {
		return nil, err
	}
	if ext == ".icns" {
		return icns(img), nil
	}
	return icoFile(img), nil
}
