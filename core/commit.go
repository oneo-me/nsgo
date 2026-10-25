package core

import (
	"fmt"
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/layout"
)

// maxRounds bounds the commits OnLayout handlers may cause in one batch.
const maxRounds = 8

// Commit resolves styles, prepares text, lays the tree out and returns
// what changed since the last commit, or nil when nothing did. Changes
// that OnLayout handlers make go into the same batch.
func (t *Tree) Commit() *driver.MountBatch {
	if !t.changed {
		return nil
	}
	t.items = t.items[:0]
	t.commits++
	t.commitStart = t.rt.clock.Now()
	for round := 0; t.changed && round < maxRounds; round++ {
		t.commitRound()
	}
	t.paceLater()
	if driver.CheckCommits && !t.changed {
		t.checkCommit()
	}
	if len(t.items) == 0 {
		return nil
	}
	b := &driver.MountBatch{Root: t.view.id, Items: append([]driver.MountItem(nil), t.items...), LayoutAnimation: t.layoutAnim}
	t.layoutAnim = nil
	// The first commit sends every view, and the slice it grew would stay
	// this tree's capacity for good: a long list then keeps megabytes for
	// the handful of items a busy commit sends.
	if n, c := len(t.items), cap(t.items); c >= 4096 && c > 4*n {
		t.items = make([]driver.MountItem, 0, max(512, 2*n))
	}
	return b
}

// checkCommit commits again, laying every node out anew and comparing
// every node, and panics if that finds what the commit's marks missed.
func (t *Tree) checkCommit() {
	sent := len(t.items)
	frames := map[NodeID]Bounds{}
	for _, n := range t.nodes {
		if n != nil {
			frames[n.id] = n.sent.frame
		}
	}
	for _, n := range t.nodes {
		if n != nil && !n.deleted {
			n.lstyleOK = false
			t.markLayout(n)
		}
	}
	t.full = true
	t.commitRound()
	if missed := t.items[sent:]; len(missed) > 0 {
		var b strings.Builder
		for _, it := range missed {
			fmt.Fprintf(&b, " kind %d of view %d", it.Kind, it.ID)
			if it.Kind == driver.ItemFrame {
				fmt.Fprintf(&b, " (%v, sent %v)", it.Frame, frames[it.ID])
			}
			b.WriteString(";")
		}
		panic("core: a commit missed changes it should have sent:" + b.String())
	}
}

func (t *Tree) commitRound() {
	t.changed = false
	t.fullRound, t.full = t.full, false
	if scale := t.root.Env().fontScale.GetUntracked(); scale != t.fontScale {
		t.fontScale = scale
		t.fullRound = true
	}
	if rtl := t.root.Env().rtl.GetUntracked(); rtl != t.rtl {
		t.setRTL(rtl)
	}
	live := t.stacks[:0]
	for _, s := range t.stacks {
		if !s.deleted {
			t.showTop(s)
			t.markScreens(s)
			live = append(live, s)
		}
	}
	t.stacks = live
	t.resolve(t.view, textFields{}, false)
	t.layout()
	// Lists fill their windows from the layout, which may need another.
	for range 4 {
		if !t.updateLists() {
			break
		}
		t.resolve(t.view, textFields{}, false)
		t.layout()
	}
	// System headers cover what the stacks' places decide.
	for range 2 {
		if !t.sizeHeaders() {
			break
		}
		t.layout()
	}
	t.markHeaderScrolls()
	t.sendSticky()
	t.sizeCanvases()
	t.laidOut = t.laidOut[:0]
	t.emitAccessories()
	t.emitHeaderViews()
	t.emit(t.view)
	t.emitMoves()
	t.emitAnimation()
	if t.keysChanged {
		t.keysChanged = false
		t.items = append(t.items, driver.MountItem{Kind: driver.ItemKeymap, Keys: t.keymap()})
	}
	t.emitStatusBar()
	t.items = append(t.items, t.commands...)
	t.commands = t.commands[:0]
	t.emitDeletes()
	for _, id := range t.deleted {
		t.nodes[id] = nil
		t.free = append(t.free, id)
	}
	t.deleted = t.deleted[:0]
	for _, n := range t.laidOut {
		if !n.deleted && n.h.layout != nil {
			frame := n.frame
			t.within(n.owner, func() { n.h.layout(frame) })
		}
	}
	reports := t.reports
	t.reports = nil
	for _, r := range reports {
		if !r.owner.deleted {
			t.within(r.owner.owner, r.fn)
		}
	}
}

