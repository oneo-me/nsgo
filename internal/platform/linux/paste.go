//go:build linux && !android

package linux

import (
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The caret and pastes of text inputs. The caret is GTK's own: the
// entry's text widget computes its extents, a text view its cursor
// locations. A paste is heard from the editor's paste-clipboard signal;
// GDK reads clipboards asynchronously, so the paste is reported after
// the text it inserts, with the clipboard's local files, else its image,
// and its text.

type gdkRectangle struct{ x, y, w, h int32 }

// pasting is the paste being read.
type pasting struct {
	v    *view
	clip uintptr
	ev   *driver.PasteEvent
}

var (
	gtkTextComputeCursorExtents     func(uintptr, uintptr, *grapheneRect, *grapheneRect)
	gtkEditableGetPosition          func(uintptr) int32
	gtkTextViewGetCursorLocations   func(uintptr, uintptr, *gdkRectangle, *gdkRectangle)
	gtkTextViewBufferToWindowCoords func(uintptr, int32, int32, int32, *int32, *int32)
	gtkWidgetGetClipboard           func(uintptr) uintptr
	gdkClipboardGetFormats          func(uintptr) uintptr
	gdkContentFormatsContainGtype   func(uintptr, uintptr) bool
	gdkTextureGetType               func() uintptr
	gdkClipboardReadValueAsync      func(uintptr, uintptr, int32, uintptr, uintptr, uintptr)
	gdkClipboardReadValueFinish     func(uintptr, uintptr, uintptr) uintptr
	gdkClipboardReadTextureAsync    func(uintptr, uintptr, uintptr, uintptr)
	gdkClipboardReadTextureFinish   func(uintptr, uintptr, uintptr) uintptr
	gdkTextureSaveToPngBytes        func(uintptr) uintptr
	gBytesGetData                   func(uintptr, *uintptr) uintptr
	gBytesUnref                     func(uintptr)
	cbPasteClipboard, cbPasteFiles  uintptr
	cbPasteTexture, cbPasteClipText uintptr
)

func bindPaste() {
	gtk := func(fn any, name string) { purego.RegisterLibFunc(fn, gtkLib, name) }
	gtk(&gtkTextComputeCursorExtents, "gtk_text_compute_cursor_extents")
	gtk(&gtkEditableGetPosition, "gtk_editable_get_position")
	gtk(&gtkTextViewGetCursorLocations, "gtk_text_view_get_cursor_locations")
	gtk(&gtkTextViewBufferToWindowCoords, "gtk_text_view_buffer_to_window_coords")
	gtk(&gtkWidgetGetClipboard, "gtk_widget_get_clipboard")
	gtk(&gdkClipboardGetFormats, "gdk_clipboard_get_formats")
	gtk(&gdkContentFormatsContainGtype, "gdk_content_formats_contain_gtype")
	gtk(&gdkTextureGetType, "gdk_texture_get_type")
	gtk(&gdkClipboardReadValueAsync, "gdk_clipboard_read_value_async")
	gtk(&gdkClipboardReadValueFinish, "gdk_clipboard_read_value_finish")
	gtk(&gdkClipboardReadTextureAsync, "gdk_clipboard_read_texture_async")
	gtk(&gdkClipboardReadTextureFinish, "gdk_clipboard_read_texture_finish")
	gtk(&gdkTextureSaveToPngBytes, "gdk_texture_save_to_png_bytes")
	purego.RegisterLibFunc(&gBytesGetData, glibLib, "g_bytes_get_data")
	purego.RegisterLibFunc(&gBytesUnref, glibLib, "g_bytes_unref")
	cbPasteClipboard = newCallback(pasteClipboardCallback)
	cbPasteFiles = newCallback(pasteFilesCallback)
	cbPasteTexture = newCallback(pasteTextureCallback)
	cbPasteClipText = newCallback(pasteTextCallback)
}

// caretMoved reports where v's caret is, while its editor has the focus.
func (w *window) caretMoved(v *view) {
	if w.focus != v || v.entry == 0 || !v.events.Has(driver.EventCaretMove) {
		return
	}
	var at grapheneRect
	var from uintptr
	if v.textView != 0 {
		var strong, weak gdkRectangle
		gtkTextViewGetCursorLocations(v.textView, 0, &strong, &weak)
		var x, y int32
		gtkTextViewBufferToWindowCoords(v.textView, 1, strong.x, strong.y, &x, &y) // GTK_TEXT_WINDOW_WIDGET
		at, from = grapheneRect{float32(x), float32(y), float32(max(strong.w, 1)), float32(strong.h)}, v.textView
	} else if text := gtkEditableGetDelegate(v.entry); text != 0 {
		var weak grapheneRect
		gtkTextComputeCursorExtents(text, uintptr(gtkEditableGetPosition(v.entry)), &at, &weak)
		at.w, from = max(at.w, 1), text
	}
	ev := driver.Event{Kind: driver.EventCaretMove, ID: v.id}
	var out graphenePoint
	if at.h > 0 && gtkWidgetComputePoint(from, v.widget, &graphenePoint{at.x, at.y}, &out) != 0 {
		ev.Point = driver.Point{X: driver.Points(out.x), Y: driver.Points(out.y)}
		ev.Size = driver.Size{Width: driver.Points(at.w), Height: driver.Points(at.h)}
	}
	w.engine.Dispatch(ev)
}

// pasteClipboardCallback hears the user paste into an editor: it keeps
// the text out when the input only reports pastes, and reads the
// clipboard to report it.
func pasteClipboardCallback(widget uintptr, data uintptr) {
	w, v := editorOf(widget, data)
	if v == nil || !v.events.Has(driver.EventPaste) {
		return
	}
	if v.fs.Config.PasteBehavior == driver.PasteReport {
		gSignalStopEmissionByName(widget, cstr("paste-clipboard"))
	}
	clip := gtkWidgetGetClipboard(widget)
	if clip == 0 {
		return
	}
	w.paste = &pasting{v: v, clip: clip, ev: &driver.PasteEvent{}}
	formats := gdkClipboardGetFormats(clip)
	switch {
	case gdkContentFormatsContainGtype(formats, gdkFileListGetType()):
		gdkClipboardReadValueAsync(clip, gdkFileListGetType(), 0, 0, cbPasteFiles, w.handle)
	case gdkContentFormatsContainGtype(formats, gdkTextureGetType()):
		gdkClipboardReadTextureAsync(clip, 0, cbPasteTexture, w.handle)
	default:
		gdkClipboardReadTextAsync(clip, 0, cbPasteClipText, w.handle)
	}
}

func pasteFilesCallback(clip, result, data uintptr) {
	w := windowOf(data)
	if w == nil || w.paste == nil {
		return
	}
	if value := gdkClipboardReadValueFinish(clip, result, 0); value != 0 {
		w.paste.ev.URIs = fileURIs(value)
	}
	gdkClipboardReadTextAsync(clip, 0, cbPasteClipText, w.handle)
}

func pasteTextureCallback(clip, result, data uintptr) {
	w := windowOf(data)
	if w == nil || w.paste == nil {
		return
	}
	if texture := gdkClipboardReadTextureFinish(clip, result, 0); texture != 0 {
		if bytes := gdkTextureSaveToPngBytes(texture); bytes != 0 {
			var n uintptr
			if p := gBytesGetData(bytes, &n); p != 0 && n > 0 {
				b := make([]byte, n)
				copy(b, unsafe.Slice((*byte)(cPointer(p)), n))
				w.paste.ev.Files = []driver.PastedFile{{MIME: "image/png", Data: b}}
			}
			gBytesUnref(bytes)
		}
		gObjectUnref(texture)
	}
	gdkClipboardReadTextAsync(clip, 0, cbPasteClipText, w.handle)
}

func pasteTextCallback(clip, result, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing || w.paste == nil {
		return
	}
	p := w.paste
	w.paste = nil
	if s := gdkClipboardReadTextFinish(clip, result, 0); s != 0 {
		p.ev.Text = goString(s)
		gFree(s)
	}
	if !p.v.deleted {
		w.engine.Dispatch(driver.Event{Kind: driver.EventPaste, ID: p.v.id, Data: p.ev})
	}
}
