//go:build windows

package windows

import "testing"

func TestWebRequestHeadersAreLines(t *testing.T) {
	got := webRequestHeaders(map[string]string{"X-B": "2", "X-A": "1", "Bad\r\nName": "x", "X-C": "a\r\nInjected: y", "": "z"})
	if want := "X-A: 1\r\nX-B: 2"; got != want {
		t.Errorf("webRequestHeaders = %q, want %q", got, want)
	}
	if got := webRequestHeaders(nil); got != "" {
		t.Errorf("webRequestHeaders(nil) = %q", got)
	}
}

func TestWebDownloadFilenameStaysInDownloadDirectory(t *testing.T) {
	for _, test := range []struct {
		path, want string
	}{
		{`C:\Users\me\Downloads\report.pdf`, "report.pdf"},
		{`..\..\secret.txt`, "secret.txt"},
		{`..`, "download"},
		{`C:\`, "download"},
	} {
		if got := webDownloadFilename(test.path); got != test.want {
			t.Errorf("webDownloadFilename(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}
