//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A canvas's named parts are NSAccessibilityElements, its accessibility
// children at their bounds in it, with their role, label, identifier and
// selected state. VoiceOver's press on one is a press at its center, on
// the view under that point that takes presses, as a click there.

var canvasNodeClass objc.Class

// axNodes maps the elements to their canvas and node.
var axNodes = map[objc.ID]axNode{}

type axNode struct {
	v    *view
	node driver.CanvasNode
}

func registerCanvasNodes() {
	canvasNodeClass = register("NsgoCanvasNode", apple.Class("NSAccessibilityElement"), nil, []objc.MethodDef{
		{Cmd: sel("accessibilityPerformPress"), Fn: canvasNodePress},
	})
}

// setCanvasNodes makes v's drawing's nodes its accessibility children.
func (v *view) setCanvasNodes(nodes []driver.CanvasNode) {
	if len(nodes) == 0 && len(v.axNodes) == 0 {
		return
	}
	for _, el := range v.axNodes {
		delete(axNodes, el)
		el.Send(selRelease)
	}
	v.axNodes = v.axNodes[:0]
	for _, n := range nodes {
		el := objc.ID(canvasNodeClass).Send(selNew)
		role := roles[n.Role]
		if role[0] == "" {
			role = [2]string{"AXGroup", ""}
		}
		el.Send(selSetAccessibilityRole, apple.NSString(role[0]))
		el.Send(selSetAccessibilitySubrole, nsOrNil(role[1]))
		el.Send(sel("setAccessibilityLabel:"), nsOrNil(n.Label))
		el.Send(selSetAccessibilityIdentifier, nsOrNil(n.TestID))
		el.Send(selSetAccessibilitySelected, n.Selected)
		el.Send(sel("setAccessibilityParent:"), v.native)
		b := n.Bounds
		el.Send(sel("setAccessibilityFrameInParentSpace:"), apple.Rect(float64(b.Origin.X), float64(b.Origin.Y),
			float64(b.Size.Width), float64(b.Size.Height)))
		axNodes[el] = axNode{v: v, node: n}
		v.axNodes = append(v.axNodes, el)
	}
	v.updateAccessibility()
}

// releaseCanvasNodes lets a deleted canvas's elements go.
func (v *view) releaseCanvasNodes() {
	for _, el := range v.axNodes {
		delete(axNodes, el)
		el.Send(selRelease)
	}
	v.axNodes = nil
}

func canvasNodePress(self objc.ID, _ objc.SEL) bool {
	n, ok := axNodes[self]
	if !ok || n.v.surface.views == nil {
		return false
	}
	v, b := n.v, n.node.Bounds
	center := apple.CGPoint{X: float64(b.Origin.X + b.Size.Width/2), Y: float64(b.Origin.Y + b.Size.Height/2)}
	loc := objc.Send[apple.CGPoint](v.native, sel("convertPoint:toView:"), center, objc.ID(0))
	s := v.surface
	up := s.native.Send(selSuperview)
	if up == 0 {
		return false
	}
	hit := s.native.Send(sel("hitTest:"), objc.Send[apple.CGPoint](up, selConvertPoint, loc, objc.ID(0)))
	for ; hit != 0 && hit != up; hit = hit.Send(selSuperview) {
		w := views[hit]
		if w == nil || w.native != hit || int(w.id) >= len(s.views) || s.views[w.id] != w {
			continue
		}
		if !w.events.Has(driver.EventPress) && !w.events.Has(driver.EventPressDown) {
			continue
		}
		p := objc.Send[apple.CGPoint](w.native, selConvertPoint, loc, objc.ID(0))
		e := driver.Event{Point: driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}, Page: w.page(loc), Count: 1}
		for _, k := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
			e.Kind = k
			w.report(e)
		}
		return true
	}
	return false
}
