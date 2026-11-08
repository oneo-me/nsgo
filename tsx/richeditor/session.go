package richeditor

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/oneo-me/nsgo/base/richeditor"
)

// RichEditorOptions are createRichEditor's: the extensions, StarterKit and
// TableKit when none, the document as Markdown, HTML or Tiptap's JSON, and
// plugins after the extensions'.
type RichEditorOptions struct {
	Extensions []*Extension
	Markdown   string
	HTML       string
	JSON       map[string]any
	Plugins    []RichPluginSpec
	// Editable false keeps the user from changing the document.
	Editable *bool
	// OpenLink opens a link pressed with Mod, or in a read-only editor.
	OpenLink func(href string)
}

// Editor is RichEditorSession: the session a RichEditor shows and the app
// drives, its document and selection read on demand.
type Editor struct {
	s       *richeditor.Session
	stop    func()
	focused bool
	// handlers are the listeners by event, nil once removed.
	handlers map[string][]*func(RichEditorEvent)
	last     *richeditor.State
	closed   bool
	open     func(href string)
	// check and problems are what validate reports.
	check    []checked
	problems []string
}

// RichEditorEvent is what an editor's events carry.
type RichEditorEvent struct {
	Revision   int
	DocChanged bool
	Origin     string
}

// RichSelection is a selection as JS reads it: its kind ("text", "node",
// "all", "cell", "gap") and positions.
type RichSelection struct {
	Type                   string
	Anchor, Head, From, To int
	Empty                  bool
}

// RichRect is a box in points.
type RichRect struct{ X, Y, Width, Height float64 }

// RichSuggestion is an active suggestion's, as a mention list reads it.
type RichSuggestion struct {
	Active   bool
	Query    string
	From, To int
	LastKey  string
	KeyCount int
}

var origins = []string{"user", "input", "paste", "drop", "remote", "history", "plugin", "api"}

func originName(o richeditor.Origin) string {
	if int(o) < len(origins) {
		return origins[o]
	}
	return "user"
}

// createRichEditor makes an editor.
func createRichEditor(o RichEditorOptions) (*Editor, error) {
	exts := o.Extensions
	if len(exts) == 0 {
		exts = []*Extension{StarterKit(nil), TableKit(nil)}
	}
	e := &Editor{handlers: map[string][]*func(RichEditorEvent){}, open: o.OpenLink}
	var list []richeditor.Extension
	for _, x := range exts {
		if x == nil {
			return nil, errors.New("createRichEditor: an extension is missing")
		}
		made, err := x.make(e, x.options)
		if err != nil {
			return nil, fmt.Errorf("createRichEditor: %s: %w", x.name, err)
		}
		list = append(list, made...)
	}
	plugins := make([]*richeditor.Plugin, len(o.Plugins))
	for i, p := range o.Plugins {
		plugins[i] = p.plugin(e, fmt.Sprintf("plugin%d", i))
	}
	c := richeditor.SessionConfig{Extensions: list, Plugins: plugins, Markdown: o.Markdown, HTML: o.HTML, ReadOnly: o.Editable != nil && !*o.Editable}
	if o.JSON != nil {
		b, err := json.Marshal(o.JSON)
		if err != nil {
			return nil, fmt.Errorf("createRichEditor: %w", err)
		}
		c.JSON = string(b)
	}
	s, err := richeditor.NewSession(c)
	if err != nil {
		return nil, fmt.Errorf("createRichEditor: %w", err)
	}
	e.s, e.last = s, s.State()
	if err := e.validate(); err != nil {
		return nil, err
	}
	e.stop = s.Subscribe(e.changed)
	return e, nil
}

func (e *Editor) changed(st *richeditor.State, trs []*richeditor.Transaction) {
	prev := e.last
	e.last = st
	ev := RichEditorEvent{Revision: st.Revision, DocChanged: prev.Doc != st.Doc}
	if len(trs) > 0 {
		ev.Origin = originName(trs[0].Origin)
		e.emit("transaction", ev)
	}
	e.emit("revision", ev)
	if ev.DocChanged {
		e.emit("update", ev)
	}
	if !prev.Selection.Eq(st.Selection) {
		e.emit("selectionUpdate", ev)
	}
}

func (e *Editor) emit(name string, ev RichEditorEvent) {
	for _, h := range slices.Clone(e.handlers[name]) {
		(*h)(ev)
	}
}

