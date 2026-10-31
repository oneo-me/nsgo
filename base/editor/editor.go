package editor

import (
	"github.com/oneo-me/nsgo/base/internal/syntax"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/base/internal/window"
	"github.com/oneo-me/nsgo/core"
)

const (
	// hoverDelay is how long the mouse rests before OnHover.
	hoverDelay = 150 * time.Millisecond
	// doubleTap is the longest between two taps that select a word.
	doubleTap = 300 * time.Millisecond
	// measureAll is the most lines whose widths are all measured; past it
	// the scroll width grows to the widest row shown.
	measureAll = 20_000
	// pad is the room before the text, after the gutter.
	pad = 4
	// inlineDelay is how long typing pauses before an inline completion
	// is asked for.
	inlineDelay = 300 * time.Millisecond
	// edgeInner is how far inside the viewport's edges a drag starts
	// scrolling; edgeTick how often it scrolls, from edgeMin to edgeMax
	// points a tick over edgeRamp points past the edge.
	edgeInner        = 16
	edgeTick         = 16 * time.Millisecond
	edgeMin, edgeMax = 12, 64
	edgeRamp         = 80
)

// mark is a diagnostic or decoration resolved to the text and carried
// through edits.
type mark struct {
	from, to pos
	diag     *Diagnostic
	deco     *Decoration
}

// completion is a completion session: where its query starts, the
// request out and the answer.
type completion struct {
	id      int
	start   pos
	items   []CompletionItem
	shown   bool
	partial bool
}

// ed is an editor's controller: the state, the revisions bindings follow
// and what props, keys, typing, presses and the ref do to them.
type ed struct {
	p  Editor
	cx *core.Scope
	// looks are the parts' styles, Styles or what StylesFunc returns.
	looks *core.Memo[*Styles]
	st    *state
	hl    *highlighter
	// textRev counts changes of the text or its layout, selRev of the
	// selections, hlRev of highlighting, marksRev of diagnostics,
	// decorations and matches, compRev of the completion list.
	textRev, selRev, hlRev, marksRev, compRev *core.Signal[int]
	scrollX, scrollY                          *core.Signal[core.Points]
	viewport                                  *core.Signal[core.Size]
	focused                                   *core.Signal[bool]
	composing                                 *core.Signal[string]
	sink                                      *core.Signal[string]
	link                                      *core.Signal[span]
	hasLink                                   *core.Signal[bool]
	sinkRef, vs, hs, box                      *core.Ref
	win                                       *window.Rows
	font                                      core.Font
	charW, lh                                 float32
	widest                                    int
	// The last text OnChange reported, to know its echo.
	reported string
	version  int
	// Search.
	search  Search
	re      *regexp.Regexp
	matches []span
	scope   []span
	marks   []mark
	comp    completion
	compSel int
	nextID  int
	// The pending definition request and the hover.
	defID     int
	hoverWord span
	hoverStop func() bool
	// A press's anchor and unit, for drags: 1 characters, 2 words, 3 lines.
	pressAt   selection
	unit      int
	lastTap   time.Time
	touchSel  *selection
	hlRunning bool
	// block is a block drag's press: its row and x in the rows' box.
	block   bool
	blockAt core.Point
	// The selections moves left, for undoSelection, and expandSelection's
	// steps, for shrinkSelection.
	selHist              [][]selection
	selAt                int
	restoring, keepGrown bool
	grown                []grownSels
	// The inline completion: the request out, its text and the pause it
	// waits for.
	inlineID   int
	inlineStop func() bool
	inline     *core.Signal[string]
	// The drag's pointer in the viewport and the timer scrolling past an
	// edge.
	dragAt   core.Point
	dragStop func() bool
	dragging bool
}

func newEd(cx *core.Scope, p Editor) *ed {
	text := p.DefaultValue
	if p.Value != nil {
		text = core.Untracked(cx, p.Value)
	}
	e := &ed{p: p, cx: cx, st: newState(text), reported: text, looks: stylesMemo(cx, p.Styles, p.StylesFunc),
		textRev: core.NewSignal(cx, 0), selRev: core.NewSignal(cx, 0), hlRev: core.NewSignal(cx, 0),
		marksRev: core.NewSignal(cx, 0), compRev: core.NewSignal(cx, 0),
		scrollX: core.NewSignal(cx, core.Points(0)), scrollY: core.NewSignal(cx, core.Points(0)),
		viewport: core.NewSignal(cx, core.Size{}), focused: core.NewSignal(cx, false),
		composing: core.NewSignal(cx, ""), sink: core.NewSignal(cx, ""),
		link: core.NewSignalFunc(cx, span{}, func(a, b span) bool { return a == b }), hasLink: core.NewSignal(cx, false),
		inline:  core.NewSignal(cx, ""),
		sinkRef: &core.Ref{}, vs: &core.Ref{}, hs: &core.Ref{}, box: &core.Ref{}, win: window.New(4, 16)}
	lang := languageOf(p.Language)
	e.st.lang = lang
	e.st.tab, e.st.hard = lang.tabSize, lang.hardTabs
	if p.TabSize > 0 {
		e.st.tab = p.TabSize
	}
	if p.HardTabs {
		e.st.hard = true
	}
	e.st.autoClose, e.st.autoIndent = !p.NoAutoClose, !p.NoAutoIndent
	e.font = core.Font{Family: p.FontFamily, Size: p.FontSize}
	if e.font.Family == "" {
		e.font.Family = core.FontMonospace
	}
	if e.font.Size == 0 {
		e.font.Size = 13
	}
	e.lh = float32(p.LineHeight)
	if e.lh == 0 {
		e.lh = 20
	}
	e.charW = max(cx.MeasureText("0000000000", e.font).Width/10, 1)
	if p.Highlighter != nil {
		if app := p.Highlighter(p.Language); app != nil {
			e.hl = &highlighter{app: app, tokens: make([][]tok, e.st.r().lines()), fresh: make([]bool, e.st.r().lines())}
		}
	}
	if e.hl == nil && !p.NoHighlighting {
		e.hl = newHighlighter(lang, e.st.r().lines())
	}
	if p.Ref != nil {
		p.Ref.e = e
	}
	e.measure()
	e.recordSelections()
	e.follow()
	return e
}

