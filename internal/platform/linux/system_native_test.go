//go:build linux && !android

package linux

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
)

var (
	gtkWidgetGetVisible          func(widget uintptr) int32
	gtkWindowGetToplevels        func() uintptr
	gTypeNameFromInstance        func(instance uintptr) uintptr
	gtkFileChooserSetFile        func(chooser, file, err uintptr) int32
	gtkDialogResponse            func(dialog uintptr, response int32)
	gtkFileChooserGetAction      func(chooser uintptr) int32
	gtkFileChooserSetCurrentName func(chooser uintptr, name *byte)
)

func init() {
	if openLibs() == nil {
		purego.RegisterLibFunc(&gtkWidgetGetVisible, gtkLib, "gtk_widget_get_visible")
		purego.RegisterLibFunc(&gtkWindowGetToplevels, gtkLib, "gtk_window_get_toplevels")
		purego.RegisterLibFunc(&gTypeNameFromInstance, gobjectLib, "g_type_name_from_instance")
		purego.RegisterLibFunc(&gtkFileChooserSetFile, gtkLib, "gtk_file_chooser_set_file")
		purego.RegisterLibFunc(&gtkDialogResponse, gtkLib, "gtk_dialog_response")
		purego.RegisterLibFunc(&gtkFileChooserGetAction, gtkLib, "gtk_file_chooser_get_action")
		purego.RegisterLibFunc(&gtkFileChooserSetCurrentName, gtkLib, "gtk_file_chooser_set_current_name")
	}
}

// chooserDialog returns the file chooser dialog GtkFileDialog shows in
// the process when it uses no portal, or 0; the newest when several are.
func chooserDialog() uintptr {
	list := gtkWindowGetToplevels()
	var found uintptr
	for i := range gListModelGetNItems(list) {
		win := gListModelGetItem(list, i)
		gObjectUnref(win) // the list keeps it
		if goString(gTypeNameFromInstance(win)) == "GtkFileChooserDialog" && gtkWidgetGetVisible(win) != 0 {
			found = win
		}
	}
	return found
}

