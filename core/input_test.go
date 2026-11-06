package core

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestInputSubmitResolvesWithMultiline(t *testing.T) {
	for _, c := range []struct {
		set       SubmitBehavior
		multiline bool
		want      SubmitBehavior
	}{
		{SubmitBehaviorDefault, false, SubmitBehaviorBlurAndSubmit},
		{SubmitBehaviorDefault, true, SubmitBehaviorNewline},
		{SubmitBehaviorNewline, false, SubmitBehaviorBlurAndSubmit},
		{SubmitBehaviorNewline, true, SubmitBehaviorNewline},
		{SubmitBehaviorSubmit, true, SubmitBehaviorSubmit},
		{SubmitBehaviorBlurAndSubmit, true, SubmitBehaviorBlurAndSubmit},
	} {
		cfg := driver.InputConfig{SubmitBehavior: c.set, Multiline: c.multiline}
		if got := cfg.Submit(); got != c.want {
			t.Errorf("Submit(%v, multiline %v) = %v, want %v", c.set, c.multiline, got, c.want)
		}
	}
}

func TestInputConfigCarriesProps(t *testing.T) {
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		return View{Style: Style{SelectionColor: RGB(0x00ff00)}, Children: []Node{
			Input{TestID: "in", Multiline: true, MaxLength: 5, ReturnKeyType: ReturnKeySend,
				SubmitBehavior: SubmitBehaviorSubmit, AutoCapitalize: AutoCapitalizeWords, NoAutoCorrect: true,
				AutoComplete: AutoCompleteEmail, IOS: IOSInputProps{KeyboardAppearance: KeyboardAppearanceDark}, HideCaret: true,
				Style: Style{PlaceholderColor: RGB(0xff0000)}},
		}}
	})
	b := h.commit()
	var c *driver.InputConfig
	for _, it := range b.Items {
		if it.Kind == driver.ItemInput {
			c = it.Input
		}
	}
	if c == nil {
		t.Fatal("no ItemInput")
	}
	want := driver.InputConfig{Multiline: true, MaxLength: 5, ReturnKey: ReturnKeySend, SubmitBehavior: SubmitBehaviorSubmit,
		AutoCapitalize: AutoCapitalizeWords, NoAutoCorrect: true, AutoComplete: AutoCompleteEmail,
		KeyboardAppearance: KeyboardAppearanceDark, HideCaret: true,
		PlaceholderColor: RGB(0xff0000), SelectionColor: RGB(0x00ff00)}
	got := *c
	got.Style, got.Content = driver.InputConfig{}.Style, Bounds{}
	if got != want {
		t.Fatalf("config = %+v\nwant %+v", got, want)
	}
	if AutoCompleteNewPassword.String() != "new-password" || AutoCompleteBirthdate.String() != "bday" {
		t.Fatal("autocomplete names")
	}
}

func TestInputTakesItsThemesColors(t *testing.T) {
	accent := func(c Color) *Theme {
		th := DefaultTheme().Clone()
		th.Light.Accent, th.Light.Selection = c, c.Faded(0.2)
		return th
	}
	var scope *Scope
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		scope = cx
		cx.SetTheme(accent(RGB(0x112233)))
		return View{Children: []Node{
			Input{TestID: "plain"},
			Input{TestID: "styled", Style: Style{SelectionColor: RGB(0x00ff00), CaretColor: RGB(0xff0000)}},
			Input{TestID: "selection", Style: Style{SelectionColor: RGB(0x00ff00)}},
		}}
	})
	inputs := func(b *driver.MountBatch) map[NodeID]*driver.InputConfig {
		out := map[NodeID]*driver.InputConfig{}
		for _, it := range b.Items {
			if it.Kind == driver.ItemInput {
				out[it.ID] = it.Input
			}
		}
		return out
	}
	got := inputs(h.commit())
	plain := got[h.find("plain").id]
	if plain == nil || plain.SelectionColor != RGB(0x112233).Faded(0.2) || plain.Caret() != RGB(0x112233) {
		t.Fatalf("plain = %+v", plain)
	}
	if c := got[h.find("styled").id]; c.SelectionColor != RGB(0x00ff00) || c.Caret() != RGB(0xff0000) {
		t.Fatalf("styled = %+v", c)
	}
	if c := got[h.find("selection").id]; c.SelectionColor != RGB(0x00ff00) || c.Caret() != RGB(0x00ff00) {
		t.Fatalf("selection = %+v", c)
	}
	scope.SetTheme(accent(RGB(0x445566)))
	if c := inputs(h.commit())[h.find("plain").id]; c == nil || c.Caret() != RGB(0x445566) {
		t.Fatalf("after the theme changed, plain = %+v", c)
	}
}

