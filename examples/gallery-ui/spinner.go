package ui

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// ringSize is a ProgressRing's and a Spinner's diameter by default.
const ringSize core.Points = 32

// ProgressRing is Progress as a ring: the accent runs clockwise from
// twelve o'clock over a track, or, Indeterminate, a quarter of it turns
// once every 1.4 s, still under reduced motion. Value is from 0 to 100, as Progress's.
// Size is its diameter, 32 pt when 0; the line is a tenth of it, 3 pt at
// most.
type ProgressRing struct {
	Value         func() float32
	Indeterminate func() bool
	Size          core.Points
	// Label names the task for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (p ProgressRing) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	size := p.Size
	if size <= 0 {
		size = ringSize
	}
	value := func() float32 {
		if p.Value == nil {
			return 0
		}
		v := p.Value()
		if v != v {
			return 0
		}
		return min(max(v, 0), 100)
	}
	turn := core.NewAnimatedValue(cx, 0)
	if p.Indeterminate != nil && !reduceMotion(cx) {
		cx.StartAnimation(core.ValueKeyframes(turn, core.KeyframeTiming{Duration: 1400 * time.Millisecond, Iterations: core.Forever},
			core.Stops(0, 360)...), nil)
	}
	draw := func(d *core.Drawing, s core.Size) {
		c := colors.Now()
		dia := float32(min(s.Width, s.Height))
		if dia <= 0 {
			return
		}
		width := min(dia/10, 3)
		r := (dia - width) / 2
		x, y := float32(s.Width)/2, float32(s.Height)/2
		d.SetLineWidth(width).SetLineCap(core.CapRound)
		d.BeginPath().Arc(x, y, r, 0, 2*math.Pi, false).SetStrokeColor(c.Line).Stroke()
		start, sweep := float32(-math.Pi/2), value()/100*2*math.Pi
		if is(p.Indeterminate) {
			sweep = math.Pi / 2
		}
		if sweep <= 0 {
			return
		}
		d.BeginPath().Arc(x, y, r, start, start+sweep, false).SetStrokeColor(c.Accent).Stroke()
	}
	return core.View{TestID: p.TestID, Accessible: true, Role: core.RoleProgressBar, Label: p.Label,
		AccessibilityValue: func() core.AccessibilityValue {
			if is(p.Indeterminate) {
				return core.AccessibilityValue{Min: 0, Max: 100, NoNow: true}
			}
			return core.AccessibilityValue{Min: 0, Max: 100, Now: value(), Text: fmt.Sprintf("%d%%", int(value()+0.5))}
		},
		Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), FlexShrink: 0}.Merge(p.Style),
		StyleFunc: func() core.Style {
			if !is(p.Indeterminate) {
				return core.Style{}
			}
			return core.Style{Animated: &core.AnimatedStyle{Rotate: turn}}
		},
		Children: []core.Node{core.Canvas{Draw: draw, Style: core.Style{FlexGrow: 1}}}}
}

// SpinnerVariant is a Spinner's look.
type SpinnerVariant uint8

const (
	// SpinnerRing is an indeterminate ProgressRing. It is the default.
	SpinnerRing SpinnerVariant = iota
	// SpinnerSpokes is twelve fading spokes that step around in the
	// muted text's color.
	SpinnerSpokes
)

// Spinner is work of unknown length. Label names it for assistive
// technology; Size is its diameter, 32 pt when 0. Under reduced motion
// it stands still.
type Spinner struct {
	Variant SpinnerVariant
	Size    core.Points
	Label   string
	TestID  string
	Style   core.Style
}

// Render implements core.Node.
func (s Spinner) Render(cx *core.Scope) core.Node {
	if s.Variant != SpinnerSpokes {
		return ProgressRing{Indeterminate: func() bool { return true }, Size: s.Size, Label: s.Label, TestID: s.TestID, Style: s.Style}
	}
	size := s.Size
	if size <= 0 {
		size = ringSize
	}
	turn := core.NewAnimatedValue(cx, 0)
	if !reduceMotion(cx) {
		// A spoke's turn at a time, as the spokes fade along.
		cx.StartAnimation(core.ValueKeyframes(turn, core.KeyframeTiming{Duration: 900 * time.Millisecond, Iterations: core.Forever,
			Easing: core.StepsEasing(12, core.JumpEnd)}, core.Stops(0, 360)...), nil)
	}
	return core.View{TestID: s.TestID, Accessible: true, Role: core.RoleProgressBar, Label: s.Label,
		Style:     core.Style{Width: core.Pt(size), Height: core.Pt(size), FlexShrink: 0}.Merge(s.Style),
		StyleFunc: func() core.Style { return core.Style{Animated: &core.AnimatedStyle{Rotate: turn}} },
		Children: []core.Node{core.Icon{SVG: spokes(), Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)},
			StyleFunc: Colored(ColorsOf(cx).Muted)}}}
}

// spokes are the twelve spokes, fading behind the one at twelve o'clock,
// as MyGo's indicator draws them.
var spokes = sync.OnceValue(func() *core.SVG {
	var svg strings.Builder
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="-50 -50 100 100" fill="none" stroke="currentColor" stroke-width="9" stroke-linecap="round">`)
	for i := range 12 {
		age := (12 - i) % 12
		fmt.Fprintf(&svg, `<path d="M0 -24V-44" transform="rotate(%d)" stroke-opacity="%g"/>`, i*30, 1-float32(age)/12*0.85)
	}
	svg.WriteString(`</svg>`)
	return core.MustParseSVG([]byte(svg.String()))
})
