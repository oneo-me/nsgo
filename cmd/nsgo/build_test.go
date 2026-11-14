package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestExpandAllTargets(t *testing.T) {
	got, err := expandTargets([]string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	want := goAndXcodeTargets()
	if !slices.Equal(got, want) {
		t.Fatalf("expandTargets(all) = %v, want %v", got, want)
	}
	if slices.Contains(got, "android_x64") {
		t.Error("all includes android_x64, which needs an NDK")
	}
	for _, mp := range miniProgramTargets() {
		if slices.Contains(got, mp) {
			t.Errorf("all includes %s, which only some apps fit", mp)
		}
	}
}

func TestExpandTargetsDropsDuplicates(t *testing.T) {
	got, err := expandTargets([]string{"all", "web", "android_x64"})
	if err != nil {
		t.Fatal(err)
	}
	if want := len(targets) - len(miniProgramPlatforms); len(got) != want {
		t.Fatalf("expandTargets = %v, want %d targets: all but the mini programs", got, want)
	}
	if got[0] != "macos_arm64" || got[len(got)-1] != "android_x64" {
		t.Errorf("order: got %v, want macos_arm64 first and android_x64 last", got)
	}
	seen := make(map[string]bool, len(got))
	for _, target := range got {
		if seen[target] {
			t.Errorf("duplicate target %q", target)
		}
		seen[target] = true
	}
}

func TestExpandTargetsRejectsUnknown(t *testing.T) {
	if _, err := expandTargets([]string{"all", "plan9"}); err == nil || !strings.Contains(err.Error(), "all") {
		t.Fatalf("expandTargets = %v, want an error naming all", err)
	}
}

func TestLoadExpandsAllTargets(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: Test\nbuild:\n  targets: [all]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Build.Targets, goAndXcodeTargets()) {
		t.Errorf("build.targets: got %v, want every Go and Xcode target", p.Build.Targets)
	}
}

func TestSmallJSON(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("needs the go command")
	}
	t.Setenv("GOEXPERIMENT", "")
	for _, c := range []struct {
		name, src string
		small     bool
	}{
		{"v1", "package main\n\nimport \"encoding/json\"\n\nfunc main() { json.Marshal(1) }\n", true},
		{"v2", "package main\n\nimport \"encoding/json/v2\"\n\nfunc main() { json.Marshal(1) }\n", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module app\n\ngo 1.26\n"), 0o644)
			os.WriteFile(filepath.Join(dir, "main.go"), []byte(c.src), 0o644)
			env, err := smallJSON(t.Context(), &project{Dir: dir}, []string{"GOOS=js", "GOARCH=wasm"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if small := slices.Contains(env, "GOEXPERIMENT=nojsonv2"); small != c.small {
				t.Fatalf("env = %v, want nojsonv2 %v", env, c.small)
			}
		})
	}
	t.Setenv("GOEXPERIMENT", "jsonv2")
	if env, err := smallJSON(t.Context(), &project{Dir: t.TempDir()}, nil, nil); err != nil || len(env) != 0 {
		t.Fatalf("with GOEXPERIMENT set, env = %v, %v", env, err)
	}
}

func TestBuildJobs(t *testing.T) {
	p := &project{}
	for _, c := range []struct{ jobs, targets, want int }{
		{3, 5, 3},
		{8, 2, 2},
		{1, 4, 1},
	} {
		p.Build.Jobs = c.jobs
		if got, err := p.buildJobs(c.targets); err != nil || got != c.want {
			t.Errorf("build.jobs %d for %d targets: got %d, %v; want %d", c.jobs, c.targets, got, err, c.want)
		}
	}
	p.Build.Jobs = 0
	if got, _ := p.buildJobs(100); got != max(1, runtime.NumCPU()/2) {
		t.Errorf("by default: got %d, want half of %d CPUs", got, runtime.NumCPU())
	}
	p.Build.Jobs = -1
	if _, err := p.buildJobs(2); err == nil {
		t.Error("build.jobs -1: got no error")
	}
}

func TestBuildWindowsCommand(t *testing.T) {
	dir := t.TempDir()
	p := &project{Name: "tool"}
	if err := buildWindowsCommand(t.Context(), p, dir, "amd64"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "tool.com"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || string(data[:2]) != "MZ" {
		t.Fatalf("tool.com is not a Windows executable: header %q", data[:min(len(data), 2)])
	}
}
