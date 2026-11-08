package nsgotest

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/internal/driver"
)

// defaultLongPress is how long a press is held to be a long one when the
// view's DelayLongPress does not say, as on every platform.
const defaultLongPress = 500 * time.Millisecond

func pressable(v *View) bool {
	return v.Events.Has(driver.EventPress) || v.Events.Has(driver.EventPressDown)
}

// pressTarget is the view a press on selector goes to: the match, or its
// nearest pressable ancestor (a text inside a button); a canvas node's
// press goes to the view under its middle. at is where it lands, in
// surface points, and index the byte offset of the characters a text:
// selector names in a text with pressable spans.
func (a *App) pressTarget(selector string) (v *View, at driver.Point, index int) {
	a.t.Helper()
	if m := a.Query(selector); len(m) == 1 && m[0].Node != nil {
		_, at = m[0].center()
		if a.target != nil {
			at = a.target.reveal(m[0])
		}
		v = a.pressableAt(at)
		if v == nil {
			a.t.Fatalf("nsgotest: nothing pressable under %q\n%s", selector, a.Dump())
		}
		if v.disabled(nil) {
			a.t.Fatalf("nsgotest: %q is disabled: a pressable view that takes no input\n%s", selector, a.Dump())
		}
		return v, at, 0
	}
	var match *View
	v = a.pick(selector, "pressable view", func(m *View) (*View, bool) {
		t := m.upTo(pressable)
		if t == nil {
			return nil, false
		}
		match = m
		return t, !m.disabled(t)
	})
	// The press lands on the match, as a finger on a text in a button
	// does.
	_, at = match.center()
	if a.target != nil {
		at = a.target.reveal(match)
	}
	if v == match && v.Kind == driver.KindText && len(v.Spans) > 0 {
		index = a.charsAt(selector, v)
	}
	return v, at, index
}

// charsAt returns the byte offset in text view v of the characters the
// last part of selector names, or 0.
func (a *App) charsAt(selector string, v *View) int {
	sel, err := parseSelector(selector)
	if err != nil {
		return 0
	}
	p := sel.parts[len(sel.parts)-1]
	switch {
	case p.kind != "text" && p.kind != "":
	case !p.text.matches(v.Text) && v.spanAt(p.text) >= 0:
		return v.spanAt(p.text)
	case p.text.re != nil:
		if loc := p.text.re.FindStringIndex(v.Text); loc != nil {
			return loc[0]
		}
	default:
		if i := strings.Index(v.Text, strings.TrimSpace(p.text.want)); i >= 0 {
			return i
		}
	}
	return 0
}

// Press presses what selector names, or the pressable view around it, as
// a tap does: a finger goes down and lifts on its middle, which the views
// listening to touches hear and presses outside the overlays it is not in
// report, while the view hears PressDown, PressUp and Press.
func (a *App) Press(selector string) {
	a.t.Helper()
	a.step("press(%s)", selector)
	v, at, index := a.pressTarget(selector)
	if a.target != nil {
		a.target.press(at, 0)
		a.done()
		return
	}
	a.countClick(v)
	a.touchAt(driver.GestureBegan, at, 0)
	a.pressOutside(v, at)
	// A pointer's press takes the keyboard focus off pressable views.
	for _, o := range a.views {
		if o.Focused && o.Kind != driver.KindInput {
			a.setFocus(o, false)
			a.Settle()
		}
	}
	for _, k := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
		ev := a.pressEvent(v, at)
		ev.Kind, ev.Index, ev.Count = k, index, a.clicks
		a.send(v, ev)
	}
	a.touchAt(driver.GestureEnded, at, 0)
	a.done()
}

// pressEvent is a press on v at the surface point at, one click with no
// modifiers.
func (a *App) pressEvent(v *View, at driver.Point) driver.Event {
	o := v.surfaceFrame().Origin
	return driver.Event{ID: v.ID, Point: driver.Point{X: at.X - o.X, Y: at.Y - o.Y}, Page: at, Count: 1}
}

// clickInterval is how soon a press follows the last one to count up.
const clickInterval = 500 * time.Millisecond

// countClick counts a press of v: a press of the view pressed last,
// within clickInterval of the clock, counts up, as platforms count double
// clicks.
func (a *App) countClick(v *View) {
	now := a.Clock.Now()
	if v.ID == a.lastPress && now.Sub(a.lastPressAt) <= clickInterval {
		a.clicks++
	} else {
		a.clicks = 1
	}
	a.lastPress, a.lastPressAt = v.ID, now
}

