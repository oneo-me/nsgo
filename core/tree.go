package core

import (
	"cmp"
	"errors"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/layout"
)

// The shadow tree has one node per native view and one per region. A
// region is a span of its parent's children that a Show, Each, Dynamic
// or Catch replaces; it mounts nothing natively and lays its children out
// as its parent's.
//
// The Go element layer builds the tree through a handful of operations:
// create a node, set its parts, place it among its siblings, discard it.

// Tree is the shadow tree of one surface. Commit turns its changes into a
// MountBatch. It is not safe for concurrent use: the engine owns it.
type Tree struct {
	root *Scope
	// env is a window's own environment (Scope.Env); the first tree's is
	// the graph's. reported is the platform's last report of it.
	env      *Env
	reported Environment
	// id numbers an opened surface's tree, 0 the first's; window is its
	// handle, closed set once it is gone.
	id       uint32
	window   *Window
	closed   bool
	rt       *runtime
	text     driver.TextEngine
	nodes    []*node // index is NodeID; 0 is unused
	free     []NodeID
	view     *node // the surface's root view
	touches  touchState
	size     Size
	changed  bool
	deleted  []NodeID
	commands []driver.MountItem
	moves    []move
	items    []driver.MountItem
	laidOut  []*node
	transits []*node // nodes whose next frame their layout transition moves
	ending   []*node // nodes with OnTransitionEnd restyled since the last commit
	reports  []report
	stacks   []*node
	// matchGone and matchNew are the views with a MatchID the round
	// deletes and creates, which emitMatches pairs.
	matchGone, matchNew []*node
	// pictured are the nodes whose shaders read pictures; shaderStarts
	// the transitions starting in the next batch, which come first in it;
	// navTops each stack's top screen as the last commit showed it.
	pictured     []*node
	shaderStarts []NodeID
	navTops      map[*node]*node
	lists        []*node
	accessories  []*node
	headerViews  []*node
	status       statusBars
	anim         animations
	layoutAnim   *LayoutAnimation
	fontScale    float32
	// ratio is the device pixels per point layout rounds to.
	ratio       float32
	alerts      map[uint32]alertWait
	waits       map[uint32]wait
	imageInfos  map[uint32]imageInfoWait
	keys        []*keyBinding
	keysChanged bool
	backs       []*backHandler
	// peeking is the screen a back gesture may uncover (Tree.peek).
	peeking *node
	// history is the depth ItemHistory last sent, which historyAgain
	// sends again.
	history      int
	historyAgain bool
	alertID      uint32
	menuActions  map[uint32]menuAction
	popup        popupMenu
	menuID       uint32
	menuGen      uint32
	rtl          bool // the layout direction the nodes were mapped for
	// directionSent records that the platform has the direction and
	// language, the language it was told last and whether the app forced it.
	directionSent bool
	language      string
	forced        bool
	// sticky are the views whose position is Sticky; stuckIn the scroll
	// views that pinned some at the last commit.
	sticky  map[*node]bool
	stuckIn []*node
	// full has the next commit resolve and compare every node; fullRound
	// the round running.
	full, fullRound bool
	// boundaries are the relayout boundaries a change inside marked.
	boundaries []*node
	canvases   []*node
	measures   measureCache
	// fonts counts the fonts registered as the app runs: text prepared
	// with fewer is prepared again.
	fonts uint32
	// followers are the views that follow another's frame, and followMoved
	// says what they follow changed.
	followers   []*node
	followMoved bool
	// edgeBars are the views that are bars over a scroll view's side, and
	// edgeScrolls the scroll views they last reached into.
	edgeBars, edgeScrolls []*node
	// commits counts commits, the last of which started at commitStart;
	// paceStop cancels the frame paced lists wait for.
	commits     int
	commitStart time.Time
	paceStop    func()
	// positional is set once a node has classes that follow its place
	// among its siblings (first:, odd:).
	positional bool
	// bars draws the bars of scroll views that have none of their own,
	// and watches hear scroll views move (scrollbars.go).
	bars    ScrollbarMaker
	watches []*scrollWatch
	// focus is the node with the keyboard focus, whatever tracks it, and
	// focusChain it and the nodes around it, which hold stateFocusWithin
	// (updateFocusWithin). focusMoved asks the next commit to walk them.
	focus      *node
	focusChain []*node
	focusMoved bool
	// pointerStyled is set once a style has named PointerEvents, as apps
	// without one skip their walk; pointersMoved asks the next commit
	// for it (updatePointerEvents).
	pointerStyled, pointersMoved bool
	// replacing holds, by element and component type, the fields an
	// XFunc beside them replaces (twins.go).
	replacing map[reflect.Type][]twin
	// lscratch is where restyle compares a node's layout style: a local
	// one would move to the heap at every restyle.
	lscratch layout.Style
	// lastItems is how many items the last commit sent, lastShared
	// whether its batch was these items (CommitShared).
	lastItems  int
	lastShared bool
}

// report is a handler to run under owner after the commit.
type report struct {
	owner *node
	fn    func()
}

// after runs fn under n's owner once the commit's items are out.
func (t *Tree) after(n *node, fn func()) { t.reports = append(t.reports, report{n, fn}) }

type node struct {
	id       NodeID          // 0 for regions
	kind     driver.ViewKind // 0 for regions
	parent   *node
	children []*node
	owner    *Scope

	responder *Responder // a view's part in the touch responder
	follow    *following // what its frame follows (View.Follow)
	match     string     // View.MatchID
	// tip is the tooltip's: what shows it as the pointer comes onto the
	// view, and hides it as the pointer leaves or presses.
	tip *tooltipHooks

	base   Style
	dyn    *Style // the StyleFunc's, kept once and updated in place
	style  Style  // base and dyn merged, with the states' styles
	states uint8
	// showsExpanded, ownPointer, pointerNone and pointerTarget: View's
	// ShowsExpanded, its element's PointerEvents, and what
	// updatePointerEvents worked out.
	showsExpanded              bool
	ownPointer                 PointerEvents
	pointerNone, pointerTarget bool
	// color is the text color the node has, for CurrentColor.
	color Color
	h     handlers
	props driver.ViewProps
	// bound are the animated values its gestures and scrolling set, as
	// an element last bound them (rebind).
	bound *boundValues
	// material is a view's IOS.BackdropMaterial, elevation its
	// Android.Elevation.
	material  driver.BackdropMaterial
	elevation Points
	// appearance is what its subtree forces (Scope.SetSubtreeAppearance).
	appearance Appearance
	env        *envValues // what its padding's and margin's terms come to
	// envBound is set once an element's terms are read in an effect of
	// their own (followEnvTerms).
	envBound bool
	text     *textNode
	input    *inputNode
	scroll   *scrollNode
	image    *imageNode
	hoist    *hoistNode // an overlay's or a modal's
	hosted   []*node    // the overlays and modals this root view or modal hosts
	// hidden is set, as the last layout found, for a node DisplayNone
	// hides: its own or a shadow ancestor's, across hoisting too.
	hidden bool
	attach *attachNode
	screen *screenNode
	list   *listNode
	canvas *canvasNode
	shader *shaderNode
	fx     *fxNode // an effect's, a transition's or a navigation's shader
	icon   *iconNode
	web    *webNode
	video  *videoNode
	native *nativeNode
	// accessory is an InputAccessoryView's: a layout root as wide as the
	// surface that no view holds.
	accessory *accessoryNode
	// headerView is a view of a screen's system header: a layout root
	// at its content size that the platform puts in the bar.
	headerView bool
	panning    bool           // it follows pans: OnPan or PanValues
	pinching   bool           // it follows pinches: OnPinch or PinchValue
	rotating   bool           // it follows rotations: OnRotation or RotationValue
	anim       *viewAnimation // the animated values bound to the view
	part       PartStates     // the states it is styled in as a control's part
	isStack    bool
	ref        *Ref
	// edgeBar is the scroll view side the view is a bar over.
	edgeBar *EdgeBar
	// place is the holder a place region shows; held the holder whose
	// content this region is.
	place, held *Holder
	// elem is the Element the node is, which its own owner releases.
	elem *Element
	// cls are its Tailwind classes.
	cls *nodeClasses

	frame       Bounds
	layoutDirty bool
	// synced is the count of its children's roundings syncFrames last
	// took their frames after (layout.State.Rounds).
	synced uint64
	// lstate is what layout keeps for the node; lstyle its style mapped for
	// layout, while lstyleOK.
	lstate   layout.State
	lstyle   layout.Style
	lstyleOK bool
	transit  bool        // in Tree.transits
	restyled bool        // in Tree.ending
	endStop  func() bool // stops the OnTransitionEnd waiting for its transition
	queued   bool        // a boundary waiting in Tree.boundaries
	rtl      bool        // laid out right to left
	isRoot   bool        // a layout root: the root view, an overlay or a modal
	fill     *Size       // the size a root fills, if it fills one

	created       bool
	deleted       bool
	childrenDirty bool
	flat          []*node // native children, through regions
	sent          sent

	// A commit visits only what changed (mark, markResolve): dirty views
	// compare their parts with what was sent, and dirtyKids leads the walk
	// to views inside one through the views they are emitted in; resolve
	// and resolveKids do the same for resolving, through shadow parents.
	dirty, dirtyKids        bool
	unresolved, resolveKids bool
	passed                  textFields // what it passed its children last
}

type handlers struct {
	press, longPress, pressDown, pressUp func(PressEvent)
	focusIn, focusOut                    func()
	mouseEnter, mouseLeave, mouseMove    func(MouseEvent)
	textSelection                        func(Bounds)
	focus, blur                          func()
	touch, touchCapture                  func(TouchEvent)
	contextMenu                          func(ContextMenuEvent)
	change, submit                       func(string)
	scroll, pressOutside                 func(Point)
	layout                               func(Bounds)
	cancel                               func()
	loadStart, loadEnd                   func()
	progress                             func(int64, int64)
	load                                 func(Size)
	loadError                            func(error)
	pan                                  func(PanEvent)
	pinch, rotation                      func(GestureEvent)
	wheel                                func(WheelEvent)
	fileEnter, fileLeave                 func()
	fileDrop                             func([]string)
	action                               func(string)
	transitionEnd                        func()
}

