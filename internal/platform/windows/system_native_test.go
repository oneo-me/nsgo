//go:build windows

package windows

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
)

var (
	getMenu           = user32.NewProc("GetMenu")
	getWindowLongProc = user32.NewProc("GetWindowLongPtrW")
	getWindowProc     = user32.NewProc("GetWindow")
	getDlgItem        = user32.NewProc("GetDlgItem")
	getGUIThreadInfo  = user32.NewProc("GetGUIThreadInfo")

	getWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	sendMessage              = user32.NewProc("SendMessageW")
)

// TestServicesReachTheSystem puts an image and files on the clipboard
// and reads them back, opens a URL no app handles, picks an image and a
// file and saves one through the system's dialogs, moves and floats the
// window, hides its menu bar until Alt, shows a context menu and quits,
// which returns from Run.
func TestServicesReachTheSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-services"
	var cx *core.Scope
	status := make(chan string, 16)
	dialogAnswers = status
	app := func(c *core.Scope) core.Node {
		cx = c
		c.SetApplicationMenu([]core.MenuItem{{Label: "File", Items: []core.MenuItem{{Label: "New", OnSelect: func() {}}}}})
		return core.View{Style: core.Style{Background: driver.RGB(0xffffff)}}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 360, Height: 240})
		status <- "returned"
	}()
	hwnd := waitForWindow(t, title, done)
	if hwnd == 0 {
		return
	}
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

	// An image, PNG and bitmap alike.
	m := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for i := range m.Pix {
		m.Pix[i] = 0xff
	}
	var b bytes.Buffer
	png.Encode(&b, m)
	cx.Post(func() { cx.SetClipboardImage(b.Bytes()) })
	got := ask(func(say func(string)) {
		cx.HasClipboardImage(func(has bool) {
			cx.ReadClipboardImage(func(img core.ClipboardImage, ok bool) {
				say(fmt.Sprintf("%v %v %v %d", has, ok, img.Size, len(img.PNG)))
			})
		})
	})
	if !strings.HasPrefix(got, "true true {3 2}") {
		t.Errorf("image = %s", got)
	}
	onMain(t, hwnd, func(w *window) {
		// Another app's bitmap without a PNG still reads as one.
		openClipboard.Call(w.hwnd)
		emptyClipboard.Call()
		closeClipboard.Call()
		dib := make([]byte, 40+4*4)
		binary.LittleEndian.PutUint32(dib, 40)
		binary.LittleEndian.PutUint32(dib[4:], 2)
		binary.LittleEndian.PutUint32(dib[8:], 2)
		binary.LittleEndian.PutUint16(dib[12:], 1)
		binary.LittleEndian.PutUint16(dib[14:], 32)
		putClipboard(w.hwnd, cfDIB, dib)
	})
	if got := ask(func(say func(string)) {
		cx.ReadClipboardImage(func(img core.ClipboardImage, ok bool) { say(fmt.Sprintf("%v %v", ok, img.Size)) })
	}); got != "true {2 2}" {
		t.Errorf("bitmap = %s", got)
	}

	// Files a file manager copied.
	file := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	onMain(t, hwnd, func(w *window) {
		openClipboard.Call(w.hwnd)
		emptyClipboard.Call()
		closeClipboard.Call()
		units := utf16.Encode([]rune(file + "\x00\x00"))
		drop := make([]byte, 20+2*len(units))
		binary.LittleEndian.PutUint32(drop, 20) // DROPFILES.pFiles
		binary.LittleEndian.PutUint32(drop[16:], 1)
		for i, u := range units {
			binary.LittleEndian.PutUint16(drop[20+2*i:], u)
		}
		putClipboard(w.hwnd, cfHDrop, drop)
	})
	if got := ask(func(say func(string)) {
		cx.ReadClipboardFiles(func(uris []string) {
			cx.HasClipboardImage(func(has bool) { say(fmt.Sprintf("%v %v", uris, has)) })
		})
	}); !strings.HasSuffix(got, "/a.txt] false") || !strings.HasPrefix(got, "[file:///") {
		t.Errorf("files = %s", got)
	}
	cx.Post(func() { cx.SetClipboard("") })

	if got := ask(func(say func(string)) {
		cx.OpenURL("nsgo-unhandled-scheme-7f3:x", func(ok bool) { say(fmt.Sprint(ok)) })
	}); got != "false" {
		t.Errorf("an unhandled scheme opened: %s", got)
	}

	// The open dialog picks a PNG, scaled down as asked.
	shot := filepath.Join(t.TempDir(), "shot.png")
	big := image.NewRGBA(image.Rect(0, 0, 40, 20))
	var bb bytes.Buffer
	png.Encode(&bb, big)
	os.WriteFile(shot, bb.Bytes(), 0o644)
	cx.Post(func() {
		cx.PickImages(core.PickOptions{MaxDimension: 10}, func(files []core.PickedFile) {
			var parts []string
			for _, f := range files {
				parts = append(parts, fmt.Sprintf("%s %s %v", f.Name, f.MIME, f.Size))
			}
			status <- strings.Join(parts, ",")
		})
	})
	answerFileDialog(t, hwnd, shot)
	select {
	case s := <-status:
		if s != "shot.png image/png {10 5}" {
			t.Errorf("picked %q", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the pick did not answer")
	}

	// The open dialog picks any file as it is; the save dialog copies it
	// where the name typed says.
	notes := filepath.Join(filepath.Dir(shot), "notes.txt")
	os.WriteFile(notes, []byte("notes"), 0o644)
	cx.Post(func() {
		cx.PickFiles(core.PickFilesOptions{Types: []string{".txt"}}, func(files []core.File, err error) {
			var parts []string
			for _, f := range files {
				parts = append(parts, fmt.Sprintf("%s %s %d %v", f.Name, f.MIME, f.Size, f.Path == notes))
			}
			status <- fmt.Sprint(strings.Join(parts, ","), " ", err)
		})
	})
	answerFileDialog(t, hwnd, notes)
	if s := <-status; s != "notes.txt text/plain 5 true <nil>" {
		t.Errorf("picked %q", s)
	}
	folder := filepath.Join(filepath.Dir(shot), "photos")
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
	answerFolderDialog(t, hwnd, folder)
	if s := <-status; s != "photos true <nil>" {
		t.Errorf("picked %q", s)
	}
	// Windows 11's Home is not on the file system: it opens the profile.
	home, _ := os.UserHomeDir()
	cx.Post(func() {
		cx.PickDirectories(core.PickDirectoriesOptions{}, func(dirs []core.Directory, err error) {
			var parts []string
			for _, d := range dirs {
				parts = append(parts, fmt.Sprint(strings.EqualFold(d.Path, home)))
			}
			status <- fmt.Sprint(strings.Join(parts, ","), " ", err)
		})
	})
	answerFolderDialog(t, hwnd, "shell:::"+homeFolder)
	if s := <-status; s != "true <nil>" {
		t.Errorf("picked Home as %q", s)
	}
	copied := filepath.Join(filepath.Dir(shot), "copy.txt")
	cx.Post(func() {
		cx.SaveFile(core.SaveOptions{Path: notes}, func(saved bool, err error) { status <- fmt.Sprint(saved, err) })
	})
	answerFileDialog(t, hwnd, copied)
	if s := <-status; s != "true <nil>" {
		t.Errorf("saved %q", s)
	}
	if b, _ := os.ReadFile(copied); string(b) != "notes" {
		t.Errorf("the copy holds %q", b)
	}
	cx.Post(func() {
		cx.SaveFile(core.SaveOptions{Name: "cancelled.txt", Data: []byte("x")}, func(saved bool, err error) { status <- fmt.Sprint(saved, err) })
	})
	cancelFileDialog(t, hwnd)
	if s := <-status; s != "false <nil>" {
		t.Errorf("a cancelled save answered %q", s)
	}

	// The frame is from the main screen's top left, in the window's
	// pixels; a new height keeps the top edge.
	var scale float64
	onMain(t, hwnd, func(w *window) { scale = w.scale })
	px := func(p float64) int32 { return int32(p*scale + 0.5) }
	cx.Post(func() { cx.SetWindowFrame(core.WindowFrame{X: 100, Y: 120, Move: true, Width: 500, Height: 400}) })
	cx.Post(func() { cx.SetWindowFrame(core.WindowFrame{Height: 300}) })
	cx.Post(func() { cx.SetWindowLevel(core.WindowLevelFloating) })
	deadline := time.Now().Add(3 * time.Second)
	var r rect
	var ex uintptr
	for {
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		const gwlExStyle, wsExTopmost = ^uintptr(19), 0x8
		ex, _, _ = getWindowLongProc.Call(hwnd, gwlExStyle)
		if r == (rect{px(100), px(120), px(600), px(420)}) && ex&wsExTopmost != 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if r != (rect{px(100), px(120), px(600), px(420)}) || ex&0x8 == 0 {
		t.Errorf("frame = %+v, ex style %x", r, ex)
	}
	cx.Post(func() { cx.SetWindowLevel(core.WindowLevelNormal) })

	// The menu bar hides until Alt is pressed and released alone.
	menu := func() uintptr {
		var m uintptr
		onMain(t, hwnd, func(*window) { m, _, _ = getMenu.Call(hwnd) })
		return m
	}
	if menu() == 0 {
		t.Error("no menu bar")
	}
	cx.Post(func() { cx.SetMenuBar(core.MenuBarAutoHide) })
	time.Sleep(200 * time.Millisecond)
	if menu() != 0 {
		t.Error("an auto-hiding menu bar shows")
	}
	postMessageW.Call(hwnd, wmSysKeyDown, vkMenu, 1<<29)
	postMessageW.Call(hwnd, wmSysKeyUp, vkMenu, 1<<29|1<<30|1<<31)
	time.Sleep(200 * time.Millisecond)
	if menu() == 0 {
		t.Error("Alt did not show the menu bar")
	}
	postMessageW.Call(hwnd, wmSysKeyDown, vkMenu, 1<<29)
	postMessageW.Call(hwnd, wmSysKeyUp, vkMenu, 1<<29|1<<30|1<<31)
	time.Sleep(200 * time.Millisecond)
	if menu() != 0 {
		t.Error("Alt did not hide the menu bar again")
	}

	// A context menu tracks until it ends, then reports it closed.
	cx.Post(func() {
		cx.PopupMenu([]core.MenuItem{{Label: "Copy", OnSelect: func() {}}}, core.Point{X: 20, Y: 20},
			func() { status <- "closed" })
	})
	deadline = time.Now().Add(3 * time.Second)
	for {
		if m, _, _ := findWindowExW.Call(0, 0, uintptr(unsafe.Pointer(wide("#32768"))), 0); m != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no context menu showed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	onMain(t, hwnd, func(*window) { endMenu.Call() })
	select {
	case s := <-status:
		if s != "closed" {
			t.Errorf("status = %s", s)
		}
	case <-time.After(3 * time.Second):
		t.Error("the context menu did not report closing")
	}

	// A forced dark appearance darkens the title bar.
	darkBar := func() int32 {
		var on int32
		onMain(t, hwnd, func(*window) {
			dwmGetWindowAttribute.Call(hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&on)), 4)
		})
		return on
	}
	for _, a := range []core.Appearance{core.AppearanceDark, core.AppearanceLight} {
		cx.Post(func() { cx.SetAppearance(a) })
		time.Sleep(200 * time.Millisecond)
		if got, want := darkBar(), int32(boolArg(a == core.AppearanceDark)); got != want {
			t.Errorf("appearance %v: dark title bar = %d", a, got)
		}
	}
	cx.Post(func() { cx.SetAppearance(core.AppearanceSystem) })

	if got := ask(func(say func(string)) {
		v := cx.Env().OSVersion()
		say(fmt.Sprintf("%v", v.Major >= 10))
	}); got != "true" {
		t.Errorf("OS version = %s", got)
	}

	cx.Post(cx.Quit)
	select {
	case s := <-status:
		if s != "returned" {
			t.Errorf("status = %s", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Quit did not return from Run")
	}
}

// TestCaptionButtonsAndMaterialFollowTheApp hides the caption buttons of
// a hidden title bar and shows them again, and changes the backdrop of a
// window started with a material, where Windows draws one.
func TestCaptionButtonsAndMaterialFollowTheApp(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-chrome"
	var cx *core.Scope
	app := func(c *core.Scope) core.Node {
		cx = c
		c.SetTitleBar(core.TitleBarHidden, 40)
		return core.View{}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 360, Height: 240, Material: driver.WindowMaterialMica})
	}()
	hwnd := waitForWindow(t, title, done)
	if hwnd == 0 {
		return
	}
	// until polls the window's thread until check holds.
	until := func(what string, check func(w *window) bool) {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); ; {
			ok := false
			onMain(t, hwnd, func(w *window) { ok = check(w) })
			if ok {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s did not happen", what)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	var width driver.Points
	onMain(t, hwnd, func(w *window) {
		var client rect
		getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
		width = w.point(client.Right, 0).X
	})
	closeButton := driver.Point{X: width - captionMargin - captionButtonSize/2, Y: 20}
	until("the hidden title bar", func(w *window) bool {
		return w.titleBarArea() == driver.Edges{Top: 40, Right: captionRight} && w.captionAt(closeButton) == htClose
	})
	cx.Post(func() { cx.SetWindowControls(core.WindowControls{Hidden: true}) })
	until("hiding the caption buttons", func(w *window) bool {
		return w.titleBarArea() == driver.Edges{Top: 40} && w.captionAt(closeButton) == 0 && w.env.TitleBar.Right == 0
	})
	cx.Post(func() { cx.SetWindowControls(core.WindowControls{}) })
	until("showing them again", func(w *window) bool { return w.captionAt(closeButton) == htClose })

	backdrops := false
	onMain(t, hwnd, func(w *window) { backdrops = w.material == driver.WindowMaterialMica })
	if !backdrops {
		t.Log("no system backdrop on this Windows: the material stays off")
	} else {
		cx.Post(func() { cx.SetWindowMaterial(core.WindowMaterialAcrylic) })
		until("Acrylic", func(w *window) bool {
			return w.backdrop == backdropAcrylic && w.env.WindowMaterial == driver.WindowMaterialAcrylic
		})
		cx.Post(func() { cx.SetWindowMaterial(core.WindowMaterialNone) })
		until("an opaque window", func(w *window) bool {
			return w.backdrop == backdropNone && w.env.WindowMaterial == driver.WindowMaterialNone
		})
		cx.Post(func() { cx.SetWindowMaterial(core.WindowMaterialTabbed) })
		until("Mica Alt", func(w *window) bool { return w.backdrop == backdropMicaAlt })
	}
	cx.Post(cx.Quit)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Quit did not return from Run")
	}
}

// putClipboard puts b on the open clipboard of hwnd as format.
func putClipboard(hwnd, format uintptr, b []byte) {
	openClipboard.Call(hwnd)
	defer closeClipboard.Call()
	h, _, _ := globalAlloc.Call(gmemMoveable, uintptr(len(b)))
	p, _, _ := globalLock.Call(h)
	rtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	globalUnlock.Call(h)
	setClipboardData.Call(format, h)
}

// dialogAnswers is the test's status channel, which an answer that
// comes before its dialog shows reaches.
var dialogAnswers chan string

// fileDialog waits for the open or save dialog owned by hwnd and
// returns it and its file name field once it shows: the open dialog's
// control 0x47C, or the save dialog's, which has another, as the field
// that has the focus.
func fileDialog(t *testing.T, hwnd uintptr) (dlg, field uintptr) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		dlg, _, _ = findWindowExW.Call(0, 0, uintptr(unsafe.Pointer(wide("#32770"))), 0)
		for dlg != 0 {
			const gwOwner = 4
			if owner, _, _ := getWindowProc.Call(dlg, gwOwner); owner == hwnd {
				break
			}
			dlg, _, _ = findWindowExW.Call(0, dlg, uintptr(unsafe.Pointer(wide("#32770"))), 0)
		}
		if dlg != 0 {
			time.Sleep(500 * time.Millisecond) // the dialog finishes loading its folder
			const fileName = 0x47C
			if field, _, _ = getDlgItem.Call(dlg, fileName); field == 0 {
				thread, _, _ := getWindowThreadProcessID.Call(dlg, 0)
				var info guiThreadInfo
				info.Size = uint32(unsafe.Sizeof(info))
				getGUIThreadInfo.Call(thread, uintptr(unsafe.Pointer(&info)))
				field = info.Focus
			}
			if field != 0 {
				return dlg, field
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("no file dialog showed")
		}
		if len(dialogAnswers) > 0 {
			t.Fatalf("the dialog answered without showing: %s", <-dialogAnswers)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// guiThreadInfo is GUITHREADINFO.
type guiThreadInfo struct {
	Size, Flags                                 uint32
	Active, Focus, Capture, MenuOwner, MoveSize uintptr
	Caret                                       uintptr
	CaretRect                                   [4]int32
}

// answerFileDialog types path into the dialog hwnd shows and presses OK.
func answerFileDialog(t *testing.T, hwnd uintptr, path string) {
	t.Helper()
	const wmSetText, idOK = 0x000C, 1
	dlg, field := fileDialog(t, hwnd)
	sendMessage.Call(field, wmSetText, 0, uintptr(unsafe.Pointer(wide(path))))
	postMessageW.Call(dlg, wmCommand, idOK, 0)
}

// answerFolderDialog types path into the folder dialog hwnd shows and
// presses Select Folder until it closes: a press may first open the
// folder typed, and the next picks the folder open.
func answerFolderDialog(t *testing.T, hwnd uintptr, path string) {
	t.Helper()
	const idOK = 1
	isWindow := user32.NewProc("IsWindow")
	dlg, _ := fileDialog(t, hwnd)
	answerFileDialog(t, hwnd, path)
	for range 5 {
		time.Sleep(time.Second)
		if open, _, _ := isWindow.Call(dlg); open == 0 {
			return
		}
		postMessageW.Call(dlg, wmCommand, idOK, 0)
	}
}

// cancelFileDialog presses Cancel in the dialog hwnd shows.
func cancelFileDialog(t *testing.T, hwnd uintptr) {
	t.Helper()
	const idCancel = 2
	dlg, _ := fileDialog(t, hwnd)
	postMessageW.Call(dlg, wmCommand, idCancel, 0)
}
