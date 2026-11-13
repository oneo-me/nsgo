package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
	src := kindsSwift(p, false)
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

func TestLiveActivitiesConfig(t *testing.T) {
	p := &project{ID: "dev.example.app", LiveActivities: liveActivities{Enabled: true, FrequentUpdates: true}}
	p.Android.Permissions = []string{"android.permission.CAMERA", "android.permission.POST_NOTIFICATIONS"}
	if !p.iosExtension() {
		t.Error("an app with Live Activities has no extension")
	}
	if got := p.widgetPlist(true); !strings.Contains(got, "<key>NSGoAppGroup</key>") || !strings.Contains(got, "<key>NSSupportsLiveActivities</key><true/>") ||
		!strings.Contains(got, "<key>NSSupportsLiveActivitiesFrequentUpdates</key><true/>") {
		t.Errorf("iOS plist = %s", got)
	}
	if got := p.widgetPlist(false); got != "" {
		t.Errorf("macOS plist of an app without widgets = %s", got)
	}
	if src := kindsSwift(p, true); !strings.Contains(src, "NSGoActivityWidget()") {
		t.Errorf("Kinds.swift lacks the activities:\n%s", src)
	}
	if src := kindsSwift(p, false); strings.Contains(src, "NSGoActivityWidget") {
		t.Errorf("macOS's Kinds.swift has the activities:\n%s", src)
	}
	want := []string{"android.permission.CAMERA", "android.permission.POST_NOTIFICATIONS", "android.permission.POST_PROMOTED_NOTIFICATIONS"}
	if got := p.androidPermissions(); !slices.Equal(got, want) {
		t.Errorf("Android permissions = %v, want %v", got, want)
	}
	if !slices.Equal(p.Android.Permissions, want[:2]) {
		t.Errorf("android.permissions changed to %v", p.Android.Permissions)
	}
	p.Widgets = make([]widgetKind, maxWidgets)
	for i := range p.Widgets {
		p.Widgets[i].Kind = fmt.Sprint("w", i)
	}
	if err := p.validateWidgets(); err == nil {
		t.Errorf("%d kinds of widgets and Live Activities fit a bundle", maxWidgets)
	}
}

// TestWidgetsCompile compiles the extension and the library for this Mac,
// and for the iOS simulator with Live Activities and their intents' App
// Intents metadata, which the Swift sources need to keep doing.
func TestWidgetsCompile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("needs Xcode")
	}
	if err := exec.Command("xcrun", "--find", "swiftc").Run(); err != nil {
		t.Skip("needs Xcode's swiftc")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // os.UserCacheDir reads it on macOS
	p := &project{ID: "dev.example.app", Widgets: []widgetKind{{Kind: "counter"}}, LiveActivities: liveActivities{Enabled: true}}
	for _, c := range []struct {
		target  widgetTarget
		symbols []string
	}{
		{macWidgetTarget(runtime.GOARCH), []string{"_nsgo_widgets_reload"}},
		{iosWidgetTarget(true), []string{"_nsgo_widgets_reload", "_nsgo_activity_start", "_nsgo_activity_listen"}},
	} {
		w, err := compileWidgets(t.Context(), p, c.target)
		if err != nil {
			t.Fatal(err)
		}
		files := []string{w.exe, w.lib}
		if c.target.ios {
			files = append(files, filepath.Join(w.exeIntents, "extract.actionsdata"), filepath.Join(w.libIntents, "extract.actionsdata"))
		} else if w.exeIntents != "" || w.libIntents != "" {
			t.Errorf("macOS's widgets have App Intents: %+v", w)
		}
		for _, f := range files {
			if _, err := os.Stat(f); err != nil {
				t.Error(err)
			}
		}
		out, err := exec.Command("nm", "-gU", w.lib).Output()
		for _, sym := range c.symbols {
			if err != nil || !strings.Contains(string(out), sym) {
				t.Errorf("the %s library exports no %s: %v", c.target.sdk, sym, err)
			}
		}
		if !c.target.ios {
			continue
		}
		for _, f := range []string{w.exeIntents, w.libIntents} {
			data, err := os.ReadFile(filepath.Join(f, "extract.actionsdata"))
			if err != nil || !strings.Contains(string(data), `"NSGoActionIntent"`) {
				t.Errorf("%s lacks NSGoActionIntent: %v", f, err)
			}
		}
		// iOS 15 runs the widgets, without ActivityKit and App Intents.
		out, _ = exec.Command("otool", "-l", w.exe).Output()
		if !strings.Contains(string(out), "LC_LOAD_WEAK_DYLIB") {
			t.Error("the extension links ActivityKit and App Intents strongly")
		}
	}
}
