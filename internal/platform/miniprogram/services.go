//go:build js && wasm

package miniprogram

import (
	"log"
	"path"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Services are the mini program's own: alerts and action sheets are
// WeChat's dialogs, the clipboard and storage its APIs, pictures come
// from its media picker, and the page's navigation bar shows the title.
// Requests it has no API for are answered at once, as failed.

// call calls API name of wx with opts, and done with its result once it
// succeeds or fails.
func (s *surface) call(name string, opts map[string]any, done func(res js.Value, ok bool)) {
	var success, fail js.Func
	finish := func(res js.Value, ok bool) {
		success.Release()
		fail.Release()
		if done != nil {
			done(res, ok)
		}
	}
	success = js.FuncOf(func(_ js.Value, args []js.Value) any { finish(first(args), true); return nil })
	fail = js.FuncOf(func(_ js.Value, args []js.Value) any { finish(first(args), false); return nil })
	opts["success"], opts["fail"] = success, fail
	if fn := s.wx.Get(name); fn.Type() != js.TypeFunction {
		finish(js.Undefined(), false)
		return
	}
	s.wx.Call(name, opts)
}

func first(args []js.Value) js.Value {
	if len(args) > 0 {
		return args[0]
	}
	return js.Undefined()
}

// service carries out an item no view shows.
func (s *surface) service(it *driver.MountItem) {
	switch it.Kind {
	case driver.ItemAlert:
		s.alert(it.Alert)
	case driver.ItemSetClipboard:
		s.call("setClipboardData", map[string]any{"data": it.Value}, nil)
	case driver.ItemReadClipboard:
		s.readClipboard(it)
	case driver.ItemSetClipboardImage:
		// WeChat's clipboard holds text alone.
	case driver.ItemTitle:
		s.call("setNavigationBarTitle", map[string]any{"title": it.Value}, nil)
	case driver.ItemDismissKeyboard:
		s.call("hideKeyboard", map[string]any{}, nil)
	case driver.ItemOpenURL:
		// A mini program opens no other app nor browser: the link goes on
		// the clipboard for the user to paste, as WeChat's own pages do.
		s.call("setClipboardData", map[string]any{"data": it.Value}, nil)
		s.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: it.Count})
	case driver.ItemPickImages:
		s.pickImages(*it.Pick, it.Count, "album")
	case driver.ItemTakePhoto:
		s.pickImages(*it.Pick, it.Count, "camera")
	case driver.ItemPickFiles:
		s.pickFiles(*it.Files, it.Count)
	case driver.ItemPickDirectories:
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Text: driver.NoDirectoryPicker})
	case driver.ItemOpenFile:
		s.openFile(it.Open, it.Count)
	case driver.ItemShare:
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Text: driver.NoShareSheet})
	case driver.ItemSaveFile, driver.ItemInstallPackage, driver.ItemCanInstallPackages, driver.ItemInstallSettings,
		driver.ItemPickScreenColor:
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count})
	case driver.ItemPopupMenu, driver.ItemTrayMenu:
		s.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: it.Count})
	case driver.ItemLaunchAtLogin:
		s.engine.Dispatch(driver.Event{Kind: driver.EventLaunchAtLogin, Count: it.Count})
	case driver.ItemImageInfo:
		s.imageInfo(it)
	case driver.ItemAppearance:
		s.appearance = it.Appearance
		s.navigationColors()
	case driver.ItemStatusBar:
		s.statusBar(it.StatusBar)
	case driver.ItemSpeech:
		// The environment says it has no speech: the core sends none.
	}
}