// follow keeps the editor following its props.
func (e *ed) follow() {
	if e.p.Value != nil {
		e.cx.Effect(func() {
			v := e.p.Value()
			e.cx.Untrack(func() {
				if v == e.reported {
					return
				}
				e.reported = v
				e.changed(e.st.replaceText(v), false)
			})
		})
	}
	if e.p.FoldRanges != nil {
		e.cx.Effect(func() {
			folds := e.p.FoldRanges()
			e.cx.Untrack(func() {
				e.st.appFolds = []foldRange{}
				for _, f := range folds {
					if f.EndLine > f.StartLine {
						e.st.appFolds = append(e.st.appFolds, foldRange{f.StartLine, f.EndLine})
					}
				}
				sort.Slice(e.st.appFolds, func(i, j int) bool { return e.st.appFolds[i].start < e.st.appFolds[j].start })
				e.textRev.Update(inc)
			})
		})
	}
	if e.p.Search != nil {
		e.cx.Effect(func() {
			q := e.p.Search()
			e.cx.Untrack(func() { e.setSearch(q) })
		})
	}
	if e.p.Diagnostics != nil || e.p.Decorations != nil {
		e.cx.Effect(func() {
			var diags []Diagnostic
			var decos []Decoration
			if e.p.Diagnostics != nil {
				diags = e.p.Diagnostics()
			}
			if e.p.Decorations != nil {
				decos = e.p.Decorations()
			}
			e.cx.Untrack(func() { e.setMarks(diags, decos) })
		})
	}
	// Soft wrap follows the width.
	if e.p.SoftWrap {
		e.cx.Effect(func() {
			w := e.viewport.Get().Width
			e.cx.Untrack(func() {
				cols := max(int((float32(w)-e.gutterWidth()-2*pad)/e.charW), 8)
				if cols != e.st.wrap {
					e.st.wrap = cols
					e.st.relayout()
					e.textRev.Update(inc)
				}
			})
		})
	}
	// Highlighting runs jobs for the rows in view.
	if e.hl != nil {
		e.cx.Effect(func() {
			e.textRev.Get()
			s, end := e.windowRows()
			e.cx.Untrack(func() { e.highlight(s, end) })
		})
	}
}

func inc(n int) int { return n + 1 }

// pos and position convert between places and UTF-16 positions.
func (e *ed) pos(p Position) pos {
	r := e.st.r()
	line := min(max(p.Line, 0), r.lines()-1)
	return pos{line, byteCol(r.line(line), p.Column)}
}

func (e *ed) position(p pos) Position {
	return Position{p.line, utf16Col(e.st.line(p.line), p.col)}
}

func (e *ed) rangeOf(s span) Range { return Range{e.position(s.from), e.position(s.to)} }

// changed follows changes of the text: the highlighting, marks, matches,
// completion and the app.
func (e *ed) changed(changes []change, report bool) {
	if len(changes) == 0 {
		e.selRev.Update(inc)
		e.reportSelection()
		return
	}
	if e.hl != nil {
		e.hl.edited(changes)
	}
	e.selHist, e.selAt, e.grown = nil, 0, nil
	e.recordSelections()
	e.hideInline()
	for i := 0; i < len(e.marks); i++ {
		m := &e.marks[i]
		m.from, m.to = mapThrough(changes, m.from, true), mapThrough(changes, m.to, false)
	}
	if len(e.scope) > 0 {
		for i := range e.scope {
			e.scope[i] = span{mapThrough(changes, e.scope[i].from, true), mapThrough(changes, e.scope[i].to, false)}
		}
	}
	e.measureChanged(changes)
	e.findMatches()
	e.cx.Batch(func() {
		e.textRev.Update(inc)
		e.selRev.Update(inc)
		e.marksRev.Update(inc)
	})
	e.hideLink()
	if report && e.p.OnChange != nil {
		text := e.st.r().text()
		e.reported = text
		e.p.OnChange(text)
	} else if !report {
		e.reported = e.st.r().text()
	}
	e.reportSelection()
}

func (e *ed) reportSelection() {
	if e.p.OnSelectionChange != nil {
		e.p.OnSelectionChange(e.selections())
	}
}

func (e *ed) selections() []Selection {
	out := make([]Selection, len(e.st.sels))
	for i, s := range e.st.sels {
		out[i] = Selection{e.position(s.anchor), e.position(s.head)}
	}
	return out
}

// moved follows a move of the selections: the caret into view, the
// completion and the touch selection.
func (e *ed) moved() {
	e.reveal(e.st.primary().head, false)
	e.selMoved()
}

// selMoved follows a move of the selections that leaves the scrolling
// alone, as a drag's does.
func (e *ed) selMoved() {
	e.selRev.Update(inc)
	e.checkCompletion()
	e.touchSel = nil
	e.hideInline()
	if !e.restoring {
		e.recordSelections()
	}
	if !e.keepGrown {
		e.grown = nil
	}
	e.reportSelection()
}

// measure takes the widest line, every line up to measureAll.
func (e *ed) measure() {
	r := e.st.r()
	if r.lines() > measureAll {
		return
	}
	w := 0
	for i := range r.lines() {
		w = max(w, colAt(r.line(i), len(r.line(i)), e.st.tab))
	}
	e.widest = w
}

func (e *ed) measureChanged(changes []change) {
	r := e.st.r()
	if r.lines() <= measureAll {
		e.measure()
		return
	}
	for _, c := range changes {
		for l := c.from.line; l <= c.newEnd.line && l < r.lines(); l++ {
			e.widest = max(e.widest, colAt(r.line(l), len(r.line(l)), e.st.tab))
		}
	}
}

// edit runs a command that edits, unless the editor is read-only.
func (e *ed) edit(f func() []change) {
	if e.p.ReadOnly {
		return
	}
	e.clearSink()
	changes := f()
	e.changed(changes, true)
	e.reveal(e.st.primary().head, false)
}

// type types text from the sink or a key: brackets close, completion asks.
func (e *ed) typeText(text string) {
	if e.p.ReadOnly || text == "" {
		return
	}
	before := e.st.primary().head
	e.changed(e.st.insert(text, true), true)
	e.reveal(e.st.primary().head, false)
	e.touchSel = nil
	e.waitInline()
	r, _ := utf8.DecodeLastRuneInString(text)
	switch {
	case e.p.OnCompletionRequest == nil:
	case strings.ContainsRune(e.p.CompletionTriggerCharacters, r):
		e.requestCompletion(CompletionCharacter, string(r))
	case e.st.lang.wordChar(r) && e.comp.id == 0:
		h := e.st.primary().head
		l := e.st.line(h.line)
		if h.col >= len(l) || !e.st.lang.wordChar(firstRune(l[h.col:])) {
			e.requestCompletion(CompletionTyping, "")
		}
	case e.comp.id != 0 && !e.st.lang.wordChar(r):
		e.endCompletion()
	default:
		_ = before
		e.checkCompletion()
	}
}

func firstRune(s string) rune { r, _ := utf8.DecodeRuneInString(s); return r }

// sinkChanged takes what the sink holds now: the difference from what it
// held becomes backspaces and typed text, as autocorrection and input
// methods replace their text.
func (e *ed) sinkChanged(text string) {
	held := e.sink.GetUntracked()
	if e.composing.GetUntracked() != "" {
		e.sink.Set(text)
		return
	}
	common := 0
	for common < len(held) && common < len(text) && held[common] == text[common] {
		common++
	}
	for common > 0 && !startsRune(held[min(common, len(held)-1)]) {
		common--
	}
	for range utf8.RuneCountInString(held[common:]) {
		e.edit(func() []change { return e.st.deleteBy(e.st.prevChar, true) })
	}
	e.typeText(text[common:])
	e.sink.Set(text)
	if len(text) > 256 {
		e.sink.Set("")
	}
}

func (e *ed) clearSink() {
	if e.composing.GetUntracked() == "" && e.sink.GetUntracked() != "" {
		e.sink.Set("")
	}
}