func TestInputHighlightKeepsATranslucentColor(t *testing.T) {
	for _, c := range []struct{ set, want Color }{
		{0, 0},
		{RGB(0x112233), RGB(0x112233).Faded(0.3)},
		{RGB(0x112233).Faded(0.22), RGB(0x112233).Faded(0.22)},
	} {
		cfg := driver.InputConfig{SelectionColor: c.set}
		if got := cfg.Highlight(); got != c.want {
			t.Errorf("Highlight of %08x = %08x, want %08x", uint32(c.set), uint32(got), uint32(c.want))
		}
	}
}

func TestMultilineInputGrowsWithItsText(t *testing.T) {
	var text *Signal[string]
	var sizes []Size
	h := newHost(t, Size{Width: 200, Height: 400}, func(cx *Scope) Node {
		text = NewSignal(cx, "")
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{
			Input{TestID: "in", Multiline: true, Value: text.Get, OnChange: text.Set,
				Style:               Style{Width: Pt(100), FontSize: 10},
				OnContentSizeChange: func(s Size) { sizes = append(sizes, s) }},
		}}
	})
	h.commit()
	in := h.find("in")
	if in.frame.Size.Height != 12 {
		t.Fatalf("empty height %v, want a line of 12", in.frame.Size.Height)
	}
	if len(sizes) != 1 || sizes[0] != (Size{Width: 0, Height: 12}) {
		t.Fatalf("sizes %v, want an empty line", sizes)
	}
	// 20 characters of 6 points wrap at 100 points into two lines.
	h.tree.Dispatch(driver.Event{Kind: driver.EventChangeText, ID: in.id, Text: "aaaaaaaaa bbbbbbbbbb", Count: 1})
	h.commit()
	if in.frame.Size.Height != 24 {
		t.Fatalf("height %v, want two lines", in.frame.Size.Height)
	}
	if len(sizes) != 2 || sizes[1].Height != 24 {
		t.Fatalf("sizes %v", sizes)
	}
	// A trailing line break takes a line.
	h.tree.Dispatch(driver.Event{Kind: driver.EventChangeText, ID: in.id, Text: "a\n", Count: 2})
	h.commit()
	if in.frame.Size.Height != 24 {
		t.Fatalf("height %v with a trailing line break, want two lines", in.frame.Size.Height)
	}
}

func TestInputSelectionKeysAndComposition(t *testing.T) {
	var sel *Signal[TextRange]
	var got []TextRange
	var keys []string
	var comps []Composition
	ref := &Ref{}
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		sel = NewSignal(cx, TextRange{Start: 1, End: 2})
		return Input{TestID: "in", Ref: ref, DefaultValue: "abc", SelectedRange: sel.Get,
			OnSelectionChange:   func(r TextRange) { got = append(got, r) },
			OnKeyPress:          func(k string) { keys = append(keys, k) },
			OnCompositionChange: func(c Composition) { comps = append(comps, c) }}
	})
	b := h.commit()
	in := h.find("in")
	value, selected := -1, -1
	for i, it := range b.Items {
		switch it.Kind {
		case driver.ItemInputValue:
			value = i
		case driver.ItemSelection:
			selected = i
			if it.Selection != (TextRange{Start: 1, End: 2}) {
				t.Fatalf("selection %v", it.Selection)
			}
		}
	}
	if value < 0 || selected < value {
		t.Fatalf("value at %d, selection at %d: want the selection after the value", value, selected)
	}
	for _, k := range []driver.EventKind{driver.EventSelectionChange, driver.EventKeyPress, driver.EventCompositionChange} {
		if !in.events().Has(k) {
			t.Fatalf("mask lacks %v", k)
		}
	}
	sel.Set(TextRange{Start: 0, End: 3})
	if b := h.commit(); len(b.Items) != 1 || b.Items[0].Kind != driver.ItemSelection || b.Items[0].Selection != (TextRange{Start: 0, End: 3}) {
		t.Fatalf("items %+v", b.Items)
	}
	ref.SetSelection(2, 2)
	if b := h.commit(); len(b.Items) != 1 || b.Items[0].Selection != (TextRange{Start: 2, End: 2}) {
		t.Fatalf("items %+v", b.Items)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventSelectionChange, ID: in.id, Index: 1, Count: 3})
	h.tree.Dispatch(driver.Event{Kind: driver.EventKeyPress, ID: in.id, Text: "Backspace"})
	h.tree.Dispatch(driver.Event{Kind: driver.EventCompositionChange, ID: in.id, Text: "ni"})
	h.tree.Dispatch(driver.Event{Kind: driver.EventCompositionChange, ID: in.id, Text: "ni", Finished: true})
	if len(got) != 1 || got[0] != (TextRange{Start: 1, End: 3}) || len(keys) != 1 || keys[0] != "Backspace" {
		t.Fatalf("selection %v, keys %v", got, keys)
	}
	if len(comps) != 2 || comps[0] != (Composition{true, "ni"}) || comps[1] != (Composition{}) {
		t.Fatalf("compositions %v", comps)
	}
}

