package tsx

import (
	"reflect"
	"slices"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/base/dock"
	"github.com/nsgo-dev/nsgo/core"
)

// DockArea's values in NSH's shapes: a layout's nodes tagged by type, a
// dock's open and collapsible in place of base's Closed and NoCollapse,
// tabs, docks and panels as plain values, and the panels' options a list
// by id.

// dockPanel is a panel as renderPanel gets it.
type dockPanel struct{ ID string }

// dockPanelOptions is one entry of panels: what panel id allows.
type dockPanelOptions struct {
	id string
	o  dock.PanelOptions
}

func init() {
	refDecls[reflect.TypeFor[*dock.AreaRef]()] = map[string]string{"ToggleZoom": "toggleZoom(group: number | null): void"}
	Shape("DockNode", `export type DockNode =
  | { type: "split"; id?: number; axis: "horizontal" | "vertical"; children: DockNode[]; sizes?: (number | null)[] }
  | { type: "tabs"; id?: number; panels: string[]; activeIndex?: number };`, encodeDockNode, decodeDockNode)
	Shape("DockRegion", `export interface DockRegion { layout: DockNode; size?: number; open?: boolean; collapsible?: boolean }`,
		func(r dock.Region) map[string]any {
			size := r.Size
			if size == 0 {
				size = 2 * base.PanelMinSize
			}
			return map[string]any{"layout": r.Root, "size": js32(size), "open": !r.Closed, "collapsible": !r.NoCollapse}
		},
		func(m map[string]any) dock.Region {
			r := dock.Region{Size: core.Points(jsNumber(m["size"])), Closed: m["open"] == false,
				NoCollapse: m["collapsible"] == false}
			if l, ok := m["layout"].(map[string]any); ok {
				r.Root = decodeDockNode(l)
			}
			return r
		})
	Shape("DockTab", `export interface DockTab { panel: string; group: number; index: number; active: boolean; closable: boolean; zoomed: boolean }`,
		func(t dock.Tab) map[string]any {
			return map[string]any{"panel": t.Key, "group": t.Group, "index": call(t.Index), "active": call(t.Selected),
				"closable": call(t.Closable), "zoomed": call(t.Zoomed)}
		}, nil)
	Shape("DockInfo", `export interface DockInfo { placement: DockPlacement; open: boolean; size: number; collapsible: boolean }`,
		func(d dock.Info) map[string]any {
			return map[string]any{"placement": d.Placement, "open": call(d.Open), "size": js32(call(d.Size)),
				"collapsible": call(d.Collapsible)}
		}, nil)
	Shape("DockDropIndicator", `export interface DockDropIndicator { group: number; side?: Placement; index?: number }`,
		func(d dock.DropIndicator) map[string]any {
			m := map[string]any{"group": d.Group}
			if d.Side != core.PlaceDefault {
				m["side"] = d.Side
			}
			if d.Index >= 0 {
				m["index"] = d.Index
			}
			return m
		}, nil)
	Shape("DockPanel", `export interface DockPanel { id: string }`,
		func(p dockPanel) map[string]any { return map[string]any{"id": p.ID} }, nil)
	Shape("DockPanelOptions", `export interface DockPanelOptions { id: string; closable?: boolean; zoomable?: boolean; visible?: boolean }`,
		func(p dockPanelOptions) map[string]any {
			return map[string]any{"id": p.id, "closable": !p.o.NoClose, "zoomable": !p.o.NoZoom, "visible": !p.o.Hidden}
		},
		func(m map[string]any) dockPanelOptions {
			id, _ := m["id"].(string)
			return dockPanelOptions{id, dock.PanelOptions{NoClose: m["closable"] == false, NoZoom: m["zoomable"] == false,
				Hidden: m["visible"] == false}}
		})
}

