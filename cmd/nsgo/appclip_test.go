package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppClipConfig(t *testing.T) {
	for _, c := range []struct {
		yaml, err string
	}{
		{"app_clip: {main: ./clip, domains: [example.com, '*.shop.example.com']}", ""},
		{"app_clip: {id: dev.other.clip}", "not under the app's ID"},
		{"app_clip: {minimum: '15.0'}", "from 16.0"},
		{"app_clip: {domains: ['https://example.com']}", "is not a domain"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: app\nid: dev.example.app\n"+c.yaml+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := load(dir)
		if c.err == "" && err != nil || c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s: %v, want %q", c.yaml, err, c.err)
		}
	}
}

func TestAppClipProject(t *testing.T) {
	dir := t.TempDir()
	yaml := "name: Cafe\nid: dev.example.cafe\napp_clip: {main: ./clip, minimum: '17', location: true}\nwidgets: [{kind: a}]\n"
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.IOSMinimum() != "15.0" || p.clipPlist() != "" {
		t.Errorf("the app is built as a clip: %s, %q", p.IOSMinimum(), p.clipPlist())
	}
	c := p.clipProject()
	if c.Name != "Cafe Clip" || c.ID != "dev.example.cafe.Clip" || c.Dir != filepath.Join(dir, "clip") {
		t.Errorf("clip = %s, %s, %s", c.Name, c.ID, c.Dir)
	}
	if c.AppClip != nil || c.Widgets != nil {
		t.Error("the clip has the app's clip or widgets")
	}
	if c.IOSMinimum() != "17.0" || p.AppClip.limit() != clipDigitalSize {
		t.Errorf("minimum %s, limit %d", c.IOSMinimum(), p.AppClip.limit())
	}
	want := "<key>NSAppClip</key><dict><key>NSAppClipRequestEphemeralUserNotification</key><false/><key>NSAppClipRequestLocationConfirmation</key><true/></dict>"
	if got := c.clipPlist(); got != want {
		t.Errorf("clip plist = %s", got)
	}
	// The simulator launches nothing claiming a developer entitlement ad hoc.
	for _, ents := range []map[string]any{p.simulatorEntitlements(), c.simulatorEntitlements()} {
		for k := range ents {
			if strings.HasPrefix(k, "com.apple.developer.") {
				t.Errorf("ad hoc entitlement %s", k)
			}
		}
	}
}

func TestXcodeFreeTeam(t *testing.T) {
	prefs := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>IDEProvisioningTeamByIdentifier</key><dict>
<key>A</key><array>
<dict><key>isFreeProvisioningTeam</key><true/><key>teamID</key><string>FREE123</string></dict>
<dict><key>isFreeProvisioningTeam</key><false/><key>teamID</key><string>PAID123</string></dict>
</array></dict></dict></plist>`)
	if !xcodeFreeTeam(prefs, "FREE123") || xcodeFreeTeam(prefs, "PAID123") || xcodeFreeTeam(prefs, "OTHER") {
		t.Error("personal teams are misread")
	}
}

func TestQRLines(t *testing.T) {
	lines := qrLines("https://cafe.example.com/table?n=12")
	// Version 3 at level M: 29 modules and a quiet zone of 4 each side.
	if len(lines) != (29+8+1)/2 {
		t.Fatalf("%d lines", len(lines))
	}
	for i, l := range lines {
		if n := strings.Count(l, "▀"); n != 29+8 {
			t.Errorf("line %d has %d modules", i, n)
		}
	}
	// The quiet zone's first line is white over white.
	if !strings.HasPrefix(lines[0], "\x1b[97;107m▀") {
		t.Errorf("first line starts %q", lines[0][:20])
	}
}
