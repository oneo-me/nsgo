package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWalkMobileResourcesLeavesOutExcludedPaths(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"servers/linux/server", "servers.txt", "icons/a.png"} {
		path := filepath.Join(dir, resourcesDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := &project{Dir: dir}
	p.Mobile.ExcludeResources = []string{"servers/"}
	var got []string
	if err := walkMobileResources(p, func(rel string, _ []byte) error { got = append(got, rel); return nil }); err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	if want := []string{"icons/a.png", "servers.txt"}; !slices.Equal(got, want) {
		t.Fatalf("walked %q, want %q", got, want)
	}
}
