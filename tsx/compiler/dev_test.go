//go:build nsgodev

package compiler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx/bundler"
)

// TestDevServerBundlesReplaceTheApp serves a bundle as nsgo dev does
// after a save: the running app becomes it, with no build.
func TestDevServerBundlesReplaceTheApp(t *testing.T) {
	next, err := bundler.Compile(fstest.MapFS{"App.tsx": {Data: []byte(`import { mount, Text } from "nsgo";
mount(() => <Text>Second</Text>);`)}}, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	send := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/_nsgo/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: hello\n\n")
		w.(http.Flusher).Flush()
		select {
		case msg := <-send:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/_nsgo/bundle", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, next) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	defer srv.CloseClientConnections()
	t.Setenv("NSGO_DEV_SERVER", srv.URL)

	app := nsgotest.Start(t, App(fstest.MapFS{"App.tsx": {Data: []byte(`import { mount, Text } from "nsgo";
mount(() => <Text>First</Text>);`)}}, "App.tsx"))
	app.WaitFor("text:First", time.Second)
	send <- "bundle App.tsx"
	app.WaitFor("text:Second", 2*time.Second)
}
