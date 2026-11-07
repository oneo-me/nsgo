package ui

import (
	"math"
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Slider's geometry: a 4-point track under a white capsule knob 20 × 16,
// which grows by a third while held.
const (
	sliderTrack core.Points = 4
	sliderKnobW core.Points = 20
	sliderKnobH core.Points = 16
	// sliderTicks is the most ticks a stepped slider shows.
	sliderTicks = 60
)

// Slider is base.Slider in Onevra's look: a white capsule knob on a thin
// track that stays inside its ends, or with High two that pick a range
// from Value up to High. Step above 0 snaps the values to steps from Min
// and marks them under the track, up to 60 of them; at 0 the keys move a
// hundredth of the range.
type Slider struct {
	Value    func() float64
	OnChange func(value float64)
	// High and OnHighChange make it a range, Value its low end.
	High         func() float64
	OnHighChange func(value float64)
	// Min and Max are the range, 0 to 100 when Max is not above Min.
	Min, Max float64
	Step     float64
	Disabled func() bool
	// Label names it for assistive technology; a range's knobs add
	// LowLabel and HighLabel after it, such as "minimum" and "maximum".
	Label               string
	LowLabel, HighLabel string
	// ValueText is what assistive technology reads as a value, the
	// number when nil.
	ValueText func(value float64) string
	// TestID names the slider; its knobs add "-thumb", or "-low" and
	// "-high" in a range.
	TestID string
	Style  core.Style
}

// bounds are the slider's range.
func (s Slider) bounds() (lo, hi float64) {
	if s.Max > s.Min {
		return s.Min, s.Max
	}
	return 0, 100
}

// Render implements core.Node.
func (s Slider) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	lo, hi := s.bounds()
	ranged := s.High != nil
	read := func(fn func() float64) float64 {
		if fn == nil {
			return lo
		}
		return fn()
	}
	value := func() base.SliderValue {
		if ranged {
			return base.RangeValue(read(s.Value), read(s.High))
		}
		return base.SingleValue(read(s.Value))
	}
	change := func(v base.SliderValue) {
		now := core.Untracked(cx, value)
		switch {
		case !ranged && s.OnChange != nil:
			s.OnChange(v.End)
		case ranged && v.Start != now.Start && s.OnChange != nil:
			s.OnChange(v.Start)
		case ranged && v.End != now.End && s.OnHighChange != nil:
			s.OnHighChange(v.End)
		}
	}
	const height = sliderKnobH + 4
	tall := core.Points(height)
	var marks []core.Node
	if s.Step > 0 {
		tall += 8
		if n := int(math.Floor((hi - lo) / s.Step)); hi > lo && n <= sliderTicks {
			for i := 0; i <= n; i++ {
				f := float32(float64(i) * s.Step / (hi - lo))
				marks = append(marks, core.View{PointerEvents: core.PointerEventsNone,
					Style: core.Style{Position: core.Absolute, Top: core.Pt(sliderTrack/2 + 10), Left: core.Pct(f * 100), Width: core.Pt(2), Height: core.Pt(2),
						CornerRadius: 1, Margin: core.Edges{Left: -1}},
					StyleFunc: Filled(colors.Track)})
			}
		}
	}
	label, startLabel := s.Label, ""
	thumb, startThumb := "", ""
	if ranged {
		label, startLabel = s.Label+" "+s.HighLabel, s.Label+" "+s.LowLabel
	}
	if s.TestID != "" {
		thumb = s.TestID + "-thumb"
		if ranged {
			thumb, startThumb = s.TestID+"-high", s.TestID+"-low"
		}
	}
	number := func(p *float64) func() float64 { return func() float64 { return *p } }
	valueText := s.ValueText
	if valueText == nil {
		valueText = func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	}
	return base.Slider{Value: value, OnChange: change, Min: number(&lo), Max: number(&hi), Step: number(&s.Step), Disabled: s.Disabled,
		Label: label, StartLabel: startLabel, ValueText: valueText, TestID: s.TestID, ThumbTestID: thumb, StartThumbTestID: startThumb, Marks: marks,
		Style:              core.Style{Height: core.Pt(tall), MinWidth: core.Pt(80), Disabled: Dimmed()}.Merge(s.Style),
		TrackStyle:         core.Style{Height: core.Pt(tall)},
		IndicatorStyle:     core.Style{Height: core.Pt(sliderTrack), CornerRadius: sliderTrack / 2, Margin: core.Edges{Top: (height - sliderTrack) / 2, Left: sliderKnobW / 2, Right: sliderKnobW / 2}},
		IndicatorStyleFunc: Filled(colors.Line),
		RangeStyle:         core.Style{CornerRadius: sliderTrack / 2},
		RangeStyleFunc:     Filled(colors.AccentFill),
		ThumbStyle: core.Style{Width: core.Pt(sliderKnobW), Height: core.Pt(sliderKnobH), CornerRadius: sliderKnobH / 2,
			Transition: core.Transition{Duration: quick.Duration, Easing: core.EaseOut, Properties: core.TransitionTransform}},
		ThumbStyleFunc: func() core.Style {
			return FinOf(cx).Knob().Over(core.Style{Background: colors.Knob(), BorderWidth: 1, BorderColor: colors.Border(), Focus: FocusRing(cx, 2),
				Active: &core.Style{Transform: core.Transform{Scale: 1.3, TranslateXPercent: -50, TranslateYPercent: -50}, Shadows: []core.BoxShadow{Shadow(0, 6, 14, -3, Black(0.18))}}})
		}}
}
