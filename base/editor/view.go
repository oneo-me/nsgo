package editor

import (
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/base/internal/syntax"
	"github.com/oneo-me/nsgo/core"
)

// The view: a vertical Scroll of a row of the gutter and the text area (a
// horizontal Scroll without soft wrap). The rows' box takes presses,
// hovers and drags, and holds its layers back to front: the active lines,
// the lines' decorations, search matches, selections, matching brackets,
// the rows in view (a Text each), the definition link, the carets, the
// placeholder and a touch selection's handles. A native Input, the sink,
// sits at the primary caret, so input methods and soft keyboards work
// there.

// rect is a box of a layer in the rows' box, styled by class and style.
type rect struct {
	x, y, w, h float32
	style      *core.Style
	class      string
}

// joinClasses returns a's classes then b's, which win.
func joinClasses(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}

func at(x, y, w, h float32) core.Style {
	return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(x)), Top: core.Pt(core.Points(y)),
		Width: core.Pt(core.Points(w)), Height: core.Pt(core.Points(h))}
}

func merge(styles ...core.Style) core.Style {
	var out core.Style
	for _, s := range styles {
		if !s.IsZero() {
			out = out.Merge(s)
		}
	}
	return out
}

// Render implements core.Node.
func (p Editor) Render(cx *core.Scope) core.Node {
	e := newEd(cx, p)
	text := e.textArea()
	if !p.SoftWrap {
		text = core.Scroll{Horizontal: true, Ref: e.hs, Style: core.Style{FlexGrow: 1, FlexShrink: 1},
			OnScroll: func(ev core.ScrollEvent) { e.scrollX.Set(ev.Offset.X) }, Children: []core.Node{text}}
	} else {
		text = core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1}, Children: []core.Node{text}}
	}
	styleFunc := p.StyleFunc
	if e.autoHeight() {
		// The editor grows with its text.
		styleFunc = func() core.Style {
			e.textRev.Get()
			st := core.Style{Height: core.Pt(core.Points(e.rootHeight()))}
			if p.StyleFunc != nil {
				st = merge(st, p.StyleFunc())
			}
			return st
		}
	}
	return core.View{
		Class:     p.Class,
		Style:     merge(core.Style{Overflow: core.Hidden, FontFamily: e.font.Family, FontSize: e.font.Size}, p.Style),
		StyleFunc: styleFunc,
		TestID:    p.TestID,
		Children: []core.Node{
			core.Scroll{
				Ref: e.vs, TestID: "editor-scroll",
				Style:    core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
				OnScroll: func(ev core.ScrollEvent) { e.scrollY.Set(ev.Offset.Y) },
				OnLayout: func(b core.Bounds) { e.viewport.Set(b.Size) },
				Children: []core.Node{core.View{
					Style: core.Style{Direction: core.Row},
					StyleFunc: func() core.Style {
						e.textRev.Get()
						e.viewport.Get()
						return core.Style{Height: core.Pt(core.Points(e.contentHeight()))}
					},
					Children: []core.Node{e.gutter(), text},
				}},
			},
			e.sinkView(),
			e.completionList(),
			core.Show{When: func() bool { return e.focused.Get() && e.composing.Get() == "" }, Then: func(cx *core.Scope) core.Node {
				e.keys(cx)
				return core.Group{}
			}},
			// Tab takes an inline completion and Escape drops it, over the
			// editor's keys.
			core.Show{When: func() bool { return e.focused.Get() && e.inline.Get() != "" }, Then: func(cx *core.Scope) core.Node {
				cx.OnKeyWith("tab", core.KeyOptions{When: core.KeyAlways}, e.acceptInline)
				cx.OnKeyWith("escape", core.KeyOptions{When: core.KeyAlways}, e.hideInline)
				return core.Group{}
			}},
		},
	}
}

// rows is the display rows made, for Each.
func (e *ed) rows() []int {
	s, end := e.windowRows()
	out := make([]int, end-s)
	for i := range out {
		out[i] = s + i
	}
	return out
}

