//go:build linux && !android

package linux

import "testing"

func TestWebDownloadFilenameStaysInDownloadDirectory(t *testing.T) {
	tests := map[string]string{
		"report.pdf":       "report.pdf",
		"../../report.pdf": "report.pdf",
		"":                 "download",
		".":                "download",
		"..":               "download",
		"/":                "download",
	}
	for suggested, want := range tests {
		if got := webDownloadFilename(suggested); got != want {
			t.Errorf("webDownloadFilename(%q) = %q, want %q", suggested, got, want)
		}
	}
}
