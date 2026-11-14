//go:build js && wasm

package harmonyos

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/webstyle"
)

// mounter turns each mount batch into operations the ArkTS render layer
// applies to its tree of views: it keeps the views' frames, styles and text,
// as a batch sends what changed, and writes one operation for each.
type mounter struct {
	s     *surface
	views map[driver.NodeID]*view
	// order counts the inserts, so a press goes to the view drawn last
	// among those stacked.
	order uint32
	root  driver.NodeID
	o     ops
	anim  *driver.Animator
	// frame is called when a frame the engine asked for begins.
	frame func()
	// dropped records once the kinds this mounter has no case for, so the
	// missing view is said out loud instead of vanishing in a switch.
	dropped map[driver.ItemKind]bool
	// keys is the surface's keyboard shortcut table, which the page's
	// physical key events are matched against here.
	keys []driver.KeyBinding
	// focused is the view with the keyboard focus, for a shortcut that
	// must not fire while an input edits text.
	focused driver.NodeID
	// lastKey is the last key pressed without a release between, so its
	// auto-repeats are told apart from a fresh press.
	lastKey driver.NodeID
}

// view is what the render layer keeps for a view.
type view struct {
	id   driver.NodeID
	kind driver.ViewKind
	// parent, frame, order and pressable are what a press at a point needs:
	// the view's frame in its parent, the order it was inserted in, and
	// whether it asks for a press.
	parent    driver.NodeID
	frame     driver.Bounds
	order     uint32
	pressable bool
	style     driver.ViewStyle
	text      *driver.TextContent
	value     string
	// edits counts the user's edits, so a value the engine computed from an
	// older edit is dropped instead of overwriting what was typed since.
	edits   uint32
	drawing *driver.Drawing
	scroll  *driver.ScrollConfig
	// modal is how the modal this view presents appears, when it is one.
	modal   *driver.ModalConfig
	content driver.Size
	offset  driver.Point
	// applied counts the scrolls the engine asked for, which its event
	// answers carry.
	applied uint32
}

func newMounter(s *surface) *mounter {
	return &mounter{s: s, views: map[driver.NodeID]*view{}, dropped: map[driver.ItemKind]bool{}}
}

// mount applies a batch and hands its operations to the render layer.
func (m *mounter) mount(b *driver.MountBatch) {
	m.o.reset()
	if m.anim != nil {
		// The animator starts a run where its clock is, so bring it to now
		// before a batch adds one.
		m.s.clock.Step(m.s.clock.Elapsed())
	}
	if b.LayoutAnimation != nil && b.LayoutAnimation.Update != nil {
		u := b.LayoutAnimation.Update
		m.o.op("la", 0).num(float64(u.Duration.Milliseconds())).num(float64(u.Curve))
	}
	for i := range b.Items {
		m.item(b, &b.Items[i])
	}
	m.flush()
	if m.anim != nil && m.anim.Running() {
		m.s.RequestFrame()
	}
}

func (m *mounter) flush() {
	if !m.o.empty() {
		m.s.submit(m.o.bytes())
		m.o.reset()
	}
}

// applyAnimated writes what the animator moved this frame: each view's
// style with its animated values, and each canvas's drawing with its slots.
// It reports whether it wrote anything, which is a frame the screen shows.
func (m *mounter) applyAnimated() bool {
	if m.anim == nil {
		return false
	}
	changed := false
	m.anim.Changed(func(id driver.NodeID, _ driver.AnimatedValues) {
		if v := m.views[id]; v != nil {
			m.style(id, &v.style)
			changed = true
		}
	})
	m.anim.ChangedCanvases(func(id driver.NodeID) {
		if v := m.views[id]; v != nil && v.drawing != nil {
			m.o.op("cv", id).str(canvasJSON(m.anim.CanvasOps(id, v.drawing)))
			changed = true
		}
	})
	if changed {
		m.flush()
	}
	return changed
}

