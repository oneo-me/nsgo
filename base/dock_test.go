package base_test

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// surface is v's frame in the surface.
func surface(v *nsgotest.View) core.Bounds {
	b := v.Frame
	for p := v.Parent; p != nil; p = p.Parent {
		b.Origin.X += p.Frame.Origin.X - p.Offset.X
		b.Origin.Y += p.Frame.Origin.Y - p.Offset.Y
	}
	return b
}

func center(b core.Bounds) core.Point {
	return core.Point{X: b.Origin.X + b.Size.Width/2, Y: b.Origin.Y + b.Size.Height/2}
}

func dockApp(t *testing.T, width core.Points, layouts *[]base.DockLayout, events *[]string) (*nsgotest.App, *base.DockAreaRef) {
	ref := new(base.DockAreaRef)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.DockArea{Ref: ref, TestID: "dock", Style: core.Style{Width: core.Pt(width), Height: core.Pt(600)},
			DefaultLayout: base.DockLayout{
				Center: base.DockSplit(base.Horizontal, base.DockTabs("editor", "notes"), base.DockTabs("preview")),
				Left:   &base.DockRegion{Root: base.DockTabs("files"), Size: 200},
			},
			Panels: map[string]base.DockPanelOptions{"notes": {Closable: true}},
			RenderPanel: func(cx *core.Scope, key string) core.Node {
				return core.View{TestID: "panel-" + key, Style: core.Style{FlexGrow: 1},
					Children: []core.Node{base.Input{TestID: "input-" + key}}}
			},
			OnLayoutChange:      func(l base.DockLayout) { *layouts = append(*layouts, l) },
			OnPanelActiveChange: func(k string, on bool) { *events = append(*events, k+map[bool]string{true: "+", false: "-"}[on]) },
			OnPanelRemove:       func(k string) { *events = append(*events, "remove "+k) },
			OnDockOpenChange: func(p base.DockPlacement, open bool) {
				*events = append(*events, p.String()+map[bool]string{true: " open", false: " closed"}[open])
			},
		}
	}, nsgotest.Size(width, 600), nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop}))
	return app, ref
}

func TestDockAreaShowsItsRegionsAndRearrangesThem(t *testing.T) {
	var layouts []base.DockLayout
	var events []string
	app, ref := dockApp(t, 1000, &layouts, &events)
	if w := app.Find("id:panel-files").Parent.Parent.Frame.Size.Width; w != 200 {
		t.Errorf("left dock content %v wide", w)
	}
	if app.Exists("id:panel-notes") || !app.Exists("id:panel-editor") || !app.Exists("id:panel-preview") {
		t.Fatal("each group shows its displayed panel only")
	}
	app.Type("id:input-editor", "kept")
	app.Press("label:notes")
	if !app.Exists("id:panel-notes") || len(layouts) != 1 || layouts[0].Center.Children[0].Active != 1 {
		t.Fatalf("selecting a tab: layouts %d", len(layouts))
	}
	if !slices.Equal(events, []string{"editor-", "notes+"}) {
		t.Errorf("events %q", events)
	}
	app.Press("label:editor")
	if got := app.Find("id:input-editor").Text; got != "kept" {
		t.Errorf("the editor has %q after switching tabs, want its state kept", got)
	}

	// A drag into the middle of the other group's content merges.
	from := center(surface(app.Find("label:editor")))
	to := center(surface(app.Find("id:panel-preview")))
	app.Drag("label:editor", core.Point{X: 10}, core.Point{X: to.X - from.X, Y: to.Y - from.Y})
	last := layouts[len(layouts)-1]
	if g := last.Center.Children[1]; !slices.Equal(g.Panels, []string{"preview", "editor"}) || g.Active != 1 {
		t.Fatalf("after a merge: %+v", last.Center)
	}
	// A drag to the bottom of the left dock's content splits it.
	from = center(surface(app.Find("label:notes")))
	files := surface(app.Find("id:panel-files"))
	to = core.Point{X: files.Origin.X + files.Size.Width/2, Y: files.Origin.Y + files.Size.Height - 10}
	app.Drag("label:notes", core.Point{X: 10}, core.Point{X: to.X - from.X, Y: to.Y - from.Y})
	last = layouts[len(layouts)-1]
	if last.Left.Root.Tabs || last.Left.Root.Axis != base.Vertical || last.Left.Root.Children[1].Panels[0] != "notes" {
		t.Fatalf("after a split in the dock: %+v", last.Left.Root)
	}

	ref.ToggleDock(base.DockLeft)
	app.Settle()
	if app.Exists("id:panel-files") || !layouts[len(layouts)-1].Left.Closed {
		t.Error("the closed dock still shows")
	}
	if events[len(events)-1] != "left closed" {
		t.Errorf("events %q", events)
	}
	ref.ToggleZoom(ref.GroupOf("preview"))
	app.Settle()
	if app.Exists("id:panel-preview") == app.Exists("id:panel-editor") || ref.ZoomedGroup() != ref.GroupOf("preview") {
		t.Error("zooming shows more than one group")
	}
	ref.ToggleZoom(0)
	app.Settle()
	ref.ClosePanel("editor") // not closable
	app.Settle()
	if !app.Exists("label:editor") {
		t.Error("a panel that is not closable closed")
	}
}