// resolve inherits text styles and directions down the tree and prepares
// changed text: in the nodes marked for it, and in every node inside one
// whose inherited fields changed, or everywhere when force is set.
func (t *Tree) resolve(n *node, inherited textFields, force bool) {
	force = force || t.fullRound
	// A text resolves its attachments itself.
	self := force || n.unresolved || n.resolveKids && n.text != nil
	if !self && !n.resolveKids {
		return
	}
	n.unresolved, n.resolveKids = false, false
	// A region resolves anew only as it is made or moves, with what it
	// holds.
	kids := force || self && n.kind == 0
	if n.kind != 0 {
		inherited = n.style.inherit(inherited)
		if self && inherited != n.passed {
			n.passed = inherited
			kids = true
		}
	}
	if !self {
		for _, c := range n.children {
			t.resolve(c, inherited, kids)
		}
		return
	}
	t.mark(n)
	if n.kind != 0 {
		n.color = inherited.Color
		if n.color == 0 {
			n.color = RGB(0x000000)
		}
		rtl := t.rtl
		if d := inherited.LayoutDirection; d != 0 {
			rtl = d == DirectionRTL
		}
		if rtl != n.rtl {
			n.rtl = rtl
			n.lstyleOK = false
			t.markLayout(n)
		}
	}
	switch {
	case n.icon != nil:
		// An icon paints in the text color it inherits.
		in := n.icon
		in.text = t.scaled(inherited.textStyle())
		t.within(n.owner, func() { in.color.Set(in.text.Color) })
	case n.canvas != nil:
		t.resolvePaint(n, &inherited)
	case n.text != nil:
		n.props.SelectionColor = inherited.SelectionColor
		tn := n.text
		n.props.Selectable = tn.selectable
		if tn.selectable == SelectableAuto {
			switch inherited.UserSelect {
			case UserSelectText:
				n.props.Selectable = SelectableOn
			case UserSelectNone:
				n.props.Selectable = SelectableOff
			}
		}
		if noWrap := inherited.WhiteSpace == WhiteSpaceNoWrap; noWrap != tn.noWrap {
			tn.noWrap = noWrap
			tn.resent = true
			t.markLayout(n)
		}
		tn.engine, tn.measures = t.text, &t.measures
		if inherited.TextAlignVertical != tn.alignV {
			tn.alignV = inherited.TextAlignVertical
			tn.resent = true
		}
		scale := tn.fontMultiplier(t.fontScale)
		ts := scaleText(inherited.textStyle(), scale)
		ts.RTL = n.textRTL(inherited, tn.content)
		sized := tn.attachments > 0 && t.sizeAttachments(n, inherited, kids)
		shadow := inherited.textShadow()
		if tn.stale || sized || tn.prepared == nil || ts != tn.style || shadow != tn.text.Shadow {
			at := driver.AttributedText{Text: inherited.TextTransform.Apply(tn.content), Style: ts, MaxLines: tn.maxLines,
				Ellipsize: tn.ellipsize, Shadow: shadow}
			if at.Ellipsize == 0 {
				at.Ellipsize = EllipsizeTail
			}
			if len(tn.spans) > 0 {
				var b strings.Builder
				for _, sp := range tn.spans {
					start := b.Len()
					if w := sp.attach; w != nil {
						b.WriteRune(driver.AttachmentRune)
						at.Attachments = append(at.Attachments, driver.TextAttachment{Offset: start,
							Size: w.attach.size, Align: w.verticalAlign()})
						at.Spans = append(at.Spans, driver.TextSpan{Start: start, End: b.Len(), Style: ts})
						continue
					}
					own := sp.style.inherit(inherited)
					b.WriteString(own.TextTransform.Apply(sp.content))
					spanStyle := scaleText(own.textStyle(), scale)
					spanStyle.RTL = ts.RTL
					at.Spans = append(at.Spans, driver.TextSpan{Start: start, End: b.Len(),
						Style: spanStyle, Link: sp.press != nil,
						Background: driver.TextBackground{Color: sp.style.Background, Radius: sp.style.CornerRadius}})
				}
				at.Text = b.String()
			}
			tn.style = ts
			tn.stale = false
			if tn.prepared == nil || !at.Equal(&tn.text) {
				if tn.prepared != nil {
					tn.prepared.Release()
				}
				// A text that lays out as before, its colors aside, or
				// that measures as before, lays nothing out: only its own
				// lines change. Attachments are placed by them.
				h := t.measures.hash(&at)
				same := tn.prepared != nil && h == tn.hash
				tn.text, tn.hash = at, h
				tn.prepared = t.text.Prepare(&at)
				tn.minWidth = -1
				tn.resent = true
				if tn.attachments > 0 || !same && !layout.MeasuresAsBefore(n) {
					t.markLayout(n)
				}
			}
		}
	case n.input != nil:
		in := n.input
		in.config.PlaceholderColor, in.config.SelectionColor = inherited.PlaceholderColor, inherited.SelectionColor
		ts := t.scaled(inherited.textStyle())
		ts.RTL = n.textRTL(inherited, in.value)
		if ts != in.config.Style || in.lineH == 0 {
			in.config.Style = ts
			p := t.text.Prepare(&driver.AttributedText{Text: "Ag", Style: ts})
			in.lineH = p.Measure(Auto).Height
			p.Release()
			t.markLayout(n)
		}
		t.prepareInput(n)
	}
	if n.text != nil {
		return // its children are attachments, which it resolved
	}
	for _, c := range n.children {
		t.resolve(c, inherited, kids)
	}
}

