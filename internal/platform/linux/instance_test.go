//go:build linux && !android

package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstanceHandsOffReopenAndFiles(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", privateTempDir(t))
	first, already := claimInstance("dev.nsgo.instance-test", nil)
	if already || first == nil {
		t.Fatalf("first claim = (%v, %v), want owner", first, already)
	}
	defer first.close()

	got := make(chan []string, 1)
	second, already := claimInstance("dev.nsgo.instance-test", []string{"/tmp/example.txt"})
	if !already || second != nil {
		t.Fatalf("second claim = (%v, %v), want handoff", second, already)
	}
	first.setDelivery(func(paths []string) { got <- paths })

	select {
	case paths := <-got:
		if len(paths) != 1 || paths[0] != "/tmp/example.txt" {
			t.Fatalf("handoff paths = %v", paths)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handoff")
	}
}

// TestInstanceHandsOffInAFlatpak hands a launch over between two Flatpak instances, each with an $XDG_RUNTIME_DIR of its own whose app/$FLATPAK_ID is one shared directory, as Flatpak lays them out.
func TestInstanceHandsOffInAFlatpak(t *testing.T) {
	const id = "dev.nsgo.FlatpakTest"
	t.Setenv("FLATPAK_ID", id)
	// Short directories, not t.TempDir's, which name the test and would leave no room in the socket's address.
	root, err := os.MkdirTemp("", "fp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	shared, err := os.MkdirTemp(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(shared, id), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime := func() string {
		dir, err := os.MkdirTemp(root, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(shared, filepath.Join(dir, "app")); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	t.Setenv("XDG_RUNTIME_DIR", runtime())
	first, already := claimInstance(id, nil)
	if already || first == nil {
		t.Fatalf("first claim = (%v, %v), want owner", first, already)
	}
	defer first.close()
	if filepath.Dir(first.path) != filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "app", id) {
		t.Fatalf("the socket is %s, want one in app/%s", first.path, id)
	}

	got := make(chan []string, 1)
	first.setDelivery(func(paths []string) { got <- paths })
	t.Setenv("XDG_RUNTIME_DIR", runtime())
	second, already := claimInstance(id, []string{"/tmp/example.txt"})
	if !already || second != nil {
		second.close()
		t.Fatalf("a second instance's claim = (%v, %v), want handoff", second, already)
	}
	select {
	case paths := <-got:
		if len(paths) != 1 || paths[0] != "/tmp/example.txt" {
			t.Fatalf("handoff paths = %v", paths)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handoff")
	}

	// Without the app's directory, or with an app ID too long for the address beneath it, the socket is in $XDG_RUNTIME_DIR, which later Flatpaks share.
	long := "dev.nsgo." + strings.Repeat("Long", 20)
	if err := os.Mkdir(filepath.Join(shared, long), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, app := range []string{"dev.nsgo.Missing", long} {
		t.Setenv("FLATPAK_ID", app)
		if path, err := instancePath(app); err != nil || filepath.Dir(path) != os.Getenv("XDG_RUNTIME_DIR") {
			t.Fatalf("with FLATPAK_ID %s, instancePath = %q, %v, want one in $XDG_RUNTIME_DIR", app, path, err)
		}
	}
}

// TestInstanceReplacesAStaleSocket starts as the owner where a process that died left its socket, and where the owner before it closed.
func TestInstanceReplacesAStaleSocket(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", privateTempDir(t))
	path, err := instancePath("dev.nsgo.stale-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	first, already := claimInstance("dev.nsgo.stale-test", nil)
	if already || first == nil || first.listener == nil {
		t.Fatalf("claim over a stale socket = (%v, %v), want a listening owner", first, already)
	}
	first.close()
	second, already := claimInstance("dev.nsgo.stale-test", nil)
	if already || second == nil {
		t.Fatalf("claim after the owner closed = (%v, %v), want owner", second, already)
	}
	second.close()
}

// TestInstancePathIsPrivate keeps the socket and its lock where no other user can take or hold them: in $XDG_RUNTIME_DIR when it is private, else in a private directory of this user's in the temporary one, and nowhere when that directory is another's, open to others or a link.
func TestInstancePathIsPrivate(t *testing.T) {
	runtime := privateTempDir(t)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	path, err := instancePath("dev.nsgo.private-test")
	if err != nil || filepath.Dir(path) != runtime {
		t.Fatalf("instancePath = %q, %v, want one in %s", path, err, runtime)
	}

	tmp := privateTempDir(t)
	t.Setenv("TMPDIR", tmp)
	if err := os.Chmod(runtime, 0o755); err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(tmp, fmt.Sprintf("nsgo-%d", os.Getuid()))
	path, err = instancePath("dev.nsgo.private-test")
	if err != nil || filepath.Dir(path) != private {
		t.Fatalf("with a shared runtime directory, instancePath = %q, %v, want one in %s", path, err, private)
	}
	if info, err := os.Lstat(private); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("the private directory: %v, %v", info, err)
	}

	t.Setenv("XDG_RUNTIME_DIR", "")
	if err := os.Chmod(private, 0o777); err != nil {
		t.Fatal(err)
	}
	if path, err := instancePath("dev.nsgo.private-test"); err == nil {
		t.Fatalf("a directory open to others gave %q", path)
	}
	if err := os.Remove(private); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), private); err != nil {
		t.Fatal(err)
	}
	if path, err := instancePath("dev.nsgo.private-test"); err == nil {
		t.Fatalf("a link gave %q", path)
	}
}

// privateTempDir is a temporary directory only this user may enter, as $XDG_RUNTIME_DIR is: t.TempDir's are open to others to read.
func privateTempDir(t *testing.T) string {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}
