//go:build darwin

// Package apple holds what the AppKit and UIKit backends share, called
// through purego without cgo: CoreFoundation strings, CoreGraphics colors
// and canvases, layer styles and transitions, the CoreText text engine,
// which prepares and lays text out on the engine goroutine for the main
// thread to draw, and image views, which load and decode off it.
package apple

import (
	"structs"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

// CGPoint, CGSize and CGRect match CoreGraphics' layouts on 64-bit Apple
// platforms.
type CGPoint struct {
	_    structs.HostLayout
	X, Y float64
}

type CGSize struct {
	_             structs.HostLayout
	Width, Height float64
}

type CGRect struct {
	_      structs.HostLayout
	Origin CGPoint
	Size   CGSize
}

// Rect returns a CGRect.
func Rect(x, y, w, h float64) CGRect {
	return CGRect{Origin: CGPoint{X: x, Y: y}, Size: CGSize{Width: w, Height: h}}
}

// NSRange is Foundation's range of UTF-16 units.
type NSRange struct {
	_                structs.HostLayout
	Location, Length uint
}

type cfRange struct {
	_                structs.HostLayout
	Location, Length int
}

const utf8Encoding = 0x08000100

var (
	cfRelease               func(uintptr)
	cfStringCreateWithBytes func(alloc uintptr, bytes *byte, n int, encoding uint32, bom bool) uintptr
	cfDictionaryCreate      func(alloc uintptr, keys, values *uintptr, n int, keyCB, valueCB uintptr) uintptr
	cfNumberCreate          func(alloc uintptr, kind int, value unsafe.Pointer) uintptr
	cgColorCreateSRGB       func(r, g, b, a float64) uintptr
	keyCallBacks, valueCBs  uintptr
	cfArrayCreate           func(alloc uintptr, values *uintptr, n int, cb uintptr) uintptr
	arrayCallBacks          uintptr
)

func load(path string) uintptr {
	lib, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	return lib
}

func symbol(lib uintptr, name string) uintptr {
	addr, err := purego.Dlsym(lib, name)
	if err != nil {
		panic(err)
	}
	return addr
}

// registerSyscall sets fn, a pointer to a func variable of one of the
// types below, to call lib's function name through purego.SyscallN, as
// purego.RegisterLibFunc would through reflection. Only functions whose
// arguments and result are pointers and integers can go this way;
// RegisterLibFunc builds every call's arguments with reflection and
// allocates, at twice the cost of the call or more, and canvases make
// thousands of these calls a frame.
func registerSyscall(fn any, lib uintptr, name string) {
	sym := symbol(lib, name)
	switch fn := fn.(type) {
	case *func(uintptr):
		*fn = func(a uintptr) { purego.SyscallN(sym, a) }
	case *func(uintptr) uintptr:
		*fn = func(a uintptr) uintptr { r, _, _ := purego.SyscallN(sym, a); return r }
	case *func(uintptr) bool:
		*fn = func(a uintptr) bool { r, _, _ := purego.SyscallN(sym, a); return byte(r) != 0 }
	case *func(uintptr, uintptr):
		*fn = func(a, b uintptr) { purego.SyscallN(sym, a, b) }
	case *func(uintptr, int32):
		*fn = func(a uintptr, v int32) { purego.SyscallN(sym, a, uintptr(v)) }
	default:
		panic("apple: registerSyscall of an unknown function type")
	}
}

// constant reads a global CFStringRef constant of lib.
func constant(lib uintptr, name string) uintptr {
	addr := symbol(lib, name)
	return **(**uintptr)(unsafe.Pointer(&addr))
}

func init() {
	cf := load("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	cg := load("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	registerSyscall(&cfRelease, cf, "CFRelease")
	purego.RegisterLibFunc(&cfStringCreateWithBytes, cf, "CFStringCreateWithBytes")
	purego.RegisterLibFunc(&cfDictionaryCreate, cf, "CFDictionaryCreate")
	purego.RegisterLibFunc(&cfNumberCreate, cf, "CFNumberCreate")
	keyCallBacks = symbol(cf, "kCFTypeDictionaryKeyCallBacks")
	valueCBs = symbol(cf, "kCFTypeDictionaryValueCallBacks")
	purego.RegisterLibFunc(&cfArrayCreate, cf, "CFArrayCreate")
	arrayCallBacks = symbol(cf, "kCFTypeArrayCallBacks")
	purego.RegisterLibFunc(&cgColorCreateSRGB, cg, "CGColorCreateSRGB")
	loadText(cf, cg)
	loadSystem()
	loadImage(cf, cg)
	loadCanvas(cg)
	loadCanvasLayer(cg)
	loadBorders(cg)
	qc := load("/System/Library/Frameworks/QuartzCore.framework/QuartzCore")
	purego.RegisterLibFunc(&caCurrentMediaTime, qc, "CACurrentMediaTime")
}

// Release calls CFRelease on a CoreFoundation object.
func Release(ref uintptr) {
	if ref != 0 {
		cfRelease(ref)
	}
}

// dictionary returns a new CFDictionary of CF keys and values. The
// caller releases it.
func dictionary(keys, values []uintptr) uintptr {
	if len(keys) == 0 {
		return cfDictionaryCreate(0, nil, nil, 0, keyCallBacks, valueCBs)
	}
	return cfDictionaryCreate(0, &keys[0], &values[0], len(keys), keyCallBacks, valueCBs)
}

// cfArray returns a new CFArray of CF values. The caller releases it.
func cfArray(values []uintptr) uintptr {
	if len(values) == 0 {
		return cfArrayCreate(0, nil, 0, arrayCallBacks)
	}
	return cfArrayCreate(0, &values[0], len(values), arrayCallBacks)
}

// float64Number and int32Number return new CFNumbers. The caller
// releases them.
func float64Number(v float64) uintptr { return cfNumberCreate(0, 6, unsafe.Pointer(&v)) }
func int32Number(v int32) uintptr     { return cfNumberCreate(0, 3, unsafe.Pointer(&v)) }

// CFString returns a new CFString (an NSString) holding s. The caller
// releases it.
func CFString(s string) uintptr {
	b := []byte(s)
	if len(b) == 0 {
		b = []byte{0}
		return cfStringCreateWithBytes(0, &b[0], 0, utf8Encoding, false)
	}
	return cfStringCreateWithBytes(0, &b[0], len(b), utf8Encoding, false)
}

// NSString returns an autoreleased NSString holding s.
func NSString(s string) objc.ID {
	ref := CFString(s)
	return objc.ID(ref).Send(objc.RegisterName("autorelease"))
}

// GoString returns the Go string of an NSString.
func GoString(ns objc.ID) string {
	if ns == 0 {
		return ""
	}
	p := objc.Send[*byte](ns, objc.RegisterName("UTF8String"))
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// CGColor returns a new CGColor of c. The caller releases it.
func CGColor(c driver.Color) uintptr {
	r, g, b, a := c.Components()
	return cgColorCreateSRGB(r, g, b, a)
}

// Modifiers returns the modifier keys among the flags AppKit and UIKit
// share.
func Modifiers(flags uint) driver.Modifiers {
	const shift, control, option, command = 1 << 17, 1 << 18, 1 << 19, 1 << 20
	return driver.Modifiers{Control: flags&control != 0, Alt: flags&option != 0, Shift: flags&shift != 0,
		Platform: flags&command != 0}
}
