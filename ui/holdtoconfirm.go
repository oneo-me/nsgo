package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// HoldVariant is a HoldToConfirm's look: the color it fills with.
type HoldVariant uint8

const (
	// HoldAccent fills with the accent.
	HoldAccent HoldVariant = iota
	// HoldDanger tints the face and fills with the danger color, for an
	// action that cannot be undone.
	HoldDanger
	// HoldNeutral fills with the text's color.
	HoldNeutral
)

// holdRewind is how long an empty-to-full fill takes to drain.
const holdRewind = 400 * time.Millisecond

// holdLinear moves a hold's fill at one speed both ways.
var holdLinear = core.BezierEasing(core.Linear)

// HoldToConfirm is a button for what a stray press must not trigger: held
// down for Duration, 1.2 s when 0, a fill sweeps across it from the leading
// edge, and once it is full OnConfirm runs, assistive technology hears
// ConfirmedText, and Text and Icon (lucide.Trash when empty) give way to
// ConfirmedText and a check. Letting go early rewinds the fill. The
// keyboard and assistive technology, which cannot hold, start it with a
// press and cancel it with another or by moving the focus away.
//
// Confirmed is the app's: true shows the done state, and setting it back
// to false resets the button. Hint is what assistive technology reads
// after the label, such as "Hold to confirm". Prefer an AlertDialog when
// people need to read the consequences first.
type HoldToConfirm struct {
	Text          string
	ConfirmedText string
	Icon          func() *core.SVG
	Variant       HoldVariant
	Duration      time.Duration
	Confirmed     func() bool
	OnConfirm     func()
	Hint          string
	Disabled      func() bool
	TestID        string
}

// Render implements core.Node.
func (h HoldToConfirm) Render(cx *core.Scope) core.Node {
	confirmed := func() bool { return is(h.Confirmed) }
	colors := ColorsOf(cx)
	icon := h.Icon
	if icon == nil {
		icon = lucide.Trash
	}
	scale := core.NewAnimatedValue(cx, 1)
	width := core.NewSignal[core.Points](cx, 0)
	still := ReduceMotion(cx)

	ink, fillColor, onFill, border := colors.Text, colors.AccentFill, colors.OnAccent, colors.Border
	switch h.Variant {
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
	content := func(fill *core.AnimatedValue) []core.Node {
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
	duration := h.Duration
	if duration <= 0 {
		duration = 1200 * time.Millisecond
	}
	holding := core.NewSignal(cx, false)
	setHolding := func(on bool) {
		holding.Set(on)
		if !on {
			Spring(cx, scale, 1, pressSpring)
		} else if !still {
			Spring(cx, scale, 0.97, pressSpring)
		}
	}
	// The fill moves linearly both ways, so where it is follows from when
	// its last run started, where from and toward what.
	var runStart time.Time
	var runFor time.Duration
	runFrom, runTo := 0.0, 0.0
	if core.Untracked(cx, confirmed) {
		runFrom, runTo = 1, 1
	}
	fill := core.NewAnimatedValue(cx, runTo)
	at := func() float64 {
		if runFor <= 0 {
			return runTo
		}
		p := min(float64(cx.Now().Sub(runStart))/float64(runFor), 1)
		return runFrom + (runTo-runFrom)*p
	}
	run := func(to float64, d time.Duration, done func(bool)) {
		runFrom, runTo, runStart, runFor = at(), to, cx.Now(), d
		if d <= 0 {
			fill.Set(to)
			return
		}
		cx.StartAnimation(core.Timing(fill, to, core.TimingConfig{Duration: d, Easing: holdLinear}), done)
	}
	begin := func() {
		if core.Untracked(cx, confirmed) || core.Untracked(cx, func() bool { return is(h.Disabled) }) || holding.GetUntracked() {
			return
		}
		setHolding(true)
		run(1, time.Duration((1-at())*float64(duration)), func(finished bool) {
			if !finished || !holding.GetUntracked() {
				return
			}
			setHolding(false)
			if h.ConfirmedText != "" {
				cx.Announce(h.ConfirmedText)
			}
			if h.OnConfirm != nil {
				h.OnConfirm()
			}
		})
	}
	release := func() {
		if !holding.GetUntracked() {
			return
		}
		setHolding(false)
		run(0, time.Duration(at()*float64(holdRewind)), nil)
	}
	cx.Effect(func() {
		on := confirmed()
		if core.Untracked(cx, holding.Get) {
			return
		}
		if on && at() < 1 {
			run(1, 0, nil)
		} else if !on && at() > 0 {
			run(0, holdRewind, nil)
		}
	})
	// upAt is when a pointer's press last ended.
	var upAt time.Time
	return core.View{Role: core.RoleButton, LabelFunc: text, Hint: h.Hint, TestID: h.TestID, Disabled: h.Disabled,
		Style: core.Style{Group: "hold", AlignSelf: core.AlignStart, CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, Transition: quick, Disabled: Dimmed(),
			Animated: &core.AnimatedStyle{Scale: scale}},
		StyleFunc: func() core.Style {
			s := core.Style{Background: colors.Surface(), BorderColor: border(), Focus: FocusRing(cx, 2)}
			if !confirmed() {
				s.Hover = &core.Style{Background: Mix(colors.Surface(), fillColor(), 0.06)}
			}
			return s
		},
		OnPressDown: func(core.PressEvent) { begin() },
		OnPressUp: func(core.PressEvent) {
			upAt = cx.Now()
			release()
		},
		OnPress: func(core.PressEvent) {
			// A pointer's press may end in a press too, right after it lets
			// go; only the keyboard's and assistive technology's start or
			// cancel a hold here.
			if !upAt.IsZero() && cx.Now().Sub(upAt) < 250*time.Millisecond {
				upAt = time.Time{}
				return
			}
			if holding.GetUntracked() {
				release()
			} else {
				begin()
			}
		},
		OnBlur: release, Children: content(fill)}
}
