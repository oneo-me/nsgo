//go:build linux && !android

package linux

import (
	"bytes"
	"errors"
	"image/png"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/picked"
)

// The services GTK performs: the clipboard's images and files, opening
// URLs (GtkUriLauncher), the photo, file and folder pickers and the save
// dialog (GtkFileDialog), context menus (GtkPopoverMenu), the app's appearance
// and the menu bar that hides.

var (
	gBytesNew                                func(data unsafe.Pointer, size uintptr) uintptr
	gdkTextureNewFromBytes                   func(bytes uintptr, err uintptr) uintptr
	gdkClipboardSetTexture                   func(clip, texture uintptr)
	gtkURILauncherNew                        func(uri *byte) uintptr
	gtkURILauncherLaunch                     func(launcher, parent, cancellable, callback, data uintptr)
	gtkURILauncherLaunchFinish               func(launcher, result, err uintptr) int32
	gtkFileDialogNew                         func() uintptr
	gtkFileDialogSetModal                    func(dialog uintptr, modal int32)
	gtkFileDialogSetDefaultFilter            func(dialog, filter uintptr)
	gtkFileDialogOpen                        func(dialog, parent, cancellable, callback, data uintptr)
	gtkFileDialogOpenFinish                  func(dialog, result, err uintptr) uintptr
	gtkFileDialogOpenMultiple                func(dialog, parent, cancellable, callback, data uintptr)
	gtkFileDialogOpenMultipleFinsh           func(dialog, result, err uintptr) uintptr
	gtkFileDialogSelectFolder                func(dialog, parent, cancellable, callback, data uintptr)
	gtkFileDialogSelectFolderFinish          func(dialog, result, err uintptr) uintptr
	gtkFileDialogSelectMultipleFolders       func(dialog, parent, cancellable, callback, data uintptr)
	gtkFileDialogSelectMultipleFoldersFinish func(dialog, result, err uintptr) uintptr
	gtkFileFilterNew                         func() uintptr
	gtkFileFilterSetName                     func(filter uintptr, name *byte)
	gtkFileFilterAddSuffix                   func(filter uintptr, suffix *byte)
	gtkFileFilterAddMIMEType                 func(filter uintptr, mime *byte)
	gtkFileDialogSetInitialName              func(dialog uintptr, name *byte)
	gtkFileDialogSave                        func(dialog, parent, cancellable, callback, data uintptr)
	gtkFileDialogSaveFinish                  func(dialog, result, err uintptr) uintptr
	gtkDialogErrorQuark                      func() uint32
	gListModelGetNItems                      func(model uintptr) uint32
	gListModelGetItem                        func(model uintptr, i uint32) uintptr
	gtkPopoverMenuNewFromModel               func(model uintptr) uintptr
	gtkPopoverSetPointingTo                  func(popover uintptr, rect *gdkRectangle)
	gtkPopoverSetHasArrow                    func(popover uintptr, on int32)
	gtkPopoverSetPosition                    func(popover uintptr, position int32)
	gtkPopoverPopup                          func(popover uintptr)
	gtkPopoverPopdown                        func(popover uintptr)
	gObjectSetBool                           func(object uintptr, name *byte, value int32, end uintptr)
	gtkGetMinorVersion                       func() uint32
	cbAsync, cbPopoverClosed                 uintptr

	// asyncs are the callbacks of the asynchronous calls in flight, by
	// the data their C callback gets.
	asyncs    sync.Map // uintptr -> func(source, result uintptr)
	nextAsync uintptr
)

