package ui

import (
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Image is a picture in a rounded well that shows Placeholder while it
// loads and if it fails: a faint picture icon when nil. It keeps
// AspectRatio, else the one Source declares, else 3:2, unless its style
// sizes both sides. With a Label it is an image to assistive technology;
// without one, decoration.
type Image struct {
	Source core.ImageSource
	Label  string
	// ResizeMode fits the picture into the well: Cover when 0.
	ResizeMode  core.ResizeMode
	AspectRatio float32
	Placeholder core.Node
	OnLoad      func(pixels core.Size)
	OnError     func(err error)
	// TestID names the picture, which tests report loads on.
	TestID string
	// Style merges over the well's.
	Style core.Style
}

// Render implements core.Node.
func (i Image) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	ratio := i.AspectRatio
	if ratio == 0 && i.Source.Size.Width > 0 && i.Source.Size.Height > 0 {
		ratio = float32(i.Source.Size.Width / i.Source.Size.Height)
	}
	if ratio == 0 {
		ratio = 3.0 / 2
	}
	loaded, failed := core.NewSignal(cx, false), core.NewSignal(cx, false)
	fade := 200 * time.Millisecond
	if ReduceMotion(cx) {
		fade = time.Nanosecond
	}
	placeholder := func() core.Node {
		if i.Placeholder != nil {
			return i.Placeholder
		}
		return IconOf(func() *core.SVG {
			if failed.Get() {
				return lucide.ImageOff()
			}
			return lucide.Image()
		}, ButtonHeight, colors.Faint)
	}
	fill := core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)}
	children := []core.Node{
		core.Show{When: func() bool { return !loaded.Get() }, Then: func(cx *core.Scope) core.Node {
			return core.View{Style: fill.Merge(core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}),
				Children: []core.Node{placeholder()}}
		}},
	}
	if hasImage(i.Source) {
		children = append(children, core.Show{When: func() bool { return !failed.Get() }, Then: func(cx *core.Scope) core.Node {
			return core.Image{Source: i.Source, ResizeMode: i.ResizeMode, FadeDuration: fade, Label: i.Label, TestID: i.TestID, Style: fill,
				OnLoad: func(px core.Size) {
					loaded.Set(true)
					if i.OnLoad != nil {
						i.OnLoad(px)
					}
				},
				OnError: func(err error) {
					failed.Set(true)
					if i.OnError != nil {
						i.OnError(err)
					}
				}}
		}})
	}
	return core.View{Style: core.Style{AspectRatio: ratio, Overflow: core.Hidden, CornerRadius: RadiusControl}.Merge(i.Style),
		StyleFunc: Filled(colors.Inset), Children: children}
}

// hasImage reports whether src names a picture.
func hasImage(src core.ImageSource) bool { return src.URI != "" || len(src.Data) > 0 }
