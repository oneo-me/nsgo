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
	s.choose(func(input js.Value) {
		if len(r.Types) > 0 {
			input.Set("accept", strings.Join(r.Types, ","))
		}
		input.Set("multiple", r.Limit > 1)
	}, func(list js.Value) {
		var files []driver.File
		for i := range min(list.Length(), max(r.Limit, 1)) {
			blob := list.Index(i)
			size := int64(blob.Get("size").Float())
			files = append(files, driver.File{Name: blob.Get("name").String(), MIME: blob.Get("type").String(), Size: size,
				Modified: modified(blob),
				Opener:   func() (io.ReadCloser, error) { return &blobReader{blob: blob, size: size}, nil }})
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Data: files})
	})
}

// choose shows an <input type=file> that set configures and calls done
// with the files chosen, an empty list when cancelled.
func (s *surface) choose(set func(input js.Value), done func(list js.Value)) {
	input := s.doc.Call("createElement", "input")
	input.Set("type", "file")
	set(input)
	var change, cancel js.Func
	answered := false
	answer := func(list js.Value) {
		if answered {
			return
		}
		answered = true
		change.Release()
		cancel.Release()
		done(list)
	}
	change = js.FuncOf(func(js.Value, []js.Value) any { answer(input.Get("files")); return nil })
	cancel = js.FuncOf(func(js.Value, []js.Value) any { answer(js.Global().Get("Array").New()); return nil })
	input.Call("addEventListener", "change", change)
	input.Call("addEventListener", "cancel", cancel)
	input.Call("click")
}

// pickDirectories shows the browser's folder picker, which picks one
// folder: showDirectoryPicker, whose handle reads the folder as the app
// opens it, or where the browser has none an <input webkitdirectory>,
// which lists the folder whole and leaves out the folders holding no
// files. A cancelled picker answers with none.
func (s *surface) pickDirectories(id uint32) {
	answer := func(dirs []driver.Directory, err error) {
		ev := driver.Event{Kind: driver.EventFiles, Count: id, Data: dirs}
		if err != nil {
			ev.Text = err.Error()
		}
		s.engine.Dispatch(ev)
	}
	picker := js.Global().Get("showDirectoryPicker")
	if picker.Type() != js.TypeFunction {
		s.pickTree(answer)
		return
	}
	then(picker.Invoke(map[string]any{"mode": "read"}), func(h js.Value) {
		answer([]driver.Directory{{Name: h.Get("name").String(), FS: driver.FolderFS(handles{}, driver.FolderEntry{Handle: h})}}, nil)
	}, func(e js.Value) {
		if errorName(e) == "AbortError" {
			answer(nil, nil)
		} else {
			answer(nil, jsError(e))
		}
	})
}

// pickTree picks a folder with <input webkitdirectory>, whose files name
// their paths from the folder picked.
func (s *surface) pickTree(answer func([]driver.Directory, error)) {
	s.choose(func(input js.Value) { input.Set("webkitdirectory", true) }, func(list js.Value) {
		if list.Length() == 0 {
			answer(nil, nil)
			return
		}
		var name string
		files := make([]driver.TreeFile, list.Length())
		for i := range files {
			blob := list.Index(i)
			size := int64(blob.Get("size").Float())
			var rel string
			name, rel, _ = strings.Cut(blob.Get("webkitRelativePath").String(), "/")
			files[i] = driver.TreeFile{Path: rel, Size: size, Modified: modified(blob),
				Open: func() (io.ReadSeekCloser, error) { return &blobReader{blob: blob, size: size}, nil }}
		}
		answer([]driver.Directory{{Name: name, FS: driver.TreeFS(files)}}, nil)
	})
}

// handles reads a folder through its FileSystemDirectoryHandle, each
// entry's handle its FolderEntry's.
type handles struct{}

func (handles) List(_ string, e driver.FolderEntry) ([]driver.FolderEntry, error) {
	it := e.Handle.(js.Value).Call("values")
	var entries []driver.FolderEntry
	var files []any // the File of each file, in order
	for {
		next, err := await(it.Call("next"))
		if err != nil {
			return nil, err
		}
		if next.Get("done").Truthy() {
			break
		}
		h := next.Get("value")
		dir := h.Get("kind").String() == "directory"
		entries = append(entries, driver.FolderEntry{Name: h.Get("name").String(), Dir: dir, Handle: h})
		if !dir {
			files = append(files, h.Call("getFile"))
		}
	}
	// The files' sizes and times, read together.
	all, err := await(js.Global().Get("Promise").Call("all", files))
	if err != nil {
		return nil, err
	}
	n := 0
	for i := range entries {
		if e := &entries[i]; !e.Dir {
			blob := all.Index(n)
			n++
			e.Size, e.Modified = int64(blob.Get("size").Float()), modified(blob)
		}
	}
	return entries, nil
}

func (handles) Open(_ string, e driver.FolderEntry) (io.ReadSeekCloser, error) {
	blob, err := await(e.Handle.(js.Value).Call("getFile"))
	if err != nil {
		return nil, err
	}
	return &blobReader{blob: blob, size: int64(blob.Get("size").Float())}, nil
}

// await waits for promise on a goroutine other than the one running
// JavaScript's callbacks, which would never see it settle.
func await(promise js.Value) (js.Value, error) {
	type result struct {
		v   js.Value
		err error
	}
	done := make(chan result, 1)
	then(promise, func(v js.Value) { done <- result{v: v} }, func(e js.Value) { done <- result{err: jsError(e)} })
	r := <-done
	return r.v, r.err
}

// modified is when a File last changed.
func modified(blob js.Value) time.Time {
	return time.UnixMilli(int64(blob.Get("lastModified").Float()))
}

// errorName is a rejection's DOMException name, "" for another reason.
func errorName(e js.Value) string {
	if e.Type() == js.TypeObject && e.Get("name").Type() == js.TypeString {
		return e.Get("name").String()
	}
	return ""
}

// blobReader reads a Blob a slice at a time, each a promise the reading
// goroutine waits for, so a large file never sits in memory whole; it
// seeks anywhere.
type blobReader struct {
	blob      js.Value
	off, size int64
}

func (b *blobReader) Read(p []byte) (int, error) {
	if b.off >= b.size {
		return 0, io.EOF
	}
	n := min(int64(len(p)), b.size-b.off, 1<<20)
	buf, err := await(b.blob.Call("slice", b.off, b.off+n).Call("arrayBuffer"))
	if err != nil {
		return 0, err
	}
	got := js.CopyBytesToGo(p, js.Global().Get("Uint8Array").New(buf))
	b.off += int64(got)
	return got, nil
}

func (b *blobReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		offset += b.off
	case io.SeekEnd:
		offset += b.size
	}
	if offset < 0 {
		return 0, errors.New("seek before the start")
	}
	b.off = offset
	return offset, nil
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
		switch errorName(e) {
		case "AbortError":
			answer(false, nil)
		case "SecurityError", "NotAllowedError":
			download()
		default:
			fail(e)
		}
	})
}
