package richeditor

import (
	"regexp"
	"testing"
	"time"
)

func testSchema(t *testing.T) *Schema {
	t.Helper()
	s, err := NewSchema(SchemaSpec{
		Nodes: []NodeSpec{
			{Name: "doc", Content: "block+"},
			{Name: "paragraph", Content: "inline*", Group: "block"},
			{Name: "blockquote", Content: "block+", Group: "block", Defining: true},
			{Name: "heading", Content: "inline*", Group: "block", Defining: true, Attrs: []AttrSpec{{Name: "level", Default: 1, HasDefault: true}}},
			{Name: "code_block", Content: "text*", Group: "block", Code: true, Marks: "none", Defining: true},
			{Name: "horizontal_rule", Group: "block"},
			{Name: "list", Content: "item+", Group: "block"},
			{Name: "item", Content: "paragraph block*", Defining: true},
			{Name: "text", Group: "inline"},
			{Name: "image", Inline: true, Group: "inline", Attrs: []AttrSpec{{Name: "src"}}},
			{Name: "hard_break", Inline: true, Group: "inline", NoSelect: true},
		},
		Marks: []MarkSpec{{Name: "em"}, {Name: "strong"}, {Name: "code", Excludes: "_"}, {Name: "link", NotInclusive: true, Attrs: []AttrSpec{{Name: "href"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func p(s *Schema, kids ...*Node) *Node { return s.Node("paragraph", nil, kids...) }

func stateAt(t *testing.T, doc *Node, anchor, head int, plugins ...*Plugin) *State {
	t.Helper()
	st, err := NewState(StateConfig{Doc: doc, Selection: TextSelectionAt(doc, anchor, head), Plugins: plugins})
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// run applies c, failing when it does not apply.
func run(t *testing.T, st *State, c Command) *State {
	t.Helper()
	next := st
	if !c(st, func(tr *Transaction) {
		n, err := st.Apply(tr)
		if err != nil {
			t.Fatal(err)
		}
		next = n
	}) {
		t.Fatalf("the command did not apply to %s", st.Doc)
	}
	return next
}

func wantDoc(t *testing.T, st *State, want *Node) {
	t.Helper()
	if !st.Doc.Eq(want) {
		t.Fatalf("doc is\n%s\nwant\n%s", st.Doc, want)
	}
	if err := st.Doc.Check(); err != nil {
		t.Fatal(err)
	}
}

func TestContentExpressions(t *testing.T) {
	s := testSchema(t)
	item := s.Nodes["item"]
	if m := item.ContentMatch.MatchType(s.Nodes["paragraph"]); m == nil || !m.ValidEnd {
		t.Fatal("an item takes a paragraph and may end")
	}
	if item.ContentMatch.MatchType(s.Nodes["heading"]) != nil {
		t.Fatal("an item starts with a paragraph")
	}
	if _, err := NewSchema(SchemaSpec{Nodes: []NodeSpec{{Name: "doc", Content: "text"}, {Name: "text"}}}); err == nil {
		t.Fatal("a required text is a dead end")
	}
	w := s.Nodes["list"].ContentMatch.FindWrapping(s.Nodes["paragraph"])
	if len(w) != 1 || w[0] != item {
		t.Fatalf("wrapping %v", w)
	}
	r, _ := NewSchema(SchemaSpec{Nodes: []NodeSpec{{Name: "doc", Content: "(a | b){2,3} c?"}, {Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "text"}}})
	doc := r.TopNode.CreateAndFill(nil, nil, nil)
	if doc == nil || doc.ChildCount() != 2 {
		t.Fatalf("filled %v", doc)
	}
}

func TestPositionsCountUTF16(t *testing.T) {
	s := testSchema(t)
	doc := s.Node("doc", nil, p(s, s.Text("a😀b")))
	if doc.Content.Size != 6 {
		t.Fatalf("size %d", doc.Content.Size)
	}
	tr := NewTransform(doc).Delete(2, 4)
	if tr.Err != nil || tr.Doc.TextContent() != "ab" {
		t.Fatalf("%v %q", tr.Err, tr.Doc.TextContent())
	}
	if got := doc.TextBetween(1, 4, "", ""); got != "a😀" {
		t.Fatalf("between %q", got)
	}
}

func TestStepsInvertAndMap(t *testing.T) {
	s := testSchema(t)
	doc := s.Node("doc", nil, p(s, s.Text("hello")), p(s, s.Text("world")))
	tr := NewTransform(doc)
	tr.Delete(3, 10)
	if tr.Err != nil || tr.Doc.ChildCount() != 1 || tr.Doc.TextContent() != "herld" {
		t.Fatalf("%v %s", tr.Err, tr.Doc)
	}
	inv := tr.Steps[0].Invert(doc)
	back, err := inv.Apply(tr.Doc)
	if err != nil || !back.Eq(doc) {
		t.Fatalf("%v %s", err, back)
	}
	m := tr.Mapping
	if m.Map(2, 1) != 2 || m.Map(5, 1) != 3 || m.Map(12, 1) != 5 {
		t.Fatalf("maps %d %d %d", m.Map(2, 1), m.Map(5, 1), m.Map(12, 1))
	}
	if r := m.MapResult(5, 1); !r.DeletedAcross() || !r.Deleted() {
		t.Fatalf("result %+v", r)
	}
	wrap := NewTransform(doc)
	rng := doc.Resolve(1).BlockRange(doc.Resolve(1), nil)
	wrap.Wrap(rng, FindWrapping(rng, s.Nodes["blockquote"], nil, nil))
	if wrap.Err != nil || wrap.Doc.Child(0).Type.Name != "blockquote" {
		t.Fatalf("%v %s", wrap.Err, wrap.Doc)
	}
	un, err := wrap.Steps[0].Invert(doc).Apply(wrap.Doc)
	if err != nil || !un.Eq(doc) {
		t.Fatalf("%v %s", err, un)
	}
}

func TestFitterClosesAndWraps(t *testing.T) {
	s := testSchema(t)
	doc := s.Node("doc", nil, p(s, s.Text("one")), p(s, s.Text("two")))
	// A list item pasted over a word in a paragraph.
	list := FragmentFrom(s.Node("list", nil, s.Node("item", nil, p(s, s.Text("x")))))
	tr := NewTransform(doc).Replace(2, 3, NewSlice(list, 3, 3))
	if tr.Err != nil || tr.Doc.Child(0).TextContent() != "oxe" {
		t.Fatalf("%v %s", tr.Err, tr.Doc)
	}
	// A heading into a list's item keeps the item valid.
	ld := s.Node("doc", nil, s.Node("list", nil, s.Node("item", nil, p(s, s.Text("a")))))
	tr = NewTransform(ld).Replace(4, 4, NewSlice(FragmentFrom(s.Node("heading", nil, s.Text("h"))), 0, 0))
	if tr.Err != nil {
		t.Fatal(tr.Err)
	}
	if err := tr.Doc.Check(); err != nil {
		t.Fatalf("%v: %s", err, tr.Doc)
	}
	// A range across two blocks joins them.
	tr = NewTransform(doc).ReplaceRange(2, 7, EmptySlice)
	if tr.Err != nil || tr.Doc.ChildCount() != 1 || tr.Doc.TextContent() != "owo" {
		t.Fatalf("%v %s", tr.Err, tr.Doc)
	}
}

func TestMarksCommand(t *testing.T) {
	s := testSchema(t)
	doc := s.Node("doc", nil, p(s, s.Text("one two")))
	st := stateAt(t, doc, 1, 4)
	em := s.Marks["em"]
	st = run(t, st, ToggleMark(em, nil))
	wantDoc(t, st, s.Node("doc", nil, p(s, s.Text("one", s.Mark("em", nil)), s.Text(" two"))))
	st = run(t, st, ToggleMark(em, nil))
	wantDoc(t, st, doc)
	// At a cursor the mark waits in the stored marks for typed text.
	st = stateAt(t, doc, 4, 4)
	st = run(t, st, ToggleMark(em, nil))
	if len(st.StoredMarks) != 1 {
		t.Fatalf("stored %v", st.StoredMarks)
	}
	n, err := st.Apply(st.Tr().InsertText("!", -1, -1))
	if err != nil {
		t.Fatal(err)
	}
	wantDoc(t, n, s.Node("doc", nil, p(s, s.Text("one"), s.Text("!", s.Mark("em", nil)), s.Text(" two"))))
	if n.StoredMarks != nil {
		t.Fatal("a step drops the stored marks")
	}
	// A link does not grow at its end, and code excludes the rest.
	link := s.Mark("link", Attrs{"href": "x"})
	ld := s.Node("doc", nil, p(s, s.Text("ab", link)))
	if len(ld.Resolve(3).MarksHere()) != 0 {
		t.Fatal("text after a link took it")
	}
	if set := s.Mark("code", nil).AddToSet([]*Mark{s.Mark("em", nil)}); len(set) != 1 || set[0].Type.Name != "code" {
		t.Fatalf("set %v", set)
	}
}

func TestSplitJoinAndLift(t *testing.T) {
	s := testSchema(t)
	doc := s.Node("doc", nil, p(s, s.Text("onetwo")))
	st := run(t, stateAt(t, doc, 4, 4), SplitBlock)
	wantDoc(t, st, s.Node("doc", nil, p(s, s.Text("one")), p(s, s.Text("two"))))
	if st.Selection.Head() != 6 {
		t.Fatalf("head %d", st.Selection.Head())
	}
	st = run(t, st, ChainCommands(DeleteSelection, JoinBackward))
	wantDoc(t, st, doc)
	if st.Selection.Head() != 4 {
		t.Fatalf("head %d", st.Selection.Head())
	}
	st = run(t, stateAt(t, doc, 1, 1), WrapIn(s.Nodes["blockquote"], nil))
	wantDoc(t, st, s.Node("doc", nil, s.Node("blockquote", nil, p(s, s.Text("onetwo")))))
	st = run(t, st, Lift)
	wantDoc(t, st, doc)
	// A heading at Enter's end gives a paragraph.
	h := s.Node("doc", nil, s.Node("heading", Attrs{"level": 2}, s.Text("t")))
	st = run(t, stateAt(t, h, 2, 2), SplitBlock)
	wantDoc(t, st, s.Node("doc", nil, s.Node("heading", Attrs{"level": 2}, s.Text("t")), p(s)))
	// JoinForward takes in the next block, and deletes an empty one.
	j := s.Node("doc", nil, p(s, s.Text("a")), p(s), p(s, s.Text("b")))
	st = run(t, stateAt(t, j, 3, 3), ChainCommands(JoinBackward))
	wantDoc(t, st, s.Node("doc", nil, p(s, s.Text("a")), p(s, s.Text("b"))))
	st = run(t, stateAt(t, st.Doc, 2, 2), JoinForward)
	wantDoc(t, st, s.Node("doc", nil, p(s, s.Text("ab"))))
}

func TestLists(t *testing.T) {
	s := testSchema(t)
	list, item := s.Nodes["list"], s.Nodes["item"]
	doc := s.Node("doc", nil, p(s, s.Text("a")), p(s, s.Text("b")))
	st := run(t, stateAt(t, doc, 1, 5), WrapInList(list, nil))
	li := func(kids ...*Node) *Node { return s.Node("item", nil, kids...) }
	wantDoc(t, st, s.Node("doc", nil, s.Node("list", nil, li(p(s, s.Text("a"))), li(p(s, s.Text("b"))))))
	// Enter at an item's end makes the next.
	st = stateAt(t, st.Doc, 4, 4)
	st = run(t, st, SplitListItem(item, nil))
	wantDoc(t, st, s.Node("doc", nil, s.Node("list", nil, li(p(s, s.Text("a"))), li(p(s)), li(p(s, s.Text("b"))))))
	// Enter in an empty item leaves the list.
	st = run(t, st, ChainCommands(SplitListItem(item, nil), LiftEmptyBlock))
	wantDoc(t, st, s.Node("doc", nil, s.Node("list", nil, li(p(s, s.Text("a")))), p(s), s.Node("list", nil, li(p(s, s.Text("b"))))))
	// Tab nests an item, Shift-Tab brings it back.
	n := s.Node("doc", nil, s.Node("list", nil, li(p(s, s.Text("a"))), li(p(s, s.Text("b")))))
	st = run(t, stateAt(t, n, 8, 8), SinkListItem(item))
	wantDoc(t, st, s.Node("doc", nil, s.Node("list", nil, li(p(s, s.Text("a")), s.Node("list", nil, li(p(s, s.Text("b"))))))))
	st = run(t, st, LiftListItem(item))
	wantDoc(t, st, n)
	// Lifting the first item out of a list.
	st = run(t, stateAt(t, n, 3, 3), LiftListItem(item))
	wantDoc(t, st, s.Node("doc", nil, p(s, s.Text("a")), s.Node("list", nil, li(p(s, s.Text("b"))))))
}

func TestPluginsFilterAndAppend(t *testing.T) {
	s := testSchema(t)
	doc := s.Node("doc", nil, p(s, s.Text("a")))
	count := &Plugin{Key: "count", Init: func(*State) any { return 0 },
		Apply: func(tr *Transaction, v any, _, _ *State) any { return v.(int) + len(tr.Steps) }}
	refuse := &Plugin{Key: "refuse", FilterTransaction: func(tr *Transaction, _ *State) bool { return tr.Meta("no") == nil }}
	upper := &Plugin{Key: "append", AppendTransaction: func(trs []*Transaction, _, n *State) *Transaction {
		if n.Doc.TextContent() == "ab" {
			return n.Tr().InsertText("c", n.Doc.Content.Size-1, -1)
		}
		return nil
	}}
	st := stateAt(t, doc, 2, 2, count, refuse, upper)
	if _, err := NewState(StateConfig{Doc: doc, Plugins: []*Plugin{count, count}}); err == nil {
		t.Fatal("two plugins with one key")
	}
	same, err := st.Apply(st.Tr().InsertText("x", -1, -1).SetMeta("no", true))
	if err != nil || same != st {
		t.Fatalf("a filtered transaction applied: %v", err)
	}
	n, trs, err := st.ApplyTransaction(st.Tr().InsertText("b", -1, -1))
	if err != nil || len(trs) != 2 || n.Doc.TextContent() != "abc" || n.PluginState("count") != 2 {
		t.Fatalf("%v %d %s %v", err, len(trs), n.Doc, n.PluginState("count"))
	}
	loop := &Plugin{Key: "loop", AppendTransaction: func(_ []*Transaction, _, n *State) *Transaction { return n.Tr().InsertText("x", 1, -1) }}
	ping := &Plugin{Key: "ping", AppendTransaction: loop.AppendTransaction}
	st = stateAt(t, doc, 2, 2, loop, ping)
	if _, _, err := st.ApplyTransaction(st.Tr().InsertText("b", -1, -1)); err == nil {
		t.Fatal("plugins answering each other must stop")
	}
}

func TestSelections(t *testing.T) {
	s := testSchema(t)
	doc := s.Node("doc", nil, p(s, s.Text("ab")), s.Node("horizontal_rule", nil), p(s, s.Text("cd")))
	near := SelectionNear(doc.Resolve(4), 1)
	if _, ok := near.(*NodeSelection); !ok {
		t.Fatalf("near a rule: %T", near)
	}
	st := stateAt(t, doc, 1, 3)
	tr := st.Tr().Delete(1, 2)
	if got := tr.Selection(); got.From() != 1 || got.To() != 2 {
		t.Fatalf("mapped selection %d %d", got.From(), got.To())
	}
	node := NodeSelectionAt(doc, 4)
	if c := node.Content(); c.Content.ChildCount() != 1 || c.Content.Child(0).Type.Name != "horizontal_rule" {
		t.Fatalf("content %v", c)
	}
	nst, _ := NewState(StateConfig{Doc: doc, Selection: node})
	n, err := nst.Apply(nst.Tr().DeleteSelection())
	if err != nil || n.Doc.ChildCount() != 2 {
		t.Fatalf("%v %s", err, n.Doc)
	}
	all, _ := NewState(StateConfig{Doc: doc, Selection: NewAllSelection(doc)})
	n, err = all.Apply(all.Tr().DeleteSelection())
	if err != nil || n.Doc.ChildCount() != 1 || n.Doc.Content.Size != 2 {
		t.Fatalf("%v %s", err, n.Doc)
	}
}

func TestHistoryGroupsByTime(t *testing.T) {
	s := testSchema(t)
	doc := s.Node("doc", nil, p(s))
	v := &Session{state: stateAt(t, doc, 1, 1, History(HistoryOptions{NewGroupDelay: time.Second}))}
	at := time.Now()
	typeAt := func(text string, after time.Duration) {
		at = at.Add(after)
		tr := v.State().Tr().InsertText(text, -1, -1)
		tr.Time = at
		v.Dispatch(tr)
	}
	typeAt("a", 0)
	typeAt("b", 100*time.Millisecond)
	typeAt("c", 2*time.Second)
	if UndoDepth(v.State()) != 2 {
		t.Fatalf("depth %d", UndoDepth(v.State()))
	}
	if !Undo(v.State(), v.Dispatch) || v.State().Doc.TextContent() != "ab" {
		t.Fatalf("undo left %q", v.State().Doc.TextContent())
	}
	if !Undo(v.State(), v.Dispatch) || v.State().Doc.TextContent() != "" {
		t.Fatalf("undo left %q", v.State().Doc.TextContent())
	}
	if Undo(v.State(), nil) {
		t.Fatal("nothing is left to undo")
	}
	Redo(v.State(), v.Dispatch)
	Redo(v.State(), v.Dispatch)
	if v.State().Doc.TextContent() != "abc" || v.State().Selection.Head() != 4 {
		t.Fatalf("redo left %q at %d", v.State().Doc.TextContent(), v.State().Selection.Head())
	}
	// A change kept out of the history maps it.
	Undo(v.State(), v.Dispatch)
	v.Dispatch(v.State().Tr().Insert(1, s.Text("X")).SetMeta(addToHistory, false))
	Undo(v.State(), v.Dispatch)
	if v.State().Doc.TextContent() != "X" {
		t.Fatalf("undo over a remote change left %q", v.State().Doc.TextContent())
	}
	// An input method's commits are one step, however far apart.
	v = &Session{state: stateAt(t, doc, 1, 1, History(HistoryOptions{}))}
	for i, c := range []string{"你", "好"} {
		tr := v.State().Tr().InsertText(c, -1, -1).SetMeta(compositionID, 7)
		tr.Time = at.Add(time.Duration(i) * time.Hour)
		v.Dispatch(tr)
	}
	if UndoDepth(v.State()) != 1 {
		t.Fatalf("composition depth %d", UndoDepth(v.State()))
	}
}

func TestInputRulesAndKeymap(t *testing.T) {
	s := testSchema(t)
	rules := []InputRule{
		WrappingInputRule(regexp.MustCompile(`^\s*>\s$`), s.Nodes["blockquote"], nil, nil),
		TextblockTypeInputRule(regexp.MustCompile(`^(#{1,3})\s$`), s.Nodes["heading"], func(m []string) Attrs { return Attrs{"level": len(m[1])} }),
		MarkInputRule(regexp.MustCompile(`(^|\s)\*\*([^*]+)\*\*$`), s.Marks["strong"], nil),
		TextInputRule(regexp.MustCompile(`--$`), "—"),
	}
	plugin := InputRules(rules)
	keys := Keymap("keys", BaseKeymap())
	typed := func(v *Session, text string) {
		sel := v.State().Selection
		for _, p := range v.State().Plugins {
			if p.HandleTextInput != nil && p.HandleTextInput(v, sel.From(), sel.To(), text) {
				return
			}
		}
		v.Dispatch(v.State().Tr().InsertText(text, sel.From(), sel.To()))
	}
	session := func(text string) *Session {
		doc := s.Node("doc", nil, p(s))
		v := &Session{state: stateAt(t, doc, 1, 1, plugin, keys)}
		for _, r := range text {
			typed(v, string(r))
		}
		return v
	}
	v := session("> hi")
	wantDoc(t, v.State(), s.Node("doc", nil, s.Node("blockquote", nil, p(s, s.Text("hi")))))
	v = session("## t")
	wantDoc(t, v.State(), s.Node("doc", nil, s.Node("heading", Attrs{"level": 2}, s.Text("t"))))
	v = session("a **b** c")
	wantDoc(t, v.State(), s.Node("doc", nil, p(s, s.Text("a "), s.Text("b", s.Mark("strong", nil)), s.Text(" c"))))
	v = session("a--")
	wantDoc(t, v.State(), s.Node("doc", nil, p(s, s.Text("a—"))))
	// Backspace after a rule gives the typed text back.
	if !keys.HandleKeyDown(v, "Backspace") {
		t.Fatal("backspace")
	}
	wantDoc(t, v.State(), s.Node("doc", nil, p(s, s.Text("a--"))))
	if normalizeKey("Shift-Mod-b") != "Mod-Shift-b" {
		t.Fatal(normalizeKey("Shift-Mod-b"))
	}
}
