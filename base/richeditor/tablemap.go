package richeditor

import (
	"maps"
	"slices"
	"sync"
)

// Tables are prosemirror-tables' (MIT, © Marijn Haverbeke and others) with
// Tiptap's table extensions: nothing here knows node names, a type's role
// being its spec's Extra["tableRole"].

// TableRole is what a node type is in a table.
func tableRole(t *NodeType) string {
	r, _ := t.Spec.Extra["tableRole"].(string)
	return r
}

func isCellType(t *NodeType) bool {
	r := tableRole(t)
	return r == "cell" || r == "header_cell"
}

// tableTypes are a schema's table node types.
type tableTypes struct{ table, row, cell, header *NodeType }

func tableNodeTypes(s *Schema) tableTypes {
	var out tableTypes
	for _, t := range s.nodeOrder {
		switch tableRole(t) {
		case "table":
			out.table = t
		case "row":
			out.row = t
		case "cell":
			out.cell = t
		case "header_cell":
			out.header = t
		}
	}
	return out
}

func colspan(n *Node) int { return max(int(n.AttrNum("colspan")), 1) }
func rowspan(n *Node) int { return max(int(n.AttrNum("rowspan")), 1) }

func colwidth(n *Node) []float64 {
	raw, _ := n.Attrs["colwidth"].([]any)
	if raw == nil {
		return nil
	}
	out := make([]float64, len(raw))
	for i, v := range raw {
		out[i], _ = v.(float64)
	}
	return out
}

func copyAttrs(a Attrs) Attrs { return maps.Clone(a) }

// Rect is a rectangle of a table's map, its right and bottom past the end.
type Rect struct{ Left, Top, Right, Bottom int }

type tableProblem struct {
	kind   string // "collision", "missing", "overlong_rowspan", "colwidth mismatch"
	row    int
	pos    int
	n      int
	widths []any
}

// TableMap is a table's grid, each slot the cell covering it as an offset
// from the table's start.
type TableMap struct {
	Width, Height int
	Map           []int
	problems      []tableProblem
}

var (
	tableMaps    = map[*Node]*TableMap{}
	tableOrder   []*Node
	tableMapLock sync.Mutex
)

const tableMapCache = 16

// TableMapOf is table's map, cached by node.
func TableMapOf(table *Node) *TableMap {
	tableMapLock.Lock()
	defer tableMapLock.Unlock()
	if m := tableMaps[table]; m != nil {
		return m
	}
	m := computeMap(table)
	tableMaps[table] = m
	tableOrder = append(tableOrder, table)
	if len(tableOrder) > tableMapCache {
		delete(tableMaps, tableOrder[0])
		tableOrder = tableOrder[1:]
	}
	return m
}

func findWidth(table *Node) int {
	width := -1
	hasRowSpan := false
	for row := 0; row < table.ChildCount(); row++ {
		rowNode := table.Child(row)
		rowWidth := 0
		if hasRowSpan {
			for j := range row {
				prev := table.Child(j)
				for i := 0; i < prev.ChildCount(); i++ {
					if cell := prev.Child(i); j+rowspan(cell) > row {
						rowWidth += colspan(cell)
					}
				}
			}
		}
		for i := 0; i < rowNode.ChildCount(); i++ {
			cell := rowNode.Child(i)
			rowWidth += colspan(cell)
			if rowspan(cell) > 1 {
				hasRowSpan = true
			}
		}
		width = max(width, rowWidth)
	}
	return width
}

