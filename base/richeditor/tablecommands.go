package richeditor

import (
	"slices"
)

// selectedRect is the selected cells' rectangle with their table.
type selectedRect struct {
	Rect
	tableStart int
	m          *TableMap
	table      *Node
}

func selectionCell(s *State) *ResolvedPos {
	switch sel := s.Selection.(type) {
	case *CellSelection:
		if sel.AnchorCell.Pos > sel.HeadCell.Pos {
			return sel.AnchorCell
		}
		return sel.HeadCell
	case *NodeSelection:
		if isCellType(sel.Node.Type) {
			return sel.AnchorPos()
		}
	}
	if c := cellAround(s.Selection.HeadPos()); c != nil {
		return c
	}
	return cellAround(s.Selection.FromPos())
}

func getSelectedRect(s *State) *selectedRect {
	pos := selectionCell(s)
	table := pos.Node(-1)
	start := pos.Start(-1)
	m := TableMapOf(table)
	var rect Rect
	if c, ok := s.Selection.(*CellSelection); ok {
		rect = m.RectBetween(c.AnchorCell.Pos-start, c.HeadCell.Pos-start)
	} else {
		rect = m.FindCell(pos.Pos - start)
	}
	return &selectedRect{rect, start, m, table}
}

func withColspan(attrs Attrs, pos, n int) Attrs {
	out := copyAttrs(attrs)
	cs := max(int(num(attrs["colspan"])), 1)
	out["colspan"] = float64(cs + n)
	if widths, ok := attrs["colwidth"].([]any); ok {
		w := slices.Clone(widths)
		for len(w) < cs {
			w = append(w, 0.0)
		}
		w = slices.Insert(w, min(pos, len(w)), make([]any, n)...)
		for i := range w {
			if w[i] == nil {
				w[i] = 0.0
			}
		}
		out["colwidth"] = w
	}
	return out
}

func withoutColspan(attrs Attrs, pos int) Attrs {
	out := copyAttrs(attrs)
	out["colspan"] = float64(int(num(attrs["colspan"])) - 1)
	if widths, ok := attrs["colwidth"].([]any); ok {
		w := slices.Clone(widths)
		if pos < len(w) {
			w = slices.Delete(w, pos, pos+1)
		}
		if slices.ContainsFunc(w, func(v any) bool { return num(v) > 0 }) {
			out["colwidth"] = w
		} else {
			out["colwidth"] = nil
		}
	}
	return out
}

func num(v any) float64 {
	f, _ := normalize(v).(float64)
	return f
}

func columnIsHeader(m *TableMap, table *Node, col int) bool {
	header := tableNodeTypes(table.Type.Schema).header
	for row := 0; row < m.Height; row++ {
		if table.NodeAt(m.Map[col+row*m.Width]).Type != header {
			return false
		}
	}
	return true
}

func rowIsHeader(m *TableMap, table *Node, row int) bool {
	header := tableNodeTypes(table.Type.Schema).header
	for col := 0; col < m.Width; col++ {
		if table.NodeAt(m.Map[col+row*m.Width]).Type != header {
			return false
		}
	}
	return true
}

func addColumn(tr *Transaction, r *selectedRect, col int) {
	m, table, start := r.m, r.table, r.tableStart
	ref := 0
	useRef := true
	if col > 0 {
		ref = -1
	}
	if columnIsHeader(m, table, col+ref) {
		if col == 0 || col == m.Width {
			useRef = false
		} else {
			ref = 0
		}
	}
	for row := 0; row < m.Height; row++ {
		index := row*m.Width + col
		if col > 0 && col < m.Width && m.Map[index-1] == m.Map[index] {
			pos := m.Map[index]
			cell := table.NodeAt(pos)
			tr.SetNodeMarkup(tr.Mapping.Map(start+pos, 1), nil, withColspan(cell.Attrs, col-m.ColCount(pos), 1), nil)
			row += rowspan(cell) - 1
		} else {
			t := tableNodeTypes(table.Type.Schema).cell
			if useRef {
				t = table.NodeAt(m.Map[index+ref]).Type
			}
			tr.Insert(tr.Mapping.Map(start+m.PositionAt(row, col, table), 1), t.CreateAndFill(nil, nil, nil))
		}
	}
}