func (e *ed) gutter() core.Node {
	if e.p.NoLineNumbers && e.p.NoFolding {
		return core.Group{}
	}
	s, c := e.styles, &e.p.Classes
	return core.View{
		Class: c.Gutter,
		StyleFunc: func() core.Style {
			e.textRev.Get()
			return merge(core.Style{Width: core.Pt(core.Points(e.gutterWidth())), FlexShrink: 0}, s().Gutter)
		},
		Children: []core.Node{core.Each(e.rows, func(r int) int { return r }, func(cx *core.Scope, row func() int, _ func() int) core.Node {
			line := func() int {
				e.textRev.Get()
				r := row()
				if r >= e.st.disp.count() {
					return -1
				}
				d := e.st.disp.row(r, e.st.r())
				if d.start != 0 {
					return -1 // a wrapped line's continuation
				}
				return d.line
			}
			active := func() bool {
				e.selRev.Get()
				l := line()
				for _, sel := range e.st.sels {
					if sel.head.line == l {
						return true
					}
				}
				return false
			}
			var children []core.Node
			if !e.p.NoLineNumbers {
				children = append(children, core.Text{
					ContentFunc: func() string {
						if l := line(); l >= 0 {
							return itoa(l + 1)
						}
						return ""
					},
					Class: c.LineNumber,
					ClassFunc: func() string {
						if c.ActiveLineNumber != "" && active() {
							return c.ActiveLineNumber
						}
						return ""
					},
					StyleFunc: func() core.Style {
						st := merge(core.Style{FlexGrow: 1, TextAlign: core.TextAlignEnd, LineHeight: core.Points(e.lh)}, s().LineNumber)
						if active() {
							st = merge(st, s().ActiveLineNumber)
						}
						return st
					},
					OnPress: func(core.PressEvent) {
						if l := line(); l >= 0 && e.p.OnLineNumberPress != nil {
							e.p.OnLineNumberPress(l)
						}
					},
				})
			}
			if !e.p.NoFolding {
				folded := func() bool {
					e.textRev.Get()
					l := line()
					return slices.ContainsFunc(e.st.folded, func(f foldRange) bool { return f.start == l })
				}
				children = append(children, core.View{
					Class:     c.FoldMarker,
					Style:     core.Style{Width: core.Pt(core.Points(e.lh)), AlignItems: core.AlignCenter},
					StyleFunc: func() core.Style { return s().FoldMarker },
					OnPress: func(core.PressEvent) {
						if l := line(); l >= 0 {
							e.fold(l, !folded())
						}
					},
					NoFocus: true,
					Children: []core.Node{core.Text{ContentFunc: func() string {
						l := line()
						if l < 0 {
							return ""
						}
						if folded() {
							return "›"
						}
						if slices.ContainsFunc(e.st.foldCandidates(), func(f foldRange) bool { return f.start == l }) {
							return "⌄"
						}
						return ""
					}, Style: core.Style{LineHeight: core.Points(e.lh)}}},
				})
			}
			return core.View{
				StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(core.Points(float32(row()) * e.lh)),
						Height: core.Pt(core.Points(e.lh)), Direction: core.Row, Padding: core.Edges{Left: 12, Right: 4}}
				},
				Children: children,
			}
		})},
	}
}

// boxWidth is the rows' box's width.
func (e *ed) boxWidth() float32 {
	vp := e.viewport.Get()
	w := float32(vp.Width) - e.gutterWidth()
	if e.p.SoftWrap {
		return w
	}
	return max(float32(e.widest+2)*e.charW+2*pad, w)
}

func (e *ed) textArea() core.Node {
	s, c := e.styles, &e.p.Classes
	desktop := core.Untracked(e.cx, e.cx.Env().Desktop)
	box := core.View{
		Ref:   e.box,
		Class: c.Content,
		StyleFunc: func() core.Style {
			e.textRev.Get()
			return merge(core.Style{Width: core.Pt(core.Points(e.boxWidth())), Height: core.Pt(core.Points(e.contentHeight())),
				Cursor: core.CursorText}, s().Content)
		},
		OnHoverMove: func(ev core.HoverEvent) { e.pointer(ev.Position, ev.PagePosition, ev.Modifiers) },
		OnHoverOut:  e.pointerLeft,
		OnContextMenu: func(ev core.ContextMenuEvent) {
			if e.p.OnContextMenu != nil {
				e.p.OnContextMenu(ContextMenuEvent{Position: e.position(e.posAt(ev.Position)), At: ev.PagePosition})
			}
		},
		NoFocus: true,
		TestID:  "editor-text",
	}
	if desktop {
		box.OnPressDown = func(ev core.PressEvent) {
			e.pressDown(ev.Position, ev.Clicks, ev.Modifiers)
			e.sinkRef.Focus()
			e.textPress(ev.Position, ev.PagePosition, ev.Modifiers, ev.Clicks)
		}
		// A drag extends what the press selected.
		box.Responder = &core.Responder{Claims: []core.TouchClaim{{OnMove: true}},
			OnMove:      func(ev core.ResponderEvent) { e.dragMove(ev.Position) },
			OnRelease:   func(core.ResponderEvent) { e.dragEnd() },
			OnTerminate: func(core.ResponderEvent) { e.dragEnd() }}
	} else {
		box.OnPress = func(ev core.PressEvent) {
			e.tap(ev.Position, ev.PagePosition)
			e.textPress(ev.Position, ev.PagePosition, ev.Modifiers, max(ev.Clicks, 1))
		}
		box.OnLongPress = func(ev core.PressEvent) { e.longPress(ev.Position, ev.PagePosition) }
	}
	box.Children = []core.Node{
		e.layer(func() []rect { return e.lineRects() }),
		e.layer(func() []rect { return e.rulerRects() }),
		e.layer(func() []rect { return e.guideRects() }),
		e.layer(func() []rect { return e.matchRects() }),
		e.layer(func() []rect { return e.selectionRects() }),
		e.layer(func() []rect { return e.bracketRects() }),
		core.Each(e.rows, func(r int) int { return r }, func(cx *core.Scope, row func() int, _ func() int) core.Node {
			return core.View{
				Class: c.Line,
				StyleFunc: func() core.Style {
					return merge(core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0),
						Top: core.Pt(core.Points(float32(row()) * e.lh)), Height: core.Pt(core.Points(e.lh))}, s().Line)
				},
				Children: []core.Node{core.Text{
					MaxLines:  1,
					SpansFunc: func() []core.Span { return e.rowSpans(row()) },
					Style: core.Style{Position: core.Absolute, Left: core.Pt(pad), Top: core.Pt(0), LineHeight: core.Points(e.lh),
						WhiteSpace: core.WhiteSpaceNoWrap},
				}},
			}
		}),
		e.glyphs(),
		e.layer(func() []rect { return e.linkRects() }),
		e.layer(func() []rect { return e.caretRects() }),
		e.inlineView(),
		core.Show{When: func() bool {
			e.textRev.Get()
			return e.p.Placeholder != "" && e.st.r().lines() == 1 && e.st.line(0) == ""
		},
			Then: func(cx *core.Scope) core.Node {
				return core.Text{Content: e.p.Placeholder, MaxLines: 1, Class: c.Placeholder, Style: core.Style{Position: core.Absolute, Left: core.Pt(pad),
					Top: core.Pt(0), LineHeight: core.Points(e.lh)}, StyleFunc: func() core.Style { return s().Placeholder }}
			}},
		e.handles(),
	}
	return box
}

