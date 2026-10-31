package base_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

func TestResizablePanelsShareTheGroupAndResizeByDragsAndSteps(t *testing.T) {
	var reports [][]core.Points
	var first []core.Points
	var sidebar *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		sidebar = core.NewSignal(cx, true)
		return base.ResizablePanelGroup{TestID: "group", HandleLabel: "Resize", Style: core.Style{Width: core.Pt(600), Height: core.Pt(100)},
			OnResize:    func(sizes []core.Points) { reports = append(reports, sizes) },
			HandleStyle: core.Style{Background: core.RGB(0x888888)},
			Panels: []base.ResizablePanel{
				{TestID: "a", Size: func() core.Points { return 200 }, MinSize: 150, MaxSize: 300,
					OnResize: func(size core.Points) { first = append(first, size) }},
				{TestID: "b"},
				{TestID: "c", Size: func() core.Points { return 120 }, Visible: sidebar.Get},
			}}
	}, nsgotest.Size(700, 400))
	width := func(id string) core.Points { return app.Find("id:" + id).Frame.Size.Width }
	if a, b, c := width("a"), width("b"), width("c"); a != 200 || b != 280 || c != 120 {
		t.Fatalf("panels %v %v %v, want 200, the rest and 120", a, b, c)
	}
	handle := app.Find("id:group-handle-0")
	if handle.Props.Role != core.RoleAdjustable || handle.Props.Label != "Resize" || handle.Props.Value.Now != 200 {
		t.Errorf("handle props %+v", handle.Props)
	}
	if line := handle.Children[0]; line.Frame.Size.Width != 1 || line.Frame.Size.Height != 100 {
		t.Errorf("the line is %v", line.Frame.Size)
	}
	// The band lies over the boundary within the group, where platforms
	// give it its cursor and track the pointer, and takes no room.
	if f := handle.Frame; f.Origin.X != 196 || f.Size.Width != 9 || handle.Parent.Props.TestID != "group" {
		t.Errorf("the band is at %v in %q, want 9 points over the boundary at 200, in the group", f, handle.Parent.Props.TestID)
	}
	if handle.Style.Cursor != core.CursorColResize {
		t.Errorf("the band's cursor is %v, want the column resize cursor", handle.Style.Cursor)
	}
	if x := handle.Children[0].Frame.Origin.X + handle.Frame.Origin.X; x != 200 {
		t.Errorf("the line is at %v, want on the boundary", x)
	}
	app.Pan("id:group-handle-0", core.Point{X: 30}, core.Point{X: 60})
	if a, b := width("a"), width("b"); a != 260 || b != 220 {
		t.Errorf("after a drag of 60: %v %v", a, b)
	}
	app.Pan("id:group-handle-0", core.Point{X: 200})
	if a := width("a"); a != 300 {
		t.Errorf("past its maximum: %v", a)
	}
	if len(reports) != 2 || !slices.Equal(reports[0], []core.Points{260, 220, 120}) || !slices.Equal(first, []core.Points{260, 300}) {
		t.Errorf("reports %v, first %v", reports, first)
	}
	app.AccessibilityAction("id:group-handle-0", core.ActionDecrement)
	if a := width("a"); a != 290 {
		t.Errorf("a step back: %v", a)
	}
	sidebar.Set(false)
	app.Settle()
	if app.Exists("id:group-handle-1") || width("b") != 310 {
		t.Errorf("hiding the last panel: handle 1 %v, b %v", app.Exists("id:group-handle-1"), width("b"))
	}
	sidebar.Set(true)
	app.Settle()
	if c := width("c"); c != 120 {
		t.Errorf("shown again at %v, want the size it wanted", c)
	}
}

func TestResizeHandleReportsItsDrag(t *testing.T) {
	var log []core.Points
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(200), Height: core.Pt(100)}, Children: []core.Node{
			base.ResizeHandle{TestID: "edge", Edge: base.HandleTrailing,
				OnDragMove: func(d core.Points) { log = append(log, d) },
				OnDragEnd:  func(d core.Points) { log = append(log, -d) }},
		}}
	})
	if f := app.Find("id:edge"); f.Frame.Origin.X != 191 || f.Frame.Size.Width != 9 || f.Children[0].Frame.Origin.X != 8 {
		t.Errorf("the band at %v, its line at %v, want hugging the trailing edge from inside, the line on it",
			f.Frame, f.Children[0].Frame)
	}
	app.Pan("id:edge", core.Point{X: -10}, core.Point{X: -25})
	if !slices.Equal(log, []core.Points{-10, -25, 25}) {
		t.Errorf("log %v", log)
	}
}

func TestResizablePanelsFollowTheDragInRightToLeft(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.ResizablePanelGroup{TestID: "group", Style: core.Style{Width: core.Pt(600), Height: core.Pt(100)},
			Panels: []base.ResizablePanel{{TestID: "a", Size: func() core.Points { return 200 }}, {TestID: "b"}}}
	})
	app.SetEnvironment(core.Environment{RTL: true})
	if a := app.Find("id:a").Frame; a.Origin.X != 400 {
		t.Fatalf("the first panel is at %v, want on the right in right-to-left", a)
	}
	app.Pan("id:group-handle-0", core.Point{X: -50})
	if w := app.Find("id:a").Frame.Size.Width; w != 250 {
		t.Errorf("first panel %v wide, want a drag toward it from the right to grow it", w)
	}
}

