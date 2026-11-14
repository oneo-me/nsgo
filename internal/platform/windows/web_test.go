//go:build windows

package windows

import "testing"

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
