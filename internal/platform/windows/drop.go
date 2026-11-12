//go:build windows

package windows

// Files dropped from other apps, as NSH's file_drop.rs: while views take
// dropped files, the window is an OLE drop target, an IDropTarget
// implemented in Go. Entering checks that the drag carries files
// (CF_HDROP) without reading them; entering and moving convert the screen
// point to the client's and find the destination by the window's own hit
// testing, bounded by the newest modal: the innermost view under the
// pointer that takes dropped files and is enabled. It advertises Copy, and
// nothing without a destination. Dropping reads the file list in its
// order as file: URLs. The answers come from the mounted views at once,
// never from the engine. OLE needs the thread in a single-threaded
// apartment, which OleInitialize makes, as WebView2 needs anyway.

import (
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	oleInitialize    = ole32.NewProc("OleInitialize")
	registerDragDrop = ole32.NewProc("RegisterDragDrop")
	revokeDragDrop   = ole32.NewProc("RevokeDragDrop")
	releaseStgMedium = ole32.NewProc("ReleaseStgMedium")
	shell32          = syscall.NewLazyDLL("shell32.dll")
	dragQueryFileW   = shell32.NewProc("DragQueryFileW")

	iidDropTarget = parseGUID("00000122-0000-0000-c000-000000000046")
)

const (
	cfHDrop          = 15
	dvAspectContent  = 1
	tymedHGlobal     = 1
	dropEffectNone   = 0
	dropEffectCopy   = 1
	dataGetData      = 3 // IDataObject
	dataQueryGetData = 5
)

// formatEtc is FORMATETC.
type formatEtc struct {
	format uint16
	device uintptr
	aspect uint32
	index  int32
	tymed  uint32
}

// stgMedium is STGMEDIUM.
type stgMedium struct {
	tymed   uint32
	handle  uintptr
	release uintptr
}

func hdropFormat() formatEtc {
	return formatEtc{format: cfHDrop, aspect: dvAspectContent, index: -1, tymed: tymedHGlobal}
}

// dropTarget is the window's IDropTarget. OLE calls it on the window's
// thread.
type dropTarget struct {
	vtbl *[7]uintptr
	refs atomic.Int32
	w    *window
	// offered is whether the drag in progress carries files; target is
	// the view it would drop them on.
	offered bool
	target  *view
}

func dropQueryInterface(t *dropTarget, iid *guid, out *unsafe.Pointer) uintptr {
	if *iid == iidUnknown || *iid == iidDropTarget {
		*out = unsafe.Pointer(t)
		t.refs.Add(1)
		return sOK
	}
	*out = nil
	return eNoInterface
}

func dropAddRef(t *dropTarget) uintptr { return uintptr(t.refs.Add(1)) }

// dropRelease counts down; the window keeps the target alive while it is
// registered, so nothing is freed here.
func dropRelease(t *dropTarget) uintptr { return uintptr(t.refs.Add(-1)) }

func dropLeave(t *dropTarget) uintptr {
	t.offered = false
	t.setTarget(nil)
	return sOK
}

// enter answers DragEnter: whether the drag carries files, then where it
// is.
func (t *dropTarget) enter(data *comObject, at point, effect *uint32) uintptr {
	f := hdropFormat()
	t.offered = data != nil && data.call(dataQueryGetData, uintptr(unsafe.Pointer(&f))) == sOK
	t.over(at, effect)
	return sOK
}

// over answers DragOver at screen pixel at: the destination there, and
// Copy when there is one and the source allows it.
func (t *dropTarget) over(at point, effect *uint32) uintptr {
	if effect == nil {
		t.setTarget(nil)
		return sOK
	}
	var dest *view
	if t.offered && *effect&dropEffectCopy != 0 {
		screenToClient.Call(t.w.hwnd, uintptr(unsafe.Pointer(&at)))
		dest = t.w.dropTargetAt(t.w.point(at.X, at.Y))
	}
	if dest != nil {
		*effect = dropEffectCopy
	} else {
		*effect = dropEffectNone
	}
	t.setTarget(dest)
	return sOK
}