func TestDockAreaIsCompactWhenNarrow(t *testing.T) {
	var layouts []base.DockLayout
	var events []string
	app, ref := dockApp(t, 390, &layouts, &events)
	if app.Exists("id:panel-files") {
		t.Error("the dock shows at compact width")
	}
	if !slices.Equal(events, []string{"left closed"}) {
		t.Errorf("events %q, want the dock reported hidden", events)
	}
	if app.Exists("id:panel-preview") || !app.Exists("id:panel-editor") || !app.Exists("label:preview") {
		t.Fatal("the compact center does not show one group with every center panel's tab")
	}
	app.Press("label:preview")
	if !app.Exists("id:panel-preview") || app.Exists("id:panel-editor") {
		t.Error("selecting another group's panel did not make its group the current one")
	}
	ref.ToggleDock(base.DockLeft)
	app.Settle()
	app.Advance(300 * time.Millisecond)
	if !app.Exists("id:panel-files") {
		t.Error("the dock is not presented as a drawer")
	}
	for _, l := range layouts {
		if l.Left.Closed {
			t.Error("presenting a drawer changed the layout's open")
		}
	}
}

func TestDockPanelsKeepTheirStateAsTheyMove(t *testing.T) {
	var layouts []base.DockLayout
	var events []string
	app, ref := dockApp(t, 1000, &layouts, &events)
	app.Type("id:input-editor", "kept")
	editor := app.Find("id:panel-editor").ID
	kept := func(when string) {
		t.Helper()
		if v := app.Find("id:panel-editor"); v.ID != editor || app.Find("id:input-editor").Text != "kept" {
			t.Errorf("%s: the editor was made again (%v) or lost its text %q", when, v.ID != editor, app.Find("id:input-editor").Text)
		}
	}
	// Merged into the other group.
	from := center(surface(app.Find("label:editor")))
	to := center(surface(app.Find("id:panel-preview")))
	app.Drag("label:editor", core.Point{X: 10}, core.Point{X: to.X - from.X, Y: to.Y - from.Y})
	kept("merged into another group")
	// Split off into the left dock.
	from = center(surface(app.Find("label:editor")))
	files := surface(app.Find("id:panel-files"))
	to = core.Point{X: files.Origin.X + files.Size.Width/2, Y: files.Origin.Y + files.Size.Height - 10}
	app.Drag("label:editor", core.Point{X: 10}, core.Point{X: to.X - from.X, Y: to.Y - from.Y})
	if last := layouts[len(layouts)-1]; last.Left.Root.Tabs {
		t.Fatalf("the editor did not move into the dock: %+v", last.Left.Root)
	}
	kept("moved into a dock")
	ref.ToggleZoom(ref.GroupOf("editor"))
	app.Settle()
	ref.ToggleZoom(ref.GroupOf("editor"))
	app.Settle()
	kept("zoomed and back")

	// A closed panel goes for good.
	ref.ClosePanel("notes")
	app.Settle()
	ref.AddPanel("notes", base.DockCenter, 0)
	app.Settle()
	ref.SelectPanel("notes")
	app.Settle()
	if app.Find("id:input-notes").Text != "" {
		t.Error("a panel added again kept the state of the one closed")
	}
}