// run runs a command.
func (e *ed) run(c Command) {
	s := e.st
	page := max(int(float32(e.viewport.GetUntracked().Height)/e.lh)-1, 1)
	move := func(extend bool, f func(p pos) pos) { s.moveBy(extend, f); e.moved() }
	vert := func(extend bool, n int) {
		s.move(extend, func(sel selection) selection { return s.vertical(sel, n) })
		e.moved()
	}
	switch c {
	case MoveLeft, SelectLeft:
		s.moveChar(c == SelectLeft, true)
		e.moved()
	case MoveRight, SelectRight:
		s.moveChar(c == SelectRight, false)
		e.moved()
	case MoveUp, SelectUp:
		vert(c == SelectUp, -1)
	case MoveDown, SelectDown:
		vert(c == SelectDown, 1)
	case MovePageUp, SelectPageUp:
		vert(c == SelectPageUp, -page)
	case MovePageDown, SelectPageDown:
		vert(c == SelectPageDown, page)
	case MoveWordLeft, SelectWordLeft:
		move(c == SelectWordLeft, s.wordLeft)
	case MoveWordRight, SelectWordRight:
		move(c == SelectWordRight, s.wordRight)
	case MoveSubwordLeft, SelectSubwordLeft:
		move(c == SelectSubwordLeft, s.subwordLeft)
	case MoveSubwordRight, SelectSubwordRight:
		move(c == SelectSubwordRight, s.subwordRight)
	case MoveLineStart, SelectLineStart:
		move(c == SelectLineStart, s.lineStart)
	case MoveLineEnd, SelectLineEnd:
		move(c == SelectLineEnd, s.lineEnd)
	case MoveDocumentStart, SelectDocumentStart:
		move(c == SelectDocumentStart, func(pos) pos { return pos{} })
	case MoveDocumentEnd, SelectDocumentEnd:
		move(c == SelectDocumentEnd, func(pos) pos { return s.r().end() })
	case MoveToMatchingBracket:
		move(false, func(p pos) pos {
			if _, m, ok := s.matchingBracket(p); ok {
				return m
			}
			return p
		})
	case SelectAll:
		s.setSels([]selection{{anchor: pos{}, head: s.r().end(), goal: -1}})
		e.moved()
	case SelectLine:
		s.selectLine()
		e.moved()
	case SelectNextOccurrence:
		s.selectNext()
		e.moved()
	case SelectPreviousOccurrence:
		s.selectPrevious()
		e.moved()
	case SelectWord:
		next := make([]selection, len(s.sels))
		for i, sel := range s.sels {
			a, b := s.lang.wordAt(s.line(sel.head.line), sel.head.col)
			next[i] = selection{anchor: pos{sel.head.line, a}, head: pos{sel.head.line, b}, goal: -1}
		}
		s.setSels(next)
		e.moved()
	case MoveParagraphUp, SelectParagraphUp, MoveParagraphDown, SelectParagraphDown:
		down := c == MoveParagraphDown || c == SelectParagraphDown
		s.moveBy(c == SelectParagraphUp || c == SelectParagraphDown, func(p pos) pos { return s.paragraphEdge(p, down) })
		e.moved()
	case SelectAllOccurrences:
		s.selectAllOccurrences()
		e.moved()
	case AddCaretAbove:
		s.addCaret(-1)
		e.moved()
	case AddCaretBelow:
		s.addCaret(1)
		e.moved()
	case SingleSelection:
		if len(s.sels) > 1 {
			s.setSels([]selection{s.primary()})
		} else if p := s.primary(); !p.empty() {
			s.setSels([]selection{caret(p.head)})
		}
		e.moved()
	case DeleteBackward:
		e.edit(func() []change { return s.deleteBy(s.prevChar, true) })
		e.checkCompletion()
	case DeleteForward:
		e.edit(func() []change { return s.deleteBy(s.nextChar, false) })
	case DeleteWordLeft:
		e.edit(func() []change { return s.deleteBy(s.wordLeft, false) })
	case DeleteWordRight:
		e.edit(func() []change { return s.deleteBy(s.wordRight, false) })
	case DeleteToLineStart:
		e.edit(func() []change { return s.deleteBy(func(p pos) pos { return pos{p.line, 0} }, false) })
	case DeleteToLineEnd:
		e.edit(func() []change { return s.deleteBy(s.lineEnd, false) })
	case DeleteLine:
		e.edit(s.deleteLines)
	case Newline:
		e.endCompletion()
		e.edit(s.newline)
	case NewlineBelow:
		s.moveBy(false, s.lineEnd)
		e.edit(s.newline)
	case NewlineAbove:
		e.edit(func() []change {
			s.moveBy(false, func(p pos) pos { return pos{p.line, 0} })
			changes := s.insert("\n", false)
			s.moveBy(false, func(p pos) pos { return pos{p.line - 1, 0} })
			return changes
		})
	case Indent:
		e.edit(s.indent)
	case Outdent:
		e.edit(s.outdent)
	case ToggleComment:
		e.edit(s.toggleComment)
	case ToggleBlockComment:
		e.edit(s.toggleBlockComment)
	case SortLinesAscending, SortLinesDescending:
		e.edit(func() []change { return s.sortLines(c == SortLinesDescending) })
	case TransformToUppercase:
		e.edit(func() []change { return s.transform(strings.ToUpper) })
	case TransformToLowercase:
		e.edit(func() []change { return s.transform(strings.ToLower) })
	case TransformToTitleCase:
		e.edit(func() []change { return s.transform(titleCase) })
	case TrimTrailingWhitespace:
		e.edit(s.trimTrailing)
	case MoveLinesUp, MoveLinesDown:
		e.edit(func() []change {
			if c == MoveLinesUp {
				return s.moveLines(-1)
			}
			return s.moveLines(1)
		})
	case DuplicateLinesUp, DuplicateLinesDown:
		e.edit(func() []change { return s.duplicateLines(c == DuplicateLinesUp) })
	case JoinLines:
		e.edit(s.joinLines)
	case TransposeCharacters:
		e.edit(s.transpose)
	case Undo, Redo:
		if e.p.ReadOnly {
			return
		}
		take := s.buf.takeUndo
		if c == Redo {
			take = s.buf.takeRedo
		}
		if changes, sels, ok := take(); ok {
			s.afterEdit(changes)
			s.setSels(slices.Clone(sels))
			e.changed(changes, true)
			e.reveal(s.primary().head, false)
		}
	case Fold:
		e.fold(s.primary().head.line, true)
	case Unfold:
		e.fold(s.primary().head.line, false)
	case FoldAll:
		s.folded = slices.Clone(s.foldCandidates())
		e.foldsChanged()
	case UnfoldAll:
		s.folded = nil
		e.foldsChanged()
	case FoldRecursive:
		(&Ref{e}).FoldRecursive(s.primary().head.line)
	case UnfoldRecursive:
		(&Ref{e}).UnfoldRecursive(s.primary().head.line)
	case FoldSelection:
		for _, sel := range slices.Clone(s.sels) {
			if a, b := sel.start(), sel.end(); b.line > a.line {
				(&Ref{e}).FoldRange(FoldRange{a.line, b.line})
			}
		}
	case FindNext, FindPrevious:
		e.findStep(c == FindNext)
	case FindWithSelection:
		if e.p.OnFindWithSelection != nil {
			p := s.primary()
			text := s.r().slice(p.start(), p.end())
			if p.empty() {
				a, b := s.lang.wordAt(s.line(p.head.line), p.head.col)
				text = s.line(p.head.line)[a:b]
			}
			e.p.OnFindWithSelection(text)
		}
	case Copy, Cut:
		text, _ := s.copyText()
		e.cx.SetClipboard(text)
		if c == Cut {
			e.edit(func() []change {
				for _, sel := range s.sels {
					if !sel.empty() {
						return s.deleteBy(func(p pos) pos { return p }, false)
					}
				}
				return s.deleteLines()
			})
		}
	case Paste:
		e.cx.ReadClipboard(func(text string) {
			if text != "" {
				e.edit(func() []change { return s.paste(text, false) })
			}
		})
	case ShowCompletions:
		e.requestCompletion(CompletionInvoked, "")
	case GoToDefinition:
		e.requestDefinition(s.primary().head, DefinitionKey)
	case ShowCodeActions:
		if e.p.OnCodeActionRequest != nil {
			p := s.primary()
			e.p.OnCodeActionRequest(e.rangeOf(span{p.start(), p.end()}))
		}
	case ExpandSelection:
		e.expandSelections()
	case ShrinkSelection:
		e.shrinkSelections()
	case UndoSelection:
		e.stepSelections(-1)
	case RedoSelection:
		e.stepSelections(1)
	case SplitSelectionIntoLines:
		s.splitLines()
		e.moved()
	}
}

