package driver

import (
	"math"
	"slices"
	"testing"
)

// ops parses doc and returns the drawing operations it records.
func svgOps(t *testing.T, doc string) []DrawOp {
	t.Helper()
	s, err := ParseSVG([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return s.ops
}

func opKinds(ops []DrawOp) []DrawKind {
	out := make([]DrawKind, len(ops))
	for i, op := range ops {
		out[i] = op.Kind
	}
	return out
}

func TestSVGPathCommands(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
		<path d="M2 2h4v4H2z"/>
	</svg>`)
	want := []DrawKind{DrawBeginPath, DrawMoveTo, DrawLineTo, DrawLineTo, DrawLineTo, DrawClosePath,
		DrawStrokeColor, DrawLineWidth, DrawLineCap, DrawLineJoin, DrawStroke}
	if got := opKinds(ops); !slices.Equal(got, want) {
		t.Fatalf("ops = %v, want %v", got, want)
	}
	points := [][2]float32{{2, 2}, {6, 2}, {6, 6}, {2, 6}}
	for i, p := range points {
		if op := ops[1+i]; op.X != p[0] || op.Y != p[1] {
			t.Errorf("point %d = %g,%g, want %g,%g", i, op.X, op.Y, p[0], p[1])
		}
	}
	if ops[6].Color != 0 || ops[7].W != 2 || ops[8].Cap != CapRound || ops[9].Join != JoinRound {
		t.Errorf("stroke = %+v %+v %+v %+v", ops[6], ops[7], ops[8], ops[9])
	}
}

func TestSVGPathCurves(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 24 24"><path d="M0 0 C1 1 2 2 3 3 S5 5 6 6 Q7 7 8 8 T10 10"/></svg>`)
	if got := opKinds(ops); !slices.Equal(got, []DrawKind{DrawBeginPath, DrawMoveTo, DrawCubicTo, DrawCubicTo, DrawQuadTo, DrawQuadTo, DrawFillColor, DrawFill}) {
		t.Fatalf("ops = %v", got)
	}
	// S reflects C's second control point about (3, 3).
	if op := ops[3]; op.X1 != 4 || op.Y1 != 4 || op.X2 != 5 || op.Y2 != 5 || op.X != 6 || op.Y != 6 {
		t.Errorf("smooth cubic = %+v", op)
	}
	// T reflects Q's control point about (8, 8).
	if op := ops[5]; op.X1 != 9 || op.Y1 != 9 || op.X != 10 || op.Y != 10 {
		t.Errorf("smooth quad = %+v", op)
	}
}

func TestSVGPathArcs(t *testing.T) {
	// A half circle of radius 5 from (0, 0) to (10, 0).
	ops := svgOps(t, `<svg viewBox="0 0 24 24"><path d="M0 0 A5 5 0 0 1 10 0"/></svg>`)
	if got := opKinds(ops); !slices.Equal(got, []DrawKind{DrawBeginPath, DrawMoveTo, DrawCubicTo, DrawCubicTo, DrawFillColor, DrawFill}) {
		t.Fatalf("ops = %v", got)
	}
	last := ops[3]
	if math.Abs(float64(last.X-10)) > 0.01 || math.Abs(float64(last.Y)) > 0.01 {
		t.Fatalf("arc ends at %g,%g, want 10,0", last.X, last.Y)
	}
	// The sweep flag picks the other half: 1 goes over the top, 0 under it.
	ops = svgOps(t, `<svg viewBox="0 0 24 24"><path d="M0 0 A5 5 0 0 0 10 0"/></svg>`)
	below := false
	for _, op := range ops {
		if op.Kind == DrawCubicTo && (op.Y1 > 0 || op.Y2 > 0) {
			below = true
		}
	}
	if !below {
		t.Fatal("sweep 0 should pass below the chord")
	}
	// Rotated, relative arcs and flags without separators, as galaxy.svg has.
	ops = svgOps(t, `<svg viewBox="0 0 24 24"><path d="M16.005 15.108a5.041 6.52 28.25 00-8.008-6.217"/></svg>`)
	var lastCubic DrawOp
	cubics := 0
	for _, op := range ops {
		if op.Kind == DrawCubicTo {
			cubics++
			lastCubic = op
		}
	}
	if cubics < 2 {
		t.Fatalf("arc drew %d cubics", cubics)
	}
	if math.Abs(float64(lastCubic.X-7.997)) > 0.01 || math.Abs(float64(lastCubic.Y-8.891)) > 0.01 {
		t.Errorf("relative arc ends at %g,%g, want 7.997,8.891", lastCubic.X, lastCubic.Y)
	}
	for _, op := range ops {
		if math.IsNaN(float64(op.X)) || math.IsInf(float64(op.X), 0) {
			t.Fatalf("non-finite op %+v", op)
		}
	}
}

func TestSVGShapes(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 24 24">
		<circle cx="4" cy="4" r="2"/>
		<ellipse cx="10" cy="4" rx="3" ry="2"/>
		<line x1="0" y1="10" x2="10" y2="10"/>
		<polyline points="0,12 4,16 8,12"/>
		<polygon points="12,12 16,16 20,12"/>
		<rect x="1" y="18" width="6" height="4" rx="1"/>
	</svg>`)
	want := []DrawKind{
		DrawBeginPath, DrawEllipse, DrawFillColor, DrawFill,
		DrawBeginPath, DrawEllipse, DrawFillColor, DrawFill,
		DrawBeginPath, DrawMoveTo, DrawLineTo, DrawFillColor, DrawFill,
		DrawBeginPath, DrawMoveTo, DrawLineTo, DrawLineTo, DrawFillColor, DrawFill,
		DrawBeginPath, DrawMoveTo, DrawLineTo, DrawLineTo, DrawClosePath, DrawFillColor, DrawFill,
		DrawBeginPath, DrawRoundRect, DrawFillColor, DrawFill,
	}
	if got := opKinds(ops); !slices.Equal(got, want) {
		t.Fatalf("ops = %v, want %v", got, want)
	}
	if op := ops[1]; op.R != 2 || op.RY != 2 {
		t.Errorf("circle = %+v", op)
	}
	if op := ops[27]; op.Kind != DrawRoundRect || op.R != 1 {
		t.Errorf("rect = %+v", op)
	}
}

func TestSVGDrawFitsCentersAndColors(t *testing.T) {
	s := mustParse(t, []byte(`<svg width="24" height="24" viewBox="0 0 24 24"><rect width="24" height="24" fill="currentColor"/></svg>`))
	if w, h := s.Size(); w != 24 || h != 24 {
		t.Fatalf("size = %gx%g", w, h)
	}
	d := &Drawing{}
	s.Draw(d, Size{Width: 48, Height: 24}, RGB(0x123456))
	want := []DrawOp{
		{Kind: DrawSave},
		{Kind: DrawTranslate, X: 12, Y: 0},
		{Kind: DrawScale, X: 1, Y: 1},
		{Kind: DrawTranslate, X: 0, Y: 0},
		{Kind: DrawBeginPath},
		{Kind: DrawRect, W: 24, H: 24},
		{Kind: DrawFillColor, Color: RGB(0x123456)},
		{Kind: DrawFill},
		{Kind: DrawRestore},
	}
	if len(d.Ops) != len(want) {
		t.Fatalf("ops = %+v", d.Ops)
	}
	for i := range want {
		if d.Ops[i] != want[i] {
			t.Errorf("op %d = %+v, want %+v", i, d.Ops[i], want[i])
		}
	}
	// A zero color paints black, not the transparent zero value.
	d = &Drawing{}
	s.Draw(d, Size{Width: 24, Height: 24}, 0)
	if d.Ops[6].Color != RGB(0x000000) {
		t.Errorf("zero color = %#x", d.Ops[6].Color)
	}
}

func TestSVGViewBoxAndStyles(t *testing.T) {
	s := mustParse(t, []byte(`<svg viewBox="2 4 10 20" fill="none">
		<g stroke="currentColor" stroke-width="3" stroke-linecap="square" stroke-linejoin="bevel">
			<path d="M0 0L1 1"/>
		</g>
		<path d="M0 0L1 1" fill="#f00" stroke="none"/>
	</svg>`))
	if w, h := s.Size(); w != 10 || h != 20 {
		t.Fatalf("size from viewBox = %gx%g", w, h)
	}
	d := &Drawing{}
	s.Draw(d, Size{Width: 20, Height: 20}, RGB(0x00ff00))
	// The group strokes in the icon color, with its width, cap and join.
	var cap LineCap
	var join LineJoin
	var width float32
	var strokes, fills int
	for _, op := range d.Ops {
		switch op.Kind {
		case DrawStroke:
			strokes++
		case DrawFill:
			fills++
		case DrawLineWidth:
			width = op.W
		case DrawLineCap:
			cap = op.Cap
		case DrawLineJoin:
			join = op.Join
		case DrawStrokeColor:
			if op.Color != RGB(0x00ff00) {
				t.Errorf("stroke color = %#x", op.Color)
			}
		}
	}
	if strokes != 1 || fills != 1 || width != 3 || cap != CapSquare || join != JoinBevel {
		t.Errorf("strokes %d fills %d width %g cap %v join %v", strokes, fills, width, cap, join)
	}
	// The viewBox's origin shifts the drawing.
	if op := d.Ops[3]; op.Kind != DrawTranslate || op.X != -2 || op.Y != -4 {
		t.Errorf("viewBox translate = %+v", op)
	}
}

func TestSVGRejectsBadDocuments(t *testing.T) {
	for _, doc := range []string{
		``,
		`<div/>`,
		`<svg/>`,
		`<svg viewBox="0 0 24 24"><path d="M0 0 L"/></svg>`,
		`<svg viewBox="0 0 24 24"><rect width="x"/></svg>`,
	} {
		if _, err := ParseSVG([]byte(doc)); err == nil {
			t.Errorf("ParseSVG(%q) succeeded", doc)
		}
	}
}

// kindsOf returns the operations of the given kind.
func kindsOf(ops []DrawOp, kind DrawKind) []DrawOp {
	var out []DrawOp
	for _, op := range ops {
		if op.Kind == kind {
			out = append(out, op)
		}
	}
	return out
}

func TestSVGTransforms(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 24 24">
		<g transform="translate(2 3) scale(2)">
			<rect x="1" y="1" width="2" height="3"/>
			<circle cx="1" cy="1" r="1" fill="none" stroke="#000" stroke-width="0.5"/>
		</g>
		<rect width="2" height="2" transform="rotate(45)"/>
		<ellipse cx="0" cy="0" rx="2" ry="1" transform="skewX(10)"/>
	</svg>`)
	rects := kindsOf(ops, DrawRect)
	if len(rects) != 1 || rects[0].X != 4 || rects[0].Y != 5 || rects[0].W != 4 || rects[0].H != 6 {
		t.Errorf("scaled rect = %+v", rects)
	}
	ellipses := kindsOf(ops, DrawEllipse)
	if len(ellipses) != 1 || ellipses[0].X != 4 || ellipses[0].Y != 5 || ellipses[0].R != 2 || ellipses[0].RY != 2 {
		t.Errorf("scaled circle = %+v", ellipses)
	}
	// Strokes scale with the transform.
	if w := kindsOf(ops, DrawLineWidth); len(w) != 1 || w[0].W != 1 {
		t.Errorf("line width = %+v", w)
	}
	// A rotated rect becomes a path through its rotated corners.
	moves := kindsOf(ops, DrawMoveTo)
	if len(moves) != 2 || moves[0].X != 0 || moves[0].Y != 0 {
		t.Fatalf("moves = %+v", moves)
	}
	lines := kindsOf(ops, DrawLineTo)
	if len(lines) != 3 || math.Abs(float64(lines[0].X)-math.Sqrt2) > 1e-5 || math.Abs(float64(lines[0].Y)-math.Sqrt2) > 1e-5 {
		t.Errorf("rotated corner = %+v", lines)
	}
	// A skewed ellipse becomes four curves.
	if c := kindsOf(ops, DrawCubicTo); len(c) != 4 {
		t.Errorf("skewed ellipse curves = %d", len(c))
	}
}

