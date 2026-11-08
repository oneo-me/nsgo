package richeditor

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// Origin says where a transaction comes from.
type Origin uint8

const (
	OriginUser Origin = iota
	OriginInput
	OriginPaste
	OriginDrop
	OriginRemote
	OriginHistory
	OriginPlugin
	OriginAPI
)

// Transaction is one user action's changes: a Transform with the state's
// selection, stored marks, metadata and an origin.
type Transaction struct {
	Transform
	Time   time.Time
	Origin Origin

	state *State
	// sel is the selection set, at selFor steps; marks the stored marks,
	// which a step after marksAt drops.
	sel     Selection
	selFor  int
	selSet  bool
	marks   []*Mark
	marksAt int
	scroll  bool
	meta    map[any]any
}

// Selection is the transaction's selection, mapped through its steps.
func (tr *Transaction) Selection() Selection {
	if tr.selFor < len(tr.Steps) {
		tr.sel = tr.sel.Map(tr.Doc, tr.Mapping.Slice(tr.selFor, len(tr.Mapping.Maps)))
		tr.selFor = len(tr.Steps)
	}
	return tr.sel
}

// SelectionSet says the selection was set.
func (tr *Transaction) SelectionSet() bool { return tr.selSet }

// SetSelection sets the selection, which drops the stored marks.
func (tr *Transaction) SetSelection(s Selection) *Transaction {
	tr.sel, tr.selFor, tr.selSet, tr.marks = s, len(tr.Steps), true, nil
	return tr
}

// StoredMarks are the marks stored now: any step drops them.
func (tr *Transaction) StoredMarks() []*Mark {
	if len(tr.Steps) > tr.marksAt {
		return nil
	}
	return tr.marks
}

// SetStoredMarks sets the marks the next typed text takes.
func (tr *Transaction) SetStoredMarks(marks []*Mark) *Transaction {
	tr.marks, tr.marksAt = marks, len(tr.Steps)
	return tr
}

func (tr *Transaction) marksOrHere() []*Mark {
	if m := tr.StoredMarks(); m != nil {
		return m
	}
	return tr.Selection().FromPos().MarksHere()
}

// EnsureMarks keeps marks as the stored marks unless the selection's place
// has them.
func (tr *Transaction) EnsureMarks(marks []*Mark) *Transaction {
	if !marksEqual(tr.marksOrHere(), marks) {
		tr.SetStoredMarks(marks)
	}
	return tr
}

// AddStoredMark adds a mark to the ensured marks.
func (tr *Transaction) AddStoredMark(m *Mark) *Transaction {
	return tr.EnsureMarks(m.AddToSet(tr.marksOrHere()))
}

// RemoveStoredMark removes a mark type from the ensured marks.
func (tr *Transaction) RemoveStoredMark(t *MarkType) *Transaction {
	return tr.EnsureMarks(t.RemoveFromSet(tr.marksOrHere()))
}

// ReplaceSelection replaces the selection with slice.
func (tr *Transaction) ReplaceSelection(slice *Slice) *Transaction {
	tr.Selection().Replace(tr, slice)
	return tr
}

// ReplaceSelectionWith replaces the selection with node, which takes the
// marks there when inheritMarks.
func (tr *Transaction) ReplaceSelectionWith(node *Node, inheritMarks bool) *Transaction {
	sel := tr.Selection()
	if inheritMarks {
		stored := tr.StoredMarks()
		if stored == nil {
			if sel.Empty() {
				stored = sel.FromPos().MarksHere()
			} else {
				stored = sel.FromPos().MarksAcross(sel.ToPos())
			}
		}
		node = node.Mark(stored)
	}
	sel.ReplaceWith(tr, node)
	return tr
}

// DeleteSelection deletes the selection.
func (tr *Transaction) DeleteSelection() *Transaction {
	tr.Selection().Replace(tr, nil)
	return tr
}

// InsertText inserts text from from to to, or over the selection when from
// is negative, with the marks there, and puts the cursor after it.
func (tr *Transaction) InsertText(text string, from, to int) *Transaction {
	schema := tr.Doc.Type.Schema
	if from < 0 {
		if text == "" {
			return tr.DeleteSelection()
		}
		return tr.ReplaceSelectionWith(schema.Text(text), true)
	}
	if to < 0 {
		to = from
	}
	if text == "" {
		tr.DeleteRange(from, to)
		return tr
	}
	marks := tr.StoredMarks()
	if marks == nil {
		r := tr.Doc.Resolve(from)
		if to == from {
			marks = r.MarksHere()
		} else {
			marks = r.MarksAcross(tr.Doc.Resolve(to))
		}
	}
	tr.ReplaceRangeWith(from, to, schema.Text(text, marks...))
	if !tr.Selection().Empty() && tr.Err == nil {
		tr.SetSelection(SelectionNear(tr.Selection().ToPos(), 1))
	}
	return tr
}