// keys binds the editor's keys while it has the focus: Apple's
// conventions on Apple platforms, the others' elsewhere, on a common set.
func (e *ed) keys(cx *core.Scope) {
	apple := core.Untracked(cx, cx.Env().AppleKeys)
	mod := "ctrl"
	if apple {
		mod = "cmd"
	}
	word := "ctrl"
	if apple {
		word = "alt"
	}
	bind := func(c Command, keys ...string) {
		for _, k := range keys {
			cx.OnKeyWith(k, core.KeyOptions{When: core.KeyAlways, Repeat: true}, func() { e.run(c) })
		}
	}
	bind(MoveLeft, "left")
	bind(MoveRight, "right")
	bind(MoveUp, "up")
	bind(MoveDown, "down")
	bind(SelectLeft, "shift+left")
	bind(SelectRight, "shift+right")
	bind(SelectUp, "shift+up")
	bind(SelectDown, "shift+down")
	bind(MoveWordLeft, word+"+left")
	bind(MoveWordRight, word+"+right")
	bind(SelectWordLeft, word+"+shift+left")
	bind(SelectWordRight, word+"+shift+right")
	bind(MovePageUp, "pageup")
	bind(MovePageDown, "pagedown")
	bind(SelectPageUp, "shift+pageup")
	bind(SelectPageDown, "shift+pagedown")
	bind(DeleteBackward, "backspace", "shift+backspace")
	bind(DeleteForward, "delete")
	bind(DeleteWordLeft, word+"+backspace")
	bind(DeleteWordRight, word+"+delete")
	bind(Newline, "enter", "shift+enter")
	bind(NewlineBelow, mod+"+enter")
	bind(NewlineAbove, mod+"+shift+enter")
	bind(Indent, "tab", mod+"+]")
	bind(Outdent, "shift+tab", mod+"+[")
	bind(Undo, mod+"+z")
	bind(Redo, mod+"+shift+z")
	bind(SelectAll, mod+"+a")
	bind(SelectLine, mod+"+l")
	bind(SelectNextOccurrence, mod+"+d")
	bind(SelectAllOccurrences, mod+"+shift+l")
	bind(AddCaretAbove, mod+"+alt+up")
	bind(AddCaretBelow, mod+"+alt+down")
	bind(SingleSelection, "escape")
	bind(MoveLinesUp, "alt+up")
	bind(MoveLinesDown, "alt+down")
	bind(DuplicateLinesUp, "alt+shift+up")
	bind(DuplicateLinesDown, "alt+shift+down")
	bind(DeleteLine, mod+"+shift+k")
	bind(ToggleComment, mod+"+/")
	bind(ToggleBlockComment, "shift+alt+a")
	bind(MoveToMatchingBracket, mod+"+shift+\\")
	bind(JoinLines, "ctrl+j")
	bind(FindNext, "f3")
	bind(FindPrevious, "shift+f3")
	bind(GoToDefinition, "f12")
	bind(ShowCodeActions, mod+"+.")
	bind(Copy, mod+"+c")
	bind(Cut, mod+"+x")
	bind(SplitSelectionIntoLines, "shift+alt+i")
	if apple {
		bind(MoveLineStart, "cmd+left", "home", "ctrl+a")
		bind(MoveLineEnd, "cmd+right", "end", "ctrl+e")
		bind(SelectLineStart, "cmd+shift+left", "shift+home")
		bind(SelectLineEnd, "cmd+shift+right", "shift+end")
		bind(MoveDocumentStart, "cmd+up")
		bind(MoveDocumentEnd, "cmd+down")
		bind(SelectDocumentStart, "cmd+shift+up")
		bind(SelectDocumentEnd, "cmd+shift+down")
		bind(DeleteToLineStart, "cmd+backspace")
		bind(DeleteToLineEnd, "cmd+delete", "ctrl+k")
		bind(TransposeCharacters, "ctrl+t")
		bind(MoveSubwordLeft, "ctrl+alt+left")
		bind(MoveSubwordRight, "ctrl+alt+right")
		bind(SelectSubwordLeft, "ctrl+alt+shift+left")
		bind(SelectSubwordRight, "ctrl+alt+shift+right")
		bind(FindNext, "cmd+g")
		bind(FindPrevious, "cmd+shift+g")
		bind(FindWithSelection, "cmd+e")
		bind(Fold, "cmd+alt+[")
		bind(Unfold, "cmd+alt+]")
		bind(UndoSelection, "cmd+u")
		bind(RedoSelection, "cmd+shift+u")
		bind(ExpandSelection, "ctrl+shift+cmd+right")
		bind(ShrinkSelection, "ctrl+shift+cmd+left")
	} else {
		bind(MoveLineStart, "home")
		bind(MoveLineEnd, "end")
		bind(SelectLineStart, "shift+home")
		bind(SelectLineEnd, "shift+end")
		bind(MoveDocumentStart, "ctrl+home")
		bind(MoveDocumentEnd, "ctrl+end")
		bind(SelectDocumentStart, "ctrl+shift+home")
		bind(SelectDocumentEnd, "ctrl+shift+end")
		bind(DeleteToLineStart, "ctrl+shift+backspace")
		bind(DeleteToLineEnd, "ctrl+shift+delete")
		bind(Redo, "ctrl+y")
		bind(Fold, "ctrl+shift+[")
		bind(Unfold, "ctrl+shift+]")
		bind(UndoSelection, "ctrl+u")
		bind(RedoSelection, "ctrl+shift+u")
		bind(ExpandSelection, "shift+alt+right")
		bind(ShrinkSelection, "shift+alt+left")
	}
}

