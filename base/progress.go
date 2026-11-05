package base

import (
	"math"

	"github.com/oneo-me/nsgo/core"
)

// Progress is a linear progress bar: how far a task of a known extent is,
// or that one of an unknown extent is under way. Its parts are a track
// and an indicator inside it, as wide as Value's share of the track while
// determinate, then its children, which assistive technology reads.
type Progress struct {
	// Value is a percentage from 0 to 100, clamped; NaN is none yet.
	Value func() float32
	// Indeterminate leaves the indicator's width and motion to the app,
	// and reports no value.
	Indeterminate      func() bool
	Class              string
	Style              core.Style
	StyleFunc          func() core.Style
	TrackClass         string
	TrackStyle         core.Style
	TrackStyleFunc     func() core.Style
	IndicatorClass     string
	IndicatorStyle     core.Style
	IndicatorStyleFunc func() core.Style
	Children           []core.Node
	Label              string
	TestID             string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// progressMax is the range of Progress.Value, gpui-base's percentage.
const progressMax = 100

// Render implements core.Node.
func (p Progress) Render(*core.Scope) core.Node {
	value := func() float32 {
		if p.Value == nil {
			return 0
		}
		v := p.Value()
		if math.IsNaN(float64(v)) {
			return 0
		}
		return min(max(v, 0), progressMax)
	}
	// The app's width wins, as for an indeterminate bar's.
	indicator := core.View{Class: p.IndicatorClass, StyleFunc: func() core.Style {
		var s core.Style
		if !is(p.Indeterminate) {
			s.Width = core.Pct(value())
		}
		s = s.Merge(p.IndicatorStyle)
		if p.IndicatorStyleFunc != nil {
			s = s.Merge(p.IndicatorStyleFunc())
		}
		return s
	}}
	return core.View{Class: p.Class, Style: p.Style, StyleFunc: p.StyleFunc, Role: core.RoleProgressBar, Label: p.Label, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip,
		Accessible: true,
		AccessibilityValue: func() core.AccessibilityValue {
			if is(p.Indeterminate) {
				return core.AccessibilityValue{Min: 0, Max: progressMax, NoNow: true}
			}
			return core.AccessibilityValue{Min: 0, Max: progressMax, Now: value()}
		},
		Children: append([]core.Node{core.View{Class: p.TrackClass, Style: p.TrackStyle, StyleFunc: p.TrackStyleFunc,
			Children: []core.Node{indicator}}}, p.Children...)}
}
