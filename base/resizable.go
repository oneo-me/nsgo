package base

import (
	"math"
	"slices"
	"strconv"

	"github.com/oneo-me/nsgo/core"
)

// Resizable panels follow gpui-base's resizable module, as NSH's do: a
// ResizablePanelGroup lays its ResizablePanels out along one axis with a
// handle between each two visible ones, whose drag resizes the panel
// before it; a ResizeHandle is the same band on its own, for anything
// else a drag resizes (a dock's edge).

// Axis is the axis resizable panels are laid out and resized along.
type Axis uint8

const (
	// Horizontal lays panels out side by side, their handles dragged left
	// and right.
	Horizontal Axis = iota
	// Vertical stacks them, their handles dragged up and down.
	Vertical
)

// PanelMinSize is the smallest a resizable panel gets by default, and
// PanelMaxSize what it is unbounded at.
const (
	PanelMinSize core.Points = 100
	PanelMaxSize core.Points = math.MaxFloat32
)

// A handle's band is 9 points across, its line in the middle, and takes
// 10 points more either side on touch screens; screen readers move it 10
// points a step.
const (
	handleBand  core.Points = 9
	handleSlop  core.Points = 10
	handleStep  core.Points = 10
	handleGroup             = "resize-handle"
)

// ResizablePanelGroup lays out its Panels along Axis, each visible
// one at the size it wants: the size the user last dragged it to, else
// its Size. Panels without a size share what is left; when every panel
// has one, they share what is left over in proportion. A group too small
// for them shrinks them, fixed ones aside, and they come back as room
// returns; one too short for their minimums shares its length in
// proportion to them.
//
// A handle sits on each boundary between two visible panels: a band with
// the resize cursor and the adjustable role, whose drag resizes the
// panel before it, taking from the panels after it, nearest first, as
// far as their minimums let it. Screen readers move it 10 points a step.
// The band is a group (Style.Group "resize-handle"), so the app's
// HandleStyle and HandleThumbStyle follow its hover and press with a
// GroupStyle.
type ResizablePanelGroup struct {
	Axis   Axis
	Panels []ResizablePanel
	// OnResize receives every visible panel's size after a drag or a
	// screen reader's step changed them, never after the group resized.
	OnResize func(sizes []core.Points)
	Disabled func() bool

	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	// HandleStyle styles each handle's line: 1 point across and as long
	// as the boundary unless it says otherwise.
	HandleClass     string
	HandleStyle     core.Style
	HandleStyleFunc func() core.Style
	// HandleThumbClass or HandleThumbStyle, when set, styles a grip
	// centered on the line, drawn upright as on a vertical line and
	// turned a quarter on a horizontal one.
	HandleThumbClass     string
	HandleThumbStyle     core.Style
	HandleThumbStyleFunc func() core.Style
	// RenderHandle renders what is inside each handle's line, told which
	// handle it is and how far the pointer has gone with it; it is drawn
	// again when the handle's state changes, never as a drag moves.
	RenderHandle func(cx *core.Scope, h ResizeHandleContext) core.Node
	// HandleLabel names the handles for assistive technology.
	HandleLabel string
	TestID      string
}

// ResizeHandleState is how far the pointer has gone with a handle. A
// drag takes the pointer off the band within a point or two, so the
// handle follows its progression itself rather than its hover.
type ResizeHandleState uint8

const (
	// HandleIdle is a handle the pointer is elsewhere from.
	HandleIdle ResizeHandleState = iota
	// HandleHovered is one the pointer is over.
	HandleHovered
	// HandlePressed is one the pointer went down on and has not dragged.
	HandlePressed
	// HandleDragging is one being dragged.
	HandleDragging
)

func (s ResizeHandleState) String() string {
	return [...]string{"idle", "hovered", "pressed", "dragging"}[s]
}

// Active reports whether the pointer holds the handle: pressed on it or
// dragging it.
func (s ResizeHandleState) Active() bool { return s == HandlePressed || s == HandleDragging }

// ResizeHandleContext is what RenderHandle is told about a handle.
type ResizeHandleContext struct {
	// Index is the boundary after the Index'th visible panel.
	Index int
	Axis  Axis
	State ResizeHandleState
}