// reveal scrolls p into view, a row's margin around it, to the middle
// with center.
func (e *ed) reveal(p pos, center bool) {
	vp := e.viewport.GetUntracked()
	if vp.Height == 0 {
		return
	}
	row := e.st.disp.rowOf(p)
	top := float32(row) * e.lh
	y := float32(e.scrollY.GetUntracked())
	h := float32(vp.Height)
	around := e.p.CursorSurroundingLines
	if around <= 0 {
		around = 1
	}
	margin := min(float32(around)*e.lh, h/4)
	to := y
	switch {
	case center:
		to = top - h/2 + e.lh/2
	case top-margin < y:
		to = top - margin
	case top+e.lh+margin > y+h:
		to = top + e.lh + margin - h
	}
	to = max(min(to, e.contentHeight()-h), 0)
	if to != y {
		e.vs.ScrollTo(core.Point{Y: core.Points(to)}, false)
		e.scrollY.Set(core.Points(to))
	}
	if !e.p.SoftWrap {
		x := e.xOf(p)
		sx := float32(e.scrollX.GetUntracked())
		w := float32(vp.Width) - e.gutterWidth()
		toX := sx
		switch {
		case x-e.charW < sx:
			toX = max(x-4*e.charW, 0)
		case x+2*e.charW > sx+w:
			toX = x + 4*e.charW - w
		}
		if toX != sx {
			e.hs.ScrollTo(core.Point{X: core.Points(toX)}, false)
			e.scrollX.Set(core.Points(toX))
		}
	}
}

// xOf is where p is across its row, in the rows' box.
func (e *ed) xOf(p pos) float32 {
	l := e.st.line(p.line)
	d := e.st.disp.row(e.st.disp.rowOf(p), e.st.r())
	return pad + float32(colAt(l, p.col, e.st.tab)-colAt(l, d.start, e.st.tab))*e.charW
}

// posAt is the place at pt in the rows' box.
func (e *ed) posAt(pt core.Point) pos {
	n := e.st.disp.count()
	row := min(max(int(float32(pt.Y)/e.lh), 0), n-1)
	if float32(pt.Y) < 0 {
		return pos{}
	}
	if int(float32(pt.Y)/e.lh) >= n {
		return e.st.r().end()
	}
	d := e.st.disp.row(row, e.st.r())
	l := e.st.line(d.line)
	end := d.end
	if row+1 < n && e.st.disp.row(row+1, e.st.r()).line == d.line && end > d.start {
		_, w := utf8.DecodeLastRuneInString(l[:end])
		end -= w
	}
	col := int((float32(pt.X) - pad + e.charW/2) / e.charW)
	return pos{d.line, byteAtCol(l, d.start, end, colAt(l, d.start, e.st.tab)+max(col, 0), e.st.tab)}
}

func (e *ed) gutterWidth() float32 {
	if e.p.NoLineNumbers && e.p.NoFolding {
		return 0
	}
	w := float32(0)
	if !e.p.NoLineNumbers {
		w += float32(max(len(itoa(e.st.r().lines())), 2))*e.charW + 12
	}
	if !e.p.NoFolding {
		w += e.lh
	}
	return w + 4
}

func (e *ed) contentHeight() float32 {
	rows := float32(e.st.disp.count()) * e.lh
	switch {
	case e.autoHeight() || e.p.ScrollBeyondLastLine < 0:
		return rows
	case e.p.ScrollBeyondLastLine > 0:
		return rows + float32(e.p.ScrollBeyondLastLine)*e.lh
	}
	h := float32(e.viewport.GetUntracked().Height)
	return rows + max(h-2*e.lh, 0)
}

// autoHeight reports whether the editor grows with its text.
func (e *ed) autoHeight() bool { return e.p.MinRows > 0 || e.p.MaxRows > 0 }

// rootHeight is the height of an editor growing with its text: its rows
// within MinRows and MaxRows.
func (e *ed) rootHeight() float32 {
	n := e.st.disp.count()
	if e.p.MinRows > 0 {
		n = max(n, e.p.MinRows)
	}
	if e.p.MaxRows > 0 {
		n = min(n, e.p.MaxRows)
	}
	return float32(n) * e.lh
}

// windowRows is the rows made (tracked).
func (e *ed) windowRows() (int, int) {
	e.textRev.Get()
	vp := e.viewport.Get()
	y := float32(e.scrollY.Get())
	n := e.st.disp.count()
	first := min(max(int(y/e.lh), 0), n)
	last := min(int((y+max(float32(vp.Height), 40*e.lh))/e.lh)+1, n)
	return e.win.Span(first, max(last, first), n)
}

// highlight hands the next job to a goroutine of its own.
func (e *ed) highlight(from, to int) {
	if e.hlRunning || e.hl == nil {
		return
	}
	r := e.st.r()
	lineFrom := e.st.disp.row(min(from, max(e.st.disp.count()-1, 0)), r).line
	lineTo := e.st.disp.row(min(max(to-1, 0), max(e.st.disp.count()-1, 0)), r).line + 1
	start, end, ok := e.hl.next(r, lineFrom, lineTo)
	if !ok {
		return
	}
	version := e.st.buf.version
	lexer, app := e.hl.lexer, e.hl.app
	var lines []string
	if app != nil {
		// The app's highlighter sees the whole text.
		lines = make([]string, r.lines())
		for i := range lines {
			lines[i] = r.line(i)
		}
	} else {
		lines = make([]string, end-start)
		for i := range lines {
			lines[i] = r.line(start + i)
		}
	}
	e.hlRunning = true
	go func() {
		var toks [][]tok
		if app != nil {
			toks = syntax.HighlightLines(app, lines, start, end)
		} else {
			toks = syntax.LexLines(lexer, lines)
		}
		e.cx.Post(func() {
			e.hlRunning = false
			if e.st.buf.version == version {
				e.hl.take(start, toks)
				e.hlRev.Update(inc)
			}
			var s, end int
			e.cx.Untrack(func() { s, end = e.windowRows() })
			e.highlight(s, end)
		})
	}()
}

// setSearch finds q's matches.
func (e *ed) setSearch(q Search) {
	turnedOn := q.InSelection && !e.search.InSelection
	e.search = q
	e.re = nil
	result := SearchResult{Current: -1}
	if turnedOn {
		e.scope = nil
		for _, s := range e.st.sels {
			if s.empty() {
				e.scope = append(e.scope, span{pos{s.head.line, 0}, pos{s.head.line, len(e.st.line(s.head.line))}})
			} else {
				e.scope = append(e.scope, span{s.start(), s.end()})
			}
		}
	}
	if !q.InSelection {
		e.scope = nil
	}
	if q.Query != "" {
		re, err := compile(q)
		if err != nil {
			result.Err = err.Error()
		}
		e.re = re
	}
	e.findMatches()
	if e.p.OnSearchChange != nil {
		result.Count, result.Current = len(e.matches), e.currentMatch()
		e.p.OnSearchChange(result)
	}
}

