package picked

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// withOrientation returns a JPEG of m carrying EXIF orientation turn.
func withOrientation(t *testing.T, m image.Image, turn uint16) []byte {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, m, nil); err != nil {
		t.Fatal(err)
	}
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01")
	entry := make([]byte, 12)
	binary.BigEndian.PutUint16(entry, 0x0112)
	binary.BigEndian.PutUint16(entry[2:], 3)
	binary.BigEndian.PutUint32(entry[4:], 1)
	binary.BigEndian.PutUint16(entry[8:], turn)
	seg := append([]byte("Exif\x00\x00"), append(append(tiff, entry...), 0, 0, 0, 0)...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(seg)+2))
	out := append([]byte{0xFF, 0xD8}, append(app1, seg...)...)
	return append(out, b.Bytes()[2:]...)
}

func picture(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{0, 0, 255, 255}
			if x < w/2 {
				c = color.RGBA{255, 0, 0, 255}
			}
			m.Set(x, y, c)
		}
	}
	return m
}

func TestPrepare(t *testing.T) {
	photo := withOrientation(t, picture(40, 20), 6)
	if got := orientation(photo); got != 6 {
		t.Fatalf("orientation = %d", got)
	}
	var shot bytes.Buffer
	png.Encode(&shot, picture(400, 100))

	// Kept as they are, with their sizes as they show.
	files := Prepare(driver.PickRequest{Limit: 5}, []driver.PickedFile{{Name: "IMG_1.JPG", Data: photo}, {Name: "shot", Data: shot.Bytes()}})
	if len(files) != 2 || !bytes.Equal(files[0].Data, photo) || files[0].Size != (driver.Size{Width: 20, Height: 40}) ||
		files[1].Name != "shot.png" || files[1].Size != (driver.Size{Width: 400, Height: 100}) {
		t.Fatalf("kept = %+v", files)
	}

	// Scaled down: turned upright, a PNG stays a PNG.
	files = Prepare(driver.PickRequest{Limit: 1, MaxDimension: 10}, []driver.PickedFile{{Name: "IMG_1.JPG", Data: photo},
		{Name: "second.png", Data: shot.Bytes()}})
	if len(files) != 1 || files[0].MIME != "image/jpeg" || files[0].Size != (driver.Size{Width: 5, Height: 10}) {
		t.Fatalf("scaled = %+v", files)
	}
	m, err := jpeg.Decode(bytes.NewReader(files[0].Data))
	if err != nil || m.Bounds().Dx() != 5 || m.Bounds().Dy() != 10 {
		t.Fatalf("decoded %v, %v", m.Bounds(), err)
	}
	// Orientation 6 turns clockwise: the left half, red, ends on top.
	if r, _, b, _ := m.At(2, 1).RGBA(); r < b {
		t.Fatalf("the photo was not turned upright: %v", m.At(2, 1))
	}
	files = Prepare(driver.PickRequest{Limit: 1, MaxDimension: 100}, []driver.PickedFile{{Name: "s.png", Data: shot.Bytes()}})
	if files[0].MIME != "image/png" || files[0].Size != (driver.Size{Width: 100, Height: 25}) {
		t.Fatalf("png = %+v", files[0])
	}
	// A quality encodes JPEGs again.
	files = Prepare(driver.PickRequest{Limit: 1, Quality: 0.2}, []driver.PickedFile{{Name: "a.jpg", Data: photo}})
	if bytes.Equal(files[0].Data, photo) || files[0].Size != (driver.Size{Width: 20, Height: 40}) {
		t.Fatalf("requality = %+v", files[0].Size)
	}
}
