package ui

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// HoldTone is the color a HoldToConfirm fills with.
type HoldTone uint8

const (
	// HoldAccent fills with the accent.
	HoldAccent HoldTone = iota
	// HoldDanger tints the face and fills with the danger color, for an
	// action that cannot be undone.
	HoldDanger
	// HoldNeutral fills with the text's color.
	HoldNeutral
)

// HoldToConfirm is base.HoldToConfirm in Onevra's look: a fill sweeps
// across it from the leading edge, and once it is full Text and Icon
// (lucide.Trash when empty) give way to ConfirmedText and a check.
//
// Confirmed is the app's: true shows the done state, and setting it back
// to false resets the button. Hint is what assistive technology reads
// after the label, such as "Hold to confirm". Prefer an AlertDialog when
// people need to read the consequences first.
type HoldToConfirm struct {
	Text          string
	ConfirmedText string
	Icon          func() *core.SVG
	Tone          HoldTone
	Duration      time.Duration
	Confirmed     func() bool
	OnConfirm     func()
	Hint          string
	Disabled      func() bool
	TestID        string
}

// Render implements core.Node.
func (h HoldToConfirm) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	icon := h.Icon
	if icon == nil {
		icon = lucide.Trash
	}
	confirmed := func() bool { return is(h.Confirmed) }
	scale := core.NewAnimatedValue(cx, 1)
	width := core.NewSignal[core.Points](cx, 0)
	still := ReduceMotion(cx)

	ink, fillColor, onFill, border := colors.Text, colors.Accent, colors.OnAccent, colors.Border
	switch h.Tone {
	case HoldDanger:
		ink, fillColor, onFill = colors.Danger, colors.Danger, colors.Background
		border = func() core.Color { return Mix(colors.Border(), colors.Danger(), 0.32) }
	case HoldNeutral:
		fillColor, onFill = colors.Text, colors.Background
	}
	text := func() string {
		if confirmed() {
			return h.ConfirmedText
		}
		return h.Text
	}
	glyph := func() *core.SVG {
		if confirmed() {
			return lucide.Check()
		}
		return icon()
	}
	face := func(c func() core.Color) core.Node {
		t := Textf(text, 13, Regular, c)
		t.MaxLines = 1
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 8,
			Height: core.Pt(ButtonHeight - 2), Padding: core.Symmetric(0, 13)}, Children: []core.Node{IconOf(glyph, 14, c), t}}
	}
	content := func(_ *core.Scope, fill *core.AnimatedValue) []core.Node {
		// A second face in the fill's colors shows through it, so the text
		// changes color exactly at its edge: the fill slides in from the
		// leading edge while its face slides the other way, staying put.
		fullSize := core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0)}
		slide := func(sign float64) func() core.Style {
			return func() core.Style {
				w := float64(width.Get())
				return core.Style{Animated: &core.AnimatedStyle{TranslateX: Lerp(fill, sign*-w, 0)}}
			}
		}
		inverse := core.View{PointerEvents: core.PointerEventsNone,
			Style: core.Style{Position: core.Absolute, Left: core.Pt(-1), Right: core.Pt(-1), Top: core.Pt(-1), Bottom: core.Pt(-1), Overflow: core.Hidden, CornerRadius: RadiusControl},
			// Until it is measured the fill could not slide out of sight.
			StyleFunc: func() core.Style {
				if width.Get() == 0 {
					return core.Style{Opacity: core.Alpha(0)}
				}
				return core.Style{}
			},
			OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) },
			// The fill clips its face, which stays where the button's is.
			Children: []core.Node{core.View{Style: fullSize.Merge(core.Style{Overflow: core.Hidden}), StyleFunc: func() core.Style {
				return core.Style{Background: fillColor()}.Merge(slide(1)())
			}, Children: []core.Node{core.View{Style: fullSize.Merge(core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}),
				StyleFunc: slide(-1), Children: []core.Node{face(onFill)}}}}}}

		return []core.Node{face(ink), inverse}
	}
	return base.HoldToConfirm{Duration: h.Duration, Confirmed: h.Confirmed, OnConfirm: h.OnConfirm, ConfirmedLabel: h.ConfirmedText, Content: content,
		OnHoldChange: func(holding bool) {
			if !holding {
				Spring(cx, scale, 1, pressSpring)
			} else if !still {
				Spring(cx, scale, 0.97, pressSpring)
			}
		},
		LabelFunc: text, Hint: h.Hint, TestID: h.TestID, Disabled: h.Disabled,
		Style: core.Style{Group: "hold", AlignSelf: core.AlignStart, CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, Transition: quick, Disabled: Dimmed(),
			Animated: &core.AnimatedStyle{Scale: scale}},
		StyleFunc: func() core.Style {
			s := core.Style{Background: colors.Surface(), BorderColor: border(), Focus: FocusRing(cx, 2)}
			if !confirmed() {
				s.Hover = &core.Style{Background: Mix(colors.Surface(), fillColor(), 0.06)}
			}
			return s
		}}
}