func bindSystem() {
	gtk := func(fn any, name string) { purego.RegisterLibFunc(fn, gtkLib, name) }
	purego.RegisterLibFunc(&gBytesNew, glibLib, "g_bytes_new")
	purego.RegisterLibFunc(&gListModelGetNItems, gioLib, "g_list_model_get_n_items")
	purego.RegisterLibFunc(&gListModelGetItem, gioLib, "g_list_model_get_item")
	purego.RegisterLibFunc(&gObjectSetBool, gobjectLib, "g_object_set")
	gtk(&gtkGetMinorVersion, "gtk_get_minor_version")
	gtk(&gdkTextureNewFromBytes, "gdk_texture_new_from_bytes")
	gtk(&gdkClipboardSetTexture, "gdk_clipboard_set_texture")
	gtk(&gtkURILauncherNew, "gtk_uri_launcher_new")
	gtk(&gtkURILauncherLaunch, "gtk_uri_launcher_launch")
	gtk(&gtkURILauncherLaunchFinish, "gtk_uri_launcher_launch_finish")
	gtk(&gtkFileDialogNew, "gtk_file_dialog_new")
	gtk(&gtkFileDialogSetModal, "gtk_file_dialog_set_modal")
	gtk(&gtkFileDialogSetDefaultFilter, "gtk_file_dialog_set_default_filter")
	gtk(&gtkFileDialogOpen, "gtk_file_dialog_open")
	gtk(&gtkFileDialogOpenFinish, "gtk_file_dialog_open_finish")
	gtk(&gtkFileDialogOpenMultiple, "gtk_file_dialog_open_multiple")
	gtk(&gtkFileDialogOpenMultipleFinsh, "gtk_file_dialog_open_multiple_finish")
	gtk(&gtkFileDialogSelectFolder, "gtk_file_dialog_select_folder")
	gtk(&gtkFileDialogSelectFolderFinish, "gtk_file_dialog_select_folder_finish")
	gtk(&gtkFileDialogSelectMultipleFolders, "gtk_file_dialog_select_multiple_folders")
	gtk(&gtkFileDialogSelectMultipleFoldersFinish, "gtk_file_dialog_select_multiple_folders_finish")
	gtk(&gtkFileFilterNew, "gtk_file_filter_new")
	gtk(&gtkFileFilterSetName, "gtk_file_filter_set_name")
	gtk(&gtkFileFilterAddSuffix, "gtk_file_filter_add_suffix")
	gtk(&gtkFileFilterAddMIMEType, "gtk_file_filter_add_mime_type")
	gtk(&gtkFileDialogSetInitialName, "gtk_file_dialog_set_initial_name")
	gtk(&gtkFileDialogSave, "gtk_file_dialog_save")
	gtk(&gtkFileDialogSaveFinish, "gtk_file_dialog_save_finish")
	gtk(&gtkDialogErrorQuark, "gtk_dialog_error_quark")
	gtk(&gtkPopoverMenuNewFromModel, "gtk_popover_menu_new_from_model")
	gtk(&gtkPopoverSetPointingTo, "gtk_popover_set_pointing_to")
	gtk(&gtkPopoverSetHasArrow, "gtk_popover_set_has_arrow")
	gtk(&gtkPopoverSetPosition, "gtk_popover_set_position")
	gtk(&gtkPopoverPopup, "gtk_popover_popup")
	gtk(&gtkPopoverPopdown, "gtk_popover_popdown")
	cbAsync = newCallback(asyncCallback)
	cbPopoverClosed = newCallback(popoverClosedCallback)
}

// async returns the data of an asynchronous call whose answer fn takes.
func async(fn func(source, result uintptr)) uintptr {
	nextAsync++
	asyncs.Store(nextAsync, fn)
	return nextAsync
}

func asyncCallback(source, result, data uintptr) {
	if fn, ok := asyncs.LoadAndDelete(data); ok {
		fn.(func(source, result uintptr))(source, result)
	}
}

// clipboard returns the display's clipboard.
func displayClipboard() uintptr { return gdkDisplayGetClipboard(gdkDisplayGetDefault()) }

// gTypeString is G_TYPE_STRING: fundamental type 16, shifted.
const gTypeString = 16 << 2

