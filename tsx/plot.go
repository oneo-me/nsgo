package tsx

import (
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/base/plot"
	"github.com/nsgo-dev/nsgo/core"
)

// nsgo/base/plot: base/plot's marks with NSH's names and props. Rows are
// the plot's data, and what a mark reads of them a key, a function of a row
// and its index, or the values: Go's columns are functions returning slices.
func init() {
	Register("nsgo/base/plot", map[string]core.Node{
		"Plot": plot.Plot{}, "PlotAppearScope": plot.AppearScope{}, "Line": plot.Line{}, "Area": plot.Area{}, "Grid": plot.Grid{},
		"Bar": plot.Bar{}, "Rule": plot.Rule{}, "Dot": plot.Dot{}, "Cell": plot.Cell{}, "Pie": plot.Pie{}, "Arc": plot.Arc{},
		"RadialLine": plot.RadialLine{}, "RadialGrid": plot.RadialGrid{}, "PlotAxis": plot.Axis{}, "PlotLabel": plot.Label{},
		"Sankey": plot.Sankey{}, "Treemap": plot.Treemap{},
	}, append(append(slices.Clone(enumOptions), plotEnums...), plotProps...)...)
	// A plot's scales as NSH declares them; plotScale and plotColors read
	// them.
	Shape("Scale", `export type Scale = { type: "point"; reverse?: boolean } | { type: "linear"; domain?: [number, number]; reverse?: boolean } | { type: "band"; domain?: string[]; paddingInner?: number; paddingOuter?: number; maxBandWidth?: number; bandCount?: number; groupPadding?: number; reverse?: boolean };`,
		func(plot.Scale) map[string]any { return nil }, nil)
	Shape("ColorScale", `export type ColorScale = { type: "ordinal"; domain?: string[]; range: string[]; unknown?: string } | { type: "sequential"; domain?: [number, number]; range: string[]; unknown?: string } | { type: "diverging"; domain?: [number, number]; pivot?: number; range: string[]; unknown?: string };`,
		func(plot.ColorScale) map[string]any { return nil }, nil)
	Shape("BarCornerRadii", `export type BarCornerRadii = number | { topLeft?: number; topRight?: number; bottomRight?: number; bottomLeft?: number };`,
		func(plot.Corners) map[string]any { return nil }, nil)
	Shape("RadiusScale", `export type RadiusScale = { type: "sqrt"; domain?: [number, number]; range: [number, number] };`,
		func(plot.RadiusScale) map[string]any { return nil }, nil)
}

var plotEnums = []Option{
	Enum(map[string]plot.Curve{"linear": plot.CurveLinear, "natural": plot.CurveNatural, "stepAfter": plot.CurveStepAfter}),
	Enum(map[string]plot.TooltipPlacement{"top": plot.TooltipTop, "pointer": plot.TooltipPointer}),
	Enum(map[string]plot.ValueScale{"y": plot.ScaleY, "y2": plot.ScaleY2}),
	Enum(map[string]plot.AxisPlacement{"bottom": plot.AxisBottom, "top": plot.AxisTop, "left": plot.AxisLeft, "right": plot.AxisRight}),
	Enum(map[string]plot.AxisLabelPlacement{"outside": plot.LabelsOutside, "inside": plot.LabelsInside}),
	Enum(map[string]plot.TextAlign{"left": plot.TextLeft, "center": plot.TextCenter, "right": plot.TextRight}),
	Enum(map[string]plot.SankeyAlign{"justify": plot.SankeyJustify, "left": plot.SankeyLeft, "right": plot.SankeyRight, "center": plot.SankeyCenter}),
	Enum(map[string]plot.SankeyValueScale{"linear": plot.SankeyLinear, "sqrt": plot.SankeySqrt}),
	Enum(map[string]plot.TreemapSort{"value": plot.TreemapByValue, "data": plot.TreemapByRow}),
	Enum(map[string]plot.TreemapLabelPlacement{"start": plot.TreemapLabelStart, "center": plot.TreemapLabelCenter}),
}

