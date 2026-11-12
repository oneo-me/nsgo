//go:build darwin

package apple

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A font request that names the same face must hand back the same font
// object. Without that, every cell of every frame rebuilds it, which is
// what this cost before: a frame of the quotes case asked forty times.
func TestFontIsReused(t *testing.T) {
	f := driver.Font{Size: 13, Weight: driver.FontWeightSemibold}
	a, b := Font(f), Font(f)
	defer Release(a)
	defer Release(b)
	if a != b {
		t.Fatalf("the same font was built twice (%#x, %#x): every caller pays the seven CoreText calls", a, b)
	}
}

// Different fonts must stay different — the cache keys on all four
// things that choose a face.
func TestFontCacheKeysOnEveryField(t *testing.T) {
	base := driver.Font{Size: 13}
	others := map[string]driver.Font{
		"size":   {Size: 14},
		"weight": {Size: 13, Weight: driver.FontWeightBold},
		"italic": {Size: 13, Italic: true},
		"family": {Size: 13, Family: "Menlo"},
	}
	a := Font(base)
	defer Release(a)
	for name, f := range others {
		b := Font(f)
		if b == a {
			t.Errorf("%s: two different fonts came back as one object", name)
		}
		Release(b)
	}
}

// The font cache is bounded: an app that animates its font size must not
// grow it forever.
func TestFontCacheIsBounded(t *testing.T) {
	for i := range fontCacheLimit * 2 {
		if f := Font(driver.Font{Size: driver.Points(10 + i)}); f != 0 {
			Release(f)
		}
	}
	if n := fontCache.len(); n > fontCacheLimit {
		t.Fatalf("the cache holds %d fonts, over its limit of %d", n, fontCacheLimit)
	}
}

// The attributes depend on the style alone, so asking twice must not
// build them twice.
func TestTextAttributesAreReused(t *testing.T) {
	st := driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111)}
	a, b := textAttributes(st, false), textAttributes(st, false)
	defer Release(a)
	defer Release(b)
	if a != b {
		t.Fatalf("the same style built its attributes twice (%#x, %#x)", a, b)
	}
}

// A style that differs in any field must not share the other's
// attributes: alignment would change the paragraph style, colour the
// foreground, and so on.
func TestTextAttributesKeyOnEveryField(t *testing.T) {
	base := driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111)}
	others := map[string]driver.TextStyle{
		"size":           {Font: driver.Font{Size: 14}, Color: driver.RGB(0x111111)},
		"weight":         {Font: driver.Font{Size: 13, Weight: driver.FontWeightBold}, Color: driver.RGB(0x111111)},
		"color":          {Font: driver.Font{Size: 13}, Color: driver.RGB(0xcf222e)},
		"align":          {Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111), Align: driver.TextAlignEnd},
		"line height":    {Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111), LineHeight: 18},
		"letter spacing": {Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111), LetterSpacing: 1},
		"underline":      {Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111), Decoration: driver.TextDecorationUnderline},
	}
	a := textAttributes(base, false)
	defer Release(a)
	for name, st := range others {
		b := textAttributes(st, false)
		if b == a {
			t.Errorf("%s: two different styles came back as one dictionary", name)
		}
		Release(b)
	}
}

// A cached dictionary outlives the call that built it, so the cache must
// keep its own reference: releasing every handle a caller got must leave
// the next one whole.
func TestCachedAttributesSurviveRelease(t *testing.T) {
	st := driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(0x222222)}
	for range 4 {
		d := textAttributes(st, false)
		if d == 0 {
			t.Fatal("the attributes were released with the caller's handle")
		}
		Release(d)
	}
}

// The cache is bounded: a colour that animates must not grow it forever.
func TestAttributesCacheIsBounded(t *testing.T) {
	for i := range attrsCacheLimit * 2 {
		st := driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(uint32(i%0xffffff) | 0x100000)}
		Release(textAttributes(st, false))
	}
	if n := attrsCache.len(); n > attrsCacheLimit {
		t.Fatalf("the cache holds %d entries, over its limit of %d", n, attrsCacheLimit)
	}
}

// Layout asks the same prepared text its size at the same width more than
// once per frame; the answer must be kept rather than measured again.
func TestMeasureIsCachedPerWidth(t *testing.T) {
	var e TextEngine
	p := e.Prepare(&driver.AttributedText{
		Text:  "123.45",
		Style: driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111)},
	})
	defer p.Release()

	a := p.Measure(driver.Points(90))
	if n := len(p.(*prepared).measured); n != 1 {
		t.Fatalf("after one measure the cache holds %d entries, want 1", n)
	}
	b := p.Measure(driver.Points(90))
	if a != b {
		t.Fatalf("the same width measured twice gave %v then %v", a, b)
	}
	if n := len(p.(*prepared).measured); n != 1 {
		t.Fatalf("measuring the same width again left %d entries, want 1", n)
	}
}

// A different width is a different answer, so it must not hit the other's
// entry: a table's cells change width when the layout tries Auto first.
func TestMeasureCacheKeysOnWidth(t *testing.T) {
	var e TextEngine
	p := e.Prepare(&driver.AttributedText{
		Text:  "123.45",
		Style: driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111)},
	})
	defer p.Release()

	wide := p.Measure(driver.Auto)
	narrow := p.Measure(driver.Points(20))
	if len(p.(*prepared).measured) != 2 {
		t.Fatalf("two widths left %d entries, want 2", len(p.(*prepared).measured))
	}
	if wide.Width <= narrow.Width {
		t.Fatalf("a 20pt wrap should be no wider than unwrapped: %v vs %v", narrow, wide)
	}
}

// Whether the text holds an attachment changes the paragraph style (a line
// with one may grow past its line height), so it is part of the key: the
// two cases must not share attributes, and each must hit its own entry.
func TestTextAttributesKeyOnAttachments(t *testing.T) {
	st := driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(0x222222), LineHeight: 20}
	plain, plain2 := textAttributes(st, false), textAttributes(st, false)
	withAtt := textAttributes(st, true)
	defer Release(plain)
	defer Release(plain2)
	defer Release(withAtt)
	if plain != plain2 {
		t.Fatalf("the same key built attributes twice (%#x, %#x)", plain, plain2)
	}
	if plain == withAtt {
		t.Fatal("text with an attachment shared plain text's attributes: its line cannot grow past the line height")
	}
	again := textAttributes(st, true)
	if again != withAtt {
		t.Fatalf("the attachment key built attributes twice (%#x, %#x)", again, withAtt)
	}
	Release(again)
}

// A span's attributes must not carry the paragraph style: NSH sets it for
// the whole text, so a span's own alignment or line height would otherwise
// rewrite its paragraph.
func TestSpanAttributesCarryNoParagraph(t *testing.T) {
	st := driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111), Align: driver.TextAlignEnd, LineHeight: 20}
	span, span2 := spanAttributes(st), spanAttributes(st)
	text := textAttributes(st, false)
	defer Release(span)
	defer Release(span2)
	defer Release(text)
	if span != span2 {
		t.Fatalf("the same span style built its attributes twice (%#x, %#x)", span, span2)
	}
	if span == text {
		t.Fatal("a span shared the whole text's attributes: its paragraph style would rewrite the paragraph")
	}
}
