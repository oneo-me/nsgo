package core

import (
	"fmt"
	"slices"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Element is an element built one change at a time, as nsh-core's
// operations (ops.rs) build what NSH's renderer runs: the tsx package's.
// It is made detached, shows where it is inserted into another element or
// mounted as a Node, and is released with the owner it was made under.
type Element struct {
	t        *Tree
	n        *node
	owner    *Scope
	released bool
}

// NewElement returns a detached element whose props are props: a View, a
// Text, a Scroll, an Input or an Image. Its functions are read when it is made and on each Set, never
// bound: the tsx runtime binds what changes itself. With nil props it is
// a region, whose children lay out as its parent's.
func NewElement(cx *Scope, props Node) *Element {
	t, owner := cx.rt.tree, cx.current()
	var n *node
	switch props.(type) {
	case nil:
		n = t.newRegion(nil, owner)
	case View:
		n = t.newNode(driver.KindView, nil, owner)
	case Text:
		n = t.newNode(driver.KindText, nil, owner)
	case Scroll:
		n = t.newNode(driver.KindScroll, nil, owner)
		n.scroll = &scrollNode{}
	case Input:
		n = t.newNode(driver.KindInput, nil, owner)
	case Image:
		n = t.newNode(driver.KindImage, nil, owner)
	default:
		panic(fmt.Sprintf("core: no element of %T", props))
	}
	e := &Element{t: t, n: n, owner: owner}
	n.elem = e
	if props != nil {
		e.Set(props)
	}
	owner.addCleanup(e.Release)
	return e
}

// Render implements Node: mounted, the element moves there.
func (e *Element) Render(*Scope) Node { return e }

// Set replaces the element's props with props, of the type it was made
// with.
func (e *Element) Set(props Node) {
	t, n := e.t, e.n
	if n.deleted {
		return
	}
	t.within(e.owner, func() {
		switch v := props.(type) {
		case View:
			t.setView(n, v)
		case Text:
			t.setText(e.owner, n, v)
		case Scroll:
			t.setScroll(n, v)
		case Input:
			t.setInput(n, v)
		case Image:
			t.setImage(n, v)
		default:
			panic(fmt.Sprintf("core: an element's props are a View, a Text, a Scroll, an Input or an Image, not %T", props))
		}
	})
}

// Insert puts child among e's children before before, at the end when
// before is nil or not e's, moving it from where it was.
func (e *Element) Insert(child, before *Element) {
	var at *node
	if before != nil && before.n.parent == e.n {
		at = before.n
	}
	if !e.n.deleted && !child.n.deleted && child != e {
		e.t.place(child.n, e.n, at)
	}
}

// Mount shows node in region element e, built under e's owner: Go
// components among an element's children.
func (e *Element) Mount(node Node) {
	t := e.t
	if e.n.kind == 0 && !e.n.deleted {
		t.within(e.owner, func() { t.mount(e.owner, e.n, node) })
	}
}

// Remove takes child out of e's children; it stays until released.
func (e *Element) Remove(child *Element) {
	if child.n.parent == e.n {
		e.t.detach(child.n)
	}
}

// Release takes the element out and frees it, with what Go code mounted
// in it; the elements among its children only leave it.
func (e *Element) Release() {
	if e.released {
		return
	}
	e.released = true
	e.t.detach(e.n)
	e.t.discard(e.n)
}

// place moves c into p's children before at, or to the end.
func (t *Tree) place(c, p, at *node) {
	for a := p; a != nil; a = a.parent {
		if a == c {
			panic("core: an element cannot go inside itself")
		}
	}
	t.detach(c)
	i := len(p.children)
	if at != nil {
		if j := slices.Index(p.children, at); j >= 0 {
			i = j
		}
	}
	p.children = slices.Insert(p.children, i, c)
	c.parent = p
	t.childrenChanged(p)
	// It inherits from its new parent, and brings its marks.
	t.carry(c)
	t.markResolve(c)
	t.markLayout(c)
	t.rehost(c)
}

// detach takes n out of its parent's children.
func (t *Tree) detach(n *node) {
	if p := n.parent; p != nil {
		p.children = slices.DeleteFunc(p.children, func(c *node) bool { return c == n })
		n.parent = nil
		t.childrenChanged(p)
	}
}

// setView sets view n's parts from v, reading its functions once.
func (t *Tree) setView(n *node, v View) {
	base := v.Style
	if v.StyleFunc != nil {
		base = base.Merge(v.StyleFunc())
	}
	n.base = base
	t.setViewParts(n, v)
	class := v.Class
	if v.ClassFunc != nil {
		class = joinClasses(class, v.ClassFunc())
	}
	t.setClasses(n, class)
	t.bindAccessibility(nil, n, a11y{actions: v.AccessibilityActions, accessible: v.Accessible,
		important: v.ImportantForAccessibility})
	p := &n.props
	if v.LabelFunc != nil {
		p.Label = v.LabelFunc()
	}
	if v.AccessibilityValue != nil {
		p.Value = v.AccessibilityValue()
	}
	if v.Expanded != nil {
		p.Expandable, p.Expanded = true, v.Expanded()
	}
	if v.AccessibilityElementsHidden != nil && !p.ElementsHidden {
		p.ElementsHidden = v.AccessibilityElementsHidden()
	}
	p.Disabled = read(v.Disabled)
	p.Selected = read(v.Selected)
	p.Checked = read(v.Checked)
	p.Indeterminate = read(v.Indeterminate)
	n.part = PartStates{}
	if v.PartStates != nil {
		n.part = v.PartStates()
	}
	t.restyled(n)
	t.restyleGroup(n)
}

// setText sets text n's parts from v, reading its functions once, and
// mounts its attachments under cx.
func (t *Tree) setText(cx *Scope, n *node, v Text) {
	base := v.Style
	if v.StyleFunc != nil {
		base = base.Merge(v.StyleFunc())
	}
	n.base = base
	t.setClasses(n, v.Class)
	n.h = handlers{press: v.OnPress, layout: v.OnLayout}
	n.props = driver.ViewProps{Role: v.Role, Label: v.Label, Hint: v.Hint, LiveRegion: v.LiveRegion, TestID: v.TestID,
		Selectable: v.Selectable}
	if n.props.Role == RoleNone {
		n.props.Role = RoleText
	}
	if v.LabelFunc != nil {
		n.props.Label = v.LabelFunc()
	}
	t.setRef(n, v.Ref)
	tn := n.text
	if tn == nil {
		tn = &textNode{minWidth: -1}
		n.text = tn
	}
	content := v.Content
	if v.ContentFunc != nil {
		content = v.ContentFunc()
	}
	spans := v.Spans
	if v.SpansFunc != nil {
		spans = v.SpansFunc()
	}
	// Attachments are views of the text's, made again.
	for _, c := range n.children {
		t.discard(c)
	}
	n.children = n.children[:0]
	t.childrenChanged(n)
	tn.spans, tn.attachments = tn.spans[:0], 0
	for _, sp := range spans {
		style := sp.Style
		if sp.StyleFunc != nil {
			style = style.Merge(sp.StyleFunc())
		}
		if sp.Attachment != nil {
			w := t.newNode(driver.KindView, n, cx)
			w.attach = &attachNode{}
			w.base = style
			t.setClasses(w, sp.Class)
			t.restyle(w)
			t.mount(cx, w, sp.Attachment)
			tn.spans = append(tn.spans, spanNode{attach: w})
			tn.attachments++
			continue
		}
		s := sp.Content
		if sp.ContentFunc != nil {
			s = sp.ContentFunc()
		}
		style, cls := t.spanStyle(n, sp.Class, style)
		tn.spans = append(tn.spans, spanNode{content: s, style: style, press: sp.OnPress, cls: cls})
		if sp.OnPress != nil && n.h.press == nil {
			n.h.press = func(PressEvent) {} // presses come with the offset of the span
		}
	}
	tn.content, tn.maxLines, tn.ellipsize = content, v.MaxLines, v.EllipsizeMode
	tn.noScale, tn.maxScale = v.NoFontScaling, v.MaxFontSizeMultiplier
	tn.fit, tn.minScale, tn.lines = v.AdjustsFontSizeToFit, max(v.MinimumFontScale, 0.1), v.OnTextLayout
	tn.selectable = v.Selectable
	tn.stale = true
	t.followSpanClasses(n)
	t.restyled(n)
}

// setScroll sets scroll view n's parts from v, reading its functions
// once.
func (t *Tree) setScroll(n *node, v Scroll) {
	base := scrollDefaults.Merge(v.Style)
	if v.StyleFunc != nil {
		base = base.Merge(v.StyleFunc())
	}
	n.base = base
	t.setClasses(n, v.Class)
	n.h = handlers{layout: v.OnLayout}
	if on := v.OnScroll; on != nil {
		n.h.scroll = func(p Point) { on(n.scrollEvent(p)) }
	}
	n.props = driver.ViewProps{TestID: v.TestID}
	t.setRef(n, v.Ref)
	sc := n.scroll
	first := sc.zoom == 0 // bindScrollSpec sets it
	sc.config.Horizontal, sc.config.HideIndicator = v.Horizontal, v.HideIndicator
	sc.effects = v.EdgeEffects
	behavior := v.Behavior
	if v.BehaviorFunc != nil {
		behavior = v.BehaviorFunc()
	}
	zoom := v.ZoomScale
	if v.ZoomScaleFunc != nil {
		zoom = v.ZoomScaleFunc()
	}
	offset := v.ContentOffset
	if !first {
		offset = Point{}
	}
	t.bindScrollSpec(nil, n, scrollSpec{behavior: behavior, offset: offset, onRefresh: v.OnRefresh,
		onScroll: v.OnScroll, throttle: v.ScrollEventThrottle, onDragStart: v.OnScrollDragStart, onDragEnd: v.OnScrollDragEnd,
		onMomentum: v.OnMomentumStart, onMomentumEnd: v.OnMomentumEnd, minZoom: v.MinZoomScale,
		maxZoom: v.MaxZoomScale, zoom: zoom, noBouncesZoom: v.NoBouncesZoom, noPinch: v.NoPinchZoom,
		centerContent: v.CenterContent})
	sc.config.Refreshing = read(v.Refreshing)
	t.restyled(n)
}

// setInput sets text field n's parts from v, reading its functions once:
// a Value makes it controlled, following the value of the last Set.
func (t *Tree) setInput(n *node, v Input) {
	base := v.Style
	if v.StyleFunc != nil {
		base = base.Merge(v.StyleFunc())
	}
	n.base = base
	t.setClasses(n, v.Class)
	n.h = handlers{change: v.OnChange, submit: v.OnSubmit, focus: v.OnFocus, blur: v.OnBlur, layout: v.OnLayout}
	n.props = driver.ViewProps{Role: RoleTextField, Label: v.Label, TestID: v.TestID}
	t.setRef(n, v.Ref)
	in := n.input
	if in == nil {
		in = &inputNode{value: v.DefaultValue, autoFocus: v.AutoFocus}
		n.input = in
	}
	in.accessory = v.InputAccessoryViewID
	in.selection, in.keyPress, in.composition = v.OnSelectionChange, v.OnKeyPress, v.OnCompositionChange
	in.contentSize, in.caret, in.paste = v.OnContentSizeChange, v.OnCaretMove, v.OnPaste
	in.config = driver.InputConfig{Placeholder: v.Placeholder, Masked: v.Masked, Readonly: v.Readonly,
		KeyboardType: v.KeyboardType, Multiline: v.Multiline, MaxLength: max(v.MaxLength, 0),
		SubmitBehavior: v.SubmitBehavior, ReturnKey: v.ReturnKeyType, AutoCapitalize: v.AutoCapitalize,
		NoAutoCorrect: v.NoAutoCorrect, AutoComplete: v.AutoComplete, KeyboardAppearance: v.IOS.KeyboardAppearance,
		HideCaret: v.HideCaret, PasteBehavior: v.PasteBehavior, Disabled: read(v.Disabled)}
	in.valueFn = nil
	if v.Value != nil {
		value := v.Value()
		in.valueFn = func() string { return value }
		in.value = value
	}
	t.markResolve(n) // a multiline field measures its value
	t.restyled(n)
}

// setImage sets image n's parts from v, reading its functions once.
func (t *Tree) setImage(n *node, v Image) {
	base := v.Style
	if v.StyleFunc != nil {
		base = base.Merge(v.StyleFunc())
	}
	n.base = base
	t.setClasses(n, v.Class)
	t.setRef(n, v.Ref)
	n.h = handlers{press: v.OnPress, layout: v.OnLayout, loadStart: v.OnLoadStart, loadEnd: v.OnLoadEnd,
		progress: v.OnProgress, load: v.OnLoad, loadError: v.OnError}
	n.props = driver.ViewProps{Role: RoleImage, Label: v.Label, TestID: v.TestID}
	src := v.Source
	if v.SourceFunc != nil {
		src = v.SourceFunc()
	}
	if n.image == nil {
		n.image = &imageNode{}
	}
	if src.Size != n.image.config.Source.Size {
		t.markLayout(n)
	}
	n.image.config = driver.ImageConfig{Source: src, Placeholder: v.Placeholder, ResizeMode: resolveMode(v.ResizeMode, Cover),
		BlurRadius: v.BlurRadius, FadeDuration: v.FadeDuration}
	t.restyled(n)
}

// restyled merges n's style again after its parts changed and marks
// what follows from it.
func (t *Tree) restyled(n *node) {
	if t.restyle(n) {
		t.markLayout(n)
	}
	t.syncPosition(n)
	t.mark(n)
}

func read(fn func() bool) bool { return fn != nil && fn() }

// RenderEffect runs fn now and again, before the app's effects, whenever
// what it read changes, as element bindings do: the tsx package's
// bindings (NSH's render effects).
func (cx *Scope) RenderEffect(fn func()) { cx.renderEffect(fn) }

// Run runs fn untracked with cx owning what it creates, whichever owner
// runs now, as Solid's runWithOwner: the tsx package's roots and rows
// (NSH's with_root). A panic goes to the nearest Catch above cx.
func (cx *Scope) Run(fn func()) { cx.rt.run(cx, fn) }
