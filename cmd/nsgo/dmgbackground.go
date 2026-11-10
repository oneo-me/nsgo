package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// The disk image's window shows a picture behind the app and the
// Applications link: macos.dmg_background, whose size in points sizes the
// window, else a built-in one, an arrow from the app to Applications that
// says what to do without words. A Retina screen shows the picture's @2x
// twin when there is one: tiffutil, which macOS ships, joins both into a
// TIFF, as Finder picks the resolution from its pages.

// dmgDefaultWidth and dmgDefaultHeight size the window with the built-in
// picture.
const dmgDefaultWidth, dmgDefaultHeight = 640, 400

// dmgPicture is the background picture: its file on the volume and the
// window's size in points.
type dmgPicture struct {
	name          string
	data          []byte
	width, height int
}

// dmgBackground makes the disk image's background picture in work.
func dmgBackground(ctx context.Context, p *project, work string) (*dmgPicture, error) {
	if path := p.MacOS.DMGBackground; path != "" {
		img, err := readPNG(path)
		if err != nil {
			return nil, err
		}
		b := img.Bounds()
		pic := &dmgPicture{name: "background.png", width: b.Dx(), height: b.Dy()}
		hidpi := strings.TrimSuffix(path, filepath.Ext(path)) + "@2x" + filepath.Ext(path)
		if _, err := os.Stat(hidpi); err != nil {
			pic.data, err = os.ReadFile(path)
			return pic, err
		}
		img2, err := readPNG(hidpi)
		if err != nil {
			return nil, err
		}
		if b2 := img2.Bounds(); b2.Dx() != 2*b.Dx() || b2.Dy() != 2*b.Dy() {
			return nil, fmt.Errorf("macos.dmg_background: %s is %dx%d, not twice %s's %dx%d", rel(hidpi), b2.Dx(), b2.Dy(), rel(path), b.Dx(), b.Dy())
		}
		pic.name = "background.tiff"
		pic.data, err = joinHiDPI(ctx, work, path, hidpi)
		return pic, err
	}
	l := layoutFor(dmgDefaultWidth, dmgDefaultHeight)
	pic := &dmgPicture{name: "background.tiff", width: l.Width, height: l.Height}
	var paths []string
	for _, scale := range []int{1, 2} {
		path := filepath.Join(work, fmt.Sprintf("background-%dx.png", scale))
		var b bytes.Buffer
		if err := png.Encode(&b, arrowBackground(l, scale)); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	var err error
	pic.data, err = joinHiDPI(ctx, work, paths[0], paths[1])
	return pic, err
}

// joinHiDPI joins a picture and its @2x twin into one TIFF.
func joinHiDPI(ctx context.Context, work, path, hidpi string) ([]byte, error) {
	out := filepath.Join(work, "background.tiff")
	if err := run(ctx, "tiffutil", "-cathidpicheck", path, hidpi, "-out", out); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

// readPNG decodes a PNG.
func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel(path), err)
	}
	return img, nil
}

// arrowBackground draws the built-in picture at scale pixels a point: a
// soft gray from top to bottom, and between the two icons a rounded arrow
// pointing to Applications.
func arrowBackground(l dmgLayout, scale int) *image.NRGBA {
	w, h := l.Width*scale, l.Height*scale
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	top, bottom := [3]float64{0xfb, 0xfb, 0xfc}, [3]float64{0xec, 0xed, 0xf0}
	ink := [3]float64{0xa9, 0xae, 0xb8}
	s := float64(scale)
	// The arrow runs from past the app's icon to short of Applications',
	// at their centers' height, its head two strokes meeting at the tip.
	gap := float64(l.IconSize)/2 + 30
	y := float64(l.AppY) * s
	x0, x1 := (float64(l.AppX)+gap)*s, (float64(l.ApplicationsX)-gap)*s
	head, radius := 20*s, 3*s
	angle := 40 * math.Pi / 180
	strokes := [][4]float64{
		{x0, y, x1, y},
		{x1, y, x1 - head*math.Cos(angle), y - head*math.Sin(angle)},
		{x1, y, x1 - head*math.Cos(angle), y + head*math.Sin(angle)},
	}
	for py := range h {
		t := float64(py) / float64(h-1)
		var bg [3]float64
		for i := range bg {
			bg[i] = top[i] + (bottom[i]-top[i])*t
		}
		for px := range w {
			cx, cy := float64(px)+0.5, float64(py)+0.5
			d := math.Inf(1)
			for _, st := range strokes {
				d = math.Min(d, segmentDistance(cx, cy, st)-radius)
			}
			// Coverage of the pixel by the strokes, for smooth edges.
			cov := math.Max(0, math.Min(1, 0.5-d))
			var c [3]uint8
			for i := range c {
				c[i] = uint8(math.Round(bg[i] + (ink[i]-bg[i])*cov))
			}
			img.SetNRGBA(px, py, color.NRGBA{c[0], c[1], c[2], 0xff})
		}
	}
	return img
}

// segmentDistance is how far (x, y) is from the segment s.
func segmentDistance(x, y float64, s [4]float64) float64 {
	dx, dy := s[2]-s[0], s[3]-s[1]
	t := ((x-s[0])*dx + (y-s[1])*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(x-(s[0]+t*dx), y-(s[1]+t*dy))
}