type textNode struct {
	// fonts is the tree's fonts it was prepared with.
	fonts     uint32
	content   string
	maxLines  int
	ellipsize EllipsizeMode
	// noScale and maxScale limit the user's text size setting.
	noScale  bool
	maxScale float32
	// fit shrinks the text into its box, down to minScale.
	fit      bool
	minScale float32
	lines    func([]TextLine) // OnTextLayout
	alignV   TextAlignVertical
	spans    []spanNode
	text     driver.AttributedText // what was prepared
	style    driver.TextStyle
	prepared driver.PreparedText
	// layout is the layout sent last, which Ref's text queries read.
	layout driver.TextLayout
	// fitted is the text shrunk to fit its box, when fit shrank it.
	fitted   driver.PreparedText
	stale    bool // content or style changed since prepared
	box      Bounds
	sentBox  Bounds
	shownBox Bounds // where the layout went: box, moved by its vertical alignment
	resent   bool   // a new prepared text needs a new layout
	// attachments counts the spans that are attachments.
	attachments int
	// selectable is the text's own Selectable, over the UserSelect it
	// inherits.
	selectable Selectable
	// noWrap keeps the lines unwrapped (WhiteSpaceNoWrap).
	noWrap bool
	// engine measures the words of minContent, whose width minWidth
	// caches while it is not negative.
	engine   driver.TextEngine
	minWidth Points
	// ratio is the device pixels per point its sizes round up to.
	ratio float32
	// hash is text's layout hash, which measures keys its sizes by.
	hash     uint64
	measures *measureCache
}

// size returns what the prepared text measures wrapped at width.
func (tn *textNode) size(width Points) Size {
	if tn.measures == nil {
		return tn.prepared.Measure(width)
	}
	return tn.measures.size(tn.hash, tn.prepared, width)
}

// minContent returns the text's min-content width: its widest word,
// measured alone.
func (tn *textNode) minContent() Points {
	if tn.minWidth >= 0 {
		return tn.minWidth
	}
	tn.minWidth = 0
	if tn.engine == nil {
		return 0
	}
	seen := map[string]bool{}
	for _, word := range breakWords(tn.text.Text) {
		if seen[word] {
			continue
		}
		seen[word] = true
		var w Points
		if tn.measures != nil {
			w = tn.measures.word(tn.engine, word, tn.text.Style)
		} else {
			p := tn.engine.Prepare(&driver.AttributedText{Text: word, Style: tn.text.Style})
			w = p.Measure(Auto).Width
			p.Release()
		}
		tn.minWidth = max(tn.minWidth, ceil(w, tn.ratio))
	}
	return tn.minWidth
}

// breakWords splits s where lines may break: after spaces and hyphens, and
// around each Han, Kana and Hangul character, as most of Unicode's line
// breaking rules allow. Trailing spaces hang, so words leave them out.
func breakWords(s string) []string {
	var words []string
	start := 0
	cut := func(end int) {
		if w := strings.TrimRight(s[start:end], " \t\n"); w != "" {
			words = append(words, w)
		}
		start = end
	}
	for i, r := range s {
		size := utf8.RuneLen(r)
		switch {
		case wide(r):
			cut(i)
			cut(i + size)
		case r == ' ' || r == '\t' || r == '\n' || r == '-' || r == '\u2014':
			cut(i + size)
		}
	}
	cut(len(s))
	return words
}

