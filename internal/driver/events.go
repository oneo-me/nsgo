package driver

import (
	"time"
)

// EventKind is what a platform reports about a view.
type EventKind uint8

const (
	// EventPress is a completed press: tap, click or accessibility action.
	EventPress EventKind = iota + 1
	// EventPressDown and EventPressUp bracket a press as it happens.
	EventPressDown
	EventPressUp
	// EventLongPress is a press held past the platform's delay.
	//
	// The four press events carry the position in the view (Point) and in
	// its root view (Page), the Modifiers held when the press began and
	// the click count (Count), where the platform knows them.
	EventLongPress
	// EventHoverIn and EventHoverOut follow a pointer over a view.
	EventHoverIn
	EventHoverOut
	// EventFocus and EventBlur follow keyboard focus.
	EventFocus
	EventBlur
	// EventChangeText carries a text field's text after a user edit
	// (Text) and the edit's number (Count).
	EventChangeText
	// EventSubmitEditing is Return in a text field (Text).
	EventSubmitEditing
	// EventScroll carries a scroll view's offset (Point), the
	// ItemScrollTo and ItemScrollBy items it applied since it was made
	// (Count) and, when it zooms, its zoom scale (Value).
	EventScroll
	// EventLoadStart, EventProgress (Loaded, Total), EventLoad (Size, the
	// source's pixels), EventError (Text) and EventLoadEnd follow an image
	// view's loading. A web view reports EventLoadStart and EventLoad with
	// its Navigation, and EventError with the URL in Navigation. A video
	// reports EventLoadStart, EventLoad (Duration and Size, in points),
	// EventProgress (Time and Duration) and EventError.
	EventLoadStart
	EventProgress
	EventLoad
	EventError
	EventLoadEnd
	// EventPressOutside is a press starting outside the overlay ID, at
	// Point in surface points.
	EventPressOutside
	// EventBack is Escape, the back button or gesture, or the browser's
	// back; the engine routes it to the topmost overlay, modal or stack.
	// With the ID of a screen it is that screen's header's back button or
	// swipe, which goes to the screen's stack while the screen is on top.
	EventBack
	// EventAlert answers ItemAlert Count with the button at Index, or -1.
	EventAlert
	// EventClipboard answers ItemReadClipboard Count with what it read:
	// the Text, the image in Data (a *ClipboardImage, nil for none), the
	// files' URIs, or whether the clipboard has text or an image
	// (Finished).
	EventClipboard
	// EventContextMenu is a secondary click (right click, control click)
	// on ID at Point in the view's coordinates and Page in its root
	// view's, with the Modifiers held.
	EventContextMenu
	// EventKey is a key press matching a binding of the keymap (Text).
	EventKey
	// EventPan follows an active pan: its phase (Phase), translation
	// (Point) and velocity (Velocity).
	EventPan
	// EventPinch and EventRotation follow a pinch's scale and a rotation's
	// angle in radians (Value) with their phase (Phase) and velocity
	// (Velocity.X).
	EventPinch
	EventRotation
	// EventMenu is an application menu item chosen (Count, its ID).
	EventMenu
	// EventMessage carries what a web view's page posted (Text).
	EventMessage
	// EventNavigationStateChange is a web view's page or history changing
	// (Navigation).
	EventNavigationStateChange
	// EventEnd is a video reaching its end, also when it repeats.
	EventEnd
	// EventBuffer is a video starting or stopping waiting for data
	// (Buffering).
	EventBuffer
	// EventFileDragEnter and EventFileDragLeave follow files dragged over
	// the view that would receive them, and EventFileDrop delivers them
	// (URIs).
	EventFileDragEnter
	EventFileDragLeave
	EventFileDrop
	// EventNative is an event a native view's component emitted: its
	// name (Text) and Data.
	EventNative
	// EventNativeSize is the size a native view's component asks for
	// (Size).
	EventNativeSize
	// EventAnimation reports an animation's end or a value read (Count,
	// the request), with Finished and the value's Value.
	EventAnimation
	// EventScrollDragStart and EventScrollDragEnd bracket the user
	// dragging a scroll view, its content or its scroll bar, and
	// EventMomentumStart and EventMomentumEnd the glide or snap after;
	// each carries the offset (Point), the scrolls applied (Count) and
	// zoom scale (Value) as EventScroll does.
	EventScrollDragStart
	EventScrollDragEnd
	EventMomentumStart
	EventMomentumEnd
	// EventRefresh is the user pulling a refreshable scroll view past
	// its top; the platform shows its spinner until a config says else.
	EventRefresh
	// EventImageInfo answers ItemImageInfo Count with the image's Size in
	// pixels, or the error in Text.
	EventImageInfo
	// EventSelectionChange carries a text field's selection after the
	// user moved it: Index to Count, in UTF-16 units.
	EventSelectionChange
	// EventKeyPress is a key typed in a text field (Text: what it enters,
	// "Backspace" or "Enter").
	EventKeyPress
	// EventCompositionChange follows an input method's composition in a
	// text field: the text being composed (Text) as it starts and
	// changes, then its end (Finished), after the EventChangeText of the
	// text it commits.
	EventCompositionChange
	// EventAccessibilityAction is an action assistive technology
	// performed on ID, by name (Text).
	EventAccessibilityAction
	// EventRequestClose asks to close the modal ID for the reason Text
	// ("swipe" for an iOS sheet); Finished says the platform closed it.
	EventRequestClose
	// EventModalSize is the Size and Insets of a modal the platform
	// presents at a size of its own, such as an iOS sheet.
	EventModalSize
	// EventHoverMove is the mouse moving over a hovered view with no
	// button down: Point, Page and Modifiers as EventContextMenu's.
	EventHoverMove
	// EventMouseMove is the mouse moving over the view, also with a
	// button down (Point, in the view).
	EventMouseMove
	// EventTouch reports the fingers on the surface (Touches, with their
	// PagePosition and Changed) as one goes down (Phase GestureBegan),
	// moves (GestureChanged), lifts (GestureEnded) or the system takes
	// them (GestureCancelled), at Time. Platforms send it while a view's
	// mask has it, with no ID, or with the ID of the view a finger went
	// down on, which then starts its path; a mouse is finger 0.
	EventTouch
	// EventCaretMove is where a focused input's caret is: Point and Size
	// in the input's view.
	EventCaretMove
	// EventPaste is what the user pasted into an input (Data, a
	// *PasteEvent), before the text it inserts is reported.
	EventPaste
	// EventScreenHeader is the height of a screen's system header bar
	// (Value), its large title shown, as the platform measured it; the
	// status bar above it is the core's to add.
	EventScreenHeader
	// EventOpenURL answers ItemOpenURL Count with whether the URL opened
	// (Finished).
	EventOpenURL
	// EventPick answers ItemPickImages Count with the files picked (Data,
	// a []PickedFile), none when the user cancelled.
	EventPick
	// EventMenuClosed answers ItemPopupMenu Count once its menu closed,
	// after the EventMenu of an item chosen.
	EventMenuClosed
	// EventBackStarted says a back gesture began, before its EventBack or
	// EventBackCancelled: the engine answers with ItemBackPreview and lays
	// the screen it would uncover out under the one it pops.
	EventBackStarted
	// EventBackCancelled says the back gesture EventBackStarted announced
	// was given up.
	EventBackCancelled
	// EventScrollEnd says a scroll view came to rest: a drag that set
	// nothing in motion ended, or a glide, a snap, an animated scroll or
	// the wheel's turns did; with the offset as EventScroll's.
	EventScrollEnd
	// EventFiles answers ItemPickFiles Count with the files picked (Data,
	// a []File), ItemPickDirectories Count with the folders picked (a
	// []Directory), none when the user cancelled, ItemSaveFile Count
	// with whether the file was saved (Finished), ItemOpenFile Count with
	// whether an app opened the file and ItemShare Count with whether the
	// share sheet showed; Text is what failed.
	EventFiles
	// EventModalShown and EventModalDismissed report that the platform
	// finished presenting modal ID and taking it away, where it says so.
	EventModalShown
	EventModalDismissed
	// EventScreenInsets reports what covers screen ID's content (Insets):
	// its header and the system's bars, where the platform measures them.
	EventScreenInsets
)