func (e *ed) findMatches() {
	e.matches = nil
	if e.re != nil {
		e.matches = findAll(e.st.r(), e.re, e.scope)
	}
	e.marksRev.Update(inc)
}

// currentMatch is the match the primary selection is on, -1 for none.
func (e *ed) currentMatch() int {
	p := e.st.primary()
	for i, m := range e.matches {
		if m.from == p.start() && m.to == p.end() {
			return i
		}
	}
	return -1
}

func (e *ed) findStep(next bool) {
	if len(e.matches) == 0 {
		return
	}
	p := e.st.primary()
	i := matchBefore(e.matches, p.start())
	if next {
		i = matchAfter(e.matches, p.end())
	}
	m := e.matches[i]
	e.st.setSels([]selection{{anchor: m.from, head: m.to, goal: -1}})
	e.moved()
	if e.p.OnSearchChange != nil {
		e.p.OnSearchChange(SearchResult{Count: len(e.matches), Current: i})
	}
}

// replace replaces the current match with with and selects the next.
func (e *ed) replace(with string) {
	i := e.currentMatch()
	if i < 0 {
		e.findStep(true)
		return
	}
	m := e.matches[i]
	text := expand(e.re, e.search.Regex, e.st.line(m.from.line), m, with)
	e.st.setSels([]selection{{anchor: m.from, head: m.to, goal: -1}})
	e.edit(func() []change { return e.st.paste(text, false) })
	e.findStep(true)
}

// replaceAll replaces every match as one edit.
func (e *ed) replaceAll(with string) int {
	if len(e.matches) == 0 || e.p.ReadOnly {
		return 0
	}
	edits := make([]edit, len(e.matches))
	for i, m := range e.matches {
		edits[i] = edit{from: m.from, to: m.to, text: expand(e.re, e.search.Regex, e.st.line(m.from.line), m, with)}
	}
	n := len(edits)
	e.edit(func() []change {
		before := slices.Clone(e.st.sels)
		changes := e.st.buf.apply(edits)
		next := make([]selection, len(e.st.sels))
		for i, sel := range e.st.sels {
			next[i] = selection{anchor: mapThrough(changes, sel.anchor, false), head: mapThrough(changes, sel.head, false), goal: -1}
		}
		e.st.afterEdit(changes)
		e.st.setSels(next)
		e.st.buf.record(changes, before, slices.Clone(e.st.sels), kindOther)
		return changes
	})
	return n
}

// setMarks resolves the app's diagnostics and decorations.
func (e *ed) setMarks(diags []Diagnostic, decos []Decoration) {
	e.marks = e.marks[:0]
	for i := range diags {
		d := diags[i]
		from, to := e.pos(d.Range.Start), e.pos(d.Range.End)
		if from == to {
			// An empty range marks the character after it, or before it
			// at a line's end.
			if l := e.st.line(from.line); from.col < len(l) {
				to = e.st.nextChar(from)
			} else if from.col > 0 {
				from = e.st.prevChar(from)
			}
		}
		e.marks = append(e.marks, mark{from: from, to: to, diag: &d})
	}
	for i := range decos {
		d := decos[i]
		e.marks = append(e.marks, mark{from: e.pos(d.Range.Start), to: e.pos(d.Range.End), deco: &d})
	}
	sort.SliceStable(e.marks, func(i, j int) bool { return e.marks[i].from.less(e.marks[j].from) })
	e.marksRev.Update(inc)
}

// fold folds or unfolds the fold starting on line, or the innermost one
// around it.
func (e *ed) fold(line int, fold bool) {
	s := e.st
	if !fold {
		before := len(s.folded)
		s.folded = slices.DeleteFunc(s.folded, func(f foldRange) bool { return f.start == line || f.start < line && line <= f.end })
		if len(s.folded) != before {
			e.foldsChanged()
		}
		return
	}
	var best *foldRange
	for i, f := range s.foldCandidates() {
		if f.start == line {
			best = &s.foldCandidates()[i]
			break
		}
		if f.start < line && line <= f.end && (best == nil || f.start > best.start) {
			best = &s.foldCandidates()[i]
		}
	}
	if best == nil || slices.Contains(s.folded, *best) {
		return
	}
	s.folded = append(s.folded, *best)
	// Carets inside it go to the end of its first line.
	sels := slices.Clone(s.sels)
	for i, sel := range sels {
		if sel.head.line > best.start && sel.head.line <= best.end {
			sels[i] = caret(pos{best.start, len(s.line(best.start))})
		}
	}
	s.sels = sels
	e.foldsChanged()
}

func (e *ed) foldsChanged() {
	e.st.relayout()
	e.textRev.Update(inc)
	e.selRev.Update(inc)
	if e.p.OnFoldChange != nil {
		e.p.OnFoldChange(e.folds())
	}
}

func (e *ed) folds() []FoldRange {
	out := make([]FoldRange, len(e.st.folded))
	for i, f := range e.st.folded {
		out[i] = FoldRange{f.start, f.end}
	}
	return out
}

// requestCompletion asks the app for completions at the primary caret.
func (e *ed) requestCompletion(trigger CompletionTrigger, char string) {
	if e.p.OnCompletionRequest == nil {
		return
	}
	h := e.st.primary().head
	a, _ := e.st.lang.wordAt(e.st.line(h.line), h.col)
	if trigger == CompletionCharacter {
		a = h.col
	}
	e.nextID++
	e.comp = completion{id: e.nextID, start: pos{h.line, a}}
	e.compRev.Update(inc)
	e.p.OnCompletionRequest(CompletionRequest{ID: e.nextID, Position: e.position(h), Query: e.st.line(h.line)[a:h.col],
		Trigger: trigger, Character: char})
}

// checkCompletion ends the session when the caret left its word.
func (e *ed) checkCompletion() {
	if e.comp.id == 0 {
		return
	}
	p := e.st.primary()
	h := p.head
	if !p.empty() || len(e.st.sels) > 1 || h.line != e.comp.start.line || h.col < e.comp.start.col {
		e.endCompletion()
		return
	}
	for _, r := range e.st.line(h.line)[e.comp.start.col:h.col] {
		if !e.st.lang.wordChar(r) {
			e.endCompletion()
			return
		}
	}
	if e.comp.partial {
		id := e.comp.id
		e.requestCompletion(CompletionTyping, "")
		_ = id
		return
	}
	e.compSel = 0
	e.compRev.Update(inc)
}

func (e *ed) endCompletion() {
	if e.comp.id != 0 {
		e.comp = completion{}
		e.compRev.Update(inc)
	}
}

// query is the session's query: from its start to the caret.
func (e *ed) query() string {
	h := e.st.primary().head
	if e.comp.id == 0 || h.line != e.comp.start.line || h.col < e.comp.start.col {
		return ""
	}
	return e.st.line(h.line)[e.comp.start.col:h.col]
}