func TestSVGColorsAndOpacity(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 10 10">
		<rect width="1" height="1" fill="rgb(255, 0, 0)" fill-opacity="0.5"/>
		<rect width="1" height="1" style="fill: rgba(0, 0, 100%, 50%)" opacity="0.5"/>
		<rect width="1" height="1" fill="cornflowerblue" fill-rule="evenodd"/>
		<rect width="1" height="1" fill="#0000"/>
		<rect width="1" height="1" fill="hsl(120, 100%, 25%)"/>
		<g color="#ff0"><rect width="1" height="1" fill="currentColor"/></g>
		<rect width="1" height="1" fill="transparent"/>
		<rect width="1" height="1" fill="unknown"/>
	</svg>`)
	want := []Color{0xff000080, 0x0000ff40, RGB(0x6495ed), RGB(0x008000), RGB(0xffff00), RGB(0x000000)}
	got := kindsOf(ops, DrawFillColor)
	if len(got) != len(want) {
		t.Fatalf("fill colors = %+v", got)
	}
	for i, c := range want {
		if got[i].Color != c {
			t.Errorf("fill %d = %#x, want %#x", i, got[i].Color, c)
		}
	}
	fills := kindsOf(ops, DrawFill)
	if !fills[2].EvenOdd || fills[0].EvenOdd {
		t.Errorf("fill rules = %+v", fills)
	}
	// currentColor keeps the color zero and fades with the global alpha.
	ops = svgOps(t, `<svg viewBox="0 0 10 10"><g opacity="0.5"><rect width="1" height="1" fill="currentColor"/></g></svg>`)
	wantKinds := []DrawKind{DrawBeginPath, DrawRect, DrawSave, DrawAlpha, DrawFillColor, DrawFill, DrawRestore}
	if got := opKinds(ops); !slices.Equal(got, wantKinds) || ops[3].A != 0.5 || ops[4].Color != 0 {
		t.Errorf("faded currentColor = %+v", ops)
	}
}

func TestSVGGradients(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 10 10">
		<rect x="2" y="4" width="4" height="2" fill="url(#b)" stroke="url('#r')"/>
		<rect width="1" height="1" fill="url(#missing) #0f0"/>
		<rect width="1" height="1" fill="url(#missing)"/>
		<defs>
			<linearGradient id="a"><stop offset="0" stop-color="#f00"/><stop offset="100%" style="stop-color: #00f"/></linearGradient>
			<linearGradient id="b" xlink:href="#a" xmlns:xlink="http://www.w3.org/1999/xlink" x2="0" y2="1"/>
			<radialGradient id="r" gradientUnits="userSpaceOnUse" cx="5" cy="5" r="5" spreadMethod="reflect">
				<stop offset="0.25" stop-color="#fff" stop-opacity="0.5"/><stop offset="0.75" stop-color="#000"/>
			</radialGradient>
		</defs>
	</svg>`)
	want := []DrawKind{DrawBeginPath, DrawRect, DrawFillColor, DrawFillGradient, DrawFill,
		DrawStrokeColor, DrawStrokeGradient, DrawLineWidth, DrawLineCap, DrawLineJoin, DrawStroke,
		DrawBeginPath, DrawRect, DrawFillColor, DrawFill}
	if got := opKinds(ops); !slices.Equal(got, want) {
		t.Fatalf("ops = %v, want %v", got, want)
	}
	// The fallback before each gradient is its mean color.
	if c := ops[2].Color; c != 0x800080ff {
		t.Errorf("fallback = %#x", c)
	}
	g := ops[3].Gradient
	if g == nil || g.Radial || g.X1 != 0 || g.Y1 != 0 || g.X2 != 0 || g.Y2 != 1 || len(g.Stops) != 2 ||
		g.Stops[0] != (GradientStop{0, RGB(0xff0000)}) || g.Stops[1] != (GradientStop{1, RGB(0x0000ff)}) {
		t.Fatalf("linear gradient = %+v", g)
	}
	// Bounding box units map the unit square onto the rect.
	if g.Transform != [6]float32{4, 0, 0, 2, 2, 4} {
		t.Errorf("transform = %v", g.Transform)
	}
	r := ops[6].Gradient
	if r == nil || !r.Radial || r.X2 != 5 || r.Y2 != 5 || r.R2 != 5 || r.X1 != 5 || r.R1 != 0 ||
		r.Spread != SpreadReflect || r.Transform != [6]float32{1, 0, 0, 1, 0, 0} || r.Stops[0].Color != 0xffffff80 {
		t.Errorf("radial gradient = %+v", r)
	}
	if c := ops[13].Color; c != RGB(0x00ff00) {
		t.Errorf("fallback of a missing gradient = %#x", c)
	}
}