// layer is a layer of boxes, each a view styled by its part.
func (e *ed) layer(rects func() []rect) core.Node {
	list := core.NewMemoFunc(e.cx, rects, func(a, b []rect) bool { return slices.Equal(a, b) })
	return core.Each(func() []int {
		n := len(list.Get())
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
		return core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
			all := list.Get()
			if i() >= len(all) {
				return core.Style{Display: core.DisplayNone}
			}
			r := all[i()]
			return merge(at(r.x, r.y, r.w, r.h), *r.style)
		}, ClassFunc: func() string {
			if all := list.Get(); i() < len(all) {
				return all[i()].class
			}
			return ""
		}}
	})
}

// spanRects is the boxes of the text from a to b on the rows made, a
// character's width past a line's end when the range goes on.
func (e *ed) spanRects(a, b pos, style *core.Style, class string, out []rect) []rect {
	s, end := e.windowRows()
	return e.rowRects(a, b, s, end, style, class, out)
}

// rowRects is the boxes of the text from a to b on rows from to to.
func (e *ed) rowRects(a, b pos, from, to int, style *core.Style, class string, out []rect) []rect {
	r := e.st.r()
	first := max(e.st.disp.rowOf(a), from)
	last := min(e.st.disp.rowOf(b), to-1)
	for row := first; row <= last; row++ {
		d := e.st.disp.row(row, r)
		if !e.st.disp.visibleLine(d.line) {
			continue
		}
		l := r.line(d.line)
		x0 := float32(pad)
		if a.line == d.line && a.col >= d.start {
			x0 = e.xOf(maxPos(a, pos{d.line, d.start}))
		}
		x1 := e.xOf(pos{d.line, d.end})
		if b.line == d.line && b.col <= d.end {
			x1 = e.xOf(b)
		} else if d.end == len(l) {
			x1 += e.charW / 2
		}
		if x1 > x0 {
			out = append(out, rect{x0, float32(row) * e.lh, x1 - x0, e.lh, style, class})
		}
	}
	return out
}

func (e *ed) lineRects() []rect {
	e.selRev.Get()
	e.marksRev.Get()
	var out []rect
	w := e.boxWidth()
	s, end := e.windowRows()
	if !e.p.NoActiveLine {
		seen := map[int]bool{}
		for _, sel := range e.st.sels {
			if !sel.empty() || seen[sel.head.line] {
				continue
			}
			seen[sel.head.line] = true
			row := e.st.disp.lineRow(sel.head.line)
			if row >= s && row < end {
				out = append(out, rect{0, float32(row) * e.lh, w, e.lh, &e.styles().ActiveLine, e.p.Classes.ActiveLine})
			}
		}
	}
	for _, m := range e.marks {
		if m.deco == nil || m.deco.LineStyle.IsZero() && m.deco.LineClass == "" {
			continue
		}
		for line := m.from.line; line <= m.to.line; line++ {
			row := e.st.disp.lineRow(line)
			if row >= s && row < end && e.st.disp.visibleLine(line) {
				out = append(out, rect{0, float32(row) * e.lh, w, e.lh, &m.deco.LineStyle, m.deco.LineClass})
			}
		}
	}
	return out
}

func (e *ed) matchRects() []rect {
	e.marksRev.Get()
	e.selRev.Get()
	if len(e.matches) == 0 {
		return nil
	}
	current := e.currentMatch()
	s, end := e.windowRows()
	r := e.st.r()
	from := e.st.disp.row(min(s, e.st.disp.count()-1), r).line
	to := e.st.disp.row(max(min(end, e.st.disp.count())-1, 0), r).line
	i := matchAfter(e.matches, pos{from, 0})
	if len(e.matches) > 0 && e.matches[0].from.line >= from {
		i = 0
		for i < len(e.matches) && e.matches[i].from.line < from {
			i++
		}
	}
	var out []rect
	for ; i < len(e.matches) && e.matches[i].from.line <= to; i++ {
		style, class := &e.styles().SearchMatch, e.p.Classes.SearchMatch
		if i == current {
			cur := merge(e.styles().SearchMatch, e.styles().CurrentSearchMatch)
			style, class = &cur, joinClasses(class, e.p.Classes.CurrentSearchMatch)
		}
		out = e.spanRects(e.matches[i].from, e.matches[i].to, style, class, out)
	}
	return out
}

