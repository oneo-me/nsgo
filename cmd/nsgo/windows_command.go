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
const windowsCommandSource = `package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	target := filepath.Join(filepath.Dir(self), filepath.Base(self[:len(self)-len(filepath.Ext(self))])+".exe")
	command := exec.Command(target, os.Args[1:]...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
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
