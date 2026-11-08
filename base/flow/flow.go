// Package flow draws a directed diagram, such as a workflow, a dependency
// graph or a plan of tasks, and lays it out automatically: nodes fall into
// ranks along the edges' direction, ranks are ordered to avoid crossings
// and edges run between them as orthogonal lines. Applications build each
// node's content and own selection and what opening a node means; the view
// owns layout, routing and the camera.
package flow

import (
	"github.com/oneo-me/nsgo/core"
)

// Direction is the way edges point.
type Direction uint8

const (
	// TopToBottom places sources at the top and edges pointing down.
	TopToBottom Direction = iota
	// LeftToRight places sources at the left and edges pointing right.
	LeftToRight
)

// Node is one card of the diagram. ID must be nonempty, unique and stable
// across changes; later duplicates are ignored. Label is its accessible
// name, the ID when empty.
//
// Content is the node's face, made when its ID first appears and kept
// while the ID stays, following the signals it reads. Positive Width and
// Height fix the node's size; otherwise the layout measures Content,
// laying the diagram out again when that size changes. Muted fades the
// node and its edges, as for skipped or inactive steps.
type Node struct {
	ID, Label     string
	Width, Height core.Points
	Muted         bool
	Content       core.Node
}

// Edge points from Source to Target. Edges to unknown nodes and self-loops
// are ignored; edges closing a cycle are drawn against the layout's
// direction. Label, when set, sits on the line. A zero Color takes the
// look's line and a zero Width 1.5 points.
type Edge struct {
	Source, Target string
	Label          string
	Dashed         bool
	Color          core.Color
	Width          core.Points
}

// Data is what the diagram shows; the view never modifies it.
type Data struct {
	Nodes []Node
	Edges []Edge
}

// Look colors what the view draws itself. Line is an edge's color and
// Emphasis that of the edges of the hovered or selected node; Label,
// LabelBackground and LabelBorder color an edge's label. Zero fields take
// the theme's text, with lines and the label's border faded, and the
// label's background is the theme's background.
type Look struct {
	Line, Emphasis                      core.Color
	Label, LabelBackground, LabelBorder core.Color
}

// View is the diagram: it fills what the app sizes and lays out Data,
// read as a signal, again when it changes. Drag to pan, scroll with two
// fingers on a touchpad to pan, turn the wheel or pinch to zoom around the
// pointer, and double-click empty space to zoom in, or out with Shift.
// Nodes take the keyboard focus in data order when the application can
// select or open them, and the camera follows the focused node.
//
// A press on a node selects it (OnSelect) and a double press opens it
// (OnOpen); a press on empty space selects "". The view finds the node
// under the pointer itself, so what a node holds takes no pointer.
type View struct {
	Data func() Data
	// Direction is the way edges point, read as a signal; nil is top to
	// bottom.
	Direction func() Direction
	// NodeGap separates nodes within a rank (40 when 0) and RankGap the
	// ranks (72 when 0).
	NodeGap, RankGap core.Points
	// Selected is the application's selected node, which the view rings
	// and whose edges it emphasizes.
	Selected func() string
	OnSelect func(id string)
	OnOpen   func(id string)
	// State holds the camera between mounts and controls it; nil keeps one
	// of the view's own.
	State *State
	// Empty replaces the diagram while it has no nodes, an em dash when
	// nil. Overlay sits in the top trailing corner above the diagram, such
	// as zoom controls.
	Empty, Overlay core.Node
	// Label describes the diagram for assistive technology, its count of
	// nodes and edges when empty.
	Label     string
	LabelFunc func() string
	LookFunc  func() Look
	// NodeClass, NodeStyle and NodeStyleFunc are each node's box, around
	// its Content, such as its focus ring; RingClass, RingStyle and
	// RingStyleFunc the ring around the selected node, 3 points outside
	// it, 2 points wide in the text's color unless they say otherwise.
	NodeClass     string
	NodeStyle     core.Style
	NodeStyleFunc func() core.Style
	RingClass     string
	RingStyle     core.Style
	RingStyleFunc func() core.Style
	Class         string
	Style         core.Style
	TestID        string
}

// State retains measured sizes, the layout and the camera between mounts,
// and moves the camera of the view showing it. Its zero value is ready to
// use; keep one per diagram. Its methods run on the UI's goroutine and do
// nothing while no view shows it.
type State struct {
	v *view
	// What outlives a mount: measured sizes by ID and the camera.
	sizes     map[string]core.Size
	cam       camera
	userMoved bool
	placed    bool
}

// Ready reports whether every node has been measured, laid out and shown.
func (s *State) Ready() bool { return s.v != nil && s.v.ready.GetUntracked() }

// Camera returns where the camera puts the diagram's origin in the view,
// and its scale.
func (s *State) Camera() (x, y, zoom float32) {
	return float32(s.cam.x), float32(s.cam.y), s.Zoom()
}

// Zoom returns the camera's scale.
func (s *State) Zoom() float32 {
	if s.cam.zoom == 0 {
		return 1
	}
	return float32(s.cam.zoom)
}

// Fit moves the camera to show the whole diagram.
func (s *State) Fit() {
	if s.v != nil {
		s.v.fit(false)
	}
}

// ZoomIn and ZoomOut scale the view around its center.
func (s *State) ZoomIn() {
	if s.v != nil {
		s.v.zoomAt(s.v.width/2, s.v.height/2, zoomStep, true)
	}
}

func (s *State) ZoomOut() {
	if s.v != nil {
		s.v.zoomAt(s.v.width/2, s.v.height/2, 1/zoomStep, true)
	}
}

// Reveal moves the camera to center the node with id, if it is shown.
func (s *State) Reveal(id string) {
	if s.v != nil {
		s.v.reveal(id)
	}
}

// Position returns the top left corner of the node with id in diagram
// coordinates, and false if it is not laid out.
func (s *State) Position(id string) (x, y float32, ok bool) {
	if s.v == nil {
		return 0, 0, false
	}
	i, ok := s.v.index[id]
	if !ok || i >= len(s.v.result.x) {
		return 0, 0, false
	}
	return float32(s.v.result.x[i]), float32(s.v.result.y[i]), true
}