// drop answers Drop: the files go to the destination at the point, which
// then has the drag leave.
func (t *dropTarget) drop(data *comObject, at point, effect *uint32) uintptr {
	t.over(at, effect)
	var uris []string
	if dest := t.target; dest != nil && data != nil {
		uris = droppedFiles(data)
		if len(uris) > 0 {
			t.w.engine.Dispatch(driver.Event{Kind: driver.EventFileDrop, ID: dest.id, URIs: uris})
		}
	}
	if len(uris) == 0 && effect != nil {
		*effect = dropEffectNone
	}
	t.offered = false
	t.setTarget(nil)
	return sOK
}

// setTarget makes next the destination, telling the old one the files
// left and the new one they entered, where they listen.
func (t *dropTarget) setTarget(next *view) {
	prev := t.target
	if prev == next {
		return
	}
	t.target = next
	if prev != nil && !prev.deleted && prev.events.Has(driver.EventFileDragLeave) {
		t.w.engine.Dispatch(driver.Event{Kind: driver.EventFileDragLeave, ID: prev.id})
	}
	if next != nil && next.events.Has(driver.EventFileDragEnter) {
		t.w.engine.Dispatch(driver.Event{Kind: driver.EventFileDragEnter, ID: next.id})
	}
}

// droppedFiles reads the CF_HDROP list of data as file: URLs.
func droppedFiles(data *comObject) []string {
	f := hdropFormat()
	var m stgMedium
	if data.call(dataGetData, uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&m))) != sOK {
		return nil
	}
	// The medium owns the list: DragFinish would free it twice.
	defer releaseStgMedium.Call(uintptr(unsafe.Pointer(&m)))
	if m.tymed != tymedHGlobal || m.handle == 0 {
		return nil
	}
	return hdropFiles(m.handle)
}

// fileURL returns the file: URL of an absolute Windows path, "" for a
// relative one: file:///C:/dir/a%20b.txt for a drive's, and
// file://server/share/x for a UNC path.
func fileURL(path string) string {
	if !filepath.IsAbs(path) {
		return ""
	}
	p := filepath.ToSlash(path)
	u := url.URL{Scheme: "file"}
	if rest, ok := strings.CutPrefix(p, "//"); ok {
		host, share, _ := strings.Cut(rest, "/")
		u.Host, u.Path = host, "/"+share
	} else {
		u.Path = "/" + p
	}
	return u.String()
}

// dropTargetAt returns the view files dropped at p (client points) go to:
// the view under p, or its nearest ancestor, that takes them and is
// enabled, within the newest modal.
func (w *window) dropTargetAt(p driver.Point) *view {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	scope := w.focusScope()
	for v := pick(root, p); v != nil; v = v.parent {
		if v.events.Has(driver.EventFileDrop) && !v.props.Disabled {
			if isAncestor(scope, v) {
				return v
			}
			return nil
		}
		if v == scope {
			break
		}
	}
	return nil
}

// dropEvents follows v's events: the window takes drops while a view
// does.
func (w *window) dropEvents(v *view) {
	if v.events.Has(driver.EventFileDrop) {
		w.drops[v] = true
	} else {
		w.dropGone(v)
	}
}

// dropGone forgets v as a destination: a drag over it has none, and v,
// going, hears nothing more.
func (w *window) dropGone(v *view) {
	delete(w.drops, v)
	if t := w.drop; t != nil && t.target == v {
		t.target = nil
	}
}

// updateDrop registers the window as a drop target while views take
// drops, and revokes it when none does.
func (w *window) updateDrop() {
	switch {
	case len(w.drops) > 0 && w.drop == nil:
		const rpcEChangedMode = 0x80010106
		if hr, _, _ := oleInitialize.Call(0); uint32(hr) == rpcEChangedMode {
			log.Print("nsgo: file drops need a single-threaded COM apartment on the UI thread")
			return
		}
		t := &dropTarget{vtbl: &dropVtbl, w: w}
		t.refs.Store(1)
		if hr, _, _ := registerDragDrop.Call(w.hwnd, uintptr(unsafe.Pointer(t))); failed(hr) {
			log.Print(hresultError("nsgo: the window cannot take file drops", hr))
			return
		}
		w.drop = t
	case len(w.drops) == 0 && w.drop != nil:
		w.revokeDrop()
	}
}

// revokeDrop stops taking drops.
func (w *window) revokeDrop() {
	if t := w.drop; t != nil {
		t.setTarget(nil)
		revokeDragDrop.Call(w.hwnd)
		w.drop = nil
	}
}
