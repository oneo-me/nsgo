//go:build windows

package windows

import (
	"encoding/binary"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The caret and pastes of text fields, from NSGo's own editor: the caret
// it draws, and the clipboard it pastes from, read as the user pastes.

var (
	globalSize               = kernel32.NewProc("GlobalSize")
	registerClipboardFormatW = user32.NewProc("RegisterClipboardFormatW")
)

const cfDIB = 8

// caretMoved reports where in's caret is, while it has the focus.
func (in *textInput) caretMoved() {
	if !in.focused || !in.fs.Events.Has(driver.EventCaretMove) {
		return
	}
	var ev driver.Event
	if x, y, h, ok := in.caretRect(); ok {
		c := in.config.Content.Origin
		ev.Point = driver.Point{X: c.X + driver.Points(x), Y: c.Y + driver.Points(y)}
		ev.Size = driver.Size{Width: driver.Points(caretWidth()), Height: driver.Points(h)}
	}
	ev.Kind, ev.ID = driver.EventCaretMove, in.v.id
	in.w.engine.Dispatch(ev)
}

// pasteEvent reports a paste into in, and whether its text goes in.
func (w *window) pasteEvent(in *textInput) bool {
	if !in.fs.Events.Has(driver.EventPaste) {
		return true
	}
	p := &driver.PasteEvent{Text: w.clipboard()}
	if r, _, _ := openClipboard.Call(w.hwnd); r != 0 {
		if h, _, _ := getClipboardData.Call(cfHDrop); h != 0 {
			p.URIs = hdropFiles(h)
		}
		if len(p.URIs) == 0 {
			p.Files = clipboardImage()
		}
		closeClipboard.Call()
	}
	w.engine.Dispatch(driver.Event{Kind: driver.EventPaste, ID: in.v.id, Data: p})
	return p.Text != "" && in.config.PasteBehavior == driver.PasteInsert
}

// clipboardImage returns the open clipboard's image: its PNG, else its
// device-independent bitmap as a BMP file.
func clipboardImage() []driver.PastedFile {
	name, _ := syscall.UTF16PtrFromString("PNG")
	if png, _, _ := registerClipboardFormatW.Call(uintptr(unsafe.Pointer(name))); png != 0 {
		if h, _, _ := getClipboardData.Call(png); h != 0 {
			if b := globalBytes(h); len(b) > 0 {
				return []driver.PastedFile{{MIME: "image/png", Data: b}}
			}
		}
	}
	h, _, _ := getClipboardData.Call(cfDIB)
	dib := globalBytes(h)
	if len(dib) < 40 {
		return nil
	}
	// A BMP file is the DIB after a 14-byte header that says where its
	// pixels start: after the info header and the color table.
	header := binary.LittleEndian.Uint32(dib)
	bits, used := binary.LittleEndian.Uint16(dib[14:]), binary.LittleEndian.Uint32(dib[32:])
	compression := binary.LittleEndian.Uint32(dib[16:])
	colors := used
	if colors == 0 && bits <= 8 {
		colors = 1 << bits
	}
	offset := 14 + header + colors*4
	if compression == 3 && header == 40 { // BI_BITFIELDS: three masks follow
		offset += 12
	}
	file := make([]byte, 14, 14+len(dib))
	file[0], file[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(file[2:], uint32(14+len(dib)))
	binary.LittleEndian.PutUint32(file[10:], offset)
	return []driver.PastedFile{{MIME: "image/bmp", Data: append(file, dib...)}}
}

// globalBytes copies the memory of global handle h.
func globalBytes(h uintptr) []byte {
	if h == 0 {
		return nil
	}
	n, _, _ := globalSize.Call(h)
	p, _, _ := globalLock.Call(h)
	if p == 0 || n == 0 {
		return nil
	}
	defer globalUnlock.Call(h)
	b := make([]byte, n)
	rtlMoveMemory.Call(uintptr(unsafe.Pointer(&b[0])), p, n)
	return b
}

// hdropFiles reads an HDROP list as file: URLs.
func hdropFiles(h uintptr) []string {
	count, _, _ := dragQueryFileW.Call(h, 0xFFFFFFFF, 0, 0)
	var uris []string
	for i := range uint32(count) {
		n, _, _ := dragQueryFileW.Call(h, uintptr(i), 0, 0)
		if n == 0 {
			continue
		}
		buf := make([]uint16, n+1)
		if got, _, _ := dragQueryFileW.Call(h, uintptr(i), uintptr(unsafe.Pointer(&buf[0])), n+1); got != n {
			continue
		}
		if uri := fileURL(syscall.UTF16ToString(buf)); uri != "" {
			uris = append(uris, uri)
		}
	}
	return uris
}