// ResizablePanel is one panel of a ResizablePanelGroup.
type ResizablePanel struct {
	// Size is the size the panel wants along the group, within its
	// range; a new one from the app replaces what the user dragged it to.
	// Without it, or while it returns 0, the panel shares what the others
	// leave.
	Size func() core.Points
	// MinSize and MaxSize bound it: PanelMinSize when MinSize is 0 and
	// none when it is negative, unbounded when MaxSize is 0.
	MinSize, MaxSize core.Points
	// Fixed keeps its size while the group is too small for the panels.
	Fixed bool
	// Visible hides the panel while it returns false; it comes back at
	// the size it wanted.
	Visible func() bool
	// OnResize receives its size after a drag or a step changed it.
	OnResize  func(size core.Points)
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
}

func (p *ResizablePanel) limits() (lo, hi core.Points) {
	lo, hi = p.MinSize, p.MaxSize
	switch {
	case lo == 0:
		lo = PanelMinSize
	case lo < 0:
		lo = 0
	}
	if hi <= 0 {
		hi = PanelMaxSize
	}
	return lo, max(hi, lo)
}

// Render implements core.Node.
func (g ResizablePanelGroup) Render(cx *core.Scope) core.Node {
	horizontal := g.Axis != Vertical
	n := len(g.Panels)
	// user is what a drag gave each panel, NaN for nothing; measured
	// what each was last laid out at.
	user := make([]*core.Signal[core.Points], n)
	measured := make([]core.Points, n)
	for i := range user {
		user[i] = core.NewSignal(cx, core.Points(math.NaN()))
		if f := g.Panels[i].Size; f != nil {
			// A new size from the app replaces the user's.
			first := true
			cx.Effect(func() {
				f()
				if !first {
					cx.Untrack(func() { user[i].Set(core.Points(math.NaN())) })
				}
				first = false
			})
		}
	}
	length := core.NewSignal(cx, core.Points(0))
	visible := func(i int) bool { return g.Panels[i].Visible == nil || g.Panels[i].Visible() }
	shown := core.NewMemoFunc(cx, func() []int {
		var out []int
		for i := range g.Panels {
			if visible(i) {
				out = append(out, i)
			}
		}
		return out
	}, slices.Equal)
	wanted := func(i int) (core.Points, bool) {
		if u := user[i].Get(); !isNaN(u) {
			return u, true
		}
		if f := g.Panels[i].Size; f != nil {
			if size := f(); size > 0 {
				return size, true
			}
		}
		return 0, false
	}
	// scale cuts every minimum by one ratio when the group is too short
	// for them together.
	scale := core.NewMemo(cx, func() float32 {
		var sum core.Points
		for _, i := range shown.Get() {
			lo, _ := g.Panels[i].limits()
			sum += lo
		}
		if l := length.Get(); sum > l && l > 0 {
			return float32(l / sum)
		}
		return 1
	})
	ranged := func(i int) (lo, hi core.Points) {
		lo, hi = g.Panels[i].limits()
		return lo * core.Points(scale.Get()), hi
	}
	panelStyle := func(i int) func() core.Style {
		return func() core.Style {
			if !visible(i) {
				return core.Style{Display: core.DisplayNone}
			}
			lo, hi := ranged(i)
			all := true
			for _, j := range shown.Get() {
				if _, ok := wanted(j); !ok {
					all = false
				}
			}
			var s core.Style
			if w, ok := wanted(i); ok {
				w = min(max(w, lo), hi)
				s.FlexBasis, s.FlexGrow, s.FlexShrink = core.Pt(w), 0, float32(w)
				if all {
					s.FlexGrow = float32(w)
				}
				if g.Panels[i].Fixed {
					s.FlexShrink = 0
				}
			} else {
				s.FlexBasis, s.FlexGrow, s.FlexShrink = core.Pt(0), 1, 1
			}
			if horizontal {
				s.MinWidth, s.MaxWidth = core.Pt(lo), maxLength(hi)
			} else {
				s.MinHeight, s.MaxHeight = core.Pt(lo), maxLength(hi)
			}
			if f := g.Panels[i].StyleFunc; f != nil {
				s = s.Merge(f())
			}
			return s
		}
	}
	disabled := func() bool { return g.Disabled != nil && g.Disabled() }

	// A drag or a step resizes on a state made from the visible panels'
	// sizes as it starts, then reports. Event handlers read untracked.
	var drag, moved *resizeState
	start := func() *resizeState {
		st := &resizeState{container: length.GetUntracked()}
		for _, i := range shown.GetUntracked() {
			lo, hi := ranged(i)
			st.sizes = append(st.sizes, measured[i])
			st.ranges = append(st.ranges, [2]core.Points{lo, hi})
			st.fixed = append(st.fixed, g.Panels[i].Fixed)
		}
		return st
	}
	// apply has the panels a resize moved want their new sizes; the one
	// panel without a size, when there is one, keeps none and takes the
	// rest, which is what the resize gives it.
	apply := func(from, to *resizeState) {
		vis := shown.GetUntracked()
		unsized, count := -1, 0
		for _, i := range vis {
			if _, ok := wanted(i); !ok {
				unsized, count = i, count+1
			}
		}
		cx.Batch(func() {
			for k, i := range vis {
				if to.sizes[k] != from.sizes[k] && (count != 1 || i != unsized) {
					user[i].Set(to.sizes[k])
				}
			}
		})
	}
	report := func(from, to *resizeState) {
		if slices.Equal(from.sizes, to.sizes) {
			return
		}
		for k, i := range shown.GetUntracked() {
			if f := g.Panels[i].OnResize; f != nil && from.sizes[k] != to.sizes[k] {
				f(to.sizes[k])
			}
		}
		if g.OnResize != nil {
			g.OnResize(slices.Clone(to.sizes))
		}
	}
	resize := func(from *resizeState, h int, d core.Points) *resizeState {
		to := from.clone()
		to.resizeAtHandle(h, from.sizes[h]+d)
		apply(from, to)
		return to
	}

	// handle is the handle before panel i.
	handle := func(cx *core.Scope, i int) core.Node {
		at := func() int { return slices.Index(shown.GetUntracked(), i) - 1 }
		pan := func(e core.PanEvent) {
			if disabled() {
				return
			}
			switch e.Phase {
			case core.GestureBegan:
				drag, moved = start(), nil
			case core.GestureEnded, core.GestureCancelled:
				if drag != nil && moved != nil {
					report(drag, moved)
				}
				drag, moved = nil, nil
				return
			}
			if drag == nil || at() < 0 {
				return
			}
			d := e.Translation.X
			if !horizontal {
				d = e.Translation.Y
			} else if cx.Env().RTL() {
				d = -d
			}
			moved = resize(drag, at(), d)
		}
		step := func(name string) {
			h := at()
			if disabled() || h < 0 {
				return
			}
			d := handleStep
			switch name {
			case core.ActionIncrement:
			case core.ActionDecrement:
				d = -d
			default:
				return
			}
			from := start()
			report(from, resize(from, h, d))
		}
		value := func() core.AccessibilityValue {
			vis := shown.Get()
			k := slices.Index(vis, i) - 1
			if k < 0 {
				return core.AccessibilityValue{}
			}
			before := vis[k]
			lo, hi := ranged(before)
			size, ok := wanted(before)
			if !ok {
				size = measured[before]
			}
			return core.AccessibilityValue{Min: float32(lo), Max: float32(min(hi, length.Get())), Now: float32(min(max(size, lo), hi))}
		}
		testID := ""
		if g.TestID != "" {
			testID = g.TestID + "-handle-" + strconv.Itoa(i-1)
		}
		state := core.NewSignal(cx, HandleIdle)
		var content core.Node
		if g.RenderHandle != nil {
			content = core.Dynamic(func(cx *core.Scope) core.Node {
				return g.RenderHandle(cx, ResizeHandleContext{Index: slices.Index(shown.Get(), i) - 1, Axis: g.Axis, State: state.Get()})
			})
		}
		return handleBandNode(handleBandSpec{horizontal: horizontal, line: part{g.HandleClass, g.HandleStyle, g.HandleStyleFunc},
			thumb: part{g.HandleThumbClass, g.HandleThumbStyle, g.HandleThumbStyleFunc}, label: g.HandleLabel, testID: testID,
			disabled: g.Disabled, onPan: pan, onStep: step, value: value, touch: !core.Untracked(cx, cx.Env().Desktop),
			state: state, content: content})
	}

	children := make([]core.Node, 0, 2*n)
	for i := range g.Panels {
		p := g.Panels[i]
		if i > 0 {
			// The handle before panel i, shown while a visible panel is on
			// either side of it.
			children = append(children, core.Show{When: func() bool { return slices.Index(shown.Get(), i) > 0 },
				Then: func(cx *core.Scope) core.Node { return handle(cx, i) }})
		}
		children = append(children, core.View{Class: p.Class, Style: p.Style, StyleFunc: panelStyle(i), TestID: p.TestID,
			OnLayout: func(b core.Bounds) {
				if horizontal {
					measured[i] = b.Size.Width
				} else {
					measured[i] = b.Size.Height
				}
			},
			Children: p.Children})
	}
	style := func() core.Style {
		out := core.Style{Direction: core.Row}
		if !horizontal {
			out.Direction = core.Column
		}
		if g.StyleFunc != nil {
			out = out.Merge(g.StyleFunc())
		}
		return out
	}
	return core.View{Class: g.Class, Style: g.Style, StyleFunc: style, TestID: g.TestID,
		OnLayout: func(b core.Bounds) {
			size := b.Size.Width
			if !horizontal {
				size = b.Size.Height
			}
			if size != length.GetUntracked() {
				length.Set(size)
			}
		},
		Children: children}
}

