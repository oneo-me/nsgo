package textview

import "github.com/oneo-me/nsgo/core"

// Part is a part of a TextView the app styles. A node with several parts
// (PartHeading then PartHeading2) merges their styles, later parts winning.
type Part uint8

const (
	// PartRoot is the text view itself, a column of its blocks: text
	// styles here pass to every part.
	PartRoot Part = iota
	PartParagraph
	// PartHeading is every heading, before its level's part.
	PartHeading
	PartHeading1
	PartHeading2
	PartHeading3
	PartHeading4
	PartHeading5
	PartHeading6
	// PartBlockquote is quotations and alerts: a column of their blocks.
	PartBlockquote
	// PartAlert is every alert (> [!NOTE]), after PartBlockquote and
	// before its kind's part.
	PartAlert
	PartAlertNote
	PartAlertTip
	PartAlertImportant
	PartAlertWarning
	PartAlertCaution
	// PartAlertTitle is the title line of an alert ("Note"), inside the
	// alert's box.
	PartAlertTitle
	// PartAlertContent is the column of an alert's blocks, below its
	// title.
	PartAlertContent
	// PartList is every list, a column of its items, before
	// PartOrderedList or PartUnorderedList.
	PartList
	PartOrderedList
	PartUnorderedList
	// PartListItem is every item: a row of its marker and its content.
	PartListItem
	// PartListMarker is an item's marker: "•", "◦" or "▪" by depth, or its
	// number.
	PartListMarker
	// PartTaskMarker is a task item's box in place of PartListMarker,
	// PartTaskMarkerChecked after it when done.
	PartTaskMarker
	PartTaskMarkerChecked
	// PartListItemContent is the column of an item's blocks.
	PartListItemContent
	// PartTaskDone is the content of a done task, after
	// PartListItemContent.
	PartTaskDone
	// PartCodeBlock is a code block: its scroll view and the actions over
	// it.
	PartCodeBlock
	// PartCodeBlockHeader is the row of a code block's actions, absolutely
	// placed over it.
	PartCodeBlockHeader
	// PartCodeBlockAction is each action button in PartCodeBlockHeader.
	PartCodeBlockAction
	// PartCodeBlockScroll is the horizontal scroll view of a block's code.
	PartCodeBlockScroll
	// PartCodeBlockCode is the text of a block's code.
	PartCodeBlockCode
	// PartCode is code in text.
	PartCode
	// PartKeyboard is keys to press (<kbd>).
	PartKeyboard
	PartStrong
	PartEmphasis
	PartStrikethrough
	PartUnderline
	// PartMark is marked text (<mark>), under the color it asks for.
	PartMark
	PartSuperscript
	PartSubscript
	PartSmall
	PartLink
	// PartImage is images alone in a paragraph: their natural size, at
	// most the text's width, unless they ask for one.
	PartImage
	// PartTable is a table: the scroll view of its cells.
	PartTable
	// PartTableActions holds RenderTableActions' content, above the table.
	PartTableActions
	// PartTableScroll is the horizontal scroll view of a table's grid;
	// it scrolls when the columns cannot shrink further.
	PartTableScroll
	// PartTableRow is every row: a view behind its cells, for backgrounds
	// and borders. Text styles go on the cells.
	PartTableRow
	// PartTableHead is the header row, after PartTableRow.
	PartTableHead
	PartTableCell
	// PartTableHeaderCell is the header row's cells, after PartTableCell.
	PartTableHeaderCell
	// PartRule is a thematic break, a view.
	PartRule
	// PartMath is a formula in text: its TeX.
	PartMath
	// PartMathBlock is a formula block: its TeX.
	PartMathBlock
	// PartDetails is a disclosure: a column of its summary and, while
	// open, its blocks.
	PartDetails
	// PartSummary is a disclosure's summary, a row of its marker and its
	// text; a press opens or closes it.
	PartSummary
	// PartSummaryMarker is the summary's marker: "▸" closed, "▾" open.
	PartSummaryMarker
	// PartFootnotes is the column of footnotes after the blocks.
	PartFootnotes
	// PartFootnote is a footnote: a row of its label and its blocks.
	PartFootnote
	PartFootnoteLabel
	// PartFootnoteReference is a reference to a footnote in text: its
	// number in brackets.
	PartFootnoteReference
	// PartCustom is custom nodes shown as their content.
	PartCustom
)

// Styles are styles by part.
type Styles map[Part]core.Style

// Classes are Tailwind classes by part, under the parts' styles.
type Classes map[Part]string
