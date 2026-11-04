//go:build js && wasm

package web

import (
	"strconv"
	"syscall/js"
)

// Styles as classes, as NSH's web runtime does (its web.md, "Styles as
// classes"): no element has an inline style but its geometry and what
// animates, so the page does not parse the same text again for every
// element of every batch. A view's look, what its events make of the
// pointer and selection, and a text's paragraph each go to a class named
// nsgo- and 8 hex digits of the FNV-1a hash of their CSS, inserted as
// .name.name{css} into the surface's sheet the first time it is used: the
// doubled class wins over the runtime's rules of a class and a
// pseudo-class, as inline styles did.
//
// A view whose style keeps changing would add a rule each time: after
// defining ownRulesAfter classes, a slot takes a rule of its own in a
// second sheet, changed in place, which the view's deletion frees for
// another.

const ownRulesAfter = 16

// classSheet is a surface's classes.
type classSheet struct {
	sheet, own  js.Value          // the CSSStyleSheets of classes and own rules
	rules, owns int               // the rules each holds
	names       map[string]string // class names by CSS
	taken       map[string]bool
	free        []*ownRule
}

// ownRule is a rule of one slot's own, under a class of its own.
type ownRule struct {
	style js.Value // the rule's CSSStyleDeclaration
	name  string
}

// classSlot is a part of an element's style a class carries.
type classSlot struct {
	css   string // the CSS last applied
	name  string // its class on the element, "" for none
	count int    // the classes it has defined
	own   *ownRule
}

// cssText builds a declaration block.
type cssText []byte

// add appends name: value; for a value that is not empty.
func (c *cssText) add(name, value string) {
	if value != "" {
		*c = append(append(append(append(*c, name...), ':'), value...), ';')
	}
}

func (s *surface) classSheet() *classSheet {
	c := &s.classes
	if c.names == nil {
		head := s.doc.Get("head")
		for _, id := range []string{"nsgo-classes", "nsgo-own"} {
			el := s.doc.Call("createElement", "style")
			el.Set("id", id)
			head.Call("appendChild", el)
			if id == "nsgo-classes" {
				c.sheet = el.Get("sheet")
			} else {
				c.own = el.Get("sheet")
			}
		}
		c.names, c.taken = map[string]string{}, map[string]bool{}
	}
	return c
}

// class returns the class of css, and whether it inserted its rule:
// the first time.
func (c *classSheet) class(css string) (string, bool) {
	if name, ok := c.names[css]; ok {
		return name, false
	}
	h := uint32(2166136261)
	for i := 0; i < len(css); i++ {
		h = (h ^ uint32(css[i])) * 16777619
	}
	hex := strconv.FormatUint(uint64(h)|1<<32, 16)[1:]
	name := "nsgo-" + hex
	// Two texts of one hash: the later gets a suffix.
	for i := 1; c.taken[name]; i++ {
		name = "nsgo-" + hex + "-" + strconv.Itoa(i)
	}
	c.taken[name] = true
	c.names[css] = name
	c.sheet.Call("insertRule", "."+name+"."+name+"{"+css+"}", c.rules)
	c.rules++
	return name, true
}

// ownRule returns a free rule of a slot's own.
func (c *classSheet) ownRule() *ownRule {
	if n := len(c.free); n > 0 {
		r := c.free[n-1]
		c.free = c.free[:n-1]
		return r
	}
	name := "nsgo-own-" + strconv.Itoa(c.owns)
	c.own.Call("insertRule", "."+name+"."+name+"{}", c.owns)
	r := &ownRule{style: c.own.Get("cssRules").Index(c.owns).Get("style"), name: name}
	c.owns++
	return r
}

// setClass gives el the class of css in slot, taking the old one off.
func (s *surface) setClass(el js.Value, slot *classSlot, css cssText) {
	if string(css) == slot.css {
		return
	}
	slot.css = string(css)
	if slot.own != nil {
		slot.own.style.Set("cssText", slot.css)
		return
	}
	name := ""
	switch c := s.classSheet(); {
	case slot.css == "":
	case slot.count >= ownRulesAfter:
		slot.own = c.ownRule()
		slot.own.style.Set("cssText", slot.css)
		name = slot.own.name
	default:
		var defined bool
		if name, defined = c.class(slot.css); defined {
			slot.count++
		}
	}
	if name == slot.name {
		return
	}
	list := el.Get("classList")
	switch {
	case slot.name == "":
		list.Call("add", name)
	case name == "":
		list.Call("remove", slot.name)
	default:
		list.Call("replace", slot.name, name)
	}
	slot.name = name
}

// freeClass gives slot's own rule back for another slot.
func (s *surface) freeClass(slot *classSlot) {
	if r := slot.own; r != nil {
		r.style.Set("cssText", "")
		s.classes.free = append(s.classes.free, r)
		slot.own = nil
	}
}