// item applies one mount item.
func (m *mounter) item(b *driver.MountBatch, it *driver.MountItem) {
	switch it.Kind {
	case driver.ItemCreate:
		v := &view{id: it.ID, kind: it.View, style: driver.DefaultViewStyle}
		m.views[it.ID] = v
		if m.anim != nil {
			m.anim.Touch(it.ID)
		}
		m.o.op("c", it.ID).num(float64(it.View))
		if it.ID == b.Root {
			m.root = it.ID
			m.o.op("root", it.ID)
		}
	case driver.ItemInsert:
		if v := m.views[it.ID]; v != nil {
			v.parent = it.Parent
			m.order++
			v.order = m.order
		}
		m.o.op("i", it.ID).num(float64(it.Parent)).num(float64(it.Before))
	case driver.ItemRemove:
		m.o.op("r", it.ID)
	case driver.ItemDelete:
		if m.anim != nil {
			m.anim.Forget(it.ID)
		}
		delete(m.views, it.ID)
		m.o.op("d", it.ID)
	case driver.ItemAnimation:
		if m.anim != nil {
			m.anim.Apply(it.Animation)
		}
	case driver.ItemFrame:
		if v := m.views[it.ID]; v != nil {
			v.frame = it.Frame
		}
		m.o.op("f", it.ID).num(float64(it.Frame.Origin.X)).num(float64(it.Frame.Origin.Y)).
			num(float64(it.Frame.Size.Width)).num(float64(it.Frame.Size.Height))
	case driver.ItemViewStyle:
		if v := m.views[it.ID]; v != nil {
			v.style = *it.ViewStyle
		}
		m.style(it.ID, it.ViewStyle)
	case driver.ItemText:
		if v := m.views[it.ID]; v != nil {
			v.text = it.Text
			if it.Text != nil && it.Text.Layout != nil {
				it.Text.Layout.Release()
			}
		}
		m.text(it.ID, it.Text)
	case driver.ItemInput:
		m.input(it.ID, it.Input)
	case driver.ItemCanvas:
		if v := m.views[it.ID]; v != nil {
			v.drawing = it.Drawing
		}
		var ops []driver.DrawOp
		if it.Drawing != nil {
			ops = it.Drawing.Ops
			if m.anim != nil && it.Drawing.Slots != nil {
				m.anim.BindCanvas(it.ID, it.Drawing.Slots)
				ops = m.anim.CanvasOps(it.ID, it.Drawing)
			}
		}
		m.o.op("cv", it.ID).str(canvasJSON(ops))
	case driver.ItemImage:
		src, fit, data := "", "cover", ""
		if it.Image != nil {
			src = it.Image.Source.URI
			if len(it.Image.Source.Data) > 0 {
				data = base64.StdEncoding.EncodeToString(it.Image.Source.Data)
			}
			switch it.Image.ResizeMode {
			case driver.Contain:
				fit = "contain"
			case driver.ScaleDown:
				fit = "scale-down"
			case driver.Stretch:
				fit = "fill"
			case driver.Center:
				fit = "none"
			}
		}
		m.o.op("im", it.ID).str(src).str(fit).str(data)
	case driver.ItemInputValue:
		// A value the engine computed before the user's latest edit is
		// stale; one the field already shows is not sent again, which would
		// move the caret.
		if v := m.views[it.ID]; v != nil {
			if it.Count < v.edits || v.value == it.Value {
				return
			}
			v.value = it.Value
		}
		m.o.op("iv", it.ID).str(it.Value)
	case driver.ItemScrollConfig:
		horizontal, hide, refreshable, refreshing := false, false, false, false
		var threshold float64
		zmin, zmax, zoom, pinch := float64(1), float64(1), float64(1), true
		if it.Scroll != nil {
			horizontal, hide = it.Scroll.Horizontal, it.Scroll.HideIndicator
			refreshable, refreshing = it.Scroll.Refreshable, it.Scroll.Refreshing
			threshold = float64(it.Scroll.Behavior.RefreshThreshold())
			zmin, zmax, zoom = float64(it.Scroll.MinZoomScale), float64(it.Scroll.MaxZoomScale), float64(it.Scroll.ZoomScale)
			pinch = !it.Scroll.NoPinchZoom
			if zmin <= 0 {
				zmin = 1
			}
			if zmax < zmin {
				zmax = zmin
			}
			if v := m.views[it.ID]; v != nil {
				v.scroll = it.Scroll
			}
		}
		m.o.op("sc", it.ID).boolean(horizontal).boolean(hide).
			boolean(refreshable).boolean(refreshing).num(threshold).
			num(zmin).num(zmax).num(zoom).boolean(pinch)
	case driver.ItemZoomTo:
		if it.Zoom != nil {
			cx, cy := float64(0), float64(0)
			if it.Zoom.Center != nil {
				cx, cy = float64(it.Zoom.Center.X), float64(it.Zoom.Center.Y)
			}
			m.o.op("zoom", it.ID).num(float64(it.Zoom.Scale)).num(cx).num(cy).boolean(it.Animated)
		}
	case driver.ItemScrollContent:
		if v := m.views[it.ID]; v != nil {
			v.content = it.Size
		}
		m.o.op("ss", it.ID).num(float64(it.Size.Width)).num(float64(it.Size.Height))
	case driver.ItemDirection:
		// The core mirrors the frames for a right-to-left app, so the
		// imperative tree has no global direction to set; alert labels
		// arrive localized. Read on every first commit, applied nowhere.
	case driver.ItemOverflow:
		// ArkUI hit-tests the whole node tree, so a child outside its
		// parent's bounds still takes presses — a child of a 0x0 parent
		// answered a tap on the device. UIKit, AppKit and Android need
		// this area to know where to look; here there is nothing to apply.
	case driver.ItemStickyCells:
		// Each cell's id and the offsets it travels between. The render
		// layer moves it by clamp(offset - Start, 0, End - Start) from its
		// frame, as Android's runtime does. An animated inset has no value
		// to follow here, so Start already counts it.
		type cell struct {
			Cell  driver.NodeID `json:"c"`
			Start driver.Points `json:"s"`
			End   driver.Points `json:"e"`
		}
		cells := make([]cell, 0, len(it.Sticky))
		for _, c := range it.Sticky {
			cells = append(cells, cell{Cell: c.Cell, Start: c.Start, End: c.End})
		}
		if data, err := json.Marshal(cells); err == nil {
			m.o.op("sticky", it.ID).str(string(data))
		}
	case driver.ItemScrollTo:
		m.scrollTo(it.ID, it.Point, it.Animated)
	case driver.ItemScrollBy:
		to := it.Point
		if v := m.views[it.ID]; v != nil {
			to = driver.Point{X: v.offset.X + it.Point.X, Y: v.offset.Y + it.Point.Y}
		}
		m.scrollTo(it.ID, to, false)
	case driver.ItemModal:
		anim, noSwipe, noBackdrop := driver.ModalFade, false, false
		if it.Modal != nil {
			if v := m.views[it.ID]; v != nil {
				v.modal = it.Modal
			}
			anim, noSwipe, noBackdrop = it.Modal.Animation, it.Modal.NoCloseOnSwipe, it.Modal.NoCloseOnBackdropPress
		}
		m.o.op("mo", it.ID).num(float64(anim)).boolean(noSwipe).boolean(noBackdrop)
	case driver.ItemAlert:
		if it.Alert != nil {
			m.alert(it.Alert)
		}
	case driver.ItemReadClipboard:
		m.s.bridge.Call("clipread", int(it.Clipboard), int(it.Count))
	case driver.ItemSetClipboard:
		m.s.bridge.Call("clipset", it.Value)
	case driver.ItemSetClipboardImage:
		if data, ok := it.Data.([]byte); ok && len(data) > 0 {
			m.s.bridge.Call("clipimage", base64.StdEncoding.EncodeToString(data))
		}
	case driver.ItemOpenURL:
		m.s.bridge.Call("openurl", int(it.Count), it.Value)
	case driver.ItemPickImages:
		limit := 1
		if it.Pick != nil && it.Pick.Limit > 0 {
			limit = it.Pick.Limit
		}
		m.s.bridge.Call("pickimages", int(it.Count), limit)
	case driver.ItemPickFiles:
		limit := 1
		if it.Files != nil && it.Files.Limit > 0 {
			limit = it.Files.Limit
		}
		m.s.bridge.Call("pickfiles", int(it.Count), limit)
	case driver.ItemPickDirectories:
		limit := 1
		if it.Directories != nil && it.Directories.Limit > 0 {
			limit = it.Directories.Limit
		}
		m.s.bridge.Call("pickdirectories", int(it.Count), limit)
	case driver.ItemSaveFile:
		m.saveFile(it.Save, it.Count)
	case driver.ItemVideo:
		if it.Video != nil {
			fit := "contain"
			switch it.Video.ResizeMode {
			case driver.Cover:
				fit = "cover"
			case driver.Stretch:
				fit = "fill"
			case driver.Center:
				fit = "none"
			case driver.ScaleDown:
				fit = "scale-down"
			}
			m.o.op("video", it.ID).str(it.Video.Source).boolean(it.Video.Paused).boolean(it.Video.Muted).
				boolean(it.Video.Repeat).boolean(it.Video.Controls).str(fit)
		}
	case driver.ItemVideoSeek:
		m.o.op("vseek", it.ID).num(it.Time.Seconds())
	case driver.ItemWeb:
		if it.Web != nil {
			m.o.op("web", it.ID).str(it.Web.Source.Src).boolean(it.Web.NoJavaScript)
		}
	case driver.ItemWebCommand:
		m.o.op("wcmd", it.ID).num(float64(it.WebCommand)).str(it.Value)
	case driver.ItemScreen:
		if c := it.Screen; c != nil {
			m.o.op("scr", it.ID).str(c.Title).boolean(c.HeaderHidden).
				boolean(c.CanGoBack && !c.BackHidden).str(c.BackTitle).
				num(float64(c.HeaderHeight)).boolean(c.HeaderTransparent).
				str(webstyle.Fill(c.Background)).str(webstyle.Fill(c.TitleColor)).
				str(webstyle.Fill(c.TintColor))
		}
	case driver.ItemMatch:
		// The created view shares the element the deleted view Before
		// showed: the render layer starts it at Before's frame and moves
		// it to its own over the batch's layout animation.
		m.o.op("match", it.ID).num(float64(it.Before))
	case driver.ItemKeymap:
		m.keys = append([]driver.KeyBinding(nil), it.Keys...)
		m.lastKey = 0
	case driver.ItemAppearance:
		m.o.op("app", 0).num(float64(it.Appearance))
	case driver.ItemStatusBar:
		if it.StatusBar != nil {
			m.o.op("sb", 0).num(float64(it.StatusBar.Style)).boolean(it.StatusBar.Hidden)
		}
	case driver.ItemTitle:
		// A phone has no window title: the task switcher's label is the
		// app's name.
		m.s.bridge.Call("title", it.Value)
	case driver.ItemAccessibilityLabel:
		m.o.op("al", it.ID).str(it.Value)
	case driver.ItemAnnounce:
		m.s.bridge.Call("announce", it.Value)
	case driver.ItemViewProps:
		if it.ViewProps != nil {
			m.o.op("vp", it.ID).str(it.ViewProps.TestID).num(float64(it.ViewProps.PointerEvents)).str(it.ViewProps.Label)
		}
	case driver.ItemImageInfo:
		if it.Image != nil {
			uri, data := it.Image.Source.URI, ""
			if len(it.Image.Source.Data) > 0 {
				data = base64.StdEncoding.EncodeToString(it.Image.Source.Data)
			}
			m.s.bridge.Call("imageinfo", int(it.Count), uri, data)
		}
	case driver.ItemSelection:
		m.o.op("sel", it.ID).num(float64(it.Selection.Start)).num(float64(it.Selection.End))
	case driver.ItemFocus:
		// The page reports the focus too; the engine's own request keeps the
		// mounter's idea of it right when no page event comes, as a test's
		// synthesized press has none.
		m.focused = it.ID
		m.o.op("fo", it.ID)
	case driver.ItemBlur:
		if m.focused == it.ID {
			m.focused = 0
		}
		m.o.op("bl", it.ID)
	case driver.ItemDismissKeyboard:
		m.s.bridge.Call("dismisskeyboard")
	case driver.ItemTakePhoto:
		m.s.bridge.Call("takephoto", int(it.Count))
	case driver.ItemOpenFile:
		if it.Open != nil {
			m.s.bridge.Call("openfile", int(it.Count), it.Open.Path, it.Open.MIME)
		}
	case driver.ItemBackUnhandled:
		// The engine found nothing to take the back gesture, so the host
		// lets the system have it and the app leaves.
		m.s.bridge.Call("backunhandled")
	case driver.ItemNotificationPermission:
		// No notification APIs are wired here, so the app is told it may
		// not show one rather than waiting for an answer.
		m.s.dispatch(driver.Event{Kind: driver.EventNotification, Count: it.Count, Index: int(driver.NotificationsDenied)})
	case driver.ItemShowNotification:
		m.s.dispatch(driver.Event{Kind: driver.EventNotification, Count: it.Count, Finished: false})
	case driver.ItemShare:
		// The SDK has no share sheet, so the app hears that rather than
		// waiting for an answer that never comes.
		m.s.dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Text: driver.NoShareSheet})
	case driver.ItemCheckForUpdates:
		// The SDK has no updater either: Android's is Play Core's.
		m.s.dispatch(driver.Event{Kind: driver.EventUpdate, Count: it.Count,
			Data: &driver.Update{Err: "the OpenHarmony SDK has no updater"}})
	case driver.ItemEvents:
		bits := it.Events
		if v := m.views[it.ID]; v != nil {
			// A field takes a press to focus even when the app asks for
			// none, as ArkUI's own hit test does.
			v.pressable = bits.Has(driver.EventPress) || v.kind == driver.KindInput || v.kind == driver.KindWeb
		}
		m.o.op("e", it.ID).
			boolean(bits.Has(driver.EventPress)).
			boolean(bits.Has(driver.EventLongPress)).
			boolean(bits.Has(driver.EventPressDown) || bits.Has(driver.EventPressUp) ||
				bits.Has(driver.EventTouch))
	default:
		// Every kind that reaches here is registered in the missing table of
		// cmd/nsgo/harmony/items_test.go, which fails when one is not. Say it
		// once on the device too, so a view that never draws has a trace.
		if !m.dropped[it.Kind] {
			m.dropped[it.Kind] = true
			log.Printf("nsgo harmony: mount item %v has no renderer yet (see items_test.go)", it.Kind)
		}
	}
}

