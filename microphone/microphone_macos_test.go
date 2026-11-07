//go:build darwin && !ios

package microphone

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeclaresUsage(t *testing.T) {
	bundle := func(name, plist string) string {
		contents := filepath.Join(t.TempDir(), name+".app", "Contents")
		if err := os.MkdirAll(contents, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.Join(contents, "MacOS", name)
	}
	if !declaresUsage(bundle("With", "<key>NSMicrophoneUsageDescription</key>")) {
		t.Error("a declared description was missed")
	}
	if declaresUsage(bundle("Without", "<key>CFBundleName</key>")) {
		t.Error("an app without a description declares one")
	}
	if declaresUsage("/usr/local/bin/tool") {
		t.Error("a bare program declares a description")
	}
}