// pressOutside reports a press at at to the overlays that listen and do
// not hold v.
func (a *App) pressOutside(v *View, at driver.Point) {
	var outside []*View
	walk(a.views[a.rootID], func(o *View) {
		if o.Events.Has(driver.EventPressOutside) && !v.within(o) {
			outside = append(outside, o)
		}
	})
	for _, o := range outside {
		a.send(o, driver.Event{Kind: driver.EventPressOutside, Point: at})
	}
}

// touchAt reports finger id (0 without one) at the surface point at, as
// platforms report it while a view listens to touches; elapsed is the
// time since it went down.
func (a *App) touchAt(phase driver.GesturePhase, at driver.Point, elapsed time.Duration, id ...int) {
	if !a.listensToTouches() {
		return
	}
	a.dispatch(driver.Event{Kind: driver.EventTouch, Phase: phase, Time: elapsed,
		Touches: []driver.Touch{{ID: append(id, 0)[0], PagePosition: at, Changed: true}}})
}

func (a *App) listensToTouches() bool {
	found := false
	walk(a.views[a.rootID], func(v *View) {
		found = found || v.Events.Has(driver.EventTouch)
	})
	return found
}

// walk calls fn for v and its visible descendants.
func walk(v *View, fn func(*View)) {
	if v == nil || v.Style.Hidden {
		return
	}
	fn(v)
	for _, c := range v.Children {
		walk(c, fn)
	}
}

// pressableAt returns the topmost visible view at the surface point p
// that takes presses, following visible overflow until a view that clips
// stops it, as a pointer there finds it.
func (a *App) pressableAt(p driver.Point) *View {
	return a.viewAt(p, pressable)
}

// viewAt returns the topmost visible view at the surface point p that ok
// accepts, as a pointer there finds it.
func (a *App) viewAt(p driver.Point, ok func(*View) bool) *View {
	var found *View
	var visit func(v *View, origin driver.Point, clip *driver.Bounds)
	visit = func(v *View, origin driver.Point, clip *driver.Bounds) {
		if v.Style.Hidden || v.Node != nil {
			return
		}
		f := v.Frame
		f.Origin.X += origin.X
		f.Origin.Y += origin.Y
		in := p.X >= f.Origin.X && p.Y >= f.Origin.Y && p.X < f.Origin.X+f.Size.Width && p.Y < f.Origin.Y+f.Size.Height
		if clip != nil && !(p.X >= clip.Origin.X && p.Y >= clip.Origin.Y &&
			p.X < clip.Origin.X+clip.Size.Width && p.Y < clip.Origin.Y+clip.Size.Height) {
			return
		}
		pe := v.Props.PointerEvents
		if in && pe != driver.PointerEventsNone && pe != driver.PointerEventsBoxNone && ok(v) {
			found = v
		}
		if pe == driver.PointerEventsNone || pe == driver.PointerEventsBoxOnly {
			return
		}
		if v.Style.Clip {
			c := f
			clip = &c
		}
		inner := driver.Point{X: f.Origin.X - v.Offset.X, Y: f.Origin.Y - v.Offset.Y}
		for _, c := range v.Children {
			visit(c, inner, clip)
		}
	}
	if r := a.views[a.rootID]; r != nil {
		visit(r, driver.Point{}, nil)
	}
	return found
}

// PressDown starts a press on the view selector matches, or the pressable
// view around it, and holds it until PressUp.
func (a *App) PressDown(selector string) { a.t.Helper(); a.half(selector, driver.EventPressDown) }

// PressUp ends a press PressDown started.
func (a *App) PressUp(selector string) { a.t.Helper(); a.half(selector, driver.EventPressUp) }

func (a *App) half(selector string, kind driver.EventKind) {
	a.t.Helper()
	a.step("%s(%s)", map[driver.EventKind]string{driver.EventPressDown: "pressDown", driver.EventPressUp: "pressUp",
		driver.EventContextMenu: "rightClick"}[kind], selector)
	v := a.pick(selector, "view hearing it", func(m *View) (*View, bool) {
		t := m.upTo(func(v *View) bool { return v.Events.Has(kind) })
		return t, t != nil && (kind == driver.EventContextMenu || !m.disabled(t))
	})
	_, at := v.center()
	ev := a.pressEvent(v, at)
	ev.Kind = kind
	if kind == driver.EventContextMenu {
		ev.Count = 0
	}
	a.send(v, ev)
	a.done()
}