// alert hands a native alert to the host, which shows it as the system's
// dialog and answers with EventAlert carrying the alert's ID and the button.
func (m *mounter) alert(a *driver.AlertConfig) {
	type action struct {
		Label string `json:"l"`
		Role  uint8  `json:"r"`
	}
	out := struct {
		ID      uint32   `json:"id"`
		Title   string   `json:"t"`
		Message string   `json:"m"`
		Sheet   bool     `json:"s"`
		Cancel  bool     `json:"c"`
		Actions []action `json:"a"`
	}{ID: a.ID, Title: a.Title, Message: a.Message, Sheet: a.Sheet, Cancel: a.Cancelable}
	for _, act := range a.Actions {
		out.Actions = append(out.Actions, action{Label: act.Label, Role: uint8(act.Role)})
	}
	data, err := json.Marshal(out)
	if err != nil {
		log.Printf("nsgo harmony: alert %d: %v", a.ID, err)
		return
	}
	m.s.bridge.Call("alert", string(data))
}

// saveFile hands a save to the host, which shows the system's save dialog
// where the user chose, and answers EventFiles.
func (m *mounter) saveFile(r *driver.SaveRequest, count uint32) {
	if r == nil {
		m.s.bridge.Call("saved", int(count), false, "no file")
		return
	}
	src, err := r.Open()
	if err != nil {
		m.s.bridge.Call("saved", int(count), false, err.Error())
		return
	}
	defer src.Close()
	data, err := io.ReadAll(src)
	if err != nil {
		m.s.bridge.Call("saved", int(count), false, err.Error())
		return
	}
	m.s.bridge.Call("savefile", int(count), r.Name, r.MIME, base64.StdEncoding.EncodeToString(data))
}

