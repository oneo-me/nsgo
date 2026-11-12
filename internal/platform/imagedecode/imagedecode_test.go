package imagedecode

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The WebP fixtures in testdata were written with Pillow:
//
//	Image.new("RGBA", (2, 2), (255, 0, 0, 255)).save("red-2x2.webp", lossless=True)
//	Image.new("RGBA", (2, 2), (0, 0, 255, 128)).save("blue-half-2x2.webp", lossless=True)
//	Image.new("RGBA", (16, 16), (0, 255, 0, 128)).save("green-half-lossy-16x16.webp", quality=90)
//	f0 = Image.new("RGBA", (8, 8), (255, 0, 0, 255))
//	f1 = f0.copy(); f1.paste((0, 0, 255, 255), (4, 4, 8, 8))
//	f2 = Image.new("RGBA", (8, 8), (0, 0, 0, 0)); f2.paste((0, 255, 0, 255), (2, 2, 6, 6))
//	f0.save("animated-8x8.webp", save_all=True, append_images=[f1, f2],
//		duration=[50, 120, 5], loop=2, lossless=True)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// px returns the premultiplied RGBA pixel at (x, y) of an RGBA frame.
func px(img *Image, frame, x, y int) [4]byte {
	p := img.Frames[frame].Pix[(y*img.Width+x)*4:]
	return [4]byte{p[0], p[1], p[2], p[3]}
}

func near(a, b [4]byte, tolerance int) bool {
	for i := range a {
		if d := int(a[i]) - int(b[i]); d > tolerance || d < -tolerance {
			return false
		}
	}
	return true
}

