package tsx

import (
	"math"
	"reflect"
	"slices"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/datatable"
	"github.com/oneo-me/nsgo/base/dock"
	"github.com/oneo-me/nsgo/base/plot"
	"github.com/oneo-me/nsgo/base/toast"
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
	Enum(map[string]toast.Anchor{"topLeft": toast.AnchorTopLeft, "topCenter": toast.AnchorTopCenter,
		"topRight": toast.AnchorTopRight, "bottomLeft": toast.AnchorBottomLeft,
		"bottomCenter": toast.AnchorBottomCenter, "bottomRight": toast.AnchorBottomRight,
		"leftCenter": toast.AnchorLeftCenter, "rightCenter": toast.AnchorRightCenter}),
	Enum(map[string]datatable.Align{"start": datatable.AlignStartColumn, "center": datatable.AlignCenterColumn,
		"end": datatable.AlignEndColumn}),
	Enum(map[string]dock.Placement{"left": dock.Left, "right": dock.Right, "bottom": dock.Bottom,
		"center": dock.Center}),
	Enum(map[string]base.HandleEdge{"centered": base.HandleCentered, "leading": base.HandleLeading,
		"trailing": base.HandleTrailing}),
	Enum(map[string]plot.Curve{"linear": plot.CurveLinear, "natural": plot.CurveNatural,
		"stepAfter": plot.CurveStepAfter}),
	Enum(map[string]base.ResizeHandleState{"idle": base.HandleIdle, "hovered": base.HandleHovered,
		"pressed": base.HandlePressed, "dragging": base.HandleDragging}),
	Enum(map[string]toast.SwipeDirection{"off": toast.SwipeOff, "up": toast.SwipeUp, "down": toast.SwipeDown,
		"left": toast.SwipeLeft, "right": toast.SwipeRight}),
	Enum(map[string]toast.Status{"starting": toast.Starting, "present": toast.Present,
		"ending": toast.Ending}),
	Enum(map[string]plot.TooltipPlacement{"top": plot.TooltipTop, "pointer": plot.TooltipPointer}),
	Enum(map[string]base.Reason{"cancel": base.ReasonCancel, "backdropPress": base.ReasonBackdropPress,
		"swipe": base.ReasonSwipe, "pressOutside": base.ReasonPressOutside, "triggerPress": base.ReasonTriggerPress,
		"hover": base.ReasonHover, "select": base.ReasonSelect, "contextMenu": base.ReasonContextMenu,
		"longPress": base.ReasonLongPress}),
	Enum(map[string]base.AlertPresentation{"alert": base.AlertPresentAlert,
		"actionSheet": base.AlertPresentActionSheet}),
	Enum(roles),
}