func computeMap(table *Node) *TableMap {
	width, height := findWidth(table), table.ChildCount()
	grid := make([]int, max(width*height, 0))
	mapPos := 0
	var problems []tableProblem
	colWidths := make([]float64, max(width, 0)*2)
	pos := 0
	for row := range height {
		rowNode := table.Child(row)
		pos++
		for i := 0; ; i++ {
			for mapPos < len(grid) && grid[mapPos] != 0 {
				mapPos++
			}
			if i == rowNode.ChildCount() {
				break
			}
			cell := rowNode.Child(i)
			cs, rs, cw := colspan(cell), rowspan(cell), colwidth(cell)
			for h := range rs {
				if h+row >= height {
					problems = append(problems, tableProblem{kind: "overlong_rowspan", pos: pos, n: rs - h})
					break
				}
				start := mapPos + h*width
				for w := range cs {
					if start+w < len(grid) && grid[start+w] == 0 {
						grid[start+w] = pos
					} else {
						problems = append(problems, tableProblem{kind: "collision", row: row, pos: pos, n: cs - w})
					}
					if w < len(cw) && cw[w] != 0 {
						index := ((start + w) % width) * 2
						prev := colWidths[index]
						if prev == 0 || prev != cw[w] && colWidths[index+1] == 1 {
							colWidths[index], colWidths[index+1] = cw[w], 1
						} else if prev == cw[w] {
							colWidths[index+1]++
						}
					}
				}
			}
			mapPos += cs
			pos += cell.NodeSize()
		}
		expected := (row + 1) * width
		missing := 0
		for mapPos < expected {
			if grid[mapPos] == 0 {
				missing++
			}
			mapPos++
		}
		if missing > 0 {
			problems = append(problems, tableProblem{kind: "missing", row: row, n: missing})
		}
		pos++
	}
	m := &TableMap{Width: width, Height: height, Map: grid, problems: problems}
	bad := false
	for i := 0; i < len(colWidths); i += 2 {
		if colWidths[i] != 0 && int(colWidths[i+1]) < height {
			bad = true
		}
	}
	if bad {
		findBadWidths(m, colWidths, table)
	}
	return m
}

func findBadWidths(m *TableMap, colWidths []float64, table *Node) {
	seen := map[int]bool{}
	for i, pos := range m.Map {
		if seen[pos] {
			continue
		}
		seen[pos] = true
		node := table.NodeAt(pos)
		if node == nil {
			continue
		}
		var updated []any
		widths := colwidth(node)
		for j := range colspan(node) {
			col := (i + j) % m.Width
			want := colWidths[col*2]
			have := 0.0
			if j < len(widths) {
				have = widths[j]
			}
			if want != 0 && have != want {
				if updated == nil {
					updated = make([]any, colspan(node))
					for k := range updated {
						updated[k] = 0.0
						if k < len(widths) {
							updated[k] = widths[k]
						}
					}
				}
				updated[j] = want
			}
		}
		if updated != nil {
			m.problems = append([]tableProblem{{kind: "colwidth mismatch", pos: pos, widths: updated}}, m.problems...)
		}
	}
}

// FindCell is the rectangle of the cell at offset pos.
func (m *TableMap) FindCell(pos int) Rect {
	for i, p := range m.Map {
		if p != pos {
			continue
		}
		left, top := i%m.Width, i/m.Width
		right, bottom := left+1, top+1
		for right < m.Width && m.Map[i+right-left] == pos {
			right++
		}
		for bottom < m.Height && m.Map[i+m.Width*(bottom-top)] == pos {
			bottom++
		}
		return Rect{left, top, right, bottom}
	}
	panic("richeditor: no cell at offset in table")
}

// ColCount is the column the cell at pos starts in.
func (m *TableMap) ColCount(pos int) int {
	for i, p := range m.Map {
		if p == pos {
			return i % m.Width
		}
	}
	panic("richeditor: no cell at offset in table")
}

// NextCell is the cell next to the one at pos on axis ("horiz" or "vert"),
// -1 for none.
func (m *TableMap) NextCell(pos int, axis string, dir int) int {
	r := m.FindCell(pos)
	if axis == "horiz" {
		if dir < 0 && r.Left == 0 || dir > 0 && r.Right == m.Width {
			return -1
		}
		col := r.Right
		if dir < 0 {
			col = r.Left - 1
		}
		return m.Map[col+r.Top*m.Width]
	}
	if dir < 0 && r.Top == 0 || dir > 0 && r.Bottom == m.Height {
		return -1
	}
	row := r.Bottom
	if dir < 0 {
		row = r.Top - 1
	}
	return m.Map[r.Left+m.Width*row]
}

