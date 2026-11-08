package richeditor

import (
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/core"
)

const (
	edgeTick         = 16 * time.Millisecond
	edgeZone         = 16.0
	edgeMin, edgeMax = 12.0, 64.0
	edgeRamp         = 80.0
)

// pressState is a drag's: where it started and by what unit.
type pressState struct {
	dragging bool
	unit     int
	anchor   int
	// from and to are what the press selected, for a drag by words or
	// blocks.
	from, to int
	last     core.Point
	// mod held, a drag across blocks covers them whole on release.
	mod bool
}

// sinkView is the invisible input at the caret that takes typing and input
// methods, as the code Editor's.
func (v *rv) sinkView() core.Node {
	clear := core.RGBA(0xffffff, 0)
	return core.Input{Ref: v.sinkRef, Label: v.p.Label, AutoFocus: v.p.AutoFocus, Value: v.sink.Get, HideCaret: true,
		PasteBehavior: core.PasteReport, SubmitBehavior: core.SubmitBehaviorSubmit,
		Readonly: !v.s.Editable(),
		OnPaste:  v.pasted, OnChange: v.sinkChanged,
		OnSubmit: func(string) {
			v.clearSink()
			v.s.HandleKey("Enter")
		},
		OnKeyPress: func(key string) {
			if key == "Backspace" && v.sink.GetUntracked() == "" {
				v.s.HandleKey("Backspace")
			}
		},
		OnCompositionChange: func(c core.Composition) {
			if c.Composing {
				v.setComposing(c.Text)
				return
			}
			v.setComposing("")
			v.flushSink(true)
		},
		OnFocus: func() {
			v.focused.Set(true)
			if v.p.OnFocus != nil {
				v.p.OnFocus()
			}
		},
		OnBlur: func() {
			v.focused.Set(false)
			if v.p.OnBlur != nil {
				v.p.OnBlur()
			}
		},
		StyleFunc: func() core.Style {
			v.rev.Get()
			v.geo.Get()
			r, _ := v.CoordsAtPos(v.s.State().Selection.Head())
			y := core.Points(r.Y) - v.scrollY.Get()
			return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(r.X)), Top: core.Pt(max(y, 0)), Width: core.Pt(1), Height: core.Pt(core.Points(max(r.Height, 1))),
				Color: clear, Background: clear, SelectionColor: clear, Padding: core.Edges{}, BorderWidth: 0}
		}}
}

func (v *rv) pasted(p core.PasteEvent) {
	if len(p.Files) > 0 || len(p.URIs) > 0 {
		if v.p.OnPasteFiles != nil {
			paths := append([]string(nil), p.URIs...)
			for _, f := range p.Files {
				paths = append(paths, f.Name)
			}
			v.p.OnPasteFiles(paths)
		}
		return
	}
	if v.s.Editable() && (p.Text != "" || p.HTML != "") {
		v.s.Paste(p.Text, p.HTML)
	}
}

func (v *rv) setComposing(text string) {
	if v.composing.GetUntracked() == "" && text != "" {
		v.compID++
		v.s.OpenGap()
	}
	v.composing.Set(text)
	if b := v.textblockAt(v.s.State().Selection.From()); b != nil {
		v.buildText(b)
	}
}

func (v *rv) sinkChanged(text string) {
	v.sink.Set(text)
	if v.composing.GetUntracked() == "" {
		v.flushSink(false)
	}
}

// flushSink turns what the sink holds beyond what it held into typing: the
// commit of a composition is one transaction, tagged for the history.
func (v *rv) flushSink(composed bool) {
	text, held := v.sink.GetUntracked(), v.typed
	common := 0
	for common < len(held) && common < len(text) && held[common] == text[common] {
		common++
	}
	for common > 0 && common < len(held) && !utf8.RuneStart(held[common]) {
		common--
	}
	for range utf8.RuneCountInString(held[common:]) {
		v.s.HandleKey("Backspace")
	}
	if added := text[common:]; added != "" && v.s.Editable() {
		var comp any
		if composed {
			comp = v.compID
		}
		v.s.TypeText(added, comp)
	}
	v.typed = text
	if len(text) > 256 {
		v.typed = ""
		v.sink.Set("")
	}
}

func (v *rv) clearSink() {
	if v.composing.GetUntracked() == "" && v.sink.GetUntracked() != "" {
		v.typed = ""
		v.sink.Set("")
	}
}

// keyChars are what the modifiers combine with.
const keyChars = "abcdefghijklmnopqrstuvwxyz0123456789,./;'[]\\-=`"

