package core

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestTextFontScaling(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{FontSize: 10}, Children: []Node{
			Text{Content: "a", TestID: "scaled"},
			Text{Content: "b", TestID: "fixed", NoFontScaling: true},
			Text{Content: "c", TestID: "capped", MaxFontSizeMultiplier: 1.5},
		}}
	})
	h.tree.SetEnvironment(Environment{FontScale: 2})
	h.commit()
	for id, want := range map[string]Points{"scaled": 20, "fixed": 10, "capped": 15} {
		if got := h.find(id).text.style.Font.Size; got != want {
			t.Errorf("%s: font size = %v, want %v", id, got, want)
		}
	}
}

func TestTextTransformShadowAndDecoration(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{TextTransform: TextTransformUppercase, TextShadowOffset: Point{X: 1, Y: 2}}, Children: []Node{
			Text{TestID: "text", Style: Style{TextDecoration: TextDecorationUnderline, TextDecorationStyle: DecorationWavy,
				TextDecorationColor: RGB(0xff0000)}, Spans: []Span{
				{Content: "hello "},
				{Content: "big world", Style: Style{TextTransform: TextTransformCapitalize}},
			}},
		}}
	})
	h.commit()
	tn := h.find("text").text
	if got, want := tn.text.Text, "HELLO Big World"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
	if got, want := tn.text.Shadow, (driver.TextShadow{Color: Color(0x00000055), Offset: Point{X: 1, Y: 2}}); got != want {
		t.Errorf("shadow = %+v, want %+v", got, want)
	}
	if s := tn.style; s.DecorationStyle != DecorationWavy || s.DecorationColor != RGB(0xff0000) {
		t.Errorf("decoration = %v %v, want wavy red", s.DecorationStyle, s.DecorationColor)
	}
	if tn.text.Ellipsize != EllipsizeTail {
		t.Errorf("ellipsize = %v, want tail by default", tn.text.Ellipsize)
	}
}

func TestFontVariantAndDecorationLines(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{FontVariant: FontSmallCaps, FontSize: 20, TextUnderlineOffset: Pct(10)}, Children: []Node{
			Text{TestID: "inherited", Content: "abc", Style: Style{TextDecoration: TextDecorationUnderline,
				TextDecorationThickness: Pt(3)}},
			Text{TestID: "own", Content: "123", Style: Style{FontVariant: FontTabularNums}},
			Text{TestID: "normal", Content: "123", Style: Style{FontVariant: FontVariantNormal,
				TextDecoration: TextDecorationLineThrough, TextUnderlineOffset: Pt(0)}},
		}}
	})
	h.commit()
	if s := h.find("inherited").text.style; s.Font.Variant != FontSmallCaps || s.DecorationThickness != 3 ||
		s.UnderlineOffset != Pt(2) {
		t.Errorf("inherited: variant %v, thickness %v, offset %v; want small caps, 3, 2", s.Font.Variant,
			s.DecorationThickness, s.UnderlineOffset)
	}
	if v := h.find("own").text.style.Font.Variant; v != FontTabularNums {
		t.Errorf("own variant = %v, want tabular figures alone", v)
	}
	if s := h.find("normal").text.style; s.Font.Variant != 0 || !s.UnderlineOffset.IsAuto() {
		t.Errorf("normal: variant %v, offset %v; want none, and no offset without an underline", s.Font.Variant,
			s.UnderlineOffset)
	}
}

