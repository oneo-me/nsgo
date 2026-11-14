package textview

import "testing"

func TestDocumentFind(t *testing.T) {
	doc := Document{Blocks: []Block{
		Paragraph{Content: []Inline{Text{Text: "Read the README"}, Strong{Content: []Inline{Text{Text: " again"}}}}},
		Heading{Content: []Inline{Text{Text: "README"}}},
		CodeBlock{Code: "readme := true"},
		Blockquote{Children: []Block{Paragraph{Content: []Inline{Text{Text: "README in a quote"}}}}},
	}}
	got := doc.Find("readme", FindOptions{})
	if len(got) != 4 {
		t.Fatalf("got %d matches, want 4: %#v", len(got), got)
	}
	if got[0].From != 9 || got[0].To != 15 || got[0].Text.Block[0] != 0 {
		t.Fatalf("first match = %#v", got[0])
	}
	if got[3].Text.Block[0] != 3 || got[3].Text.Block[1] != 0 {
		t.Fatalf("nested match path = %#v", got[3].Text.Block)
	}
}

func TestDocumentFindOptions(t *testing.T) {
	doc := Document{Blocks: []Block{Paragraph{Content: []Inline{Text{Text: "Reader read README"}}}}}
	if got := doc.Find("read", FindOptions{WholeWord: true}); len(got) != 1 {
		t.Fatalf("whole-word matches = %d, want 1", len(got))
	}
	if got := doc.Find("REAd", FindOptions{CaseSensitive: true}); len(got) != 0 {
		t.Fatalf("case-sensitive matches = %d, want 0", len(got))
	}
}
