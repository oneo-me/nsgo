//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A canvas's named parts are UIAccessibilityElements the canvas holds as
// a container, at their bounds in it, with their traits, label,
// identifier and selected state. Activating one is a tap at its center,
// on the view under that point that takes presses.

var canvasNodeClass objc.Class

// axNodes maps the elements to their canvas and node.
var axNodes = map[objc.ID]axNode{}

type axNode struct {
	v    *view
	node driver.CanvasNode
}

func registerCanvasNodes() {
	canvasNodeClass = register("NsgoCanvasNode", apple.Class("UIAccessibilityElement"), nil, []objc.MethodDef{
		{Cmd: sel("accessibilityActivate"), Fn: canvasNodeActivate},
	})
}

// setCanvasNodes makes v's drawing's nodes its accessibility elements.
func (v *view) setCanvasNodes(nodes []driver.CanvasNode) {
	if len(nodes) == 0 && len(v.axNodes) == 0 {
		return
	}
	v.releaseCanvasNodes()
	for _, n := range nodes {
		el := objc.ID(canvasNodeClass).Send(sel("alloc")).Send(sel("initWithAccessibilityContainer:"), v.native)
		el.Send(selSetIsAccessibilityElement, true)
		el.Send(sel("setAccessibilityLabel:"), nsOrNil(n.Label))
		el.Send(selSetAccessibilityIdentifier, nsOrNil(n.TestID))
		var t uint64
		switch n.Role {
		case driver.RoleButton, driver.RoleCheckbox, driver.RoleSwitch, driver.RoleRadio, driver.RoleTab,
			driver.RoleMenuItem:
			t = traits["Button"]
		case driver.RoleLink:
			t = traits["Link"]
		case driver.RoleHeader:
			t = traits["Header"]
		case driver.RoleImage:
			t = traits["Image"]
		case driver.RoleText:
			t = traits["StaticText"]
		}
		if n.Selected {
			t |= traits["Selected"]
		}
		el.Send(selSetAccessibilityTraits, t)
		b := n.Bounds
		el.Send(sel("setAccessibilityFrameInContainerSpace:"), apple.Rect(float64(b.Origin.X), float64(b.Origin.Y),
			float64(b.Size.Width), float64(b.Size.Height)))
		axNodes[el] = axNode{v: v, node: n}
		v.axNodes = append(v.axNodes, el)
	}
	// A canvas with named parts is their container, not an element.
	v.native.Send(selSetIsAccessibilityElement, len(v.axNodes) == 0 && v.axElement)
	v.native.Send(sel("setAccessibilityElements:"), nsArray(v.axNodes))
}

// releaseCanvasNodes lets a canvas's elements go.
func (v *view) releaseCanvasNodes() {
	for _, el := range v.axNodes {
		delete(axNodes, el)
		el.Send(selRelease)
	}
	v.axNodes = v.axNodes[:0]
}

// nsArray returns an autoreleased array of items, empty for none.
func nsArray(items []objc.ID) objc.ID {
	a := objc.ID(apple.Class("NSMutableArray")).Send(sel("array"))
	for _, it := range items {
		a.Send(sel("addObject:"), it)
	}
	return a
}

func canvasNodeActivate(self objc.ID, _ objc.SEL) bool {
	n, ok := axNodes[self]
	if !ok {
		return false
	}
	v, b := n.v, n.node.Bounds
	s := v.surface
	center := apple.CGPoint{X: float64(b.Origin.X + b.Size.Width/2), Y: float64(b.Origin.Y + b.Size.Height/2)}
	at := objc.Send[apple.CGPoint](v.native, sel("convertPoint:toView:"), center, s.native)
	hit := objc.Send[objc.ID](s.native, sel("hitTest:withEvent:"), at, objc.ID(0))
	for ; hit != 0; hit = hit.Send(selSuperview) {
		w := views[hit]
		if w == nil || w.native != hit || int(w.id) >= len(s.views) || s.views[w.id] != w {
			continue
		}
		if !w.events.Has(driver.EventPress) && !w.events.Has(driver.EventPressDown) {
			continue
		}
		p := objc.Send[apple.CGPoint](s.native, sel("convertPoint:toView:"), at, w.native)
		e := driver.Event{Point: driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)},
			Page: driver.Point{X: driver.Points(at.X), Y: driver.Points(at.Y)}, Count: 1}
		for _, k := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
			e.Kind = k
			w.report(e)
		}
		return true
	}
	return false
}
