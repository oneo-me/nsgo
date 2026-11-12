//go:build linux && !android

package main

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
)

// The Linux host is GTK 4 through purego: a GtkWindow holding a status
// label, a button and the two NSGo views in boxes, run by its own
// GMainLoop.

func init() {
	// GTK runs on the thread that initialized it, which main starts on.
	runtime.LockOSThread()
}

var (
	gtkInit           func()
	gtkWindowNew      func() uintptr
	gtkWindowSetTitle func(uintptr, string)
	gtkWindowSetSize  func(uintptr, int32, int32)
	gtkWindowSetChild func(uintptr, uintptr)
	gtkWindowPresent  func(uintptr)
	gtkBoxNew         func(orientation, spacing int32) uintptr
	gtkBoxAppend      func(box, child uintptr)
	gtkBoxSetHomog    func(uintptr, int32)
	gtkLabelNew       func(string) uintptr
	gtkLabelSetText   func(uintptr, string)
	gtkButtonNew      func(string) uintptr
	gtkButtonSetLabel func(uintptr, string)
	gtkWidgetSetHexp  func(uintptr, int32)
	gtkWidgetSetMargs func(uintptr, int32)
	gObjectRefSink    func(uintptr) uintptr
	gObjectUnref      func(uintptr)
	gSignalConnect    func(instance uintptr, signal string, handler, data, notify uintptr, flags int32) uint64
	gIdleAdd          func(fn, data uintptr) uint32
	gMainLoopNew      func(ctx uintptr, running int32) uintptr
	gMainLoopRun      func(uintptr)
	gMainLoopQuit     func(uintptr)
)

// clicks are the host's button callbacks, by button.
var clicks = map[uintptr]func(){}

var clicked = purego.NewCallback(func(button, data uintptr) uintptr {
	if fn := clicks[button]; fn != nil {
		fn()
	}
	return 0
})

// queued are the functions onMain runs on GTK's thread.
var (
	queuedMu sync.Mutex
	queued   []func()
	runQueue = purego.NewCallback(func(uintptr) uintptr {
		queuedMu.Lock()
		fns := queued
		queued = nil
		queuedMu.Unlock()
		for _, fn := range fns {
			fn()
		}
		return 0 // G_SOURCE_REMOVE
	})
)

func runHost() {
	gtk, err := purego.Dlopen("libgtk-4.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	for name, fn := range map[string]any{
		"gtk_init": &gtkInit, "gtk_window_new": &gtkWindowNew, "gtk_window_set_title": &gtkWindowSetTitle,
		"gtk_window_set_default_size": &gtkWindowSetSize, "gtk_window_set_child": &gtkWindowSetChild,
		"gtk_window_present": &gtkWindowPresent, "gtk_box_new": &gtkBoxNew, "gtk_box_append": &gtkBoxAppend,
		"gtk_box_set_homogeneous": &gtkBoxSetHomog, "gtk_label_new": &gtkLabelNew, "gtk_label_set_text": &gtkLabelSetText,
		"gtk_button_new_with_label": &gtkButtonNew, "gtk_button_set_label": &gtkButtonSetLabel,
		"gtk_widget_set_hexpand": &gtkWidgetSetHexp, "gtk_widget_set_margin_start": &gtkWidgetSetMargs,
		"g_object_ref_sink": &gObjectRefSink, "g_object_unref": &gObjectUnref,
		"g_signal_connect_data": &gSignalConnect, "g_idle_add": &gIdleAdd, "g_main_loop_new": &gMainLoopNew,
		"g_main_loop_run": &gMainLoopRun, "g_main_loop_quit": &gMainLoopQuit,
	} {
		purego.RegisterLibFunc(fn, gtk, name)
	}
	gtkInit()
	loop := gMainLoopNew(0, 0)
	window := gtkWindowNew()
	gtkWindowSetTitle(window, "An app that embeds NSGo")
	gtkWindowSetSize(window, 900, 640)
	gSignalConnect(window, "close-request", purego.NewCallback(func(uintptr, uintptr) uintptr {
		gMainLoopQuit(loop)
		return 0
	}), 0, 0, 0)

	column := gtkBoxNew(1, 8)
	bar := gtkBoxNew(0, 8)
	label := gtkLabelNew("The host's own widgets are above; NSGo's below.")
	gtkWidgetSetHexp(label, 1)
	gtkWidgetSetMargs(label, 12)
	h := &host{
		status: func(s string) { gtkLabelSetText(label, s) },
		onMain: func(fn func()) {
			queuedMu.Lock()
			queued = append(queued, fn)
			queuedMu.Unlock()
			gIdleAdd(runQueue, 0)
		},
	}
	button := gtkButtonNew("Send a message")
	clicks[button] = h.send
	gSignalConnect(button, "clicked", clicked, 0, 0, 0)
	gtkBoxAppend(bar, label)
	gtkBoxAppend(bar, button)
	gtkBoxAppend(column, bar)

	views := gtkBoxNew(0, 0)
	gtkBoxSetHomog(views, 1)
	for _, v := range h.newViews() {
		gtkBoxAppend(views, v.Handle())
	}
	gtkBoxAppend(column, views)
	gtkWindowSetChild(window, column)
	gtkWindowPresent(window)
	gMainLoopRun(loop)
	h.close()
}

// badge is the host's badge: a GtkButton showing the app's count, which
// emits "tap" when clicked.
type badge struct{ button uintptr }

func newBadge(cx *core.NativeContext) core.NativeComponent {
	b := &badge{button: gObjectRefSink(gtkButtonNew("Badge 0"))}
	clicks[b.button] = func() { cx.Emit("tap", nil) }
	gSignalConnect(b.button, "clicked", clicked, 0, 0, 0)
	return b
}

func (b *badge) View() any { return b.button }

func (b *badge) SetProp(name string, value any) {
	if name == "count" {
		gtkButtonSetLabel(b.button, fmt.Sprintf("Badge %v", value))
	}
}

func (b *badge) Command(string, any) {}

func (b *badge) Close() {
	delete(clicks, b.button)
	gObjectUnref(b.button)
}