func init() {
	// Configs an app gives in part: what it leaves out is the default.
	Optional[core.TimingConfig]("Duration", "Easing", "Delay")
	Optional[core.SpringConfig]("Stiffness", "Damping", "Mass", "Velocity")
	Optional[core.AccessibilityValue]("Min", "Max", "Now")
	Optional[datatable.Column]("Width", "MinWidth", "MaxWidth", "FitPadding")
	Fields[datatable.Column](map[string]string{"NoResize": "resizable", "NoMove": "movable", "NoSelect": "selectable"})
	Required[datatable.Dump]("Headers", "Rows")
	Required[datatable.ColumnMove]("Key", "From", "To", "Order")
	// NSH's sort names its direction.
	Shape("DataTableSort", `export type DataTableSortDirection = "ascending" | "descending";
export interface DataTableSort { key: string; direction?: DataTableSortDirection }`,
		func(s datatable.Sort) map[string]any {
			if s.Key == "" {
				return nil
			}
			return map[string]any{"key": s.Key, "direction": map[bool]string{false: "ascending", true: "descending"}[s.Descending]}
		},
		func(m map[string]any) datatable.Sort {
			key, _ := m["key"].(string)
			return datatable.Sort{Key: key, Descending: m["direction"] == "descending"}
		})
	// NSH's selection leaves out what it does not select; null selects
	// nothing.
	Shape("DataTableSelection", `export interface DataTableSelection { row?: number | null; column?: number | null }`,
		func(s datatable.Selection) map[string]any {
			if s.Row < 0 && s.Column < 0 {
				return nil
			}
			m := map[string]any{}
			if s.Row >= 0 {
				m["row"] = s.Row
			}
			if s.Column >= 0 {
				m["column"] = s.Column
			}
			return m
		},
		func(m map[string]any) datatable.Selection {
			s := datatable.NoTableSelection
			if m["row"] != nil {
				s.Row = int(jsNumber(m["row"]))
			}
			if m["column"] != nil {
				s.Column = int(jsNumber(m["column"]))
			}
			return s
		})
	shapeNone(datatable.NoTableSelection)
	Shape("DataTableContextMenu", `export interface DataTableContextMenu { row?: number | null; column?: number | null; x: number; y: number }`,
		func(e datatable.ContextMenu) map[string]any {
			m := map[string]any{"x": js32(e.At.X), "y": js32(e.At.Y)}
			if e.Row >= 0 {
				m["row"] = e.Row
			}
			if e.Column >= 0 {
				m["column"] = e.Column
			}
			return m
		}, nil)
	registerPart("nsgo/base", "ResizablePanel", base.ResizablePanel{}, append(enumOptions, Event("OnResize", "size"))...)
	Register("nsgo/base", map[string]core.Node{
		"Button":              base.Button{},
		"Checkbox":            base.Checkbox{},
		"Collapsible":         base.Collapsible{},
		"DataTable":           datatable.DataTable{},
		"MenuSeparator":       base.MenuSeparator{},
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
		"Toast":               toast.Toast{},
		"Tooltip":             base.Tooltip{},
	}, append(enumOptions, baseRenames...)...)
	Register("nsgo/base", map[string]core.Node{"DockArea": dock.Area{}}, slices.Concat(enumOptions, baseRenames, dockOptions)...)
	Register("nsgo/base/plot", map[string]core.Node{
		"Plot": plot.Plot{},
		"Line": plot.Line{},
		"Area": plot.Area{},
		"Grid": plot.Grid{},
	}, append(append(enumOptions, baseRenames...), Rename("Tooltip", "tooltipContent"))...)
}

// baseRenames are the props whose NSH names base's fields do not give,
// and the props whose shape is not their field's.
var baseRenames = []Option{
	Rename("NoRowSelect", "rowSelectable"), Rename("NoColumnSelect", "columnSelectable"),
	Rename("CellSelect", "cellSelectable"), Rename("NoResizeColumns", "resizableColumns"),
	Rename("NoMoveColumns", "movableColumns"), Rename("NoBounce", "bounces"), Rename("NoHover", "interactive"),
	Rename("Status", "transitionStatus"),
	Event("OnChange", "checked"), Event("OnOpenChange", "open", "reason"), Event("OnResize", "sizes"),
	Event("OnColumnResize", "key", "width"), Event("OnSelectionChange", "selection"), Event("OnSortChange", "sort"),
	custom("Text", tableText, "data?: readonly unknown[]", "dataStart?: number"),
	custom("RowCount", tableRowCount, "rowCount?: number"),
	custom("RowClass", tableRowClass, "rowClasses?: readonly string[]"),
	custom("OnDoublePress", func(p *props, v reflect.Value) {
		if h := p.handler("onDoublePress"); h != nil {
			v.Set(reflect.ValueOf(func(row, column int) {
				e := p.r.vm.NewObject()
				_ = e.Set("target", p.r.target(row, column))
				h(e)
			}))
		}
	}, "onDoublePress?: (e: { target: DataTableTarget }) => void"),
	custom("OnVisibleRowsChange", rangeHandler("onVisibleRowsChange"),
		"onVisibleRowsChange?: (e: { range: DataTableRange }) => void"),
	custom("OnVisibleColumnsChange", rangeHandler("onVisibleColumnsChange"),
		"onVisibleColumnsChange?: (e: { range: DataTableRange }) => void"),
	custom("OnHoverChange", func(p *props, v reflect.Value) {
		if h := p.handler("onHoverChange"); h != nil {
			v.Set(reflect.ValueOf(func(index int) {
				e := p.r.vm.NewObject()
				_ = e.Set("index", goja.Null())
				if index >= 0 {
					_ = e.Set("index", index)
				}
				h(e)
			}))
		}
	}, "onHoverChange?: (e: { index: number | null }) => void"),
	custom("Y", plotValues("y"), "data?: readonly any[]", plotAxis("y")),
	custom("X", plotValues("x"), "data?: readonly any[]", plotAxis("x")),
	custom("Y1", plotValues("y"), "data?: readonly any[]", plotAxis("y")),
	custom("Y0", func(p *props, v reflect.Value) {
		if p.has("y0") {
			v.Set(reflect.ValueOf(func() float64 { return p.float("y0") }))
		}
	}, "y0?: number"),
	typed("XScale", custom("XScale", plotScale("xScale"), "xScale?: Scale"), reflect.TypeFor[plot.Scale]()),
	typed("YScale", custom("YScale", plotScale("yScale"), "yScale?: Scale"), reflect.TypeFor[plot.Scale]()),
}

