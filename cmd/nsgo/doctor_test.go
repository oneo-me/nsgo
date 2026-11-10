package main

import (
	"slices"
	"testing"
)

func TestDoctorVerdicts(t *testing.T) {
	mac := &machine{goos: "darwin", goarch: "arm64", goVersion: "go1.26.2", xcode: "Xcode 26.1",
		sdks:         map[string]bool{"iphoneos": true, "iphonesimulator": true, "appletvos": true, "appletvsimulator": true},
		simulators:   map[string]int{"iOS": 2},
		appleDevices: map[string]int{},
		adb:          "/sdk/platform-tools/adb", android: []androidDevice{{serial: "emulator-5554", abi: "arm64-v8a"}},
		miniTools: map[string]bool{"wechat": true}}
	linux := &machine{goos: "linux", goarch: "amd64", goVersion: "go1.27", linuxLibs: []string{"GTK 4"},
		sdks: map[string]bool{}, simulators: map[string]int{}, appleDevices: map[string]int{}, miniTools: map[string]bool{}}
	oldGo := &machine{goos: "windows", goarch: "amd64", goVersion: "go1.25.4"}

	for _, c := range []struct {
		m            *machine
		target       string
		build, run   bool
		runElsewhere bool
	}{
		{mac, "macos_arm64", true, true, false},
		{mac, "macos_x64", true, false, true},
		{mac, "windows_x64", true, false, true},
		{mac, "ios", true, false, false},
		{mac, "ios_simulator", true, true, false},
		{mac, "tvos_simulator", true, false, false},
		{mac, "watchos_simulator", false, false, false},
		{mac, "android_arm64", true, true, false},
		{mac, "android_x64", false, false, false},
		{mac, "web", true, true, false},
		{mac, "miniprogram_wechat", true, true, false},
		{mac, "miniprogram_qq", true, false, false},
		{linux, "linux_x64", true, false, false},
		{linux, "linux_arm64", true, false, true},
		{linux, "ios", false, false, true},
		{linux, "android_arm64", true, false, false},
		{linux, "miniprogram_wechat", true, false, true},
		{oldGo, "windows_x64", false, true, false},
	} {
		build, run := c.m.build(c.target), c.m.run(c.target)
		if build.ok != c.build || run.ok != c.run || run.elsewhere != c.runElsewhere {
			t.Errorf("%s on %s: build %+v, run %+v", c.target, c.m.goos, build, run)
		}
		if !build.ok && build.why == "" || !run.ok && run.why == "" {
			t.Errorf("%s on %s: a target it cannot build or run must say why", c.target, c.m.goos)
		}
	}
}

func TestParseAdbDevices(t *testing.T) {
	out := "List of devices attached\nemulator-5554\tdevice\nR58M\tunauthorized\n0A1B\toffline\nPX9\tdevice\n"
	if got := parseAdbDevices(out); !slices.Equal(got, []string{"emulator-5554", "PX9"}) {
		t.Errorf("parseAdbDevices = %q", got)
	}
}

func TestGTKNewEnough(t *testing.T) {
	for name, want := range map[string]bool{
		"libgtk-4.so.1.1400.2": true,
		"libgtk-4.so.1.1600.0": true,
		"libgtk-4.so.1.1200.5": false,
		"libgtk-4.so.1":        true,
	} {
		if got := gtkNewEnough(name); got != want {
			t.Errorf("gtkNewEnough(%q) = %v", name, got)
		}
	}
}

func TestOSRelease(t *testing.T) {
	if got := osRelease("NAME=\"Ubuntu\"\nPRETTY_NAME=\"Ubuntu 24.04.1 LTS\"\n"); got != "Ubuntu 24.04.1 LTS" {
		t.Errorf("osRelease = %q", got)
	}
}
