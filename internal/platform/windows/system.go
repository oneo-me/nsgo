//go:build windows

package windows

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/image/bmp"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/imagedecode"
	"github.com/nsgo-dev/nsgo/internal/platform/picked"
)

// The services Win32 performs: the clipboard's images and files, opening
// URLs, the photo and file pickers, the save dialog, context menus, the app's appearance, the
// window's frame and level, and the menu bar that hides.

var (
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	uxtheme              = syscall.NewLazyDLL("uxtheme.dll")
	getOpenFileNameW     = comdlg32.NewProc("GetOpenFileNameW")
	getSaveFileNameW     = comdlg32.NewProc("GetSaveFileNameW")
	commDlgExtendedError = comdlg32.NewProc("CommDlgExtendedError")
	shellExecuteExW      = shell32.NewProc("ShellExecuteExW")
	isIconic             = user32.NewProc("IsIconic")
	setForegroundWindow  = user32.NewProc("SetForegroundWindow")
	bringWindowToTop     = user32.NewProc("BringWindowToTop")
	getWindowRect        = user32.NewProc("GetWindowRect")
	endMenu              = user32.NewProc("EndMenu")
)

const (
	swShowNormal = 1

	hwndBottom    = 1
	hwndTopmost   = ^uintptr(0) // -1
	hwndNoTopmost = ^uintptr(1) // -2

	ofnAllowMultiSelect = 0x00000200
	ofnPathMustExist    = 0x00000800
	ofnFileMustExist    = 0x00001000
	ofnExplorer         = 0x00080000
	ofnNoChangeDir      = 0x00000008
	ofnOverwritePrompt  = 0x00000002

	wmDialog       = 0x8000 + 5 // an open or save dialog waits
	wmPopup        = 0x8000 + 6 // a context menu waits
	wmFolderChange = 0x8000 + 8 // the folder dialog opened another folder
	wmSysKeyUp     = 0x0105

	wmEnterMenuLoop = 0x0211
	wmExitMenuLoop  = 0x0212
	vkF10           = 0x79
)

// pngFormat is the clipboard format PNG-aware apps share images in.
func pngFormat() uintptr {
	name, _ := syscall.UTF16PtrFromString("PNG")
	f, _, _ := registerClipboardFormatW.Call(uintptr(unsafe.Pointer(name)))
	return f
}

func formatAvailable(f uintptr) bool {
	r, _, _ := isClipboardFormatAvailable.Call(f)
	return f != 0 && r != 0
}

// readClipboard answers an ItemReadClipboard.
func (w *window) readClipboard(it *driver.MountItem) {
	ev := driver.Event{Kind: driver.EventClipboard, Count: it.Count}
	switch it.Clipboard {
	case driver.ClipboardReadText:
		ev.Text = w.clipboard()
	case driver.ClipboardReadHasText:
		ev.Finished = formatAvailable(cfUnicodeText)
	case driver.ClipboardReadHasImage:
		ev.Finished = !formatAvailable(cfHDrop) && (formatAvailable(pngFormat()) || formatAvailable(cfDIB))
	case driver.ClipboardReadFiles, driver.ClipboardReadImage:
		if r, _, _ := openClipboard.Call(w.hwnd); r == 0 {
			break
		}
		h, _, _ := getClipboardData.Call(cfHDrop)
		if h != 0 {
			ev.URIs = hdropFiles(h)
		}
		var files []driver.PastedFile
		if it.Clipboard == driver.ClipboardReadImage && len(ev.URIs) == 0 {
			files = clipboardImage()
		}
		closeClipboard.Call()
		if it.Clipboard == driver.ClipboardReadImage {
			ev.URIs = nil
			if len(files) > 0 {
				// Converting a large bitmap takes a while: off the thread.
				data := files[0].Data
				go func() {
					ev.Data = pngOf(data)
					w.engine.Dispatch(ev)
				}()
				return
			}
		}
	}
	w.engine.Dispatch(ev)
}