func removeColumn(tr *Transaction, r *selectedRect, col int) {
	m, table, start := r.m, r.table, r.tableStart
	mapStart := len(tr.Mapping.Maps)
	for row := 0; row < m.Height; {
		index := row*m.Width + col
		pos := m.Map[index]
		cell := table.NodeAt(pos)
		mapping := tr.Mapping.Slice(mapStart, len(tr.Mapping.Maps))
		if col > 0 && m.Map[index-1] == pos || col < m.Width-1 && m.Map[index+1] == pos {
			tr.SetNodeMarkup(mapping.Map(start+pos, 1), nil, withoutColspan(cell.Attrs, col-m.ColCount(pos)), nil)
		} else {
			from := mapping.Map(start+pos, 1)
			tr.Delete(from, from+cell.NodeSize())
		}
		row += rowspan(cell)
	}
}

func addRow(tr *Transaction, r *selectedRect, row int) {
	m, table, start := r.m, r.table, r.tableStart
	rowPos := start
	for i := range row {
		rowPos += table.Child(i).NodeSize()
	}
	var cells []*Node
	types := tableNodeTypes(table.Type.Schema)
	ref, useRef := 0, true
	if row > 0 {
		ref = -1
	}
	if rowIsHeader(m, table, row+ref) {
		if row == 0 || row == m.Height {
			useRef = false
		} else {
			ref = 0
		}
	}
	for col, index := 0, m.Width*row; col < m.Width; col, index = col+1, index+1 {
		if row > 0 && row < m.Height && m.Map[index] == m.Map[index-m.Width] {
			pos := m.Map[index]
			cell := table.NodeAt(pos)
			attrs := copyAttrs(cell.Attrs)
			attrs["rowspan"] = float64(rowspan(cell) + 1)
			tr.SetNodeMarkup(start+pos, nil, attrs, nil)
			col += colspan(cell) - 1
			index += colspan(cell) - 1
		} else {
			t := types.cell
			if useRef {
				t = table.NodeAt(m.Map[index+ref*m.Width]).Type
			}
			cells = append(cells, t.CreateAndFill(nil, nil, nil))
		}
	}
	tr.Insert(tr.Mapping.Map(rowPos, 1), types.row.Create(nil, FragmentFrom(cells...), nil))
}

func removeRow(tr *Transaction, r *selectedRect, row int) {
	m, table, start := r.m, r.table, r.tableStart
	rowPos := 0
	for i := range row {
		rowPos += table.Child(i).NodeSize()
	}
	next := rowPos + table.Child(row).NodeSize()
	mapFrom := len(tr.Mapping.Maps)
	tr.Delete(rowPos+start, next+start)
	seen := map[int]bool{}
	for col, index := 0, row*m.Width; col < m.Width; col, index = col+1, index+1 {
		pos := m.Map[index]
		if seen[pos] {
			continue
		}
		seen[pos] = true
		mapping := tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps))
		if row > 0 && pos == m.Map[index-m.Width] {
			cell := table.NodeAt(pos)
			attrs := copyAttrs(cell.Attrs)
			attrs["rowspan"] = float64(rowspan(cell) - 1)
			tr.SetNodeMarkup(mapping.Map(pos+start, 1), nil, attrs, nil)
		} else if row < m.Height-1 && pos == m.Map[index+m.Width] {
			cell := table.NodeAt(pos)
			attrs := copyAttrs(cell.Attrs)
			attrs["rowspan"] = float64(rowspan(cell) - 1)
			at := m.PositionAt(row+1, col, table)
			tr.Insert(mapping.Map(start+at, 1), cell.Type.Create(attrs, cell.Content, nil))
		}
	}
}

func tableCommand(run func(s *State, r *selectedRect, tr *Transaction) bool) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if !isInTable(s) {
			return false
		}
		tr := s.Tr()
		if !run(s, getSelectedRect(s), tr) {
			return false
		}
		if dispatch != nil {
			dispatch(tr)
		}
		return true
	}
}

