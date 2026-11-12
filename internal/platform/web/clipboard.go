//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// then calls done with what promise resolves to, or fail with why it is
// rejected.
func then(promise js.Value, done, fail func(js.Value)) {
	var ok, bad js.Func
	settle := func(fn func(js.Value), args []js.Value) any {
		ok.Release()
		bad.Release()
		arg := js.Undefined()
		if len(args) > 0 {
			arg = args[0]
		}
		fn(arg)
		return nil
	}
	ok = js.FuncOf(func(_ js.Value, args []js.Value) any { return settle(done, args) })
	bad = js.FuncOf(func(_ js.Value, args []js.Value) any { return settle(fail, args) })
	promise.Call("then", ok, bad)
}

func (s *surface) setClipboard(text string) {
	if c := js.Global().Get("navigator").Get("clipboard"); c.Truthy() {
		then(c.Call("writeText", text), func(js.Value) {}, func(js.Value) { s.copyByCommand(text) })
		return
	}
	s.copyByCommand(text)
}

// copyByCommand copies text through a selected text area, for pages
// without the Clipboard API, such as ones served over plain http.
func (s *surface) copyByCommand(text string) {
	focus := s.doc.Get("activeElement")
	area := s.doc.Call("createElement", "textarea")
	area.Set("value", text)
	setStyle(area, map[string]string{"position": "fixed", "left": "-10000px", "top": "0", "opacity": "0"})
	s.doc.Get("body").Call("appendChild", area)
	area.Call("select")
	s.doc.Call("execCommand", "copy")
	area.Call("remove")
	if focus.Truthy() {
		focus.Call("focus", map[string]any{"preventScroll": true})
	}
}

// readClipboard answers an ItemReadClipboard. The page reads the text,
// and the PNG browsers put images on the clipboard as; it has no files.
// Text it may not read reads as nothing, an image as a failure.
func (s *surface) readClipboard(it *driver.MountItem) {
	ev := driver.Event{Kind: driver.EventClipboard, Count: it.Count}
	answer := func() { s.engine.Dispatch(ev) }
	c := js.Global().Get("navigator").Get("clipboard")
	switch it.Clipboard {
	case driver.ClipboardReadText:
		if !c.Truthy() || !c.Get("readText").Truthy() {
			answer()
			return
		}
		then(c.Call("readText"), func(v js.Value) {
			if v.Type() == js.TypeString {
				ev.Text = v.String()
			}
			answer()
		}, func(js.Value) { answer() })
	case driver.ClipboardReadFiles:
		answer()
	default:
		if !c.Truthy() || !c.Get("read").Truthy() {
			answer()
			return
		}
		// An image the page holds but may not read fails its read.
		failed := func(err js.Value) {
			if it.Clipboard == driver.ClipboardReadImage {
				ev.Data = driver.ReadClipboardImage(nil, driver.Size{}, jsError(err))
			}
			answer()
		}
		then(c.Call("read"), func(items js.Value) {
			want := "text/plain"
			if it.Clipboard != driver.ClipboardReadHasText {
				want = "image/png"
			}
			for i := range items.Length() {
				item := items.Index(i)
				if !item.Get("types").Call("includes", want).Bool() {
					continue
				}
				if it.Clipboard != driver.ClipboardReadImage {
					ev.Finished = true
					break
				}
				then(item.Call("getType", want), func(blob js.Value) {
					then(blob.Call("arrayBuffer"), func(buf js.Value) {
						data := make([]byte, buf.Get("byteLength").Int())
						js.CopyBytesToGo(data, js.Global().Get("Uint8Array").New(buf))
						ev.Data = driver.ClipboardPNG(data)
						answer()
					}, failed)
				}, failed)
				return
			}
			answer()
		}, failed)
	}
}

// setClipboardImage puts the image data encodes on the clipboard as the
// PNG browsers take.
func (s *surface) setClipboardImage(data []byte) {
	c := js.Global().Get("navigator").Get("clipboard")
	item := js.Global().Get("ClipboardItem")
	if !c.Truthy() || !c.Get("write").Truthy() || !item.Truthy() {
		return
	}
	write := func(png []byte) {
		then(c.Call("write", []any{item.New(map[string]any{"image/png": blobOf(png, "image/png")})}),
			func(js.Value) {}, func(js.Value) {})
	}
	mime := driver.SniffImage(data)
	if mime == "image/png" {
		write(data)
		return
	}
	decodeImage(data, mime, func(bitmap js.Value) {
		encodeImage(bitmap, bitmap.Get("width").Int(), bitmap.Get("height").Int(), "image/png", 1, write, func() {})
	}, func() {})
}

// blobOf returns a Blob of data of type mime.
func blobOf(data []byte, mime string) js.Value {
	arr := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(arr, data)
	return js.Global().Get("Blob").New([]any{arr}, map[string]any{"type": mime})
}

