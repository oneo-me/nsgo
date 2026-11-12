// Package imageloadtest checks a backend's imageload.HTTP against a
// local server, so each backend's tests hold its system client to the
// same behavior: status, headers both ways, the body, progress and
// cancelling.
package imageloadtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

// CheckHTTP runs get's checks.
func CheckHTTP(t *testing.T, get func(ctx context.Context, url string, header imageload.Header, progress imageload.Progress) (*imageload.Response, error)) {
	t.Helper()
	body := make([]byte, 256<<10)
	for i := range body {
		body[i] = byte(i)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			<-r.Context().Done()
		case "/missing":
			http.NotFound(w, r)
		case "/big":
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Write(body)
		default:
			w.Header().Set("ETag", `"v1"`)
			w.Header().Add("Cache-Control", "max-age=60")
			w.Write([]byte(r.Header.Get("If-None-Match") + " png"))
		}
	}))
	defer srv.Close()

	resp, err := get(context.Background(), srv.URL+"/a.png?x=1", imageload.Header{"If-None-Match": {`"v0"`}}, nil)
	if err != nil || resp.Status != 200 || string(resp.Body) != `"v0" png` || resp.Header.Get("Etag") != `"v1"` ||
		resp.Header.Get("cache-control") != "max-age=60" {
		t.Fatalf("get = %+v, %v", resp, err)
	}
	var loaded, total int64
	resp, err = get(context.Background(), srv.URL+"/big", imageload.Header{}, func(n, of int64) { loaded, total = n, of })
	if err != nil || resp.Status != 200 || len(resp.Body) != len(body) || resp.Body[len(body)-1] != body[len(body)-1] ||
		loaded != int64(len(body)) || total != int64(len(body)) {
		t.Fatalf("a big body = %d bytes, %v, progress %d of %d", len(resp.Body), err, loaded, total)
	}
	if resp, err := get(context.Background(), srv.URL+"/missing", imageload.Header{}, nil); err != nil || resp.Status != 404 {
		t.Fatalf("a missing image = %+v, %v", resp, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := get(ctx, srv.URL+"/slow", imageload.Header{}, nil); err != context.DeadlineExceeded || time.Since(start) > 5*time.Second {
		t.Fatalf("a cancelled download = %v after %v", err, time.Since(start))
	}
}