// AddColumnBefore and the commands after it edit the table at the selection,
// by Tiptap's names.
var (
	AddColumnBefore = tableCommand(func(_ *State, r *selectedRect, tr *Transaction) bool {
		addColumn(tr, r, r.Left)
		return true
	})
	AddColumnAfter = tableCommand(func(_ *State, r *selectedRect, tr *Transaction) bool {
		addColumn(tr, r, r.Right)
		return true
	})
	AddRowBefore = tableCommand(func(_ *State, r *selectedRect, tr *Transaction) bool {
		addRow(tr, r, r.Top)
		return true
	})
	AddRowAfter = tableCommand(func(_ *State, r *selectedRect, tr *Transaction) bool {
		addRow(tr, r, r.Bottom)
		return true
	})
	// DeleteColumn and DeleteRow refuse to delete every column or row.
	DeleteColumn = tableCommand(func(s *State, r *selectedRect, tr *Transaction) bool {
		if r.Left == 0 && r.Right == r.m.Width {
			return false
		}
		for i := r.Right - 1; ; i-- {
			removeColumn(tr, r, i)
			if i == r.Left {
				break
			}
			table := tr.Doc.Resolve(r.tableStart - 1).NodeAfter()
			if table == nil {
				return false
			}
			r = &selectedRect{r.Rect, r.tableStart, TableMapOf(table), table}
		}
		return true
	})
	DeleteRow = tableCommand(func(s *State, r *selectedRect, tr *Transaction) bool {
		if r.Top == 0 && r.Bottom == r.m.Height {
			return false
		}
		for i := r.Bottom - 1; ; i-- {
			removeRow(tr, r, i)
			if i == r.Top {
				break
			}
			table := tr.Doc.Resolve(r.tableStart - 1).NodeAfter()
			if table == nil {
				return false
			}
			r = &selectedRect{r.Rect, r.tableStart, TableMapOf(table), table}
		}
		return true
	})
)

func isEmptyCell(cell *Node) bool {
	for c := cell.Content; c.ChildCount() == 1; {
		only := c.FirstChild()
		if only.IsTextblock() && only.Content.Size == 0 {
			return true
		}
		if only.IsLeaf() {
			return false
		}
		c = only.Content
	}
	return cell.Content.Size == 0
}

func cellsOverlapRect(m *TableMap, r Rect) bool {
	indexTop := r.Top*m.Width + r.Left
	indexLeft := indexTop
	indexBottom := (r.Bottom-1)*m.Width + r.Left
	indexRight := indexTop + (r.Right - r.Left - 1)
	for i := r.Top; i < r.Bottom; i++ {
		if r.Left > 0 && m.Map[indexLeft] == m.Map[indexLeft-1] || r.Right < m.Width && m.Map[indexRight] == m.Map[indexRight+1] {
			return true
		}
		indexLeft += m.Width
		indexRight += m.Width
	}
	for i := r.Left; i < r.Right; i++ {
		if r.Top > 0 && m.Map[indexTop] == m.Map[indexTop-m.Width] || r.Bottom < m.Height && m.Map[indexBottom] == m.Map[indexBottom+m.Width] {
			return true
		}
		indexTop++
		indexBottom++
	}
	return false
}