var plotProps = []Option{
	Rename("NoHover", "interactive"), Rename("Tooltip", "tooltipContent"), Rename("Corners", "cornerRadii"),
	custom("Count", plotCount, "data?: readonly any[]"),
	custom("OnHoverChange", func(p *props, v reflect.Value) {
		if h := p.handler("onHoverChange"); h != nil {
			v.Set(reflect.ValueOf(func(index int) {
				e := p.r.vm.NewObject()
				_ = e.Set("index", js.Null())
				if index >= 0 {
					_ = e.Set("index", index)
				}
				h(e)
			}))
		}
	}, "onHoverChange?: (e: { index: number | null }) => void"),
	custom("Y", plotColumn("y"), rowProp("y", "number")),
	custom("X", plotColumn("x"), rowProp("x", "number | string")),
	custom("Y1", plotColumn("y1", "y"), rowProp("y1", "number")),
	custom("Value", plotColumn("value"), rowProp("value", "number")),
	custom("ColorValue", plotColumn("colorValue"), rowProp("colorValue", "number")),
	custom("Radius", plotColumn("radius"), rowProp("radius", "number")),
	custom("Angle", plotColumn("angle"), rowProp("angle", "number")),
	custom("Color", plotColumn("color"), rowProp("color", "string | number")),
	custom("Label", plotColumn("label"), rowProp("label", "string | number")),
	custom("NodeLabel", plotColumn("nodeLabel"), rowProp("nodeLabel", "string")),
	custom("GroupLabel", plotColumn("groupLabel"), rowProp("groupLabel", "string")),
	custom("ID", plotColumn("id"), rowProp("id", "string | number")),
	custom("Parent", plotColumn("parent"), rowProp("parent", "string | number")),
	// A number for every row, or what reads each row's.
	custom("Y0", plotConstant("y0"), rowProp("y0", "number", "number")),
	custom("Lows", plotPerRow("y0")),
	custom("Base", plotConstant("base"), rowProp("base", "number", "number")),
	custom("Bases", plotPerRow("base")),
	custom("Sizes", plotPerRow("r")),
	custom("R", func(p *props, v reflect.Value) {
		if x := p.get("r"); defined(x) && !isFunction(x) && !isKey(x) {
			v.SetFloat(x.ToFloat())
		}
	}, rowProp("r", "number", "number")),
	// NSH's opacity of what is away from the hover, or how much it fades.
	custom("HoverDim", func(p *props, v reflect.Value) {
		switch {
		case p.has("inactiveOpacity"):
			v.SetFloat(1 - p.float("inactiveOpacity"))
		case p.has("hoverDim"):
			v.SetFloat(p.float("hoverDim"))
		}
	}, "inactiveOpacity?: number", "hoverDim?: number"),
	typed("Corners", custom("Corners", plotCorners, "cornerRadii?: BarCornerRadii"), reflect.TypeFor[plot.Corners]()),
	typed("RScale", custom("RScale", plotRadiusScale, "rScale?: RadiusScale"), reflect.TypeFor[plot.RadiusScale]()),
	typed("XScale", custom("XScale", plotScale("xScale"), "xScale?: Scale"), reflect.TypeFor[plot.Scale]()),
	typed("YScale", custom("YScale", plotScale("yScale"), "yScale?: Scale"), reflect.TypeFor[plot.Scale]()),
	typed("Y2Scale", custom("Y2Scale", plotScale("y2Scale"), "y2Scale?: Extract<Scale, { type: \"linear\" }>"), reflect.TypeFor[plot.Scale]()),
	typed("ColorScale", custom("ColorScale", plotColors, "colorScale?: ColorScale"), reflect.TypeFor[plot.ColorScale]()),
	custom("InnerRadius", plotLength("innerRadius"), "innerRadius?: number | `${number}%`"),
	custom("OuterRadius", plotLength("outerRadius"), "outerRadius?: number | `${number}%`"),
	custom("SliceClass", plotNone), custom("SliceClassOf", plotClass("sliceClass", false), classProp("sliceClass", "any")),
	custom("LabelLineClass", plotNone), custom("LabelLineClassOf", plotClass("labelLineClass", false), classProp("labelLineClass", "any")),
	custom("NodeClass", plotNone), custom("NodeClassOf", plotClass("nodeClass", false), classProp("nodeClass", "any")),
	custom("LeafClass", plotNone), custom("LeafClassOf", plotClass("leafClass", false), classProp("leafClass", "any")),
	custom("GroupClass", plotNone), custom("GroupClassOf", plotClass("groupClass", false), classProp("groupClass", "any")),
	custom("LinkClass", plotNone), custom("LinkClassOf", plotClass("linkClass", true), classProp("linkClass", "SankeyLink")),
	custom("LinkBackgroundClass", plotNone), custom("LinkBackgroundClassOf", plotClass("linkBackgroundClass", true), classProp("linkBackgroundClass", "SankeyLink")),
	custom("ValueLabel", plotValueLabel, "valueLabel?: (row: any, value: number, index: number) => string"),
	custom("SliceStyleOf", plotNone), custom("LabelLineStyleOf", plotNone), custom("NodeStyleOf", plotNone),
	custom("LinkStyleOf", plotNone), custom("LinkBackgroundStyleOf", plotNone),
}