// textRTL reports whether text node n, styled inherited, writes content
// right to left.
func (n *node) textRTL(inherited textFields, content string) bool {
	switch inherited.WritingDirection {
	case WritingLTR:
		return false
	case WritingRTL:
		return true
	case WritingAuto:
		if n.text != nil && len(n.text.spans) > 0 {
			var b strings.Builder
			for _, sp := range n.text.spans {
				b.WriteString(sp.content)
			}
			content = b.String()
		}
		return rtlText(content)
	}
	return n.rtl
}

// attachmentSpace is the space attachments are laid out in: they take
// their content size.
const attachmentSpace = 1 << 20

// sizeAttachments resolves and lays out the attachments of text n, each a
// layout root at its content size, and reports whether a size changed.
func (t *Tree) sizeAttachments(n *node, inherited textFields, force bool) bool {
	changed := false
	for _, sp := range n.text.spans {
		w := sp.attach
		if w == nil {
			continue
		}
		t.resolve(w, inherited, force)
		if !w.layoutDirty {
			continue
		}
		t.layoutRoot(w, Size{Width: attachmentSpace, Height: attachmentSpace}, false)
		t.syncFrames(w)
		l := &w.lstate.Rounded
		if size := (Size{Width: Points(l.Size.Width), Height: Points(l.Size.Height)}); size != w.attach.size {
			w.attach.size = size
			changed = true
		}
	}
	return changed
}

// verticalAlign is where attachment view w sits in its line: its span's
// VerticalAlign, else that of the one view the attachment made.
func (w *node) verticalAlign() VerticalAlign {
	if a := w.style.VerticalAlign; a != 0 {
		return a
	}
	if kids := w.flatChildren(); len(kids) == 1 {
		return kids[0].style.VerticalAlign
	}
	return 0
}

// placeAttachments puts the views of text n's attachments where layout
// put their placeholders, hiding those its line limit cut off.
func (t *Tree) placeAttachments(n *node, l driver.TextLayout) {
	al, _ := l.(AttachmentLayout)
	i := 0
	for _, sp := range n.text.spans {
		w := sp.attach
		if w == nil {
			continue
		}
		var b Bounds
		ok := false
		if al != nil {
			b, ok = al.Attachment(i)
		}
		i++
		if w.attach.cut != !ok {
			w.attach.cut = !ok
			t.mark(w)
		}
		if ok {
			b.Origin.X += n.text.shownBox.Origin.X
			b.Origin.Y += n.text.shownBox.Origin.Y
			t.setFrame(w, Bounds{Origin: b.Origin, Size: w.attach.size})
		}
	}
}

