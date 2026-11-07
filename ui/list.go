package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Selection is base.Selection, the rows a List or a Tree has chosen.
type Selection = base.Selection

// NewSelection returns an empty selection of one row, or with multiple
// of any number.
func NewSelection(cx *core.Scope, multiple bool) *Selection { return base.NewSelection(cx, multiple) }

// List is base.List in Onevra's look: rounded rows, the chosen ones in the
// selection's color and the keyboard's cursor ringed. Give it a height, or
// let it fill what the app sizes.
type List struct {
	Count func() int
	// Key returns row i's identity, a comparable value such as an ID,
	// which the selection holds; without it, the selection holds indexes.
	Key func(i int) any
	// Row builds a row's content: a cell is built once and shows other
	// rows as it scrolls, so read row's index as a signal.
	Row       func(cx *core.Scope, row core.ListRow) core.Node
	Selection *Selection
	// OnActivate receives the row Enter or a double click activates, or
	// on touch screens a tap in a list that is not Multiple.
	OnActivate func(i int)
	// TapSelects has a tap only select on touch screens, and a second
	// activate, for rows a tap should not leave.
	TapSelects bool
	// RowLabel is what assistive technology reads for row i, its text
	// when nil.
	RowLabel func(i int) string
	// Columns lays rows side by side, 1 when 0; the arrows then move
	// along a line and between lines.
	Columns int
	// EstimatedSize is a row's expected height, a row's height when 0.
	EstimatedSize core.Points
	Gap           core.Points
	Header        core.Node
	Footer        core.Node
	// Empty shows while Count is 0.
	Empty core.Node
	// OnLoadMore runs as the end of the rows comes near, for the next page.
	OnLoadMore func()
	// Label names the list for assistive technology.
	Label  string
	Ref    *core.Ref
	TestID string
	Style  core.Style
	// RowStyle merges over each row's, and RowStyleFunc, read as the row's
	// colors are, over that.
	RowStyle     core.Style
	RowStyleFunc func() core.Style
}

// rowLook is a row's colors in a List or a Tree: the chosen rows' and the
// ring of the keyboard's cursor.
func rowLook(cx *core.Scope, interactive bool, style core.Style, styleFunc func() core.Style) func() core.Style {
	colors := ColorsOf(cx)
	return func() core.Style {
		c := colors.Now()
		s := core.Style{Color: c.Text}
		if interactive {
			s.Selected = &core.Style{Background: c.Selection}
			s.Focus = &core.Style{OutlineWidth: 2, OutlineColor: c.Info, OutlineOffset: -2}
		}
		s = s.Merge(style)
		if styleFunc != nil {
			s = s.Merge(styleFunc())
		}
		return s
	}
}

// Render implements core.Node.
func (l List) Render(cx *core.Scope) core.Node {
	interactive := l.Selection != nil || l.OnActivate != nil
	row := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(RowHeight - 8),
		Padding: core.Symmetric(4, 10), CornerRadius: RadiusControl, MinWidth: core.Pt(0)}
	if interactive {
		row.Cursor = core.CursorDefault
	}
	size := l.EstimatedSize
	if size == 0 {
		size = RowHeight
	}
	return base.List{Count: l.Count, Key: l.Key, Row: l.Row, Selection: l.Selection, OnActivate: l.OnActivate, TapSelects: l.TapSelects, RowLabel: l.RowLabel,
		Columns: l.Columns, EstimatedSize: size, Gap: l.Gap, Header: l.Header, Footer: l.Footer, Empty: l.Empty, OnLoadMore: l.OnLoadMore,
		Label: l.Label, Ref: l.Ref, TestID: l.TestID,
		Style:        core.Style{MinHeight: core.Pt(0), CornerRadius: RadiusControl}.Merge(l.Style),
		StyleFunc:    func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		ListStyle:    core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Padding: core.All(4)},
		RowStyle:     row,
		RowStyleFunc: rowLook(cx, interactive, l.RowStyle, l.RowStyleFunc)}
}
