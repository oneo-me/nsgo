package driver

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestClipboardPNG(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	if c := ClipboardPNG(b.Bytes()); c == nil || c.Err != "" || c.Size != (Size{Width: 3, Height: 2}) {
		t.Fatalf("ClipboardPNG = %+v", c)
	}
	for _, data := range [][]byte{[]byte("GIF89a"), b.Bytes()[:20]} {
		if c := ClipboardPNG(data); c == nil || c.Err == "" {
			t.Errorf("ClipboardPNG(%q) = %+v, want an error", data, c)
		}
	}
}