// LongPress holds a finger on what selector names, or the nearest view
// around it that hears long presses, presses or touches, for its
// DelayLongPress (500 ms by default) of the clock's time, as a long
// press does: PressDown, LongPress, then PressUp, and no Press.
func (a *App) LongPress(selector string) {
	a.t.Helper()
	a.step("longPress(%s)", selector)
	var match *View
	v := a.pick(selector, "pressable view", func(m *View) (*View, bool) {
		t := m.upTo(func(v *View) bool {
			return v.Events.Has(driver.EventLongPress) || pressable(v) || v.Events.Has(driver.EventTouch)
		})
		match = m
		return t, t != nil && !m.disabled(t)
	})
	_, at := v.center()
	index := 0
	if v == match && v.Kind == driver.KindText && len(v.Spans) > 0 {
		index = a.charsAt(selector, v)
	}
	if a.target != nil {
		at = a.target.reveal(v)
	}
	hold := v.Props.DelayLongPress
	if hold <= 0 {
		hold = defaultLongPress
	}
	if a.target != nil {
		// A finger rests as a person's does: past what a scroll view holds
		// its content's touches back, with the test's clock going for what
		// times the hold on the engine, as a context menu does.
		hold = max(hold, defaultLongPress) + 200*time.Millisecond
		a.target.pressWhile(at, hold, func() { a.target.advance(hold, true) })
		a.done()
		return
	}
	a.touchAt(driver.GestureBegan, at, 0)
	a.pressOutside(v, at)
	ev := a.pressEvent(v, at)
	ev.Kind, ev.Index = driver.EventPressDown, index
	a.send(v, ev)
	a.elapse(hold)
	ev.Kind = driver.EventLongPress
	a.send(v, ev)
	// A finger rests as long as a person's, past a shorter delay.
	a.elapse(defaultLongPress - hold)
	ev.Kind = driver.EventPressUp
	a.send(v, ev)
	a.touchAt(driver.GestureEnded, at, max(hold, defaultLongPress))
	a.done()
}

// RightClick sends a secondary click to the view selector matches, or the
// one around it that listens, as a desktop's mouse does.
func (a *App) RightClick(selector string) {
	a.t.Helper()
	if a.target != nil {
		a.step("rightClick(%s)", selector)
		v := a.pick(selector, "view hearing it", func(m *View) (*View, bool) {
			t := m.upTo(func(v *View) bool { return v.Events.Has(driver.EventContextMenu) })
			return t, t != nil
		})
		a.target.rightClick(a.target.reveal(v))
		a.done()
		return
	}
	a.half(selector, driver.EventContextMenu)
}

// inputStep starts a step on the field selector names, the match or the
// first field in it, and focuses it as a tap would; a field the user
// cannot edit fails the step.
func (a *App) inputStep(selector, step string) *View {
	a.t.Helper()
	a.step("%s(%s)", step, selector)
	isInput := func(v *View) bool { return v.Kind == driver.KindInput }
	v := a.pick(selector, "text field", func(m *View) (*View, bool) {
		t := m.downTo(isInput)
		return t, t != nil && !t.disabled(nil) && !t.Input.Readonly
	})
	if !v.Focused {
		if a.target != nil {
			a.target.press(a.target.reveal(v), 0)
			a.target.settle()
		} else {
			for _, o := range a.views {
				if o != v {
					a.setFocus(o, false)
				}
			}
			a.setFocus(v, true)
			a.Settle()
		}
	}
	return v
}

// Focus focuses the text field selector names, as a tap on it would.
func (a *App) Focus(selector string) {
	a.t.Helper()
	a.inputStep(selector, "focus")
	a.done()
}

// Type types text at the end of the field selector names (the match or
// the first field in it), as a keyboard does: the field takes the focus,
// then each character is a key (KeyPress) and an edit (ChangeText); a
// line break in a single-line field is the return key, as Submit. A
// character past MaxLength is dropped.
func (a *App) Type(selector, text string) {
	a.t.Helper()
	v := a.inputStep(selector, "type")
	if a.target != nil {
		a.target.typeText(v, text)
		a.done()
		return
	}
	for _, c := range text {
		if c == '\n' && !v.Input.Multiline {
			a.returnKey(v)
		} else {
			a.key(v, c)
		}
	}
	a.done()
}

