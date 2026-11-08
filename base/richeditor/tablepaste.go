package richeditor

import "slices"

// Pasting cells is prosemirror-tables' copypaste (MIT, © Marijn Haverbeke
// and others): cells pasted over a cell selection fill it, repeated or
// clipped, and cells pasted into a cell land there, the table growing to
// hold them.

// CellArea is a rectangle of pasted cells, a fragment of cells per row.
type CellArea struct {
	Width, Height int
	Rows          []*Fragment
}

// PastedCells is the cells slice holds, nil unless it is a table's rows or
// cells.
func PastedCells(slice *Slice) *CellArea {
	if slice == nil || slice.Size() == 0 {
		return nil
	}
	content, openStart, openEnd := slice.Content, slice.OpenStart, slice.OpenEnd
	for content.ChildCount() == 1 && (openStart > 0 && openEnd > 0 || tableRole(content.Child(0).Type) == "table") {
		openStart, openEnd = max(openStart-1, 0), max(openEnd-1, 0)
		content = content.Child(0).Content
	}
	if content.ChildCount() == 0 {
		return nil
	}
	first := content.Child(0)
	schema := first.Type.Schema
	types := tableNodeTypes(schema)
	var rows []*Fragment
	switch tableRole(first.Type) {
	case "row":
		for i := range content.ChildCount() {
			cells := content.Child(i).Content
			left, right := 0, 0
			if i == 0 {
				left = max(openStart-1, 0)
			}
			if i == content.ChildCount()-1 {
				right = max(openEnd-1, 0)
			}
			if left > 0 || right > 0 {
				cells = fitSlice(types.row, NewSlice(cells, left, right)).Content
			}
			rows = append(rows, cells)
		}
	case "cell", "header_cell":
		if openStart > 0 || openEnd > 0 {
			rows = append(rows, fitSlice(types.row, NewSlice(content, openStart, openEnd)).Content)
		} else {
			rows = append(rows, content)
		}
	default:
		return nil
	}
	return ensureRectangular(schema, rows)
}

// ensureRectangular fills rows short of the widest with empty cells.
func ensureRectangular(s *Schema, rows []*Fragment) *CellArea {
	var widths []int
	for i, row := range rows {
		for j := row.ChildCount() - 1; j >= 0; j-- {
			cell := row.Child(j)
			for r := i; r < i+rowspan(cell); r++ {
				for len(widths) <= r {
					widths = append(widths, 0)
				}
				widths[r] += colspan(cell)
			}
		}
	}
	width := 0
	for _, w := range widths {
		width = max(width, w)
	}
	for r, w := range widths {
		if r >= len(rows) {
			rows = append(rows, EmptyFragment())
		}
		if w < width {
			empty := tableNodeTypes(s).cell.CreateAndFill(nil, nil, nil)
			cells := make([]*Node, width-w)
			for i := range cells {
				cells[i] = empty
			}
			rows[r] = rows[r].Append(FragmentFrom(cells...))
		}
	}
	return &CellArea{Width: width, Height: len(rows), Rows: rows}
}

// fitSlice is a node of t holding slice, fitted as a replace fits it.
func fitSlice(t *NodeType, slice *Slice) *Node {
	node := t.CreateAndFill(nil, nil, nil)
	return NewTransform(node).Replace(0, node.Content.Size, slice).Doc
}

// removeColspan takes n columns out of a cell's span at pos.
func removeColspan(attrs Attrs, pos, n int) Attrs {
	out := copyAttrs(attrs)
	out["colspan"] = float64(max(int(num(attrs["colspan"])), 1) - n)
	if widths, ok := attrs["colwidth"].([]any); ok {
		w := slices.Clone(widths)
		w = slices.Delete(w, min(pos, len(w)), min(pos+n, len(w)))
		if slices.ContainsFunc(w, func(v any) bool { return num(v) > 0 }) {
			out["colwidth"] = w
		} else {
			out["colwidth"] = nil
		}
	}
	return out
}