// style writes what a view draws.
func (m *mounter) style(id driver.NodeID, s *driver.ViewStyle) {
	if s == nil {
		return
	}
	anim, animated := driver.AnimatedValues{}, false
	if m.anim != nil {
		anim, animated = m.anim.Values(id)
	}
	vs := *s
	if animated {
		vs = anim.Apply(vs)
	}
	s = &vs
	radius := float64(s.CornerRadius)
	if s.Corners != (driver.Corners{}) {
		radius = float64(s.Corners.TopLeft)
	}
	border := float64(s.BorderWidth)
	borderColor := s.BorderColor
	if s.Borders != (driver.Edges{}) {
		border = float64(s.Borders.Top)
	}
	if s.BorderColors != (driver.EdgeColors{}) {
		borderColor = s.BorderColors.Top
	}
	opacity := float64(s.Opacity)
	if !animated && opacity == 0 && s.Background == 0 && radius == 0 && border == 0 && !s.Hidden {
		return
	}
	t := s.Transform
	sx, sy := t.ScaleX, t.ScaleY
	if sx == 0 {
		sx = t.Scale
	}
	if sy == 0 {
		sy = t.Scale
	}
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	m.o.op("s", id).str(webstyle.Color(s.Background)).num(radius).num(border).
		str(webstyle.Color(borderColor)).num(opacity).boolean(s.Hidden).
		num(float64(t.TranslateX)).num(float64(t.TranslateY)).
		num(float64(sx)).num(float64(sy)).num(float64(t.Rotate))
}