// RectBetween is the rectangle around the cells at a and b, grown over the
// cells it cuts.
func (m *TableMap) RectBetween(a, b int) Rect {
	ra, rb := m.FindCell(a), m.FindCell(b)
	return Rect{min(ra.Left, rb.Left), min(ra.Top, rb.Top), max(ra.Right, rb.Right), max(ra.Bottom, rb.Bottom)}
}

// CellsInRect are the cells whose top left is in rect.
func (m *TableMap) CellsInRect(rect Rect) []int {
	var result []int
	seen := map[int]bool{}
	for row := rect.Top; row < rect.Bottom; row++ {
		for col := rect.Left; col < rect.Right; col++ {
			index := row*m.Width + col
			pos := m.Map[index]
			if seen[pos] {
				continue
			}
			seen[pos] = true
			if col == rect.Left && col > 0 && m.Map[index-1] == pos || row == rect.Top && row > 0 && m.Map[index-m.Width] == pos {
				continue
			}
			result = append(result, pos)
		}
	}
	return result
}

// PositionAt is where a cell at row and col of the table would go.
func (m *TableMap) PositionAt(row, col int, table *Node) int {
	for i, rowStart := 0, 0; ; i++ {
		rowEnd := rowStart + table.Child(i).NodeSize()
		if i == row {
			index := col + row*m.Width
			rowEndIndex := (row + 1) * m.Width
			for index < rowEndIndex && m.Map[index] < rowStart {
				index++
			}
			if index == rowEndIndex {
				return rowEnd - 1
			}
			return m.Map[index]
		}
		rowStart = rowEnd
	}
}

// Cells are each cell's offset with its rectangle, row by row.
func (m *TableMap) Cells() []CellRect {
	var out []CellRect
	seen := map[int]bool{}
	for _, pos := range m.Map {
		if !seen[pos] {
			seen[pos] = true
			out = append(out, CellRect{pos, m.FindCell(pos)})
		}
	}
	return out
}

// CellRect is a cell's offset and rectangle.
type CellRect struct {
	Pos  int
	Rect Rect
}

// ColumnWidths are a table's widths by column, 0 for none.
func ColumnWidths(table *Node) []float64 {
	m := TableMapOf(table)
	out := make([]float64, m.Width)
	for _, c := range m.Cells() {
		widths := colwidth(table.NodeAt(c.Pos))
		for i := 0; i < c.Rect.Right-c.Rect.Left && i < len(widths); i++ {
			if widths[i] != 0 {
				out[c.Rect.Left+i] = widths[i]
			}
		}
	}
	return out
}

func cellAround(r *ResolvedPos) *ResolvedPos {
	for d := r.Depth - 1; d > 0; d-- {
		if tableRole(r.Node(d).Type) == "row" {
			return r.Doc().Resolve(r.Before(d + 1))
		}
	}
	return nil
}

func isInTable(s *State) bool {
	r := s.Selection.HeadPos()
	for d := r.Depth; d > 0; d-- {
		if tableRole(r.Node(d).Type) == "row" {
			return true
		}
	}
	return false
}

func pointsAtCell(r *ResolvedPos) bool {
	return tableRole(r.Parent().Type) == "row" && r.NodeAfter() != nil
}

func inSameTable(a, b *ResolvedPos) bool {
	return a.Depth == b.Depth && a.Pos >= b.Start(-1) && a.Pos <= b.End(-1)
}

// CellSelection selects a rectangle of cells: Replace fills the first and
// empties the others.
type CellSelection struct {
	selBase
	AnchorCell, HeadCell *ResolvedPos
}