func (e *ed) selectionRects() []rect {
	e.selRev.Get()
	var out []rect
	for _, sel := range e.st.sels {
		if !sel.empty() {
			out = e.spanRects(sel.start(), sel.end(), &e.styles().Selection, e.p.Classes.Selection, out)
		}
	}
	return out
}

func (e *ed) bracketRects() []rect {
	e.selRev.Get()
	if !e.focused.Get() || e.styles().MatchingBracket.IsZero() && e.p.Classes.MatchingBracket == "" {
		return nil
	}
	p := e.st.primary()
	if !p.empty() {
		return nil
	}
	a, b, ok := e.st.matchingBracket(p.head)
	if !ok {
		return nil
	}
	var out []rect
	for _, x := range []pos{a, b} {
		out = e.spanRects(x, pos{x.line, x.col + 1}, &e.styles().MatchingBracket, e.p.Classes.MatchingBracket, out)
	}
	return out
}

func (e *ed) linkRects() []rect {
	if !e.hasLink.Get() {
		return nil
	}
	l := e.link.Get()
	var out []rect
	for _, r := range e.spanRects(l.from, l.to, &e.styles().DefinitionLink, e.p.Classes.DefinitionLink, nil) {
		out = append(out, rect{r.x, r.y + e.lh - 2, r.w, 1, r.style, r.class})
	}
	return out
}

func (e *ed) caretRects() []rect {
	e.selRev.Get()
	if !e.focused.Get() || e.composing.Get() != "" {
		return nil
	}
	s, end := e.windowRows()
	var out []rect
	for _, sel := range e.st.sels {
		row := e.st.disp.rowOf(sel.head)
		if row < s || row >= end {
			continue
		}
		out = append(out, rect{e.xOf(sel.head), float32(row) * e.lh, 1, e.lh, &e.styles().Caret, e.p.Classes.Caret})
	}
	return out
}

