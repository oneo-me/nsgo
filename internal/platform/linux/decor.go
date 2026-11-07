//go:build linux && !android

package linux

import (
	"math"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/driver"
)

// GSK draws what a style needs past a rounded box: corners of their own
// radii in its rounded rectangles; continuous corners, dashes and outlines
// as paths it fills and strokes; layered and inset shadows, gradients,
// color matrices, blurs, drop shadows and masks as render nodes. It has
// no node that reads what lies behind a widget, so backdrop filters and
// blend modes draw nothing here.

// gskColorStop is GskColorStop.
type gskColorStop struct {
	offset float32
	color  gdkRGBA
}

var (
	gtkSnapshotAppendInset     func(uintptr, *gskRoundedRect, *gdkRGBA, float32, float32, float32, float32)
	gtkSnapshotAppendLinear    func(uintptr, *grapheneRect, *graphenePoint, *graphenePoint, *gskColorStop, uintptr)
	gtkSnapshotAppendRadial    func(uintptr, *grapheneRect, *graphenePoint, float32, float32, float32, float32, *gskColorStop, uintptr)
	gtkSnapshotAppendConic     func(uintptr, *grapheneRect, *graphenePoint, float32, *gskColorStop, uintptr)
	gtkSnapshotPushColorMatrix func(uintptr, unsafe.Pointer, unsafe.Pointer)
	gtkSnapshotPushMask        func(uintptr, int32)
	gtkSnapshotPushFill        func(uintptr, uintptr, int32)
	gtkSnapshotAppendFill      func(uintptr, uintptr, int32, *gdkRGBA)
	gtkSnapshotAppendStroke    func(uintptr, uintptr, uintptr, *gdkRGBA)
	gskPathBuilderNew          func() uintptr
	gskPathBuilderMoveTo       func(uintptr, float32, float32)
	gskPathBuilderLineTo       func(uintptr, float32, float32)
	gskPathBuilderCubicTo      func(uintptr, float32, float32, float32, float32, float32, float32)
	gskPathBuilderClose        func(uintptr)
	gskPathBuilderFreeToPath   func(uintptr) uintptr
	gskPathUnref               func(uintptr)
	gskStrokeNew               func(float32) uintptr
	gskStrokeSetDash           func(uintptr, *float32, uintptr)
	gskStrokeSetLineCap        func(uintptr, int32)
	gskStrokeFree              func(uintptr)
	gskTransformMatrix         func(uintptr, unsafe.Pointer) uintptr
	gskTransformRotate         func(uintptr, float32) uintptr
	gskTransformSkew           func(uintptr, float32, float32) uintptr
	gskTransformScale          func(uintptr, float32, float32) uintptr
)

const (
	maskAlpha    = 0 // GSK_MASK_MODE_ALPHA
	fillWinding  = 0 // GSK_FILL_RULE_WINDING
	fillEvenOdd  = 1
	lineCapRound = 1 // GSK_LINE_CAP_ROUND
	lineCapButt  = 0
)

func bindDecor() {
	gtk := func(fn any, name string) { purego.RegisterLibFunc(fn, gtkLib, name) }
	gtk(&gtkSnapshotAppendInset, "gtk_snapshot_append_inset_shadow")
	gtk(&gtkSnapshotAppendLinear, "gtk_snapshot_append_linear_gradient")
	gtk(&gtkSnapshotAppendRadial, "gtk_snapshot_append_radial_gradient")
	gtk(&gtkSnapshotAppendConic, "gtk_snapshot_append_conic_gradient")
	gtk(&gtkSnapshotPushColorMatrix, "gtk_snapshot_push_color_matrix")
	gtk(&gtkSnapshotPushMask, "gtk_snapshot_push_mask")
	gtk(&gtkSnapshotPushFill, "gtk_snapshot_push_fill")
	gtk(&gtkSnapshotAppendFill, "gtk_snapshot_append_fill")
	gtk(&gtkSnapshotAppendStroke, "gtk_snapshot_append_stroke")
	gtk(&gskPathBuilderNew, "gsk_path_builder_new")
	gtk(&gskPathBuilderMoveTo, "gsk_path_builder_move_to")
	gtk(&gskPathBuilderLineTo, "gsk_path_builder_line_to")
	gtk(&gskPathBuilderCubicTo, "gsk_path_builder_cubic_to")
	gtk(&gskPathBuilderClose, "gsk_path_builder_close")
	gtk(&gskPathBuilderFreeToPath, "gsk_path_builder_free_to_path")
	gtk(&gskPathUnref, "gsk_path_unref")
	gtk(&gskStrokeNew, "gsk_stroke_new")
	gtk(&gskStrokeSetDash, "gsk_stroke_set_dash")
	gtk(&gskStrokeSetLineCap, "gsk_stroke_set_line_cap")
	gtk(&gskStrokeFree, "gsk_stroke_free")
	gtk(&gskTransformMatrix, "gsk_transform_matrix")
	gtk(&gskTransformRotate, "gsk_transform_rotate")
	gtk(&gskTransformSkew, "gsk_transform_skew")
	gtk(&gskTransformScale, "gsk_transform_scale")
}