// filtered is the items matching the query, best first: prefix matches,
// then the most compact subsequence, then their sort text.
func (e *ed) filtered() ([]CompletionItem, [][]int) {
	q := strings.ToLower(e.query())
	type scored struct {
		item   CompletionItem
		prefix bool
		gaps   int
		hits   []int
	}
	var list []scored
	for _, it := range e.comp.items {
		text := it.FilterText
		if text == "" {
			text = it.Label
		}
		hits, gaps, ok := subsequence(strings.ToLower(text), q)
		if !ok {
			continue
		}
		if it.FilterText != "" {
			hits, _, _ = subsequence(strings.ToLower(it.Label), q)
		}
		list = append(list, scored{it, strings.HasPrefix(strings.ToLower(text), q), gaps, hits})
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.prefix != b.prefix {
			return a.prefix
		}
		if a.gaps != b.gaps {
			return a.gaps < b.gaps
		}
		return strings.ToLower(sortText(a.item)) < strings.ToLower(sortText(b.item))
	})
	items := make([]CompletionItem, len(list))
	hits := make([][]int, len(list))
	for i, s := range list {
		items[i], hits[i] = s.item, s.hits
	}
	return items, hits
}

func sortText(it CompletionItem) string {
	if it.SortText != "" {
		return it.SortText
	}
	return it.Label
}

// subsequence finds q's characters in order in text, the most compact
// match: the bytes matched and the characters between them.
func subsequence(text, q string) ([]int, int, bool) {
	if q == "" {
		return nil, 0, true
	}
	var best []int
	bestGaps := -1
	for start := 0; start < len(text); start++ {
		if r, _ := utf8.DecodeRuneInString(text[start:]); r != firstRune(q) {
			continue
		}
		var hits []int
		at := start
		for _, c := range q {
			i := strings.IndexRune(text[at:], c)
			if i < 0 {
				hits = nil
				break
			}
			hits = append(hits, at+i)
			at += i + utf8.RuneLen(c)
		}
		if hits == nil {
			break
		}
		gaps := hits[len(hits)-1] - hits[0] - len(hits) + 1
		if bestGaps < 0 || gaps < bestGaps {
			best, bestGaps = hits, gaps
		}
	}
	return best, bestGaps, bestGaps >= 0
}

// accept puts item in place of its range, at the primary caret and every
// caret after the same text, as one undo step.
func (e *ed) accept(item CompletionItem) {
	text := item.InsertText
	if text == "" {
		text = item.Label
	}
	start := e.comp.start
	h := e.st.primary().head
	if item.Range != nil {
		start = e.pos(item.Range.Start)
	}
	prefix := e.st.r().slice(start, h)
	var edits []edit
	for _, sel := range e.st.sels {
		c := sel.head
		from := pos{c.line, c.col - len(prefix)}
		if from.col >= 0 && e.st.r().slice(from, c) == prefix {
			edits = append(edits, edit{from: from, to: c, text: text})
		}
	}
	e.endCompletion()
	e.edit(func() []change {
		before := slices.Clone(e.st.sels)
		e.st.buf.open = false
		changes := e.st.buf.apply(edits)
		next := make([]selection, len(e.st.sels))
		for i, sel := range e.st.sels {
			next[i] = caret(mapThrough(changes, sel.head, false))
		}
		e.st.afterEdit(changes)
		e.st.setSels(next)
		e.st.buf.record(changes, before, slices.Clone(e.st.sels), kindOther)
		return changes
	})
	if e.p.OnCompletionAccept != nil {
		e.p.OnCompletionAccept(item)
	}
}

// requestDefinition asks the app for the definition of the word at p.
func (e *ed) requestDefinition(p pos, trigger DefinitionTrigger) {
	if e.p.OnDefinitionRequest == nil {
		return
	}
	l := e.st.line(p.line)
	a, b := e.st.lang.wordAt(l, p.col)
	if a == b {
		return
	}
	e.nextID++
	e.defID = e.nextID
	e.hoverWord = span{pos{p.line, a}, pos{p.line, b}}
	e.p.OnDefinitionRequest(DefinitionRequest{ID: e.nextID, Position: e.position(p), Word: e.rangeOf(e.hoverWord), Trigger: trigger})
}

func (e *ed) hideLink() {
	e.defID = 0
	if e.hasLink.GetUntracked() {
		e.hasLink.Set(false)
	}
}

// pointer is a hover move at pt in the rows' box, with mods held.
func (e *ed) pointer(pt core.Point, page core.Point, mods core.Modifiers) {
	p := e.posAt(pt)
	l := e.st.line(p.line)
	a, b := e.st.lang.wordAt(l, p.col)
	word := span{pos{p.line, a}, pos{p.line, b}}
	cmd := mods.Control
	if core.Untracked(e.cx, e.cx.Env().AppleKeys) {
		cmd = mods.Platform
	}
	switch {
	case !cmd || mods.Alt || a == b:
		e.hideLink()
	case e.hasLink.GetUntracked() && !p.less(e.link.GetUntracked().from) && p.less(e.link.GetUntracked().to):
	case word != e.hoverWord || e.defID == 0:
		e.hideLink()
		e.requestDefinition(p, DefinitionHover)
	}
	if e.p.OnHover == nil {
		return
	}
	if e.hoverStop != nil {
		e.hoverStop()
	}
	e.hoverStop = e.cx.AfterFunc(hoverDelay, func() {
		var diags []Diagnostic
		for _, m := range e.marks {
			if m.diag != nil && !p.less(m.from) && p.less(m.to) {
				diags = append(diags, *m.diag)
			}
		}
		var bounds core.Bounds
		if rs := e.rowRects(word.from, word.to, e.st.disp.rowOf(word.from), e.st.disp.rowOf(word.from)+1, nil, "", nil); len(rs) > 0 {
			b, _ := e.boxOf(rs[0])
			bounds = b.Page
		}
		e.p.OnHover(HoverEvent{Position: e.position(p), Offset: e.offset(p), Word: e.rangeOf(word), At: page, Bounds: bounds,
			Modifiers: mods, Diagnostics: diags})
	})
}

func (e *ed) pointerLeft() {
	if e.hoverStop != nil {
		e.hoverStop()
		e.hoverStop = nil
	}
	e.hideLink()
	if e.p.OnHoverEnd != nil {
		e.p.OnHoverEnd()
	}
}