// text writes a text view's content, in its style.
func (m *mounter) text(id driver.NodeID, t *driver.TextContent) {
	if t == nil {
		m.o.op("t", id).null()
		return
	}
	st := t.Style
	f := st.Font
	if f.Size == 0 {
		f.Size = 16
	}
	if f.Weight == 0 {
		f.Weight = 400
	}
	align := 0
	switch st.Align {
	case driver.TextAlignCenter:
		align = 1
	case driver.TextAlignEnd:
		align = 2
	}
	m.o.op("t", id).str(t.Text).num(float64(f.Size)).num(float64(f.Weight)).
		boolean(f.Italic).str(f.Family).str(webstyle.Color(st.Color)).
		num(float64(st.LineHeight)).num(float64(align)).num(float64(t.MaxLines))
	if runs := textRunsJSON(t); runs != "" {
		m.o.op("tr", id).str(runs)
	}
}

// input writes a text field's place, look and value.
func (m *mounter) input(id driver.NodeID, c *driver.InputConfig) {
	if c == nil {
		return
	}
	f := c.Style.Font
	if f.Size == 0 {
		f.Size = 16
	}
	if f.Weight == 0 {
		f.Weight = 400
	}
	m.o.op("in", id).str(c.Placeholder).num(float64(f.Size)).num(float64(f.Weight)).
		str(webstyle.Color(c.Style.Color)).str(webstyle.Color(c.PlaceholderColor)).
		boolean(c.Masked).boolean(c.Multiline)
}