// readClipboard answers an ItemReadClipboard; GDK reads asynchronously.
func (w *window) readClipboard(it *driver.MountItem) {
	ev := driver.Event{Kind: driver.EventClipboard, Count: it.Count}
	answer := func() {
		if !w.closing {
			w.engine.Dispatch(ev)
		}
	}
	clip := displayClipboard()
	if clip == 0 {
		answer()
		return
	}
	formats := gdkClipboardGetFormats(clip)
	files := gdkContentFormatsContainGtype(formats, gdkFileListGetType())
	switch it.Clipboard {
	case driver.ClipboardReadHasText:
		ev.Finished = gdkContentFormatsContainGtype(formats, gTypeString)
		answer()
	case driver.ClipboardReadHasImage:
		ev.Finished = !files && gdkContentFormatsContainGtype(formats, gdkTextureGetType())
		answer()
	case driver.ClipboardReadText:
		gdkClipboardReadTextAsync(clip, 0, cbAsync, async(func(source, result uintptr) {
			if p := gdkClipboardReadTextFinish(source, result, 0); p != 0 {
				ev.Text = goString(p)
				gFree(p)
			}
			answer()
		}))
	case driver.ClipboardReadFiles:
		if !files {
			answer()
			return
		}
		gdkClipboardReadValueAsync(clip, gdkFileListGetType(), 0, 0, cbAsync, async(func(source, result uintptr) {
			if value := gdkClipboardReadValueFinish(source, result, 0); value != 0 {
				ev.URIs = fileURIs(value)
			}
			answer()
		}))
	case driver.ClipboardReadImage:
		if files || !gdkContentFormatsContainGtype(formats, gdkTextureGetType()) {
			answer()
			return
		}
		gdkClipboardReadTextureAsync(clip, 0, cbAsync, async(func(source, result uintptr) {
			if texture := gdkClipboardReadTextureFinish(source, result, 0); texture != 0 {
				if data := texturePNG(texture); len(data) <= driver.MaxClipboardImage {
					if c, err := png.DecodeConfig(bytes.NewReader(data)); err == nil {
						ev.Data = &driver.ClipboardImage{PNG: data,
							Size: driver.Size{Width: driver.Points(c.Width), Height: driver.Points(c.Height)}}
					}
				}
				gObjectUnref(texture)
			}
			answer()
		}))
	}
}

// texturePNG returns texture encoded as a PNG.
func texturePNG(texture uintptr) []byte {
	b := gdkTextureSaveToPngBytes(texture)
	if b == 0 {
		return nil
	}
	defer gBytesUnref(b)
	var n uintptr
	p := gBytesGetData(b, &n)
	if p == 0 || n == 0 {
		return nil
	}
	return bytes.Clone(unsafe.Slice((*byte)(cPointer(p)), n))
}

// setClipboardImage puts the image data encodes on the clipboard as a
// texture, which GDK offers other apps as PNG and the formats they ask.
func setClipboardImage(data []byte) {
	clip := displayClipboard()
	if clip == 0 || len(data) == 0 {
		return
	}
	b := gBytesNew(unsafe.Pointer(&data[0]), uintptr(len(data)))
	defer gBytesUnref(b)
	texture := gdkTextureNewFromBytes(b, 0)
	if texture == 0 {
		return
	}
	gdkClipboardSetTexture(clip, texture)
	gObjectUnref(texture)
}

// openURL opens u with GtkUriLauncher, through the desktop portal where
// there is one, and reports whether it opened.
func (w *window) openURL(u string, id uint32) {
	launcher := gtkURILauncherNew(cstr(u))
	if launcher == 0 {
		w.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: id})
		return
	}
	gtkURILauncherLaunch(launcher, w.toplevel(), 0, cbAsync, async(func(source, result uintptr) {
		ok := gtkURILauncherLaunchFinish(source, result, 0) != 0
		gObjectUnref(source)
		if !w.closing {
			w.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: id, Finished: ok})
		}
	}))
}

// pickImages shows GtkFileDialog for the images r asks for, then reads
// and prepares the ones chosen off the main thread.
func (w *window) pickImages(r driver.PickRequest, id uint32) {
	dialog := gtkFileDialogNew()
	if dialog == 0 {
		w.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: id})
		return
	}
	filter := gtkFileFilterNew()
	gtkFileFilterSetName(filter, cstr("Images"))
	for _, ext := range r.DialogExtensions() {
		gtkFileFilterAddSuffix(filter, cstr(ext))
	}
	gtkFileDialogSetDefaultFilter(dialog, filter)
	gObjectUnref(filter)
	w.openDialog(dialog, r.Limit > 1, false, func(paths []string, _ error) {
		go func() {
			files := picked.ReadFiles(r, paths)
			w.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: id, Data: files})
		}()
	})
}

