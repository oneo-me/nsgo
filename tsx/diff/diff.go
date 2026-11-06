// Package diff gives TSX nsgo/base's DiffView, the changes between
// texts, which links packages other apps should not: an app whose TSX
// uses it imports this package for its effect.
package diff

import (
	"github.com/oneo-me/nsgo/base/diff"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
)

func init() {
	tsx.Register("nsgo/base", map[string]core.Node{"DiffView": diff.DiffView{}},
		tsx.Enum(map[string]diff.Expansion{"down": diff.ExpandDown, "all": diff.ExpandEvery, "up": diff.ExpandUp}),
		tsx.Flag("Unified", "layout", "unified", "DiffLayout"),
		tsx.Event("OnChange", "change"), tsx.Event("OnLineNumberPress", "line"), tsx.Event("OnStatsChange", "stats"),
		tsx.Event("OnLineSelectionChange", "range"), tsx.Event("OnFileToggle", "file"),
		tsx.Enum(map[string]diff.FileStatus{"added": diff.FileAdded, "copied": diff.FileCopied, "deleted": diff.FileDeleted, "modified": diff.FileModified, "renamed": diff.FileRenamed}),
		tsx.Enum(map[string]diff.Indicators{"bars": diff.IndicatorBars, "classic": diff.IndicatorClassic, "none": diff.IndicatorNone}),
		tsx.Enum(map[string]diff.Inline{"char": diff.InlineChar, "none": diff.InlineNone, "word": diff.InlineWord, "wordAlt": diff.InlineWordAlt}),
		tsx.Enum(map[string]diff.Side{"new": diff.New, "old": diff.Old}),
	)
	tsx.Required[diff.Change]("OldValue", "NewValue")
	// NSH's lines count from 0, Go's from 1.
	tsx.Shape("DiffHunk", `export interface DiffHunk { index: number; oldStart: number; oldLines: number; newStart: number; newLines: number; file: number }`,
		func(h diff.Hunk) map[string]any {
			return map[string]any{"index": h.Index, "oldStart": max(h.OldStart-1, 0), "oldLines": h.OldLines,
				"newStart": max(h.NewStart-1, 0), "newLines": h.NewLines, "file": h.File}
		}, nil)
}
