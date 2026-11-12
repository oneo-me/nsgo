package lucide

import (
	"sync"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestAllIcons(t *testing.T) {
	if len(allIcons) == 0 {
		t.Fatal("empty icon collection")
	}
	for _, icon := range allIcons {
		t.Run(icon.name, func(t *testing.T) {
			t.Parallel()
			results := make([]*core.SVG, 8)
			var wg sync.WaitGroup
			for i := range results {
				wg.Go(func() { results[i] = icon.get() })
			}
			wg.Wait()
			first := results[0]
			if first == nil {
				t.Fatal("nil SVG")
			}
			for _, result := range results {
				if result != first {
					t.Fatal("accessor did not reuse its SVG")
				}
			}
			if w, h := first.Size(); w != 24 || h != 24 {
				t.Fatalf("size = %gx%g", w, h)
			}
		})
	}
}

func TestLazyIcon(t *testing.T) {
	var cache lazyIcon
	if cache.svg != nil {
		t.Fatal("initialized before use")
	}
	first := cache.get(`<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/></svg>`)
	if cache.get("not parsed again") != first {
		t.Fatal("not cached")
	}
}
