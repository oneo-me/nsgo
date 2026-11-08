package richeditor

import "testing"

func tableSchema(t *testing.T) *Schema {
	t.Helper()
	cellAttrs := []AttrSpec{{Name: "colspan", Default: 1, HasDefault: true}, {Name: "rowspan", Default: 1, HasDefault: true}, {Name: "colwidth", HasDefault: true}}
	s, err := NewSchema(SchemaSpec{Nodes: []NodeSpec{
		{Name: "doc", Content: "block+"},
		{Name: "paragraph", Content: "inline*", Group: "block"},
		{Name: "table", Content: "tableRow+", Group: "block", Isolating: true, Extra: map[string]any{"tableRole": "table"}},
		{Name: "tableRow", Content: "(tableCell | tableHeader)*", Attrs: []AttrSpec{{Name: "height", HasDefault: true}}, Extra: map[string]any{"tableRole": "row"}},
		{Name: "tableCell", Content: "block+", Isolating: true, Attrs: cellAttrs, Extra: map[string]any{"tableRole": "cell"}},
		{Name: "tableHeader", Content: "block+", Isolating: true, Attrs: cellAttrs, Extra: map[string]any{"tableRole": "header_cell"}},
		{Name: "text", Group: "inline"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func tcell(s *Schema, text string, attrs ...Attrs) *Node {
	var a Attrs
	if len(attrs) > 0 {
		a = attrs[0]
	}
	content := p(s)
	if text != "" {
		content = p(s, s.Text(text))
	}
	return s.Node("tableCell", a, content)
}

func trow(s *Schema, cells ...*Node) *Node { return s.Node("tableRow", nil, cells...) }

// grid is a table's cell texts by row.
func grid(table *Node) [][]string {
	var out [][]string
	for r := 0; r < table.ChildCount(); r++ {
		var row []string
		for c := 0; c < table.Child(r).ChildCount(); c++ {
			row = append(row, table.Child(r).Child(c).TextContent())
		}
		out = append(out, row)
	}
	return out
}

func wantGrid(t *testing.T, st *State, want ...[]string) {
	t.Helper()
	got := grid(st.Doc.Child(0))
	if len(got) != len(want) {
		t.Fatalf("grid %v, want %v", got, want)
	}
	for i := range got {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("grid %v, want %v", got, want)
		}
		for j := range got[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("grid %v, want %v", got, want)
			}
		}
	}
	if err := st.Doc.Check(); err != nil {
		t.Fatal(err)
	}
	if m := TableMapOf(st.Doc.Child(0)); len(m.problems) != 0 {
		t.Fatalf("problems %+v in %s", m.problems, st.Doc)
	}
}

func abcd(s *Schema) *Node {
	return s.Node("doc", nil, s.Node("table", nil,
		trow(s, tcell(s, "a"), tcell(s, "b"), tcell(s, "c")),
		trow(s, tcell(s, "d"), tcell(s, "e"), tcell(s, "f")),
		trow(s, tcell(s, "g"), tcell(s, "h"), tcell(s, "i"))))
}

// cellPos is the position before row and col's cell of the first table.
func cellPos(doc *Node, row, col int) int {
	m := TableMapOf(doc.Child(0))
	return 1 + m.Map[row*m.Width+col]
}

func withCells(t *testing.T, doc *Node, ar, ac, hr, hc int, plugins ...*Plugin) *State {
	t.Helper()
	st, err := NewState(StateConfig{Doc: doc, Plugins: plugins,
		Selection: NewCellSelection(doc.Resolve(cellPos(doc, ar, ac)), doc.Resolve(cellPos(doc, hr, hc)))})
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestTableMapAndSpans(t *testing.T) {
	s := tableSchema(t)
	doc := s.Node("doc", nil, s.Node("table", nil,
		trow(s, tcell(s, "a", Attrs{"colspan": 2}), tcell(s, "b", Attrs{"rowspan": 2})),
		trow(s, tcell(s, "c"), tcell(s, "d"))))
	table := doc.Child(0)
	m := TableMapOf(table)
	if m.Width != 3 || m.Height != 2 || len(m.problems) != 0 {
		t.Fatalf("map %+v", m)
	}
	if m.Map[0] != m.Map[1] || m.Map[2] != m.Map[5] {
		t.Fatalf("spans %v", m.Map)
	}
	if r := m.FindCell(m.Map[0]); r != (Rect{0, 0, 2, 1}) {
		t.Fatalf("rect %+v", r)
	}
	if TableMapOf(table) != m {
		t.Fatal("the map is not cached")
	}
	if got := m.NextCell(m.Map[3], "horiz", 1); got != m.Map[4] {
		t.Fatalf("next %d", got)
	}
}

func TestTableAddAndDelete(t *testing.T) {
	s := tableSchema(t)
	st := run(t, withCells(t, abcd(s), 1, 1, 1, 1), AddColumnAfter)
	wantGrid(t, st, []string{"a", "b", "", "c"}, []string{"d", "e", "", "f"}, []string{"g", "h", "", "i"})
	st = run(t, withCells(t, abcd(s), 1, 1, 1, 1), AddRowBefore)
	wantGrid(t, st, []string{"a", "b", "c"}, []string{"", "", ""}, []string{"d", "e", "f"}, []string{"g", "h", "i"})
	st = run(t, withCells(t, abcd(s), 0, 0, 1, 1), DeleteColumn)
	wantGrid(t, st, []string{"c"}, []string{"f"}, []string{"i"})
	st = run(t, withCells(t, abcd(s), 0, 0, 1, 0), DeleteRow)
	wantGrid(t, st, []string{"g", "h", "i"})
	if DeleteRow(withCells(t, abcd(s), 0, 0, 2, 0), nil) {
		t.Fatal("deleted every row")
	}
}

func TestTableMergeAndSplit(t *testing.T) {
	s := tableSchema(t)
	st := run(t, withCells(t, abcd(s), 0, 0, 1, 1, FixTables()), MergeCells)
	table := st.Doc.Child(0)
	first := table.Child(0).Child(0)
	if colspan(first) != 2 || rowspan(first) != 2 {
		t.Fatalf("merged %s %v", first, first.Attrs)
	}
	if got := first.TextContent(); got != "abde" {
		t.Fatalf("merged text %q", got)
	}
	if err := st.Doc.Check(); err != nil {
		t.Fatal(err)
	}
	if len(TableMapOf(table).problems) != 0 {
		t.Fatalf("problems %+v", TableMapOf(table).problems)
	}
	st = run(t, st, SplitCell)
	wantGrid(t, st, []string{"abde", "", "c"}, []string{"", "", "f"}, []string{"g", "h", "i"})
}

func TestTableMovesRowsAndColumns(t *testing.T) {
	s := tableSchema(t)
	st := run(t, withCells(t, abcd(s), 0, 0, 0, 0), MoveTableRow(0, 2, -1))
	wantGrid(t, st, []string{"d", "e", "f"}, []string{"g", "h", "i"}, []string{"a", "b", "c"})
	if c, ok := st.Selection.(*CellSelection); !ok || !c.IsRowSelection() {
		t.Fatalf("selection %T", st.Selection)
	}
	st = run(t, withCells(t, abcd(s), 0, 0, 0, 0), MoveTableColumn(2, 0, -1))
	wantGrid(t, st, []string{"c", "a", "b"}, []string{"f", "d", "e"}, []string{"i", "g", "h"})
	if c, ok := st.Selection.(*CellSelection); !ok || !c.IsColSelection() {
		t.Fatalf("selection %T", st.Selection)
	}
	// A merged cell moves its whole span, and a target in it is refused.
	merged := s.Node("doc", nil, s.Node("table", nil,
		trow(s, tcell(s, "a", Attrs{"colspan": 2}), tcell(s, "b")),
		trow(s, tcell(s, "c"), tcell(s, "d"), tcell(s, "e"))))
	st = run(t, withCells(t, merged, 1, 0, 1, 0), MoveTableColumn(0, 2, -1))
	wantGrid(t, st, []string{"b", "a"}, []string{"e", "c", "d"})
	if MoveTableColumn(0, 1, -1)(withCells(t, merged, 1, 0, 1, 0), nil) {
		t.Fatal("moved a column inside its span")
	}
}

func TestTableWidthsAndHeights(t *testing.T) {
	s := tableSchema(t)
	st := run(t, withCells(t, abcd(s), 0, 0, 0, 0), SetColumnWidth(1, 90, -1))
	if w := ColumnWidths(st.Doc.Child(0)); len(w) != 3 || w[1] != 90 || w[0] != 0 {
		t.Fatalf("widths %v", w)
	}
	st = run(t, st, SetRowHeight(1, 44, -1))
	if h := st.Doc.Child(0).Child(1).AttrNum("height"); h != 44 {
		t.Fatalf("height %v", h)
	}
	if len(TableMapOf(st.Doc.Child(0)).problems) != 0 {
		t.Fatal("widths disagree")
	}
}

func TestTableSelectionsAndKeys(t *testing.T) {
	s := tableSchema(t)
	doc := abcd(s)
	st := withCells(t, doc, 0, 0, 1, 1)
	sel := st.Selection.(*CellSelection)
	if len(sel.Ranges()) != 4 || sel.IsRowSelection() {
		t.Fatalf("ranges %d", len(sel.Ranges()))
	}
	if c := sel.Content(); c.Content.Child(0).ChildCount() != 2 {
		t.Fatalf("content %s", c.Content)
	}
	// Typing over cells fills the first and empties the others.
	n, err := st.Apply(st.Tr().ReplaceSelection(NewSlice(FragmentFrom(s.Text("x")), 0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	wantGrid(t, n, []string{"", "", "c"}, []string{"", "x", "f"}, []string{"g", "h", "i"})
	st = run(t, withCells(t, doc, 0, 0, 1, 1), DeleteCellSelection)
	wantGrid(t, st, []string{"", "", "c"}, []string{"", "", "f"}, []string{"g", "h", "i"})
	// Tab walks the cells, and past the last adds a row.
	at := stateAt(t, doc, cellPos(doc, 0, 0)+2, cellPos(doc, 0, 0)+2)
	at = run(t, at, GoToNextCell(1))
	if got := at.Doc.TextBetween(at.Selection.From(), at.Selection.To(), "", ""); got != "b" {
		t.Fatalf("next cell selected %q", got)
	}
	end := stateAt(t, doc, cellPos(doc, 2, 2)+2, cellPos(doc, 2, 2)+2)
	end = run(t, end, GoToNextCellOrAddRow)
	if end.Doc.Child(0).ChildCount() != 4 {
		t.Fatalf("no row added: %s", end.Doc)
	}
	// A drag across two cells selects cells.
	st, err = NewState(StateConfig{Doc: doc, Plugins: []*Plugin{TableEditing()}})
	if err != nil {
		t.Fatal(err)
	}
	n, err = st.Apply(st.Tr().SetSelection(TextSelectionAt(doc, cellPos(doc, 0, 0)+2, cellPos(doc, 1, 1)+2)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := n.Selection.(*CellSelection); !ok {
		t.Fatalf("selection %T", n.Selection)
	}
	st = run(t, withCells(t, doc, 0, 0, 0, 0), ToggleHeaderRow)
	if st.Doc.Child(0).Child(0).Child(2).Type.Name != "tableHeader" || st.Doc.Child(0).Child(1).Child(0).Type.Name == "tableHeader" {
		t.Fatalf("header row: %s", st.Doc)
	}
	st = run(t, st, ToggleHeaderRow)
	if st.Doc.Child(0).Child(0).Child(0).Type.Name != "tableCell" {
		t.Fatalf("header row off: %s", st.Doc)
	}
}

func TestFixTables(t *testing.T) {
	s := tableSchema(t)
	bad := s.Node("doc", nil, s.Node("table", nil, trow(s, tcell(s, "a"), tcell(s, "b")), trow(s, tcell(s, "c"))))
	st := stateAt(t, bad, 3, 3, FixTables())
	n, _, err := st.ApplyTransaction(st.Tr().Insert(3, s.Text("x")))
	if err != nil {
		t.Fatal(err)
	}
	m := TableMapOf(n.Doc.Child(0))
	if len(m.problems) != 0 || n.Doc.Child(0).Child(1).ChildCount() != 2 {
		t.Fatalf("fixed %s %+v", n.Doc, m.problems)
	}
}
