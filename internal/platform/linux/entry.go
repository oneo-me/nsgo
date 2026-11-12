//go:build linux && !android

package linux

import (
	"runtime"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// setClipboard puts text on the clipboard.
func (w *window) setClipboard(text string) {
	clip := gdkDisplayGetClipboard(gdkDisplayGetDefault())
	if clip == 0 {
		return
	}
	gdkClipboardSetText(clip, cstr(text))
}

// showAlert queues a native alert and shows the next one when the last
// has been answered.
func (w *window) showAlert(a *driver.AlertConfig) {
	w.alerts = append(w.alerts, *a)
	if !w.alerting {
		w.nextAlert()
	}
}

// nextAlert shows the first queued alert, if any.
func (w *window) nextAlert() {
	if w.alerting || len(w.alerts) == 0 {
		return
	}
	a := w.alerts[0]
	w.alerts = w.alerts[1:]
	w.alerting, w.alertingID, w.alertShown = true, a.ID, a
	// The constructor formats its message: a NULL format crashes GTK.
	dialog := gtkAlertDialogNew(cstr(""))
	if a.Title != "" {
		gtkAlertDialogSetMessage(dialog, cstr(a.Title))
	}
	if a.Message != "" {
		gtkAlertDialogSetDetail(dialog, cstr(a.Message))
	}
	labels := make([]string, len(a.Actions))
	cancel, def := -1, 0
	for i, act := range a.Actions {
		labels[i] = act.Label
		if act.Role == driver.AlertCancel && cancel < 0 {
			cancel = i
		}
		if act.Role == driver.AlertDefault {
			def = i
		}
	}
	if len(labels) > 0 {
		strs := stringArray(labels)
		gtkAlertDialogSetButtons(dialog, strs.array())
		runtime.KeepAlive(strs)
		gtkAlertDialogSetDefaultButton(dialog, int32(def))
		if cancel >= 0 {
			gtkAlertDialogSetCancelButton(dialog, int32(cancel))
		}
	}
	gtkAlertDialogChoose(dialog, w.toplevel(), 0, cbAlertChosen, w.handle)
}

func alertChosenCallback(dialog uintptr, result uintptr, data uintptr) {
	w := windowOf(data)
	if w == nil {
		gObjectUnref(dialog)
		return
	}
	index := int(gtkAlertDialogChooseFinish(dialog, result, 0))
	gObjectUnref(dialog)
	if !w.closing {
		w.engine.Dispatch(driver.Event{Kind: driver.EventAlert, Count: w.alertingID, Index: index})
	}
	w.alerting = false
	w.nextAlert()
}

// goString reads a NUL-terminated C string.
func goString(p uintptr) string {
	if p == 0 {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(cPointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(cPointer(p)), n))
}