// MergeCells merges the selected cells, their contents joined.
func MergeCells(s *State, dispatch func(*Transaction)) bool {
	sel, ok := s.Selection.(*CellSelection)
	if !ok || sel.AnchorCell.Pos == sel.HeadCell.Pos {
		return false
	}
	r := getSelectedRect(s)
	if cellsOverlapRect(r.m, r.Rect) {
		return false
	}
	if dispatch == nil {
		return true
	}
	tr := s.Tr()
	seen := map[int]bool{}
	content := emptyFragment
	mergedPos := -1
	var merged *Node
	for row := r.Top; row < r.Bottom; row++ {
		for col := r.Left; col < r.Right; col++ {
			pos := r.m.Map[row*r.m.Width+col]
			cell := r.table.NodeAt(pos)
			if seen[pos] || cell == nil {
				continue
			}
			seen[pos] = true
			if mergedPos < 0 {
				mergedPos, merged = pos, cell
				continue
			}
			if !isEmptyCell(cell) {
				content = content.Append(cell.Content)
			}
			mapped := tr.Mapping.Map(pos+r.tableStart, 1)
			tr.Delete(mapped, mapped+cell.NodeSize())
		}
	}
	attrs := withColspan(merged.Attrs, colspan(merged), r.Right-r.Left-colspan(merged))
	attrs["rowspan"] = float64(r.Bottom - r.Top)
	tr.SetNodeMarkup(mergedPos+r.tableStart, nil, attrs, nil)
	if content.Size > 0 {
		end := mergedPos + 1 + merged.Content.Size
		start := end
		if isEmptyCell(merged) {
			start = mergedPos + 1
		}
		tr.ReplaceWith(start+r.tableStart, end+r.tableStart, content.Content...)
	}
	tr.SetSelection(NewCellSelection(tr.Doc.Resolve(mergedPos+r.tableStart), nil))
	dispatch(tr)
	return true
}

// SplitCell splits a merged cell into cells of one slot.
func SplitCell(s *State, dispatch func(*Transaction)) bool {
	var cell *Node
	cellPos := -1
	if sel, ok := s.Selection.(*CellSelection); ok {
		if sel.AnchorCell.Pos != sel.HeadCell.Pos {
			return false
		}
		cell, cellPos = sel.AnchorCell.NodeAfter(), sel.AnchorCell.Pos
	} else if around := cellAround(s.Selection.FromPos()); around != nil {
		cell, cellPos = around.NodeAfter(), around.Pos
	}
	if cell == nil || colspan(cell) == 1 && rowspan(cell) == 1 {
		return false
	}
	if dispatch == nil {
		return true
	}
	base := copyAttrs(cell.Attrs)
	base["rowspan"], base["colspan"] = 1.0, 1.0
	widths := colwidth(cell)
	r := getSelectedRect(s)
	tr := s.Tr()
	attrs := make([]Attrs, r.Right-r.Left)
	for i := range attrs {
		attrs[i] = base
		if widths != nil {
			a := copyAttrs(base)
			a["colwidth"] = nil
			if i < len(widths) && widths[i] != 0 {
				a["colwidth"] = []any{widths[i]}
			}
			attrs[i] = a
		}
	}
	last := -1
	for row := r.Top; row < r.Bottom; row++ {
		pos := r.m.PositionAt(row, r.Left, r.table)
		if row == r.Top {
			pos += cell.NodeSize()
		}
		for col, i := r.Left, 0; col < r.Right; col, i = col+1, i+1 {
			if col == r.Left && row == r.Top {
				continue
			}
			last = tr.Mapping.Map(pos+r.tableStart, 1)
			tr.Insert(last, cell.Type.CreateAndFill(attrs[i], nil, nil))
		}
	}
	tr.SetNodeMarkup(cellPos, cell.Type, attrs[0], nil)
	if sel, ok := s.Selection.(*CellSelection); ok {
		head := tr.Doc.Resolve(sel.AnchorCell.Pos)
		if last >= 0 {
			head = tr.Doc.Resolve(last)
		}
		tr.SetSelection(NewCellSelection(tr.Doc.Resolve(sel.AnchorCell.Pos), head))
	}
	dispatch(tr)
	return true
}

func headerEnabled(kind string, r *selectedRect, types tableTypes) bool {
	rect := Rect{0, 0, 1, 1}
	if kind == "row" {
		rect.Right = r.m.Width
	} else {
		rect.Bottom = r.m.Height
	}
	for _, pos := range r.m.CellsInRect(rect) {
		if cell := r.table.NodeAt(pos); cell != nil && cell.Type != types.header {
			return false
		}
	}
	return true
}

