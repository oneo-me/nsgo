package richeditor

import (
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

// Styles are the styles of the editor's parts, which supply its whole look.
type Styles struct {
	// Content is the box that scrolls, whose padding is the text's.
	Content core.Style
	// Nodes style blocks by type name, then "type[attr=value]"; Marks
	// inline text by mark name, as "mention" does its leaves.
	Nodes, Marks map[string]core.Style
	// Placeholder is the text of an empty document; Caret the caret, 2
	// points wide unless it gives a width; Selection behind selected text;
	// Composition what an input method composes.
	Placeholder, Caret, Selection, Composition core.Style
	// SelectedNode goes on the block a node selection holds, and SelectedCell
	// on the box in each selected cell.
	SelectedNode, SelectedCell core.Style
	// ListMarker is the marker a list's item shows; TaskCheckbox a task
	// item's box, whose Selected state is its check.
	ListMarker, TaskCheckbox core.Style
	// TableResizeHandle is the line of a column or row being resized;
	// TableGrip a table's grip over a column or beside a row;
	// TableDropIndicator the line where one dragged will land.
	TableResizeHandle, TableGrip, TableDropIndicator core.Style
}

// Classes are Tailwind classes of the parts, by Styles' names, each under
// its part's style.
type Classes struct {
	Content                                          string
	Nodes, Marks                                     map[string]string
	Placeholder, Caret, Selection, Composition       string
	SelectedNode, SelectedCell                       string
	ListMarker, TaskCheckbox                         string
	TableResizeHandle, TableGrip, TableDropIndicator string
}

// NodeProps is what a node view follows.
type NodeProps struct {
	// Node is the node now, and Pos where it is.
	Node func() *Node
	Pos  func() int
	// Selected says a node selection holds it.
	Selected func() bool
	// Content is the editor's rendering of the node's content, editable
	// where the view places it; nil for a leaf.
	Content core.Node
	Editor  *Session
	// SetAttrs changes attributes as a transaction, undoable as any other.
	SetAttrs func(attrs Attrs)
}

// NodeView renders a node type's blocks, in place of the editor's: its
// element follows Node, and never changes the node but through SetAttrs.
type NodeView func(cx *core.Scope, props NodeProps) core.Node

// View is a rich text editor over a Session: the document's blocks in a
// scroll view, the app styling every part (Classes, Styles), with typing,
// input methods composing in the line, selections by press and drag, keys
// through the session's keymaps, and tables' handles while they can be edited.
type View struct {
	// Editor is the session it shows; without one it makes its own of
	// StarterKit and TableKit from Markdown.
	Editor   *Session
	Markdown string
	// Placeholder shows while the document is empty.
	Placeholder string
	// NodeViews render node types by name.
	NodeViews map[string]NodeView
	// BubbleMenu is the app's content in an overlay over the selection's
	// lines, shown while it returns one.
	BubbleMenu func() core.Node
	// BubbleMenuPlacement is its side, above when 0, and BubbleMenuOffset
	// its gap.
	BubbleMenuPlacement core.Placement
	BubbleMenuOffset    core.Points
	// OnChange receives the revision after a change of the document,
	// and OnSelectionChange after a move of the selection: read them from
	// the session.
	OnChange          func(revision int)
	OnSelectionChange func(s Selection)
	// OnPasteFiles receives pasted files and images, which insert nothing.
	OnPasteFiles func(paths []string)
	OnFocus      func()
	OnBlur       func()
	AutoFocus    bool
	Label        string
	Classes      Classes
	Styles       Styles
	StylesFunc   func() Styles
	Class        string
	Style        core.Style
	StyleFunc    func() core.Style
	TestID       string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
	Ref      *Ref
}

// Ref is a View's methods.
type Ref struct{ v *rv }

// Session is the session it shows.
func (r *Ref) Session() *Session {
	if r.v == nil {
		return nil
	}
	return r.v.s
}

// PageBounds is the caret's box at pos in the surface, for an Overlay the app
// places by the text, such as a mention's list.
func (r *Ref) PageBounds(pos int) core.Bounds {
	if r.v == nil {
		return core.Bounds{}
	}
	r.v.rev.Get()
	r.v.geo.Get()
	c, ok := r.v.CoordsAtPos(pos)
	origin, have := r.v.content.Bounds()
	if !ok || !have {
		return core.Bounds{}
	}
	return core.Bounds{Origin: core.Point{X: origin.Origin.X + core.Points(c.X), Y: origin.Origin.Y + core.Points(c.Y)}, Size: core.Size{Height: core.Points(c.Height)}}
}

// Focus takes the keyboard.
func (r *Ref) Focus() {
	if r.v != nil {
		r.v.sinkRef.Focus()
	}
}

// Blur gives the keyboard back.
func (r *Ref) Blur() {
	if r.v != nil {
		r.v.sinkRef.Blur()
	}
}

// block is a document's block as the view holds it: a view keyed by id,
// which keeps its id while it is the same node, or the node at its index
// with its type.
type block struct {
	id   int
	typ  *NodeType
	node *core.Signal[*Node]
	// pos is where it is, and nowNode what node holds, off the graph.
	pos     int
	nowNode *Node
	// parent is its block, and index its place among the kids.
	parent *block
	kids   *core.Signal[[]*block]
	ref    *core.Ref
	// runs and pieces are a textblock's; textRev counts their changes.
	runs    []textRun
	pieces  []piece
	textRev *core.Signal[int]
	// rect is a table cell's place in its table's grid; posRev changes as
	// pos does.
	rect   *core.Signal[Rect]
	posRev *core.Signal[int]
	// listIndex is its place in an ordered list.
	listIndex *core.Signal[int]
}

// textRun is a stretch of a textblock: DocLen positions at DocAt shown as
// TextLen characters at TextAt. An atom's text is one position however long;
// a composition takes none.
type textRun struct {
	docAt, docLen, textAt, textLen int
	atom                           bool
}

// piece is a span of a textblock.
type piece struct {
	text        string
	marks       []*Mark
	classes     []string
	composition bool
	leaf        *Node
}

// rv is a View's controller.
type rv struct {
	p  View
	cx *core.Scope
	s  *Session
	// looks are the parts' styles.
	looks *core.Memo[*Styles]
	root  *block
	// textblocks are in document order.
	textblocks []*block
	// leaves are the blocks that are atoms, in document order.
	leaves []*block
	tables []*block
	// tableList is tables as a signal, for their handles.
	tableList *core.Signal[[]*block]
	nextID    int
	decos     []Decoration
	// rev counts states; geo the layouts they took.
	rev, geo             *core.Signal[int]
	focused              *core.Signal[bool]
	composing, sink      *core.Signal[string]
	typed                string
	compID               int
	content, vs, sinkRef *core.Ref
	viewport             *core.Signal[core.Size]
	scrollY              *core.Signal[core.Points]
	reveal               bool
	lastSel              Selection
	lastDoc              *Node
	goalX                float64
	hasGoal              bool
	press                pressState
	tableDrag            *core.Signal[tableDragState]
	hovered              *core.Signal[string]
	stop                 func() bool
}

func parseStyles(p View) func() *Styles {
	return func() *Styles {
		s := p.Styles
		if p.StylesFunc != nil {
			s = p.StylesFunc()
		}
		return &s
	}
}

// Render implements core.Node.
func (p View) Render(cx *core.Scope) core.Node {
	s := p.Editor
	if s == nil {
		var err error
		s, err = NewSession(SessionConfig{Extensions: append(StarterKit(StarterOptions{}), TableKit(TableOptions{})), Markdown: p.Markdown})
		if err != nil {
			log.Printf("richeditor: %v", err)
			return core.Group{}
		}
	}
	v := &rv{p: p, cx: cx, s: s, rev: core.NewSignal(cx, 0), geo: core.NewSignal(cx, 0), focused: core.NewSignal(cx, false),
		composing: core.NewSignal(cx, ""), sink: core.NewSignal(cx, ""), content: new(core.Ref), vs: new(core.Ref), sinkRef: new(core.Ref),
		viewport: core.NewSignal(cx, core.Size{}), scrollY: core.NewSignal(cx, core.Points(0)),
		tableDrag: core.NewSignal(cx, tableDragState{}), hovered: core.NewSignal(cx, ""), tableList: core.NewSignalFunc(cx, []*block(nil), slices.Equal[[]*block])}
	v.looks = core.NewMemoFunc(cx, parseStyles(p), func(a, b *Styles) bool { return false })
	v.root = v.newBlock(s.State().Doc, nil)
	v.sync(s.State())
	v.lastSel, v.lastDoc = s.State().Selection, s.State().Doc
	h := &viewHandle{focus: v.sinkRef.Focus, blur: v.sinkRef.Blur, geometry: v}
	s.views = append(s.views, h)
	stop := s.Subscribe(func(st *State, trs []*Transaction) { v.changed(st, trs) })
	cx.Cleanup(func() {
		stop()
		s.views = slices.DeleteFunc(s.views, func(o *viewHandle) bool { return o == h })
	})
	if p.Ref != nil {
		p.Ref.v = v
	}
	cx.Effect(func() {
		v.geo.Get()
		v.rev.Get()
		cx.Untrack(v.revealCaret)
	})
	return core.View{
		Class: p.Class, Style: core.Style{Overflow: core.Hidden}.Merge(p.Style), StyleFunc: p.StyleFunc, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip,
		Role: core.RoleGroup, Label: p.Label,
		Children: []core.Node{
			core.Scroll{Ref: v.vs, TestID: "rich-editor-scroll",
				Style:    core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
				OnScroll: func(e core.ScrollEvent) { v.scrollY.Set(e.Offset.Y) },
				OnLayout: func(b core.Bounds) { v.viewport.Set(b.Size) },
				Children: []core.Node{v.contentBox()}},
			v.sinkView(),
			v.bubbleMenu(),
			core.Show{When: func() bool { return v.focused.Get() && v.composing.Get() == "" }, Then: func(cx *core.Scope) core.Node {
				v.keys(cx)
				return core.Group{}
			}},
		},
	}
}

func (v *rv) newBlock(n *Node, parent *block) *block {
	v.nextID++
	return &block{id: v.nextID, typ: n.Type, node: core.NewSignalFunc(v.cx, n, func(a, b *Node) bool { return a == b }), nowNode: n, parent: parent,
		kids: core.NewSignalFunc(v.cx, []*block(nil), slices.Equal[[]*block]), ref: new(core.Ref), textRev: core.NewSignal(v.cx, 0),
		rect: core.NewSignal(v.cx, Rect{}), posRev: core.NewSignal(v.cx, 0), listIndex: core.NewSignal(v.cx, 0)}
}

// entry is a node and where it is.
type entry struct {
	node *Node
	pos  int
}

// children are n's blocks: a table's are its cells, row after row, since
// rows have no view.
func children(n *Node, start int) []entry {
	var out []entry
	if tableRole(n.Type) == "table" {
		for r, pos := 0, start; r < n.ChildCount(); r++ {
			row := n.Child(r)
			cellPos := pos + 1
			for c := 0; c < row.ChildCount(); c++ {
				out = append(out, entry{row.Child(c), cellPos})
				cellPos += row.Child(c).NodeSize()
			}
			pos += row.NodeSize()
		}
		return out
	}
	pos := start
	for _, c := range n.Content.Content {
		out = append(out, entry{c, pos})
		pos += c.NodeSize()
	}
	return out
}

func (v *rv) reconcile(b *block, n *Node, pos int) {
	moved := b.pos != pos
	b.nowNode, b.pos = n, pos
	b.node.Set(n)
	if moved {
		b.posRev.Update(func(i int) int { return i + 1 })
	}
	if n.IsTextblock() {
		v.textblocks = append(v.textblocks, b)
		v.buildText(b)
		return
	}
	if n.IsLeaf() || n.Type.Spec.Atom {
		v.leaves = append(v.leaves, b)
		return
	}
	if tableRole(n.Type) == "table" {
		v.tables = append(v.tables, b)
	}
	entries := children(n, pos+1)
	old := b.kids.GetUntracked()
	used := make([]bool, len(old))
	next := make([]*block, len(entries))
	for i, e := range entries {
		for j, o := range old {
			if !used[j] && o.nowNode == e.node {
				used[j], next[i] = true, o
				break
			}
		}
	}
	for i, e := range entries {
		if next[i] == nil && i < len(old) && !used[i] && old[i].typ == e.node.Type {
			used[i], next[i] = true, old[i]
		}
	}
	for i, e := range entries {
		if next[i] == nil {
			next[i] = v.newBlock(e.node, b)
		}
		next[i].parent = b
		next[i].listIndex.Set(i)
	}
	var cells []CellRect
	if tableRole(n.Type) == "table" {
		cells = TableMapOf(n).Cells()
	}
	for i, e := range entries {
		v.reconcile(next[i], e.node, e.pos)
		if i < len(cells) {
			next[i].rect.Set(cells[i].Rect)
		}
	}
	b.kids.Set(next)
}

// sync gives the blocks the state's document.
func (v *rv) sync(st *State) {
	v.decos = v.decos[:0]
	for _, p := range st.Plugins {
		if p.Decorations != nil {
			v.decos = append(v.decos, p.Decorations(st)...)
		}
	}
	v.textblocks, v.leaves, v.tables = v.textblocks[:0], v.leaves[:0], v.tables[:0]
	v.reconcile(v.root, st.Doc, -1)
	v.tableList.Set(slices.Clone(v.tables))
}

func (v *rv) changed(st *State, trs []*Transaction) {
	v.cx.Batch(func() {
		v.sync(st)
		v.rev.Update(func(n int) int { return n + 1 })
		for _, tr := range trs {
			if tr.ScrolledIntoView() {
				v.reveal = true
			}
			if tr.DocChanged() || tr.SelectionSet() {
				v.hasGoal = v.hasGoal && tr.Meta(keepGoal) != nil
			}
		}
	})
	if v.p.OnChange != nil && st.Doc != v.lastDoc {
		v.p.OnChange(st.Revision)
	}
	if v.p.OnSelectionChange != nil && !st.Selection.Eq(v.lastSel) {
		v.p.OnSelectionChange(st.Selection)
	}
	v.lastSel, v.lastDoc = st.Selection, st.Doc
}

const keepGoal metaKey = "keepGoal"

// buildText makes a textblock's pieces and runs: its children split at the
// decorations, the composition at the caret.
func (v *rv) buildText(b *block) {
	n, start := b.nowNode, b.pos+1
	var pieces []piece
	var runs []textRun
	comp := v.composing.GetUntracked()
	compAt := -1
	if comp != "" {
		if head := v.s.State().Selection.From(); head >= start && head <= start+n.Content.Size {
			compAt = head - start
		}
	}
	textAt := 0
	add := func(p piece, docAt, docLen int, atom bool) {
		l := utf16Len(p.text)
		pieces = append(pieces, p)
		runs = append(runs, textRun{docAt, docLen, textAt, l, atom})
		textAt += l
	}
	composed := func(at int) {
		if compAt == at {
			add(piece{text: comp, composition: true}, at, 0, false)
			compAt = -1
		}
	}
	cuts := func(from, to int) []int {
		out := []int{from, to}
		if compAt > from && compAt < to {
			out = append(out, compAt)
		}
		for _, d := range v.decos {
			if !d.Node && d.From-start > from && d.From-start < to {
				out = append(out, d.From-start)
			}
			if !d.Node && d.To-start > from && d.To-start < to {
				out = append(out, d.To-start)
			}
		}
		slices.Sort(out)
		return slices.Compact(out)
	}
	classes := func(from, to int) []string {
		var out []string
		for _, d := range v.decos {
			if !d.Node && d.Class != "" && d.From-start <= from && d.To-start >= to {
				out = append(out, d.Class)
			}
		}
		return out
	}
	at := 0
	for _, c := range n.Content.Content {
		size := c.NodeSize()
		switch {
		case c.IsText():
			edges := cuts(at, at+size)
			for i := 0; i+1 < len(edges); i++ {
				composed(edges[i])
				add(piece{text: cut16(c.Text, edges[i]-at, edges[i+1]-at), marks: c.Marks, classes: classes(edges[i], edges[i+1])}, edges[i], edges[i+1]-edges[i], false)
			}
		default:
			composed(at)
			text := "￼"
			switch {
			case c.Type.Spec.RenderText != nil:
				text = c.Type.Spec.RenderText(c)
			case c.Type.Spec.LeafText != nil:
				text = c.Type.Spec.LeafText(c)
			}
			add(piece{text: text, marks: c.Marks, leaf: c, classes: classes(at, at+size)}, at, size, true)
		}
		at += size
	}
	composed(at)
	if slices.EqualFunc(pieces, b.pieces, samePiece) && slices.Equal(runs, b.runs) {
		return
	}
	b.pieces, b.runs = pieces, runs
	b.textRev.Update(func(i int) int { return i + 1 })
}

func samePiece(a, b piece) bool {
	return a.text == b.text && a.composition == b.composition && a.leaf == b.leaf && marksEqual(a.marks, b.marks) && slices.Equal(a.classes, b.classes)
}

// textOf is the text offset of document offset off.
func (b *block) textOf(off int) int {
	for _, r := range b.runs {
		switch {
		case r.docLen == 0:
			continue
		case off < r.docAt+r.docLen:
			if r.atom {
				return r.textAt
			}
			return r.textAt + max(off-r.docAt, 0)
		case off == r.docAt+r.docLen:
			return r.textAt + r.textLen
		}
	}
	if n := len(b.runs); n > 0 {
		return b.runs[n-1].textAt + b.runs[n-1].textLen
	}
	return 0
}

// docOf is the document offset of text offset t: inside an atom, its nearer
// side.
func (b *block) docOf(t int) int {
	for _, r := range b.runs {
		if t > r.textAt+r.textLen {
			continue
		}
		switch {
		case r.docLen == 0:
			return r.docAt
		case r.atom:
			if t-r.textAt > r.textLen/2 {
				return r.docAt + r.docLen
			}
			return r.docAt
		}
		return r.docAt + t - r.textAt
	}
	return b.nowNode.Content.Size
}

func (v *rv) markStyle(p piece, s *Styles) (core.Style, string) {
	var st core.Style
	var class []string
	name := func(n string) {
		if m, ok := s.Marks[n]; ok {
			st = st.Merge(m)
		}
		if c := v.p.Classes.Marks[n]; c != "" {
			class = append(class, c)
		}
	}
	for _, m := range p.marks {
		name(m.Type.Name)
	}
	if p.leaf != nil {
		name(p.leaf.Type.Name)
	}
	if p.composition {
		st = st.Merge(core.Style{TextDecoration: core.TextDecorationUnderline}).Merge(s.Composition)
		class = append(class, v.p.Classes.Composition)
	}
	class = append(class, p.classes...)
	return st, strings.TrimSpace(strings.Join(class, " "))
}

func (v *rv) spans(b *block) []core.Span {
	b.textRev.Get()
	s := v.looks.Get()
	if len(b.pieces) == 0 {
		// An empty line still has a height and a caret.
		return []core.Span{{Content: "​"}}
	}
	out := make([]core.Span, len(b.pieces))
	for i, p := range b.pieces {
		st, class := v.markStyle(p, s)
		out[i] = core.Span{Content: p.text, Style: st, Class: class}
	}
	return out
}

func (v *rv) nodeLook(b *block, s *Styles) (core.Style, string) {
	n := b.node.Get()
	st := s.Nodes[n.Type.Name]
	class := []string{v.p.Classes.Nodes[n.Type.Name]}
	keys := make([]string, 0, len(n.Attrs))
	for k := range n.Attrs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		key := n.Type.Name + "[" + k + "=" + attrText(n.Attrs[k]) + "]"
		if m, ok := s.Nodes[key]; ok {
			st = st.Merge(m)
		}
		if c := v.p.Classes.Nodes[key]; c != "" {
			class = append(class, c)
		}
	}
	switch n.AttrStr("textAlign") {
	case "left":
		st = st.Merge(core.Style{TextAlign: core.TextAlignLeft})
	case "center":
		st = st.Merge(core.Style{TextAlign: core.TextAlignCenter})
	case "right":
		st = st.Merge(core.Style{TextAlign: core.TextAlignRight})
	case "justify":
		st = st.Merge(core.Style{TextAlign: core.TextAlignJustify})
	}
	for _, d := range v.decos {
		if d.Node && d.From == b.pos {
			class = append(class, d.Class)
		}
	}
	if v.nodeSelected(b) {
		st = st.Merge(s.SelectedNode)
		class = append(class, v.p.Classes.SelectedNode)
	}
	return st, strings.Join(slices.DeleteFunc(class, func(s string) bool { return s == "" }), " ")
}

