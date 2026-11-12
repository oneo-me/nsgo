package ui

import (
	"slices"
	"strings"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// TagInput is a field of bordered chips that edits a list of tags: each
// with a remove button, then the text being typed, which Return, a comma
// or leaving the field adds as a tag. Backspace in the empty field removes
// the last tag. Tags are trimmed, and empty ones and repeats dropped.
type TagInput struct {
	Tags     func() []string
	OnChange func(tags []string)
	// Accept, when set, keeps out the tags it rejects, which stay typed.
	Accept      func(tag string) bool
	Placeholder string
	Disabled    func() bool
	// RemoveLabel names a tag's remove button for assistive technology,
	// such as "Remove design".
	RemoveLabel func(tag string) string
	// Label names the field for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (in TagInput) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	const chipSize = ButtonHeight - 8
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
		remove := base.Button{Label: label, Disabled: in.Disabled,
			Style: core.Style{Width: core.Pt(chipSize - 6), Height: core.Pt(chipSize - 6), CornerRadius: RadiusFull,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Transition: quick},
			StyleFunc: func() core.Style {
				return core.Style{Color: colors.Muted(), Hover: &core.Style{Background: colors.Pressed(), Color: colors.Text()}, Focus: FocusRing(cx, 0)}
			},
			OnPress: func(core.PressEvent) {
				gone := core.Untracked(cx, tag)
				set(slices.DeleteFunc(slices.Clone(core.Untracked(cx, tags)), func(s string) bool { return s == gone }))
			}, Children: []core.Node{Icon(lucide.X, TypeCaption.FontSize, Inherited)}}
		return core.View{Style: TypeBody.Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2, Height: core.Pt(chipSize),
			Padding: core.Edges{Left: 8, Right: 2}, CornerRadius: RadiusSmall, BorderWidth: 1, MinWidth: core.Pt(0), FlexShrink: 1, Cursor: core.CursorDefault}),
			StyleFunc: func() core.Style {
				return core.Style{BorderColor: colors.Border(), Background: colors.Raised(), Color: colors.Text()}
			},
			Children: []core.Node{core.Text{ContentFunc: tag, MaxLines: 1, Style: core.Style{FlexShrink: 1}}, remove}}
	}
	field := base.Input{Value: draft.Get, Placeholder: in.Placeholder, Disabled: in.Disabled, Label: in.Label,
		Style: TypeBody.Merge(core.Style{FlexGrow: 1, FlexBasis: core.Pt(80), MinWidth: core.Pt(80), Height: core.Pt(chipSize),
			Padding: core.Symmetric(0, 4)}),
		StyleFunc: func() core.Style {
			return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Now().Selection, CaretColor: colors.Accent()}
		},
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
	return core.View{TestID: in.TestID, Disabled: in.Disabled,
		Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 4, MinHeight: core.Pt(ButtonHeight),
			Padding: core.Symmetric(3, 4), CornerRadius: RadiusControl, BorderWidth: 1, Cursor: core.CursorText, Transition: quick},
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(), Color: colors.Text(),
				Hover:    &core.Style{BorderColor: Mix(colors.FieldBorder(), colors.Text(), 0.25)},
				Focus:    &core.Style{BorderColor: colors.Accent()},
				Disabled: &core.Style{Opacity: core.Alpha(0.5), Background: colors.Inset(), BorderColor: colors.Border()}}.Merge(in.Style)
		},
		PartStates: func() core.PartStates { return core.PartStates{Focused: focused.Get()} },
		Children:   []core.Node{core.Each(tags, func(s string) string { return s }, chip), field}}
}
