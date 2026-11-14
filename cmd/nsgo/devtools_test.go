package main

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// events follows an event stream at url, handing each data line to a channel.
func events(t *testing.T, url string) <-chan string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	out := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				out <- data
			}
		}
	}()
	return out
}

func next(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
		return ""
	}
}

func TestDevToolsServerPassesMessagesBothWays(t *testing.T) {
	s, err := startDevServer(0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	base := s.url()

	resp, err := http.Get(base + "/_nsgo/devtools/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), "NSGo DevTools") || !strings.Contains(string(page), "devtools.js") {
		t.Fatalf("the page is %q", page)
	}

	// The app's messages reach every page, one event a line.
	pages := events(t, base+"/_nsgo/devtools/stream")
	if got := next(t, pages); got != `{"type":"connected"}` {
		t.Fatalf("the page's first event = %q", got)
	}
	resp, err = http.Post(base+"/_nsgo/devtools/out", "application/x-ndjson", strings.NewReader("{\"type\":\"commit\"}\n\n{\"type\":\"hot\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if a, b := next(t, pages), next(t, pages); a != `{"type":"commit"}` || b != `{"type":"hot"}` {
		t.Fatalf("pages heard %q and %q", a, b)
	}

	// A page's command reaches the app as one devtools event.
	app := events(t, base+"/_nsgo/events")
	// The server greets an app once it follows its events.
	if got := next(t, app); got != "hello" {
		t.Fatalf("the app's first event = %q", got)
	}
	resp, err = http.Post(base+"/_nsgo/devtools/command", "application/json", strings.NewReader("{\n  \"op\": \"elements\",\n  \"surface\": 0\n}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := next(t, app); got != `devtools {"op":"elements","surface":0}` {
		t.Fatalf("the app heard %q", got)
	}

	// Another site's page cannot drive the app or speak for it: it sends neither JSON without asking first, which the server never allows, nor its own origin.
	for _, c := range []struct {
		method, path, body, contentType, origin string
		want                                    int
	}{
		{"GET", "/_nsgo/devtools/command", "", "", "", http.StatusMethodNotAllowed},
		{"POST", "/_nsgo/devtools/command", "not json", "application/json", "", http.StatusBadRequest},
		{"GET", "/_nsgo/devtools/out", "", "", "", http.StatusMethodNotAllowed},
		{"POST", "/_nsgo/devtools/command", `{"op":"pick","on":true}`, "text/plain", "", http.StatusUnsupportedMediaType},
		{"POST", "/_nsgo/devtools/command", `{"op":"pick","on":true}`, "application/json", "https://evil.example", http.StatusForbidden},
		{"POST", "/_nsgo/devtools/command", `{"op":"surfaces"}`, "application/json; charset=utf-8", base, http.StatusOK},
		{"POST", "/_nsgo/devtools/out", `{"type":"step"}`, "text/plain", "", http.StatusUnsupportedMediaType},
		{"POST", "/_nsgo/devtools/out", `{"type":"step"}`, "application/x-ndjson", "null", http.StatusForbidden},
	} {
		req, _ := http.NewRequest(c.method, base+c.path, strings.NewReader(c.body))
		if c.contentType != "" {
			req.Header.Set("Content-Type", c.contentType)
		}
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("%s %s %q from %q = %d, want %d", c.method, c.path, c.body, c.origin, resp.StatusCode, c.want)
		}
	}
}

// A phone is told the server's first address, which is the one the machine reaches other hosts through, not a virtual network's that comes first among the interfaces.
func TestDevServerTellsPhonesTheOutboundAddressFirst(t *testing.T) {
	out := outboundIP()
	if out == nil || !out.IsPrivate() {
		t.Skip("no private outbound address here")
	}
	s := &devServer{port: 8080}
	urls := s.lanURLs()
	if len(urls) == 0 || urls[0] != "http://"+out.String()+":8080" {
		t.Fatalf("lanURLs = %v, want %s first", urls, out)
	}
}