// scrollTo scrolls a scroll view to offset, as the app asked.
func (m *mounter) scrollTo(id driver.NodeID, offset driver.Point, animated bool) {
	if v := m.views[id]; v != nil {
		v.applied++
		v.offset = offset
	}
	m.o.op("so", id).num(float64(offset.X)).num(float64(offset.Y)).boolean(animated)
}

// scrolled reports where the user left scroll view id.
func (m *mounter) scrolled(id driver.NodeID, offset driver.Point) {
	v := m.views[id]
	if v == nil {
		return
	}
	v.offset = offset
	m.s.dispatch(driver.Event{Kind: driver.EventScroll, ID: id, Point: offset, Count: v.applied})
}

// zoomed reports scroll view id's zoom at scale, the offset where it
// left the content: the engine keeps the scale and the view's range.
func (m *mounter) zoomed(id driver.NodeID, scale float32, offset driver.Point) {
	v := m.views[id]
	if v == nil {
		return
	}
	v.offset = offset
	m.s.dispatch(driver.Event{Kind: driver.EventScroll, ID: id, Point: offset, Value: scale, Count: v.applied})
}

// scrollPhase reports a drag of scroll view id starting or ending, or its
// scrolling coming to rest.
func (m *mounter) scrollPhase(id driver.NodeID, kind driver.EventKind) {
	v := m.views[id]
	if v == nil {
		return
	}
	m.s.dispatch(driver.Event{Kind: kind, ID: id, Point: v.offset, Count: v.applied})
}

