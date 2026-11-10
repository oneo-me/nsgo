// Command orbit shows six live native views standing on one ring in
// perspective, over a calibrated dial a WGSL shader draws on the floor.
// Each view is an instrument made of NSGo's own parts: a shader, a text
// field laid out as it is typed, a spring the pointer flings, switches, an
// effect pressed through and a staggered loop.
//
// The ring is native 3D: each card turns around the ring's axis behind it
// (TransformOriginZ) in its parent's perspective. One angle drives every
// motion, on the platform's frame clock: the cards' turn, fade, lift,
// shadow and inner parallax, the dial's scale, the odometer in the header
// and the index's underline. A drag on the floor moves that angle on the
// main thread (PanValues) and lets go into a spring, so neither costs a
// commit per frame; a touchpad's swipe turns it as a drag does. Only the card in front takes the pointer, as it alone
// faces the viewer without a turn.
package main

import (
	"fmt"
	"time"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
)

func main() {
	nsgo.Run(Root, nsgo.WindowSize(1180, 820), nsgo.Appearance(core.AppearanceLight),
		nsgo.WindowConstraints(core.WindowConstraints{MinWidth: 980, MinHeight: 760}))
}

// The palette is print's: warm paper, ink in three strengths and one
// signal color, kept for what is live or chosen.
var (
	paper  = core.RGB(0xece8df)
	sheet  = core.RGB(0xf8f6f1)
	recess = core.RGB(0xe7e3d9)
	ink    = core.RGB(0x1a1915)
	ink2   = core.RGB(0x5e5a51)
	ink3   = core.RGB(0x9a958a)
	rule   = core.RGBA(0x1a1915, 0.12)
	signal = core.RGB(0xff4f00)
)

// serif is the display face: Iowan Old Style ships with macOS and iOS,
// Georgia with Windows.
const serif = "Iowan Old Style, Georgia, serif"

// The curves of what moves: quintOut arrives fast and settles long;
// lift and drop are a beat's rise and fall.
var (
	quintOut = core.Curve{X1: 0.22, Y1: 1, X2: 0.36, Y2: 1}
	lift     = core.Curve{X1: 0.3, Y1: 0, X2: 0.2, Y2: 1}
	drop     = core.Curve{X1: 0.5, Y1: 0, X2: 0.7, Y2: 1}
	outQuint = core.BezierEasing(quintOut)
)

// mono is a caption in the fixed-pitch face, tracked out as print sets
// small capitals.
func mono(content string, color core.Color) core.Node {
	return core.Text{Content: content, Style: core.Style{FontFamily: ui.Mono, FontSize: 10.5, LetterSpacing: 1.2, Color: color}}
}

// Root is the app: the dial, the ring and what steers it.
func Root(cx *core.Scope) core.Node {
	theme := core.DefaultTheme()
	theme.Light.Accent, theme.Light.Colors["accentFill"] = ink, ink
	cx.SetTheme(theme)

	r := newRing(cx)
	cx.OnKey("left", r.prev)
	cx.OnKey("right", r.next)

	cards := make([]core.Node, len(instruments))
	for i, in := range instruments {
		cards[i] = r.slot(i, func(depth core.AnimatedNode) core.Node { return card(cx, i, in, depth) })
	}
	full := core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)}
	center := full
	center.AlignItems, center.JustifyContent = core.AlignCenter, core.JustifyCenter

	// A touchpad's swipe turns the ring anywhere over the window, the
	// cards included.
	sw := &swipe{r: r}
	return core.View{TestID: "orbit", OnWheel: sw.wheel,
		Style: core.Style{FlexGrow: 1, Background: paper, Overflow: core.Hidden}, Children: []core.Node{
		dial(cx, r, full),
		r.space(full),
		// The stage centers the ring; only its cards take the pointer, so
		// a drag anywhere else reaches the floor below.
		core.View{PointerEvents: core.PointerEventsBoxNone, Style: center, Children: []core.Node{
			core.View{PointerEvents: core.PointerEventsBoxNone, Style: core.Style{Width: core.Pt(cardWidth), Height: core.Pt(cardHeight),
				Perspective: perspective, Margin: core.Edges{Bottom: stageLift * 2}}, Children: cards},
		}},
		header(cx, r),
		index(cx, r),
	}}
}

// arrive is how a piece of the page comes in at launch: up from below and
// out of nothing, after delay.
func arrive(cx *core.Scope, delay time.Duration) *core.AnimatedStyle {
	if cx.Env().ReduceMotion() {
		return nil
	}
	y, o := core.NewAnimatedValue(cx, 14), core.NewAnimatedValue(cx, 0)
	c := core.TimingConfig{Duration: 900 * time.Millisecond, Easing: outQuint, Delay: delay}
	cx.StartAnimation(core.Parallel(core.Timing(y, 0, c), core.Timing(o, 1, c)), nil)
	return &core.AnimatedStyle{TranslateY: y, Opacity: o}
}

