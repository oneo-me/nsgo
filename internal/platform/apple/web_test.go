//go:build darwin

package apple

import (
	"testing"

	"github.com/ebitengine/purego/objc"
)

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

func TestWebKitHandlesItsOwnSchemes(t *testing.T) {
	loadWebKit()
	handles := func(name string) bool {
		return objc.Send[bool](objc.ID(Class("WKWebView")), sel("handlesURLScheme:"), NSString(name))
	}
	// core's webBuiltinSchemes refuses these, as WebKit would end the app.
	for _, name := range []string{"ws", "wss", "javascript", "ftp", "applewebdata", "webkit-fake-url", "blob", "about", "data", "file", "http", "https"} {
		if !handles(name) {
			t.Errorf("WKWebView does not handle %q", name)
		}
	}
	if handles("gallery") {
		t.Error("WKWebView handles an app's scheme")
	}
}
