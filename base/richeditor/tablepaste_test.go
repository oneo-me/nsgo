package richeditor

import "testing"

func tableSession(t *testing.T, doc *Node, sel func(doc *Node) Selection) *Session {
	t.Helper()
	v, err := NewSession(SessionConfig{Schema: doc.Type.Schema, Doc: doc, Plugins: []*Plugin{TableEditing(), FixTables()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Apply(v.State().Tr().SetSelection(sel(doc))); err != nil {
		t.Fatal(err)
	}
	return v
}

func cells(doc *Node, ar, ac, hr, hc int) func(*Node) Selection {
	return func(*Node) Selection {
		return NewCellSelection(doc.Resolve(cellPos(doc, ar, ac)), doc.Resolve(cellPos(doc, hr, hc)))
	}
}

func TestPastedCellsReadsCellSelections(t *testing.T) {
	s := tableSchema(t)
	st := withCells(t, abcd(s), 0, 0, 1, 1)
	area := PastedCells(st.Selection.Content())
	if area == nil || area.Width != 2 || area.Height != 2 {
		t.Fatalf("area %+v", area)
	}
	if got := area.Rows[1].Child(1).TextContent(); got != "e" {
		t.Fatalf("cell %q", got)
	}
	if PastedCells(NewSlice(FragmentFrom(p(s, s.Text("x"))), 0, 0)) != nil {
		t.Fatal("a paragraph read as cells")
	}
}

func TestClipCellsRepeatsAndCuts(t *testing.T) {
	s := tableSchema(t)
	area := &CellArea{Width: 2, Height: 1, Rows: []*Fragment{FragmentFrom(tcell(s, "x"), tcell(s, "y"))}}
	got := ClipCells(area, 3, 2)
	if got.Width != 3 || got.Height != 2 {
		t.Fatalf("clipped %+v", got)
	}
	for _, row := range got.Rows {
		if row.ChildCount() != 3 || row.Child(2).TextContent() != "x" {
			t.Fatalf("row %s", row)
		}
	}
	wide := &CellArea{Width: 2, Height: 1, Rows: []*Fragment{FragmentFrom(tcell(s, "w", Attrs{"colspan": 2, "colwidth": []any{40.0, 60.0}}))}}
	cut := ClipCells(wide, 1, 1).Rows[0].Child(0)
	if colspan(cut) != 1 || len(colwidth(cut)) != 1 || colwidth(cut)[0] != 40 {
		t.Fatalf("cut %v", cut.Attrs)
	}
}

func TestPasteCellsOverACellSelection(t *testing.T) {
	s := tableSchema(t)
	doc := abcd(s)
	v := tableSession(t, doc, cells(doc, 0, 0, 1, 1))
	copied := withCells(t, abcd(s), 2, 2, 2, 2).Selection.Content()
	if !pasteIntoTable(v, copied) {
		t.Fatal("not pasted")
	}
	wantGrid(t, v.State(), []string{"i", "i", "c"}, []string{"i", "i", "f"}, []string{"g", "h", "i"})
	sel, ok := v.State().Selection.(*CellSelection)
	if !ok || len(sel.Ranges()) != 4 {
		t.Fatalf("selection %T %v", v.State().Selection, sel)
	}
}

func TestPasteTextOverACellSelectionFillsEachCell(t *testing.T) {
	s := tableSchema(t)
	doc := abcd(s)
	v := tableSession(t, doc, cells(doc, 1, 0, 1, 2))
	if !pasteIntoTable(v, NewSlice(FragmentFrom(p(s, s.Text("z"))), 1, 1)) {
		t.Fatal("not pasted")
	}
	wantGrid(t, v.State(), []string{"a", "b", "c"}, []string{"z", "z", "z"}, []string{"g", "h", "i"})
}

func TestPasteCellsGrowsTheTable(t *testing.T) {
	s := tableSchema(t)
	doc := abcd(s)
	v := tableSession(t, doc, func(doc *Node) Selection { return TextSelectionAt(doc, cellPos(doc, 2, 2)+2, cellPos(doc, 2, 2)+2) })
	copied := withCells(t, abcd(s), 0, 0, 1, 1).Selection.Content()
	if !pasteIntoTable(v, copied) {
		t.Fatal("not pasted")
	}
	wantGrid(t, v.State(),
		[]string{"a", "b", "c", ""},
		[]string{"d", "e", "f", ""},
		[]string{"g", "h", "a", "b"},
		[]string{"", "", "d", "e"})
}

func TestPasteCellsSplitsMergedCells(t *testing.T) {
	s := tableSchema(t)
	doc := s.Node("doc", nil, s.Node("table", nil,
		trow(s, tcell(s, "a", Attrs{"colspan": 2}), tcell(s, "b")),
		trow(s, tcell(s, "c"), tcell(s, "d"), tcell(s, "e"))))
	v := tableSession(t, doc, func(doc *Node) Selection {
		return NewCellSelection(doc.Resolve(cellPos(doc, 0, 2)), doc.Resolve(cellPos(doc, 1, 1)))
	})
	pasted := &CellArea{Width: 1, Height: 1, Rows: []*Fragment{FragmentFrom(tcell(s, "x"))}}
	if PastedCells(NewSlice(FragmentFrom(s.Node("tableRow", nil, pasted.Rows[0].Content...)), 0, 0)) == nil {
		t.Fatal("a row not read as cells")
	}
	if !pasteIntoTable(v, NewSlice(FragmentFrom(s.Node("tableRow", nil, tcell(s, "x"))), 0, 0)) {
		t.Fatal("not pasted")
	}
	wantGrid(t, v.State(), []string{"a", "x", "x"}, []string{"c", "x", "x"})
}

func TestPasteOutsideTablesIsNotTaken(t *testing.T) {
	s := tableSchema(t)
	doc := s.Node("doc", nil, p(s, s.Text("hi")), abcd(s).Child(0))
	v, err := NewSession(SessionConfig{Schema: s, Doc: doc, Plugins: []*Plugin{TableEditing()}})
	if err != nil {
		t.Fatal(err)
	}
	if pasteIntoTable(v, withCells(t, abcd(s), 0, 0, 0, 0).Selection.Content()) {
		t.Fatal("pasted outside a table")
	}
}

func TestCopiedCellsPasteThroughTheClipboard(t *testing.T) {
	v := kitSession(t, SessionConfig{Markdown: "| a | b |\n| --- | --- |\n| c | d |\n"})
	doc := v.State().Doc
	v.Apply(v.State().Tr().SetSelection(NewCellSelection(doc.Resolve(cellPos(doc, 0, 0)), doc.Resolve(cellPos(doc, 0, 1)))))
	text, html := v.Copy()
	v.Apply(v.State().Tr().SetSelection(NewCellSelection(doc.Resolve(cellPos(doc, 1, 0)), doc.Resolve(cellPos(doc, 1, 1)))))
	v.Paste(text, html)
	if got := grid(v.State().Doc.Child(0)); got[1][0] != "a" || got[1][1] != "b" || len(got) != 2 {
		t.Fatalf("grid %v from %q", got, html)
	}
}