func TestTextFitsItsBox(t *testing.T) {
	var lines []TextLine
	var batch *driver.MountBatch
	h := newHost(t, Size{Width: 400, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{
			Text{Content: "aaaaaaaaaa", TestID: "text", Style: Style{FontSize: 10, Width: Pt(30), Height: Pt(8)},
				AdjustsFontSizeToFit: true, MinimumFontScale: 0.2, OnTextLayout: func(l []TextLine) { lines = l }},
		}}
	})
	batch = h.commit()
	var text *driver.TextContent
	for _, it := range batch.Items {
		if it.Kind == driver.ItemText {
			text = it.Text
		}
	}
	if text == nil {
		t.Fatal("no text item")
	}
	l := text.Layout.(*ApproxLayout)
	// Ten characters 6 wide at 10 points fit 30 points at a scale of 0.5.
	if len(l.Lines) != 1 || len(lines) != 1 {
		t.Fatalf("lines = %q, reported %+v, want one", l.Lines, lines)
	}
	if w := lines[0].Bounds.Size.Width; w > 30 || w < 28 {
		t.Errorf("line width = %v, want just under 30", w)
	}
	if lines[0].Text != "aaaaaaaaaa" || lines[0].Start != 0 || lines[0].End != 10 {
		t.Errorf("line = %+v, want the whole text", lines[0])
	}
}

func TestTextAlignVertical(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{
			Text{Content: "a", TestID: "text", Style: Style{FontSize: 10, Height: Pt(40),
				TextAlignVertical: TextAlignVerticalBottom}},
		}}
	})
	b := h.commit()
	for _, it := range b.Items {
		if it.Kind == driver.ItemText {
			if got, want := it.Text.Content, (Bounds{Origin: Point{X: 0, Y: 28}, Size: Size{Width: 6, Height: 12}}); got != want {
				t.Errorf("content = %v, want %v", got, want)
			}
			return
		}
	}
	t.Fatal("no text item")
}

func TestCutToFit(t *testing.T) {
	text := driver.AttributedText{Text: "abcdefghij", Ellipsize: EllipsizeMiddle,
		Spans: []driver.TextSpan{{Start: 0, End: 5}, {Start: 5, End: 10, Link: true}}}
	fits := func(t *driver.AttributedText) bool { return len([]rune(t.Text)) <= 5 }
	got := text.CutToFit(fits)
	if got.Text != "ab…ij" {
		t.Errorf("middle = %q, want ab…ij", got.Text)
	}
	if len(got.Spans) != 2 || got.Spans[0] != (driver.TextSpan{Start: 0, End: 5}) || got.Spans[1].Start != 5 || got.Spans[1].End != 7 {
		t.Errorf("spans = %+v, want the ellipsis in the first", got.Spans)
	}
	text.Ellipsize = EllipsizeHead
	if got := text.CutToFit(fits); got.Text != "…ghij" {
		t.Errorf("head = %q, want …ghij", got.Text)
	}
}

func TestTextDefaultsToFourteenPoints(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 100}, func(cx *Scope) Node {
		return Text{Content: "a", TestID: "text"}
	})
	h.commit()
	if got := h.find("text").text.style.Font.Size; got != 14 {
		t.Errorf("font size = %v, want 14", got)
	}
}

func TestSpansFuncRestylesTheText(t *testing.T) {
	var color *Signal[Color]
	h := newHost(t, Size{Width: 400, Height: 100}, func(cx *Scope) Node {
		color = NewSignal(cx, RGB(0xff0000))
		return Text{SpansFunc: func() []Span {
			return []Span{{Content: "func"}, {Content: " main", Style: Style{Color: color.Get()}}}
		}}
	})
	sent := func() *driver.TextContent {
		for _, it := range h.commit().Items {
			if it.Kind == driver.ItemText {
				return it.Text
			}
		}
		return nil
	}
	if text := sent(); text == nil || text.Text != "func main" || text.Spans[1].Style.Color != RGB(0xff0000) {
		t.Fatalf("text %+v, want the spans", text)
	}
	color.Set(RGB(0x00ff00))
	if text := sent(); text == nil || text.Spans[1].Style.Color != RGB(0x00ff00) {
		t.Fatalf("restyled %+v", text)
	}
	if text := sent(); text != nil {
		t.Fatal("unchanged spans sent the text again")
	}
}