func toggleHeader(kind string) Command {
	return tableCommand(func(s *State, r *selectedRect, tr *Transaction) bool {
		types := tableNodeTypes(s.Schema)
		rowOn, colOn := headerEnabled("row", r, types), headerEnabled("column", r, types)
		from := 0
		var rect Rect
		newType := types.cell
		switch kind {
		case "column":
			from = boolInt(rowOn)
			rect = Rect{0, from, 1, r.m.Height}
			if !colOn {
				newType = types.header
			}
		case "row":
			from = boolInt(colOn)
			rect = Rect{from, 0, r.m.Width, 1}
			if !rowOn {
				newType = types.header
			}
		default:
			rect = r.Rect
			for _, pos := range r.m.CellsInRect(rect) {
				if cell := r.table.NodeAt(pos); cell != nil && cell.Type != types.header {
					newType = types.header
				}
			}
		}
		for _, rel := range r.m.CellsInRect(rect) {
			pos := rel + r.tableStart
			if cell := tr.Doc.NodeAt(pos); cell != nil {
				tr.SetNodeMarkup(pos, newType, cell.Attrs, nil)
			}
		}
		return true
	})
}

// ToggleHeaderRow, ToggleHeaderColumn and ToggleHeaderCell turn cells to
// header cells and back.
var (
	ToggleHeaderRow    = toggleHeader("row")
	ToggleHeaderColumn = toggleHeader("column")
	ToggleHeaderCell   = toggleHeader("cell")
)

func findNextCell(cell *ResolvedPos, dir int) int {
	if dir < 0 {
		if before := cell.NodeBefore(); before != nil {
			return cell.Pos - before.NodeSize()
		}
		for row, rowEnd := cell.Index(-1)-1, cell.Before(cell.Depth); row >= 0; row-- {
			rowNode := cell.Node(-1).Child(row)
			if last := rowNode.LastChild(); last != nil {
				return rowEnd - 1 - last.NodeSize()
			}
			rowEnd -= rowNode.NodeSize()
		}
		return -1
	}
	if cell.Index(cell.Depth) < cell.Parent().ChildCount()-1 {
		return cell.Pos + cell.NodeAfter().NodeSize()
	}
	table := cell.Node(-1)
	for row, rowStart := cell.IndexAfter(-1), cell.After(cell.Depth); row < table.ChildCount(); row++ {
		rowNode := table.Child(row)
		if rowNode.ChildCount() > 0 {
			return rowStart + 1
		}
		rowStart += rowNode.NodeSize()
	}
	return -1
}

// GoToNextCell moves to the next cell, or the previous when dir is
// negative.
func GoToNextCell(dir int) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if !isInTable(s) {
			return false
		}
		cell := findNextCell(selectionCell(s), dir)
		if cell < 0 {
			return false
		}
		if dispatch != nil {
			r := s.Doc.Resolve(cell)
			dispatch(s.Tr().SetSelection(TextSelectionBetween(r, s.Doc.Resolve(cell+r.NodeAfter().NodeSize()), 0)).ScrollIntoView())
		}
		return true
	}
}

// GoToNextCellOrAddRow is Tab: past the last cell a new row.
func GoToNextCellOrAddRow(s *State, dispatch func(*Transaction)) bool {
	if GoToNextCell(1)(s, dispatch) {
		return true
	}
	if !isInTable(s) {
		return false
	}
	if dispatch == nil {
		return true
	}
	AddRowAfter(s, func(tr *Transaction) {
		n, err := s.Apply(tr)
		if err != nil {
			return
		}
		if GoToNextCell(1)(n, func(next *Transaction) {
			tr.SetSelection(next.Selection().Map(tr.Doc, NewMapping()))
		}) {
			dispatch(tr)
		}
	})
	return true
}

// DeleteTable deletes the table at the selection.
func DeleteTable(s *State, dispatch func(*Transaction)) bool {
	r := s.Selection.AnchorPos()
	for d := r.Depth; d > 0; d-- {
		if tableRole(r.Node(d).Type) == "table" {
			if dispatch != nil {
				dispatch(s.Tr().Delete(r.Before(d), r.After(d)).ScrollIntoView())
			}
			return true
		}
	}
	return false
}

