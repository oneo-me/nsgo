//go:build darwin && !ios

package apple

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestPreparePicked(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for i := range m.Pix {
		m.Pix[i] = 0x80
	}
	m.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	png.Encode(&b, m)
	shot := b.Bytes()

	f := PreparePicked(driver.PickRequest{Limit: 1}, driver.PickedFile{Name: "shot.png", Data: shot})
	if !bytes.Equal(f.Data, shot) || f.Size != (driver.Size{Width: 400, Height: 200}) || f.MIME != "image/png" {
		t.Fatalf("kept = %v %v", f.MIME, f.Size)
	}
	f = PreparePicked(driver.PickRequest{Limit: 1, MaxDimension: 100}, driver.PickedFile{Name: "shot.png", Data: shot})
	if f.MIME != "image/png" || f.Size != (driver.Size{Width: 100, Height: 50}) || driver.SniffImage(f.Data) != "image/png" {
		t.Fatalf("scaled = %v %v", f.MIME, f.Size)
	}

	// HEIC, which browsers do not show, comes as JPEG.
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.png"), filepath.Join(dir, "IMG_1.HEIC")
	os.WriteFile(in, shot, 0o644)
	if err := exec.Command("sips", "-s", "format", "heic", in, "--out", out).Run(); err != nil {
		t.Skip("sips cannot write HEIC here:", err)
	}
	heic, _ := os.ReadFile(out)
	if driver.SniffImage(heic) != "image/heic" {
		t.Fatalf("sips wrote %q", driver.SniffImage(heic))
	}
	f = PreparePicked(driver.PickRequest{Limit: 1, MaxDimension: 200}, driver.PickedFile{Name: "IMG_1.HEIC", Data: heic})
	if f.MIME != "image/jpeg" || f.Name != "IMG_1.jpg" || f.Size != (driver.Size{Width: 200, Height: 100}) ||
		driver.SniffImage(f.Data) != "image/jpeg" {
		t.Fatalf("heic = %v %v %v", f.Name, f.MIME, f.Size)
	}
	f = PreparePicked(driver.PickRequest{Limit: 1, Format: driver.PickCurrent}, driver.PickedFile{Name: "IMG_1.HEIC", Data: heic})
	if f.MIME != "image/heic" || !bytes.Equal(f.Data, heic) {
		t.Fatalf("current = %v", f.MIME)
	}
	// The clipboard reads TIFF as PNG.
	tiff := filepath.Join(dir, "a.tiff")
	exec.Command("sips", "-s", "format", "tiff", in, "--out", tiff).Run()
	data, _ := os.ReadFile(tiff)
	p, size, err := PNG(data)
	if err != nil || driver.SniffImage(p) != "image/png" || size != (driver.Size{Width: 400, Height: 200}) {
		t.Fatalf("PNG = %v %v", size, err)
	}
}
