package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testIcon(size int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 0x80, 0xff})
		}
	}
	return img
}

func TestScaledPNG(t *testing.T) {
	for _, size := range []int{16, 256, 1024} {
		img, err := png.Decode(bytes.NewReader(scaledPNG(testIcon(512), size)))
		if err != nil {
			t.Fatal(err)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Errorf("scaled to %d: got %v", size, b)
		}
	}
}

func TestICNS(t *testing.T) {
	data := icns(testIcon(64))
	if string(data[:4]) != "icns" || int(binary.BigEndian.Uint32(data[4:])) != len(data) {
		t.Fatalf("bad header % x", data[:8])
	}
	var kinds []string
	for at := 8; at < len(data); {
		size := int(binary.BigEndian.Uint32(data[at+4:]))
		if !bytes.HasPrefix(data[at+8:], []byte("\x89PNG")) {
			t.Errorf("%s is not a PNG", data[at:at+4])
		}
		kinds = append(kinds, string(data[at:at+4]))
		at += size
	}
	if len(kinds) != len(icnsTypes) {
		t.Errorf("got entries %v", kinds)
	}
}

func TestIconResource(t *testing.T) {
	images := [][]byte{scaledPNG(testIcon(64), 16), scaledPNG(testIcon(64), 32)}
	for _, arch := range []string{"amd64", "arm64"} {
		syso, err := iconResource(images, []int{16, 32}, arch)
		if err != nil {
			t.Fatal(err)
		}
		f, err := pe.NewFile(bytes.NewReader(syso))
		if err != nil {
			t.Fatal(err)
		}
		sec := f.Section(".rsrc")
		if sec == nil {
			t.Fatalf("%s: no .rsrc section", arch)
		}
		// Two icons and the group, each with a data entry to relocate.
		if len(sec.Relocs) != 3 {
			t.Errorf("%s: %d relocations, want 3", arch, len(sec.Relocs))
		}
		data, err := sec.Data()
		if err != nil {
			t.Fatal(err)
		}
		// The first data entry points at the first image.
		at := binary.LittleEndian.Uint32(data[sec.Relocs[0].VirtualAddress:])
		if !bytes.HasPrefix(data[at:], images[0]) {
			t.Errorf("%s: the first entry does not point at the first image", arch)
		}
	}
	if _, err := iconResource(images, []int{16, 32}, "riscv64"); err == nil {
		t.Error("riscv64 made a resource")
	}
}

func TestTVBrandAssets(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles an asset catalog")
	}
	if _, err := exec.LookPath("xcrun"); err != nil {
		t.Skip("needs Xcode's actool")
	}
	catalog := filepath.Join(t.TempDir(), "Assets.xcassets")
	if err := writeTVBrandAssets(filepath.Join(catalog, "AppIcon.brandassets"), testIcon(256)); err != nil {
		t.Fatal(err)
	}
	front, err := os.ReadFile(filepath.Join(catalog, "AppIcon.brandassets", "App Icon.imagestack", "Front.imagestacklayer", "Content.imageset", "front@2x.png"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(front))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("the icon's front layer at 2x is %v, want 800x480", b)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Error("the front layer covers its corner; the back layer would not show")
	}
	plist, err := compileIcon(t.Context(), catalog, t.TempDir(), "appletvsimulator", "17.0", []string{"tv"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plist, "CFBundleIcons") {
		t.Errorf("actool named no icon: %q", plist)
	}
}

func TestEdgeColor(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.Set(x, y, color.NRGBA{0x20, 0x40, 0x80, 0xff})
		}
	}
	img.Set(4, 4, color.NRGBA{0xff, 0, 0, 0xff})
	if r, g, b, a := edgeColor(img).RGBA(); r>>8 != 0x20 || g>>8 != 0x40 || b>>8 != 0x80 || a != 0xffff {
		t.Errorf("edge color = %x %x %x %x, want the border's 20 40 80, opaque", r>>8, g>>8, b>>8, a)
	}
}