// openURL opens a web page in a new tab, and has the browser hand any
// other link to the app that handles it. The browser tells only whether
// a tab opened, and a popup blocker answers no.
func (s *surface) openURL(u string, id uint32) {
	opened := true
	if scheme, _, _ := strings.Cut(u, ":"); scheme == "http" || scheme == "https" {
		w := js.Global().Call("open", u, "_blank")
		if opened = w.Truthy(); opened {
			w.Set("opener", js.Null())
		}
	} else {
		a := s.doc.Call("createElement", "a")
		a.Set("href", u)
		a.Call("click")
	}
	s.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: id, Finished: opened})
}

// pickImages shows the browser's file chooser for images and prepares
// the ones chosen as r asks; a cancelled chooser answers with none.
func (s *surface) pickImages(r driver.PickRequest, id uint32) { s.chooseImages(r, id, false) }

// takePhoto shows the file chooser of an input that asks for the rear
// camera, which mobile browsers open, and prepares the photo always
// encoded again, upright; others offer files.
func (s *surface) takePhoto(r driver.PickRequest, id uint32) {
	r.Quality = r.JPEGQuality()
	s.chooseImages(r, id, true)
}

// chooseImages shows the file chooser for images, or the camera.
func (s *surface) chooseImages(r driver.PickRequest, id uint32, camera bool) {
	input := s.doc.Call("createElement", "input")
	input.Set("type", "file")
	if camera {
		input.Set("accept", "image/*")
		input.Call("setAttribute", "capture", "environment")
	} else if r.Format == driver.PickCompatible {
		input.Set("accept", "image/jpeg,image/png,image/gif,image/webp")
	} else {
		input.Set("accept", "image/*")
	}
	input.Set("multiple", r.Limit > 1)
	var change, cancel js.Func
	answered := false
	answer := func(files []driver.PickedFile) {
		if answered {
			return
		}
		answered = true
		change.Release()
		cancel.Release()
		s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: id, Data: files})
	}
	change = js.FuncOf(func(js.Value, []js.Value) any {
		list := input.Get("files")
		n := min(list.Length(), max(r.Limit, 1))
		files := make([]driver.PickedFile, n)
		left := n
		if n == 0 {
			answer(nil)
			return nil
		}
		for i := range n {
			f := list.Index(i)
			files[i] = driver.PickedFile{Name: f.Get("name").String(), MIME: f.Get("type").String(),
				Modified: time.UnixMilli(int64(f.Get("lastModified").Float()))}
			then(f.Call("arrayBuffer"), func(buf js.Value) {
				files[i].Data = make([]byte, buf.Get("byteLength").Int())
				js.CopyBytesToGo(files[i].Data, js.Global().Get("Uint8Array").New(buf))
				if left--; left == 0 {
					preparePicked(r, files, answer)
				}
			}, func(js.Value) {
				if left--; left == 0 {
					preparePicked(r, files, answer)
				}
			})
		}
		return nil
	})
	cancel = js.FuncOf(func(js.Value, []js.Value) any { answer(nil); return nil })
	input.Call("addEventListener", "change", change)
	input.Call("addEventListener", "cancel", cancel)
	input.Call("click")
}

// setAppearance has the browser draw its own parts, such as form
// controls and scroll bars, in a, the appearance the app forces, or else
// as the page's host or the system's appearance asks. An app in an element
// sets its element's, leaving the page's to the page.
func (s *surface) setAppearance(a driver.Appearance) {
	s.forced = a
	if a == driver.AppearanceSystem {
		a = s.hosted
	}
	scheme := ""
	switch a {
	case driver.AppearanceLight:
		scheme = "light"
	case driver.AppearanceDark:
		scheme = "dark"
	}
	target := s.host
	if s.whole {
		target = s.doc.Get("documentElement")
	}
	target.Get("style").Set("colorScheme", scheme)
}

// setHostEnvironment follows the appearance the page's host sets.
func (s *surface) setHostEnvironment(env engine.HostEnvironment) {
	s.hosted = env.Appearance
	s.setAppearance(s.forced)
}

// announce has screen readers say text: each message is a new node of a
// live region, so repeating one is read again.
func (s *surface) announce(text string) {
	if !s.live.Truthy() {
		s.live = s.doc.Call("createElement", "div")
		s.live.Call("setAttribute", "role", "status")
		s.live.Call("setAttribute", "aria-live", "polite")
		setStyle(s.live, map[string]string{
			"position": "fixed", "width": "1px", "height": "1px", "overflow": "hidden",
			"clip-path": "inset(50%)", "white-space": "nowrap",
		})
		s.doc.Get("body").Call("appendChild", s.live)
	}
	msg := s.doc.Call("createElement", "div")
	msg.Set("textContent", text)
	s.live.Call("replaceChildren", msg)
}

// pickScreenColor opens the browser's EyeDropper, answering id.
func (s *surface) pickScreenColor(id uint32) {
	answer := func(data any, finished bool) {
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Finished: finished, Data: data})
	}
	then(js.Global().Get("EyeDropper").New().Call("open"), func(result js.Value) {
		if c, err := driver.ParseColor(result.Get("sRGBHex").String()); err == nil {
			answer(c, true)
			return
		}
		answer(nil, false)
	}, func(js.Value) { answer(nil, false) })
}