// pickFiles shows GtkFileDialog for the files r offers and answers with
// their paths.
func (w *window) pickFiles(r driver.FilesRequest, id uint32) {
	dialog := gtkFileDialogNew()
	if dialog == 0 {
		w.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Text: "no file dialog"})
		return
	}
	if len(r.Types) > 0 {
		filter := gtkFileFilterNew()
		gtkFileFilterSetName(filter, cstr("Files"))
		for _, t := range r.Types {
			if ext, ok := strings.CutPrefix(t, "."); ok {
				gtkFileFilterAddSuffix(filter, cstr(ext))
			} else {
				gtkFileFilterAddMIMEType(filter, cstr(t))
			}
		}
		gtkFileDialogSetDefaultFilter(dialog, filter)
		gObjectUnref(filter)
	}
	w.openDialog(dialog, r.Limit > 1, false, func(paths []string, err error) {
		ev := driver.Event{Kind: driver.EventFiles, Count: id}
		if err != nil {
			ev.Text = err.Error()
		}
		var files []driver.File
		for _, p := range paths {
			files = append(files, driver.File{Path: p})
		}
		ev.Data = files
		w.engine.Dispatch(ev)
	})
}

// pickDirectories shows GtkFileDialog for folders and answers with
// their paths.
func (w *window) pickDirectories(r driver.DirectoriesRequest, id uint32) {
	dialog := gtkFileDialogNew()
	if dialog == 0 {
		w.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Text: "no file dialog"})
		return
	}
	w.openDialog(dialog, r.Limit > 1, true, func(paths []string, err error) {
		ev := driver.Event{Kind: driver.EventFiles, Count: id}
		if err != nil {
			ev.Text = err.Error()
		}
		var dirs []driver.Directory
		for _, p := range paths {
			dirs = append(dirs, driver.Directory{Path: p})
		}
		ev.Data = dirs
		w.engine.Dispatch(ev)
	})
}

// saveFile shows GtkFileDialog's save dialog suggesting r's name, then
// writes the file where the user chose off the main thread.
func (w *window) saveFile(r driver.SaveRequest, id uint32) {
	ev := driver.Event{Kind: driver.EventFiles, Count: id}
	dialog := gtkFileDialogNew()
	if dialog == 0 {
		ev.Text = "no file dialog"
		w.engine.Dispatch(ev)
		return
	}
	gtkFileDialogSetModal(dialog, 1)
	gtkFileDialogSetInitialName(dialog, cstr(r.Name))
	gtkFileDialogSave(dialog, w.toplevel(), 0, cbAsync, async(func(source, result uintptr) {
		var gerr uintptr
		file := gtkFileDialogSaveFinish(source, result, uintptr(unsafe.Pointer(&gerr)))
		gObjectUnref(dialog)
		if file == 0 {
			if err := dialogError(gerr); err != nil {
				ev.Text = err.Error()
			}
			w.engine.Dispatch(ev)
			return
		}
		path := filePath(file)
		gObjectUnref(file)
		go func() {
			if path == "" {
				ev.Text = "the chosen file has no path"
			} else if err := r.SaveTo(path); err != nil {
				ev.Text = err.Error()
			} else {
				ev.Finished = true
			}
			w.engine.Dispatch(ev)
		}()
	}))
}

