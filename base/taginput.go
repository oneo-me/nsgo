package base

import (
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

// TagInput is a field that edits a list of tags: each with a remove
// button, then the text being typed, which Return, a comma or leaving the
// field adds as a tag. Backspace in the empty field removes the last tag.
// Tags are trimmed, and empty ones and repeats dropped. Style.Focus styles
// it while the field has the focus.
type TagInput struct {
	Tags     func() []string
	OnChange func(tags []string)
	// Accept, when set, keeps out the tags it rejects, which stay typed.
	Accept      func(tag string) bool
	Placeholder string
	Disabled    func() bool
	// RemoveLabel names a tag's remove button for assistive technology.
	RemoveLabel func(tag string) string
	// RemoveIcon is what the remove button shows.
	RemoveIcon      core.Node
	Class           string
	Style           core.Style
	StyleFunc       func() core.Style
	TagClass        string
	TagStyle        core.Style
	TagStyleFunc    func() core.Style
	TextClass       string
	TextStyle       core.Style
	RemoveClass     string
	RemoveStyle     core.Style
	RemoveStyleFunc func() core.Style
	InputClass      string
	InputStyle      core.Style
	InputStyleFunc  func() core.Style
	// Label names the field for assistive technology.
	Label  string
	TestID string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (in TagInput) Render(cx *core.Scope) core.Node {
	draft, focused := core.NewSignal(cx, ""), core.NewSignal(cx, false)
	tags := func() []string {
		if in.Tags == nil {
			return nil
		}
		return in.Tags()
	}
	set := func(next []string) {
		if in.OnChange != nil {
			in.OnChange(next)
		}
	}
	// add adds the tags in text and returns what it kept out.
	add := func(text string) string {
		next := slices.Clone(core.Untracked(cx, tags))
		before := len(next)
		var kept []string
		for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == '\n' }) {
			part = strings.TrimSpace(part)
			switch {
			case part == "" || slices.Contains(next, part):
			case in.Accept != nil && !in.Accept(part):
				kept = append(kept, part)
			default:
				next = append(next, part)
			}
		}
		if len(next) != before {
			set(next)
		}
		return strings.Join(kept, ", ")
	}
	chip := func(cx *core.Scope, tag func() string, _ func() int) core.Node {
		label := ""
		if in.RemoveLabel != nil {
			label = in.RemoveLabel(core.Untracked(cx, tag))
		}
		remove := Button{Label: label, Disabled: in.Disabled, Class: in.RemoveClass, Style: in.RemoveStyle, StyleFunc: in.RemoveStyleFunc,
			OnPress: func(core.PressEvent) {
				gone := core.Untracked(cx, tag)
				set(slices.DeleteFunc(slices.Clone(core.Untracked(cx, tags)), func(s string) bool { return s == gone }))
			}}
		if in.RemoveIcon != nil {
			remove.Children = []core.Node{in.RemoveIcon}
		}
		return core.View{Class: in.TagClass, Style: in.TagStyle, StyleFunc: in.TagStyleFunc,
			Children: []core.Node{core.Text{ContentFunc: tag, MaxLines: 1, Class: in.TextClass, Style: in.TextStyle}, remove}}
	}
	field := Input{Value: draft.Get, Placeholder: in.Placeholder, Disabled: in.Disabled, Label: in.Label,
		Class: in.InputClass, Style: in.InputStyle, StyleFunc: in.InputStyleFunc,
		OnChange: func(s string) {
			if i := strings.LastIndexAny(s, ",\n"); i >= 0 {
				kept := add(s[:i])
				s = strings.TrimLeft(s[i+1:], " ")
				if kept != "" {
					s = kept + ", " + s
				}
			}
			draft.Set(s)
		},
		OnSubmit: func(s string) { draft.Set(add(s)) },
		OnKeyPress: func(key string) {
			if key == "Backspace" && draft.GetUntracked() == "" {
				if all := core.Untracked(cx, tags); len(all) > 0 {
					set(slices.Clone(all[:len(all)-1]))
				}
			}
		},
		OnFocus: func() { focused.Set(true) },
		OnBlur: func() {
			focused.Set(false)
			draft.Set(add(draft.GetUntracked()))
		}}
	return core.View{Class: in.Class, Style: in.Style, StyleFunc: in.StyleFunc, TestID: in.TestID, OnLayout: in.OnLayout, Tooltip: in.Tooltip, Disabled: in.Disabled,
		PartStates: func() core.PartStates { return core.PartStates{Focused: focused.Get()} },
		Children:   []core.Node{core.Each(tags, func(s string) string { return s }, chip), field}}
}
