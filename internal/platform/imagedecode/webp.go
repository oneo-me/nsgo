package imagedecode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"time"

	"golang.org/x/image/webp"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var errWebP = errors.New("imagedecode: invalid WebP")

// riffChunk is a chunk of a RIFF container.
type riffChunk struct {
	id   string
	data []byte
}

// riffChunks reads the chunks in b, stopping at a truncated one.
func riffChunks(b []byte) []riffChunk {
	var chunks []riffChunk
	for len(b) >= 8 {
		n := binary.LittleEndian.Uint32(b[4:])
		if uint64(n) > uint64(len(b)-8) {
			break
		}
		chunks = append(chunks, riffChunk{string(b[:4]), b[8 : 8+n]})
		b = b[8+n:]
		if n&1 == 1 && len(b) > 0 {
			b = b[1:]
		}
	}
	return chunks
}

// riff writes chunks as a WebP file.
func riff(chunks ...riffChunk) []byte {
	var body bytes.Buffer
	body.WriteString("WEBP")
	for _, c := range chunks {
		body.WriteString(c.id)
		_ = binary.Write(&body, binary.LittleEndian, uint32(len(c.data)))
		body.Write(c.data)
		if len(c.data)&1 == 1 {
			body.WriteByte(0)
		}
	}
	out := make([]byte, 8, 8+body.Len())
	copy(out, "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(body.Len()))
	return append(out, body.Bytes()...)
}

func le24(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }

// decodeWebP decodes a still WebP with x/image/webp, or the frames of an
// animated one composited on its canvas. x/image/webp reads still images
// only, so each frame of an animation becomes a still WebP of its own.
func decodeWebP(data []byte) (*Image, error) {
	end := len(data)
	if size := uint64(binary.LittleEndian.Uint32(data[4:])) + 8; size < uint64(end) {
		end = int(max(size, 12))
	}
	body := data[12:end]
	chunks := riffChunks(body)
	if len(chunks) == 0 || chunks[0].id != "VP8X" || len(chunks[0].data) < 10 || chunks[0].data[0]&0x02 == 0 {
		cfg, err := webp.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if err := checkBitmap(cfg.Width, cfg.Height); err != nil {
			return nil, err
		}
		m, err := webp.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return still(m), nil
	}
	vp8x := chunks[0].data
	cw, ch := 1+le24(vp8x[4:]), 1+le24(vp8x[7:])
	if cw*ch > MaxPixels {
		return nil, fmt.Errorf("%w: a %d×%d animation", ErrTooLarge, cw, ch)
	}
	img := &Image{
		Width:  cw,
		Height: ch,
		Size:   driver.Size{Width: driver.Points(cw), Height: driver.Points(ch)},
	}
	var frames [][]byte
	for _, c := range chunks[1:] {
		switch c.id {
		case "ANIM":
			// The background color is a hint browsers ignore, as this does.
			if len(c.data) >= 6 {
				img.Plays = int(binary.LittleEndian.Uint16(c.data[4:]))
			}
		case "ANMF":
			frames = append(frames, c.data)
		}
	}
	keep := min(len(frames), MaxFrames)
	if keep*cw*ch*4 > MaxAnimationBytes {
		keep = 1
	}
	canvas := image.NewRGBA(image.Rect(0, 0, cw, ch))
	var dispose image.Rectangle
	for i, f := range frames[:keep] {
		m, x, y, delay, flags, err := webpFrame(f)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			break // a damaged frame ends the animation; what decoded plays
		}
		draw.Draw(canvas, dispose, image.Transparent, image.Point{}, draw.Src)
		b := m.Bounds()
		r := image.Rect(x, y, x+b.Dx(), y+b.Dy()).Intersect(canvas.Rect)
		op := draw.Over
		if flags&0x02 != 0 {
			op = draw.Src
		}
		draw.Draw(canvas, r, m, b.Min.Add(r.Min.Sub(image.Pt(x, y))), op)
		img.Frames = append(img.Frames, Frame{Pix: bytes.Clone(canvas.Pix), Delay: frameDelay(delay)})
		dispose = image.Rectangle{}
		if flags&0x01 != 0 {
			dispose = r
		}
	}
	if len(img.Frames) == 0 {
		return nil, fmt.Errorf("%w: an animation without frames", errWebP)
	}
	if len(img.Frames) == 1 {
		img.Frames[0].Delay = 0
	}
	return img, nil
}

// webpFrame decodes an ANMF chunk: its picture, where it goes on the
// canvas, how long it shows, and its blending (0x02: none) and disposal
// (0x01: to transparent) flags.
func webpFrame(f []byte) (m image.Image, x, y int, delay time.Duration, flags byte, err error) {
	if len(f) < 16 {
		return nil, 0, 0, 0, 0, errWebP
	}
	x, y = 2*le24(f), 2*le24(f[3:])
	w, h := 1+le24(f[6:]), 1+le24(f[9:])
	delay = time.Duration(le24(f[12:])) * time.Millisecond
	flags = f[15]
	if w*h > MaxPixels {
		return nil, 0, 0, 0, 0, ErrTooLarge
	}
	var alph, vp8, vp8l []byte
	for _, c := range riffChunks(f[16:]) {
		switch c.id {
		case "ALPH":
			alph = c.data
		case "VP8 ":
			vp8 = c.data
		case "VP8L":
			vp8l = c.data
		}
	}
	var file []byte
	switch {
	case vp8l != nil:
		file = riff(riffChunk{"VP8L", vp8l})
	case vp8 != nil && alph != nil:
		header := make([]byte, 10)
		header[0] = 0x10 // alpha
		header[4], header[5], header[6] = byte(w-1), byte((w-1)>>8), byte((w-1)>>16)
		header[7], header[8], header[9] = byte(h-1), byte((h-1)>>8), byte((h-1)>>16)
		file = riff(riffChunk{"VP8X", header}, riffChunk{"ALPH", alph}, riffChunk{"VP8 ", vp8})
	case vp8 != nil:
		file = riff(riffChunk{"VP8 ", vp8})
	default:
		return nil, 0, 0, 0, 0, fmt.Errorf("%w: a frame without a picture", errWebP)
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(file))
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	if cfg.Width != w || cfg.Height != h {
		return nil, 0, 0, 0, 0, fmt.Errorf("%w: a %d×%d frame holds a %d×%d picture", errWebP, w, h, cfg.Width, cfg.Height)
	}
	m, err = webp.Decode(bytes.NewReader(file))
	return m, x, y, delay, flags, err
}
