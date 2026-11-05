package vt

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"runtime"
	"testing"
)

// sendPNG transmits a PNG as an image of the Kitty graphics protocol
// (f=100), in chunks.
func sendPNG(term *Terminal, id int, data []byte) {
	enc := base64.StdEncoding.EncodeToString(data)
	var b bytes.Buffer
	for i := 0; i == 0 || i < len(enc); i += 4096 {
		more := 0
		if i+4096 < len(enc) {
			more = 1
		}
		part := enc[i:min(i+4096, len(enc))]
		if i == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=t,f=100,i=%d,q=2,m=%d;%s\x1b\\", id, more, part)
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, part)
		}
	}
	term.Write(b.Bytes())
}

// pngClaiming returns a PNG whose header claims w×h gray pixels, with
// almost none of them.
func pngClaiming(w, h int) []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(typ string, data []byte) {
		binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(typ)
		b.Write(data)
		binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), data...)))
	}
	ihdr := binary.BigEndian.AppendUint32(nil, uint32(w))
	ihdr = binary.BigEndian.AppendUint32(ihdr, uint32(h))
	chunk("IHDR", append(ihdr, 8, 0, 0, 0, 0)) // 8-bit gray
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(make([]byte, 64))
	zw.Close()
	chunk("IDAT", z.Bytes())
	chunk("IEND", nil)
	return b.Bytes()
}

// PNGs of every kind decode to RGBA, and one that claims more pixels than
// Ghostty takes is refused before it is decoded.
func TestPNGLimits(t *testing.T) {
	loadLib(t)
	term, err := NewTerminal(20, 5, Effects{})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Free()
	term.SetImageLimit(320 << 20)

	r := image.Rect(0, 0, 3, 2)
	nrgba := image.NewNRGBA(r)
	rgb := image.NewRGBA(r)
	gray := image.NewGray(r)
	pal := image.NewPaletted(r, color.Palette{color.NRGBA{1, 2, 3, 255}, color.NRGBA{200, 100, 50, 128}})
	deep := image.NewNRGBA64(r)
	for y := range 2 {
		for x := range 3 {
			c := color.NRGBA{uint8(40 * x), uint8(90 * y), 77, uint8(100 + 50*x)}
			nrgba.SetNRGBA(x, y, c)
			rgb.Set(x, y, color.NRGBA{c.R, c.G, c.B, 255})
			gray.SetGray(x, y, color.Gray{uint8(60*x + 30*y)})
			pal.SetColorIndex(x, y, uint8((x+y)%2))
			deep.Set(x, y, c)
		}
	}
	for i, img := range []image.Image{nrgba, rgb, gray, pal, deep} {
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		id := i + 1
		sendPNG(term, id, b.Bytes())
		got, ok := term.Image(uint32(id))
		if !ok {
			t.Errorf("image %d (%T) refused", id, img)
			continue
		}
		want := image.NewNRGBA(r)
		draw.Draw(want, r, img, image.Point{}, draw.Src)
		if got.Format != ImageRGBA || got.Width != 3 || got.Height != 2 || !bytes.Equal(got.Pixels, want.Pix) {
			t.Errorf("image %d (%T): format %d, %d×%d, pixels %v, want %v", id, img, got.Format, got.Width, got.Height, got.Pixels, want.Pix)
		}
	}

	// Ghostty's limits: no side over 10000 pixels.
	var b bytes.Buffer
	png.Encode(&b, image.NewGray(image.Rect(0, 0, maxImageSide+1, 1)))
	sendPNG(term, 10, b.Bytes())
	if _, ok := term.Image(10); ok {
		t.Error("an image 10001 pixels wide was taken")
	}
	b.Reset()
	png.Encode(&b, image.NewGray(image.Rect(0, 0, maxImageSide, 1)))
	sendPNG(term, 11, b.Bytes())
	if _, ok := term.Image(11); !ok {
		t.Error("an image 10000 pixels wide was refused")
	}

	// A header that claims 20000×20000 pixels, which decoding would
	// allocate before reading them: refused with next to no memory.
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	sendPNG(term, 12, pngClaiming(20000, 20000))
	runtime.ReadMemStats(&after)
	if _, ok := term.Image(12); ok {
		t.Error("an image of 20000×20000 pixels was taken")
	}
	if n := after.TotalAlloc - before.TotalAlloc; n > 16<<20 {
		t.Errorf("refusing it allocated %d MB", n>>20)
	}
}