// pngOf returns the clipboard's PNG or bitmap as a PNG, or why it could
// not.
func pngOf(data []byte) *driver.ClipboardImage {
	if driver.SniffImage(data) != "image/png" {
		m, err := bmp.Decode(bytes.NewReader(data))
		if err != nil {
			return driver.ReadClipboardImage(nil, driver.Size{}, err)
		}
		var b bytes.Buffer
		if err := png.Encode(&b, m); err != nil {
			return driver.ReadClipboardImage(nil, driver.Size{}, err)
		}
		data = b.Bytes()
	}
	return driver.ClipboardPNG(data)
}

// setClipboardImage puts the image data encodes on the clipboard as a
// PNG, which keeps its transparency, and a device-independent bitmap
// over white, which every app reads.
func (w *window) setClipboardImage(data []byte) {
	img, err := imagedecode.Decode(data, imagedecode.Options{Layout: imagedecode.RGBA})
	if err != nil || len(img.Frames) == 0 {
		return
	}
	pix := img.Frames[0].Pix
	rgba := &image.RGBA{Pix: pix, Stride: img.Width * 4, Rect: image.Rect(0, 0, img.Width, img.Height)}
	var encoded bytes.Buffer
	if png.Encode(&encoded, rgba) != nil {
		return
	}
	// BITMAPINFOHEADER, then rows bottom up of blue, green, red and an
	// unused byte.
	dib := make([]byte, 40+len(pix))
	binary.LittleEndian.PutUint32(dib[0:], 40)
	binary.LittleEndian.PutUint32(dib[4:], uint32(img.Width))
	binary.LittleEndian.PutUint32(dib[8:], uint32(img.Height))
	binary.LittleEndian.PutUint16(dib[12:], 1)
	binary.LittleEndian.PutUint16(dib[14:], 32)
	binary.LittleEndian.PutUint32(dib[20:], uint32(len(pix)))
	row := img.Width * 4
	for y := range img.Height {
		src := pix[y*row : (y+1)*row]
		dst := dib[40+(img.Height-1-y)*row:]
		for x := 0; x < row; x += 4 {
			white := 255 - src[x+3] // premultiplied over white
			dst[x], dst[x+1], dst[x+2], dst[x+3] = src[x+2]+white, src[x+1]+white, src[x]+white, 255
		}
	}
	if r, _, _ := openClipboard.Call(w.hwnd); r == 0 {
		return
	}
	defer closeClipboard.Call()
	emptyClipboard.Call()
	put := func(format uintptr, b []byte) {
		h, _, _ := globalAlloc.Call(gmemMoveable, uintptr(len(b)))
		if h == 0 {
			return
		}
		p, _, _ := globalLock.Call(h)
		if p == 0 {
			globalFree.Call(h)
			return
		}
		rtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
		globalUnlock.Call(h)
		if r, _, _ := setClipboardData.Call(format, h); r == 0 {
			globalFree.Call(h)
		}
	}
	if f := pngFormat(); f != 0 {
		put(f, encoded.Bytes())
	}
	put(cfDIB, dib)
}

// shellExecuteInfo is SHELLEXECUTEINFOW.
type shellExecuteInfo struct {
	Size       uint32
	Mask       uint32
	Hwnd       uintptr
	Verb       *uint16
	File       *uint16
	Parameters *uint16
	Directory  *uint16
	Show       int32
	InstApp    uintptr
	IDList     uintptr
	Class      *uint16
	KeyClass   uintptr
	HotKey     uint32
	Icon       uintptr
	Process    uintptr
}

// openURL has the shell open u with the app registered for it. A scheme
// no app handles answers false without the shell asking for an app.
func (w *window) openURL(u string, id uint32) {
	// The shell asks the user for an app when none handles the scheme,
	// even without UI: ask the registry first.
	scheme, _, _ := strings.Cut(u, ":")
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CLASSES_ROOT, wide(scheme), 0, syscall.KEY_READ, &key) != nil {
		w.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: id})
		return
	}
	syscall.RegCloseKey(key)
	const flagNoUI = 0x00000400 // SEE_MASK_FLAG_NO_UI
	info := shellExecuteInfo{Mask: flagNoUI, Hwnd: w.hwnd, Verb: wide("open"), File: wide(u), Show: swShowNormal}
	info.Size = uint32(unsafe.Sizeof(info))
	r, _, _ := shellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	w.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: id, Finished: r != 0})
}

