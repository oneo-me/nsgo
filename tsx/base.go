package tsx

import (
	"math"
	"reflect"
	"slices"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// nsgo/base: base's components as Go components, with the names NSH's
// nsh/base gives their props where base's differ; Input and Dialog are
// the runtime's, and the navigation, AlertDialog and ToastManager are
// written out (navigation.go, toasts.go). TextView, Editor and DiffView
// link packages of their own, so tsx/textview, tsx/editor and tsx/diff
// register them.

// enumOptions name the values of the enums base's and core's props take.
var enumOptions = []Option{
	Enum(map[string]core.Placement{"top": core.PlaceTop, "bottom": core.PlaceBottom, "left": core.PlaceLeft,
		"right": core.PlaceRight}),
	Enum(map[string]core.Align{"start": core.AlignStart, "center": core.AlignCenter, "end": core.AlignEnd}),
	Enum(map[string]base.Axis{"horizontal": base.Horizontal, "vertical": base.Vertical}),
	Enum(map[string]base.Anchor{"topLeft": base.AnchorTopLeft, "topCenter": base.AnchorTopCenter,
		"topRight": base.AnchorTopRight, "bottomLeft": base.AnchorBottomLeft,
		"bottomCenter": base.AnchorBottomCenter, "bottomRight": base.AnchorBottomRight,
		"leftCenter": base.AnchorLeftCenter, "rightCenter": base.AnchorRightCenter}),
	Enum(map[string]base.DataTableAlign{"start": base.AlignStartColumn, "center": base.AlignCenterColumn,
		"end": base.AlignEndColumn}),
	Enum(map[string]base.DockPlacement{"left": base.DockLeft, "right": base.DockRight, "bottom": base.DockBottom,
		"center": base.DockCenter}),
	Enum(map[string]base.HandleEdge{"centered": base.HandleCentered, "leading": base.HandleLeading,
		"trailing": base.HandleTrailing}),
	Enum(map[string]base.PlotCurve{"linear": base.CurveLinear, "natural": base.CurveNatural,
		"stepAfter": base.CurveStepAfter}),
	Enum(map[string]base.ResizeHandleState{"idle": base.HandleIdle, "hovered": base.HandleHovered,
		"pressed": base.HandlePressed, "dragging": base.HandleDragging}),
	Enum(map[string]base.SwipeDirection{"off": base.SwipeOff, "up": base.SwipeUp, "down": base.SwipeDown,
		"left": base.SwipeLeft, "right": base.SwipeRight}),
	Enum(map[string]base.ToastStatus{"starting": base.ToastStarting, "present": base.ToastPresent,
		"ending": base.ToastEnding}),
	Enum(map[string]base.TooltipPlacement{"top": base.TooltipTop, "pointer": base.TooltipPointer}),
	Enum(map[string]base.Reason{"cancel": base.ReasonCancel, "backdropPress": base.ReasonBackdropPress,
		"swipe": base.ReasonSwipe, "pressOutside": base.ReasonPressOutside, "triggerPress": base.ReasonTriggerPress,
		"hover": base.ReasonHover, "select": base.ReasonSelect, "contextMenu": base.ReasonContextMenu,
		"longPress": base.ReasonLongPress}),
	Enum(map[string]base.AlertPresentation{"alert": base.AlertPresentAlert,
		"actionSheet": base.AlertPresentActionSheet}),
}

func init() {
	// Configs an app gives in part: what it leaves out is the default.
	Optional[core.TimingConfig]("Duration", "Easing", "Delay")
	Optional[core.SpringConfig]("Stiffness", "Damping", "Mass", "Velocity")
	Optional[core.AccessibilityValue]("Min", "Max", "Now")
	Optional[base.DataTableColumn]("Width", "MinWidth", "MaxWidth", "FitPadding")
	Fields[base.DataTableColumn](map[string]string{"NoResize": "resizable", "NoMove": "movable", "NoSelect": "selectable"})
	// NSH's sort names its direction.
	Shape("DataTableSort", `export type DataTableSortDirection = "ascending" | "descending";
export interface DataTableSort { key: string; direction?: DataTableSortDirection }`,
		func(s base.DataTableSort) map[string]any {
			if s.Key == "" {
				return nil
			}
			return map[string]any{"key": s.Key, "direction": map[bool]string{false: "ascending", true: "descending"}[s.Descending]}
		},
		func(m map[string]any) base.DataTableSort {
			key, _ := m["key"].(string)
			return base.DataTableSort{Key: key, Descending: m["direction"] == "descending"}
		})
	registerPart("nsgo/base", "ResizablePanel", base.ResizablePanel{}, enumOptions...)
	Register("nsgo/base", map[string]core.Node{
		"Button":              base.Button{},
		"Checkbox":            base.Checkbox{},
		"Collapsible":         base.Collapsible{},
		"ContextMenu":         base.ContextMenu{},
		"DataTable":           base.DataTable{},
		"Menu":                base.Menu{},
		"MenuItem":            base.MenuItem{},
		"MenuSeparator":       base.MenuSeparator{},
		"Popover":             base.Popover{},
		"Progress":            base.Progress{},
		"Radio":               base.Radio{},
		"RadioGroup":          base.RadioGroup{},
		"ResizablePanelGroup": base.ResizablePanelGroup{},
		"Sheet":               base.Sheet{},
		"Submenu":             base.Submenu{},
		"Switch":              base.Switch{},
		"Tab":                 base.Tab{},
		"Tabs":                base.Tabs{},
		"Table":               base.Table{},
		"TableHeader":         base.TableHeader{},
		"TableBody":           base.TableBody{},
		"TableFooter":         base.TableFooter{},
		"TableRow":            base.TableRow{},
		"TableHead":           base.TableHead{},
		"TableCell":           base.TableCell{},
		"TableCaption":        base.TableCaption{},
		"Toast":               base.Toast{},
		"Tooltip":             base.Tooltip{},
	}, append(enumOptions, baseRenames...)...)
	Register("nsgo/base", map[string]core.Node{"DockArea": base.DockArea{}}, slices.Concat(enumOptions, baseRenames, dockOptions)...)
	Register("nsgo/base/plot", map[string]core.Node{
		"Plot": base.Plot{},
		"Line": base.PlotLine{},
		"Area": base.PlotArea{},
		"Grid": base.PlotGrid{},
	}, append(append(enumOptions, baseRenames...), Rename("Tooltip", "tooltipContent"))...)
}

// baseRenames are the props whose NSH names base's fields do not give,
// and the props whose shape is not their field's.
var baseRenames = []Option{
	Rename("NoRowSelect", "rowSelectable"), Rename("NoColumnSelect", "columnSelectable"),
	Rename("CellSelect", "cellSelectable"), Rename("NoResizeColumns", "resizableColumns"),
	Rename("NoMoveColumns", "movableColumns"), Rename("NoBounce", "bounces"), Rename("NoHover", "interactive"),
	Rename("Status", "transitionStatus"),
	custom("Text", tableText, "data?: readonly unknown[]", "dataStart?: number"),
	custom("RowCount", tableRowCount, "rowCount?: number"),
	custom("RowClass", tableRowClass, "rowClasses?: readonly string[]"),
	custom("OnDoublePress", func(p *props, v reflect.Value) {
		if h := p.handler("onDoublePress"); h != nil {
			v.Set(reflect.ValueOf(func(row, column int) { h(p.r.target(row, column)) }))
		}
	}, "onDoublePress?: (target: { row: number; column: number }) => void"),
	custom("OnVisibleRowsChange", rangeHandler("onVisibleRowsChange"),
		"onVisibleRowsChange?: (range: { start: number; end: number }) => void"),
	custom("OnVisibleColumnsChange", rangeHandler("onVisibleColumnsChange"),
		"onVisibleColumnsChange?: (range: { start: number; end: number }) => void"),
	custom("Y", plotValues("y"), "data?: readonly any[]", plotAxis("y")),
	custom("X", plotValues("x"), "data?: readonly any[]", plotAxis("x")),
	custom("Y1", plotValues("y"), "data?: readonly any[]", plotAxis("y")),
	custom("Y0", func(p *props, v reflect.Value) {
		if p.has("y0") {
			v.Set(reflect.ValueOf(func() float64 { return p.float("y0") }))
		}
	}, "y0?: number"),
	custom("XScale", plotScale("xScale"), "xScale?: "+scaleTS),
	custom("YScale", plotScale("yScale"), "yScale?: "+scaleTS),
}

const scaleTS = `{ type: "point" } | { type: "linear"; domain?: [number, number] }`

// plotAxis declares a plot's axis: a row's key, a function of a row and
// its index, or the values.
func plotAxis(axis string) string {
	return axis + "?: string | ((row: any, index: number) => number) | readonly number[]"
}

// target is { row, column }.
func (r *runtime) target(row, column int) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("row", row)
	_ = o.Set("column", column)
	return o
}