// wide reports whether lines may break on either side of r: a Han, Kana
// or Hangul character.
func wide(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

// release frees the text's prepared texts.
func (tn *textNode) release() {
	for _, p := range []*driver.PreparedText{&tn.prepared, &tn.fitted} {
		if *p != nil {
			(*p).Release()
			*p = nil
		}
	}
}

// fontMultiplier is what the user's text size setting, scale, multiplies
// the text's sizes by.
func (tn *textNode) fontMultiplier(scale float32) float32 {
	return fontMultiplier(scale, tn.noScale, tn.maxScale)
}

// fontMultiplier is what scale, the user's text size setting, multiplies
// a text's sizes by when noScale keeps them or maxScale caps it.
func fontMultiplier(scale float32, noScale bool, maxScale float32) float32 {
	if noScale {
		return 1
	}
	if maxScale >= 1 {
		return min(scale, maxScale)
	}
	return scale
}

type spanNode struct {
	content string
	style   Style
	press   func(PressEvent)
	long    func(PressEvent)
	attach  *node        // the view of an attachment
	cls     *spanClasses // its classes, when it has any
}

// attachNode is the view of a text's attachment: a layout root of its own
// at its content size, which the text places in its lines.
type attachNode struct {
	size Size // what the text was prepared with
	cut  bool // cut off by the text's line limit
}

type inputNode struct {
	sent      driver.InputConfig // the config last sent
	config    driver.InputConfig
	valueFn   func() string
	value     string // the app's
	shown     string // what the field shows, as far as the engine knows
	count     uint32 // the platform's last edit
	autoFocus bool
	accessory string // the InputAccessoryView's ID
	// themeSelection and themeCaret are its theme's colors, which the
	// input shows without a SelectionColor of its style's.
	themeSelection, themeCaret Color
	lineH                      Points
	// ratio is the device pixels per point its sizes round up to.
	ratio   float32
	lineFor driver.TextStyle
	// noScale and maxScale limit the user's text size setting.
	noScale  bool
	maxScale float32

	selection   func(TextRange)
	keyPress    func(string)
	composition func(Composition)
	contentSize func(Size)
	caret       func(CaretEvent)
	paste       func(PasteEvent)
	// measured is what a multiline field is measured with: its value, or
	// its placeholder while empty; sized the value's text, for
	// OnContentSizeChange.
	measured, sized preparedInput
	reported        Size // the content size last reported
	reportedOK      bool
}

// preparedInput is text of a field prepared in its style.
type preparedInput struct {
	text     driver.AttributedText
	prepared driver.PreparedText
}

type scrollNode struct {
	// ownBar is what draws its bar over the app's; hideIndicator is what
	// the app set, config's also while a bar stands in for the indicator.
	// barHost holds the bar (scrollbars.go).
	ownBar        ScrollbarMaker
	hideIndicator bool
	barHost       *node
	bar           *Signal[ScrollbarMaker]
	barSize       *Signal[Size]
	// sent and sentContent are the config and content size last sent.
	sent        driver.ScrollConfig
	sentContent Size
	// sentFitted is the content size the platform has (fittedContent), and
	// contentEnd how far down the content reaches, even short of the view.
	sentFitted Size
	contentEnd Points
	config     driver.ScrollConfig
	content    Size
	// offset is the reported offset with the scrolls the platform had not
	// applied yet replayed over it.
	offset Point
	// start is the ContentOffset it was sent before it first showed.
	start  Point
	zoom   float32 // the last reported
	on     scrollSpec
	resend bool // EventRefresh came: send the config again
	// issued counts the ItemScrollTo and ItemScrollBy items sent, and
	// pending holds those a report has not shown applied, oldest first.
	issued  uint32
	applied uint32
	pending []pendingScroll
	// by is the ItemScrollBy the lists ask for in this commit, sent with
	// the content size; to the ItemScrollTo their parent-viewport lists
	// ask for.
	by Point
	to *driver.MountItem
	// dependents are the lists in this view's viewport; sticky the cells
	// sent in its last ItemStickyCells.
	dependents []*listNode
	sticky     []StickyCell
	// reported is when OnScroll last ran, latest the offset it runs with
	// next, and waiting whether a timer waits for the throttle to pass.
	reported time.Time
	latest   Point
	waiting  bool
	// effects are the edge effects it asks for, and edges what its bars
	// measured (markEdgeScrolls).
	effects EdgeEffects
	edges   [4]driver.ScrollEdge
}

// pendingScroll is a scroll the core sent and the platform has not shown
// applied.
type pendingScroll struct {
	n  uint32
	to bool // an ItemScrollTo; else an ItemScrollBy
	p  Point
}

// issue records a scroll sent to the platform.
func (sc *scrollNode) issue(to bool, p Point) {
	sc.issued++
	sc.pending = append(sc.pending, pendingScroll{sc.issued, to, p})
	if to {
		sc.offset = p
	} else {
		sc.offset.X += p.X
		sc.offset.Y += p.Y
	}
}

// report records an offset the platform reported after applying applied
// scrolls: those it had not applied yet still count, so a report that
// predates them does not move the content back.
func (sc *scrollNode) report(p Point, applied uint32) {
	sc.applied = max(sc.applied, applied)
	k := 0
	for k < len(sc.pending) && sc.pending[k].n <= sc.applied {
		k++
	}
	sc.pending = sc.pending[k:]
	for _, s := range sc.pending {
		if s.to {
			p = s.p
		} else {
			p.X += s.p.X
			p.Y += s.p.Y
		}
	}
	sc.offset = p
}

// settled reports whether every scroll the core asked for was applied.
func (sc *scrollNode) settled() bool {
	return len(sc.pending) == 0 && sc.by == (Point{}) && sc.to == nil
}

// main is p along the scroll view's axis.
func (sc *scrollNode) main(p Point) Points {
	if sc.config.Horizontal {
		return p.X
	}
	return p.Y
}

// along is a point offset by d along the scroll view's axis.
func (sc *scrollNode) along(d Points) Point {
	if sc.config.Horizontal {
		return Point{X: d}
	}
	return Point{Y: d}
}

type sent struct {
	frame     Bounds
	viewStyle driver.ViewStyle
	props     driver.ViewProps
	events    driver.EventMask
	children  []*node
	layout    Bounds
	overflow  Bounds // the area ItemOverflow last gave
	// parent is the view the view was last inserted into, nil once
	// removed; removed is set from then until it is inserted again.
	parent  *node
	removed bool
}

// NewTree returns an empty tree under root, laying text out with text.
func NewTree(root *Scope, text driver.TextEngine) *Tree {
	root.rt.tree = nil
	t := newTree(root, text)
	root.rt.tree = t
	return t
}

// newTree returns an empty tree under root, which becomes the graph's
// first unless one is.
func newTree(root *Scope, text driver.TextEngine) *Tree {
	t := &Tree{root: root, rt: root.rt, text: text, nodes: []*node{nil}, measures: newMeasureCache(), ratio: 1}
	t.anim.t = t
	if root.rt.tree != nil {
		root.surf = t
	}
	t.view = t.newNode(driver.KindView, nil, root)
	t.view.style = Style{Direction: Column}
	return t
}

// Mount builds what app returns as the root view's content.
func (t *Tree) Mount(app func(cx *Scope) Node) {
	t.within(t.root, func() {
		t.mount(t.root, t.view, Func(app))
	})
}

// Resize sets the surface size the root fills.
func (t *Tree) Resize(size Size) {
	if size != t.size {
		t.size = size
		t.markLayout(t.view)
		t.changed = true
		t.root.Env().window.Set(size)
	}
}

// SetEnvironment records what the platform reports about where the app
// runs.
func (t *Tree) SetEnvironment(env Environment) {
	t.reported = env
	e := t.root.Env()
	safe, headers := e.safe.GetUntracked(), e.headers
	// In one batch, so what reads the environment sees all of it new.
	t.root.Batch(func() { e.set(env) })
	// Modals and system headers are placed by the safe area.
	if e.safe.GetUntracked() != safe || e.headers != headers {
		t.markLayout(t.view)
	}
	// Images pick their scales, screens their headers from it.
	t.markAll()
}

// Size returns the surface size.
func (t *Tree) Size() Size { return t.size }

// Changed reports whether a Commit would produce items.
func (t *Tree) Changed() bool { return t.changed }

// within runs fn with o owning what it creates, untracked, in a batch.
func (t *Tree) within(o *Scope, fn func()) { t.rt.run(o, fn) }

func (t *Tree) newNode(kind driver.ViewKind, parent *node, owner *Scope) *node {
	n := &node{kind: kind, parent: parent, owner: owner, layoutDirty: true, childrenDirty: true, rtl: t.rtl}
	t.pointersMoved = t.pointerStyled
	if kind != 0 {
		if k := len(t.free); k > 0 {
			n.id = t.free[k-1]
			t.free = t.free[:k-1]
			t.nodes[n.id] = n
		} else {
			n.id = NodeID(len(t.nodes))
			t.nodes = append(t.nodes, n)
		}
	}
	if parent != nil {
		addChild(parent, n)
		t.childrenChanged(parent)
	}
	t.markResolve(n)
	if kind != 0 && owner != nil && owner.rt.subtreeAppearance {
		owner.renderEffect(func() {
			if a := subtreeAppearance(owner); a != n.appearance {
				n.appearance = a
				t.mark(n)
			}
		})
	}
	return n
}

func (t *Tree) newRegion(parent *node, owner *Scope) *node {
	return t.newNode(0, parent, owner)
}

// nativeParent is the view n's native view goes into.
func (n *node) nativeParent() *node {
	if n.hoist != nil {
		return n.hoist.host
	}
	p := n.parent
	for p != nil && p.kind == 0 {
		p = p.parent
	}
	return p
}

func (t *Tree) childrenChanged(n *node) {
	for n != nil && n.kind == 0 {
		n = n.parent
	}
	if n != nil {
		n.flat = nil
		t.reorder(n)
		t.markLayout(n)
		if t.positional || n.cls != nil && n.cls.list.spacing {
			t.classKids(n)
		}
	}
}

// reorder records that n's native children may have changed or moved.
func (t *Tree) reorder(n *node) {
	n.childrenDirty = true
	t.mark(n)
	t.markGroups(n)
}

// markGroups marks the groups n is in that read their texts for a label
// (sentProps): what n shows of its own may have changed.
func (t *Tree) markGroups(n *node) {
	for p := n.parent; p != nil; p = p.parent {
		if p.props.Accessible == AccessibleYes && p.props.Label == "" && p.text == nil {
			t.mark(p)
		}
	}
}

// mark records that n's parts may differ from what was sent, so the next
// commit compares them. A region marks the view it lays out in.
func (t *Tree) mark(n *node) {
	t.changed = true
	for n != nil && n.kind == 0 {
		n = n.parent
	}
	if n == nil {
		return
	}
	n.dirty = true
	for p := n.nativeParent(); p != nil && !p.dirtyKids; p = p.nativeParent() {
		p.dirtyKids = true
	}
}

// markResolve records that n's styles or content changed, so the next
// commit resolves it and sends what changed.
func (t *Tree) markResolve(n *node) {
	t.mark(n)
	t.markGroups(n)
	n.unresolved = true
	for p := n.parent; p != nil && !p.resolveKids; p = p.parent {
		p.resolveKids = true
	}
}

// carry marks the chains above n again after n moved to another parent,
// for the marks inside it.
func (t *Tree) carry(n *node) {
	if n.unresolved || n.resolveKids {
		for p := n.parent; p != nil && !p.resolveKids; p = p.parent {
			p.resolveKids = true
		}
	}
	if n.dirty || n.dirtyKids {
		for p := n.nativeParent(); p != nil && !p.dirtyKids; p = p.nativeParent() {
			p.dirtyKids = true
		}
	}
}

// markAll has the next commit resolve and compare every node, as after a
// change of the environment they all read.
func (t *Tree) markAll() {
	t.full = true
	t.changed = true
}

// touch records that n's parts changed without a layout change.
func (t *Tree) touch(n *node) {
	t.mark(n)
	t.markGroups(n)
}

// relayout records that n's parts changed its layout.
func (t *Tree) relayout(n *node) {
	t.markLayout(n)
	t.markResolve(n)
}

// clear discards a region's children.
func (t *Tree) clear(region *node) {
	for _, c := range region.children {
		t.discard(c)
	}
	region.children = region.children[:0]
	t.childrenChanged(region)
}

// discard deletes n's subtree. Its owner disposes the bindings.
func (t *Tree) discard(n *node) {
	if n.deleted {
		return
	}
	if n.elem != nil && !n.elem.released {
		// An element only leaves; its owner releases it.
		n.parent = nil
		return
	}
	n.deleted = true
	if n.states&stateFocusWithin != 0 || t.focus == n {
		t.focusMoved = true
	}
	if n.place != nil {
		// What it shows is the holder's, which keeps it.
		n.place.unplace(n)
	}
	delete(t.sticky, n)
	if n.ref != nil && n.ref.n == n {
		n.ref.n = nil
	}
	if n.hoist != nil {
		h := n.hoist.host
		t.reorder(h)
		t.markLayout(h)
	}
	if n.accessory != nil {
		t.markAll() // the inputs naming it
	}
	if n.kind != 0 {
		if n.created {
			t.deleted = append(t.deleted, n.id)
			if n.match != "" {
				t.matchGone = append(t.matchGone, n)
			}
		} else {
			t.nodes[n.id] = nil
			t.free = append(t.free, n.id)
		}
	}
	if n.text != nil {
		n.text.release()
	}
	if n.input != nil {
		n.input.measured.release()
		n.input.sized.release()
	}
	for _, c := range n.children {
		t.discard(c)
	}
	t.changed = true
}

// setChildren replaces a region's children with nodes already in it.
func (t *Tree) setChildren(region *node, children []*node) {
	region.children = append(region.children[:0], children...)
	t.childrenChanged(region)
}

func (t *Tree) node(id NodeID) *node {
	if int(id) <= 0 || int(id) >= len(t.nodes) {
		return nil
	}
	if n := t.nodes[id]; n != nil && !n.deleted {
		return n
	}
	return nil
}

// Building elements.

// mount appends what el builds under cx to parent's children.
func (t *Tree) mount(cx *Scope, parent *node, el Node) {
	t.checkTwins(el)
	switch el := el.(type) {
	case nil:
	case mounter:
		el.mountIn(t, cx, parent)
	case Group:
		for _, c := range el {
			t.mount(cx, parent, c)
		}
	case Dynamic:
		t.mountDynamic(cx, parent, el)
	case Show:
		t.mountShow(cx, parent, el)
	case Catch:
		t.mountCatch(cx, parent, el)
	case *Element:
		if el.t == t {
			t.place(el.n, parent, nil)
		}
	case regionMounter:
		el.mount(t, t.newRegion(parent, cx), cx)
	default:
		child := cx.Child()
		t.within(child, func() { t.mount(child, parent, el.Render(child)) })
	}
}

// mounter is an element the tree mounts: each element's own method,
// rather than a case here, links its mounting only into apps that make
// one.
type mounter interface {
	mountIn(t *Tree, cx *Scope, parent *node)
}

type regionMounter interface {
	mount(t *Tree, region *node, owner *Scope)
}

// bindStyle sets n's style, binding StyleFunc when there is one.
func (t *Tree) bindStyle(cx *Scope, n *node, base Style, fn func() Style) {
	n.base = base
	t.restyle(n)
	t.syncPosition(n)
	if fn == nil {
		if safe, keyboard := base.envUses(); safe || keyboard {
			cx.renderEffect(func() {
				t.readEnv(cx, n, &n.base)
				if t.restyle(n) {
					t.markLayout(n)
				}
			})
		}
		return
	}
	cx.renderEffect(func() {
		if n.dyn == nil {
			n.dyn = new(Style)
		}
		*n.dyn = fn()
		t.readEnv(cx, n, &n.base, n.dyn)
		changed := t.restyle(n)
		t.syncPosition(n)
		if changed {
			t.markLayout(n)
		}
	})
}

// restyle merges n's style from its parts and states, which the next
// commit resolves and sends, and reports whether what layout reads of it
// changed: a color does not.
func (t *Tree) restyle(n *node) bool {
	t.markResolve(n)
	if n.h.transitionEnd != nil && n.created && !n.restyled {
		n.restyled = true
		t.ending = append(t.ending, n)
	}
	p := n.props
	if n.input != nil && n.input.config.Disabled {
		p.Disabled = true
	}
	p.Checked = p.Checked || n.part.Checked
	p.Indeterminate = p.Indeterminate || n.part.Indeterminate
	p.Selected = p.Selected || n.part.Selected
	p.Disabled = p.Disabled || n.part.Disabled
	t.showTrigger(n, &p)
	var dyn Style
	if n.dyn != nil {
		dyn = *n.dyn
	}
	own := n.base.Merge(dyn)
	if p.Disabled {
		// A disabled view's own state styles wait, but disabled's; its
		// classes' variants, which merge next, still apply (NSH's
		// state_style).
		disabled := own.Disabled
		var none Style
		own.states().copyFrom(none.states())
		own.Disabled = disabled
	}
	merged := n.withClasses(own)
	var g shownStates
	if gs := merged.GroupStyle; gs != nil {
		g = n.groupState(gs.Name)
	}
	z, sticky := n.stackRank()
	was := &n.style
	lines, overflow, pointer := was.NumberOfLines, was.TextOverflow, was.PointerEvents
	n.style = merged.withStates(statesOf(n.shownInteraction(), &p, n.part.Data), g)
	if p.Disabled {
		// The platform still follows the states a disabled view's own
		// styles wait for, which the event mask reads.
		full := n.withClasses(n.base.Merge(dyn))
		n.style.Hover, n.style.Focus, n.style.Active, n.style.FocusWithin = full.Hover, full.Focus, full.Active, full.FocusWithin
	}
	if n.style.NumberOfLines != lines || n.style.TextOverflow != overflow {
		if n.text != nil {
			n.text.stale = true
		}
	}
	if n.style.PointerEvents != pointer {
		t.pointerStyled, t.pointersMoved = true, true
	}
	n.applyEnv()
	if z2, sticky2 := n.stackRank(); z2 != z || sticky2 != sticky {
		if parent := n.nativeParent(); parent != nil {
			t.reorder(parent)
		}
	}
	if n.screen != nil && n.screen.hidden {
		n.style.Display = DisplayNone
	}
	// Roots are placed by style the layout leaves out of theirs.
	if n.lstyleOK && !n.isRoot && n.hoist == nil {
		t.lscratch = n.layoutStyle()
		if t.lscratch.Equal(&n.lstyle) {
			return false
		}
	}
	n.lstyleOK = false
	// A style transition moves the frame its own change gives n, as CSS
	// transitions a changed width; a parent's resize alone does not.
	if n.created && !n.transit && n.style.Transition.Properties&TransitionLayout != 0 && n.style.Transition.Duration > 0 {
		n.transit = true
		t.transits = append(t.transits, n)
	}
	return true
}

// groupState returns the states of n's nearest group named name, or of
// any group when name is empty.
func (n *node) groupState(name string) shownStates {
	for a := n.parent; a != nil; a = a.parent {
		if g := a.style.Group; g != "" && (name == "" || g == name) {
			p := a.props
			p.Checked = p.Checked || a.part.Checked
			p.Indeterminate = p.Indeterminate || a.part.Indeterminate
			p.Selected = p.Selected || a.part.Selected
			p.Disabled = p.Disabled || a.part.Disabled || a.input != nil && a.input.config.Disabled
			return statesOf(a.shownInteraction(), &p, a.part.Data)
		}
	}
	return shownStates{}
}

// restyleExpanded restyles n, and the view that shows its expanded state,
// as it changed.
func (t *Tree) restyleExpanded(n *node) {
	nodes := []*node{n}
	if n.showsExpanded {
		nodes = append(nodes, n.flatChildren()...)
	}
	for _, k := range nodes {
		if t.restyle(k) {
			t.markLayout(k)
		}
		t.restyleGroup(k)
	}
}

// showTrigger gives n the expanded state of a trigger around it that
// shows it (View.ShowsExpanded), for its styles and the platform's prop.
// The trigger stays the control screen readers hear.
func (t *Tree) showTrigger(n *node, p *driver.ViewProps) {
	f := n.nativeParent()
	if f == nil || !f.showsExpanded {
		return
	}
	if !n.props.Expandable || n.props.Expanded != f.props.Expanded {
		n.props.Expandable, n.props.Expanded = true, f.props.Expanded
		t.touch(n)
	}
	p.Expandable, p.Expanded = true, f.props.Expanded
}

// setState records an interaction state and restyles n if a state style
// depends on it, and the nodes inside it that follow it as their group.
func (t *Tree) setState(n *node, state uint8, on bool) {
	was := n.states
	if on {
		n.states |= state
	} else {
		n.states &^= state
	}
	if n.states == was {
		return
	}
	if n.style.Hover != nil || n.style.Focus != nil || n.style.Active != nil || n.style.FocusWithin != nil {
		if t.restyle(n) {
			t.markLayout(n)
		}
	}
	t.restyleGroup(n)
}

// focusReported records what the platform says of the keyboard focus. A
// view taking it is the focus, in whatever order a move is reported: the
// blur of another view changes nothing, and the root taking it is none.
func (t *Tree) focusReported(n *node, focused bool) {
	switch {
	case focused && n.isRoot:
		n = nil
	case !focused && t.focus != n:
		return
	case !focused:
		n = nil
	}
	if t.focus != n {
		t.focus, t.focusMoved = n, true
		t.changed = true
	}
}

// shownInteraction is n's interaction states with those its part shows.
func (n *node) shownInteraction() uint8 {
	if n.part.Focused {
		return n.states | stateFocused
	}
	return n.states
}

// updateFocusWithin puts stateFocusWithin on the focus and the nodes
// around it, and takes it off those it left: the two chains are walked
// and nothing else. The nodes around are parents in the core's tree,
// where an overlay is the child of the view the app put it in, so the
// focus in a popup is within the view holding its trigger, as in a React
// portal.
func (t *Tree) updateFocusWithin() {
	t.focusMoved = false
	if t.focus != nil && t.focus.deleted {
		t.focus = nil
	}
	old := t.focusChain
	var chain []*node
	for n := t.focus; n != nil; n = n.parent {
		chain = append(chain, n)
	}
	t.focusChain = chain
	for _, n := range old {
		if !n.deleted && !slices.Contains(chain, n) {
			t.setState(n, stateFocusWithin, false)
			t.focusCrossed(n, n.h.focusOut)
		}
	}
	for _, n := range chain {
		if !slices.Contains(old, n) {
			t.focusCrossed(n, n.h.focusIn)
		}
		t.setState(n, stateFocusWithin, true)
	}
}

// focusCrossed runs fn, a view's OnFocusIn or OnFocusOut, after the commit
// that saw the focus cross it.
func (t *Tree) focusCrossed(n *node, fn func()) {
	if fn != nil {
		n.owner.Post(fn)
	}
}

// updatePointerEvents works out what each node mounts of the pointer
// events of the styles around it, as CSS's none inherits: a node that is
// none by its own value or the nearest above mounts box-none when a node
// inside it takes the pointer, none at the top of what it blocks, and
// nothing below that.
func (t *Tree) updatePointerEvents() {
	t.pointersMoved = false
	if !t.pointerStyled || t.view == nil {
		return
	}
	var mark func(n *node, none bool) bool
	mark = func(n *node, none bool) bool {
		own := cmpOr(n.style.PointerEvents, n.ownPointer)
		switch own {
		case PointerEventsNone:
			none = true
		case 0:
		default:
			none = false
		}
		n.pointerNone = none
		// The box values are the view's alone: what is in it inherits auto.
		target := own != 0 && own != PointerEventsNone
		for _, k := range n.children {
			if mark(k, none) {
				target = true
			}
		}
		n.pointerTarget = target
		return target
	}
	var apply func(n *node, covered bool)
	apply = func(n *node, covered bool) {
		v := cmpOr(n.style.PointerEvents, n.ownPointer)
		switch {
		case n.pointerNone && n.pointerTarget:
			v, covered = PointerEventsBoxNone, false
		case n.pointerNone && covered:
			v = PointerEventsAuto
		case n.pointerNone:
			v, covered = PointerEventsNone, true
		case v == PointerEventsOn:
			v = PointerEventsAuto
		}
		if n.props.PointerEvents != v {
			n.props.PointerEvents = v
			t.touch(n)
		}
		for _, k := range n.children {
			apply(k, covered)
		}
	}
	mark(t.view, false)
	apply(t.view, false)
}

// restyleGroup restyles the nodes inside group n that follow its states.
func (t *Tree) restyleGroup(n *node) {
	if n.style.Group == "" {
		return
	}
	t.restyleFollowers(n)
}

// restyleFollowers restyles the nodes inside n that follow a group's
// states.
func (t *Tree) restyleFollowers(n *node) {
	var walk func(c *node)
	walk = func(c *node) {
		for _, k := range c.children {
			if k.style.GroupStyle != nil {
				if t.restyle(k) {
					t.markLayout(k)
				}
			}
			walk(k)
		}
		if c.text != nil {
			for _, sp := range c.text.spans {
				if sp.attach != nil {
					walk(sp.attach)
				}
			}
		}
	}
	walk(n)
}

func (t *Tree) bindBool(cx *Scope, n *node, fn func() bool, dst *bool) {
	if fn == nil {
		return
	}
	cx.renderEffect(func() {
		*dst = fn()
		if t.restyle(n) {
			t.markLayout(n)
		}
		t.restyleGroup(n)
	})
}

func (t *Tree) mountView(cx *Scope, parent *node, v View) {
	n := t.newNode(driver.KindView, parent, cx)
	t.bindClasses(cx, n, v.Class, v.ClassFunc)
	t.bindStyle(cx, n, v.Style, v.StyleFunc)
	t.setViewParts(n, v)
	t.bindAccessibility(cx, n, a11y{labelFunc: v.LabelFunc, value: v.AccessibilityValue, expanded: v.Expanded, expandable: v.Expandable,
		hidden: v.AccessibilityElementsHidden, actions: v.AccessibilityActions, accessible: v.Accessible,
		important: v.ImportantForAccessibility})
	t.bindBool(cx, n, v.Disabled, &n.props.Disabled)
	if v.Follow != nil {
		t.bindFollow(cx, n, v)
	}
	if focusable := v.Focusable; focusable != nil {
		cx.renderEffect(func() {
			n.props.Focusable = focusable()
			n.props.NoFocus = !n.props.Focusable
			t.restyle(n)
		})
	}
	t.bindBool(cx, n, v.Selected, &n.props.Selected)
	t.bindBool(cx, n, v.Checked, &n.props.Checked)
	t.bindBool(cx, n, v.Indeterminate, &n.props.Indeterminate)
	t.bindBool(cx, n, v.Pressed, &n.props.Pressed)
	t.bindBool(cx, n, v.Busy, &n.props.Busy)
	if v.HitSlopFunc != nil {
		cx.renderEffect(func() {
			if e := v.HitSlopFunc(); e != n.props.HitSlop {
				n.props.HitSlop = e
				t.touch(n)
			}
		})
	}
	if v.AccessibilityLevel != nil {
		cx.renderEffect(func() {
			if l := v.AccessibilityLevel(); l != n.props.Level {
				n.props.Level = l
				t.touch(n)
			}
		})
	}
	if v.PartStates != nil {
		cx.renderEffect(func() {
			n.part = v.PartStates()
			if t.restyle(n) {
				t.markLayout(n)
			}
			t.restyleGroup(n)
		})
	}
	t.bindPan(cx, n, v.PanValues)
	t.bindGestureValue(n, driver.AnimBindPinch, v.PinchValue)
	t.bindGestureValue(n, driver.AnimBindRotation, v.RotationValue)
	for _, c := range v.Children {
		t.mount(cx, n, c)
	}
	if v.Tooltip != nil {
		t.mountTooltip(cx, n, v.Tooltip)
	}
}

// setViewParts sets the parts of view n that v gives as values: its
// handlers and props, before the accessibility and states bound over
// them.
func (t *Tree) setViewParts(n *node, v View) {
	n.ownPointer, n.showsExpanded = v.PointerEvents, v.ShowsExpanded
	t.pointersMoved = t.pointerStyled
	n.h = handlers{focusIn: v.OnFocusIn, focusOut: v.OnFocusOut, press: v.OnPress, longPress: v.OnLongPress, pressDown: v.OnPressDown, pressUp: v.OnPressUp,
		mouseEnter: v.OnMouseEnter, mouseLeave: v.OnMouseLeave, mouseMove: v.OnMouseMove,
		textSelection: v.OnTextSelection,
		layout:        v.OnLayout, contextMenu: v.OnContextMenu, touch: v.OnTouch, touchCapture: v.OnTouchCapture,
		pan: v.OnPan, pinch: v.OnPinch, rotation: v.OnRotation, wheel: v.OnWheel,
		fileEnter: v.OnFileDragEnter, fileLeave: v.OnFileDragLeave, fileDrop: v.OnFileDrop,
		focus: v.OnFocus, blur: v.OnBlur, transitionEnd: v.OnTransitionEnd}
	n.h.action = v.OnAccessibilityAction
	n.responder = v.Responder
	n.match = v.MatchID
	n.props = driver.ViewProps{Role: v.Role, Label: v.Label, Hint: v.Hint, LiveRegion: v.LiveRegion, TestID: v.TestID,
		WindowDrag: v.Desktop.WindowDrag, SelectionArea: v.Selectable, PointerEvents: v.PointerEvents, HitSlop: v.HitSlop,
		DelayLongPress: v.DelayLongPress, NoFocus: v.NoFocus, Ripple: v.Android.Ripple}
	n.material, n.elevation = v.IOS.BackdropMaterial, v.Android.Elevation
	if v.EdgeBar.Scroll != nil {
		bar := v.EdgeBar
		n.edgeBar = &bar
		t.edgeBars = append(t.edgeBars, n)
	}
	_, panClaim := v.Responder.panClaim()
	n.panning = v.OnPan != nil || v.PanValues != (AnimatedXY{}) || panClaim
	n.props.PanActivation, n.props.PanScrollHandoff = v.PanActivation, v.PanScrollHandoff
	t.showTrigger(n, &n.props)
	n.pinching, n.rotating = v.OnPinch != nil || v.PinchValue != nil, v.OnRotation != nil || v.RotationValue != nil
	t.setRef(n, v.Ref)
}

// tooltipDelay is how long the pointer rests on a view before its
// tooltip shows, as NSH's and GPUI's.
const tooltipDelay = 500 * time.Millisecond

// mountTooltip shows tip in a hint overlay above n while the pointer
// rests on it, on desktops.
func (t *Tree) mountTooltip(cx *Scope, n *node, tip Node) {
	if n.ref == nil {
		t.setRef(n, &Ref{})
	}
	anchor := n.ref
	if h, ok := tip.(HintTooltip); ok {
		hint, hover := h.Hint(cx, anchor)
		n.tip = &tooltipHooks{show: func() { hover(true) }, hide: func() { hover(false) }}
		t.mount(cx, n, hint)
		return
	}
	shown := NewSignal(cx, false)
	var stop func() bool
	hide := func() {
		if stop != nil {
			stop()
			stop = nil
		}
		shown.Set(false)
	}
	n.tip = &tooltipHooks{show: func() {
		if cx.Env().Desktop() && stop == nil {
			stop = cx.AfterFunc(tooltipDelay, func() { stop = nil; shown.Set(true) })
		}
	}, hide: hide}
	t.mount(cx, n, Show{When: shown.Get, Then: func(*Scope) Node {
		return Overlay{Anchor: anchor, Placement: PlaceTop, Offset: 4, Margin: 4, PointerEvents: PointerEventsNone,
			Children: []Node{tip}}
	}})
}

// tooltipHooks are what a view's tooltip hears of the pointer: show as
// it comes onto the view, hide as it leaves or, where the view takes
// presses already, presses.
type tooltipHooks struct{ show, hide func() }

func (t *Tree) mountText(cx *Scope, parent *node, v Text) {
	n := t.newNode(driver.KindText, parent, cx)
	t.bindClasses(cx, n, v.Class, v.ClassFunc)
	t.bindStyle(cx, n, v.Style, v.StyleFunc)
	n.h = handlers{press: v.OnPress, longPress: v.OnLongPress, layout: v.OnLayout}
	n.props = driver.ViewProps{Role: v.Role, Label: v.Label, Hint: v.Hint, LiveRegion: v.LiveRegion, TestID: v.TestID,
		Selectable: v.Selectable, PointerEvents: v.PointerEvents}
	n.ownPointer = v.PointerEvents
	if n.props.Role == RoleNone {
		n.props.Role = RoleText
	}
	t.bindAccessibility(cx, n, a11y{labelFunc: v.LabelFunc})
	t.setRef(n, v.Ref)
	n.text = &textNode{content: v.Content, maxLines: v.MaxLines, ellipsize: v.EllipsizeMode,
		noScale: v.NoFontScaling, maxScale: v.MaxFontSizeMultiplier,
		fit: v.AdjustsFontSizeToFit, minScale: max(v.MinimumFontScale, 0.1), lines: v.OnTextLayout, stale: true,
		selectable: v.Selectable, minWidth: -1}
	for i, sp := range v.Spans {
		if sp.Attachment != nil {
			w := t.newNode(driver.KindView, n, cx)
			w.attach = &attachNode{}
			t.setClasses(w, sp.Class)
			t.bindStyle(cx, w, sp.Style, sp.StyleFunc)
			t.mount(cx, w, sp.Attachment)
			n.text.spans = append(n.text.spans, spanNode{attach: w})
			n.text.attachments++
			continue
		}
		style, cls := t.spanStyle(n, sp.Class, sp.Style)
		n.text.spans = append(n.text.spans, spanNode{content: sp.Content, style: style, press: sp.OnPress, long: sp.OnLongPress, cls: cls})
		t.listenForSpan(n, sp)
		if sp.ContentFunc != nil {
			cx.renderEffect(func() {
				if s := sp.ContentFunc(); s != n.text.spans[i].content {
					n.text.spans[i].content = s
					n.text.stale = true
					t.markResolve(n) // which lays it out unless it measures as before
				}
			})
		}
		if sp.StyleFunc != nil {
			cx.renderEffect(func() {
				s := sp.Style.Merge(sp.StyleFunc())
				if c := n.text.spans[i].cls; c != nil {
					// The classes go under what StyleFunc gives.
					c.base = s
					var env classEnv
					Untracked(n.owner, func() bool { env = t.classEnv(n); return true })
					s = c.resolve(&env)
				}
				if !reflect.DeepEqual(s, n.text.spans[i].style) {
					n.text.spans[i].style = s
					n.text.stale = true
					t.markResolve(n)
				}
			})
		}
	}
	t.followSpanClasses(n)
	if v.SpansFunc != nil {
		// attached owns the attachments' views, made again whenever the
		// spans change, since a Node cannot be compared.
		var attached *Scope
		cx.renderEffect(func() {
			spans := v.SpansFunc()
			for i, sp := range spans {
				if sp.StyleFunc != nil {
					spans[i].Style = sp.Style.Merge(sp.StyleFunc())
				}
			}
			same := len(spans) == len(n.text.spans) && n.text.prepared != nil
			for i := 0; same && i < len(spans); i++ {
				old := &n.text.spans[i]
				own := old.style
				if old.cls != nil {
					own = old.cls.base
				}
				same = spans[i].Attachment == nil && old.attach == nil && spans[i].Content == old.content && (old.cls == nil) == (spans[i].Class == "") &&
					(old.cls == nil || old.cls.list.class == spans[i].Class) && reflect.DeepEqual(spans[i].Style, own)
			}
			if same {
				return
			}
			if attached != nil {
				attached.Dispose()
				attached = nil
				for _, old := range n.text.spans {
					if old.attach != nil {
						t.discard(old.attach)
					}
				}
				n.children = slices.DeleteFunc(n.children, func(c *node) bool { return c.deleted })
				t.childrenChanged(n)
				n.text.attachments = 0
			}
			n.text.spans = n.text.spans[:0]
			for _, sp := range spans {
				if sp.Attachment != nil {
					if attached == nil {
						attached = &Scope{}
						cx.adopt(attached)
					}
					w := t.newNode(driver.KindView, n, attached)
					w.attach = &attachNode{}
					t.setClasses(w, sp.Class)
					t.bindStyle(attached, w, sp.Style, nil)
					t.within(attached, func() { t.mount(attached, w, sp.Attachment) })
					n.text.spans = append(n.text.spans, spanNode{attach: w})
					n.text.attachments++
					continue
				}
				style, cls := t.spanStyle(n, sp.Class, sp.Style)
				n.text.spans = append(n.text.spans, spanNode{content: sp.Content, style: style, cls: cls})
			}
			t.followSpanClasses(n)
			n.text.stale = true
			t.markResolve(n)
		})
	}
	if v.ContentFunc != nil {
		cx.renderEffect(func() {
			if s := v.ContentFunc(); s != n.text.content || n.text.prepared == nil {
				n.text.content = s
				n.text.stale = true
				t.markResolve(n) // which lays it out unless it measures as before
			}
		})
	}
}

// bindInputTheme keeps input n's theme colors, as cx's palette has them.
func (t *Tree) bindInputTheme(cx *Scope, n *node) {
	cx.renderEffect(func() {
		p, in := cx.Palette(), n.input
		if n.deleted || p.Selection == in.themeSelection && p.Accent == in.themeCaret {
			return
		}
		in.themeSelection, in.themeCaret = p.Selection, p.Accent
		t.markResolve(n)
	})
}

func (t *Tree) mountInput(cx *Scope, parent *node, v Input) {
	n := t.newNode(driver.KindInput, parent, cx)
	t.setClasses(n, v.Class)
	t.bindStyle(cx, n, v.Style, v.StyleFunc)
	n.h = handlers{change: v.OnChange, submit: v.OnSubmit, focus: v.OnFocus, blur: v.OnBlur, layout: v.OnLayout}
	n.props = driver.ViewProps{Role: cmpOr(v.Role, RoleTextField), Label: v.Label, TestID: v.TestID}
	n.h.action = v.OnAccessibilityAction
	t.bindAccessibility(cx, n, a11y{value: v.AccessibilityValue, actions: v.AccessibilityActions, expanded: v.Expanded})
	t.setRef(n, v.Ref)
	in := &inputNode{valueFn: v.Value, value: v.DefaultValue, autoFocus: v.AutoFocus, accessory: v.InputAccessoryViewID,
		selection: v.OnSelectionChange, keyPress: v.OnKeyPress, composition: v.OnCompositionChange,
		contentSize: v.OnContentSizeChange, caret: v.OnCaretMove, paste: v.OnPaste,
		noScale: v.NoFontScaling, maxScale: v.MaxFontSizeMultiplier}
	in.config = driver.InputConfig{Placeholder: v.Placeholder, Masked: v.Masked, Readonly: v.Readonly, KeyboardType: v.KeyboardType,
		Multiline: v.Multiline, MaxLength: max(v.MaxLength, 0), SubmitBehavior: v.SubmitBehavior, ReturnKey: v.ReturnKeyType,
		AutoCapitalize: v.AutoCapitalize, NoAutoCorrect: v.NoAutoCorrect, AutoComplete: v.AutoComplete,
		KeyboardAppearance: v.IOS.KeyboardAppearance, HideCaret: v.HideCaret, PasteBehavior: v.PasteBehavior}
	n.input = in
	t.bindInputTheme(cx, n)
	t.bindBool(cx, n, v.Disabled, &in.config.Disabled)
	if v.PlaceholderFunc != nil {
		cx.renderEffect(func() {
			if p := v.PlaceholderFunc(); p != in.config.Placeholder {
				in.config.Placeholder = p
				t.markResolve(n) // a field fits it
				t.mark(n)
			}
		})
	}
	if v.Value != nil {
		cx.renderEffect(func() {
			in.value = v.Value()
			t.markResolve(n) // a multiline field measures it
		})
	}
	if v.ReadonlyFunc != nil {
		cx.renderEffect(func() {
			if r := v.ReadonlyFunc(); r != in.config.Readonly {
				in.config.Readonly = r
				t.touch(n)
			}
		})
	}
	if v.KeyboardTypeFunc != nil {
		cx.renderEffect(func() {
			if k := v.KeyboardTypeFunc(); k != in.config.KeyboardType {
				in.config.KeyboardType = k
				t.touch(n)
			}
		})
	}
	if v.SelectedRange != nil {
		// After the value, which the commit sends before its commands.
		cx.renderEffect(func() {
			r := v.SelectedRange()
			t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemSelection, ID: n.id, Selection: r})
			t.changed = true
		})
	}
}

