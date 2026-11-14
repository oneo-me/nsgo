package driver

import (
	"bytes"
	"encoding/binary"
	"mime"
	"path/filepath"
)

// WebMIME returns the media type a web scheme response should use. It first
// trusts the URL's extension, then applies the small signature set browsers
// use for responses without one. The fallback is always a valid media type.
func WebMIME(path string, body []byte) string {
	if typ := mime.TypeByExtension(filepath.Ext(path)); typ != "" {
		return typ
	}
	if len(body) > webSniffLen {
		body = body[:webSniffLen]
	}
	first := 0
	for first < len(body) {
		switch body[first] {
		case '\t', '\n', '\f', '\r', ' ':
			first++
		default:
			goto signatures
		}
	}

signatures:
	// The BOMs' patterns are four bytes long, of which WHATWG's masks compare only the BOM.
	if len(body) >= 4 {
		switch {
		case bytes.HasPrefix(body, []byte("\xfe\xff")):
			return "text/plain; charset=utf-16be"
		case bytes.HasPrefix(body, []byte("\xff\xfe")):
			return "text/plain; charset=utf-16le"
		case bytes.HasPrefix(body, []byte("\xef\xbb\xbf")):
			return "text/plain; charset=utf-8"
		}
	}
	rest := body[first:]
	for _, sig := range webHTMLSignatures {
		if len(rest) > len(sig) && bytes.EqualFold(rest[:len(sig)], sig) && webTagEnd(rest[len(sig)]) {
			return "text/html; charset=utf-8"
		}
	}
	// A comment needs no tag-terminating byte.
	if bytes.HasPrefix(rest, []byte("<!--")) {
		return "text/html; charset=utf-8"
	}
	for _, sig := range webExactSignatures {
		if bytes.HasPrefix(body, sig.prefix) {
			return sig.mime
		}
	}
	if bytes.HasPrefix(rest, []byte("<?xml")) {
		return "text/xml; charset=utf-8"
	}
	if len(body) >= 12 && bytes.Equal(body[4:8], []byte("ftyp")) {
		size := int(binary.BigEndian.Uint32(body[:4]))
		if size >= 12 && size <= len(body) && size%4 == 0 {
			for i := 8; i+3 < size; i += 4 {
				if bytes.Equal(body[i:i+3], []byte("mp4")) {
					return "video/mp4"
				}
			}
		}
	}
	if len(body) >= 12 && bytes.Equal(body[:4], []byte("RIFF")) {
		switch string(body[8:12]) {
		case "WEBP":
			if len(body) >= 14 && bytes.Equal(body[12:14], []byte("VP")) {
				return "image/webp"
			}
		case "AVI ":
			return "video/avi"
		case "WAVE":
			return "audio/wave"
		}
	}
	if len(body) >= 12 && bytes.Equal(body[:4], []byte("FORM")) && bytes.Equal(body[8:12], []byte("AIFF")) {
		return "audio/aiff"
	}
	if webText(rest) {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

const webSniffLen = 512

var webHTMLSignatures = [][]byte{
	[]byte("<!DOCTYPE HTML"), []byte("<HTML"), []byte("<HEAD"), []byte("<SCRIPT"),
	[]byte("<IFRAME"), []byte("<H1"), []byte("<DIV"), []byte("<FONT"), []byte("<TABLE"),
	[]byte("<A"), []byte("<STYLE"), []byte("<TITLE"), []byte("<B"), []byte("<BODY"),
	[]byte("<BR"), []byte("<P"),
}

var webExactSignatures = []struct {
	prefix []byte
	mime   string
}{
	{[]byte("%PDF-"), "application/pdf"},
	{[]byte("%!PS-Adobe-"), "application/postscript"},
	{[]byte("\x00\x00\x01\x00"), "image/x-icon"},
	{[]byte("\x00\x00\x02\x00"), "image/x-icon"},
	{[]byte("BM"), "image/bmp"},
	{[]byte("GIF87a"), "image/gif"},
	{[]byte("GIF89a"), "image/gif"},
	{[]byte("\x89PNG\r\n\x1a\n"), "image/png"},
	{[]byte("\xff\xd8\xff"), "image/jpeg"},
	{[]byte("ID3"), "audio/mpeg"},
	{[]byte("OggS\x00"), "application/ogg"},
	{[]byte("MThd\x00\x00\x00\x06"), "audio/midi"},
	{[]byte("\x1a\x45\xdf\xa3"), "video/webm"},
	{[]byte("\x00\x01\x00\x00"), "font/ttf"},
	{[]byte("OTTO"), "font/otf"},
	{[]byte("ttcf"), "font/collection"},
	{[]byte("wOFF"), "font/woff"},
	{[]byte("wOF2"), "font/woff2"},
	{[]byte("\x1f\x8b\x08"), "application/x-gzip"},
	{[]byte("PK\x03\x04"), "application/zip"},
	{[]byte("Rar!\x1a\x07\x00"), "application/x-rar-compressed"},
	{[]byte("Rar!\x1a\x07\x01\x00"), "application/x-rar-compressed"},
	{[]byte("\x00asm"), "application/wasm"},
}

func webTagEnd(b byte) bool { return b == ' ' || b == '>' }

func webText(data []byte) bool {
	for _, b := range data {
		switch {
		case b <= 0x08, b == 0x0b, 0x0e <= b && b <= 0x1a, 0x1c <= b && b <= 0x1f:
			return false
		}
	}
	return true
}
