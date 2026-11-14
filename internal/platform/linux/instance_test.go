//go:build linux && !android

package linux

import (
	"os"
	"testing"
	"time"
)

func TestInstanceHandsOffReopenAndFiles(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
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
	t.Setenv("XDG_DATA_HOME", t.TempDir())
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
