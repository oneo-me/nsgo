package dock

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

func TestDockTreesNormalizeToAFixpoint(t *testing.T) {
	var next uint64
	l := installDock(Layout{Center: Split(base.Horizontal,
		Tabs(),
		Split(base.Vertical, Tabs("a")),
		Split(base.Horizontal, Tabs("b"), Tabs("c")),
	)}, &next)
	c := l.Center
	if c.Tabs || len(c.Children) != 3 || !c.Children[0].Tabs || c.Children[0].Panels[0] != "a" {
		t.Fatalf("center = %+v, want the empty group gone, the one-child split replaced and the same-axis split spliced", c)
	}
	if got := c.panels(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("panels %v", got)
	}
	again := l.Clone()
	if again.Center.normalize(true) || !reflect.DeepEqual(again, l) {
		t.Error("normalizing a normalized tree changed it")
	}
	// A slot's size is shared out to the children spliced into it.
	n := Node{ID: 1, Axis: base.Horizontal, Sizes: []core.Points{300, 0}, Children: []Node{
		{ID: 2, Axis: base.Horizontal, Sizes: []core.Points{100, 200}, Children: []Node{Tabs("a"), Tabs("b")}},
		Tabs("c"),
	}}
	n.normalize(true)
	if !slices.Equal(n.Sizes, []core.Points{100, 200, 0}) {
		t.Errorf("sizes %v", n.Sizes)
	}
}

func TestDockInstallRepairsWhatTheAppWrote(t *testing.T) {
	var next uint64 = 0
	l := installDock(Layout{
		Center: Node{ID: 7, Axis: base.Horizontal, Sizes: []core.Points{240, 1, 2}, Children: []Node{
			{ID: 7, Tabs: true, Panels: []string{"a", "b", "a"}, Active: 5},
			Tabs("b", "c"),
		}},
		Left: &Region{Root: Tabs("files")},
	}, &next)
	ids := map[uint64]bool{}
	for _, p := range regions {
		if t := l.tree(p); t != nil {
			t.walk(func(c *Node) {
				if c.ID == 0 || ids[c.ID] {
					panic("an ID left out or repeated")
				}
				ids[c.ID] = true
			})
		}
	}
	g := l.Center.Children[0]
	if !slices.Equal(g.Panels, []string{"a", "b"}) || g.Active != 1 || !slices.Equal(l.Center.Children[1].Panels, []string{"c"}) {
		t.Errorf("groups %+v %+v", g, l.Center.Children[1])
	}
	if len(l.Center.Sizes) != 2 || l.Center.Sizes[0] != 240 {
		t.Errorf("sizes %v, want fitted to two children", l.Center.Sizes)
	}
	// The center stays a split; a dock's root collapses.
	if l.Center.Tabs || !l.Left.Root.Tabs {
		t.Errorf("roots: center %+v, left %+v", l.Center, l.Left.Root)
	}
}

func TestDockPanelsMoveIntoGroupsAndBesideThem(t *testing.T) {
	var next uint64
	l := installDock(Layout{Center: Split(base.Horizontal, Tabs("a", "b"), Tabs("c"))}, &next)
	alloc := func() uint64 { next++; return next }
	left, right := l.Center.Children[0].ID, l.Center.Children[1].ID
	// Into the other group, displayed.
	c := &l.Center
	c.detach("b")
	c.insert("b", dockTarget{node: right, index: 0, activate: true}, alloc)
	c.normalize(true)
	if g := c.find(right); !slices.Equal(g.Panels, []string{"b", "c"}) || g.Active != 0 {
		t.Errorf("right group %+v", g)
	}
	// Below the right group: a vertical split wraps it, keeping its ID.
	c.detach("a")
	c.insert("a", dockTarget{node: right, split: true, side: core.PlaceBottom}, alloc)
	c.normalize(true)
	if c.find(left) != nil {
		t.Error("the emptied group stayed")
	}
	inner := c.Children[0]
	if inner.Tabs || inner.Axis != base.Vertical || inner.Children[0].ID != right || inner.Children[1].Panels[0] != "a" {
		t.Errorf("center %+v", *c)
	}
	// Beside the right group on the same axis as its parent: a sibling.
	c.insert("d", dockTarget{node: right, split: true, side: core.PlaceTop}, alloc)
	c.normalize(true)
	if inner := c.Children[0]; len(inner.Children) != 3 || inner.Children[0].Panels[0] != "d" {
		t.Errorf("after a split on the parent's axis: %+v", inner)
	}
}

func TestDockLayoutsRoundTripGpuiBaseFiles(t *testing.T) {
	file := `{"center": {"panel_name": "StackPanel", "children": [
		{"panel_name": "TabPanel", "children": [{"panel_name": "Alpha", "children": [], "info": {"panel": null}}], "info": {"tabs": {"active_index": 0}}},
		{"panel_name": "TabPanel", "children": [], "info": {"panel": null}},
		{"panel_name": "TabPanel", "children": [
			{"panel_name": "TabPanel", "children": [{"panel_name": "Panel", "children": [], "info": {"panel": {"id": "beta"}}}], "info": {"tabs": {"active_index": 0}}}
		], "info": {"tabs": {"active_index": 0}}}
	], "info": {"stack": {"sizes": [0.0, 120.0, 200.0], "axis": 1}}},
	"left_dock": {"panel": {"panel_name": "TabPanel", "children": [{"panel_name": "StoryContainer", "children": [], "info": {"panel": {"story_klass": "ListStory"}}}], "info": {"tabs": {"active_index": 0}}},
		"placement": "left", "size": 350.0, "open": false, "resizeable": true}}`
	var l Layout
	if err := json.Unmarshal([]byte(file), &l); err != nil {
		t.Fatal(err)
	}
	var next uint64
	l = installDock(l, &next)
	if got := l.Center.panels(); !slices.Equal(got, []string{"Alpha", "beta"}) {
		t.Errorf("center panels %v, want the leaves' ids or names, nested tabs flattened", got)
	}
	if l.Center.Axis != base.Vertical || !slices.Equal(l.Center.Sizes, []core.Points{0, 200}) {
		t.Errorf("center %+v, want a vertical split, the zero size none and the empty legacy group gone", l.Center)
	}
	if l.Left == nil || !l.Left.Closed || l.Left.Size != 350 || l.Left.Root.Panels[0] != "StoryContainer" {
		t.Errorf("left dock %+v", l.Left)
	}
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	var back AreaState
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.LeftDock == nil || back.LeftDock.Placement != "left" || back.Center.PanelName != "StackPanel" ||
		back.Center.Children[0].PanelName != "TabPanel" || PanelKeyOf(back.Center.Children[1].Children[0]) != "beta" {
		t.Errorf("written %s", data)
	}
	var again Layout
	if err := json.Unmarshal(data, &again); err != nil {
		t.Fatal(err)
	}
	again = installDock(again, &next)
	if !slices.Equal(again.Center.panels(), l.Center.panels()) || again.Left.Size != 350 {
		t.Errorf("round trip lost %s", data)
	}
}
