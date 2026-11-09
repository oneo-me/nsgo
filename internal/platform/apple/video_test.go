//go:build darwin

package apple

import (
	"testing"
	"time"
)

func TestLocate(t *testing.T) {
	for _, c := range []struct {
		source, root, url, path string
		ok                      bool
	}{
		{"asset:/clips/a.mp4", "/app/Resources", "", "/app/Resources/clips/a.mp4", true},
		{"asset:clips/a.mp4", "/app/Resources", "", "/app/Resources/clips/a.mp4", true},
		{"asset:/a.mp4", "", "", "", false},
		{"https://example.com/a.mp4", "/r", "https://example.com/a.mp4", "", true},
		{"file:///tmp/a.mp4", "/r", "file:///tmp/a.mp4", "", true},
		{"/tmp/a.mp4", "/r", "", "/tmp/a.mp4", true},
	} {
		url, path, ok := locate(c.source, c.root)
		if url != c.url || path != c.path || ok != c.ok {
			t.Errorf("locate(%q) = %q, %q, %v; want %q, %q, %v", c.source, url, path, ok, c.url, c.path, c.ok)
		}
	}
}

func TestLocal(t *testing.T) {
	for url, want := range map[string]bool{
		"":                          true,
		"file:///tmp/a.flac":        true,
		"FILE:///tmp/a.flac":        true,
		"https://example.com/a.mp4": false,
		"http://example.com/file":   false,
	} {
		if got := local(url); got != want {
			t.Errorf("local(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestCMTime(t *testing.T) {
	if got := cmTimeOf(1500 * time.Millisecond); got.Value != 900 || got.Timescale != 600 || got.Flags != 1 {
		t.Errorf("cmTimeOf(1.5s) = %+v", got)
	}
	if d := cmTimeOf(2 * time.Second).duration(); d != 2*time.Second {
		t.Errorf("duration = %v, want 2s", d)
	}
	if d := (cmTime{Value: 0, Timescale: 0, Flags: 1 | 0x10}).duration(); d != 0 {
		t.Errorf("indefinite duration = %v, want 0", d)
	}
}
