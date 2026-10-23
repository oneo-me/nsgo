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

	"github.com/oneo-me/nsgo/core"
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

// constant reads a global CFStringRef constant of lib.
func constant(lib uintptr, name string) uintptr {
	addr := symbol(lib, name)
	return **(**uintptr)(unsafe.Pointer(&addr))
}

// coreFoundation is CoreFoundation, kept for CFStringConstant.
var coreFoundation uintptr

// CFStringConstant returns a global CFString constant such as
// kCFRunLoopCommonModes: a run loop mode must be the real constant, not a
// string that merely compares equal.
func CFStringConstant(name string) uintptr {
	return constant(coreFoundation, name)
}

func init() {
	cf := load("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	coreFoundation = cf
	cg := load("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
	purego.RegisterLibFunc(&cfStringCreateWithBytes, cf, "CFStringCreateWithBytes")
	purego.RegisterLibFunc(&cfDictionaryCreate, cf, "CFDictionaryCreate")
	purego.RegisterLibFunc(&cfNumberCreate, cf, "CFNumberCreate")
	keyCallBacks = symbol(cf, "kCFTypeDictionaryKeyCallBacks")
	valueCBs = symbol(cf, "kCFTypeDictionaryValueCallBacks")
	purego.RegisterLibFunc(&cgColorCreateSRGB, cg, "CGColorCreateSRGB")
	loadText(cf, cg)
	loadSystem()
	loadImage(cf, cg)
	loadCanvas(cg)
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
func CGColor(c core.Color) uintptr {
	r, g, b, a := c.Components()
	return cgColorCreateSRGB(r, g, b, a)
}
