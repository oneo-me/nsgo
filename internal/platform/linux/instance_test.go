//go:build linux && !android

package linux

import (
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