// maxLength is a maximum size, none when unbounded.
func maxLength(hi core.Points) core.Length {
	if hi >= PanelMaxSize {
		return core.Length{}
	}
	return core.Pt(hi)
}

func isNaN(p core.Points) bool { return p != p }

// handleBandSpec is a handle's band: its axis, parts and behavior.
type handleBandSpec struct {
	horizontal bool
	// edge places the band hugging its container's edge from inside;
	// zero centers it on a zero-length slot between two panels.
	edge        HandleEdge
	line, thumb part
	label       string
	testID      string
	disabled    func() bool
	onPan       func(core.PanEvent)
	onStep      func(string)
	value       func() core.AccessibilityValue
	touch       bool
	// state follows the pointer's progression with the band; content is
	// the app's, inside the line.
	state   *core.Signal[ResizeHandleState]
	content core.Node
}

// HandleEdge is where a ResizeHandle sits on its container.
type HandleEdge uint8

const (
	// HandleCentered straddles the boundary it sits on.
	HandleCentered HandleEdge = iota
	// HandleLeading hugs the container's leading edge from inside: its
	// left, or top across.
	HandleLeading
	// HandleTrailing hugs its trailing edge from inside.
	HandleTrailing
)

// handleBandNode is a handle: a band 9 points across with the resize
// cursor, its line in the middle, or on the edge of the container it
// hugs from inside. Between two panels it takes no room: negative
// margins give back its thickness, so the band lies over the boundary
// within its parent's bounds, where the platforms give it its cursor and
// track the pointer over it.
func handleBandNode(h handleBandSpec) core.Node {
	zero := core.Pt(0)
	band := core.Style{Group: handleGroup, ZIndex: 1, FlexShrink: 0,
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	line := core.Style{}
	// The thumb is centered by alignment on a point of the line in its
	// middle, which holds the quarter turn, so the app's thumb keeps its
	// own Transform. The point is the line's width square, so the thumb
	// lies on the line's pixels when its width is odd too.
	over := core.Style{Position: core.Absolute, Width: core.Pt(1), Height: core.Pt(1),
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	thumb := core.Style{Position: core.Absolute}
	switch h.edge {
	case HandleLeading:
		band.Position = core.Absolute
		band.AlignItems, band.JustifyContent = core.AlignStart, core.JustifyStart
	case HandleTrailing:
		band.Position = core.Absolute
		band.AlignItems, band.JustifyContent = core.AlignEnd, core.JustifyEnd
	}
	// The margins either side give the band's thickness back, in whole
	// points, so its line stays on a pixel of the boundary.
	before := core.Points(math.Floor(float64(handleBand) / 2))
	after := handleBand - before
	var activation core.PanActivation
	if h.horizontal {
		band.Width, band.Cursor = core.Pt(handleBand), core.CursorColResize
		switch h.edge {
		case HandleLeading:
			band.Top, band.Bottom, band.Left = zero, zero, zero
			over.Left = zero
		case HandleTrailing:
			band.Top, band.Bottom, band.Right = zero, zero, zero
			over.Right = zero
		default:
			band.Margin = core.Edges{Left: -before, Right: -after}
			over.Left = core.Pt(before)
		}
		line.Width, line.Height = core.Pt(1), core.Pct(100)
		over.Top = core.Pct(50)
		activation = core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12}
	} else {
		band.Height, band.Cursor = core.Pt(handleBand), core.CursorRowResize
		switch h.edge {
		case HandleLeading:
			band.Left, band.Right, band.Top = zero, zero, zero
			over.Top = zero
		case HandleTrailing:
			band.Left, band.Right, band.Bottom = zero, zero, zero
			over.Bottom = zero
		default:
			band.Margin = core.Edges{Top: -before, Bottom: -after}
			over.Top = core.Pt(before)
		}
		line.Height, line.Width = core.Pt(1), core.Pct(100)
		over.Left = core.Pct(50)
		over.Transform.Rotate = 90
		activation = core.PanActivation{ActiveOffsetY: 2, FailOffsetX: 12}
	}
	var slop core.Edges
	if h.touch {
		if h.horizontal {
			slop = core.Edges{Left: handleSlop, Right: handleSlop}
		} else {
			slop = core.Edges{Top: handleSlop, Bottom: handleSlop}
		}
	}
	parts := []core.Node{core.View{Class: h.line.class, Style: line.Merge(h.line.style), StyleFunc: h.line.fn, Children: []core.Node{h.content}}}
	if h.thumb.set() {
		parts = append(parts, core.View{Style: over, Children: []core.Node{
			core.View{Class: h.thumb.class, Style: thumb.Merge(h.thumb.style), StyleFunc: h.thumb.fn}}})
	}
	set := func(next ResizeHandleState) {
		if h.state.GetUntracked() != next {
			h.state.Set(next)
		}
	}
	off := func() bool { return h.disabled != nil && h.disabled() }
	pan := func(e core.PanEvent) {
		switch e.Phase {
		case core.GestureBegan:
			if !off() {
				set(HandleDragging)
			}
		case core.GestureEnded, core.GestureCancelled:
			set(HandleIdle)
		}
		h.onPan(e)
	}
	return core.View{
		Style: band, Disabled: h.disabled, HitSlop: slop,
		OnPan: pan, PanActivation: activation,
		OnHoverIn: func() {
			if h.state.GetUntracked() == HandleIdle {
				set(HandleHovered)
			}
		},
		OnHoverOut: func() {
			if h.state.GetUntracked() == HandleHovered {
				set(HandleIdle)
			}
		},
		// A pointer or finger going down presses the handle until it lifts
		// or a drag takes over, which ends with the pan.
		OnTouch: func(e core.TouchEvent) {
			switch e.Phase {
			case core.TouchStart:
				if !off() && h.state.GetUntracked() != HandleDragging {
					set(HandlePressed)
				}
			case core.TouchEnd, core.TouchCancel:
				if h.state.GetUntracked() == HandlePressed {
					set(HandleIdle)
				}
			}
		},
		Role: core.RoleAdjustable, Label: h.label, TestID: h.testID,
		AccessibilityActions: handleActions, OnAccessibilityAction: h.onStep, AccessibilityValue: h.value,
		Children: parts,
	}
}

// handleActions are what assistive technology moves a handle with.
var handleActions = []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}}