// scrollDefaults lets a scroll view shrink below its content, as React
// Native's ScrollView does; its content scrolls instead.
var scrollDefaults = Style{FlexShrink: 1}

func (t *Tree) mountScroll(cx *Scope, parent *node, v Scroll) {
	n := t.newNode(driver.KindScroll, parent, cx)
	t.setClasses(n, v.Class)
	t.bindStyle(cx, n, scrollDefaults.Merge(v.Style), v.StyleFunc)
	n.h = handlers{layout: v.OnLayout}
	if on := v.OnScroll; on != nil {
		n.h.scroll = func(p Point) { on(n.scrollEvent(p)) }
	}
	n.props = driver.ViewProps{Role: v.Role, Label: v.Label, TestID: v.TestID}
	t.bindBool(cx, n, v.Busy, &n.props.Busy)
	t.setRef(n, v.Ref)
	n.scroll = &scrollNode{config: driver.ScrollConfig{Horizontal: v.Horizontal, HideIndicator: v.HideIndicator}, effects: v.EdgeEffects,
		ownBar: v.Bar, hideIndicator: v.HideIndicator}
	t.bindScrollSpec(cx, n, scrollSpec{behavior: v.Behavior, behaviorFunc: v.BehaviorFunc, offset: v.ContentOffset,
		refreshing: v.Refreshing, onRefresh: v.OnRefresh, onScroll: v.OnScroll, throttle: v.ScrollEventThrottle,
		onDragStart: v.OnScrollDragStart, onDragEnd: v.OnScrollDragEnd,
		onMomentum: v.OnMomentumStart, onMomentumEnd: v.OnMomentumEnd, onScrollEnd: v.OnScrollEnd,
		minZoom: v.MinZoomScale, maxZoom: v.MaxZoomScale, zoom: v.ZoomScale, zoomFunc: v.ZoomScaleFunc,
		noBouncesZoom: v.NoBouncesZoom, noPinch: v.NoPinchZoom, centerContent: v.CenterContent})
	t.bindScroll(cx, n, v.OffsetValues, v.PullValue)
	for _, c := range v.Children {
		t.mount(cx, n, c)
	}
	t.refreshBar(n)
}