// sizeCanvases tells canvases their laid-out size, which draws those
// that changed.
func (t *Tree) sizeCanvases() {
	live := t.canvases[:0]
	for _, n := range t.canvases {
		if n.deleted {
			continue
		}
		live = append(live, n)
		if n.canvas.size.GetUntracked() != n.frame.Size {
			size := n.frame.Size
			t.within(n.owner, func() { n.canvas.size.Set(size) })
		}
	}
	clear(t.canvases[len(live):])
	t.canvases = live
}

// within2 runs fn under n's owner and returns its result.
func (t *Tree) within2(n *node, fn func() bool) (r bool) {
	t.within(n.owner, func() { r = fn() })
	return r
}

// scaled applies the user's text size setting.
func (t *Tree) scaled(ts driver.TextStyle) driver.TextStyle { return scaleText(ts, t.fontScale) }

// scaleText multiplies ts's font size and line height by scale.
func scaleText(ts driver.TextStyle, scale float32) driver.TextStyle {
	if s := Points(scale); s != 1 && s > 0 {
		ts.Font.Size *= s
		ts.LineHeight *= s
	}
	return ts
}

// emit sends what changed of view n and the views inside it, visiting
// only those marked, and reports whether n moved, resized or changed how
// it shows, which its parent's overflow area follows.
func (t *Tree) emit(n *node) bool {
	self := n.dirty || t.fullRound
	if !self && !n.dirtyKids {
		return false
	}
	n.dirty, n.dirtyKids = false, false
	first := !n.created
	moved := false
	if self {
		moved = t.emitParts(n)
	}
	kids := n.nativeChildren()
	kidsMoved := false
	for _, c := range kids {
		if t.emit(c) {
			kidsMoved = true
		}
	}
	if n.childrenDirty {
		t.syncChildren(n, kids)
		n.childrenDirty = false
		kidsMoved = true
	}
	if n != t.view && (self || kidsMoved) {
		if area := n.overflowArea(kids); area != n.sent.overflow {
			n.sent.overflow = area
			t.items = append(t.items, driver.MountItem{Kind: driver.ItemOverflow, ID: n.id, Frame: area})
			moved = true
		}
	}
	if self && (first || n.frame != n.sent.frame) {
		n.sent.frame = n.frame
		t.items = append(t.items, driver.MountItem{Kind: driver.ItemFrame, ID: n.id, Frame: n.frame})
		if n.h.layout != nil {
			t.laidOut = append(t.laidOut, n)
		}
		moved = true
	}
	return moved
}