// dockOptions are DockArea's props in NSH's shapes.
var dockOptions = []Option{
	custom("Compact", func(p *props, v reflect.Value) {
		v.Set(reflect.ValueOf(func() bool { return p.bool("compact") }))
	}, "compact?: boolean"),
	Event("OnDockOpenChange", "placement", "open"), Event("OnLayoutChange", "layout"),
	Event("OnPanelActiveChange", "panel", "active"), Event("OnPanelRemove", "panel"),
	custom("OnZoomChange", func(p *props, v reflect.Value) {
		if h := p.handler("onZoomChange"); h != nil {
			v.Set(reflect.ValueOf(func(key string) {
				e := p.r.vm.NewObject()
				_ = e.Set("panel", js.Null())
				if key != "" {
					_ = e.Set("panel", key)
				}
				h(e)
			}))
		}
	}, "onZoomChange?: (e: { panel: string | null }) => void"),
	typed("Panels", custom("Panels", func(p *props, v reflect.Value) {
		x := p.get("panels")
		if !defined(x) {
			return
		}
		m := map[string]dock.PanelOptions{}
		for _, o := range p.r.decode(x, reflect.TypeFor[[]dockPanelOptions]()).Interface().([]dockPanelOptions) {
			m[o.id] = o.o
		}
		v.Set(reflect.ValueOf(m))
	}, "panels?: readonly DockPanelOptions[]"), reflect.TypeFor[dockPanelOptions]()),
	typed("RenderPanel", custom("RenderPanel", func(p *props, v reflect.Value) {
		f := reflect.New(reflect.TypeFor[func(*core.Scope, dockPanel) core.Node]()).Elem()
		renderSetter(f.Type(), "renderPanel")(p, f)
		if f.IsNil() {
			return
		}
		render := f.Interface().(func(*core.Scope, dockPanel) core.Node)
		v.Set(reflect.ValueOf(func(cx *core.Scope, key string) core.Node { return render(cx, dockPanel{key}) }))
	}, "renderPanel?: (panel: DockPanel) => JSX.Element"), reflect.TypeFor[dockPanel]()),
}

// typed declares the types field's custom declarations name.
// DockOptions are DockArea's props, for a package that registers a form
// of it.
func DockOptions() []Option { return append(slices.Clone(enumOptions), dockOptions...) }

func typed(field string, o Option, types ...reflect.Type) Option {
	return func(s *faceSpec) {
		o(s)
		s.types[field] = types
	}
}

func encodeDockNode(n dock.Node) map[string]any {
	m := map[string]any{}
	if n.ID != 0 {
		m["id"] = n.ID
	}
	if n.Tabs {
		m["type"], m["panels"], m["activeIndex"] = "tabs", append([]string{}, n.Panels...), n.Active
		return m
	}
	sizes := make([]any, len(n.Children))
	for i := range sizes {
		if i < len(n.Sizes) && n.Sizes[i] > 0 {
			sizes[i] = js32(n.Sizes[i])
		}
	}
	m["type"], m["axis"], m["children"], m["sizes"] = "split", n.Axis, append([]dock.Node{}, n.Children...), sizes
	return m
}

func decodeDockNode(m map[string]any) dock.Node {
	n := dock.Node{ID: uint64(jsNumber(m["id"]))}
	if m["type"] == "tabs" {
		n.Tabs, n.Active = true, int(jsNumber(m["activeIndex"]))
		for _, p := range list(m["panels"]) {
			if s, ok := p.(string); ok {
				n.Panels = append(n.Panels, s)
			}
		}
		return n
	}
	if m["axis"] == "vertical" {
		n.Axis = base.Vertical
	}
	for _, c := range list(m["children"]) {
		if c, ok := c.(map[string]any); ok {
			n.Children = append(n.Children, decodeDockNode(c))
		}
	}
	if sizes := list(m["sizes"]); len(sizes) > 0 {
		n.Sizes = make([]core.Points, len(n.Children))
		for i := range min(len(sizes), len(n.Sizes)) {
			n.Sizes[i] = core.Points(jsNumber(sizes[i]))
		}
	}
	return n
}

// call is what f returns, or its zero value without f.
func call[T any](f func() T) T {
	var zero T
	if f == nil {
		return zero
	}
	return f()
}

// jsNumber is an exported JS number.
func jsNumber(x any) float64 {
	switch v := x.(type) {
	case int64:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

// jsList is an exported JS array.
func list(x any) []any {
	l, _ := x.([]any)
	return l
}
