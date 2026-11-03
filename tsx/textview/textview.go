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
		tsx.Omit("StyleOf", "Highlighter", "CustomSpan"),
		tsx.Enum(map[string]textview.Alert{"caution": textview.AlertCaution, "important": textview.AlertImportant, "none": textview.AlertNone, "note": textview.AlertNote, "tip": textview.AlertTip, "warning": textview.AlertWarning}),
		tsx.Enum(map[string]textview.ColumnAlign{"center": textview.AlignCenter, "left": textview.AlignLeft, "none": textview.AlignNone, "right": textview.AlignRight}),
		tsx.Enum(map[string]textview.Part{"alert": textview.PartAlert, "alertCaution": textview.PartAlertCaution, "alertContent": textview.PartAlertContent, "alertImportant": textview.PartAlertImportant, "alertNote": textview.PartAlertNote, "alertTip": textview.PartAlertTip, "alertTitle": textview.PartAlertTitle, "alertWarning": textview.PartAlertWarning, "blockquote": textview.PartBlockquote, "code": textview.PartCode, "codeBlock": textview.PartCodeBlock, "codeBlockAction": textview.PartCodeBlockAction, "codeBlockCode": textview.PartCodeBlockCode, "codeBlockHeader": textview.PartCodeBlockHeader, "codeBlockScroll": textview.PartCodeBlockScroll, "custom": textview.PartCustom, "details": textview.PartDetails, "emphasis": textview.PartEmphasis, "footnote": textview.PartFootnote, "footnoteLabel": textview.PartFootnoteLabel, "footnoteReference": textview.PartFootnoteReference, "footnotes": textview.PartFootnotes, "heading": textview.PartHeading, "heading1": textview.PartHeading1, "heading2": textview.PartHeading2, "heading3": textview.PartHeading3, "heading4": textview.PartHeading4, "heading5": textview.PartHeading5, "heading6": textview.PartHeading6, "image": textview.PartImage, "keyboard": textview.PartKeyboard, "link": textview.PartLink, "list": textview.PartList, "listItem": textview.PartListItem, "listItemContent": textview.PartListItemContent, "listMarker": textview.PartListMarker, "mark": textview.PartMark, "math": textview.PartMath, "mathBlock": textview.PartMathBlock, "orderedList": textview.PartOrderedList, "paragraph": textview.PartParagraph, "root": textview.PartRoot, "rule": textview.PartRule, "small": textview.PartSmall, "strikethrough": textview.PartStrikethrough, "strong": textview.PartStrong, "subscript": textview.PartSubscript, "summary": textview.PartSummary, "summaryMarker": textview.PartSummaryMarker, "superscript": textview.PartSuperscript, "table": textview.PartTable, "tableActions": textview.PartTableActions, "tableCell": textview.PartTableCell, "tableHead": textview.PartTableHead, "tableHeaderCell": textview.PartTableHeaderCell, "tableRow": textview.PartTableRow, "tableScroll": textview.PartTableScroll, "taskDone": textview.PartTaskDone, "taskMarker": textview.PartTaskMarker, "taskMarkerChecked": textview.PartTaskMarkerChecked, "underline": textview.PartUnderline, "unorderedList": textview.PartUnorderedList}),
	)
	tsx.Required[textview.Image]("Src")
	tsx.Required[textview.Math]("TeX", "Display")
	tsx.Required[textview.CodeBlock]("Code")
	tsx.Required[textview.CodeAction]("ID")
	tsx.Required[textview.LinkPressEvent]("URL")
	tsx.Optional[textview.Image]("Width", "Height")
	// NSH's options say what is read, Go's what is not.
	tsx.Fields[textview.Options](map[string]string{"NoMath": "math", "NoHTML": "html", "NoAlerts": "alerts"})
}