// ResizeHandle is a handle's band on its own, on a container's edge
// (Edge) or straddling a boundary: it reports a drag, and the app
// resizes what it edges. Screen readers' steps report as a drag of 10
// points.
type ResizeHandle struct {
	// Axis is the axis it moves along.
	Axis Axis
	Edge HandleEdge
	// OnDragStart, OnDragMove and OnDragEnd follow a drag, with its
	// offset along Axis since it started.
	OnDragStart func()
	OnDragMove  func(offset core.Points)
	OnDragEnd   func(offset core.Points)
	Disabled    func() bool
	// OnStateChange follows the pointer's progression with the handle.
	OnStateChange func(ResizeHandleState)
	// Value is what assistive technology says the handle is at.
	Value          func() core.AccessibilityValue
	Class          string // the line's
	Style          core.Style
	StyleFunc      func() core.Style
	ThumbClass     string
	ThumbStyle     core.Style
	ThumbStyleFunc func() core.Style
	Label          string
	// Children are the app's, inside the line.
	Children []core.Node
	TestID   string
}

// Render implements core.Node.
func (r ResizeHandle) Render(cx *core.Scope) core.Node {
	horizontal := r.Axis != Vertical
	state := core.NewSignal(cx, HandleIdle)
	if f := r.OnStateChange; f != nil {
		reported := HandleIdle
		cx.Effect(func() {
			if now := state.Get(); now != reported {
				reported = now
				cx.Untrack(func() { f(now) })
			}
		})
	}
	offset := func(e core.PanEvent) core.Points {
		if !horizontal {
			return e.Translation.Y
		}
		if cx.Env().RTL() {
			return -e.Translation.X
		}
		return e.Translation.X
	}
	off := func() bool { return r.Disabled != nil && r.Disabled() }
	pan := func(e core.PanEvent) {
		if off() {
			return
		}
		switch e.Phase {
		case core.GestureBegan:
			if r.OnDragStart != nil {
				r.OnDragStart()
			}
		case core.GestureEnded, core.GestureCancelled:
			if r.OnDragEnd != nil {
				r.OnDragEnd(offset(e))
			}
			return
		}
		if r.OnDragMove != nil {
			r.OnDragMove(offset(e))
		}
	}
	step := func(name string) {
		if off() {
			return
		}
		d := handleStep
		switch name {
		case core.ActionIncrement:
		case core.ActionDecrement:
			d = -d
		default:
			return
		}
		if r.OnDragStart != nil {
			r.OnDragStart()
		}
		if r.OnDragMove != nil {
			r.OnDragMove(d)
		}
		if r.OnDragEnd != nil {
			r.OnDragEnd(d)
		}
	}
	return handleBandNode(handleBandSpec{horizontal: horizontal, edge: r.Edge, line: part{r.Class, r.Style, r.StyleFunc},
		thumb: part{r.ThumbClass, r.ThumbStyle, r.ThumbStyleFunc}, label: r.Label, testID: r.TestID, disabled: r.Disabled, onPan: pan, onStep: step,
		value: r.Value, touch: !core.Untracked(cx, cx.Env().Desktop), state: state, content: r.content()})
}