// rowSpans is row's text in runs of one style: the highlighting's kind,
// the decorations' and the diagnostics' over it, tabs as spaces.
func (e *ed) rowSpans(row int) []core.Span {
	e.textRev.Get()
	e.hlRev.Get()
	e.marksRev.Get()
	dark := e.cx.Dark()
	r := e.st.r()
	if row >= e.st.disp.count() {
		return nil
	}
	d := e.st.disp.row(row, r)
	l := r.line(d.line)
	if d.end > len(l) {
		d.end = len(l)
	}
	palette := &syntax.Light
	if dark {
		palette = &syntax.Dark
	}
	// The cuts: token bounds and marks' bounds within the row.
	cuts := []int{d.start, d.end}
	toks := e.hl.line(d.line)
	if e.p.NoHighlighting {
		toks = nil
	}
	for _, t := range toks {
		cuts = append(cuts, t.Start, t.End)
	}
	var marks []mark
	for _, m := range e.marks {
		if m.from.line > d.line {
			break
		}
		if m.to.line < d.line || m.diag == nil && m.deco.Style.IsZero() && m.deco.Class == "" && m.deco.Kind == "" {
			continue
		}
		marks = append(marks, m)
		if m.from.line == d.line {
			cuts = append(cuts, m.from.col)
		}
		if m.to.line == d.line {
			cuts = append(cuts, m.to.col)
		}
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	var out []core.Span
	col := colAt(l, d.start, e.st.tab)
	for i := 0; i+1 < len(cuts); i++ {
		a, b := cuts[i], cuts[i+1]
		if a < d.start || b > d.end || a >= b {
			continue
		}
		var style core.Style
		var class string
		for _, t := range toks {
			if t.Start <= a && b <= t.End {
				style = palette[t.Kind]
			}
		}
		for _, m := range marks {
			from, to := m.from, m.to
			if (from.line < d.line || from.col <= a) && (to.line > d.line || b <= to.col) {
				switch {
				case m.deco != nil:
					if k := syntax.KindNamed(m.deco.Kind); k != syntax.Plain {
						style = palette[k]
					}
					style = merge(style, m.deco.Style)
					class = joinClasses(class, m.deco.Class)
				case m.diag != nil:
					style = merge(style, e.diagStyle(m.diag))
					class = joinClasses(class, e.diagClass(m.diag))
				}
			}
		}
		// Tabs to the next stop, as spaces.
		var text strings.Builder
		for _, c := range l[a:b] {
			if c == '\t' {
				n := e.st.tab - col%e.st.tab
				text.WriteString(strings.Repeat(" ", n))
				col += n
				continue
			}
			text.WriteRune(c)
			col += runeCols(c, col, e.st.tab)
		}
		out = append(out, core.Span{Content: text.String(), Class: class, Style: style})
	}
	if folded := slices.ContainsFunc(e.st.folded, func(f foldRange) bool { return f.start == d.line && f.end > f.start }) &&
		d.end == len(l); folded {
		out = append(out, core.Span{Content: " ⋯ ", Class: e.p.Classes.FoldPlaceholder, Style: e.styles().FoldPlaceholder})
	}
	if len(out) == 0 {
		out = append(out, core.Span{Content: ""})
	}
	return out
}

func (e *ed) diagStyle(d *Diagnostic) core.Style {
	s := e.styles
	var st core.Style
	switch d.Severity {
	case SeverityError:
		st = s().DiagnosticError
	case SeverityWarning:
		st = s().DiagnosticWarning
	case SeverityInfo:
		st = s().DiagnosticInfo
	default:
		st = s().DiagnosticHint
	}
	if d.Unnecessary {
		st = merge(st, s().UnnecessaryText)
	}
	if d.Deprecated {
		st = merge(st, s().DeprecatedText)
	}
	return st
}

func (e *ed) diagClass(d *Diagnostic) string {
	c := &e.p.Classes
	class := [...]string{SeverityError: c.DiagnosticError, SeverityWarning: c.DiagnosticWarning, SeverityInfo: c.DiagnosticInfo,
		SeverityHint: c.DiagnosticHint}[min(d.Severity, SeverityHint)]
	if d.Unnecessary {
		class = joinClasses(class, c.UnnecessaryText)
	}
	if d.Deprecated {
		class = joinClasses(class, c.DeprecatedText)
	}
	return class
}

// caretPage is the primary caret's box in the root view.
func (e *ed) caretPage() core.Bounds {
	b, _ := e.box.Bounds()
	h := e.st.primary().head
	return core.Bounds{Origin: core.Point{X: b.Origin.X + core.Points(e.xOf(h)), Y: b.Origin.Y + core.Points(float32(e.st.disp.rowOf(h))*e.lh)},
		Size: core.Size{Width: 1, Height: core.Points(e.lh)}}
}

// sinkView is the input that takes typing, at the primary caret in the
// editor's box, invisible.
func (e *ed) sinkView() core.Node {
	clear := core.RGBA(0, 0)
	return core.Input{
		Ref:            e.sinkRef,
		Label:          e.p.Label,
		AutoFocus:      e.p.AutoFocus,
		Value:          e.sink.Get,
		HideCaret:      true,
		NoAutoCorrect:  true,
		AutoCapitalize: core.AutoCapitalizeNone,
		PasteBehavior:  core.PasteReport,
		OnPaste: func(p core.PasteEvent) {
			if p.Text != "" {
				e.edit(func() []change {
					return e.st.paste(p.Text, strings.HasSuffix(p.Text, "\n") && e.st.primary().empty() && false)
				})
			}
		},
		OnChange: e.sinkChanged,
		OnCompositionChange: func(c core.Composition) {
			if c.Composing {
				e.composing.Set(c.Text)
			} else {
				e.composing.Set("")
			}
		},
		OnFocus: func() {
			e.focused.Set(true)
			if e.p.OnFocus != nil {
				e.p.OnFocus()
			}
		},
		OnBlur: func() {
			e.focused.Set(false)
			e.endCompletion()
			if e.p.OnBlur != nil {
				e.p.OnBlur()
			}
		},
		StyleFunc: func() core.Style {
			e.selRev.Get()
			e.textRev.Get()
			h := e.st.primary().head
			vp := e.viewport.Get()
			x := e.gutterWidth() + e.xOf(h) - float32(e.scrollX.Get())
			y := float32(e.st.disp.rowOf(h))*e.lh - float32(e.scrollY.Get())
			x = max(min(x, float32(vp.Width)-2), 0)
			y = max(min(y, float32(vp.Height)-e.lh), 0)
			return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(x)), Top: core.Pt(core.Points(y)),
				Width: core.Pt(1), Height: core.Pt(core.Points(e.lh)), Color: clear, Background: clear,
				SelectionColor: clear, FontSize: e.font.Size, Padding: core.Edges{}, BorderWidth: 0}
		},
	}
}