func decode(t *testing.T, data []byte, opts Options) *Image {
	t.Helper()
	if opts.Layout == BGRA {
		opts.Layout = RGBA
	}
	img, err := Decode(data, opts)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

var (
	transparent = [4]byte{}
	red         = [4]byte{255, 0, 0, 255}
	green       = [4]byte{0, 255, 0, 255}
	blue        = [4]byte{0, 0, 255, 255}
	black       = [4]byte{0, 0, 0, 255}
)

func gzipped(b []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(b)
	zw.Close()
	return buf.Bytes()
}

func TestSniff(t *testing.T) {
	var pngData, jpegData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	jpeg.Encode(&jpegData, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil)
	for _, c := range []struct {
		data string
		want Format
	}{
		{pngData.String(), PNG},
		{jpegData.String(), JPEG},
		{"GIF89a....", GIF},
		{string(readFixture(t, "red-2x2.webp")), WebP},
		{`<svg xmlns="http://www.w3.org/2000/svg"/>`, SVG},
		{"\xef\xbb\xbf<?xml version=\"1.0\"?>\n<!-- a comment -->\n<!DOCTYPE svg [ <!ENTITY a \"b>\"> ]>\n<svg>", SVG},
		{string(gzipped([]byte(`<?xml version="1.0"?><svg viewBox="0 0 1 1"/>`))), SVG},
		{`<svgx/>`, Unknown},
		{`<html><svg/></html>`, Unknown},
		{`<!-- unterminated <svg>`, Unknown},
		{"", Unknown},
	} {
		if got := Sniff([]byte(c.data)); got != c.want {
			t.Errorf("Sniff(%.40q) = %v, want %v", c.data, got, c.want)
		}
	}
}

func TestSize(t *testing.T) {
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 3, 2)))
	for _, c := range []struct {
		name string
		data []byte
		want driver.Size
	}{
		{"png", pngData.Bytes(), driver.Size{Width: 3, Height: 2}},
		{"webp", readFixture(t, "green-half-lossy-16x16.webp"), driver.Size{Width: 16, Height: 16}},
		{"animated webp", readFixture(t, "animated-8x8.webp"), driver.Size{Width: 8, Height: 8}},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10.4" height="20.6"/>`), driver.Size{Width: 10, Height: 21}},
	} {
		if got, err := Size(c.data); err != nil || got != c.want {
			t.Errorf("%s: Size = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
	if _, err := Size([]byte("BM....")); !errors.Is(err, ErrFormat) {
		t.Errorf("unknown format: err = %v, want ErrFormat", err)
	}
}

func TestDecodeUnknownFormat(t *testing.T) {
	if _, err := Decode([]byte("BM...."), Options{}); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestDecodePNGInLayouts(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	m.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	m.Set(1, 0, color.NRGBA{0, 0, 255, 128})
	var buf bytes.Buffer
	png.Encode(&buf, m)
	img := decode(t, buf.Bytes(), Options{Layout: RGBA})
	if img.Format != PNG || img.Width != 2 || img.Height != 1 || len(img.Frames) != 1 {
		t.Fatalf("image = %v %dx%d, %d frames", img.Format, img.Width, img.Height, len(img.Frames))
	}
	if got := px(img, 0, 0, 0); got != red {
		t.Errorf("red = %v", got)
	}
	// Premultiplied: half-transparent blue keeps half its blue.
	if got := px(img, 0, 1, 0); got != [4]byte{0, 0, 128, 128} {
		t.Errorf("half blue = %v", got)
	}
	bgra, err := Decode(buf.Bytes(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := bgra.Frames[0].Pix[:4]; !bytes.Equal(got, []byte{0, 0, 255, 255}) {
		t.Errorf("BGRA red = %v", got)
	}
}

func TestDecodeRefusesHugeBitmaps(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1, 1)))
	data := buf.Bytes()
	// Claim 10000×10000 pixels in IHDR, with its checksum fixed.
	binary.BigEndian.PutUint32(data[16:], 10000)
	binary.BigEndian.PutUint32(data[20:], 10000)
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	if _, err := Decode(data, Options{}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

// gifFrame is a frame of palette indexes: 0 transparent, 1 red, 2 green,
// 3 blue.
func gifFrame(r image.Rectangle, index ...uint8) *image.Paletted {
	pal := color.Palette{color.RGBA{}, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}}
	m := image.NewPaletted(r, pal)
	for i := range m.Pix {
		m.Pix[i] = index[i%len(index)]
	}
	return m
}

func encodeGIF(t *testing.T, g *gif.GIF) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestGIFDisposalAndDelays(t *testing.T) {
	g := &gif.GIF{
		Image: []*image.Paletted{
			gifFrame(image.Rect(0, 0, 4, 4), 1),    // red everywhere
			gifFrame(image.Rect(0, 0, 2, 2), 2),    // green top left, then cleared
			gifFrame(image.Rect(2, 2, 4, 4), 3),    // blue bottom right, then undone
			gifFrame(image.Rect(2, 0, 4, 2), 0, 2), // see-through, then green
		},
		Delay:     []int{0, 5, 1, 2},
		Disposal:  []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalPrevious, gif.DisposalNone},
		LoopCount: 2,
		Config:    image.Config{Width: 4, Height: 4},
	}
	img := decode(t, encodeGIF(t, g), Options{})
	if img.Format != GIF || len(img.Frames) != 4 || img.Width != 4 || img.Height != 4 {
		t.Fatalf("image = %v %dx%d, %d frames", img.Format, img.Width, img.Height, len(img.Frames))
	}
	if img.Plays != 3 {
		t.Errorf("plays = %d, want 3 (a loop count of 2 repeats twice)", img.Plays)
	}
	wantDelays := []time.Duration{100 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond, 20 * time.Millisecond}
	for i, f := range img.Frames {
		if f.Delay != wantDelays[i] {
			t.Errorf("frame %d delay = %v, want %v", i, f.Delay, wantDelays[i])
		}
	}
	check := func(frame int, want map[[2]int][4]byte) {
		t.Helper()
		for at, w := range want {
			if got := px(img, frame, at[0], at[1]); got != w {
				t.Errorf("frame %d at %v = %v, want %v", frame, at, got, w)
			}
		}
	}
	check(0, map[[2]int][4]byte{{0, 0}: red, {3, 3}: red})
	check(1, map[[2]int][4]byte{{0, 0}: green, {1, 1}: green, {2, 2}: red})
	// Frame 1 was disposed to transparent; frame 2 draws over that.
	check(2, map[[2]int][4]byte{{0, 0}: transparent, {1, 1}: transparent, {3, 3}: blue, {3, 0}: red})
	// Frame 2 was undone; frame 3's transparent pixels show what was there.
	check(3, map[[2]int][4]byte{{0, 0}: transparent, {3, 3}: red, {2, 0}: red, {3, 0}: green, {2, 1}: red, {3, 1}: green})
}

func TestGIFPlays(t *testing.T) {
	for loops, want := range map[int]int{0: 0, -1: 1, 1: 2, 5: 6} {
		if got := gifPlays(loops); got != want {
			t.Errorf("gifPlays(%d) = %d, want %d", loops, got, want)
		}
	}
	still := &gif.GIF{Image: []*image.Paletted{gifFrame(image.Rect(0, 0, 1, 1), 1)}, Delay: []int{0}, LoopCount: -1}
	img := decode(t, encodeGIF(t, still), Options{})
	if len(img.Frames) != 1 || img.Frames[0].Delay != 0 {
		t.Errorf("a still GIF has %d frames, delay %v", len(img.Frames), img.Frames[0].Delay)
	}
}

func TestGIFFrameLimits(t *testing.T) {
	g := &gif.GIF{Config: image.Config{Width: 1, Height: 1}}
	for range MaxFrames + 5 {
		g.Image = append(g.Image, gifFrame(image.Rect(0, 0, 1, 1), 1))
		g.Delay = append(g.Delay, 3)
	}
	img := decode(t, encodeGIF(t, g), Options{})
	if len(img.Frames) != MaxFrames {
		t.Errorf("%d frames, want MaxFrames", len(img.Frames))
	}
	// Four frames of a 2000×2000 canvas are over MaxAnimationBytes: only
	// the first decodes.
	big := &gif.GIF{Config: image.Config{Width: 2000, Height: 2000}}
	for i := range 4 {
		big.Image = append(big.Image, gifFrame(image.Rect(i, 0, i+1, 1), 1))
		big.Delay = append(big.Delay, 3)
	}
	img = decode(t, encodeGIF(t, big), Options{})
	if len(img.Frames) != 1 || img.Width != 2000 {
		t.Errorf("%d frames of %d pixels wide, want the first alone", len(img.Frames), img.Width)
	}
}

func TestWebPStill(t *testing.T) {
	img := decode(t, readFixture(t, "red-2x2.webp"), Options{})
	if img.Format != WebP || img.Width != 2 || len(img.Frames) != 1 || px(img, 0, 1, 1) != red {
		t.Fatalf("lossless = %v %dx%d %v", img.Format, img.Width, img.Height, px(img, 0, 1, 1))
	}
	img = decode(t, readFixture(t, "green-half-lossy-16x16.webp"), Options{})
	if got := px(img, 0, 8, 8); !near(got, [4]byte{0, 128, 0, 128}, 12) {
		t.Errorf("lossy with alpha = %v, want half green premultiplied", got)
	}
}

func TestWebPAnimation(t *testing.T) {
	img := decode(t, readFixture(t, "animated-8x8.webp"), Options{})
	if img.Width != 8 || img.Height != 8 || len(img.Frames) != 3 || img.Plays != 2 {
		t.Fatalf("animation = %dx%d, %d frames, %d plays", img.Width, img.Height, len(img.Frames), img.Plays)
	}
	wantDelays := []time.Duration{50 * time.Millisecond, 120 * time.Millisecond, 100 * time.Millisecond}
	for i, f := range img.Frames {
		if f.Delay != wantDelays[i] {
			t.Errorf("frame %d delay = %v, want %v", i, f.Delay, wantDelays[i])
		}
	}
	for y := range 8 {
		for x := range 8 {
			want := [3][4]byte{red, red, transparent}
			if x >= 4 && y >= 4 {
				want[1] = blue
			}
			if x >= 2 && x < 6 && y >= 2 && y < 6 {
				want[2] = green
			}
			for f := range 3 {
				if got := px(img, f, x, y); got != want[f] {
					t.Fatalf("frame %d at %d,%d = %v, want %v", f, x, y, got, want[f])
				}
			}
		}
	}
}

// anmf makes an ANMF chunk of the picture in a still WebP's file.
func anmf(t *testing.T, still []byte, x, y, w, h, ms int, flags byte) riffChunk {
	t.Helper()
	b := make([]byte, 16)
	put24 := func(at, v int) { b[at], b[at+1], b[at+2] = byte(v), byte(v>>8), byte(v>>16) }
	put24(0, x/2)
	put24(3, y/2)
	put24(6, w-1)
	put24(9, h-1)
	put24(12, ms)
	b[15] = flags
	var payload []byte
	for _, c := range riffChunks(still[12:]) {
		if c.id == "VP8X" {
			continue
		}
		payload = append(payload, riff(c)[12:]...)
	}
	return riffChunk{"ANMF", append(b, payload...)}
}

func TestWebPAnimationBlendingAndDisposal(t *testing.T) {
	redStill := readFixture(t, "red-2x2.webp")
	blueStill := readFixture(t, "blue-half-2x2.webp")
	greenStill := readFixture(t, "green-half-lossy-16x16.webp")
	header := make([]byte, 10)
	header[0] = 0x12 // animation and alpha
	header[4], header[7] = 15, 15
	const (
		dispose = 0x01
		noBlend = 0x02
	)
	file := riff(
		riffChunk{"VP8X", header},
		riffChunk{"ANIM", []byte{0, 0, 0, 0, 3, 0}},
		anmf(t, redStill, 0, 0, 2, 2, 0, dispose),
		anmf(t, blueStill, 2, 2, 2, 2, 40, 0),
		anmf(t, blueStill, 2, 2, 2, 2, 40, 0),
		anmf(t, blueStill, 2, 2, 2, 2, 40, noBlend),
		anmf(t, greenStill, 0, 0, 16, 16, 40, noBlend),
		riffChunk{"ANMF", append(make([]byte, 16), riff(riffChunk{"VP8L", []byte("damaged")})[12:]...)},
	)
	img := decode(t, file, Options{})
	if img.Width != 16 || img.Height != 16 || img.Plays != 3 {
		t.Fatalf("animation = %dx%d, %d plays", img.Width, img.Height, img.Plays)
	}
	// The damaged frame ends the animation; the frames before it play.
	if len(img.Frames) != 5 {
		t.Fatalf("%d frames, want 5", len(img.Frames))
	}
	if img.Frames[0].Delay != 100*time.Millisecond || img.Frames[1].Delay != 40*time.Millisecond {
		t.Errorf("delays = %v, %v", img.Frames[0].Delay, img.Frames[1].Delay)
	}
	halfBlue := [4]byte{0, 0, 128, 128}
	for _, c := range []struct {
		frame, x, y int
		want        [4]byte
	}{
		{0, 0, 0, red}, {0, 2, 2, transparent},
		// The red frame was disposed of to transparent.
		{1, 0, 0, transparent}, {1, 3, 3, halfBlue},
		// Blended over itself: alpha 128 + 128 × (1 - 128/255).
		{2, 3, 3, [4]byte{0, 0, 192, 192}},
		// Not blended: it replaces what was there.
		{3, 3, 3, halfBlue}, {3, 0, 0, transparent},
		{4, 0, 0, [4]byte{0, 128, 0, 128}}, {4, 3, 3, [4]byte{0, 128, 0, 128}},
	} {
		if got := px(img, c.frame, c.x, c.y); !near(got, c.want, 12) {
			t.Errorf("frame %d at %d,%d = %v, want %v", c.frame, c.x, c.y, got, c.want)
		}
	}
}

func TestWebPAnimationLimits(t *testing.T) {
	header := make([]byte, 10)
	header[0] = 0x02
	header[4], header[5], header[6] = 0xff, 0xff, 0x0f // 1Mi wide
	header[7] = 99
	file := riff(riffChunk{"VP8X", header}, riffChunk{"ANIM", make([]byte, 6)})
	if _, err := Decode(file, Options{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a huge canvas: err = %v, want ErrTooLarge", err)
	}
	header[4], header[5], header[6], header[7] = 1, 0, 0, 1
	file = riff(riffChunk{"VP8X", header}, riffChunk{"ANIM", make([]byte, 6)})
	if _, err := Decode(file, Options{}); err == nil {
		t.Error("an animation without frames decoded")
	}
}

func svgImage(t *testing.T, doc string, opts Options) *Image {
	t.Helper()
	img := decode(t, []byte(doc), opts)
	if img.Format != SVG || len(img.Frames) != 1 {
		t.Fatalf("image = %v, %d frames", img.Format, len(img.Frames))
	}
	return img
}

func TestSVGFillCoverage(t *testing.T) {
	img := svgImage(t, `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10">
		<rect x="2" y="2" width="6" height="6" fill="#f00"/>
		<rect x="2.5" y="9" width="1" height="1" fill="#00f"/>
	</svg>`, Options{Scale: 1})
	if img.Width != 10 || img.Height != 10 || img.Size.Width != 10 {
		t.Fatalf("raster %dx%d, size %v", img.Width, img.Height, img.Size)
	}
	for _, c := range []struct {
		x, y int
		want [4]byte
	}{
		{5, 5, red}, {2, 2, red}, {7, 7, red},
		{1, 5, transparent}, {8, 5, transparent}, {5, 1, transparent}, {0, 0, transparent},
	} {
		if got := px(img, 0, c.x, c.y); got != c.want {
			t.Errorf("at %d,%d = %v, want %v", c.x, c.y, got, c.want)
		}
	}
	// Half of each of two pixels: the coverage across a row is exact.
	for _, x := range []int{2, 3} {
		if got := px(img, 0, x, 9); !near(got, [4]byte{0, 0, 128, 128}, 1) {
			t.Errorf("half-covered pixel %d = %v", x, got)
		}
	}
}

func TestSVGFillRules(t *testing.T) {
	const rings = `M0 0H20V20H0Z M5 5H15V15H5Z`
	evenOdd := svgImage(t, `<svg width="20" height="20"><path fill-rule="evenodd" d="`+rings+`"/></svg>`, Options{Scale: 1})
	nonzero := svgImage(t, `<svg width="20" height="20"><path d="`+rings+`"/></svg>`, Options{Scale: 1})
	if got := px(evenOdd, 0, 10, 10); got != transparent {
		t.Errorf("even-odd hole = %v", got)
	}
	if got := px(evenOdd, 0, 2, 10); got != black {
		t.Errorf("even-odd ring = %v", got)
	}
	if got := px(nonzero, 0, 10, 10); got != black {
		t.Errorf("nonzero center = %v", got)
	}
	// Wound the other way, the inner square is a hole by either rule.
	hole := svgImage(t, `<svg width="20" height="20"><path d="M0 0H20V20H0Z M5 5V15H15V5Z"/></svg>`, Options{Scale: 1})
	if got := px(hole, 0, 10, 10); got != transparent {
		t.Errorf("nonzero hole = %v", got)
	}
}

func TestSVGStrokes(t *testing.T) {
	img := svgImage(t, `<svg width="20" height="20">
		<line x1="0" y1="10" x2="20" y2="10" stroke="#000" stroke-width="4"/>
		<line x1="5" y1="2" x2="15" y2="2" stroke="#f00" stroke-width="2" stroke-linecap="square"/>
		<line x1="5" y1="17" x2="15" y2="17" stroke="#00f" stroke-width="2"/>
	</svg>`, Options{Scale: 1})
	for y, want := range map[int][4]byte{7: transparent, 8: black, 11: black, 12: transparent} {
		if got := px(img, 0, 10, y); got != want {
			t.Errorf("4-wide stroke at row %d = %v, want %v", y, got, want)
		}
	}
	// A square cap reaches half the width past the end; a butt cap stops.
	if got := px(img, 0, 4, 2); got != red {
		t.Errorf("square cap = %v", got)
	}
	if got := px(img, 0, 3, 2); got != transparent {
		t.Errorf("past the square cap = %v", got)
	}
	if got := px(img, 0, 4, 17); got != transparent {
		t.Errorf("past the butt cap = %v", got)
	}
	if got := px(img, 0, 5, 17); got != blue {
		t.Errorf("butt cap = %v", got)
	}
}

func TestSVGJoinsUnion(t *testing.T) {
	// A closed square stroked thick: its pieces overlap at every corner,
	// and the union stays opaque, without seams.
	img := svgImage(t, `<svg width="40" height="40"><rect x="10" y="10" width="20" height="20" fill="none" stroke="#000" stroke-width="8"/></svg>`, Options{Scale: 1})
	for _, at := range [][2]int{{6, 6}, {10, 10}, {33, 33}, {20, 8}, {12, 20}} {
		if got := px(img, 0, at[0], at[1]); got != black {
			t.Errorf("stroke at %v = %v", at, got)
		}
	}
	for _, at := range [][2]int{{20, 20}, {5, 5}, {34, 34}} {
		if got := px(img, 0, at[0], at[1]); got != transparent {
			t.Errorf("outside the stroke at %v = %v", at, got)
		}
	}
	// Round joins round the outer corners.
	round := svgImage(t, `<svg width="40" height="40"><rect x="10" y="10" width="20" height="20" fill="none" stroke="#000" stroke-width="8" stroke-linejoin="round"/></svg>`, Options{Scale: 1})
	if got := px(round, 0, 6, 6); got != transparent {
		t.Errorf("round corner = %v", got)
	}
}

func TestSVGGradients(t *testing.T) {
	img := svgImage(t, `<svg width="100" height="20">
		<linearGradient id="g" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="100" y2="0">
			<stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/>
		</linearGradient>
		<rect width="100" height="10" fill="url(#g)"/>
		<rect y="10" width="100" height="10" fill="url(#r)"/>
		<radialGradient id="r" r="0.5"><stop offset="0" stop-color="#fff"/><stop offset="1" stop-color="#000"/></radialGradient>
	</svg>`, Options{Scale: 1})
	if got := px(img, 0, 0, 5); !near(got, red, 3) {
		t.Errorf("start = %v", got)
	}
	if got := px(img, 0, 99, 5); !near(got, blue, 3) {
		t.Errorf("end = %v", got)
	}
	if got := px(img, 0, 50, 5); !near(got, [4]byte{128, 0, 128, 255}, 3) {
		t.Errorf("middle = %v", got)
	}
	// The radial gradient in the second rect's box: white at its center,
	// black at its edge and padded past it.
	if got := px(img, 0, 50, 15); !near(got, [4]byte{255, 255, 255, 255}, 30) {
		t.Errorf("radial center = %v", got)
	}
	if got := px(img, 0, 0, 10); got != black {
		t.Errorf("radial corner = %v", got)
	}
}

func TestSVGTransformsAndOpacity(t *testing.T) {
	img := svgImage(t, `<svg width="20" height="20">
		<g transform="translate(10 0)"><rect width="5" height="5" fill="#f00"/></g>
		<rect width="10" height="2" fill="#00f" transform="rotate(90 0 0) translate(10 -2)"/>
		<rect x="15" y="15" width="5" height="5" fill="#000" fill-opacity="0.5"/>
	</svg>`, Options{Scale: 1})
	if got := px(img, 0, 12, 2); got != red {
		t.Errorf("translated = %v", got)
	}
	if got := px(img, 0, 2, 2); got != transparent {
		t.Errorf("before translation = %v", got)
	}
	// rotate(90) maps (x, y) to (-y, x): the rect spans x 0 to 2 and y 10
	// to 20.
	if got := px(img, 0, 1, 15); got != blue {
		t.Errorf("rotated = %v", got)
	}
	if got := px(img, 0, 17, 17); !near(got, [4]byte{0, 0, 0, 128}, 1) {
		t.Errorf("half opacity = %v", got)
	}
}

func TestSVGClipPath(t *testing.T) {
	img := svgImage(t, `<svg width="20" height="20">
		<clipPath id="c"><rect width="10" height="20"/></clipPath>
		<rect width="20" height="20" fill="#0f0" clip-path="url(#c)"/>
	</svg>`, Options{Scale: 1})
	if got := px(img, 0, 5, 5); got != green {
		t.Errorf("inside the clip = %v", got)
	}
	if got := px(img, 0, 15, 5); got != transparent {
		t.Errorf("outside the clip = %v", got)
	}
	// Nested clips intersect.
	img = svgImage(t, `<svg width="20" height="20">
		<clipPath id="left"><rect width="10" height="20"/></clipPath>
		<clipPath id="top"><rect x="5" width="15" height="10"/></clipPath>
		<g clip-path="url(#left)"><g clip-path="url(#top)"><rect width="20" height="20" fill="#0f0"/></g></g>
	</svg>`, Options{Scale: 1})
	for at, want := range map[[2]int][4]byte{{7, 5}: green, {2, 5}: transparent, {12, 5}: transparent, {7, 15}: transparent} {
		if got := px(img, 0, at[0], at[1]); got != want {
			t.Errorf("nested clips at %v = %v, want %v", at, got, want)
		}
	}
}

func TestSVGRasterSizes(t *testing.T) {
	doc := `<svg width="100" height="50"><rect width="100" height="50"/></svg>`
	for _, c := range []struct {
		opts   Options
		w, h   int
		reason string
	}{
		{Options{}, 400, 200, "Oversample pixels per point"},
		{Options{Scale: 2}, 200, 100, "the intrinsic size times the scale"},
		{Options{Cover: 30}, 60, 30, "the shorter side Cover long"},
	} {
		img := svgImage(t, doc, c.opts)
		if img.Width != c.w || img.Height != c.h {
			t.Errorf("%+v: %dx%d, want %dx%d: %s", c.opts, img.Width, img.Height, c.w, c.h, c.reason)
		}
		if img.Size.Width != 100 || img.Size.Height != 50 {
			t.Errorf("%+v: size %v, want the intrinsic 100×50 points", c.opts, img.Size)
		}
	}
	wide := svgImage(t, `<svg width="100" height="10"/>`, Options{Scale: 1000})
	if wide.Width != MaxSide || wide.Height != 819 {
		t.Errorf("%dx%d, want at most MaxSide", wide.Width, wide.Height)
	}
	big := svgImage(t, `<svg width="1000" height="1000" viewBox="0 0 1 1"/>`, Options{})
	if big.Width != MaxOversampledSide || big.Height != MaxOversampledSide {
		t.Errorf("oversampled %dx%d, want MaxOversampledSide", big.Width, big.Height)
	}
	huge := svgImage(t, `<svg width="8000" height="4000" viewBox="0 0 2 1"><rect width="2" height="1"/></svg>`, Options{Scale: 1})
	if huge.Width*huge.Height > MaxPixels || huge.Width != 5792 || px(huge, 0, 5791, 2895) != black {
		t.Errorf("%dx%d, want within MaxPixels", huge.Width, huge.Height)
	}
	// The raster fills the whole box, even when its sides round apart.
	odd := svgImage(t, `<svg width="3" height="1"><rect width="3" height="1"/></svg>`, Options{Scale: 3.4})
	if odd.Width != 10 || odd.Height != 3 || px(odd, 0, 9, 2) != black || px(odd, 0, 0, 0) != black {
		t.Errorf("%dx%d raster leaves its edge: %v", odd.Width, odd.Height, px(odd, 0, 9, 2))
	}
	size, err := SVGSize([]byte(`<svg width="10" viewBox="0 0 4 2"/>`))
	if err != nil || size.Width != 10 || size.Height != 5 {
		t.Errorf("SVGSize = %v, %v, want 10×5", size, err)
	}
}

func TestSVGCompressed(t *testing.T) {
	img := svgImage(t, string(gzipped([]byte(`<svg width="4" height="4"><rect width="4" height="4" fill="#00f"/></svg>`))), Options{Scale: 1})
	if got := px(img, 0, 1, 1); got != blue {
		t.Errorf(".svgz = %v", got)
	}
}

func TestSVGBounds(t *testing.T) {
	large := []byte(`<svg width="1" height="1">` + strings.Repeat(" ", MaxBytes) + `</svg>`)
	if _, err := Decode(large, Options{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a large document: err = %v, want ErrTooLarge", err)
	}
	if _, err := Decode(gzipped(large), Options{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a large .svgz: err = %v, want ErrTooLarge", err)
	}
	deep := `<svg width="1" height="1">` + strings.Repeat("<g>", 10000) + strings.Repeat("</g>", 10000) + `</svg>`
	if _, err := Decode([]byte(deep), Options{}); err == nil {
		t.Error("deeply nested groups decoded")
	}
	// Each level draws the one below ten times: a billion paths in a few
	// hundred bytes.
	var laughs strings.Builder
	laughs.WriteString(`<svg width="10" height="10"><path id="l0" d="M0 0h1v1z"/>`)
	for i := 1; i < 10; i++ {
		laughs.WriteString(`<g id="l` + string(rune('0'+i)) + `">`)
		for range 10 {
			laughs.WriteString(`<use href="#l` + string(rune('0'+i-1)) + `"/>`)
		}
		laughs.WriteString(`</g>`)
	}
	laughs.WriteString(`</svg>`)
	start := time.Now()
	if _, err := Decode([]byte(laughs.String()), Options{}); err == nil {
		t.Error("a billion uses decoded")
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("refusing a billion uses took %v", d)
	}
	// A use that draws itself draws once.
	if _, err := Decode([]byte(`<svg width="1" height="1"><g id="a"><use href="#a"/></g></svg>`), Options{}); err != nil {
		t.Errorf("a use cycle: %v", err)
	}
	if _, err := Decode([]byte(`<svg viewBox="0 0 0 0"/>`), Options{}); err == nil {
		t.Error("an SVG without area decoded")
	}
}

func TestSVGPathologicalGeometry(t *testing.T) {
	// Far-away and enormous curves stay cheap and draw nothing amiss.
	img := svgImage(t, `<svg width="10" height="10">
		<path d="M-1e30 -1e30 C1e30 -1e30 1e30 1e30 -1e30 1e30Z" fill="#f00"/>
		<path d="M0 0 C1e9 0 1e9 10 0 10 Z" fill="#00f" stroke="#0f0" stroke-width="1e-9"/>
	</svg>`, Options{Scale: 1})
	_ = img
}
