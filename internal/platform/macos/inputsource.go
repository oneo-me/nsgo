//go:build darwin && !ios

package macos

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/platform/apple"
)

const carbonPath = "/System/Library/Frameworks/Carbon.framework/Carbon"

// Text Input Sources, which change the source whichever view has the
// keyboard: a field's input context would be gone by the time a palette
// restores what it replaced.
var (
	inputSources = sync.OnceValue(func() (tis struct {
		current func() uintptr
		find    func(filter uintptr, all bool) uintptr
		sel     func(source uintptr) int32
		name    func(source, key uintptr) uintptr
		id      uintptr
	}) {
		lib, err := purego.Dlopen(carbonPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			panic(err)
		}
		purego.RegisterLibFunc(&tis.current, lib, "TISCopyCurrentKeyboardInputSource")
		purego.RegisterLibFunc(&tis.find, lib, "TISCreateInputSourceList")
		purego.RegisterLibFunc(&tis.sel, lib, "TISSelectInputSource")
		purego.RegisterLibFunc(&tis.name, lib, "TISGetInputSourceProperty")
		tis.id = uintptr(apple.Global(carbonPath, "kTISPropertyInputSourceID"))
		return tis
	})
	// inputSourceWas is what inputSourceSet replaced, for restoring.
	inputSourceWas, inputSourceSet string
)

// currentInputSource is the ID of the selected input source.
func currentInputSource() string {
	tis := inputSources()
	source := tis.current()
	defer apple.Release(source)
	return apple.GoString(objc.ID(tis.name(source, tis.id)))
}

// selectInputSource selects the enabled source of id, reporting whether
// there is one.
func selectInputSource(id string) bool {
	tis := inputSources()
	filter := objc.ID(apple.Class("NSDictionary")).Send(sel("dictionaryWithObject:forKey:"), apple.NSString(id), tis.id)
	found := objc.ID(tis.find(uintptr(filter), false))
	defer apple.Release(uintptr(found))
	return found != 0 && found.Send(sel("count")) != 0 && tis.sel(uintptr(found.Send(sel("firstObject")))) == 0
}

// setInputSource selects id, or with none goes back to what was selected
// before, unless the user chose another since. Marked text ends as AppKit
// ends it: nothing here touches a field.
func setInputSource(id string) {
	if id != "" {
		was := currentInputSource()
		if was != id && selectInputSource(id) {
			inputSourceWas, inputSourceSet = was, id
		}
		return
	}
	was, set := inputSourceWas, inputSourceSet
	inputSourceWas, inputSourceSet = "", ""
	if set != "" && currentInputSource() == set {
		selectInputSource(was)
	}
}