// EventMask is a set of event kinds.
type EventMask uint64

// Has reports whether m holds k.
func (m EventMask) Has(k EventKind) bool { return m&(1<<k) != 0 }

// Event is one report from the platform to the engine.
type Event struct {
	Kind  EventKind
	ID    NodeID
	Text  string
	Count uint32
	Point Point
	// Page is a pointer's position in the root view, the surface or the
	// modal, and Modifiers the keys held.
	Page      Point
	Modifiers Modifiers
	Size      Size
	// Loaded and Total are an image's bytes loaded and expected.
	Loaded, Total int64
	Index         int
	// Phase, Velocity and Value are a gesture's.
	Phase    GesturePhase
	Velocity Point
	Value    float32
	// Navigation is a web view's state.
	Navigation *WebNavigation
	// Time, Duration and Buffering are a video's.
	Time, Duration time.Duration
	Buffering      bool
	// URIs are the file: URLs of dropped files.
	URIs []string
	// Data is what a native component emitted.
	Data any
	// Finished is whether an animation ran to its end, or the platform
	// closed a modal.
	Finished bool
	// Insets are what system UI covers of a modal (EventModalSize).
	Insets Edges
	// Touches are EventTouch's fingers.
	Touches []Touch
	// Values are the values a pan moves as it begins and ends
	// (Animator.Gesture).
	Values Point
}

// Mergeable reports whether a later event of the same kind and view
// replaces this one before the engine handles it: scrolls, progress and
// mouse moves and a gesture's moves, never its first or last report.
func (e Event) Mergeable() bool {
	switch e.Kind {
	case EventScroll, EventProgress, EventHoverMove, EventMouseMove, EventCaretMove:
		return true
	case EventPan, EventPinch, EventRotation, EventTouch:
		return e.Phase == GestureChanged
	}
	return false
}