// keys binds, while it has the keyboard, every key a keymap could use, each
// offered to the session first.
func (v *rv) keys(cx *core.Scope) {
	apple := core.Untracked(cx, cx.Env().AppleKeys)
	bind := func(key, name string, def func()) {
		cx.OnKeyWith(key, core.KeyOptions{When: core.KeyAlways, Repeat: true}, func() {
			v.clearSink()
			if !v.s.HandleKey(name) && def != nil {
				def()
			}
		})
	}
	for _, c := range keyChars {
		ch := string(c)
		switch ch {
		case "c", "x", "v":
			bind("mod+shift+"+ch, "Mod-Shift-"+ch, nil)
			bind("mod+alt+"+ch, "Alt-Mod-"+ch, nil)
			continue
		}
		bind("mod+"+ch, "Mod-"+ch, nil)
		bind("mod+shift+"+ch, "Mod-Shift-"+ch, nil)
		bind("mod+alt+"+ch, "Alt-Mod-"+ch, nil)
	}
	cx.OnKeyWith("mod+c", core.KeyOptions{When: core.KeyAlways}, v.copy)
	cx.OnKeyWith("mod+x", core.KeyOptions{When: core.KeyAlways}, func() {
		if !v.s.Editable() {
			v.copy()
			return
		}
		text, _ := v.s.Cut()
		if text != "" {
			v.cx.SetClipboard(text)
		}
	})
	word := "ctrl"
	wordName := "Mod"
	if apple {
		word, wordName = "alt", "Alt"
	}
	move := func(dir int, extend bool) func() { return func() { v.s.MoveChar(dir, extend) } }
	bind("left", "ArrowLeft", move(-1, false))
	bind("right", "ArrowRight", move(1, false))
	bind("shift+left", "Shift-ArrowLeft", move(-1, true))
	bind("shift+right", "Shift-ArrowRight", move(1, true))
	bind(word+"+left", wordName+"-ArrowLeft", func() { v.s.MoveWord(-1, false) })
	bind(word+"+right", wordName+"-ArrowRight", func() { v.s.MoveWord(1, false) })
	bind(word+"+shift+left", wordName+"-Shift-ArrowLeft", func() { v.s.MoveWord(-1, true) })
	bind(word+"+shift+right", wordName+"-Shift-ArrowRight", func() { v.s.MoveWord(1, true) })
	line := func(dir int, extend bool) func() { return func() { v.moveLine(dir, extend) } }
	bind("up", "ArrowUp", line(-1, false))
	bind("down", "ArrowDown", line(1, false))
	bind("shift+up", "Shift-ArrowUp", line(-1, true))
	bind("shift+down", "Shift-ArrowDown", line(1, true))
	ends := func(dir int, extend bool) func() { return func() { v.lineEnd(dir, extend) } }
	bind("home", "Home", ends(-1, false))
	bind("end", "End", ends(1, false))
	bind("shift+home", "Shift-Home", ends(-1, true))
	bind("shift+end", "Shift-End", ends(1, true))
	if apple {
		bind("mod+left", "Mod-ArrowLeft", ends(-1, false))
		bind("mod+right", "Mod-ArrowRight", ends(1, false))
		bind("mod+shift+left", "Mod-Shift-ArrowLeft", ends(-1, true))
		bind("mod+shift+right", "Mod-Shift-ArrowRight", ends(1, true))
	}
	doc := func(dir int, extend bool) func() { return func() { v.docEnd(dir, extend) } }
	first, last := "mod+up", "mod+down"
	bind(first, "Mod-ArrowUp", doc(-1, false))
	bind(last, "Mod-ArrowDown", doc(1, false))
	bind("mod+shift+up", "Mod-Shift-ArrowUp", doc(-1, true))
	bind("mod+shift+down", "Mod-Shift-ArrowDown", doc(1, true))
	page := func(dir int, extend bool) func() { return func() { v.movePage(dir, extend) } }
	bind("pageup", "PageUp", page(-1, false))
	bind("pagedown", "PageDown", page(1, false))
	bind("shift+pageup", "Shift-PageUp", page(-1, true))
	bind("shift+pagedown", "Shift-PageDown", page(1, true))
	bind("enter", "Enter", nil)
	bind("shift+enter", "Shift-Enter", nil)
	bind("mod+enter", "Mod-Enter", nil)
	bind("backspace", "Backspace", nil)
	bind("shift+backspace", "Shift-Backspace", nil)
	bind("delete", "Delete", nil)
	bind(word+"+backspace", wordName+"-Backspace", func() { v.deleteWord(-1) })
	bind(word+"+delete", wordName+"-Delete", func() { v.deleteWord(1) })
	if apple {
		bind("mod+backspace", "Mod-Backspace", func() { v.deleteToLineStart() })
	}
	bind("tab", "Tab", nil)
	bind("shift+tab", "Shift-Tab", nil)
	bind("escape", "Escape", nil)
}

