package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWidgetsConfig(t *testing.T) {
	for _, c := range []struct {
		yaml, err string
	}{
		{"widgets: [{kind: counter, families: [small, extra_large, inline]}]", ""},
		{"widgets: [{kind: a/b}]", "is not a kind"},
		{"widgets: [{kind: a}, {kind: a}]", "listed twice"},
		{"widgets: [{kind: a, families: [huge]}]", "is none of"},
		{"apple: {app_group: shared}\nwidgets: [{kind: a}]", "is not an app group"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: app\n"+c.yaml+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := load(dir)
		if c.err == "" && err != nil || c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s: %v, want %q", c.yaml, err, c.err)
		}
	}
}

func TestKindsSwift(t *testing.T) {
	p := &project{ID: "dev.example.app", Widgets: []widgetKind{
		{Kind: "counter", Name: `The "count"`, Families: []string{"small", "circular"}},
		{Kind: "agenda"},
	}}
	src := kindsSwift(p)
	for _, want := range []string{
		`StaticConfiguration(kind: "counter", provider: NSGoProvider(kind: "counter"))`,
		`.configurationDisplayName(Text(verbatim: "The \"count\""))`,
		`.supportedFamilies(families(["small", "circular"]))`,
		`.supportedFamilies(families(["small", "medium", "large"]))`,
		"        NSGoWidget0()\n        NSGoWidget1()\n",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Kinds.swift lacks %s:\n%s", want, src)
		}
	}
	if got := p.widgetPlist(true); got != "<key>NSGoAppGroup</key><string>group.dev.example.app</string>" {
		t.Errorf("iOS plist = %s", got)
	}
	if got := p.widgetPlist(false); got != "<key>NSGoWidgetsPath</key><string>Library/Application Support/dev.example.app/Widgets</string>" {
		t.Errorf("macOS plist = %s", got)
	}
	if (&project{}).widgetPlist(true) != "" {
		t.Error("an app without widgets says where their timelines are")
	}
}

// TestWidgetsCompile compiles the extension and the reload library for
// this Mac, which the Swift sources need to keep doing.
func TestWidgetsCompile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("needs Xcode")
	}
	if err := exec.Command("xcrun", "--find", "swiftc").Run(); err != nil {
		t.Skip("needs Xcode's swiftc")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // os.UserCacheDir reads it on macOS
	p := &project{ID: "dev.example.app", Widgets: []widgetKind{{Kind: "counter"}}}
	exe, lib, err := compileWidgets(t.Context(), p, macWidgetTarget(runtime.GOARCH))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{exe, lib} {
		if _, err := os.Stat(f); err != nil {
			t.Error(err)
		}
	}
	out, err := exec.Command("nm", "-gU", lib).Output()
	if err != nil || !strings.Contains(string(out), "_nsgo_widgets_reload") {
		t.Errorf("the reload library exports no nsgo_widgets_reload: %v", err)
	}
}
