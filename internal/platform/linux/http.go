//go:build linux && !android

package linux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

// Image downloads go through libsoup 3, which WebKitGTK and GNOME's
// apps share, with GIO's proxies and TLS.
func init() { imageload.HTTP = httpGet }

var soup struct {
	once sync.Once
	err  error

	sessionNew           func() uintptr
	messageNew           func(method, uri *byte) uintptr
	requestHeaders       func(msg uintptr) uintptr
	responseHeaders      func(msg uintptr) uintptr
	headersAppend        func(headers uintptr, name, value *byte)
	headersIterInit      func(iter *[3]uintptr, headers uintptr)
	headersIterNext      func(iter *[3]uintptr, name, value *uintptr) bool
	status               func(msg uintptr) uint32
	send                 func(session, msg, cancellable uintptr, gerr *uintptr) uintptr
	inputStreamRead      func(stream uintptr, buf unsafe.Pointer, count uintptr, cancellable uintptr, gerr *uintptr) int
	inputStreamClose     func(stream, cancellable uintptr, gerr *uintptr) bool
	cancellableNew       func() uintptr
	cancellableCancel    func(c uintptr)
	objectUnref, errFree func(uintptr)
}

// loadSoup opens libsoup and the GLib calls downloads make, once.
func loadSoup() error {
	soup.once.Do(func() {
		open := func(names ...string) uintptr {
			for _, name := range names {
				if h, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
					return h
				}
			}
			if soup.err == nil {
				soup.err = fmt.Errorf("linux: %s not found", names[0])
			}
			return 0
		}
		lib, gio, gobject, glib := open("libsoup-3.0.so.0", "libsoup-3.0.so"), open("libgio-2.0.so.0", "libgio-2.0.so"),
			open("libgobject-2.0.so.0", "libgobject-2.0.so"), open("libglib-2.0.so.0", "libglib-2.0.so")
		if soup.err != nil {
			return
		}
		purego.RegisterLibFunc(&soup.sessionNew, lib, "soup_session_new")
		purego.RegisterLibFunc(&soup.messageNew, lib, "soup_message_new")
		purego.RegisterLibFunc(&soup.requestHeaders, lib, "soup_message_get_request_headers")
		purego.RegisterLibFunc(&soup.responseHeaders, lib, "soup_message_get_response_headers")
		purego.RegisterLibFunc(&soup.headersAppend, lib, "soup_message_headers_append")
		purego.RegisterLibFunc(&soup.headersIterInit, lib, "soup_message_headers_iter_init")
		purego.RegisterLibFunc(&soup.headersIterNext, lib, "soup_message_headers_iter_next")
		purego.RegisterLibFunc(&soup.status, lib, "soup_message_get_status")
		purego.RegisterLibFunc(&soup.send, lib, "soup_session_send")
		purego.RegisterLibFunc(&soup.inputStreamRead, gio, "g_input_stream_read")
		purego.RegisterLibFunc(&soup.inputStreamClose, gio, "g_input_stream_close")
		purego.RegisterLibFunc(&soup.cancellableNew, gio, "g_cancellable_new")
		purego.RegisterLibFunc(&soup.cancellableCancel, gio, "g_cancellable_cancel")
		purego.RegisterLibFunc(&soup.objectUnref, gobject, "g_object_unref")
		purego.RegisterLibFunc(&soup.errFree, glib, "g_error_free")
	})
	return soup.err
}

// httpGet is imageload.HTTP: libsoup's synchronous API on one thread,
// with a session of its own, as a session is used from the thread that
// made it, and a GCancellable that ctx cancels.
func httpGet(ctx context.Context, url string, header imageload.Header, progress imageload.Progress) (*imageload.Response, error) {
	if err := loadSoup(); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	msg := soup.messageNew(cstr("GET"), cstr(url))
	if msg == 0 {
		return nil, errors.New("linux: not a URL: " + url)
	}
	defer soup.objectUnref(msg)
	headers := soup.requestHeaders(msg)
	for k, vs := range header {
		for _, v := range vs {
			soup.headersAppend(headers, cstr(k), cstr(v))
		}
	}
	session := soup.sessionNew()
	defer soup.objectUnref(session)
	// The cancellable outlives a cancel running as the request ends.
	cancellable := soup.cancellableNew()
	var mu sync.Mutex
	stop := context.AfterFunc(ctx, func() {
		mu.Lock()
		defer mu.Unlock()
		if cancellable != 0 {
			soup.cancellableCancel(cancellable)
		}
	})
	defer func() {
		stop()
		mu.Lock()
		defer mu.Unlock()
		soup.objectUnref(cancellable)
		cancellable = 0
	}()

	var gerr uintptr
	stream := soup.send(session, msg, cancellable, &gerr)
	if stream == 0 {
		return nil, soupError(ctx, gerr)
	}
	defer soup.objectUnref(stream)
	defer soup.inputStreamClose(stream, 0, nil)
	resp := &imageload.Response{Status: int(soup.status(msg)), Header: imageload.Header{}}
	var iter [3]uintptr
	soup.headersIterInit(&iter, soup.responseHeaders(msg))
	for {
		var name, value uintptr
		if !soup.headersIterNext(&iter, &name, &value) {
			break
		}
		resp.Header.Add(goString(name), goString(value))
	}
	length, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	var err error
	resp.Body, err = imageload.ReadAll(soupBody{ctx, stream, cancellable}, length, progress)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// soupBody reads a response's stream.
type soupBody struct {
	ctx                 context.Context
	stream, cancellable uintptr
}

func (b soupBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	var gerr uintptr
	n := soup.inputStreamRead(b.stream, unsafe.Pointer(&p[0]), uintptr(len(p)), b.cancellable, &gerr)
	switch {
	case n < 0:
		return 0, soupError(b.ctx, gerr)
	case n == 0:
		return 0, io.EOF
	}
	return n, nil
}

// soupError is ctx's error once it ended, else what gerr says, which it
// frees.
func soupError(ctx context.Context, gerr uintptr) error {
	msg := "linux: download failed"
	if gerr != 0 {
		msg = "linux: " + goString((*gError)(cPointer(gerr)).message)
		soup.errFree(gerr)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.New(msg)
}