func (v *rv) copy() {
	if v.s.State().Selection.Empty() {
		return
	}
	text, _ := v.s.Copy()
	v.cx.SetClipboard(text)
}

func (v *rv) deleteWord(dir int) {
	before := v.s.State().Selection
	if !before.Empty() {
		v.s.Run(DeleteSelection)
		return
	}
	v.s.MoveWord(dir, true)
	v.s.Run(DeleteSelection)
}

func (v *rv) deleteToLineStart() {
	if !v.s.State().Selection.Empty() {
		v.s.Run(DeleteSelection)
		return
	}
	v.lineEnd(-1, true)
	v.s.Run(DeleteSelection)
}

// moveLine goes to the line above or below at the goal x.
func (v *rv) moveLine(dir int, extend bool) {
	st := v.s.State()
	sel := st.Selection
	if !extend {
		if _, cell := sel.(*CellSelection); cell || !sel.Empty() {
			at := sel.From()
			if dir > 0 {
				at = sel.To()
			}
			v.s.setHead(at, false, dir)
			return
		}
	}
	r, ok := v.CoordsAtPos(sel.Head())
	if !ok {
		return
	}
	if !v.hasGoal {
		v.goalX, v.hasGoal = r.X, true
	}
	y := r.Y - 1
	if dir > 0 {
		y = r.Y + r.Height + 1
	}
	pos, ok := v.PosAtCoords(v.goalX, y)
	if !ok {
		return
	}
	if pos == sel.Head() {
		pos = 0
		if dir > 0 {
			pos = st.Doc.Content.Size
		}
		v.docEnd(dir, extend)
		return
	}
	goal, had := v.goalX, v.hasGoal
	v.s.setHead(pos, extend, dir)
	v.goalX, v.hasGoal = goal, had
}

func (v *rv) movePage(dir int, extend bool) {
	r, ok := v.CoordsAtPos(v.s.State().Selection.Head())
	if !ok {
		return
	}
	y := r.Y + float64(dir)*float64(v.viewport.GetUntracked().Height)
	if pos, ok := v.PosAtCoords(r.X, y); ok {
		v.s.setHead(pos, extend, dir)
	}
}

// lineEnd goes to the visual line's start or end.
func (v *rv) lineEnd(dir int, extend bool) {
	head := v.s.State().Selection.Head()
	b := v.textblockAt(head)
	if b == nil {
		return
	}
	r, ok := v.CoordsAtPos(head)
	if !ok {
		return
	}
	frame, _ := v.boxOf(b)
	x := frame.X - 1
	if dir > 0 {
		x = frame.X + frame.Width + 1
	}
	if pos, ok := v.PosAtCoords(x, r.Y+r.Height/2); ok {
		v.s.setHead(pos, extend, dir)
	}
}

func (v *rv) docEnd(dir int, extend bool) {
	doc := v.s.State().Doc
	at := SelectionAtStart(doc).From()
	if dir > 0 {
		at = SelectionAtEnd(doc).To()
	}
	v.s.setHead(at, extend, dir)
}

