package main

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getProcessMemoryInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

// An elevated runner enables its debug privilege, which opening dwm.exe, a
// protected process, takes; without it the compositor goes unread.
func init() {
	var token windows.Token
	if windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token) != nil {
		return
	}
	defer token.Close()
	var luid windows.LUID
	name, _ := windows.UTF16PtrFromString("SeDebugPrivilege")
	if windows.LookupPrivilegeValue(nil, name, &luid) != nil {
		return
	}
	p := windows.Tokenprivileges{PrivilegeCount: 1}
	p.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	windows.AdjustTokenPrivileges(token, false, &p, 0, nil, nil)
}

// process holds a handle of the process.
type process struct{ h windows.Handle }

func openProcess(pid int) (*process, error) {
	return openWith(pid, windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ)
}

func openWith(pid int, access uint32) (*process, error) {
	h, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	return &process{h: h}, nil
}

func (p *process) close() { windows.CloseHandle(p.h) }

// cpu is the process' kernel and user time.
func (p *process) cpu() (time.Duration, error) {
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(p.h, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	ticks := func(f windows.Filetime) int64 { return int64(f.HighDateTime)<<32 | int64(f.LowDateTime) }
	return time.Duration(ticks(kernel)+ticks(user)) * 100, nil
}

// processMemoryCounters is PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// memory is the process' working set.
func (p *process) memory() (int64, error) {
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	if r, _, err := getProcessMemoryInfo.Call(uintptr(p.h), uintptr(unsafe.Pointer(&c)), uintptr(c.cb)); r == 0 {
		return 0, err
	}
	return int64(c.workingSetSize), nil
}

func cpuAndMemory() (string, int64) {
	out, _ := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_Processor).Name; (Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory").Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	name := strings.TrimSpace(lines[0])
	var mem int64
	if len(lines) > 1 {
		mem, _ = strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
	}
	return name, mem
}

// compositorCPU is the Desktop Window Manager's CPU time, which
// composites every window.
func compositorCPU() (time.Duration, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), "dwm.exe") {
			// dwm.exe is protected: its times, not its memory.
			p, err := openWith(int(e.ProcessID), windows.PROCESS_QUERY_LIMITED_INFORMATION)
			if err != nil {
				return 0, err
			}
			defer p.close()
			return p.cpu()
		}
	}
	return 0, errProcess
}
