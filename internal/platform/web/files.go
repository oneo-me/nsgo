//go:build js && wasm

package web

import (
	"errors"
	"io"
	"strings"
	"syscall/js"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// pickFiles shows the browser's file chooser for the files r offers; a
// picked file has no path, so it reads from its blob. A cancelled
// chooser answers with none.
func (s *surface) pickFiles(r driver.FilesRequest, id uint32) {
	input := s.doc.Call("createElement", "input")
	input.Set("type", "file")
	if len(r.Types) > 0 {
		input.Set("accept", strings.Join(r.Types, ","))
	}
	input.Set("multiple", r.Limit > 1)
	var change, cancel js.Func
	answered := false
	answer := func(files []driver.File) {
		if answered {
			return
		}
		answered = true
		change.Release()
		cancel.Release()
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Data: files})
	}
	change = js.FuncOf(func(js.Value, []js.Value) any {
		list := input.Get("files")
		var files []driver.File
		for i := range min(list.Length(), max(r.Limit, 1)) {
			blob := list.Index(i)
			size := int64(blob.Get("size").Float())
			files = append(files, driver.File{Name: blob.Get("name").String(), MIME: blob.Get("type").String(), Size: size,
				Modified: time.UnixMilli(int64(blob.Get("lastModified").Float())),
				Opener:   func() (io.ReadCloser, error) { return &blobReader{blob: blob, size: size}, nil }})
		}
		answer(files)
		return nil
	})
	cancel = js.FuncOf(func(js.Value, []js.Value) any { answer(nil); return nil })
	input.Call("addEventListener", "change", change)
	input.Call("addEventListener", "cancel", cancel)
	input.Call("click")
}

// blobReader reads a Blob a slice at a time, each a promise the reading
// goroutine waits for, so a large file never sits in memory whole.
type blobReader struct {
	blob      js.Value
	off, size int64
}

func (b *blobReader) Read(p []byte) (int, error) {
	if b.off >= b.size {
		return 0, io.EOF
	}
	n := min(int64(len(p)), b.size-b.off, 1<<20)
	type result struct {
		buf js.Value
		err error
	}
	done := make(chan result, 1)
	then(b.blob.Call("slice", b.off, b.off+n).Call("arrayBuffer"), func(buf js.Value) { done <- result{buf: buf} },
		func(e js.Value) { done <- result{err: jsError(e)} })
	res := <-done
	if res.err != nil {
		return 0, res.err
	}
	got := js.CopyBytesToGo(p, js.Global().Get("Uint8Array").New(res.buf))
	b.off += int64(got)
	return got, nil
}

func (*blobReader) Close() error { return nil }

// jsError is a rejection's reason as an error.
func jsError(e js.Value) error {
	if e.Type() == js.TypeObject && e.Get("message").Type() == js.TypeString {
		return errors.New(e.Get("message").String())
	}
	return errors.New(e.String())
}

// saveFile saves r's data where the user chose with the browser's save
// picker, which writes it whole or not at all, or downloads it where the
// browser has none or the press's user activation is gone.
func (s *surface) saveFile(r driver.SaveRequest, id uint32) {
	answer := func(saved bool, err error) {
		ev := driver.Event{Kind: driver.EventFiles, Count: id, Finished: saved}
		if err != nil {
			ev.Text = err.Error()
		}
		s.engine.Dispatch(ev)
	}
	src, err := r.Open()
	var data []byte
	if err == nil {
		data, err = io.ReadAll(src)
		src.Close()
	}
	if err != nil {
		answer(false, err)
		return
	}
	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	download := func() {
		blob := js.Global().Get("Blob").New([]any{buf}, map[string]any{"type": r.MIME})
		url := js.Global().Get("URL").Call("createObjectURL", blob)
		a := s.doc.Call("createElement", "a")
		a.Set("href", url)
		a.Set("download", r.Name)
		a.Call("click")
		time.AfterFunc(time.Minute, func() { js.Global().Get("URL").Call("revokeObjectURL", url) })
		answer(true, nil)
	}
	picker := js.Global().Get("showSaveFilePicker")
	if picker.Type() != js.TypeFunction {
		download()
		return
	}
	fail := func(e js.Value) { answer(false, jsError(e)) }
	then(picker.Invoke(map[string]any{"suggestedName": r.Name}), func(handle js.Value) {
		then(handle.Call("createWritable"), func(w js.Value) {
			then(w.Call("write", buf), func(js.Value) {
				then(w.Call("close"), func(js.Value) { answer(true, nil) }, fail)
			}, func(e js.Value) {
				w.Call("abort") // leaves the file as it was
				fail(e)
			})
		}, fail)
	}, func(e js.Value) {
		name := ""
		if e.Type() == js.TypeObject {
			name = e.Get("name").String()
		}
		switch name {
		case "AbortError":
			answer(false, nil)
		case "SecurityError", "NotAllowedError":
			download()
		default:
			fail(e)
		}
	})
}
