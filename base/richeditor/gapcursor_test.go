package richeditor

import "testing"

func TestGapCursorBeforeATable(t *testing.T) {
	v := kitSession(t, SessionConfig{Markdown: "| a | b |\n| --- | --- |\n| c | d |\n\nafter\n"})
	doc := v.State().Doc
	if !GapCursorValid(doc.Resolve(0)) {
		t.Fatal("no gap before a table that starts the document")
	}
	if GapCursorValid(doc.Resolve(doc.Child(0).NodeSize())) {
		t.Fatal("a gap between a table and a paragraph")
	}
	in := cellPos(doc, 0, 0) + 2
	v.Apply(v.State().Tr().SetSelection(TextSelectionAt(doc, in, in)))
	if !v.HandleKey("ArrowLeft") {
		t.Fatal("ArrowLeft did not reach the gap")
	}
	g, ok := v.State().Selection.(*GapCursor)
	if !ok || g.Head() != 0 {
		t.Fatalf("selection %T at %d", v.State().Selection, v.State().Selection.Head())
	}
	v.TypeText("x", nil)
	doc = v.State().Doc
	if doc.Child(0).Type.Name != "paragraph" || doc.Child(0).TextContent() != "x" || doc.Child(1).Type.Name != "table" {
		t.Fatalf("doc %s", doc)
	}
	if _, ok := v.State().Selection.(*TextSelection); !ok || v.State().Selection.Head() != 2 {
		t.Fatalf("selection %T at %d", v.State().Selection, v.State().Selection.Head())
	}
}

func TestGapCursorMapsAndOpens(t *testing.T) {
	v := kitSession(t, SessionConfig{Markdown: "| a |\n| --- |\n| b |\n"})
	st := v.State()
	v.Apply(st.Tr().SetSelection(NewGapCursor(st.Doc.Resolve(0))))
	if !v.State().Selection.Eq(gapBookmark{0}.Resolve(v.State().Doc)) {
		t.Fatal("the bookmark does not resolve to the gap")
	}
	if !v.OpenGap() {
		t.Fatal("not opened")
	}
	doc := v.State().Doc
	if doc.Child(0).Type.Name != "paragraph" || v.State().Selection.Head() != 1 {
		t.Fatalf("doc %s, head %d", doc, v.State().Selection.Head())
	}
	v.Run(Undo)
	if v.State().Doc.Child(0).Type.Name != "table" {
		t.Fatalf("undo left %s", v.State().Doc)
	}
}

func TestGapCursorClick(t *testing.T) {
	v := kitSession(t, SessionConfig{Markdown: "| a |\n| --- |\n| b |\n"})
	for _, p := range v.State().Plugins {
		if p.HandleClick != nil && p.HandleClick(v, 0, 1, false) {
			if _, ok := v.State().Selection.(*GapCursor); !ok {
				t.Fatalf("selection %T", v.State().Selection)
			}
			return
		}
	}
	t.Fatal("no plugin took a press in the gap")
}
