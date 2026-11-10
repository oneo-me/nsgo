package imageload

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// server answers with body and the headers set returns, counting requests
// and the validators they carry.
type server struct {
	*httptest.Server
	hits, revalidations atomic.Int32
	status              atomic.Int32
	headers             atomic.Value // func(http.Header)
}

func newServer(t *testing.T, body string) *server {
	s := &server{}
	s.status.Store(http.StatusOK)
	s.headers.Store(func(http.Header) {})
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		s.headers.Load().(func(http.Header))(w.Header())
		if r.Header.Get("If-None-Match") != "" && r.Header.Get("If-None-Match") == w.Header().Get("Etag") {
			s.revalidations.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(int(s.status.Load()))
		w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func TestDiskCacheFreshness(t *testing.T) {
	s := newServer(t, "png bytes")
	s.headers.Store(func(h http.Header) { h.Set("Cache-Control", "max-age=60"); h.Set("Etag", `"v1"`) })
	c := newDiskCache(t.TempDir(), 1<<20)
	now := time.Now()
	c.now = func() time.Time { return now }
	ctx := context.Background()
	for range 2 {
		b, err := c.fetch(ctx, s.URL+"/a.png", nil)
		if err != nil || string(b) != "png bytes" {
			t.Fatalf("fetch = %q, %v", b, err)
		}
	}
	if s.hits.Load() != 1 {
		t.Fatalf("a fresh copy was fetched %d times", s.hits.Load())
	}
	// Stale, it revalidates with its ETag, and a 304 refreshes it.
	now = now.Add(2 * time.Minute)
	if b, err := c.fetch(ctx, s.URL+"/a.png", nil); err != nil || string(b) != "png bytes" || s.revalidations.Load() != 1 {
		t.Fatalf("revalidated %q, %v, %d", b, err, s.revalidations.Load())
	}
	c.fetch(ctx, s.URL+"/a.png", nil)
	if s.hits.Load() != 2 {
		t.Fatalf("the refreshed copy was not served: %d requests", s.hits.Load())
	}
	// Offline, a stale copy still shows.
	now = now.Add(time.Hour)
	s.Close()
	if b, err := c.fetch(ctx, s.URL+"/a.png", nil); err != nil || string(b) != "png bytes" {
		t.Fatalf("offline = %q, %v", b, err)
	}
	// Another handle on the same directory, as a new launch, reads it.
	again := newDiskCache(c.dir, 1<<20)
	again.now = c.now
	if e := again.read(s.URL + "/a.png"); e == nil || string(e.body) != "png bytes" {
		t.Fatalf("a new launch reads %+v", e)
	}
}

func TestDiskCachePolicy(t *testing.T) {
	ctx := context.Background()
	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name    string
		headers map[string]string
		stored  bool
		fresh   time.Duration // fresh for this long after storing
	}{
		{"no-store", map[string]string{"Cache-Control": "no-store, max-age=60"}, false, 0},
		{"vary star", map[string]string{"Vary": "*", "Cache-Control": "max-age=60"}, false, 0},
		{"no-cache", map[string]string{"Cache-Control": "no-cache"}, true, 0},
		{"expires", map[string]string{"Date": date.Format(http.TimeFormat),
			"Expires": date.Add(time.Hour).Format(http.TimeFormat)}, true, time.Hour},
		{"heuristic", map[string]string{"Date": date.Format(http.TimeFormat),
			"Last-Modified": date.Add(-10 * time.Hour).Format(http.TimeFormat)}, true, time.Hour},
		{"heuristic cap", map[string]string{"Date": date.Format(http.TimeFormat),
			"Last-Modified": date.Add(-1000 * 24 * time.Hour).Format(http.TimeFormat)}, true, maxHeuristic},
		{"age", map[string]string{"Cache-Control": "max-age=600", "Age": "540"}, true, time.Minute},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newServer(t, "x")
			s.headers.Store(func(h http.Header) {
				for k, v := range c.headers {
					h.Set(k, v)
				}
			})
			dc := newDiskCache(t.TempDir(), 1<<20)
			dc.now = func() time.Time { return date }
			if _, err := dc.fetch(ctx, s.URL, nil); err != nil {
				t.Fatal(err)
			}
			e := dc.read(s.URL)
			if (e != nil) != c.stored {
				t.Fatalf("stored = %v", e != nil)
			}
			if e == nil {
				return
			}
			if got := e.fresh(date.Add(c.fresh - time.Second)); got != (c.fresh > 0) {
				t.Fatalf("fresh before %v = %v", c.fresh, got)
			}
			if e.fresh(date.Add(c.fresh + time.Second)) {
				t.Fatalf("fresh after %v", c.fresh)
			}
		})
	}
}

