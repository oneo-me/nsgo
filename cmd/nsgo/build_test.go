package main

import (
	"os"
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
	if slices.Contains(got, "miniprogram") {
		t.Error("all includes miniprogram, which only some apps fit")
	}
}

func TestExpandTargetsDropsDuplicates(t *testing.T) {
	got, err := expandTargets([]string{"all", "web", "android_x64"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(targets)-1 {
		t.Fatalf("expandTargets = %v, want %d targets: all but miniprogram", got, len(targets)-1)
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