func (t *Tree) setRef(n *node, r *Ref) {
	if r != nil {
		r.t, r.n = t, n
		n.ref = r
	}
}

// branch builds what fn returns in region under a new scope of owner's.
func (t *Tree) branch(owner *Scope, region *node, fn func(cx *Scope) Node) *Scope {
	s := &Scope{}
	owner.adopt(s)
	if fn != nil {
		t.within(s, func() { t.mount(s, region, fn(s)) })
	}
	return s
}

func (t *Tree) mountDynamic(cx *Scope, parent *node, d Dynamic) {
	region := t.newRegion(parent, cx)
	var b *Scope
	cx.renderEffect(func() {
		if b != nil {
			b.Dispose()
			t.clear(region)
		}
		b = &Scope{}
		cx.adopt(b)
		// fn's own reads are tracked; what it returns mounts untracked.
		rt := t.rt
		prevOwner := rt.owner
		rt.owner = b
		var el Node
		rt.guard(b, func() { el = d(b) })
		rt.owner = prevOwner
		t.within(b, func() { t.mount(b, region, el) })
	})
}

func (t *Tree) mountShow(cx *Scope, parent *node, s Show) {
	region := t.newRegion(parent, cx)
	when := NewMemo(cx, s.When)
	var b *Scope
	cx.renderEffect(func() {
		on := when.Get()
		if b != nil {
			b.Dispose()
			t.clear(region)
		}
		if on {
			b = t.branch(cx, region, s.Then)
		} else {
			b = t.branch(cx, region, s.Else)
		}
	})
}

