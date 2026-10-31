// Package imagedecode decodes image bytes into premultiplied pixels for
// the backends that decode images themselves: PNG, JPEG, GIF and WebP,
// animated GIF and WebP with every frame composited, and SVG drawn by a
// rasterizer of its own from core.ParseSVG's drawing, as NSH's svg module
// draws with resvg. It is pure Go, so every target decodes alike.
//
// Images come from the network, so decoding is bounded: an SVG document
// is at most MaxBytes (an .svgz once inflated) and its raster at most
// MaxSide a side and MaxPixels in all; a bitmap is at most
// MaxBitmapPixels; an animation keeps at most MaxFrames frames and
// MaxAnimationBytes of pixels. A panic while decoding is an error. Every
// backend's decodes Wait for one of as many slots as processors.
package imagedecode

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"time"

	"golang.org/x/image/webp"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Bounds on what decoding one image takes, NSH's where it has them.
const (
	// MaxBytes is the largest SVG document read, in bytes, an .svgz once
	// inflated.
	MaxBytes = 16 << 20
	// MaxSide is the longest side of an SVG raster, in pixels.
	MaxSide = 8192
	// MaxPixels is the most pixels of an SVG raster or an animation's
	// canvas: a full-screen image on a 5K display fits. An SVG asked for
	// larger is drawn smaller, keeping its aspect ratio.
	MaxPixels = 1 << 24
	// MaxBitmapPixels is the most pixels of a still bitmap: a
	// 48-megapixel photo fits. Larger ones are refused.
	MaxBitmapPixels = 1 << 26
	// Oversample is how many pixels per point Decode draws an SVG at
	// when Options asks for no size.
	Oversample = 4
	// MaxOversampledSide is the longest side of an oversampled SVG, in
	// pixels.
	MaxOversampledSide = 2048
	// MaxFrames is the most frames of an animation decoded; longer ones
	// stop at the last of them.
	MaxFrames = 600
	// MaxAnimationBytes is the most bytes an animation's frames take:
	// larger animations decode their first frame only.
	MaxAnimationBytes = 48 << 20
)

var (
	// ErrFormat is returned for bytes in no format Decode reads.
	ErrFormat = errors.New("imagedecode: unknown image format")
	// ErrTooLarge is returned for an SVG document over MaxBytes or a
	// bitmap over MaxBitmapPixels (an animation's canvas over MaxPixels).
	ErrTooLarge = errors.New("imagedecode: image too large")
	// ErrTooComplex is returned for an SVG whose drawing takes more work
	// than a document may ask for.
	ErrTooComplex = errors.New("imagedecode: SVG too complex to draw")
)

// Format is an image file format.
type Format uint8

const (
	Unknown Format = iota
	PNG
	JPEG
	GIF
	WebP
	SVG
)

func (f Format) String() string {
	switch f {
	case PNG:
		return "PNG"
	case JPEG:
		return "JPEG"
	case GIF:
		return "GIF"
	case WebP:
		return "WebP"
	case SVG:
		return "SVG"
	}
	return "unknown"
}

// Layout is the byte order of a pixel. Pixels are always premultiplied.
type Layout uint8

const (
	// BGRA is blue, green, red and alpha, as Direct2D and Cairo take.
	BGRA Layout = iota
	// RGBA is red, green, blue and alpha, as image.RGBA holds.
	RGBA
)

// Options control Decode. The zero value decodes premultiplied BGRA and
// draws an SVG oversampled.
type Options struct {
	Layout Layout
	// Cover, when positive, draws an SVG with its shorter side Cover
	// pixels long: given a view's longer side in pixels, the raster is
	// never enlarged however the view fits it (NSH's
	// svg::rasterize_covering).
	Cover int
	// Scale, when positive and Cover is not, draws an SVG at its
	// intrinsic size times Scale, a screen's scale factor
	// (svg::rasterize_scaled). With neither, an SVG is drawn at
	// Oversample pixels per point and at most MaxOversampledSide a side
	// (svg::rasterize_oversampled).
	Scale float32
}

// Image is a decoded image: one frame, or the frames of an animation, each
// a whole picture of the same size.
type Image struct {
	Format Format
	// Width and Height are the frames' size in pixels.
	Width, Height int
	// Size is the image's intrinsic size: an SVG's in points (its width
	// and height, or its viewBox's), whatever size it was drawn at; a
	// bitmap's pixels.
	Size   driver.Size
	Frames []Frame
	// Plays is how many times an animation plays: 0 for ever.
	Plays int
}

// Frame is one picture of an Image.
type Frame struct {
	// Pix holds Width*Height pixels of 4 premultiplied bytes in the
	// Options' Layout, rows top to bottom without padding.
	Pix []byte
	// Delay is how long an animation shows the frame; 0 for a still
	// image. Delays of 10ms or less, which tools write meaning "as fast as
	// possible", are 100ms, as browsers show them.
	Delay time.Duration
}