func attrText(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(n)
	}
	return ""
}

func (v *rv) nodeSelected(b *block) bool {
	v.rev.Get()
	n, ok := v.s.State().Selection.(*NodeSelection)
	return ok && n.From() == b.pos
}

func (v *rv) kidViews(b *block) core.Node {
	return core.Each(b.kids.Get, func(k *block) int { return k.id }, func(cx *core.Scope, k func() *block, _ func() int) core.Node {
		return v.blockView(cx, k())
	})
}

func (v *rv) blockView(cx *core.Scope, b *block) core.Node {
	if f := v.p.NodeViews[b.typ.Name]; f != nil {
		var content core.Node
		if !b.typ.IsLeaf() {
			content = v.innerView(cx, b)
		}
		return core.View{Ref: b.ref, Children: []core.Node{f(cx, NodeProps{Node: b.node.Get, Editor: v.s, Content: content,
			Pos:      func() int { b.posRev.Get(); return b.pos },
			Selected: func() bool { return v.nodeSelected(b) },
			SetAttrs: func(a Attrs) {
				tr := v.s.State().Tr()
				for k, val := range a {
					tr.SetNodeAttribute(b.pos, k, val)
				}
				v.s.Dispatch(tr)
			}})}}
	}
	return v.innerView(cx, b)
}

