package core

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// command returns the last item of kind in b.
func command(b *driver.MountBatch, kind driver.ItemKind) *driver.MountItem {
	var found *driver.MountItem
	if b == nil {
		return nil
	}
	for i := range b.Items {
		if b.Items[i].Kind == kind {
			found = &b.Items[i]
		}
	}
	return found
}

func TestClipboardReads(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	var text string
	var has, hasImage bool
	var img ClipboardImage
	var ok bool
	var files []string
	cx.ReadClipboard(func(s string) { text = s })
	cx.HasClipboardText(func(b bool) { has = b })
	cx.ReadClipboardImage(func(i ClipboardImage, o bool, _ error) { img, ok = i, o })
	cx.HasClipboardImage(func(b bool) { hasImage = b })
	cx.ReadClipboardFiles(func(u []string) { files = u })
	cx.SetClipboardImage([]byte{1, 2, 3})
	b := h.commit()
	var reads []driver.MountItem
	for _, it := range b.Items {
		if it.Kind == driver.ItemReadClipboard {
			reads = append(reads, it)
		}
	}
	want := []driver.ClipboardRead{driver.ClipboardReadText, driver.ClipboardReadHasText, driver.ClipboardReadImage,
		driver.ClipboardReadHasImage, driver.ClipboardReadFiles}
	if len(reads) != len(want) {
		t.Fatalf("reads = %+v", reads)
	}
	for i, r := range reads {
		if r.Clipboard != want[i] {
			t.Fatalf("read %d = %v, want %v", i, r.Clipboard, want[i])
		}
	}
	if it := command(b, driver.ItemSetClipboardImage); it == nil || !slices.Equal(it.Data.([]byte), []byte{1, 2, 3}) {
		t.Fatalf("set image = %+v", it)
	}
	answers := []driver.Event{
		{Text: "hi"}, {Finished: true}, {Data: &driver.ClipboardImage{PNG: []byte{9}, Size: Size{Width: 2, Height: 3}}},
		{Finished: true}, {URIs: []string{"file:///a.txt"}},
	}
	for i, ev := range answers {
		ev.Kind, ev.Count = driver.EventClipboard, reads[i].Count
		h.tree.Dispatch(ev)
		h.tree.Dispatch(ev) // answered once
	}
	if text != "hi" || !has || !ok || img.Size.Width != 2 || !hasImage || !slices.Equal(files, []string{"file:///a.txt"}) {
		t.Fatalf("text %q has %v image %+v %v hasImage %v files %v", text, has, img, ok, hasImage, files)
	}
	var err error
	read := func(data *driver.ClipboardImage) {
		cx.ReadClipboardImage(func(_ ClipboardImage, o bool, e error) { ok, err = o, e })
		h.tree.Dispatch(driver.Event{Kind: driver.EventClipboard, Count: command(h.commit(), driver.ItemReadClipboard).Count,
			Data: data})
	}
	read(nil)
	if ok || err != nil {
		t.Fatalf("a clipboard without an image reads as one: %v", err)
	}
	read(&driver.ClipboardImage{TooLarge: MaxClipboardImage + 1})
	if ok || ErrorCode(err) != CodeQuotaExceeded {
		t.Fatalf("too large an image = %v %v", ok, err)
	}
	read(&driver.ClipboardImage{Err: "no decoder"})
	if ok || err == nil || !strings.Contains(err.Error(), "no decoder") {
		t.Fatalf("an unreadable image = %v %v", ok, err)
	}
}

