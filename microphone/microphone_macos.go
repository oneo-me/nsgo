//go:build darwin && !ios

package microphone

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

// enterSession does nothing on macOS, whose recordings need no session
// and end only when stopped.
func enterSession(func()) (leave func(), err error) { return func() {}, nil }

// checkUsageDescription stands in front of the permission prompt: macOS asks
// on behalf of the responsible process, which an app started from another
// app is not, and aborts this one when that process declares no usage
// description. Where the system cannot name that process, it lets the
// prompt go ahead.
func checkUsageDescription() error {
	responsible, err := purego.Dlsym(purego.RTLD_DEFAULT, "responsibility_get_pid_responsible_for_pid")
	if err != nil {
		return nil
	}
	pidPath, err := purego.Dlsym(purego.RTLD_DEFAULT, "proc_pidpath")
	if err != nil {
		return nil
	}
	pid, _, _ := purego.SyscallN(responsible, uintptr(os.Getpid()))
	path := make([]byte, 4096)
	n, _, _ := purego.SyscallN(pidPath, pid, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
	if int32(n) <= 0 {
		return nil
	}
	if !declaresUsage(string(path[:n])) {
		return fmt.Errorf("%w: %s", ErrNoUsageDescription, path[:n])
	}
	return nil
}

// declaresUsage says the app bundle holding the executable has a usage
// description. Keys are plain ASCII in both XML and binary property lists.
func declaresUsage(executable string) bool {
	end := strings.LastIndex(executable, ".app/Contents/")
	if end < 0 {
		return false
	}
	info, err := os.ReadFile(executable[:end] + ".app/Contents/Info.plist")
	return err == nil && bytes.Contains(info, []byte("NSMicrophoneUsageDescription"))
}
