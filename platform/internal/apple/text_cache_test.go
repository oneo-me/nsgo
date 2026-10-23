//go:build darwin

package apple

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// A font request that names the same face must hand back the same font
// object. Without that, every cell of every frame rebuilds it, which is
// what this cost before: a frame of the quotes case asked forty times.
func TestFontIsReused(t *testing.T) {
	f := core.Font{Size: 13, Weight: core.FontWeightSemibold}
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
	base := core.Font{Size: 13}
	others := map[string]core.Font{
		"size":   {Size: 14},
		"weight": {Size: 13, Weight: core.FontWeightBold},
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

// The attributes depend on the style alone, so asking twice must not
// build them twice.
func TestStyleAttributesAreReused(t *testing.T) {
	st := core.TextStyle{Font: core.Font{Size: 13}, Color: core.RGB(0x111111)}
	a, b := styleAttributes(st, false), styleAttributes(st, false)
	defer Release(a)
	defer Release(b)
	if a != b {
		t.Fatalf("the same style built its attributes twice (%#x, %#x)", a, b)
	}
}

// A style that differs in any field must not share the other's
// attributes: alignment would change the paragraph style, colour the
// foreground, and so on.
func TestStyleAttributesKeyOnEveryField(t *testing.T) {
	base := core.TextStyle{Font: core.Font{Size: 13}, Color: core.RGB(0x111111)}
	others := map[string]core.TextStyle{
		"size":           {Font: core.Font{Size: 14}, Color: core.RGB(0x111111)},
		"weight":         {Font: core.Font{Size: 13, Weight: core.FontWeightBold}, Color: core.RGB(0x111111)},
		"color":          {Font: core.Font{Size: 13}, Color: core.RGB(0xcf222e)},
		"align":          {Font: core.Font{Size: 13}, Color: core.RGB(0x111111), Align: core.TextAlignEnd},
		"line height":    {Font: core.Font{Size: 13}, Color: core.RGB(0x111111), LineHeight: 18},
		"letter spacing": {Font: core.Font{Size: 13}, Color: core.RGB(0x111111), LetterSpacing: 1},
		"underline":      {Font: core.Font{Size: 13}, Color: core.RGB(0x111111), Decoration: core.TextDecorationUnderline},
	}
	a := styleAttributes(base, false)
	defer Release(a)
	for name, st := range others {
		b := styleAttributes(st, false)
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
	st := core.TextStyle{Font: core.Font{Size: 13}, Color: core.RGB(0x222222)}
	for range 4 {
		d := styleAttributes(st, false)
		if d == 0 {
			t.Fatal("the attributes were released with the caller's handle")
		}
		Release(d)
	}
}

// The cache is bounded: a colour that animates must not grow it forever.
func TestAttributesCacheIsBounded(t *testing.T) {
	for i := range attrsCacheLimit * 2 {
		st := core.TextStyle{Font: core.Font{Size: 13}, Color: core.RGB(uint32(i%0xffffff) | 0x100000)}
		Release(styleAttributes(st, false))
	}
	attrsMu.Lock()
	n := len(attrsCache)
	attrsMu.Unlock()
	if n > attrsCacheLimit {
		t.Fatalf("the cache holds %d entries, over its limit of %d", n, attrsCacheLimit)
	}
}

// Layout asks the same prepared text its size at the same width more than
// once per frame; the answer must be kept rather than measured again.
func TestMeasureIsCachedPerWidth(t *testing.T) {
	var e TextEngine
	p := e.Prepare(&core.AttributedText{
		Text:  "123.45",
		Style: core.TextStyle{Font: core.Font{Size: 13}, Color: core.RGB(0x111111)},
	})
	defer p.Release()

	a := p.Measure(core.Points(90))
	if n := len(p.(*prepared).measured); n != 1 {
		t.Fatalf("after one measure the cache holds %d entries, want 1", n)
	}
	b := p.Measure(core.Points(90))
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
	p := e.Prepare(&core.AttributedText{
		Text:  "123.45",
		Style: core.TextStyle{Font: core.Font{Size: 13}, Color: core.RGB(0x111111)},
	})
	defer p.Release()

	wide := p.Measure(core.Auto)
	narrow := p.Measure(core.Points(20))
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
func TestStyleAttributesKeyOnAttachments(t *testing.T) {
	st := core.TextStyle{Font: core.Font{Size: 13}, Color: core.RGB(0x222222), LineHeight: 20}
	plain, plain2 := styleAttributes(st, false), styleAttributes(st, false)
	withAtt := styleAttributes(st, true)
	defer Release(plain)
	defer Release(plain2)
	defer Release(withAtt)
	if plain != plain2 {
		t.Fatalf("the same key built attributes twice (%#x, %#x)", plain, plain2)
	}
	if plain == withAtt {
		t.Fatal("text with an attachment shared plain text's attributes: its line cannot grow past the line height")
	}
	again := styleAttributes(st, true)
	if again != withAtt {
		t.Fatalf("the attachment key built attributes twice (%#x, %#x)", again, withAtt)
	}
	Release(again)
}
