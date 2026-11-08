// Package editor gives TSX nsgo/base's Editor, a code editor, which
// links packages other apps should not: an app whose TSX uses it imports
// this package for its effect.
package editor

import (
	"github.com/oneo-me/nsgo/base/editor"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
)

func init() {
	tsx.Register("nsgo/base", map[string]core.Node{"Editor": editor.Editor{}},
		tsx.Enum(map[string]editor.CompletionTrigger{"character": editor.CompletionCharacter, "invoked": editor.CompletionInvoked, "typing": editor.CompletionTyping}),
		tsx.Enum(map[string]editor.DefinitionTrigger{"hover": editor.DefinitionHover, "key": editor.DefinitionKey, "press": editor.DefinitionPress}),
		tsx.Enum(map[string]editor.Severity{"error": editor.SeverityError, "hint": editor.SeverityHint, "info": editor.SeverityInfo, "warning": editor.SeverityWarning}),
		tsx.Enum(map[string]editor.DiagnosticTag{"unnecessary": editor.TagUnnecessary, "deprecated": editor.TagDeprecated}),
		tsx.Enum(map[string]editor.GutterPlacement{"lines": editor.GutterLines, "before": editor.GutterBefore}),
		tsx.Enum(map[string]editor.OverviewRulerLane{"center": editor.LaneCenter, "left": editor.LaneLeft, "right": editor.LaneRight, "full": editor.LaneFull}),
		// TSX apps edit in NSH's look of diagnostics, under their own
		// classes and styles; Go's Editor takes the app's.
		tsx.Preset("Under", look),
		tsx.Event("OnChange", "value"), tsx.Event("OnSelectionChange", "selections"),
		tsx.Event("OnSearchChange", "result"), tsx.Event("OnLineNumberPress", "line"),
		tsx.Event("OnFindWithSelection", "query"), tsx.Event("OnFoldChange", "folds"),
	)
	tsx.Required[editor.CompletionItem]("Label")
	tsx.Required[editor.Diagnostic]("Message")
	tsx.Required[editor.RelatedInformation]("Message")
	tsx.Required[editor.CodeDescription]("Href")
	tsx.Required[editor.Search]("Query")
	tsx.Required[editor.Edit]("Text")
	// NSH's: no current match is null, and no error none.
	tsx.Shape("EditorSearchResult", `export interface EditorSearchResult { count: number; current?: number | null; error?: string | null }`,
		func(r editor.SearchResult) map[string]any {
			m := map[string]any{"count": r.Count, "current": nil, "error": nil}
			if r.Current >= 0 {
				m["current"] = r.Current
			}
			if r.Error != "" {
				m["error"] = r.Error
			}
			return m
		}, nil)
	// NSH's: the pointer as x and y, and the touch selection as a rect.
	tsx.Shape("EditorContextMenuEvent", `export interface EditorRect { x: number; y: number; width: number; height: number }
export interface EditorContextMenuEvent { position: EditorPosition; x: number; y: number; selection?: EditorRect | null }`,
		func(e editor.ContextMenuEvent) map[string]any {
			m := map[string]any{"position": e.Position, "x": float64(e.At.X), "y": float64(e.At.Y), "selection": nil}
			if b := e.Selection; b != (core.Bounds{}) {
				m["selection"] = map[string]any{"x": float64(b.Origin.X), "y": float64(b.Origin.Y),
					"width": float64(b.Size.Width), "height": float64(b.Size.Height)}
			}
			return m
		}, nil)
}

// look is NSH's Editor's look of diagnostics (nsh-base's editor/view.rs):
// a wavy line under the text in the severity's color, faded unnecessary
// code and struck deprecated code.
var look = func() editor.Styles {
	mark := func(c core.Color) core.Style {
		return core.Style{TextDecoration: core.TextDecorationUnderline, TextDecorationStyle: core.DecorationWavy,
			TextDecorationColor: c, TextDecorationThickness: core.Pt(1)}
	}
	return editor.Styles{
		DiagnosticError:   mark(core.RGB(0xe7000b)),
		DiagnosticWarning: mark(core.RGB(0xd97706)),
		DiagnosticInfo:    mark(core.RGB(0x2563eb)),
		DiagnosticHint:    mark(core.RGB(0x737373)),
		UnnecessaryText:   core.Style{Color: core.RGB(0x737373)},
		DeprecatedText:    core.Style{TextDecoration: core.TextDecorationLineThrough},
	}
}()
