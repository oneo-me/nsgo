package imageload

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// The tests download through net/http, standing in for the system's
// client a backend sets.
func init() { HTTP = netHTTP }

func netHTTP(ctx context.Context, url string, header Header, progress Progress) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ReadAll(resp.Body, resp.ContentLength, progress)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Header: Header(resp.Header), Body: body}, nil
}

func TestHeaderKeys(t *testing.T) {
	for in, want := range map[string]string{"etag": "Etag", "ETag": "Etag", "cache-control": "Cache-Control",
		"LAST-MODIFIED": "Last-Modified", " vary ": "Vary"} {
		if got := CanonicalKey(in); got != want {
			t.Errorf("CanonicalKey(%q) = %q, want %q", in, got, want)
		}
	}
	h := Header{}
	h.Add("cache-control", "max-age=1")
	h.Add("Cache-Control", "public")
	if h.Get("CACHE-CONTROL") != "max-age=1" || len(h.Values("cache-control")) != 2 {
		t.Fatalf("header = %v", h)
	}
}

func TestParseTime(t *testing.T) {
	want := time.Date(1994, 11, 6, 8, 49, 37, 0, time.UTC)
	for _, s := range []string{"Sun, 06 Nov 1994 08:49:37 GMT", "Sunday, 06-Nov-94 08:49:37 GMT", "Sun Nov  6 08:49:37 1994"} {
		if got, err := parseTime(s); err != nil || !got.Equal(want) {
			t.Errorf("parseTime(%q) = %v, %v", s, got, err)
		}
	}
	if _, err := parseTime("yesterday"); err == nil {
		t.Error("parseTime accepted yesterday")
	}
}

func TestEntryRoundTrip(t *testing.T) {
	e := &cached{url: "https://example.com/a.png", stored: time.Unix(0, 42), body: []byte("bytes\r\n\r\nmore"),
		header: Header{"Etag": {`"v1"`}, "Cache-Control": {"max-age=60", "public"}}}
	got, err := decodeEntry(encodeEntry(e))
	if err != nil || got.url != e.url || !got.stored.Equal(e.stored) || string(got.body) != string(e.body) ||
		got.header.Get("ETag") != `"v1"` || len(got.header.Values("Cache-Control")) != 2 {
		t.Fatalf("decoded %+v, %v", got, err)
	}
}