// NewCellSelection selects the cells between two positions before cells.
func NewCellSelection(anchor, head *ResolvedPos) *CellSelection {
	if head == nil {
		head = anchor
	}
	table := anchor.Node(-1)
	m := TableMapOf(table)
	start := anchor.Start(-1)
	rect := m.RectBetween(anchor.Pos-start, head.Pos-start)
	doc := anchor.Doc()
	cells := []int{head.Pos - start}
	for _, c := range m.CellsInRect(rect) {
		if c != head.Pos-start {
			cells = append(cells, c)
		}
	}
	ranges := make([]SelectionRange, len(cells))
	for i, pos := range cells {
		cell := table.NodeAt(pos)
		from := pos + start + 1
		ranges[i] = SelectionRange{doc.Resolve(from), doc.Resolve(from + cell.Content.Size)}
	}
	return &CellSelection{selBase{ranges[0].From, ranges[0].To, ranges}, anchor, head}
}

func (s *CellSelection) Eq(o Selection) bool {
	c, ok := o.(*CellSelection)
	return ok && c.AnchorCell.Pos == s.AnchorCell.Pos && c.HeadCell.Pos == s.HeadCell.Pos
}

func (s *CellSelection) Map(doc *Node, m Mappable) Selection {
	anchor, head := doc.Resolve(m.Map(s.AnchorCell.Pos, 1)), doc.Resolve(m.Map(s.HeadCell.Pos, 1))
	if pointsAtCell(anchor) && pointsAtCell(head) && inSameTable(anchor, head) {
		changed := s.AnchorCell.Node(-1) != anchor.Node(-1)
		switch {
		case changed && s.IsRowSelection():
			return RowSelection(anchor, head)
		case changed && s.IsColSelection():
			return ColSelection(anchor, head)
		}
		return NewCellSelection(anchor, head)
	}
	return TextSelectionBetween(anchor, head, 0)
}

// Content is the rectangle as rows.
func (s *CellSelection) Content() *Slice {
	table := s.AnchorCell.Node(-1)
	m := TableMapOf(table)
	start := s.AnchorCell.Start(-1)
	rect := m.RectBetween(s.AnchorCell.Pos-start, s.HeadCell.Pos-start)
	var rows []*Node
	seen := map[int]bool{}
	for row := rect.Top; row < rect.Bottom; row++ {
		var cells []*Node
		for col := rect.Left; col < rect.Right; col++ {
			pos := m.Map[row*m.Width+col]
			if seen[pos] {
				continue
			}
			seen[pos] = true
			cell := table.NodeAt(pos)
			cr := m.FindCell(pos)
			attrs := copyAttrs(cell.Attrs)
			attrs["rowspan"] = float64(min(cr.Bottom, rect.Bottom) - max(cr.Top, rect.Top))
			attrs["colspan"] = float64(min(cr.Right, rect.Right) - max(cr.Left, rect.Left))
			cells = append(cells, cell.Type.Create(attrs, cell.Content, nil))
		}
		rows = append(rows, table.Child(row).Copy(FragmentFrom(cells...)))
	}
	return NewSlice(FragmentFrom(table.Copy(FragmentFrom(rows...))), 1, 1)
}

func (s *CellSelection) Replace(tr *Transaction, content *Slice) {
	if content == nil {
		content = EmptySlice
	}
	mapFrom := len(tr.Steps)
	for i, r := range s.ranges {
		mapping := tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps))
		c := EmptySlice
		if i == 0 {
			c = content
		}
		tr.Transform.Replace(mapping.Map(r.From.Pos, 1), mapping.Map(r.To.Pos, 1), c)
	}
	if sel := SelectionFindFrom(tr.Doc.Resolve(tr.Mapping.Slice(mapFrom, len(tr.Mapping.Maps)).Map(s.To(), 1)), -1, false); sel != nil {
		tr.SetSelection(sel)
	}
}

