//go:build windows

package windows

import (
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A canvas plays its drawing into Direct2D command lists, as NSH's do,
// and a frame draws the lists: a drawing is played once, not on every
// frame the window draws. A drawing with bound arguments is two lists:
// its static part (driver.CanvasSplit), played once, and the rest,
// played again when an argument changes, which plays the static part's
// state (transforms, styles, the path, clips) without painting it. A
// rest that would combine with what the static part drew, under a
// composite operation other than source-over or with DrawClear, is
// played whole on each change instead. Command lists are vector: they
// stay sharp under any transform.
//
// Lists are played on a context of their own (gfx.record), so the frame's
// layers stay pushed while they are made.

// canvasCache is a canvas's lists.
type canvasCache struct {
	drawing *driver.Drawing
	split   driver.CanvasSplit // At 0 when the drawing is one list
	// head is the static part's list, or the whole drawing's; tail the
	// rest's, nil until played for the arguments' current values.
	head, tail *comObject
	scale      float32
	generation uint64
}

// setDrawing takes v's new drawing.
func (c *canvasCache) setDrawing(d *driver.Drawing) {
	c.release()
	c.drawing, c.split = d, driver.CanvasSplit{}
	if d == nil {
		return
	}
	if s := driver.SplitCanvas(d); s.Static(d) && !mixesAfter(d.Ops, s.At) {
		c.split = s
	}
}

// mixesAfter reports whether the ops from at on combine with what those
// before drew: a composite operation other than source-over set before
// at or from at on, or a DrawClear from at on.
func mixesAfter(ops []driver.DrawOp, at int) bool {
	for i, op := range ops {
		switch {
		case op.Kind == driver.DrawComposite && op.Composite != driver.CompositeSourceOver:
			return true
		case op.Kind == driver.DrawClear && i >= at:
			return true
		}
	}
	return false
}

// argsChanged drops the list the arguments' old values played.
func (c *canvasCache) argsChanged() {
	if c.split.At > 0 {
		releaseAll(&c.tail)
	} else {
		releaseAll(&c.head)
	}
}

func (c *canvasCache) release() { releaseAll(&c.head, &c.tail) }

// canvas draws v's drawing from its lists, played as needed; it plays
// the drawing into the frame where a list cannot be made.
func (p *painter) canvas(v *view, width, height float32) {
	c := &v.canvasCache
	if c.drawing != v.drawing {
		c.setDrawing(v.drawing)
	}
	if c.scale != p.scale || c.generation != gfxGeneration {
		c.release()
		c.scale, c.generation = p.scale, gfxGeneration
	}
	ops := p.w.animator.CanvasOps(v.id, v.drawing)
	if c.head == nil {
		if c.split.At > 0 {
			c.head = p.recordCanvas(v, ops[:c.split.At], driver.CanvasSplit{}, width, height)
		} else {
			c.head = p.recordCanvas(v, ops, driver.CanvasSplit{}, width, height)
		}
	}
	if c.split.At > 0 && c.tail == nil {
		c.tail = p.recordCanvas(v, ops, c.split, width, height)
	}
	if c.head == nil || c.split.At > 0 && c.tail == nil {
		p.playCanvas(v, ops, width, height)
		return
	}
	p.setTransform(p.m)
	for _, list := range []*comObject{c.head, c.tail} {
		if list != nil {
			p.dc.call(dcDrawImage, list.ptr(), 0, 0, d2dInterpolationLinear, 0)
		}
	}
}

// playCanvas plays ops into the frame.
func (p *painter) playCanvas(v *view, ops []driver.DrawOp, width, height float32) {
	mixes := false
	for i := range ops {
		if op := &ops[i]; op.Kind == driver.DrawClear || op.Kind == driver.DrawComposite && op.Composite != 0 {
			mixes = true
			break
		}
	}
	if mixes && p.canvasOffscreen(v, ops, width, height) {
		return
	}
	c := p.newCanvasPlayer(v, p.m, width, height)
	c.playAll(ops)
}

// recordCanvas plays ops into a new command list on the recording
// context, leaving out what split skips; nil when it cannot.
func (p *painter) recordCanvas(v *view, ops []driver.DrawOp, split driver.CanvasSplit, width, height float32) *comObject {
	rec := p.g.record
	var list *comObject
	if failed(rec.call(dcCreateCommandList, uintptr(unsafe.Pointer(&list)))) {
		return nil
	}
	rec.call(dcSetTarget, list.ptr())
	rec.callF(rtSetDpi, 0b11, f32(96*p.scale), f32(96*p.scale))
	rec.call(rtBeginDraw)
	// The brush is the device's, which any of its contexts draws with.
	r := &painter{w: p.w, g: p.g, dc: rec, scale: p.scale, m: identity, alpha: 1, brush: p.brush}
	r.setTransform(identity)
	mixes := false
	for i := range ops {
		if op := &ops[i]; !split.Skips(i, op) &&
			(op.Kind == driver.DrawClear || op.Kind == driver.DrawComposite && op.Composite != 0) {
			mixes = true
			break
		}
	}
	if !(mixes && split.At == 0 && r.canvasOffscreen(v, ops, width, height)) {
		c := r.newCanvasPlayer(v, identity, width, height)
		c.split = split
		c.playAll(ops)
	}
	for len(r.pushed) > 0 {
		r.pop()
	}
	hr := rec.call(rtEndDraw, 0, 0)
	rec.call(dcSetTarget, 0)
	if failed(hr) || failed(list.call(commandListClose)) {
		list.release()
		return nil
	}
	return list
}