// Clear empties the field selector names, as selecting all and deleting
// does: one KeyPress of Backspace and an edit to nothing.
func (a *App) Clear(selector string) {
	a.t.Helper()
	v := a.inputStep(selector, "clear")
	if a.target != nil {
		a.target.clear(v)
		a.done()
		return
	}
	a.send(v, driver.Event{Kind: driver.EventKeyPress, Text: "Backspace"})
	if v.Text != "" {
		a.edit(v, "")
	}
	a.done()
}

// Submit presses Return in the field selector names, as its
// SubmitBehavior says: SubmitEditing, after which the focus leaves a
// single-line field by default, or a line break in a multiline field.
func (a *App) Submit(selector string) {
	a.t.Helper()
	v := a.inputStep(selector, "submit")
	if a.target != nil {
		a.target.keys(v, "Enter")
		a.done()
		return
	}
	a.returnKey(v)
	a.done()
}

// KeyPress types key in the field selector names, as a key the user
// presses: its text, "Backspace" or "Enter". It reports the key, then
// edits the end of the text as the key would; Enter is the return key.
func (a *App) KeyPress(selector, key string) {
	a.t.Helper()
	v := a.inputStep(selector, "keyPress")
	if a.target != nil {
		a.target.keys(v, key)
		a.done()
		return
	}
	a.typeKey(v, key)
	a.done()
}

// typeKey is key in field v: "Enter", "Backspace" or what it enters.
func (a *App) typeKey(v *View, key string) {
	switch key {
	case "Enter":
		a.returnKey(v)
	case "Backspace":
		a.send(v, driver.Event{Kind: driver.EventKeyPress, Text: key})
		if r := []rune(v.Text); len(r) > 0 {
			a.edit(v, string(r[:len(r)-1]))
		}
	default:
		a.send(v, driver.Event{Kind: driver.EventKeyPress, Text: key})
		if add, ok := driver.LimitEdit(v.Text, 0, key, v.Input.MaxLength); ok && add != "" {
			a.edit(v, v.Text+add)
		}
	}
}

// key types c at the end of field v: a key, then the edit, within
// MaxLength.
func (a *App) key(v *View, c rune) {
	name := string(c)
	if c == '\n' {
		name = "Enter"
	}
	a.send(v, driver.Event{Kind: driver.EventKeyPress, Text: name})
	if max := v.Input.MaxLength; max > 0 && driver.UTF16Len(v.Text)+utf16Len(c) > max {
		return
	}
	a.edit(v, v.Text+string(c))
}

func utf16Len(c rune) int {
	if c >= 0x10000 {
		return 2
	}
	return 1
}

// returnKey is Return in field v, as its SubmitBehavior resolves.
func (a *App) returnKey(v *View) {
	switch v.Input.Submit() {
	case driver.SubmitBehaviorNewline:
		a.key(v, '\n')
		return
	case driver.SubmitBehaviorBlurAndSubmit:
		a.send(v, driver.Event{Kind: driver.EventKeyPress, Text: "Enter"})
		a.send(v, driver.Event{Kind: driver.EventSubmitEditing, Text: v.Text})
		a.setFocus(v, false)
		a.Settle()
		return
	}
	a.send(v, driver.Event{Kind: driver.EventKeyPress, Text: "Enter"})
	a.send(v, driver.Event{Kind: driver.EventSubmitEditing, Text: v.Text})
}

// edit sets field v's text as a user's edit, the caret at its end.
func (a *App) edit(v *View, text string) {
	a.edits[v.ID]++
	v.Text = text
	a.dispatch(driver.Event{Kind: driver.EventChangeText, ID: v.ID, Text: text, Count: a.edits[v.ID]})
	end := driver.UTF16Len(text)
	a.moveSelection(v, driver.TextRange{Start: end, End: end})
	if v.Events.Has(driver.EventCaretMove) && !a.noCaretMoves {
		at, size := v.caret()
		a.dispatch(driver.Event{Kind: driver.EventCaretMove, ID: v.ID, Point: at, Size: size})
	}
}