func TestCheckURL(t *testing.T) {
	for _, u := range []string{"https://example.com/a?b=c#d", "http://localhost:8080", "mailto:ada@example.com?subject=Hi%20there",
		"tel:+1-555-0100", "sms:+15550100", "maps://?q=Paris", "lb://router/stock/700.HK"} {
		if err := CheckURL(u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	long := "https://example.com/" + string(make([]byte, MaxURLLength))
	for _, u := range []string{"", "example.com", "/relative/path", "javascript:alert(1)", "JavaScript:alert(1)",
		"data:text/html,<b>x</b>", "file:///etc/passwd", "blob:https://example.com/1", "about:blank", "asset:/logo.png",
		"https://exa mple.com", "https://example.com/\n", "https:///path", long} {
		if CheckURL(u) == nil {
			t.Errorf("%.40q is opened", u)
		}
	}
}

func TestOpenURL(t *testing.T) {
	var cx *Scope
	var hosted []string
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	var got []bool
	cx.OpenURL("https://example.com", func(ok bool) { got = append(got, ok) })
	it := command(h.commit(), driver.ItemOpenURL)
	if it == nil || it.Value != "https://example.com" {
		t.Fatalf("item = %+v", it)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: it.Count, Finished: true})
	cx.OpenURL("javascript:alert(1)", func(ok bool) { got = append(got, ok) })
	if command(h.commit(), driver.ItemOpenURL) != nil {
		t.Fatal("a refused URL was sent")
	}
	cx.OpenURL("tel:1", nil)
	h.tree.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: command(h.commit(), driver.ItemOpenURL).Count})
	if !slices.Equal(got, []bool{true, false}) {
		t.Fatalf("answers = %v", got)
	}

	// An embedding host's handler takes URLs first.
	host := NewHost()
	host.HandleOpenURL(func(u string, answer func(bool)) {
		hosted = append(hosted, u)
		go answer(u == "lb://a")
	})
	var posts []func()
	var mu sync.Mutex
	root := NewRoot(RootOptions{Host: host, Post: func(fn func()) { mu.Lock(); posts = append(posts, fn); mu.Unlock() }})
	tree := NewTree(root, ApproxTextEngine{})
	tree.Mount(func(c *Scope) Node { cx = c; return View{} })
	tree.Commit()
	got = nil
	cx.OpenURL("lb://a", func(ok bool) { got = append(got, ok) })
	cx.OpenURL("https://b.example", func(ok bool) { got = append(got, ok) })
	for deadline := time.Now().Add(time.Second); ; {
		mu.Lock()
		n := len(posts)
		mu.Unlock()
		if n == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	for _, fn := range posts {
		fn()
	}
	b := tree.Commit()
	if !slices.Equal(hosted, []string{"lb://a", "https://b.example"}) || !slices.Equal(got, []bool{true}) ||
		command(b, driver.ItemOpenURL).Value != "https://b.example" {
		t.Fatalf("hosted %v answers %v", hosted, got)
	}
}

func TestPickImages(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	var got []PickedFile
	cx.PickImages(PickOptions{Multiple: true, Limit: 2, MaxDimension: 1000, Quality: 3}, func(f []PickedFile) { got = f })
	it := command(h.commit(), driver.ItemPickImages)
	if it == nil || *it.Pick != (driver.PickRequest{Limit: 2, MaxDimension: 1000, Quality: 1}) {
		t.Fatalf("pick = %+v", it)
	}
	png := []byte("\x89PNG\r\n\x1a\n....")
	h.tree.Dispatch(driver.Event{Kind: driver.EventPick, Count: it.Count, Data: []driver.PickedFile{
		{Name: "IMG_1.HEIC", MIME: "image/jpeg"}, {Name: "/tmp/shot", Data: png}, {Name: "third.png"}}})
	if len(got) != 2 || got[0].Name != "IMG_1.jpg" || got[1].Name != "shot.png" || got[1].MIME != "image/png" {
		t.Fatalf("files = %+v", got)
	}
	cx.PickImages(PickOptions{Limit: 5}, nil)
	if r := command(h.commit(), driver.ItemPickImages).Pick; r.Limit != 1 {
		t.Fatalf("a single pick asks for %d", r.Limit)
	}
	cx.PickImages(PickOptions{Multiple: true}, nil)
	if r := command(h.commit(), driver.ItemPickImages).Pick; r.Limit != MaxPicked {
		t.Fatalf("a pick without a limit asks for %d", r.Limit)
	}
}

