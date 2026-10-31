//go:build nsgodev

package nsgo

import (
	"bytes"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/engine"
)

// devTrace is "1" in nsgo dev -trace's builds, set with -ldflags -X.
var devTrace = ""

// traceTo has every engine of the process send server its commits, each
// printed as nsgo test -trace prints a batch, numbered from the first.
// One goroutine posts what gathered while the last post was on its way,
// so the engine never waits for the network.
func traceTo(server string) {
	var (
		mu      sync.Mutex
		pending bytes.Buffer
		commits int
	)
	wake := make(chan struct{}, 1)
	engine.Trace = func(b *driver.MountBatch, took time.Duration) {
		mu.Lock()
		commits++
		driver.WriteTrace(&pending, "commit "+strconv.Itoa(commits), b, took)
		mu.Unlock()
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	go func() {
		for range wake {
			mu.Lock()
			body := bytes.Clone(pending.Bytes())
			pending.Reset()
			mu.Unlock()
			if resp, err := http.Post(server+"/_nsgo/trace", "text/plain", bytes.NewReader(body)); err == nil {
				resp.Body.Close()
			}
		}
	}()
}
