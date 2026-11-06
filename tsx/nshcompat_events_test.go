package tsx_test

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

// compatPayload is an event step's payload: key=value pairs, a key that
// repeats (uris) keeping each value.
type compatPayload map[string][]string

func compatPayloadOf(fields []string) compatPayload {
	p := compatPayload{}
	for _, f := range fields {
		k, v, _ := strings.Cut(f, "=")
		if uq, err := strconv.Unquote(v); err == nil {
			v = uq
		}
		p[k] = append(p[k], v)
	}
	return p
}

func (p compatPayload) str(k string) string {
	if v := p[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (p compatPayload) num(k string) float32 {
	f, _ := strconv.ParseFloat(p.str(k), 32)
	return float32(f)
}

func (p compatPayload) pt(x, y string) driver.Point {
	return driver.Point{X: driver.Points(p.num(x)), Y: driver.Points(p.num(y))}
}

func (p compatPayload) modifiers() driver.Modifiers {
	return driver.Modifiers{Control: p.str("modifiers.control") == "true", Alt: p.str("modifiers.alt") == "true",
		Shift: p.str("modifiers.shift") == "true", Platform: p.str("modifiers.platform") == "true"}
}

func (p compatPayload) ms(k string) time.Duration {
	return time.Duration(float64(p.num(k)) * float64(time.Second))
}

// compatButtons are the mouse buttons a mouse event's payload names.
var compatButtons = map[string]driver.MouseButton{"left": driver.MouseButtonLeft, "right": driver.MouseButtonRight,
	"middle": driver.MouseButtonMiddle, "back": driver.MouseButtonBack, "forward": driver.MouseButtonForward}

// compatHandlers are the event kinds of NSH's handlers a platform reports.
var compatHandlers = map[string]driver.EventKind{
	"onPressDown": driver.EventPressDown, "onPressUp": driver.EventPressUp, "onMouseEnter": driver.EventMouseEnter,
	"onMouseLeave": driver.EventMouseLeave, "onMouseMove": driver.EventMouseMove,
	"onContextMenu": driver.EventContextMenu, "onFocus": driver.EventFocus, "onBlur": driver.EventBlur,
	"onChange": driver.EventChangeText, "onSubmit": driver.EventSubmitEditing,
	"onSelectionChange": driver.EventSelectionChange, "onCompositionChange": driver.EventCompositionChange,
	"onCaretMove": driver.EventCaretMove, "onPaste": driver.EventPaste,
	"onAccessibilityAction": driver.EventAccessibilityAction, "onLoadStart": driver.EventLoadStart,
	"onLoad": driver.EventLoad, "onLoadEnd": driver.EventLoadEnd, "onError": driver.EventError,
	"onProgress": driver.EventProgress, "onBuffer": driver.EventBuffer, "onEnd": driver.EventEnd,
	"onMessage": driver.EventMessage, "onNavigationStateChange": driver.EventNavigationStateChange,
	"onPanStart": driver.EventPan, "onPanEnd": driver.EventPan, "onPinchStart": driver.EventPinch,
	"onPinchEnd": driver.EventPinch, "onRotationStart": driver.EventRotation, "onRotationEnd": driver.EventRotation,
	"onScrollDragStart": driver.EventScrollDragStart, "onScrollDragEnd": driver.EventScrollDragEnd,
	"onMomentumStart": driver.EventMomentumStart, "onMomentumEnd": driver.EventMomentumEnd,
	"onScrollEnd": driver.EventScrollEnd, "onRefresh": driver.EventRefresh,
	"onFileDragEnter": driver.EventFileDragEnter, "onFileDragLeave": driver.EventFileDragLeave,
	"onFileDrop": driver.EventFileDrop, "onCancel": driver.EventBack, "onBackPress": driver.EventBack,
	"onInsets": driver.EventScreenInsets, "onDismiss": driver.EventRequestClose,
	"onOpen": driver.EventModalShown, "onClose": driver.EventModalDismissed, "onDownload": driver.EventDownload,
}

// compatEvent reports handler's event, with payload, to the view selector
// names, or the nearest view around it that hears it, as the platform
// would.
func compatEvent(app *nsgotest.App, selector, handler string, payload []string) error {
	kind, ok := compatHandlers[handler]
	if !ok {
		return fmt.Errorf("no event %s known to the runner", handler)
	}
	p := compatPayloadOf(payload)
	if handler == "onCancel" && p.str("reason") == "swipe" {
		kind = driver.EventRequestClose
	}
	if handler == "onDismiss" && len(app.QueryMounted(compatSelectorOnly(selector))) == 0 {
		// NSGo's core pops a screen on back and reports it dismissed
		// itself: the app removed it already.
		return nil
	}
	v := compatView(app, selector)
	if kind == driver.EventScreenInsets {
		// A dialog's insets come with its size.
		for m := v; m != nil; m = m.Parent {
			if m.Kind == driver.KindModal {
				kind = driver.EventModalSize
				break
			}
		}
	}
	if kind == driver.EventBack {
		// A modal's window hears its own back, asking to close it.
		for m := v; m != nil; m = m.Parent {
			if m.Kind == driver.KindModal {
				app.Report(driver.Event{Kind: driver.EventRequestClose, ID: m.ID, Text: "cancel"})
				return nil
			}
		}
		// NSGo's platforms report Escape and back to the core, which
		// gives them to the overlay or the screen of the view they name,
		// else to the topmost overlay.
		id := v.ID
		for s := v; s != nil; s = s.Parent {
			if s.Screen != (driver.ScreenConfig{}) {
				id = s.ID
				break
			}
		}
		app.Report(driver.Event{Kind: kind, ID: id})
		return nil
	}
	for v != nil && !compatHears(v, kind) {
		v = v.Parent
	}
	if v == nil {
		return fmt.Errorf("no view at %s hears %s", selector, handler)
	}
	ev := driver.Event{Kind: kind, ID: v.ID}
	switch handler {
	case "onMouseEnter", "onMouseLeave", "onMouseMove", "onContextMenu":
		ev.Point, ev.Modifiers = p.pt("position.x", "position.y"), p.modifiers()
		at := compatPageOrigin(v)
		ev.Page = driver.Point{X: at.X + ev.Point.X, Y: at.Y + ev.Point.Y}
		if p.str("pageX") != "" {
			ev.Page = p.pt("pageX", "pageY")
			ev.Point = driver.Point{X: ev.Page.X - at.X, Y: ev.Page.Y - at.Y}
		}
		ev.Button = compatButtons[p.str("pressedButton")]
	case "onChange", "onSubmit", "onPaste", "onAccessibilityAction", "onError", "onMessage":
		ev.Text = p.str("text") + p.str("actionName") + p.str("message") + p.str("data")
		if handler == "onPaste" {
			ev.Data = &driver.PasteEvent{Text: ev.Text}
		}
	case "onSelectionChange":
		ev.Index, ev.Count = int(p.num("start")), uint32(p.num("end"))
	case "onCompositionChange":
		ev.Text, ev.Finished = p.str("text"), p.str("composing") == "false"
	case "onLoad", "onLoadStart", "onNavigationStateChange":
		switch {
		case p.str("url") != "" || v.Kind == driver.KindWeb:
			ev.Navigation = &driver.WebNavigation{URL: p.str("url"), Title: p.str("title"),
				Loading: p.str("loading") == "true", CanGoBack: p.str("canGoBack") == "true",
				CanGoForward: p.str("canGoForward") == "true"}
		default:
			ev.Size = driver.Size{Width: driver.Points(p.num("width")), Height: driver.Points(p.num("height"))}
			ev.Duration = p.ms("duration")
		}
	case "onProgress":
		ev.Loaded, ev.Total = int64(p.num("loaded")), int64(p.num("total"))
		ev.Time, ev.Duration = p.ms("currentTime"), p.ms("duration")
	case "onBuffer":
		ev.Buffering = p.str("isBuffering") == "true"
	case "onPanStart", "onPanEnd":
		ev.Phase = compatPhase(handler)
		ev.Point, ev.Velocity = p.pt("translationX", "translationY"), p.pt("velocityX", "velocityY")
	case "onPinchStart", "onPinchEnd", "onRotationStart", "onRotationEnd":
		ev.Phase = compatPhase(handler)
		ev.Value, ev.Velocity.X = p.num("scale")+p.num("rotation"), driver.Points(p.num("velocity"))
	case "onScrollDragStart", "onScrollDragEnd", "onMomentumStart", "onMomentumEnd", "onScrollEnd":
		ev.Point, ev.Count = p.pt("x", "y"), v.Applied
	case "onRefresh":
		// Where the scroll view is, as a pull leaves it.
		ev.Point, ev.Count = v.Offset, v.Applied
	case "onDownload":
		ev.Data = &driver.WebDownload{State: map[string]driver.WebDownloadState{"started": driver.WebDownloadStarted,
			"finished": driver.WebDownloadFinished, "failed": driver.WebDownloadFailed}[p.str("state")],
			ID: p.str("id"), URL: p.str("url"), Filename: p.str("filename"), Path: p.str("path"),
			Bytes: int64(p.num("bytes")), Message: p.str("message")}
	case "onFileDrop":
		ev.URIs = p["uris"]
	case "onCancel", "onDismiss":
		ev.Text = p.str("reason")
	case "onInsets":
		ev.Insets = driver.Edges{Top: driver.Points(p.num("top")), Right: driver.Points(p.num("right")),
			Bottom: driver.Points(p.num("bottom")), Left: driver.Points(p.num("left"))}
		ev.Size = v.Frame.Size
	}
	app.Report(ev)
	return nil
}

// compatHears reports whether the platform reports kind to v: what its
// events ask for, and what a modal or a screen always hears.
func compatHears(v *nsgotest.View, kind driver.EventKind) bool {
	switch kind {
	case driver.EventBack, driver.EventRequestClose, driver.EventModalSize, driver.EventModalShown,
		driver.EventModalDismissed:
		return v.Kind == driver.KindModal || v.Events.Has(kind)
	case driver.EventScreenInsets:
		return v.Screen != (driver.ScreenConfig{})
	}
	return v.Events.Has(kind)
}

// compatPageOrigin is where v's top left corner is in the root view: its
// frames' origins less the scroll offsets around it.
func compatPageOrigin(v *nsgotest.View) driver.Point {
	var at driver.Point
	for c := v; c != nil; c = c.Parent {
		at.X += c.Frame.Origin.X
		at.Y += c.Frame.Origin.Y
		if p := c.Parent; p != nil && p.Kind == driver.KindScroll {
			at.X -= p.Offset.X
			at.Y -= p.Offset.Y
		}
	}
	return at
}

func compatPhase(handler string) driver.GesturePhase {
	if strings.HasSuffix(handler, "Start") {
		return driver.GestureBegan
	}
	return driver.GestureEnded
}

// compatSelectorOnly is selector as nsgotest reads it, without what
// moves from the view it names.
func compatSelectorOnly(selector string) string {
	sel, _, _ := compatSelector(selector)
	return sel
}