func (r ResizeHandle) content() core.Node {
	if len(r.Children) == 0 {
		return nil
	}
	return core.Group(r.Children)
}

// resizeState is gpui-base's sizing as data: the visible panels' sizes,
// ranges and fixed flags in a container, which a drag resizes.
type resizeState struct {
	sizes     []core.Points
	ranges    [][2]core.Points
	fixed     []bool
	container core.Points
}

func (s *resizeState) clone() *resizeState {
	return &resizeState{sizes: slices.Clone(s.sizes), ranges: s.ranges, fixed: s.fixed, container: s.container}
}

// resizeAtHandle gives the panel before handle ix size: growing takes
// from the panels after it, nearest first, as far as their minimums let
// it, passing over fixed ones but the next; shrinking below its minimum
// takes from the panels before it, and the panel after the handle gets
// what it gave. The total never exceeds the container.
func (s *resizeState) resizeAtHandle(ix int, size core.Points) {
	old := s.sizes
	if len(old) == 0 || ix >= len(old)-1 {
		return
	}
	moved := size - old[ix]
	if moved == 0 {
		return
	}
	lo, hi := s.ranges[ix][0], s.ranges[ix][1]
	next := min(max(size, lo), hi)
	main := ix
	sizes := slices.Clone(old)
	if moved > 0 {
		changed := next - old[ix]
		sizes[ix] = next
		for changed > 0 && ix < len(old)-1 {
			ix++
			if s.fixed[ix] && ix != main+1 {
				continue
			}
			take := min(changed, max(sizes[ix]-s.ranges[ix][0], 0))
			sizes[ix] -= take
			changed -= take
		}
	} else {
		changed := next - size
		sizes[ix] = next
		for changed > 0 && ix > 0 {
			ix--
			if s.fixed[ix] {
				continue
			}
			take := min(changed, max(sizes[ix]-s.ranges[ix][0], 0))
			changed -= take
			sizes[ix] -= take
		}
		sizes[main+1] += old[main] - size - changed
	}
	var total core.Points
	for _, x := range sizes {
		total += x
	}
	if total > s.container {
		sizes[main] = max(sizes[main]-(total-s.container), lo)
	}
	s.sizes = sizes
}