func rangeHandler(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if h := p.handler(prop); h != nil {
			v.Set(reflect.ValueOf(func(start, end int) {
				o := p.r.vm.NewObject()
				_ = o.Set("start", start)
				_ = o.Set("end", end)
				h(o)
			}))
		}
	}
}

// tableRow is row of a DataTable's data, whose first row is dataStart.
func tableRow(p *props, row int) goja.Value {
	items := p.r.arrayItems(p.get("data"))
	if i := row - p.int("dataStart"); i >= 0 && i < len(items) {
		return items[i]
	}
	return nil
}

// tableText is a cell's text: its row's value under its column's key.
func tableText(p *props, v reflect.Value) {
	if !p.has("data") {
		return
	}
	v.Set(reflect.ValueOf(func(row, column int) string {
		cols := p.r.arrayItems(p.get("columns"))
		if column < 0 || column >= len(cols) {
			return ""
		}
		key := stringOf(p.r.options(cols[column]).get("key"))
		o, ok := tableRow(p, row).(*goja.Object)
		if !ok {
			return ""
		}
		return stringOf(o.Get(key))
	}))
}

func tableRowCount(p *props, v reflect.Value) {
	v.Set(reflect.ValueOf(func() int {
		if p.has("rowCount") {
			return p.int("rowCount")
		}
		return p.int("dataStart") + len(p.r.arrayItems(p.get("data")))
	}))
}

