package imagedecode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// decodeGIF decodes a GIF's frames composited as browsers show them: on a
// transparent canvas, each frame drawn over what the ones before left, then
// disposed of as it asks.
func decodeGIF(data []byte) (*Image, error) {
	data, err := limitGIF(data)
	if err != nil {
		return nil, err
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w, h := g.Config.Width, g.Config.Height
	if err := checkBitmap(w, h); err != nil {
		return nil, err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	img := &Image{
		Width:  w,
		Height: h,
		Size:   driver.Size{Width: driver.Points(w), Height: driver.Points(h)},
		Plays:  gifPlays(g.LoopCount),
	}
	var saved []byte
	for i, m := range g.Image {
		var disposal byte
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			saved = append(saved[:0], canvas.Pix...)
		}
		r := m.Bounds().Intersect(canvas.Rect)
		draw.Draw(canvas, r, m, r.Min, draw.Over)
		var delay time.Duration
		if i < len(g.Delay) {
			delay = time.Duration(g.Delay[i]) * 10 * time.Millisecond
		}
		img.Frames = append(img.Frames, Frame{Pix: bytes.Clone(canvas.Pix), Delay: frameDelay(delay)})
		switch disposal {
		case gif.DisposalBackground:
			// Browsers clear to transparent, not to the background color.
			draw.Draw(canvas, r, image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			copy(canvas.Pix, saved)
		}
	}
	if len(img.Frames) == 1 {
		img.Frames[0].Delay = 0
	}
	return img, nil
}

// gifPlays converts image/gif's LoopCount, which counts repetitions after
// the first play, into plays.
func gifPlays(loops int) int {
	switch {
	case loops == 0:
		return 0
	case loops < 0:
		return 1
	}
	return loops + 1
}

// limitGIF cuts a GIF after the frames decodeGIF keeps, since image/gif
// decodes all frames at once: MaxFrames, or the first alone when they
// would take more than MaxAnimationBytes. It refuses an animation's canvas
// over MaxPixels and leaves bytes it cannot follow to image/gif to report.
func limitGIF(data []byte) ([]byte, error) {
	if len(data) < 13 {
		return data, nil
	}
	w := int(binary.LittleEndian.Uint16(data[6:]))
	h := int(binary.LittleEndian.Uint16(data[8:]))
	pos := 13
	if data[10]&0x80 != 0 {
		pos += 3 << (data[10]&7 + 1)
	}
	var ends []int // where each frame ends
scan:
	for pos < len(data) {
		switch data[pos] {
		case 0x21: // an extension: a label and sub-blocks
			pos = skipSubBlocks(data, pos+2)
		case 0x2c: // an image: a descriptor, a color table and sub-blocks
			if pos+11 > len(data) {
				break scan
			}
			packed := data[pos+9]
			pos += 10
			if packed&0x80 != 0 {
				pos += 3 << (packed&7 + 1)
			}
			pos = skipSubBlocks(data, pos+1) // after the LZW code size
			if pos > 0 {
				ends = append(ends, pos)
			}
		default: // the trailer, or bytes that are no block
			break scan
		}
		if pos < 0 {
			break
		}
	}
	if len(ends) > 1 && w*h > MaxPixels {
		return nil, fmt.Errorf("%w: a %d×%d animation", ErrTooLarge, w, h)
	}
	keep := min(len(ends), MaxFrames)
	if keep*w*h*4 > MaxAnimationBytes {
		keep = 1
	}
	if keep == 0 || keep == len(ends) {
		return data, nil
	}
	end := ends[keep-1]
	return append(data[:end:end], 0x3b), nil
}

// skipSubBlocks returns where the sub-blocks at pos end, or -1 when the
// data ends first.
func skipSubBlocks(data []byte, pos int) int {
	for pos >= 0 && pos < len(data) {
		n := int(data[pos])
		pos++
		if n == 0 {
			return pos
		}
		pos += n
	}
	return -1
}
