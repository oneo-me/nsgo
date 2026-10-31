//go:build darwin && !ios

package macos

import (
	"math"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// Text and canvas views draw into a content layer of their own: a plain
// CALayer whose delegate draws in drawLayer:inContext:, below the view's
// sublayers as a backing store's contents would be. No view implements
// drawRect:, so AppKit gives none a backing store and never asks one to
// draw. AppKit's display pass recorded every drawRect: into a display list
// and replayed it, and scrolling a clip view marked every drawRect: view
// of its document for display. A content layer draws again only when its
// content or its size changes, and only while some of it can be seen.

var (
	// contentDrawer is the delegate of every content layer, which does
	// not retain it.
	contentDrawer objc.ID
	// contentActions turns off implicit animations of content layers.
	contentActions objc.ID
	// drawers maps content layers to their views.
	drawers = map[objc.ID]*view{}

	selContentsAreFlipped       = sel("contentsAreFlipped")
	selSetContentsScale         = sel("setContentsScale:")
	selSetNeedsDisplayLayer     = sel("setNeedsDisplay")
	selSetNeedsDisplayInRect    = sel("setNeedsDisplayInRect:")
	selSetNeedsDisplayOnResize  = sel("setNeedsDisplayOnBoundsChange:")
	selSetZPosition, selActions = sel("setZPosition:"), sel("setActions:")
)

// contentZ puts a content layer below the view's other sublayers (its
// children and its border rings), where a backing store's contents draw.
const contentZ = -1

func registerContent() {
	drawer := register("NsgoContentDrawer", objc.GetClass("NSObject"), nil, []objc.MethodDef{{
		Cmd: sel("drawLayer:inContext:"),
		Fn:  drawContent,
	}})
	contentDrawer = objc.ID(drawer).Send(selNew)
	null := objc.ID(objc.GetClass("NSNull")).Send(sel("null"))
	keys := []string{"contents", "bounds", "position", "frame", "hidden", "contentsScale",
		"onOrderIn", "onOrderOut", "sublayers"}
	dict := objc.ID(objc.GetClass("NSMutableDictionary")).Send(selNew)
	for _, k := range keys {
		dict.Send(sel("setObject:forKey:"), null, apple.NSString(k))
	}
	contentActions = dict
}

// drawContent draws a view's text or canvas into its content layer, top
// down as a flipped view's drawRect: did.
func drawContent(_ objc.ID, _ objc.SEL, layer objc.ID, ctx uintptr) {
	v := drawers[layer]
	if v == nil || ctx == 0 || (v.layout == nil && v.drawing == nil) {
		return
	}
	// The layer holds the view and the ink of its text past it
	// (sizeContent).
	h, f := float64(v.frame.Size.Height), v.contentFrame()
	apple.DrawTopDown(ctx, f.Size.Height, !apple.MsgBool(layer, selContentsAreFlipped), func(baseUp bool) {
		apple.TranslateCTM(ctx, -f.Origin.X, -f.Origin.Y)
		if v.drawing != nil {
			v.paintCanvas(ctx, layer, baseUp)
			return
		}
		box := v.box
		v.layout.DrawBackgrounds(ctx, apple.Rect(float64(box.Origin.X), float64(box.Origin.Y), 0, 0))
		v.drawSelection(ctx)
		v.layout.Draw(ctx, h, apple.Rect(float64(box.Origin.X), float64(box.Origin.Y),
			float64(box.Size.Width), float64(box.Size.Height)), baseUp)
		if v.animated.Has(driver.PropTextColor) {
			// A bound color paints over the whole text.
			apple.Tint(ctx, f, v.animated.TextColor)
		}
	})
}

// redraw draws v's content again: now if some of v can be seen, else
// once it shows (surface.revisit). Lists keep rows out of sight, and
// hidden screens stay mounted.
func (v *view) redraw() {
	if v.contentLayer == 0 && v.layout == nil && v.drawing == nil && v.shader == nil {
		return
	}
	if v.shown() {
		v.display()
		return
	}
	if !v.stale {
		v.stale = true
		v.surface.stale = append(v.surface.stale, v)
	}
}

// redrawAnimated draws again what a canvas's animated arguments changed,
// as NSH's partial redraws: the area they cover in the ops it last drew
// and in those it plays now (apple.Damaged), or all of it where that
// cannot tell. Core Animation clips the replay of the whole drawing to
// the area, so the pixels are a full redraw's.
func (v *view) redrawAnimated() {
	if v.drawing == nil || len(v.played) == 0 || !v.shown() {
		v.redraw()
		return
	}
	area, whole := apple.Damaged(v.played, v.surface.animator.CanvasOps(v.id, v.drawing))
	switch {
	case whole:
		v.display()
	case area.Size.Width > 0:
		v.drawLayer().Send(selSetNeedsDisplayInRect, area)
	}
}

// drawLayer returns v's content layer, made on first use at v's size.
func (v *view) drawLayer() objc.ID {
	if v.contentLayer != 0 {
		return v.contentLayer
	}
	layer := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	layer.Send(selActions, contentActions)
	layer.Send(selSetDelegate, contentDrawer)
	layer.Send(selSetNeedsDisplayOnResize, true)
	layer.Send(selSetContentsScale, float64(v.surface.env.PixelRatio))
	layer.Send(selSetZPosition, apple.Z(contentZ))
	layer.Send(selSetFrame, v.contentFrame())
	v.native.Send(selLayer).Send(sel("addSublayer:"), layer)
	layer.Send(selRelease) // its host keeps it
	v.contentLayer = layer
	drawers[layer] = v
	return layer
}

// shown reports whether some of v can be seen: in the tree, neither it
// nor an ancestor hidden, and not outside what the scroll views around it
// show. It reads the batches' state, as NSH's does, rather than ask
// AppKit for a window, hidden ancestors and a visible CGRect for every
// text a frame changes. Transforms are left out: one at most draws a view
// early or late.
func (v *view) shown() bool {
	s := v.surface
	// r is v's placed frame in the space of n's parent's content.
	r := v.frame
	r.Origin.X += v.shift.X
	r.Origin.Y += v.shift.Y
	for n := v; ; n = n.up {
		if n == nil || s.views[n.id] != n || n.style.Hidden {
			return false
		}
		if n.id == s.root {
			return true
		}
		p := n.up
		if p == nil {
			return false
		}
		if p.sc != nil {
			// The content shows from the offset, as large as the view.
			o, size := p.sc.at, p.frame.Size
			x0, y0 := max(r.Origin.X, o.X), max(r.Origin.Y, o.Y)
			x1 := min(r.Origin.X+r.Size.Width, o.X+size.Width)
			y1 := min(r.Origin.Y+r.Size.Height, o.Y+size.Height)
			if x1 <= x0 || y1 <= y0 {
				return false
			}
			r = driver.Bounds{Origin: driver.Point{X: x0 - o.X, Y: y0 - o.Y},
				Size: driver.Size{Width: x1 - x0, Height: y1 - y0}}
		}
		r.Origin.X += p.frame.Origin.X + p.shift.X
		r.Origin.Y += p.frame.Origin.Y + p.shift.Y
	}
}

// revisit draws the views whose content changed out of sight and that
// show now: after batches, scrolls and layout animations. Deleted views
// are dropped; views still out of sight wait.
func (s *surface) revisit() {
	if len(s.stale) == 0 {
		return
	}
	waiting := s.stale[:0]
	for _, v := range s.stale {
		if s.views[v.id] != v {
			v.stale = false
			continue
		}
		if !v.shown() {
			waiting = append(waiting, v)
			continue
		}
		v.stale = false
		v.display()
	}
	clear(s.stale[len(waiting):])
	s.stale = waiting
}

// display draws v's content, which can be seen: a content layer when Core
// Animation next displays, a shader once the batch or the frame is
// applied.
func (v *view) display() {
	if v.shader != nil {
		v.surface.shadeLater(v)
		return
	}
	apple.Msg(v.drawLayer(), selSetNeedsDisplayLayer)
}

// sizeContent keeps v's content layer over its bounds; a new size draws
// it again (needsDisplayOnBoundsChange).
func (v *view) sizeContent() {
	if v.shader != nil {
		v.sizeShader()
	}
	if v.contentLayer == 0 {
		return
	}
	v.contentLayer.Send(selSetFrame, v.contentFrame())
}

// contentFrame is the frame of v's content layer: its bounds, grown to
// hold its text's ink past them.
func (v *view) contentFrame() apple.CGRect {
	w, h := float64(v.frame.Size.Width), float64(v.frame.Size.Height)
	if v.layout == nil {
		return apple.Rect(0, 0, w, h)
	}
	b := v.box
	return v.layout.ContentFrame(w, h, apple.Rect(float64(b.Origin.X), float64(b.Origin.Y), float64(b.Size.Width), float64(b.Size.Height)))
}

// rescale draws every content layer again at the window's backing scale.
func (s *surface) rescale() {
	for _, v := range s.views {
		if v != nil && v.shader != nil {
			v.sizeShader()
		}
		if v == nil || v.contentLayer == 0 {
			continue
		}
		v.contentLayer.Send(selSetContentsScale, float64(s.env.PixelRatio))
		v.redraw()
	}
}

// forgetContent drops v's content layer, which goes with its view.
func (v *view) forgetContent() {
	if v.contentLayer != 0 {
		delete(drawers, v.contentLayer)
	}
	v.canvas.set(nil)
}

// A canvas draws at once on the main thread, as NSH's does, unless it
// paints often (frequentPaints a second or more: an animation, live
// data) and keeps no layer, when Core Animation replays it on its
// threads (drawsAsynchronously) and the main thread stays free. Drawing
// there holds graphics memory while any canvas does (tens of MB for the
// process) and Metal work per layer, which a canvas redrawn now and then
// gains nothing from. A drawing painted unchanged stablePaints times
// whose static part (driver.CanvasSplit) is long keeps that part as a
// CGLayer, and each paint draws the layer and plays only the ops after
// it: a frame of a pulsing dot over a chart plays the dot, not the
// chart. Short or changing drawings replay whole: drawing a layer costs
// about what playing a sparkline does, and making one runs on the main
// thread.
const (
	frequentPaints = 10
	stablePaints   = 30
	minStaticOps   = 128
)

// canvasPaint is how a canvas view paints: its static part, the layer
// kept of it, and how often it paints.
type canvasPaint struct {
	split   driver.CanvasSplit // At is 0 when no part is kept
	layer   *apple.CanvasLayer
	painted int // paints of the drawing without a layer
	async   bool

	// The paints in the second that began at since, and whether that
	// second or the one before it had frequentPaints.
	since    time.Time
	paints   int
	frequent bool
}

// set takes a new drawing, nil when the view goes.
func (c *canvasPaint) set(d *driver.Drawing) {
	c.layer.Release()
	c.layer, c.painted, c.split = nil, 0, driver.CanvasSplit{}
	if d == nil {
		return
	}
	// Short static parts play faster than a layer draws.
	if s := driver.SplitCanvas(d); s.At >= minStaticOps && s.Static(d) {
		c.split = s
	}
}

// count counts a paint at now.
func (c *canvasPaint) count(now time.Time) {
	if !c.since.IsZero() && now.Sub(c.since) < time.Second {
		c.paints++
		if c.paints >= frequentPaints {
			c.frequent = true
		}
		return
	}
	// A new second: the last one was slow, or a quiet second came
	// between.
	if !c.since.IsZero() && (c.paints < frequentPaints || now.Sub(c.since) >= 2*time.Second) {
		c.frequent = false
	}
	c.since, c.paints = now, 1
}

// keeps reports whether the static part is or is about to be kept as a
// layer; such a canvas draws at once, where its layer is made.
func (c *canvasPaint) keeps() bool { return c.split.At > 0 && c.painted >= stablePaints }

// paintCanvas plays v's drawing into ctx, its content layer's top-down
// context, over the layer of its static part where it keeps one, and
// sets where the next paint happens.
func (v *view) paintCanvas(ctx uintptr, layer objc.ID, baseUp bool) {
	c := &v.canvas
	c.count(time.Now())
	ops := v.surface.animator.CanvasOps(v.id, v.drawing)
	host := apple.CanvasHost{Ratio: float64(v.surface.env.PixelRatio), Redraw: v.redraw, Owner: v, BaseUp: baseUp}
	w, h := float64(v.frame.Size.Width), float64(v.frame.Size.Height)
	if kept := v.staticLayer(ctx, ops, w, h, host); kept != nil {
		kept.Draw(ctx, w, h)
		apple.PlayOver(ctx, ops, c.split, host)
	} else {
		apple.Play(ctx, ops, host)
	}
	v.played = append(v.played[:0], ops...)
	if async := c.frequent && !c.keeps(); async != c.async {
		c.async = async
		layer.Send(sel("setDrawsAsynchronously:"), async)
	}
}

// staticLayer returns the layer of v's static part at its size, made
// once the drawing was painted stablePaints times, or nil.
func (v *view) staticLayer(ctx uintptr, ops []driver.DrawOp, w, h float64, host apple.CanvasHost) *apple.CanvasLayer {
	c := &v.canvas
	if c.split.At == 0 || c.split.At >= len(ops) {
		return nil
	}
	scale := float64(v.surface.env.PixelRatio)
	pw, ph := int(math.Ceil(w*scale)), int(math.Ceil(h*scale))
	if l := c.layer; l != nil && l.Drawing == v.drawing && l.Width == pw && l.Height == ph {
		return l
	}
	c.layer.Release()
	c.layer = nil
	c.painted++
	// A layer is made for the context it draws into: once the canvas
	// draws at once.
	if c.painted <= stablePaints || c.async {
		return nil
	}
	c.layer = apple.NewCanvasLayer(ctx, v.drawing, ops, c.split, w, h, scale, host)
	return c.layer
}
