//go:build darwin && !ios

package apple

import (
	"bytes"
	"os"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

// holdSpeechSession does nothing on macOS, whose input needs no session.
func holdSpeechSession() (leave func(), err error) { return func() {}, nil }

// declaresUsage reports whether the app macOS asks on behalf of declares
// key in its Info.plist: the responsible process, which an app started
// from another app is not, and which the system ends this one for when it
// declares none. Where the system cannot name that process, it says yes.
func declaresUsage(key string) bool {
	responsible, err := purego.Dlsym(purego.RTLD_DEFAULT, "responsibility_get_pid_responsible_for_pid")
	if err != nil {
		return true
	}
	pidPath, err := purego.Dlsym(purego.RTLD_DEFAULT, "proc_pidpath")
	if err != nil {
		return true
	}
	pid, _, _ := purego.SyscallN(responsible, uintptr(os.Getpid()))
	path := make([]byte, 4096)
	n, _, _ := purego.SyscallN(pidPath, pid, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
	if int32(n) <= 0 {
		return true
	}
	executable := string(path[:n])
	end := strings.LastIndex(executable, ".app/Contents/")
	if end < 0 {
		return false
	}
	// Keys are plain ASCII in both XML and binary property lists.
	info, err := os.ReadFile(executable[:end] + ".app/Contents/Info.plist")
	return err == nil && bytes.Contains(info, []byte(key))
}