func TestLimitEditAndUTF16(t *testing.T) {
	if UTF16Len("a😀") != 3 || driver.ClipUTF16("a😀b", 2) != "a" || driver.ClipUTF16("a😀b", 3) != "a😀" {
		t.Fatal("utf-16 lengths")
	}
	if driver.UTF16Offset("a😀b", 3) != 5 || driver.UTF16Units("a😀b", 5) != 3 {
		t.Fatal("utf-16 offsets")
	}
	if kept, ok := driver.LimitEdit("abc", 0, "de", 4); kept != "d" || !ok {
		t.Fatalf("kept %q %v", kept, ok)
	}
	if _, ok := driver.LimitEdit("abcd", 0, "e", 4); ok {
		t.Fatal("a full field took a character")
	}
	if kept, ok := driver.LimitEdit("abcd", 2, "xyz", 4); kept != "xy" || !ok {
		t.Fatalf("replacing kept %q %v", kept, ok)
	}
}

func TestCaretAndPasteReachTheInputAndAnOverlayAnchorsToTheCaret(t *testing.T) {
	var caret *Signal[Bounds]
	var pasted []PasteEvent
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		caret = NewSignal(cx, Bounds{})
		return View{Style: Style{Padding: All(20)}, Children: []Node{
			Input{TestID: "field", Style: Style{Height: Pt(30)}, PasteBehavior: PasteReport,
				OnCaretMove: func(e CaretEvent) { caret.Set(e.PageBounds) },
				OnPaste:     func(e PasteEvent) { pasted = append(pasted, e) }},
			Overlay{AnchorBounds: caret.Get, Offset: 4, TestID: "mentions",
				Style: Style{Width: Pt(80), Height: Pt(40)}},
		}}
	})
	h.commit()
	field := h.find("field")
	if m := field.events(); !m.Has(driver.EventCaretMove) || !m.Has(driver.EventPaste) {
		t.Fatal("an input with OnCaretMove and OnPaste does not report them")
	}
	if field.input.config.PasteBehavior != PasteReport {
		t.Fatal("PasteBehavior does not reach the platform")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventCaretMove, ID: field.id, Point: Point{X: 50, Y: 4}, Size: Size{Width: 2, Height: 20}})
	if got, want := caret.GetUntracked(), (Bounds{Origin: Point{X: 70, Y: 24}, Size: Size{Width: 2, Height: 20}}); got != want {
		t.Fatalf("caret page bounds = %v, want %v", got, want)
	}
	h.commit()
	if got := h.find("mentions").frame.Origin; got != (Point{X: 31, Y: 48}) {
		t.Fatalf("overlay at %v, want centred under the caret, 4 points below", got)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPaste, ID: field.id, Data: &PasteEvent{Text: "hi",
		Files: []PastedFile{{MIME: "image/png", Data: []byte{1}}}}})
	if len(pasted) != 1 || pasted[0].Text != "hi" || len(pasted[0].Files) != 1 {
		t.Fatalf("pasted %+v", pasted)
	}
}

func TestInputPlaceholderFollowsItsSignal(t *testing.T) {
	var text *Signal[string]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		text = NewSignal(cx, "one")
		return Input{PlaceholderFunc: text.Get, TestID: "in"}
	})
	sent := func(b *driver.MountBatch) string {
		for _, it := range b.Items {
			if it.Kind == driver.ItemInput {
				return it.Input.Placeholder
			}
		}
		return ""
	}
	if got := sent(h.commit()); got != "one" {
		t.Fatalf("placeholder %q", got)
	}
	text.Set("two")
	if got := sent(h.commit()); got != "two" {
		t.Fatalf("placeholder %q after it changed", got)
	}
}
