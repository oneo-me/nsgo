// Package picked prepares the images a user picked as a PickRequest
// asks, in pure Go, for the backends whose pickers hand over files as
// they are: Windows and Linux. It reads each file's type and size,
// applies a JPEG's EXIF orientation, scales what is larger than the
// request's MaxDimension and encodes again what needs it, as NSH's
// pickers do; Apple platforms, Android and the web do the same with their
// own codecs.
package picked

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Prepare returns files as r asks for them: at most r.Limit, each with
// its type, its size as it shows and its bytes encoded again when r
// needs it. A file that does not decode when it must be encoded again is
// kept as it is.
func Prepare(r driver.PickRequest, files []driver.PickedFile) []driver.PickedFile {
	files = driver.FinishPick(r, files, nil)
	for i := range files {
		prepare(r, &files[i])
	}
	return files
}

// ReadFiles reads the files at paths as picked files, at most r.Limit,
// leaving out those that cannot be read, and prepares them.
func ReadFiles(r driver.PickRequest, paths []string) []driver.PickedFile {
	var files []driver.PickedFile
	for _, p := range paths {
		if len(files) >= max(r.Limit, 1) {
			break
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		f := driver.PickedFile{Name: filepath.Base(p), Data: data}
		if info, err := os.Stat(p); err == nil {
			f.Modified = info.ModTime()
		}
		files = append(files, f)
	}
	return Prepare(r, files)
}

func prepare(r driver.PickRequest, f *driver.PickedFile) {
	if m := driver.SniffImage(f.Data); m != "" {
		f.MIME = m
	}
	turn := 1
	if f.MIME == "image/jpeg" {
		turn = orientation(f.Data)
	}
	if c, _, err := decodeConfig(f.Data, f.MIME); err == nil {
		f.Size = driver.Size{Width: driver.Points(c.Width), Height: driver.Points(c.Height)}
		if turn >= 5 {
			f.Size.Width, f.Size.Height = f.Size.Height, f.Size.Width
		}
	}
	enc := r.Encoding(f.MIME, f.Size)
	if enc == driver.PickKeep {
		return
	}
	m, err := decode(f.Data, f.MIME)
	if err != nil {
		return
	}
	m = orient(m, turn)
	size := r.ScaledSize(driver.Size{Width: driver.Points(m.Bounds().Dx()), Height: driver.Points(m.Bounds().Dy())})
	if w, h := int(size.Width), int(size.Height); w != m.Bounds().Dx() || h != m.Bounds().Dy() {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), m, m.Bounds(), xdraw.Src, nil)
		m = dst
	}
	var out bytes.Buffer
	mime := "image/jpeg"
	if enc == driver.PickPNG {
		mime = "image/png"
		err = png.Encode(&out, m)
	} else {
		err = jpeg.Encode(&out, opaque(m), &jpeg.Options{Quality: int(r.JPEGQuality()*100 + 0.5)})
	}
	if err != nil {
		return
	}
	f.Data, f.MIME = out.Bytes(), mime
	f.Size = driver.Size{Width: driver.Points(m.Bounds().Dx()), Height: driver.Points(m.Bounds().Dy())}
	f.Name = driver.PickedName(f.Name, mime)
}

func decodeConfig(data []byte, mime string) (image.Config, string, error) {
	if mime == "image/webp" {
		c, err := webp.DecodeConfig(bytes.NewReader(data))
		return c, "webp", err
	}
	return image.DecodeConfig(bytes.NewReader(data))
}

func decode(data []byte, mime string) (image.Image, error) {
	switch mime {
	case "image/webp":
		return webp.Decode(bytes.NewReader(data))
	case "image/gif":
		return gif.Decode(bytes.NewReader(data))
	}
	m, _, err := image.Decode(bytes.NewReader(data))
	return m, err
}

// opaque returns m over white, since JPEG has no alpha.
func opaque(m image.Image) image.Image {
	if _, ok := m.(*image.YCbCr); ok {
		return m
	}
	dst := image.NewRGBA(m.Bounds())
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), m, m.Bounds().Min, draw.Over)
	return dst
}

// orientation returns the EXIF orientation of a JPEG, 1 when it has
// none.
func orientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if marker == 0xDA || n < 2 || i+2+n > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+n]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return exifOrientation(seg[6:])
		}
		i += 2 + n
	}
	return 1
}

func exifOrientation(t []byte) int {
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return 1
	}
	count := int(order.Uint16(t[ifd:]))
	for k := range count {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if order.Uint16(t[e:]) == 0x0112 {
			if v := int(order.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient returns m turned and flipped upright for EXIF orientation turn.
func orient(m image.Image, turn int) image.Image {
	if turn <= 1 || turn > 8 {
		return m
	}
	b := m.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if turn >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range h {
		for x := range w {
			var dx, dy int
			switch turn {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, m.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
