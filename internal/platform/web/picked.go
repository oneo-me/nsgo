//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The page prepares picked images and the images it copies with the
// browser's codecs, through createImageBitmap and a canvas, so it links
// no Go image codec: what package picked does on Windows and Linux.

// preparePicked returns files as r asks for them to done, as
// picked.Prepare does: each image's size as it shows, its EXIF
// orientation applied, then, when r asks, the image scaled to fit and
// encoded again. A file the browser cannot decode is kept as it is.
func preparePicked(r driver.PickRequest, files []driver.PickedFile, done func([]driver.PickedFile)) {
	files = driver.FinishPick(r, files, nil)
	var next func(i int)
	next = func(i int) {
		if i == len(files) {
			done(files)
			return
		}
		preparePickedFile(r, &files[i], func() { next(i + 1) })
	}
	next(0)
}

func preparePickedFile(r driver.PickRequest, f *driver.PickedFile, done func()) {
	if m := driver.SniffImage(f.Data); m != "" {
		f.MIME = m
	}
	decodeImage(f.Data, f.MIME, func(bitmap js.Value) {
		f.Size = driver.Size{Width: driver.Points(bitmap.Get("width").Int()), Height: driver.Points(bitmap.Get("height").Int())}
		enc := r.Encoding(f.MIME, f.Size)
		if enc == driver.PickKeep {
			bitmap.Call("close")
			done()
			return
		}
		mime, quality := "image/jpeg", float64(r.JPEGQuality())
		if enc == driver.PickPNG {
			mime = "image/png"
		}
		size := r.ScaledSize(f.Size)
		encodeImage(bitmap, int(size.Width), int(size.Height), mime, quality, func(data []byte) {
			f.Data, f.MIME, f.Size = data, mime, size
			f.Name = driver.PickedName(f.Name, mime)
			done()
		}, done)
	}, done)
}

// decodeImage decodes data, of type mime, upright as its EXIF says, to an
// ImageBitmap the caller closes.
func decodeImage(data []byte, mime string, done func(bitmap js.Value), fail func()) {
	create := js.Global().Get("createImageBitmap")
	if !create.Truthy() {
		fail()
		return
	}
	then(create.Invoke(blobOf(data, mime), map[string]any{"imageOrientation": "from-image"}),
		done, func(js.Value) { fail() })
}

// encodeImage draws bitmap, which it closes, at w by h pixels and encodes
// it as mime, a JPEG over white at quality from 0 to 1.
func encodeImage(bitmap js.Value, w, h int, mime string, quality float64, done func([]byte), fail func()) {
	var canvas js.Value
	offscreen := js.Global().Get("OffscreenCanvas")
	if offscreen.Truthy() {
		canvas = offscreen.New(w, h)
	} else {
		canvas = js.Global().Get("document").Call("createElement", "canvas")
		canvas.Set("width", w)
		canvas.Set("height", h)
	}
	ctx := canvas.Call("getContext", "2d")
	if !ctx.Truthy() {
		bitmap.Call("close")
		fail()
		return
	}
	ctx.Set("imageSmoothingQuality", "high")
	if mime == "image/jpeg" {
		ctx.Set("fillStyle", "#fff")
		ctx.Call("fillRect", 0, 0, w, h)
	}
	ctx.Call("drawImage", bitmap, 0, 0, w, h)
	bitmap.Call("close")
	read := func(blob js.Value) {
		if !blob.Truthy() {
			fail()
			return
		}
		then(blob.Call("arrayBuffer"), func(buf js.Value) {
			data := make([]byte, buf.Get("byteLength").Int())
			js.CopyBytesToGo(data, js.Global().Get("Uint8Array").New(buf))
			done(data)
		}, func(js.Value) { fail() })
	}
	if offscreen.Truthy() {
		then(canvas.Call("convertToBlob", map[string]any{"type": mime, "quality": quality}), read, func(js.Value) { fail() })
		return
	}
	var cb js.Func
	cb = js.FuncOf(func(_ js.Value, args []js.Value) any {
		cb.Release()
		read(args[0])
		return nil
	})
	canvas.Call("toBlob", cb, mime, quality)
}
