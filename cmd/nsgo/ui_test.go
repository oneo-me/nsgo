package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSizesAndTimesReadWell(t *testing.T) {
	for _, c := range []struct {
		size int64
		want string
	}{
		{500, "500 B"},
		{3255, "3.2 KB"},
		{3 << 20, "3.0 MB"},
	} {
		if got := formatSize(c.size); got != c.want {
			t.Errorf("formatSize(%d) = %q, want %q", c.size, got, c.want)
		}
	}
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{21 * time.Millisecond, "21ms"},
		{12_640 * time.Millisecond, "12.6s"},
		{63 * time.Second, "1m 3s"},
	} {
		if got := formatDuration(c.d); got != c.want {
			t.Errorf("formatDuration(%s) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestDirSizeAddsItsFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, n int) {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a", 100)
	write("b", 200)
	if got := dirSize(dir); got != 300 {
		t.Fatalf("dirSize = %d, want 300", got)
	}
}

// captureStderr runs f and returns what it wrote to stderr.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	f()
	w.Close()
	os.Stderr = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// The panel starts with the nsgo that builds it, the app's name and
// version, then the targets.
func TestHeaderNamesTheToolTheAppAndTheTargets(t *testing.T) {
	want := "NSGo v0.1.0 · Gallery 0.1.0 · macos_arm64, web"
	if got := headerLine("v0.1.0", "Gallery", "0.1.0", "macos_arm64, web"); got != want {
		t.Fatalf("headerLine = %q, want %q", got, want)
	}
	if got := headerLine("", "Gallery", "0.1.0", "web"); got != "NSGo · Gallery 0.1.0 · web" {
		t.Fatalf("headerLine without a version = %q", got)
	}
}

// A failed build prints its error, then the tool's output under it.
func TestReportPrintsTheToolOutputBelowTheError(t *testing.T) {
	got := captureStderr(t, func() {
		report(errors.New("macos_arm64: go build failed\n# gallery\napp.go:3: undefined: x"))
	})
	want := "  error macos_arm64: go build failed\n    # gallery\n    app.go:3: undefined: x\n"
	if got != want {
		t.Fatalf("report wrote %q, want %q", got, want)
	}
}

func TestReportPrintsOneLineErrors(t *testing.T) {
	got := captureStderr(t, func() { report(errors.New("nsgo.yaml: name is required")) })
	if want := "  error nsgo.yaml: name is required\n"; got != want {
		t.Fatalf("report wrote %q, want %q", got, want)
	}
}
