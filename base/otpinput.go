package base

import (
	"slices"
	"time"
	"unicode/utf16"

	"github.com/oneo-me/nsgo/core"
)

const (
	// otpBlink is how long the caret shows and hides while it blinks, and
	// otpBlinkPause how long it holds after an edit: gpui-base's.
	otpBlink      = 500 * time.Millisecond
	otpBlinkPause = 300 * time.Millisecond
	// otpMask is what a filled cell shows while Masked.
	otpMask = "•"
	// otpEchoes is how many reported codes the input waits to see come
	// back as Value: a few keys behind, not an app that keeps its code.
	otpEchoes = 16
)

// OtpInput is a fixed number of digits entered into cells, a code from a
// text message. One native field, transparent and over the cells, owns the
// keyboard: typing, input methods, paste, the system's code autofill and
// what assistive technology knows; the cells only show the code, so the
// caret stays at the end and nothing is edited in the middle. The code is
// the app's (Value) or its own (DefaultValue), and the look is the app's:
// focus styles the cell that takes the next digit, data-filled those that
// hold one. A press anywhere on it focuses it.
type OtpInput struct {
	Length int
	// Value makes the code controlled, shown as it is; DefaultValue starts
	// its own.
	Value        func() string
	DefaultValue string
	// Masked shows bullets, in a secure field.
	Masked   bool
	Disabled func() bool
	// Groups is how many groups the cells split into, as evenly as they
	// go.
	Groups int
	// Label is what assistive technology says for the field.
	Label string
	// OnChange receives the code the user made; a Value the app passes is
	// never reported. OnComplete receives it once full, after OnChange.
	OnChange, OnComplete func(value string)
	OnFocus, OnBlur      func()
	// Ref's Focus takes the keyboard focus.
	Ref       *OtpInputRef
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	CellClass string
	CellStyle core.Style
	// CellStyleFunc and CaretStyleFunc are the parts' styles following
	// what they read.
	CellStyleFunc  func() core.Style
	GroupClass     string
	GroupStyle     core.Style
	CaretClass     string
	CaretStyle     core.Style
	CaretStyleFunc func() core.Style
	TestID         string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// OtpInputRef is an OtpInput's ref.
type OtpInputRef struct{ field *core.Ref }

// Focus takes the keyboard focus, showing the software keyboard.
func (r *OtpInputRef) Focus() {
	if r.field != nil {
		r.field.Focus()
	}
}

// otpDigit is c as an ASCII digit: itself, or a full-width digit's.
func otpDigit(c rune) (rune, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c, true
	case c >= '０' && c <= '９':
		return c - '０' + '0', true
	}
	return 0, false
}

// otpDiff is what the field lost at its end, in characters, and what it
// gained, from before to after.
func otpDiff(before, after string) (int, string) {
	b, a := []rune(before), []rune(after)
	prefix := 0
	for prefix < len(b) && prefix < len(a) && b[prefix] == a[prefix] {
		prefix++
	}
	return len(b) - prefix, string(a[prefix:])
}

// otpEdit is the code after an edit of shown that removed deleted
// characters at its end and inserted inserted: its digits appended while
// the code is shorter than length, or, length of them at once, a paste or
// an autofill, the code replaced by the first length.
func otpEdit(shown string, deleted int, inserted string, length int) string {
	var digits []rune
	for _, c := range inserted {
		if d, ok := otpDigit(c); ok {
			digits = append(digits, d)
		}
	}
	if length > 0 && len(digits) >= length {
		return string(digits[:length])
	}
	chars := []rune(shown)
	chars = chars[:max(len(chars)-deleted, 0)]
	for _, d := range digits {
		if len(chars) >= length {
			break
		}
		chars = append(chars, d)
	}
	return string(chars)
}

// otpEchoed takes value out of echoes, the codes reported and not seen
// back, with those reported before it, reporting whether it is one.
func otpEchoed(echoes *[]string, value string) bool {
	if at := slices.Index(*echoes, value); at >= 0 {
		*echoes = (*echoes)[at+1:]
		return true
	}
	return false
}

// otpGroups is how many groups requested splits length cells into: at
// least one, at most one per cell.
func otpGroups(length, requested int) int { return min(max(requested, 1), max(length, 1)) }

func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }

