package woff

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// wrap packs a font file as a WOFF file.
func wrap(t *testing.T, font []byte) []byte {
	t.Helper()
	be := binary.BigEndian
	n := int(be.Uint16(font[4:]))
	out := make([]byte, 44+n*20)
	copy(out, "wOFF")
	copy(out[4:], font[:4])
	be.PutUint16(out[12:], uint16(n))
	for i := range n {
		e := font[12+i*16:]
		at, length := be.Uint32(e[8:]), be.Uint32(e[12:])
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		w.Write(font[at : at+length])
		w.Close()
		packed := z.Bytes()
		if len(packed) >= int(length) {
			packed = font[at : at+length]
		}
		d := out[44+i*20:]
		copy(d, e[:4])
		be.PutUint32(d[4:], uint32(len(out)))
		be.PutUint32(d[8:], uint32(len(packed)))
		be.PutUint32(d[12:], length)
		out = append(out, packed...)
	}
	return out
}

// outlines reads every glyph of a font, its advance first.
func outlines(t *testing.T, data []byte) []sfnt.Segments {
	t.Helper()
	f, err := sfnt.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var b sfnt.Buffer
	var out []sfnt.Segments
	ppem := fixed.I(int(f.UnitsPerEm()))
	for g := range f.NumGlyphs() {
		s, err := f.LoadGlyph(&b, sfnt.GlyphIndex(g), ppem, nil)
		if err != nil {
			t.Fatalf("glyph %d: %v", g, err)
		}
		adv, err := f.GlyphAdvance(&b, sfnt.GlyphIndex(g), ppem, font.HintingNone)
		if err != nil {
			t.Fatalf("glyph %d: %v", g, err)
		}
		out = append(out, append(sfnt.Segments{{Args: [3]fixed.Point26_6{{X: adv}}}}, s...))
	}
	return out
}

func same(t *testing.T, got, want []sfnt.Segments) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d glyphs, want %d", len(got), len(want))
	}
	for g := range want {
		if len(got[g]) != len(want[g]) {
			t.Fatalf("glyph %d: %d segments, want %d", g, len(got[g]), len(want[g]))
		}
		for i := range want[g] {
			if got[g][i] != want[g][i] {
				t.Fatalf("glyph %d, segment %d: %v, want %v", g, i, got[g][i], want[g][i])
			}
		}
	}
}

func TestPlainFontsPass(t *testing.T) {
	got, err := Unpack(gomedium.TTF)
	if err != nil || !bytes.Equal(got, gomedium.TTF) {
		t.Fatalf("a TrueType file changed: %v", err)
	}
}

func TestWOFF(t *testing.T) {
	file := wrap(t, gomedium.TTF)
	got, err := Unpack(file)
	if err != nil {
		t.Fatal(err)
	}
	same(t, outlines(t, got), outlines(t, gomedium.TTF))
	if sum := checksum(got); sum != 0xB1B0AFBA {
		t.Errorf("the file sums to %#x", sum)
	}
}

func TestWOFFRefusesForgedSizes(t *testing.T) {
	file := wrap(t, gomedium.TTF)
	binary.BigEndian.PutUint32(file[44+12:], 0xFFFFFFFF)
	if _, err := Unpack(file); err == nil {
		t.Error("a table of 4 GiB unpacked")
	}
	if _, err := Unpack([]byte("wOF2 and nothing else")); err == nil {
		t.Error("a file cut short unpacked")
	}
}

// Pacifico, the same font three ways, unpacks to the same outlines.
func TestWOFF2(t *testing.T) {
	dir := "testdata"
	ttf, err := os.ReadFile(filepath.Join(dir, "Pacifico-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	want := outlines(t, ttf)
	for _, name := range []string{"Pacifico-Regular.woff", "Pacifico-Regular.woff2"} {
		file, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := Unpack(file)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		same(t, outlines(t, got), want)
		if sum := checksum(got); sum != 0xB1B0AFBA {
			t.Errorf("%s sums to %#x", name, sum)
		}
	}
}