// emitParts sends what changed of n's own parts, creating its view the
// first time, and reports whether its view style changed.
func (t *Tree) emitParts(n *node) bool {
	add := func(it driver.MountItem) { t.items = append(t.items, it) }
	first := !n.created
	styled := false
	if first {
		add(driver.MountItem{Kind: driver.ItemCreate, ID: n.id, View: n.kind})
		n.created = true
	}
	// What goes into an item is a copy made only when it is sent: the
	// comparison alone would otherwise put both values on the heap for
	// every view of every commit.
	if vs := n.viewStyle(); first || vs != n.sent.viewStyle {
		n.sent.viewStyle = vs
		sent := vs
		add(driver.MountItem{Kind: driver.ItemViewStyle, ID: n.id, ViewStyle: &sent})
		styled = true
	}
	if p := n.sentProps(); first || p != n.sent.props {
		// A label changing alone goes alone: it is the one that follows
		// app state (a count, a price).
		rest := p
		rest.Label = n.sent.props.Label
		if !first && rest == n.sent.props {
			add(driver.MountItem{Kind: driver.ItemAccessibilityLabel, ID: n.id, Value: p.Label})
		} else {
			sent := p
			add(driver.MountItem{Kind: driver.ItemViewProps, ID: n.id, ViewProps: &sent})
		}
		n.sent.props = p
	}
	if n.style.Animated != nil || n.anim != nil {
		t.bindAnimated(n)
	}
	if ev := n.events(); first || ev != n.sent.events {
		n.sent.events = ev
		add(driver.MountItem{Kind: driver.ItemEvents, ID: n.id, Events: ev})
	}
	switch {
	case n.text != nil:
		tn := n.text
		if tn.prepared != nil && (tn.resent || tn.box != tn.sentBox) {
			tn.resent = false
			tn.sentBox = tn.box
			t.emitText(n)
		}
	case n.input != nil:
		in := n.input
		in.config.Accessory = t.accessoryFor(in.accessory)
		if first || in.config != n.sent.input {
			c := in.config
			n.sent.input = c
			add(driver.MountItem{Kind: driver.ItemInput, ID: n.id, Input: &c})
		}
		if first || in.value != in.shown {
			in.shown = in.value
			add(driver.MountItem{Kind: driver.ItemInputValue, ID: n.id, Value: in.value, Count: in.count})
		}
		if first && in.autoFocus {
			t.command(driver.ItemFocus, n)
		}
		t.reportContentSize(n)
	case n.image != nil:
		im := n.image
		c := im.config
		c.Position = n.style.ObjectPosition
		c.Source = t.pickScale(c.Source)
		c.Placeholder = t.pickScale(c.Placeholder)
		if first || !c.Same(im.sent) {
			im.sent = c
			add(driver.MountItem{Kind: driver.ItemImage, ID: n.id, Image: &c})
		}
	case n.hoist != nil && n.hoist.modal != nil:
		if first {
			m := n.hoist.modal
			add(driver.MountItem{Kind: driver.ItemModal, ID: n.id, Modal: &driver.ModalConfig{Animation: m.Animation, Presentation: m.Presentation,
				NoCloseOnSwipe: m.NoCloseOnSwipe, Orientations: m.SupportedOrientations}})
		}
	case n.canvas != nil:
		if cn := n.canvas; cn.fresh && cn.drawing != nil {
			cn.fresh = false
			add(driver.MountItem{Kind: driver.ItemCanvas, ID: n.id, Drawing: cn.drawing.Painted(&cn.paint, cn.color)})
		}
	case n.shader != nil:
		if sn := n.shader; sn.fresh {
			sn.fresh = false
			add(driver.MountItem{Kind: driver.ItemShader, ID: n.id, Shader: sn.sent})
		}
	case n.web != nil:
		if wn := n.web; first || wn.config != wn.sent {
			c := wn.config
			wn.sent = c
			add(driver.MountItem{Kind: driver.ItemWeb, ID: n.id, Web: &c})
		}
	case n.native != nil:
		t.emitNative(n)
	case n.video != nil:
		if vn := n.video; first || vn.config != vn.sent {
			c := vn.config
			vn.sent = c
			add(driver.MountItem{Kind: driver.ItemVideo, ID: n.id, Video: &c})
		}
	case n.scroll != nil:
		sc := n.scroll
		c := sc.config
		if inset := t.headerInset(n); inset > 0 {
			c.HeaderInset = inset
			c.Behavior.ContentInset.Top += inset
		}
		if first || sc.resend || !c.Equal(&n.sent.scroll) {
			sc.resend = false
			c.Behavior.SnapToOffsets = slices.Clone(c.Behavior.SnapToOffsets)
			n.sent.scroll = c
			add(driver.MountItem{Kind: driver.ItemScrollConfig, ID: n.id, Scroll: &c})
		}
		t.emitScrollBy(n, true)
		if first || sc.content != n.sent.content {
			n.sent.content = sc.content
			add(driver.MountItem{Kind: driver.ItemScrollContent, ID: n.id, Size: sc.content})
		}
		t.emitScrollBy(n, false)
		t.emitScrollTo(n)
	}
	if sc := n.screen; sc != nil && sc.native {
		t.emitScreen(n)
	}
	return styled
}