// aligned16 returns n float32s at a 16-byte boundary, as graphene's SIMD
// matrices and vectors are laid out.
func aligned16(n int) []float32 {
	buf := make([]float32, n+4)
	off := int((16 - uintptr(unsafe.Pointer(&buf[0]))%16) % 16 / 4)
	return buf[off : off+n]
}

// gskMatrixTransform returns a new GskTransform of m after next. A flat
// m is GSK's own translation, rotation, skew and scale, which keep it a
// 2D transform every renderer draws, Cairo's too; only depth takes a
// matrix.
func gskMatrixTransform(next uintptr, m driver.Matrix3D) uintptr {
	if a, b, c, d, e, f, ok := m.Affine(); ok {
		t := gskTransformTranslate(next, &graphenePoint{float32(e), float32(f)})
		if a == 1 && b == 0 && c == 0 && d == 1 {
			return t
		}
		// The rotation of the first column, then what is left of the
		// columns: a skew and a scale.
		sx := math.Hypot(a, b)
		if sx == 0 {
			return gskTransformScale(t, 0, 0)
		}
		angle := math.Atan2(b, a)
		k := (a*c + b*d) / sx
		sy := (a*d - b*c) / sx
		skew := 0.0
		if sy != 0 {
			skew = math.Atan(k/sy) * 180 / math.Pi
		}
		t = gskTransformRotate(t, float32(angle*180/math.Pi))
		if skew != 0 {
			t = gskTransformSkew(t, float32(skew), 0)
		}
		return gskTransformScale(t, float32(sx), float32(sy))
	}
	// graphene multiplies row vectors, so its rows are m's columns, the
	// order m keeps them in.
	f := aligned16(16)
	for i := range f {
		f[i] = float32(m[i])
	}
	return gskTransformMatrix(next, unsafe.Pointer(&f[0]))
}

// gskPath returns a new GskPath of segments; the caller unrefs it.
func gskPath(segments []driver.Segment) uintptr {
	b := gskPathBuilderNew()
	for _, s := range segments {
		p := s.P
		switch s.Kind {
		case driver.SegmentMove:
			gskPathBuilderMoveTo(b, float32(p[0][0]), float32(p[0][1]))
		case driver.SegmentLine:
			gskPathBuilderLineTo(b, float32(p[0][0]), float32(p[0][1]))
		case driver.SegmentCubic:
			gskPathBuilderCubicTo(b, float32(p[0][0]), float32(p[0][1]), float32(p[1][0]), float32(p[1][1]),
				float32(p[2][0]), float32(p[2][1]))
		case driver.SegmentClose:
			gskPathBuilderClose(b)
		}
	}
	return gskPathBuilderFreeToPath(b)
}

// shape is a view's box: a rounded rectangle GSK draws natively, or a
// path for continuous corners.
type shape struct {
	w, h       float32
	radii      driver.Corners
	continuous bool
	rect       gskRoundedRect
}

func newShape(s *driver.ViewStyle, w, h float32) shape {
	radii := s.Radii(driver.Size{Width: driver.Points(w), Height: driver.Points(h)})
	return shape{w: w, h: h, radii: radii, continuous: s.ContinuousCorners && s.Rounded(),
		rect: cornersRect(0, 0, w, h, radii)}
}

// cornersRect is a rounded rectangle with fitted corners.
func cornersRect(x, y, w, h float32, c driver.Corners) gskRoundedRect {
	r := c.Array()
	var corner [4]grapheneSize
	for i := range r {
		corner[i] = grapheneSize{float32(r[i]), float32(r[i])}
	}
	return gskRoundedRect{bounds: grapheneRect{x, y, w, h}, corner: corner}
}

// path returns the box inset by d (outset when negative) as a new path.
func (sh *shape) path(d float32) uintptr {
	dd := float64(d)
	return gskPath(driver.RoundedRect(nil, dd, dd, float64(sh.w)-2*dd, float64(sh.h)-2*dd,
		sh.radii.Inset(driver.Points(d)), sh.continuous, false))
}

// pushClip clips what follows to the box, until a pop.
func (sh *shape) pushClip(snap uintptr) {
	if !sh.continuous {
		gtkSnapshotPushRoundedClip(snap, &sh.rect)
		return
	}
	p := sh.path(0)
	gtkSnapshotPushFill(snap, p, fillWinding)
	gskPathUnref(p)
}

// fill paints c inside the box.
func (sh *shape) fill(snap uintptr, c driver.Color) {
	if !sh.continuous {
		fillRounded(snap, &sh.rect, c)
		return
	}
	p := sh.path(0)
	color := rgba(c)
	gtkSnapshotAppendFill(snap, p, fillWinding, &color)
	gskPathUnref(p)
}

// fillRim paints c inside the box inset by d, under its border.
func (sh *shape) fillRim(snap uintptr, c driver.Color, d driver.Points) {
	p := sh.path(float32(d))
	color := rgba(c)
	gtkSnapshotAppendFill(snap, p, fillWinding, &color)
	gskPathUnref(p)
}

