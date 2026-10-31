package nsgotest

import (
	"bytes"
	"fmt"
	"image/png"
	"slices"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// View is the model of one native view, as the last batch left it.
type View struct {
	ID       driver.NodeID
	Kind     driver.ViewKind
	Parent   *View
	Children []*View
	Frame    driver.Bounds
	Style    driver.ViewStyle
	Props    driver.ViewProps
	Events   driver.EventMask
	Text     string // a text view's text or a field's value
	// TextStyle and Spans are a text view's resolved style and its
	// spans', which SpanStyle reads.
	TextStyle driver.TextStyle
	Spans     []driver.TextSpan
	// Hovered says the mouse is over the view, as Hover left it.
	Hovered bool
	Input   driver.InputConfig
	Content driver.Size  // a scroll view's content
	Offset  driver.Point // and its offset
	// Applied counts the ItemScrollTo and ItemScrollBy items the scroll
	// view applied, which its reports carry; Sticky are its sticky
	// cells.
	Applied uint32
	Sticky  []driver.StickyCell
	// Scroll is a scroll view's configuration, and ZoomScale its zoom
	// (0 until it zooms).
	Scroll    driver.ScrollConfig
	ZoomScale float32
	Image     driver.ImageConfig
	Web       driver.WebConfig
	// WebCommands are what the app asked a web view to do, oldest first.
	WebCommands []WebCommand
	Video       driver.VideoConfig
	// Seeks are the positions the app asked a video to jump to, oldest
	// first.
	Seeks []time.Duration
	// Component and NativeProps are a native view's component and props,
	// an animated one at the value the frame clock last handed over;
	// NativeCommands what Ref.Command sent it, oldest first.
	Component      string
	NativeProps    map[string]any
	NativeCommands []NativeCommand
	// Animated are the values of the animated properties bound to the
	// view, which the model's animator steps with the clock.
	Animated driver.AnimatedValues
	Focused  bool
	// Selection is a field's selection, as the app last set it or Select
	// moved it, in UTF-16 units.
	Selection driver.TextRange
	// Composing is the text an input method composes in a field (Compose);
	// held is a value the app set meanwhile, applied when it ends.
	Composing string
	held      *driver.MountItem
	// Screen is a screen's system header, sent where the environment
	// has HeaderHeight; Modal a modal's config.
	Screen driver.ScreenConfig
	Modal  driver.ModalConfig
	// Drawing is a canvas's last drawing, and Ops its operations as the
	// platform plays them now, with the values of the animated nodes its
	// arguments are bound to.
	Drawing *driver.Drawing
	Ops     []driver.DrawOp
	// Node is the part of a drawing a view of a canvas's names: a canvas
	// shows each of its drawing's nodes as a child, which selectors find
	// by its label (label: and text:), test ID and role, and which presses
	// at its center.
	Node *driver.CanvasNode
	anim *driver.Animator
}

// clamp keeps a scroll view's offset within its content and insets, as
// platforms do.
func (v *View) clamp(p driver.Point) driver.Point {
	in := v.Scroll.Behavior.ContentInset
	p.X = min(max(p.X, -in.Left), max(v.Content.Width-v.Frame.Size.Width, 0)+in.Right)
	p.Y = min(max(p.Y, -in.Top), max(v.Content.Height-v.Frame.Size.Height, 0)+in.Bottom)
	return p
}

// StickyShift is how far the sticky cell id of scroll view v moves from
// its frame at v's offset, as a platform moves it.
func (v *View) StickyShift(id driver.NodeID) driver.Points {
	for _, c := range v.Sticky {
		if c.Cell == id {
			if v.Scroll.Horizontal {
				return v.anim.StickyShift(c, v.Offset.X)
			}
			return v.anim.StickyShift(c, v.Offset.Y)
		}
	}
	return 0
}

// NativeCommand is a command a native view received.
type NativeCommand struct {
	Name string
	Args any
}

// WebCommand is a command a web view received, with its message or script.
type WebCommand struct {
	Command driver.WebCommand
	Value   string
}

// animate steps the animator to the clock and records the values it
// moved.
func (a *App) animate() {
	if a.anim.Running() {
		a.anim.Step(a.Clock.Now().Sub(a.start))
	}
	a.anim.Changed(func(id driver.NodeID, vals driver.AnimatedValues) {
		if v := a.views[id]; v != nil {
			v.Animated = vals
		}
	})
	a.anim.ChangedCanvases(func(id driver.NodeID) {
		if v := a.views[id]; v != nil && v.Drawing != nil {
			v.Ops = slices.Clone(a.anim.CanvasOps(id, v.Drawing))
		}
	})
	a.anim.ChangedNatives(func(id driver.NodeID, b driver.NativeBinding, num float64, col driver.Color) {
		if v := a.views[id]; v != nil {
			if v.NativeProps == nil {
				v.NativeProps = map[string]any{}
			}
			if b.Color {
				v.NativeProps[b.Name] = col
			} else {
				v.NativeProps[b.Name] = num
			}
		}
	})
}

// drawCanvas keeps canvas v's drawing, binds its slots and shows its
// nodes as children.
func (a *App) drawCanvas(v *View, d *driver.Drawing) {
	if v == nil {
		return
	}
	v.Drawing = d
	a.anim.BindCanvas(v.ID, d.Slots)
	v.Ops = slices.Clone(a.anim.CanvasOps(v.ID, d))
	kept := v.Children[:0]
	for _, c := range v.Children {
		if c.Node == nil {
			kept = append(kept, c)
		}
	}
	v.Children = kept
	for i := range d.Nodes {
		n := &d.Nodes[i]
		v.Children = append(v.Children, &View{Kind: driver.KindView, Parent: v, Frame: n.Bounds, Node: n,
			Style: driver.ViewStyle{Opacity: 1},
			Props: driver.ViewProps{Role: n.Role, Label: n.Label, TestID: n.TestID, Selected: n.Selected}})
	}
}

func (a *App) dispatch(ev driver.Event) {
	if a.target != nil {
		// What the test makes up, a real platform reports itself.
		a.t.Skipf("nsgotest: the test reports what a platform would (%s): it runs headless only", a.stepName)
	}
	a.tree.Dispatch(ev)
	a.Settle()
}

// send dispatches ev to v when v reports its kind, as platforms do.
func (a *App) send(v *View, ev driver.Event) {
	if v.Events.Has(ev.Kind) {
		ev.ID = v.ID
		a.dispatch(ev)
	}
}

func (a *App) apply(b *driver.MountBatch) {
	a.rootID = b.Root
	if b.LayoutAnimation != nil {
		a.LayoutAnimation = b.LayoutAnimation
	}
	for _, it := range b.Items {
		v := a.views[it.ID]
		switch it.Kind {
		case driver.ItemCreate:
			a.views[it.ID] = &View{ID: it.ID, Kind: it.View, Style: driver.ViewStyle{Opacity: 1}, anim: a.anim}
		case driver.ItemDelete:
			if v != nil {
				v.detach()
			}
			a.anim.Forget(it.ID)
			delete(a.views, it.ID)
			delete(a.edits, it.ID)
		case driver.ItemRemove:
			v.detach()
		case driver.ItemInsert:
			v.detach()
			p := a.views[it.Parent]
			at := len(p.Children)
			for i, c := range p.Children {
				if c.ID == it.Before {
					at = i
				}
			}
			p.Children = append(p.Children[:at], append([]*View{v}, p.Children[at:]...)...)
			v.Parent = p
		case driver.ItemFrame:
			v.Frame = it.Frame
		case driver.ItemViewStyle:
			v.Style = *it.ViewStyle
		case driver.ItemViewProps:
			v.Props = *it.ViewProps
		case driver.ItemAccessibilityLabel:
			v.Props.Label = it.Value
		case driver.ItemAccessibilityFocus:
			a.AccessibilityFocus = v
		case driver.ItemEvents:
			v.Events = it.Events
		case driver.ItemText:
			v.Text, v.TextStyle, v.Spans = it.Text.Text, it.Text.Style, it.Text.Spans
		case driver.ItemInput:
			v.Input = *it.Input
		case driver.ItemInputValue:
			if v.Composing != "" {
				held := it
				v.held = &held
			} else if a.edits[it.ID] <= it.Count {
				v.Text = it.Value
			}
		case driver.ItemSelection:
			n := driver.UTF16Len(v.Text)
			r := it.Selection
			v.Selection = driver.TextRange{Start: min(max(r.Start, 0), n), End: min(max(r.End, 0), n)}
		case driver.ItemScrollContent:
			v.Content = it.Size
		case driver.ItemScrollConfig:
			if !v.Scroll.Zooms() && it.Scroll.Zooms() || it.Scroll.ZoomScale != v.Scroll.ZoomScale {
				v.ZoomScale = it.Scroll.ZoomScale
			}
			v.Scroll = *it.Scroll
			a.pullValue(v)
		case driver.ItemZoomTo:
			v.ZoomScale = it.Zoom.Scale
			id, at, z := it.ID, v.Offset, v.ZoomScale
			a.post(driver.Event{Kind: driver.EventScroll, ID: id, Point: at, Value: z})
		case driver.ItemImage:
			v.Image = *it.Image
		case driver.ItemWeb:
			v.Web = *it.Web
		case driver.ItemWebCommand:
			v.WebCommands = append(v.WebCommands, WebCommand{it.WebCommand, it.Value})
		case driver.ItemVideo:
			v.Video = *it.Video
		case driver.ItemVideoSeek:
			v.Seeks = append(v.Seeks, it.Time)
		case driver.ItemNative:
			v.Component = it.Native.Component
			if v.NativeProps == nil {
				v.NativeProps = map[string]any{}
			}
			for k, p := range it.Native.Props {
				if p == nil {
					delete(v.NativeProps, k)
				} else {
					v.NativeProps[k] = p
				}
			}
			if it.Native.Bind {
				a.anim.BindNative(it.ID, it.Native.Bindings)
			}
		case driver.ItemNativeCommand:
			v.NativeCommands = append(v.NativeCommands, NativeCommand{it.Value, it.Data})
		case driver.ItemCanvas:
			a.drawCanvas(v, it.Drawing)
		case driver.ItemAnimation:
			a.anim.Apply(it.Animation)
		case driver.ItemFocus:
			for _, o := range a.views {
				if o != v {
					a.setFocus(o, false)
				}
			}
			a.setFocus(v, true)
		case driver.ItemBlur:
			a.setFocus(v, false)
		case driver.ItemScrollTo, driver.ItemScrollBy:
			v.Applied++
			if it.Kind == driver.ItemScrollBy {
				v.Offset.X += it.Point.X
				v.Offset.Y += it.Point.Y
			} else {
				v.Offset = v.clamp(it.Point)
			}
			// A platform reports where a scroll lands.
			id, at, n := it.ID, v.Offset, v.Applied
			a.post(driver.Event{Kind: driver.EventScroll, ID: id, Point: at, Count: n})
		case driver.ItemStickyCells:
			v.Sticky = it.Sticky
		case driver.ItemSetClipboard:
			a.Clipboard, a.ClipboardImage, a.ClipboardFiles = it.Value, nil, nil
		case driver.ItemSetClipboardImage:
			data, _ := it.Data.([]byte)
			a.Clipboard, a.ClipboardImage, a.ClipboardFiles = "", data, nil
		case driver.ItemReadClipboard:
			a.readClipboard(&it)
		case driver.ItemOpenURL:
			a.OpenedURLs = append(a.OpenedURLs, it.Value)
			opened := a.OpenURL == nil || a.OpenURL(it.Value)
			a.post(driver.Event{Kind: driver.EventOpenURL, Count: it.Count, Finished: opened})
		case driver.ItemPickImages:
			a.Picks = append(a.Picks, *it.Pick)
			var files []driver.PickedFile
			if a.PickImages != nil {
				files = a.PickImages(*it.Pick)
			}
			a.post(driver.Event{Kind: driver.EventPick, Count: it.Count, Data: files})
		case driver.ItemAppearance:
			a.Appearance = it.Appearance
		case driver.ItemQuit:
			a.Quits++
		case driver.ItemPopupMenu:
			if a.popupID != 0 {
				a.post(driver.Event{Kind: driver.EventMenuClosed, Count: a.popupID})
			}
			a.PopupMenu, a.PopupAt, a.popupID = it.Menu, it.Point, it.Count
		case driver.ItemImageInfo:
			ev := driver.Event{Kind: driver.EventImageInfo, Count: it.Count}
			if size, ok := a.ImageSizes[it.Image.Source.URI]; ok {
				ev.Size = size
			} else {
				ev.Text = "nsgotest: no image at " + it.Image.Source.URI
			}
			a.post(ev)
		case driver.ItemKeymap:
			a.Keymap = a.Keymap[:0]
			for _, b := range it.Keys {
				if !slices.Contains(a.Keymap, b.Key) {
					a.Keymap = append(a.Keymap, b.Key)
				}
			}
		case driver.ItemTitle:
			a.Title = it.Value
		case driver.ItemStatusBar:
			a.StatusBar = *it.StatusBar
		case driver.ItemScreen:
			v.Screen = *it.Screen
		case driver.ItemModal:
			v.Modal = *it.Modal
		case driver.ItemMenu:
			a.Menu = it.Menu
		case driver.ItemWindow:
			a.Window = append(a.Window, *it.Window)
		case driver.ItemAnnounce:
			a.Announced = append(a.Announced, it.Value)
		case driver.ItemAlert:
			a.alerts = append(a.alerts, *it.Alert)
		case driver.ItemBlockNative:
			// The touch responder keeps native drags from starting.
			a.blocked = it.ID != 0
		case driver.ItemBackUnhandled:
			a.BackUnhandled++
		case driver.ItemDismissKeyboard:
			for _, o := range a.views {
				a.setFocus(o, false)
			}
		}
	}
}

// post has ev reach the app when it next settles, as a platform's answer
// does.
func (a *App) post(ev driver.Event) {
	if a.target != nil {
		// The platform answers itself.
		return
	}
	a.mu.Lock()
	a.posts = append(a.posts, func() { a.tree.Dispatch(ev) })
	a.mu.Unlock()
}

// readClipboard answers a clipboard read from the model's clipboard.
func (a *App) readClipboard(it *driver.MountItem) {
	ev := driver.Event{Kind: driver.EventClipboard, Count: it.Count}
	switch it.Clipboard {
	case driver.ClipboardReadText:
		ev.Text = a.Clipboard
	case driver.ClipboardReadHasText:
		ev.Finished = a.Clipboard != ""
	case driver.ClipboardReadImage:
		if len(a.ClipboardImage) > 0 {
			img := &driver.ClipboardImage{PNG: a.ClipboardImage}
			if c, err := png.DecodeConfig(bytes.NewReader(a.ClipboardImage)); err == nil {
				img.Size = driver.Size{Width: driver.Points(c.Width), Height: driver.Points(c.Height)}
			}
			ev.Data = img
		}
	case driver.ClipboardReadHasImage:
		ev.Finished = len(a.ClipboardImage) > 0
	case driver.ClipboardReadFiles:
		ev.URIs = slices.Clone(a.ClipboardFiles)
	}
	a.post(ev)
}

// setFocus moves focus to or from v and, as a platform does, reports the
// change to the app.
func (a *App) setFocus(v *View, on bool) {
	if v.Focused == on {
		return
	}
	v.Focused = on
	kind := driver.EventBlur
	if on {
		kind = driver.EventFocus
	}
	a.post(driver.Event{Kind: kind, ID: v.ID})
}

func (v *View) detach() {
	if p := v.Parent; p != nil {
		for i, c := range p.Children {
			if c == v {
				p.Children = append(p.Children[:i], p.Children[i+1:]...)
				break
			}
		}
		v.Parent = nil
	}
}

// Dump returns the visible views as an indented outline, for failure
// messages and golden files.
func (a *App) Dump() string {
	var b strings.Builder
	var walk func(*View, int)
	walk = func(v *View, depth int) {
		if v.Style.Hidden {
			return
		}
		f := v.Frame
		fmt.Fprintf(&b, "%s%s %g,%g %gx%g", strings.Repeat("  ", depth), v.Kind,
			f.Origin.X, f.Origin.Y, f.Size.Width, f.Size.Height)
		if r := roleName(v.Props.Role); r != "" && r != "text" && r != "textbox" {
			fmt.Fprintf(&b, " role=%s", r)
		}
		if v.Props.TestID != "" {
			fmt.Fprintf(&b, " id=%s", v.Props.TestID)
		}
		if v.Kind == driver.KindText || v.Kind == driver.KindInput {
			fmt.Fprintf(&b, " %q", v.Text)
		}
		b.WriteByte('\n')
		for _, c := range v.Children {
			walk(c, depth+1)
		}
	}
	if r := a.views[a.rootID]; r != nil {
		walk(r, 0)
	}
	return b.String()
}

// center returns the middle of v in its own and in surface points.
func (v *View) center() (local, page driver.Point) {
	at := v.surfaceFrame()
	local = driver.Point{X: at.Size.Width / 2, Y: at.Size.Height / 2}
	return local, driver.Point{X: at.Origin.X + local.X, Y: at.Origin.Y + local.Y}
}

// surfaceFrame is v's frame in surface points, as its ancestors and their
// scroll offsets place it; a modal's views are placed over the window.
func (v *View) surfaceFrame() driver.Bounds {
	b := v.Frame
	for p := v.Parent; p != nil; p = p.Parent {
		b.Origin.X += p.Frame.Origin.X - p.Offset.X
		b.Origin.Y += p.Frame.Origin.Y - p.Offset.Y
	}
	return b
}

// within reports whether v is o or inside it.
func (v *View) within(o *View) bool {
	for p := v; p != nil; p = p.Parent {
		if p == o {
			return true
		}
	}
	return false
}