// emitText lays text n out in its box and sends it: moved down by its
// vertical alignment, shrunk to fit when it asks to be, its lines
// reported after the commit.
func (t *Tree) emitText(n *node) {
	tn := n.text
	box := tn.box
	if f := tn.alignV.fraction(); f > 0 {
		if h := tn.size(box.Size.Width).Height; h < box.Size.Height {
			box.Origin.Y += f * (box.Size.Height - h)
			box.Size.Height = h
		}
	}
	tn.shownBox = box
	prepared, text := tn.prepared, &tn.text
	if tn.fitted != nil {
		tn.fitted.Release()
		tn.fitted = nil
	}
	if tn.fit {
		if fitted, ok := fitText(t.text, text, box.Size, tn.minScale); ok {
			tn.fitted = t.text.Prepare(&fitted)
			prepared, text = tn.fitted, &fitted
		}
	}
	if tn.noWrap {
		// Unwrapped lines overflow the box.
		box.Size.Width = max(box.Size.Width, ceil(prepared.Measure(Auto).Width))
	}
	l := prepared.Layout(box.Size)
	if tn.attachments > 0 {
		t.placeAttachments(n, l)
	}
	if tn.lines != nil {
		if ll, ok := l.(LineLayout); ok {
			lines := ll.TextLines()
			for i := range lines {
				a, b := min(max(lines[i].Start, 0), len(text.Text)), min(max(lines[i].End, 0), len(text.Text))
				lines[i].Text = text.Text[a:max(a, b)]
			}
			report := tn.lines
			t.after(n, func() { report(lines) })
		}
	}
	t.items = append(t.items, driver.MountItem{Kind: driver.ItemText, ID: n.id, Text: &driver.TextContent{
		Text: text.Text, Layout: l, Content: box, Style: text.Style, Spans: text.Spans,
	}})
}