// openFile has the shell open f with the app associated with its type.
// A type no app opens answers false without the shell asking for an app.
func (w *window) openFile(f driver.SharedFile, id uint32) {
	const flagNoUI = 0x00000400 // SEE_MASK_FLAG_NO_UI
	info := shellExecuteInfo{Mask: flagNoUI, Hwnd: w.hwnd, File: wide(f.Path), Show: swShowNormal}
	info.Size = uint32(unsafe.Sizeof(info))
	ev := driver.Event{Kind: driver.EventFiles, Count: id}
	r, _, err := shellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	const errNoAssociation = 1155 // ERROR_NO_ASSOCIATION
	switch {
	case r != 0:
		ev.Finished = true
	case err != syscall.Errno(errNoAssociation):
		ev.Text = err.Error()
	}
	w.engine.Dispatch(ev)
}

// openFileName is OPENFILENAMEW.
type openFileName struct {
	StructSize    uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	Hook          uintptr
	TemplateName  *uint16
	Reserved      uintptr
	ReservedDW    uint32
	FlagsEx       uint32
}

// pickImages queues a pick: the open dialog's modal loop must not run in
// the batch's. The images chosen are read as they are on a goroutine.
func (w *window) pickImages(r driver.PickRequest, id uint32) {
	w.queueDialog(func() {
		paths, _ := w.openDialog(syscall.StringToUTF16("Images"), r.DialogExtensions(), r.Limit)
		go func() {
			w.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: id, Data: picked.ReadFiles(r, paths)})
		}()
	})
}

