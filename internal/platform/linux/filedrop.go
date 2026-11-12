//go:build linux && !android

package linux

import (
	"net/url"
	"path/filepath"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// fileDrop is the surface's GtkDropTarget for files from other apps, as
// NSH's file_drop.rs has it: installed while a view takes them, it
// answers each move from the mounted views, never waiting for the
// engine. The destination is the nearest view taking files above the one
// under the pointer, within the newest modal.
type fileDrop struct {
	target uintptr // the GtkDropTarget, 0 while no view takes files
	views  int     // views whose events hold EventFileDrop
	over   *view   // the destination the drag is over
}

// GDK's drag actions.
const gdkActionCopy = 1 // GDK_ACTION_COPY

// setEvents applies v's events, counting the views that take files.
func (w *window) setEvents(v *view, events driver.EventMask) {
	v.fs.Events = events
	had, has := v.events.Has(driver.EventFileDrop), events.Has(driver.EventFileDrop)
	v.events = events
	w.setCursor(v)
	switch {
	case !had && has:
		w.drop.views++
	case had && !has:
		w.drop.views--
		if w.drop.over == v {
			w.setDropOver(nil)
		}
	}
}

// forgetDestination stops counting v, which is deleted: no event reaches
// it again, as its ID may name another view soon.
func (w *window) forgetDestination(v *view) {
	if w.drop.over == v {
		w.drop.over = nil
	}
	if v.events.Has(driver.EventFileDrop) {
		w.drop.views--
	}
	v.events = 0
}

// updateDropTarget installs the drop target while a view takes files and
// removes it when none does.
func (w *window) updateDropTarget() {
	switch d := &w.drop; {
	case d.views > 0 && d.target == 0:
		t := gtkDropTargetNew(gdkFileListGetType(), gdkActionCopy)
		gtkEventControllerSetPropagationPhase(t, propagationPhaseCapture)
		gSignalConnect(t, "enter", cbDropMotion, w.handle)
		gSignalConnect(t, "motion", cbDropMotion, w.handle)
		gSignalConnect(t, "leave", cbDropLeave, w.handle)
		gSignalConnect(t, "drop", cbDrop, w.handle)
		gtkWidgetAddController(w.surface, t)
		d.target = t
	case d.views <= 0 && d.target != 0:
		w.setDropOver(nil)
		gtkWidgetRemoveController(w.surface, d.target)
		d.target = 0
	}
}

// dropDestination returns the view files dropped at the surface point at
// would reach, or nil.
func (w *window) dropDestination(at driver.Point) *view {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	scope := topModal(root)
	if scope != root {
		origin := w.frameAt(scope.parent)
		at = driver.Point{X: at.X - origin.X, Y: at.Y - origin.Y}
	}
	for v := hit(scope, at, func(*view) bool { return true }); v != nil; v = v.parent {
		if v.events.Has(driver.EventFileDrop) && !v.props.Disabled {
			return v
		}
		if v == scope {
			break
		}
	}
	return nil
}

// topModal returns the newest modal shown under root, else root.
func topModal(root *view) *view {
	scope := root
	var find func(v *view)
	find = func(v *view) {
		if v.style.Hidden || v.style.Opacity <= 0 {
			return
		}
		if v.kind == driver.KindModal {
			scope = v
		}
		for _, c := range v.children {
			find(c)
		}
	}
	find(root)
	return scope
}

// setDropOver makes v the destination the drag is over, telling the old
// one it left and the new one it entered.
func (w *window) setDropOver(v *view) {
	old := w.drop.over
	if old == v {
		return
	}
	w.drop.over = v
	if old != nil && !old.deleted && old.events.Has(driver.EventFileDragLeave) {
		w.engine.Dispatch(driver.Event{Kind: driver.EventFileDragLeave, ID: old.id})
	}
	if v != nil && v.events.Has(driver.EventFileDragEnter) {
		w.engine.Dispatch(driver.Event{Kind: driver.EventFileDragEnter, ID: v.id})
	}
}

// dragOver follows a drag of files at the surface point at, and returns
// the action to offer: Copy over a destination, else none.
func (w *window) dragOver(at driver.Point) uint32 {
	v := w.dropDestination(at)
	w.setDropOver(v)
	if v == nil {
		return 0
	}
	return gdkActionCopy
}

// dropFiles delivers files dropped at at, and reports whether a view
// took them.
func (w *window) dropFiles(at driver.Point, uris []string) bool {
	v := w.dropDestination(at)
	if v == nil || len(uris) == 0 {
		w.setDropOver(nil)
		return false
	}
	w.engine.Dispatch(driver.Event{Kind: driver.EventFileDrop, ID: v.id, URIs: uris})
	// The drag is over.
	w.setDropOver(nil)
	return true
}

// fileURIs returns the file: URLs of the local files in a GValue holding
// a GdkFileList. Files without a local path (remote ones) are left out.
func fileURIs(value uintptr) []string {
	list := gValueGetBoxed(value)
	if list == 0 {
		return nil
	}
	files := gdkFileListGetFiles(list)
	defer gSlistFree(files)
	var uris []string
	for node := files; node != 0; node = (*gSList)(cPointer(node)).next {
		p := gFileGetPath((*gSList)(cPointer(node)).data)
		if p == 0 {
			continue
		}
		path := goString(p)
		gFree(p)
		if filepath.IsAbs(path) {
			uris = append(uris, fileURL(path))
		}
	}
	return uris
}

// fileURL is path as a file: URL, its spaces and other bytes escaped.
func fileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

// gSList is GLib's singly linked list node.
type gSList struct {
	data, next uintptr
}

func dropMotionCallback(target uintptr, x, y float64, data uintptr) uint32 {
	w := windowOf(data)
	if w == nil || w.closing {
		return 0
	}
	return w.dragOver(driver.Point{X: driver.Points(x), Y: driver.Points(y)})
}

func dropLeaveCallback(target uintptr, data uintptr) {
	if w := windowOf(data); w != nil && !w.closing {
		w.setDropOver(nil)
	}
}

func dropCallback(target, value uintptr, x, y float64, data uintptr) int32 {
	w := windowOf(data)
	if w == nil || w.closing {
		return 0
	}
	return boolInt(w.dropFiles(driver.Point{X: driver.Points(x), Y: driver.Points(y)}, fileURIs(value)))
}

// The bindings of file drops.
var (
	gtkDropTargetNew          func(uintptr, uint32) uintptr
	gtkWidgetRemoveController func(uintptr, uintptr)
	gdkFileListGetType        func() uintptr
	gdkFileListGetFiles       func(uintptr) uintptr
	gdkFileListNewFromArray   func(*uintptr, uintptr) uintptr
	gValueGetBoxed            func(uintptr) uintptr
	gValueInit                func(uintptr, uintptr) uintptr
	gValueTakeBoxed           func(uintptr, uintptr)
	gValueUnset               func(uintptr)
	gSlistFree                func(uintptr)
	gFileGetPath              func(uintptr) uintptr
)

// bindFileDrop registers the bindings of file drops; bind calls it.
func bindFileDrop() {
	gtk := func(fn any, name string) { purego.RegisterLibFunc(fn, gtkLib, name) }
	gtk(&gtkDropTargetNew, "gtk_drop_target_new")
	gtk(&gtkWidgetRemoveController, "gtk_widget_remove_controller")
	gtk(&gdkFileListGetType, "gdk_file_list_get_type")
	gtk(&gdkFileListGetFiles, "gdk_file_list_get_files")
	gtk(&gdkFileListNewFromArray, "gdk_file_list_new_from_array")
	purego.RegisterLibFunc(&gValueGetBoxed, gobjectLib, "g_value_get_boxed")
	purego.RegisterLibFunc(&gValueInit, gobjectLib, "g_value_init")
	purego.RegisterLibFunc(&gValueTakeBoxed, gobjectLib, "g_value_take_boxed")
	purego.RegisterLibFunc(&gValueUnset, gobjectLib, "g_value_unset")
	purego.RegisterLibFunc(&gSlistFree, glibLib, "g_slist_free")
	purego.RegisterLibFunc(&gFileGetPath, gioLib, "g_file_get_path")
}