func plotNone(*props, reflect.Value) {}

// rowProp declares a prop of the rows: a key, a function of a row and its
// index giving out, or the values; or one of others too.
func rowProp(name, out string, others ...string) string {
	types := append(others, "string", "((row: any, index: number) => "+out+")", "readonly ("+out+")[]")
	return name + "?: " + strings.Join(types, " | ")
}

func classProp(name, row string) string {
	return name + "?: string | ((d: " + row + ", index: number) => string)"
}

// isKey reports whether v is a key of the rows, or the values themselves.
func isKey(v js.Value) bool {
	switch v.Export().(type) {
	case string, []any:
		return true
	}
	return false
}

// plotOwner is the props that hold the rows a mark reads: its own data, or
// its plot's.
func plotOwner(p *props) *props {
	if p.has("data") {
		return p
	}
	for n := p.n.parent; n != nil; n = n.parent {
		if n.kind == kindFace && n.name == "Plot" {
			return n.props
		}
	}
	return p
}

// plotCount is how many rows a plot's data has.
func plotCount(p *props, v reflect.Value) {
	if p.has("data") {
		v.Set(reflect.ValueOf(func() int { return len(p.r.arrayItems(p.get("data"))) }))
	}
}

// readRows calls set with the value each row gives by, a key, a function of
// a row and its index, or the values, over owner's rows.
func readRows(p, owner *props, by js.Value, set func(i int, x js.Value), size func(n int)) {
	r := p.r
	if items := r.arrayItems(by); items != nil {
		size(len(items))
		for i, it := range items {
			set(i, it)
		}
		return
	}
	rows := r.arrayItems(owner.get("data"))
	size(len(rows))
	for i, row := range rows {
		var x js.Value
		if isFunction(by) {
			x = r.call(by, row, r.vm.ToValue(i))
		} else if o, ok := row.(*js.Object); ok {
			x = o.Get(by.String())
		}
		set(i, x)
	}
}

// plotColumn reads a column of a plot or a mark, numbers or texts as its
// field takes, from the first of names it has; without one a mark reads the
// plot's.
func plotColumn(names ...string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		accessor := func() js.Value {
			for _, name := range names {
				if x := p.get(name); defined(x) {
					return x
				}
			}
			return js.Undefined()
		}
		if defined(core.Untracked(p.r.cx, accessor)) {
			setRead(v, p, plotOwner(p), accessor)
		}
	}
}

// plotPerRow reads prop when it is what reads each row, not a number.
func plotPerRow(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		x := p.get(prop)
		if !defined(x) || !isFunction(x) && !isKey(x) {
			return
		}
		setRead(v, p, plotOwner(p), func() js.Value { return p.get(prop) })
	}
}

// setRead sets v, a function returning a column, to read accessor's.
func setRead(v reflect.Value, p, owner *props, accessor func() js.Value) {
	switch v.Interface().(type) {
	case func() []string:
		v.Set(reflect.ValueOf(func() []string {
			var out []string
			readRows(p, owner, accessor(), func(i int, x js.Value) {
				if defined(x) {
					out[i] = stringOf(x)
				}
			}, func(n int) { out = make([]string, n) })
			return out
		}))
	default:
		v.Set(reflect.ValueOf(func() []float64 {
			var out []float64
			readRows(p, owner, accessor(), func(i int, x js.Value) {
				out[i] = math.NaN()
				if defined(x) {
					out[i] = x.ToFloat()
				}
			}, func(n int) { out = make([]float64, n) })
			return out
		}))
	}
}

// plotConstant reads prop when it is a number.
func plotConstant(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if x := p.get(prop); defined(x) && !isFunction(x) && !isKey(x) {
			v.Set(reflect.ValueOf(func() float64 { return p.float(prop) }))
		}
	}
}

// plotScale reads a Scale.
func plotScale(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if !p.has(prop) {
			return
		}
		v.Set(reflect.ValueOf(func() plot.Scale {
			o := p.r.options(p.get(prop))
			var s plot.Scale
			switch o.str("type", "") {
			case "point":
				s = plot.PointScale()
			case "band":
				band := plot.Band{PaddingInner: float32(o.float("paddingInner", 0)), PaddingOuter: float32(o.float("paddingOuter", 0)),
					MaxBandWidth: core.Points(o.float("maxBandWidth", 0)), BandCount: int(o.float("bandCount", 0)),
					GroupPadding: float32(o.float("groupPadding", 0))}
				if o.has("domain") {
					band.Domain = []string{}
					for _, d := range p.r.arrayItems(o.get("domain")) {
						band.Domain = append(band.Domain, stringOf(d))
					}
				}
				s = plot.BandScale(band)
			default:
				var domain []float64
				for _, d := range p.r.arrayItems(o.get("domain")) {
					domain = append(domain, d.ToFloat())
				}
				s = plot.LinearScale(domain...)
			}
			if defined(o.get("reverse")) && o.get("reverse").ToBoolean() {
				s = s.Reversed()
			}
			return s
		}))
	}
}

