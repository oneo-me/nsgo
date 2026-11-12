// Package diff is base/diff's DiffView in ui's colors. It is a package of
// its own, as base/diff is, so that only apps that show diffs link chroma's
// lexers. Its types are base/diff's: this package adds the look alone.
package diff

import (
	"math"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/base/diff"
	"github.com/nsgo-dev/nsgo/base/locales"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// View is diff.DiffView in ui's look: 12 point text on 18
// point lines (FontSize and LineHeight replace them), added and deleted
// lines tinted across their gutters in MyGo's change colors, in the darker
// appearance more, the words changed in them over the tint, a solid bar on
// added lines and a dashed one on deleted lines, and gray rounded
// separators, buttons that show the lines they collapse. The app's Styles
// or StylesFunc merge over ui's, part by part.
type View diff.DiffView

// Render implements core.Node.
func (v View) Render(cx *core.Scope) core.Node {
	out := diff.DiffView(v)
	if out.FontSize == 0 {
		out.FontSize = ui.TypeCode.FontSize
	}
	if out.LineHeight == 0 {
		out.LineHeight = out.FontSize * 1.5
	}
	if out.SeparatorHeight == 0 {
		out.SeparatorHeight = 32
	}
	if out.Ref == nil {
		out.Ref = &diff.Ref{}
	}
	if out.RenderSeparator == nil {
		out.RenderSeparator = func(cx *core.Scope, s diff.Separator) core.Node { return separator(cx, s, out.Ref) }
	}
	out.StylesFunc = ui.Themed(cx, func() diff.Styles {
		s := styles(cx, out.LineHeight)
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

// separator is a separator's content: a hunk's header, or a button across
// it that shows the lines it collapses, its icon pointing the way they come
// from and its label counting them.
func separator(cx *core.Scope, s diff.Separator, ref *diff.Ref) core.Node {
	colors := ui.ColorsOf(cx)
	place := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexGrow: 1, AlignSelf: core.AlignStretch, Padding: core.Symmetric(0, 8)}
	if s.Lines == 0 {
		return core.View{Style: place, Children: []core.Node{core.Text{Content: s.Header, MaxLines: 1,
			Style: core.Style{FontFamily: ui.Mono, FontSize: 11, FlexShrink: 1}, StyleFunc: ui.Colored(colors.Muted)}}}
	}
	icon := lucide.ChevronsUpDown
	switch {
	case s.First:
		icon = lucide.ChevronUp
	case s.Last:
		icon = lucide.ChevronDown
	}
	place.CornerRadius = 6
	return base.Button{Label: locales.DiffExpandAll(cx), OnPress: func(core.PressEvent) { ref.Expand(s.Gap, diff.ExpandEvery, 0) },
		Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff}, Style: place,
		StyleFunc: func() core.Style {
			return core.Style{Hover: &core.Style{Background: ui.Alpha(colors.Text(), 0.08)}, Active: &core.Style{Background: ui.Alpha(colors.Text(), 0.14)},
				FocusVisible: ui.FocusRing(cx, -2)}
		},
		Children: []core.Node{ui.IconOf(icon, 14, colors.Muted),
			core.Text{Content: locales.DiffUnchangedLines(cx, s.Lines), MaxLines: 1, Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)}}}
}

// RegionActions are the actions on a diff.Region, for RenderRegionActions:
// a small button staging its lines and another discarding them, which
// OnDiscard confirms. Either is left out when nil.
type RegionActions struct {
	OnStage, OnDiscard func()
}

// Render implements core.Node.
func (a RegionActions) Render(cx *core.Scope) core.Node {
	var buttons []core.Node
	if a.OnStage != nil {
		buttons = append(buttons, regionAction(cx, locales.DiffStage(cx), a.OnStage))
	}
	if a.OnDiscard != nil {
		buttons = append(buttons, regionAction(cx, locales.DiffDiscardChanges(cx), a.OnDiscard))
	}
	return core.View{Style: core.Style{Direction: core.Row, Gap: 4}, Children: buttons}
}

// regionAction is a button a line tall, solid over the lines under it.
func regionAction(cx *core.Scope, label string, press func()) core.Node {
	colors := ui.ColorsOf(cx)
	return base.Button{OnPress: func(core.PressEvent) { press() }, Label: label,
		Style: core.Style{Height: core.Pt(20), Padding: core.Symmetric(0, 8), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: 5, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Raised(), BorderColor: colors.Border(), Hover: &core.Style{Background: colors.Hover()},
				Active: &core.Style{Background: colors.Pressed()}, FocusVisible: ui.FocusRing(cx, -2)}
		},
		Children: []core.Node{core.Text{Content: label, MaxLines: 1, Style: core.Style{FontSize: 11}, StyleFunc: ui.Colored(colors.Text)}}}
}

