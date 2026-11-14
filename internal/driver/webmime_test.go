package driver

import (
	"bytes"
	"testing"
)

func TestWebMIME(t *testing.T) {
	for _, test := range []struct {
		name, path, want string
		body             []byte
	}{
		{name: "extension", path: "index.html", want: "text/html; charset=utf-8"},
		{name: "html", path: "index", want: "text/html; charset=utf-8", body: []byte("<!doctype html>")},
		{name: "png", path: "asset", want: "image/png", body: []byte("\x89PNG\r\n\x1a\n")},
		{name: "webp", path: "asset", want: "image/webp", body: []byte("RIFF\x00\x00\x00\x00WEBPVP")},
		{name: "wave", path: "asset", want: "audio/wave", body: []byte("RIFF\x00\x00\x00\x00WAVE")},
		{name: "plain", path: "data", want: "text/plain; charset=utf-8", body: []byte("hello")},
		{name: "binary", path: "data", want: "application/octet-stream", body: []byte{0, 1, 2}},
		{name: "html after whitespace", path: "index", want: "text/html; charset=utf-8", body: []byte(" \t\n<HTML><body>")},
		{name: "html cut after whitespace", path: "index", want: "text/plain; charset=utf-8", body: []byte("\n<b")},
		{name: "html at the end", path: "index", want: "text/html; charset=utf-8", body: []byte("\n<b>")},
		{name: "html tag unterminated", path: "index", want: "text/plain; charset=utf-8", body: []byte("<bold>")},
		{name: "comment", path: "index", want: "text/html; charset=utf-8", body: []byte("\r\n<!--x-->")},
		{name: "html at the cap", path: "index", want: "text/html; charset=utf-8", body: append(bytes.Repeat([]byte(" "), 509), "<p>"...)},
		{name: "html cut by the cap", path: "index", want: "text/plain; charset=utf-8", body: append(bytes.Repeat([]byte(" "), 510), "<p>"...)},
		{name: "html past the cap", path: "index", want: "text/plain; charset=utf-8", body: append(bytes.Repeat([]byte(" "), 512), "<html>"...)},
		{name: "xml after whitespace", path: "feed", want: "text/xml; charset=utf-8", body: []byte("\n<?xml version=\"1.0\"?>")},
		{name: "xml cut after whitespace", path: "feed", want: "text/plain; charset=utf-8", body: []byte("\n<?xm")},
		{name: "utf-16be", path: "text", want: "text/plain; charset=utf-16be", body: []byte("\xfe\xff\x00h\x00i")},
		{name: "utf-16le", path: "text", want: "text/plain; charset=utf-16le", body: []byte("\xff\xfeh\x00i\x00")},
		{name: "utf-8 bom", path: "text", want: "text/plain; charset=utf-8", body: []byte("\xef\xbb\xbfhi")},
		{name: "whitespace only", path: "text", want: "text/plain; charset=utf-8", body: []byte(" \n")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := WebMIME(test.path, test.body); got != test.want {
				t.Fatalf("WebMIME(%q, %q) = %q, want %q", test.path, test.body, got, test.want)
			}
		})
	}
}
