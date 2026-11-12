package imagedecode

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// sniffInflated is how much of an .svgz Sniff inflates to find the root
// element.
const sniffInflated = 4096

func sniffSVG(data []byte) bool {
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return false
		}
		// A truncated read still holds the start of the document.
		head, _ := io.ReadAll(io.LimitReader(zr, sniffInflated))
		return sniffSVGText(head)
	}
	return sniffSVGText(data)
}

func sniffSVGText(b []byte) bool {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	for {
		b = bytes.TrimLeft(b, " \t\r\n")
		var rest []byte
		var ok bool
		switch {
		case bytes.HasPrefix(b, []byte("<?")):
			_, rest, ok = bytes.Cut(b, []byte("?>"))
		case bytes.HasPrefix(b, []byte("<!--")):
			_, rest, ok = bytes.Cut(b, []byte("-->"))
		case bytes.HasPrefix(b, []byte("<!")):
			// A doctype, maybe with an internal subset ([ ... ]).
			open, end := bytes.IndexByte(b, '['), bytes.IndexByte(b, '>')
			if open >= 0 && end >= 0 && open < end {
				if _, r, found := bytes.Cut(b[open:], []byte("]")); found {
					_, rest, ok = bytes.Cut(r, []byte(">"))
				}
			} else {
				_, rest, ok = bytes.Cut(b, []byte(">"))
			}
		default:
			if !bytes.HasPrefix(b, []byte("<svg")) || len(b) < 5 {
				return false
			}
			switch b[4] {
			case ' ', '\t', '\r', '\n', '>', '/':
				return true
			}
			return false
		}
		if !ok {
			return false
		}
		b = rest
	}
}

// svgDocument returns an SVG's document, inflated when it is an .svgz.
func svgDocument(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		if len(data) > MaxBytes {
			return nil, fmt.Errorf("%w: an SVG over %d MiB", ErrTooLarge, MaxBytes>>20)
		}
		return data, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	doc, err := io.ReadAll(io.LimitReader(zr, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(doc) > MaxBytes {
		return nil, fmt.Errorf("%w: an SVG over %d MiB once inflated", ErrTooLarge, MaxBytes>>20)
	}
	return doc, nil
}

func parseSVG(data []byte) (*driver.SVG, error) {
	doc, err := svgDocument(data)
	if err != nil {
		return nil, err
	}
	return driver.ParseSVG(doc)
}

// SVGSize returns an SVG's intrinsic size in points: its width and height,
// one of them and its viewBox's aspect ratio, or its viewBox's size.
func SVGSize(data []byte) (size driver.Size, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("imagedecode: SVG parser panicked: %v", r)
		}
	}()
	s, err := parseSVG(data)
	if err != nil {
		return driver.Size{}, err
	}
	w, h := s.Size()
	return driver.Size{Width: w, Height: h}, nil
}

// decodeSVG draws an SVG at the size opts asks for. Its currentColor is
// black, as an image has no text color to inherit.
func decodeSVG(data []byte, opts Options) (*Image, error) {
	s, err := parseSVG(data)
	if err != nil {
		return nil, err
	}
	pw, ph := s.Size()
	w, h := float64(pw), float64(ph)
	var longest float64
	maxSide := MaxSide
	switch {
	case opts.Cover > 0:
		longest = float64(opts.Cover) * max(w, h) / min(w, h)
	case opts.Scale > 0:
		longest = max(w, h) * float64(opts.Scale)
	default:
		longest = max(w, h) * Oversample
		maxSide = MaxOversampledSide
	}
	width, height, err := rasterSize(w, h, longest, maxSide)
	if err != nil {
		return nil, err
	}
	d := &driver.Drawing{}
	d.Scale(float32(float64(width)/w), float32(float64(height)/h))
	s.Draw(d, driver.Size{Width: pw, Height: ph}, driver.RGB(0x000000))
	pix, err := rasterize(d, width, height)
	if err != nil {
		return nil, err
	}
	return &Image{
		Width:  width,
		Height: height,
		Size:   driver.Size{Width: pw, Height: ph},
		Frames: []Frame{{Pix: pix}},
	}, nil
}

// rasterSize returns the raster size of an SVG of w by h points drawn with
// its longest side longest pixels long: its aspect ratio kept, each side
// at least a pixel and at most maxSide, and at most MaxPixels in all.
func rasterSize(w, h, longest float64, maxSide int) (int, int, error) {
	if !(longest > 0) || math.IsInf(longest, 0) || !(w > 0) || !(h > 0) {
		return 0, 0, errors.New("imagedecode: an SVG raster of no pixels")
	}
	longest = min(max(longest, 1), float64(maxSide))
	scale := longest / max(w, h)
	side := func(v float64) int { return int(min(max(v, 1), float64(maxSide))) }
	width, height := side(math.Round(w*scale)), side(math.Round(h*scale))
	if width*height <= MaxPixels {
		return width, height, nil
	}
	// Rounded down, so that the product stays within the budget.
	scale *= min(math.Sqrt(MaxPixels/(w*h*scale*scale)), 1)
	return side(math.Floor(w * scale)), side(math.Floor(h * scale)), nil
}
