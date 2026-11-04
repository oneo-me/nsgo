package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHotReload(t *testing.T) {
	for _, tt := range []struct {
		yaml string
		want bool
	}{
		{"", true},
		{"  hot_reload: true\n", true},
		{"  hot_reload: false\n", false},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "nsgo.yaml")
		if err := os.WriteFile(path, []byte("name: Test\ndev:\n"+tt.yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.hotReload(); got != tt.want {
			t.Errorf("dev.hot_reload %q: got %v, want %v", tt.yaml, got, tt.want)
		}
	}
}

func TestAndroidPermissions(t *testing.T) {
	dir := t.TempDir()
	yaml := "name: Test\nandroid:\n  permissions: [REQUEST_INSTALL_PACKAGES, com.example.READ]\n"
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Android.Permissions; len(got) != 2 || got[0] != "android.permission.REQUEST_INSTALL_PACKAGES" || got[1] != "com.example.READ" {
		t.Fatalf("permissions = %v", got)
	}
}

func TestBuildOpen(t *testing.T) {
	for _, tt := range []struct {
		yaml string
		want bool
	}{
		{"", false},
		{"open: true\n", true},
		{"open: false\n", false},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "nsgo.yaml")
		if err := os.WriteFile(path, []byte("name: Test\nbuild:\n  "+tt.yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.openOutput(); got != tt.want {
			t.Errorf("build.open %q: got %v, want %v", tt.yaml, got, tt.want)
		}
	}
}

func TestIcons(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	load := func(yaml string) (*project, error) {
		if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: Test\n"+yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		return load(dir)
	}

	p, err := load("")
	if err != nil {
		t.Fatal(err)
	}
	if p.Icon.Apple != "" || p.Icon.PNG != "" {
		t.Errorf("without icons: got %+v", p.Icon)
	}

	write("assets/AppIcon.icns")
	write("assets/AppIcon.icon/icon.json")
	write("assets/Icon.png")
	p, err = load("")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "assets", "AppIcon.icon"); p.Icon.Apple != want {
		t.Errorf("default Apple icon: got %s, want %s", p.Icon.Apple, want)
	}
	if want := filepath.Join(dir, "assets", "Icon.png"); p.Icon.PNG != want {
		t.Errorf("default PNG icon: got %s, want %s", p.Icon.PNG, want)
	}

	write("art/App.icns")
	write("art/app.png")
	p, err = load("icon:\n  apple: art/App.icns\n  png: art/app.png\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "art", "App.icns"); p.Icon.Apple != want {
		t.Errorf("icon.apple: got %s, want %s", p.Icon.Apple, want)
	}

	if _, err := load("icon:\n  png: art/missing.png\n"); err == nil {
		t.Error("a missing icon.png loaded")
	}
	if _, err := load("icon:\n  apple: art/app.png\n"); err == nil {
		t.Error("a PNG loaded as icon.apple")
	}

	// nsgo dev takes the -dev defaults and dev.icon over icon.
	write("assets/AppIcon-dev.icon/icon.json")
	p, err = load("dev:\n  icon:\n    png: art/app.png\n")
	if err != nil {
		t.Fatal(err)
	}
	p.useDevIcons()
	if want := filepath.Join(dir, "assets", "AppIcon-dev.icon"); p.Icon.Apple != want {
		t.Errorf("default dev Apple icon: got %s, want %s", p.Icon.Apple, want)
	}
	if want := filepath.Join(dir, "art", "app.png"); p.Icon.PNG != want {
		t.Errorf("dev.icon.png: got %s, want %s", p.Icon.PNG, want)
	}
	if _, err := load("dev:\n  icon:\n    apple: art/app.png\n"); err == nil {
		t.Error("a PNG loaded as dev.icon.apple")
	}
}