// InsertTable inserts a table of rows and cols, a header row with
// withHeader, its first cell selected.
func InsertTable(rows, cols int, withHeader bool) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		types := tableNodeTypes(s.Schema)
		if types.table == nil {
			return false
		}
		if dispatch == nil {
			return true
		}
		kids := make([]*Node, rows)
		for i := range kids {
			cells := make([]*Node, cols)
			for j := range cells {
				t := types.cell
				if withHeader && i == 0 {
					t = types.header
				}
				cells[j] = t.CreateAndFill(nil, nil, nil)
			}
			kids[i] = types.row.Create(nil, FragmentFrom(cells...), nil)
		}
		table := types.table.Create(nil, FragmentFrom(kids...), nil)
		tr := s.Tr().ReplaceSelectionWith(table, false)
		at := tr.Mapping.Map(s.Selection.From(), -1)
		tr.SetSelection(SelectionFindFromOrNear(tr.Doc.Resolve(at), 1))
		dispatch(tr.ScrollIntoView())
		return true
	}
}

// SetCellSelection selects from the cell at anchor to the one at head.
func SetCellSelection(anchor, head int) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if dispatch != nil {
			dispatch(s.Tr().SetSelection(NewCellSelection(s.Doc.Resolve(anchor), s.Doc.Resolve(head))))
		}
		return true
	}
}

// tableAt is the table at tablePos, the position before it, or around the
// selection when negative, with its position.
func tableAt(s *State, tablePos int) (*Node, int) {
	if tablePos >= 0 {
		if n := s.Doc.NodeAt(tablePos); n != nil && tableRole(n.Type) == "table" {
			return n, tablePos
		}
		return nil, -1
	}
	if !isInTable(s) {
		return nil, -1
	}
	cell := selectionCell(s)
	return cell.Node(-1), cell.Before(cell.Depth - 1)
}

// spanRange is the smallest span around index that no cell crosses, of
// rows or columns.
func spanRange(m *TableMap, index int, rows bool) (int, int) {
	lines, across := m.Width, m.Height
	if rows {
		lines, across = m.Height, m.Width
	}
	crosses := func(i int) bool {
		for j := range across {
			a, b := m.Map[j*m.Width+i-1], m.Map[j*m.Width+i]
			if rows {
				a, b = m.Map[(i-1)*m.Width+j], m.Map[i*m.Width+j]
			}
			if a == b {
				return true
			}
		}
		return false
	}
	from, to := index, index+1
	for from > 0 && crosses(from) {
		from--
	}
	for to < lines && crosses(to) {
		to++
	}
	return from, to
}

// moveOrder is the lines' new order, moving the span of from to the place of
// the span of to; nil when to is inside the moved span.
func moveOrder(m *TableMap, from, to int, rows bool) (order []int, start, count int) {
	lines := m.Width
	if rows {
		lines = m.Height
	}
	if from < 0 || to < 0 || from >= lines || to >= lines {
		return nil, 0, 0
	}
	a, b := spanRange(m, from, rows)
	c, d := spanRange(m, to, rows)
	if to >= a && to < b {
		return nil, 0, 0
	}
	moved := make([]int, 0, b-a)
	var rest []int
	for i := range lines {
		if i >= a && i < b {
			moved = append(moved, i)
		} else {
			rest = append(rest, i)
		}
	}
	at := slices.Index(rest, c)
	if to > from {
		at = slices.Index(rest, d-1) + 1
	}
	return slices.Insert(rest, at, moved...), at, len(moved)
}

// MoveTableRow moves row from to the place of row to; table is the position
// before a table, the selection's when negative. The moved rows end up
// selected.
func MoveTableRow(from, to, tablePos int) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		table, pos := tableAt(s, tablePos)
		if table == nil {
			return false
		}
		m := TableMapOf(table)
		order, start, count := moveOrder(m, from, to, true)
		if order == nil {
			return false
		}
		if dispatch != nil {
			rows := make([]*Node, len(order))
			for i, o := range order {
				rows[i] = table.Child(o)
			}
			finishMove(s, dispatch, table.Copy(FragmentFrom(rows...)), table, pos, func(n *TableMap) (int, int) {
				return n.Map[start*n.Width], n.Map[(start+count)*n.Width-1]
			})
		}
		return true
	}
}