func (t *Tree) mountCatch(cx *Scope, parent *node, c Catch) {
	region := t.newRegion(parent, cx)
	failure := NewSignalFunc[error](cx, nil, nil)
	reset := func() { failure.Set(nil) }
	var b *Scope
	cx.renderEffect(func() {
		err := failure.Get()
		if b != nil {
			b.Dispose()
			t.clear(region)
		}
		b = &Scope{}
		cx.adopt(b)
		if err == nil {
			b.catch = func(err error) { failure.Set(err) }
			t.within(b, func() { t.mount(b, region, c.Try(b)) })
		} else if c.Fallback != nil {
			t.within(b, func() { t.mount(b, region, c.Fallback(b, err, reset)) })
		}
	})
}

// Events.

// Dispatch handles a platform event.
func (t *Tree) Dispatch(ev driver.Event) {
	if ev.Kind >= driver.EventWindowClose && t.surfaceEvent(ev) {
		return
	}
	switch ev.Kind {
	case driver.EventTouch:
		t.touched(ev)
		return
	case driver.EventBack:
		n := t.node(ev.ID)
		// Escape on a view of an overlay is that overlay's.
		var overlay *node
		for p := n; p != nil && overlay == nil; p = p.parent {
			if p.h.cancel != nil {
				overlay = p
			}
		}
		switch {
		case n != nil && n.screen != nil:
			t.backFrom(n)
		case overlay != nil:
			t.within(overlay.owner, overlay.h.cancel)
		default:
			t.back()
		}
		t.peek(nil, nil)
		// The browser took an entry back for it, which a back the app
		// refused gets again.
		t.historyAgain = true
		t.changed = true
		return
	case driver.EventBackCancelled:
		t.peek(nil, nil)
		return
	case driver.EventBackStarted:
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemBackPreview, ID: t.backPreview()})
		t.changed = true
		return
	case driver.EventScreenHeader:
		if n := t.node(ev.ID); n != nil && n.screen != nil {
			t.screenHeader(n, Points(ev.Value))
		}
		return
	case driver.EventScreenInsets:
		if n := t.node(ev.ID); n != nil && n.screen != nil {
			if r := n.screen.reported.GetUntracked(); r == nil || *r != ev.Insets {
				in := ev.Insets
				t.within(n.owner, func() { n.screen.reported.Set(&in) })
			}
		}
		return
	case driver.EventRequestClose, driver.EventModalSize, driver.EventModalShown, driver.EventModalDismissed:
		if n := t.node(ev.ID); n != nil && n.hoist != nil && n.hoist.modal != nil {
			t.modalEvent(n, ev)
		}
		return
	case driver.EventAlert:
		t.answerAlert(ev.Count, ev.Index)
		return
	case driver.EventClipboard, driver.EventOpenURL, driver.EventPick, driver.EventFiles, driver.EventUpdate:
		t.answer(ev)
		return
	case driver.EventMenuClosed:
		t.popupClosed(ev.Count)
		return
	case driver.EventSpeech:
		t.rt.speechEvent(ev)
		return
	case driver.EventImageInfo:
		t.answerImageInfo(ev)
		return
	case driver.EventKey:
		t.pressKey(ev.Text)
		return
	case driver.EventMenu:
		t.chooseMenu(ev.Count)
		return
	case driver.EventAnimation:
		t.animationEvent(ev)
		return
	}
	n := t.node(ev.ID)
	if n == nil {
		return
	}
	if n.web != nil {
		t.dispatchWeb(n, ev)
		return
	}
	if n.video != nil && t.dispatchVideo(n, ev) {
		return
	}
	if n.native != nil && t.dispatchNative(n, ev) {
		return
	}
	h := &n.h
	call := func(fn func()) {
		if fn != nil {
			t.within(n.owner, fn)
		}
	}
	press := func(fn func(PressEvent)) {
		if fn != nil {
			t.within(n.owner, func() { fn(pressOf(ev)) })
		}
	}
	mouse := func(fn func(MouseEvent)) {
		if fn != nil {
			t.within(n.owner, func() { fn(mouseOf(ev)) })
		}
	}
	switch ev.Kind {
	case driver.EventPress:
		t.setState(n, statePressed, false)
		if n.props.Disabled {
			break
		}
		// A span's own press, else the text's.
		if tn := n.text; tn != nil && len(tn.spans) > 0 {
			if sp := tn.spanAt(ev.Index); sp != nil && sp.press != nil {
				press(sp.press)
				break
			}
		}
		press(h.press)
	case driver.EventLongPress:
		if n.props.Disabled {
			break
		}
		if tn := n.text; tn != nil && len(tn.spans) > 0 {
			if sp := tn.spanAt(ev.Index); sp != nil && sp.long != nil {
				press(sp.long)
				break
			}
		}
		press(h.longPress)
	case driver.EventAccessibilityAction:
		if h.action != nil && !n.props.Disabled {
			t.within(n.owner, func() { h.action(ev.Text) })
		}
	case driver.EventPressDown:
		t.setState(n, statePressed, !n.props.Disabled)
		press(h.pressDown)
		if n.tip != nil {
			call(n.tip.hide)
		}
	case driver.EventPressUp:
		t.setState(n, statePressed, false)
		press(h.pressUp)
	case driver.EventMouseEnter:
		t.setState(n, stateHovered, true)
		mouse(h.mouseEnter)
		if n.tip != nil {
			call(n.tip.show)
		}
	case driver.EventMouseLeave:
		t.setState(n, stateHovered, false)
		mouse(h.mouseLeave)
		if n.tip != nil {
			call(n.tip.hide)
		}
	case driver.EventMouseMove:
		mouse(h.mouseMove)
	case driver.EventFocus:
		t.setState(n, stateFocused, true)
		t.focusReported(n, true)
		call(h.focus)
	case driver.EventBlur:
		t.setState(n, stateFocused, false)
		t.focusReported(n, false)
		call(h.blur)
	case driver.EventChangeText:
		in := n.input
		if in == nil {
			return
		}
		in.shown, in.value, in.count = ev.Text, ev.Text, ev.Count
		if h.change != nil {
			t.within(n.owner, func() { h.change(ev.Text) })
		}
		// A controlled field the app did not follow goes back to its value.
		if in.valueFn != nil {
			if v := Untracked(t.root, in.valueFn); v != in.value {
				in.value = v
			}
		}
		t.markResolve(n)
	case driver.EventSubmitEditing:
		if h.submit != nil {
			t.within(n.owner, func() { h.submit(ev.Text) })
		}
	case driver.EventSelectionChange:
		if in := n.input; in != nil && in.selection != nil {
			t.within(n.owner, func() { in.selection(TextRange{Start: ev.Index, End: int(ev.Count)}) })
		}
		if h.textSelection != nil {
			t.within(n.owner, func() { h.textSelection(Bounds{Origin: ev.Point, Size: ev.Size}) })
		}
	case driver.EventKeyPress:
		if in := n.input; in != nil && in.keyPress != nil {
			t.within(n.owner, func() { in.keyPress(ev.Text) })
		}
	case driver.EventCompositionChange:
		if in := n.input; in != nil && in.composition != nil {
			c := Composition{Composing: !ev.Finished, Text: ev.Text}
			if ev.Finished {
				c.Text = ""
			}
			t.within(n.owner, func() { in.composition(c) })
		}
	case driver.EventCaretMove:
		if in := n.input; in != nil && in.caret != nil {
			b := Bounds{Origin: ev.Point, Size: ev.Size}
			page := b
			o := t.surfaceBounds(n).Origin
			page.Origin = Point{X: o.X + b.Origin.X, Y: o.Y + b.Origin.Y}
			t.within(n.owner, func() { in.caret(CaretEvent{Bounds: b, PageBounds: page}) })
		}
	case driver.EventPaste:
		if in := n.input; in != nil && in.paste != nil {
			if p, ok := ev.Data.(*PasteEvent); ok {
				t.within(n.owner, func() { in.paste(*p) })
			}
		}
	case driver.EventPan:
		if ev.Phase == GestureBegan {
			t.panActivated(n)
		}
		if h.pan != nil {
			if ev.Phase == GestureBegan {
				t.setState(n, statePressed, false) // the pan took the press
			}
			t.within(n.owner, func() {
				h.pan(PanEvent{Phase: ev.Phase, Translation: ev.Point, Velocity: ev.Velocity, Values: ev.Values})
			})
		}
	case driver.EventPinch, driver.EventRotation:
		fn := h.pinch
		if ev.Kind == driver.EventRotation {
			fn = h.rotation
		}
		if fn != nil {
			t.within(n.owner, func() {
				fn(GestureEvent{Phase: ev.Phase, Value: ev.Value, Velocity: float32(ev.Velocity.X), Position: ev.Point})
			})
		}
	case driver.EventWheel:
		if h.wheel != nil {
			t.within(n.owner, func() {
				h.wheel(WheelEvent{Position: ev.Point, Delta: ev.Delta, Modifiers: ev.Modifiers, Touchpad: ev.Precise,
					Phase: ev.Phase, Velocity: ev.Velocity})
			})
		}
	case driver.EventContextMenu:
		if h.contextMenu != nil && !n.props.Disabled {
			t.within(n.owner, func() { h.contextMenu(ContextMenuEvent{ev.Point, ev.Page, ev.Modifiers}) })
		}
	case driver.EventPressOutside:
		if n.hoist != nil {
			t.pressOutside(n, ev.Point)
		}
	case driver.EventFileDragEnter:
		call(h.fileEnter)
	case driver.EventFileDragLeave:
		call(h.fileLeave)
	case driver.EventFileDrop:
		if h.fileDrop != nil && !n.props.Disabled {
			t.within(n.owner, func() { h.fileDrop(ev.URIs) })
		}
	case driver.EventLoadStart:
		call(h.loadStart)
	case driver.EventLoadEnd:
		call(h.loadEnd)
	case driver.EventProgress:
		if h.progress != nil {
			t.within(n.owner, func() { h.progress(ev.Loaded, ev.Total) })
		}
	case driver.EventLoad:
		if h.load != nil {
			t.within(n.owner, func() { h.load(ev.Size) })
		}
	case driver.EventError:
		if h.loadError != nil {
			t.within(n.owner, func() { h.loadError(errors.New(ev.Text)) })
		}
	case driver.EventScrollDragStart, driver.EventScrollDragEnd, driver.EventMomentumStart, driver.EventMomentumEnd,
		driver.EventScrollEnd, driver.EventRefresh:
		if n.scroll != nil {
			t.scrolled(n, ev)
			t.dispatchScroll(n, ev)
		}
	case driver.EventScroll:
		if n.scroll != nil {
			t.scrolled(n, ev)
			if ev.Value > 0 {
				n.scroll.zoom = ev.Value
			}
			if anchored(t.view) {
				t.markLayout(t.view) // anchored overlays follow the scroll
				t.changed = true
			}
		}
		if h.scroll != nil {
			if n.scroll != nil {
				t.reportScroll(n, ev.Point)
			} else {
				t.within(n.owner, func() { h.scroll(ev.Point) })
			}
		}
	}
}