func tableRowClass(p *props, v reflect.Value) {
	if !p.has("rowClasses") {
		return
	}
	v.Set(reflect.ValueOf(func(row int) string {
		items := p.r.arrayItems(p.get("rowClasses"))
		if row >= 0 && row < len(items) {
			return stringOf(items[row])
		}
		return ""
	}))
}

// plotValues reads a plot's or a mark's values along axis: its prop, a
// row's key or a function of a row and its index, over its rows or the
// plot's.
func plotValues(axis string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		r := p.r
		owner := p
		if !p.has("data") {
			for n := p.n.parent; n != nil; n = n.parent {
				if n.kind == kindFace && n.name == "Plot" {
					owner = n.props
					break
				}
			}
		}
		accessor := func() goja.Value {
			if x := p.get(axis); defined(x) {
				return x
			}
			return owner.get(axis)
		}
		if axis == "x" && !defined(core.Untracked(r.cx, accessor)) {
			return
		}
		read := func() []float64 {
			by := accessor()
			if items := r.arrayItems(by); items != nil {
				out := make([]float64, len(items))
				for i, it := range items {
					out[i] = it.ToFloat()
				}
				return out
			}
			rows := r.arrayItems(owner.get("data"))
			out := make([]float64, len(rows))
			for i, row := range rows {
				var x goja.Value
				if isFunction(by) {
					x = r.call(by, row, r.vm.ToValue(i))
				} else if o, ok := row.(*goja.Object); ok {
					x = o.Get(by.String())
				}
				out[i] = math.NaN()
				if defined(x) {
					out[i] = x.ToFloat()
				}
			}
			return out
		}
		v.Set(reflect.ValueOf(read))
	}
}

// plotScale reads { type: "point" } or { type: "linear", domain }.
func plotScale(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if !p.has(prop) {
			return
		}
		v.Set(reflect.ValueOf(func() base.PlotScale {
			o := p.r.options(p.get(prop))
			if o.str("type", "") == "point" {
				return base.PointScale()
			}
			var domain []float64
			for _, d := range p.r.arrayItems(o.get("domain")) {
				domain = append(domain, d.ToFloat())
			}
			return base.LinearScale(domain...)
		}))
	}
}
