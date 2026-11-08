package richeditor

import (
	"fmt"
	"regexp"
	"slices"
	"unicode"
)

// SessionConfig makes a Session: Extensions make its schema and plugins, or
// Schema and Plugins give them. Content is a Markdown text, unless Doc gives
// the document or JSON Tiptap's.
type SessionConfig struct {
	Extensions []Extension
	Schema     *Schema
	Plugins    []*Plugin
	Doc        *Node
	Markdown   string
	JSON       string
	HTML       string
	// ReadOnly keeps the user from changing it.
	ReadOnly bool
}

// Session is the one live editor a view and the app share, on the engine
// goroutine as the reactive graph is: it holds the state, applies
// transactions, runs commands and tells its subscribers.
type Session struct {
	state    *State
	commands map[string]CommandFactory
	editable bool
	// DispatchTransaction, when set, takes every transaction in place of
	// Dispatch's applying it, the app committing with UpdateState.
	DispatchTransaction func(tr *Transaction)
	subs                []*func(*State, []*Transaction)
	views               []*viewHandle
}

// viewHandle is what a mounted view gives its session.
type viewHandle struct {
	focus, blur func()
	geometry    Geometry
}

// Geometry answers where positions are, from the last view mounted.
type Geometry interface {
	// CoordsAtPos is the caret's box at pos in the content box, the box
	// that scrolls.
	CoordsAtPos(pos int) (Rect2, bool)
	// PosAtCoords is the position nearest a point of the content box.
	PosAtCoords(x, y float64) (int, bool)
	// SelectionRect is the box over the selection's lines.
	SelectionRect() (Rect2, bool)
}

// Rect2 is a box in points.
type Rect2 struct{ X, Y, Width, Height float64 }