// caret is where field v's caret is at the end of its text, in the
// field's view, as the approximate text engine lays it out.
func (v *View) caret() (driver.Point, driver.Size) {
	size := v.Input.Style.Font.Size
	if size == 0 {
		size = 14
	}
	line := v.Input.Style.LineHeight
	if line == 0 {
		line = size * 1.2
	}
	lines := strings.Split(v.Text, "\n")
	last := lines[len(lines)-1]
	c := v.Input.Content
	return driver.Point{X: c.Origin.X + driver.Points(utf8.RuneCountInString(last))*size*0.6,
		Y: c.Origin.Y + driver.Points(len(lines)-1)*line}, driver.Size{Width: 1, Height: line}
}

// Paste pastes into the field selector names as the user would: the
// field hears it with OnPaste, then, unless its PasteBehavior is
// PasteReport, the text goes in at its end.
func (a *App) Paste(selector string, p driver.PasteEvent) {
	a.t.Helper()
	v := a.inputStep(selector, "paste")
	if v.Events.Has(driver.EventPaste) {
		a.dispatch(driver.Event{Kind: driver.EventPaste, ID: v.ID, Data: &p})
	}
	if p.Text != "" && v.Input.PasteBehavior == driver.PasteInsert {
		text := p.Text
		if !v.Input.Multiline {
			text = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(text)
		}
		a.edit(v, driver.ClipUTF16(v.Text+text, v.Input.MaxLength))
	}
	a.done()
}

// Select moves the selection of the field selector names, as the user
// would, in UTF-16 units.
func (a *App) Select(selector string, start, end int) {
	a.t.Helper()
	v := a.inputStep(selector, "select")
	a.moveSelection(v, driver.TextRange{Start: start, End: end})
	a.done()
}

func (a *App) moveSelection(v *View, r driver.TextRange) {
	if r == v.Selection {
		return
	}
	v.Selection = r
	if v.Events.Has(driver.EventSelectionChange) {
		a.dispatch(driver.Event{Kind: driver.EventSelectionChange, ID: v.ID, Index: r.Start, Count: uint32(r.End)})
	}
}

// Compose has an input method compose text in the field selector names;
// "" ends the composition, after the Type of what it commits, or Commit
// commits it. While it lasts the field holds the values the app sets, as
// platforms do. Under nsgo test -target it goes through the platform's
// text input client, as an input method's does.
func (a *App) Compose(selector, text string) {
	a.t.Helper()
	if a.target != nil {
		a.step("compose(%s, %q)", selector, text)
		a.Find(selector)
		a.target.compose(text)
		a.done()
		return
	}
	v := a.Find(selector)
	if text != "" {
		v.Composing = text
		if v.Events.Has(driver.EventCompositionChange) {
			a.dispatch(driver.Event{Kind: driver.EventCompositionChange, ID: v.ID, Text: text})
		}
		return
	}
	v.Composing = ""
	if v.Events.Has(driver.EventCompositionChange) {
		a.dispatch(driver.Event{Kind: driver.EventCompositionChange, ID: v.ID, Finished: true})
	}
	if it := v.held; it != nil {
		v.held = nil
		if a.edits[v.ID] <= it.Count {
			v.Text = it.Value
		}
	}
}

// Commit has the input method composing in the field selector names
// commit text, ending its composition: the Type of text, then the
// composition's end, or the platform's commit under nsgo test -target.
func (a *App) Commit(selector, text string) {
	a.t.Helper()
	if a.target != nil {
		a.step("commit(%s, %q)", selector, text)
		a.Find(selector)
		a.target.commit(text)
		a.done()
		return
	}
	a.Type(selector, text)
	a.Compose(selector, "")
}

