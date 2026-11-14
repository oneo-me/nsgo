package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// windowsCommandSource is a console-subsystem companion for a GUI app. A
// .com file wins Windows' command lookup before the same-name .exe, while the
// GUI executable remains the one Explorer and the Start menu launch.
//
// The app, a GUI program, is attached to no console, so the console's Ctrl+C, Ctrl+Break and close reach the companion alone: the companion ends the app before it exits, and a job that kills its processes as its last handle closes ends the app when the companion is killed outright. The job lets the app's own children break away, so an app that restarts itself or runs its updater's installer keeps them.
const windowsCommandSource = `package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"unsafe"
)

// statusControlCExit is STATUS_CONTROL_C_EXIT, the code a console program ends with on Ctrl+C.
const statusControlCExit = 0xC000013A

// Job objects, as winnt.h declares them.
const (
	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitSilentBreakawayOK        = 0x1000
	jobObjectLimitKillOnJobClose           = 0x2000
	processSetQuota                        = 0x0100
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                [6]uint64
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	createJobObject          = kernel32.NewProc("CreateJobObjectW")
	setInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
)

// killWithCompanion puts the process in a job that Windows ends as the companion's handle to it closes, when the companion exits or is killed. Without one, which an old system's job may refuse, the companion still ends the app on the console's events.
func killWithCompanion(pid int) {
	job, _, _ := createJobObject.Call(0, 0)
	if job == 0 {
		return
	}
	var info jobObjectExtendedLimitInformation
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose | jobObjectLimitSilentBreakawayOK
	if ok, _, _ := setInformationJobObject.Call(job, jobObjectExtendedLimitInformationClass, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); ok == 0 {
		syscall.CloseHandle(syscall.Handle(job))
		return
	}
	process, err := syscall.OpenProcess(processSetQuota|syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		syscall.CloseHandle(syscall.Handle(job))
		return
	}
	defer syscall.CloseHandle(process)
	if ok, _, _ := assignProcessToJobObject.Call(job, uintptr(process)); ok == 0 {
		syscall.CloseHandle(syscall.Handle(job))
	}
}

func main() {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	target := filepath.Join(filepath.Dir(self), filepath.Base(self[:len(self)-len(filepath.Ext(self))])+".exe")
	command := exec.Command(target, os.Args[1:]...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Ctrl+C and Ctrl+Break arrive as os.Interrupt, the console's close, logoff and shutdown as SIGTERM.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	if err := command.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	killWithCompanion(command.Process.Pid)
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err = <-done:
	case <-signals:
		command.Process.Kill()
		os.Exit(statusControlCExit)
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

// buildWindowsCommand writes the console companion beside p.Name.exe. It is
// a separate Go program so the app keeps the GUI subsystem and Explorer never
// opens a console for the normal executable.
func buildWindowsCommand(ctx context.Context, p *project, dir, arch string) error {
	temp, err := os.MkdirTemp("", "nsgo-windows-command-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	source := filepath.Join(temp, "main.go")
	if err := os.WriteFile(source, []byte(windowsCommandSource), 0o644); err != nil {
		return err
	}
	output := filepath.Join(dir, p.Name+".com")
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-buildvcs=false", "-o", output, "main.go")
	cmd.Dir = temp
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+arch, "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build Windows command companion failed: %w\n%s", err, out)
	}
	return nil
}