// Sniff tells an image's format from its first bytes; file names and MIME
// types are never trusted. An SVG is a document whose root element is
// <svg, after an optional byte order mark, XML declaration, comments and
// doctype, or such a document compressed with gzip (.svgz).
func Sniff(data []byte) Format {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return PNG
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		return JPEG
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return GIF
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP":
		return WebP
	case sniffSVG(data):
		return SVG
	}
	return Unknown
}

// Decode decodes an image in any format Sniff tells.
func Decode(data []byte, opts Options) (img *Image, err error) {
	defer func() {
		if r := recover(); r != nil {
			img, err = nil, fmt.Errorf("imagedecode: decoder panicked: %v", r)
		}
	}()
	format := Sniff(data)
	switch format {
	case PNG, JPEG:
		img, err = decodeStill(data, format)
	case GIF:
		img, err = decodeGIF(data)
	case WebP:
		img, err = decodeWebP(data)
	case SVG:
		img, err = decodeSVG(data, opts)
	default:
		return nil, ErrFormat
	}
	if err != nil {
		return nil, err
	}
	img.Format = format
	if opts.Layout == BGRA {
		for _, f := range img.Frames {
			swapRB(f.Pix)
		}
	}
	return img, nil
}

// Size returns an image's size as an image view's EventLoad reports it,
// read from its header without decoding its pixels: a bitmap's pixels,
// an SVG's intrinsic size in whole points.
func Size(data []byte) (size driver.Size, err error) {
	defer func() {
		if r := recover(); r != nil {
			size, err = driver.Size{}, fmt.Errorf("imagedecode: decoder panicked: %v", r)
		}
	}()
	var cfg image.Config
	switch Sniff(data) {
	case PNG:
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
	case JPEG:
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(data))
	case GIF:
		cfg, err = gif.DecodeConfig(bytes.NewReader(data))
	case WebP:
		cfg, err = webp.DecodeConfig(bytes.NewReader(data))
	case SVG:
		s, err := SVGSize(data)
		if err != nil {
			return driver.Size{}, err
		}
		return driver.Size{Width: driver.Points(math.Round(float64(s.Width))),
			Height: driver.Points(math.Round(float64(s.Height)))}, nil
	default:
		return driver.Size{}, ErrFormat
	}
	if err != nil {
		return driver.Size{}, err
	}
	return driver.Size{Width: driver.Points(cfg.Width), Height: driver.Points(cfg.Height)}, nil
}

// decodeStill decodes a PNG or a JPEG.
func decodeStill(data []byte, format Format) (*Image, error) {
	decodeConfig, decode := png.DecodeConfig, png.Decode
	if format == JPEG {
		decodeConfig, decode = jpeg.DecodeConfig, jpeg.Decode
	}
	cfg, err := decodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := checkBitmap(cfg.Width, cfg.Height); err != nil {
		return nil, err
	}
	m, err := decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return still(m), nil
}

// checkBitmap refuses a bitmap without pixels or over MaxBitmapPixels.
func checkBitmap(w, h int) error {
	if w <= 0 || h <= 0 {
		return errors.New("imagedecode: image has no pixels")
	}
	if int64(w)*int64(h) > MaxBitmapPixels {
		return fmt.Errorf("%w: %d×%d pixels", ErrTooLarge, w, h)
	}
	return nil
}

// still makes a one-frame Image of m.
func still(m image.Image) *Image {
	rgba := premultiplied(m)
	w, h := rgba.Rect.Dx(), rgba.Rect.Dy()
	return &Image{
		Width:  w,
		Height: h,
		Size:   driver.Size{Width: driver.Points(w), Height: driver.Points(h)},
		Frames: []Frame{{Pix: rgba.Pix}},
	}
}

// premultiplied returns m's pixels as an image.RGBA at the origin, with
// no padding between rows.
func premultiplied(m image.Image) *image.RGBA {
	b := m.Bounds()
	if r, ok := m.(*image.RGBA); ok && b.Min == (image.Point{}) && r.Stride == 4*b.Dx() {
		return r
	}
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Rect, m, b.Min, draw.Src)
	return rgba
}

// swapRB turns RGBA pixels into BGRA ones, or back.
func swapRB(pix []byte) {
	for i := 0; i+3 < len(pix); i += 4 {
		pix[i], pix[i+2] = pix[i+2], pix[i]
	}
}

// frameDelay returns how long a frame of an animation shows, as browsers
// time them.
func frameDelay(d time.Duration) time.Duration {
	if d <= 10*time.Millisecond {
		return 100 * time.Millisecond
	}
	return d
}
