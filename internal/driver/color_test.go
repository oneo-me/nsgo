package driver

import "testing"

// TestModernCSSColors reads oklab(), oklch() and color-mix() as browsers
// do.
func TestModernCSSColors(t *testing.T) {
	for _, c := range []struct {
		css  string
		want Color
	}{
		{"oklch(0.145 0 0)", 0x0a0a0aff},
		{"oklch(1 0 0)", 0xffffffff},
		{"oklab(0.628 0.225 0.126)", 0xff0000ff},
		{"color-mix(in srgb, #ff0000 10%, transparent)", 0xff00001a},
		{"color-mix(in oklab, red 50%, blue)", 0x8c53a2ff},
		{"color-mix(in srgb, red, blue)", 0x800080ff},
		{"color-mix(in srgb, red 30%, blue 30%)", 0x80008099},
	} {
		got, err := ParseColor(c.css)
		if err != nil || colorDistance(got, c.want) > 1 {
			t.Errorf("%s = %#x, %v; want %#x", c.css, got, err, c.want)
		}
	}
	if _, err := ParseColor("color-mix(in srgb, undefined 10%, transparent)"); err == nil {
		t.Error("a mix of no color reads")
	}
}