// Render implements core.Node.
func (o OtpInput) Render(cx *core.Scope) core.Node {
	own := core.NewSignal(cx, o.DefaultValue)
	code := func() string {
		if o.Value != nil {
			return o.Value()
		}
		return own.Get()
	}
	// field is what the field holds as far as the input knows; typed the
	// code as the user edited it, which the next edit applies to, as a
	// controlled Value may come back keys later.
	start := core.Untracked(cx, code)
	field := core.NewSignal(cx, start)
	typed := start
	var echoes []string
	focused := core.NewSignal(cx, false)
	blink := newOtpBlink(cx)
	ref := new(core.Ref)
	if o.Ref != nil {
		o.Ref.field = ref
	}
	cx.Effect(func() {
		value := code()
		cx.Untrack(func() {
			if otpEchoed(&echoes, value) {
				return
			}
			echoes = nil
			typed = value
			field.Set(value)
		})
	})
	changed := func(text string) {
		before := field.GetUntracked()
		deleted, inserted := otpDiff(before, text)
		next := otpEdit(typed, deleted, inserted, o.Length)
		if next != typed {
			typed = next
			if len(echoes) == otpEchoes {
				echoes = echoes[1:]
			}
			echoes = append(echoes, next)
			if o.Value == nil {
				own.Set(next)
			}
			if focused.GetUntracked() {
				blink.start(otpBlinkPause)
			}
			if o.OnChange != nil {
				o.OnChange(next)
			}
			if len([]rune(next)) == o.Length && o.OnComplete != nil {
				o.OnComplete(next)
			}
		}
		end := func() { ref.SetSelection(utf16Len(next), utf16Len(next)) }
		switch {
		case next == text:
			field.Set(next)
		case next != before:
			field.Set(next)
			end()
		default:
			// The binding's last value is the code already: the field's
			// text goes there first, then back, so the field gets it.
			field.Set(text)
			cx.Post(func() {
				field.Set(next)
				end()
			})
		}
	}
	disabled := func() bool { return is(o.Disabled) }
	cell := func(ix int) core.Node {
		digit := func() (string, bool) {
			chars := []rune(code())
			if ix < len(chars) {
				return string(chars[ix]), true
			}
			return "", false
		}
		filled := func() bool { _, ok := digit(); return ok }
		active := func() bool { return min(len([]rune(code())), max(o.Length-1, 0)) == ix }
		content := core.Show{When: filled, Then: func(*core.Scope) core.Node {
			return core.Text{ContentFunc: func() string {
				if o.Masked {
					return otpMask
				}
				d, _ := digit()
				return d
			}}
		}, Else: func(*core.Scope) core.Node {
			return core.Show{When: func() bool { return focused.Get() && active() }, Then: func(*core.Scope) core.Node {
				return core.View{Class: o.CaretClass, Style: core.Style{Animated: &core.AnimatedStyle{Opacity: blink.opacity}}.Merge(o.CaretStyle), StyleFunc: o.CaretStyleFunc}
			}}
		}}
		return core.View{Class: o.CellClass, Style: o.CellStyle, StyleFunc: o.CellStyleFunc, Children: []core.Node{content},
			PartStates: func() core.PartStates {
				s := core.PartStates{Focused: focused.Get() && !disabled() && active(), Disabled: disabled()}
				if filled() {
					s.Data = []string{"filled"}
				}
				return s
			}}
	}
	perGroup := 1
	if o.Length > 0 {
		perGroup = max((o.Length+otpGroups(o.Length, o.Groups)-1)/otpGroups(o.Length, o.Groups), 1)
	}
	var groups []core.Node
	for at := 0; at < o.Length; at += perGroup {
		var cells []core.Node
		for ix := at; ix < min(at+perGroup, o.Length); ix++ {
			cells = append(cells, cell(ix))
		}
		// The field tells assistive technology the code.
		groups = append(groups, core.View{Class: o.GroupClass, Style: o.GroupStyle, Children: cells,
			AccessibilityElementsHidden: func() bool { return true },
			PartStates:                  func() core.PartStates { return core.PartStates{Disabled: disabled()} }})
	}
	input := core.Input{Ref: ref, Label: o.Label, Value: field.Get, Masked: o.Masked, Disabled: o.Disabled, HideCaret: true,
		KeyboardType: core.KeyboardNumber, AutoComplete: core.AutoCompleteOneTimeCode, NoAutoCorrect: true, AutoCapitalize: core.AutoCapitalizeNone,
		Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Padding: core.All(0),
			BorderWidth: 0, Background: core.Transparent, Color: core.Transparent, SelectionColor: core.Transparent},
		OnChange: changed,
		OnSelectionChange: func(r core.TextRange) {
			// Past the end is a key on its way: platforms report the caret
			// before the text.
			if end := utf16Len(field.GetUntracked()); r.Start < end {
				ref.SetSelection(end, end)
			}
		},
		OnFocus: func() {
			focused.Set(true)
			blink.start(otpBlink)
			if o.OnFocus != nil {
				o.OnFocus()
			}
		},
		OnBlur: func() {
			focused.Set(false)
			blink.stop()
			if o.OnBlur != nil {
				o.OnBlur()
			}
		}}
	return core.View{Class: o.Class, Style: o.Style, StyleFunc: o.StyleFunc, Disabled: o.Disabled, TestID: o.TestID, OnLayout: o.OnLayout, Tooltip: o.Tooltip,
		Children: append(groups, input)}
}

// otpBlinker blinks the caret: its opacity, moved natively, so a blink
// costs the engine nothing.
type otpBlinker struct {
	cx      *core.Scope
	opacity *core.AnimatedValue
	run     *core.AnimationRun
}

func newOtpBlink(cx *core.Scope) *otpBlinker {
	return &otpBlinker{cx: cx, opacity: core.NewAnimatedValue(cx, 1)}
}

func (b *otpBlinker) stop() {
	if b.run != nil {
		b.run.Stop()
		b.run = nil
	}
}

// start shows the caret, then blinks it after hold, unless the user asked
// for less motion.
func (b *otpBlinker) start(hold time.Duration) {
	b.stop()
	b.opacity.Set(1)
	if core.Untracked(b.cx, b.cx.Env().ReduceMotion) {
		return
	}
	jump := func(to float64) *core.Animation {
		return core.Timing(b.opacity, to, core.TimingConfig{Duration: time.Millisecond})
	}
	loop := core.Loop(core.Sequence(jump(0), core.Delay(otpBlink), jump(1), core.Delay(otpBlink)), 0)
	b.run = b.cx.StartAnimation(core.Sequence(core.Delay(hold), loop), nil)
}