func TestDiskCacheFailures(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, "x")
	s.headers.Store(func(h http.Header) { h.Set("Cache-Control", "max-age=0") })
	dc := newDiskCache(t.TempDir(), 1<<20)
	if _, err := dc.fetch(ctx, s.URL, nil); err != nil {
		t.Fatal(err)
	}
	// A 5xx serves the stale copy; a 4xx drops it.
	s.status.Store(http.StatusServiceUnavailable)
	if b, err := dc.fetch(ctx, s.URL, nil); err != nil || string(b) != "x" {
		t.Fatalf("5xx = %q, %v", b, err)
	}
	s.status.Store(http.StatusNotFound)
	if _, err := dc.fetch(ctx, s.URL, nil); err == nil || dc.read(s.URL) != nil {
		t.Fatalf("4xx = %v, kept %v", err, dc.read(s.URL) != nil)
	}
	// must-revalidate copies are never served stale.
	s.status.Store(http.StatusOK)
	s.headers.Store(func(h http.Header) { h.Set("Cache-Control", "max-age=0, must-revalidate") })
	dc.fetch(ctx, s.URL, nil)
	s.status.Store(http.StatusBadGateway)
	if _, err := dc.fetch(ctx, s.URL, nil); err == nil {
		t.Fatal("a must-revalidate copy was served stale")
	}
	// A broken entry is a miss, and deleted.
	path := dc.path(s.URL)
	os.WriteFile(path, []byte("NSGOIMG1garbage"), 0o644)
	if dc.read(s.URL) != nil {
		t.Fatal("a broken entry was read")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("a broken entry was kept")
	}
}

func TestDiskCacheTrims(t *testing.T) {
	dir := t.TempDir()
	dc := newDiskCache(dir, 1<<20)
	dc.scanned = true // no trims while writing
	now := time.Now()
	dc.now = func() time.Time { return now }
	old := filepath.Join(dir, "tmp-left")
	os.WriteFile(old, []byte("x"), 0o644)
	os.Chtimes(old, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
	for i := range 10 {
		e := &cached{url: "https://example.com/" + string(rune('a'+i)), stored: now, header: Header{},
			body: []byte(strings.Repeat("x", 450))}
		dc.write(e)
		os.Chtimes(dc.path(e.url), now.Add(time.Duration(i)*time.Second), now.Add(time.Duration(i)*time.Second))
	}
	dc.limit = 4000
	dc.trim()
	if dc.size > 4000*3/4 {
		t.Fatalf("size %d after trimming", dc.size)
	}
	if dc.read("https://example.com/a") != nil || dc.read("https://example.com/j") == nil {
		t.Fatal("trimming kept the oldest or dropped the newest")
	}
	if _, err := os.Stat(old); err == nil {
		t.Fatal("an old temporary file was kept")
	}
	// Images larger than an eighth of the cache are not kept.
	big := &cached{url: "https://example.com/big", header: Header{}, body: make([]byte, 600)}
	if big.storable(4000) {
		t.Fatal("an image over an eighth of the cache is kept")
	}
}
