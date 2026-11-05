package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/core"
)

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
	if !ReduceMotion(cx) {
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