// MoveTableColumn moves a column, as MoveTableRow a row.
func MoveTableColumn(from, to, tablePos int) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		table, pos := tableAt(s, tablePos)
		if table == nil {
			return false
		}
		m := TableMapOf(table)
		order, start, count := moveOrder(m, from, to, false)
		if order == nil {
			return false
		}
		if dispatch != nil {
			place := make([]int, len(order))
			for i, o := range order {
				place[o] = i
			}
			rows := make([][]CellRect, m.Height)
			for _, c := range m.Cells() {
				rows[c.Rect.Top] = append(rows[c.Rect.Top], c)
			}
			kids := make([]*Node, table.ChildCount())
			for i := range kids {
				cells := rows[i]
				slices.SortFunc(cells, func(a, b CellRect) int { return place[a.Rect.Left] - place[b.Rect.Left] })
				nodes := make([]*Node, len(cells))
				for j, c := range cells {
					nodes[j] = table.NodeAt(c.Pos)
				}
				kids[i] = table.Child(i).Copy(FragmentFrom(nodes...))
			}
			finishMove(s, dispatch, table.Copy(FragmentFrom(kids...)), table, pos, func(n *TableMap) (int, int) {
				return n.Map[start], n.Map[n.Width*(n.Height-1)+start+count-1]
			})
		}
		return true
	}
}

func finishMove(s *State, dispatch func(*Transaction), moved, table *Node, pos int, cells func(*TableMap) (int, int)) {
	tr := s.Tr().ReplaceWith(pos, pos+table.NodeSize(), moved)
	anchor, head := cells(TableMapOf(moved))
	start := pos + 1
	tr.SetSelection(NewCellSelection(tr.Doc.Resolve(start+anchor), tr.Doc.Resolve(start+head)))
	dispatch(tr)
}

// SetColumnWidth sets col's width on every cell covering it, 0 for none.
func SetColumnWidth(col int, width float64, tablePos int) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		table, pos := tableAt(s, tablePos)
		if table == nil {
			return false
		}
		m := TableMapOf(table)
		if col < 0 || col >= m.Width {
			return false
		}
		if dispatch != nil {
			tr := s.Tr()
			seen := map[int]bool{}
			for row := range m.Height {
				cellPos := m.Map[row*m.Width+col]
				if seen[cellPos] {
					continue
				}
				seen[cellPos] = true
				cell := table.NodeAt(cellPos)
				widths := make([]any, colspan(cell))
				old := colwidth(cell)
				for i := range widths {
					widths[i] = 0.0
					if i < len(old) {
						widths[i] = old[i]
					}
				}
				widths[col-m.ColCount(cellPos)] = width
				attrs := copyAttrs(cell.Attrs)
				attrs["colwidth"] = widths
				if !slices.ContainsFunc(widths, func(v any) bool { return num(v) > 0 }) {
					attrs["colwidth"] = nil
				}
				tr.SetNodeMarkup(pos+1+cellPos, nil, attrs, nil)
			}
			dispatch(tr)
		}
		return true
	}
}

// SetRowHeight sets row's height, 0 for none.
func SetRowHeight(row int, height float64, tablePos int) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		table, pos := tableAt(s, tablePos)
		if table == nil || row < 0 || row >= table.ChildCount() {
			return false
		}
		if dispatch != nil {
			rowPos := pos + 1
			for i := range row {
				rowPos += table.Child(i).NodeSize()
			}
			var v any
			if height > 0 {
				v = height
			}
			dispatch(s.Tr().SetNodeAttribute(rowPos, "height", v))
		}
		return true
	}
}