// Key presses a key on a hardware keyboard, as the app's keymap reads it
// ("mod+s", "Escape", "a"): a binding of the keymap fires; else, in a
// macOS or web environment, Tab and Shift+Tab move the keyboard focus
// between fields and pressable views, and Return or Space press the
// pressable view it is on; else the focused field takes a character,
// Backspace or Enter; else Escape goes back. A key nothing takes does
// nothing, as on a device.
func (a *App) Key(key string) {
	a.t.Helper()
	a.step("key(%s)", key)
	if a.target != nil {
		a.target.keys(nil, key)
		a.done()
		return
	}
	k := driver.NormalizeKey(key)
	names := a.keyNames(k)
	for _, b := range a.Keymap {
		if slices.Contains(names, b) {
			a.dispatch(driver.Event{Kind: driver.EventKey, Text: b})
			a.done()
			return
		}
	}
	if a.tabs() {
		switch k {
		case "tab", "shift+tab":
			a.moveFocus(k == "shift+tab")
			a.done()
			return
		case "enter", "space":
			// Return or Space press the pressable the keyboard focus is on.
			for _, v := range a.shown() {
				if v.Focused && v.Kind != driver.KindInput && pressable(v) {
					for _, kind := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
						a.send(v, driver.Event{Kind: kind})
					}
					a.done()
					return
				}
			}
		}
	}
	// The focused field takes the keys that edit it, a character shifted
	// as the US layout shifts it.
	name, shift := k, false
	if rest, ok := strings.CutPrefix(k, "shift+"); ok {
		name, shift = rest, true
	}
	for _, v := range a.views {
		if !v.Focused || v.Kind != driver.KindInput || v.Input.Readonly || v.Input.Disabled || strings.Contains(name, "+") {
			continue
		}
		typed := key[len(key)-len(name):]
		switch {
		case name == "enter" && !shift:
			typed = "Enter"
		case name == "backspace" && !shift:
			typed = "Backspace"
		case name == "space":
			typed = " "
		case utf8.RuneCountInString(typed) != 1:
			continue
		case shift:
			typed = shifted(typed)
		}
		a.typeKey(v, typed)
		a.done()
		return
	}
	if k == "escape" {
		a.dispatch(driver.Event{Kind: driver.EventBack, ID: a.rootID})
	}
	a.done()
}

// shifted is what the key typing c types with Shift on the US layout.
func shifted(c string) string {
	const plain, shift = "`1234567890-=[]\\;',./", "~!@#$%^&*()_+{}|:\"<>?"
	if i := strings.Index(plain, c); i >= 0 && len(c) == 1 {
		return shift[i : i+1]
	}
	return strings.ToUpper(c)
}

// keyNames are the names a press of normalized key k matches bindings
// by, as a platform's: "mod" is Command on Apple platforms and Control
// elsewhere, whichever of the two the key names.
func (a *App) keyNames(k string) []string {
	cmd := "ctrl"
	if a.env != nil && (a.env.OS == driver.OSMacOS || a.env.OS == driver.OSIOS) {
		cmd = "meta"
	}
	parts := strings.Split(k, "+")
	mods, name := parts[:len(parts)-1], parts[len(parts)-1]
	swap := func(from, to string) string {
		out := []string{}
		for _, m := range mods {
			if m == from {
				m = to
			}
			out = append(out, m)
		}
		return driver.NormalizeKey(strings.Join(append(out, name), "+"))
	}
	switch {
	case slices.Contains(mods, "mod"):
		return []string{k, swap("mod", cmd)}
	case slices.Contains(mods, cmd):
		return []string{k, swap(cmd, "mod")}
	}
	return []string{k}
}

// tabs says whether Tab moves the keyboard focus between pressable views
// and fields, as on macOS and the web.
func (a *App) tabs() bool {
	return a.env != nil && (a.env.OS == driver.OSMacOS || a.env.OS == driver.OSWeb)
}

// moveFocus moves the keyboard focus to the next Tab stop, or the
// previous one, around: editable fields and the pressable views not
// NoFocus, disabled or out of sight and the Focusable ones, in tree
// order.
func (a *App) moveFocus(back bool) {
	var stops []*View
	at := -1
	for _, v := range a.shown() {
		stop := v.Kind == driver.KindInput && !v.Input.Readonly ||
			v.Kind != driver.KindInput && (pressable(v) || v.Props.Focusable) && !v.Props.NoFocus
		if !stop || v.disabled(nil) {
			continue
		}
		if v.Focused {
			at = len(stops)
		}
		stops = append(stops, v)
	}
	if len(stops) == 0 {
		return
	}
	next := 0
	switch {
	case at >= 0 && back:
		next = (at + len(stops) - 1) % len(stops)
	case at >= 0:
		next = (at + 1) % len(stops)
	case back:
		next = len(stops) - 1
	}
	for _, v := range a.views {
		if v != stops[next] {
			a.setFocus(v, false)
		}
	}
	a.setFocus(stops[next], true)
}

// Back goes back as Escape, the back button or the back gesture does.
func (a *App) Back() {
	a.t.Helper()
	a.step("back")
	if a.target != nil {
		a.target.back()
		a.done()
		return
	}
	a.dispatch(driver.Event{Kind: driver.EventBack, ID: a.rootID})
	a.done()
}
