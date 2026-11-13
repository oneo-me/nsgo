//go:build darwin

package widget

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// On macOS and iOS (the darwin build constraint holds for both), the
// bundle's Info.plist, which nsgo build writes, says where the app and
// its widget extension keep the timelines: in the container of the app
// group they share (appGroupKey), or on macOS in a directory of the
// user's home (widgetsPathKey), which the extension, in the App Sandbox,
// may read. WidgetKit's WidgetCenter, which reloads widgets, and
// ActivityKit have no Objective-C interface: a library of Swift that nsgo
// build puts among the bundle's frameworks calls them (kitLibraries),
// loaded as the app first needs it.

const (
	appGroupKey    = "NSGoAppGroup"
	widgetsPathKey = "NSGoWidgetsPath"
	// activitiesKey is the Info.plist's NSSupportsLiveActivities, which
	// nsgo build sets in an app with live_activities.
	activitiesKey = "NSSupportsLiveActivities"
)

// kitLibraries are where the bundle's frameworks hold the library of
// Swift, iOS's and macOS's.
var kitLibraries = []string{"NSGoWidgetKit.framework/NSGoWidgetKit", "libNSGoWidgetKit.dylib"}

var native struct {
	setup sync.Once
	dir   string // the directory of the timelines in the container
	err   error
	// frameworks is the bundle's directory of frameworks.
	frameworks string
	// activities is whether the bundle has Live Activities.
	activities bool

	kit    sync.Once
	kitLib uintptr
	kitErr error

	reload    sync.Once
	reloadFn  uintptr
	reloadErr error

	msgSend, poolPush, poolPop uintptr
}

func load() {
	native.setup.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if err == nil {
			_, err = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		}
		for _, s := range []struct {
			addr *uintptr
			name string
		}{{&native.msgSend, "objc_msgSend"}, {&native.poolPush, "objc_autoreleasePoolPush"}, {&native.poolPop, "objc_autoreleasePoolPop"}} {
			if err == nil {
				*s.addr, err = purego.Dlsym(lib, s.name)
			}
		}
		if err != nil {
			native.err = fmt.Errorf("widget: %w", err)
			return
		}
		withPool(func() {
			bundle := send(class("NSBundle"), "mainBundle")
			group := goString(send(bundle, "objectForInfoDictionaryKey:", nsString(appGroupKey)))
			path := goString(send(bundle, "objectForInfoDictionaryKey:", nsString(widgetsPathKey)))
			native.frameworks = goString(send(bundle, "privateFrameworksPath"))
			native.activities = send(send(bundle, "objectForInfoDictionaryKey:", nsString(activitiesKey)), "boolValue")&0xff != 0
			switch {
			case group == "" && path == "":
				native.err = ErrNoWidgets
				return
			case group == "":
				home, err := os.UserHomeDir()
				if err != nil {
					native.err = fmt.Errorf("widget: %w", err)
					return
				}
				native.dir = filepath.Join(home, path)
				return
			}
			fm := send(class("NSFileManager"), "defaultManager")
			url := send(fm, "containerURLForSecurityApplicationGroupIdentifier:", nsString(group))
			if url == 0 {
				native.err = fmt.Errorf("widget: the app cannot reach the container of its app group %s: its signature lacks the group", group)
				return
			}
			native.dir = filepath.Join(goString(send(url, "path")), "NSGoWidgets")
		})
	})
}

func containerDir() (string, error) {
	load()
	return native.dir, native.err
}

// kit loads the bundle's library of Swift.
func kit() (uintptr, error) {
	native.kit.Do(func() {
		load()
		for _, name := range kitLibraries {
			lib, err := purego.Dlopen(filepath.Join(native.frameworks, name), purego.RTLD_LOCAL|purego.RTLD_NOW)
			if err == nil {
				native.kitLib = lib
				return
			}
		}
		native.kitErr = fmt.Errorf("widget: the app has no NSGoWidgetKit library in %s", native.frameworks)
	})
	return native.kitLib, native.kitErr
}

// reload asks WidgetKit to read the kind's timeline again.
func reload(kind string) error {
	native.reload.Do(func() {
		lib, err := kit()
		if err == nil {
			native.reloadFn, err = purego.Dlsym(lib, "nsgo_widgets_reload")
		}
		if err != nil {
			native.reloadErr = fmt.Errorf("widget: the timeline is written, but the app cannot reload its widgets: %w", err)
		}
	})
	if native.reloadErr != nil {
		return native.reloadErr
	}
	call(native.reloadFn, unsafe.Pointer(unsafe.StringData(kind+"\x00")))
	return nil
}

// call calls a C function of one pointer, which stays alive and in
// place until it returns.
//
//go:uintptrescapes
func call(fn uintptr, p unsafe.Pointer) { purego.SyscallN(fn, uintptr(p)) }

// send sends an Objective-C message of integer and pointer arguments.
//
//go:uintptrescapes
func send(obj objc.ID, selector string, args ...uintptr) objc.ID {
	a := append([]uintptr{uintptr(obj), uintptr(objc.RegisterName(selector))}, args...)
	r, _, _ := purego.SyscallN(native.msgSend, a...)
	return objc.ID(r)
}

func class(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

// withPool runs fn in an autorelease pool, on the thread that pushed it.
func withPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool, _, _ := purego.SyscallN(native.poolPush)
	defer purego.SyscallN(native.poolPop, pool)
	fn()
}

// nsString returns an autoreleased NSString of s.
func nsString(s string) uintptr {
	b := append([]byte(s), 0)
	str := send(class("NSString"), "stringWithUTF8String:", uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
	return uintptr(str)
}

// goString copies an NSString, or returns "" for anything else.
func goString(str objc.ID) string {
	if str == 0 || send(str, "isKindOfClass:", uintptr(class("NSString")))&0xff == 0 {
		return ""
	}
	p := send(str, "UTF8String")
	if p == 0 {
		return ""
	}
	n := int(send(str, "lengthOfBytesUsingEncoding:", 4)) // UTF-8
	return string(unsafe.Slice(*(**byte)(unsafe.Pointer(&p)), n))
}