// Styles are View's styles where cx is, for a diff.DiffView the app styles
// further. It reads the palette, so read it where ui.Themed does.
func Styles(cx *core.Scope) diff.Styles {
	return styles(cx, ui.TypeCode.LineHeight)
}

// styles are the styles of lines lh tall.
func styles(cx *core.Scope, lh core.Points) diff.Styles {
	c := ui.ColorsOf(cx).Now()
	line, word := float32(0.12), float32(0.15)
	up, down, modified := core.RGB(0x0dbe4e), core.RGB(0xff2e3f), core.RGB(0x009fff)
	if cx.Dark() {
		line, word = 0.2, 0.2
		up, down, modified = core.RGB(0x5ecc71), core.RGB(0xff6762), core.RGB(0x69b1ff)
	}
	return diff.Styles{
		Side:              core.Style{Borders: core.Edges{Right: 1}, BorderColor: c.Line},
		LineNumber:        core.Style{Color: c.Faint},
		LineNumberAdded:   core.Style{Color: up},
		LineNumberDeleted: core.Style{Color: down},
		Content:           core.Style{Color: c.Text},
		LineAdded:         core.Style{Background: ui.Alpha(up, line)},
		LineDeleted:       core.Style{Background: ui.Alpha(down, line)},
		Filler:            core.Style{Color: ui.Alpha(c.Text, 0.07)},
		WordAdded:         core.Style{Background: ui.Alpha(up, word), CornerRadius: 3},
		WordDeleted:       core.Style{Background: ui.Alpha(down, word), CornerRadius: 3},
		IndicatorAdded:    core.Style{Background: up, Color: up},
		IndicatorDeleted:  core.Style{Gradients: []core.Gradient{dashes(down, lh)}, Color: down},
		Separator:         core.Style{Background: ui.Alpha(c.Text, 0.06), Color: c.Muted, CornerRadius: 6, Margin: core.All(4)},
		FileHeader: core.Style{Background: c.Raised, Color: c.Text, Padding: core.Symmetric(0, 12), Borders: core.Edges{Bottom: 1},
			BorderColor: c.Border},
		LineSelected:  core.Style{Background: ui.Alpha(c.Accent, 0.18)},
		Annotation:    core.Style{Padding: core.Symmetric(6, 12)},
		AddAnnotation: core.Style{Padding: core.Edges{Right: 2}},
		Region:        core.Style{Borders: core.Edges{Left: 3, Top: 1, Right: 1, Bottom: 1}, BorderColor: modified},
	}
}

// dashes are a deleted line's bar: dashes about 2 points long every 4,
// fitting lh a whole number of times, so that they run on across rows.
func dashes(c core.Color, lh core.Points) core.Gradient {
	period := lh / core.Points(max(math.Round(float64(lh/4)), 1))
	return core.Gradient{Kind: core.GradientLinear, Angle: 180, Repeating: true, Stops: []core.ColorStop{
		{Color: c, At: core.Pt(0)}, {Color: c, At: core.Pt(period / 2)},
		{Color: 0, At: core.Pt(period / 2)}, {Color: 0, At: core.Pt(period)},
	}}
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
	s.Region = s.Region.Merge(o.Region)
	return s
}