func TestVerticalPanelsHaveRowResizeHandles(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.ResizablePanelGroup{TestID: "group", Axis: base.Vertical, Style: core.Style{Width: core.Pt(300), Height: core.Pt(400)},
			Panels: []base.ResizablePanel{{TestID: "top"}, {TestID: "bottom"}}}
	})
	h := app.Find("id:group-handle-0")
	if h.Style.Cursor != core.CursorRowResize || h.Frame.Origin.Y != 196 || h.Frame.Size.Height != 9 {
		t.Errorf("handle %v with cursor %v", h.Frame, h.Style.Cursor)
	}
	app.Pan("id:group-handle-0", core.Point{Y: 40})
	if a, b := app.Find("id:top").Frame.Size.Height, app.Find("id:bottom").Frame.Size.Height; a != 240 || b != 160 {
		t.Errorf("after a drag of 40: %v and %v", a, b)
	}
}

func TestHandleThumbsCenterOnTheLineAndKeepTheirTransform(t *testing.T) {
	thumb := core.Style{Width: core.Pt(3), Height: core.Pt(44), Transform: core.Transform{ScaleY: 0.5}}
	for _, axis := range []base.Axis{base.Horizontal, base.Vertical} {
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			return base.ResizablePanelGroup{TestID: "group", Axis: axis, HandleThumbStyle: thumb,
				Style:  core.Style{Width: core.Pt(300), Height: core.Pt(400)},
				Panels: []base.ResizablePanel{{TestID: "a"}, {TestID: "b"}}}
		})
		h := app.Find("id:group-handle-0")
		line, over := h.Children[0].Frame, h.Children[1]
		turn := float32(0)
		if axis == base.Vertical {
			turn = 90
		}
		mid := core.Point{X: line.Origin.X + line.Size.Width/2, Y: line.Origin.Y + line.Size.Height/2}
		at := core.Point{X: over.Frame.Origin.X + 0.5, Y: over.Frame.Origin.Y + 0.5}
		across, along := at.X-mid.X, at.Y-mid.Y
		if axis == base.Vertical {
			across, along = along, across
		}
		if across != 0 || along < -0.5 || along > 0.5 || over.Style.Transform.Rotate != turn {
			t.Errorf("axis %v: the thumb's point at %v turned %v, want the line's middle %v turned %v",
				axis, at, over.Style.Transform.Rotate, mid, turn)
		}
		// Upright, before any turn, the thumb straddles the point exactly.
		f, s := over.Children[0].Frame, over.Children[0].Style
		if f.Origin.X+f.Size.Width/2 != 0.5 || s.Transform.ScaleY != 0.5 {
			t.Errorf("axis %v: the thumb at %v with %+v, want centered on the line keeping the app's transform", axis, f, s.Transform)
		}
	}
}

func TestRenderHandleFollowsTheHandlesState(t *testing.T) {
	var drawn []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.ResizablePanelGroup{TestID: "group", Style: core.Style{Width: core.Pt(600), Height: core.Pt(100)},
			RenderHandle: func(cx *core.Scope, h base.ResizeHandleContext) core.Node {
				drawn = append(drawn, fmt.Sprint(h.Index, " ", h.State))
				return core.View{TestID: fmt.Sprint("grip-", h.Index, "-", h.State), Style: core.Style{Width: core.Pt(1), Height: core.Pt(8)}}
			},
			Panels: []base.ResizablePanel{{TestID: "a"}, {TestID: "b"}, {TestID: "c"}}}
	}, nsgotest.Size(700, 400))
	if !app.Exists("id:grip-0-idle") || !app.Exists("id:grip-1-idle") {
		t.Fatalf("drawn %v, want each handle's content in its line", drawn)
	}
	if grip := app.Find("id:grip-1-idle"); grip.Parent != app.Find("id:group-handle-1").Children[0] {
		t.Errorf("the content is in %+v, want the handle's line", grip.Parent.Props)
	}
	app.Hover("id:group-handle-0")
	app.HoverOut()
	drawn = nil
	app.Drag("id:group-handle-0", core.Point{X: 20}, core.Point{X: 40}, core.Point{X: 60})
	if !slices.Equal(drawn, []string{"0 pressed", "0 dragging", "0 idle"}) {
		t.Errorf("a drag drew %v, want once per state, never per move", drawn)
	}
	if w := app.Find("id:a").Frame.Size.Width; w != 260 {
		t.Errorf("the first panel is %v wide after the drag, want 260", w)
	}
}

func TestResizeHandleReportsItsState(t *testing.T) {
	var states []base.ResizeHandleState
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(200), Height: core.Pt(100)}, Children: []core.Node{
			base.ResizeHandle{TestID: "edge", Edge: base.HandleTrailing,
				OnStateChange: func(s base.ResizeHandleState) { states = append(states, s) }},
		}}
	})
	app.Hover("id:edge")
	app.HoverOut()
	if !slices.Equal(states, []base.ResizeHandleState{base.HandleHovered, base.HandleIdle}) {
		t.Errorf("states %v, want hovered then idle", states)
	}
}
