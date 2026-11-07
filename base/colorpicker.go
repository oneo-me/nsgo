package base

import (
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// colorReportLimit is how many reported colors a picker waits to see back
// from the app.
const colorReportLimit = 32

// colorReport is a color reported, and what its text reads as.
type colorReport struct{ color, read core.Color }

// colorState is what a ColorPicker's parts reach through context.
type colorState struct {
	cx       *core.Scope
	model    *core.Signal[colorModel]
	disabled func() bool
	format   ColorFormat
	report   func(c core.Color, formatted string, end bool)
	reported []colorReport
}

var colorKey = core.NewKey[*colorState]("base.colorPicker", nil)

// colorStateOf is the picker around cx, or one of the part's own outside
// any: its color goes nowhere.
func colorStateOf(cx *core.Scope) *colorState {
	if s := colorKey.Use(cx); s != nil {
		return s
	}
	return &colorState{cx: cx, model: core.NewSignal(cx, colorModelOf(0))}
}

func (s *colorState) isDisabled() bool { return is(s.disabled) }

func (s *colorState) now() colorModel { return s.model.GetUntracked() }

// set makes next the color, reporting it.
func (s *colorState) set(next colorModel) {
	if next != s.now() {
		s.model.Set(next)
		s.emit(false)
	}
}

// end reports a change's end.
func (s *colorState) end() { s.emit(true) }

func (s *colorState) emit(end bool) {
	if s.report == nil {
		return
	}
	m := s.now()
	formatted := m.format(s.format)
	// The app sets back either: the color, or its text read again, which
	// the text's rounding may move off the color.
	read, _ := core.ParseColor(formatted)
	if len(s.reported) == colorReportLimit {
		s.reported = s.reported[1:]
	}
	s.reported = append(s.reported, colorReport{m.color(), read})
	s.report(m.color(), formatted, end)
}

// ColorPicker holds a color for its parts, ColorArea, ColorSlider,
// ColorField, ColorSwatch and ColorEyeDropper, which the app lays out
// inside it. Value makes the color controlled, with its own from
// DefaultValue otherwise, black when 0. It keeps a color in every space,
// so a gray keeps its hue. OnChange reports each change and OnChangeEnd
// its end, with the color written as Format says.
type ColorPicker struct {
	Value        func() core.Color
	DefaultValue core.Color
	Format       ColorFormat
	Disabled     func() bool
	OnChange     func(value core.Color, formatted string)
	OnChangeEnd  func(value core.Color, formatted string)
	Children     []core.Node
	Class        string
	Style        core.Style
	TestID       string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (p ColorPicker) Render(cx *core.Scope) core.Node {
	first := p.DefaultValue
	if p.Value != nil {
		first = core.Untracked(cx, p.Value)
	}
	s := &colorState{cx: cx, model: core.NewSignal(cx, colorModelOf(first)), disabled: p.Disabled, format: p.Format,
		report: func(c core.Color, formatted string, end bool) {
			f := p.OnChange
			if end {
				f = p.OnChangeEnd
			}
			if f != nil {
				f(c, formatted)
			}
		}}
	if p.Value != nil {
		cx.Effect(func() {
			value := p.Value()
			cx.Untrack(func() {
				// A color reported: the app following the user, maybe behind.
				at := slices.IndexFunc(s.reported, func(r colorReport) bool { return value == r.color || value == r.read })
				if at >= 0 {
					s.reported = s.reported[at+1:]
					return
				}
				s.reported = nil
				if now := s.now(); value != now.color() {
					s.model.Set(now.follow(value))
				}
			})
		})
	}
	colorKey.Provide(cx, s)
	return core.View{Class: p.Class, Style: p.Style, Disabled: p.Disabled, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip, Children: p.Children}
}

// colorNudge is what a key does to a color's control.
type colorNudge struct {
	x, y     float64
	min, max bool
}

// colorKeyBinding is a key of a color's control: sided keys swap in a
// right-to-left layout.
type colorKeyBinding struct {
	key    string
	repeat bool
	nudge  colorNudge
	sided  bool
}

var sliderKeys = []colorKeyBinding{
	{"right", true, colorNudge{x: 1}, true}, {"up", true, colorNudge{x: 1}, false},
	{"left", true, colorNudge{x: -1}, true}, {"down", true, colorNudge{x: -1}, false},
	{"shift+right", true, colorNudge{x: 10}, true}, {"shift+up", true, colorNudge{x: 10}, false},
	{"shift+left", true, colorNudge{x: -10}, true}, {"shift+down", true, colorNudge{x: -10}, false},
	{"pageup", true, colorNudge{x: 10}, false}, {"pagedown", true, colorNudge{x: -10}, false},
	{"home", false, colorNudge{min: true}, false}, {"end", false, colorNudge{max: true}, false},
}

var areaKeys = []colorKeyBinding{
	{"right", true, colorNudge{x: 1}, true}, {"left", true, colorNudge{x: -1}, true},
	{"up", true, colorNudge{y: 1}, false}, {"down", true, colorNudge{y: -1}, false},
	{"shift+right", true, colorNudge{x: 10}, true}, {"shift+left", true, colorNudge{x: -10}, true},
	{"shift+up", true, colorNudge{y: 10}, false}, {"shift+down", true, colorNudge{y: -10}, false},
	{"pageup", true, colorNudge{y: 10}, false}, {"pagedown", true, colorNudge{y: -10}, false},
}

// colorKeys binds keys while active holds; mirrored swaps the sided ones.
func colorKeys(active func() bool, mirrored func() bool, keys []colorKeyBinding, nudge func(colorNudge)) core.Node {
	return core.Show{When: active, Then: func(cx *core.Scope) core.Node {
		for _, k := range keys {
			cx.OnKeyWith(k.key, core.KeyOptions{When: core.KeyNotEditing, Repeat: k.repeat}, func() {
				n := k.nudge
				if k.sided && core.Untracked(cx, mirrored) {
					n.x = -n.x
				}
				nudge(n)
			})
		}
		return nil
	}}
}

var colorActions = []core.AccessibilityAction{{Name: "increment"}, {Name: "decrement"}}

var colorFill = core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)}

// colorThumb centers a thumb on the point its Left and Top name.
var colorThumb = core.Style{Position: core.Absolute, Transform: core.Transform{TranslateXPercent: -50, TranslateYPercent: -50}}

// checkerboard draws squares of size in color under a color with alpha,
// none while color is 0.
func checkerboard(color core.Color, size core.Points) core.Node {
	if color == 0 {
		return nil
	}
	cell := float32(max(size, 1))
	return core.Canvas{Style: colorFill.Merge(core.Style{PointerEvents: core.PointerEventsNone}), Draw: func(d *core.Drawing, area core.Size) {
		d.SetFillColor(color)
		columns, rows := int(ceilDiv(float32(area.Width), cell)), int(ceilDiv(float32(area.Height), cell))
		for row := range rows {
			for column := row % 2; column < columns; column += 2 {
				d.FillRect(float32(column)*cell, float32(row)*cell, cell, cell)
			}
		}
	}}
}

func ceilDiv(a, b float32) float32 {
	n := int(a / b)
	if float32(n)*b < a {
		n++
	}
	return float32(n)
}