func TestTakePhoto(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	var got *PickedFile
	var gotErr error
	answered := 0
	take := func(p *PickedFile, err error) { got, gotErr = p, err; answered++ }
	cx.TakePhoto(PhotoOptions{}, take)
	if answered != 1 || !errors.Is(gotErr, ErrNoCamera) || command(h.commit(), driver.ItemTakePhoto) != nil {
		t.Fatalf("without a camera: %+v, %v", got, gotErr)
	}
	h.tree.SetEnvironment(Environment{Camera: true})
	h.commit()
	cx.TakePhoto(PhotoOptions{MaxDimension: 2048, Quality: 3}, take)
	it := command(h.commit(), driver.ItemTakePhoto)
	if it == nil || *it.Pick != (driver.PickRequest{Limit: 1, MaxDimension: 2048, Quality: 1}) {
		t.Fatalf("photo = %+v", it)
	}
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPick, Count: it.Count, Data: []driver.PickedFile{{Name: "IMG_7", Data: jpeg}}})
	if answered != 2 || gotErr != nil || got == nil || got.Name != "IMG_7.jpg" || got.MIME != "image/jpeg" {
		t.Fatalf("photo taken = %+v, %v", got, gotErr)
	}
	cx.TakePhoto(PhotoOptions{}, take)
	it = command(h.commit(), driver.ItemTakePhoto)
	h.tree.Dispatch(driver.Event{Kind: driver.EventPick, Count: it.Count})
	if answered != 3 || got != nil || gotErr != nil {
		t.Fatalf("a cancel answers %+v, %v", got, gotErr)
	}
	cx.TakePhoto(PhotoOptions{}, take)
	it = command(h.commit(), driver.ItemTakePhoto)
	h.tree.Dispatch(driver.Event{Kind: driver.EventPick, Count: it.Count, Text: "camera access denied"})
	if answered != 4 || got != nil || gotErr == nil || gotErr.Error() != "camera access denied" {
		t.Fatalf("a failure answers %+v, %v", got, gotErr)
	}
}

func TestPickFiles(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	var got []File
	var gotErr error
	answered := 0
	pick := func(files []File, err error) { got, gotErr = files, err; answered++ }
	cx.PickFiles(PickFilesOptions{Multiple: true, Limit: 2, Types: []string{" Application/PDF", "zip", "", "image/*"}}, pick)
	it := command(h.commit(), driver.ItemPickFiles)
	if it == nil || it.Files.Limit != 2 || !slices.Equal(it.Files.Types, []string{"application/pdf", ".zip", "image/*"}) {
		t.Fatalf("pick = %+v", it)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.7"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Data: []driver.File{
		{Path: path}, {Name: "notes", Path: filepath.Join(dir, "gone")}, {Name: "third.zip"}}})
	if answered != 1 || gotErr != nil || len(got) != 2 {
		t.Fatalf("files = %+v, %v", got, gotErr)
	}
	if f := got[0]; f.Name != "report.pdf" || f.MIME != "application/pdf" || f.Size != 8 || f.Modified.IsZero() {
		t.Fatalf("file = %+v", f)
	}
	if f := got[1]; f.MIME != "application/octet-stream" || f.Size != 0 {
		t.Fatalf("a file without a type or a size = %+v", f)
	}
	cx.PickFiles(PickFilesOptions{Types: []string{"text/plain", "*/*"}}, pick)
	it = command(h.commit(), driver.ItemPickFiles)
	if it.Files.Limit != 1 || it.Files.Types != nil {
		t.Fatalf("a single pick of every file = %+v", it.Files)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Text: "provider failed"})
	if gotErr == nil || gotErr.Error() != "provider failed" || got != nil {
		t.Fatalf("a failed pick answers %+v, %v", got, gotErr)
	}
}