func (v *rv) innerView(cx *core.Scope, b *block) core.Node {
	look := func() core.Style {
		st, _ := v.nodeLook(b, v.looks.Get())
		return st
	}
	class := func() string {
		_, c := v.nodeLook(b, v.looks.Get())
		return c
	}
	bump := func(core.Bounds) { v.geo.Update(func(i int) int { return i + 1 }) }
	switch {
	case b.typ.IsTextblock():
		return core.Text{Ref: b.ref, SpansFunc: func() []core.Span { return v.spans(b) }, StyleFunc: look, ClassFunc: class,
			PointerEvents: core.PointerEventsNone,
			OnLayout:      bump, OnTextLayout: func([]core.TextLine) { v.geo.Update(func(i int) int { return i + 1 }) }}
	case tableRole(b.typ) == "table":
		return v.tableView(cx, b, look, class)
	case b.typ.IsAtom():
		return core.View{Ref: b.ref, StyleFunc: look, ClassFunc: class, OnLayout: bump}
	}
	kids := []core.Node{v.kidViews(b)}
	view := core.View{Ref: b.ref, StyleFunc: look, ClassFunc: class, OnLayout: bump, Children: kids}
	if b.parent != nil && b.parent.typ != nil && isListType(b.parent.typ) {
		view.Style = core.Style{Position: core.Relative}
		view.Children = append(kids, v.marker(b))
	}
	return view
}

