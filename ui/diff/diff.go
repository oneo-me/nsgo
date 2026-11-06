// Package diff is base/diff's DiffView in ui's colors. It is a package of
// its own, as base/diff is, so that only apps that show diffs link chroma's
// lexers. Its types are base/diff's: this package adds the look alone.
package diff

import (
	"github.com/oneo-me/nsgo/base/diff"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

// View is diff.DiffView in ui's colors: added and deleted lines tinted, in
// the darker appearance more, as Onevra's are, the words changed in them
// over the tint, and separators and file headers on the inset and raised
// colors. The app's Styles or StylesFunc merge over ui's, part by part.
type View diff.DiffView

// Render implements core.Node.
func (v View) Render(cx *core.Scope) core.Node {
	out := diff.DiffView(v)
	out.StylesFunc = ui.Themed(cx, func() diff.Styles {
		s := Styles(cx)
		if v.Unified {
			s.Side = core.Style{}
		}
		app := v.Styles
		if v.StylesFunc != nil {
			app = v.StylesFunc()
		}
		return merge(s, app)
	})
	return out
}

// Styles are View's styles where cx is, for a diff.DiffView the app styles
// further. It reads the palette, so read it where ui.Themed does.
func Styles(cx *core.Scope) diff.Styles {
	c := ui.ColorsOf(cx).Now()
	line, word := float32(0.12), float32(0.15)
	if cx.Dark() {
		line, word = 0.2, 0.2
	}
	return diff.Styles{
		Side:             core.Style{Borders: core.Edges{Right: 1}, BorderColor: c.Line},
		LineNumber:       core.Style{Color: c.Faint, FontSize: 12},
		Content:          core.Style{Color: c.Text},
		LineAdded:        core.Style{Background: ui.Alpha(c.Up, line)},
		LineDeleted:      core.Style{Background: ui.Alpha(c.Down, line)},
		Filler:           core.Style{Background: ui.Alpha(c.Inset, 0.5)},
		WordAdded:        core.Style{Background: ui.Alpha(c.Up, word)},
		WordDeleted:      core.Style{Background: ui.Alpha(c.Down, word)},
		IndicatorAdded:   core.Style{Background: c.Up, Color: c.Up},
		IndicatorDeleted: core.Style{Background: c.Down, Color: c.Down},
		Separator:        core.Style{Background: c.Inset, Color: c.Muted, Padding: core.Symmetric(0, 12)},
		FileHeader: core.Style{Background: c.Raised, Color: c.Text, Padding: core.Symmetric(0, 12), Borders: core.Edges{Bottom: 1},
			BorderColor: c.Border},
		LineSelected:  core.Style{Background: ui.Alpha(c.Accent, 0.18)},
		Annotation:    core.Style{Padding: core.Symmetric(6, 12)},
		AddAnnotation: core.Style{Padding: core.Edges{Right: 2}},
	}
}

// merge is s with o over it, part by part.
func merge(s, o diff.Styles) diff.Styles {
	s.Side = s.Side.Merge(o.Side)
	s.Gutter = s.Gutter.Merge(o.Gutter)
	s.LineNumber = s.LineNumber.Merge(o.LineNumber)
	s.LineNumberAdded = s.LineNumberAdded.Merge(o.LineNumberAdded)
	s.LineNumberDeleted = s.LineNumberDeleted.Merge(o.LineNumberDeleted)
	s.Content = s.Content.Merge(o.Content)
	s.Line = s.Line.Merge(o.Line)
	s.LineAdded = s.LineAdded.Merge(o.LineAdded)
	s.LineDeleted = s.LineDeleted.Merge(o.LineDeleted)
	s.Filler = s.Filler.Merge(o.Filler)
	s.WordAdded = s.WordAdded.Merge(o.WordAdded)
	s.WordDeleted = s.WordDeleted.Merge(o.WordDeleted)
	s.IndicatorAdded = s.IndicatorAdded.Merge(o.IndicatorAdded)
	s.IndicatorDeleted = s.IndicatorDeleted.Merge(o.IndicatorDeleted)
	s.Separator = s.Separator.Merge(o.Separator)
	s.HunkActions = s.HunkActions.Merge(o.HunkActions)
	s.FileHeader = s.FileHeader.Merge(o.FileHeader)
	s.LineSelected = s.LineSelected.Merge(o.LineSelected)
	s.Annotation = s.Annotation.Merge(o.Annotation)
	s.AddAnnotation = s.AddAnnotation.Merge(o.AddAnnotation)
	return s
}
