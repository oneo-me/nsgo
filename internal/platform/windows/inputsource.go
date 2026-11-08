//go:build windows

package windows

import (
	"slices"
	"strings"
	"syscall"
	"unsafe"
)

// Input sources are keyboard layouts, named by their layout IDs (KLIDs,
// "00000409" for US English), which the process switches to as Windows'
// language bar would.
var (
	getKeyboardLayoutNameW = user32.NewProc("GetKeyboardLayoutNameW")
	getKeyboardLayoutList  = user32.NewProc("GetKeyboardLayoutList")
	loadKeyboardLayoutW    = user32.NewProc("LoadKeyboardLayoutW")
	unloadKeyboardLayout   = user32.NewProc("UnloadKeyboardLayout")
	activateKeyboardLayout = user32.NewProc("ActivateKeyboardLayout")

	// inputSourceWas is what inputSourceSet replaced, for restoring.
	inputSourceWas, inputSourceSet string
)

// currentInputSource is the KLID of the UI thread's active layout.
func currentInputSource() string {
	var name [9]uint16
	if r, _, _ := getKeyboardLayoutNameW.Call(uintptr(unsafe.Pointer(&name[0]))); r == 0 {
		return ""
	}
	return syscall.UTF16ToString(name[:])
}

// selectInputSource activates the enabled layout id for the process,
// reporting whether there is one: a layout the user has not enabled is
// unloaded again rather than added to theirs.
func selectInputSource(id string) bool {
	const klfNoTellShell, klfSetForProcess = 0x80, 0x100
	n, _, _ := getKeyboardLayoutList.Call(0, 0)
	enabled := make([]uintptr, n)
	if n > 0 {
		n, _, _ = getKeyboardLayoutList.Call(n, uintptr(unsafe.Pointer(&enabled[0])))
		enabled = enabled[:n]
	}
	klid, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return false
	}
	hkl, _, _ := loadKeyboardLayoutW.Call(uintptr(unsafe.Pointer(klid)), klfNoTellShell)
	if hkl == 0 {
		return false
	}
	if !slices.Contains(enabled, hkl) {
		unloadKeyboardLayout.Call(hkl)
		return false
	}
	was, _, _ := activateKeyboardLayout.Call(hkl, klfSetForProcess)
	if was == 0 {
		return false
	}
	// Windows loads its default layout for an ID it lacks.
	if !strings.EqualFold(currentInputSource(), id) {
		activateKeyboardLayout.Call(was, klfSetForProcess)
		return false
	}
	return true
}

// setInputSource selects id, or with none goes back to what was selected
// before, unless the user chose another since.
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
