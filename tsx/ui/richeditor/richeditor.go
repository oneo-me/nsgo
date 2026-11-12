// Package richeditor gives TSX nsgo/ui's RichEditor, ui/richeditor's View
// over a RichEditorSession, which links packages other apps should not: an
// app whose TSX uses it imports this package for its effect.
package richeditor

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
	trich "github.com/nsgo-dev/nsgo/tsx/richeditor"
	uirich "github.com/nsgo-dev/nsgo/ui/richeditor"
)

// View is nsgo/base's RichEditor in ui's look.
type View trich.View

// Render implements core.Node.
func (v View) Render(*core.Scope) core.Node { return uirich.View(trich.View(v).Base()) }

func init() {
	tsx.Register("nsgo/ui", map[string]core.Node{"RichEditor": View{}}, tsx.Event("OnChange", "revision"))
}
