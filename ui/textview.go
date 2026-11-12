package ui

import (
	"github.com/nsgo-dev/nsgo/base/textview"
	"github.com/nsgo-dev/nsgo/core"
)

// TextView is textview.TextView in ui's type and colors: its
// headings, quotes and alerts, lists, code, tables and links. The app's
// StyleOf merges over ui's styles, and its Styles over both, as
// they do in base. Code is colored by the app's Highlighter, such as
// highlight.New's.
type TextView textview.TextView

// Render implements core.Node.
func (v TextView) Render(cx *core.Scope) core.Node {
	styles := Themed(cx, func() textview.Styles { return TextViewStyles(cx) })
	out := textview.TextView(v)
	app := v.StyleOf
	out.StyleOf = func(p textview.Part) core.Style {
		s := styles()[p]
		if app != nil {
			s = s.Merge(app(p))
		}
		return s
	}
	return out
}

// TextViewStyles are TextView's styles where cx is, by part, for a
// textview.TextView the app styles further. It reads the palette, so
// read it where Themed does.
func TextViewStyles(cx *core.Scope) textview.Styles {
	c := ColorsOf(cx).Now()
	semibold := core.Style{FontWeight: Semibold}
	code := TypeCode.Merge(core.Style{Background: c.Inset, CornerRadius: RadiusSmall, Color: c.Code})
	alert := func(color core.Color) core.Style {
		return core.Style{BorderColors: core.EdgeColors{Left: color}, Color: c.Text}
	}
	size := func(s core.Style, by core.Points) core.Style {
		s.FontSize += by
		s.LineHeight += by + by/3
		return s
	}
	return textview.Styles{
		textview.PartRoot:      TypeBody.Merge(core.Style{Color: c.Text, Gap: 12, SelectionColor: c.Selection}),
		textview.PartParagraph: {},
		textview.PartHeading:   semibold.Merge(core.Style{Color: c.Text, Margin: core.Edges{Top: 8}}),
		textview.PartHeading1:  size(TypeHeading, 4),
		textview.PartHeading2:  TypeHeading,
		textview.PartHeading3:  TypeTitle,
		textview.PartHeading4:  size(TypeLabel, 1).Merge(semibold),
		textview.PartHeading5:  TypeLabel.Merge(semibold),
		textview.PartHeading6:  TypeLabel.Merge(semibold).Merge(core.Style{Color: c.Muted}),

		textview.PartBlockquote: {Gap: 8, Padding: core.Edges{Left: 12}, Borders: core.Edges{Left: 3},
			BorderColors: core.EdgeColors{Left: c.Border}, Color: c.Muted},
		textview.PartAlert: {Gap: 4, Padding: core.Symmetric(8, 12), Background: c.Inset, Borders: core.Edges{Left: 3},
			CornerRadii: core.Corners{TopRight: RadiusSmall, BottomRight: RadiusSmall}},
		textview.PartAlertNote:      alert(c.Info),
		textview.PartAlertTip:       alert(c.Up),
		textview.PartAlertImportant: alert(c.Accent),
		textview.PartAlertWarning:   alert(c.Warn),
		textview.PartAlertCaution:   alert(c.Danger),
		textview.PartAlertTitle:     TypeLabel,
		textview.PartAlertContent:   {Gap: 8},

		textview.PartList:       {Gap: 4},
		textview.PartListItem:   {Direction: core.Row, AlignItems: core.AlignStart, Gap: 8},
		textview.PartListMarker: {Color: c.Muted, MinWidth: core.Pt(TypeBody.FontSize)},
		textview.PartTaskMarker: {Width: core.Pt(TypeBody.FontSize), Height: core.Pt(TypeBody.FontSize),
			Margin: core.Edges{Top: (TypeBody.LineHeight - TypeBody.FontSize) / 2}, CornerRadius: RadiusSmall, BorderWidth: 1,
			BorderColor: c.FieldBorder, Background: c.Field, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Color: c.OnAccent, FontSize: TypeCaption.FontSize, LineHeight: TypeCaption.FontSize},
		textview.PartTaskMarkerChecked: {Background: c.AccentFill, BorderColor: c.AccentFill},
		textview.PartListItemContent:   {Gap: 4, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)},
		textview.PartTaskDone:          {Color: c.Muted, TextDecoration: core.TextDecorationLineThrough},

		textview.PartCodeBlock:       {Background: c.Inset, CornerRadius: RadiusInset, BorderWidth: 1, BorderColor: c.Line},
		textview.PartCodeBlockHeader: {Direction: core.Row, Gap: 4, Padding: core.All(4), Top: core.Pt(0), Right: core.Pt(0)},
		textview.PartCodeBlockAction: TypeCaption.Merge(core.Style{Padding: core.Symmetric(2, 8), CornerRadius: RadiusSmall,
			Background: c.Raised, BorderWidth: 1, BorderColor: c.Border, Color: c.Muted,
			Hover: &core.Style{Color: c.Text}}),
		textview.PartCodeBlockScroll: {Padding: core.Symmetric(12, 16)},
		textview.PartCodeBlockCode:   TypeCode.Merge(core.Style{Color: c.Code}),
		textview.PartCode:            code,
		textview.PartKeyboard:        code.Merge(core.Style{Background: c.Raised}),

		textview.PartStrong:        semibold,
		textview.PartEmphasis:      {FontStyle: core.FontStyleItalic},
		textview.PartStrikethrough: {TextDecoration: core.TextDecorationLineThrough},
		textview.PartUnderline:     {TextDecoration: core.TextDecorationUnderline},
		textview.PartMark:          {Background: c.Selection},
		textview.PartSuperscript:   {FontSize: TypeCaption.FontSize},
		textview.PartSubscript:     {FontSize: TypeCaption.FontSize},
		textview.PartSmall:         {FontSize: TypeCaption.FontSize},
		textview.PartLink:          {Color: c.Accent, TextDecoration: core.TextDecorationUnderline, TextDecorationColor: c.Accent.Faded(0.4)},
		textview.PartImage:         {CornerRadius: RadiusControl},

		textview.PartTable:           {BorderWidth: 1, BorderColor: c.Border, CornerRadius: RadiusControl, Overflow: core.Hidden},
		textview.PartTableRow:        {Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: c.Line}},
		textview.PartTableHead:       {Background: c.Inset, Borders: core.Edges{}},
		textview.PartTableCell:       {Padding: core.Symmetric(8, 12)},
		textview.PartTableHeaderCell: semibold,
		textview.PartTableActions:    {Direction: core.Row, JustifyContent: core.JustifyEnd, Gap: 4},

		textview.PartRule:      {Height: core.Pt(1), Background: c.Line, Margin: core.Symmetric(8, 0)},
		textview.PartMath:      TypeCode,
		textview.PartMathBlock: TypeCode.Merge(core.Style{Padding: core.All(12), Background: c.Inset, CornerRadius: RadiusControl}),

		textview.PartDetails:       {Gap: 8, Padding: core.Symmetric(8, 12), BorderWidth: 1, BorderColor: c.Border, CornerRadius: RadiusControl},
		textview.PartSummary:       TypeLabel.Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}),
		textview.PartSummaryMarker: {Color: c.Muted},

		textview.PartFootnotes: TypeCaption.Merge(core.Style{Gap: 4, Padding: core.Edges{Top: 12}, Borders: core.Edges{Top: 1},
			BorderColors: core.EdgeColors{Top: c.Line}, Color: c.Muted}),
		textview.PartFootnote:          {Direction: core.Row, Gap: 8},
		textview.PartFootnoteLabel:     {Color: c.Muted},
		textview.PartFootnoteReference: {Color: c.Accent, FontSize: TypeCaption.FontSize},
	}
}
