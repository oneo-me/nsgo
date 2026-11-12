package dock

import (
	"encoding/json"
	"errors"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// AreaState is gpui-base's persisted dock layout, its field names and
// tags frozen, so a layout file is the same in both. It is machine
// written: Layout's MarshalJSON and UnmarshalJSON go through it with
// each panel kept by its key.
type AreaState struct {
	Version    *int       `json:"version"`
	Center     PanelState `json:"center"`
	LeftDock   *State     `json:"left_dock,omitempty"`
	RightDock  *State     `json:"right_dock,omitempty"`
	BottomDock *State     `json:"bottom_dock,omitempty"`
}

// State is one dock's persisted state.
type State struct {
	Panel     PanelState  `json:"panel"`
	Placement string      `json:"placement"`
	Size      core.Points `json:"size"`
	Open      bool        `json:"open"`
}

// PanelState is a persisted container or panel: a split ("StackPanel"
// with stack info), a tab group ("TabPanel" with tabs info) or a panel's
// leaf, whose info is the app's.
type PanelState struct {
	PanelName string       `json:"panel_name"`
	Children  []PanelState `json:"children"`
	Info      PanelInfo    `json:"info"`
}

// PanelInfo is what a PanelState holds: a split's sizes and axis (0
// horizontal, 1 vertical), a tab group's active index, or a panel's own
// data, written as gpui-base's externally tagged enum.
type PanelInfo struct {
	Stack *StackInfo
	Tabs  *TabsInfo
	Panel json.RawMessage
}

// StackInfo is a split's persisted info.
type StackInfo struct {
	Sizes []core.Points `json:"sizes"`
	Axis  int           `json:"axis"`
}

// TabsInfo is a tab group's persisted info.
type TabsInfo struct {
	ActiveIndex int `json:"active_index"`
}

// MarshalJSON implements json.Marshaler.
func (i PanelInfo) MarshalJSON() ([]byte, error) {
	switch {
	case i.Stack != nil:
		return json.Marshal(map[string]*StackInfo{"stack": i.Stack})
	case i.Tabs != nil:
		return json.Marshal(map[string]*TabsInfo{"tabs": i.Tabs})
	}
	panel := i.Panel
	if panel == nil {
		panel = json.RawMessage("null")
	}
	return json.Marshal(map[string]json.RawMessage{"panel": panel})
}

// UnmarshalJSON implements json.Unmarshaler.
func (i *PanelInfo) UnmarshalJSON(data []byte) error {
	var tagged map[string]json.RawMessage
	if err := json.Unmarshal(data, &tagged); err != nil {
		return err
	}
	*i = PanelInfo{}
	switch {
	case tagged["stack"] != nil:
		i.Stack = new(StackInfo)
		return json.Unmarshal(tagged["stack"], i.Stack)
	case tagged["tabs"] != nil:
		i.Tabs = new(TabsInfo)
		return json.Unmarshal(tagged["tabs"], i.Tabs)
	case tagged["panel"] != nil:
		i.Panel = tagged["panel"]
		return nil
	}
	return errors.New("base: a panel's info is none of stack, tabs or panel")
}

// The names a persisted layout gives its containers, and the panels it
// keeps by key alone.
const (
	stackPanelName = "StackPanel"
	tabPanelName   = "TabPanel"
	// PanelKeyName is the panel_name of the leaves PanelKey writes.
	PanelKeyName = "Panel"
)

// PanelKey is a panel kept by its key alone: a leaf named PanelKeyName
// with info {"id": key}.
func PanelKey(key string) PanelState {
	info, _ := json.Marshal(map[string]string{"id": key})
	return PanelState{PanelName: PanelKeyName, Info: PanelInfo{Panel: info}}
}

// PanelKeyOf reads a leaf's key back: its info's "id", else its
// panel_name.
func PanelKeyOf(s PanelState) string {
	var info struct {
		ID *string `json:"id"`
	}
	if json.Unmarshal(s.Info.Panel, &info) == nil && info.ID != nil {
		return *info.ID
	}
	return s.PanelName
}

// NewAreaState writes l in gpui-base's format, each panel's leaf by
// leaf (PanelKey when nil).
func NewAreaState(l Layout, leaf func(key string) PanelState) AreaState {
	if leaf == nil {
		leaf = PanelKey
	}
	s := AreaState{Center: nodeState(&l.Center, leaf)}
	for _, p := range regions[1:] {
		r := l.Region(p)
		if r == nil {
			continue
		}
		size := r.Size
		if size <= 0 {
			size = dockDefaultSize
		}
		d := &State{Panel: nodeState(&r.Root, leaf), Placement: p.String(), Size: max(size, base.PanelMinSize), Open: !r.Closed}
		switch p {
		case Left:
			s.LeftDock = d
		case Right:
			s.RightDock = d
		default:
			s.BottomDock = d
		}
	}
	return s
}

func nodeState(n *Node, leaf func(string) PanelState) PanelState {
	if n.Tabs {
		s := PanelState{PanelName: tabPanelName, Children: []PanelState{}, Info: PanelInfo{Tabs: &TabsInfo{ActiveIndex: n.Active}}}
		for _, k := range n.Panels {
			s.Children = append(s.Children, leaf(k))
		}
		return s
	}
	axis := 0
	if n.Axis == base.Vertical {
		axis = 1
	}
	s := PanelState{PanelName: stackPanelName, Children: []PanelState{},
		Info: PanelInfo{Stack: &StackInfo{Sizes: fitSizes(n.Sizes, len(n.Children)), Axis: axis}}}
	for i := range n.Children {
		s.Children = append(s.Children, nodeState(&n.Children[i], leaf))
	}
	return s
}

// Layout reads s back, each leaf's key by key (PanelKeyOf when nil),
// with NSH's compatibility rules: a tab group of tab groups is
// flattened, a bare leaf where a container belongs is a tab group of it,
// and a "TabPanel" holding panel info is an empty tab group.
func (s AreaState) Layout(key func(PanelState) string) Layout {
	if key == nil {
		key = PanelKeyOf
	}
	l := Layout{Center: stateNode(s.Center, key)}
	for _, d := range []*State{s.LeftDock, s.RightDock, s.BottomDock} {
		if d == nil {
			continue
		}
		p := map[string]Placement{"left": Left, "right": Right, "bottom": Bottom}[d.Placement]
		if p == Center {
			continue
		}
		l.setRegion(p, &Region{Root: stateNode(d.Panel, key), Size: d.Size, Closed: !d.Open})
	}
	return l
}

func stateNode(s PanelState, key func(PanelState) string) Node {
	switch {
	case s.Info.Stack != nil:
		n := Node{Axis: base.Horizontal}
		if s.Info.Stack.Axis != 0 {
			n.Axis = base.Vertical
		}
		for i, c := range s.Children {
			n.Children = append(n.Children, stateNode(c, key))
			var size core.Points
			if i < len(s.Info.Stack.Sizes) && s.Info.Stack.Sizes[i] > 0 {
				size = s.Info.Stack.Sizes[i]
			}
			n.Sizes = append(n.Sizes, size)
		}
		return n
	case s.Info.Tabs != nil:
		return Node{Tabs: true, Panels: tabPanels(s.Children, key), Active: s.Info.Tabs.ActiveIndex}
	case s.PanelName == tabPanelName:
		return Node{Tabs: true}
	}
	return Node{Tabs: true, Panels: []string{key(s)}}
}

// tabPanels flattens one level of tab nesting, which gpui-base's old
// writer could produce.
func tabPanels(children []PanelState, key func(PanelState) string) []string {
	var out []string
	for _, c := range children {
		switch {
		case c.Info.Tabs != nil:
			out = append(out, tabPanels(c.Children, key)...)
		case c.Info.Panel != nil && c.PanelName == tabPanelName:
		default:
			out = append(out, key(c))
		}
	}
	return out
}

// MarshalJSON writes l in gpui-base's layout format, each panel as
// {"id": key}.
func (l Layout) MarshalJSON() ([]byte, error) { return json.Marshal(NewAreaState(l, nil)) }

// UnmarshalJSON reads a gpui-base layout file into l, each panel keyed
// by its "id", else its panel_name.
func (l *Layout) UnmarshalJSON(data []byte) error {
	var s AreaState
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*l = s.Layout(nil)
	return nil
}