// marker is a list item's, placed in the indent before it: it is not in the
// document and takes no positions, but for a task item's box.
func (v *rv) marker(b *block) core.Node {
	list := b.parent
	place := core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(-24), Width: core.Pt(20)}
	if b.typ.Name == "taskItem" || list.typ.Spec.Extra["list"] == true {
		return core.View{Role: core.RoleCheckbox, NoFocus: true, Class: v.p.Classes.TaskCheckbox,
			Selected:  func() bool { c, _ := b.node.Get().Attrs["checked"].(bool); return c },
			StyleFunc: func() core.Style { return place.Merge(v.looks.Get().TaskCheckbox) },
			OnPress: func(core.PressEvent) {
				if !v.s.Editable() {
					return
				}
				c, _ := b.nowNode.Attrs["checked"].(bool)
				v.s.Dispatch(v.s.State().Tr().SetNodeAttribute(b.pos, "checked", !c))
			}}
	}
	return core.Text{Class: v.p.Classes.ListMarker, PointerEvents: core.PointerEventsNone,
		StyleFunc: func() core.Style {
			return place.Merge(core.Style{TextAlign: core.TextAlignRight}).Merge(v.looks.Get().ListMarker)
		},
		ContentFunc: func() string {
			if list.typ.Spec.Extra["listMarker"] == "ordered" {
				start := max(int(list.node.Get().AttrNum("start")), 1)
				return strconv.Itoa(start+b.listIndex.Get()) + "."
			}
			depth := 0
			for p := list.parent; p != nil; p = p.parent {
				if p.typ.Spec.Extra["listMarker"] == "bullet" {
					depth++
				}
			}
			return []string{"•", "◦", "▪"}[depth%3]
		}}
}