// DeleteCellSelection empties the selected cells.
func DeleteCellSelection(s *State, dispatch func(*Transaction)) bool {
	sel, ok := s.Selection.(*CellSelection)
	if !ok {
		return false
	}
	if dispatch != nil {
		tr := s.Tr()
		mapFrom := len(tr.Steps)
		for _, r := range sel.ranges {
			mapping := tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps))
			tr.Transform.Replace(mapping.Map(r.From.Pos, 1), mapping.Map(r.To.Pos, 1), EmptySlice)
		}
		if tr.DocChanged() {
			dispatch(tr)
		}
	}
	return true
}

func arrowCell(s *State, axis string, dir int) *ResolvedPos {
	var cell *ResolvedPos
	if c, ok := s.Selection.(*CellSelection); ok {
		cell = c.HeadCell
	} else if t, ok := s.Selection.(*TextSelection); ok {
		cell = cellAround(t.HeadPos())
	}
	if cell == nil {
		return nil
	}
	table := cell.Node(-1)
	m := TableMapOf(table)
	start := cell.Start(-1)
	next := m.NextCell(cell.Pos-start, axis, dir)
	if next < 0 {
		return nil
	}
	return s.Doc.Resolve(start + next)
}

// ShiftArrow extends a cell selection, starting one from a text selection
// at a cell's edge.
func ShiftArrow(axis string, dir int) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		var anchor, head *ResolvedPos
		if c, ok := s.Selection.(*CellSelection); ok {
			anchor, head = c.AnchorCell, arrowCell(s, axis, dir)
			if head == nil {
				return false
			}
		} else if t, ok := s.Selection.(*TextSelection); ok {
			at := cellAround(t.HeadPos())
			if at == nil || !t.Empty() && t.AnchorPos().Pos != t.HeadPos().Pos {
				return false
			}
			inner := t.HeadPos()
			if dir < 0 && inner.ParentOffset != 0 || dir > 0 && inner.ParentOffset != inner.Parent().Content.Size {
				return false
			}
			anchor, head = at, at
		} else {
			return false
		}
		if dispatch != nil {
			dispatch(s.Tr().SetSelection(NewCellSelection(anchor, head)).ScrollIntoView())
		}
		return true
	}
}

// TableEditing normalizes selections in tables: a selected cell, row or table
// becomes a cell selection, and a text selection across two cells of one
// table does.
func TableEditing() *Plugin {
	return &Plugin{Key: "tableEditing", AppendTransaction: func(_ []*Transaction, _, n *State) *Transaction {
		var normal Selection
		switch sel := n.Selection.(type) {
		case *NodeSelection:
			switch tableRole(sel.Node.Type) {
			case "cell", "header_cell":
				normal = NewCellSelection(n.Doc.Resolve(sel.From()), nil)
			case "row":
				r := n.Doc.Resolve(sel.From() + 1)
				normal = RowSelection(r, r)
			case "table":
				m := TableMapOf(sel.Node)
				start := sel.From() + 1
				normal = NewCellSelection(n.Doc.Resolve(start+m.Map[0]), n.Doc.Resolve(start+m.Map[len(m.Map)-1]))
			}
		case *TextSelection:
			a, h := cellAround(sel.AnchorPos()), cellAround(sel.HeadPos())
			if a != nil && h != nil && a.Pos != h.Pos && inSameTable(a, h) {
				normal = NewCellSelection(a, h)
			}
		}
		if normal == nil {
			return nil
		}
		return n.Tr().SetSelection(normal)
	}, HandleKeyDown: func(v *Session, key string) bool {
		var c Command
		switch key {
		case "Shift-ArrowLeft":
			c = ShiftArrow("horiz", -1)
		case "Shift-ArrowRight":
			c = ShiftArrow("horiz", 1)
		case "Shift-ArrowUp":
			c = ShiftArrow("vert", -1)
		case "Shift-ArrowDown":
			c = ShiftArrow("vert", 1)
		case "Backspace", "Mod-Backspace", "Delete", "Mod-Delete":
			c = DeleteCellSelection
		default:
			return false
		}
		return c(v.State(), v.Dispatch)
	}, HandlePaste: pasteIntoTable}
}