// Session is the Go session, for a Go component the app hands it to.
func (e *Editor) session() *richeditor.Session { return e.s }

// On listens to an event (revision, update, selectionUpdate, transaction,
// focus, blur, destroy) and returns what stops it.
func (e *Editor) On(event string, handler func(RichEditorEvent)) func() {
	h := &handler
	e.handlers[event] = append(e.handlers[event], h)
	return func() {
		e.handlers[event] = slices.DeleteFunc(e.handlers[event], func(o *func(RichEditorEvent)) bool { return o == h })
	}
}

// Run runs a command and says whether it applied; Can says whether it
// would.
func (e *Editor) Run(c *Command) (bool, error) {
	cmd, err := c.resolve(e)
	if err != nil {
		return false, err
	}
	return e.s.Run(cmd), nil
}

// Can says whether a command would apply.
func (e *Editor) Can(c *Command) (bool, error) {
	cmd, err := c.resolve(e)
	if err != nil {
		return false, err
	}
	return e.s.Can(cmd), nil
}

// IsActive says whether the node or mark of name, with attrs, is at the
// selection.
func (e *Editor) IsActive(name string, attrs map[string]any) bool { return e.s.IsActive(name, attrs) }

// GetAttributes are the attributes of the mark or node of name at the
// selection, empty for none.
func (e *Editor) GetAttributes(name string) map[string]any {
	st := e.s.State()
	if t := st.Schema.Marks[name]; t != nil {
		sel := st.Selection
		marks := sel.FromPos().MarksHere()
		if st.StoredMarks != nil {
			marks = st.StoredMarks
		}
		if m := t.IsInSet(marks); m != nil {
			return maps.Clone(m.Attrs)
		}
		if after := sel.FromPos().NodeAfter(); after != nil {
			if m := t.IsInSet(after.Marks); m != nil {
				return maps.Clone(m.Attrs)
			}
		}
		return map[string]any{}
	}
	if t := st.Schema.Nodes[name]; t != nil {
		if n, ok := st.Selection.(*richeditor.NodeSelection); ok && n.Node.Type == t {
			return maps.Clone(n.Node.Attrs)
		}
		r := st.Selection.FromPos()
		for d := r.Depth; d > 0; d-- {
			if r.Node(d).Type == t {
				return maps.Clone(r.Node(d).Attrs)
			}
		}
	}
	return map[string]any{}
}

// Markdown, HTML, Text and DocJSON are the document in those forms.
func (e *Editor) Markdown() string        { return e.s.Markdown() }
func (e *Editor) HTML() string            { return e.s.HTML() }
func (e *Editor) Text() string            { return e.s.Text() }
func (e *Editor) DocJSON() map[string]any { return richeditor.NodeJSON(e.s.State().Doc) }

// SetContent replaces the document, kept out of the history: a string is
// Markdown, an object Tiptap's JSON.
func (e *Editor) SetContent(content any) error {
	schema := e.s.State().Schema
	var doc *richeditor.Node
	var err error
	switch c := content.(type) {
	case string:
		doc, err = richeditor.ParseMarkdown(schema, c)
	case map[string]any:
		doc, err = richeditor.NodeFromJSON(schema, c)
	default:
		err = errors.New("content is a Markdown string or a document's JSON")
	}
	if err != nil {
		return fmt.Errorf("setContent: %w", err)
	}
	e.s.SetContent(doc)
	return nil
}

// SetHTML replaces the document with HTML's, kept out of the history.
func (e *Editor) SetHTML(html string) error {
	doc, err := richeditor.ParseHTML(e.s.State().Schema, html)
	if err != nil {
		return fmt.Errorf("setHTML: %w", err)
	}
	e.s.SetContent(doc)
	return nil
}

// Focus and Blur are requests to the views showing it.
func (e *Editor) Focus() { e.s.Focus() }
func (e *Editor) Blur()  { e.s.Blur() }

// Revision counts the transactions applied.
func (e *Editor) Revision() int { return e.s.State().Revision }

// Editable says the user can change the document.
func (e *Editor) Editable() bool { return e.s.Editable() }

// SetEditable sets whether the user can change the document.
func (e *Editor) SetEditable(on bool) { e.s.SetEditable(on) }

// IsFocused says a view showing it has the keyboard.
func (e *Editor) IsFocused() bool { return e.focused }

