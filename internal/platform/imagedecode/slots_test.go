package imagedecode

import (
	"context"
	"testing"
)

// Wait hands out as many slots as processors; the next waits until one
// comes back, and gives up when its context ends.
func TestWaitBoundsDecodes(t *testing.T) {
	var dones []func()
	for range cap(slots) {
		done, err := Wait(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		dones = append(dones, done)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Wait(ctx); err != context.Canceled {
		t.Fatalf("Wait with every slot taken = %v, want %v", err, context.Canceled)
	}
	dones[0]()
	done, err := Wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done()
	for _, done := range dones[1:] {
		done()
	}
}