// completionList is the list of completions, below the query's start or
// above it without room, with the selected item's documentation beside.
func (e *ed) completionList() core.Node {
	s, c := e.styles, &e.p.Classes
	shown := func() bool {
		e.compRev.Get()
		e.selRev.Get()
		if e.comp.id == 0 || !e.comp.shown {
			return false
		}
		items, _ := e.filtered()
		return len(items) > 0
	}
	return core.Show{When: shown, Then: func(cx *core.Scope) core.Node {
		// The list's keys win over the editor's while it shows.
		bind := func(key string, fn func()) {
			cx.OnKeyWith(key, core.KeyOptions{When: core.KeyAlways, Repeat: true}, fn)
		}
		count := func() int { items, _ := e.filtered(); return len(items) }
		step := func(d int) func() {
			return func() {
				n := count()
				e.compSel = ((e.compSel+d)%n + n) % n
				e.compRev.Update(inc)
			}
		}
		acceptSelected := func() {
			items, _ := e.filtered()
			if e.compSel < len(items) {
				e.accept(items[e.compSel])
			}
		}
		bind("up", step(-1))
		bind("down", step(1))
		bind("ctrl+p", step(-1))
		bind("ctrl+n", step(1))
		bind("enter", acceptSelected)
		bind("tab", acceptSelected)
		bind("escape", e.endCompletion)
		list := core.NewMemoFunc(cx, func() []int {
			e.compRev.Get()
			e.selRev.Get()
			items, _ := e.filtered()
			out := make([]int, min(len(items), 50))
			for i := range out {
				out[i] = i
			}
			return out
		}, slices.Equal)
		row := func(cx *core.Scope, i func() int, _ func() int) core.Node {
			item := func() (CompletionItem, []int) {
				e.compRev.Get()
				e.selRev.Get()
				items, hits := e.filtered()
				if i() >= len(items) {
					return CompletionItem{}, nil
				}
				return items[i()], hits[i()]
			}
			return core.View{
				Role:  core.RoleMenuItem,
				Class: c.CompletionItem,
				ClassFunc: func() string {
					e.compRev.Get()
					if i() == e.compSel {
						return c.SelectedCompletionItem
					}
					return ""
				},
				StyleFunc: func() core.Style {
					e.compRev.Get()
					st := merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(core.Points(e.lh))}, s().CompletionItem)
					if i() == e.compSel {
						st = merge(st, s().SelectedCompletionItem)
					}
					return st
				},
				Selected:    func() bool { e.compRev.Get(); return i() == e.compSel },
				OnHoverIn:   func() { e.compSel = i(); e.compRev.Update(inc) },
				OnPressDown: func(core.PressEvent) { it, _ := item(); e.accept(it) },
				NoFocus:     true,
				Children: []core.Node{
					core.Text{ContentFunc: func() string { it, _ := item(); return it.Kind }, MaxLines: 1, Class: c.CompletionKind,
						StyleFunc: func() core.Style { return s().CompletionKind }},
					core.Text{MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1}, SpansFunc: func() []core.Span {
						it, hits := item()
						return matchSpans(it.Label, hits, s().CompletionMatch, c.CompletionMatch)
					}},
					core.Text{ContentFunc: func() string { it, _ := item(); return it.Detail }, MaxLines: 1, Class: c.CompletionDetail,
						Style: core.Style{TextAlign: core.TextAlignEnd}, StyleFunc: func() core.Style { return s().CompletionDetail }},
				},
			}
		}
		doc := func() string {
			e.compRev.Get()
			items, _ := e.filtered()
			if e.compSel < len(items) {
				return items[e.compSel].Documentation
			}
			return ""
		}
		return core.Overlay{
			AnchorBounds: func() core.Bounds {
				e.selRev.Get()
				b := e.caretPage()
				q := len(e.query())
				b.Origin.X -= core.Points(float32(q) * e.charW)
				return b
			},
			Placement: core.PlaceBottom, Align: core.AlignStart, Offset: 2, Margin: 4,
			Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignStart, Gap: 4}, Children: []core.Node{
				core.Scroll{Class: c.CompletionMenu, Style: core.Style{MaxHeight: core.Pt(core.Points(10 * e.lh)), MinWidth: core.Pt(160), MaxWidth: core.Pt(480)},
					StyleFunc: func() core.Style { return s().CompletionMenu },
					Children:  []core.Node{core.Each(list.Get, func(i int) int { return i }, row)}},
				core.Show{When: func() bool { return doc() != "" }, Then: func(cx *core.Scope) core.Node {
					return core.View{Class: c.CompletionDocumentation, Style: core.Style{MaxWidth: core.Pt(320)},
						StyleFunc: func() core.Style { return s().CompletionDocumentation },
						Children:  []core.Node{core.Text{ContentFunc: doc}}}
				}},
			}}},
		}
	}}
}

// matchSpans is label with the characters hits matched styled.
func matchSpans(label string, hits []int, style core.Style, class string) []core.Span {
	if len(hits) == 0 {
		return []core.Span{{Content: label}}
	}
	var out []core.Span
	last := 0
	for _, h := range hits {
		if h >= len(label) {
			break
		}
		if h > last {
			out = append(out, core.Span{Content: label[last:h]})
		}
		_, n := firstRuneLen(label[h:])
		out = append(out, core.Span{Content: label[h : h+n], Class: class, Style: style})
		last = h + n
	}
	if last < len(label) {
		out = append(out, core.Span{Content: label[last:]})
	}
	return out
}

func firstRuneLen(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return 0, 0
}

// tap is a tap on a touch screen: the caret where it lands, the word
// under it on a second tap.
func (e *ed) tap(pt, page core.Point) {
	now := e.cx.Now()
	double := now.Sub(e.lastTap) < doubleTap
	e.lastTap = now
	e.sinkRef.Focus()
	if double {
		e.longPress(pt, page)
		return
	}
	if e.touchSel != nil && e.touchSel.at(e.posAt(pt)) {
		e.contextMenuAt(page)
		return
	}
	e.st.setSels([]selection{caret(e.posAt(pt))})
	e.moved()
}