func (s *CellSelection) ReplaceWith(tr *Transaction, node *Node) {
	s.Replace(tr, NewSlice(FragmentFrom(node), 0, 0))
}

// IsRowSelection says the selection covers whole rows.
func (s *CellSelection) IsRowSelection() bool {
	m := TableMapOf(s.AnchorCell.Node(-1))
	start := s.AnchorCell.Start(-1)
	a, h := m.ColCount(s.AnchorCell.Pos-start), m.ColCount(s.HeadCell.Pos-start)
	if min(a, h) > 0 {
		return false
	}
	return max(a+colspan(s.AnchorCell.NodeAfter()), h+colspan(s.HeadCell.NodeAfter())) == m.Width
}

// IsColSelection says the selection covers whole columns.
func (s *CellSelection) IsColSelection() bool {
	anchorTop, headTop := s.AnchorCell.Index(-1), s.HeadCell.Index(-1)
	if min(anchorTop, headTop) > 0 {
		return false
	}
	return max(anchorTop+rowspan(s.AnchorCell.NodeAfter()), headTop+rowspan(s.HeadCell.NodeAfter())) == s.HeadCell.Node(-1).ChildCount()
}

// RowSelection selects the rows from anchor's to head's.
func RowSelection(anchor, head *ResolvedPos) *CellSelection {
	if head == nil {
		head = anchor
	}
	table := anchor.Node(-1)
	m := TableMapOf(table)
	start := anchor.Start(-1)
	a, h := m.FindCell(anchor.Pos-start), m.FindCell(head.Pos-start)
	doc := anchor.Doc()
	aRow, hRow := a.Top, h.Top
	var ac, hc int
	if aRow <= hRow {
		ac, hc = m.Map[aRow*m.Width], m.Map[h.Bottom*m.Width-1]
	} else {
		ac, hc = m.Map[a.Bottom*m.Width-1], m.Map[hRow*m.Width]
	}
	return NewCellSelection(doc.Resolve(start+ac), doc.Resolve(start+hc))
}

// ColSelection selects the columns from anchor's to head's.
func ColSelection(anchor, head *ResolvedPos) *CellSelection {
	if head == nil {
		head = anchor
	}
	table := anchor.Node(-1)
	m := TableMapOf(table)
	start := anchor.Start(-1)
	a, h := m.FindCell(anchor.Pos-start), m.FindCell(head.Pos-start)
	doc := anchor.Doc()
	var ac, hc int
	if a.Left <= h.Left {
		ac, hc = m.Map[a.Left], m.Map[m.Width*(m.Height-1)+h.Right-1]
	} else {
		ac, hc = m.Map[a.Right-1+m.Width*(m.Height-1)], m.Map[h.Left]
	}
	return NewCellSelection(doc.Resolve(start+ac), doc.Resolve(start+hc))
}

func (s *CellSelection) Bookmark() Bookmark { return cellBookmark{s.AnchorCell.Pos, s.HeadCell.Pos} }

type cellBookmark struct{ anchor, head int }

func (b cellBookmark) Map(m Mappable) Bookmark {
	return cellBookmark{m.Map(b.anchor, 1), m.Map(b.head, 1)}
}

func (b cellBookmark) Resolve(doc *Node) Selection {
	a, h := doc.Resolve(b.anchor), doc.Resolve(b.head)
	if pointsAtCell(a) && pointsAtCell(h) && inSameTable(a, h) {
		return NewCellSelection(a, h)
	}
	return SelectionNear(h, 1)
}

// fixTables is prosemirror-tables' fixTables over the tables changed since
// old, every table when it is nil.
func fixTables(s *State, old *State) *Transaction {
	tr := s.Tr()
	check := func(table *Node, pos int) {
		if tableRole(table.Type) == "table" {
			fixTable(s, table, pos, tr)
		}
	}
	if old == nil {
		s.Doc.Descendants(func(n *Node, pos int, _ *Node, _ int) bool {
			check(n, pos)
			return true
		})
	} else if old.Doc != s.Doc {
		changedDescendants(old.Doc, s.Doc, 0, check)
	}
	if tr.DocChanged() {
		return tr
	}
	return nil
}