func init() {
	// A plot's scale as NSH's declares it; plotScale reads it.
	Shape("Scale", `export type Scale = { type: "point" } | { type: "linear"; domain?: [number, number] };`,
		func(plot.Scale) map[string]any { return nil }, nil)
}

// plotAxis declares a plot's axis: a row's key, a function of a row and
// its index, or the values.
func plotAxis(axis string) string {
	return axis + "?: string | ((row: any, index: number) => number) | readonly number[]"
}

// target is { row, column }, leaving out the one that is -1, as NSH's
// DataTableTarget does.
func (r *runtime) target(row, column int) *goja.Object {
	o := r.vm.NewObject()
	if row >= 0 {
		_ = o.Set("row", row)
	}
	if column >= 0 {
		_ = o.Set("column", column)
	}
	return o
}

func rangeHandler(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if h := p.handler(prop); h != nil {
			v.Set(reflect.ValueOf(func(start, end int) {
				o := p.r.vm.NewObject()
				_ = o.Set("start", start)
				_ = o.Set("end", end)
				e := p.r.vm.NewObject()
				_ = e.Set("range", o)
				h(e)
			}))
		}
	}
}

// tableRow is row of a DataTable's data, whose first row is dataStart.
func tableRow(p *props, row int) goja.Value {
	return p.r.item(p.get("data"), row-p.int("dataStart"))
}

// tableText is a cell's text: its row's value under its column's key,
// or at its column's place in a row that is an array.
func tableText(p *props, v reflect.Value) {
	if !p.has("data") {
		return
	}
	v.Set(reflect.ValueOf(func(row, column int) string {
		cols := p.r.arrayItems(p.get("columns"))
		if column < 0 || column >= len(cols) {
			return ""
		}
		o, ok := tableRow(p, row).(*goja.Object)
		if !ok {
			return ""
		}
		if p.r.isArray(o) {
			return stringOf(p.r.item(o, column))
		}
		return stringOf(o.Get(stringOf(p.r.options(cols[column]).get("key"))))
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
	// Row dataStart + i takes rowClasses[i], as data's.
	v.Set(reflect.ValueOf(func(row int) string {
		items := p.r.arrayItems(p.get("rowClasses"))
		if i := row - p.int("dataStart"); i >= 0 && i < len(items) {
			return stringOf(items[i])
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
		v.Set(reflect.ValueOf(func() plot.Scale {
			o := p.r.options(p.get(prop))
			if o.str("type", "") == "point" {
				return plot.PointScale()
			}
			var domain []float64
			for _, d := range p.r.arrayItems(o.get("domain")) {
				domain = append(domain, d.ToFloat())
			}
			return plot.LinearScale(domain...)
		}))
	}
}