// stroke draws a line of width in c along the box inset by inset, dashed
// as style says.
func (sh *shape) stroke(snap uintptr, inset, width driver.Points, c driver.Color, style driver.BorderStyle) {
	p := sh.path(float32(inset))
	stroke := gskStrokeNew(float32(width))
	if on, off, round, ok := style.Dashes(width); ok {
		dash := []float32{float32(on), float32(off)}
		gskStrokeSetDash(stroke, &dash[0], 2)
		if round {
			gskStrokeSetLineCap(stroke, lineCapRound)
		}
	}
	color := rgba(c)
	gtkSnapshotAppendStroke(snap, p, stroke, &color)
	gskStrokeFree(stroke)
	gskPathUnref(p)
}

// colorStops returns GSK's stops of p.
func colorStops(p *driver.GradientPaint) []gskColorStop {
	out := make([]gskColorStop, len(p.Stops))
	for i, s := range p.Stops {
		out[i] = gskColorStop{s.Offset, rgba(s.Color)}
	}
	return out
}

// appendGradient paints p over a w by h box.
func appendGradient(snap uintptr, p *driver.GradientPaint, w, h float32) {
	stops := colorStops(p)
	if len(stops) < 2 {
		return
	}
	box := grapheneRect{0, 0, w, h}
	switch p.Kind {
	case driver.GradientRadial:
		center := graphenePoint{p.CX * w, p.CY * h}
		gtkSnapshotAppendRadial(snap, &box, &center, max(p.RX*w, 1e-3), max(p.RY*h, 1e-3), 0, 1, &stops[0], uintptr(len(stops)))
	case driver.GradientConic:
		center := graphenePoint{p.CX * w, p.CY * h}
		gtkSnapshotAppendConic(snap, &box, &center, p.Angle, &stops[0], uintptr(len(stops)))
	default:
		start, end := graphenePoint{p.X0 * w, p.Y0 * h}, graphenePoint{p.X1 * w, p.Y1 * h}
		gtkSnapshotAppendLinear(snap, &box, &start, &end, &stops[0], uintptr(len(stops)))
	}
}

// pushFilters wraps what follows in the filters, the first applied
// first, and returns how many to pop.
func pushFilters(snap uintptr, filters []driver.Filter) int {
	for i := len(filters) - 1; i >= 0; i-- {
		f := filters[i]
		switch f.Kind {
		case driver.FilterBlur:
			// GSK's radius is twice CSS's standard deviation.
			gtkSnapshotPushBlur(snap, float64(2*f.Amount))
		case driver.FilterDropShadow:
			sh := gskShadow{rgba(f.Shadow.Color), float32(f.Shadow.Offset.X), float32(f.Shadow.Offset.Y), float32(f.Shadow.Blur)}
			gtkSnapshotPushShadow(snap, &sh, 1)
		default:
			m, _ := f.Matrix()
			// graphene multiplies row vectors: row i holds what input i
			// gives each output.
			mat, offset := aligned16(16), aligned16(4)
			for in := range 4 {
				for out := range 4 {
					mat[in*4+out] = m[out*5+in]
				}
				offset[in] = m[in*5+4]
			}
			gtkSnapshotPushColorMatrix(snap, unsafe.Pointer(&mat[0]), unsafe.Pointer(&offset[0]))
		}
	}
	return len(filters)
}

// keyframe returns the step of v's keyframe animation now, and whether
// one runs.
func (v *view) keyframe() (driver.Keyframe, bool) {
	a := v.style.Animation
	if a.Keyframes == 0 {
		return driver.Keyframe{Opacity: 1, Scale: 1}, false
	}
	k, _ := a.At(time.Since(v.keyframeStart))
	return k, true
}

// placement returns the transform v's widget is allocated with in its
// parent: its frame's place less the parent's scroll offset ox, oy, the
// parent's perspective, its keyframes, then its transform with a layout
// animation's scale.
func (w *window) placement(v *view, ox, oy float32) driver.Matrix3D {
	f := v.shownFrame()
	at := driver.Point{X: f.Origin.X - driver.Points(ox), Y: f.Origin.Y - driver.Points(oy)}
	m := driver.Translate3D(float64(at.X), float64(at.Y), 0)
	if p := v.parent; p != nil {
		if d := w.styleOf(p).ChildPerspective; d > 0 {
			m = m.Mul(driver.ChildPerspective(d, p.shownFrame().Size, at))
		}
	}
	if k, ok := v.keyframe(); ok {
		m = m.Mul(k.Matrix(f.Size))
	}
	vs := *w.styleOf(v)
	if ls := v.layoutScale(); ls != 1 {
		sx, sy := vs.Transform.Scales()
		vs.Transform.Scale, vs.Transform.ScaleX, vs.Transform.ScaleY = 0, sx*ls, sy*ls
	}
	return m.Mul(vs.Matrix(f.Size))
}

// facingAway reports whether m turns a box's back to the viewer.
func facingAway(m driver.Matrix3D) bool {
	x0, y0 := m.Apply(0, 0)
	x1, y1 := m.Apply(1, 0)
	x2, y2 := m.Apply(0, 1)
	return (x1-x0)*(y2-y0)-(y1-y0)*(x2-x0) < 0
}
