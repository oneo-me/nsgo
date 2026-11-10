package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
