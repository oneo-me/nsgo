package core

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Services the platform performs for the app, asked for with command
// items and answered with events.

// ShowAlert is the core's part of base.ShowAlertDialog, which apps use:
// it shows a native alert and calls done with the index of the button
// chosen, or -1 when it was dismissed without one. done runs even
// when the owner that showed the alert is gone, such as a closed menu.
func (cx *Scope) ShowAlert(a AlertConfig, done func(index int)) {
	t := cx.tree()
	if t == nil {
		panic("core: ShowAlert needs a mounted tree")
	}
	t.alertID++
	a.ID = t.alertID
	if t.alerts == nil {
		t.alerts = map[uint32]alertWait{}
	}
	t.alerts[a.ID] = alertWait{cx.current(), done}
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemAlert, Alert: &a})
	t.changed = true
}

type alertWait struct {
	owner *Scope
	done  func(int)
}

func (t *Tree) answerAlert(id uint32, index int) {
	w, ok := t.alerts[id]
	if !ok {
		return
	}
	delete(t.alerts, id)
	owner := w.owner
	if owner.disposed {
		owner = t.root
	}
	if w.done != nil {
		t.within(owner, func() { w.done(index) })
	}
}

// wait is a request the platform answers with an event.
type wait struct {
	owner *Scope
	done  func(driver.Event)
}

// request numbers item, sends it and calls done with the event that
// answers it, under the current owner, or the root's when that owner is
// gone by then.
func (t *Tree) request(cx *Scope, item driver.MountItem, done func(driver.Event)) {
	t.alertID++
	item.Count = t.alertID
	if t.waits == nil {
		t.waits = map[uint32]wait{}
	}
	t.waits[item.Count] = wait{cx.current(), done}
	t.commands = append(t.commands, item)
	t.changed = true
}

// answer runs the wait ev answers, once.
func (t *Tree) answer(ev driver.Event) {
	w, ok := t.waits[ev.Count]
	if !ok {
		return
	}
	delete(t.waits, ev.Count)
	owner := w.owner
	if owner.disposed {
		owner = t.root
	}
	t.within(owner, func() { w.done(ev) })
}

// DismissKeyboard takes focus away from the focused text field, hiding
// the software keyboard.
func (cx *Scope) DismissKeyboard() {
	if t := cx.tree(); t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemDismissKeyboard})
		t.changed = true
	}
}

// SetClipboard puts text on the system clipboard, replacing what it
// holds.
func (cx *Scope) SetClipboard(text string) {
	if t := cx.tree(); t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemSetClipboard, Value: text})
		t.changed = true
	}
}

// SetClipboardImage puts the image data encodes on the system clipboard,
// replacing what it holds: PNG, JPEG, GIF or WebP bytes, as
// ReadClipboardImage gives them or an ImageSource holds them.
func (cx *Scope) SetClipboardImage(data []byte) {
	if t := cx.tree(); t != nil && len(data) > 0 {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemSetClipboardImage, Data: data})
		t.changed = true
	}
}

func (cx *Scope) readClipboard(what driver.ClipboardRead, done func(driver.Event)) {
	if t := cx.tree(); t != nil {
		t.request(cx, driver.MountItem{Kind: driver.ItemReadClipboard, Clipboard: what}, done)
	}
}

// ReadClipboard calls done with the clipboard's text, "" when it holds
// none. On iOS, reading it may show the system's paste notice.
func (cx *Scope) ReadClipboard(done func(text string)) {
	cx.readClipboard(driver.ClipboardReadText, func(ev driver.Event) { done(ev.Text) })
}

// HasClipboardText calls done with whether the clipboard holds text,
// without reading it, so iOS shows no paste notice.
func (cx *Scope) HasClipboardText(done func(bool)) {
	cx.readClipboard(driver.ClipboardReadHasText, func(ev driver.Event) { done(ev.Finished) })
}

