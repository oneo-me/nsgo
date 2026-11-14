//go:build darwin

package apple

import "testing"

func TestFileURLPathAndScope(t *testing.T) {
	for raw, want := range map[string]string{
		"file:///tmp/opened/a%20b/index.html":    "/tmp/opened/a b/index.html",
		"FILE://localhost/tmp/opened/index.html": "/tmp/opened/index.html",
	} {
		if got, ok := fileURLPath(raw); !ok || got != want {
			t.Errorf("fileURLPath(%q) = %q, %v; want %q, true", raw, got, ok, want)
		}
	}
	for raw, want := range map[string]bool{
		"file:///tmp/opened/a/index.html":       true,
		"file:///tmp/opened/a/sub/asset.css":    true,
		"file:///tmp/opened/a/../secret.txt":    false,
		"file:///tmp/opened/ab/secret.txt":      false,
		"file://server/tmp/opened/a/index.html": false,
		"https://example.com/a":                 false,
	} {
		if got := fileURLInDir(raw, "/tmp/opened/a"); got != want {
			t.Errorf("fileURLInDir(%q) = %v, want %v", raw, got, want)
		}
	}
}