// wordAround is the word at pos in its textblock.
func wordAround(r *ResolvedPos) (int, int) {
	parent := r.Parent()
	text := []rune(parent.TextBetween(0, parent.Content.Size, "", "￼"))
	at := len([]rune(parent.TextBetween(0, r.ParentOffset, "", "￼")))
	word := func(c rune) bool { return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' }
	a, z := at, at
	switch {
	case at < len(text) && word(text[at]), at > 0 && word(text[at-1]):
		for a > 0 && word(text[a-1]) {
			a--
		}
		for z < len(text) && word(text[z]) {
			z++
		}
	case at < len(text):
		z++
	}
	start := r.Start(r.Depth)
	return start + utf16Len(string(text[:a])), start + utf16Len(string(text[:z]))
}

func (v *rv) pressDown(pt core.Point, clicks int, mods core.Modifiers) {
	v.sinkRef.Focus()
	x, y := float64(pt.X), float64(pt.Y)
	pos, ok := v.PosAtCoords(x, y)
	if !ok {
		return
	}
	mod := mods.Control
	if core.Untracked(v.cx, v.cx.Env().AppleKeys) {
		mod = mods.Platform
	}
	// A press between blocks offers the plugins the gap's position.
	at := pos
	if gap, ok := v.gapAt(x, y); ok && !mods.Shift {
		at = gap
	}
	st := v.s.State()
	for _, p := range st.Plugins {
		if p.HandleClick != nil && p.HandleClick(v.s, at, clicks, mod) {
			return
		}
	}
	if l := v.leafAt(x, y); l != nil && isNodeSelectable(l.nowNode) {
		v.s.Dispatch(st.Tr().SetSelection(NodeSelectionAt(st.Doc, l.pos)))
		return
	}
	r := st.Doc.Resolve(pos)
	v.press = pressState{unit: max(min(clicks, 3), 1), anchor: pos, from: pos, to: pos, last: pt, mod: mod}
	var sel Selection
	switch {
	case mods.Shift:
		v.press.anchor = st.Selection.Anchor()
		v.press.from, v.press.to = v.press.anchor, v.press.anchor
		sel = TextSelectionBetween(st.Selection.AnchorPos(), r, 0)
	case clicks == 2:
		a, z := wordAround(r)
		v.press.from, v.press.to = a, z
		sel = TextSelectionAt(st.Doc, a, z)
	case clicks >= 3:
		a, z := r.Start(r.Depth), r.End(r.Depth)
		v.press.from, v.press.to = a, z
		sel = TextSelectionAt(st.Doc, a, z)
	default:
		sel = NewTextSelection(r, nil)
	}
	v.clearSink()
	v.s.Dispatch(st.Tr().SetSelection(sel))
}

func (v *rv) extendTo(pt core.Point) {
	pos, ok := v.PosAtCoords(float64(pt.X), float64(pt.Y))
	if !ok {
		return
	}
	st := v.s.State()
	r := st.Doc.Resolve(pos)
	from, to := v.press.from, v.press.to
	anchor, head := v.press.anchor, pos
	if v.press.unit > 1 && r.Parent().InlineContent() {
		a, z := r.Start(r.Depth), r.End(r.Depth)
		if v.press.unit == 2 {
			a, z = wordAround(r)
		}
		if pos < from {
			anchor, head = to, a
		} else {
			anchor, head = from, z
		}
	}
	sel := TextSelectionBetween(st.Doc.Resolve(anchor), st.Doc.Resolve(head), 0)
	if !sel.Eq(st.Selection) {
		v.s.Dispatch(st.Tr().SetSelection(sel))
	}
}

// dragMove extends the selection to pt and, near or past the viewport's
// edges, scrolls on a timer and extends it again, faster the farther out.
func (v *rv) dragMove(pt core.Point) {
	v.press.dragging, v.press.last = true, pt
	v.extendTo(pt)
	if v.stop == nil && v.edgeSpeed() != 0 {
		v.stop = v.cx.AfterFunc(edgeTick, v.edgeTick)
	}
}

func (v *rv) edgeSpeed() float64 {
	y := float64(v.press.last.Y) - float64(v.scrollY.GetUntracked())
	height := float64(v.viewport.GetUntracked().Height)
	over := 0.0
	dir := 0.0
	switch {
	case y < edgeZone:
		over, dir = edgeZone-y, -1
	case y > height-edgeZone:
		over, dir = y-(height-edgeZone), 1
	default:
		return 0
	}
	return dir * (edgeMin + (edgeMax-edgeMin)*min(over/edgeRamp, 1))
}

func (v *rv) edgeTick() {
	v.stop = nil
	speed := v.edgeSpeed()
	if speed == 0 || !v.press.dragging {
		return
	}
	y := float64(v.scrollY.GetUntracked()) + speed
	if box, ok := v.content.Frame(); ok {
		y = min(y, float64(box.Size.Height)-float64(v.viewport.GetUntracked().Height))
	}
	y = max(y, 0)
	v.vs.ScrollTo(core.Point{Y: core.Points(y)}, false)
	v.press.last.Y += core.Points(y) - v.scrollY.GetUntracked()
	v.scrollY.Set(core.Points(y))
	v.extendTo(v.press.last)
	v.stop = v.cx.AfterFunc(edgeTick, v.edgeTick)
}

func (v *rv) stopEdge() {
	if v.stop != nil {
		v.stop()
		v.stop = nil
	}
}

func (v *rv) dragEnd() {
	if v.press.dragging && v.press.mod {
		v.coverBlocks()
	}
	v.press.dragging = false
	v.stopEdge()
}

// coverBlocks widens a text selection across blocks to the whole of them,
// Tiptap's NodeRange, ready for a handle to carry.
func (v *rv) coverBlocks() {
	st := v.s.State()
	sel, ok := st.Selection.(*TextSelection)
	if !ok || sel.FromPos().SameParent(sel.ToPos()) {
		return
	}
	rng := sel.FromPos().BlockRange(sel.ToPos(), nil)
	if rng == nil {
		return
	}
	a := SelectionFindFrom(st.Doc.Resolve(rng.Start()), 1, true)
	z := SelectionFindFrom(st.Doc.Resolve(rng.End()), -1, true)
	if a == nil || z == nil {
		return
	}
	anchor, head := a.From(), z.To()
	if sel.Head() < sel.Anchor() {
		anchor, head = head, anchor
	}
	if next := TextSelectionAt(st.Doc, anchor, head); !next.Eq(sel) {
		v.s.Dispatch(st.Tr().SetSelection(next))
	}
}