func TestDockAreaRefEditsTheLayout(t *testing.T) {
	var layouts []base.DockLayout
	var events []string
	app, ref := dockApp(t, 1000, &layouts, &events)
	app.Type("id:input-editor", "kept")
	left, right := ref.GroupOf("editor"), ref.GroupOf("preview")
	if left == 0 || right == 0 || left == right || ref.GroupOf("missing") != 0 {
		t.Fatalf("groups %v %v", left, right)
	}
	last := func() base.DockLayout { return layouts[len(layouts)-1] }
	ref.MovePanel("editor", base.DockTarget{Group: right, Before: "preview"})
	app.Settle()
	if g := last().Center.Children[1]; !slices.Equal(g.Panels, []string{"editor", "preview"}) || g.Active != 0 {
		t.Fatalf("moved before preview: %+v", last().Center)
	}
	if app.Find("id:input-editor").Text != "kept" {
		t.Error("a moved panel lost its state")
	}
	ref.MovePanel("notes", base.DockTarget{Group: right, NoActivate: true})
	app.Settle()
	if g := last().Center; len(g.Children) != 1 || !slices.Equal(g.Children[0].Panels, []string{"editor", "preview", "notes"}) || g.Children[0].Active != 0 {
		t.Fatalf("moved last without displaying it: %+v", g)
	}
	ref.SplitAt(right, "notes", core.PlaceBottom)
	app.Settle()
	if c := last().Center.Children[0]; c.Tabs || c.Axis != base.Vertical || c.Children[1].Panels[0] != "notes" {
		t.Fatalf("split below: %+v", c)
	}
	ref.MovePanel("files", base.DockTarget{Group: ref.GroupOf("notes"), Side: core.PlaceRight, Size: 120})
	app.Settle()
	if c := last().Center.Children[0].Children[1]; c.Tabs || c.Axis != base.Horizontal || c.Sizes[1] != 120 || last().Left != nil && len(last().Left.Root.Panels) != 0 {
		t.Fatalf("split right of notes at 120: %+v, left %+v", c, last().Left)
	}
	ref.SelectTab(right, 1)
	app.Settle()
	if !app.Exists("id:panel-preview") || app.Exists("id:panel-editor") {
		t.Error("selecting the second tab did not display preview")
	}
	ref.RemovePanel("editor") // not closable, removed all the same
	app.Settle()
	if ref.GroupOf("editor") != 0 || events[len(events)-1] != "remove editor" {
		t.Errorf("removing: events %q", events)
	}
	ref.AddPanel("terminal", base.DockBottom, 150)
	app.Settle()
	if b := last().Bottom; b == nil || b.Size != 150 || b.Root.Panels[0] != "terminal" {
		t.Fatalf("a new bottom dock: %+v", b)
	}
	ref.SetDockSize(base.DockBottom, 40)
	app.Settle()
	if b := last().Bottom; b.Size != base.PanelMinSize {
		t.Errorf("dock size %v, want at least the minimum", b.Size)
	}
	ref.AddPanel("preview", base.DockBottom, 0)
	app.Settle()
	if b := last().Bottom; !slices.Equal(b.Root.Panels, []string{"terminal", "preview"}) || ref.GroupOf("preview") != ref.GroupOf("terminal") {
		t.Errorf("a held panel added to a dock moves there: %+v", b.Root)
	}
}

func TestDockPanelsKeepTheirStateAcrossTheCompactPresentation(t *testing.T) {
	ref := new(base.DockAreaRef)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.DockArea{Ref: ref, TestID: "dock", Style: core.Style{FlexGrow: 1},
			DefaultLayout: base.DockLayout{
				Center: base.DockTabs("editor"),
				Left:   &base.DockRegion{Root: base.DockTabs("files"), Size: 200},
			},
			RenderPanel: func(cx *core.Scope, key string) core.Node {
				return core.View{TestID: "panel-" + key, Style: core.Style{FlexGrow: 1},
					Children: []core.Node{base.Input{TestID: "input-" + key}}}
			},
		}
	}, nsgotest.Size(1000, 600), nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop}))
	app.Type("id:input-files", "kept")
	files := app.Find("id:panel-files").ID
	kept := func(when string) {
		t.Helper()
		if v := app.Find("id:panel-files"); v.ID != files || app.Find("id:input-files").Text != "kept" {
			t.Errorf("%s: the dock's panel was made again (%v) or lost its text %q", when, v.ID != files, app.Find("id:input-files").Text)
		}
	}
	app.Resize(390, 600)
	app.Settle()
	if app.Exists("id:panel-files") {
		t.Fatal("the dock shows at compact width")
	}
	ref.ToggleDock(base.DockLeft)
	app.Settle()
	app.Advance(300 * time.Millisecond)
	kept("in the drawer")
	ref.ToggleDock(base.DockLeft)
	app.Settle()
	app.Advance(300 * time.Millisecond)
	ref.ToggleDock(base.DockLeft)
	app.Settle()
	app.Advance(300 * time.Millisecond)
	kept("in the drawer again")
	app.Resize(1000, 600)
	app.Settle()
	kept("back at full width")
}