func parseColor(s string) core.Color {
	c, _ := cssColor(s)
	return c
}

// plotColors reads a ColorScale.
func plotColors(p *props, v reflect.Value) {
	if !p.has("colorScale") {
		return
	}
	v.Set(reflect.ValueOf(func() plot.ColorScale {
		o := p.r.options(p.get("colorScale"))
		var colors []core.Color
		for _, c := range p.r.arrayItems(o.get("range")) {
			colors = append(colors, parseColor(stringOf(c)))
		}
		var s plot.ColorScale
		switch o.str("type", "") {
		case "sequential":
			s = plot.SequentialColors(colors...)
		case "diverging":
			s = plot.DivergingColors(colors...).Pivot(o.float("pivot", 0))
		default:
			var domain []string
			for _, d := range p.r.arrayItems(o.get("domain")) {
				domain = append(domain, stringOf(d))
			}
			s = plot.OrdinalColors(domain, colors...)
		}
		if d := p.r.arrayItems(o.get("domain")); len(d) == 2 && o.str("type", "") != "ordinal" {
			s = s.Over(d[0].ToFloat(), d[1].ToFloat())
		}
		if o.has("unknown") {
			s = s.Unknown(parseColor(o.str("unknown", "")))
		}
		return s
	}))
}

// plotCorners reads a number for all four corners, or the corners named.
func plotCorners(p *props, v reflect.Value) {
	x := p.get("cornerRadii")
	if !defined(x) {
		return
	}
	if _, ok := x.(*js.Object); !ok {
		v.Set(reflect.ValueOf(plot.AllCorners(core.Points(x.ToFloat()))))
		return
	}
	o := p.r.options(x)
	corner := func(k string) core.Points { return core.Points(o.float(k, 0)) }
	v.Set(reflect.ValueOf(plot.Corners{TopLeft: corner("topLeft"), TopRight: corner("topRight"),
		BottomRight: corner("bottomRight"), BottomLeft: corner("bottomLeft")}))
}

// plotRadiusScale reads { type: "sqrt", domain?, range }.
func plotRadiusScale(p *props, v reflect.Value) {
	if !p.has("rScale") {
		return
	}
	o := p.r.options(p.get("rScale"))
	s := &plot.RadiusScale{}
	if rng := p.r.arrayItems(o.get("range")); len(rng) == 2 {
		s.Min, s.Max = core.Points(rng[0].ToFloat()), core.Points(rng[1].ToFloat())
	}
	if d := p.r.arrayItems(o.get("domain")); len(d) == 2 {
		s.Domain, s.HasDomain = [2]float64{d[0].ToFloat(), d[1].ToFloat()}, true
	}
	v.Set(reflect.ValueOf(s))
}

// plotLength reads points, or a percentage as "60%".
func plotLength(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if !p.has(prop) {
			return
		}
		v.Set(reflect.ValueOf(func() core.Length {
			x := p.get(prop)
			if text, ok := x.Export().(string); ok && strings.HasSuffix(text, "%") {
				pct, _ := strconv.ParseFloat(strings.TrimSuffix(text, "%"), 32)
				return core.Pct(float32(pct))
			}
			return core.Pt(core.Points(x.ToFloat()))
		}))
	}
}

// plotClass reads the classes of each row, or with links of each link: the
// same on every one, or a function of it and its index.
func plotClass(prop string, links bool) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if !p.has(prop) {
			return
		}
		owner := plotOwner(p)
		v.Set(reflect.ValueOf(func(i int) string {
			x := p.get(prop)
			if !isFunction(x) {
				return stringOf(x)
			}
			rows := owner.get("data")
			if links {
				rows = p.get("links")
			}
			return stringOf(p.r.call(x, p.r.item(rows, i), p.r.vm.ToValue(i)))
		}))
	}
}

// plotValueLabel reads a function of a row, its value and its index.
func plotValueLabel(p *props, v reflect.Value) {
	if !p.has("valueLabel") {
		return
	}
	owner := plotOwner(p)
	v.Set(reflect.ValueOf(func(row int, value float64) string {
		return stringOf(p.r.call(p.get("valueLabel"), p.r.item(owner.get("data"), row), p.r.vm.ToValue(value), p.r.vm.ToValue(row)))
	}))
}
