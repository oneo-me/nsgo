package richeditor

import "testing"

func TestDragSourceIsTheBlockOrItsDefiningParent(t *testing.T) {
	v := kitSession(t, SessionConfig{Markdown: "one\n\n- a\n- b\n\nend\n"})
	doc := v.State().Doc
	from, to, ok := DragSource(v.State(), 2)
	if !ok || from != 0 || to != doc.Child(0).NodeSize() {
		t.Fatalf("paragraph source %d..%d %v", from, to, ok)
	}
	list := doc.Child(0).NodeSize()
	in := list + 3 // in the first item's paragraph
	from, to, ok = DragSource(v.State(), in)
	if !ok || from != list+1 || to != list+1+doc.Child(1).Child(0).NodeSize() {
		t.Fatalf("item source %d..%d %v in %s", from, to, ok, doc)
	}
	from, to, _ = DragSource(v.State(), list)
	if from != list || to != list+doc.Child(1).NodeSize() {
		t.Fatalf("a position before the list carries %d..%d", from, to)
	}
}

func TestDragSourceTakesTheSelectedSiblings(t *testing.T) {
	v := kitSession(t, SessionConfig{Markdown: "one\n\ntwo\n\nthree\n"})
	doc := v.State().Doc
	v.Apply(v.State().Tr().SetSelection(TextSelectionAt(doc, 2, 7)))
	from, to, _ := DragSource(v.State(), 3)
	if from != 0 || to != doc.Child(0).NodeSize()+doc.Child(1).NodeSize() {
		t.Fatalf("source %d..%d", from, to)
	}
}

func TestMoveBlocksIsOneUndoStep(t *testing.T) {
	v := kitSession(t, SessionConfig{Markdown: "one\n\ntwo\n\nthree\n"})
	doc := v.State().Doc
	first := doc.Child(0).NodeSize()
	if v.Can(MoveBlocks(0, first, 2)) || v.Can(MoveBlocks(0, first, first)) {
		t.Fatal("moved inside itself or to where it is")
	}
	if !v.Run(MoveBlocks(0, first, doc.Content.Size)) {
		t.Fatal("not moved")
	}
	if got := v.Text(); got != "two\n\nthree\n\none" {
		t.Fatalf("moved to %q", got)
	}
	if sel, ok := v.State().Selection.(*NodeSelection); !ok || sel.Node.TextContent() != "one" {
		t.Fatalf("selection %T", v.State().Selection)
	}
	v.Run(Undo)
	if got := v.Text(); got != "one\n\ntwo\n\nthree" {
		t.Fatalf("undo left %q", got)
	}
}

func TestMoveBlocksRefusesWhatTheParentCannotTake(t *testing.T) {
	v := kitSession(t, SessionConfig{Markdown: "- a\n- b\n\nend\n"})
	doc := v.State().Doc
	item := doc.Child(0).Child(0)
	if v.Can(MoveBlocks(1, 1+item.NodeSize(), doc.Content.Size)) {
		t.Fatal("a list item dropped in the document")
	}
	if !v.Run(MoveBlocks(1, 1+item.NodeSize(), doc.Child(0).NodeSize()-1)) {
		t.Fatal("not moved within its list")
	}
	if got := v.Markdown(); got != "- b\n- a\n\nend\n" {
		t.Fatalf("moved to %q", got)
	}
}