// TestServicesReachGTK puts an image on the clipboard and reads it back,
// forces the appearance, sizes the window, hides its menu bar until Alt,
// shows a context menu and quits.
func TestServicesReachGTK(t *testing.T) {
	scopes := make(chan *core.Scope, 1)
	status := make(chan string, 16)
	app := func(c *core.Scope) core.Node {
		scopes <- c
		c.SetApplicationMenu([]core.MenuItem{{Label: "File", Items: []core.MenuItem{{Label: "New", OnSelect: func() {}}}}})
		return core.View{Style: core.Style{Background: driver.RGB(0xffffff)}}
	}
	w := startTestWindow(t, app, Options{Title: "nsgo-services", Width: 360, Height: 240})
	cx := <-scopes
	ask := func(fn func(say func(string))) string {
		t.Helper()
		cx.Post(func() { fn(func(s string) { status <- s }) })
		select {
		case s := <-status:
			return s
		case <-time.After(5 * time.Second):
			t.Fatal("no answer")
		}
		return ""
	}
	wait := func(what string, ok func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var done bool
			w.onLoop(t, func() { done = ok() })
			if done {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s did not happen", what)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	m := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for i := range m.Pix {
		m.Pix[i] = 0xff
	}
	var b bytes.Buffer
	png.Encode(&b, m)
	cx.Post(func() { cx.SetClipboardImage(b.Bytes()) })
	if got := ask(func(say func(string)) {
		cx.HasClipboardImage(func(has bool) {
			cx.ReadClipboardImage(func(img core.ClipboardImage, ok bool) {
				cx.HasClipboardText(func(text bool) { say(fmt.Sprintf("%v %v %v %v", has, ok, img.Size, text)) })
			})
		})
	}); got != "true true {3 2} false" {
		t.Errorf("image = %s", got)
	}
	cx.Post(func() { cx.SetClipboard("linux text") })
	if got := ask(func(say func(string)) {
		cx.ReadClipboard(func(s string) { cx.HasClipboardImage(func(has bool) { say(fmt.Sprintf("%s %v", s, has)) }) })
	}); got != "linux text false" {
		t.Errorf("text = %s", got)
	}

	// A forced appearance has GTK draw dark, and the system's setting
	// comes back.
	dark := func() bool {
		if colorScheme() {
			return settingInt(gtkSettingsGetDefault(), "gtk-interface-color-scheme") == schemeDark
		}
		return settingBool(gtkSettingsGetDefault(), "gtk-application-prefer-dark-theme")
	}
	var before bool
	w.onLoop(t, func() { before = dark() })
	cx.Post(func() { cx.SetAppearance(core.AppearanceDark) })
	wait("a dark appearance", dark)
	cx.Post(func() { cx.SetAppearance(core.AppearanceSystem) })
	wait("the system's appearance", func() bool { return dark() == before })

	cx.Post(func() { cx.SetWindowFrame(core.WindowFrame{Width: 500, Height: 300}) })
	wait("the new size", func() bool {
		var width, height int32
		gtkWindowGetDefaultSize(w.win, &width, &height)
		return width == 500 && height == 300
	})

	// The menu bar hides until Alt is pressed and released alone.
	if w.menuBar == 0 {
		t.Error("no menu bar")
	}
	cx.Post(func() { cx.SetMenuBar(core.MenuBarAutoHide) })
	wait("the menu bar hiding", func() bool { return w.menuBar == 0 })
	w.onLoop(t, func() {
		w.menuBarKey(gdkAltL, 0, true)
		w.menuBarKey(gdkAltL, gdkAlt, false)
	})
	wait("Alt showing the menu bar", func() bool { return w.menuBar != 0 })
	w.onLoop(t, func() {
		w.menuBarKey(gdkAltL, 0, true)
		w.menuBarKey(gdkAltL, gdkAlt, false)
	})
	wait("Alt hiding the menu bar", func() bool { return w.menuBar == 0 })

	// A context menu pops up, and closing it reports it closed.
	chose := make(chan string, 2)
	cx.Post(func() {
		cx.PopupMenu([]core.MenuItem{{Label: "Copy", OnSelect: func() { chose <- "copy" }}}, core.Point{X: 20, Y: 20},
			func() { status <- "closed" })
	})
	wait("the context menu", func() bool { return w.popover != 0 && gtkWidgetGetVisible(w.popover) != 0 })
	w.onLoop(t, func() {
		for action := range w.popupRefs {
			gActionActivate(action, 0)
		}
		gtkPopoverPopdown(w.popover)
	})
	select {
	case s := <-status:
		if s != "closed" {
			t.Errorf("status = %s", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the context menu did not report closing")
	}
	select {
	case s := <-chose:
		if s != "copy" {
			t.Errorf("chose %s", s)
		}
	case <-time.After(5 * time.Second):
		t.Error("the item chosen did not run")
	}

	if got := ask(func(say func(string)) { say(cx.Env().OSVersion().String()) }); got == "0.0" || !strings.Contains(got, ".") {
		t.Errorf("OS version = %s", got)
	}

	// Quitting ends the main loop, so nothing runs on it after.
	cx.Post(cx.Quit)
	time.Sleep(500 * time.Millisecond)
	if !w.closing {
		t.Error("Quit did not close the window")
	}
}

// TestFileDialogsReachGTK picks a file as it is through GtkFileDialog,
// saves a copy of it, and saves nothing when the dialog is cancelled. The
// dialogs show in the process only without the desktop's portal, which
// GTK reads from its environment as it starts, so the test runs in a
// process of its own started with GDK_DEBUG=no-portals.
func TestFileDialogsReachGTK(t *testing.T) {
	if !strings.Contains(os.Getenv("GDK_DEBUG"), "no-portals") {
		cmd := exec.Command(os.Args[0], "-test.run=^TestFileDialogsReachGTK$", "-test.v")
		cmd.Env = append(os.Environ(), "GDK_DEBUG=no-portals")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	scopes := make(chan *core.Scope, 1)
	status := make(chan string, 16)
	app := func(c *core.Scope) core.Node {
		scopes <- c
		return core.View{Style: core.Style{Background: driver.RGB(0xffffff)}}
	}
	w := startTestWindow(t, app, Options{Title: "nsgo-file-dialogs", Width: 360, Height: 240})
	cx := <-scopes
	wait := func(what string, ok func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var done bool
			w.onLoop(t, func() { done = ok() })
			if done {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s did not happen", what)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	dir := t.TempDir()
	notes := filepath.Join(dir, "notes.txt")
	os.WriteFile(notes, []byte("notes"), 0o644)
	next := func(what string) string {
		t.Helper()
		select {
		case s := <-status:
			return s
		case <-time.After(5 * time.Second):
			t.Fatalf("%s did not answer", what)
		}
		return ""
	}
	answer := func(path string) {
		t.Helper()
		var dlg uintptr
		wait("a file dialog", func() bool { dlg = chooserDialog(); return dlg != 0 })
		const accept, cancel = -3, -6 // GTK_RESPONSE_*
		if path == "" {
			w.onLoop(t, func() { gtkDialogResponse(dlg, cancel) })
		} else {
			w.onLoop(t, func() {
				file := gFileNewForPath(cstr(path))
				gtkFileChooserSetFile(dlg, file, 0)
				gObjectUnref(file)
				const save = 1 // GTK_FILE_CHOOSER_ACTION_SAVE
				if gtkFileChooserGetAction(dlg) == save {
					// The name the dialog suggested stays otherwise.
					gtkFileChooserSetCurrentName(dlg, cstr(filepath.Base(path)))
				}
			})
			time.Sleep(time.Second) // the chooser loads the folder, then selects the file
			w.onLoop(t, func() { gtkDialogResponse(dlg, accept) })
		}
		wait("the file dialog closing", func() bool { return chooserDialog() == 0 })
	}
	cx.Post(func() {
		cx.PickFiles(core.PickFilesOptions{Types: []string{"text/plain"}}, func(files []core.File, err error) {
			var parts []string
			for _, f := range files {
				parts = append(parts, fmt.Sprintf("%s %s %d %v", f.Name, f.MIME, f.Size, f.Path == notes))
			}
			status <- fmt.Sprint(strings.Join(parts, ","), " ", err)
		})
	})
	answer(notes)
	if s := next("the pick"); s != "notes.txt text/plain 5 true <nil>" {
		t.Errorf("picked %q", s)
	}
	folder := filepath.Join(dir, "photos")
	os.Mkdir(folder, 0o755)
	cx.Post(func() {
		cx.PickDirectories(core.PickDirectoriesOptions{}, func(dirs []core.Directory, err error) {
			var parts []string
			for _, d := range dirs {
				parts = append(parts, fmt.Sprintf("%s %v", d.Name, d.Path == folder))
			}
			status <- fmt.Sprint(strings.Join(parts, ","), " ", err)
		})
	})
	answer(folder)
	if s := next("the folder pick"); s != "photos true <nil>" {
		t.Errorf("picked %q", s)
	}
	copied := filepath.Join(dir, "copy.txt")
	cx.Post(func() {
		cx.SaveFile(core.SaveOptions{Path: notes}, func(saved bool, err error) { status <- fmt.Sprint(saved, err) })
	})
	answer(copied)
	if s := next("the save"); s != "true <nil>" {
		t.Errorf("saved %q", s)
	}
	if b, _ := os.ReadFile(copied); string(b) != "notes" {
		t.Errorf("the copy holds %q", b)
	}
	cx.Post(func() {
		cx.SaveFile(core.SaveOptions{Name: "cancelled.txt", Data: []byte("x")}, func(saved bool, err error) { status <- fmt.Sprint(saved, err) })
	})
	answer("")
	if s := next("the cancelled save"); s != "false <nil>" {
		t.Errorf("a cancelled save answered %q", s)
	}

}