func TestPickDirectories(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	var got []Directory
	var gotErr error
	answered := 0
	pick := func(dirs []Directory, err error) { got, gotErr = dirs, err; answered++ }
	cx.PickDirectories(PickDirectoriesOptions{Multiple: true, Limit: 2}, pick)
	it := command(h.commit(), driver.ItemPickDirectories)
	if it == nil || it.Directories.Limit != 2 {
		t.Fatalf("pick = %+v", it)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Data: []driver.Directory{
		{Path: filepath.Join("home", "photos")}, {Name: "Music", Path: "/m"}, {Path: "/third"}}})
	if answered != 1 || gotErr != nil || len(got) != 2 || got[0].Name != "photos" || got[1].Name != "Music" {
		t.Fatalf("dirs = %+v, %v", got, gotErr)
	}
	cx.PickDirectories(PickDirectoriesOptions{Limit: 5}, pick)
	it = command(h.commit(), driver.ItemPickDirectories)
	if it.Directories.Limit != 1 {
		t.Fatalf("a single pick = %+v", it.Directories)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count})
	if answered != 2 || gotErr != nil || got != nil {
		t.Fatalf("a cancelled pick answers %+v, %v", got, gotErr)
	}
	cx.PickDirectories(PickDirectoriesOptions{}, pick)
	it = command(h.commit(), driver.ItemPickDirectories)
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Text: driver.NoDirectoryPicker})
	if gotErr == nil || gotErr.Error() != driver.NoDirectoryPicker || got != nil {
		t.Fatalf("a failed pick answers %+v, %v", got, gotErr)
	}
}

func TestSaveFile(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	dir := t.TempDir()
	src := filepath.Join(dir, "download.bin")
	if err := os.WriteFile(src, []byte("bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	saved, answered, savedAt := false, 0, ""
	var saveErr error
	save := func(ok bool, path string, err error) { saved, savedAt, saveErr = ok, path, err; answered++ }
	cx.SaveFile(SaveOptions{Path: src}, save)
	it := command(h.commit(), driver.ItemSaveFile)
	if it == nil || it.Save.Name != "download.bin" || it.Save.MIME != "application/octet-stream" || it.Save.Path != src {
		t.Fatalf("save = %+v", it)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: true, Data: "/chosen/download.bin"})
	if answered != 1 || !saved || savedAt != "/chosen/download.bin" || saveErr != nil {
		t.Fatalf("saved = %v at %q, %v", saved, savedAt, saveErr)
	}
	cx.SaveFile(SaveOptions{Name: "../notes/a.txt", Data: []byte("hi")}, save)
	it = command(h.commit(), driver.ItemSaveFile)
	if it.Save.Name != "a.txt" || it.Save.MIME != "text/plain" {
		t.Fatalf("save = %+v", it.Save)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count})
	if answered != 2 || saved || saveErr != nil {
		t.Fatalf("a cancelled save answers %v, %v", saved, saveErr)
	}
	cx.SaveFile(SaveOptions{Path: filepath.Join(dir, "missing")}, save)
	if answered != 3 || saveErr == nil || command(h.commit(), driver.ItemSaveFile) != nil {
		t.Fatalf("a save of a missing file asked the platform, or answered %v", saveErr)
	}
}

