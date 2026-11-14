//go:build linux && !android

package linux

import (
	"fmt"
	"os"
	"path/filepath"
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
