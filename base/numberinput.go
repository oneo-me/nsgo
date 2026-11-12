package base

import (
	"cmp"
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

// pageSteps is how many steps Page Up and Page Down go.
const pageSteps = 10

// NumberInput is a field for a number with buttons that step it. It owns
// what the field takes as the user types, other keyboards' digits
// included, the step and its decimals, held to Min and Max, WAI-ARIA's spin
// button keys while the field edits, and its role and value for assistive
// technology, and has no look. Its parts are the decrement button, the
// field between Prefix and Suffix, and the increment button; with
// ControlsRight the field between them and a column with the increment
// over the decrement.
type NumberInput struct {
	// Value makes it controlled once it returns a number; nil is an empty
	// field. Without it the input keeps its own, from DefaultValue.
	Value        func() *float64
	DefaultValue *float64
	// Min and Max bound it, nil for none. Steps stop there; a typed value
	// is held to them as the field is left or Enter is pressed.
	Min, Max func() *float64
	// Step is how far a step goes, 1 when unset or not positive.
	Step     func() float64
	Disabled func() bool
	// Readonly can be selected and copied but not changed.
	Readonly    func() bool
	Placeholder func() string
	// Label is what assistive technology says for the field.
	Label string
	// ControlsRight is read as it is made.
	ControlsRight bool
	// Format writes a value it did not type, the shortest text that reads
	// back as it when nil; Parse reads the field, as a number when nil.
	Format func(value float64) string
	Parse  func(text string) (float64, bool)
	// OnCommit leaves the field's text alone as the user types and takes
	// it as it is left or Return pressed, for a Format of the app's.
	OnCommit bool
	// Live, with OnCommit, also reports what is typed as soon as it reads
	// as a number in range, leaving the text alone until the field is left.
	Live bool
	// Snap rounds what is committed to Precision, or Step while that is 0,
	// from Min.
	Snap      bool
	Precision float64
	// IncrementLabel and DecrementLabel name the buttons for assistive
	// technology, which otherwise steps the field itself.
	IncrementLabel, DecrementLabel string
	// Scrub is a handle a drag across steps the value with, a step each
	// ScrubDistance points, 4 when 0.
	Scrub          core.Node
	ScrubDistance  core.Points
	ScrubClass     string
	ScrubStyle     core.Style
	ScrubStyleFunc func() core.Style
	ScrubTestID    string
	// OnChange receives the number the field reads as, nil while it reads
	// as none, as the user types and steps.
	OnChange func(value *float64)
	// OnChangeEnd follows the OnChange calls of one change: a step, a
	// scrub's release, or the field left or submitted after typing.
	OnChangeEnd func()
	// OnSubmit receives its value on Return, held to Min and Max.
	OnSubmit func(value *float64)
	// OnFocus and OnBlur follow the field.
	OnFocus, OnBlur func()
	Class           string
	Style           core.Style
	StyleFunc       func() core.Style
	InputClass      string
	InputStyle      core.Style
	InputStyleFunc  func() core.Style
	// ButtonLook is both buttons' style following what it reads.
	ButtonLook           func() core.Style
	DecrementButton      core.Node
	DecrementButtonClass string
	DecrementButtonStyle core.Style
	IncrementButton      core.Node
	IncrementButtonClass string
	IncrementButtonStyle core.Style
	ControlsClass        string
	ControlsStyle        core.Style
	Prefix, Suffix       core.Node
	TestID               string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// finite is p when it holds a finite number, else nil.
func finite(p *float64) *float64 {
	if p == nil || !isFinite(*p) {
		return nil
	}
	return p
}

func sameNumber(a, b *float64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// numberCtl is what the input's parts and listeners share.
type numberCtl struct {
	n  NumberInput
	cx *core.Scope
	// text is what the field shows.
	text *core.Signal[string]
	// held is the number the field holds, the last it reported or the
	// app's; controlled that the app passed a number, so it owns the value
	// from then on.
	held       *float64
	controlled bool
	// pendingNumber and pendingText are a step's, shown once the app's
	// value follows it.
	pendingNumber *float64
	pendingText   string
	// unsettled is a change reported whose end is not.
	unsettled bool
}

func (c *numberCtl) bounds() (lo, hi *float64) {
	if c.n.Min != nil {
		lo = finite(c.n.Min())
	}
	if c.n.Max != nil {
		hi = finite(c.n.Max())
	}
	if lo != nil && hi != nil && *hi < *lo {
		hi = lo
	}
	return lo, hi
}

func (c *numberCtl) step() float64 {
	if c.n.Step != nil {
		if s := c.n.Step(); isFinite(s) && s > 0 {
			return s
		}
	}
	return 1
}

func (c *numberCtl) stepped(up bool, n float64) (string, bool) {
	lo, hi := c.bounds()
	text := c.text.Get()
	if c.n.Format != nil {
		// What the app writes is not ours to read: from the number.
		text = ""
		if c.held != nil {
			text = numberText(*c.held)
		}
	}
	return stepText(text, up, timesStep(c.step(), n), lo, hi)
}

// numberOf is the number text reads as, nil for none.
func (c *numberCtl) numberOf(text string) *float64 {
	if c.n.Parse != nil {
		if v, ok := c.n.Parse(text); ok && isFinite(v) {
			return &v
		}
		return nil
	}
	return numberOf(text)
}

func (c *numberCtl) textOf(n *float64) string {
	if n != nil && c.n.Format != nil {
		return c.n.Format(*n)
	}
	return textOf(n)
}

// fit holds v to the range, and with Snap rounds it to the step.
func (c *numberCtl) fit(v float64) float64 {
	lo, hi := c.bounds()
	if c.n.Snap {
		step, from := cmp.Or(c.n.Precision, c.step()), 0.0
		if lo != nil {
			from = *lo
		}
		scale := math.Pow(10, float64(decimals(step)))
		v = math.Round((from+math.Round((v-from)/step)*step)*scale) / scale
	}
	return clampTo(v, lo, hi)
}

// scrub moves the value to steps from what it was as the drag began.
func (c *numberCtl) scrub(from float64, steps float64) {
	v := c.fit(from + steps*core.Untracked(c.cx, c.step))
	next := &v
	if sameNumber(c.held, next) {
		return
	}
	if !c.controlled {
		c.text.Set(c.textOf(next))
		c.held = next
	}
	c.unsettled = true
	if c.n.OnChange != nil {
		c.n.OnChange(next)
	}
}

// settle reports the end of a change reported.
func (c *numberCtl) settle() {
	if c.unsettled {
		c.unsettled = false
		if c.n.OnChangeEnd != nil {
			c.n.OnChangeEnd()
		}
	}
}

// inRange reports whether v is within the range.
func (c *numberCtl) inRange(v float64) bool {
	lo, hi := c.bounds()
	return clampTo(v, lo, hi) == v
}

func (c *numberCtl) locked() bool { return is(c.n.Disabled) || is(c.n.Readonly) }

func (c *numberCtl) report(n *float64) {
	if sameNumber(c.held, n) {
		return
	}
	c.held = n
	c.unsettled = true
	if c.n.OnChange != nil {
		c.n.OnChange(n)
	}
}

func numberOf(text string) *float64 {
	if v, ok := parseNumber(text); ok {
		return &v
	}
	return nil
}

// edit takes the field's new text when it is a number or on the way to
// one, else the field goes back to its text.
func (c *numberCtl) edit(typed string) {
	if c.n.OnCommit {
		c.text.Set(typed)
		if n := c.numberOf(typed); c.n.Live && n != nil && core.Untracked(c.cx, func() bool { return c.inRange(*n) }) {
			c.report(n)
		}
		return
	}
	normalized := normalizeNumberText(typed)
	if !isNumberText(normalized) {
		c.show(typed, c.text.GetUntracked())
		return
	}
	c.pendingNumber = nil
	c.show(typed, normalized)
	c.report(c.numberOf(normalized))
}

// show has the field, whose text the user just made typed, show text. The
// field's binding sends only what differs from its last text, so an edit
// taken back to the text it had takes the typed text first.
func (c *numberCtl) show(typed, text string) {
	if text == typed || text != c.text.GetUntracked() {
		c.text.Set(text)
		return
	}
	c.text.Set(typed)
	c.cx.Post(func() { c.text.Set(text) })
}

// nudge is a key's, a button's or a screen reader's step. The app that
// owns the value hears it and moves the field by setting it.
func (c *numberCtl) nudge(up bool, steps float64) {
	if core.Untracked(c.cx, func() bool { return c.locked() }) {
		return
	}
	next, ok := "", false
	c.cx.Untrack(func() { next, ok = c.stepped(up, steps) })
	if !ok {
		return
	}
	n := numberOf(next)
	if c.controlled {
		if n != nil {
			c.pendingNumber, c.pendingText = n, c.textOf(n)
			if c.n.Format == nil {
				c.pendingText = next
			}
		}
		c.unsettled = true
		if c.n.OnChange != nil {
			c.n.OnChange(n)
		}
		c.settle()
		return
	}
	if c.n.Format != nil {
		next = c.textOf(n)
	}
	c.text.Set(next)
	c.report(n)
	c.settle()
}

// commit holds the field's number to the range, and shows the app's value;
// a text that reads as no number stays.
func (c *numberCtl) commit() *float64 {
	typed := c.numberOf(c.text.GetUntracked())
	var held *float64
	if typed != nil {
		var v float64
		c.cx.Untrack(func() { v = c.fit(*typed) })
		held = &v
		c.report(held)
	}
	c.pendingNumber = nil
	if c.controlled {
		value := finite(core.Untracked(c.cx, c.n.Value))
		c.held = value
		if !sameNumber(typed, value) || c.n.Format != nil {
			c.text.Set(c.textOf(value))
		}
	} else if typed != nil && *typed != *held {
		c.text.Set(c.textOf(held))
	}
	c.settle()
	return held
}

func textOf(n *float64) string {
	if n == nil {
		return ""
	}
	return numberText(*n)
}

// follow has the field show a value the app set, unless it is the number
// it holds: what the user is typing stays.
func (c *numberCtl) follow(value *float64) {
	if value != nil {
		c.controlled = true
	}
	if !c.controlled || sameNumber(c.held, value) {
		return
	}
	c.held = value
	shown := c.textOf(value)
	if c.pendingNumber != nil && sameNumber(c.pendingNumber, value) {
		shown = c.pendingText
	}
	c.pendingNumber = nil
	c.text.Set(shown)
}

func (c *numberCtl) a11yValue() core.AccessibilityValue {
	lo, hi := c.bounds()
	text := c.text.Get()
	v := core.AccessibilityValue{Text: text, NoNow: true}
	if lo != nil {
		v.Min = float32(*lo)
	}
	if hi != nil {
		v.Max = float32(*hi)
	}
	if n := c.numberOf(text); n != nil {
		v.Now, v.NoNow = float32(*n), false
	}
	return v
}

// Render implements core.Node.
func (n NumberInput) Render(cx *core.Scope) core.Node {
	var start *float64
	if n.Value != nil {
		start = finite(core.Untracked(cx, n.Value))
	}
	c := &numberCtl{n: n, cx: cx, controlled: start != nil}
	if start == nil {
		start = finite(n.DefaultValue)
	}
	c.held = start
	c.text = core.NewSignal(cx, c.textOf(start))
	if n.Value != nil {
		cx.Effect(func() {
			v := finite(n.Value())
			cx.Untrack(func() { c.follow(v) })
		})
	}
	focused := core.NewSignal(cx, false)
	nudge := func(up bool, steps float64) func() { return func() { c.nudge(up, steps) } }
	keys := bindWhile(focused.Get, core.KeyAlways, false,
		controlKey{"up", true, nudge(true, 1)}, controlKey{"down", true, nudge(false, 1)},
		controlKey{"pageup", true, nudge(true, pageSteps)}, controlKey{"pagedown", true, nudge(false, pageSteps)})
	field := new(core.Ref)
	input := core.Input{Ref: field, Class: n.InputClass, Style: n.InputStyle, StyleFunc: n.InputStyleFunc, Value: c.text.Get, PlaceholderFunc: n.Placeholder,
		Label: n.Label, Role: core.RoleSpinButton, NoAutoCorrect: true, Disabled: n.Disabled,
		ReadonlyFunc: func() bool { return is(n.Readonly) || is(n.Disabled) },
		KeyboardTypeFunc: func() core.KeyboardType {
			// Phones' decimal pads have no minus sign.
			if lo, _ := c.bounds(); lo != nil && *lo >= 0 {
				return core.KeyboardDecimal
			}
			return core.KeyboardDefault
		},
		AccessibilityValue:   c.a11yValue,
		AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
		OnAccessibilityAction: func(name string) {
			switch name {
			case core.ActionIncrement:
				c.nudge(true, 1)
			case core.ActionDecrement:
				c.nudge(false, 1)
			}
		},
		OnChange: c.edit,
		OnFocus: func() {
			if focused.GetUntracked() {
				return
			}
			focused.Set(true)
			if n.OnFocus != nil {
				n.OnFocus()
			}
		},
		OnBlur: func() {
			focused.Set(false)
			c.commit()
			if n.OnBlur != nil {
				n.OnBlur()
			}
		},
		OnSubmit: func(string) {
			v := c.commit()
			if n.OnSubmit != nil {
				n.OnSubmit(v)
			}
		}}
	env := cx.Env()
	// The buttons step the field and never take the focus; on desktops a
	// press moves it to the field. Screen readers step the field itself.
	button := func(up bool, label, class string, style core.Style, content core.Node) core.Node {
		var children []core.Node
		if content != nil {
			children = []core.Node{content}
		}
		role := core.RoleNone
		if label != "" {
			role = core.RoleButton
		}
		return core.View{Class: class, Style: style, StyleFunc: n.ButtonLook, Role: role, Label: label, NoFocus: true, Children: children,
			AccessibilityElementsHidden: func() bool { return label == "" },
			Disabled: func() bool {
				if c.locked() {
					return true
				}
				_, ok := c.stepped(up, 1)
				return !ok
			},
			OnPress: func(core.PressEvent) {
				c.nudge(up, 1)
				if env.Desktop() && !focused.GetUntracked() {
					field.Focus()
				}
			}}
	}
	inc := button(true, n.IncrementLabel, n.IncrementButtonClass, n.IncrementButtonStyle, n.IncrementButton)
	dec := button(false, n.DecrementLabel, n.DecrementButtonClass, n.DecrementButtonStyle, n.DecrementButton)
	if n.Scrub != nil {
		var from float64
		distance := float64(cmp.Or(n.ScrubDistance, 4))
		n.Prefix = core.Group{n.Prefix, core.View{TestID: n.ScrubTestID, Class: n.ScrubClass, Style: n.ScrubStyle, StyleFunc: n.ScrubStyleFunc,
			ImportantForAccessibility: core.ImportantNoHideDescendants, PanActivation: core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12},
			OnPan: func(e core.PanEvent) {
				if is(n.Disabled) || is(n.Readonly) {
					return
				}
				switch e.Phase {
				case core.GestureBegan:
					from = 0
					if c.held != nil {
						from = *c.held
					}
				case core.GestureChanged, core.GestureEnded:
					c.scrub(from, math.Round(float64(e.Translation.X)/distance))
				}
				if e.Phase != core.GestureBegan && e.Phase != core.GestureChanged {
					c.settle()
				}
			},
			Children: []core.Node{n.Scrub}}}
	}
	var children []core.Node
	if n.ControlsRight {
		controls := core.View{Class: n.ControlsClass, Style: n.ControlsStyle, Children: []core.Node{inc, dec},
			PartStates: func() core.PartStates { return core.PartStates{Disabled: is(n.Disabled)} }}
		children = []core.Node{n.Prefix, input, n.Suffix, controls}
	} else {
		children = []core.Node{dec, n.Prefix, input, n.Suffix, inc}
	}
	return core.View{Class: n.Class, Style: n.Style, StyleFunc: n.StyleFunc, Disabled: n.Disabled, TestID: n.TestID, OnLayout: n.OnLayout, Tooltip: n.Tooltip,
		PartStates: func() core.PartStates { return core.PartStates{Focused: focused.Get()} },
		Children:   append(children, keys)}
}
