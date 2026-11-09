//go:build darwin

package apple

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Encoding images with ImageIO, for the clipboard's PNGs and the photos
// a user picks: decoded upright, scaled down and encoded again as PNG or
// JPEG, HEIC included.

var (
	encodeOnce                 sync.Once
	cfDataCreateMutable        func(alloc uintptr, capacity int) uintptr
	cfDataGetLength            func(data uintptr) int
	cfDataGetBytePtr           func(data uintptr) *byte
	cgImageDestinationCreate   func(data, uti uintptr, count int, options uintptr) uintptr
	cgImageDestinationAddImage func(dest, image, properties uintptr)
	cgImageDestinationFinalize func(dest uintptr) bool
	keyLossyQuality            uintptr
	utiPNG, utiJPEG            uintptr
)

func loadEncode() {
	cf := load("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	io := load("/System/Library/Frameworks/ImageIO.framework/ImageIO")
	purego.RegisterLibFunc(&cfDataCreateMutable, cf, "CFDataCreateMutable")
	purego.RegisterLibFunc(&cfDataGetLength, cf, "CFDataGetLength")
	purego.RegisterLibFunc(&cfDataGetBytePtr, cf, "CFDataGetBytePtr")
	purego.RegisterLibFunc(&cgImageDestinationCreate, io, "CGImageDestinationCreateWithData")
	purego.RegisterLibFunc(&cgImageDestinationAddImage, io, "CGImageDestinationAddImage")
	purego.RegisterLibFunc(&cgImageDestinationFinalize, io, "CGImageDestinationFinalize")
	keyLossyQuality = constant(io, "kCGImageDestinationLossyCompressionQuality")
	utiPNG, utiJPEG = CFString("public.png"), CFString("public.jpeg")
}

// Bytes copies the bytes of an NSData.
func Bytes(data objc.ID) []byte {
	if data == 0 {
		return nil
	}
	n := objc.Send[uint](data, sel("length"))
	if n == 0 {
		return nil
	}
	b := make([]byte, n)
	copy(b, unsafe.Slice(objc.Send[*byte](data, sel("bytes")), n))
	return b
}

// NSData returns an autoreleased NSData holding a copy of b.
func NSData(b []byte) objc.ID {
	if len(b) == 0 {
		return objc.ID(Class("NSData")).Send(sel("data"))
	}
	return objc.ID(Class("NSData")).Send(sel("dataWithBytes:length:"), unsafe.Pointer(&b[0]), uint(len(b)))
}

// Encode decodes data upright, scales it so its longer side has at most
// maxPixels (0: its size) and encodes it as a PNG, or as a JPEG of
// quality when jpeg. It returns the bytes and their size in pixels.
func Encode(data []byte, maxPixels int, jpeg bool, quality float32) ([]byte, driver.Size, error) {
	encodeOnce.Do(loadEncode)
	if len(data) == 0 {
		return nil, driver.Size{}, errors.New("apple: empty image data")
	}
	cfData := cfDataCreate(0, &data[0], len(data))
	defer Release(cfData)
	src := cgImageSourceCreateWithData(cfData, 0)
	if src == 0 {
		return nil, driver.Size{}, errors.New("apple: unsupported image data")
	}
	defer Release(src)
	keys := []uintptr{keyAlways, keyUpright}
	values := []uintptr{cfTrue, cfTrue}
	if maxPixels <= 0 {
		// The thumbnail is as large as the image only when asked to be.
		if size, err := imageSize(data); err == nil {
			maxPixels = int(max(size.Width, size.Height))
		}
	}
	if maxPixels > 0 {
		n := float64Number(float64(maxPixels))
		defer Release(n)
		keys, values = append(keys, keyMax), append(values, n)
	}
	opts := dictionary(keys, values)
	defer Release(opts)
	img := cgImageSourceCreateThumbnail(src, 0, opts)
	if img == 0 {
		return nil, driver.Size{}, errors.New("apple: cannot decode the image")
	}
	defer Release(img)
	out := cfDataCreateMutable(0, 0)
	defer Release(out)
	uti := utiPNG
	if jpeg {
		uti = utiJPEG
	}
	dest := cgImageDestinationCreate(out, uti, 1, 0)
	if dest == 0 {
		return nil, driver.Size{}, errors.New("apple: cannot encode the image")
	}
	defer Release(dest)
	var props uintptr
	if jpeg {
		q := float64Number(float64(quality))
		defer Release(q)
		props = dictionary([]uintptr{keyLossyQuality}, []uintptr{q})
		defer Release(props)
	}
	cgImageDestinationAddImage(dest, img, props)
	if !cgImageDestinationFinalize(dest) {
		return nil, driver.Size{}, errors.New("apple: cannot encode the image")
	}
	n := cfDataGetLength(out)
	b := make([]byte, n)
	if n > 0 {
		copy(b, unsafe.Slice(cfDataGetBytePtr(out), n))
	}
	return b, driver.Size{Width: driver.Points(cgImageGetWidth(img)), Height: driver.Points(cgImageGetHeight(img))}, nil
}

// PNG returns data as a PNG and its size in pixels: itself when it is
// one.
func PNG(data []byte) ([]byte, driver.Size, error) {
	if driver.SniffImage(data) == "image/png" {
		size, err := imageSize(data)
		return data, size, err
	}
	return Encode(data, 0, false, 0)
}

// PreparePicked returns f as r asks for it: its type and size as it
// shows filled in, encoded again as PNG or JPEG when r needs it.
func PreparePicked(r driver.PickRequest, f driver.PickedFile) driver.PickedFile {
	if m := driver.SniffImage(f.Data); m != "" {
		f.MIME = m
	}
	if size, err := imageSize(f.Data); err == nil {
		f.Size = size
	}
	enc := r.Encoding(f.MIME, f.Size)
	if enc == driver.PickKeep {
		return f
	}
	maxPixels := 0
	if s := r.ScaledSize(f.Size); s != f.Size {
		maxPixels = int(max(s.Width, s.Height))
	}
	data, size, err := Encode(f.Data, maxPixels, enc == driver.PickJPEG, r.JPEGQuality())
	if err != nil {
		return f
	}
	f.Data, f.Size, f.MIME = data, size, "image/png"
	if enc == driver.PickJPEG {
		f.MIME = "image/jpeg"
	}
	f.Name = driver.PickedName(f.Name, f.MIME)
	return f
}
