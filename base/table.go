package base

import "github.com/nsgo-dev/nsgo/core"

// Table is a static table of the app's own rows and cells, with a table's
// roles for screen readers and no look of its own, as gpui-base's Table
// and its parts: TableHeader, TableBody and TableFooter around TableRows of
// TableHeads and TableCells, and a TableCaption. Label is what screen
// readers call it; a caption is content the app shows. For many rows,
// sorting, selection and resizing, DataTable is the table.
type Table struct {
	Label     string
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t Table) Render(*core.Scope) core.Node {
	return core.View{Role: core.RoleTable, Label: t.Label, Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Children: t.Children, TestID: t.TestID,
		OnLayout: t.OnLayout, Tooltip: t.Tooltip}
}

// TablePart holds a part of a Table: Children, styled by Class, Style and
// StyleFunc.
type TablePart struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

func (p TablePart) view(role core.Role, row bool) core.Node {
	s := p.Style
	if row && s.Direction == 0 {
		s.Direction = core.Row
	}
	return core.View{Role: role, Class: p.Class, Style: s, StyleFunc: p.StyleFunc, Children: p.Children, TestID: p.TestID,
		OnLayout: p.OnLayout, Tooltip: p.Tooltip}
}

// TableHeader is a Table's header group: the rows of column headers.
type TableHeader TablePart

// Render implements core.Node.
func (p TableHeader) Render(*core.Scope) core.Node {
	return TablePart(p).view(core.RoleRowGroup, false)
}

// TableBody is a Table's body group: its rows of data.
type TableBody TablePart

// Render implements core.Node.
func (p TableBody) Render(*core.Scope) core.Node { return TablePart(p).view(core.RoleRowGroup, false) }

// TableFooter is a Table's footer group: totals, a summary row.
type TableFooter TablePart

// Render implements core.Node.
func (p TableFooter) Render(*core.Scope) core.Node {
	return TablePart(p).view(core.RoleRowGroup, false)
}

// TableRow is a row of a Table: its cells side by side.
type TableRow TablePart

// Render implements core.Node.
func (p TableRow) Render(*core.Scope) core.Node { return TablePart(p).view(core.RoleRow, true) }

// TableHead is a column's header cell, in a TableHeader's row.
type TableHead TablePart

// Render implements core.Node.
func (p TableHead) Render(*core.Scope) core.Node {
	return TablePart(p).view(core.RoleColumnHeader, false)
}

// TableCell is a cell of a TableRow.
type TableCell TablePart

// Render implements core.Node.
func (p TableCell) Render(*core.Scope) core.Node { return TablePart(p).view(core.RoleCell, false) }

// TableCaption is content the app shows about a Table: a title, a note
// under it.
type TableCaption TablePart

// Render implements core.Node.
func (p TableCaption) Render(*core.Scope) core.Node { return TablePart(p).view(core.RoleNone, false) }