// ReadClipboardImage calls done with the clipboard's image as a PNG, or
// ok false when it holds none. An image it cannot read, or whose PNG is
// over MaxClipboardImage bytes (CodeQuotaExceeded), fails. On iOS,
// reading it may show the system's paste notice.
func (cx *Scope) ReadClipboardImage(done func(img ClipboardImage, ok bool, err error)) {
	cx.readClipboard(driver.ClipboardReadImage, func(ev driver.Event) {
		img, _ := ev.Data.(*driver.ClipboardImage)
		switch {
		case img == nil:
			done(ClipboardImage{}, false, nil)
		case img.Err != "":
			done(ClipboardImage{}, false, fmt.Errorf("nsgo: the clipboard's image cannot be read: %s", img.Err))
		case img.TooLarge > 0:
			done(ClipboardImage{}, false, &Error{Code: CodeQuotaExceeded,
				Message: fmt.Sprintf("nsgo: the clipboard's image is %d bytes as a PNG, over %d", img.TooLarge, MaxClipboardImage)})
		case len(img.PNG) == 0:
			done(ClipboardImage{}, false, nil)
		default:
			done(*img, true, nil)
		}
	})
}

// HasClipboardImage calls done with whether the clipboard holds an
// image, without reading it.
func (cx *Scope) HasClipboardImage(done func(bool)) {
	cx.readClipboard(driver.ClipboardReadHasImage, func(ev driver.Event) { done(ev.Finished) })
}

// ReadClipboardFiles calls done with the file: URLs of the local files
// on the clipboard, such as files copied in a file manager, none when it
// holds none.
func (cx *Scope) ReadClipboardFiles(done func(uris []string)) {
	cx.readClipboard(driver.ClipboardReadFiles, func(ev driver.Event) { done(ev.URIs) })
}

// Announce has the screen reader say text.
func (cx *Scope) Announce(text string) {
	if t := cx.tree(); t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemAnnounce, Value: text})
		t.changed = true
	}
}

// SelectInputSource is the core's part of base.Command's InputSource: it
// selects the enabled keyboard input source of id, remembering the one it
// replaces. An id is the platform's: a Text Input Sources ID on macOS
// ("com.apple.keylayout.ABC"), a keyboard layout ID on Windows
// ("00000409"), and a Fcitx 5 input method ("keyboard-us") or an IBus
// engine ("xkb:us::eng") on Linux. Phones and the web ignore it.
func (cx *Scope) SelectInputSource(id string) {
	if t := cx.tree(); t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemInputSource, Value: id})
		t.changed = true
	}
}

// RestoreInputSource is the core's part of base.Command's InputSource: it
// goes back to the source SelectInputSource replaced, unless the user chose
// another since.
func (cx *Scope) RestoreInputSource() { cx.SelectInputSource("") }

// MaxURLLength is the longest URL OpenURL opens, in bytes: a mailto:
// link with a body fits.
const MaxURLLength = 16 << 10

// The schemes OpenURL never opens: they run code or reach the app's own
// files and data rather than another app.
var refusedSchemes = []string{"javascript", "vbscript", "data", "blob", "file", "about", "asset"}

// CheckURL reports why OpenURL would refuse u: it must be absolute, at
// most MaxURLLength bytes, without spaces or control characters, a web
// address with a host, and of a scheme that opens another app, not
// javascript:, data:, blob:, file:, about: or asset:.
func CheckURL(u string) error {
	if len(u) > MaxURLLength {
		return fmt.Errorf("core: a URL has at most %d bytes, this one %d", MaxURLLength, len(u))
	}
	if strings.IndexFunc(u, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return errors.New("core: a URL has no spaces or control characters")
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("core: not a URL: %w", err)
	}
	if parsed.Scheme == "" {
		return fmt.Errorf("core: %q has no scheme", u)
	}
	scheme := strings.ToLower(parsed.Scheme)
	for _, s := range refusedSchemes {
		if scheme == s {
			return fmt.Errorf("core: %s: URLs are not opened", scheme)
		}
	}
	if (scheme == "http" || scheme == "https") && parsed.Host == "" {
		return fmt.Errorf("core: a web address has a host (%s)", u)
	}
	return nil
}

