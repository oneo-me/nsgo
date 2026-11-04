//go:build linux && !android

package linux

import "sync"

// systemDirection is GTK's default direction as the app started, the
// system's, before the app's replaced it.
var systemDirection = sync.OnceValue(func() int32 { return gtkWidgetGetDefaultDirection() })

// setDirection lays GTK's widgets out in the app's direction, as its
// language is written, whatever the system's: its menus, fields and
// native components. A standalone app makes it GTK's default, which every
// widget without one of its own follows; an embedded surface gives it to
// its own widgets, leaving the host's alone.
func (w *window) setDirection(rtl bool) {
	systemDirection()
	w.dir = gtkTextDirLTR
	if rtl {
		w.dir = gtkTextDirRTL
	}
	if !w.embedded {
		gtkWidgetSetDefaultDirection(w.dir)
	}
	if w.surface != 0 {
		gtkWidgetSetDirection(w.surface, w.dir)
	}
}