func TestOpenFile(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	dir := t.TempDir()
	src := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(src, []byte("%PDF"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, answered := false, 0
	var openErr error
	open := func(ok bool, err error) { opened, openErr = ok, err; answered++ }
	cx.OpenFile(SharedFile{Path: src}, open)
	it := command(h.commit(), driver.ItemOpenFile)
	if it == nil || *it.Open != (driver.SharedFile{Path: src, Name: "report.pdf", MIME: "application/pdf"}) {
		t.Fatalf("open = %+v", it)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: true})
	if answered != 1 || !opened || openErr != nil {
		t.Fatalf("opened = %v, %v", opened, openErr)
	}
	cx.OpenFile(SharedFile{Path: src, Name: "../Q3.bin", MIME: "application/x-q"}, open)
	it = command(h.commit(), driver.ItemOpenFile)
	if it.Open.Name != "Q3.bin" || it.Open.MIME != "application/x-q" {
		t.Fatalf("open = %+v", it.Open)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count})
	if answered != 2 || opened || openErr != nil {
		t.Fatalf("a file no app opens answers %v, %v", opened, openErr)
	}
	cx.OpenFile(SharedFile{Path: src}, open)
	it = command(h.commit(), driver.ItemOpenFile)
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Text: driver.NoFileOpener})
	if answered != 3 || openErr == nil || openErr.Error() != driver.NoFileOpener {
		t.Fatalf("a failed open answers %v", openErr)
	}
	for _, path := range []string{"", filepath.Join(dir, "missing"), dir} {
		cx.OpenFile(SharedFile{Path: path}, open)
		if openErr == nil || command(h.commit(), driver.ItemOpenFile) != nil {
			t.Fatalf("opening %q asked the platform, or answered %v", path, openErr)
		}
	}
	// A desktop opens a folder in its file manager.
	h.tree.SetEnvironment(Environment{OS: OSMacOS})
	cx.OpenFile(SharedFile{Path: dir}, open)
	it = command(h.commit(), driver.ItemOpenFile)
	if it == nil || *it.Open != (driver.SharedFile{Path: dir, Name: filepath.Base(dir), MIME: "inode/directory"}) {
		t.Fatalf("a folder opens as %+v", it)
	}
}

func TestShare(t *testing.T) {
	var cx *Scope
	var anchor Ref
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node {
		cx = c
		return View{Children: []Node{View{Ref: &anchor, Style: Style{Width: Pt(40), Height: Pt(20)}}}}
	})
	h.commit()
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.png")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	shown, answered := false, 0
	var shareErr error
	share := func(ok bool, err error) { shown, shareErr = ok, err; answered++ }
	cx.Share(ShareOptions{Files: []SharedFile{{Path: a, Name: "Notes.txt"}, {Path: b}}, Text: "hi", Anchor: &anchor}, share)
	it := command(h.commit(), driver.ItemShare)
	want := []driver.SharedFile{{Path: a, Name: "Notes.txt", MIME: "text/plain"}, {Path: b, Name: "b.png", MIME: "image/png"}}
	if it == nil || !slices.Equal(it.Share.Files, want) || it.Share.Text != "hi" || it.Share.From.Size != (Size{Width: 40, Height: 20}) {
		t.Fatalf("share = %+v", it.Share)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: true})
	if answered != 1 || !shown || shareErr != nil {
		t.Fatalf("shown = %v, %v", shown, shareErr)
	}
	cx.Share(ShareOptions{Text: "https://example.com"}, share)
	it = command(h.commit(), driver.ItemShare)
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Text: driver.NoShareSheet})
	if answered != 2 || shown || shareErr == nil {
		t.Fatalf("a failed share answers %v, %v", shown, shareErr)
	}
	for _, o := range []ShareOptions{{}, {Files: []SharedFile{{Path: a}, {Path: filepath.Join(dir, "missing")}}}} {
		cx.Share(o, share)
		if shareErr == nil || command(h.commit(), driver.ItemShare) != nil {
			t.Fatalf("share %+v asked the platform, or answered %v", o, shareErr)
		}
	}
}

