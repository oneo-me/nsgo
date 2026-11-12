//go:build linux && !android

package linux

import (
	"math"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A canvas is rasterized with Cairo into an image surface at the
// surface's scale and shown as textures of tile pixels a side, as NSH's
// Linux canvases are (canvasRaster): a frame in which the canvas did not
// change appends the same textures, which GSK keeps on the GPU, and only
// a new drawing, size or scale plays it whole. A change of its bound
// arguments draws again only what driver.CanvasDamage says they cover:
// the area is cleared and the drawing played clipped to it, and only the
// tiles it touches become new textures. What the drawing's static part
// (driver.CanvasSplit) painted is kept from the last full play and
// copied back into the area, so only the painting after it plays again.
//
// A canvas out of sight, outside a clipping ancestor or the window, is
// not drawn: what changed waits, and it draws once a layout shows it
// (revealCanvases).

var (
	cairoSurfaceSetDeviceScale func(surface uintptr, x, y float64)
	gdkMemoryTextureNew        func(w, h, format int32, bytes, stride uintptr) uintptr
	gtkSnapshotAppendTexture   func(snap, texture uintptr, bounds *grapheneRect)
)

func bindRaster(cairoLib uintptr) {
	reg := purego.RegisterLibFunc
	reg(&cairoSurfaceSetDeviceScale, cairoLib, "cairo_surface_set_device_scale")
	reg(&gdkMemoryTextureNew, gtkLib, "gdk_memory_texture_new")
	reg(&gtkSnapshotAppendTexture, gtkLib, "gtk_snapshot_append_texture")
}

// tile is the side of a canvas's textures, in pixels.
const tile = 256

// canvasRaster is a canvas's pixels.
type canvasRaster struct {
	surface uintptr // ARGB32 at scale
	pw, ph  int
	scale   float64
	// tiles are the surface's textures, row by row.
	tiles []uintptr
	// static is what the ops before the split painted, or 0.
	static uintptr
}

// canvasDraw is what of a canvas must be drawn again.
type canvasDraw struct {
	// split is where its static part ends, At 0 when none is kept.
	split driver.CanvasSplit
	// last are the ops as its bound arguments last changed them, for the
	// damage of the next change.
	last []driver.DrawOp
	// whole or area, in points, must be drawn again.
	whole bool
	area  driver.Bounds
	// hidden is set while it waits out of sight with something to draw.
	hidden bool
	raster *canvasRaster
}

// setCanvas takes v's new drawing, drawn whole.
func (w *window) setCanvas(v *view, d *driver.Drawing) {
	v.drawing = d
	c := &v.canvas
	c.split, c.last, c.whole, c.area = driver.CanvasSplit{}, c.last[:0], true, driver.Bounds{}
	if s := driver.SplitCanvas(d); s.Static(d) {
		c.split = s
	}
	w.animator.BindCanvas(v.id, d.Slots)
	w.redrawCanvas(v)
}

// redrawCanvas has v drawn again unless it waits out of sight.
func (w *window) redrawCanvas(v *view) {
	if !v.canvas.hidden {
		w.changed(v)
	}
}

// changedCanvases draws again what the canvases' bound arguments
// changed.
func (w *window) changedCanvases() {
	w.animator.ChangedCanvases(func(id driver.NodeID) {
		v := w.view(id)
		if v == nil || v.drawing == nil {
			return
		}
		c := &v.canvas
		ops := w.animator.CanvasOps(v.id, v.drawing)
		if !c.whole {
			if len(c.last) != len(ops) {
				c.whole = true
			} else if area, ok := driver.CanvasDamage(c.last, ops); !ok {
				c.whole = true
			} else if area.Size.Width > 0 && area.Size.Height > 0 {
				c.area = union(c.area, area)
			} else {
				c.last = append(c.last[:0], ops...)
				return
			}
		}
		c.last = append(c.last[:0], ops...)
		w.redrawCanvas(v)
	})
}

// union is the bounds covering a and b, either of which may be empty.
func union(a, b driver.Bounds) driver.Bounds {
	if a.Size.Width <= 0 || a.Size.Height <= 0 {
		return b
	}
	x0, y0 := min(a.Origin.X, b.Origin.X), min(a.Origin.Y, b.Origin.Y)
	x1 := max(a.Origin.X+a.Size.Width, b.Origin.X+b.Size.Width)
	y1 := max(a.Origin.Y+a.Size.Height, b.Origin.Y+b.Size.Height)
	return driver.Bounds{Origin: driver.Point{X: x0, Y: y0}, Size: driver.Size{Width: x1 - x0, Height: y1 - y0}}
}

// snapshotCanvas appends v's raster, drawn again where it changed,
// unless v is out of sight.
func (w *window) snapshotCanvas(v *view, snap uintptr, width, height float64) {
	c := &v.canvas
	if len(v.drawing.Ops) == 0 {
		return
	}
	scale := max(float64(w.env.PixelRatio), 1)
	pw, ph := int(math.Ceil(width*scale)), int(math.Ceil(height*scale))
	r := c.raster
	fits := r != nil && r.scale == scale && r.pw == pw && r.ph == ph
	dirty := c.whole || c.area.Size.Width > 0
	if (!fits || dirty) && !w.inSight(v) {
		if !c.hidden {
			c.hidden = true
			w.hiddenCanvases = append(w.hiddenCanvases, v)
		}
		return
	}
	if !fits || dirty {
		if !fits {
			c.whole = true
		}
		w.rasterize(v, pw, ph, scale)
		c.whole, c.area = false, driver.Bounds{}
		if len(c.last) == 0 {
			c.last = append(c.last, w.animator.CanvasOps(v.id, v.drawing)...)
		}
	}
	r = c.raster
	if r == nil {
		return
	}
	i := 0
	for y := 0; y < ph; y += tile {
		for x := 0; x < pw; x += tile {
			tw, th := min(tile, pw-x), min(tile, ph-y)
			at := grapheneRect{float32(float64(x) / scale), float32(float64(y) / scale),
				float32(float64(tw) / scale), float32(float64(th) / scale)}
			gtkSnapshotAppendTexture(snap, r.tiles[i], &at)
			i++
		}
	}
}

// rasterize plays v's drawing into its raster where it must be drawn
// again, into a new one at pw × ph pixels when the size or scale
// changed, and makes the textures of the tiles it touched.
func (w *window) rasterize(v *view, pw, ph int, scale float64) {
	c := &v.canvas
	r := c.raster
	if r == nil || r.scale != scale || r.pw != pw || r.ph != ph {
		r.release()
		surf := cairoImageSurfaceCreate(0, int32(pw), int32(ph))
		if surf == 0 {
			c.raster = nil
			return
		}
		cairoSurfaceSetDeviceScale(surf, scale, scale)
		r = &canvasRaster{surface: surf, pw: pw, ph: ph, scale: scale}
		c.raster = r
		c.whole = true
	}
	cr := cairoCreate(r.surface)
	split := c.split
	// The area drawn again, in whole pixels.
	x0, y0, x1, y1 := 0, 0, pw, ph
	over := false
	if c.whole {
		if split.At == 0 && r.static != 0 {
			cairoSurfaceDestroy(r.static)
			r.static = 0
		}
		cairoSetOperator(cr, operatorClear)
		cairoPaint(cr)
		cairoSetOperator(cr, operatorSourceOver)
	} else {
		const aa = 2
		a := c.area
		px := func(v float64, round func(float64) float64, hi int) int {
			return min(max(int(round(v*scale)), 0), hi)
		}
		x0, y0 = px(math.Floor(float64(a.Origin.X)-aa), math.Floor, pw), px(math.Floor(float64(a.Origin.Y)-aa), math.Floor, ph)
		x1 = px(math.Ceil(float64(a.Origin.X+a.Size.Width)+aa), math.Ceil, pw)
		y1 = px(math.Ceil(float64(a.Origin.Y+a.Size.Height)+aa), math.Ceil, ph)
		cairoRectangle(cr, float64(x0)/scale, float64(y0)/scale, float64(x1-x0)/scale, float64(y1-y0)/scale)
		cairoClip(cr)
		if r.static != 0 {
			cairoSetSourceSurface(cr, r.static, 0, 0)
			cairoSetOperator(cr, operatorSource)
			over = true
		} else {
			cairoSetOperator(cr, operatorClear)
		}
		cairoPaint(cr)
		cairoSetOperator(cr, operatorSourceOver)
		cairoSetSourceRGBA(cr, 0, 0, 0, 0)
	}
	ops := w.animator.CanvasOps(v.id, v.drawing)
	size := rect{w: float64(pw) / scale, h: float64(ph) / scale}
	switch {
	case over:
		w.playCanvasFrom(v, cr, size, ops, split, nil)
	case c.whole && split.At > 0:
		// A full play keeps what the static part painted.
		w.playCanvasFrom(v, cr, size, ops, driver.CanvasSplit{}, func(at int) {
			if at == split.At {
				r.keepStatic()
			}
		})
	default:
		w.playCanvasFrom(v, cr, size, ops, driver.CanvasSplit{}, nil)
	}
	cairoDestroy(cr)
	cairoSurfaceFlush(r.surface)
	r.texture(x0, y0, x1, y1)
}

// keepStatic copies the surface's pixels into static.
func (r *canvasRaster) keepStatic() {
	cairoSurfaceFlush(r.surface)
	if r.static == 0 {
		r.static = cairoImageSurfaceCreate(0, int32(r.pw), int32(r.ph))
		if r.static == 0 {
			return
		}
		cairoSurfaceSetDeviceScale(r.static, r.scale, r.scale)
	}
	cr := cairoCreate(r.static)
	cairoSetSourceSurface(cr, r.surface, 0, 0)
	cairoSetOperator(cr, operatorSource)
	cairoPaint(cr)
	cairoDestroy(cr)
	cairoSurfaceFlush(r.static)
}

// texture makes the textures of the tiles that meet the pixels from
// (x0, y0) to (x1, y1), and of all of them the first time.
func (r *canvasRaster) texture(x0, y0, x1, y1 int) {
	data := cairoImageSurfaceGetData(r.surface)
	stride := int(cairoImageSurfaceGetStride(r.surface))
	if data == 0 {
		return
	}
	pix := unsafe.Slice((*byte)(cPointer(data)), stride*r.ph)
	fresh := len(r.tiles) == 0
	var row []byte
	i := 0
	for y := 0; y < r.ph; y += tile {
		for x := 0; x < r.pw; x += tile {
			tw, th := min(tile, r.pw-x), min(tile, r.ph-y)
			if fresh {
				r.tiles = append(r.tiles, 0)
			}
			if r.tiles[i] != 0 && (x >= x1 || x+tw <= x0 || y >= y1 || y+th <= y0) {
				i++
				continue
			}
			var bytes uintptr
			if x == 0 && tw*4 == stride {
				bytes = gBytesNew(unsafe.Pointer(&pix[y*stride]), uintptr(th*stride))
			} else {
				row = row[:0]
				for yy := y; yy < y+th; yy++ {
					row = append(row, pix[yy*stride+x*4:yy*stride+(x+tw)*4]...)
				}
				bytes = gBytesNew(unsafe.Pointer(&row[0]), uintptr(len(row)))
			}
			// Cairo's ARGB32 is premultiplied and native-endian: BGRA in
			// memory on little-endian machines.
			const bgraPremultiplied = 0
			t := gdkMemoryTextureNew(int32(tw), int32(th), bgraPremultiplied, bytes, uintptr(tw*4))
			gBytesUnref(bytes)
			if r.tiles[i] != 0 {
				gObjectUnref(r.tiles[i])
			}
			r.tiles[i] = t
			i++
		}
	}
}

// release lets r's surfaces and textures go; r may be nil.
func (r *canvasRaster) release() {
	if r == nil {
		return
	}
	for _, t := range r.tiles {
		if t != 0 {
			gObjectUnref(t)
		}
	}
	r.tiles = nil
	if r.static != 0 {
		cairoSurfaceDestroy(r.static)
		r.static = 0
	}
	if r.surface != 0 {
		cairoSurfaceDestroy(r.surface)
		r.surface = 0
	}
}

// inSight reports whether some of v may show: false only when it lies
// wholly outside a clipping ancestor or the window.
func (w *window) inSight(v *view) bool {
	if v.widget == 0 || gtkWidgetGetMapped(v.widget) == 0 {
		return false
	}
	for a := gtkWidgetGetParent(v.widget); a != 0; a = gtkWidgetGetParent(a) {
		ref, ok := lookupWidget(a)
		if !ok {
			continue
		}
		root := ref.v == nil // the surface: the window's bounds
		if !root && !clips(ref.v) {
			continue
		}
		var b grapheneRect
		if gtkWidgetComputeBounds(v.widget, a, &b) == 0 {
			return true
		}
		aw, ah := float32(gtkWidgetGetWidth(a)), float32(gtkWidgetGetHeight(a))
		if b.x >= aw || b.y >= ah || b.x+b.w <= 0 || b.y+b.h <= 0 {
			return false
		}
		if root {
			return true
		}
	}
	return true
}

// clips reports whether v clips its children, as snapshotView does.
func clips(v *view) bool {
	return v.style.Clip || v.kind == driver.KindScroll || v.kind == driver.KindImage || v.kind == driver.KindInput
}

// revealCanvases draws the canvases waiting out of sight that a layout
// showed; the others wait on.
func (w *window) revealCanvases() {
	if len(w.hiddenCanvases) == 0 {
		return
	}
	waiting := w.hiddenCanvases[:0]
	for _, v := range w.hiddenCanvases {
		switch {
		case v.deleted || v.drawing == nil:
			v.canvas.hidden = false
		case !w.inSight(v):
			waiting = append(waiting, v)
		default:
			v.canvas.hidden = false
			w.changed(v)
		}
	}
	clear(w.hiddenCanvases[len(waiting):])
	w.hiddenCanvases = waiting
}
