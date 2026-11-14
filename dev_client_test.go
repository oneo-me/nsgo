//go:build nsgodev

package nsgo

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// Run starts DevTools before any graph, so an app's function that shows its view with SetMainView has its trees registered and its commits sent, and SetMainView, however often it is called, starts nothing again.
func TestDevClientStartsDevToolsOnceBeforeTheGraph(t *testing.T) {
	var mu sync.Mutex
	commits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_nsgo/devtools/out" {
			return
		}
		sc := bufio.NewScanner(r.Body)
		for sc.Scan() {
			if strings.Contains(sc.Text(), `"type":"commit"`) {
				mu.Lock()
				commits++
				mu.Unlock()
			}
		}
	}))
	defer server.Close()
	devServer, devDevtools = server.URL, "1"
	t.Cleanup(func() {
		devServer, devDevtools = "", ""
		engine.Trace, driver.Applied = nil, nil
		devtools.Enabled, driver.TraceCauses = false, false
	})

	// As Run does before it starts the engine.
	startDevClient()
	if !devtools.Enabled || engine.Trace == nil {
		t.Fatalf("after startDevClient devtools.Enabled = %v and engine.Trace is set: %v", devtools.Enabled, engine.Trace != nil)
	}
	root := core.NewRoot(core.RootOptions{Post: func(func()) {}})
	core.NewTree(root, core.ApproxTextEngine{})
	defer root.Dispose()
	if len(devtools.Surfaces()) == 0 {
		t.Fatal("a tree made after startDevClient did not register with DevTools")
	}

	// The app's function shows a view, then another: each is wrapped, nothing starts again.
	view := func(*core.Scope) core.Node { return core.View{} }
	devApp(view)
	devApp(view)
	engine.Trace(&driver.MountBatch{}, time.Millisecond)
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return commits
	}
	for deadline := time.Now().Add(5 * time.Second); count() == 0 && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
	}
	// The client posts a quarter of a second's messages at once: another wrapper's would have come with them.
	time.Sleep(500 * time.Millisecond)
	if n := count(); n != 1 {
		t.Fatalf("one commit reached the page %d times", n)
	}
}