func TestInstallPackage(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	apk := filepath.Join(t.TempDir(), "update.apk")
	if err := os.WriteFile(apk, []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	started, allowed, opened := false, true, true
	overlayAllowed, overlaySettings := true, true
	var installErr error
	cx.InstallPackage(apk, func(ok bool, err error) { started, installErr = ok, err })
	cx.CanInstallPackages(func(ok bool) { allowed = ok })
	cx.OpenInstallSettings(func(ok bool) { opened = ok })
	cx.CanDrawOverlays(func(ok bool) { overlayAllowed = ok })
	cx.OpenOverlaySettings(func(ok bool) { overlaySettings = ok })
	if installErr == nil || allowed || opened || overlayAllowed || overlaySettings || len(h.commit().Items) != 0 {
		t.Fatalf("off Android: %v, %v, %v", installErr, allowed, opened)
	}

	h.tree.SetEnvironment(Environment{OS: OSAndroid})
	h.commit()
	cx.InstallPackage(apk, func(ok bool, err error) { started, installErr = ok, err })
	it := command(h.commit(), driver.ItemInstallPackage)
	if it == nil || it.Open.Path != apk || it.Open.MIME != "application/vnd.android.package-archive" {
		t.Fatalf("install = %+v", it)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: true})
	if !started || installErr != nil {
		t.Fatalf("started = %v, %v", started, installErr)
	}
	cx.CanInstallPackages(func(ok bool) { allowed = ok })
	it = command(h.commit(), driver.ItemCanInstallPackages)
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: true})
	if !allowed {
		t.Fatal("not allowed")
	}
	cx.CanDrawOverlays(func(ok bool) { overlayAllowed = ok })
	it = command(h.commit(), driver.ItemCanDrawOverlays)
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: true})
	if !overlayAllowed {
		t.Fatal("overlay not allowed")
	}
	cx.OpenOverlaySettings(func(ok bool) { overlaySettings = ok })
	it = command(h.commit(), driver.ItemOverlaySettings)
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: true})
	if !overlaySettings {
		t.Fatal("overlay settings did not open")
	}
	cx.InstallPackage(filepath.Join(filepath.Dir(apk), "missing.apk"), func(ok bool, err error) { started, installErr = ok, err })
	if installErr == nil || command(h.commit(), driver.ItemInstallPackage) != nil {
		t.Fatalf("installing a missing file answered %v", installErr)
	}
}

func TestSharedFileStage(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x1.bin")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p, err := (driver.SharedFile{Path: src, Name: "x1.bin"}).Stage(filepath.Join(dir, "share")); err != nil || p != src {
		t.Fatalf("a file under its own name staged as %q, %v", p, err)
	}
	stage := filepath.Join(dir, "share")
	old := filepath.Join(stage, "old")
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	then := time.Now().Add(-2 * driver.ShareStageAge)
	os.Chtimes(old, then, then)
	p, err := (driver.SharedFile{Path: src, Name: "Report.bin"}).Stage(stage)
	if err != nil || filepath.Base(p) != "Report.bin" {
		t.Fatalf("staged %q, %v", p, err)
	}
	if b, _ := os.ReadFile(p); string(b) != "x" {
		t.Fatalf("staged %q", b)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("an old stage stayed")
	}
}