func (v *rv) contentBox() core.Node {
	desktop := core.Untracked(v.cx, v.cx.Env().Desktop)
	box := core.View{Ref: v.content, TestID: "rich-editor-content", Class: v.p.Classes.Content, NoFocus: true,
		StyleFunc: func() core.Style {
			return core.Style{Position: core.Relative, MinHeight: core.Pt(v.viewport.Get().Height), Cursor: core.CursorText}.Merge(v.looks.Get().Content)
		},
		OnLayout: func(core.Bounds) { v.geo.Update(func(i int) int { return i + 1 }) },
	}
	if desktop {
		box.OnPressDown = func(e core.PressEvent) { v.pressDown(e.Position, e.Clicks, e.Modifiers) }
		box.Responder = &core.Responder{Claims: []core.TouchClaim{{OnMove: true}},
			OnMove:      func(e core.ResponderEvent) { v.dragMove(e.Position) },
			OnRelease:   func(core.ResponderEvent) { v.dragEnd() },
			OnTerminate: func(core.ResponderEvent) { v.dragEnd() }}
	} else {
		box.OnPress = func(e core.PressEvent) { v.pressDown(e.Position, max(e.Clicks, 1), e.Modifiers) }
	}
	box.Children = []core.Node{v.selectionLayer(), v.kidViews(v.root), v.placeholder(), v.caret(), v.tableControls()}
	return box
}

