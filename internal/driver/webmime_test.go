package driver

import "testing"

func TestWebMIME(t *testing.T) {
	for _, test := range []struct {
		name, path, want string
		body             []byte
	}{
		{name: "extension", path: "index.html", want: "text/html; charset=utf-8"},
		{name: "html", path: "index", want: "text/html; charset=utf-8", body: []byte("<!doctype html>")},
		{name: "png", path: "asset", want: "image/png", body: []byte("\x89PNG\r\n\x1a\n")},
		{name: "plain", path: "data", want: "text/plain; charset=utf-8", body: []byte("hello")},
		{name: "binary", path: "data", want: "application/octet-stream", body: []byte{0, 1, 2}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := WebMIME(test.path, test.body); got != test.want {
				t.Fatalf("WebMIME(%q, %q) = %q, want %q", test.path, test.body, got, test.want)
			}
		})
	}
}