func TestSaveRequestSaveTo(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (driver.SaveRequest{Name: "out.txt", Path: filepath.Join(dir, "missing")}).SaveTo(dst); err == nil {
		t.Fatal("a save from a missing file succeeded")
	}
	if err := (driver.SaveRequest{Name: "out.txt", Data: []byte("new")}).SaveTo(dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "new" {
		t.Fatalf("saved %q", b)
	}
	if err := (driver.SaveRequest{Name: "out.txt", Path: filepath.Join(dir, "nowhere", "x")}).SaveTo(filepath.Join(dir, "nowhere", "y")); err == nil {
		t.Fatal("a save into a missing directory succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("a failed save left %v", entries)
	}
	staged, err := (driver.SaveRequest{Name: "Report.txt", Path: dst}).Stage(filepath.Join(dir, "stage"))
	if err != nil || filepath.Base(staged) != "Report.txt" {
		t.Fatalf("staged %q, %v", staged, err)
	}
	if b, _ := os.ReadFile(staged); string(b) != "new" {
		t.Fatalf("staged %q", b)
	}
}

func TestFilesRequestFilters(t *testing.T) {
	r := driver.FilesRequest{Types: []string{".zip", "application/pdf", "image/*"}}
	if types, any := r.MIMETypes(); any || !slices.Equal(types, []string{"application/zip", "application/pdf", "image/*"}) {
		t.Fatalf("types = %v, %v", types, any)
	}
	if _, any := (driver.FilesRequest{Types: []string{".nsgo-unknown"}}).MIMETypes(); !any {
		t.Fatal("an extension without a type filtered the picker")
	}
	exts, any := r.Extensions()
	if any || !slices.Contains(exts, "zip") || !slices.Contains(exts, "pdf") || !slices.Contains(exts, "heic") || !slices.Contains(exts, "svg") {
		t.Fatalf("extensions = %v, %v", exts, any)
	}
	if _, any := (driver.FilesRequest{Types: []string{"application/x-nsgo-unknown"}}).Extensions(); !any {
		t.Fatal("a type without extensions filtered the dialog")
	}
	if _, any := (driver.FilesRequest{}).Extensions(); !any {
		t.Fatal("no types filtered the dialog")
	}
}

func TestPickRequestEncodings(t *testing.T) {
	px := func(w, h Points) Size { return Size{Width: w, Height: h} }
	var r driver.PickRequest
	for _, c := range []struct {
		mime string
		size Size
		want driver.PickEncoding
	}{{"image/jpeg", px(4000, 3000), driver.PickKeep}, {"image/heic", px(400, 300), driver.PickJPEG},
		{"image/gif", px(400, 300), driver.PickKeep}} {
		if got := r.Encoding(c.mime, c.size); got != c.want {
			t.Errorf("%s: %v, want %v", c.mime, got, c.want)
		}
	}
	r.Format = driver.PickCurrent
	if r.Encoding("image/heic", px(400, 300)) != driver.PickKeep {
		t.Error("PickCurrent converts HEIC")
	}
	r.MaxDimension = 1000
	if r.Encoding("image/heic", px(4000, 3000)) != driver.PickJPEG || r.Encoding("image/png", px(4000, 3000)) != driver.PickPNG ||
		r.Encoding("image/png", px(400, 300)) != driver.PickKeep {
		t.Error("scaling encodings")
	}
	r.Quality = 0.5
	if r.Encoding("image/jpeg", px(400, 300)) != driver.PickJPEG {
		t.Error("a quality keeps JPEGs")
	}
	if r.ScaledSize(px(4000, 3000)) != px(1000, 750) || r.ScaledSize(px(300, 4000)) != px(75, 1000) ||
		r.ScaledSize(px(400, 300)) != px(400, 300) {
		t.Error("scaled sizes")
	}
	for _, c := range [][3]string{{"IMG_0042.HEIC", "image/jpeg", "IMG_0042.jpg"}, {"a.jpeg", "image/jpeg", "a.jpeg"},
		{"", "image/png", "image.png"}, {"photo", "image/webp", "photo.webp"}, {"notes.txt", "", "notes.txt"},
		{`C:\pics\b.PNG`, "image/png", "b.PNG"}} {
		if got := driver.PickedName(c[0], c[1]); got != c[2] {
			t.Errorf("PickedName(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestPopupMenu(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.tree.SetEnvironment(Environment{OS: OSMacOS, Idiom: IdiomDesktop})
	h.commit()
	var log []string
	cx.PopupMenu([]MenuItem{{Label: "Copy", OnSelect: func() { log = append(log, "copy") }},
		{Label: "More", Items: []MenuItem{{Label: "Deep", OnSelect: func() { log = append(log, "deep") }}}}},
		Point{X: 10, Y: 20}, func() { log = append(log, "closed") })
	it := command(h.commit(), driver.ItemPopupMenu)
	if it == nil || it.Point != (Point{X: 10, Y: 20}) || len(it.Menu) != 2 || it.Menu[1].Items[0].ID == 0 {
		t.Fatalf("popup = %+v", it)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMenu, Count: it.Menu[1].Items[0].ID})
	h.tree.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: it.Count})
	h.tree.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: it.Count})

	// A platform may report the item chosen after the menu closed.
	cx.PopupMenu([]MenuItem{{Label: "Late", OnSelect: func() { log = append(log, "late") }}}, Point{}, nil)
	late := command(h.commit(), driver.ItemPopupMenu)
	h.tree.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: late.Count})
	h.tree.Dispatch(driver.Event{Kind: driver.EventMenu, Count: late.Menu[0].ID})
	if log[len(log)-1] != "late" {
		t.Fatalf("log = %q", log)
	}
	log = log[:len(log)-1]

	// A second menu closes the first.
	cx.PopupMenu([]MenuItem{{Label: "A"}}, Point{}, func() { log = append(log, "first") })
	cx.PopupMenu([]MenuItem{{Label: "B"}}, Point{}, nil)
	if !slices.Equal(log, []string{"deep", "closed", "first"}) {
		t.Fatalf("log = %q", log)
	}

	// Phones have no context menus: closed runs at once.
	h.tree.SetEnvironment(Environment{OS: OSIOS, Idiom: IdiomPhone})
	h.commit()
	cx.PopupMenu([]MenuItem{{Label: "A"}}, Point{}, func() { log = append(log, "phone") })
	if command(h.commit(), driver.ItemPopupMenu) != nil || log[len(log)-1] != "phone" {
		t.Fatalf("log = %q", log)
	}
}