// RegisterFont registers the font file data (TrueType, OpenType or a
// collection) with the platform, so text finds its families as a Style's
// FontFamily names them, as nsgo.Fonts' are, and prepares the text shown
// again: call it before the text that uses it shows. Without a platform
// that registers fonts as the app runs (a headless test's) it does
// nothing.
func (cx *Scope) RegisterFont(data []byte) error {
	t := cx.tree()
	if t == nil {
		return nil
	}
	fr, ok := t.text.(driver.FontRegistrar)
	if !ok {
		return nil
	}
	if err := fr.RegisterFont(data); err != nil {
		return err
	}
	t.FontsChanged()
	return nil
}

// FontsChanged prepares every text again, with fonts the platform made
// available since it was prepared.
func (t *Tree) FontsChanged() {
	t.fonts++
	for _, n := range t.nodes {
		if n == nil || n.deleted {
			continue
		}
		switch {
		case n.text != nil:
			n.text.stale = true
			t.markLayout(n)
		case n.input != nil:
			n.input.lineH = 0
			t.markLayout(n)
		}
	}
	t.measures = newMeasureCache()
	t.markAll()
}

// OpenURL opens u outside the app: a web page in the browser, a mailto:
// or tel: link in its app, any scheme another app registered. An
// embedding host's handler (Host.HandleOpenURL) takes it first. done,
// when not nil, learns whether it opened: false for a URL CheckURL
// refuses, or one no app handles.
func (cx *Scope) OpenURL(u string, done func(opened bool)) {
	answer := func(ok bool) {
		if done != nil {
			done(ok)
		}
	}
	if CheckURL(u) != nil {
		answer(false)
		return
	}
	o := cx.current()
	platform := func() {
		t := cx.tree()
		if t == nil || o.disposed {
			answer(false)
			return
		}
		t.request(o, driver.MountItem{Kind: driver.ItemOpenURL, Value: u}, func(ev driver.Event) { answer(ev.Finished) })
	}
	asked := cx.rt.host.openURL(u, func(handled bool) {
		if handled {
			cx.rt.run(o, func() { answer(true) })
		} else {
			cx.rt.run(o, platform)
		}
	})
	if !asked {
		platform()
	}
}

// PickOptions say how PickImages picks and prepares images.
type PickOptions struct {
	// Multiple lets the user pick several images, at most Limit (0: up
	// to MaxPicked).
	Multiple bool
	Limit    int
	// MaxDimension scales images whose longer side has more pixels down
	// to it, encoding them again; 0 keeps their size.
	MaxDimension int
	// Quality is the quality JPEGs are encoded at, from 0 (smallest) to
	// 1; 0 means 0.9, and keeps JPEGs that need no other change as they
	// are.
	Quality float32
	// Format says which formats images come in: by default JPEG, PNG, GIF
	// and WebP as they are and anything else, such as HEIC, as JPEG.
	Format PickFormat
}

// request is o as the platform carries it out.
func (o PickOptions) request() driver.PickRequest {
	return driver.PickRequest{Limit: pickLimit(o.Multiple, o.Limit), MaxDimension: max(o.MaxDimension, 0),
		Quality: min(max(o.Quality, 0), 1), Format: o.Format}
}

// pickLimit is the most a pick answers with: 1 for a single pick, else
// limit, up to MaxPicked (0: MaxPicked).
func pickLimit(multiple bool, limit int) int {
	switch {
	case !multiple:
		return 1
	case limit > 0:
		return min(limit, driver.MaxPicked)
	}
	return driver.MaxPicked
}