func changedDescendants(old, cur *Node, offset int, f func(*Node, int)) {
	oldSize, curSize := old.ChildCount(), cur.ChildCount()
outer:
	for i, j := 0, 0; j < curSize; j++ {
		child := cur.Child(j)
		for scan, e := i, min(oldSize, j+1); scan < e; scan++ {
			if old.Child(scan) == child {
				i = scan + 1
				offset += child.NodeSize()
				continue outer
			}
		}
		f(child, offset)
		if i < oldSize && old.Child(i).SameMarkup(child) {
			changedDescendants(old.Child(i), child, offset+1, f)
		} else {
			child.NodesBetween(0, child.Content.Size, func(n *Node, pos int, _ *Node, _ int) bool {
				f(n, pos)
				return true
			}, offset+1)
		}
		offset += child.NodeSize()
	}
}

func fixTable(s *State, table *Node, tablePos int, tr *Transaction) {
	m := TableMapOf(table)
	if len(m.problems) == 0 {
		return
	}
	mustAdd := make([]int, m.Height)
	for _, p := range m.problems {
		switch p.kind {
		case "collision":
			cell := table.NodeAt(p.pos)
			if cell == nil {
				continue
			}
			attrs := copyAttrs(cell.Attrs)
			for j := range rowspan(cell) {
				if p.row+j < len(mustAdd) {
					mustAdd[p.row+j] += p.n
				}
			}
			attrs["colspan"] = float64(colspan(cell) - p.n)
			if widths, ok := attrs["colwidth"].([]any); ok {
				attrs["colwidth"] = slices.Clone(widths[:max(len(widths)-p.n, 0)])
				if len(widths)-p.n <= 0 {
					attrs["colwidth"] = nil
				}
			}
			tr.SetNodeMarkup(tr.Mapping.Map(tablePos+1+p.pos, 1), nil, attrs, nil)
		case "missing":
			mustAdd[p.row] += p.n
		case "overlong_rowspan":
			cell := table.NodeAt(p.pos)
			if cell == nil {
				continue
			}
			attrs := copyAttrs(cell.Attrs)
			attrs["rowspan"] = float64(rowspan(cell) - p.n)
			tr.SetNodeMarkup(tr.Mapping.Map(tablePos+1+p.pos, 1), nil, attrs, nil)
		case "colwidth mismatch":
			cell := table.NodeAt(p.pos)
			if cell == nil {
				continue
			}
			attrs := copyAttrs(cell.Attrs)
			attrs["colwidth"] = p.widths
			tr.SetNodeMarkup(tr.Mapping.Map(tablePos+1+p.pos, 1), nil, attrs, nil)
		}
	}
	types := tableNodeTypes(s.Schema)
	pos := tablePos + 1
	for i := 0; i < len(mustAdd) && i < table.ChildCount(); i++ {
		end := pos + table.Child(i).NodeSize()
		if mustAdd[i] > 0 {
			t := types.cell
			row := table.Child(i)
			if row.ChildCount() > 0 && row.Child(0).Type == types.header && i == 0 {
				t = types.header
			}
			cells := make([]*Node, mustAdd[i])
			for j := range cells {
				cells[j] = t.CreateAndFill(nil, nil, nil)
			}
			at := tr.Mapping.Map(end-1, 1)
			tr.Insert(at, cells...)
		}
		pos = end
	}
}

// FixTables keeps the tables valid after every change.
func FixTables() *Plugin {
	return &Plugin{Key: "fixTables", AppendTransaction: func(trs []*Transaction, old, n *State) *Transaction {
		return fixTables(n, old)
	}}
}