// refresh reports a pull past refreshable scroll view id's top, which
// starts its refresh.
func (m *mounter) refresh(id driver.NodeID) {
	v := m.views[id]
	if v == nil {
		return
	}
	m.s.dispatch(driver.Event{Kind: driver.EventRefresh, ID: id, Point: v.offset, Count: v.applied})
}

// edited reports the user changing text field id's text, and remembers the
// value and its count so the engine's later values can be judged stale.
func (m *mounter) edited(id driver.NodeID, text string) {
	if v := m.views[id]; v != nil {
		v.edits++
		v.value = text
		m.s.dispatch(driver.Event{Kind: driver.EventChangeText, ID: id, Text: text, Count: v.edits})
	}
}

// pressTarget returns the view a press at p goes to: the deepest view
// that asks for a press and holds p, the latest inserted among equally
// deep ones. A page gets the view from ArkUI's hit test; a test has only
// the point, so it asks the frames the engine sent.
func (m *mounter) pressTarget(p driver.Point) driver.NodeID {
	var best *view
	bestDepth := -1
	for _, v := range m.views {
		if !v.pressable {
			continue
		}
		b, depth := m.surfaceBounds(v)
		if p.X < b.Origin.X || p.Y < b.Origin.Y || p.X >= b.Origin.X+b.Size.Width || p.Y >= b.Origin.Y+b.Size.Height {
			continue
		}
		if best == nil || depth > bestDepth || (depth == bestDepth && v.order > best.order) {
			best, bestDepth = v, depth
		}
	}
	if best == nil {
		return 0
	}
	return best.id
}

// surfaceBounds returns v's frame on the page and how deep it is: its
// parents' frames added, with each scroll view's offset taken off.
func (m *mounter) surfaceBounds(v *view) (driver.Bounds, int) {
	b := v.frame
	depth := 0
	seen := map[driver.NodeID]bool{v.id: true}
	for p := m.views[v.parent]; p != nil && !seen[p.id]; p = m.views[p.parent] {
		seen[p.id] = true
		b.Origin.X += p.frame.Origin.X
		b.Origin.Y += p.frame.Origin.Y
		if p.kind == driver.KindScroll {
			b.Origin.X -= p.offset.X
			b.Origin.Y -= p.offset.Y
		}
		depth++
	}
	return b, depth
}

// scrollTarget returns the deepest scroll view that holds p, which a
// test's scroll drives: ArkUI scrolls its own view, so a synthesized
// touch does not move it.
func (m *mounter) scrollTarget(p driver.Point) driver.NodeID {
	var best *view
	bestDepth := -1
	for _, v := range m.views {
		if v.kind != driver.KindScroll {
			continue
		}
		b, depth := m.surfaceBounds(v)
		if p.X < b.Origin.X || p.Y < b.Origin.Y || p.X >= b.Origin.X+b.Size.Width || p.Y >= b.Origin.Y+b.Size.Height {
			continue
		}
		if best == nil || depth > bestDepth {
			best, bestDepth = v, depth
		}
	}
	if best == nil {
		return 0
	}
	return best.id
}

// press reports a press on id at page.
func (m *mounter) press(id driver.NodeID, kind driver.EventKind, page driver.Point) {
	m.s.dispatch(driver.Event{Kind: kind, ID: id, Point: page, Page: page})
}

