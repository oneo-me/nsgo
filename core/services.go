package core

import (
	"errors"
	"fmt"
	"net/url"
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
	t := cx.rt.tree
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
	if t := cx.rt.tree; t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemDismissKeyboard})
		t.changed = true
	}
}

// SetClipboard puts text on the system clipboard, replacing what it
// holds.
func (cx *Scope) SetClipboard(text string) {
	if t := cx.rt.tree; t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemSetClipboard, Value: text})
		t.changed = true
	}
}

// SetClipboardImage puts the image data encodes on the system clipboard,
// replacing what it holds: PNG, JPEG, GIF or WebP bytes, as
// ReadClipboardImage gives them or an ImageSource holds them.
func (cx *Scope) SetClipboardImage(data []byte) {
	if t := cx.rt.tree; t != nil && len(data) > 0 {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemSetClipboardImage, Data: data})
		t.changed = true
	}
}

func (cx *Scope) readClipboard(what driver.ClipboardRead, done func(driver.Event)) {
	if t := cx.rt.tree; t != nil {
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
// ok false when it holds none or one over MaxClipboardImage bytes. On
// iOS, reading it may show the system's paste notice.
func (cx *Scope) ReadClipboardImage(done func(img ClipboardImage, ok bool)) {
	cx.readClipboard(driver.ClipboardReadImage, func(ev driver.Event) {
		if img, ok := ev.Data.(*driver.ClipboardImage); ok && img != nil && len(img.PNG) > 0 {
			done(*img, true)
			return
		}
		done(ClipboardImage{}, false)
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
	if t := cx.rt.tree; t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemAnnounce, Value: text})
		t.changed = true
	}
}

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
	t := cx.rt.tree
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
		t := cx.rt.tree
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
	r := driver.PickRequest{Limit: 1, MaxDimension: max(o.MaxDimension, 0), Quality: min(max(o.Quality, 0), 1),
		Format: o.Format}
	if o.Multiple {
		r.Limit = driver.MaxPicked
		if o.Limit > 0 {
			r.Limit = min(o.Limit, driver.MaxPicked)
		}
	}
	return r
}

// PickImages shows the system's photo picker and calls done with the
// images the user picked, in the order picked, prepared as o says, or
// none when they cancelled: PHPickerViewController on iOS, Android's
// photo picker, an open panel on the desktops and a file chooser on the
// web. The mobile pickers need no permission.
func (cx *Scope) PickImages(o PickOptions, done func(files []PickedFile)) {
	t := cx.rt.tree
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