// Modifiers are the modifier keys held during an event.
type Modifiers struct {
	Control, Alt, Shift bool
	// Platform is Command on Apple platforms, the Windows or Super key
	// elsewhere.
	Platform bool
}

// PointerEvents says which of a view and its children take touches and
// the pointer.
type PointerEvents uint8

const (
	// PointerEventsAuto lets the view and its children take them.
	PointerEventsAuto PointerEvents = iota
	// PointerEventsNone lets neither take them: they reach what is below.
	PointerEventsNone
	// PointerEventsBoxNone lets only the children take them.
	PointerEventsBoxNone
	// PointerEventsBoxOnly lets only the view take them, never its
	// children.
	PointerEventsBoxOnly
)

// Reach reports what a point p, in the coordinates of a view's parent's
// content, may reach of a view at frame with props: the view itself, by
// its bounds and HitSlop, and its children, by its bounds and area, the
// one ItemOverflow gave it. Platforms that hit-test their own views ask
// it on the way down.
func Reach(frame Bounds, props *ViewProps, area Bounds, p Point) (self, children bool) {
	pe, slop := PointerEventsAuto, Edges{}
	if props != nil {
		pe, slop = props.PointerEvents, props.HitSlop
	}
	if pe == PointerEventsNone {
		return false, false
	}
	in := frame.Contains(p)
	if pe != PointerEventsBoxNone {
		self = in || slop != (Edges{}) && Bounds{
			Origin: Point{frame.Origin.X - slop.Left, frame.Origin.Y - slop.Top},
			Size:   Size{frame.Size.Width + slop.Horizontal(), frame.Size.Height + slop.Vertical()},
		}.Contains(p)
	}
	if pe != PointerEventsBoxOnly {
		children = in || area.Size.Width > 0 && area.Size.Height > 0 && Bounds{
			Origin: Point{frame.Origin.X + area.Origin.X, frame.Origin.Y + area.Origin.Y}, Size: area.Size,
		}.Contains(p)
	}
	return self, children
}

// GesturePhase is where a gesture is in its course.
type GesturePhase uint8

const (
	// GestureBegan is the first report, once the platform recognized it.
	GestureBegan GesturePhase = iota + 1
	// GestureChanged follows each move.
	GestureChanged
	// GestureEnded is the last report of a gesture that completed.
	GestureEnded
	// GestureCancelled is the last report of one the system took away,
	// such as by a scroll view or an incoming call.
	GestureCancelled
)

// PanActivation decides on the main thread when a drag on a view becomes
// its pan, so it can share touches with the scroll views around it, as
// react-native-gesture-handler's offsets do. A fail offset passed first
// leaves the touch to others; an active offset passed activates the pan.
// With no active offset the pan activates after 10 points either way.
// Zero fields are unset.
type PanActivation struct {
	ActiveOffsetX, ActiveOffsetY Points
	FailOffsetX, FailOffsetY     Points
}

// DefaultPanSlop is the distance a drag moves before it activates a pan
// without active offsets.
const DefaultPanSlop Points = 10

// Decide returns whether a drag translated by d activates the pan (1),
// fails it (-1) or is undecided (0). Platforms call it on each move until
// it decides.
func (a PanActivation) Decide(d Point) int {
	past := func(v, limit Points) bool { return limit != 0 && abs(v) >= abs(limit) }
	if past(d.X, a.FailOffsetX) || past(d.Y, a.FailOffsetY) {
		return -1
	}
	if a.ActiveOffsetX == 0 && a.ActiveOffsetY == 0 {
		if abs(d.X) >= DefaultPanSlop || abs(d.Y) >= DefaultPanSlop {
			return 1
		}
		return 0
	}
	if past(d.X, a.ActiveOffsetX) || past(d.Y, a.ActiveOffsetY) {
		return 1
	}
	return 0
}

func abs(v Points) Points {
	if v < 0 {
		return -v
	}
	return v
}

// Touch is one finger of a TouchEvent.
type Touch struct {
	// ID tells the finger while it is down.
	ID int
	// Position is in the view hearing the event, PagePosition in the
	// surface.
	Position     Point
	PagePosition Point
	// Changed says the event is about this finger: it went down, moved
	// or lifted.
	Changed bool
}

// PasteEvent is what the user pasted into an input, read from the
// clipboard as they pasted: its plain text (empty without any), files
// with their bytes, such as an image copied in another app, and the
// file: URLs of local files copied in a file manager.
type PasteEvent struct {
	Text  string
	Files []PastedFile
	URIs  []string
}

// PastedFile is a file the clipboard held: its name when it has one, its
// MIME type and its bytes.
type PastedFile struct {
	Name string
	MIME string
	Data []byte
}

// PasteBehavior says whether an input inserts the text the user pastes.
type PasteBehavior uint8

const (
	// PasteInsert inserts it, as inputs do.
	PasteInsert PasteBehavior = iota
	// PasteReport only reports it with OnPaste, for the app to handle.
	PasteReport
)