func (v *rv) placeholder() core.Node {
	if v.p.Placeholder == "" {
		return core.Group{}
	}
	empty := func() bool {
		v.rev.Get()
		d := v.s.State().Doc
		return d.ChildCount() == 1 && d.FirstChild().IsTextblock() && d.FirstChild().Content.Size == 0 && v.composing.Get() == ""
	}
	return core.Show{When: empty, Then: func(cx *core.Scope) core.Node {
		return core.Text{Content: v.p.Placeholder, Class: v.p.Classes.Placeholder, PointerEvents: core.PointerEventsNone,
			StyleFunc: func() core.Style {
				v.geo.Get()
				st := core.Style{Position: core.Absolute}
				if len(v.textblocks) > 0 {
					if b, ok := v.boxOf(v.textblocks[0]); ok {
						st.Left, st.Top = core.Pt(core.Points(b.X)), core.Pt(core.Points(b.Y))
					}
				}
				return st.Merge(v.looks.Get().Placeholder)
			}}
	}}
}

func (v *rv) caret() core.Node {
	shown := func() bool {
		v.rev.Get()
		t, ok := v.s.State().Selection.(*TextSelection)
		return ok && t.Empty() && v.focused.Get() && v.s.Editable()
	}
	return core.Show{When: shown, Then: func(cx *core.Scope) core.Node {
		return core.View{Class: v.p.Classes.Caret, TestID: "rich-editor-caret", PointerEvents: core.PointerEventsNone,
			StyleFunc: func() core.Style {
				v.rev.Get()
				v.geo.Get()
				v.composing.Get()
				r, _ := v.CoordsAtPos(v.s.State().Selection.Head())
				return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(r.X)), Top: core.Pt(core.Points(r.Y)),
					Width: core.Pt(2), Height: core.Pt(core.Points(r.Height))}.Merge(v.looks.Get().Caret)
			}}
	}}
}