// PickImages shows the system's photo picker and calls done with the
// images the user picked, in the order picked, prepared as o says, or
// none when they cancelled: PHPickerViewController on iOS, Android's
// photo picker, an open panel on the desktops and a file chooser on the
// web. The mobile pickers need no permission.
func (cx *Scope) PickImages(o PickOptions, done func(files []PickedFile)) {
	t := cx.tree()
	if t == nil {
		return
	}
	r := o.request()
	t.request(cx, driver.MountItem{Kind: driver.ItemPickImages, Pick: &r}, func(ev driver.Event) {
		files, _ := ev.Data.([]driver.PickedFile)
		if done != nil {
			done(driver.FinishPick(r, files, nil))
		}
	})
}

// PhotoOptions say how TakePhoto prepares the photo taken.
type PhotoOptions struct {
	// MaxDimension scales a photo whose longer side has more pixels down
	// to it; 0 keeps its size.
	MaxDimension int
	// Quality is the quality the JPEG is encoded at, from 0 (smallest) to
	// 1; 0 means 0.9.
	Quality float32
}

// ErrNoCamera is TakePhoto's failure where Env.CanTakePhoto is false.
var ErrNoCamera = errors.New("core: no camera to take a photo with")

// TakePhoto shows the system's camera for the user to take one photo
// and calls done with it, a JPEG turned upright and prepared as o says,
// or with nil when they cancelled; err says what failed, such as a
// refused permission or ErrNoCamera. iOS shows UIImagePickerController
// and needs NSCameraUsageDescription in the app's Info.plist; Android
// starts the camera app, and the web a file chooser that offers it.
func (cx *Scope) TakePhoto(o PhotoOptions, done func(photo *PickedFile, err error)) {
	t := cx.tree()
	if t == nil {
		return
	}
	if !Untracked(cx, cx.Env().CanTakePhoto) {
		if done != nil {
			done(nil, ErrNoCamera)
		}
		return
	}
	r := driver.PickRequest{Limit: 1, MaxDimension: max(o.MaxDimension, 0), Quality: min(max(o.Quality, 0), 1)}
	t.request(cx, driver.MountItem{Kind: driver.ItemTakePhoto, Pick: &r}, func(ev driver.Event) {
		if done == nil {
			return
		}
		if ev.Text != "" {
			done(nil, errors.New(ev.Text))
			return
		}
		files, _ := ev.Data.([]driver.PickedFile)
		if files = driver.FinishPick(r, files, nil); len(files) == 0 {
			done(nil, nil)
			return
		}
		done(&files[0], nil)
	})
}

// PickFilesOptions say which files PickFiles offers.
type PickFilesOptions struct {
	// Multiple lets the user pick several files, at most Limit (0: up to
	// MaxPicked).
	Multiple bool
	Limit    int
	// Types are the files offered: media types (application/pdf),
	// families (image/*) and extensions (.zip); none offers every file.
	Types []string
}

// request is o as the platform carries it out.
func (o PickFilesOptions) request() driver.FilesRequest {
	r := driver.FilesRequest{Limit: pickLimit(o.Multiple, o.Limit)}
	for _, t := range o.Types {
		t = strings.ToLower(strings.TrimSpace(t))
		switch {
		case t == "*" || t == "*/*" || t == ".*":
			r.Types = nil
			return r
		case t == "":
			continue
		case !strings.HasPrefix(t, ".") && !strings.Contains(t, "/"):
			t = "." + t
		}
		r.Types = append(r.Types, t)
	}
	return r
}

// PickFiles shows the system's file picker and calls done with the files
// the user picked, as they are, or none when they cancelled; err says
// what failed. Each is a file on disk the app opens and streams: the
// user's own on desktops, a copy in the app's storage on iOS and Android
// (File.Temporary) that is the app's to delete, and none on the web,
// where File.Open reads it. The pickers need no permission.
func (cx *Scope) PickFiles(o PickFilesOptions, done func(files []File, err error)) {
	t := cx.tree()
	if t == nil {
		return
	}
	r := o.request()
	t.request(cx, driver.MountItem{Kind: driver.ItemPickFiles, Files: &r}, func(ev driver.Event) {
		if done == nil {
			return
		}
		if ev.Text != "" {
			done(nil, errors.New(ev.Text))
			return
		}
		files, _ := ev.Data.([]driver.File)
		done(driver.FinishFiles(r, files), nil)
	})
}

