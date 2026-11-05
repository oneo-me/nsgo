package ui

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// scrubDistance is how far a drag on a NumberInput's handle moves to
// change the value by a step.
const scrubDistance = 4

// NumberInput is a field of a number with buttons that step it: stacked
// at its end on desktops, a minus and a plus on touch screens. The arrow
// keys step it while it has the keyboard focus, and a drag across its
// Leading handle scrubs it. What is typed is read as the field is left
// or Return pressed, and kept to the range and the step; text that is no
// number gives the value back.
type NumberInput struct {
	Value    func() float64
	OnChange func(value float64)
	// Min and Max are the range, when Max is above Min.
	Min, Max float64
	// Step is what the buttons, the keys and the scrub change the value
	// by, and what it is rounded to from Min; 1 when 0.
	Step float64
	// Format writes the value, with as many decimals as Step has when
	// nil; Parse reads it back, strconv.ParseFloat's when nil.
	Format func(value float64) string
	Parse  func(text string) (float64, bool)
	// Leading is the scrub handle's icon, or LeadingText its text, such
	// as "W"; without either the field has no handle. Its test ID is the
	// field's with "-scrub".
	Leading     func() *core.SVG
	LeadingText string
	// HideSteppers leaves the buttons out.
	HideSteppers bool
	// IncrementLabel and DecrementLabel name the buttons for assistive
	// technology.
	IncrementLabel, DecrementLabel string
	Disabled                       func() bool
	// Label names the field for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// step is the value's step.
func (n NumberInput) step() float64 {
	if n.Step > 0 {
		return n.Step
	}
	return 1
}

// fit keeps v to the range and the step.
func (n NumberInput) fit(v float64) float64 {
	step, from := n.step(), 0.0
	ranged := n.Max > n.Min
	if ranged {
		from = n.Min
	}
	v = from + math.Round((v-from)/step)*step
	if ranged {
		v = min(max(v, n.Min), n.Max)
	}
	// Steps such as 0.1 leave binary noise; the step's decimals hold
	// the value.
	scale := math.Pow(10, float64(decimals(step)))
	return math.Round(v*scale) / scale
}

// decimals is how many digits after the point step needs.
func decimals(step float64) int {
	s := strconv.FormatFloat(step, 'f', -1, 64)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

// Render implements core.Node.
func (n NumberInput) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	value := func() float64 {
		if n.Value == nil {
			return 0
		}
		return n.Value()
	}
	format := n.Format
	if format == nil {
		d := decimals(n.step())
		format = func(v float64) string { return strconv.FormatFloat(v, 'f', d, 64) }
	}
	parse := n.Parse
	if parse == nil {
		parse = func(s string) (float64, bool) {
			v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
		}
	}
	set := func(v float64) {
		if v = n.fit(v); v != value() && n.OnChange != nil {
			n.OnChange(v)
		}
	}
	by := func(steps float64) { set(value() + steps*n.step()) }
	at := func(edge float64) func() bool {
		return func() bool { return is(n.Disabled) || n.Max > n.Min && value() == edge }
	}
	// While the field has the focus it shows what is typed.
	focused, draft := core.NewSignal(cx, false), core.NewSignal(cx, "")
	commit := func() {
		if v, ok := parse(draft.Get()); ok {
			set(v)
		}
		draft.Set(format(value()))
	}
	field := base.Input{Label: n.Label, Disabled: n.Disabled, KeyboardType: core.KeyboardDecimal,
		Value: func() string {
			if focused.Get() {
				return draft.Get()
			}
			return format(value())
		},
		OnChange: draft.Set, OnSubmit: func(string) { commit() },
		OnFocus: func() { draft.Set(format(value())); focused.Set(true) },
		OnBlur:  func() { commit(); focused.Set(false) },
		Style:   typeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(32), Padding: core.Symmetric(0, 10)}),
		StyleFunc: func() core.Style {
			return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Now().Selection}
		}}
	var children []core.Node
	if n.Leading != nil || n.LeadingText != "" {
		children = append(children, n.handle(cx, value, set))
		field.Style.Padding.Left = 4
	}
	children = append(children, field, core.Show{When: focused.Get, Then: func(cx *core.Scope) core.Node {
		// The arrows move the caret in other fields; this one steps.
		opts := core.KeyOptions{When: core.KeyAlways, Repeat: true}
		cx.OnKeyWith("up", opts, func() { by(1); draft.Set(format(value())) })
		cx.OnKeyWith("down", opts, func() { by(-1); draft.Set(format(value())) })
		return nil
	}})
	if !n.HideSteppers {
		children = append(children, n.steppers(cx, by, at))
	}
	return core.View{TestID: n.TestID, Children: children,
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignStretch, Height: core.Pt(ButtonHeight), Width: core.Pt(160),
			CornerRadius: RadiusControl, BorderWidth: 1, Overflow: core.Hidden, Transition: quick},
		StyleFunc: func() core.Style {
			s := core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(),
				Hover: &core.Style{BorderColor: Mix(colors.FieldBorder(), colors.Text(), 0.25)}}
			switch {
			case is(n.Disabled):
				s = core.Style{Opacity: core.Alpha(0.5), Background: colors.Inset(), BorderColor: colors.Border()}
			case focused.Get():
				s.BorderColor, s.Hover = colors.Accent(), nil
			}
			return s.Merge(n.Style)
		}}
}