// openDialog shows dialog, modal, for one file or folder or several and
// calls done with the paths chosen: none when the user cancelled, err
// when it failed.
func (w *window) openDialog(dialog uintptr, multiple, folders bool, done func(paths []string, err error)) {
	gtkFileDialogSetModal(dialog, 1)
	finish := func(paths []string, gerr uintptr) {
		gObjectUnref(dialog)
		done(paths, dialogError(gerr))
	}
	open, openFinish := gtkFileDialogOpen, gtkFileDialogOpenFinish
	openMultiple, openMultipleFinish := gtkFileDialogOpenMultiple, gtkFileDialogOpenMultipleFinsh
	if folders {
		open, openFinish = gtkFileDialogSelectFolder, gtkFileDialogSelectFolderFinish
		openMultiple, openMultipleFinish = gtkFileDialogSelectMultipleFolders, gtkFileDialogSelectMultipleFoldersFinish
	}
	if multiple {
		openMultiple(dialog, w.toplevel(), 0, cbAsync, async(func(source, result uintptr) {
			var paths []string
			var gerr uintptr
			if list := openMultipleFinish(source, result, uintptr(unsafe.Pointer(&gerr))); list != 0 {
				for i := range gListModelGetNItems(list) {
					if file := gListModelGetItem(list, i); file != 0 {
						if p := filePath(file); p != "" {
							paths = append(paths, p)
						}
						gObjectUnref(file)
					}
				}
				gObjectUnref(list)
			}
			finish(paths, gerr)
		}))
		return
	}
	open(dialog, w.toplevel(), 0, cbAsync, async(func(source, result uintptr) {
		var paths []string
		var gerr uintptr
		if file := openFinish(source, result, uintptr(unsafe.Pointer(&gerr))); file != 0 {
			if p := filePath(file); p != "" {
				paths = append(paths, p)
			}
			gObjectUnref(file)
		}
		finish(paths, gerr)
	}))
}

// filePath returns a GFile's local path, "" for one without.
func filePath(file uintptr) string {
	p := gFileGetPath(file)
	if p == 0 {
		return ""
	}
	defer gFree(p)
	return goString(p)
}

// dialogError returns what a file dialog's GError says failed and frees
// it: nil for none, or for a dialog the user cancelled or dismissed.
func dialogError(gerr uintptr) error {
	if gerr == 0 {
		return nil
	}
	defer gErrorFree(gerr)
	e := (*gError)(cPointer(gerr))
	const cancelled, dismissed = 1, 2 // GTK_DIALOG_ERROR_*
	if e.domain == gtkDialogErrorQuark() && (e.code == cancelled || e.code == dismissed) {
		return nil
	}
	return errors.New(goString(e.message))
}

// popupMenu shows entries as a GtkPopoverMenu pointing at p, one at a
// time, and reports it closed.
func (w *window) popupMenu(entries []driver.MenuEntry, p driver.Point, id uint32) {
	if w.popover != 0 {
		gtkPopoverPopdown(w.popover) // reports the one before closed
	}
	group := gSimpleActionGroupNew()
	w.building = menuBuild{group: group, prefix: "popup.", refs: map[uintptr]menuRef{}}
	menu := w.buildSubmenu(entries)
	w.popupRefs = w.building.refs
	w.building = menuBuild{}
	gtkWidgetInsertActionGroup(w.toplevel(), cstr("popup"), group)
	gObjectUnref(group)
	if menu == 0 {
		w.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: id})
		return
	}
	popover := gtkPopoverMenuNewFromModel(menu)
	gObjectUnref(menu)
	gtkWidgetSetParent(popover, w.surface)
	gtkPopoverSetHasArrow(popover, 0)
	const bottom = 3 // GTK_POS_BOTTOM
	gtkPopoverSetPosition(popover, bottom)
	gtkPopoverSetPointingTo(popover, &gdkRectangle{int32(p.X), int32(p.Y), 1, 1})
	w.popover, w.popoverID = popover, id
	gSignalConnect(popover, "closed", cbPopoverClosed, w.handle)
	gtkPopoverPopup(popover)
}

// popoverClosedCallback reports a context menu closed, after the action
// of an item chosen, and drops it once GTK is done with it.
func popoverClosedCallback(popover, data uintptr) {
	w := windowOf(data)
	if w == nil {
		return
	}
	id := w.popoverID
	if w.popover == popover {
		w.popover, w.popoverID = 0, 0
	}
	w.post(func() {
		gtkWidgetUnparent(popover)
		if !w.closing {
			w.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: id})
		}
	})
}