// editing says whether a text input or a web view has the keyboard focus,
// which keeps a shortcut from firing over a key the field edits.
func (m *mounter) editing() bool {
	v := m.views[m.focused]
	return v != nil && (v.kind == driver.KindInput || v.kind == driver.KindWeb)
}

// key matches a physical key the page reported against the surface's
// keymap, as Android's dispatchKeyEvent does: a press names the key with
// and without "mod" for Control, and FindKey decides what fires.
func (m *mounter) key(code driver.NodeID, action, mods int, text string) {
	if action == 1 { // a release clears the held key
		m.lastKey = 0
		return
	}
	if len(m.keys) == 0 {
		return
	}
	repeat := m.lastKey == code
	m.lastKey = code
	name := keyName(code, text)
	if name == "" {
		return
	}
	rest := ""
	if mods&2 != 0 {
		rest += "alt+"
	}
	if mods&4 != 0 {
		rest += "shift+"
	}
	if mods&8 != 0 {
		rest += "meta+"
	}
	rest += name
	names := []string{rest, rest}
	if mods&1 != 0 {
		names = []string{"mod+" + rest, "ctrl+" + rest}
	}
	if key, _ := driver.FindKey(m.keys, names, m.editing(), repeat); key != "" {
		m.s.dispatch(driver.Event{Kind: driver.EventKey, Text: key})
	}
}

// keyName returns the core's name for an OHOS KeyCode, or for the Linux
// input code the emulator's uinput passes through as a keyCode.
func keyName(code driver.NodeID, text string) string {
	switch {
	case code >= 2017 && code <= 2042: // KEYCODE_A .. KEYCODE_Z
		return string(rune('a' + int(code) - 2017))
	case code >= 2000 && code <= 2009: // KEYCODE_0 .. KEYCODE_9
		return string(rune('0' + int(code) - 2000))
	case code >= 2090 && code <= 2101: // KEYCODE_F1 .. KEYCODE_F12
		return fmt.Sprintf("f%d", int(code)-2089)
	}
	switch code {
	case 2012:
		return "up"
	case 2013:
		return "down"
	case 2014:
		return "left"
	case 2015:
		return "right"
	case 2049:
		return "tab"
	case 2050:
		return "space"
	case 2054:
		return "enter"
	case 2055:
		return "backspace"
	case 2068:
		return "pageup"
	case 2069:
		return "pagedown"
	case 2070:
		return "escape"
	case 2071:
		return "delete"
	case 2081:
		return "home"
	case 2082:
		return "end"
	}
	if name := linuxKeys[code]; name != "" {
		return name
	}
	// Other keys by the character they type, such as "/".
	if text != "" && text != "KEYCODE_INVALID" && len([]rune(text)) == 1 {
		return strings.ToLower(text)
	}
	return ""
}

// linuxKeys are the Linux input event codes the emulator's uinput injects;
// a real keyboard reports OHOS KeyCodes instead.
var linuxKeys = map[driver.NodeID]string{
	1: "escape", 2: "1", 3: "2", 4: "3", 5: "4", 6: "5", 7: "6", 8: "7", 9: "8", 10: "9", 11: "0",
	14: "backspace", 15: "tab", 16: "q", 17: "w", 18: "e", 19: "r", 20: "t",
	21: "y", 22: "u", 23: "i", 24: "o", 25: "p", 28: "enter",
	30: "a", 31: "s", 32: "d", 33: "f", 34: "g", 35: "h", 36: "j", 37: "k", 38: "l",
	44: "z", 45: "x", 46: "c", 47: "v", 48: "b", 49: "n", 50: "m", 57: "space",
	59: "f1", 60: "f2", 61: "f3", 62: "f4", 63: "f5", 64: "f6", 65: "f7",
	66: "f8", 67: "f9", 68: "f10", 87: "f11", 88: "f12",
	102: "home", 103: "up", 104: "pageup", 105: "left", 106: "right",
	107: "end", 108: "down", 109: "pagedown", 111: "delete",
}
