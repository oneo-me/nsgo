package base

import (
	"slices"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// Select picks one value from a list it opens under its trigger on
// desktops, the selected item over the trigger, and as a sheet on touch
// screens. Value makes the value controlled, nil for none, with its own
// from DefaultValue otherwise. The list has no look: the app styles each
// part, a row's Focused style following the highlight, its Selected style
// the value. Up, Down, Return and Space open it from the focused trigger,
// and letters choose by typeahead.
type Select struct {
	Items        func() []SelectItem
	Value        func() *string
	DefaultValue string
	Open         func() bool
	DefaultOpen  bool
	Disabled     func() bool
	// Searchable shows a field above the rows that filters them by title.
	Searchable        func() bool
	SearchPlaceholder string
	SearchLabel       string
	// Cleanable shows a button while an item is chosen, which clears the
	// value and reports nil.
	Cleanable bool

	Placement core.Placement
	// Align lines the list up with its trigger, at its start when 0.
	Align          core.Align
	Offset, Margin core.Points

	Class     string // the trigger's
	Style     core.Style
	StyleFunc func() core.Style
	// RootClass and RootStyle are the view around the trigger and its
	// clear button.
	RootClass      string
	RootStyle      core.Style
	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	// BackdropClass and BackdropStyle are the sheet's, on touch screens.
	BackdropClass string
	BackdropStyle core.Style
	ListClass     string
	ListStyle     core.Style
	ItemClass     string
	ItemStyle     core.Style
	// ItemStyleFunc is each row's style following what it reads.
	ItemStyleFunc  func() core.Style
	GroupClass     string
	GroupStyle     core.Style
	GroupStyleFunc func() core.Style
	SearchClass    string
	SearchStyle    core.Style
	// SearchField draws the search field, as a styled form does.
	SearchField func(core.Input) core.Node

	ClearButtonClass string
	ClearButtonStyle core.Style
	ClearButton      core.Node
	ClearButtonLabel string

	Trigger core.Node
	// Empty shows in the list's place while no item matches.
	Empty core.Node
	// RenderItem draws what a row is, and RenderGroup a header: a move of
	// the highlight or a choice draws nothing again.
	RenderItem  func(SelectEntry) core.Node
	RenderGroup func(SelectGroup) core.Node

	OnChange     func(value *string)
	OnOpenChange func(open bool, reason Reason)
	OnOpen       func()
	OnClose      func()
	OnCancel     func()
	Label        string
	TestID       string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// opening is how a list was asked to open: which item it starts on when
// none is chosen.
type opening uint8

const (
	openedByPress opening = iota
	openedByDown
	openedByUp
)

// initial is the item a list starts on with none chosen.
func (o opening) initial(s *listShown) int {
	switch o {
	case openedByDown:
		return s.step(-1, listMove{kind: moveFirst})
	case openedByUp:
		return s.step(-1, listMove{kind: moveLast})
	}
	return -1
}

// Render implements core.Node.
func (s Select) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, s.Open, s.DefaultOpen, nil, nil, nil)
	own := core.NewSignalFunc(cx, ownValue(s.DefaultValue), sameValue)
	value := func() *string {
		if s.Value != nil {
			return s.Value()
		}
		return own.Get()
	}
	current := core.NewMemoFunc(cx, value, sameValue)
	searchable := func() bool { return is(s.Searchable) }
	lb := newListbox(cx, s.Items, searchable, listQuery{})
	anchor := new(core.Ref)
	width := core.NewSignal[core.Points](cx, -1)
	focused := core.NewSignal(cx, false)
	how, closedBy := openedByPress, Reason(0)
	disabled := func() bool { return is(s.Disabled) }
	// The report comes before the state, so OnOpen and OnClose follow it.
	change := func(next bool, r Reason) {
		if next && core.Untracked(cx, disabled) || core.Untracked(cx, st.open) == next {
			return
		}
		if !next {
			closedBy = r
		}
		if s.OnOpenChange != nil {
			s.OnOpenChange(next, r)
		}
		if st.own != nil {
			st.own.Set(next)
		}
	}
	choose := func(next *string) {
		if sameValue(core.Untracked(cx, value), next) {
			return
		}
		if s.Value == nil {
			own.Set(next)
		}
		if s.OnChange != nil {
			s.OnChange(next)
		}
	}
	choosable := func(item int) *string {
		if items := lb.index.GetUntracked().items; item >= 0 && item < len(items) && !items[item].Disabled {
			return &items[item].Value
		}
		return nil
	}
	pick := func(item int) {
		if next := choosable(item); next != nil {
			choose(next)
			change(false, ReasonSelect)
		}
	}
	dismiss := func(r Reason) {
		if s.OnCancel != nil {
			s.OnCancel()
		}
		change(false, r)
	}
	// Each opening starts on the chosen item, else as it was asked for.
	was := false
	cx.Effect(func() {
		now := st.open()
		if now == was {
			return
		}
		was = now
		cx.Untrack(func() {
			if !now {
				lb.closed()
				return
			}
			asked := how
			how = openedByPress
			lb.opened(func(shown *listShown, items []SelectItem) int {
				if v := current.GetUntracked(); v != nil {
					i := slices.IndexFunc(items, func(it SelectItem) bool { return it.Value == *v && !it.Disabled })
					if i >= 0 && shown.rowOf(i) >= 0 {
						return i
					}
				}
				return asked.initial(shown)
			})
		})
	})
	opener := func(o opening) func() {
		return func() {
			how = o
			change(true, ReasonTriggerPress)
		}
	}
	closedKeys := bindWhile(func() bool { return focused.Get() && !st.open() && !disabled() }, core.KeyNotEditing, false,
		controlKey{key: "down", run: opener(openedByDown)}, controlKey{key: "up", run: opener(openedByUp)},
		controlKey{key: "enter", run: opener(openedByDown)}, controlKey{key: "space", run: opener(openedByDown)})
	// Typeahead: on the focused trigger it chooses, in the open list
	// without a search field it highlights.
	ahead := core.NewMemo(cx, func() bool {
		if st.open() {
			return !searchable()
		}
		return focused.Get() && !disabled()
	})
	typed := typeaheadKeys(ahead.Get, func(r rune) {
		if core.Untracked(cx, st.open) {
			lb.typeAhead(r)
			return
		}
		from := -1
		if v := core.Untracked(cx, value); v != nil {
			from = slices.IndexFunc(lb.index.GetUntracked().items, func(it SelectItem) bool { return it.Value == *v })
		}
		if next := choosable(lb.typed(r, from)); next != nil {
			choose(next)
		}
	})
	hooks := listHooks{open: st.open, choose: pick, dismiss: dismiss,
		selected: func(v string) bool { c := current.Get(); return c != nil && *c == v },
		shown: func(shown bool) {
			if shown {
				if s.OnOpen != nil {
					s.OnOpen()
				}
				return
			}
			if s.OnClose != nil {
				s.OnClose()
			}
			// The keyboard goes back to the trigger, unless a press outside
			// took it somewhere else.
			if core.Untracked(cx, cx.Env().Desktop) && closedBy != ReasonPressOutside {
				anchor.Focus()
			}
			closedBy = 0
		}}
	list := listParts{
		frame: dropdownFrame{placement: s.Placement, align: cmpAlign(s.Align), offset: s.Offset, margin: s.Margin,
			popup:    parts.Of(s.PopupClass, s.PopupStyle, s.PopupStyleFunc),
			backdrop: parts.Part{Class: s.BackdropClass, Style: s.BackdropStyle}},
		list:   part{Class: s.ListClass, Style: s.ListStyle},
		item:   parts.Of(s.ItemClass, s.ItemStyle, s.ItemStyleFunc),
		group:  parts.Of(s.GroupClass, s.GroupStyle, s.GroupStyleFunc),
		search: &listSearch{class: s.SearchClass, style: s.SearchStyle, placeholder: s.SearchPlaceholder, label: s.SearchLabel, field: s.SearchField},
		cover:  newDropdownCover(cx), renderItem: s.RenderItem, renderGroup: s.RenderGroup, empty: s.Empty}
	measure := func() {
		if b, ok := anchor.ContentBounds(); ok {
			width.Set(b.Size.Width)
		}
	}
	trigger := core.View{Ref: anchor, Class: s.Class, Style: s.Style, StyleFunc: s.StyleFunc, Label: s.Label, TestID: s.TestID, Role: core.RoleCombobox,
		Disabled: s.Disabled, Focusable: func() bool { return !disabled() }, Expanded: st.open, ShowsExpanded: true,
		AccessibilityValue: func() core.AccessibilityValue {
			if v := current.Get(); v != nil {
				return selectedTitles(s.Items(), []string{*v})
			}
			return core.AccessibilityValue{}
		},
		OnLayout: func(core.Bounds) { measure() },
		OnFocus:  func() { focused.Set(true) }, OnBlur: func() { focused.Set(false) },
		OnPress: func(core.PressEvent) {
			how = openedByPress
			change(!core.Untracked(cx, st.open), ReasonTriggerPress)
		},
		Children: []core.Node{s.Trigger}}
	// A sibling of the trigger, so a press on it never opens the list.
	clear := core.Show{When: func() bool { return s.Cleanable && current.Get() != nil }, Then: func(*core.Scope) core.Node {
		return core.View{Role: core.RoleButton, Label: s.ClearButtonLabel, NoFocus: true, Disabled: s.Disabled,
			Class: s.ClearButtonClass, Style: s.ClearButtonStyle,
			OnPress:  func(core.PressEvent) { choose(nil) },
			Children: []core.Node{s.ClearButton}}
	}}
	return core.View{Class: s.RootClass, Style: s.RootStyle, OnLayout: s.OnLayout, Tooltip: s.Tooltip, Children: []core.Node{trigger, clear,
		lb.keys(st.open, pick), closedKeys, typed, lb.present(cx, anchor, width, true, list, hooks)}}
}

// ownValue is the value a select starts with, nil for none.
func ownValue(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// sameValue reports whether two values are the same, nil being none.
func sameValue(a, b *string) bool {
	return a == b || a != nil && b != nil && *a == *b
}

// cmpAlign is a dropdown's alignment: its trigger's start when 0.
func cmpAlign(a core.Align) core.Align {
	if a == 0 {
		return core.AlignStart
	}
	return a
}
