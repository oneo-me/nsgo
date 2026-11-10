package imageload

import (
	"context"
	"errors"
	"strings"
	"time"
)

// HTTP downloads http(s) sources: it gets url with header, calling
// progress as the body comes in, and returns the response whatever its
// status. Each native backend sets it to the system's client (URLSession,
// WinHTTP, libsoup, the Android runtime's HttpURLConnection) as it
// starts, so apps link no Go HTTP and TLS stack for their images, and
// downloads follow the system's proxies, certificates and policies such
// as App Transport Security.
var HTTP func(ctx context.Context, url string, header Header, progress Progress) (*Response, error)

// Response is what HTTP returns: the status, the headers and the whole
// body.
type Response struct {
	Status int
	Header Header
	Body   []byte
}

var errNoHTTP = errors.New("imageload: no HTTP client")

// get is HTTP, or an error when the backend set none.
func get(ctx context.Context, url string, header Header, progress Progress) (*Response, error) {
	if HTTP == nil {
		return nil, errNoHTTP
	}
	return HTTP(ctx, url, header, progress)
}

// Header holds HTTP headers by their canonical keys (CanonicalKey), as
// net/http's Header does.
type Header map[string][]string

// Add appends value to key's values.
func (h Header) Add(key, value string) {
	key = CanonicalKey(key)
	h[key] = append(h[key], value)
}

// Set replaces key's values with value.
func (h Header) Set(key, value string) { h[CanonicalKey(key)] = []string{value} }

// Get returns key's first value, "" when it has none.
func (h Header) Get(key string) string {
	if v := h[CanonicalKey(key)]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// Values returns key's values.
func (h Header) Values(key string) []string { return h[CanonicalKey(key)] }

// CanonicalKey is key with its first letter and those after a hyphen
// upper-cased and the rest lower-cased, as net/textproto's
// CanonicalMIMEHeaderKey makes it: "etag" and "ETag" are "Etag".
func CanonicalKey(key string) string {
	b := []byte(strings.TrimSpace(key))
	upper := true
	for i, c := range b {
		switch {
		case upper && 'a' <= c && c <= 'z':
			b[i] = c - 'a' + 'A'
		case !upper && 'A' <= c && c <= 'Z':
			b[i] = c - 'A' + 'a'
		}
		upper = c == '-'
	}
	return string(b)
}

// parseTime parses an HTTP date in any of the three formats RFC 9110
// allows, as net/http's ParseTime does.
func parseTime(s string) (time.Time, error) {
	var err error
	for _, layout := range [...]string{"Mon, 02 Jan 2006 15:04:05 GMT", time.RFC850, time.ANSIC} {
		var t time.Time
		if t, err = time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, err
}
