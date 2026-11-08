// Package richeditor gives TSX nsgo/base's RichEditor over its own session of
// the starter kit and tables, which links packages other apps should not: an
// app whose TSX uses it imports this package for its effect.
package richeditor

import (
	"github.com/oneo-me/nsgo/base/richeditor"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
)

func init() {
	tsx.Register("nsgo/base", map[string]core.Node{"RichEditor": richeditor.View{}},
		// Go values, for Go apps.
		tsx.Omit("Editor", "NodeViews", "BubbleMenu", "BubbleMenuPlacement", "BubbleMenuOffset", "OnSelectionChange", "OnPasteFiles", "Ref"),
		tsx.Event("OnChange", "revision"),
	)
}
