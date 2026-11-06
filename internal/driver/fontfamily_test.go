package driver

import (
	"reflect"
	"testing"
)

func TestFontFamilies(t *testing.T) {
	got := FontFamilies(` Inter , "Noto Sans SC",'Brand, Inc', ui-monospace, sans-serif, "serif",`)
	want := []Family{{Name: "Inter"}, {Name: "Noto Sans SC"}, {Name: "Brand, Inc"}, {Kind: FamilyMonospace},
		{Kind: FamilySystem}, {Name: "serif"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
	if got := FontFamilies(" , "); len(got) != 0 {
		t.Errorf("got %+v from nothing", got)
	}
}

func TestUprightEmoji(t *testing.T) {
	italic := TextStyle{Font: Font{Italic: true}}
	spans := func(s string) [][2]string {
		at := AttributedText{Text: s, Style: italic}
		UprightEmoji(&at)
		var out [][2]string
		for _, sp := range at.Spans {
			kind := "upright"
			if sp.Style.Font.Italic {
				kind = "italic"
			}
			out = append(out, [2]string{s[sp.Start:sp.End], kind})
		}
		return out
	}
	for _, c := range []struct {
		text string
		want [][2]string
	}{
		{"plain", nil},
		{"é only", nil},
		{"a😀b", [][2]string{{"a", "italic"}, {"😀", "upright"}, {"b", "italic"}}},
		{"👍🏽", [][2]string{{"👍🏽", "upright"}}},
		{"👩‍💻!", [][2]string{{"👩‍💻", "upright"}, {"!", "italic"}}},
		{"🇨🇳🇯🇵", [][2]string{{"🇨🇳", "upright"}, {"🇯🇵", "upright"}}},
		{"♥️", [][2]string{{"♥️", "upright"}}},
		{"♥", nil},
		{"😀︎", nil},
		{"1️⃣", [][2]string{{"1️⃣", "upright"}}},
	} {
		if got := spans(c.text); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %q, want %q", c.text, got, c.want)
		}
	}
}