func TestWindowFrameAndLevel(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	cx.SetWindowFrame(WindowFrame{Height: 300, Width: -5, Animated: true})
	cx.SetWindowLevel(WindowLevelFloating)
	cx.OrderFront()
	cx.SetMenuBar(MenuBarAutoHide)
	var got []WindowCommand
	for _, it := range h.commit().Items {
		if it.Kind == driver.ItemWindow {
			got = append(got, *it.Window)
		}
	}
	want := []WindowCommand{{Action: WindowSetFrame, Frame: WindowFrame{Height: 300, Animated: true}},
		{Action: WindowSetLevel, Level: WindowLevelFloating}, {Action: WindowOrderFront},
		{Action: WindowSetMenuBar, MenuBar: MenuBarAutoHide}}
	if !slices.Equal(got, want) {
		t.Fatalf("commands = %+v", got)
	}
}

func TestAppearanceForcesDark(t *testing.T) {
	var cx *Scope
	var seen []bool
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node {
		cx = c
		c.Effect(func() { seen = append(seen, c.Env().Dark()) })
		return View{}
	})
	h.tree.SetEnvironment(Environment{Dark: false, OSVersion: OSVersion{Major: 17, Minor: 4}})
	h.commit()
	cx.SetAppearance(AppearanceDark)
	if it := command(h.commit(), driver.ItemAppearance); it == nil || it.Appearance != AppearanceDark {
		t.Fatalf("appearance item = %+v", it)
	}
	// The system turning light or dark changes nothing while forced.
	h.tree.SetEnvironment(Environment{Dark: true})
	h.tree.SetEnvironment(Environment{Dark: false})
	cx.SetAppearance(AppearanceDark)
	if command(h.commit(), driver.ItemAppearance) != nil {
		t.Fatal("the same appearance was sent again")
	}
	cx.SetAppearance(AppearanceSystem)
	h.tree.SetEnvironment(Environment{Dark: true})
	h.commit()
	if !slices.Equal(seen, []bool{false, true, false, true}) || cx.Env().Appearance() != AppearanceSystem {
		t.Fatalf("dark = %v", seen)
	}
	h.tree.SetEnvironment(Environment{OSVersion: OSVersion{Major: 17, Minor: 4}})
	if v := cx.Env().OSVersion(); !v.AtLeast(17, 0) || v.AtLeast(17, 5) || v.AtLeast(18, 0) || v.String() != "17.4" {
		t.Fatalf("version = %v", v)
	}
}

func TestQuit(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	cx.Quit()
	b := h.commit()
	if command(b, driver.ItemQuit) == nil {
		t.Fatalf("items = %+v", b.Items)
	}
}

func TestRestart(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	defer restarting.Store(false)
	cx.Restart()
	if it := command(h.commit(), driver.ItemQuit); it == nil || !it.On || !Restarting() {
		t.Fatalf("restart's quit is %+v", it)
	}
}
