package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestInitTemplates writes each template's app and runs its tests, Go's
// and TSX's, against this checkout of NSGo.
func TestInitTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip("builds three apps")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range templates {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "app")
			if err := initApp(dir, "My App", "example.com/myapp", kind); err != nil {
				t.Fatal(err)
			}
			if err := initApp(dir, "My App", "", kind); err == nil {
				t.Error("init wrote over the app")
			}
			run := func(args ...string) {
				t.Helper()
				cmd := exec.Command("go", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %v: %v\n%s", args, err, out)
				}
			}
			run("mod", "edit", "-require=github.com/oneo-me/nsgo@v0.0.0", "-replace=github.com/oneo-me/nsgo="+root)
			run("mod", "tidy")
			p, err := load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if kind != "go" {
				tsxTest, err := p.tsxTestFile()
				if err != nil || tsxTest == "" {
					t.Fatalf("no TSX test: %v", err)
				}
				ov, err := p.overlay(map[string]string{"zz_nsgo_tsx_test.go": tsxTest})
				if err != nil {
					t.Fatal(err)
				}
				run("test", "-overlay", ov, "-run", "TestTSX", ".")
				return
			}
			run("test", ".")
		})
	}
}