func (v *rv) selectionLayer() core.Node {
	rects := core.NewMemoFunc(v.cx, func() []Rect2 {
		v.rev.Get()
		v.geo.Get()
		return v.selectionRects()
	}, slices.Equal[[]Rect2])
	return core.Each(func() []int {
		out := make([]int, len(rects.Get()))
		for i := range out {
			out[i] = i
		}
		return out
	}, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
		return core.View{Class: v.p.Classes.Selection, TestID: "rich-editor-selection", PointerEvents: core.PointerEventsNone,
			StyleFunc: func() core.Style {
				r := Rect2{}
				if all := rects.Get(); i() < len(all) {
					r = all[i()]
				}
				return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(r.X)), Top: core.Pt(core.Points(r.Y)),
					Width: core.Pt(core.Points(r.Width)), Height: core.Pt(core.Points(r.Height))}.Merge(v.looks.Get().Selection)
			}}
	})
}

func (v *rv) bubbleMenu() core.Node {
	if v.p.BubbleMenu == nil {
		return core.Group{}
	}
	shown := core.NewMemo(v.cx, func() bool { return v.p.BubbleMenu() != nil })
	return core.Show{When: shown.Get, Then: func(cx *core.Scope) core.Node {
		placement := v.p.BubbleMenuPlacement
		return core.Overlay{Placement: placement, Offset: v.p.BubbleMenuOffset,
			AnchorBounds: func() core.Bounds {
				v.rev.Get()
				v.geo.Get()
				r, ok := v.SelectionRect()
				origin, have := v.content.Bounds()
				if !ok || !have {
					return core.Bounds{}
				}
				return core.Bounds{Origin: core.Point{X: origin.Origin.X + core.Points(r.X), Y: origin.Origin.Y + core.Points(r.Y)},
					Size: core.Size{Width: core.Points(r.Width), Height: core.Points(r.Height)}}
			},
			Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node { return v.p.BubbleMenu() })}}
	}}
}
