// Package textview gives TSX nsgo/base's TextView, Markdown and HTML
// documents, which links packages other apps should not: an app whose
// TSX uses it imports this package for its effect.
package textview

import (
	"github.com/oneo-me/nsgo/base/textview"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
)

func init() {
	tsx.Register("nsgo/base", map[string]core.Node{"TextView": textview.TextView{}},
		tsx.Event("OnImagePress", "image"),
		tsx.Union[textview.Block](map[string]any{"paragraph": textview.Paragraph{}, "heading": textview.Heading{},
			"blockquote": textview.Blockquote{}, "list": textview.List{}, "codeBlock": textview.CodeBlock{},
			"table": textview.Table{}, "rule": textview.Rule{}, "details": textview.Details{}, "math": textview.Math{},
			"custom": textview.Custom{}}),
		tsx.Union[textview.Inline](map[string]any{"text": textview.Text{}, "strong": textview.Strong{},
			"emphasis": textview.Emphasis{}, "strikethrough": textview.Strikethrough{},
			"underline": textview.Underline{}, "highlight": textview.Mark{}, "superscript": textview.Superscript{},
			"subscript": textview.Subscript{}, "small": textview.Small{}, "code": textview.Code{},
			"keyboard": textview.Keyboard{}, "math": textview.Math{}, "link": textview.Link{}, "image": textview.Image{},
			"break": textview.Break{}, "footnoteReference": textview.FootnoteReference{}, "custom": textview.Custom{}}),
		tsx.Flag("OpenLinks", "links", "open", "TextViewLinks"),
		// Go functions of Go values, for Go apps.
		tsx.Omit("Highlighter", "CustomSpan"),
		// TSX apps read documents in NSH's typography, under their own
		// classes and styles; Go's TextView takes the app's.
		tsx.Preset("StyleOf", typography),
		tsx.Enum(map[string]textview.Alert{"caution": textview.AlertCaution, "important": textview.AlertImportant, "none": textview.AlertNone, "note": textview.AlertNote, "tip": textview.AlertTip, "warning": textview.AlertWarning}),
		tsx.Enum(map[string]textview.ColumnAlign{"center": textview.AlignCenter, "left": textview.AlignLeft, "none": textview.AlignNone, "right": textview.AlignRight}),
		tsx.Enum(map[string]textview.Part{"alert": textview.PartAlert, "alertCaution": textview.PartAlertCaution, "alertContent": textview.PartAlertContent, "alertImportant": textview.PartAlertImportant, "alertNote": textview.PartAlertNote, "alertTip": textview.PartAlertTip, "alertTitle": textview.PartAlertTitle, "alertWarning": textview.PartAlertWarning, "blockquote": textview.PartBlockquote, "code": textview.PartCode, "codeBlock": textview.PartCodeBlock, "codeBlockAction": textview.PartCodeBlockAction, "codeBlockCode": textview.PartCodeBlockCode, "codeBlockHeader": textview.PartCodeBlockHeader, "codeBlockScroll": textview.PartCodeBlockScroll, "custom": textview.PartCustom, "details": textview.PartDetails, "emphasis": textview.PartEmphasis, "footnote": textview.PartFootnote, "footnoteLabel": textview.PartFootnoteLabel, "footnoteReference": textview.PartFootnoteReference, "footnotes": textview.PartFootnotes, "heading": textview.PartHeading, "heading1": textview.PartHeading1, "heading2": textview.PartHeading2, "heading3": textview.PartHeading3, "heading4": textview.PartHeading4, "heading5": textview.PartHeading5, "heading6": textview.PartHeading6, "image": textview.PartImage, "keyboard": textview.PartKeyboard, "link": textview.PartLink, "list": textview.PartList, "listItem": textview.PartListItem, "listItemContent": textview.PartListItemContent, "listMarker": textview.PartListMarker, "mark": textview.PartMark, "math": textview.PartMath, "mathBlock": textview.PartMathBlock, "orderedList": textview.PartOrderedList, "paragraph": textview.PartParagraph, "root": textview.PartRoot, "rule": textview.PartRule, "small": textview.PartSmall, "strikethrough": textview.PartStrikethrough, "strong": textview.PartStrong, "subscript": textview.PartSubscript, "summary": textview.PartSummary, "summaryMarker": textview.PartSummaryMarker, "superscript": textview.PartSuperscript, "table": textview.PartTable, "tableActions": textview.PartTableActions, "tableCell": textview.PartTableCell, "tableHead": textview.PartTableHead, "tableHeaderCell": textview.PartTableHeaderCell, "tableRow": textview.PartTableRow, "tableScroll": textview.PartTableScroll, "taskDone": textview.PartTaskDone, "taskMarker": textview.PartTaskMarker, "taskMarkerChecked": textview.PartTaskMarkerChecked, "underline": textview.PartUnderline, "unorderedList": textview.PartUnorderedList}),
	)
	// NSH's static functions of TextView: a document of Markdown or HTML,
	// as the view would read it, and a document's text.
	tsx.RegisterStatics("nsgo/base", "TextView", map[string]any{
		"parse": func(markdown string, o textview.Options, extensions []textview.Extension) textview.Document {
			return textview.ParseWith(markdown, o, extensions...)
		},
		"parseHtml": func(html string, extensions []textview.Extension) textview.Document {
			return textview.ParseHTML(html, extensions...)
		},
		"plainText": func(d textview.Document) string { return d.PlainText() },
	})
	tsx.Required[textview.Image]("Src")
	tsx.Required[textview.Math]("TeX", "Display")
	tsx.Required[textview.CodeBlock]("Code")
	tsx.Required[textview.CodeAction]("ID")
	tsx.Required[textview.LinkPressEvent]("URL")
	tsx.Optional[textview.Image]("Width", "Height")
	// NSH's options say what is read, Go's what is not.
	tsx.Fields[textview.Options](map[string]string{"NoMath": "math", "NoHTML": "html", "NoAlerts": "alerts"})
}

// typography is NSH's reading typography of TextView (nsh-base's
// text_view/view.rs): a 15-point body, semibold headings sized by level
// and strong text.
func typography(p textview.Part) core.Style {
	heading := func(size, line, tracking float32) core.Style {
		return core.Style{FontSize: core.Points(size), FontWeight: core.FontWeightSemibold,
			LineHeight: core.Points(size * line), LetterSpacing: core.Points(size * tracking)}
	}
	switch p {
	case textview.PartRoot:
		return core.Style{FontSize: 15, LineHeight: 15 * 1.7}
	case textview.PartHeading1:
		return heading(27, 1.25, -0.02)
	case textview.PartHeading2:
		return heading(20, 1.35, -0.012)
	case textview.PartHeading3:
		return heading(17, 1.35, -0.012)
	case textview.PartHeading4, textview.PartHeading5, textview.PartHeading6:
		return heading(15, 1.35, -0.012)
	case textview.PartStrong:
		return core.Style{FontWeight: core.FontWeightSemibold}
	}
	return core.Style{}
}