// ClipCells repeats or cuts an area to a width and height.
func ClipCells(a *CellArea, width, height int) *CellArea {
	rows := a.Rows
	if a.Width != width {
		added := map[int]int{}
		next := make([]*Fragment, 0, len(rows))
		for row, frag := range rows {
			var cells []*Node
			for col, i := added[row], 0; col < width; i++ {
				cell := frag.Child(i % frag.ChildCount())
				if span := colspan(cell); col+span > width {
					over := col + span - width
					cell = cell.Type.Create(removeColspan(cell.Attrs, span-over, over), cell.Content, nil)
				}
				cells = append(cells, cell)
				col += colspan(cell)
				for j := 1; j < rowspan(cell); j++ {
					added[row+j] += colspan(cell)
				}
			}
			next = append(next, FragmentFrom(cells...))
		}
		rows = next
	}
	if a.Height != height {
		next := make([]*Fragment, 0, height)
		for row := range height {
			source := rows[row%a.Height]
			cells := make([]*Node, 0, source.ChildCount())
			for _, cell := range source.Content {
				if row+rowspan(cell) > height {
					attrs := copyAttrs(cell.Attrs)
					attrs["rowspan"] = float64(max(1, height-rowspan(cell)))
					cell = cell.Type.Create(attrs, cell.Content, nil)
				}
				cells = append(cells, cell)
			}
			next = append(next, FragmentFrom(cells...))
		}
		rows = next
	}
	return &CellArea{Width: width, Height: height, Rows: rows}
}

// growTable adds columns and rows until the table is width by height.
func growTable(tr *Transaction, m *TableMap, table *Node, start, width, height, mapFrom int) bool {
	types := tableNodeTypes(tr.Doc.Type.Schema)
	var empty, emptyHead *Node
	cell := func(header bool) *Node {
		if header {
			if emptyHead == nil {
				emptyHead = types.header.CreateAndFill(nil, nil, nil)
			}
			return emptyHead
		}
		if empty == nil {
			empty = types.cell.CreateAndFill(nil, nil, nil)
		}
		return empty
	}
	mapped := func(pos int) int { return tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps)).Map(pos, 1) }
	if width > m.Width {
		rowEnd := 0
		for row := range m.Height {
			rowNode := table.Child(row)
			rowEnd += rowNode.NodeSize()
			last := rowNode.LastChild()
			add := cell(last != nil && last.Type != types.cell)
			cells := make([]*Node, width-m.Width)
			for i := range cells {
				cells[i] = add
			}
			tr.Insert(mapped(rowEnd-1+start), cells...)
		}
	}
	if height > m.Height {
		var cells []*Node
		last := (m.Height - 1) * m.Width
		for i := range max(m.Width, width) {
			header := i < m.Width && table.NodeAt(m.Map[last+i]).Type == types.header
			cells = append(cells, cell(header))
		}
		emptyRow := types.row.Create(nil, FragmentFrom(cells...), nil)
		rows := make([]*Node, height-m.Height)
		for i := range rows {
			rows[i] = emptyRow
		}
		tr.Insert(mapped(start+table.NodeSize()-2), rows...)
	}
	return empty != nil || emptyHead != nil
}

// isolateHorizontal splits the cells crossing the line above row top.
func isolateHorizontal(tr *Transaction, m *TableMap, table *Node, start, left, right, top, mapFrom int) bool {
	if top == 0 || top == m.Height {
		return false
	}
	found := false
	for col := left; col < right; col++ {
		index := top*m.Width + col
		pos := m.Map[index]
		if m.Map[index-m.Width] != pos {
			continue
		}
		found = true
		cell := table.NodeAt(pos)
		r := m.FindCell(pos)
		mapping := tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps))
		attrs := copyAttrs(cell.Attrs)
		attrs["rowspan"] = float64(top - r.Top)
		tr.SetNodeMarkup(mapping.Map(pos+start, 1), nil, attrs, nil)
		below := copyAttrs(cell.Attrs)
		below["rowspan"] = float64(r.Top + rowspan(cell) - top)
		tr.Insert(tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps)).Map(m.PositionAt(top, r.Left, table)+start, 1), cell.Type.CreateAndFill(below, nil, nil))
		col += colspan(cell) - 1
	}
	return found
}

