package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// dmgName is the disk image's file: <slug>-<version>-<arch>.dmg, each
// architecture's named apart so a release keeps both.
func dmgName(p *project, arch string) string {
	return fmt.Sprintf("%s-%s-%s.dmg", p.packageSlug(), p.Version, arch)
}

// buildDMG puts app, the signed bundle, into a compressed read-only disk
// image in dir. Opening it shows the app beside a link to /Applications
// over the background picture (dmgbackground.go), laid out by a .DS_Store
// the tool writes (dsstore.go), under the volume's icon; only hdiutil and
// tiffutil, which macOS ships, make it. sign.identity signs the image,
// which an ad hoc build leaves unsigned. It returns the image's path.
func buildDMG(ctx context.Context, p *project, app, dir, arch string) (string, error) {
	stage("making the disk image")
	work, err := os.MkdirTemp(dir, ".dmg-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)

	// The image starts as a copy of a folder holding what it shows, which
	// hdiutil sizes. ditto keeps the bundle as signed and copies it
	// without cloning, since hdiutil counts the blocks a file takes.
	src := filepath.Join(work, "src")
	name := filepath.Base(app)
	if err := os.MkdirAll(filepath.Join(src, ".background"), 0o755); err != nil {
		return "", err
	}
	if err := run(ctx, "ditto", "--noclone", app, filepath.Join(src, name)); err != nil {
		return "", err
	}
	if err := os.Symlink("/Applications", filepath.Join(src, "Applications")); err != nil {
		return "", err
	}
	pic, err := dmgBackground(ctx, p, work)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(src, ".background", pic.name), pic.data, 0o644); err != nil {
		return "", err
	}
	// The layout names the picture by its alias, which only the mounted
	// volume tells: a placeholder of the same size holds its space.
	layout := layoutFor(pic.width, pic.height)
	if err := writeDSStore(filepath.Join(src, ".DS_Store"), name, layout); err != nil {
		return "", err
	}
	icon, err := p.iconFile(p.MacOS.DMGIcon, ".icns")
	if err != nil {
		return "", err
	}
	if icon != nil {
		if err := os.WriteFile(filepath.Join(src, ".VolumeIcon.icns"), icon, 0o644); err != nil {
			return "", err
		}
	}
	volume, err := p.displayName()
	if err != nil {
		return "", err
	}
	rw := filepath.Join(work, "rw.dmg")
	if err := hdiutil(ctx, "create", "-ov", "-volname", volume, "-srcfolder", src,
		"-fs", "HFS+", "-fsargs", "-c c=64,a=16,e=16", "-format", "UDRW", rw); err != nil {
		return "", err
	}
	mnt := filepath.Join(work, "mnt")
	if err := os.Mkdir(mnt, 0o755); err != nil {
		return "", err
	}
	disk, err := hdiutilAttach(ctx, "-readwrite", "-noverify", "-noautoopen", "-nobrowse", "-mountpoint", mnt, rw)
	if err != nil {
		return "", err
	}
	err = finishVolume(ctx, mnt, volume, name, pic, layout, icon != nil)
	// macOS creates these while the image is mounted.
	os.RemoveAll(filepath.Join(mnt, ".fseventsd"))
	os.RemoveAll(filepath.Join(mnt, ".Trashes"))
	if derr := hdiutil(ctx, "detach", disk); derr != nil {
		exec.Command("hdiutil", "detach", "-force", disk).Run()
		err = errors.Join(err, derr)
	}
	if err != nil {
		return "", err
	}
	dmg := filepath.Join(dir, dmgName(p, arch))
	if err := convertDMG(ctx, rw, dmg); err != nil {
		return "", err
	}
	if p.Sign.Identity != "" && p.Sign.Identity != "-" {
		stage("signing the disk image")
		if err := run(ctx, "codesign", "--force", "--timestamp", "--sign", p.Sign.Identity, dmg); err != nil {
			return "", err
		}
	}
	return dmg, nil
}

// finishVolume completes the mounted image at mnt: the layout with the
// background picture's alias, the picture's folder hidden, and the
// volume's icon flagged.
func finishVolume(ctx context.Context, mnt, volume, app string, pic *dmgPicture, layout dmgLayout, icon bool) error {
	background := filepath.Join(mnt, ".background", pic.name)
	alias, err := aliasOnVolume(mnt, volume, background)
	if err != nil {
		return err
	}
	layout.BackgroundAlias = macAlias(alias)
	if err := writeDSStore(filepath.Join(mnt, ".DS_Store"), app, layout); err != nil {
		return err
	}
	if err := run(ctx, "chflags", "hidden", filepath.Join(mnt, ".background")); err != nil {
		return err
	}
	if icon {
		return setFinderFlags(ctx, mnt, finderHasCustomIcon)
	}
	return nil
}

// finderHasCustomIcon is the Finder flag of folders with an icon of
// their own.
const finderHasCustomIcon = 0x0400

// setFinderFlags sets the Finder flags of a file or folder, at offset 8 of
// its com.apple.FinderInfo attribute.
func setFinderFlags(ctx context.Context, path string, flags uint16) error {
	var info [32]byte
	info[8], info[9] = byte(flags>>8), byte(flags)
	return run(ctx, "/usr/bin/xattr", "-wx", "com.apple.FinderInfo", hex.EncodeToString(info[:]), path)
}

// convertDMG writes rw as a compressed read-only image, LZMA's, the
// smallest, which macOS 10.15 and later open. macOS 27 fails hdiutil
// convert with "Resource temporarily unavailable" and takes diskutil's
// image verb instead, which older systems lack.
func convertDMG(ctx context.Context, rw, dmg string) error {
	if exec.CommandContext(ctx, "diskutil", "image", "create", "from", "-h").Run() == nil {
		os.Remove(dmg)
		return run(ctx, "diskutil", "image", "create", "from", "--format", "ULMO", rw, dmg)
	}
	return hdiutil(ctx, "convert", rw, "-format", "ULMO", "-ov", "-o", dmg)
}

// hdiutil runs hdiutil, again while it fails with "Resource busy", as
// Spotlight or an antivirus holding an image makes it: after 2, 4, 8 and
// 16 seconds.
func hdiutil(ctx context.Context, args ...string) error {
	var err error
	for i := range 5 {
		if err = run(ctx, "hdiutil", args...); err == nil || !strings.Contains(err.Error(), "Resource busy") {
			return err
		}
		if i < 4 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(2<<i) * time.Second):
			}
		}
	}
	return err
}

// hdiutilAttach attaches an image and returns the device of its whole
// disk, which hdiutil prints first and which detaches even when the
// volume is already gone.
func hdiutilAttach(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "hdiutil", append([]string{"attach"}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("hdiutil attach: %w\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) > 0 && strings.HasPrefix(f[0], "/dev/disk") {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("hdiutil attach attached no device\n%s", out)
}
