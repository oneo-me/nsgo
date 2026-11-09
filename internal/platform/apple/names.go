//go:build darwin

package apple

import (
	"sync"

	"github.com/ebitengine/purego/objc"
)

// The backends name selectors and classes as they send messages, many on
// every frame: sel_registerName copies the name into a C string and takes
// the runtime's lock each time, a quarter of a microsecond, so Sel and
// Class keep what they found.
var (
	sels    sync.Map // string → objc.SEL
	classes sync.Map // string → objc.Class
)

// Sel returns the selector named name.
func Sel(name string) objc.SEL {
	if s, ok := sels.Load(name); ok {
		return s.(objc.SEL)
	}
	s := objc.RegisterName(name)
	sels.Store(name, s)
	return s
}

// Class returns the class named name, or 0 while none is loaded.
func Class(name string) objc.Class {
	if c, ok := classes.Load(name); ok {
		return c.(objc.Class)
	}
	c := objc.GetClass(name)
	if c != 0 {
		classes.Store(name, c)
	}
	return c
}
