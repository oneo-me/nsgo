//go:build windows

package windows

// The environment's options, as WebView2EnvironmentOptions.h's CoreWebView2EnvironmentOptions gives them to WebView2Loader, implemented in Go: NSGo calls the runtime's DLL itself, so it hands it these. They carry only what the backend sets, the app's URL schemes (ICoreWebView2EnvironmentOptions4), and the defaults of the header's class otherwise; the runtime asks for the later options interfaces and takes their defaults when they are missing, as it does from an older SDK's object.

import (
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var coTaskMemAlloc = ole32.NewProc("CoTaskMemAlloc")

var (
	iidEnvironmentOptions       = parseGUID("2fde08a8-1e9a-4766-8c05-95a9ceb9d1c5")
	iidEnvironmentOptions4      = parseGUID("ac52d13f-0d38-475a-9dca-876580d6793e")
	iidCustomSchemeRegistration = parseGUID("d60ac92c-37a6-4b26-a39e-95cfe59047bb")
	webEnvironmentObjectsMu     sync.Mutex
	webEnvironmentObjects       = map[unsafe.Pointer]struct{}{} // what COM holds, kept from the collector until released
	webEnvironmentOptionsVtbl   [11]uintptr
	webEnvironmentOptions4Vtbl  [5]uintptr
	webSchemeRegistrationVtbl   [10]uintptr
	webOptionsVtblsOnce         sync.Once
)

// webEnvironmentOptions is an ICoreWebView2EnvironmentOptions whose ICoreWebView2EnvironmentOptions4 is its second vtable.
type webEnvironmentOptions struct {
	vtbl    *[11]uintptr
	vtbl4   *[5]uintptr
	refs    atomic.Int32
	schemes []*webSchemeRegistration
}

// webSchemeRegistration is an ICoreWebView2CustomSchemeRegistration of an app scheme: a secure context with an authority (gallery://inline/index.html), which pages of other schemes cannot request, as no origins are allowed.
type webSchemeRegistration struct {
	vtbl *[10]uintptr
	refs atomic.Int32
	name string
}

// newWebEnvironmentOptions returns options registering schemes, holding the caller's reference.
func newWebEnvironmentOptions(schemes []string) *webEnvironmentOptions {
	webOptionsVtblsOnce.Do(makeWebEnvironmentOptionsVtbls)
	o := &webEnvironmentOptions{vtbl: &webEnvironmentOptionsVtbl, vtbl4: &webEnvironmentOptions4Vtbl}
	for _, name := range schemes {
		r := &webSchemeRegistration{vtbl: &webSchemeRegistrationVtbl, name: name}
		r.refs.Store(1)
		keepWebEnvironmentObject(unsafe.Pointer(r))
		o.schemes = append(o.schemes, r)
	}
	o.refs.Store(1)
	keepWebEnvironmentObject(unsafe.Pointer(o))
	return o
}

func (o *webEnvironmentOptions) ptr() uintptr { return uintptr(unsafe.Pointer(o)) }

func (o *webEnvironmentOptions) release() { webOptionsRelease(o) }

func keepWebEnvironmentObject(p unsafe.Pointer) {
	webEnvironmentObjectsMu.Lock()
	webEnvironmentObjects[p] = struct{}{}
	webEnvironmentObjectsMu.Unlock()
}

func dropWebEnvironmentObject(p unsafe.Pointer) {
	webEnvironmentObjectsMu.Lock()
	delete(webEnvironmentObjects, p)
	webEnvironmentObjectsMu.Unlock()
}

// webTargetCompatibleVersion is CORE_WEBVIEW_TARGET_PRODUCT_VERSION of SDK 1.0.2903.40, whose vtables the backend follows.
const webTargetCompatibleVersion = "131.0.2903.40"

const (
	eNotImpl     = 0x80004001
	ePointer     = 0x80004003
	eOutOfMemory = 0x8007000e
)

// coAlloc returns n bytes the caller frees with CoTaskMemFree, nil when there is no memory.
func coAlloc(n uintptr) unsafe.Pointer {
	p, _, _ := coTaskMemAlloc.Call(n)
	// Memory COM allocated, which the collector does not move.
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

// coString returns s as a string the caller frees with CoTaskMemFree, 0 when there is no memory.
func coString(s string) uintptr {
	u, _ := syscall.UTF16FromString(s)
	p := coAlloc(uintptr(len(u) * 2))
	if p != nil {
		copy(unsafe.Slice((*uint16)(p), len(u)), u)
	}
	return uintptr(p)
}

// putString stores s for an out LPWSTR.
func putString(out *uintptr, s string) uintptr {
	if out == nil {
		return ePointer
	}
	if *out = coString(s); *out == 0 {
		return eOutOfMemory
	}
	return sOK
}

// putBool stores b for an out BOOL.
func putBool(out *int32, b bool) uintptr {
	if out == nil {
		return ePointer
	}
	*out = 0
	if b {
		*out = 1
	}
	return sOK
}

// optionsOf returns the options whose ICoreWebView2EnvironmentOptions4 is this.
func optionsOf(this unsafe.Pointer) *webEnvironmentOptions {
	return (*webEnvironmentOptions)(unsafe.Add(this, -int(unsafe.Offsetof(webEnvironmentOptions{}.vtbl4))))
}

func webOptionsQueryInterface(o *webEnvironmentOptions, iid *guid, out *unsafe.Pointer) uintptr {
	if out == nil {
		return ePointer
	}
	switch *iid {
	case iidUnknown, iidEnvironmentOptions:
		*out = unsafe.Pointer(o)
	case iidEnvironmentOptions4:
		*out = unsafe.Pointer(&o.vtbl4)
	default:
		*out = nil
		return eNoInterface
	}
	o.refs.Add(1)
	return sOK
}

func webOptionsAddRef(o *webEnvironmentOptions) uintptr { return uintptr(o.refs.Add(1)) }

func webOptionsRelease(o *webEnvironmentOptions) uintptr {
	n := o.refs.Add(-1)
	if n == 0 {
		for _, r := range o.schemes {
			webSchemeRelease(r)
		}
		dropWebEnvironmentObject(unsafe.Pointer(o))
	}
	return uintptr(n)
}

func webSchemeRelease(r *webSchemeRegistration) uintptr {
	n := r.refs.Add(-1)
	if n == 0 {
		dropWebEnvironmentObject(unsafe.Pointer(r))
	}
	return uintptr(n)
}

func makeWebEnvironmentOptionsVtbls() {
	// A setter takes one word after this; the options are read only.
	notImpl := syscall.NewCallback(func(this, value uintptr) uintptr { return eNotImpl })
	nullString := syscall.NewCallback(func(o *webEnvironmentOptions, out *uintptr) uintptr {
		if out == nil {
			return ePointer
		}
		*out = 0
		return sOK
	})
	// ICoreWebView2EnvironmentOptions
	webEnvironmentOptionsVtbl = [11]uintptr{
		syscall.NewCallback(webOptionsQueryInterface),
		syscall.NewCallback(webOptionsAddRef),
		syscall.NewCallback(webOptionsRelease),
		nullString, // get_AdditionalBrowserArguments
		notImpl,    // put_AdditionalBrowserArguments
		nullString, // get_Language
		notImpl,    // put_Language
		syscall.NewCallback(func(o *webEnvironmentOptions, out *uintptr) uintptr { // get_TargetCompatibleBrowserVersion
			return putString(out, webTargetCompatibleVersion)
		}),
		notImpl, // put_TargetCompatibleBrowserVersion
		syscall.NewCallback(func(o *webEnvironmentOptions, out *int32) uintptr { // get_AllowSingleSignOnUsingOSPrimaryAccount
			return putBool(out, false)
		}),
		notImpl, // put_AllowSingleSignOnUsingOSPrimaryAccount
	}
	// ICoreWebView2EnvironmentOptions4
	webEnvironmentOptions4Vtbl = [5]uintptr{
		syscall.NewCallback(func(this unsafe.Pointer, iid *guid, out *unsafe.Pointer) uintptr {
			return webOptionsQueryInterface(optionsOf(this), iid, out)
		}),
		syscall.NewCallback(func(this unsafe.Pointer) uintptr { return webOptionsAddRef(optionsOf(this)) }),
		syscall.NewCallback(func(this unsafe.Pointer) uintptr { return webOptionsRelease(optionsOf(this)) }),
		syscall.NewCallback(func(this unsafe.Pointer, count *uint32, out *uintptr) uintptr { // GetCustomSchemeRegistrations
			if count == nil || out == nil {
				return ePointer
			}
			o := optionsOf(this)
			*count, *out = 0, 0
			if len(o.schemes) == 0 {
				return sOK
			}
			p := coAlloc(uintptr(len(o.schemes)) * unsafe.Sizeof(uintptr(0)))
			if p == nil {
				return eOutOfMemory
			}
			list := unsafe.Slice((*uintptr)(p), len(o.schemes))
			for i, r := range o.schemes {
				r.refs.Add(1)
				list[i] = uintptr(unsafe.Pointer(r))
			}
			*count, *out = uint32(len(o.schemes)), uintptr(p)
			return sOK
		}),
		syscall.NewCallback(func(this unsafe.Pointer, count, registrations uintptr) uintptr { return eNotImpl }), // SetCustomSchemeRegistrations
	}
	// ICoreWebView2CustomSchemeRegistration
	webSchemeRegistrationVtbl = [10]uintptr{
		syscall.NewCallback(func(r *webSchemeRegistration, iid *guid, out *unsafe.Pointer) uintptr {
			if out == nil {
				return ePointer
			}
			if *iid != iidUnknown && *iid != iidCustomSchemeRegistration {
				*out = nil
				return eNoInterface
			}
			*out = unsafe.Pointer(r)
			r.refs.Add(1)
			return sOK
		}),
		syscall.NewCallback(func(r *webSchemeRegistration) uintptr { return uintptr(r.refs.Add(1)) }),
		syscall.NewCallback(webSchemeRelease),
		syscall.NewCallback(func(r *webSchemeRegistration, out *uintptr) uintptr { return putString(out, r.name) }), // get_SchemeName
		syscall.NewCallback(func(r *webSchemeRegistration, out *int32) uintptr { return putBool(out, true) }),       // get_TreatAsSecure
		notImpl, // put_TreatAsSecure
		syscall.NewCallback(func(r *webSchemeRegistration, count *uint32, out *uintptr) uintptr { // GetAllowedOrigins
			if count == nil || out == nil {
				return ePointer
			}
			*count, *out = 0, 0
			return sOK
		}),
		syscall.NewCallback(func(r *webSchemeRegistration, count, origins uintptr) uintptr { return eNotImpl }), // SetAllowedOrigins
		syscall.NewCallback(func(r *webSchemeRegistration, out *int32) uintptr { return putBool(out, true) }),   // get_HasAuthorityComponent
		notImpl, // put_HasAuthorityComponent
	}
}