// NewSession makes a session.
func NewSession(c SessionConfig) (*Session, error) {
	s := &Session{editable: !c.ReadOnly, commands: map[string]CommandFactory{}}
	schema, plugins := c.Schema, c.Plugins
	if len(c.Extensions) > 0 {
		if schema != nil {
			return nil, fmt.Errorf("richeditor: a session takes extensions or a schema, not both")
		}
		built, err := Build(c.Extensions...)
		if err != nil {
			return nil, err
		}
		schema = built.Schema
		s.commands = built.Commands
		plugins = append(built.Plugins, plugins...)
	}
	doc := c.Doc
	var err error
	switch {
	case doc != nil:
	case c.JSON != "":
		doc, err = DocFromJSONString(schema, c.JSON)
	case c.HTML != "":
		doc, err = ParseHTML(schema, c.HTML)
	case c.Markdown != "":
		doc, err = ParseMarkdown(schema, c.Markdown)
	}
	if err != nil {
		return nil, err
	}
	s.state, err = NewState(StateConfig{Schema: schema, Doc: doc, Plugins: plugins})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// State is the state now.
func (v *Session) State() *State { return v.state }

// Editable says the user can change the document.
func (v *Session) Editable() bool { return v.editable }

// SetEditable sets whether the user can change the document.
func (v *Session) SetEditable(on bool) {
	if v.editable != on {
		v.editable = on
		v.notify(nil)
	}
}

// Dispatch applies tr, or hands it to DispatchTransaction.
func (v *Session) Dispatch(tr *Transaction) {
	if v.DispatchTransaction != nil {
		v.DispatchTransaction(tr)
		return
	}
	v.Apply(tr)
}

// Apply applies tr at once, dropping one that fails: the document stays as
// it was.
func (v *Session) Apply(tr *Transaction) error {
	n, trs, err := v.state.ApplyTransaction(tr)
	if err != nil {
		return err
	}
	if n != v.state {
		v.state = n
		v.notify(trs)
	}
	return nil
}

// UpdateState replaces the state.
func (v *Session) UpdateState(s *State) {
	if s != v.state {
		v.state = s
		v.notify(nil)
	}
}

func (v *Session) notify(trs []*Transaction) {
	for _, f := range slices.Clone(v.subs) {
		(*f)(v.state, trs)
	}
}

// Subscribe calls f after every change, and the returned function stops it.
func (v *Session) Subscribe(f func(s *State, trs []*Transaction)) func() {
	p := &f
	v.subs = append(v.subs, p)
	return func() {
		v.subs = slices.DeleteFunc(v.subs, func(o *func(*State, []*Transaction)) bool { return o == p })
	}
}

// Run runs a command, and says whether it applied.
func (v *Session) Run(c Command) bool { return c(v.state, v.Dispatch) }

// Can says a command would apply.
func (v *Session) Can(c Command) bool { return c(v.state, nil) }

// Command is the named command with args, nil for an unknown name.
func (v *Session) Command(name string, args Attrs) Command {
	if f := v.commands[name]; f != nil {
		return f(args)
	}
	return nil
}

// RunNamed runs a named command.
func (v *Session) RunNamed(name string, args Attrs) bool {
	c := v.Command(name, args)
	return c != nil && v.Run(c)
}

// CanNamed says a named command would apply.
func (v *Session) CanNamed(name string, args Attrs) bool {
	c := v.Command(name, args)
	return c != nil && v.Can(c)
}

// IsActive says the node or mark of name is at the selection.
func (v *Session) IsActive(name string, attrs Attrs) bool { return IsActive(v.state, name, attrs) }

// Markdown, JSON, HTML and Text are the document in those forms.
func (v *Session) Markdown() string { return WriteMarkdown(v.state.Doc) }
func (v *Session) JSON() string     { return DocJSONString(v.state.Doc) }
func (v *Session) HTML() string     { return WriteHTML(v.state.Doc) }
func (v *Session) Text() string     { return PlainText(v.state.Doc) }

// SetContent replaces the document, kept out of the history.
func (v *Session) SetContent(doc *Node) {
	tr := v.state.Tr().ReplaceWith(0, v.state.Doc.Content.Size, doc.Content.Content...)
	tr.SetMeta(addToHistory, false)
	tr.Origin = OriginAPI
	v.Dispatch(tr)
}

// Focus and Blur are requests to the views showing it.
func (v *Session) Focus() {
	for _, h := range v.views {
		h.focus()
	}
}

// Blur asks its views to give the keyboard back.
func (v *Session) Blur() {
	for _, h := range v.views {
		h.blur()
	}
}

func (v *Session) geometry() Geometry {
	if n := len(v.views); n > 0 {
		return v.views[n-1].geometry
	}
	return nil
}

// CoordsAtPos is the caret's box at pos in the view's content box, false
// with no view; it panics on a position outside the document, as Resolve.
func (v *Session) CoordsAtPos(pos int) (Rect2, bool) {
	v.state.Doc.Resolve(pos)
	if g := v.geometry(); g != nil {
		return g.CoordsAtPos(pos)
	}
	return Rect2{}, false
}

// PosAtCoords is the position nearest a point of the content box.
func (v *Session) PosAtCoords(x, y float64) (int, bool) {
	if g := v.geometry(); g != nil {
		return g.PosAtCoords(x, y)
	}
	return 0, false
}

// SelectionRect is the box over the selection's lines.
func (v *Session) SelectionRect() (Rect2, bool) {
	if g := v.geometry(); g != nil {
		return g.SelectionRect()
	}
	return Rect2{}, false
}

// TypeText types text as a user does: input rules first, then the stored
// marks, tagged with an input method's composition.
func (v *Session) TypeText(text string, composition any) {
	st := v.state
	from, to := st.Selection.From(), st.Selection.To()
	if composition == nil {
		for _, p := range st.Plugins {
			if p.HandleTextInput != nil && p.HandleTextInput(v, from, to, text) {
				return
			}
		}
	}
	tr := st.Tr()
	if _, ok := st.Selection.(*TextSelection); ok {
		tr.InsertText(text, from, to)
	} else {
		tr.InsertText(text, -1, -1)
	}
	tr.Origin = OriginInput
	if composition != nil {
		tr.SetMeta(compositionID, composition)
	}
	v.Dispatch(tr.ScrollIntoView())
}

// HandleKey offers a key, by its name ("Mod-b", "Shift-Enter"), to the
// plugins, the first taking it.
func (v *Session) HandleKey(name string) bool {
	if !v.editable {
		return false
	}
	for _, p := range v.state.Plugins {
		if p.HandleKeyDown != nil && p.HandleKeyDown(v, name) {
			return true
		}
	}
	return false
}

// Paste pastes text, HTML when html is not empty, as the plugins allow.
func (v *Session) Paste(text, html string) {
	st := v.state
	r := st.Selection.FromPos()
	for _, p := range st.Plugins {
		if p.TransformPastedText != nil {
			text = p.TransformPastedText(v, text)
		}
	}
	var slice *Slice
	if html != "" && !r.Parent().Type.Spec.Code {
		if doc, err := ParseHTML(st.Schema, html); err == nil && doc.Content.Size > 0 {
			slice = MaxOpen(doc.Content, false)
		}
	}
	if slice == nil {
		slice = pastedText(st.Schema, text, r)
	}
	for _, p := range st.Plugins {
		if p.TransformPasted != nil {
			slice = p.TransformPasted(v, slice)
		}
	}
	for _, p := range st.Plugins {
		if p.HandlePaste != nil && p.HandlePaste(v, slice) {
			return
		}
	}
	tr := st.Tr().ReplaceSelection(slice)
	tr.Origin = OriginPaste
	tr.SetMeta(pasteMeta, true)
	v.Dispatch(tr.ScrollIntoView())
}

// pastedText reads text as Markdown when it has block or mark syntax, as the
// markdown plugin does, keeping plain text as lines.
func pastedText(s *Schema, text string, r *ResolvedPos) *Slice {
	if !r.Parent().Type.Spec.Code && looksLikeMarkdown(text) {
		if doc, err := ParseMarkdown(s, text); err == nil {
			return MaxOpen(doc.Content, false)
		}
	}
	return SliceFromText(s, text, r)
}

var markdownHint = regexp.MustCompile("(?m)^(#{1,6} |> |[-*+] |\\d+\\. |```|\\|.*\\|\\s*$)|\\*\\*[^*]+\\*\\*|`[^`]+`|\\[[^\\]]+\\]\\([^)]+\\)")

func looksLikeMarkdown(text string) bool { return markdownHint.MatchString(text) }

// Copy is the selection as text and HTML, as it goes to the clipboard.
func (v *Session) Copy() (text, html string) {
	slice := v.state.Selection.Content()
	return SliceToText(slice), WriteHTML(&Node{Type: v.state.Schema.TopNode, Content: slice.Content})
}

// Cut copies and deletes the selection.
func (v *Session) Cut() (text, html string) {
	text, html = v.Copy()
	if !v.state.Selection.Empty() {
		v.Dispatch(v.state.Tr().DeleteSelection().ScrollIntoView())
	}
	return text, html
}

// MoveChar moves the head a grapheme by dir, extending with extend.
func (v *Session) MoveChar(dir int, extend bool) {
	v.setHead(v.charFrom(v.state.Selection.HeadPos(), dir), extend, dir)
}

func (v *Session) charFrom(r *ResolvedPos, dir int) int {
	sel := v.state.Selection
	if _, ok := sel.(*NodeSelection); ok || !sel.Empty() {
		if dir < 0 {
			return sel.From()
		}
		return sel.To()
	}
	if !r.Parent().InlineContent() {
		return r.Pos
	}
	if dir < 0 {
		if r.ParentOffset == 0 {
			return SelectionPosNext(r, -1)
		}
		text := r.Parent().TextBetween(0, r.ParentOffset, "", "￼")
		return r.Pos - clusterBefore(text, utf16Len(text))
	}
	if r.ParentOffset == r.Parent().Content.Size {
		return SelectionPosNext(r, 1)
	}
	text := r.Parent().TextBetween(r.ParentOffset, r.Parent().Content.Size, "", "￼")
	return r.Pos + clusterAfter(text)
}

// SelectionPosNext is the caret of the next textblock going dir.
func SelectionPosNext(r *ResolvedPos, dir int) int {
	doc := r.Doc()
	var pos *ResolvedPos
	if dir < 0 {
		if r.Depth == 0 {
			return r.Pos
		}
		pos = doc.Resolve(r.Before(r.Depth))
	} else {
		if r.Depth == 0 {
			return r.Pos
		}
		pos = doc.Resolve(r.After(r.Depth))
	}
	if s := SelectionFindFrom(pos, dir, true); s != nil {
		return s.Head()
	}
	return r.Pos
}

func clusterAfter(text string) int {
	end := 0
	total := utf16Len(text)
	for end < total {
		next := end + 1
		if b := byteAt(text, next); b < len(text) {
			// A surrogate pair and what joins it stay one.
			for byteAt(text, next) == byteAt(text, end) {
				next++
			}
		}
		end = next
		rest := text[byteAt(text, end):]
		if rest == "" {
			break
		}
		r := []rune(rest)[0]
		before := []rune(text[:byteAt(text, end)])
		if r != 0x200d && !unicode.Is(unicode.Mn, r) && !isVariation(r) && before[len(before)-1] != 0x200d {
			break
		}
	}
	return end
}

// MoveWord moves the head a word by dir.
func (v *Session) MoveWord(dir int, extend bool) {
	r := v.state.Selection.HeadPos()
	if !r.Parent().InlineContent() {
		return
	}
	text := []rune(r.Parent().TextBetween(0, r.Parent().Content.Size, "", "￼"))
	at := len([]rune(r.Parent().TextBetween(0, r.ParentOffset, "", "￼")))
	word := func(c rune) bool { return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' }
	i := at
	if dir < 0 {
		if i == 0 {
			v.setHead(SelectionPosNext(r, -1), extend, dir)
			return
		}
		for i > 0 && !word(text[i-1]) {
			i--
		}
		for i > 0 && word(text[i-1]) {
			i--
		}
	} else {
		if i == len(text) {
			v.setHead(SelectionPosNext(r, 1), extend, dir)
			return
		}
		for i < len(text) && !word(text[i]) {
			i++
		}
		for i < len(text) && word(text[i]) {
			i++
		}
	}
	v.setHead(r.Start(r.Depth)+utf16Len(string(text[:i])), extend, dir)
}

// setHead moves the head to pos.
func (v *Session) setHead(pos int, extend bool, dir int) {
	st := v.state
	doc := st.Doc
	if extend {
		v.Dispatch(st.Tr().SetSelection(TextSelectionBetween(st.Selection.AnchorPos(), doc.Resolve(pos), dir)).ScrollIntoView())
		return
	}
	r := doc.Resolve(pos)
	if !r.Parent().InlineContent() {
		v.Dispatch(st.Tr().SetSelection(SelectionNear(r, dir)).ScrollIntoView())
		return
	}
	v.Dispatch(st.Tr().SetSelection(NewTextSelection(r, nil)).ScrollIntoView())
}
