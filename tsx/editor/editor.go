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
	)
	tsx.Required[editor.CompletionItem]("Label")
	tsx.Required[editor.Diagnostic]("Message")
	tsx.Required[editor.RelatedInformation]("Message")
	tsx.Required[editor.CodeDescription]("Href")
	tsx.Required[editor.Search]("Query")
	tsx.Required[editor.Edit]("Text")
	tsx.Optional[editor.SearchResult]("Current")
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