// pickFiles queues a pick of the files r offers.
func (w *window) pickFiles(r driver.FilesRequest, id uint32) {
	w.queueDialog(func() {
		exts, every := r.Extensions()
		name := syscall.StringToUTF16("Files")
		if every {
			exts, name = nil, syscall.StringToUTF16("All files")
		}
		paths, err := w.openDialog(name, exts, r.Limit)
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

// pickDirectories queues a pick of folders.
func (w *window) pickDirectories(r driver.DirectoriesRequest, id uint32) {
	w.queueDialog(func() {
		paths, err := w.folderDialog(r.Limit)
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

// saveFile queues a save: the save dialog suggests r's name, and the
// file is written where the user chose on a goroutine.
func (w *window) saveFile(r driver.SaveRequest, id uint32) {
	w.queueDialog(func() {
		path, err := w.saveDialog(r.Name)
		ev := driver.Event{Kind: driver.EventFiles, Count: id}
		if err != nil || path == "" {
			if err != nil {
				ev.Text = err.Error()
			}
			w.engine.Dispatch(ev)
			return
		}
		go func() {
			if err := r.SaveTo(path); err != nil {
				ev.Text = err.Error()
			} else {
				ev.Finished, ev.Data = true, path
			}
			w.engine.Dispatch(ev)
		}()
	})
}

// queueDialog runs show from a message of its own, one dialog at a
// time.
func (w *window) queueDialog(show func()) {
	w.dialogs = append(w.dialogs, show)
	postMessageW.Call(w.hwnd, wmDialog, 0, 0)
}

// showDialog shows the first dialog queued.
func (w *window) showDialog() {
	if w.inDialog || len(w.dialogs) == 0 {
		return
	}
	show := w.dialogs[0]
	w.dialogs = w.dialogs[1:]
	w.inDialog = true
	show()
	w.inDialog = false
	if len(w.dialogs) > 0 {
		postMessageW.Call(w.hwnd, wmDialog, 0, 0)
	}
}

// dialogError is what CommDlgExtendedError says failed in a dialog that
// returned false, nil when the user cancelled it.
func dialogError() error {
	code, _, _ := commDlgExtendedError.Call()
	if code == 0 {
		return nil
	}
	return fmt.Errorf("common dialog error %#x", code)
}

// openDialog returns the paths of the files chosen in Windows' open
// dialog, at most limit, offering those of exts under name, or every
// file for none; none when it was cancelled.
func (w *window) openDialog(name []uint16, exts []string, limit int) ([]string, error) {
	var pattern []uint16
	for i, ext := range exts {
		if i > 0 {
			pattern = append(pattern, ';')
		}
		pattern = append(pattern, syscall.StringToUTF16("*."+ext)...)
		pattern = pattern[:len(pattern)-1]
	}
	if len(exts) == 0 {
		pattern = append(pattern, '*', '.', '*')
	}
	filter := append(name, append(pattern, 0, 0)...)
	buf := make([]uint16, 64<<10)
	ofn := openFileName{Owner: w.owner(), Filter: &filter[0], File: &buf[0], MaxFile: uint32(len(buf)),
		Flags: ofnExplorer | ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir}
	if limit > 1 {
		ofn.Flags |= ofnAllowMultiSelect
	}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	if ok, _, _ := getOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); ok == 0 {
		return nil, dialogError()
	}
	// One path, or the directory then each name, NUL-separated.
	var parts []string
	for start, i := 0, 0; i < len(buf); i++ {
		if buf[i] == 0 {
			if i == start {
				break
			}
			parts = append(parts, syscall.UTF16ToString(buf[start:i]))
			start = i + 1
		}
	}
	if len(parts) <= 1 {
		return parts, nil
	}
	dir := parts[0]
	paths := make([]string, 0, len(parts)-1)
	for _, name := range parts[1:] {
		if len(paths) < max(limit, 1) {
			paths = append(paths, dir+`\`+name)
		}
	}
	return paths, nil
}

var (
	clsidFileOpenDialog = parseGUID("DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7")
	iidFileOpenDialog   = parseGUID("D57C7288-D4AD-4768-BE02-9D969532D960")
)

// IFileOpenDialog's methods, from IModalWindow's and IFileDialog's, and
// those of the shell items it answers with.
const (
	fileDialogShow        = 3
	fileDialogAdvise      = 7
	fileDialogUnadvise    = 8
	fileDialogSetOptions  = 9
	fileDialogGetOptions  = 10
	fileDialogSetFolder   = 12
	fileDialogGetFolder   = 13
	fileDialogSetFileName = 15
	fileDialogGetResults  = 27
	itemArrayGetCount     = 7
	itemArrayGetItemAt    = 8
	shellItemGetName      = 5

	fosNoChangeDir              = 0x8
	fosPickFolders              = 0x20
	fosForceFileSystem          = 0x40
	fosAllowMultiSelect         = 0x200
	fosPathMustExist            = 0x800
	sigdnFileSysPath            = 0x80058000
	sigdnDesktopAbsoluteParsing = 0x80028000
	hresultCancelled            = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED)

	// homeFolder is the CLSID of Windows 11's Home, which is not on the
	// file system.
	homeFolder = "{F874310E-B6B7-47DC-BC84-B9E6B38F5903}"
)

var (
	iidFileDialogEvents         = parseGUID("973510DB-7D7F-452B-8975-74A85828D354")
	iidShellItem                = parseGUID("43826D1E-E718-42EE-BC55-A1E261C37BFE")
	shCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")

	// folderEventsVtbl is IFileDialogEvents': OnFolderChange answers,
	// and the rest accept.
	folderEventsVtbl = [10]uintptr{
		syscall.NewCallback(folderEventsQueryInterface),
		syscall.NewCallback(folderEventsAddRef),
		syscall.NewCallback(folderEventsRelease),
		syscall.NewCallback(func(e *folderEvents, dialog uintptr) uintptr { return sOK }),                 // OnFileOk
		syscall.NewCallback(func(e *folderEvents, dialog, folder uintptr) uintptr { return sOK }),         // OnFolderChanging
		syscall.NewCallback(folderEventsFolderChange),                                                     // OnFolderChange
		syscall.NewCallback(func(e *folderEvents, dialog uintptr) uintptr { return sOK }),                 // OnSelectionChange
		syscall.NewCallback(func(e *folderEvents, dialog, item, response uintptr) uintptr { return sOK }), // OnShareViolation
		syscall.NewCallback(func(e *folderEvents, dialog uintptr) uintptr { return sOK }),                 // OnTypeChange
		syscall.NewCallback(func(e *folderEvents, dialog, item, response uintptr) uintptr { return sOK }), // OnOverwrite
	}
)

// folderEvents is the folder dialog's IFileDialogEvents, implemented in
// Go. The dialog calls it on the window's thread, and the window keeps it
// alive while the dialog shows, so nothing is freed when it is released.
type folderEvents struct {
	vtbl *[10]uintptr
	refs atomic.Int32
	w    *window
}

func folderEventsQueryInterface(e *folderEvents, iid *guid, out *unsafe.Pointer) uintptr {
	if *iid == iidUnknown || *iid == iidFileDialogEvents {
		*out = unsafe.Pointer(e)
		e.refs.Add(1)
		return sOK
	}
	*out = nil
	return eNoInterface
}

func folderEventsAddRef(e *folderEvents) uintptr { return uintptr(e.refs.Add(1)) }

func folderEventsRelease(e *folderEvents) uintptr { return uintptr(e.refs.Add(-1)) }

// folderEventsFolderChange has the window check the folder opened once
// the dialog has finished opening it, which fills in the name field.
func folderEventsFolderChange(e *folderEvents, dialog uintptr) uintptr {
	postMessageW.Call(e.w.hwnd, wmFolderChange, 0, 0)
	return sOK
}

// leaveVirtualFolder corrects the folder dialog when it opens a folder
// that is not on the file system, whose name it puts in the name field
// though Select Folder cannot resolve it: Home opens the user's profile,
// as Explorer's does, and the field is emptied so Select Folder picks the
// folder open.
func (w *window) leaveVirtualFolder() {
	d := w.folderPick
	if d == nil {
		return
	}
	var folder *comObject
	if failed(d.call(fileDialogGetFolder, uintptr(unsafe.Pointer(&folder)))) || folder == nil {
		return
	}
	defer folder.release()
	var name *uint16
	if !failed(folder.call(shellItemGetName, sigdnFileSysPath, uintptr(unsafe.Pointer(&name)))) {
		takeString(name)
		return
	}
	name = nil
	folder.call(shellItemGetName, sigdnDesktopAbsoluteParsing, uintptr(unsafe.Pointer(&name)))
	if strings.Contains(strings.ToUpper(takeString(name)), homeFolder) {
		if home, err := os.UserHomeDir(); err == nil {
			var item *comObject
			if hr, _, _ := shCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(wide(home))), 0,
				uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item))); !failed(hr) && item != nil {
				d.call(fileDialogSetFolder, item.ptr())
				item.release()
			}
		}
	}
	empty := uint16(0)
	d.call(fileDialogSetFileName, uintptr(unsafe.Pointer(&empty)))
}

// folderDialog returns the paths of the folders chosen in Windows' open
// dialog for folders (IFileOpenDialog), at most limit; none when it was
// cancelled. The comdlg32 dialogs pick only files.
func (w *window) folderDialog(limit int) ([]string, error) {
	if err := initCOM(); err != nil {
		return nil, err
	}
	var dialog *comObject
	if hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOpenDialog)), uintptr(unsafe.Pointer(&dialog))); failed(hr) || dialog == nil {
		return nil, hresultError("no folder dialog", hr)
	}
	defer dialog.release()
	var options uint32
	dialog.call(fileDialogGetOptions, uintptr(unsafe.Pointer(&options)))
	options |= fosPickFolders | fosForceFileSystem | fosPathMustExist | fosNoChangeDir
	if limit > 1 {
		options |= fosAllowMultiSelect
	}
	dialog.call(fileDialogSetOptions, uintptr(options))
	events := &folderEvents{vtbl: &folderEventsVtbl, w: w}
	var cookie uint32
	advised := !failed(dialog.call(fileDialogAdvise, uintptr(unsafe.Pointer(events)), uintptr(unsafe.Pointer(&cookie))))
	w.folderPick = dialog
	hr := dialog.call(fileDialogShow, w.owner())
	w.folderPick = nil
	if advised {
		dialog.call(fileDialogUnadvise, uintptr(cookie))
	}
	runtime.KeepAlive(events)
	if uint32(hr) == hresultCancelled {
		return nil, nil
	} else if failed(hr) {
		return nil, hresultError("folder dialog", hr)
	}
	var items *comObject
	if hr := dialog.call(fileDialogGetResults, uintptr(unsafe.Pointer(&items))); failed(hr) || items == nil {
		return nil, hresultError("no folders chosen", hr)
	}
	defer items.release()
	var n uint32
	items.call(itemArrayGetCount, uintptr(unsafe.Pointer(&n)))
	var paths []string
	for i := range min(int(n), max(limit, 1)) {
		var item *comObject
		if failed(items.call(itemArrayGetItemAt, uintptr(i), uintptr(unsafe.Pointer(&item)))) || item == nil {
			continue
		}
		var name *uint16
		if !failed(item.call(shellItemGetName, sigdnFileSysPath, uintptr(unsafe.Pointer(&name)))) {
			if p := takeString(name); p != "" {
				paths = append(paths, p)
			}
		}
		item.release()
	}
	return paths, nil
}

// saveDialog returns the path chosen in Windows' save dialog, which
// suggests name and asks before replacing a file; "" when it was
// cancelled.
func (w *window) saveDialog(name string) (string, error) {
	buf := make([]uint16, 32<<10)
	copy(buf[:len(buf)-1], syscall.StringToUTF16(name))
	filter := append(append(syscall.StringToUTF16("All files"), syscall.StringToUTF16("*.*")...), 0)
	var defExt *uint16
	if ext := strings.TrimPrefix(filepath.Ext(name), "."); ext != "" {
		defExt = &syscall.StringToUTF16(ext)[0]
	}
	ofn := openFileName{Owner: w.owner(), Filter: &filter[0], File: &buf[0], MaxFile: uint32(len(buf)), DefExt: defExt,
		Flags: ofnExplorer | ofnPathMustExist | ofnNoChangeDir | ofnOverwritePrompt}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	if ok, _, _ := getSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); ok == 0 {
		return "", dialogError()
	}
	return syscall.UTF16ToString(buf), nil
}

type popupRequest struct {
	entries []driver.MenuEntry
	at      driver.Point
	id      uint32
}

// popupMenu queues a context menu: TrackPopupMenuEx's loop must not run
// in the batch's. One shows at a time, so one tracking ends first.
func (w *window) popupMenu(entries []driver.MenuEntry, at driver.Point, id uint32) {
	if w.popupTracking {
		endMenu.Call()
	}
	w.popups = append(w.popups, popupRequest{entries, at, id})
	postMessageW.Call(w.hwnd, wmPopup, 0, 0)
}

// showPopup tracks the first context menu queued, chooses its item and
// reports it closed. A later one replaces those queued before it.
func (w *window) showPopup() {
	if w.popupTracking || len(w.popups) == 0 {
		return
	}
	p := w.popups[0]
	w.popups = w.popups[1:]
	if len(w.popups) > 0 {
		// Replaced before it showed.
		w.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: p.id})
		postMessageW.Call(w.hwnd, wmPopup, 0, 0)
		return
	}
	menu, _, _ := createPopupMenu.Call()
	var refs []menuRef
	if menu != 0 {
		w.appendMenuTo(menu, p.entries, &refs)
		at := point{w.px(p.at.X), w.px(p.at.Y)}
		clientToScreen.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
		w.popupTracking, w.popupShown = true, p.entries
		chosen, _, _ := trackPopupMenuEx.Call(menu, w.popupFlags(), uintptr(at.X), uintptr(at.Y), w.hwnd, 0)
		w.popupTracking, w.popupShown = false, nil
		destroyMenu.Call(menu)
		if c := w.popupChoice; c != nil {
			// A test chose, which ended the tracking.
			w.popupChoice, chosen = nil, 0
			for i, r := range refs {
				if *c != 0 && r.id == *c {
					chosen = uintptr(i + 1)
				}
			}
		}
		if i := int(chosen) - 1; i >= 0 && i < len(refs) {
			w.chooseMenu(refs[i])
		}
	}
	w.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: p.id})
	if len(w.popups) > 0 {
		postMessageW.Call(w.hwnd, wmPopup, 0, 0)
	}
}

// shownDark reports whether the app shows dark: as it forces, else as
// the system is.
func (w *window) shownDark() bool {
	for _, a := range []driver.Appearance{w.appearance, w.host} {
		switch a {
		case driver.AppearanceLight:
			return false
		case driver.AppearanceDark:
			return true
		}
	}
	return w.env.Dark
}

// setAppearance shows the title bar, the caption buttons and menus in a,
// or as the system's appearance is.
func (w *window) setAppearance(a driver.Appearance) {
	w.appearance = a
	w.darkTitleBar(w.shownDark())
	darkMenus(a)
	w.redrawAll()
}

// darkMenus has Win32's menus follow a: uxtheme's undocumented
// SetPreferredAppMode (ordinal 135) and FlushMenuThemes (136), as NSH and
// Explorer use them.
func darkMenus(a driver.Appearance) {
	if uxtheme.Load() != nil {
		return
	}
	mode, err := getProcOrdinal(uxtheme.Handle(), 135)
	if err != nil {
		return
	}
	const allowDark, forceDark, forceLight = 1, 2, 3
	m := allowDark
	switch a {
	case driver.AppearanceDark:
		m = forceDark
	case driver.AppearanceLight:
		m = forceLight
	}
	syscall.SyscallN(mode, uintptr(m))
	if flush, err := getProcOrdinal(uxtheme.Handle(), 136); err == nil && flush != 0 {
		syscall.SyscallN(flush)
	}
}

// getProcOrdinal returns the function of module exported at ordinal.
func getProcOrdinal(module uintptr, ordinal uintptr) (uintptr, error) {
	p, _, err := getProcAddress.Call(module, ordinal)
	if p == 0 {
		return 0, err
	}
	return p, nil
}

// windowFrame returns the window's frame from the main screen's top left, as
// setWindowFrame takes it; a minimized window, parked out of sight, keeps
// the one it had.
func (w *window) windowFrame() driver.WindowFrame {
	if w.embedded {
		return driver.WindowFrame{}
	}
	if isIconicWindow(w.hwnd) {
		return w.env.WindowFrame
	}
	var r rect
	getWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
	placements, base := monitorLayout()
	scale := w.scale
	if monitor, _, _ := monitorFromWindow.Call(w.hwnd, monitorDefaultNear); monitor != 0 {
		for _, placement := range placements {
			if placement.monitor == monitor {
				scale = placement.scale
				break
			}
		}
	}
	corner := driver.Point{X: driver.Points(float64(r.Left-base.Left) / scale), Y: driver.Points(float64(r.Top-base.Top) / scale)}
	size := driver.Point{X: driver.Points(float64(r.Right-r.Left) / scale), Y: driver.Points(float64(r.Bottom-r.Top) / scale)}
	return driver.WindowFrame{X: corner.X, Y: corner.Y, Move: true, Width: size.X, Height: size.Y}
}

// setWindowFrame moves and sizes the window: points from the main
// screen's top left, which is the virtual screen's origin, in the
// window's pixels.
func (w *window) setWindowFrame(f driver.WindowFrame) {
	if w.chrome.fullScreen {
		return
	}
	if w.zoomed() || isIconicWindow(w.hwnd) {
		showWindow.Call(w.hwnd, swRestore)
	}
	var r rect
	getWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
	x, y, cx, cy := r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top
	scale := w.scale
	var base rect
	if f.Move {
		placement, origin, ok := placementForFrame(f)
		if ok {
			scale, base = placement.scale, origin
		}
		x, y = base.Left+int32(math.Round(float64(f.X)*scale)), base.Top+int32(math.Round(float64(f.Y)*scale))
	}
	if f.Width > 0 {
		cx = int32(math.Round(float64(f.Width) * scale))
	}
	if f.Height > 0 {
		cy = int32(math.Round(float64(f.Height) * scale))
	}
	setWindowPos.Call(w.hwnd, 0, uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), swpNoZOrder|swpNoActivate)
}

func isIconicWindow(hwnd uintptr) bool {
	r, _, _ := isIconic.Call(hwnd)
	return r != 0
}

// setLevel puts the window above the others, among them, or at the
// bottom, where activating it raises it again.
func (w *window) setLevel(level driver.WindowLevel) {
	const flags = swpNoMove | swpNoSize | swpNoActivate
	switch level {
	case driver.WindowLevelFloating:
		setWindowPos.Call(w.hwnd, hwndTopmost, 0, 0, 0, 0, flags)
	case driver.WindowLevelDesktop:
		setWindowPos.Call(w.hwnd, hwndNoTopmost, 0, 0, 0, 0, flags)
		setWindowPos.Call(w.hwnd, hwndBottom, 0, 0, 0, 0, flags)
	default:
		setWindowPos.Call(w.hwnd, hwndNoTopmost, 0, 0, 0, 0, flags)
	}
}

// orderFront shows the window, restored when minimized, in front and
// focused.
func (w *window) orderFront() {
	if isIconicWindow(w.hwnd) {
		showWindow.Call(w.hwnd, swRestore)
	} else {
		showWindow.Call(w.hwnd, swShow)
	}
	bringWindowToTop.Call(w.hwnd)
	setForegroundWindow.Call(w.hwnd)
}

// setMenuBar says when the menu bar shows.
func (w *window) setMenuBar(mode driver.MenuBarMode) {
	w.chrome.menuBar, w.chrome.revealed = mode, false
	w.showMenuBar()
}

// menuBarKey follows the keys that show a hidden menu bar: Alt pressed
// and released alone toggles it, and is kept from DefWindowProc, which
// would enter menu mode; Alt with a mnemonic and F10 show it for the menu
// DefWindowProc then opens. It reports whether it took the key.
func (w *window) menuBarKey(message uint32, vk uintptr, lParam uintptr) bool {
	c := &w.chrome
	if c.menuBar != driver.MenuBarAutoHide || c.menu == nil {
		return false
	}
	switch {
	case message == wmSysKeyDown && vk == vkMenu:
		if !repeated(lParam) {
			c.altAlone = true
		}
	case message == wmSysKeyUp && vk == vkMenu:
		alone := c.altAlone
		c.altAlone = false
		if alone {
			c.revealed = !c.revealed
			w.showMenuBar()
			return true
		}
	case message == wmSysKeyDown:
		c.altAlone = false
		if (vk == vkF10 || keyDown(vkMenu)) && !c.revealed && !c.menuOpen {
			c.menuOpen = true
			w.showMenuBar()
		}
	}
	return false
}

// menuLoop follows a menu of the bar opening and closing: the bar shows
// while it is open, and hides once it closes.
func (w *window) menuLoop(enter bool, popup bool) {
	c := &w.chrome
	if c.menuBar != driver.MenuBarAutoHide || popup {
		return
	}
	if enter {
		return
	}
	if c.menuOpen || c.revealed {
		c.menuOpen, c.revealed = false, false
		w.showMenuBar()
	}
}

// hideMenuBar hides a shown menu bar again after a press in the window.
func (w *window) hideMenuBar() {
	if c := &w.chrome; c.menuBar == driver.MenuBarAutoHide && c.revealed {
		c.revealed = false
		w.showMenuBar()
	}
}

// osVersion returns Windows' version: 10.0 on Windows 11 too.
func osVersion() driver.OSVersion {
	var v osVersionInfo
	v.Size = uint32(unsafe.Sizeof(v))
	if r, _, _ := rtlGetVersion.Call(uintptr(unsafe.Pointer(&v))); r != 0 {
		return driver.OSVersion{}
	}
	return driver.OSVersion{Major: int(v.Major), Minor: int(v.Minor)}
}
