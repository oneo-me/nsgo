package layout

import "slices"

// Equal reports whether s and o are the same style, an empty list the
// same as none. It compares field by field: reflect.DeepEqual, which the
// core's restyle used, took a tenth of the web page's time in a fast
// scroll of a table's cells. TestStyleEqualSeesEveryField keeps it to
// every field.
func (s *Style) Equal(o *Style) bool {
	return s.Display == o.Display && s.ItemIsTable == o.ItemIsTable && s.ItemIsReplaced == o.ItemIsReplaced &&
		s.ItemIsCompressibleReplaced == o.ItemIsCompressibleReplaced && s.BoxSizing == o.BoxSizing &&
		s.Direction == o.Direction && s.Overflow == o.Overflow && s.ScrollbarWidth == o.ScrollbarWidth &&
		s.Contain == o.Contain && s.Position == o.Position && s.Inset == o.Inset &&
		s.Size == o.Size && s.MinSize == o.MinSize && s.MaxSize == o.MaxSize && s.AspectRatio == o.AspectRatio &&
		s.Margin == o.Margin && s.Padding == o.Padding && s.Border == o.Border &&
		s.AlignItems == o.AlignItems && s.JustifyItems == o.JustifyItems && s.AlignSelf == o.AlignSelf &&
		s.JustifySelf == o.JustifySelf && s.AlignContent == o.AlignContent && s.JustifyContent == o.JustifyContent &&
		s.Gap == o.Gap && s.TextAlign == o.TextAlign && s.FlexDirection == o.FlexDirection &&
		s.FlexWrap == o.FlexWrap && s.FlexBasis == o.FlexBasis && s.FlexGrow == o.FlexGrow && s.FlexShrink == o.FlexShrink &&
		s.GridAutoFlow == o.GridAutoFlow && s.GridRow == o.GridRow && s.GridColumn == o.GridColumn &&
		slices.EqualFunc(s.GridTemplateRows, o.GridTemplateRows, GridTemplateComponent.equal) &&
		slices.EqualFunc(s.GridTemplateColumns, o.GridTemplateColumns, GridTemplateComponent.equal) &&
		namesEqual(s.GridTemplateRowNames, o.GridTemplateRowNames) &&
		namesEqual(s.GridTemplateColumnNames, o.GridTemplateColumnNames) &&
		s.GridTemplateAreas.Rows == o.GridTemplateAreas.Rows && s.GridTemplateAreas.Columns == o.GridTemplateAreas.Columns &&
		slices.Equal(s.GridTemplateAreas.Areas, o.GridTemplateAreas.Areas) &&
		slices.Equal(s.GridAutoRows, o.GridAutoRows) && slices.Equal(s.GridAutoColumns, o.GridAutoColumns)
}

func (c GridTemplateComponent) equal(o GridTemplateComponent) bool {
	if c.Track != o.Track || (c.Repeat == nil) != (o.Repeat == nil) {
		return false
	}
	if c.Repeat == nil || c.Repeat == o.Repeat {
		return true
	}
	r, q := c.Repeat, o.Repeat
	return r.Count == q.Count && slices.Equal(r.Tracks, q.Tracks) && namesEqual(r.LineNames, q.LineNames)
}

// namesEqual compares groups of line names.
func namesEqual(a, b [][]string) bool { return slices.EqualFunc(a, b, slices.Equal) }
