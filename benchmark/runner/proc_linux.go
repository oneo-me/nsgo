package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// process reads a process' /proc files.
type process struct{ pid int }

func openProcess(pid int) (*process, error) { return &process{pid: pid}, nil }

func (p *process) close() {}

// clockTicks is USER_HZ, 100 on every Linux the runner meets.
const clockTicks = 100

// cpu is the process' utime and stime, from /proc/<pid>/stat.
func (p *process) cpu() (time.Duration, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p.pid))
	if err != nil {
		return 0, err
	}
	// The fields after the command, which may hold spaces, in parentheses.
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return 0, errProcess
	}
	f := strings.Fields(string(b[i+1:]))
	if len(f) < 13 {
		return 0, errProcess
	}
	u, _ := strconv.ParseInt(f[11], 10, 64)
	s, _ := strconv.ParseInt(f[12], 10, 64)
	return time.Duration(u+s) * time.Second / clockTicks, nil
}

// memory is the process' proportional set size: shared libraries count in
// part.
func (p *process) memory() (int64, error) {
	return field(fmt.Sprintf("/proc/%d/smaps_rollup", p.pid), "Pss:")
}

// field reads a "Name: N kB" line of a /proc file, in bytes.
func field(file, name string) (int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), name); ok {
			kb, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimSpace(rest), " kB"), 10, 64)
			return kb << 10, err
		}
	}
	return 0, errProcess
}

func cpuAndMemory() (string, int64) {
	name := ""
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if k, v, ok := strings.Cut(sc.Text(), ":"); ok && strings.TrimSpace(k) == "model name" {
				name = strings.TrimSpace(v)
				break
			}
		}
		f.Close()
	}
	mem, _ := field("/proc/meminfo", "MemTotal:")
	return name, mem
}

// compositors are the processes that composite a Linux desktop.
var compositors = []string{"gnome-shell", "kwin_wayland", "kwin_x11", "Xorg", "Xwayland", "mutter", "weston", "sway", "Hyprland"}

// compositorCPU is the CPU time of the desktop's compositors and X
// servers.
func compositorCPU() (time.Duration, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	var total time.Duration
	found := false
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil || !slices.Contains(compositors, strings.TrimSpace(string(comm))) {
			continue
		}
		if d, err := (&process{pid: pid}).cpu(); err == nil {
			total += d
			found = true
		}
	}
	if !found {
		return 0, errProcess
	}
	return total, nil
}