// isolateVertical splits the cells crossing the line left of column left.
func isolateVertical(tr *Transaction, m *TableMap, table *Node, start, top, bottom, left, mapFrom int) bool {
	if left == 0 || left == m.Width {
		return false
	}
	found := false
	for row := top; row < bottom; row++ {
		index := row*m.Width + left
		pos := m.Map[index]
		if m.Map[index-1] != pos {
			continue
		}
		found = true
		cell := table.NodeAt(pos)
		cellLeft := m.ColCount(pos)
		at := tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps)).Map(pos+start, 1)
		tr.SetNodeMarkup(at, nil, removeColspan(cell.Attrs, left-cellLeft, colspan(cell)-(left-cellLeft)), nil)
		tr.Insert(at+tr.Doc.NodeAt(at).NodeSize(), cell.Type.CreateAndFill(removeColspan(cell.Attrs, 0, left-cellLeft), nil, nil))
		row += rowspan(cell) - 1
	}
	return found
}

// InsertCells puts cells into the table starting at tableStart with their
// top left at rect's, growing the table and splitting the cells crossing
// the area's edges, and selects them.
func InsertCells(s *State, dispatch func(*Transaction), tableStart int, rect Rect, cells *CellArea) {
	table := s.Doc.NodeAt(tableStart - 1)
	m := TableMapOf(table)
	top, left := rect.Top, rect.Left
	right, bottom := left+cells.Width, top+cells.Height
	tr := s.Tr()
	mapFrom := 0
	recomp := func() {
		table = tr.Doc.NodeAt(tableStart - 1)
		m = TableMapOf(table)
		mapFrom = len(tr.Mapping.Maps)
	}
	if growTable(tr, m, table, tableStart, right, bottom, mapFrom) {
		recomp()
	}
	if isolateHorizontal(tr, m, table, tableStart, left, right, top, mapFrom) {
		recomp()
	}
	if isolateHorizontal(tr, m, table, tableStart, left, right, bottom, mapFrom) {
		recomp()
	}
	if isolateVertical(tr, m, table, tableStart, top, bottom, left, mapFrom) {
		recomp()
	}
	if isolateVertical(tr, m, table, tableStart, top, bottom, right, mapFrom) {
		recomp()
	}
	for row := top; row < bottom; row++ {
		from, to := m.PositionAt(row, left, table), m.PositionAt(row, right, table)
		mapping := tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps))
		tr.Transform.Replace(mapping.Map(from+tableStart, 1), mapping.Map(to+tableStart, 1), NewSlice(cells.Rows[row-top], 0, 0))
	}
	recomp()
	tr.SetSelection(NewCellSelection(tr.Doc.Resolve(tableStart+m.PositionAt(top, left, table)), tr.Doc.Resolve(tableStart+m.PositionAt(bottom-1, right-1, table))))
	tr.Origin = OriginPaste
	tr.SetMeta(pasteMeta, true)
	dispatch(tr.ScrollIntoView())
}

// pasteIntoTable is the table editing plugin's paste: cells over a cell
// selection fill it, any content filling one cell of it; cells pasted in a
// cell go there.
func pasteIntoTable(v *Session, slice *Slice) bool {
	st := v.State()
	if !isInTable(st) {
		if _, ok := st.Selection.(*CellSelection); !ok {
			return false
		}
	}
	cells := PastedCells(slice)
	if sel, ok := st.Selection.(*CellSelection); ok {
		if cells == nil {
			cells = &CellArea{Width: 1, Height: 1, Rows: []*Fragment{FragmentFrom(fitSlice(tableNodeTypes(st.Schema).cell, slice))}}
		}
		table := sel.AnchorCell.Node(-1)
		start := sel.AnchorCell.Start(-1)
		rect := TableMapOf(table).RectBetween(sel.AnchorCell.Pos-start, sel.HeadCell.Pos-start)
		InsertCells(st, v.Dispatch, start, rect, ClipCells(cells, rect.Right-rect.Left, rect.Bottom-rect.Top))
		return true
	}
	if cells == nil {
		return false
	}
	cell := selectionCell(st)
	start := cell.Start(-1)
	InsertCells(st, v.Dispatch, start, TableMapOf(cell.Node(-1)).FindCell(cell.Pos-start), cells)
	return true
}