// SetMeta stores a value under key.
func (tr *Transaction) SetMeta(key, value any) *Transaction {
	if tr.meta == nil {
		tr.meta = map[any]any{}
	}
	tr.meta[key] = value
	return tr
}

// Meta reads a value stored under key.
func (tr *Transaction) Meta(key any) any { return tr.meta[key] }

// ScrollIntoView has the view reveal the selection.
func (tr *Transaction) ScrollIntoView() *Transaction {
	tr.scroll = true
	return tr
}

// ScrolledIntoView says the view should reveal the selection.
func (tr *Transaction) ScrolledIntoView() bool { return tr.scroll }

// Plugin is optional behavior over a state: a field in the state, hooks on
// transactions and hooks on the view's events, asked in plugin order, the
// first true consuming the event.
type Plugin struct {
	// Key is unique in a state.
	Key string
	// Init and Apply are the plugin's field in the state.
	Init  func(s *State) any
	Apply func(tr *Transaction, value any, old, new *State) any
	// FilterTransaction refuses a transaction before it applies.
	FilterTransaction func(tr *Transaction, s *State) bool
	// AppendTransaction may add a transaction after others applied.
	AppendTransaction func(trs []*Transaction, old, new *State) *Transaction

	HandleKeyDown   func(v *Session, key string) bool
	HandleTextInput func(v *Session, from, to int, text string) bool
	HandlePaste     func(v *Session, slice *Slice) bool
	// TransformPasted changes what is pasted, and TransformPastedText the
	// text it comes from.
	TransformPasted     func(v *Session, slice *Slice) *Slice
	TransformPastedText func(v *Session, text string) string
	// HandleClick hears a press at pos, Clicks in a row, Mod held or not.
	HandleClick func(v *Session, pos, clicks int, mod bool) bool
	// Decorations are what the view draws without changing the document.
	Decorations func(s *State) []Decoration
}

// Decoration styles a range without changing the document: Class on the
// range's text, or with Node on the block at From. With Widget it is a view
// in the text at From instead, taking no position: Side below zero puts it
// before a cursor there, else after, and Key, when set, keeps its view
// while the decoration stays.
type Decoration struct {
	From, To int
	Class    string
	Node     bool
	Widget   func(cx *core.Scope) core.Node
	Side     int
	Key      string
}

// StateConfig makes a state: Doc, or an empty document of Schema.
type StateConfig struct {
	Schema      *Schema
	Doc         *Node
	Selection   Selection
	StoredMarks []*Mark
	Plugins     []*Plugin
}

// State is an editor's state, never changed once made: Apply returns a new
// one, which is what history and the view's updates rely on.
type State struct {
	Schema      *Schema
	Doc         *Node
	Selection   Selection
	StoredMarks []*Mark
	Plugins     []*Plugin
	// Revision counts the transactions applied.
	Revision int
	fields   map[*Plugin]any
}

// StateError says a transaction could not apply.
type StateError struct{ Msg string }

func (e *StateError) Error() string { return e.Msg }

const maxAppendRounds = 64

// NewState makes a state, refusing two plugins with one key.
func NewState(c StateConfig) (*State, error) {
	schema := c.Schema
	if schema == nil && c.Doc != nil {
		schema = c.Doc.Type.Schema
	}
	if schema == nil {
		return nil, errors.New("richeditor: a state needs a document or a schema")
	}
	seen := map[string]bool{}
	for _, p := range c.Plugins {
		if p.Key != "" && seen[p.Key] {
			return nil, fmt.Errorf("richeditor: two plugins with the key %q", p.Key)
		}
		seen[p.Key] = true
	}
	doc := c.Doc
	if doc == nil {
		doc = schema.TopNode.CreateAndFill(nil, nil, nil)
	}
	s := &State{Schema: schema, Doc: doc, Selection: c.Selection, StoredMarks: c.StoredMarks, Plugins: slices.Clone(c.Plugins), fields: map[*Plugin]any{}}
	if s.Selection == nil {
		s.Selection = SelectionAtStart(doc)
	}
	for _, p := range s.Plugins {
		if p.Init != nil {
			s.fields[p] = p.Init(s)
		}
	}
	return s, nil
}