// spanAt returns the span holding byte offset i of the text.
// listenForSpan has text n listen to the presses span sp takes, which
// come with the offset of the span.
func (t *Tree) listenForSpan(n *node, sp Span) {
	if sp.OnPress != nil && n.h.press == nil {
		n.h.press = func(PressEvent) {}
	}
	if sp.OnLongPress != nil && n.h.longPress == nil {
		n.h.longPress = func(PressEvent) {}
	}
}

func (tn *textNode) spanAt(i int) *spanNode {
	for k, sp := range tn.text.Spans {
		if i >= sp.Start && i < sp.End && k < len(tn.spans) {
			return &tn.spans[k]
		}
	}
	return nil
}

// HitTest returns the innermost pressable view at p in surface points,
// for platforms that draw their own views.
func (t *Tree) HitTest(p Point) NodeID {
	id, _ := hit(t.view, p)
	return id
}

// PressAt dispatches ev as a press on the innermost pressable view at its
// Page, in surface points, with its Point in that view.
func (t *Tree) PressAt(ev driver.Event) {
	if id, local := hit(t.view, ev.Page); id != 0 {
		ev.Kind, ev.ID, ev.Point = driver.EventPress, id, local
		t.Dispatch(ev)
	}
}

func hit(n *node, p Point) (NodeID, Point) {
	return hitWith(n, p, func(n *node) bool { return n.h.press != nil || n.kind == driver.KindInput })
}