// longPress selects the word under the finger, the touch selection with
// handles, and asks for the edit menu over it.
func (e *ed) longPress(pt, page core.Point) {
	p := e.posAt(pt)
	a, b := e.st.lang.wordAt(e.st.line(p.line), p.col)
	sel := selection{anchor: pos{p.line, a}, head: pos{p.line, b}, goal: -1}
	e.st.setSels([]selection{sel})
	e.moved()
	e.touchSel = &sel
	e.selRev.Update(inc)
	e.sinkRef.Focus()
	e.contextMenuAt(page)
}

func (e *ed) contextMenuAt(page core.Point) {
	if e.p.OnContextMenu == nil || e.touchSel == nil {
		return
	}
	box, _ := e.box.Bounds()
	rs := e.spanRects(e.touchSel.start(), e.touchSel.end(), &e.styles().Selection, e.p.Classes.Selection, nil)
	var b core.Bounds
	for i, r := range rs {
		rb := core.Bounds{Origin: core.Point{X: box.Origin.X + core.Points(r.x), Y: box.Origin.Y + core.Points(r.y)},
			Size: core.Size{Width: core.Points(r.w), Height: core.Points(r.h)}}
		if i == 0 {
			b = rb
			continue
		}
		b = union(b, rb)
	}
	e.p.OnContextMenu(ContextMenuEvent{Position: e.position(e.touchSel.head), At: core.Point{X: b.Origin.X + b.Size.Width/2, Y: b.Origin.Y},
		Selection: b})
}

func union(a, b core.Bounds) core.Bounds {
	x0, y0 := min(a.Origin.X, b.Origin.X), min(a.Origin.Y, b.Origin.Y)
	x1 := max(a.Origin.X+a.Size.Width, b.Origin.X+b.Size.Width)
	y1 := max(a.Origin.Y+a.Size.Height, b.Origin.Y+b.Size.Height)
	return core.Bounds{Origin: core.Point{X: x0, Y: y0}, Size: core.Size{Width: x1 - x0, Height: y1 - y0}}
}

// handles are a touch selection's handles: a knob at each end that a
// finger drags, the other end staying.
func (e *ed) handles() core.Node {
	knob := 16 * float32(1)
	shown := func() bool {
		e.selRev.Get()
		return e.touchSel != nil && e.focused.Get() && len(e.st.sels) == 1
	}
	return core.Show{When: shown, Then: func(cx *core.Scope) core.Node {
		handle := func(start bool) core.Node {
			end := func() pos {
				p := e.st.primary()
				if start {
					return p.start()
				}
				return p.end()
			}
			return core.View{
				TestID: map[bool]string{true: "selection-handle-start", false: "selection-handle-end"}[start],
				StyleFunc: func() core.Style {
					e.selRev.Get()
					p := end()
					y := float32(e.st.disp.rowOf(p)) * e.lh
					if start {
						y -= knob
					}
					return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(e.xOf(p) - knob)), Top: core.Pt(core.Points(y)),
						Width: core.Pt(core.Points(2 * knob)), Height: core.Pt(core.Points(e.lh + knob)), AlignItems: core.AlignCenter,
						JustifyContent: map[bool]core.Justify{true: core.JustifyStart, false: core.JustifyEnd}[start]}
				},
				Responder: &core.Responder{Claims: []core.TouchClaim{{}}, Keep: true, OnMove: func(ev core.ResponderEvent) {
					box, _ := e.box.Bounds()
					p := e.posAt(core.Point{X: ev.PagePosition.X - box.Origin.X, Y: ev.PagePosition.Y - box.Origin.Y})
					sel := e.st.primary()
					other := sel.end()
					if !start {
						other = sel.start()
					}
					next := selection{anchor: other, head: p, goal: -1}
					e.st.setSels([]selection{next})
					e.touchSel = &next
					e.selRev.Update(inc)
				}, OnRelease: func(ev core.ResponderEvent) { e.contextMenuAt(ev.PagePosition) }},
				Children: []core.Node{core.View{Class: e.p.Classes.SelectionHandle, Style: core.Style{Width: core.Pt(10), Height: core.Pt(10)},
					StyleFunc: func() core.Style { return e.styles().SelectionHandle }}},
			}
		}
		return core.Group{handle(true), handle(false)}
	}}
}

// Focus gives the editor the keyboard; Blur takes it.
func (r *Ref) Focus() {
	if r.e != nil {
		r.e.sinkRef.Focus()
	}
}

// Blur takes the keyboard from the editor.
func (r *Ref) Blur() {
	if r.e != nil {
		r.e.sinkRef.Blur()
	}
}

// Value returns the text.
func (r *Ref) Value() string {
	if r.e == nil {
		return ""
	}
	return r.e.st.r().text()
}

// SetValue replaces the text, as an edit undo takes back.
func (r *Ref) SetValue(text string) {
	if r.e != nil {
		r.e.changed(r.e.st.replaceText(text), true)
	}
}

// Insert types text at each selection as it is.
func (r *Ref) Insert(text string) {
	if r.e != nil {
		r.e.edit(func() []change { return r.e.st.insert(text, false) })
	}
}

// Selections returns every selection, the primary one last.
func (r *Ref) Selections() []Selection {
	if r.e == nil {
		return nil
	}
	return r.e.selections()
}

