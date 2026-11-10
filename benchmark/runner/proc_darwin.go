package main

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	procPidRusage    func(pid int32, flavor int32, buf unsafe.Pointer) int32
	machTimebaseInfo func(info unsafe.Pointer) int32
)

func init() {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&procPidRusage, lib, "proc_pid_rusage")
	purego.RegisterLibFunc(&machTimebaseInfo, lib, "mach_timebase_info")
}

// process reads a process' rusage_info_v2.
type process struct {
	pid          int32
	numer, denom uint64
}

func openProcess(pid int) (*process, error) {
	var tb [2]uint32
	machTimebaseInfo(unsafe.Pointer(&tb))
	return &process{pid: int32(pid), numer: uint64(tb[0]), denom: uint64(tb[1])}, nil
}

func (p *process) close() {}

// usage is rusage_info_v2, by 8 bytes: the UUID, then ri_user_time,
// ri_system_time, …, ri_phys_footprint at 9.
func (p *process) usage() ([32]uint64, error) {
	var buf [32]uint64
	if procPidRusage(p.pid, 2, unsafe.Pointer(&buf)) != 0 {
		return buf, errProcess
	}
	return buf, nil
}

// cpu is the process' user and system time, in Mach time converted.
func (p *process) cpu() (time.Duration, error) {
	u, err := p.usage()
	if err != nil {
		return 0, err
	}
	return time.Duration((u[2] + u[3]) * p.numer / p.denom), nil
}

// memory is the process' physical footprint, as Activity Monitor shows it.
func (p *process) memory() (int64, error) {
	u, err := p.usage()
	if err != nil {
		return 0, err
	}
	return int64(u[9]), nil
}

func cpuAndMemory() (string, int64) {
	name, _ := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
	mem, _ := exec.Command("sysctl", "-n", "hw.memsize").Output()
	n, _ := strconv.ParseInt(strings.TrimSpace(string(mem)), 10, 64)
	return strings.TrimSpace(string(name)), n
}

// compositorCPU is WindowServer's CPU time, which draws and composites
// every app's layers: ps reads it for any user, in hundredths of a second.
func compositorCPU() (time.Duration, error) {
	pid, err := exec.Command("pgrep", "-x", "WindowServer").Output()
	if err != nil {
		return 0, err
	}
	out, err := exec.Command("ps", "-o", "time=", "-p", strings.TrimSpace(string(pid))).Output()
	if err != nil {
		return 0, err
	}
	// [hours:]minutes:seconds.hundredths
	var d time.Duration
	for _, part := range strings.Split(strings.TrimSpace(string(out)), ":") {
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0, err
		}
		d = d*60 + time.Duration(v*float64(time.Second))
	}
	return d, nil
}