// alert shows WeChat's modal for one or two actions, its action sheet
// for more or for a sheet. The sheet lists the actions but the cancel
// one, which its own cancel button is.
func (s *surface) alert(c *driver.AlertConfig) {
	answer := func(index int) {
		s.engine.Dispatch(driver.Event{Kind: driver.EventAlert, Count: c.ID, Index: index})
	}
	cancel := -1
	for i, a := range c.Actions {
		if a.Role == driver.AlertCancel {
			cancel = i
		}
	}
	actions := c.Actions
	if len(actions) == 0 {
		actions = []driver.AlertAction{{Label: "OK"}}
	}
	if c.Sheet || len(actions) > 2 {
		var labels []any
		var index []int
		for i, a := range actions {
			if i != cancel {
				labels = append(labels, a.Label)
				index = append(index, i)
			}
		}
		opts := map[string]any{"itemList": labels}
		if c.Title != "" || c.Message != "" {
			opts["alertText"] = c.Title + map[bool]string{true: "\n", false: ""}[c.Title != "" && c.Message != ""] + c.Message
		}
		s.call("showActionSheet", opts, func(res js.Value, ok bool) {
			if !ok {
				answer(cancel)
				return
			}
			if i := res.Get("tapIndex").Int(); i >= 0 && i < len(index) {
				answer(index[i])
			}
		})
		return
	}
	confirm, other := len(actions)-1, -1
	if len(actions) == 2 {
		confirm, other = 1, 0
		if cancel == 1 {
			confirm, other = 0, 1
		}
	}
	opts := map[string]any{"title": c.Title, "content": c.Message, "showCancel": other >= 0,
		"confirmText": actions[confirm].Label}
	if actions[confirm].Role == driver.AlertDestructive {
		opts["confirmColor"] = "#FA5151"
	}
	if other >= 0 {
		opts["cancelText"] = actions[other].Label
	}
	s.call("showModal", opts, func(res js.Value, ok bool) {
		switch {
		case ok && res.Get("confirm").Bool():
			answer(confirm)
		case ok && other >= 0:
			answer(other)
		default:
			answer(cancel)
		}
	})
}

func (s *surface) readClipboard(it *driver.MountItem) {
	ev := driver.Event{Kind: driver.EventClipboard, Count: it.Count}
	switch it.Clipboard {
	case driver.ClipboardReadText, driver.ClipboardReadHasText:
		s.call("getClipboardData", map[string]any{}, func(res js.Value, ok bool) {
			if ok && res.Get("data").Type() == js.TypeString {
				ev.Text = res.Get("data").String()
				ev.Finished = ev.Text != ""
				if it.Clipboard == driver.ClipboardReadHasText {
					ev.Text = ""
				}
			}
			s.engine.Dispatch(ev)
		})
	default:
		if it.Clipboard == driver.ClipboardReadImage {
			ev.Data = (*driver.ClipboardImage)(nil)
		}
		s.engine.Dispatch(ev)
	}
}

// readFile reads a file of the mini program's file system.
func (s *surface) readFile(p string) ([]byte, bool) {
	var data []byte
	ok := func() (ok bool) {
		defer func() {
			if recover() != nil {
				ok = false
			}
		}()
		buf := s.wx.Call("getFileSystemManager").Call("readFileSync", p)
		arr := js.Global().Get("Uint8Array").New(buf)
		data = make([]byte, arr.Length())
		js.CopyBytesToGo(data, arr)
		return true
	}()
	return data, ok
}

// pickImages shows WeChat's media picker for images from source, the
// album or the camera.
func (s *surface) pickImages(r driver.PickRequest, count uint32, source string) {
	answer := func(files []driver.PickedFile, failed string) {
		s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: count, Data: files, Text: failed})
	}
	sizes := []any{"original"}
	if r.MaxDimension > 0 || r.Quality > 0 {
		sizes = []any{"compressed"}
	}
	s.call("chooseMedia", map[string]any{"count": max(r.Limit, 1), "mediaType": []any{"image"},
		"sourceType": []any{source}, "sizeType": sizes}, func(res js.Value, ok bool) {
		if !ok {
			answer(nil, "")
			return
		}
		var files []driver.PickedFile
		list := res.Get("tempFiles")
		for i := range list.Length() {
			f := list.Index(i)
			p := f.Get("tempFilePath").String()
			data, ok := s.readFile(p)
			if !ok {
				continue
			}
			pf := driver.PickedFile{Name: path.Base(p), Data: data}
			if w := f.Get("width"); w.Type() == js.TypeNumber {
				pf.Size = driver.Size{Width: driver.Points(w.Float()), Height: driver.Points(f.Get("height").Float())}
			}
			files = append(files, pf)
		}
		answer(driver.FinishPick(r, files, nil), "")
	})
}