// PickScreenColor lets the user pick a color anywhere on the screen and
// calls done with it, or with false when they cancel or the platform has
// no way to (Env.CanPickScreenColor). base.ColorEyeDropper is its button.
func (cx *Scope) PickScreenColor(done func(c Color, ok bool)) {
	t := cx.tree()
	if t == nil || !Untracked(cx, cx.Env().CanPickScreenColor) {
		done(0, false)
		return
	}
	t.request(cx, driver.MountItem{Kind: driver.ItemPickScreenColor}, func(ev driver.Event) {
		c, ok := ev.Data.(Color)
		done(c, ok && ev.Finished)
	})
}

// PickDirectoriesOptions say how PickDirectories picks folders.
type PickDirectoriesOptions struct {
	// Multiple lets the user pick several folders, at most Limit (0: up
	// to MaxPicked).
	Multiple bool
	Limit    int
}

// PickDirectories shows the system's folder picker and calls done with
// the folders the user picked, or none when they cancelled; err says
// what failed. Each is the user's own folder, which the app reads as an
// fs.FS: NSOpenPanel on macOS, IFileOpenDialog on Windows, GtkFileDialog,
// through the desktop portal where there is one, on Linux and
// UIDocumentPickerViewController on iOS give its path; Android's document
// tree picker and the web's folder picker give none, so the Directory
// reads through the platform and the app reads it off the engine's
// goroutine. Android and the web pick one folder. The pickers need no
// permission.
func (cx *Scope) PickDirectories(o PickDirectoriesOptions, done func(dirs []Directory, err error)) {
	t := cx.tree()
	if t == nil {
		return
	}
	r := driver.DirectoriesRequest{Limit: pickLimit(o.Multiple, o.Limit)}
	t.request(cx, driver.MountItem{Kind: driver.ItemPickDirectories, Directories: &r}, func(ev driver.Event) {
		if done == nil {
			return
		}
		if ev.Text != "" {
			done(nil, errors.New(ev.Text))
			return
		}
		dirs, _ := ev.Data.([]driver.Directory)
		done(driver.FinishDirectories(r, dirs), nil)
	})
}

// SaveOptions say what SaveFile saves: the file at Path, or else Data.
type SaveOptions struct {
	// Name is the file name the dialog suggests; Path's when empty.
	Name string
	// MIME is its media type; Name's extension says when empty.
	MIME string
	Path string
	Data []byte
}