// Tr starts a transaction.
func (s *State) Tr() *Transaction {
	return &Transaction{Transform: Transform{Doc: s.Doc, Mapping: NewMapping()}, Time: time.Now(), state: s, sel: s.Selection, marks: s.StoredMarks}
}

// PluginState is plugin key's field.
func (s *State) PluginState(key string) any {
	for _, p := range s.Plugins {
		if p.Key == key {
			return s.fields[p]
		}
	}
	return nil
}

// Reconfigure is the state with other plugins, whose fields start again
// unless a plugin stays.
func (s *State) Reconfigure(plugins []*Plugin) (*State, error) {
	n, err := NewState(StateConfig{Schema: s.Schema, Doc: s.Doc, Selection: s.Selection, StoredMarks: s.StoredMarks, Plugins: plugins})
	if err != nil {
		return nil, err
	}
	n.Revision = s.Revision
	for p, v := range s.fields {
		if slices.Contains(plugins, p) {
			n.fields[p] = v
		}
	}
	return n, nil
}

func (s *State) applyInner(tr *Transaction) (*State, error) {
	if !tr.Before().Eq(s.Doc) {
		return nil, &StateError{"applying a mismatched transaction"}
	}
	if err := tr.Doc.Check(); err != nil {
		return nil, &StateError{err.Error()}
	}
	var marks []*Mark
	if t, ok := tr.Selection().(*TextSelection); ok && t.Cursor() != nil {
		marks = tr.StoredMarks()
	}
	n := &State{Schema: s.Schema, Doc: tr.Doc, Selection: tr.Selection(), StoredMarks: marks, Plugins: s.Plugins, Revision: s.Revision + 1, fields: make(map[*Plugin]any, len(s.fields))}
	for _, p := range s.Plugins {
		if p.Apply != nil {
			n.fields[p] = p.Apply(tr, s.fields[p], s, n)
		} else if v, ok := s.fields[p]; ok {
			n.fields[p] = v
		}
	}
	return n, nil
}

func (s *State) filter(tr *Transaction, ignore int) bool {
	for i, p := range s.Plugins {
		if i != ignore && p.FilterTransaction != nil && !p.FilterTransaction(tr, s) {
			return false
		}
	}
	return true
}

// ApplyTransaction applies tr and what plugins append to it, atomically:
// the new state, and every transaction applied. A filtered transaction
// leaves the state.
func (s *State) ApplyTransaction(root *Transaction) (*State, []*Transaction, error) {
	if root.Err != nil {
		return nil, nil, root.Err
	}
	if !s.filter(root, -1) {
		return s, nil, nil
	}
	trs := []*Transaction{root}
	newState, err := s.applyInner(root)
	if err != nil {
		return nil, nil, err
	}
	type seenAt struct {
		state *State
		n     int
	}
	var seen []seenAt
	for round := 0; ; round++ {
		if round > maxAppendRounds {
			return nil, nil, &StateError{"plugins appended transactions for more than 64 rounds"}
		}
		haveNew := false
		for i, p := range s.Plugins {
			if p.AppendTransaction == nil {
				continue
			}
			n, old := 0, s
			if seen != nil {
				n, old = seen[i].n, seen[i].state
			}
			var tr *Transaction
			if n < len(trs) {
				tr = p.AppendTransaction(trs[n:], old, newState)
			}
			if tr != nil && tr.Err == nil && newState.filter(tr, i) {
				tr.SetMeta(appendedKey, root)
				if seen == nil {
					seen = make([]seenAt, len(s.Plugins))
					for j := range seen {
						seen[j] = seenAt{s, 0}
						if j < i {
							seen[j] = seenAt{newState, len(trs)}
						}
					}
				}
				trs = append(trs, tr)
				if newState, err = newState.applyInner(tr); err != nil {
					return nil, nil, err
				}
				haveNew = true
			}
			if seen != nil {
				seen[i] = seenAt{newState, len(trs)}
			}
		}
		if !haveNew {
			return newState, trs, nil
		}
	}
}

// Apply applies tr, the new state alone.
func (s *State) Apply(tr *Transaction) (*State, error) {
	n, _, err := s.ApplyTransaction(tr)
	return n, err
}

type metaKey string

const (
	appendedKey   metaKey = "appendedTransaction"
	addToHistory  metaKey = "addToHistory"
	pasteMeta     metaKey = "paste"
	compositionID metaKey = "composition"
	uiEvent       metaKey = "uiEvent"
)
