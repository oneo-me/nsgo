package base

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// Combobox is a text input over the same list as Select's: typing opens
// the list and filters it, and a press chooses. It holds one value that a
// choice replaces, closing the list, or with Multiple several that a choice
// toggles, leaving it open. OnConfirm reports the value on every close;
// Mod+Return closes it as it is. It has no look.
type Combobox struct {
	Items        func() []SelectItem
	Multiple     bool
	Value        func() []string
	DefaultValue []string
	Open         func() bool
	DefaultOpen  bool
	Disabled     func() bool
	// Query makes the typed text the app's, nil for the field's own, which
	// each opening starts at DefaultQuery.
	Query        func() *string
	DefaultQuery string
	Placeholder  string
	// Cleanable shows a button while a value is chosen, which clears it.
	Cleanable bool

	Placement core.Placement
	// Align lines the list up with its box, at its start when 0.
	Align          core.Align
	Offset, Margin core.Points

	// Class and Style are the box's, around the input and its buttons.
	Class      string
	Style      core.Style
	StyleFunc  func() core.Style
	InputClass string
	InputStyle core.Style
	// Field draws the input, as a styled form does.
	Field          func(core.Input) core.Node
	PopupClass     string
	PopupStyle     core.Style
	PopupStyleFunc func() core.Style
	BackdropClass  string
	BackdropStyle  core.Style
	ListClass      string
	ListStyle      core.Style
	ItemClass      string
	ItemStyle      core.Style
	// ItemStyleFunc is each row's style following what it reads.
	ItemStyleFunc  func() core.Style
	GroupClass     string
	GroupStyle     core.Style
	GroupStyleFunc func() core.Style
	FooterClass    string
	FooterStyle    core.Style

	ClearButtonClass string
	ClearButtonStyle core.Style
	ClearButton      core.Node
	ClearButtonLabel string

	// Prefix and Suffix go before the input and after its buttons.
	Prefix, Suffix core.Node
	// TriggerButton opens and closes the list, hidden from screen readers.
	TriggerButton      core.Node
	TriggerButtonClass string
	TriggerButtonStyle core.Style
	Empty, Footer      core.Node
	RenderItem         func(SelectEntry) core.Node
	RenderGroup        func(SelectGroup) core.Node

	OnChange      func(value []string)
	OnConfirm     func(value []string)
	OnQueryChange func(query string)
	OnOpenChange  func(open bool, reason Reason)
	OnOpen        func()
	OnClose       func()
	OnCancel      func()
	Label         string
	TestID        string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (c Combobox) Render(cx *core.Scope) core.Node {
	st := newPopup(cx, c.Open, c.DefaultOpen, nil, nil, nil)
	own := core.NewSignalFunc(cx, slices.Clone(c.DefaultValue), slices.Equal[[]string])
	value := func() []string {
		if c.Value != nil {
			return c.Value()
		}
		return own.Get()
	}
	current := core.NewMemoFunc(cx, value, slices.Equal[[]string])
	// The value as the user last left it: what toggles start from and what
	// closing confirms, ahead of an app's that follows a turn later.
	var latest []string
	cx.Effect(func() { latest = current.Get() })
	searching := func() bool { return true }
	lb := newListbox(cx, c.Items, searching, listQuery{controlled: c.Query, def: func() string { return c.DefaultQuery }})
	anchor, field := new(core.Ref), new(core.Ref)
	// typed: something was typed since the list opened, so the input shows
	// the query, else one value's title. pending is what was typed while
	// closed, and replaced the chosen title the first characters followed.
	typed := core.NewSignal(cx, false)
	var pending, replaced *string
	width := core.NewSignal[core.Points](cx, -1)
	focused := core.NewSignal(cx, false)
	how, closedBy := openedByPress, Reason(0)
	disabled := func() bool { return is(c.Disabled) }
	change := func(next bool, r Reason) {
		if next && core.Untracked(cx, disabled) || core.Untracked(cx, st.open) == next {
			return
		}
		if !next {
			closedBy = r
		}
		if c.OnOpenChange != nil {
			c.OnOpenChange(next, r)
		}
		if st.own != nil {
			st.own.Set(next)
		}
	}
	report := func(next []string) {
		if slices.Equal(latest, next) {
			return
		}
		latest = next
		if c.Value == nil {
			own.Set(next)
		}
		if c.OnChange != nil {
			c.OnChange(next)
		}
	}
	pick := func(item int) {
		items := lb.index.GetUntracked().items
		if item < 0 || item >= len(items) || items[item].Disabled {
			return
		}
		chosen := items[item].Value
		if !c.Multiple {
			report([]string{chosen})
			change(false, ReasonSelect)
			return
		}
		next := slices.Clone(latest)
		if at := slices.Index(next, chosen); at >= 0 {
			next = slices.Delete(next, at, at+1)
		} else {
			next = append(next, chosen)
		}
		report(next)
	}
	dismiss := func(r Reason) {
		if c.OnCancel != nil {
			c.OnCancel()
		}
		change(false, r)
	}
	// Each opening starts on the first selected item shown, else as it was
	// asked for; each close confirms the value.
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
				typed.Set(false)
				replaced = nil
				if c.OnConfirm != nil {
					c.OnConfirm(latest)
				}
				return
			}
			asked := how
			how = openedByPress
			selected := current.GetUntracked()
			lb.opened(func(shown *listShown, items []SelectItem) int {
				for i, it := range items {
					if !it.Disabled && slices.Contains(selected, it.Value) && shown.rowOf(i) >= 0 {
						return i
					}
				}
				return asked.initial(shown)
			})
			// Typing opened it: the query is what was typed.
			typed.Set(false)
			if pending != nil {
				text := *pending
				pending = nil
				typed.Set(true)
				lb.edited(text)
			}
		})
	})
	opener := func(o opening) func() {
		return func() {
			how = o
			change(true, ReasonTriggerPress)
		}
	}
	// The input's arrows open the list; Return and Space are the input's.
	closedKeys := bindWhile(func() bool { return focused.Get() && !st.open() && !disabled() }, core.KeyAlways, false,
		controlKey{key: "down", run: opener(openedByDown)}, controlKey{key: "up", run: opener(openedByUp)})
	confirmKey := bindWhile(st.open, core.KeyAlways, false, controlKey{key: "mod+enter", run: func() { change(false, ReasonSelect) }})
	hooks := listHooks{open: st.open, choose: pick, dismiss: dismiss,
		selected: func(v string) bool { return slices.Contains(current.Get(), v) },
		shown: func(shown bool) {
			if shown {
				if c.OnOpen != nil {
					c.OnOpen()
				}
				return
			}
			if c.OnClose != nil {
				c.OnClose()
			}
			if core.Untracked(cx, cx.Env().Desktop) && closedBy != ReasonPressOutside {
				field.Focus()
			}
			closedBy = 0
		}}
	list := listParts{
		frame: dropdownFrame{placement: c.Placement, align: cmpAlign(c.Align), offset: c.Offset, margin: c.Margin,
			popup:    parts.Of(c.PopupClass, c.PopupStyle, c.PopupStyleFunc),
			backdrop: parts.Part{Class: c.BackdropClass, Style: c.BackdropStyle}},
		list: part{Class: c.ListClass, Style: c.ListStyle}, item: parts.Of(c.ItemClass, c.ItemStyle, c.ItemStyleFunc),
		group: parts.Of(c.GroupClass, c.GroupStyle, c.GroupStyleFunc), renderItem: c.RenderItem, renderGroup: c.RenderGroup, empty: c.Empty,
		footer: &listFooter{content: c.Footer, class: c.FooterClass, style: c.FooterStyle}}
	chosenTitle := func() string {
		v := current.Get()
		if len(v) == 0 {
			return ""
		}
		for _, it := range c.Items() {
			if it.Value == v[0] {
				return it.Title
			}
		}
		return ""
	}
	text := core.Input{Ref: field, Placeholder: c.Placeholder, NoAutoCorrect: true, ReadonlyFunc: disabled, Disabled: c.Disabled,
		Role: core.RoleCombobox, Label: c.Label, Expanded: st.open, Class: c.InputClass, Style: c.InputStyle,
		Value: func() string {
			if typed.Get() || c.Multiple {
				return lb.queryNow()
			}
			return chosenTitle()
		},
		AccessibilityValue: func() core.AccessibilityValue { return selectedTitles(c.Items(), current.Get()) },
		OnFocus: func() {
			focused.Set(true)
			// A tap on the input opens the list on a touch screen; with a
			// keyboard typing or the arrow keys do.
			if !core.Untracked(cx, cx.Env().Desktop) && !core.Untracked(cx, st.open) && !core.Untracked(cx, disabled) {
				how = openedByPress
				change(true, ReasonTriggerPress)
			}
			// The chosen title, selected: typing replaces it.
			if !c.Multiple && !typed.GetUntracked() {
				if title := core.Untracked(cx, chosenTitle); title != "" {
					field.SetSelection(0, utf8.RuneCountInString(title))
				}
			}
		},
		OnBlur: func() { focused.Set(false) },
		OnChange: func(text string) {
			// Typed after the chosen title, the caret at its end: what was
			// typed replaces it, as it would the selection.
			title := ""
			switch {
			case c.Multiple:
			case typed.GetUntracked():
				if replaced != nil {
					title = *replaced
				}
			default:
				title = core.Untracked(cx, chosenTitle)
			}
			if rest, ok := strings.CutPrefix(text, title); ok && title != "" && rest != "" {
				replaced = &title
				text = rest
			} else {
				replaced = nil
			}
			typed.Set(true)
			if core.Untracked(cx, st.open) {
				lb.edited(text)
			} else {
				pending = &text
				how = openedByPress
				change(true, ReasonInputChange)
			}
			if c.OnQueryChange != nil {
				c.OnQueryChange(text)
			}
		}}
	children := []core.Node{}
	if c.Prefix != nil {
		children = append(children, c.Prefix)
	}
	children = append(children, drawnField(c.Field, text), core.Show{When: func() bool { return c.Cleanable && len(current.Get()) > 0 }, Then: func(*core.Scope) core.Node {
		return core.View{Role: core.RoleButton, Label: c.ClearButtonLabel, NoFocus: true, Disabled: c.Disabled,
			Class: c.ClearButtonClass, Style: c.ClearButtonStyle,
			OnPress: func(core.PressEvent) {
				report(nil)
				field.Focus()
			},
			Children: []core.Node{c.ClearButton}}
	}})
	if c.TriggerButton != nil {
		// The input says whether the list is open: the button is hidden
		// from screen readers and leaves the keyboard on the input.
		children = append(children, core.View{Role: core.RoleButton, NoFocus: true, Disabled: c.Disabled,
			Class: c.TriggerButtonClass, Style: c.TriggerButtonStyle,
			AccessibilityElementsHidden: func() bool { return true },
			OnPress: func(core.PressEvent) {
				how = openedByPress
				change(!core.Untracked(cx, st.open), ReasonTriggerPress)
				if core.Untracked(cx, cx.Env().Desktop) {
					field.Focus()
				}
			},
			Children: []core.Node{c.TriggerButton}})
	}
	if c.Suffix != nil {
		children = append(children, c.Suffix)
	}
	children = append(children, lb.keys(st.open, pick), closedKeys, confirmKey, lb.present(cx, anchor, width, false, list, hooks))
	return core.View{Ref: anchor, Class: c.Class, Style: core.Style{Direction: core.Row}.Merge(c.Style), StyleFunc: c.StyleFunc,
		TestID: c.TestID, Tooltip: c.Tooltip,
		OnLayout: func(b core.Bounds) {
			width.Set(b.Size.Width)
			if c.OnLayout != nil {
				c.OnLayout(b)
			}
		}, Children: children}
}