// handle is the leading handle a drag across scrubs the value with; the
// steppers and the keys do as much for assistive technology.
func (n NumberInput) handle(cx *core.Scope, value func() float64, set func(float64)) core.Node {
	colors := ColorsOf(cx)
	var from float64
	var content core.Node
	if n.Leading != nil {
		content = Icon(n.Leading, iconSize-2, colors.Muted)
	} else {
		content = core.Text{Content: n.LeadingText, MaxLines: 1, Style: typeLabel, StyleFunc: Colored(colors.Muted)}
	}
	id := ""
	if n.TestID != "" {
		id = n.TestID + "-scrub"
	}
	return core.View{TestID: id, ImportantForAccessibility: core.ImportantNoHideDescendants,
		PanActivation: core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12},
		OnPan: func(e core.PanEvent) {
			if is(n.Disabled) {
				return
			}
			switch e.Phase {
			case core.GestureBegan:
				from = value()
			case core.GestureChanged, core.GestureEnded:
				set(from + math.Round(float64(e.Translation.X)/scrubDistance)*n.step())
			}
		},
		Style: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, MinWidth: core.Pt(ButtonHeight - 6),
			Padding: core.Edges{Left: 8}, Cursor: core.CursorColResize},
		Children: []core.Node{content}}
}

// steppers are the buttons that step the value: stacked chevrons on
// desktops, a minus and a plus on touch screens.
func (n NumberInput) steppers(cx *core.Scope, by func(float64), at func(float64) func() bool) core.Node {
	colors := ColorsOf(cx)
	button := func(icon func() *core.SVG, label string, steps float64, disabled func() bool, s core.Style) core.Node {
		return base.Button{Label: label, Disabled: disabled, OnPress: func(core.PressEvent) { by(steps) },
			Style: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Cursor: core.CursorPointer,
				Transition: quick, Borders: core.Edges{Left: 1}, Disabled: &core.Style{Cursor: core.CursorDefault}}.Merge(s),
			StyleFunc: func() core.Style {
				c := colors.Now()
				return core.Style{Color: c.Muted, BorderColors: core.EdgeColors{Left: c.Line, Top: c.Line},
					Hover: &core.Style{Background: c.Hover, Color: c.Text}, Active: &core.Style{Background: c.Pressed},
					Focus: FocusRing(cx, -2), Disabled: &core.Style{Color: c.Faint, Background: transparent}}
			},
			Children: []core.Node{Icon(icon, iconSize-4, inherited)}}
	}
	if !touch(cx) {
		half := core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Width: core.Pt(22)}
		return core.View{Children: []core.Node{
			button(lucide.ChevronUp, n.IncrementLabel, 1, at(n.Max), half),
			button(lucide.ChevronDown, n.DecrementLabel, -1, at(n.Min), half.Merge(core.Style{Borders: core.Edges{Left: 1, Top: 1}})),
		}}
	}
	square := core.Style{Width: core.Pt(ButtonHeight)}
	return core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
		button(lucide.Minus, n.DecrementLabel, -1, at(n.Min), square),
		button(lucide.Plus, n.IncrementLabel, 1, at(n.Max), square),
	}}
}

// Rating is a score of stars from 0 to Max (5 when 0). A press on a star
// gives that score, or 0 when it is the score already, and on desktops
// the pointer previews the score under it. Assistive technology reads it
// as one adjustable element, and on desktops it is a Tab stop the arrow
// keys adjust.
type Rating struct {
	Value    func() int
	OnChange func(value int)
	Max      int
	Readonly bool
	Disabled func() bool
	// ValueText is what assistive technology reads as the value, such as
	// "3 stars", the number when nil.
	ValueText func(value int) string
	Label     string
	TestID    string
	Style     core.Style
}