func (e *Editor) setFocused(on bool) {
	if e.focused != on {
		e.focused = on
		name := "blur"
		if on {
			name = "focus"
		}
		e.emit(name, RichEditorEvent{Revision: e.s.State().Revision})
	}
}

// Selection is the selection now.
func (e *Editor) Selection() RichSelection {
	sel := e.s.State().Selection
	kind := "text"
	switch sel.(type) {
	case *richeditor.NodeSelection:
		kind = "node"
	case *richeditor.AllSelection:
		kind = "all"
	case *richeditor.CellSelection:
		kind = "cell"
	case *richeditor.GapCursor:
		kind = "gap"
	}
	return RichSelection{Type: kind, Anchor: sel.Anchor(), Head: sel.Head(), From: sel.From(), To: sel.To(), Empty: sel.Empty()}
}

// Size is the document's content size, the last position.
func (e *Editor) Size() int { return e.s.State().Doc.Content.Size }

func (e *Editor) checkPos(pos int) error {
	if size := e.s.State().Doc.Content.Size; pos < 0 || pos > size {
		return rangeError("position %d outside 0..%d", pos, size)
	}
	return nil
}

func rect(r richeditor.Rect2) RichRect { return RichRect{r.X, r.Y, r.Width, r.Height} }

// CoordsAtPos is the caret's box at pos in the editor's content box, null
// with no view showing it.
func (e *Editor) CoordsAtPos(pos int) (RichRect, bool, error) {
	if err := e.checkPos(pos); err != nil {
		return RichRect{}, false, err
	}
	r, ok := e.s.CoordsAtPos(pos)
	return rect(r), ok, nil
}

// PosAtCoords is the position nearest a point of the content box.
func (e *Editor) PosAtCoords(x, y float64) (int, bool) { return e.s.PosAtCoords(x, y) }

// SelectionRect is the box over the selection's lines.
func (e *Editor) SelectionRect() (RichRect, bool) {
	r, ok := e.s.SelectionRect()
	return rect(r), ok
}

// PageCoordsAtPos and PosAtPageCoords answer in the page's points, for
// what the app places outside the editor.
func (e *Editor) PageCoordsAtPos(pos int) (RichRect, bool, error) {
	if err := e.checkPos(pos); err != nil {
		return RichRect{}, false, err
	}
	r, ok := e.s.PageCoordsAtPos(pos)
	return rect(r), ok, nil
}

// PosAtPageCoords is the position nearest a point of the page.
func (e *Editor) PosAtPageCoords(x, y float64) (int, bool) { return e.s.PosAtPageCoords(x, y) }

// DragBlock starts a handle's drag at a point of the page, carrying the
// block at pos, DragTo moves its drop cursor and EndDrag drops it, as one
// undo step.
func (e *Editor) DragBlock(pos int, x, y float64) (bool, error) {
	if err := e.checkPos(pos); err != nil {
		return false, err
	}
	return e.s.DragBlock(pos, x, y), nil
}

// DragTo moves a handle's drag to a point of the page.
func (e *Editor) DragTo(x, y float64) { e.s.DragTo(x, y) }

// EndDrag drops what a handle carries and says whether it moved.
func (e *Editor) EndDrag() bool { return e.s.EndDrag() }

// Characters counts the document's characters, every position with nodes;
// Words its words.
func (e *Editor) Characters(nodes bool) int { return richeditor.Characters(e.s.State(), nodes) }
func (e *Editor) Words() int                { return richeditor.Words(e.s.State()) }

// Suggestion is the suggestion of key, null when it has none.
func (e *Editor) Suggestion(key string) (RichSuggestion, bool) {
	s := richeditor.SuggestionStateOf(e.s.State(), key)
	if s == nil {
		return RichSuggestion{}, false
	}
	return RichSuggestion{Active: s.Active, Query: s.Query, From: s.From, To: s.To, LastKey: s.LastKey, KeyCount: s.KeyCount}, true
}

// UndoDepth and RedoDepth count the steps undo and redo can take.
func (e *Editor) UndoDepth() int { return richeditor.UndoDepth(e.s.State()) }
func (e *Editor) RedoDepth() int { return richeditor.RedoDepth(e.s.State()) }

// Close stops it hearing its session, which dispose() calls.
func (e *Editor) Close() error {
	if !e.closed {
		e.closed = true
		e.stop()
		e.emit("destroy", RichEditorEvent{Revision: e.s.State().Revision})
		e.handlers = map[string][]*func(RichEditorEvent){}
	}
	return nil
}