// colorScheme reports whether GTK has gtk-interface-color-scheme, 4.20
// and later, where gtk-application-prefer-dark-theme is deprecated.
func colorScheme() bool { return gtkGetMinorVersion() >= 20 }

// The values of GtkInterfaceColorScheme.
const (
	schemeDefault = 1
	schemeDark    = 2
	schemeLight   = 3
)

// setAppearance has GTK draw the widgets it hosts, its dialogs and menus
// in a, or as the session's settings say.
func (w *window) setAppearance(a driver.Appearance) {
	settings := gtkSettingsGetDefault()
	if settings == 0 || a == w.appearance {
		return
	}
	w.appearance = a
	if colorScheme() {
		scheme := map[driver.Appearance]int32{driver.AppearanceSystem: schemeDefault, driver.AppearanceLight: schemeLight,
			driver.AppearanceDark: schemeDark}[a]
		gObjectSetBool(settings, cstr("gtk-interface-color-scheme"), scheme, 0)
		return
	}
	if !w.preferDarkKnown {
		w.preferDark, w.preferDarkKnown = settingBool(settings, "gtk-application-prefer-dark-theme"), true
	}
	dark := w.preferDark
	switch a {
	case driver.AppearanceLight:
		dark = false
	case driver.AppearanceDark:
		dark = true
	}
	gObjectSetBool(settings, cstr("gtk-application-prefer-dark-theme"), boolInt(dark), 0)
}

// setFrame sizes the window: GTK keeps the size, and the compositor
// places windows on Wayland, so a position is ignored.
func (w *window) setFrame(f driver.WindowFrame) {
	var width, height int32
	gtkWindowGetDefaultSize(w.win, &width, &height)
	if f.Width > 0 {
		width = int32(f.Width)
	}
	if f.Height > 0 {
		height = int32(f.Height)
	}
	gtkWindowSetDefaultSize(w.win, width, height)
}

// The keyvals of the menu bar's keys.
const (
	gdkAltL = 0xffe9
	gdkAltR = 0xffea
	gdkF10  = 0xffc7
)

// menuBarKey follows the keys that show an auto-hiding menu bar: Alt
// pressed and released alone toggles it; Alt with another key and F10
// show it for the menu they open. It reports whether it took the key.
func (w *window) menuBarKey(keyval uint32, state uint32, pressed bool) bool {
	c := &w.chrome
	if c.menuBar != driver.MenuBarAutoHide || c.menu == nil {
		return false
	}
	alt := keyval == gdkAltL || keyval == gdkAltR
	switch {
	case pressed && alt:
		c.altAlone = true
	case !pressed && alt:
		alone := c.altAlone
		c.altAlone = false
		if alone {
			c.revealed = !c.revealed
			w.showMenuBar()
			return true
		}
	case pressed:
		c.altAlone = false
		if (keyval == gdkF10 || state&gdkAlt != 0) && !c.revealed {
			c.revealed = true
			w.showMenuBar()
		}
	}
	return false
}

// hideMenuBar hides a shown auto-hiding menu bar after a press in the
// window.
func (w *window) hideMenuBar() {
	if c := &w.chrome; c.menuBar == driver.MenuBarAutoHide && c.revealed {
		c.revealed = false
		w.showMenuBar()
	}
}

// osVersion returns the kernel's major and minor version.
func osVersion() driver.OSVersion {
	var u syscall.Utsname
	if syscall.Uname(&u) != nil {
		return driver.OSVersion{}
	}
	var b strings.Builder
	for _, c := range u.Release {
		if c == 0 {
			break
		}
		b.WriteByte(byte(c))
	}
	major, rest, _ := strings.Cut(b.String(), ".")
	minor, _, _ := strings.Cut(rest, ".")
	v := driver.OSVersion{}
	v.Major, _ = strconv.Atoi(major)
	v.Minor, _ = strconv.Atoi(strings.TrimRightFunc(minor, func(r rune) bool { return r < '0' || r > '9' }))
	return v
}