// hitWith returns the innermost view at p in n's parent that ok accepts,
// and p in it.
func hitWith(n *node, p Point, ok func(*node) bool) (NodeID, Point) {
	self, inside := driver.Reach(n.frame, &n.props, n.sent.overflow, p)
	if n.style.Display == DisplayNone || n.style.Visibility == VisibilityHidden || n.hidden || !self && !inside ||
		(n.attach != nil && n.attach.cut) {
		return 0, Point{}
	}
	local := Point{X: p.X - n.frame.Origin.X, Y: p.Y - n.frame.Origin.Y}
	at := local
	if n.scroll != nil {
		local.X += n.scroll.offset.X
		local.Y += n.scroll.offset.Y
	}
	if inside {
		kids := n.nativeChildren()
		for i := len(kids) - 1; i >= 0; i-- {
			if id, in := hitWith(kids[i], local, ok); id != 0 {
				return id, in
			}
		}
	}
	if self && ok(n) {
		return n.id, at
	}
	return 0, Point{}
}

// command queues a command item for n after the next commit's changes.
func (t *Tree) command(kind driver.ItemKind, n *node) {
	t.commands = append(t.commands, driver.MountItem{Kind: kind, ID: n.id})
	t.changed = true
}

// flatChildren returns the views laid out as n's children: its children
// and those of its regions, without the overlays and modals it hoists.
func (n *node) flatChildren() []*node {
	if n.flat == nil {
		n.flat = appendNative(make([]*node, 0, len(n.children)), n.children)
	}
	return n.flat
}

func appendNative(out []*node, children []*node) []*node {
	for _, c := range children {
		switch {
		case c.kind == 0:
			out = appendNative(out, c.children)
		case c.hoist == nil && c.attach == nil && c.accessory == nil && !c.headerView:
			out = append(out, c)
		}
	}
	return out
}

// nativeChildren returns n's native children: the views it lays out, a
// text's attachments, then the overlays and modals it hosts, last on top.
func (n *node) nativeChildren() []*node {
	attached := n.text != nil && n.text.attachments > 0
	flat := stacked(n.flatChildren())
	if len(n.hosted) == 0 && !attached {
		return flat
	}
	out := append([]*node(nil), flat...)
	if attached {
		for _, sp := range n.text.spans {
			if sp.attach != nil {
				out = append(out, sp.attach)
			}
		}
	}
	for _, h := range n.hosted {
		if !h.deleted {
			out = append(out, h)
		}
	}
	return out
}

// stackRank orders a view among its siblings: by ZIndex, a sticky view
// above the others of its ZIndex.
func (n *node) stackRank() (int, bool) {
	return n.style.ZIndex, n.style.Position == Sticky
}

// stacked returns kids in the order they stack, lowest first: by their
// stack ranks, the order they came in among equals. It returns kids
// itself when that is their order.
func stacked(kids []*node) []*node {
	sorted := true
	for i := 1; i < len(kids) && sorted; i++ {
		a, as := kids[i-1].stackRank()
		b, bs := kids[i].stackRank()
		sorted = a < b || a == b && (!as || bs)
	}
	if sorted {
		return kids
	}
	out := slices.Clone(kids)
	slices.SortStableFunc(out, func(x, y *node) int {
		a, as := x.stackRank()
		b, bs := y.stackRank()
		if a != b {
			return cmp.Compare(a, b)
		}
		switch {
		case as == bs:
			return 0
		case as:
			return 1
		}
		return -1
	})
	return out
}

// takesPresses reports whether n's handlers hear presses.
func (n *node) takesPresses() bool {
	h := &n.h
	return h.press != nil || h.longPress != nil || h.pressDown != nil || h.pressUp != nil
}

// insidePressable reports whether a view around n in its layer takes
// presses.
func (n *node) insidePressable() bool {
	for p := n.parent; p != nil && n.hoist == nil; n, p = p, p.parent {
		if p.takesPresses() {
			return true
		}
	}
	return false
}

func (n *node) events() driver.EventMask {
	var m driver.EventMask
	bit := func(k driver.EventKind, on bool) {
		if on {
			m |= 1 << k
		}
	}
	h, s := &n.h, &n.style
	// A group reports the states the styles inside it follow.
	group := s.Group != ""
	// Presses a view follows only for its styles go to a view around it
	// that takes them, as a press on a button's face is the button's.
	pressStyled := (s.Active != nil || group) && (n.takesPresses() || !n.insidePressable())
	bit(driver.EventPress, h.press != nil)
	bit(driver.EventLongPress, h.longPress != nil)
	// A press hides the tooltip of a view that takes presses.
	bit(driver.EventPressDown, h.pressDown != nil || pressStyled || n.tip != nil && h.press != nil)
	bit(driver.EventPressUp, h.pressUp != nil || pressStyled)
	bit(driver.EventMouseEnter, h.mouseEnter != nil || s.Hover != nil || group || n.tip != nil)
	bit(driver.EventMouseLeave, h.mouseLeave != nil || s.Hover != nil || group || n.tip != nil)
	bit(driver.EventMouseMove, h.mouseMove != nil)
	bit(driver.EventTouch, n.hearsTouches())
	// The focus of any view that can take it is reported, for the core's
	// own focus (focusReported).
	focusable := n.kind == driver.KindInput || n.takesPresses() || n.props.Focusable
	bit(driver.EventFocus, h.focus != nil || s.Focus != nil || focusable)
	bit(driver.EventBlur, h.blur != nil || s.Focus != nil || focusable)
	bit(driver.EventSubmitEditing, h.submit != nil)
	bit(driver.EventScroll, h.scroll != nil)
	bit(driver.EventPressOutside, h.pressOutside != nil)
	bit(driver.EventContextMenu, h.contextMenu != nil)
	bit(driver.EventLoadStart, h.loadStart != nil)
	bit(driver.EventProgress, h.progress != nil)
	bit(driver.EventLoad, h.load != nil)
	bit(driver.EventError, h.loadError != nil)
	bit(driver.EventLoadEnd, h.loadEnd != nil)
	bit(driver.EventChangeText, n.kind == driver.KindInput)
	bit(driver.EventSelectionChange, h.textSelection != nil)
	if in := n.input; in != nil {
		bit(driver.EventSelectionChange, in.selection != nil)
		bit(driver.EventKeyPress, in.keyPress != nil)
		bit(driver.EventCompositionChange, in.composition != nil)
		bit(driver.EventCaretMove, in.caret != nil)
		bit(driver.EventPaste, in.paste != nil)
	}
	bit(driver.EventPan, n.panning)
	bit(driver.EventPinch, n.pinching)
	bit(driver.EventRotation, n.rotating)
	bit(driver.EventWheel, h.wheel != nil)
	bit(driver.EventFileDragEnter, h.fileEnter != nil)
	bit(driver.EventFileDragLeave, h.fileLeave != nil)
	bit(driver.EventFileDrop, h.fileDrop != nil)
	bit(driver.EventAccessibilityAction, h.action != nil)
	if n.native != nil {
		bit(driver.EventNative, n.native.event != nil)
		bit(driver.EventNativeSize, true)
	}
	if n.web != nil {
		m |= n.web.events()
	}
	if n.video != nil {
		m &^= maskOf(driver.EventLoadStart, driver.EventProgress, driver.EventLoad, driver.EventError)
		m |= n.video.events()
	}
	if n.scroll != nil {
		m |= n.scroll.events()
	}
	return m
}

func maskOf(kinds ...driver.EventKind) driver.EventMask {
	var m driver.EventMask
	for _, k := range kinds {
		m |= 1 << k
	}
	return m
}