// pressDown places the caret at pt: a word with two clicks, a line with
// three, extending with Shift, adding a caret with Alt.
func (e *ed) pressDown(pt core.Point, clicks int, mods core.Modifiers) {
	e.endCompletion()
	e.st.buf.open = false
	p := e.posAt(pt)
	sel := caret(p)
	e.unit = max(min(clicks, 3), 1)
	switch e.unit {
	case 2:
		a, b := e.st.lang.wordAt(e.st.line(p.line), p.col)
		sel = selection{anchor: pos{p.line, a}, head: pos{p.line, b}, goal: -1}
	case 3:
		end := pos{p.line + 1, 0}
		if end.line >= e.st.r().lines() {
			end = e.st.r().end()
		}
		sel = selection{anchor: pos{p.line, 0}, head: end, goal: -1}
	}
	e.pressAt = sel
	e.block = mods.Alt && mods.Shift || mods.Control && mods.Alt && core.Untracked(e.cx, e.cx.Env().OS) == core.OSLinux
	if e.block {
		// A block from the press's x, as the pointer drags it.
		e.blockAt = pt
		e.st.setSels(e.blockSels(pt))
		e.moved()
		return
	}
	switch {
	case mods.Shift:
		pr := e.st.primary()
		e.st.setSels(append(slices.Clone(e.st.sels[:len(e.st.sels)-1]), selection{anchor: pr.anchor, head: p, goal: -1}))
		e.pressAt = selection{anchor: pr.anchor, head: pr.anchor, goal: -1}
	case mods.Alt:
		e.st.setSels(append(slices.Clone(e.st.sels), sel))
	default:
		e.st.setSels([]selection{sel})
	}
	e.moved()
	cmd := mods.Control
	if core.Untracked(e.cx, e.cx.Env().AppleKeys) {
		cmd = mods.Platform
	}
	if cmd && !mods.Alt {
		e.requestDefinition(p, DefinitionPress)
	}
}

// dragTo extends the press's selection to pt, by its unit.
func (e *ed) dragTo(pt core.Point) {
	if e.block {
		e.st.setSels(e.blockSels(pt))
		e.selMoved()
		return
	}
	p := e.posAt(pt)
	a := e.pressAt
	head := p
	anchor := a.anchor
	switch e.unit {
	case 2:
		x, y := e.st.lang.wordAt(e.st.line(p.line), p.col)
		if p.less(a.start()) {
			head, anchor = pos{p.line, x}, a.end()
		} else {
			head, anchor = pos{p.line, y}, a.start()
		}
	case 3:
		if p.less(a.start()) {
			head, anchor = pos{p.line, 0}, a.end()
		} else {
			head = pos{p.line + 1, 0}
			if head.line >= e.st.r().lines() {
				head = e.st.r().end()
			}
			anchor = a.start()
		}
	}
	sels := slices.Clone(e.st.sels)
	sels[len(sels)-1] = selection{anchor: anchor, head: head, goal: -1}
	e.st.setSels(sels)
	e.selMoved()
}

// dragMove is a drag's move to pt in the rows' box: it extends the
// selection there and, while the pointer is near or past the viewport's
// edges, scrolls on and extends it again, faster the farther out.
func (e *ed) dragMove(pt core.Point) {
	e.dragging = true
	e.dragAt = core.Point{X: pt.X - e.scrollX.GetUntracked(), Y: pt.Y - e.scrollY.GetUntracked()}
	e.dragTo(pt)
	if e.dragStop == nil {
		e.edgeScroll()
	}
}

func (e *ed) dragEnd() {
	e.dragging = false
	e.recordSelections()
	if e.dragStop != nil {
		e.dragStop()
		e.dragStop = nil
	}
}

// edgeScroll scrolls a tick while the drag is near an edge.
func (e *ed) edgeScroll() {
	e.dragStop = nil
	if !e.dragging {
		return
	}
	vp := e.viewport.GetUntracked()
	dy := edgeSpeed(float32(e.dragAt.Y), float32(vp.Height))
	dx := float32(0)
	if !e.p.SoftWrap {
		dx = edgeSpeed(float32(e.dragAt.X), float32(vp.Width)-e.gutterWidth())
	}
	if dx == 0 && dy == 0 {
		return
	}
	y := float32(e.scrollY.GetUntracked())
	toY := max(min(y+dy, e.contentHeight()-float32(vp.Height)), 0)
	x := float32(e.scrollX.GetUntracked())
	toX := max(min(x+dx, core.Untracked(e.cx, e.boxWidth)-(float32(vp.Width)-e.gutterWidth())), 0)
	if toY != y {
		e.vs.ScrollTo(core.Point{Y: core.Points(toY)}, false)
		e.scrollY.Set(core.Points(toY))
	}
	if toX != x {
		e.hs.ScrollTo(core.Point{X: core.Points(toX)}, false)
		e.scrollX.Set(core.Points(toX))
	}
	e.dragTo(core.Point{X: e.dragAt.X + core.Points(toX), Y: e.dragAt.Y + core.Points(toY)})
	e.dragStop = e.cx.AfterFunc(edgeTick, e.edgeScroll)
}

// edgeSpeed is how far a tick scrolls with the pointer at at along a
// viewport length long: from edgeMin inside the edge's band to edgeMax
// far past it, negative toward the start.
func edgeSpeed(at, length float32) float32 {
	if length <= 0 {
		return 0
	}
	inner := min(float32(edgeInner), length/4)
	ramp := func(past float32) float32 {
		t := min(past/(inner+edgeRamp), 1)
		return edgeMin + t*(edgeMax-edgeMin)
	}
	switch {
	case at > length-inner:
		return ramp(at - (length - inner))
	case at < inner:
		return -ramp(inner - at)
	}
	return 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func isWordStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }

// waitInline asks for an inline completion once typing pauses: one wait,
// whatever the number of keystrokes.
func (e *ed) waitInline() {
	if e.p.OnInlineCompletionRequest == nil {
		return
	}
	if e.inlineStop != nil {
		e.inlineStop()
	}
	d := e.p.InlineCompletionDelay
	if d <= 0 {
		d = inlineDelay
	}
	e.inlineStop = e.cx.AfterFunc(d, func() {
		e.inlineStop = nil
		if e.comp.shown && e.comp.id != 0 {
			return
		}
		e.nextID++
		e.inlineID = e.nextID
		h := e.st.primary().head
		e.p.OnInlineCompletionRequest(InlineCompletionRequest{ID: e.inlineID, Offset: e.offset(h), Position: e.position(h)})
	})
}

// hideInline clears the inline completion and drops its request.
func (e *ed) hideInline() {
	e.inlineID = 0
	if e.inlineStop != nil {
		e.inlineStop()
		e.inlineStop = nil
	}
	if e.inline.GetUntracked() != "" {
		e.inline.Set("")
	}
}

// acceptInline inserts the inline completion at the caret as one undo
// step.
func (e *ed) acceptInline() {
	text := e.inline.GetUntracked()
	e.hideInline()
	if text == "" || e.p.ReadOnly {
		return
	}
	e.st.buf.open = false
	e.edit(func() []change { return e.st.insert(text, false) })
	e.st.buf.open = false
}

// offset is p in UTF-16 units of the text.
func (e *ed) offset(p pos) int {
	r := e.st.r()
	n := 0
	for i := range p.line {
		n += utf16Col(r.line(i), len(r.line(i))) + 1
	}
	return n + utf16Col(r.line(p.line), p.col)
}

// stylesMemo holds the parts' styles: styles, or what fn returns as it
// changes. Reading it is tracked, so a part's style follows.
func stylesMemo(cx *core.Scope, styles Styles, fn func() Styles) *core.Memo[*Styles] {
	return core.NewMemoFunc(cx, func() *Styles {
		if fn == nil {
			return &styles
		}
		s := fn()
		return &s
	}, nil)
}

// styles are the parts' styles, read where a part is styled.
func (e *ed) styles() *Styles { return e.looks.Get() }