// header names the app and rolls the front card's number as an odometer
// does, with the ring's angle.
func header(cx *core.Scope, r *ring) core.Node {
	const line = 60 // a numeral's line, which the odometer moves by per card
	digits := make([]core.Node, len(instruments)+1)
	for i := range digits {
		digits[i] = core.Text{Content: fmt.Sprintf("%02d", i%len(instruments)+1),
			Style: core.Style{Height: core.Pt(line), FontFamily: serif, FontSize: 56, LineHeight: line, Color: ink}}
	}
	return core.View{PointerEvents: core.PointerEventsBoxNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(34), Left: core.Pt(48),
		Right: core.Pt(48), Direction: core.Row, AlignItems: core.AlignEnd, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
		core.View{Style: core.Style{Gap: 8, Animated: arrive(cx, 0)}, Children: []core.Node{
			core.Text{Content: "Orbit", Role: core.RoleHeader, Style: core.Style{FontFamily: serif, FontSize: 34, Color: ink}},
			mono("A NATIVE 3D STUDY  ·  NSGO", ink3),
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 10, Animated: arrive(cx, 120*time.Millisecond)}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(line), Overflow: core.Hidden}, Children: []core.Node{
				core.View{Style: core.Style{Animated: &core.AnimatedStyle{TranslateY: core.Mul(r.phase, core.Constant(-line/step))}}, Children: digits},
			}},
			core.View{Style: core.Style{Padding: core.Edges{Bottom: 12}}, Children: []core.Node{mono(fmt.Sprintf("/ %02d", len(instruments)), ink3)}},
			// What the odometer shows, for tests and assistive technology.
			core.Text{TestID: "index", ContentFunc: func() string { return fmt.Sprintf("%02d", r.front.Get()+1) },
				Style: core.Style{Position: core.Absolute, Opacity: core.Alpha(0)}},
		}},
	}}
}

// tabWidth is an index entry's width, which its underline moves by per
// card.
const tabWidth = 116

// index jumps to a card by its name, or turns the ring a card at a time.
// Its underline follows the ring's angle, wrapping from the last entry to
// the first as the ring does.
func index(cx *core.Scope, r *ring) core.Node {
	tabs := make([]core.Node, 0, len(instruments)+2)
	for i, in := range instruments {
		tabs = append(tabs, base.Button{Label: in.title, TestID: fmt.Sprintf("chip-%d", i),
			Selected: func() bool { return r.front.Get() == i },
			OnPress:  func(core.PressEvent) { r.turnTo(i) },
			Style:    core.Style{Width: core.Pt(tabWidth), Padding: core.Edges{Top: 14, Bottom: 6}, Gap: 5},
			Children: []core.Node{
				mono(fmt.Sprintf("%02d", i+1), ink3),
				core.Text{Content: in.title, StyleFunc: func() core.Style {
					s := core.Style{FontSize: 13, Color: ink3, Transition: core.Transition{Duration: 400 * time.Millisecond}}
					if r.front.Get() == i {
						s.Color = ink
					}
					return s
				}},
			}})
	}
	// Two underlines a whole index apart: as one runs off the end, the
	// other comes in at the start.
	along := core.Mul(r.phase, core.Constant(tabWidth/step))
	underline := func(x core.AnimatedNode) core.Node {
		return core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(-1), Left: core.Pt(0), Width: core.Pt(tabWidth - 20),
			Height: core.Pt(2), Background: signal, Animated: &core.AnimatedStyle{TranslateX: x}}}
	}
	strip := core.View{Style: core.Style{Direction: core.Row, Overflow: core.Hidden, Borders: core.Edges{Top: 1}, BorderColor: rule},
		Children: append(tabs, underline(along), underline(core.Sub(along, core.Constant(tabWidth*float64(len(instruments))))))}

	turn := func(icon func() *core.SVG, label, id string, fn func()) core.Node {
		return ui.IconButton{Icon: icon, Label: label, TestID: id, OnPress: fn, Tint: func() core.Color { return ink }}
	}
	return core.View{PointerEvents: core.PointerEventsBoxNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0),
		Bottom: core.Pt(26), AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 20, Animated: arrive(cx, 240*time.Millisecond)}, Children: []core.Node{
			turn(lucide.ArrowLeft, "Previous", "prev", r.prev), strip, turn(lucide.ArrowRight, "Next", "next", r.next),
		}},
		core.View{Style: core.Style{Animated: arrive(cx, 320*time.Millisecond)}, Children: []core.Node{
			mono("DRAG THE FLOOR  ·  ← →  ·  OR CHOOSE", ink3),
		}},
	}}
}