var (
	starOnce sync.Once
	star     *core.SVG
)

// filledStar is Lucide's star filled in the text's color.
func filledStar() *core.SVG {
	starOnce.Do(func() {
		star = core.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linejoin="round">` +
			`<path d="M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a2.123 2.123 0 0 0 1.595 1.16l5.166.756a.53.53 0 0 1 .294.904l-3.736 3.638a2.123 2.123 0 0 0-.611 1.878l.882 5.14a.53.53 0 0 1-.771.56l-4.618-2.428a2.122 2.122 0 0 0-1.973 0L6.396 21.01a.53.53 0 0 1-.77-.56l.881-5.139a2.122 2.122 0 0 0-.611-1.879L2.16 9.795a.53.53 0 0 1 .294-.906l5.165-.755a2.122 2.122 0 0 0 1.597-1.16z"/></svg>`))
	})
	return star
}

// Render implements core.Node.
func (r Rating) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	limit := r.Max
	if limit <= 0 {
		limit = 5
	}
	value := func() int {
		if r.Value == nil {
			return 0
		}
		return min(max(r.Value(), 0), limit)
	}
	locked := func() bool { return r.Readonly || is(r.Disabled) }
	set := func(v int) {
		if v = min(max(v, 0), limit); v != value() && !locked() && r.OnChange != nil {
			r.OnChange(v)
		}
	}
	onDesktop := !touch(cx)
	hover := core.NewSignal(cx, 0)
	shown := func() int {
		if h := hover.Get(); h > 0 && !locked() {
			return h
		}
		return value()
	}
	const size = iconSize + 3
	var stars []core.Node
	for i := 1; i <= limit; i++ {
		s := core.Icon{SVGFunc: func() *core.SVG {
			if i <= shown() {
				return filledStar()
			}
			return lucide.Star()
		}, Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)},
			StyleFunc: func() core.Style {
				if i <= shown() {
					return core.Style{Color: colors.Warn()}
				}
				return core.Style{Color: colors.Faint()}
			}}
		v := core.View{Style: core.Style{Padding: core.All(2)}, Children: []core.Node{s}}
		if r.TestID != "" {
			v.TestID = r.TestID + "-" + strconv.Itoa(i)
		}
		if !r.Readonly {
			v.NoFocus = true
			v.Style.Cursor = core.CursorPointer
			v.OnPress = func(core.PressEvent) {
				if value() == i {
					set(0)
				} else {
					set(i)
				}
			}
			if onDesktop {
				v.OnHoverIn = func() { hover.Set(i) }
				v.OnHoverOut = func() { hover.Set(0) }
			}
		}
		stars = append(stars, v)
	}
	focused := core.NewSignal(cx, false)
	stars = append(stars, core.Show{When: focused.Get, Then: func(cx *core.Scope) core.Node {
		repeat := core.KeyOptions{Repeat: true}
		cx.OnKeyWith("right", repeat, func() { set(value() + 1) })
		cx.OnKeyWith("up", repeat, func() { set(value() + 1) })
		cx.OnKeyWith("left", repeat, func() { set(value() - 1) })
		cx.OnKeyWith("down", repeat, func() { set(value() - 1) })
		cx.OnKey("home", func() { set(0) })
		cx.OnKey("end", func() { set(limit) })
		return nil
	}})
	view := core.View{TestID: r.TestID, Accessible: true, Role: core.RoleAdjustable, Label: r.Label, Disabled: r.Disabled,
		AccessibilityValue: func() core.AccessibilityValue {
			v := core.AccessibilityValue{Min: 0, Max: float32(limit), Now: float32(value())}
			if r.ValueText != nil {
				v.Text = r.ValueText(value())
			}
			return v
		},
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, CornerRadius: radiusSmall,
			Disabled: &core.Style{Opacity: core.Alpha(0.45)}}.Merge(r.Style),
		Children: stars}
	if !r.Readonly {
		view.AccessibilityActions = []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}}
		view.OnAccessibilityAction = func(name string) {
			switch name {
			case core.ActionIncrement:
				set(value() + 1)
			case core.ActionDecrement:
				set(value() - 1)
			}
		}
		if onDesktop {
			view.OnPress = func(core.PressEvent) {}
			view.OnFocus = func() { focused.Set(true) }
			view.OnBlur = func() { focused.Set(false) }
			view.StyleFunc = func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} }
		}
	}
	return view
}