func TestSVGStyleSheetsAndUse(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 10 10">
		<style>/* rules */ .a { fill: #f00 } rect.a#x { fill: #0f0 !important }
			@media print { rect { fill: #fff } } g > rect { fill: #000 }</style>
		<rect class="a" width="1" height="1"/>
		<rect class="a" id="x" width="1" height="1"/>
		<rect class="a b" width="1" height="1" style="fill:#00f"/>
		<defs><path id="p" d="M0 0h1v1z"/></defs>
		<use href="#p" x="3" y="4" fill="#ff0"/>
	</svg>`)
	want := []Color{RGB(0xff0000), RGB(0x00ff00), RGB(0x0000ff), RGB(0xffff00)}
	got := kindsOf(ops, DrawFillColor)
	if len(got) != len(want) {
		t.Fatalf("fill colors = %+v", got)
	}
	for i, c := range want {
		if got[i].Color != c {
			t.Errorf("fill %d = %#x, want %#x", i, got[i].Color, c)
		}
	}
	if m := kindsOf(ops, DrawMoveTo); len(m) != 1 || m[0].X != 3 || m[0].Y != 4 {
		t.Errorf("used path starts at %+v", m)
	}
}

func TestSVGClipPaths(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 10 10">
		<clipPath id="c"><rect width="5" height="5" clip-rule="evenodd"/></clipPath>
		<g clip-path="url(#c)"><rect width="10" height="10"/></g>
		<clipPath id="empty"/>
		<rect width="10" height="10" clip-path="url(#empty)"/>
	</svg>`)
	want := []DrawKind{DrawSave, DrawBeginPath, DrawRect, DrawClip, DrawBeginPath, DrawRect, DrawFillColor, DrawFill, DrawRestore}
	if got := opKinds(ops); !slices.Equal(got, want) {
		t.Fatalf("ops = %v, want %v", got, want)
	}
	if !ops[3].EvenOdd || ops[2].W != 5 {
		t.Errorf("clip = %+v %+v", ops[2], ops[3])
	}
}

func TestSVGHiddenElements(t *testing.T) {
	ops := svgOps(t, `<svg viewBox="0 0 10 10">
		<rect width="1" height="1" display="none"/>
		<g visibility="hidden"><rect width="1" height="1"/><rect width="2" height="2" visibility="visible"/></g>
		<rect width="1" height="1" opacity="0"/>
		<text>Text is not drawn</text>
	</svg>`)
	if rects := kindsOf(ops, DrawRect); len(rects) != 1 || rects[0].W != 2 {
		t.Errorf("drawn = %+v", ops)
	}
}

func TestSVGSizes(t *testing.T) {
	for doc, want := range map[string][2]Points{
		`<svg width="10" viewBox="0 0 4 2"/>`:           {10, 5},
		`<svg height="10" viewBox="0 0 4 2"/>`:          {20, 10},
		`<svg width="50%" viewBox="0 0 20 10"/>`:        {10, 5},
		`<svg width="1in" height="2.54cm"/>`:            {96, 96},
		`<svg width="0" height="0" viewBox="0 0 3 4"/>`: {3, 4},
	} {
		s, err := ParseSVG([]byte(doc))
		if err != nil {
			t.Errorf("%s: %v", doc, err)
			continue
		}
		if w, h := s.Size(); w != want[0] || h != want[1] {
			t.Errorf("%s: size %gx%g, want %gx%g", doc, w, h, want[0], want[1])
		}
	}
}

func TestSVGExporterDocuments(t *testing.T) {
	// Illustrator declares its namespaces as entities, and older files
	// encodings other than UTF-8.
	ops := svgOps(t, `<?xml version="1.0" encoding="iso-8859-1"?>
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd" [
	<!ENTITY ns_svg "http://www.w3.org/2000/svg">
	<!ENTITY ns_xlink "http://www.w3.org/1999/xlink">
]>
<svg version="1.1" xmlns="&ns_svg;" xmlns:xlink="&ns_xlink;" xmlns:i="http://ns.adobe.com/AdobeIllustrator/10.0/" viewBox="0 0 10 10">
	<i:pgf>ignored</i:pgf>
	<defs><rect id="r" width="2" height="2"/></defs>
	<use xlink:href="#r" fill="#f00"/>
</svg>`)
	if got := kindsOf(ops, DrawFillColor); len(got) != 1 || got[0].Color != RGB(0xff0000) {
		t.Errorf("fills = %+v", ops)
	}
}

func mustParse(t *testing.T, data []byte) *SVG {
	t.Helper()
	s, err := ParseSVG(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSVGDashes(t *testing.T) {
	s, err := ParseSVG([]byte(`<svg viewBox="0 0 10 10"><g transform="scale(2)"><path d="M0 0L5 0" stroke="red" stroke-width="1" stroke-dasharray="1,2 3" stroke-dashoffset="4"/></g></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	ops := kindsOf(s.ops, DrawLineDash)
	if len(ops) != 2 || ops[0].Dash == nil || !slices.Equal(*ops[0].Dash, LineDash{2, 4, 6, 2, 4, 6}) || ops[1].Dash != nil {
		t.Errorf("dashes %+v", ops)
	}
	if off := kindsOf(s.ops, DrawLineDashOffset); len(off) != 2 || off[0].W != 8 || off[1].W != 0 {
		t.Errorf("offsets %+v", off)
	}
}