// fitText returns text with its fonts, line heights and letter spacing
// scaled down, to no less than least of them, until it fits size within
// its line limit: the largest scale that fits, by bisection, as NSH's
// Paragraph::fit. It reports false when text fits as it is.
func fitText(engine driver.TextEngine, text *driver.AttributedText, size Size, least float32) (driver.AttributedText, bool) {
	scaled := func(scale float32) driver.AttributedText {
		s := *text
		s.Style = fitStyle(s.Style, scale)
		s.Spans = append([]driver.TextSpan(nil), text.Spans...)
		for i := range s.Spans {
			s.Spans[i].Style = fitStyle(s.Spans[i].Style, scale)
		}
		return s
	}
	measure := func(t *driver.AttributedText) Size {
		p := engine.Prepare(t)
		defer p.Release()
		return p.Measure(size.Width)
	}
	fits := func(t driver.AttributedText) bool {
		// Untruncated, the text must be no taller than its truncated self
		// (nothing cut) and than the box.
		limit := size.Height
		if t.MaxLines > 0 {
			limit = min(limit, measure(&t).Height)
		}
		all := t
		all.MaxLines = 0
		full := measure(&all)
		return full.Height <= limit+0.5 && full.Width <= size.Width+0.5
	}
	if fits(*text) {
		return driver.AttributedText{}, false
	}
	lo, hi := least, float32(1)
	if !fits(scaled(lo)) {
		return scaled(lo), true
	}
	for range 7 {
		mid := (lo + hi) / 2
		if fits(scaled(mid)) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return scaled(lo), true
}

// fitStyle scales what fitting shrinks.
func fitStyle(ts driver.TextStyle, scale float32) driver.TextStyle {
	s := Points(scale)
	ts.Font.Size *= s
	ts.LineHeight *= s
	ts.LetterSpacing *= s
	return ts
}

func (n *node) viewStyle() driver.ViewStyle {
	vs := n.style.viewStyle(n.frame.Size, n.color, n.rtl)
	vs.Hidden = vs.Hidden || n.style.Display == DisplayNone || (n.hoist != nil && n.hidden) || (n.attach != nil && n.attach.cut)
	// Effects that draw as before keep the pointer that was sent, so the
	// comparison finds nothing to send.
	if vs.Effects != nil && vs.Effects.Equal(n.sent.viewStyle.Effects) {
		vs.Effects = n.sent.viewStyle.Effects
	}
	return vs
}

// emitDeletes deletes the views discarded since the last round. They go
// after the round's inserts, so a view moved out of a deleted one is in
// its new parent first; one taken out of the tree is removed before.
func (t *Tree) emitDeletes() {
	for _, id := range t.deleted {
		for _, c := range t.nodes[id].sent.children {
			if !c.deleted && c.sent.parent == t.nodes[id] {
				c.sent.parent, c.sent.removed = nil, true
				t.items = append(t.items, driver.MountItem{Kind: driver.ItemRemove, ID: c.id})
			}
		}
	}
	for _, id := range t.deleted {
		t.items = append(t.items, driver.MountItem{Kind: driver.ItemDelete, ID: id})
	}
}

// syncChildren inserts, moves and removes p's native children so they
// match want, moving as few as the longest run already in order allows.
// A view that changes parents is never removed: its one insert, after
// every new parent's, lets a backend move it in one step, keeping what a
// removal loses (a web page's scroll offsets).
func (t *Tree) syncChildren(p *node, want []*node) {
	old := p.sent.children
	pos := make(map[*node]int, len(old))
	for i, c := range old {
		if !c.deleted && c.sent.parent == p {
			pos[c] = i
		}
	}
	keep := make(map[*node]bool, len(want))
	for _, c := range want {
		keep[c] = true
	}
	for _, c := range old {
		if !c.deleted && !keep[c] && c.sent.parent == p && !t.insertedElsewhere(c, p) {
			c.sent.parent, c.sent.removed = nil, true
			t.items = append(t.items, driver.MountItem{Kind: driver.ItemRemove, ID: c.id})
		}
	}
	seq := make([]int, len(want))
	for i, c := range want {
		if j, ok := pos[c]; ok {
			seq[i] = j
		} else {
			seq[i] = -1
		}
	}
	stay := increasingRun(seq)
	var moving []bool
	for i := len(want) - 1; i >= 0; i-- {
		c := want[i]
		if stay[i] {
			continue
		}
		if from := c.sent.parent; from != nil && from != p || c.sent.removed {
			// Moved out of a view that still holds it, or put back: inserted
			// after the round's other inserts, once its new parent is in
			// place too.
			if moving == nil {
				moving = make([]bool, len(want))
			}
			moving[i] = true
			m := move{view: c, parent: p}
			if i+1 < len(want) {
				m.before = want[i+1]
			}
			t.moves = append(t.moves, m)
			c.sent.parent, c.sent.removed = p, false
			continue
		}
		var before NodeID
		for j := i + 1; j < len(want); j++ {
			if moving == nil || !moving[j] {
				before = want[j].id
				break
			}
		}
		c.sent.parent = p
		t.items = append(t.items, driver.MountItem{Kind: driver.ItemInsert, ID: c.id, Parent: p.id, Before: before})
	}
	p.sent.children = append(p.sent.children[:0], want...)
}

// move is the insert of a view inserted before into a parent, put off
// to the end of the round.
type move struct{ view, parent, before *node }

// emitMoves inserts the views that change parents or come back, the
// last of each parent's first, so the sibling each goes before is in
// place.
func (t *Tree) emitMoves() {
	for _, m := range t.moves {
		var before NodeID
		if m.before != nil {
			before = m.before.id
		}
		t.items = append(t.items, driver.MountItem{Kind: driver.ItemInsert, ID: m.view.id, Parent: m.parent.id, Before: before})
	}
	t.moves = t.moves[:0]
}

// insertedElsewhere reports whether c, no longer p's, is in another view
// this commit sends, whose sync inserts it.
func (t *Tree) insertedElsewhere(c, p *node) bool {
	n := c.nativeParent()
	if n == nil || n == p {
		return false
	}
	for n != nil && !n.deleted {
		switch {
		case n == t.view || n.accessory != nil || n.headerView:
			return true
		case n.hoist != nil:
			n = n.hoist.host
		default:
			n = n.parent
		}
	}
	return false
}

// increasingRun marks a longest strictly increasing subsequence of seq,
// ignoring negative entries.
func increasingRun(seq []int) []bool {
	stay := make([]bool, len(seq))
	tails := []int{} // index into seq of the smallest tail of each length
	prev := make([]int, len(seq))
	for i, v := range seq {
		if v < 0 {
			continue
		}
		lo, hi := 0, len(tails)
		for lo < hi {
			mid := (lo + hi) / 2
			if seq[tails[mid]] < v {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo > 0 {
			prev[i] = tails[lo-1]
		} else {
			prev[i] = -1
		}
		if lo == len(tails) {
			tails = append(tails, i)
		} else {
			tails[lo] = i
		}
	}
	if len(tails) > 0 {
		for i := tails[len(tails)-1]; i >= 0; i = prev[i] {
			stay[i] = true
		}
	}
	return stay
}
