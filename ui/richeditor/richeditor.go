// Package richeditor is base/richeditor's View in ui's colors. It is a
// package of its own, as base/richeditor is, so that only apps that edit
// rich text link its parsers. Its types are base/richeditor's: this package
// adds the look alone.
package richeditor

import (
	"maps"

	"github.com/nsgo-dev/nsgo/base/richeditor"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// View is richeditor.View in ui's look: reading text on the theme's
// colors, the accent for the caret and links, tables on hairlines with a
// recessed header, and images shown. The app's Styles or StylesFunc merge over
// ui's, part by part, and its NodeViews over the image's.
type View richeditor.View

// Render implements core.Node.
func (v View) Render(cx *core.Scope) core.Node {
	out := richeditor.View(v)
	out.NodeViews = maps.Clone(v.NodeViews)
	if out.NodeViews == nil {
		out.NodeViews = map[string]richeditor.NodeView{}
	}
	if out.NodeViews["image"] == nil {
		out.NodeViews["image"] = image
	}
	out.StylesFunc = ui.Themed(cx, func() richeditor.Styles {
		app := v.Styles
		if v.StylesFunc != nil {
			app = v.StylesFunc()
		}
		return merge(styles(cx), app)
	})
	return out
}

func image(cx *core.Scope, p richeditor.NodeProps) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{StyleFunc: func() core.Style {
		st := core.Style{AlignSelf: core.AlignStart, CornerRadius: ui.RadiusControl, Overflow: core.Hidden}
		if p.Selected() {
			st.OutlineWidth, st.OutlineColor = 2, colors.Accent()
		}
		return st
	}, Children: []core.Node{core.Image{ResizeMode: core.Contain,
		SourceFunc: func() core.ImageSource { return core.Src(p.Node().AttrStr("src")) },
		Style:      core.Style{MaxWidth: core.Pct(100), Height: core.Pt(200), Width: core.Pct(100)}}}}
}

func styles(cx *core.Scope) richeditor.Styles {
	c := ui.ColorsOf(cx).Now()
	heading := func(size core.Points, top core.Points) core.Style {
		return core.Style{FontSize: size, LineHeight: size * 1.3, FontWeight: core.FontWeightSemibold, Margin: core.Edges{Top: top}}
	}
	cellBorders := core.Style{Borders: core.Edges{Right: 1, Bottom: 1}, BorderColor: c.Line, Padding: core.Symmetric(4, 8), Gap: 8, MinWidth: core.Pt(0)}
	code := ui.TypeCode.Merge(core.Style{Background: c.Inset, CornerRadius: ui.RadiusSmall, Color: c.Code})
	// Blocks stand apart by their containers' gaps, so a cell's or an
	// item's one paragraph takes no margin.
	list := core.Style{Padding: core.Edges{Left: 24}, Gap: 4}
	return richeditor.Styles{
		Content: ui.TypeBody.Merge(core.Style{Color: c.Text, FontSize: 14, LineHeight: 22, Padding: core.Symmetric(12, 16), Gap: 8}),
		Nodes: map[string]core.Style{
			"heading[level=1]": heading(26, 12),
			"heading[level=2]": heading(21, 12),
			"heading[level=3]": heading(18, 8),
			"heading[level=4]": heading(16, 8),
			"heading[level=5]": heading(14, 8),
			"heading[level=6]": heading(13, 8),
			"blockquote":       {Borders: core.Edges{Left: 3}, BorderColor: c.Line, Padding: core.Edges{Left: 12}, Gap: 8, Color: c.Muted},
			"codeBlock":        code.Merge(core.Style{Padding: core.Symmetric(10, 12), CornerRadius: ui.RadiusControl}),
			"bulletList":       list,
			"orderedList":      list,
			"taskList":         list,
			"listItem":         {Gap: 4},
			"taskItem":         {Gap: 4},
			"horizontalRule":   {Height: core.Pt(1), Background: c.Line, Margin: core.Symmetric(4, 0)},
			"table":            {Borders: core.Edges{Top: 1, Left: 1}, BorderColor: c.Line},
			"tableCell":        cellBorders,
			"tableHeader":      cellBorders.Merge(core.Style{Background: c.Inset, FontWeight: core.FontWeightSemibold}),
		},
		Marks: map[string]core.Style{
			"bold":        {FontWeight: core.FontWeightSemibold},
			"italic":      {FontStyle: core.FontStyleItalic},
			"strike":      {TextDecoration: core.TextDecorationLineThrough},
			"underline":   {TextDecoration: core.TextDecorationUnderline},
			"code":        code.Merge(core.Style{FontSize: 13}),
			"link":        {Color: c.Accent, TextDecoration: core.TextDecorationUnderline},
			"highlight":   {Background: ui.Alpha(c.Accent, 0.2)},
			"subscript":   {FontSize: 11},
			"superscript": {FontSize: 11},
			"mention":     {Color: c.Accent, FontWeight: core.FontWeightMedium},
		},
		Placeholder:  core.Style{Color: c.Muted},
		Caret:        core.Style{Background: c.Accent},
		Selection:    core.Style{Background: ui.Alpha(c.Accent, 0.25)},
		Composition:  core.Style{TextDecoration: core.TextDecorationUnderline},
		GapCursor:    core.Style{Background: c.Text},
		DropCursor:   core.Style{Background: c.Accent},
		SelectedNode: core.Style{OutlineWidth: 2, OutlineColor: c.Accent},
		SelectedCell: core.Style{Background: ui.Alpha(c.Accent, 0.18)},
		ListMarker:   core.Style{Color: c.Muted},
		TaskCheckbox: core.Style{Width: core.Pt(14), Height: core.Pt(14), Top: core.Pt(4), Left: core.Pt(-21), BorderWidth: 1, BorderColor: c.Border, CornerRadius: 3,
			Selected: &core.Style{Background: c.Accent, BorderColor: c.Accent}},
		TableResizeHandle:  core.Style{Background: c.Accent},
		TableGrip:          core.Style{Background: c.Muted, CornerRadius: 3},
		TableDropIndicator: core.Style{Background: c.Accent},
	}
}

// merge is app's parts over base's.
func merge(base, app richeditor.Styles) richeditor.Styles {
	base.Content = base.Content.Merge(app.Content)
	base.Nodes = mergeMap(base.Nodes, app.Nodes)
	base.Marks = mergeMap(base.Marks, app.Marks)
	base.Placeholder = base.Placeholder.Merge(app.Placeholder)
	base.Caret = base.Caret.Merge(app.Caret)
	base.Selection = base.Selection.Merge(app.Selection)
	base.Composition = base.Composition.Merge(app.Composition)
	base.GapCursor = base.GapCursor.Merge(app.GapCursor)
	base.DropCursor = base.DropCursor.Merge(app.DropCursor)
	base.SelectedNode = base.SelectedNode.Merge(app.SelectedNode)
	base.SelectedCell = base.SelectedCell.Merge(app.SelectedCell)
	base.ListMarker = base.ListMarker.Merge(app.ListMarker)
	base.TaskCheckbox = base.TaskCheckbox.Merge(app.TaskCheckbox)
	base.TableResizeHandle = base.TableResizeHandle.Merge(app.TableResizeHandle)
	base.TableGrip = base.TableGrip.Merge(app.TableGrip)
	base.TableDropIndicator = base.TableDropIndicator.Merge(app.TableDropIndicator)
	return base
}

func mergeMap(base, app map[string]core.Style) map[string]core.Style {
	out := maps.Clone(base)
	for k, s := range app {
		out[k] = out[k].Merge(s)
	}
	return out
}