// SetSelections sets the selections, the last one primary, and scrolls it
// into view.
func (r *Ref) SetSelections(sels []Selection) {
	if r.e == nil {
		return
	}
	out := make([]selection, len(sels))
	for i, s := range sels {
		out[i] = selection{anchor: r.e.pos(s.Anchor), head: r.e.pos(s.Head), goal: -1}
	}
	r.e.st.buf.open = false
	r.e.st.setSels(out)
	r.e.moved()
}

// Run runs a command, as its keys do.
func (r *Ref) Run(c Command) {
	if r.e != nil {
		r.e.run(c)
	}
}

// Replace replaces the current match of Search and selects the next.
func (r *Ref) Replace(with string) {
	if r.e != nil {
		r.e.replace(with)
	}
}

// ReplaceAll replaces every match as one edit and returns how many.
func (r *Ref) ReplaceAll(with string) int {
	if r.e == nil {
		return 0
	}
	return r.e.replaceAll(with)
}

// Fold folds the fold on line (from 0), or the innermost one around it;
// Unfold unfolds it.
func (r *Ref) Fold(line int) {
	if r.e != nil {
		r.e.fold(line, true)
	}
}

// Unfold unfolds the fold on or around line.
func (r *Ref) Unfold(line int) {
	if r.e != nil {
		r.e.fold(line, false)
	}
}

// Folds returns the folded ranges, nested ones included.
func (r *Ref) Folds() []FoldRange {
	if r.e == nil {
		return nil
	}
	return r.e.folds()
}

// SetFolds folds exactly folds.
func (r *Ref) SetFolds(folds []FoldRange) {
	if r.e == nil {
		return
	}
	r.e.st.folded = nil
	for _, f := range folds {
		if f.EndLine > f.StartLine {
			r.e.st.folded = append(r.e.st.folded, foldRange{f.StartLine, f.EndLine})
		}
	}
	r.e.foldsChanged()
}

// CanUndo and CanRedo report whether there is an edit to take back or do
// again.
func (r *Ref) CanUndo() bool { return r.e != nil && len(r.e.st.buf.undo) > 0 }

// CanRedo reports whether there is an undone edit to do again.
func (r *Ref) CanRedo() bool { return r.e != nil && len(r.e.st.buf.redo) > 0 }

// SelectedText returns the primary selection's text.
func (r *Ref) SelectedText() string {
	if r.e == nil {
		return ""
	}
	p := r.e.st.primary()
	return r.e.st.r().slice(p.start(), p.end())
}

// RevealPosition scrolls p into view, unfolding what hides it; to the
// middle with center.
func (r *Ref) RevealPosition(p Position, center bool) {
	if r.e != nil {
		at := r.e.pos(p)
		for _, f := range slices.Clone(r.e.st.folded) {
			if at.line > f.start && at.line <= f.end {
				r.e.fold(f.start, false)
			}
		}
		r.e.reveal(at, center)
	}
}

// GoToLine puts the caret at line's start and scrolls it to the middle.
func (r *Ref) GoToLine(line int) {
	if r.e == nil {
		return
	}
	p := r.e.pos(Position{Line: line})
	r.e.st.setSels([]selection{caret(p)})
	r.e.selRev.Update(inc)
	r.e.reveal(p, true)
	r.e.reportSelection()
}

// CaretBounds returns the primary caret's box; ok is false before the
// editor is laid out.
func (r *Ref) CaretBounds() (b Box, ok bool) {
	if r.e == nil {
		return Box{}, false
	}
	h := r.e.st.primary().head
	return r.e.boxOf(rect{x: r.e.xOf(h), y: float32(r.e.st.disp.rowOf(h)) * r.e.lh, w: 1, h: r.e.lh})
}

// ShowCompletions asks for completions at the caret.
func (r *Ref) ShowCompletions() {
	if r.e != nil {
		r.e.requestCompletion(CompletionInvoked, "")
	}
}

// PresentCompletions answers completion request id with items, filtered by
// what is typed since; with incomplete, typing on asks again.
func (r *Ref) PresentCompletions(id int, items []CompletionItem, incomplete bool) {
	if r.e == nil || r.e.comp.id != id {
		return
	}
	r.e.comp.items, r.e.comp.shown, r.e.comp.partial = items, len(items) > 0, incomplete
	if r.e.comp.shown {
		r.e.hideInline()
	}
	r.e.compSel = 0
	r.e.compRev.Update(inc)
}

// HideCompletions closes the completion list.
func (r *Ref) HideCompletions() {
	if r.e != nil {
		r.e.endCompletion()
	}
}

// PresentDefinitionLink answers definition request id, of the pointer, by
// underlining rng as a link until the command key is released or the
// pointer leaves it.
func (r *Ref) PresentDefinitionLink(id int, rng Range) {
	if r.e == nil || r.e.defID != id {
		return
	}
	r.e.link.Set(span{r.e.pos(rng.Start), r.e.pos(rng.End)})
	r.e.hasLink.Set(true)
}

// HideDefinitionLink drops the definition link.
func (r *Ref) HideDefinitionLink() {
	if r.e != nil {
		r.e.hideLink()
	}
}