// pickFiles picks files from a WeChat chat, the only files a mini
// program reaches.
func (s *surface) pickFiles(r driver.FilesRequest, count uint32) {
	answer := func(files []driver.File) {
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: count, Data: files})
	}
	s.call("chooseMessageFile", map[string]any{"count": max(r.Limit, 1), "type": "all"}, func(res js.Value, ok bool) {
		if !ok {
			answer(nil)
			return
		}
		var files []driver.File
		list := res.Get("tempFiles")
		for i := range list.Length() {
			f := list.Index(i)
			name := f.Get("name").String()
			files = append(files, driver.File{Name: name, MIME: driver.MIMEForName(name),
				Size: int64(f.Get("size").Float()), Path: f.Get("path").String(), Temporary: true})
		}
		answer(files)
	})
}

// openFile opens a document in WeChat's viewer.
func (s *surface) openFile(f *driver.SharedFile, count uint32) {
	if f == nil || f.Path == "" {
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: count, Text: driver.NoFileOpener})
		return
	}
	s.call("openDocument", map[string]any{"filePath": f.Path, "showMenu": true}, func(res js.Value, ok bool) {
		ev := driver.Event{Kind: driver.EventFiles, Count: count, Finished: ok}
		if !ok {
			ev.Text = res.Get("errMsg").String()
		}
		s.engine.Dispatch(ev)
	})
}

func (s *surface) imageInfo(it *driver.MountItem) {
	ev := driver.Event{Kind: driver.EventImageInfo, Count: it.Count}
	s.call("getImageInfo", map[string]any{"src": s.imageURL(it.Image.Source)}, func(res js.Value, ok bool) {
		if ok {
			ev.Size = driver.Size{Width: driver.Points(res.Get("width").Float()), Height: driver.Points(res.Get("height").Float())}
		} else {
			ev.Text = res.Get("errMsg").String()
		}
		s.engine.Dispatch(ev)
	})
}

// statusBar sets the navigation bar's text, which the status bar takes.
func (s *surface) statusBar(c *driver.StatusBarConfig) {
	dark := s.dark()
	switch c.Style {
	case driver.StatusBarDark:
		dark = false
	case driver.StatusBarLight:
		dark = true
	}
	s.setNavigationColors(dark)
}

// dark reports whether the app shows dark: the system's or the one it
// forces.
func (s *surface) dark() bool {
	switch s.appearance {
	case driver.AppearanceDark:
		return true
	case driver.AppearanceLight:
		return false
	}
	return s.env.Dark
}

// navigationColors colors the navigation bar for the app's appearance.
func (s *surface) navigationColors() { s.setNavigationColors(s.dark()) }

func (s *surface) setNavigationColors(dark bool) {
	front, back := "#000000", "#ffffff"
	if dark {
		front, back = "#ffffff", "#000000"
	}
	s.bridge.Call("navigationColors", front, back)
}

// storage keeps the app's entries in the mini program's storage, each
// under its key with the app's prefix.
type storage struct {
	wx     js.Value
	prefix string
}

func (st *storage) Load() (entries map[string]string, err error) {
	entries = map[string]string{}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("nsgo: the app's storage could not be read: %v", r)
		}
	}()
	keys := st.wx.Call("getStorageInfoSync").Get("keys")
	for i := range keys.Length() {
		k := keys.Index(i).String()
		if len(k) >= len(st.prefix) && k[:len(st.prefix)] == st.prefix {
			if v := st.wx.Call("getStorageSync", k); v.Type() == js.TypeString {
				entries[k[len(st.prefix):]] = v.String()
			}
		}
	}
	return entries, nil
}

func (st *storage) Write(c driver.StorageChange) {
	defer func() {
		// Past the mini program's 10 MB the entries stay in memory.
		if r := recover(); r != nil {
			log.Printf("nsgo: the app's storage could not be written: %v", r)
		}
	}()
	switch c.Op {
	case driver.StorageSet:
		st.wx.Call("setStorageSync", st.prefix+c.Key, c.Value)
	case driver.StorageRemove:
		st.wx.Call("removeStorageSync", st.prefix+c.Key)
	case driver.StorageClear:
		keys := st.wx.Call("getStorageInfoSync").Get("keys")
		for i := range keys.Length() {
			if k := keys.Index(i).String(); len(k) >= len(st.prefix) && k[:len(st.prefix)] == st.prefix {
				st.wx.Call("removeStorageSync", k)
			}
		}
	}
}
