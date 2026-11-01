//go:build nsgodev

package compiler

import (
	"bufio"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
)

// devServer is nsgo dev's server, set with -ldflags -X for targets that
// pass the app no environment, as nsgo's own; NSGO_DEV_SERVER overrides
// it, and on the web the page's origin is it.
var devServer = ""

// run runs bundle code under nsgo dev: each bundle the server compiles
// from a save replaces it, in a VM of its own, with no Go build.
func run(cx *core.Scope, entry, code string) core.Node {
	server := devServer
	if s := os.Getenv("NSGO_DEV_SERVER"); s != "" {
		server = s
	}
	if server == "" {
		server = pageOrigin()
	}
	if server == "" {
		return tsx.App(entry, code)(cx)
	}
	if hot(code) {
		// A hot bundle updates in place: what did not change keeps its
		// state.
		app, update := tsx.Hot(entry, code)
		go followBundles(server, entry, func(next string) { cx.Post(func() { update(next) }) })
		return app(cx)
	}
	current := core.NewSignal(cx, code)
	go followBundles(server, entry, func(next string) { cx.Post(func() { current.Set(next) }) })
	return core.Dynamic(func(cx *core.Scope) core.Node { return tsx.App(entry, current.Get())(cx) })
}

// hot reports whether a bundle was compiled for hot updates
// (CompileHot).
func hot(code string) bool { return strings.Contains(code[:min(len(code), 64)], "__nsgo_hot") }

// followBundles reads the server's events, reconnecting while it runs,
// and gives update each new bundle of entry.
func followBundles(server, entry string, update func(string)) {
	for {
		resp, err := http.Get(server + "/_nsgo/events")
		if err == nil {
			sc := bufio.NewScanner(resp.Body)
			sc.Buffer(make([]byte, 1<<16), 16<<20)
			for sc.Scan() {
				data, ok := strings.CutPrefix(sc.Text(), "data: ")
				if !ok || data != "bundle "+entry {
					continue
				}
				if b, err := http.Get(server + "/_nsgo/bundle?entry=" + url.QueryEscape(entry)); err == nil {
					code, err := io.ReadAll(b.Body)
					b.Body.Close()
					if err == nil && b.StatusCode == http.StatusOK {
						update(string(code))
					}
				}
			}
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
}