// SaveFile shows the system's save dialog and copies the file o names to
// where the user chose, streaming it, then calls done with whether it
// was saved: false with no error when they cancelled. A save that fails
// leaves nothing where the user chose, or the file that was there. The
// web has no files on disk, so it saves Data: a save picker where the
// browser has one, else a download, which counts as saved.
func (cx *Scope) SaveFile(o SaveOptions, done func(saved bool, err error)) {
	answer := func(saved bool, err error) {
		if done != nil {
			done(saved, err)
		}
	}
	t := cx.tree()
	if t == nil {
		return
	}
	r := driver.SaveRequest{Name: o.Name, MIME: o.MIME, Path: o.Path, Data: o.Data}
	if r.Path != "" {
		if _, err := os.Stat(r.Path); err != nil {
			answer(false, err)
			return
		}
	}
	if i := strings.LastIndexAny(r.Name, `/\`); i >= 0 {
		r.Name = r.Name[i+1:]
	}
	if r.Name == "" && r.Path != "" {
		r.Name = filepath.Base(r.Path)
	}
	if r.Name == "" {
		r.Name = "file"
	}
	if r.MIME == "" {
		r.MIME = driver.MIMEForFile(r.Name)
	}
	t.request(cx, driver.MountItem{Kind: driver.ItemSaveFile, Save: &r}, func(ev driver.Event) {
		if ev.Text != "" {
			answer(false, errors.New(ev.Text))
			return
		}
		answer(ev.Finished, nil)
	})
}

// resolveShared returns f as the platform hands it on, with an absolute
// path, a name and a type, or why it cannot be: a file that is not
// there, or not a regular file.
func resolveShared(f SharedFile) (driver.SharedFile, error) {
	if f.Path == "" {
		return driver.SharedFile{}, errors.New("core: a shared file has a path")
	}
	path, err := filepath.Abs(f.Path)
	if err != nil {
		return driver.SharedFile{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return driver.SharedFile{}, err
	}
	if !info.Mode().IsRegular() {
		return driver.SharedFile{}, fmt.Errorf("core: %s is not a file", path)
	}
	r := driver.SharedFile{Path: path, Name: f.Name, MIME: f.MIME}
	if i := strings.LastIndexAny(r.Name, `/\`); i >= 0 {
		r.Name = r.Name[i+1:]
	}
	if r.Name == "" || r.Name == "." || r.Name == ".." {
		r.Name = filepath.Base(path)
	}
	if r.MIME == "" {
		r.MIME = driver.MIMEForFile(r.Name)
	}
	return r, nil
}

// OpenFile opens the file f names in the app the system associates with
// its type, the user choosing where several are: NSWorkspace on macOS,
// the shell on Windows, GtkFileLauncher on Linux, a document
// interaction's options on iOS and Intent.ACTION_VIEW on Android. done,
// when not nil, learns whether an app opened it: false with no error
// when none opens its type, and the error when the file is missing or
// the platform failed. On Android the file is in the app's own storage,
// such as DataDir and CacheDir, and the app opening it reads it through
// a content URI the runtime grants it, until its activity finishes; the
// file stays the app's to keep there that long and to delete. The web
// opens no files.
func (cx *Scope) OpenFile(f SharedFile, done func(opened bool, err error)) {
	answer := func(opened bool, err error) {
		if done != nil {
			done(opened, err)
		}
	}
	t := cx.tree()
	if t == nil {
		return
	}
	r, err := resolveShared(f)
	if err != nil {
		answer(false, err)
		return
	}
	t.request(cx, driver.MountItem{Kind: driver.ItemOpenFile, Open: &r}, func(ev driver.Event) {
		if ev.Text != "" {
			answer(false, errors.New(ev.Text))
			return
		}
		answer(ev.Finished, nil)
	})
}

// ShareOptions say what Share shares: files, text, or both.
type ShareOptions struct {
	Files []SharedFile
	// Text goes with the files, or alone: a message or a link.
	Text string
	// Anchor is the element the sheet points at where it shows as a
	// popover, on macOS and iPads; nil points at the root's center.
	Anchor *Ref
}

// Share shows the system's share sheet for what o names, for the user to
// send it to another app: UIActivityViewController on iOS,
// NSSharingServicePicker on macOS and an Intent.ACTION_SEND chooser on
// Android. Files go by path or content URI, never through the
// platform's bridge, so large ones cost nothing more. done, when not
// nil, learns whether the sheet showed, with the error when it did not:
// a missing file, or a platform without a share sheet (Windows, Linux
// and the web). That it showed says nothing of what the app the user
// chose did with what it received. On Android the files are in the
// app's own storage, such as DataDir and CacheDir, and the app chosen
// reads them through content URIs the runtime grants it, until its
// activity finishes; the files stay the app's to keep there that long
// and to delete.
func (cx *Scope) Share(o ShareOptions, done func(shown bool, err error)) {
	answer := func(shown bool, err error) {
		if done != nil {
			done(shown, err)
		}
	}
	t := cx.tree()
	if t == nil {
		return
	}
	r := driver.ShareRequest{Text: o.Text}
	for _, f := range o.Files {
		sf, err := resolveShared(f)
		if err != nil {
			answer(false, err)
			return
		}
		r.Files = append(r.Files, sf)
	}
	if len(r.Files) == 0 && r.Text == "" {
		answer(false, errors.New("core: a share has files or text"))
		return
	}
	if o.Anchor != nil {
		r.From, _ = o.Anchor.Bounds()
	}
	t.request(cx, driver.MountItem{Kind: driver.ItemShare, Share: &r}, func(ev driver.Event) {
		if ev.Text == driver.NoShareSheet {
			answer(false, Unavailable(ev.Text))
			return
		}
		if ev.Text != "" {
			answer(false, errors.New(ev.Text))
			return
		}
		answer(ev.Finished, nil)
	})
}

// errNoInstaller is what InstallPackage answers off Android.
var errNoInstaller = Unavailable("core: only Android installs packages")

// InstallPackage is the core's part of mobile.InstallPackage, which apps
// use: it hands the APK at path to Android's package installer.
func (cx *Scope) InstallPackage(path string, done func(started bool, err error)) {
	answer := func(started bool, err error) {
		if done != nil {
			done(started, err)
		}
	}
	t := cx.tree()
	if t == nil {
		return
	}
	if cx.Env().os.GetUntracked() != OSAndroid {
		answer(false, errNoInstaller)
		return
	}
	r, err := resolveShared(SharedFile{Path: path, MIME: "application/vnd.android.package-archive"})
	if err != nil {
		answer(false, err)
		return
	}
	t.request(cx, driver.MountItem{Kind: driver.ItemInstallPackage, Open: &r}, func(ev driver.Event) {
		if ev.Text != "" {
			answer(false, errors.New(ev.Text))
			return
		}
		answer(ev.Finished, nil)
	})
}

// CanInstallPackages is the core's part of mobile.CanInstallPackages,
// which apps use: it asks Android whether the user lets the app install
// packages.
func (cx *Scope) CanInstallPackages(done func(allowed bool)) {
	cx.androidAsk(driver.ItemCanInstallPackages, done)
}

// OpenInstallSettings is the core's part of mobile.OpenInstallSettings,
// which apps use: it opens Android's setting that lets the app install
// packages.
func (cx *Scope) OpenInstallSettings(done func(opened bool)) {
	cx.androidAsk(driver.ItemInstallSettings, done)
}

// androidAsk sends Android a request of kind that answers yes or no,
// and answers no elsewhere.
func (cx *Scope) androidAsk(kind driver.ItemKind, done func(bool)) {
	answer := func(ok bool) {
		if done != nil {
			done(ok)
		}
	}
	t := cx.tree()
	if t == nil {
		return
	}
	if cx.Env().os.GetUntracked() != OSAndroid {
		answer(false)
		return
	}
	t.request(cx, driver.MountItem{Kind: kind}, func(ev driver.Event) { answer(ev.Finished) })
}

// SetAppearance shows the app light or dark whatever the system's
// appearance, or follows it again with AppearanceSystem: Env.Dark
// reports the one shown, and the platform's own chrome, such as title
// bars, alerts, menus, system headers and the views it hosts, follows.
func (cx *Scope) SetAppearance(a Appearance) {
	if t := cx.rt.tree; t != nil {
		t.SetAppearance(a)
	}
}

// SetAppearance forces the appearance the app shows, as
// Scope.SetAppearance does: a host sets the one the app starts in.
func (t *Tree) SetAppearance(a Appearance) {
	e := t.root.Env()
	if e.appearance.GetUntracked() == a {
		return
	}
	t.root.Batch(func() {
		e.appearance.Set(a)
		e.dark.Set(e.shownDark())
		// The windows share the appearance the app forces.
		for _, o := range t.rt.surfaces {
			if o.env != nil {
				o.env.dark.Set(o.env.shownDark())
			}
		}
	})
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemAppearance, Appearance: a})
	t.changed = true
}

// Quit ends the app: desktops close the window, the app's last, and
// return from nsgo.Run; Android finishes the app's activity. iOS apps do
// not quit themselves, and the web leaves the page open.
func (cx *Scope) Quit() {
	if t := cx.rt.tree; t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemQuit})
		t.changed = true
	}
}
